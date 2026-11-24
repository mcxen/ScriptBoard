package mcpsvc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"scriptboard/src/internal/bar"
	"scriptboard/src/internal/plugins"
	"scriptboard/src/internal/store"
)

const demoCookieBarXML = `<bar id="cookie-demo" title="Cookie Demo" version="1.0.0" category="developer" icon="key" refresh="5m">
  <permissions>
    <network origin="https://api.example.com" />
    <network origin="https://*.example.com" />
  </permissions>
  <inputs>
    <input name="apiToken" type="password" label="API Token" default="default-tok" />
  </inputs>
  <script><![CDATA[
    const h = (type, props, ...children) => ({
      type, props: props || {}, children: children.flat().filter(c => c !== null && c !== undefined && c !== false)
    });
    let quota = null;
    async function load() {
      await fetch("https://api.example.com/login");
      const r = await fetch("https://api.example.com/quota");
      quota = await r.json();
      $metrix.storage.set("lastQuota", String(quota.pct));
      $metrix.invalidate();
    }
    export default {
      async scheduled() { await load(); },
      render(env) {
        const ck = $metrix.cookies.get("https://api.example.com/quota");
        const hasCk = Object.keys(ck).length > 0;
        const pct = quota ? quota.pct : 0;
        return {
          tray: {
            title: quota ? (pct + "%") : (hasCk ? "CK" : "--"),
            menu: [{ label: "Copy Token", onClick: () => $metrix.copy(env.inputs.apiToken || "") }]
          },
          panel: h("Column", { preferredWidth: 300, padding: 12, gap: 8 },
            h("Badge", { background: hasCk ? "accent" : "warning" }, hasCk ? "Authed" : "Guest"),
            h("Progress", { value: pct, max: 100 }),
            h("Button", { variant: "primary", onClick: () => $metrix.copy("quota-" + pct) }, "Copy Pct")
          )
        };
      }
    };
  ]]></script>
</bar>`

