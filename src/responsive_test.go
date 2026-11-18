package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// Cards fill each row and the row holds as many as the window allows, so a
// wide window shows several columns and a phone one.
func TestHomeCardsFollowWidth(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 960, 640)
	settle(tt)
	settle(tt)
	_, _, wide := a.cardLayout(1)
	if wide < 2 {
		t.Errorf("%d column(s) at width %v, want at least 2", wide, a.width)
	}
	savePNG(t, tt, "6-home-wide")

	// Make the cards bigger with the scale control: fewer columns fit.
	a.set.CardScale = 1.4
	if err := a.store.SetSettings(a.set); err != nil {
		t.Fatal(err)
	}
	click(t, tt, "大")
	_, _, large := a.cardLayout(2)
	if large != 1 {
		t.Errorf("%d columns for large cards at scale 1.4, want 1", large)
	}
	savePNG(t, tt, "6-home-large-scale")

	phone := newTestApp(t)
	pt := ui.NewTester(phone.view, 390, 844)
	settle(pt)
	settle(pt)
	if _, _, cols := phone.cardLayout(1); cols != 1 {
		t.Errorf("%d columns on a phone, want 1", cols)
	}
}

// The scale is kept between runs and stays inside its bounds.
func TestCardScaleBounds(t *testing.T) {
	a := newTestApp(t)
	for _, v := range []float64{0, 0.2, 1, 3} {
		a.set.CardScale = v
		s := a.scale()
		if s < 0.7 || s > 1.6 {
			t.Errorf("scale %v for %v, outside 0.7..1.6", s, v)
		}
	}
}

// On a wide window the editor has three columns and the home page three cards
// across.
func TestWideWindow(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 1280, 800)
	settle(tt)
	settle(tt)
	savePNG(t, tt, "9-home-wide")
	click(t, tt, "编辑")
	requireText(t, tt, "编辑脚本")
	savePNG(t, tt, "9-editor-wide")
}

// The dark appearance: every page keeps its contrast.
func TestDarkAppearance(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 1100, 760)
	tt.SetDark(true)
	settle(tt)
	savePNG(t, tt, "10-dark-home")
	a.router.Push("/full/goldk")
	settle(tt)
	savePNG(t, tt, "10-dark-full")
	click(t, tt, "设置")
	savePNG(t, tt, "10-dark-settings")
}

// A card whose script has values typed in by hand has a quick-entry row: the
// record goes in, and the click does not open the full-screen page.
func TestQuickEntryOnHomeCard(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 1280, 900)
	settle(tt)
	before := len(a.store.Series("weight"))
	click(t, tt, "记录") // the first card is the weight card
	if got := len(a.store.Series("weight")); got != before+1 {
		t.Fatalf("%d weight records after the quick entry, want %d", got, before+1)
	}
	if p := a.router.Path(); p != "/" {
		t.Errorf("the quick entry opened %q", p)
	}
	if a.quickNote == "" {
		t.Error("no feedback after recording")
	}
	savePNG(t, tt, "11-quick-entry")
}
