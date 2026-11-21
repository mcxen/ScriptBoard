package main

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

var cjk = regexp.MustCompile(`[\x{4e00}-\x{9fff}]`)

// onlyEnglish fails for any Chinese on the page except the names of the
// choices that are always shown in their own language.
func onlyEnglish(t *testing.T, tt *ui.Tester, page string) {
	t.Helper()
	for _, s := range tt.Texts() {
		if !cjk.MatchString(s) {
			continue
		}
		switch s {
		case "中文", "斤", "公斤":
			continue
		}
		t.Errorf("%s: Chinese text on the English page: %q", page, s)
	}
}

func TestEnglish(t *testing.T) {
	t.Cleanup(func() { setLang(langZH) })
	a := newTestApp(t)
	a.set.Lang = langEN
	setLang(langEN)
	tt := ui.NewTester(a.view, 1100, 760)
	settle(tt)
	requireText(t, tt, "Home")
	requireText(t, tt, "ScriptBoard")
	requireText(t, tt, "Overview")
	onlyEnglish(t, tt, "home")
	savePNG(t, tt, "14-en-home")

	click(t, tt, "Edit")
	onlyEnglish(t, tt, "edit")
	savePNG(t, tt, "14-en-edit")

	click(t, tt, "Community")
	onlyEnglish(t, tt, "community")

	click(t, tt, "Settings")
	requireText(t, tt, "Language")
	onlyEnglish(t, tt, "settings")
	savePNG(t, tt, "14-en-settings")
}

func TestLanguageSwitch(t *testing.T) {
	t.Cleanup(func() { setLang(langZH) })
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 1100, 760)
	click(t, tt, "设置")
	requireText(t, tt, "语言")
	click(t, tt, "English")
	if got := a.store.Settings().Lang; got != langEN {
		t.Fatalf("stored language %q, want en", got)
	}
	requireText(t, tt, "Language")
	requireText(t, tt, "ScriptBoard")
	click(t, tt, "中文")
	requireText(t, tt, "语言")
	requireText(t, tt, "盯盯板")
	if strings.Contains(strings.Join(tt.Texts(), " "), "Offline mode") {
		t.Errorf("English text left after switching back to Chinese: %v", tt.Texts())
	}
}

func TestEnglishScripts(t *testing.T) {
	t.Cleanup(func() { setLang(langZH) })
	a := newTestApp(t)
	setLang(langEN)
	for _, sc := range a.store.Scripts() {
		res, err := a.execute(sc, nil)
		if err != nil {
			t.Errorf("%s: %v", sc.ID, err)
			continue
		}
		for _, s := range append([]string{res.Title, res.Big, res.Sub}, res.Lines...) {
			if cjk.MatchString(s) {
				t.Errorf("%s: Chinese in the English result: %q", sc.ID, s)
			}
		}
		m, err := a.metaOf(sc)
		if err != nil {
			t.Errorf("%s: %v", sc.ID, err)
			continue
		}
		for _, d := range m.Dims {
			if cjk.MatchString(d.Label) {
				t.Errorf("%s: Chinese dimension %q", sc.ID, d.Label)
			}
			for _, o := range d.Options {
				if cjk.MatchString(o.Label) {
					t.Errorf("%s: Chinese option %q", sc.ID, o.Label)
				}
			}
		}
	}
}

// Messages from the usage packages are translated where scripts read them.
func TestUsageMessagesFollowTheLanguage(t *testing.T) {
	t.Cleanup(func() { setLang(langZH) })
	a := newTestApp(t)
	setLang(langEN)
	r := a.usageReport("claude-code", time.Now())
	for k, v := range r {
		if s, ok := v.(string); ok && (k == "note" || k == "error") && cjk.MatchString(s) {
			t.Errorf("usage %s in Chinese on the English page: %q", k, s)
		}
	}
}

// Every newly upgraded surface (all 7 Settings tabs, FullView Visual/Table/Raw
// modes, upgraded Data Table editor, and Visual Editor table + tray config)
// renders purely in English with zero leaked Chinese strings.
func TestEnglishAcrossAllNewSurfaces(t *testing.T) {
	t.Cleanup(func() { setLang(langZH) })
	a := newTestApp(t)
	a.set.Lang = langEN
	setLang(langEN)

	// 1. All 7 Settings tabs in the standalone Settings window.
	stWin := ui.NewTester(a.settingsView, 780, 640)
	settle(stWin)
	for _, tab := range []string{"Appearance", "Data", "AI usage", "Data sources", "MCP server", "Community", "About"} {
		if tab != "Appearance" {
			click(t, stWin, tab)
		}
		onlyEnglish(t, stWin, "settings:"+tab)
	}

	// 2. Full view across Visual, Data table, and Raw data modes.
	tt := ui.NewTester(a.view, 1100, 760)
	a.router.Push("/full/weight")
	settle(tt)
	onlyEnglish(t, tt, "full:visual")
	click(t, tt, "Data table")
	onlyEnglish(t, tt, "full:table")
	click(t, tt, "Raw data")
	onlyEnglish(t, tt, "full:raw")

	// 3. Swappable progressBar component full view in English.
	a.router.Push("/full/ai-claude-neon")
	settle(tt)
	onlyEnglish(t, tt, "full:progressBar")

	// 4. Upgraded Data Table plugin editor in English.
	a.router.Push("/edit/weight-data")
	settle(tt)
	onlyEnglish(t, tt, "editor:table")

	// 5. Visual script editor with table template & menubar options in English.
	a.newScript()
	a.applyTemplate("table")
	a.visual.cfg.Menubar = true
	a.applyVisualConfig()
	a.router.Push("/edit/" + a.draft.ID)
	settle(tt)
	onlyEnglish(t, tt, "editor:visual-table")
}

// Dark mode visual verification across tabbed Settings and multi-mode FullView.
func TestDarkModeVisualAudit(t *testing.T) {
	t.Cleanup(func() { setThemeMode(themeSystem); setSkin(skinDefault) })
	a := newTestApp(t)
	setThemeMode(themeDark)

	stWin := ui.NewTester(a.settingsView, 780, 640)
	settle(stWin)
	click(t, stWin, "MCP 服务器")
	savePNG(t, stWin, "31-settings-tabs-dark")

	tt := ui.NewTester(a.view, 1100, 760)
	a.router.Push("/full/weight")
	settle(tt)
	click(t, tt, "数据表")
	savePNG(t, tt, "31-fullview-table-dark")
	click(t, tt, "原始数据")
	savePNG(t, tt, "31-fullview-raw-dark")
}
