package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The settings are what is used: appearance, data, AI usage, data sources,
// updates, MCP, the community and about. What did nothing or repeated another
// row is gone.
func TestSettingsHaveNoDeadOrRepeatedRows(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.settingsView, 700, 3000)
	settle(tt)
	settle(tt)
	for _, want := range []string{"外观", "语言", "内容宽度", "数据", "体重单位", "离线模式", "本地缓存", "AI 用量", "数据源", "更新", "MCP 服务器", "社区", "关于", "数据文件", "插件目录"} {
		requireText(t, tt, want)
	}
	// A reminder that was only saved, the stack of libraries, and the plugin
	// group of two lines of explanation.
	absent(t, tt, "提醒", "每日提醒时间", "技术栈", "本地插件", "0.1.0")
}

// With the MCP server off its settings are one switch; its port, keys and the
// buttons that copy its configuration show once it is on.
func TestMCPSettingsShowOnlyWhenOn(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.settingsView, 700, 3000)
	settle(tt)
	settle(tt)
	requireText(t, tt, "开启 MCP 服务器")
	absent(t, tt, "端口", "允许局域网访问", "连接助手", "复制提示词")

	st := a.store.Settings()
	st.MCP.Enabled = true
	st.MCP.Key = "test-key-0123456789"
	if err := a.store.SetSettings(st); err != nil {
		t.Fatal(err)
	}
	tt = ui.NewTester(a.settingsView, 700, 3000)
	settle(tt)
	settle(tt)
	for _, want := range []string{"端口", "允许局域网访问", "只读", "密钥", "连接助手", "复制提示词", "复制 stdio 配置"} {
		requireText(t, tt, want)
	}
}

// The window has its title in its title row, not twice. The settings page of a
// phone, and of a run with no window, keeps its own title.
func TestSettingsWindowHasNoSecondTitle(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.settingsView, 700, 900)
	settle(tt)
	settle(tt)
	n := 0
	for _, s := range tt.Texts() {
		if s == "设置" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("the window shows 设置 %d times, want once (its title row)", n)
	}
	page := ui.NewTester(a.view, 1000, 900)
	a.router.Push("/settings")
	settle(page)
	requireText(t, page, "外观")
}

// Without a window to open the settings in (a phone, a test), a click on the
// settings goes to their page.
func TestSettingsFallBackToThePage(t *testing.T) {
	a := newTestApp(t)
	a.goTo("/settings")
	if a.router.Path() != "/settings" {
		t.Errorf("the path is %q, want /settings", a.router.Path())
	}
	a.goTo("/community")
	if a.router.Path() != "/community" {
		t.Errorf("the path is %q, want /community", a.router.Path())
	}
}

// On desktop, clicking Settings in the expanded sidebar, the folded rail, the
// Mac OS 9 sidebar, or pressing Cmd+, opens the separate settings window
// instead of navigating the main window away from its current page.
func TestDesktopSidebarSettingsOpensWindow(t *testing.T) {
	resetSkin(t)
	a := newTestApp(t)
	opened := 0
	a.openSettingsHook = func() { opened++ }

	tt := ui.NewTester(a.view, 1000, 700)
	settle(tt)

	// Default expanded sidebar.
	click(t, tt, "设置")
	if opened != 1 || a.router.Path() != "/" {
		t.Fatalf("expanded sidebar: opened=%d path=%q, want opened=1 path=/", opened, a.router.Path())
	}

	// Cmd+, / Ctrl+, shortcut.
	tt.Key(ui.Cmd, ui.KeyComma)
	settle(tt)
	if opened != 2 || a.router.Path() != "/" {
		t.Fatalf("shortcut: opened=%d path=%q, want opened=2 path=/", opened, a.router.Path())
	}

	// Folded sidebar rail.
	click(t, tt, "收起侧栏")
	click(t, tt, "设置")
	if opened != 3 || a.router.Path() != "/" {
		t.Fatalf("folded rail: opened=%d path=%q, want opened=3 path=/", opened, a.router.Path())
	}
	click(t, tt, "展开侧栏")

	// Mac OS 9 sidebar.
	setSkin(skinMac9)
	settle(tt)
	click(t, tt, "设置")
	if opened != 4 || a.router.Path() != "/" {
		t.Fatalf("mac9 sidebar: opened=%d path=%q, want opened=4 path=/", opened, a.router.Path())
	}
}

// The separate settings window applies the chosen theme mode and skin.
func TestSettingsWindowAppliesThemeMode(t *testing.T) {
	t.Cleanup(func() { setThemeMode(themeSystem); setSkin(skinDefault) })
	a := newTestApp(t)
	setThemeMode(themeDark)
	var dark bool
	tt := ui.NewTester(func(c *ui.Context) {
		a.settingsView(c)
		dark = pal(c).dark
	}, 640, 780)
	settle(tt)
	if !dark {
		t.Errorf("settings window did not apply forced dark theme mode")
	}
	savePNG(t, tt, "24-settings-window-dark")
}
