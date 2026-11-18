package main

import (
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/mcpsvc"
	"scriptboard/src/internal/store"
)

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// The MCP server follows the settings: off by default, listening on the port
// with a key once switched on, refusing a request without the key, and gone
// again when switched off. What the settings copy is what a client needs.
func TestMCPServerFollowsSettings(t *testing.T) {
	a := newTestApp(t)
	old := copyText
	copyText = func(s string) {}
	defer func() { copyText = old }()
	defer a.mcp.stop()

	m := a.store.Settings().MCP
	if m.Enabled {
		t.Fatal("the MCP server is on by default")
	}
	m.Enabled, m.Port, m.Key = true, freePort(t), mcpsvc.NewKey()
	a.saveMCP(m)
	if st := a.mcp.status(m); !strings.Contains(st, strconv.Itoa(m.Port)) {
		t.Fatalf("status %q", st)
	}

	// No kept-alive connections: the server restarts on the same port when
	// the key changes, and a connection to the old one would be reset.
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}
	url := connectURL(m)
	call := func(key string) int {
		req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return resp.StatusCode
	}
	if code := call(""); code != http.StatusUnauthorized {
		t.Errorf("no key: %d", code)
	}
	if code := call(m.Key); code != http.StatusOK {
		t.Errorf("with the key: %d", code)
	}

	// A new key replaces the old one at once.
	old2 := m.Key
	m.Key = mcpsvc.NewKey()
	a.saveMCP(m)
	if code := call(old2); code != http.StatusUnauthorized {
		t.Errorf("the old key still works: %d", code)
	}

	prompt, cmd, cfg := mcpPrompt(m), claudeCodeCommand(m), httpConfig(m)
	for _, want := range []string{url, "Bearer " + m.Key, "plugin_save", "icon_add"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt lacks %q", want)
		}
	}
	if !strings.Contains(cmd, "claude mcp add --transport http scriptboard "+url) || !strings.Contains(cmd, m.Key) {
		t.Errorf("claude command %q", cmd)
	}
	if !strings.Contains(cfg, `"type": "http"`) || !strings.Contains(cfg, url) {
		t.Errorf("json config %q", cfg)
	}
	if !strings.Contains(stdioConfig(), "--mcp-stdio") {
		t.Errorf("stdio config %q", stdioConfig())
	}

	// Public addresses are kept as given, with /mcp added.
	m.PublicURL = "https://tools.example.com/"
	if got := connectURL(m); got != "https://tools.example.com/mcp" {
		t.Errorf("public url %q", got)
	}

	m.Enabled = false
	a.saveMCP(m)
	if _, err := http.Post(url, "application/json", strings.NewReader("{}")); err == nil {
		t.Error("the server still answers after it was switched off")
	}
	if got := maskedKey(m.Key); strings.Contains(got, m.Key[6:20]) {
		t.Errorf("masked key %q shows the middle", got)
	}
}

func TestMCPSettingsPage(t *testing.T) {
	a := newTestApp(t)
	var copied string
	old := copyText
	copyText = func(s string) { copied = s }
	defer func() { copyText = old }()
	m := a.store.Settings().MCP
	m.Key = mcpsvc.NewKey()
	if err := a.store.SetSettings(withMCP(a, m)); err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(a.view, 960, 2400)
	click(t, tt, "设置")
	requireText(t, tt, "MCP 服务器")
	requireText(t, tt, "复制提示词")
	click(t, tt, "复制密钥")
	if copied != m.Key {
		t.Errorf("copied %q, want the key", copied)
	}
	savePNG(t, tt, "12-settings-mcp")
}

func withMCP(a *app, m store.MCPSettings) store.Settings {
	st := a.store.Settings()
	st.MCP = m
	return st
}
