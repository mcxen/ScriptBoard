package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/cred"
	"scriptboard/src/internal/store"
)

func TestAppScriptCredential401AutoRefreshAndRecovery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Cookie"), "token-portal-bridge_SSO_TOKEN_V3=sso-app-ok") {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"expired"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"credits": 420}`))
	}))
	defer srv.Close()

	a, err := newApp(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.closeBarInstances)
	var hookCalls atomic.Int32
	a.credLoginHook = func(ctx context.Context, sp cred.Spec) (cred.SecretPayload, error) {
		hookCalls.Add(1)
		u, _ := url.Parse(sp.Origin)
		return cred.SecretPayload{
			Type: cred.TypeCookie,
			Cookies: []cred.StoredCookie{
				{Name: "token-portal-bridge_SSO_TOKEN_V3", Value: "sso-app-ok", Domain: u.Hostname(), Path: "/"},
			},
			Provider: cred.ProviderWebView,
		}, nil
	}

	scriptSrc := `
export function meta() {
  return {
    name: "Token Hub",
    icon: "log-in",
    credentials: [{
      id: "token-hub",
      type: "cookie",
      origin: ` + `"` + srv.URL + `"` + `,
      provider: "webview",
      loginUrl: ` + `"` + srv.URL + `/bridge/usage/credits"` + `,
      ready: {
        urlIncludes: "/bridge/",
        hasCookies: ["token-portal-bridge_SSO_TOKEN_V3"]
      },
      refreshOn: [401, 403]
    }],
    sources: {
      credits: ` + `"` + srv.URL + `/bridge/usage/credits"` + `
    }
  };
}
export function render(ctx) {
  const d = ctx.sources.credits || {};
  const st = ctx.credentials["token-hub"] || {};
  return {
    title: "Token Hub",
    big: d.credits ? String(d.credits) : "--",
    sub: st.status || ""
  };
}
`
	sc := store.Script{
		ID:     "token-hub-card",
		Name:   "Token Hub",
		Source: scriptSrc,
		Home:   true,
	}
	if err := a.store.SaveScript(sc); err != nil {
		t.Fatal(err)
	}

	res, err := a.executeOpt(sc, nil, true)
	if err != nil {
		t.Fatalf("executeOpt failed: %v", err)
	}
	if res.Big != "420" {
		t.Fatalf("expected credits=420 after single-flight refresh & retry, got Big=%q Sub=%q", res.Big, res.Sub)
	}
	if hookCalls.Load() != 1 {
		t.Fatalf("expected 1 login hook call, got %d", hookCalls.Load())
	}

	// Second run uses cached Keychain vault secret without opening login window again.
	res2, err := a.executeOpt(sc, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if res2.Big != "420" || res2.Sub != "ready" {
		t.Fatalf("expected cached ready credential on rerun, got Big=%q Sub=%q", res2.Big, res2.Sub)
	}
	if hookCalls.Load() != 1 {
		t.Fatalf("expected login hook still at 1 call, got %d", hookCalls.Load())
	}
}

func TestAppBarCredentialIntegrationAndRecovery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Cookie"), "SSO=bar-ok") {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"expired"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"balance": 88}`))
	}))
	defer srv.Close()

	a, err := newApp(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.closeBarInstances)
	a.credLoginHook = func(ctx context.Context, sp cred.Spec) (cred.SecretPayload, error) {
		u, _ := url.Parse(sp.Origin)
		return cred.SecretPayload{
			Type: cred.TypeCookie,
			Cookies: []cred.StoredCookie{
				{Name: "SSO", Value: "bar-ok", Domain: u.Hostname(), Path: "/"},
			},
			Provider: cred.ProviderWebView,
		}, nil
	}

	barSrc := `<bar id="auth-bar" title="Auth Bar" version="1.0.0">
  <credentials>
    <credential id="bar-hub" type="cookie" origin="` + srv.URL + `" provider="webview" refresh-on="401,403" />
  </credentials>
  <script><![CDATA[
    let bal = "--";
    export default {
      async scheduled() {
        const res = await fetch("` + srv.URL + `/api/balance");
        if (res.ok) {
          const data = await res.json();
          bal = String(data.balance);
        }
      },
      render() {
        return { tray: { title: bal }, panel: h("Text", {}, "Balance:" + bal) };
      }
    };
  ]]></script>
</bar>`

	sc, err := a.importBarSource(barSrc)
	if err != nil {
		t.Fatal(err)
	}
	inst, _, err := a.ensureBarInstance(sc)
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close()
	if !inst.WaitIdle(5 * time.Second) {
		t.Fatal("bar instance WaitIdle timed out")
	}
	snap := inst.Snapshot()
	if snap.Tray == nil || snap.Tray.Title != "88" {
		t.Fatalf("expected bar tray title 88 after auth recovery, got %+v (err=%q)", snap.Tray, snap.Error)
	}
}

