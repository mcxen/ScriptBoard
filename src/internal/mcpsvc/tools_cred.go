package mcpsvc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"scriptboard/src/internal/bar"
	"scriptboard/src/internal/cred"
	"scriptboard/src/internal/plugins"
	"scriptboard/src/internal/store"
)

func credTools() []Tool {
	str := func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	boolProp := func(desc string) map[string]any { return map[string]any{"type": "boolean", "description": desc} }
	intProp := func(desc string) map[string]any { return map[string]any{"type": "integer", "description": desc} }
	obj := func(props map[string]any, required ...string) map[string]any {
		return map[string]any{"type": "object", "properties": props, "required": required}
	}
	return []Tool{
		{
			Name:        "cred_list",
			Description: "List all stored and plugin/bar-declared credentials with readiness status, authorized Origin, provider, storage backend, and redacted metadata (never exposes secret values).",
			InputSchema: obj(map[string]any{}),
		},
		{
			Name:        "cred_get",
			Description: "Read one credential's specification, status, cookie names, redacted preview, and storage backend. Set includeSecrets=true to return decrypted runtime values (cookies/token/header) when authorized.",
			InputSchema: obj(map[string]any{
				"id":             str("credential id"),
				"includeSecrets": map[string]any{"type": "boolean", "description": "if true, include decrypted runtime cookies/token/header data (default false)"},
			}, "id"),
		},
		{
			Name:        "cred_save",
			Description: "Create or update a credential provider specification (type, origin, provider, loginUrl, ready condition, refreshOn status codes, CDP URL, etc.).",
			InputSchema: obj(map[string]any{
				"id":             str("credential id (lowercase letters, digits, hyphens, dots, underscores)"),
				"name":           str("optional display name"),
				"type":           str("credential type: cookie (default), bearer, token, basic, or header"),
				"origin":         str("authorized origin URL, e.g. https://token-hub.alibaba-inc.com"),
				"provider":       str("auth provider: webview (default), browser, manual, ego, or cdp"),
				"loginUrl":       str("optional sign-in URL (supports {callbackUrl} & {state} placeholders for browser)"),
				"ready":          map[string]any{"type": "object", "description": "optional readiness condition: {urlIncludes, hasCookies, titleIncludes}"},
				"refreshOn":      map[string]any{"type": "array", "items": map[string]any{"type": "integer"}, "description": "HTTP status codes triggering refresh (default [401, 403])"},
				"headerName":     str("header name for type=token/header (default Authorization)"),
				"headerPrefix":   str("optional header prefix (e.g. 'Bearer ')"),
				"cdpUrl":         str("optional Chromium CDP HTTP endpoint (default http://127.0.0.1:9222)"),
				"domains":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "optional cookie/request matching domains"},
				"allowedDomains": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "optional extra authorized cookie domains (alias of domains)"},
				"cookies":        map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "optional cookie names to retain (empty keeps all cookies)"},
				"cookieKey":      str("optional single cookie name mapped into headerName"),
			}, "id"),
		},
		{
			Name:        "cred_import",
			Description: "Manually import credential secrets (Cookie header, Netscape cookies.txt, curl command, Bearer token, custom header/token, or Basic auth) into the encrypted vault scoped strictly to the credential's authorized origin.",
			InputSchema: obj(map[string]any{
				"id":           str("existing credential id"),
				"format":       str("import format: auto (default), cookie, netscape, curl, bearer, token, basic, or header"),
				"input":        str("raw Cookie header, Netscape cookies.txt content, curl command, or token string"),
				"username":     str("username (for basic auth)"),
				"password":     str("password (for basic auth)"),
				"headerName":   str("optional header name override for token/header"),
				"headerPrefix": str("optional header prefix override"),
			}, "id"),
		},
		{
			Name:        "cred_delete",
			Description: "Clear stored secrets for a credential or delete the credential specification completely.",
			InputSchema: obj(map[string]any{
				"id":         str("credential id"),
				"secretOnly": boolProp("true to keep spec and wipe only encrypted secret payload; false (default) removes spec & secret"),
			}, "id"),
		},
		{
			Name:        "cred_sync_cdp",
			Description: "Sync origin-authorized cookies from a running ego-browser session or local Chromium CDP endpoint (default http://127.0.0.1:9222) and store encrypted in the vault. Tracked in Credential Manager and cancellable via cred_cancel or the UI Stop button.",
			InputSchema: obj(map[string]any{
				"id":         str("credential id"),
				"provider":   str("ego (default) or cdp"),
				"cdpUrl":     str("optional CDP endpoint override (e.g. http://127.0.0.1:9222)"),
				"timeoutSec": intProp("max seconds to wait for Ego/CDP cookie extraction (default 30, max 120)"),
			}, "id"),
		},
		{
			Name:        "cred_login",
			Description: "Trigger host authentication (webview, browser, ego, or cdp) for a credential inside the running ScriptBoard app. Wait defaults to true for ego/cdp and false (non-blocking) for interactive webview/browser unless wait=true.",
			InputSchema: obj(map[string]any{
				"id":         str("credential id"),
				"provider":   str("optional provider override: webview, browser, ego, or cdp"),
				"wait":       boolProp("true to block until login completes or times out"),
				"timeoutSec": intProp("max seconds to wait when wait=true (default 120)"),
			}, "id"),
		},
		{
			Name:        "cred_cancel",
			Description: "Cancel an in-flight credential login or Ego/CDP cookie sync. Omit id to cancel all active credential operations.",
			InputSchema: obj(map[string]any{
				"id": str("optional credential id; omit to cancel all active credential reads/logins"),
			}),
		},
	}
}

