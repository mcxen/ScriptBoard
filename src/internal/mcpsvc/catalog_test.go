package mcpsvc

import (
	"bytes"
	"strings"
	"testing"
)

// The catalog lists the shipped icons by category, and script_check warns about
// an icon that does not exist.
func TestIconCatalogAndUnknownIconWarning(t *testing.T) {
	src := `export function meta() { return { name: "x", icon: "no-such-icon", sources: {} }; }
export function render(ctx) { return { title: "x", big: "1", sub: "", lines: [], chart: null }; }`
	msgs := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`,
		reqCall(2, "icon_catalog", map[string]any{"category": "weather"}),
		reqCall(3, "icon_catalog", map[string]any{}),
		reqCall(4, "script_check", map[string]any{"source": src}),
	}
	var out bytes.Buffer
	if err := (Server{DataDir: t.TempDir()}).Serve(strings.NewReader(strings.Join(msgs, "\n")+"\n"), &out); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "weather") || !strings.Contains(text, "cloud-rain") {
		t.Errorf("the weather category is not listed: %s", text)
	}
	if !strings.Contains(text, "finance") {
		t.Errorf("all categories are not listed without a category")
	}
	if !strings.Contains(text, "警告：图标 no-such-icon 不存在") {
		t.Errorf("no warning for an unknown icon: %s", text)
	}
}