func TestCredentialsPageZHAndEnglishParity(t *testing.T) {
	t.Cleanup(func() { setLang(langZH) })
	a := newTestApp(t)

	_, err := a.credMgr.SaveSpec(cred.Spec{
		ID:       "token-hub",
		Name:     "Token Hub",
		Type:     cred.TypeCookie,
		Origin:   "https://token-hub.alibaba-inc.com",
		Provider: cred.ProviderWebView,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Render Chinese /credentials page.
	tt := ui.NewTester(a.view, 1100, 760)
	a.openCredentials()
	settle(tt)
	requireText(t, tt, "凭据管理")
	requireText(t, tt, "Token Hub")

	// Switch to English and verify zero leaked CJK text on /credentials.
	a.set.Lang = langEN
	setLang(langEN)
	settle(tt)
	requireText(t, tt, "Credentials")
	onlyEnglish(t, tt, "credentials")
}

func TestAppScriptCredentialTemplateAndRuntimeCookies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-SSO-Token") != "sso-tmpl-9" || r.Header.Get("X-CSRF") != "csrf-tmpl-2" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error":"missing interpolated credential headers"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"user":"alice","quota":95}`))
	}))
	defer srv.Close()

	a, err := newApp(t.TempDir(), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.closeBarInstances)

	_, err = a.credMgr.SaveSpec(cred.Spec{
		ID:       "my-auth",
		Name:     "My Auth",
		Type:     cred.TypeCookie,
		Provider: cred.ProviderManual,
		Cookies:  []string{"SSO", "csrf"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.credMgr.SaveSecret("my-auth", cred.SecretPayload{
		Type:     cred.TypeCookie,
		Provider: cred.ProviderManual,
		Cookies: []cred.StoredCookie{
			{Name: "SSO", Value: "sso-tmpl-9", Path: "/"},
			{Name: "csrf", Value: "csrf-tmpl-2", Path: "/"},
			{Name: "noise", Value: "drop", Path: "/"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	scriptSrc := `
export function meta() {
  return {
    name: "Template Cred",
    icon: "key-round",
    credentials: [{
      id: "my-auth",
      type: "cookie",
      cookies: ["SSO", "csrf"],
      provider: "manual"
    }],
    sources: {
      profile: "` + srv.URL + `/me"
    },
    sourceHeaders: {
      profile: {
        "X-SSO-Token": "{cred.my-auth.cookie.SSO}",
        "X-CSRF": "{cred.my-auth.cookie.csrf}"
      }
    }
  };
}
export function render(ctx) {
  const c = ctx.credentials["my-auth"] || {};
  const p = ctx.sources.profile || {};
  return {
    title: "Template Cred",
    big: p.quota ? String(p.quota) : "--",
    sub: (c.cookies && c.cookies.SSO ? c.cookies.SSO : "") + "|" + (c.status || "")
  };
}
`
	sc := store.Script{
		ID:     "tmpl-cred-card",
		Name:   "Template Cred",
		Source: scriptSrc,
		Home:   true,
	}
	if err := a.store.SaveScript(sc); err != nil {
		t.Fatal(err)
	}

	res, err := a.executeOpt(sc, nil, true)
	if err != nil {
		t.Fatalf("executeOpt failed: %v", err)
	}
	if res.Big != "95" || res.Sub != "sso-tmpl-9|ready" {
		t.Fatalf("expected interpolated cred headers & runtime cookies, got Big=%q Sub=%q", res.Big, res.Sub)
	}

	// Verify UI form save/edit with domains, pickCookies, and 302 refreshOn.
	a.credForm.open = true
	a.credForm.id = "ui-flex"
	a.credForm.name = "UI Flex"
	a.credForm.domains = ".example.com, .auth.example.com"
	a.credForm.pickCookies = "SSO, csrf"
	a.credForm.refreshOn = "302, 401"
	a.saveCredForm()
	rec, ok := a.credMgr.Get("ui-flex")
	if !ok {
		t.Fatalf("expected ui-flex spec saved via form, note=%q", a.credForm.note)
	}
	a.editCredRecord(rec)
	if a.credForm.pickCookies != "SSO, csrf" || !strings.Contains(a.credForm.domains, ".example.com") {
		t.Fatalf("expected editCredRecord to restore domains & pickCookies, got domains=%q pickCookies=%q", a.credForm.domains, a.credForm.pickCookies)
	}
}

func TestCredentialsPageCancelButtonAndEnglishParity(t *testing.T) {
	t.Cleanup(func() { setLang(langZH) })
	a := newTestApp(t)

	started := make(chan struct{}, 1)
	released := make(chan struct{}, 1)
	a.credMgr.SetHandlers(cred.LoginHandlers{
		Ego: func(ctx context.Context, sp cred.Spec) (cred.SecretPayload, error) {
			select {
			case started <- struct{}{}:
			default:
			}
			<-ctx.Done()
			select {
			case released <- struct{}{}:
			default:
			}
			return cred.SecretPayload{}, ctx.Err()
		},
	})

	_, err := a.credMgr.SaveSpec(cred.Spec{
		ID:       "ego-ui-cred",
		Name:     "Ego UI Cred",
		Type:     cred.TypeCookie,
		Origin:   "https://example.com",
		Provider: cred.ProviderEgo,
	})
	if err != nil {
		t.Fatal(err)
	}

	syncDone := make(chan error, 1)
	go func() {
		_, err := a.credMgr.SyncCookies(context.Background(), "ego-ui-cred", cred.ProviderEgo, "")
		syncDone <- err
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Ego sync to start")
	}

	tt := ui.NewTester(a.view, 1100, 760)
	a.openCredentials()
	settle(tt)
	requireText(t, tt, "登录中…")
	requireText(t, tt, "终止")

	// Verify English parity shows "Stop" with zero CJK text while authenticating.
	a.set.Lang = langEN
	setLang(langEN)
	settle(tt)
	requireText(t, tt, "Stop")
	onlyEnglish(t, tt, "credentials-authenticating")

	// Click Stop button in English mode -> must cancel in-flight sync and hide Stop button.
	if err := tt.Click("Stop"); err != nil {
		t.Fatalf("click Stop failed: %v", err)
	}
	select {
	case <-released:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Ego sync cancellation after clicking Stop")
	}
	<-syncDone
	settle(tt)
	requireText(t, tt, "Stopped credential sync Ego UI Cred")
}
