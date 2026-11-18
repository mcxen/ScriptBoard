package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"scriptboard/internal/plugins"
)

// A plugin in the data folder shows up as a script and as a home card, with
// no login. Its icon can come from the plugin's own folder.
func TestLocalPluginLoadsWithoutLogin(t *testing.T) {
	a := newTestApp(t)
	src := `export function meta() { return { name: "本地天气", icon: "cloud-sun", sources: {} }; }
export function render(ctx) { return { title: "本地插件 · 天气", icon: "my-dot", big: "21°", sub: "来自插件", lines: [], chart: null }; }`
	if err := plugins.Save(a.dataDir, plugins.Manifest{ID: "local-weather", Name: "本地天气", Home: true}, src); err != nil {
		t.Fatal(err)
	}
	dot := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="6"/></svg>`)
	if _, err := plugins.AddIcon(a.dataDir, "local-weather", "my-dot", dot); err != nil {
		t.Fatal(err)
	}
	a.syncPlugins()

	sc, ok := a.store.Script("plugin-local-weather")
	if !ok || !sc.Home {
		t.Fatalf("plugin not loaded as a home script: %+v, %v", sc, ok)
	}
	if a.pluginNote != "" {
		t.Errorf("plugin note %q", a.pluginNote)
	}

	tt := ui.NewTester(a.view, 960, 640)
	settle(tt)
	requireText(t, tt, "本地插件 · 天气")

	// Plugin scripts are changed on disk, so saving in the editor is refused.
	a.openEditor("plugin-local-weather")
	a.draft.Name = "改名"
	a.saveDraft()
	if a.status == "已保存" {
		t.Error("a plugin script was saved from the editor")
	}

	// Removing the folder removes the script.
	if err := plugins.Delete(a.dataDir, "local-weather"); err != nil {
		t.Fatal(err)
	}
	a.syncPlugins()
	if _, ok := a.store.Script("plugin-local-weather"); ok {
		t.Error("deleted plugin is still a script")
	}
}