func TestBarToolsWorkflowCheckRunSaveCookiesStorageDelete(t *testing.T) {
	dir := t.TempDir()
	srv := Server{DataDir: dir}

	// 1. bar_guide
	guide, isErr := callMCPTool(t, srv, "bar_guide", nil)
	if isErr || !strings.Contains(guide, "$metrix.cookies") || !strings.Contains(guide, "bar_run") {
		t.Fatalf("unexpected bar_guide output (err=%v): len=%d", isErr, len(guide))
	}

	// 2. bar_check rejects invalid JS and accepts valid XML
	badOut, badErr := callMCPTool(t, srv, "bar_check", map[string]any{
		"source": `<bar id="bad" title="Bad"><script><![CDATA[export default { render( { ];]]></script></bar>`,
	})
	if !badErr {
		t.Fatalf("expected bar_check error for broken JS, got %s", badOut)
	}
	chkOut, chkErr := callMCPTool(t, srv, "bar_check", map[string]any{"source": demoCookieBarXML, "lang": "en"})
	if chkErr {
		t.Fatalf("bar_check failed: %s", chkOut)
	}
	var chk struct {
		OK      bool `json:"ok"`
		Preview struct {
			PanelTexts []string `json:"panelTexts"`
		} `json:"preview"`
	}
	if err := json.Unmarshal([]byte(chkOut), &chk); err != nil || !chk.OK {
		t.Fatalf("invalid bar_check response: %v %s", err, chkOut)
	}

	// 3. bar_run with mockFetch (Set-Cookie capture -> automatic resend -> actionID click)
	mocks := map[string]any{
		"https://api.example.com/login": map[string]any{
			"setCookie": []string{"sess=tok999111; Path=/; HttpOnly"},
			"body":      `{"ok":true}`,
		},
		"https://api.example.com/quota": map[string]any{
			"body": `{"pct":88}`,
		},
	}
	runOut, runErr := callMCPTool(t, srv, "bar_run", map[string]any{
		"source":    demoCookieBarXML,
		"mockFetch": mocks,
		"storage":   map[string]any{"init": "v0"},
	})
	if runErr {
		t.Fatalf("bar_run failed: %s", runOut)
	}
	var runRes struct {
		ID      string         `json:"id"`
		Settled bool           `json:"settled"`
		Tray    map[string]any `json:"tray"`
		Actions []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"actions"`
		Cookies []map[string]any  `json:"cookies"`
		Storage map[string]string `json:"storage"`
		Requests []struct {
			URL         string   `json:"url"`
			HasCookie   bool     `json:"hasCookie"`
			CookieNames []string `json:"cookieNames"`
		} `json:"requests"`
		CopiedTexts []string `json:"copiedTexts"`
	}
	if err := json.Unmarshal([]byte(runOut), &runRes); err != nil {
		t.Fatalf("parse bar_run JSON: %v\n%s", err, runOut)
	}
	if !runRes.Settled || runRes.Tray["title"] != "88%" {
		t.Fatalf("unexpected bar_run tray/settled: settled=%v tray=%+v", runRes.Settled, runRes.Tray)
	}
	if len(runRes.Requests) != 2 || runRes.Requests[0].HasCookie || !runRes.Requests[1].HasCookie {
		t.Fatalf("expected /login without cookie and /quota with Set-Cookie, got %+v", runRes.Requests)
	}
	if runRes.Storage["init"] != "v0" || runRes.Storage["lastQuota"] != "88" {
		t.Fatalf("unexpected sandbox storage: %+v", runRes.Storage)
	}
	if strings.Contains(runOut, "tok999111") {
		t.Fatalf("bar_run should mask cookie values in output")
	}

	var btnActionID string
	for _, act := range runRes.Actions {
		if act.Type == "Button" {
			btnActionID = act.ID
			break
		}
	}
	if btnActionID == "" {
		t.Fatalf("expected Button action in bar_run output: %+v", runRes.Actions)
	}
	clickOut, clickErr := callMCPTool(t, srv, "bar_run", map[string]any{
		"source":    demoCookieBarXML,
		"mockFetch": mocks,
		"actionID":  btnActionID,
	})
	if clickErr {
		t.Fatalf("bar_run actionID failed: %s", clickOut)
	}
	if !strings.Contains(clickOut, `"actionInvoked": true`) || !strings.Contains(clickOut, `"quota-88"`) {
		t.Fatalf("expected actionInvoked & copied quota-88, got %s", clickOut)
	}

	// 4. bar_save rejects invalid JS, saves valid XML + placement + secret input
	if _, errBool := callMCPTool(t, srv, "bar_save", map[string]any{
		"source": `<bar id="bad-save" title="Bad"><script><![CDATA[export default {};]]></script></bar>`,
	}); !errBool {
		t.Fatal("bar_save should reject missing render()")
	}
	saveOut, saveErr := callMCPTool(t, srv, "bar_save", map[string]any{
		"source":      demoCookieBarXML,
		"mode":        "panel",
		"display":     "external",
		"leftSidebar": true,
		"inputs":      map[string]any{"apiToken": "supersecret-token"},
	})
	if saveErr || !strings.Contains(saveOut, "cookie-demo") {
		t.Fatalf("bar_save failed (err=%v): %s", saveErr, saveOut)
	}

	// 5. bar_list + bar_get (secret redaction vs includeSecrets)
	listOut, listErr := callMCPTool(t, srv, "bar_list", nil)
	if listErr || !strings.Contains(listOut, "cookie-demo") || !strings.Contains(listOut, `"leftSidebar": true`) {
		t.Fatalf("unexpected bar_list (err=%v): %s", listErr, listOut)
	}
	getMasked, getErr := callMCPTool(t, srv, "bar_get", map[string]any{"id": "cookie-demo"})
	if getErr || strings.Contains(getMasked, "supersecret-token") {
		t.Fatalf("bar_get leaked password input without includeSecrets: %s", getMasked)
	}
	getRaw, getRawErr := callMCPTool(t, srv, "bar_get", map[string]any{"id": "cookie-demo", "includeSecrets": true})
	if getRawErr || !strings.Contains(getRaw, "supersecret-token") {
		t.Fatalf("bar_get with includeSecrets failed (err=%v): %s", getRawErr, getRaw)
	}

	// 6. bar_cookies_set / bar_cookies_get / bar_storage_set / bar_run(useSavedState) / clear
	setCkOut, setCkErr := callMCPTool(t, srv, "bar_cookies_set", map[string]any{
		"id":   "cookie-demo",
		"curl": `curl 'https://api.example.com/quota' -b 'a=1234567890; b=xyz'`,
	})
	if setCkErr {
		t.Fatalf("bar_cookies_set failed: %s", setCkOut)
	}
	getCkMasked, getCkErr := callMCPTool(t, srv, "bar_cookies_get", map[string]any{"id": "cookie-demo"})
	if getCkErr || strings.Contains(getCkMasked, "1234567890") {
		t.Fatalf("bar_cookies_get leaked value: %s", getCkMasked)
	}
	getCkRaw, _ := callMCPTool(t, srv, "bar_cookies_get", map[string]any{"id": "cookie-demo", "includeValues": true})
	if !strings.Contains(getCkRaw, "1234567890") {
		t.Fatalf("bar_cookies_get(includeValues) missing value: %s", getCkRaw)
	}

	if _, stErr := callMCPTool(t, srv, "bar_storage_set", map[string]any{
		"id":     "cookie-demo",
		"values": map[string]any{"persisted": "p1", "temp": "t1"},
	}); stErr {
		t.Fatal("bar_storage_set failed")
	}

	// Saved-state sandbox run must read saved cookies/storage without modifying disk!
	savedRunOut, savedRunErr := callMCPTool(t, srv, "bar_run", map[string]any{
		"id":            "cookie-demo",
		"useSavedState": true,
		"mockFetch":     mocks,
	})
	if savedRunErr || !strings.Contains(savedRunOut, `"persisted": "p1"`) {
		t.Fatalf("bar_run(useSavedState) failed (err=%v): %s", savedRunErr, savedRunOut)
	}
	kvDisk, _ := bar.NewKVStore(bar.StoragePath(dir, "cookie-demo"))
	if _, exists := kvDisk.Get("lastQuota"); exists {
		t.Fatal("bar_run(useSavedState) must not mutate persisted storage on disk")
	}

	if _, clrErr := callMCPTool(t, srv, "bar_storage_clear", map[string]any{"id": "cookie-demo", "key": "temp"}); clrErr {
		t.Fatal("bar_storage_clear single key failed")
	}
	stGet, _ := callMCPTool(t, srv, "bar_storage_get", map[string]any{"id": "cookie-demo"})
	if strings.Contains(stGet, `"temp"`) || !strings.Contains(stGet, `"persisted"`) {
		t.Fatalf("unexpected bar_storage_get after delete key: %s", stGet)
	}
	if _, clrErr := callMCPTool(t, srv, "bar_storage_clear", map[string]any{"id": "cookie-demo"}); clrErr {
		t.Fatal("bar_storage_clear all failed")
	}
	if _, clrCkErr := callMCPTool(t, srv, "bar_cookies_clear", map[string]any{"id": "cookie-demo"}); clrCkErr {
		t.Fatal("bar_cookies_clear failed")
	}

	// 7. bar_delete refuses builtin and removes user component + data dir
	st, _ := store.Open(dir)
	_ = st.SaveScript(store.Script{
		ID: "builtin-bar", Name: "Builtin", Source: demoCookieBarXML, Origin: store.OriginBuiltin, Kind: store.KindBar,
	})
	if _, errBool := callMCPTool(t, srv, "bar_delete", map[string]any{"id": "builtin-bar"}); !errBool {
		t.Fatal("expected bar_delete to refuse builtin .bar")
	}
	if delOut, delErr := callMCPTool(t, srv, "bar_delete", map[string]any{"id": "cookie-demo"}); delErr {
		t.Fatalf("bar_delete user .bar failed: %s", delOut)
	}
	if _, statErr := os.Stat(bar.ComponentDataDir(dir, "cookie-demo")); !os.IsNotExist(statErr) {
		t.Fatalf("expected bar data dir removed, stat err = %v", statErr)
	}
}

func TestFeedToolsLifecycle(t *testing.T) {
	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("sid")
		if err != nil || c.Value != "secretcookie" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"price":123.45}`))
	}))
	defer httpSrv.Close()

	srv := Server{DataDir: t.TempDir()}
	curlCmd := `curl '` + httpSrv.URL + `' -b 'sid=secretcookie' -H 'Authorization: Bearer supersecret'`

	saveOut, saveErr := callMCPTool(t, srv, "feed_save", map[string]any{
		"id":          "price-feed",
		"name":        "Price Feed",
		"request":     curlCmd,
		"extractKind": "json",
		"extractRule": "price",
		"every":       60,
	})
	if saveErr || !strings.Contains(saveOut, "price-feed") {
		t.Fatalf("feed_save failed (err=%v): %s", saveErr, saveOut)
	}

	listOut, _ := callMCPTool(t, srv, "feed_list", nil)
	if !strings.Contains(listOut, "price-feed") || strings.Contains(listOut, "secretcookie") || strings.Contains(listOut, "supersecret") {
		t.Fatalf("feed_list leaked secret or missed feed: %s", listOut)
	}

	getMasked, _ := callMCPTool(t, srv, "feed_get", map[string]any{"id": "price-feed"})
	if strings.Contains(getMasked, "secretcookie") || strings.Contains(getMasked, "supersecret") {
		t.Fatalf("feed_get leaked secret without includeSecrets: %s", getMasked)
	}
	getRaw, _ := callMCPTool(t, srv, "feed_get", map[string]any{"id": "price-feed", "includeSecrets": true})
	if !strings.Contains(getRaw, "secretcookie") || !strings.Contains(getRaw, "supersecret") {
		t.Fatalf("feed_get(includeSecrets) missing secrets: %s", getRaw)
	}

	testOut, testErr := callMCPTool(t, srv, "feed_test", map[string]any{"id": "price-feed"})
	if testErr || !strings.Contains(testOut, "123.45") {
		t.Fatalf("feed_test failed (err=%v): %s", testErr, testOut)
	}

	if delOut, delErr := callMCPTool(t, srv, "feed_delete", map[string]any{"id": "price-feed"}); delErr {
		t.Fatalf("feed_delete failed: %s", delOut)
	}
	if _, errBool := callMCPTool(t, srv, "feed_get", map[string]any{"id": "price-feed"}); !errBool {
		t.Fatal("expected feed_get error after delete")
	}
}

