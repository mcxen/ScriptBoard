package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The cap of a page follows the user's choice; in auto, data pages fill the
// window and text pages keep a reading width.
func TestPageCap(t *testing.T) {
	cases := []struct {
		mode string
		data bool
		want float32
	}{
		{widthAuto, true, 0}, {widthAuto, false, 1120},
		{widthComfortable, true, 1120}, {widthComfortable, false, 1120},
		{widthWide, true, 1600}, {widthWide, false, 1600},
		{widthFull, true, 0}, {widthFull, false, 0},
		{"no-such-mode", false, 1120},
	}
	for _, c := range cases {
		if got := pageCap(c.mode, c.data); got != c.want {
			t.Errorf("mode %q, data %v: %v, want %v", c.mode, c.data, got, c.want)
		}
	}
	if widthIndex("wide") != 2 || widthIndex("nonsense") != 0 {
		t.Error("the settings row's place of a mode is wrong")
	}
}

// The width the user chose decides how far the detail page reaches, and what is
// sized from the page (the tiles) follows it: auto and full fill a wide window,
// comfortable keeps a width and sits in the middle.
func TestContentWidthDecidesTheDetailPage(t *testing.T) {
	edges := func(mode string) (left, right float32) {
		a := newTestApp(t)
		a.set.ContentWidth = mode
		tt := ui.NewTester(a.view, 1700, 900)
		a.router.Push("/full/weight")
		settle(tt)
		settle(tt)
		first, ok1 := tt.Access("最新")
		last, ok2 := tt.Access("振幅")
		if !ok1 || !ok2 {
			t.Fatalf("mode %q: missing the tiles", mode)
		}
		if !sameRow(first.Bounds.Y, last.Bounds.Y) {
			t.Errorf("mode %q: the tiles wrapped to two rows", mode)
		}
		return first.Bounds.X, last.Bounds.X + last.Bounds.W
	}
	for _, mode := range []string{widthAuto, widthFull} {
		if l, r := edges(mode); l > 300 || r < 1600 {
			t.Errorf("mode %q: the tiles span %v to %v of a 1700 window, want them to fill it", mode, l, r)
		}
	}
	if l, r := edges(widthComfortable); l < 380 || r > 1560 {
		t.Errorf("comfortable: the tiles span %v to %v, want a capped width in the middle", l, r)
	}
}

// The choice is made in the settings, and kept.
func TestContentWidthIsChosenInTheSettings(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 1200, 900)
	a.router.Push("/settings")
	settle(tt)
	click(t, tt, "舒适")
	if a.set.ContentWidth != widthComfortable {
		t.Fatalf("the setting is %q after choosing comfortable", a.set.ContentWidth)
	}
	if got := a.store.Settings().ContentWidth; got != widthComfortable {
		t.Errorf("kept %q", got)
	}
}
