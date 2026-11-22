package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/caps"
	"scriptboard/src/internal/store"
)

// A script's origin: the app's own, the community's, a plugin's, or the user's.
func TestOrigins(t *testing.T) {
	a := newTestApp(t)
	if sc, _ := a.store.Script("weather"); a.originOf(sc) != store.OriginBuiltin {
		t.Errorf("weather origin %q", a.originOf(sc))
	}
	if got := a.originOf(store.Script{ID: "plugin-x"}); got != store.OriginPlugin {
		t.Errorf("plugin origin %q", got)
	}
	// Saved before origins existed: a built-in by its id, the user's otherwise.
	if got := a.originOf(store.Script{ID: "weather"}); got != store.OriginBuiltin {
		t.Errorf("legacy builtin origin %q", got)
	}
	if got := a.originOf(store.Script{ID: "script-1"}); got != store.OriginUser {
		t.Errorf("legacy user origin %q", got)
	}

	src := `export function meta() { return { name: "from community", sources: {} }; }
export function render(ctx) { return { title: "x", big: "1", sub: "", lines: [], chart: null }; }`
	if err := a.importScript(src, store.OriginCommunity, "c-42"); err != nil {
		t.Fatal(err)
	}
	sc, _ := a.store.Script(a.editorID)
	if a.originOf(sc) != store.OriginCommunity || sc.CommunityID != "c-42" {
		t.Errorf("community import: %+v", sc)
	}
	if err := a.importText(src); err != nil {
		t.Fatal(err)
	}
	if sc, _ := a.store.Script(a.editorID); a.originOf(sc) != store.OriginUser {
		t.Errorf("pasted import origin %q", a.originOf(sc))
	}
}

// On a phone, a script that reads the computer's files has no card, does not
// run, and says why; on the computer it is as before.
func TestDesktopOnlyScriptOnAPhone(t *testing.T) {
	t.Cleanup(func() { caps.SetPlatform("") })
	a := newTestApp(t)
	claude, ok := a.store.Script("ai-claude-neon")
	if !ok {
		t.Fatal("no ai-claude-neon script")
	}
	claude.Home = true
	a.store.SaveScript(claude)

	caps.SetPlatform(caps.Desktop)
	onHome := func() bool {
		for _, sc := range a.homeScripts() {
			if sc.ID == "ai-claude-neon" {
				return true
			}
		}
		return false
	}
	if !onHome() {
		t.Fatal("ai-claude-neon is not on the computer's home")
	}

	caps.SetPlatform(caps.Mobile)
	if onHome() {
		t.Errorf("ai-claude-neon has a card on a phone")
	}
	_, err := a.execute(claude, nil)
	if err == nil || !strings.Contains(err.Error(), "电脑") {
		t.Errorf("running on a phone: %v", err)
	}
	if note := a.scriptNote(claude); !strings.Contains(note, "仅电脑") || !strings.Contains(note, "内置") {
		t.Errorf("list note %q", note)
	}
	// A script that only needs the network runs on a phone.
	if weather, _ := a.store.Script("weather"); !a.supported(weather) {
		t.Errorf("weather is not supported on a phone")
	}
}

// A script of the user's is deleted after a second click; a built-in one is
// restored instead, and a built-in one cannot be deleted.
func TestDeleteAndRestore(t *testing.T) {
	a := newTestApp(t)
	a.openEditor("weather")
	if a.canDelete(a.draft) {
		t.Fatalf("a built-in script can be deleted")
	}
	shipped := a.draft.Source
	a.draft.Source = "// changed by the user\n" + shipped
	a.saveDraft()
	a.restoreDraft()
	if got, _ := a.store.Script("weather"); got.Source != shipped {
		t.Errorf("weather was not restored")
	}

	a.newScript()
	a.draft.Name = "mine"
	a.saveDraft()
	id := a.draft.ID
	before := len(a.store.Scripts())
	a.deleteDraft() // the first click only asks
	if len(a.store.Scripts()) != before {
		t.Fatalf("deleted at the first click")
	}
	a.deleteDraft()
	if _, ok := a.store.Script(id); ok || len(a.store.Scripts()) != before-1 {
		t.Errorf("not deleted after the second click")
	}
}

// The editor shows the contract of the script being edited.
func TestEditorShowsTheContract(t *testing.T) {
	a := newTestApp(t)
	a.openEditor("ai-claude-neon")
	tt := ui.NewTester(a.view, 1300, 900)
	click(t, tt, "组件管理")
	requireText(t, tt, "仅电脑")
	requireText(t, tt, "读本机文件")
	savePNG(t, tt, "25-editor-contract")
}

// The contract block reads in English too, with no Chinese left on the page.
func TestContractBlockInEnglish(t *testing.T) {
	t.Cleanup(func() { setLang(langZH) })
	a := newTestApp(t)
	setLang(langEN)
	a.openEditor("ai-claude-neon")
	tt := ui.NewTester(a.view, 1300, 900)
	click(t, tt, "Widgets")
	requireText(t, tt, "Computer only")
	requireText(t, tt, "Reads local files")
	onlyEnglish(t, tt, "edit with contract")
}
