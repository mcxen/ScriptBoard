package cred

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

func TestVaultAESRoundTripLargeCookiePayload(t *testing.T) {
	dir := t.TempDir()
	mem := NewMemoryBackend()
	v, err := NewVault(dir, mem)
	if err != nil {
		t.Fatal(err)
	}

	// Build a >4KB cookie payload (larger than mygo.MaxSecretSize = 2560 bytes).
	bigVal := strings.Repeat("sso_secret_chunk_", 300) // ~5.1KB
	spec := Spec{
		ID:       "token-hub",
		Name:     "Token Hub",
		Type:     TypeCookie,
		Origin:   "https://token-hub.alibaba-inc.com",
		Provider: ProviderWebView,
		LoginURL: "https://token-hub.alibaba-inc.com/bridge/usage/credits",
		Ready: &ReadyCondition{
			URLIncludes: "/bridge/",
			HasCookies:  []string{"token-portal-bridge_SSO_TOKEN_V3"},
		},
		RefreshOn: []int{401, 403},
	}.Normalized()

	mgr, err := NewManager(dir, mem, LoginHandlers{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.SaveSpec(spec); err != nil {
		t.Fatal(err)
	}
	rec, err := mgr.SaveSecret("token-hub", SecretPayload{
		Type: TypeCookie,
		Cookies: []StoredCookie{
			{
				Name:     "token-portal-bridge_SSO_TOKEN_V3",
				Value:    bigVal,
				Domain:   "token-hub.alibaba-inc.com",
				Path:     "/",
				Secure:   true,
				HTTPOnly: true,
			},
		},
		Provider: ProviderWebView,
	})
	if err != nil {
		t.Fatal(err)
	}
	if rec.Status != StatusReady || !rec.HasSecret || rec.CookieCount != 1 {
		t.Fatalf("unexpected record summary: %+v", rec)
	}

	// Ensure plaintext metadata JSON contains zero secret values.
	metaBytes, err := os.ReadFile(filepath.Join(dir, "credentials.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(metaBytes), "sso_secret_chunk") {
		t.Fatalf("credentials.json leaked secret payload")
	}
	vaultBytes, err := os.ReadFile(filepath.Join(dir, "credentials.vault"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(vaultBytes), vaultMagic) || strings.Contains(string(vaultBytes), "sso_secret_chunk") {
		t.Fatalf("credentials.vault must be encrypted ciphertext with magic header")
	}

	// Re-open with same Keychain/Memory backend and verify decrypted secret.
	mgr2, err := NewManager(dir, mem, LoginHandlers{})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := mgr2.GetSecret("token-hub")
	if !ok || len(got.Cookies) != 1 || got.Cookies[0].Value != bigVal {
		t.Fatalf("reopened manager failed to decrypt secret: ok=%v", ok)
	}
	_ = v
}

func TestVaultUnsupportedBackendFallback(t *testing.T) {
	dir := t.TempDir()
	v, err := NewVault(dir, unsupportedBackend{})
	if err != nil {
		t.Fatal(err)
	}
	if err := v.SetSecret("demo", SecretPayload{Type: TypeBearer, Token: "tok-123456"}); err != nil {
		t.Fatal(err)
	}
	if v.BackendName() != "file-encrypted" {
		t.Fatalf("expected file-encrypted fallback, got %q", v.BackendName())
	}
}

type unsupportedBackend struct{}

func (unsupportedBackend) Name() string               { return "keychain" }
func (unsupportedBackend) Get(string) ([]byte, error) { return nil, errors.ErrUnsupported }
func (unsupportedBackend) Set(string, []byte) error   { return errors.ErrUnsupported }
func (unsupportedBackend) Delete(string) error        { return errors.ErrUnsupported }

func TestFilterAuthorizedCookiesAndReady(t *testing.T) {
	sp := Spec{
		ID:     "hub",
		Type:   TypeCookie,
		Origin: "https://token-hub.alibaba-inc.com",
		Ready: &ReadyCondition{
			URLIncludes: "/bridge/",
			HasCookies:  []string{"token-portal-bridge_SSO_TOKEN_V3"},
		},
	}.Normalized()

	filtered := sp.FilterAuthorizedCookies([]StoredCookie{
		{Name: "token-portal-bridge_SSO_TOKEN_V3", Value: "sso-ok", Domain: ".alibaba-inc.com"},
		{Name: "CSRF", Value: "csrf-1", Domain: "token-hub.alibaba-inc.com"},
		{Name: "STEAL", Value: "bad", Domain: "evil-alibaba-inc.com"},
		{Name: "OTHER", Value: "bad2", Domain: "google.com"},
	})
	if len(filtered) != 2 {
		t.Fatalf("expected 2 authorized cookies, got %d (%+v)", len(filtered), filtered)
	}
	if sp.Ready.Satisfied("https://login.alibaba-inc.com/", "Login", filtered) {
		t.Fatalf("expected urlIncludes guard to stay false on IdP page")
	}
	if !sp.Ready.Satisfied("https://token-hub.alibaba-inc.com/bridge/usage/credits", "Credits", filtered) {
		t.Fatalf("expected ReadyCondition to be satisfied on bridge URL with required cookie")
	}
}

func TestParseManualImports(t *testing.T) {
	sp := Spec{
		ID:     "demo",
		Type:   TypeCookie,
		Origin: "https://api.example.com",
	}.Normalized()

	// 1. Raw Cookie header
	p, err := ParseManualImport(sp, ImportAuto, "Cookie: sid=abc123; theme=dark", "", "", "", "")
	if err != nil || len(p.Cookies) != 2 {
		t.Fatalf("cookie header parse failed: %v %+v", err, p)
	}

	// 2. Netscape cookies.txt (filters unrelated domains + handles #HttpOnly_)
	netscape := strings.Join([]string{
		"# Netscape HTTP Cookie File",
		"#HttpOnly_.example.com\tTRUE\t/\tTRUE\t1999999999\tsso_token\tnetscape_val",
		"unrelated.org\tFALSE\t/\tFALSE\t1999999999\tevil\tnope",
	}, "\n")
	p2, err := ParseManualImport(sp, ImportAuto, netscape, "", "", "", "")
	if err != nil || len(p2.Cookies) != 1 || !p2.Cookies[0].HTTPOnly || p2.Cookies[0].Value != "netscape_val" {
		t.Fatalf("netscape parse failed: %v %+v", err, p2)
	}

	// 3. Multiline curl with Cookie header
	curlCmd := "curl 'https://api.example.com/me' \\\n  -H 'Cookie: session=curl_cookie_val; a=1' \\\n  -H 'Accept: application/json'"
	p3, err := ParseManualImport(sp, ImportAuto, curlCmd, "", "", "", "")
	if err != nil || len(p3.Cookies) != 2 || p3.Cookies[0].Value != "curl_cookie_val" {
		t.Fatalf("curl cookie parse failed: %v %+v", err, p3)
	}

	// 4. Bearer curl + direct Bearer + Basic Auth
	bearerSpec := Spec{ID: "ai", Type: TypeBearer, Origin: "https://api.example.com"}.Normalized()
	p4, err := ParseManualImport(bearerSpec, ImportAuto, "curl https://api.example.com -H 'Authorization: Bearer sk-xyz999'", "", "", "", "")
	if err != nil || p4.Token != "sk-xyz999" || p4.Type != TypeBearer {
		t.Fatalf("curl bearer failed: %v %+v", err, p4)
	}
	p5, err := ParseManualImport(bearerSpec, ImportBearer, "Bearer sk-raw777", "", "", "", "")
	if err != nil || p5.Token != "sk-raw777" {
		t.Fatalf("bearer import failed: %v %+v", err, p5)
	}
	basicSpec := Spec{ID: "ba", Type: TypeBasic, Origin: "https://api.example.com"}.Normalized()
	p6, err := ParseManualImport(basicSpec, ImportBasic, "", "alice", "s3cr3t", "", "")
	if err != nil || p6.Username != "alice" || p6.Password != "s3cr3t" {
		t.Fatalf("basic import failed: %v %+v", err, p6)
	}
}

func TestAuthTransportRefreshSingleFlightAndRetry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(r.Header.Get("Cookie"), "SSO=valid") {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"expired"}`))
			return
		}
		if r.Method == http.MethodPost && string(body) != `{"q":1}` {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"credits":420}`))
	}))
	defer srv.Close()

	var loginCalls atomic.Int32
	dir := t.TempDir()
	mgr, err := NewManager(dir, NewMemoryBackend(), LoginHandlers{
		WebView: func(ctx context.Context, sp Spec) (SecretPayload, error) {
			loginCalls.Add(1)
			time.Sleep(40 * time.Millisecond)
			u, _ := url.Parse(sp.Origin)
			return SecretPayload{
				Type: TypeCookie,
				Cookies: []StoredCookie{
					{Name: "SSO", Value: "valid", Domain: u.Hostname(), Path: "/"},
				},
				Provider: ProviderWebView,
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	sp := Spec{
		ID:        "hub",
		Type:      TypeCookie,
		Origin:    srv.URL,
		Provider:  ProviderWebView,
		RefreshOn: []int{401, 403},
	}.Normalized()
	mgr.RegisterSpecs("test-plugin", []Spec{sp})

	client := mgr.WrapClient(&http.Client{Timeout: 5 * time.Second}, []Spec{sp}, nil)

	// Fire 5 concurrent requests hitting 401 simultaneously -> WebView invoked ONCE.
	const workers = 5
	var wg sync.WaitGroup
	errs := make([]error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req, err := http.NewRequest(http.MethodPost, srv.URL+"/bridge/credits", strings.NewReader(`{"q":1}`))
			if err != nil {
				errs[idx] = err
				return
			}
			resp, err := client.Do(req)
			if err != nil {
				errs[idx] = err
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				errs[idx] = errors.New("expected 200 after retry, got " + resp.Status)
			}
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("worker %d failed: %v", i, err)
		}
	}
	if got := loginCalls.Load(); got != 1 {
		t.Fatalf("expected single-flight 1 login call, got %d", got)
	}
	rec, ok := mgr.Get("hub")
	if !ok || rec.Status != StatusReady || !rec.HasSecret {
		t.Fatalf("expected ready record after login: %+v", rec)
	}
}

func TestAuthTransportCooldownAfterCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	var loginCalls atomic.Int32
	mgr, err := NewManager(t.TempDir(), NewMemoryBackend(), LoginHandlers{
		WebView: func(ctx context.Context, sp Spec) (SecretPayload, error) {
			loginCalls.Add(1)
			return SecretPayload{}, errors.New("user cancelled login")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	mgr.Cooldown = 2 * time.Second
	sp := Spec{ID: "hub", Type: TypeCookie, Origin: srv.URL, Provider: ProviderWebView}.Normalized()
	mgr.RegisterSpecs("p", []Spec{sp})
	client := mgr.WrapClient(&http.Client{Timeout: 5 * time.Second}, []Spec{sp}, nil)

	for i := 0; i < 3; i++ {
		resp, err := client.Get(srv.URL + "/api")
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401 on cancelled login, got %d", resp.StatusCode)
		}
	}
	if got := loginCalls.Load(); got != 1 {
		t.Fatalf("expected cooldown to limit login attempts to 1, got %d", got)
	}
}

func TestBrowserCallbackSession(t *testing.T) {
	sp := Spec{
		ID:       "oauth-demo",
		Type:     TypeBearer,
		Origin:   "https://api.example.com",
		Provider: ProviderBrowser,
		LoginURL: "https://auth.example.com/authorize?redirect={callbackUrl}&state={state}",
	}.Normalized()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cb, err := StartCallbackServer(ctx, sp)
	if err != nil {
		t.Fatal(err)
	}
	defer cb.Close()

	if !strings.Contains(cb.LaunchURL, url.QueryEscape(cb.State)) {
		t.Fatalf("expected state in expanded launch URL: %s", cb.LaunchURL)
	}

	// Wrong state must be rejected.
	badResp, err := http.Get(strings.Replace(cb.CallbackURL, cb.State, "wrongstate", 1) + "&token=bad")
	if err != nil {
		t.Fatal(err)
	}
	_ = badResp.Body.Close()
	if badResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for invalid state, got %d", badResp.StatusCode)
	}

	// Valid callback completes wait cleanly.
	goodResp, err := http.Get(cb.CallbackURL + "&token=tok_callback_999")
	if err != nil {
		t.Fatal(err)
	}
	_ = goodResp.Body.Close()
	if goodResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on callback, got %d", goodResp.StatusCode)
	}

	payload, err := cb.Wait(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if payload.Token != "tok_callback_999" || payload.Provider != ProviderBrowser {
		t.Fatalf("unexpected callback payload: %+v", payload)
	}
}

func TestFetchCDPCookiesOriginFiltered(t *testing.T) {
	mux := http.NewServeMux()
	var srvURL string
	mux.Handle("/ws", websocket.Handler(func(ws *websocket.Conn) {
		var req map[string]any
		if err := websocket.JSON.Receive(ws, &req); err != nil {
			return
		}
		_ = websocket.JSON.Send(ws, map[string]any{
			"id": req["id"],
			"result": map[string]any{
				"cookies": []map[string]any{
					{"name": "SSO", "value": "cdp_ok", "domain": ".alibaba-inc.com", "path": "/", "httpOnly": true, "secure": true},
					{"name": "OTHER", "value": "leak", "domain": "unrelated.com", "path": "/"},
				},
			},
		})
	}))
	mux.HandleFunc("/json/list", func(w http.ResponseWriter, r *http.Request) {
		wsURL := "ws" + strings.TrimPrefix(srvURL, "http") + "/ws"
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"type": "page", "url": "https://token-hub.alibaba-inc.com/bridge", "webSocketDebuggerUrl": wsURL},
		})
	})

	srv := httptest.NewServer(mux)
	srvURL = srv.URL
	defer srv.Close()

	sp := Spec{
		ID:     "hub",
		Type:   TypeCookie,
		Origin: "https://token-hub.alibaba-inc.com",
		CDPURL: srv.URL,
	}.Normalized()

	p, err := FetchCDPCookies(context.Background(), "", sp)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Cookies) != 1 || p.Cookies[0].Name != "SSO" || p.Cookies[0].Value != "cdp_ok" {
		t.Fatalf("expected only authorized origin cookie, got %+v", p.Cookies)
	}
}

