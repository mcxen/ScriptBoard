package main

import (
	"testing"

	"github.com/egoist/mygo"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/plugins"
	"scriptboard/src/internal/store"
)

// A script's menu entries become the items they say: a submenu of one level,
// text to copy, an action, a badge and a subtitle, a check mark; and what the
// app will not act on is shown disabled.
func TestMenuEntriesMakeTheirItems(t *testing.T) {
	a := newTestApp(t)
	var copied string
	old := copyText
	copyText = func(s string) { copied = s }
	defer func() { copyText = old }()
	sc := store.Script{ID: "bar-card", Name: "栏"}
	items := a.menuEntryItems(sc, []engine.MenuEntry{
		{Label: "更多", Items: []engine.MenuEntry{{Label: "复制它", Copy: "abc"}, {Label: "深", Items: []engine.MenuEntry{{Label: "更深"}}}}},
		{Label: "今日", Sub: "3 次", Badge: "3", Checked: true},
		{Label: "打开卡片", Action: "open"},
		{Label: "怪动作", Action: "format-disk"},
		{Label: "坏链接", URL: "javascript:alert(1)"},
		{Separator: true},
		{Label: "只是文字"},
		{Label: "停用", Action: "window", Disabled: true},
	}, 0)
	if len(items) != 8 {
		t.Fatalf("%d items", len(items))
	}
	sub := items[0].Submenu
	if len(sub) != 2 || sub[0].Label != "复制它" || sub[0].Click == nil {
		t.Fatalf("submenu %+v", sub)
	}
	sub[0].Click(sub[0], nil)
	if copied != "abc" {
		t.Errorf("copied %q", copied)
	}
	if len(sub[1].Submenu) != 0 || !sub[1].Disabled {
		t.Errorf("a submenu in a submenu is shown as one: %+v", sub[1])
	}
	if items[1].Subtitle != "3 次" || items[1].Badge != "3" || items[1].Type != mygo.MenuItemCheckbox || !items[1].Checked {
		t.Errorf("subtitle, badge and check: %+v", items[1])
	}
	items[2].Click(items[2], nil)
	if a.router.Path() != "/full/bar-card" {
		t.Errorf("the open action went to %q", a.router.Path())
	}
	for i, name := range map[int]string{3: "an unknown action", 4: "a link that is not a web address", 6: "a line of text"} {
		if !items[i].Disabled || items[i].Click != nil {
			t.Errorf("%s is not shown disabled: %+v", name, items[i])
		}
	}
	if items[5].Type != mygo.MenuItemSeparator {
		t.Errorf("rule %+v", items[5])
	}
	if !items[7].Disabled {
		t.Error("an entry that says disabled is enabled")
	}
}

// What the script asks of the item itself: a click that opens the card, and a
// title in a monospaced font.
func TestMenuBarKind(t *testing.T) {
	click, mono := menuBarKind(engine.Result{})
	if click != "menu" || mono {
		t.Errorf("default: %q, %v", click, mono)
	}
	click, mono = menuBarKind(engine.Result{MenuBar: &engine.MenuBar{Click: "open", Mono: true}})
	if click != "open" || !mono {
		t.Errorf("asked: %q, %v", click, mono)
	}
	if click, _ = menuBarKind(engine.Result{MenuBar: &engine.MenuBar{Click: "nonsense"}}); click != "menu" {
		t.Errorf("an unknown click is %q, want menu", click)
	}
}

// A plugin that asks for the menu bar is put there when it first appears, and
// stays where the user puts it after that.
func TestPluginIsPutInTheMenuBarOnceOnly(t *testing.T) {
	a := newTestApp(t)
	src := `export function meta() { return { name: "栏插件", menubar: true, sources: {} }; }
export function render(ctx) { return { title: "栏插件", big: "1", sub: "", lines: [], chart: null }; }`
	if err := plugins.Save(a.dataDir, plugins.Manifest{ID: "bar-plug", Name: "栏插件", Home: true, Menubar: true}, src); err != nil {
		t.Fatal(err)
	}
	a.syncPlugins()
	id := pluginPrefix + "bar-plug"
	if !a.menuBarOn(id) {
		t.Fatal("the plugin that asked for the menu bar is not there")
	}
	a.toggleMenuBar(id) // the user takes it out
	a.syncPlugins()
	if a.menuBarOn(id) {
		t.Error("the plugin was put back after the user took it out")
	}
	if got := a.store.Settings().MenuBar; len(got) != 0 {
		t.Errorf("kept %v", got)
	}
}
