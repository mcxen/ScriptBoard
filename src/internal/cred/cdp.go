package cred

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

const (
	defaultCDPURL         = "http://127.0.0.1:9222"
	defaultEgoTimeout     = 30 * time.Second
	egoCookieOutputMarker = "__SB_EGO_COOKIES__:"
)

// Test hooks allowing unit tests to exercise FetchEgoCookies without launching
// an external ego-browser process.
var (
	findEgoCLI   = findEgoBrowserCLI
	runEgoCLICmd = runEgoCLIExec
)

// FetchEgoCookies extracts cookies matching spec from an active ego-browser
// session via `ego-browser nodejs -e ...` (`taskSpace` + `page.cdp`).
// If ego-browser CLI is absent, or fails when an explicit CDPURL is configured,
// it falls back to FetchCDPCookies while preserving the real Ego error.
func FetchEgoCookies(ctx context.Context, spec Spec) (SecretPayload, error) {
	spec = spec.Normalized()
	if ctx != nil && ctx.Err() != nil {
		return SecretPayload{}, ctx.Err()
	}

	bin := findEgoCLI()
	if bin != "" {
		p, egoErr := fetchViaEgoCLI(ctx, bin, spec)
		if egoErr == nil {
			return p, nil
		}
		if errors.Is(egoErr, context.Canceled) || (ctx != nil && ctx.Err() != nil) {
			return SecretPayload{}, egoErr
		}
		if strings.TrimSpace(spec.CDPURL) != "" {
			p, cdpErr := FetchCDPCookies(ctx, spec.CDPURL, spec)
			if cdpErr == nil {
				return p, nil
			}
			if errors.Is(cdpErr, context.Canceled) || (ctx != nil && ctx.Err() != nil) {
				return SecretPayload{}, cdpErr
			}
			return SecretPayload{}, fmt.Errorf("ego failed (%v); cdp fallback failed (%w)", egoErr, cdpErr)
		}
		return SecretPayload{}, egoErr
	}

	p, cdpErr := FetchCDPCookies(ctx, spec.CDPURL, spec)
	if cdpErr == nil {
		return p, nil
	}
	if errors.Is(cdpErr, context.Canceled) || (ctx != nil && ctx.Err() != nil) {
		return SecretPayload{}, cdpErr
	}
	return SecretPayload{}, fmt.Errorf("ego-browser CLI not found in PATH or ~/.local/bin, and CDP fallback failed: %w", cdpErr)
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

func egoTargetURLs(spec Spec) []string {
	seen := map[string]bool{}
	var urls []string
	addURL := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" || strings.Contains(raw, "{") {
			return
		}
		u, err := url.Parse(raw)
		if err != nil {
			return
		}
		scheme := strings.ToLower(u.Scheme)
		if (scheme != "http" && scheme != "https") || u.Host == "" {
			return
		}
		canonical := scheme + "://" + strings.ToLower(u.Host)
		if !seen[canonical] {
			seen[canonical] = true
			urls = append(urls, canonical)
		}
	}
	addURL(spec.Origin)
	addURL(spec.LoginURL)
	for _, dom := range spec.effectiveDomains() {
		host := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(dom)), ".")
		if host != "" && !strings.Contains(host, "/") {
			addURL("https://" + host)
		}
	}
	if urls == nil {
		return []string{}
	}
	return urls
}

func buildEgoCookieScript(spec Spec) (string, error) {
	targetURLs := egoTargetURLs(spec)
	wantNames := spec.Cookies
	if wantNames == nil {
		wantNames = []string{}
	}
	urlsJSON, err := json.Marshal(targetURLs)
	if err != nil {
		return "", err
	}
	namesJSON, err := json.Marshal(wantNames)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`
const targetUrls = %s;
const wantNames = %s;
const task = await taskSpace("sb-cred-" + Date.now() + "-" + Math.random().toString(36).slice(2, 7));
try {
  const page = task.page("p1");
  let cookies = [];
  if (targetUrls.length > 0) {
    const res = await page.cdp("Network.getCookies", { urls: targetUrls }, { timeout: 10000 });
    if (res && Array.isArray(res.cookies)) {
      cookies = res.cookies;
    }
  }
  const missingWanted = wantNames.length > 0 && wantNames.some((n) => !cookies.some((c) => c && c.name === n));
  if (cookies.length === 0 || missingWanted) {
    const all = await page.cdp("Network.getAllCookies", undefined, { timeout: 10000 });
    if (all && Array.isArray(all.cookies) && all.cookies.length > 0) {
      cookies = all.cookies;
    }
  }
  console.log(%q + JSON.stringify(cookies));
} finally {
  try {
    await task.finish({ keep: [] });
  } catch (_) {}
}
`, urlsJSON, namesJSON, egoCookieOutputMarker), nil
}

