package main

import (
	"context"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
)

// comboSample is the card of a points account: a curve that is solid up to
// today and dashed after it, and three rings beside it.
func comboSample(t *testing.T) *engine.Chart {
	t.Helper()
	src := `export function meta() { return { name: "combo", sources: {} }; }
export function render(ctx) {
  return { title: "积分", big: "100%", sub: "", lines: [], chart: {
    type: "combo",
    series: [
      { name: "积分", color: "#2B5FCC", points: [40, 42, 47, 50, 55, 61] },
      { name: "预计", color: "#8E8EE8", points: [61, 58, 55, 52], offset: 5, dashed: true },
    ],
    labels: ["1日", "2日", "3日", "4日", "5日", "6日", "7日", "8日", "9日", "10日"],
    rings: [
      { label: "积分", percent: 100, color: "#B45309" },
      { label: "额外积分", percent: 61 },
      { label: "积分合计", percent: 66 },
    ],
  } };
}`
	s, err := engine.Compile("combo.js", src)
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Run(context.Background(), engine.Input{Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	return res.Chart
}

// A combo card shows its curve beside its rings: the rings' labels and
// percents are on the page, and the card and the full view both draw it.
func TestComboCardAndFullView(t *testing.T) {
	ch := comboSample(t)
	card := func(c *ui.Context) {
		applySkin(c)
		Spark(c, ch, 96)
	}
	tt := ui.NewTester(card, 520, 150)
	settle(tt)
	requireText(t, tt, "61%")
	requireText(t, tt, "额外积分")
	savePNG(t, tt, "18-combo-card")

	full := func(c *ui.Context) {
		applySkin(c)
		Chart(c, ChartProps{Data: ch, Height: 260, Full: true})
	}
	tf := ui.NewTester(full, 720, 320)
	settle(tf)
	requireText(t, tf, "积分合计")
	savePNG(t, tf, "18-combo-full")
}

// The rings and the dashed projection are part of the combo chart's own
// layout: a combo with no rings is just its curve.
func TestComboWithoutRings(t *testing.T) {
	ch := comboSample(t)
	ch.Rings = nil
	view := func(c *ui.Context) {
		applySkin(c)
		Spark(c, ch, 80)
	}
	tt := ui.NewTester(view, 400, 110)
	settle(tt)
	if tt.HasText("61%") {
		t.Errorf("rings drawn without rings")
	}
}
