package cred

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// CallbackSession manages a temporary loopback HTTP server for system browser
// OAuth/SSO callbacks and interactive Cookie/Token handoff forms.
type CallbackSession struct {
	Spec        Spec
	State       string
	CallbackURL string
	HandoffURL  string
	LaunchURL   string

	ln      net.Listener
	srv     *http.Server
	resCh   chan callbackResult
	closeFn sync.Once
	sendFn  sync.Once
}

type callbackResult struct {
	payload SecretPayload
	err     error
}

// StartCallbackServer binds an ephemeral 127.0.0.1 HTTP port with a random
// CSRF state token for browser-based authorization callbacks and manual handoff.
func StartCallbackServer(ctx context.Context, spec Spec) (*CallbackSession, error) {
	spec = spec.Normalized()
	stateBytes := make([]byte, 24)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, err
	}
	state := hex.EncodeToString(stateBytes)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("cred callback listen: %w", err)
	}
	baseURL := "http://" + ln.Addr().String()
	cbURL := baseURL + "/callback?state=" + url.QueryEscape(state)
	handoffURL := baseURL + "/handoff?state=" + url.QueryEscape(state)

	launch := spec.LoginURL
	if launch == "" {
		launch = spec.Origin
	}
	if strings.Contains(launch, "{callbackUrl}") || strings.Contains(launch, "{state}") {
		launch = strings.NewReplacer(
			"{callbackUrl}", url.QueryEscape(cbURL),
			"{state}", url.QueryEscape(state),
		).Replace(launch)
	} else if launch == "" {
		launch = handoffURL
	}

	cs := &CallbackSession{
		Spec:        spec,
		State:       state,
		CallbackURL: cbURL,
		HandoffURL:  handoffURL,
		LaunchURL:   launch,
		ln:          ln,
		resCh:       make(chan callbackResult, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", cs.handleCallback)
	mux.HandleFunc("/handoff", cs.handleHandoff)
	mux.HandleFunc("/", cs.handleHandoff)

	cs.srv = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if err := cs.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			cs.finish(SecretPayload{}, err)
		}
	}()

	if ctx != nil {
		go func() {
			<-ctx.Done()
			cs.finish(SecretPayload{}, ctx.Err())
			_ = cs.Close()
		}()
	}

	return cs, nil
}

// Wait blocks until credentials are received or ctx is cancelled.
func (cs *CallbackSession) Wait(ctx context.Context) (SecretPayload, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case res := <-cs.resCh:
		return res.payload, res.err
	case <-ctx.Done():
		return SecretPayload{}, ctx.Err()
	}
}

// Close shuts down the loopback callback server cleanly.
func (cs *CallbackSession) Close() error {
	var err error
	cs.closeFn.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if cs.srv != nil {
			err = cs.srv.Shutdown(ctx)
		} else if cs.ln != nil {
			err = cs.ln.Close()
		}
		cs.finish(SecretPayload{}, errors.New("browser authorization closed"))
	})
	return err
}

func (cs *CallbackSession) finish(p SecretPayload, err error) {
	cs.sendFn.Do(func() {
		cs.resCh <- callbackResult{payload: p, err: err}
	})
}

func (cs *CallbackSession) validState(got string) bool {
	if got == "" || cs.State == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(cs.State)) == 1
}