func runEgoCLIExec(ctx context.Context, bin, script string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, bin, "nodejs", "-e", script)
	cmd.WaitDelay = 2 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

func fetchViaEgoCLI(ctx context.Context, bin string, spec Spec) (SecretPayload, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if ctx.Err() != nil {
		return SecretPayload{}, ctx.Err()
	}
	runCtx, cancel := context.WithTimeout(ctx, defaultEgoTimeout)
	defer cancel()

	script, err := buildEgoCookieScript(spec)
	if err != nil {
		return SecretPayload{}, err
	}
	stdout, stderr, err := runEgoCLICmd(runCtx, bin, script)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
			return SecretPayload{}, fmt.Errorf("cred: ego cookie read cancelled: %w", context.Canceled)
		}
		if errors.Is(runCtx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			return SecretPayload{}, fmt.Errorf("cred: ego-browser cli timed out after %s: %w", defaultEgoTimeout, context.DeadlineExceeded)
		}
		detail := strings.TrimSpace(string(stderr))
		if detail == "" {
			detail = strings.TrimSpace(string(stdout))
		}
		if len(detail) > 400 {
			detail = detail[:400] + "..."
		}
		if detail != "" {
			return SecretPayload{}, fmt.Errorf("cred: ego-browser cli failed: %w (%s)", err, detail)
		}
		return SecretPayload{}, fmt.Errorf("cred: ego-browser cli failed: %w", err)
	}
	rawCookies, err := parseCDPCookiesJSON(stdout)
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
		if ctx.Err() != nil {
			return nil, fmt.Errorf("cred: CDP cookie read cancelled: %w", ctx.Err())
		}
		return nil, fmt.Errorf("cred: dial CDP websocket: %w", err)
	}
	defer ws.Close()
	stop := context.AfterFunc(ctx, func() { _ = ws.Close() })
	defer stop()
	if dl, ok := ctx.Deadline(); ok {
		_ = ws.SetDeadline(dl)
	}

	method := "Storage.getCookies"
	var params any
	if targetType == "page" && origin != "" {
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
		if ctx.Err() != nil {
			return nil, fmt.Errorf("cred: CDP cookie read cancelled: %w", ctx.Err())
		}
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
			if ctx.Err() != nil {
				return nil, fmt.Errorf("cred: CDP cookie read cancelled: %w", ctx.Err())
			}
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
	norm := strings.ReplaceAll(string(raw), "\r\n", "\n")
	lines := strings.Split(norm, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, egoCookieOutputMarker) {
			payload := strings.TrimSpace(strings.TrimPrefix(line, egoCookieOutputMarker))
			var out []cdpCookie
			if err := json.Unmarshal([]byte(payload), &out); err != nil {
				return nil, fmt.Errorf("cred: parse ego cookies output: %w", err)
			}
			return out, nil
		}
	}

	s := strings.TrimSpace(norm)
	var out []cdpCookie
	if json.Unmarshal([]byte(s), &out) == nil {
		return out, nil
	}
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			if json.Unmarshal([]byte(line), &out) == nil {
				return out, nil
			}
		}
	}
	return nil, fmt.Errorf("cred: parse ego cookies output: invalid JSON payload")
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
	hasScope := spec.Origin != "" || len(spec.effectiveDomains()) > 0
	selected := spec.SelectCookies(converted, hasScope)
	if len(selected) == 0 && len(spec.Cookies) > 0 && hasScope {
		// Retry without domain filtering when caller named explicit SSO cookies
		// that may live on a separate IdP/auth domain.
		selected = spec.SelectCookies(converted, false)
	}
	if len(selected) == 0 {
		target := spec.Origin
		if target == "" && len(spec.effectiveDomains()) > 0 {
			target = strings.Join(spec.effectiveDomains(), ",")
		}
		if target == "" {
			target = "all"
		}
		return SecretPayload{}, fmt.Errorf("cred: no matching cookies found in %s for %s (inspected %d cookies)", provider, target, len(raw))
	}
	return SecretPayload{
		Type:      TypeCookie,
		Cookies:   selected,
		Provider:  provider,
		UpdatedAt: time.Now().UnixMilli(),
	}, nil
}
