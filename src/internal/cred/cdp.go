package cred

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/net/websocket"
)

const defaultCDPURL = "http://127.0.0.1:9222"

// FetchEgoCookies attempts to extract origin-scoped cookies from an active
// ego-browser session via `ego-browser nodejs -e ...` using CDP, falling back
// transparently to FetchCDPCookies if the CLI is absent or fails.
func FetchEgoCookies(ctx context.Context, spec Spec) (SecretPayload, error) {
	spec = spec.Normalized()
	bin := findEgoBrowserCLI()
	if bin != "" {
		p, err := fetchViaEgoCLI(ctx, bin, spec)
		if err == nil {
			return p, nil
		}
	}
	return FetchCDPCookies(ctx, spec.CDPURL, spec)
}

func findEgoBrowserCLI() string {
	if p, err := exec.LookPath("ego-browser"); err == nil && p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	candidate := filepath.Join(home, ".local", "bin", "ego-browser")
	if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
		return candidate
	}
	return ""
}

func fetchViaEgoCLI(ctx context.Context, bin string, spec Spec) (SecretPayload, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	runCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Origin is passed via environment variable SB_CRED_ORIGIN (no JS string interpolation).
	script := `
const origin = process.env.SB_CRED_ORIGIN;
const extract = async () => {
  if (typeof task !== "undefined" && task && typeof task.cdp === "function") {
    const res = await task.cdp("Network.getCookies", { urls: [origin] });
    return res && res.cookies ? res.cookies : [];
  }
  if (typeof page !== "undefined" && page && typeof page.cdp === "function") {
    const res = await page.cdp("Network.getCookies", { urls: [origin] });
    return res && res.cookies ? res.cookies : [];
  }
  return [];
};
const cookies = await extract();
console.log(JSON.stringify(cookies));
`
	cmd := exec.CommandContext(runCtx, bin, "nodejs", "-e", script)
	cmd.Env = append(os.Environ(), "SB_CRED_ORIGIN="+spec.Origin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return SecretPayload{}, fmt.Errorf("ego-browser cli: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	rawCookies, err := parseCDPCookiesJSON(stdout.Bytes())
	if err != nil {
		return SecretPayload{}, err
	}
	return finalizeCDPCookies(spec, ProviderEgo, rawCookies)
}

// FetchCDPCookies connects to a Chromium DevTools HTTP/WebSocket endpoint
// (default http://127.0.0.1:9222) and extracts cookies scoped strictly to
// spec.Origin (and spec.AllowedDomains). It never reads SQLite files directly.
func FetchCDPCookies(ctx context.Context, cdpURL string, spec Spec) (SecretPayload, error) {
	spec = spec.Normalized()
	base := strings.TrimRight(strings.TrimSpace(cdpURL), "/")
	if base == "" {
		base = strings.TrimRight(strings.TrimSpace(spec.CDPURL), "/")
	}
	if base == "" {
		base = defaultCDPURL
	}
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return SecretPayload{}, fmt.Errorf("cred: invalid CDP endpoint %q", base)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	reqCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	wsURL, targetType, err := discoverCDPWebSocketURL(reqCtx, base, spec.Origin)
	if err != nil {
		return SecretPayload{}, err
	}

	rawCookies, err := queryCDPCookiesWS(reqCtx, wsURL, targetType, spec.Origin)
	if err != nil {
		return SecretPayload{}, err
	}
	return finalizeCDPCookies(spec, ProviderCDP, rawCookies)
}

type cdpTarget struct {
	Type                 string `json:"type"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

func discoverCDPWebSocketURL(ctx context.Context, base, origin string) (string, string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	listReq, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/json/list", nil)
	if err == nil {
		if resp, err := client.Do(listReq); err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
				var targets []cdpTarget
				if json.Unmarshal(body, &targets) == nil {
					for _, t := range targets {
						if t.Type == "page" && t.WebSocketDebuggerURL != "" && strings.HasPrefix(strings.ToLower(t.URL), strings.ToLower(origin)) {
							return t.WebSocketDebuggerURL, "page", nil
						}
					}
					for _, t := range targets {
						if t.Type == "page" && t.WebSocketDebuggerURL != "" {
							return t.WebSocketDebuggerURL, "page", nil
						}
					}
					for _, t := range targets {
						if t.WebSocketDebuggerURL != "" {
							return t.WebSocketDebuggerURL, t.Type, nil
						}
					}
				}
			}
		}
	}

	verReq, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/json/version", nil)
	if err != nil {
		return "", "", err
	}
	resp, err := client.Do(verReq)
	if err != nil {
		return "", "", fmt.Errorf("cred: CDP endpoint unreachable at %s (start Chromium with --remote-debugging-port=9222): %w", base, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	var ver struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.Unmarshal(body, &ver); err != nil || ver.WebSocketDebuggerURL == "" {
		return "", "", fmt.Errorf("cred: no CDP webSocketDebuggerUrl returned by %s", base)
	}
	return ver.WebSocketDebuggerURL, "browser", nil
}

type cdpCookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	Secure   bool    `json:"secure"`
	HTTPOnly bool    `json:"httpOnly"`
	SameSite string  `json:"sameSite"`
}

func queryCDPCookiesWS(ctx context.Context, wsURL, targetType, origin string) ([]cdpCookie, error) {
	cfg, err := websocket.NewConfig(wsURL, "http://127.0.0.1")
	if err != nil {
		return nil, err
	}
	ws, err := cfg.DialContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("cred: dial CDP websocket: %w", err)
	}
	defer ws.Close()
	if dl, ok := ctx.Deadline(); ok {
		_ = ws.SetDeadline(dl)
	}

	method := "Storage.getCookies"
	var params any
	if targetType == "page" {
		method = "Network.getCookies"
		params = map[string]any{"urls": []string{origin}}
	}
	reqMsg := map[string]any{
		"id":     1,
		"method": method,
	}
	if params != nil {
		reqMsg["params"] = params
	}
	if err := websocket.JSON.Send(ws, reqMsg); err != nil {
		return nil, fmt.Errorf("cred: send CDP %s: %w", method, err)
	}

	for {
		var resp struct {
			ID     int `json:"id"`
			Result struct {
				Cookies []cdpCookie `json:"cookies"`
			} `json:"result"`
			Error *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := websocket.JSON.Receive(ws, &resp); err != nil {
			return nil, fmt.Errorf("cred: read CDP response: %w", err)
		}
		if resp.ID != 1 {
			continue // skip async CDP events
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("cred: CDP %s error %d: %s", method, resp.Error.Code, resp.Error.Message)
		}
		return resp.Result.Cookies, nil
	}
}

func parseCDPCookiesJSON(raw []byte) ([]cdpCookie, error) {
	s := strings.TrimSpace(string(raw))
	if idx := strings.LastIndex(s, "["); idx >= 0 {
		if end := strings.LastIndex(s, "]"); end > idx {
			s = s[idx : end+1]
		}
	}
	var out []cdpCookie
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, fmt.Errorf("cred: parse ego cookies output: %w", err)
	}
	return out, nil
}

func finalizeCDPCookies(spec Spec, provider string, raw []cdpCookie) (SecretPayload, error) {
	converted := make([]StoredCookie, 0, len(raw))
	for _, c := range raw {
		var exp int64
		if c.Expires > 0 {
			exp = int64(c.Expires)
		}
		converted = append(converted, StoredCookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  exp,
			Secure:   c.Secure,
			HTTPOnly: c.HTTPOnly,
			SameSite: c.SameSite,
		})
	}
	filtered := spec.FilterAuthorizedCookies(converted)
	if len(filtered) == 0 {
		return SecretPayload{}, fmt.Errorf("cred: no authorized cookies found in %s for %s", provider, spec.Origin)
	}
	if spec.Ready != nil && len(spec.Ready.HasCookies) > 0 && !spec.Ready.Satisfied(spec.Origin, "", filtered) {
		return SecretPayload{}, fmt.Errorf("cred: browser session is missing required cookies (%s) for %s", strings.Join(spec.Ready.HasCookies, ", "), spec.Origin)
	}
	return SecretPayload{
		Type:      TypeCookie,
		Cookies:   filtered,
		Provider:  provider,
		UpdatedAt: time.Now().UnixMilli(),
	}, nil
}
