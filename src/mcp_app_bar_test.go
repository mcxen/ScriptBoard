package main

import (
	"strings"
	"testing"

	"scriptboard/src/internal/bar"
	"scriptboard/src/internal/store"
)

func TestReloadFromMCPRestartsModifiedAndCleansDeletedBars(t *testing.T) {
	a := newTestApp(t)
	barXML := func(title string) string {
		return `<bar id="hot-bar" title="HotBar" refresh="5m">
  <permissions><network origin="https://api.example.com" /></permissions>
  <script><![CDATA[
    export default {
      render() {
        return { tray: { title: "` + title + `" } };
      }
    };
  ]]></script>
</bar>`
	}

	sc := store.Script{
		ID:     "hot-bar",
		Name:   "HotBar",
		Source: barXML("V1"),
		Origin: store.OriginUser,
		Kind:   store.KindBar,
	}
	if err := a.store.SaveScript(sc); err != nil {
		t.Fatal(err)
	}
	inst1, _, err := a.ensureBarInstance(sc)
	if err != nil {
		t.Fatal(err)
	}
	if !inst1.WaitIdle(0) {
		t.Fatal("inst1 WaitIdle timed out")
	}
	if snap := inst1.Snapshot(); snap.Tray == nil || snap.Tray.Title != "V1" {
		t.Fatalf("inst1 tray = %+v, want V1", snap.Tray)
	}

	// Simulate an external MCP process modifying source and saving a cookie on disk.
	extStore, err := store.Open(a.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	sc.Source = barXML("V2")
	if err := extStore.SaveScript(sc); err != nil {
		t.Fatal(err)
	}
	jar, err := bar.NewCookieJar(bar.CookiesPath(a.dataDir, "hot-bar"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := jar.SetHeader("https://api.example.com", "sid=from-mcp"); err != nil {
		t.Fatal(err)
	}

	a.reloadFromMCP()
	if _, stillOld := a.barInsts["hot-bar"]; stillOld {
		t.Fatal("expected reloadFromMCP to close and clear modified bar instance")
	}

	updatedSC, ok := a.store.Script("hot-bar")
	if !ok {
		t.Fatal("hot-bar missing after reloadFromMCP")
	}
	inst2, _, err := a.ensureBarInstance(updatedSC)
	if err != nil {
		t.Fatal(err)
	}
	if !inst2.WaitIdle(0) {
		t.Fatal("inst2 WaitIdle timed out")
	}
	if snap := inst2.Snapshot(); snap.Tray == nil || snap.Tray.Title != "V2" {
		t.Fatalf("inst2 tray = %+v, want V2", snap.Tray)
	}
	if cnt := a.barCookieCount("hot-bar"); cnt != 1 {
		t.Fatalf("barCookieCount = %d, want 1", cnt)
	}

	// Verify deleting via MCP cleans up running bar instance on reloadFromMCP.
	extStore, err = store.Open(a.dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := extStore.DeleteScript("hot-bar"); err != nil {
		t.Fatal(err)
	}
	a.reloadFromMCP()
	if _, exists := a.barInsts["hot-bar"]; exists {
		t.Fatal("expected deleted bar instance to be removed by reloadFromMCP")
	}
}

func TestMCPPromptIncludesAllRegisteredTools(t *testing.T) {
	m := store.MCPSettings{Enabled: true, Port: 7878, Key: "sb_test"}
	p := mcpPrompt(m)
	for _, want := range []string{
		"bar_guide", "bar_check", "bar_run", "bar_save", "bar_delete",
		"bar_cookies_get", "bar_cookies_set", "bar_cookies_clear",
		"bar_storage_get", "bar_storage_set", "bar_storage_clear",
		"feed_list", "feed_save", "feed_test", "feed_delete",
		"plugin_save",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("mcpPrompt missing tool %q", want)
		}
	}
}
