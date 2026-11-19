package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The detail page uses the whole window, as the home page does: on a wide
// window the six statistics tiles sit in one row, not wrapped at a reading
// width with blank space to the right of the page.
func TestFullViewUsesTheWholeWindow(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 1700, 1000)
	a.router.Push("/full/weight")
	settle(tt)
	settle(tt)
	first, ok1 := tt.Access("最新")
	last, ok2 := tt.Access("振幅")
	if !ok1 || !ok2 {
		t.Fatalf("missing the tiles (%v, %v)", ok1, ok2)
	}
	if !sameRow(first.Bounds.Y, last.Bounds.Y) {
		t.Errorf("the first tile is at y=%v and the last at y=%v: they wrapped to two rows", first.Bounds.Y, last.Bounds.Y)
	}
	// The last tile reaches the right side of the window, not stops at 1120.
	if right := last.Bounds.X + last.Bounds.W; right < 1500 {
		t.Errorf("the tiles end at x=%v of a 1700 window", right)
	}
}

// sameRow says whether two things sit on one row: their tops are within a few
// pixels. Another system's fonts shift a line by half a pixel; a wrapped row is
// a whole tile lower.
func sameRow(a, b float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 4
}
