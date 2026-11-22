package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/caps"
	"scriptboard/src/internal/feed"
	"scriptboard/src/internal/store"
)

const simpleRender = `export function render(ctx) { return { title: "x", big: "1", sub: "", lines: [], chart: null }; }`

// saveUserScript stores a script of the user's with a given meta, on the home page.
func saveUserScript(t *testing.T, a *app, id, meta string) store.Script {
	t.Helper()
	sc := store.Script{ID: id, Name: id, Source: "export function meta() { return " + meta + "; }\n" + simpleRender, Home: true, Origin: store.OriginUser}
	if err := a.store.SaveScript(sc); err != nil {
		t.Fatal(err)
	}
	return sc
}

func onHome(a *app, id string) bool {
	for _, sc := range a.homeScripts() {
		if sc.ID == id {
			return true
		}
	}
	return false
}

// A script for Windows is not shown, and does not run, on macOS, Linux or a
// phone; on Windows it is as any other.
func TestWindowsOnlyScriptIsIsolated(t *testing.T) {
	t.Cleanup(func() { caps.SetOS("") })
	a := newTestApp(t)
	sc := saveUserScript(t, a, "win-only", `{ name: "win", platforms: ["windows"], needs: [] }`)

	for _, system := range []string{caps.MacOS, caps.Linux, caps.Android, caps.IOS} {
		caps.SetOS(system)
		if onHome(a, "win-only") {
			t.Errorf("%s: a Windows-only script has a card", system)
		}
		_, err := a.execute(sc, nil)
		if err == nil || !strings.Contains(err.Error(), "Windows") {
			t.Errorf("%s: running it gave %v, want a message naming Windows", system, err)
		}
		if note := a.scriptNote(sc); !strings.Contains(note, "仅 Windows") {
			t.Errorf("%s: list note %q", system, note)
		}
	}
	caps.SetOS(caps.Windows)
	if !onHome(a, "win-only") {
		t.Errorf("windows: the script has no card")
	}
	if _, err := a.execute(sc, nil); err != nil {
		t.Errorf("windows: %v", err)
	}
}

// A script that is not built in needs the user's permission for the computer's
// files, and gets it one script and one feature at a time.
func TestFilesNeedTheUsersPermission(t *testing.T) {
	a := newTestApp(t)
	sc := saveUserScript(t, a, "reads-logs", `{ name: "logs", platforms: ["desktop"], needs: ["files"], usage: ["claude-code"] }`)

	if a.granted(sc, caps.Files) || a.trusted(sc) {
		t.Fatalf("an unknown script has a permission")
	}
	_, err := a.execute(sc, nil)
	if err == nil || !strings.Contains(err.Error(), "需要授权") || !strings.Contains(err.Error(), "读本机文件") {
		t.Fatalf("without permission: %v", err)
	}
	a.setGrant(sc.ID, caps.Files, true)
	if _, err := a.execute(sc, nil); err != nil {
		t.Errorf("with permission: %v", err)
	}
	// It is for that feature and that script only.
	other := saveUserScript(t, a, "other-logs", `{ name: "o", platforms: ["desktop"], needs: ["files"], usage: ["claude-code"] }`)
	if _, err := a.execute(other, nil); err == nil {
		t.Errorf("another script ran on the first one's permission")
	}
	a.setGrant(sc.ID, caps.Files, false)
	if _, err := a.execute(sc, nil); err == nil {
		t.Errorf("the permission was not taken back")
	}
	if got := a.store.Settings().Grants["reads-logs"]; len(got) != 0 {
		t.Errorf("grants left after taking back: %v", got)
	}
}

