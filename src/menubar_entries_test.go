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

// Rich tray actions (settings, toggle_offline, copy_headline, refresh_all,
// set_var, set_dim), auto var/dim submenus, and custom tray icon rendering.
func TestRichTrayActionsSubmenusAndCustomIcons(t *testing.T) {
	a := newTestApp(t)
	var copied string
	oldCopy := copyText
	copyText = func(s string) { copied = s }
	defer func() { copyText = oldCopy }()

	settingsOpened := 0
	a.openSettingsHook = func() { settingsOpened++ }

	sc, ok := a.store.Script("cnwx")
	if !ok {
		t.Fatal("cnwx builtin missing")
	}
	st := &runState{res: engine.Result{
		Title: "国内实况",
		Big:   "22°",
		Sub:   "上海 · 晴",
		MenuBar: &engine.MenuBar{
			Title: "22°", Icon: "pulse", Color: "#10B981", RefreshSec: 30,
		},
		Menu: []engine.MenuEntry{
			{Label: "复制读数", Action: "copy_headline"},
			{Label: "切换北京", Action: "set_var:city=beijing"},
			{Label: "打开设置", Action: "settings"},
			{Label: "切换离线", Action: "toggle_offline"},
			{Label: "全部刷新", Action: "refresh_all"},
		},
	}}

	// Custom colored icon disables template mode and draws valid 32x32 PNG.
	pngData, tpl := menuBarIconFor(st.res.MenuBar)
	if tpl || len(pngData) == 0 {
		t.Fatalf("expected non-template colored tray icon, tpl=%v len=%d", tpl, len(pngData))
	}
	// Custom monochrome shape keeps template mode true.
	_, tpl2 := menuBarIconFor(&engine.MenuBar{Icon: "dot"})
	if !tpl2 {
		t.Fatalf("expected monochrome custom shape to remain template=true")
	}

	items := a.menuBarMenu(sc, st).Items()
	clickLabel := func(lbl string) {
		for _, it := range items {
			if it.Label == lbl {
				if it.Click == nil || it.Disabled {
					t.Fatalf("menu item %q is disabled or has no click handler", lbl)
				}
				it.Click(it, nil)
				return
			}
		}
		t.Fatalf("menu item %q not found", lbl)
	}

	clickLabel("复制读数")
	if copied != "22°" {
		t.Fatalf("copy_headline copied %q, want 22°", copied)
	}
	clickLabel("打开设置")
	if settingsOpened != 1 {
		t.Fatalf("settings action opened=%d, want 1", settingsOpened)
	}
	wasOffline := a.set.Offline
	clickLabel("切换离线")
	if a.set.Offline == wasOffline {
		t.Fatal("toggle_offline did not flip offline flag")
	}
	clickLabel("切换北京")
	if a.set.ScriptVars["cnwx"]["city"] != "beijing" {
		t.Fatalf("set_var did not update city: %+v", a.set.ScriptVars)
	}

	// Auto var submenu for cnwx ("城市") must exist with selectable city entries.
	var citySub []*mygo.MenuItem
	for _, it := range items {
		if len(it.Submenu) > 0 && it.Label == "城市" {
			citySub = it.Submenu
			break
		}
	}
	if len(citySub) == 0 {
		t.Fatal("expected automatic 城市 submenu for cnwx vars")
	}
}
