package bar

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCookieJarMatchingPersistenceAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "cookies.json")
	jar, err := NewCookieJar(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := jar.Set(StoredCookie{
		Name:   "host_only",
		Value:  "hval",
		Domain: "example.com",
		Path:   "/",
	}); err != nil {
		t.Fatal(err)
	}
	if err := jar.Set(StoredCookie{
		Name:   "domain_ck",
		Value:  "dval",
		Domain: ".example.com",
		Path:   "/v1",
		Secure: true,
	}); err != nil {
		t.Fatal(err)
	}

	// Host match gets host-only and HTTPS domain cookie under /v1.
	u1, _ := url.Parse("https://example.com/v1/me")
	got1 := jar.Cookies(u1)
	if len(got1) != 2 {
		t.Fatalf("expected 2 cookies for https://example.com/v1/me, got %d: %+v", len(got1), got1)
	}

	// Subdomain over HTTPS gets domain cookie but not host-only cookie.
	u2, _ := url.Parse("https://api.example.com/v1/me")
	got2 := jar.Cookies(u2)
	if len(got2) != 1 || got2[0].Name != "domain_ck" {
		t.Fatalf("subdomain expected only domain_ck, got %+v", got2)
	}

	// HTTP must not receive Secure cookie; wrong path must not receive /v1 cookie.
	u3, _ := url.Parse("http://api.example.com/v1/me")
	if got := jar.Cookies(u3); len(got) != 0 {
		t.Fatalf("http must not receive Secure cookie, got %+v", got)
	}
	u4, _ := url.Parse("https://api.example.com/other")
	if got := jar.Cookies(u4); len(got) != 0 {
		t.Fatalf("wrong path must not receive /v1 cookie, got %+v", got)
	}

	// File permissions and persistence across reopen.
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Fatalf("cookies.json mode = %o, want 0600", perm)
		}
	}

	jar2, err := NewCookieJar(path)
	if err != nil {
		t.Fatal(err)
	}
	if jar2.Count() != 2 {
		t.Fatalf("reopened jar count = %d, want 2", jar2.Count())
	}

	// SetCookies with Max-Age=-1 deletes cookie and Reload propagates changes.
	jar2.SetCookies(u2, []*http.Cookie{{Name: "domain_ck", Value: "", MaxAge: -1, Path: "/v1", Domain: ".example.com"}})
	if err := jar.Reload(); err != nil {
		t.Fatal(err)
	}
	if jar.Count() != 1 {
		t.Fatalf("after Max-Age=-1 + Reload count = %d, want 1", jar.Count())
	}

	// SetHeader + ImportCurl + CloneInMemory + Delete/Clear + MaskValue.
	n, err := jar.SetHeader("https://service.example.com/app", "Cookie: alpha=123456789; beta=xy")
	if err != nil || n != 2 {
		t.Fatalf("SetHeader n=%d err=%v", n, err)
	}
	curlURL, cn, err := jar.ImportCurl(`curl 'https://curl.example.com/v2' -H 'Cookie: token=abcdef123456; role=admin'`)
	if err != nil || cn != 2 || curlURL != "https://curl.example.com/v2" {
		t.Fatalf("ImportCurl url=%q cn=%d err=%v", curlURL, cn, err)
	}
	if hdr := jar.HeaderFor("https://curl.example.com/v2"); !strings.Contains(hdr, "token=abcdef123456") {
		t.Fatalf("HeaderFor missing token: %q", hdr)
	}

	mem := jar.CloneInMemory()
	beforeCount := jar.Count()
	_ = mem.Clear("")
	if mem.Count() != 0 || jar.Count() != beforeCount {
		t.Fatalf("CloneInMemory should be isolated from disk jar (mem=%d, disk=%d)", mem.Count(), jar.Count())
	}

	if del, err := jar.DeleteForHost("curl.example.com", "role"); err != nil || del != 1 {
		t.Fatalf("DeleteForHost del=%d err=%v", del, err)
	}
	if err := jar.Clear("service.example.com"); err != nil {
		t.Fatal(err)
	}
	if hdr := jar.HeaderFor("https://service.example.com/app"); strings.Contains(hdr, "alpha=") {
		t.Fatalf("expected service.example.com cleared, got %q", hdr)
	}
	if err := jar.Clear(""); err != nil {
		t.Fatal(err)
	}
	if jar.Count() != 0 {
		t.Fatalf("expected all cookies cleared, got %d", jar.Count())
	}

	if got := MaskValue("abcdef123456"); !strings.HasPrefix(got, "ab…56") || strings.Contains(got, "cdef1234") {
		t.Fatalf("unexpected MaskValue(long): %q", got)
	}
	if got := MaskValue("short"); got != "***" {
		t.Fatalf("unexpected MaskValue(short): %q", got)
	}
}