func (cs *CallbackSession) handleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		state := r.URL.Query().Get("state")
		if !cs.validState(state) {
			http.Error(w, "invalid state parameter", http.StatusForbidden)
			return
		}
		q := r.URL.Query()
		if errMsg := strings.TrimSpace(q.Get("error")); errMsg != "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(renderDoneHTML(false, cs.Spec, errMsg)))
			cs.finish(SecretPayload{}, fmt.Errorf("browser callback error: %s", errMsg))
			return
		}
		p, ok, err := cs.extractPayloadFromValues(q)
		if err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(renderDoneHTML(false, cs.Spec, err.Error())))
			return
		}
		if ok {
			p.Provider = ProviderBrowser
			p.UpdatedAt = time.Now().UnixMilli()
			cs.finish(p, nil)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(renderDoneHTML(true, cs.Spec, "Credential saved to ScriptBoard Keychain vault.")))
			return
		}
		// Support URL fragment OAuth callbacks (#access_token=... / #token=...).
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(renderFragmentRelayHTML(state)))
		return
	}

	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form body", http.StatusBadRequest)
			return
		}
		state := r.FormValue("state")
		if !cs.validState(state) {
			http.Error(w, "invalid state parameter", http.StatusForbidden)
			return
		}
		p, ok, err := cs.extractPayloadFromValues(r.PostForm)
		if err != nil || !ok {
			msg := "missing credential payload"
			if err != nil {
				msg = err.Error()
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(renderDoneHTML(false, cs.Spec, msg)))
			return
		}
		p.Provider = ProviderBrowser
		p.UpdatedAt = time.Now().UnixMilli()
		cs.finish(p, nil)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(renderDoneHTML(true, cs.Spec, "Credential saved to ScriptBoard Keychain vault.")))
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (cs *CallbackSession) handleHandoff(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		state := r.URL.Query().Get("state")
		if !cs.validState(state) {
			http.Error(w, "invalid state parameter", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(renderHandoffFormHTML(cs.Spec, state, "")))
		return
	}
	if r.Method == http.MethodPost {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form body", http.StatusBadRequest)
			return
		}
		state := r.FormValue("state")
		if !cs.validState(state) {
			http.Error(w, "invalid state parameter", http.StatusForbidden)
			return
		}
		format := strings.TrimSpace(r.FormValue("format"))
		raw := r.FormValue("payload")
		user := r.FormValue("username")
		pass := r.FormValue("password")
		p, err := ParseManualImport(cs.Spec, format, raw, user, pass, cs.Spec.HeaderName, cs.Spec.HeaderPrefix)
		if err != nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(renderHandoffFormHTML(cs.Spec, state, err.Error())))
			return
		}
		p.Provider = ProviderBrowser
		p.UpdatedAt = time.Now().UnixMilli()
		cs.finish(p, nil)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(renderDoneHTML(true, cs.Spec, "Imported and encrypted in OS Keychain. You can close this tab.")))
		return
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func (cs *CallbackSession) extractPayloadFromValues(vals url.Values) (SecretPayload, bool, error) {
	if ck := strings.TrimSpace(vals.Get("cookie")); ck != "" {
		p, err := ParseManualImport(cs.Spec, ImportCookie, ck, "", "", "", "")
		return p, err == nil, err
	}
	for _, key := range []string{"access_token", "token", "code"} {
		if tok := strings.TrimSpace(vals.Get(key)); tok != "" {
			if cs.Spec.Type == TypeCookie {
				// Cookie-type credential received token callback; wrap as Bearer unless cookie was provided.
				return SecretPayload{
					Type:         TypeBearer,
					Token:        stripBearerPrefix(tok),
					HeaderName:   "Authorization",
					HeaderPrefix: "Bearer ",
				}, true, nil
			}
			format := ImportBearer
			if cs.Spec.Type == TypeToken || cs.Spec.Type == TypeHeader {
				format = cs.Spec.Type
			}
			p, err := ParseManualImport(cs.Spec, format, tok, "", "", cs.Spec.HeaderName, cs.Spec.HeaderPrefix)
			return p, err == nil, err
		}
	}
	if auth := strings.TrimSpace(vals.Get("authorization")); auth != "" {
		p, err := ParseManualImport(cs.Spec, ImportToken, auth, "", "", "Authorization", "")
		return p, err == nil, err
	}
	if user := strings.TrimSpace(vals.Get("username")); user != "" {
		p, err := ParseManualImport(cs.Spec, ImportBasic, "", user, vals.Get("password"), "", "")
		return p, err == nil, err
	}
	return SecretPayload{}, false, nil
}