func (s Server) openCredManager() (*cred.Manager, error) {
	var mgr *cred.Manager
	if s.CredManager != nil {
		mgr = s.CredManager()
	}
	if mgr == nil {
		var err error
		mgr, err = cred.NewManager(s.DataDir, nil, cred.LoginHandlers{})
		if err != nil {
			return nil, err
		}
	}
	// Register declared specs from plugins & .bar components so list/get/import
	// always see up-to-date declarations even in standalone CLI mode.
	if st, err := store.Open(s.DataDir); err == nil {
		for _, sc := range st.Scripts() {
			if sc.IsBar() {
				if recs, ok := barCredsFromScript(sc.Source, sc.ID); ok {
					mgr.RegisterSpecs(sc.ID, recs)
				}
			} else if !sc.IsTable() {
				if meta, err := plugins.CheckSource(sc.ID+".js", sc.Source); err == nil && len(meta.Credentials) > 0 {
					mgr.RegisterSpecs(sc.ID, meta.Credentials)
				}
			}
		}
	}
	return mgr, nil
}

func (s Server) callCred(name string, a map[string]any) (string, bool, error) {
	return s.callCredCtx(context.Background(), name, a)
}

func (s Server) callCredCtx(reqCtx context.Context, name string, a map[string]any) (string, bool, error) {
	if reqCtx == nil {
		reqCtx = context.Background()
	}
	str := func(key string) string {
		v, _ := a[key].(string)
		return v
	}

	switch name {
	case "cred_list":
		mgr, err := s.openCredManager()
		if err != nil {
			return "", true, err
		}
		list := mgr.List()
		payload := map[string]any{
			"storageBackend": mgr.BackendName(),
			"count":          len(list),
			"credentials":    list,
		}
		b, _ := json.MarshalIndent(payload, "", "  ")
		return string(b), true, nil

	case "cred_get":
		id := strings.TrimSpace(str("id"))
		if id == "" {
			return "", true, errors.New("id 不能为空")
		}
		includeSecrets, _ := a["includeSecrets"].(bool)
		mgr, err := s.openCredManager()
		if err != nil {
			return "", true, err
		}
		rec, ok := mgr.Get(id)
		if !ok {
			return "", true, fmt.Errorf("没有凭据 %q", id)
		}
		if includeSecrets {
			rtData, _ := mgr.RuntimeDataOf(id)
			out := struct {
				cred.Record
				Runtime cred.RuntimeData `json:"runtime"`
			}{
				Record:  rec,
				Runtime: rtData,
			}
			b, _ := json.MarshalIndent(out, "", "  ")
			return string(b), true, nil
		}
		b, _ := json.MarshalIndent(rec, "", "  ")
		return string(b), true, nil

	case "cred_save":
		sp, err := parseCredSpecArgs(a)
		if err != nil {
			return "", true, err
		}
		mgr, err := s.openCredManager()
		if err != nil {
			return "", true, err
		}
		rec, err := mgr.SaveSpec(sp)
		if err != nil {
			return "", true, err
		}
		s.notifyChange()
		b, _ := json.MarshalIndent(rec, "", "  ")
		return string(b), true, nil

	case "cred_import":
		id := strings.TrimSpace(str("id"))
		if id == "" {
			return "", true, errors.New("id 不能为空")
		}
		mgr, err := s.openCredManager()
		if err != nil {
			return "", true, err
		}
		rec, err := mgr.ImportManual(
			id,
			str("format"),
			str("input"),
			str("username"),
			str("password"),
			str("headerName"),
			str("headerPrefix"),
		)
		if err != nil {
			return "", true, err
		}
		s.notifyChange()
		b, _ := json.MarshalIndent(rec, "", "  ")
		return string(b), true, nil

	case "cred_delete":
		id := strings.TrimSpace(str("id"))
		if id == "" {
			return "", true, errors.New("id 不能为空")
		}
		mgr, err := s.openCredManager()
		if err != nil {
			return "", true, err
		}
		if _, ok := mgr.Get(id); !ok {
			return "", true, fmt.Errorf("没有凭据 %q", id)
		}
		secretOnly, _ := a["secretOnly"].(bool)
		if secretOnly {
			if err := mgr.ClearSecret(id); err != nil {
				return "", true, err
			}
			s.notifyChange()
			return fmt.Sprintf("已清空凭据 %s 的密钥内容", id), true, nil
		}
		if err := mgr.Delete(id); err != nil {
			return "", true, err
		}
		s.notifyChange()
		return fmt.Sprintf("已删除凭据 %s", id), true, nil

	case "cred_sync_cdp":
		id := strings.TrimSpace(str("id"))
		if id == "" {
			return "", true, errors.New("id 不能为空")
		}
		mgr, err := s.openCredManager()
		if err != nil {
			return "", true, err
		}
		cu := strings.TrimSpace(str("cdpUrl"))
		prov := strings.ToLower(strings.TrimSpace(str("provider")))
		if prov == "" {
			prov = cred.ProviderEgo
		}
		timeoutSec := 30
		if ts, ok := a["timeoutSec"].(float64); ok && int(ts) > 0 {
			timeoutSec = min(int(ts), 120)
		}
		ctx, cancel := context.WithTimeout(reqCtx, time.Duration(timeoutSec)*time.Second)
		defer cancel()
		updated, err := mgr.SyncCookies(ctx, id, prov, cu)
		if err != nil {
			return "", true, err
		}
		s.notifyChange()
		b, _ := json.MarshalIndent(updated, "", "  ")
		return string(b), true, nil

	case "cred_login":
		id := strings.TrimSpace(str("id"))
		if id == "" {
			return "", true, errors.New("id 不能为空")
		}
		mgr, err := s.openCredManager()
		if err != nil {
			return "", true, err
		}
		rec, ok := mgr.Get(id)
		if !ok {
			return "", true, fmt.Errorf("没有凭据 %q", id)
		}
		sp := rec.Spec
		if p := strings.TrimSpace(str("provider")); p != "" {
			sp.Provider = strings.ToLower(p)
		}
		waitDefault := sp.Provider == cred.ProviderEgo || sp.Provider == cred.ProviderCDP
		waitFlag, hasWait := a["wait"].(bool)
		if !hasWait {
			waitFlag = waitDefault
		}
		timeoutSec := 120
		if ts, ok := a["timeoutSec"].(float64); ok && int(ts) > 0 {
			timeoutSec = min(int(ts), 600)
		}
		if !waitFlag {
			go func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				defer cancel()
				_, _ = mgr.Acquire(ctx, sp, 0, true)
			}()
			b, _ := json.MarshalIndent(map[string]any{
				"started":  true,
				"id":       sp.ID,
				"provider": sp.Provider,
				"origin":   sp.Origin,
			}, "", "  ")
			return string(b), true, nil
		}
		ctx, cancel := context.WithTimeout(reqCtx, time.Duration(timeoutSec)*time.Second)
		defer cancel()
		if _, err := mgr.Acquire(ctx, sp, 0, true); err != nil {
			return "", true, err
		}
		s.notifyChange()
		updated, _ := mgr.Get(id)
		b, _ := json.MarshalIndent(updated, "", "  ")
		return string(b), true, nil

	case "cred_cancel":
		mgr, err := s.openCredManager()
		if err != nil {
			return "", true, err
		}
		if id := strings.TrimSpace(str("id")); id != "" {
			cancelled := mgr.Cancel(id)
			b, _ := json.MarshalIndent(map[string]any{
				"cancelled": cancelled,
				"id":        id,
			}, "", "  ")
			return string(b), true, nil
		}
		count := mgr.CancelAll()
		b, _ := json.MarshalIndent(map[string]any{
			"cancelled": count > 0,
			"count":     count,
		}, "", "  ")
		return string(b), true, nil
	}
	return "", false, nil
}