func TestPluginPrefixAndReadOnlyGuards(t *testing.T) {
	dir := t.TempDir()
	plugDir := filepath.Join(plugins.PluginDir(dir), "demo")
	if err := os.MkdirAll(plugDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plugDir, "plugin.json"), []byte(`{"id":"demo","name":"Demo","menubar":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plugDir, "script.js"), []byte(`export function meta() { return { name: "Demo", menubar: true }; }
export function render() { return { title: "Demo", menubar: { title: "DM" } }; }`), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := Server{DataDir: dir}
	trayOut, trayErr := callMCPTool(t, srv, "tray_list", nil)
	if trayErr {
		t.Fatalf("tray_list failed: %s", trayOut)
	}
	if !strings.Contains(trayOut, `"plugin-demo"`) || strings.Contains(trayOut, `"plugin:demo"`) {
		t.Fatalf("tray_list should use plugin-demo prefix, got %s", trayOut)
	}

	// Using either plugin-demo or legacy plugin:demo alias updates the same canonical ID without duplicates.
	if out, errBool := callMCPTool(t, srv, "tray_set", map[string]any{"id": "plugin:demo", "enabled": true}); errBool || !strings.Contains(out, "plugin-demo") {
		t.Fatalf("tray_set alias failed (err=%v): %s", errBool, out)
	}
	if out, errBool := callMCPTool(t, srv, "tray_set", map[string]any{"id": "plugin-demo", "enabled": true}); errBool {
		t.Fatalf("tray_set canonical failed: %s", out)
	}
	st, _ := store.Open(dir)
	menuCount := 0
	for _, x := range st.Settings().MenuBar {
		if x == "plugin-demo" || x == "plugin:demo" {
			menuCount++
		}
	}
	if menuCount != 1 {
		t.Fatalf("expected deduplicated menu bar entry count=1, got %v", st.Settings().MenuBar)
	}

	if showOut, showErr := callMCPTool(t, srv, "menubar_show", map[string]any{"id": "plugin:demo"}); showErr || !strings.Contains(showOut, `"plugin-demo"`) {
		t.Fatalf("menubar_show alias failed (err=%v): %s", showErr, showOut)
	}

	// ReadOnly guard rejects all new mutating tools.
	roSrv := Server{DataDir: dir, ReadOnly: func() bool { return true }}
	writeCalls := []struct {
		name string
		args map[string]any
	}{
		{"bar_save", map[string]any{"source": demoCookieBarXML}},
		{"bar_delete", map[string]any{"id": "x"}},
		{"bar_cookies_set", map[string]any{"id": "x", "cookieHeader": "a=1", "url": "https://a.com"}},
		{"bar_cookies_clear", map[string]any{"id": "x"}},
		{"bar_storage_set", map[string]any{"id": "x", "key": "k", "value": "v"}},
		{"bar_storage_clear", map[string]any{"id": "x"}},
		{"feed_save", map[string]any{"id": "f", "name": "F", "request": "https://a.com", "extractKind": "json", "extractRule": "a"}},
		{"feed_delete", map[string]any{"id": "f"}},
	}
	for _, wc := range writeCalls {
		out, isErr := callMCPTool(t, roSrv, wc.name, wc.args)
		if !isErr || !strings.Contains(out, "只读") {
			t.Errorf("ReadOnly server allowed or misreported %s: err=%v out=%s", wc.name, isErr, out)
		}
	}
}

func TestBarTemplatesPreviewAndActionArgWorkflow(t *testing.T) {
	dir := t.TempDir()
	outDir := filepath.Join(dir, "previews")
	srv := Server{DataDir: dir, ReadOnly: func() bool { return true }}

	// 1. bar_templates & bar_template_get work even in ReadOnly mode.
	listOut, listErr := callMCPTool(t, srv, "bar_templates", nil)
	if listErr {
		t.Fatalf("bar_templates failed: %s", listOut)
	}
	var listRes struct {
		Count     int `json:"count"`
		Templates []struct {
			ID string `json:"id"`
		} `json:"templates"`
	}
	if err := json.Unmarshal([]byte(listOut), &listRes); err != nil || listRes.Count < 6 || listRes.Templates[0].ID != "native-glass" {
		t.Fatalf("unexpected bar_templates output (%v): %s", err, listOut)
	}

	tplOut, tplErr := callMCPTool(t, srv, "bar_template_get", map[string]any{"id": "utility-switchboard"})
	if tplErr || !strings.Contains(tplOut, `"designNotes"`) || !strings.Contains(tplOut, "<bar ") {
		t.Fatalf("bar_template_get failed (err=%v): %s", tplErr, tplOut)
	}
	var tpl struct {
		Source string `json:"source"`
	}
	if err := json.Unmarshal([]byte(tplOut), &tpl); err != nil || tpl.Source == "" {
		t.Fatalf("decode bar_template_get: %v", err)
	}

	// 2. bar_run with actionArg triggers Segmented.onChange cleanly.
	runOut, runErr := callMCPTool(t, srv, "bar_run", map[string]any{
		"source": tpl.Source,
	})
	if runErr {
		t.Fatalf("bar_run utility-switchboard failed: %s", runOut)
	}
	var runRes struct {
		Actions []struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"actions"`
	}
	if err := json.Unmarshal([]byte(runOut), &runRes); err != nil {
		t.Fatal(err)
	}
	var segID string
	for _, a := range runRes.Actions {
		if a.Type == "Segmented.onChange" {
			segID = a.ID
			break
		}
	}
	if segID == "" {
		t.Fatalf("expected Segmented.onChange action in %+v", runRes.Actions)
	}

	// 3. bar_preview with actionID + actionArg renders Light and Dark PNG files and passes audit.
	prevOut, prevErr := callMCPTool(t, srv, "bar_preview", map[string]any{
		"source":    tpl.Source,
		"theme":     "both",
		"scale":     1,
		"outputDir": outDir,
		"actionID":  segID,
		"actionArg": "actions",
	})
	if prevErr {
		t.Fatalf("bar_preview failed: %s", prevOut)
	}
	var prevRes struct {
		ActionInvoked  bool     `json:"actionInvoked"`
		DarkPath       string   `json:"darkPath"`
		LightPath      string   `json:"lightPath"`
		MeasuredHeight int      `json:"measuredHeight"`
		PanelTexts     []string `json:"panelTexts"`
		Audit          struct {
			OK        bool     `json:"ok"`
			NodeCount int      `json:"nodeCount"`
			Warnings  []string `json:"warnings"`
		} `json:"audit"`
	}
	if err := json.Unmarshal([]byte(prevOut), &prevRes); err != nil {
		t.Fatalf("decode bar_preview JSON: %v\n%s", err, prevOut)
	}
	if !prevRes.ActionInvoked || !prevRes.Audit.OK || prevRes.MeasuredHeight <= 0 {
		t.Fatalf("unexpected bar_preview result: %+v", prevRes)
	}
	if !strings.Contains(strings.Join(prevRes.PanelTexts, "|"), "Copy Diagnostic Env") {
		t.Fatalf("expected Quick Actions tab content after actionArg='actions', got %v", prevRes.PanelTexts)
	}
	for _, p := range []string{prevRes.DarkPath, prevRes.LightPath} {
		data, err := os.ReadFile(p)
		if err != nil || len(data) < 512 || string(data[:8]) != "\x89PNG\r\n\x1a\n" {
			t.Fatalf("invalid PNG at %q: err=%v len=%d", p, err, len(data))
		}
	}

	// 4. app_info exposes barTemplates and upgraded VDOM/runtime capabilities.
	infoOut, infoErr := callMCPTool(t, srv, "app_info", nil)
	if infoErr || !strings.Contains(infoOut, `"Ring"`) || !strings.Contains(infoOut, `"barTemplates": 6`) {
		t.Fatalf("unexpected app_info (err=%v): %s", infoErr, infoOut)
	}
}
