package mcpsvc

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"scriptboard/src/internal/caps"
)

const demoSource = `export function meta() { return { name: "演示", icon: "sparkles", sources: {} }; }
export function render(ctx) { return { title: "演示", big: "1", sub: "", lines: [], chart: null }; }`

// The server speaks newline-delimited JSON-RPC: initialize, list the tools,
// create a plugin and an icon, and read them back.
func TestMCPCreatesPluginAndIcon(t *testing.T) {
	dir := t.TempDir()
	msgs := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		reqCall(3, "plugin_save", map[string]any{"id": "demo-plugin", "name": "演示插件", "icon": "sparkles", "home": true, "source": demoSource}),
		reqCall(4, "plugin_list", map[string]any{}),
		reqCall(5, "icon_add", map[string]any{"name": "my-star", "svg": `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M12 2l3 7h7l-5.5 4 2 7L12 16l-6.5 4 2-7L2 9h7z"/></svg>`, "plugin": "demo-plugin"}),
		reqCall(6, "icon_add", map[string]any{"name": "evil", "svg": `<svg><script>alert(1)</script></svg>`}),
		reqCall(7, "plugin_save", map[string]any{"id": "broken-one", "name": "坏插件", "source": "export const x = ("}),
		reqCall(8, "plugin_get", map[string]any{"id": "demo-plugin"}),
		reqCall(9, "icon_list", map[string]any{}),
	}
	var out bytes.Buffer
	if err := (Server{DataDir: dir}).Serve(strings.NewReader(strings.Join(msgs, "\n")+"\n"), &out); err != nil {
		t.Fatal(err)
	}
	replies := map[string]map[string]any{}
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var r map[string]any
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("bad reply %q: %v", line, err)
		}
		replies[string(mustJSON(r["id"]))] = r
	}
	if len(replies) != 9 {
		t.Fatalf("%d replies, want 9 (the notification gets none)", len(replies))
	}

	result := func(id int) map[string]any {
		r := replies[string(mustJSON(float64(id)))]
		if r == nil {
			t.Fatalf("no reply for %d", id)
		}
		if e, ok := r["error"]; ok {
			t.Fatalf("reply %d: protocol error %v", id, e)
		}
		return r["result"].(map[string]any)
	}
	text := func(id int) string {
		res := result(id)
		content := res["content"].([]any)
		return content[0].(map[string]any)["text"].(string)
	}
	isErr := func(id int) bool {
		v, _ := result(id)["isError"].(bool)
		return v
	}

	if v := result(1)["protocolVersion"]; v != "2024-11-05" {
		t.Errorf("protocol version %v", v)
	}
	if n := len(result(2)["tools"].([]any)); n != 9 {
		t.Errorf("%d tools listed, want 9", n)
	}
	if isErr(3) {
		t.Errorf("plugin_save failed: %s", text(3))
	}
	if !strings.Contains(text(4), "demo-plugin") {
		t.Errorf("plugin_list does not show the plugin: %s", text(4))
	}
	if isErr(5) {
		t.Errorf("icon_add failed: %s", text(5))
	}
	if !isErr(6) {
		t.Error("an SVG with a script was accepted")
	}
	if !isErr(7) {
		t.Error("a script that does not compile was saved")
	}
	if !strings.Contains(text(8), "render") {
		t.Errorf("plugin_get returned %q", text(8))
	}
	if !strings.Contains(text(9), "my-star (demo-plugin)") {
		t.Errorf("icon_list does not show the plugin icon: %s", text(9))
	}

	if _, err := os.Stat(filepath.Join(dir, "plugins", "demo-plugin", "plugin.json")); err != nil {
		t.Errorf("plugin.json not written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "plugins", "broken-one")); err == nil {
		t.Error("a broken plugin left a folder behind")
	}
}

func reqCall(id int, name string, args map[string]any) string {
	b, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": id, "method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args},
	})
	return string(b)
}

