package mcpsvc

import (
	"bytes"
	"strings"
	"testing"
)

const barScript = `export function meta() { return { name: "价格", menubar: true, sources: {} }; }
export function render(ctx) { return { title: "价格", big: "1", sub: "", lines: [], chart: null,
  menubar: { title: "1", click: "open" }, menu: [{ label: "刷新", action: "refresh" }] }; }`

const badBarScript = `export function meta() { return { name: "价格", sources: {} }; }
export function render(ctx) { return { title: "价格", big: "1", sub: "", lines: [], chart: null,
  menu: [{ label: "坏链接", url: "javascript:alert(1)" }, { label: "怪", action: "format-disk" }] }; }`

func serve(t *testing.T, dir string, msgs ...string) string {
	t.Helper()
	all := append([]string{`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`}, msgs...)
	var out bytes.Buffer
	if err := (Server{DataDir: dir}).Serve(strings.NewReader(strings.Join(all, "\n")+"\n"), &out); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// plugin_save takes menubar, plugin_list says so, and a script that did not ask
// for the bar cannot be saved with it.
func TestMCPSavesAMenubarPlugin(t *testing.T) {
	dir := t.TempDir()
	out := serve(t, dir,
		reqCall(2, "plugin_save", map[string]any{"id": "price-bar", "name": "价格", "home": true, "menubar": true, "source": barScript}),
		reqCall(3, "plugin_list", map[string]any{}),
		reqCall(4, "plugin_save", map[string]any{"id": "no-cap", "name": "无", "menubar": true, "source": badBarScript}),
	)
	if !strings.Contains(out, "menubar=true") {
		t.Errorf("plugin_list does not say menubar=true: %s", out)
	}
	if !strings.Contains(out, "menubar: true") {
		t.Errorf("a script without the capability was saved with menubar, or the refusal does not say what to write: %s", out)
	}
}

// script_check warns about what the menu bar cannot use.
func TestMCPScriptCheckWarnsAboutTheMenuBar(t *testing.T) {
	out := serve(t, t.TempDir(), reqCall(2, "script_check", map[string]any{"source": badBarScript}))
	for _, want := range []string{"meta() 没有 menubar", "只能是 http", "action"} {
		if !strings.Contains(out, want) {
			t.Errorf("no warning with %q in %s", want, out)
		}
	}
	clean := serve(t, t.TempDir(), reqCall(2, "script_check", map[string]any{"source": barScript}))
	if strings.Contains(clean, "meta() 没有 menubar") || strings.Contains(clean, "只能是 http") {
		t.Errorf("a warning for a good script: %s", clean)
	}
}
