package main

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/store"
)

// Recording a value refreshes only the cards that read it: the weight card
// runs again, and the others keep their runs.
func TestRecordRefreshesOnlyItsCards(t *testing.T) {
	a := newTestApp(t)
	a.runs["weight"] = &runState{}
	a.runs["fx"] = &runState{}
	if err := a.recordMetric("weight", 65.2); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.runs["weight"]; ok {
		t.Errorf("the weight card kept its run after a weight was recorded")
	}
	if _, ok := a.runs["fx"]; !ok {
		t.Errorf("an unrelated card lost its run")
	}
}

// The card's own right-click menu is the system's native menu, which the Tester
// cannot open, so the choice is made directly here.
func TestRightClickChangesOnlyThatCard(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 1100, 760)
	settle(tt)
	// The headline: a card has no title row.
	if err := tt.RightClick("65.3 kg"); err != nil {
		t.Fatalf("right click: %v", err)
	}
	settle(tt)
	click(t, tt, "宽")
	if got := a.boxOf("weight").Span; got != gridSpans[3] {
		t.Fatalf("weight card span %d after choosing wide, want %d", got, gridSpans[3])
	}
	if got := a.boxOf("fx").Span; got == gridSpans[3] {
		t.Errorf("another card changed size with it")
	}
}

// Dragging the corner of a card changes its width and height, and the snapped
// result is kept when the drag ends.
func TestResizeCorner(t *testing.T) {
	a := newTestApp(t)
	a.cols = gridCols
	a.unit = 60
	a.rz = &resize{id: "weight", span: 4, height: 208, dx: 200, dy: 40}
	a.applyResize()
	b := a.set.CardBoxes["weight"]
	if b.Span <= 4 {
		t.Errorf("span %d after dragging the corner right, want more than 4", b.Span)
	}
	if math.Mod(float64(b.Height), heightStep) != 0 || b.Height <= 208 {
		t.Errorf("height %v, want a step above 208", b.Height)
	}
}

// Moving a card puts it before the card it is dropped on, and the order is
// kept.
func TestMoveCard(t *testing.T) {
	a := newTestApp(t)
	a.homeIDs = []string{"weight", "fx", "weather"}
	a.moveCard("weather", "weight")
	want := []string{"weather", "weight", "fx"}
	for i, id := range want {
		if a.set.CardOrder[i] != id {
			t.Fatalf("order %v, want %v", a.set.CardOrder, want)
		}
	}
	if got := a.orderedHome([]store.Script{{ID: "fx"}, {ID: "weight"}, {ID: "weather"}}); got[0].ID != "weather" || got[2].ID != "fx" {
		t.Errorf("home order %v does not follow the arrangement", got)
	}
}

// Dragging a card's corner in edit mode resizes it, with the pointer as the
// user does it: press, move, let go.
func TestEditModeDragResizesACard(t *testing.T) {
	a := newTestApp(t)
	a.editing = true
	tt := ui.NewTester(a.view, 1280, 900)
	settle(tt)
	settle(tt)
	before := a.boxOf("weight").Span
	h, ok := tt.Access("resize:weight")
	if !ok {
		t.Fatal("no corner handle on the weight card in edit mode")
	}
	x, y := h.Bounds.X+h.Bounds.W/2, h.Bounds.Y+h.Bounds.H/2
	tt.Press(x, y)
	settle(tt)
	for i := 1; i <= 10; i++ {
		tt.Move(x+float32(i)*25, y+float32(i)*3)
		settle(tt)
	}
	tt.Release(x+250, y+30)
	settle(tt)
	settle(tt)
	if got := a.boxOf("weight").Span; got <= before {
		t.Errorf("span %d after dragging the corner right, was %d", got, before)
	}
	if a.rz != nil {
		t.Error("the resize did not end with the release")
	}
	if got := a.store.Settings().CardBoxes["weight"].Span; got != a.boxOf("weight").Span {
		t.Errorf("the resize was not saved: store %d, page %d", got, a.boxOf("weight").Span)
	}
}

// Dragging a card onto another moves it to that card's place.
func TestEditModeDragMovesACard(t *testing.T) {
	a := newTestApp(t)
	a.editing = true
	tt := ui.NewTester(a.view, 1280, 900)
	settle(tt)
	settle(tt)
	from, ok1 := tt.Access("card:weather")
	to, ok2 := tt.Access("card:weight")
	if !ok1 || !ok2 {
		t.Fatal("no card in edit mode")
	}
	x, y := from.Bounds.X+from.Bounds.W/2, from.Bounds.Y+from.Bounds.H/2
	tt.Press(x, y)
	settle(tt)
	tx, ty := to.Bounds.X+to.Bounds.W/2, to.Bounds.Y+to.Bounds.H/2
	for i := 1; i <= 12; i++ {
		tt.Move(x+(tx-x)*float32(i)/12, y+(ty-y)*float32(i)/12)
		settle(tt)
	}
	tt.Release(tx, ty)
	settle(tt)
	settle(tt)
	if len(a.set.CardOrder) == 0 || a.set.CardOrder[0] != "weather" {
		t.Errorf("order %v after dropping weather on weight, want weather first", a.set.CardOrder)
	}
}
