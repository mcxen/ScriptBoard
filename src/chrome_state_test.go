package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The appearance can be forced: dark and light regardless of the system, and
// following the system again.
func TestThemeModeForcesTheAppearance(t *testing.T) {
	t.Cleanup(func() { setThemeMode(themeSystem); setSkin(skinDefault) })
	for _, tc := range []struct {
		mode string
		dark bool
	}{{themeDark, true}, {themeLight, false}} {
		setThemeMode(tc.mode)
		var got bool
		view := func(c *ui.Context) {
			applySkin(c)
			got = pal(c).dark
		}
		tt := ui.NewTester(view, 200, 80)
		settle(tt)
		if got != tc.dark {
			t.Errorf("mode %s: dark %v, want %v", tc.mode, got, tc.dark)
		}
	}
}

// The sidebar folds into a rail from its toggle, and the choice is kept.
func TestSidebarFoldsIntoARail(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 1000, 700)
	settle(tt)
	if a.sidebarW() != sidebarWidth {
		t.Fatalf("sidebar %v before folding", a.sidebarW())
	}
	click(t, tt, "收起侧栏")
	if !a.set.SidebarCollapsed || a.sidebarW() != railWidth {
		t.Fatalf("not folded: %+v width %v", a.set.SidebarCollapsed, a.sidebarW())
	}
	if !a.store.Settings().SidebarCollapsed {
		t.Errorf("the fold was not saved")
	}
	settle(tt)
	savePNG(t, tt, "23-sidebar-rail")
	click(t, tt, "展开侧栏")
	if a.set.SidebarCollapsed {
		t.Errorf("still folded after expanding")
	}
}

// A script is copied from the editor as a new home card, which is what a copy
// of the weather needs before its place is changed.
func TestEditorCopiesAScript(t *testing.T) {
	a := newTestApp(t)
	a.openEditor("weather")
	before := len(a.store.Scripts())
	src := a.draft.Source
	a.duplicateDraft()
	if got := len(a.store.Scripts()); got != before+1 {
		t.Fatalf("%d scripts after copying, want %d", got, before+1)
	}
	if a.draft.Source != src || !a.draft.Home || a.draft.ID == "weather" {
		t.Errorf("copy %+v", a.draft)
	}
	if stored, ok := a.store.Script(a.draft.ID); !ok || stored.Source != src {
		t.Errorf("the copy was not stored with its source")
	}
}