func TestOriginAllowedWildcardSubdomains(t *testing.T) {
	perms := []string{"https://*.example.com", "https://exact.test:8443"}
	cases := []struct {
		raw  string
		want bool
	}{
		{"https://app.example.com/v1", true},
		{"https://v2.api.example.com:443/me", true},
		{"https://example.com/apex", false},
		{"http://app.example.com/wrong-scheme", false},
		{"https://evil-example.com/spoof", false},
		{"https://app.example.com:8443/wrong-port", false},
		{"https://exact.test:8443/ok", true},
		{"https://exact.test/wrong-port", false},
	}
	for _, tc := range cases {
		err := OriginAllowed(tc.raw, perms)
		if tc.want && err != nil {
			t.Errorf("OriginAllowed(%q) = %v, want nil", tc.raw, err)
		}
		if !tc.want && err == nil {
			t.Errorf("OriginAllowed(%q) = nil, want error", tc.raw)
		}
	}
}

func TestInstanceCookieJarStorageCopyAndActions(t *testing.T) {
	var seenCookies []string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			http.SetCookie(w, &http.Cookie{Name: "sid", Value: "secret999", Path: "/", HttpOnly: true})
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/me":
			c, err := r.Cookie("sid")
			mu.Lock()
			if err == nil {
				seenCookies = append(seenCookies, c.Value)
			} else {
				seenCookies = append(seenCookies, "MISSING")
			}
			mu.Unlock()
			if err != nil || c.Value != "secret999" {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"user":"alice","pct":75}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	makeXML := func(login bool) string {
		loginBlock := ""
		if login {
			loginBlock = fmt.Sprintf(`await fetch(%q + "/login");`, srv.URL)
		}
		return fmt.Sprintf(`<bar id="cookie-test" title="CookieTest" refresh="5m">
  <permissions>
    <network origin=%q />
  </permissions>
  <inputs>
    <input name="secret" type="password" label="Secret" default="tok123" />
  </inputs>
  <script><![CDATA[
    const h = (type, props, ...children) => ({
      type, props: props || {}, children: children.flat().filter(c => c !== null && c !== undefined && c !== false)
    });
    let user = "";
    let blockedErr = "";
    export default {
      async scheduled(env) {
        %s
        const resp = await fetch(%q + "/me");
        const body = await resp.json();
        user = body.user || "";
        $metrix.storage.set("user", user);
        localStorage.setItem("lsKey", "lsVal");
        try {
          $metrix.cookies.get("https://blocked.invalid/test");
        } catch (e) {
          blockedErr = String(e.message || e);
        }
        $metrix.invalidate();
      },
      render(env) {
        const lsCount = localStorage.length;
        const lsFirst = localStorage.key(0) || "";
        const cookieHdr = $metrix.cookies.header(%q + "/me");
        return {
          tray: {
            title: user || "--",
            menu: [{ label: "CopyCookie", onClick: () => $metrix.copy(cookieHdr) }]
          },
          panel: h("Column", { preferredWidth: 280 },
            h("Badge", {}, user ? "Live" : "Wait"),
            h("Progress", { value: 75, max: 100 }),
            h("Text", {}, "ls:" + lsCount + ":" + lsFirst + "|blk:" + (blockedErr ? "yes" : "no")),
            h("Button", { variant: "primary", onClick: () => $metrix.copy("copied-" + user) }, "Copy User")
          )
        };
      }
    };
  ]]></script>
</bar>`, srv.URL, loginBlock, srv.URL, srv.URL)
	}

	dataDir := t.TempDir()
	spec, err := Parse(makeXML(true))
	if err != nil {
		t.Fatal(err)
	}

	var copied []string
	var copyMu sync.Mutex
	inst1, err := NewInstance(spec, Options{
		DataDir: dataDir,
		OnCopyText: func(txt string) {
			copyMu.Lock()
			copied = append(copied, txt)
			copyMu.Unlock()
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !inst1.WaitIdle(5 * time.Second) {
		inst1.Close()
		t.Fatal("inst1 WaitIdle timed out")
	}

	snap1 := inst1.Snapshot()
	if snap1.Tray == nil || snap1.Tray.Title != "alice" {
		inst1.Close()
		t.Fatalf("inst1 tray = %+v, want title alice", snap1.Tray)
	}
	actions := CollectActions(snap1)
	var btnID string
	for _, act := range actions {
		if act.Type == "Button" {
			btnID = act.ID
			break
		}
	}
	if btnID == "" {
		inst1.Close()
		t.Fatalf("expected clickable Button action, got %+v", actions)
	}
	if _, ok := inst1.InvokeActionSync(btnID, 3*time.Second); !ok {
		inst1.Close()
		t.Fatal("InvokeActionSync returned false")
	}
	copyMu.Lock()
	if len(copied) == 0 || copied[0] != "copied-alice" {
		copyMu.Unlock()
		inst1.Close()
		t.Fatalf("copied = %+v, want [copied-alice]", copied)
	}
	copyMu.Unlock()

	if val, ok := inst1.Storage().Get("user"); !ok || val != "alice" {
		inst1.Close()
		t.Fatalf("storage user = %q (%v), want alice", val, ok)
	}
	if inst1.Cookies().Count() == 0 {
		inst1.Close()
		t.Fatal("expected Set-Cookie sid persisted in inst1.Cookies()")
	}
	inst1.Close()

	// Boot second instance without /login: it must send persisted cookie from cookies.json and read storage.json.
	spec2, err := Parse(makeXML(false))
	if err != nil {
		t.Fatal(err)
	}
	inst2, err := NewInstance(spec2, Options{DataDir: dataDir})
	if err != nil {
		t.Fatal(err)
	}
	defer inst2.Close()
	if !inst2.WaitIdle(5 * time.Second) {
		t.Fatal("inst2 WaitIdle timed out")
	}
	snap2 := inst2.Snapshot()
	if snap2.Tray == nil || snap2.Tray.Title != "alice" {
		t.Fatalf("inst2 failed to reuse persisted cookie/storage: %+v (err=%q)", snap2.Tray, snap2.Error)
	}
	if val, ok := inst2.Storage().Get("lsKey"); !ok || val != "lsVal" {
		t.Fatalf("inst2 localStorage value = %q (%v), want lsVal", val, ok)
	}

	mu.Lock()
	if len(seenCookies) < 2 || seenCookies[0] != "secret999" || seenCookies[1] != "secret999" {
		t.Fatalf("server saw cookies %v, want [secret999, secret999]", seenCookies)
	}
	mu.Unlock()
}

func TestCompileCheckValidation(t *testing.T) {
	badJS := `<bar id="bad-js" title="Bad"><script><![CDATA[export default { render( { ];]]></script></bar>`
	if _, _, err := CompileCheck(badJS); err == nil {
		t.Fatal("expected CompileCheck error on syntax error")
	}
	noRender := `<bar id="no-render" title="NoRender"><script><![CDATA[export default { foo: 1 };]]></script></bar>`
	if _, _, err := CompileCheck(noRender); err == nil {
		t.Fatal("expected CompileCheck error when render is missing")
	}
	validLongTitle := `<bar id="ok-bar" title="OK">
  <permissions><network origin="not-a-valid-origin"/></permissions>
  <script><![CDATA[
    export default {
      render() { return { tray: { title: "123456789012345" } }; }
    };
  ]]></script>
</bar>`
	sp, warnings, err := CompileCheck(validLongTitle)
	if err != nil {
		t.Fatalf("unexpected CompileCheck error: %v", err)
	}
	if sp.ID != "ok-bar" || len(warnings) < 2 {
		t.Fatalf("expected warnings for invalid permission & long title, got sp=%+v warnings=%v", sp.ID, warnings)
	}
}
