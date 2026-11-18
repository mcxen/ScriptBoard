package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
)

// The record button of a quick entry stays inside a medium card, however long
// the input's label is.
func TestQuickEntryFitsTheCard(t *testing.T) {
	a := newTestApp(t)
	in := engine.ManualInput{Metric: "gold", Label: "金价实时报价很长的名字", Unit: "元/克", Step: 0.1}
	const cardW = 300
	view := func(c *ui.Context) {
		applySkin(c)
		ui.Row(c).Width(cardW).Children(func() {
			a.quickEntry(c, "quick-test", in)
		})
	}
	tt := ui.NewTester(view, cardW+40, 80)
	settle(tt)
	info, ok := tt.Access("记录")
	if !ok {
		t.Fatal("no record button on the card")
	}
	if right := info.Bounds.X + info.Bounds.W; right > cardW {
		t.Errorf("record button ends at %.0f, past the %d DIP card", right, cardW)
	}
}
