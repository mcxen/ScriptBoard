package engine

import (
	"context"
	"testing"
	"time"
)

// A combo chart's curve, its dashed projection and its rings come through
// from the script as the chart's fields.
func TestComboChartFieldsFromScript(t *testing.T) {
	src := `export function meta() { return { name: "combo", sources: {} }; }
export function render(ctx) {
  return { title: "t", big: "1", sub: "", lines: [], chart: {
    type: "combo",
    series: [
      { name: "solid", color: "#2B5FCC", points: [1, 2, 3] },
      { name: "projection", color: "#8E8EE8", points: [3, 4, 5], offset: 2, dashed: true },
    ],
    labels: ["a", "b", "c", "d", "e"],
    rings: [{ label: "积分", percent: 100, color: "#B45309" }, { label: "额外积分", percent: 61 }],
  } };
}`
	s, err := Compile("combo.js", src)
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Run(context.Background(), Input{Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	ch := res.Chart
	if ch == nil || ch.Type != "combo" {
		t.Fatalf("chart %+v", ch)
	}
	if len(ch.Series) != 2 || !ch.Series[1].Dashed || ch.Series[1].Offset != 2 || ch.Series[0].Dashed {
		t.Errorf("series %+v", ch.Series)
	}
	if len(ch.Rings) != 2 || ch.Rings[0].Percent != 100 || ch.Rings[1].Label != "额外积分" || ch.Rings[1].Color != "" {
		t.Errorf("rings %+v", ch.Rings)
	}
}