func TestSelectCookiesAndDeveloperEmpowerment(t *testing.T) {
	// Origin is optional; safe IDs and explicit cookie allowlists pass validation.
	sp := Spec{
		ID:      "dev-flexible_1",
		Type:    TypeCookie,
		Cookies: []string{"SSO", "csrf"},
	}.Normalized()
	if err := sp.Validate(); err != nil {
		t.Fatalf("expected optional-origin spec to validate cleanly, got %v", err)
	}

	raw := []StoredCookie{
		{Name: "SSO", Value: "tok_sso", Domain: ".sso.other.com", Path: "/"},
		{Name: "csrf", Value: "tok_csrf", Domain: ".app.example.com", Path: "/"},
		{Name: "noise", Value: "drop_me", Domain: ".sso.other.com", Path: "/"},
	}

	// filterDomain=false keeps developer-requested cookie names across SSO/redirect domains.
	picked := sp.SelectCookies(raw, false)
	if len(picked) != 2 || picked[0].Name != "SSO" || picked[1].Name != "csrf" {
		t.Fatalf("expected SSO & csrf without domain rejection, got %+v", picked)
	}

	// Empty Cookies keeps all cookies.
	allSpec := Spec{ID: "keep-all", Type: TypeCookie}.Normalized()
	allPicked := allSpec.SelectCookies(raw, false)
	if len(allPicked) != 3 {
		t.Fatalf("expected empty Cookies allowlist to keep all 3 cookies, got %d", len(allPicked))
	}

	// filterDomain=true applies Origin / Domains matching on top of Cookies.
	domSpec := Spec{
		ID:      "dom-scoped",
		Type:    TypeCookie,
		Origin:  "https://app.example.com",
		Domains: []string{".example.com"},
		Cookies: []string{"SSO", "csrf"},
	}.Normalized()
	domPicked := domSpec.SelectCookies(raw, true)
	if len(domPicked) != 1 || domPicked[0].Name != "csrf" {
		t.Fatalf("expected domain-filtered SelectCookies to keep csrf only, got %+v", domPicked)
	}
}

