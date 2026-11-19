package main

import (
	"bytes"
	"errors"
	"image/png"
	"strings"
	"testing"

	"github.com/egoist/mygo"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

// A card's title beside the icon is its headline, and its tooltip the name with
// the summary; a script can give its own.
func TestMenuBarTextIsTheHeadlineOrTheScriptsOwn(t *testing.T) {
	sc := store.Script{ID: "w", Name: "天气"}
	st := &runState{res: engine.Result{Title: "七日天气", Big: "18°", Sub: "今天 11°~18°"}}
	title, tip := menuBarText(sc, st)
	if title != "18°" || tip != "七日天气 · 今天 11°~18°" {
		t.Errorf("headline: %q, %q", title, tip)
	}
	st.res.MenuBar = &engine.MenuBar{Title: "18° 多云", Tooltip: "上海"}
	title, tip = menuBarText(sc, st)
	if title != "18° 多云" || tip != "上海" {
		t.Errorf("the script's own: %q, %q", title, tip)
	}
}

// While a card loads its title is a dot; when it fails, a dash and the reason.
func TestMenuBarTextWhileLoadingAndOnError(t *testing.T) {
	sc := store.Script{ID: "w", Name: "天气"}
	if title, _ := menuBarText(sc, &runState{loading: true}); title != "…" {
		t.Errorf("loading: %q", title)
	}
	title, tip := menuBarText(sc, &runState{err: errors.New("离线模式，没有缓存的数据")})
	if title != "--" || !strings.Contains(tip, "离线") {
		t.Errorf("error: %q, %q", title, tip)
	}
}

// A title that would change the bar's width is cut short.
func TestCutTitle(t *testing.T) {
	if got := cutTitle("abcdefghijklmnop", 12); got != "abcdefghijk…" {
		t.Errorf("cut: %q", got)
	}
	if got := cutTitle("18°", 12); got != "18°" {
		t.Errorf("short title changed: %q", got)
	}
}

// A script's link opens only as a web address.
func TestSafeURLIsWebOnly(t *testing.T) {
	for _, ok := range []string{"https://example.com/a", "http://example.com"} {
		if !safeURL(ok) {
			t.Errorf("%q is refused", ok)
		}
	}
	for _, bad := range []string{"javascript:alert(1)", "file:///etc/passwd", "", "https://"} {
		if safeURL(bad) {
			t.Errorf("%q is allowed", bad)
		}
	}
}

// Only a script that asks for the menu bar is offered it.
func TestMenuBarIsOfferedOnlyWhenTheScriptAsks(t *testing.T) {
	if menuBarOffered(engine.Meta{}) || !menuBarOffered(engine.Meta{Menubar: true}) {
		t.Error("the right-click menu offers the menu bar by the script's choice only")
	}
}

// The user's choice of the cards in the bar is kept, and taking one out leaves
// the others.
func TestToggleMenuBarKeepsTheChoice(t *testing.T) {
	a := newTestApp(t)
	a.toggleMenuBar("weather")
	a.toggleMenuBar("gold")
	if !a.menuBarOn("weather") || !a.menuBarOn("gold") {
		t.Fatal("the cards are not in the bar")
	}
	if got := a.store.Settings().MenuBar; len(got) != 2 {
		t.Errorf("saved %v", got)
	}
	a.toggleMenuBar("weather")
	if a.menuBarOn("weather") || !a.menuBarOn("gold") {
		t.Error("taking one card out changed the other")
	}
	if got := a.store.Settings().MenuBar; len(got) != 1 || got[0] != "gold" {
		t.Errorf("saved %v", got)
	}
}

// A card's menu has its name, the script's items (a link only when it is a web
// address), then refresh, the window and quit.
func TestMenuBarMenuHasTheScriptsItemsAndTheAppsOwn(t *testing.T) {
	a := newTestApp(t)
	sc := store.Script{ID: "w", Name: "天气"}
	st := &runState{res: engine.Result{Title: "七日天气", Sub: "今天", Menu: []engine.MenuEntry{
		{Label: "打开", URL: "https://example.com/x"},
		{Separator: true},
		{Label: "只是一行字"},
		{Label: "坏链接", URL: "javascript:alert(1)"},
	}}}
	items := a.menuBarMenu(sc, st).Items()
	if len(items) != 10 {
		t.Fatalf("%d items", len(items))
	}
	if items[0].Label != "七日天气" || !items[0].Disabled {
		t.Errorf("heading %+v", items[0])
	}
	if items[1].Label != "打开" || items[1].Disabled {
		t.Errorf("link %+v", items[1])
	}
	if items[2].Type != mygo.MenuItemSeparator {
		t.Errorf("rule %+v", items[2])
	}
	if !items[3].Disabled || !items[4].Disabled || items[4].Label != "坏链接" {
		t.Errorf("a line of text and an unsafe link must be disabled: %+v, %+v", items[3], items[4])
	}
	if items[6].Label != tr("刷新") || items[7].Label != tr("打开盯盯板") {
		t.Errorf("app items %q, %q", items[6].Label, items[7].Label)
	}
	if items[9].Role != mygo.RoleQuit {
		t.Errorf("the last item is not quit: %+v", items[9])
	}
}

// The icon is a template: black where it draws, transparent elsewhere, so that
// the system can tint it for the bar.
func TestMenuBarIconIsATemplate(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(menuBarIcon()))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 32 || b.Dy() != 32 {
		t.Fatalf("size %v", b)
	}
	drawn := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, al := img.At(x, y).RGBA()
			if al == 0 {
				continue
			}
			drawn++
			if r|g|bl != 0 {
				t.Fatalf("pixel %d,%d is not black", x, y)
			}
		}
	}
	if drawn == 0 {
		t.Error("the icon draws nothing")
	}
}
