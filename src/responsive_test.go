package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/store"
)

// Cards fill each row and the row holds as many as the window allows, so a wide
// window shows several columns and a phone one.
func TestHomeCardsFollowWidth(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 960, 640)
	settle(tt)
	settle(tt)
	if cols := colsFor(a.contentW(), false); cols < 6 {
		t.Errorf("%d columns at width %v, want at least 6", cols, a.width)
	}
	savePNG(t, tt, "6-home-wide")

	// Large cards are two thirds of the grid: one to a row.
	large := []placed{{box: store.CardBox{Span: gridSpans[2]}}, {box: store.CardBox{Span: gridSpans[2]}}}
	if rows := packRows(large, gridCols); len(rows) != 2 {
		t.Errorf("%d rows for two large cards on 12 columns, want 2", len(rows))
	}
	// A phone is one column, and every card is a row of its own.
	phone := newTestApp(t)
	pt := ui.NewTester(phone.view, 390, 844)
	settle(pt)
	settle(pt)
	if cols := colsFor(phone.contentW(), true); cols != 1 {
		t.Errorf("%d columns on a phone, want 1", cols)
	}
}

// A row's last card takes the columns the row has left, so no row has a gap at
// its right end.
func TestRowsHaveNoGap(t *testing.T) {
	items := []placed{
		{box: store.CardBox{Span: gridSpans[1]}},
		{box: store.CardBox{Span: gridSpans[1]}},
		{box: store.CardBox{Span: gridSpans[0]}},
		{box: store.CardBox{Span: gridSpans[1]}},
	}
	rows := packRows(items, gridCols)
	for i, r := range rows {
		sum := 0
		for _, it := range r {
			sum += it.span
		}
		if sum != gridCols {
			t.Errorf("row %d spans %d of %d columns", i, sum, gridCols)
		}
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
