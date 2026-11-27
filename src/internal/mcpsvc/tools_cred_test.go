package mcpsvc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"scriptboard/src/internal/cred"
)

func TestCredMCPToolsLifecycleAndRedaction(t *testing.T) {
	dir := t.TempDir()
	srv := Server{DataDir: dir}

	// 1. Save credential spec.
	saveOut, handled, err := srv.callCred("cred_save", map[string]any{
		"id":       "token-hub",
		"name":     "Token Hub",
		"type":     "cookie",
		"origin":   "https://token-hub.alibaba-inc.com",
		"provider": "webview",
		"loginUrl": "https://token-hub.alibaba-inc.com/bridge/usage/credits",
		"ready": map[string]any{
			"urlIncludes": "/bridge/",
			"hasCookies":  []any{"token-portal-bridge_SSO_TOKEN_V3"},
		},
		"refreshOn": []any{float64(401), float64(403)},
	})
	if err != nil || !handled {
		t.Fatalf("cred_save failed: handled=%v err=%v", handled, err)
	}
	if !strings.Contains(saveOut, `"id": "token-hub"`) {
		t.Fatalf("unexpected cred_save output: %s", saveOut)
	}

	// 2. Import cookie header manually -> secret is redacted in outputs!
	secretVal := "SUPER_SECRET_SSO_VALUE_DO_NOT_LEAK"
	impOut, _, err := srv.callCred("cred_import", map[string]any{
		"id":     "token-hub",
		"format": "cookie",
		"input":  "token-portal-bridge_SSO_TOKEN_V3=" + secretVal + "; csrftoken=xyz",
	})
	if err != nil {
		t.Fatalf("cred_import failed: %v", err)
	}
	if strings.Contains(impOut, secretVal) {
		t.Fatalf("cred_import leaked raw secret in output: %s", impOut)
	}
	if !strings.Contains(impOut, `"status": "ready"`) || !strings.Contains(impOut, `"cookieCount": 2`) {
		t.Fatalf("expected ready & 2 cookies, got: %s", impOut)
	}

	// 3. List & Get must never leak secret values.
	listOut, _, err := srv.callCred("cred_list", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(listOut, secretVal) || !strings.Contains(listOut, "token-portal-bridge_SSO_TOKEN_V3") {
		t.Fatalf("unexpected cred_list output: %s", listOut)
	}

	getOut, _, err := srv.callCred("cred_get", map[string]any{"id": "token-hub"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(getOut, secretVal) {
		t.Fatalf("cred_get leaked raw secret")
	}

	// 4. ReadOnly server refuses mutating cred tools.
	roSrv := Server{DataDir: dir, ReadOnly: func() bool { return true }}
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"cred_import","arguments":{"id":"token-hub","input":"a=1"}}}`
	var respObj map[string]any
	_ = json.Unmarshal(roSrv.Handle([]byte(req)), &respObj)
	resMap, _ := respObj["result"].(map[string]any)
	if isErr, _ := resMap["isError"].(bool); !isErr {
		t.Fatalf("expected ReadOnly server to reject cred_import: %s", string(roSrv.Handle([]byte(req))))
	}

	// 5. Clear secret only, then delete completely.
	if _, _, err := srv.callCred("cred_delete", map[string]any{"id": "token-hub", "secretOnly": true}); err != nil {
		t.Fatal(err)
	}
	get2, _, err := srv.callCred("cred_get", map[string]any{"id": "token-hub"})
	if err != nil || !strings.Contains(get2, `"status": "`+cred.StatusMissing+`"`) {
		t.Fatalf("expected missing status after secretOnly delete, got %s (%v)", get2, err)
	}
	if _, _, err := srv.callCred("cred_delete", map[string]any{"id": "token-hub"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := srv.callCred("cred_get", map[string]any{"id": "token-hub"}); err == nil {
		t.Fatalf("expected error after deleting credential")
	}
}

func TestCredMCPGetIncludeSecretsAndSaveFields(t *testing.T) {
	dir := t.TempDir()
	srv := Server{DataDir: dir}

	// Origin is optional; save with domains, cookies, cookieKey, headerName, refreshOn [302, 401].
	_, handled, err := srv.callCred("cred_save", map[string]any{
		"id":           "flex-cred",
		"name":         "Flex Cred",
		"type":         "cookie",
		"provider":     "manual",
		"domains":      []any{".example.com"},
		"cookies":      []any{"sid", "csrf"},
		"cookieKey":    "sid",
		"headerName":   "X-Sid",
		"headerPrefix": "Bearer ",
		"refreshOn":    []any{float64(302), float64(401)},
	})
	if err != nil || !handled {
		t.Fatalf("cred_save failed: handled=%v err=%v", handled, err)
	}

	if _, _, err := srv.callCred("cred_import", map[string]any{
		"id":     "flex-cred",
		"format": "cookie",
		"input":  "sid=SECRET_SID; csrf=SECRET_CSRF; unused=ignore",
	}); err != nil {
		t.Fatalf("cred_import failed: %v", err)
	}

	// Default cred_get still redacts secrets.
	redactedOut, _, err := srv.callCred("cred_get", map[string]any{"id": "flex-cred"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(redactedOut, "SECRET_SID") || strings.Contains(redactedOut, "SECRET_CSRF") {
		t.Fatalf("default cred_get should redact secrets, got: %s", redactedOut)
	}

	// includeSecrets=true returns decrypted runtime cookies and mapped header.
	fullOut, _, err := srv.callCred("cred_get", map[string]any{"id": "flex-cred", "includeSecrets": true})
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		cred.Record
		Runtime cred.RuntimeData `json:"runtime"`
	}
	if err := json.Unmarshal([]byte(fullOut), &decoded); err != nil {
		t.Fatalf("unmarshal full cred_get failed: %v\n%s", err, fullOut)
	}
	if decoded.Runtime.Cookies["sid"] != "SECRET_SID" || decoded.Runtime.Cookies["csrf"] != "SECRET_CSRF" {
		t.Fatalf("unexpected runtime cookies: %+v", decoded.Runtime.Cookies)
	}
	if _, leaked := decoded.Runtime.Cookies["unused"]; leaked {
		t.Fatalf("expected unused cookie filtered by cookies=[sid,csrf], got %+v", decoded.Runtime.Cookies)
	}
	if decoded.Runtime.HeaderName != "X-Sid" || decoded.Runtime.HeaderValue != "Bearer SECRET_SID" {
		t.Fatalf("unexpected mapped CookieKey header: name=%q value=%q", decoded.Runtime.HeaderName, decoded.Runtime.HeaderValue)
	}
}

func TestCredMCPCancelAndSyncThroughManager(t *testing.T) {
	dir := t.TempDir()
	started := make(chan struct{}, 2)
	released := make(chan struct{}, 2)

	mgr, err := cred.NewManager(dir, cred.NewMemoryBackend(), cred.LoginHandlers{
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
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.SaveSpec(cred.Spec{
		ID:       "ego-mcp",
		Name:     "Ego MCP",
		Type:     cred.TypeCookie,
		Origin:   "https://example.com",
		Provider: cred.ProviderEgo,
	}); err != nil {
		t.Fatal(err)
	}

	srv := Server{
		DataDir:     dir,
		CredManager: func() *cred.Manager { return mgr },
	}

	syncErrCh := make(chan error, 1)
	go func() {
		_, handled, err := srv.callCred("cred_sync_cdp", map[string]any{
			"id":         "ego-mcp",
			"provider":   "ego",
			"timeoutSec": float64(30),
		})
		if !handled {
			syncErrCh <- errors.New("cred_sync_cdp not handled")
			return
		}
		syncErrCh <- err
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for cred_sync_cdp to enter Ego handler")
	}
	if !mgr.IsAuthenticating("ego-mcp") {
		t.Fatal("expected manager to track cred_sync_cdp in IsAuthenticating")
	}

	cancelOut, handled, err := srv.callCred("cred_cancel", map[string]any{"id": "ego-mcp"})
	if err != nil || !handled || !strings.Contains(cancelOut, `"cancelled": true`) {
		t.Fatalf("unexpected cred_cancel output: handled=%v err=%v out=%s", handled, err, cancelOut)
	}
	select {
	case <-released:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Ego handler exit after cred_cancel")
	}
	syncErr := <-syncErrCh
	if !errors.Is(syncErr, context.Canceled) {
		t.Fatalf("expected cred_sync_cdp to fail with context.Canceled, got: %v", syncErr)
	}

	// ReadOnly server must reject cred_cancel as a control/mutating operation.
	roSrv := Server{DataDir: dir, ReadOnly: func() bool { return true }}
	req := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"cred_cancel","arguments":{"id":"ego-mcp"}}}`
	var respObj map[string]any
	_ = json.Unmarshal(roSrv.Handle([]byte(req)), &respObj)
	resMap, _ := respObj["result"].(map[string]any)
	if isErr, _ := resMap["isError"].(bool); !isErr {
		t.Fatalf("expected ReadOnly server to reject cred_cancel, got: %s", string(roSrv.Handle([]byte(req))))
	}
}

func TestCredMCPHTTPContextCancellation(t *testing.T) {
	dir := t.TempDir()
	started := make(chan struct{}, 1)
	released := make(chan struct{}, 1)

	mgr, err := cred.NewManager(dir, cred.NewMemoryBackend(), cred.LoginHandlers{
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
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.SaveSpec(cred.Spec{
		ID:       "ego-http",
		Type:     cred.TypeCookie,
		Origin:   "https://example.com",
		Provider: cred.ProviderEgo,
	}); err != nil {
		t.Fatal(err)
	}

	const key = "sbk-test-cancel-key"
	srv := Server{
		DataDir:     dir,
		CredManager: func() *cred.Manager { return mgr },
	}
	httpSrv := httptest.NewServer(srv.HTTPHandler(key))
	defer httpSrv.Close()

	reqCtx, cancelReq := context.WithCancel(context.Background())
	defer cancelReq()
	payload := `{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"cred_sync_cdp","arguments":{"id":"ego-http"}}}`
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, httpSrv.URL, bytes.NewBufferString(payload))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")

	doneCh := make(chan error, 1)
	go func() {
		resp, err := http.DefaultClient.Do(req)
		if resp != nil {
			_ = resp.Body.Close()
		}
		doneCh <- err
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for HTTP cred_sync_cdp handler start")
	}

	cancelReq()

	select {
	case <-released:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for HTTP request cancellation to abort Ego handler")
	}
	<-doneCh
}