func barCredsFromScript(source, _ string) ([]cred.Spec, bool) {
	sp, err := bar.Parse(source)
	if err != nil || len(sp.Credentials) == 0 {
		return nil, false
	}
	return sp.Credentials, true
}

func parseCredSpecArgs(a map[string]any) (cred.Spec, error) {
	str := func(k string) string {
		v, _ := a[k].(string)
		return strings.TrimSpace(v)
	}
	sp := cred.Spec{
		ID:         str("id"),
		Name:       str("name"),
		Type:       str("type"),
		Origin:     str("origin"),
		Provider:   str("provider"),
		LoginURL:   str("loginUrl"),
		HeaderName: str("headerName"),
		CDPURL:     str("cdpUrl"),
	}
	if hp, ok := a["headerPrefix"].(string); ok {
		sp.HeaderPrefix = hp
	}
	if rawReady, ok := a["ready"].(map[string]any); ok {
		rc := &cred.ReadyCondition{}
		if v, ok := rawReady["urlIncludes"].(string); ok {
			rc.URLIncludes = strings.TrimSpace(v)
		}
		if v, ok := rawReady["titleIncludes"].(string); ok {
			rc.TitleIncludes = strings.TrimSpace(v)
		}
		rc.HasCookies = toStringSlice(rawReady["hasCookies"])
		sp.Ready = rc
	}
	if rawCodes, ok := a["refreshOn"].([]any); ok {
		for _, item := range rawCodes {
			switch n := item.(type) {
			case float64:
				sp.RefreshOn = append(sp.RefreshOn, int(n))
			case string:
				if code, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
					sp.RefreshOn = append(sp.RefreshOn, code)
				}
			}
		}
	}
	sp.Cookies = toStringSlice(a["cookies"])
	if ck, ok := a["cookieKey"].(string); ok {
		sp.CookieKey = strings.TrimSpace(ck)
	}
	sp.Domains = toStringSlice(a["domains"])
	sp.AllowedDomains = toStringSlice(a["allowedDomains"])
	sp = sp.Normalized()
	return sp, sp.Validate()
}