const paperInkPageCSS = `
:root{color-scheme:light dark;--bg:#F7F6F3;--card:#FFFFFF;--fg:#1D1D1F;--muted:#6E6E73;--border:rgba(29,29,31,.12);--accent:#2A7B62;--accent-fg:#fff;--err:#B33A3A}
@media(prefers-color-scheme:dark){:root{--bg:#161618;--card:#1E1E21;--fg:#F5F5F7;--muted:#A1A1A6;--border:rgba(245,245,247,.12);--accent:#3B9B7E}}
*{box-sizing:border-box}
body{margin:0;padding:40px 20px;font-family:-apple-system,BlinkMacSystemFont,"SF Pro Text","Helvetica Neue",sans-serif;background:var(--bg);color:var(--fg);display:flex;justify-content:center}
.card{width:100%;max-width:540px;background:var(--card);border:1px solid var(--border);border-radius:14px;padding:24px}
h1{margin:0 0 6px;font-size:18px;font-weight:650;letter-spacing:-.02em}
p.sub{margin:0 0 18px;font-size:13px;color:var(--muted);line-height:1.5}
.meta{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:12px;padding:8px 10px;border-radius:8px;border:1px solid var(--border);margin-bottom:16px;color:var(--muted)}
label{display:block;font-size:12px;font-weight:600;color:var(--muted);margin:12px 0 6px}
select,textarea,input{width:100%;font:inherit;font-size:13px;color:var(--fg);background:transparent;border:1px solid var(--border);border-radius:8px;padding:9px 11px}
textarea{min-height:110px;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;resize:vertical}
button{margin-top:16px;width:100%;padding:10px 14px;border:0;border-radius:8px;background:var(--accent);color:var(--accent-fg);font:inherit;font-weight:600;cursor:pointer}
.err{margin:10px 0 0;font-size:12px;color:var(--err)}
`

func renderHandoffFormHTML(spec Spec, state, errMsg string) string {
	title := spec.Name
	if title == "" {
		title = spec.ID
	}
	var errHTML string
	if errMsg != "" {
		errHTML = `<p class="err">` + html.EscapeString(errMsg) + `</p>`
	}
	return fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8"><title>ScriptBoard Credential Handoff</title><style>%s</style></head><body><main class="card">
<h1>Send Credential to ScriptBoard</h1>
<p class="sub">Complete login in your browser for <strong>%s</strong>, then paste Cookie / curl / Netscape cookies.txt / Token below.</p>
<div class="meta">ID: %s &middot; Origin: %s</div>
<form method="post" action="/handoff">
<input type="hidden" name="state" value="%s">
<label>Format</label>
<select name="format">
<option value="auto" selected>Auto-detect (Cookie / curl / Netscape / Token)</option>
<option value="cookie">Cookie Header</option>
<option value="curl">curl Command</option>
<option value="netscape">Netscape cookies.txt</option>
<option value="bearer">Bearer Token</option>
<option value="token">API Token / Custom Header</option>
</select>
<label>Credential Payload</label>
<textarea name="payload" placeholder="Cookie: session=... or curl 'https://...' -H 'Cookie: ...' or Netscape cookies.txt" required></textarea>
%s
<button type="submit">Save Encrypted Credential to ScriptBoard</button>
</form></main></body></html>`,
		paperInkPageCSS,
		html.EscapeString(title),
		html.EscapeString(spec.ID),
		html.EscapeString(spec.Origin),
		html.EscapeString(state),
		errHTML,
	)
}

func renderFragmentRelayHTML(state string) string {
	return fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8"><title>Completing Login…</title><style>%s</style></head><body><main class="card">
<h1>Completing Authorization…</h1>
<p class="sub" id="msg">Forwarding callback token to ScriptBoard.</p>
<script>
(function(){
  var hash = window.location.hash ? window.location.hash.substring(1) : "";
  if (!hash) { window.location.replace("/handoff?state=%s"); return; }
  var params = new URLSearchParams(hash);
  params.set("state", %q);
  fetch("/callback", { method: "POST", headers: {"Content-Type":"application/x-www-form-urlencoded"}, body: params.toString() })
    .then(function(r){ return r.text(); })
    .then(function(h){ document.open(); document.write(h); document.close(); })
    .catch(function(){ window.location.replace("/handoff?state=%s"); });
})();
</script></main></body></html>`, paperInkPageCSS, url.QueryEscape(state), state, url.QueryEscape(state))
}

func renderDoneHTML(ok bool, spec Spec, msg string) string {
	heading := "Credential Saved"
	if !ok {
		heading = "Authorization Failed"
	}
	return fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8"><title>%s</title><style>%s</style></head><body><main class="card">
<h1>%s</h1>
<div class="meta">ID: %s &middot; Origin: %s</div>
<p class="sub">%s</p>
</main></body></html>`,
		html.EscapeString(heading),
		paperInkPageCSS,
		html.EscapeString(heading),
		html.EscapeString(spec.ID),
		html.EscapeString(spec.Origin),
		html.EscapeString(msg),
	)
}