// The HTTP server needs the key; a read-only server refuses the tools that
// write, and still answers the ones that read.
func TestHTTPKeyAndReadOnly(t *testing.T) {
	dir := t.TempDir()
	readOnly := false
	srv := httptest.NewServer((Server{DataDir: dir, ReadOnly: func() bool { return readOnly }}).HTTPHandler("sbk-test"))
	defer srv.Close()

	post := func(key, body string) (int, string) {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/mcp", strings.NewReader(body))
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	list := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	if code, _ := post("", list); code != http.StatusUnauthorized {
		t.Errorf("no key: %d, want 401", code)
	}
	if code, _ := post("wrong", list); code != http.StatusUnauthorized {
		t.Errorf("wrong key: %d, want 401", code)
	}
	if code, body := post("sbk-test", list); code != http.StatusOK || !strings.Contains(body, "plugin_save") {
		t.Errorf("right key: %d %s", code, body)
	}
	if code, _ := post("sbk-test", `{"jsonrpc":"2.0","method":"notifications/initialized"}`); code != http.StatusAccepted {
		t.Errorf("notification: %d, want 202", code)
	}

	save := reqCall(2, "plugin_save", map[string]any{"id": "ro-test", "name": "只读测试", "source": demoSource})
	readOnly = true
	if _, body := post("sbk-test", save); !strings.Contains(body, `"isError":true`) {
		t.Errorf("a read-only server saved a plugin: %s", body)
	}
	if _, body := post("sbk-test", reqCall(3, "plugin_list", map[string]any{})); strings.Contains(body, `"isError":true`) {
		t.Errorf("a read-only server refused a read: %s", body)
	}
	readOnly = false
	if _, body := post("sbk-test", save); strings.Contains(body, `"isError":true`) {
		t.Errorf("save refused when not read-only: %s", body)
	}
	if k := NewKey(); !strings.HasPrefix(k, "sbk-") || len(k) != 44 {
		t.Errorf("key %q", k)
	}
}

// A script check and a plugin save say where the script runs and what it
// needs, and warn when its meta() claims more than the script can do.
func TestMCPReportsTheContract(t *testing.T) {
	dir := t.TempDir()
	liar := `export function meta() { return { name: "x", platforms: ["desktop", "mobile"], needs: ["files"], usage: ["claude-code"], sources: {} }; }
export function render(ctx) { return { title: "x", big: "1", sub: "", lines: [], chart: null }; }`
	honest := `export function meta() { return { name: "y", version: "1.0.0", platforms: ["desktop", "mobile"], needs: ["network"], sources: { a: "https://example.com/x" } }; }
export function render(ctx) { return { title: "y", big: "1", sub: "", lines: [], chart: null }; }`
	msgs := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
		reqCall(2, "script_check", map[string]any{"source": liar}),
		reqCall(3, "script_check", map[string]any{"source": demoSource}),
		reqCall(4, "plugin_save", map[string]any{"id": "honest-one", "name": "诚实的插件", "source": honest}),
		reqCall(5, "plugin_list", map[string]any{}),
	}
	var out bytes.Buffer
	if err := (Server{DataDir: dir}).Serve(strings.NewReader(strings.Join(msgs, "\n")+"\n"), &out); err != nil {
		t.Fatal(err)
	}
	texts := map[int]string{}
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var r struct {
			ID     float64 `json:"id"`
			Result struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"result"`
		}
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatal(err)
		}
		if len(r.Result.Content) > 0 {
			texts[int(r.ID)] = r.Result.Content[0].Text
		}
	}
	if got := texts[2]; !strings.Contains(got, "平台：desktop\n") || !strings.Contains(got, "警告：") {
		t.Errorf("script that claims a phone it cannot run on: %q", got)
	}
	if got := texts[3]; !strings.Contains(got, "没有声明 platforms 和 needs") {
		t.Errorf("script with no declarations: %q", got)
	}
	if got := texts[4]; !strings.Contains(got, "平台：desktop、mobile") || strings.Contains(got, "警告") {
		t.Errorf("plugin_save of an honest script: %q", got)
	}
	if got := texts[5]; !strings.Contains(got, "platforms=desktop,mobile") || !strings.Contains(got, "needs=network") {
		t.Errorf("plugin_list: %q", got)
	}
}

// The guide an assistant reads says every platform and need the host accepts,
// who allows what, and where system differences are handled, and it can be read
// even when the server is read-only.
func TestGuideSaysWhatTheHostAccepts(t *testing.T) {
	g := Guide()
	for _, word := range caps.Platforms {
		if !strings.Contains(g, `"`+word+`"`) {
			t.Errorf("the guide does not mention the platform %q", word)
		}
	}
	for _, need := range caps.Needs {
		if !strings.Contains(g, "`"+need+"`") {
			t.Errorf("the guide does not mention the need %q", need)
		}
	}
	for _, want := range []string{
		`platforms: ["windows"]`,      // a one-system script is not offered elsewhere
		"ctx.platform", "ctx.granted", // what the host gives
		"ctx.sourceInfo", "ctx.history", "Caching and offline belong to the host",
		"allows it in the editor", // the permission rule
		"System differences belong to the host",
		"fallbacks",
		"script_check", "plugin_save",
	} {
		if !strings.Contains(g, want) {
			t.Errorf("the guide lacks %q", want)
		}
	}

	var out bytes.Buffer
	msgs := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
		reqCall(2, "plugin_guide", map[string]any{}),
	}
	srv := Server{DataDir: t.TempDir(), ReadOnly: func() bool { return true }}
	if err := srv.Serve(strings.NewReader(strings.Join(msgs, "\n")+"\n"), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "ScriptBoard plugin guide") || strings.Contains(out.String(), `"isError":true`) {
		t.Errorf("the guide cannot be read in read-only mode: %s", out.String())
	}
}

func TestScriptCheckValidatesSourcesAndFallbacks(t *testing.T) {
	bad := `export function meta() {
  return {
    name: "bad-src",
    version: "1.0.0",
    description: "x",
    platforms: ["desktop", "mobile"],
    needs: ["network"],
    sources: {
      a: { url: "https://example.com/a", fallbacks: ["ftp://example.com/b", ""] }
    }
  };
}
export function render(ctx) { return { title: "x", big: "1", sub: "", lines: [], chart: null }; }`
	var out bytes.Buffer
	srv := Server{DataDir: t.TempDir()}
	if err := srv.Serve(strings.NewReader(reqCall(1, "script_check", map[string]any{"source": bad})+"\n"), &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "备用请求 1") || !strings.Contains(got, "备用请求 2") {
		t.Errorf("expected fallback warnings in script_check output: %s", got)
	}
}