// The app's own scripts are trusted while unchanged; a changed one is not.
func TestBuiltinsAreTrustedUntilChanged(t *testing.T) {
	a := newTestApp(t)
	claude, _ := a.store.Script("ai-claude-neon")
	if !a.trusted(claude) || len(a.missingGrants(claude)) != 0 {
		t.Fatalf("an unchanged built-in is not trusted")
	}
	if _, err := a.execute(claude, nil); err != nil {
		t.Errorf("ai-claude-neon: %v", err)
	}
	claude.Source += "\n// changed\n"
	if a.trusted(claude) {
		t.Errorf("a changed built-in is still trusted")
	}
	if missing := a.missingGrants(claude); len(missing) == 0 || missing[0] != caps.Files {
		t.Errorf("a changed built-in missing grants %v", missing)
	}
	if _, err := a.execute(claude, nil); err == nil {
		t.Errorf("a changed built-in ran without permission")
	}
}

// A data source that reads a file takes the files need to every script that
// reads it, so a community script cannot read a file through a feed.
func TestFeedThatReadsAFileNeedsPermission(t *testing.T) {
	a := newTestApp(t)
	if err := a.store.SaveFeed(feed.Source{ID: "local-file", Name: "f", Every: 60,
		Request: feed.Request{URL: "file:///tmp/x.txt"}, Extract: feed.Extract{Kind: feed.KindRegex, Pattern: `([0-9]+)`}}); err != nil {
		t.Fatal(err)
	}
	sc := saveUserScript(t, a, "reads-feed", `{ name: "f", platforms: ["desktop"], needs: ["files"], feeds: ["local-file"] }`)
	if got := a.feedNeeds([]string{"local-file"}); len(got) != 1 || got[0] != caps.Files {
		t.Fatalf("feed needs %v", got)
	}
	// A script that forgot to declare it is held to it all the same.
	hidden := saveUserScript(t, a, "hides-feed", `{ name: "h", feeds: ["local-file"] }`)
	if missing := a.missingGrants(hidden); len(missing) != 1 || missing[0] != caps.Files {
		t.Errorf("undeclared file feed: %v", missing)
	}
	if _, err := a.execute(sc, nil); err == nil {
		t.Errorf("a script read a file feed without permission")
	}
}

// A script is told the system and what it may use, so it can word a message
// without testing the system itself.
func TestScriptSeesPlatformAndGrants(t *testing.T) {
	t.Cleanup(func() { caps.SetOS("") })
	a := newTestApp(t)
	src := `export function meta() { return { name: "p", platforms: ["desktop"], needs: ["files"], usage: ["claude-code"] }; }
export function render(ctx) { return { title: "x", big: ctx.platform.os + "/" + ctx.platform.kind + "/" + (ctx.granted.files === true), sub: "", lines: [], chart: null }; }`
	sc := store.Script{ID: "sees", Name: "s", Source: src, Home: true, Origin: store.OriginUser}
	a.store.SaveScript(sc)
	a.setGrant(sc.ID, caps.Files, true)
	caps.SetOS(caps.Windows)
	res, err := a.execute(sc, nil)
	if err != nil || res.Big != "windows/desktop/true" {
		t.Fatalf("big %q err %v", res.Big, err)
	}
}

// The editor lists what a script uses and lets the user allow it.
func TestEditorGrantButtons(t *testing.T) {
	a := newTestApp(t)
	sc := saveUserScript(t, a, "needs-grant", `{ name: "needs", platforms: ["desktop"], needs: ["files"], usage: ["claude-code"] }`)
	a.openEditor(sc.ID)
	tt := ui.NewTester(a.view, 1300, 900)
	click(t, tt, "组件管理")
	requireText(t, tt, "未授权")
	click(t, tt, "授权")
	if !a.granted(sc, caps.Files) {
		t.Fatalf("the grant button did not allow files")
	}
	requireText(t, tt, "已授权")
	if tt.HasText("需要授权：读本机文件。到「编辑」里这个脚本的说明处授权后才能运行。") {
		t.Errorf("the preview still says the permission is missing")
	}
	savePNG(t, tt, "26-editor-grant")
	click(t, tt, "撤销")
	if a.granted(sc, caps.Files) {
		t.Errorf("the revoke button did not take it back")
	}
}