func TestManualImportAllowsMultiDomainWithoutOverDefense(t *testing.T) {
	sp := Spec{
		ID:      "multi-domain",
		Type:    TypeCookie,
		Origin:  "https://app.example.com",
		Cookies: []string{"SSO_LOGIN", "session"},
	}.Normalized()

	netscape := strings.Join([]string{
		"# Netscape HTTP Cookie File",
		"#HttpOnly_.login.vendor-sso.com\tTRUE\t/\tTRUE\t0\tSSO_LOGIN\tsso_val_1",
		".app.example.com\tFALSE\t/\tTRUE\t0\tsession\tsess_val_2",
		".tracking.example.com\tFALSE\t/\tFALSE\t0\tutm\tad_noise",
	}, "\n")

	p, err := ParseManualImport(sp, ImportNetscape, netscape, "", "", "", "")
	if err != nil {
		t.Fatalf("expected multi-domain Netscape import without domain error, got %v", err)
	}
	if len(p.Cookies) != 2 {
		t.Fatalf("expected 2 developer-selected cookies (SSO_LOGIN & session), got %+v", p.Cookies)
	}
}

func TestCookieKeyTemplateExpansionAnd302Refresh(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-SSO") != "Bearer sso_302_ok" {
			// Return 302 without Location so Go http.Client surfaces 302 directly to AuthTransport.
			w.WriteHeader(http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	mgr, err := NewManager(t.TempDir(), NewMemoryBackend(), LoginHandlers{
		WebView: func(ctx context.Context, sp Spec) (SecretPayload, error) {
			return SecretPayload{
				Type: TypeCookie,
				Cookies: []StoredCookie{
					{Name: "SSO", Value: "sso_302_ok", Path: "/"},
					{Name: "csrf", Value: "csrf_99", Path: "/"},
				},
				Provider: ProviderWebView,
			}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	sp := Spec{
		ID:           "sso-302",
		Type:         TypeCookie,
		Origin:       srv.URL,
		Provider:     ProviderWebView,
		Cookies:      []string{"SSO", "csrf"},
		CookieKey:    "SSO",
		HeaderName:   "X-SSO",
		HeaderPrefix: "Bearer ",
		RefreshOn:    []int{http.StatusFound, http.StatusUnauthorized},
	}.Normalized()
	if err := sp.Validate(); err != nil {
		t.Fatal(err)
	}

	client := mgr.WrapClient(&http.Client{}, []Spec{sp}, nil)
	resp, err := client.Get(srv.URL + "/api/me")
	if err != nil {
		t.Fatalf("client.Get failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 after 302 refresh + CookieKey header injection, got %d", resp.StatusCode)
	}

	expanded := mgr.ExpandTemplate("token={cred.sso-302.cookie.SSO};csrf={cred.sso-302.cookie.csrf};hdr={cred.sso-302.header}", []Spec{sp})
	want := "token=sso_302_ok;csrf=csrf_99;hdr=Bearer sso_302_ok"
	if expanded != want {
		t.Fatalf("ExpandTemplate mismatch:\n got: %q\nwant: %q", expanded, want)
	}
}
