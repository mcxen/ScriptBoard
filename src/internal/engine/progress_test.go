package engine

import (
	"context"
	"strings"
	"testing"
	"time"
)

// A progress bar is its value out of its maximum, kept within 0 to 100.
func TestProgressPercent(t *testing.T) {
	cases := []struct {
		p    Progress
		want float64
	}{
		{Progress{Value: 2, Max: 5}, 40},
		{Progress{Value: 30}, 30}, // no maximum: the value is the percentage
		{Progress{Value: 150, Max: 100}, 100},
		{Progress{Value: -4, Max: 10}, 0},
		{Progress{Value: 0, Max: 0}, 0},
	}
	for _, c := range cases {
		if got := c.p.Percent(); got != c.want {
			t.Errorf("%+v: %v, want %v", c.p, got, c.want)
		}
	}
}

func TestProgressIsAKnownChartType(t *testing.T) {
	if UnknownChartType("progress") || UnknownChartType("progressRing") {
		t.Error("progress or progressRing is reported as unknown")
	}
	if !UnknownChartType("pie") {
		t.Error("pie is not reported as unknown")
	}
	if UnknownChartType("") {
		t.Error("an empty type (a line) is reported as unknown")
	}
}

// A script's progress chart and layout arrive in the result.
func TestProgressChartAndLayoutFromScript(t *testing.T) {
	src := `export function meta() { return { name: "进度", sources: {} }; }
export function render(ctx) { return { title: "进度", big: "3/10", sub: "", lines: [],
  chart: { type: "progress", progress: [{ name: "三体", value: 3, max: 10, unit: "章" }] },
  layout: { legend: "hide", summary: "hide", stats: "hide" } }; }`
	s, err := CompileLang("progress.js", src, "zh")
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Run(context.Background(), Input{Now: time.Now(), Lang: "zh"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Chart == nil || res.Chart.Type != "progress" || len(res.Chart.Progress) != 1 {
		t.Fatalf("chart %+v", res.Chart)
	}
	p := res.Chart.Progress[0]
	if p.Name != "三体" || p.Value != 3 || p.Max != 10 || p.Unit != "章" || p.Percent() != 30 {
		t.Errorf("progress %+v", p)
	}
	if res.Layout == nil || res.Layout.Legend != "hide" || res.Layout.Summary != "hide" || res.Layout.Stats != "hide" {
		t.Errorf("layout %+v", res.Layout)
	}
	if strings.TrimSpace(res.Title) == "" {
		t.Error("the title is empty")
	}
}

// A ring's options and an item's colors arrive in the result, and a value over
// its maximum is more than a whole.
func TestProgressRingOptionsFromScript(t *testing.T) {
	src := `export function meta() { return { name: "环", sources: {} }; }
export function render(ctx) { return { title: "环", big: "", sub: "", lines: [],
  chart: { type: "progressRing",
    ring: { layout: "concentric", size: 96, thickness: 0.2, cap: "butt", track: "none", fill: "solid", center: "value", align: "center", valign: "middle" },
    progress: [{ name: "移动", value: 130, max: 100, color: "#F43F5E", to: "#FB923C" }] } }; }`
	s, err := CompileLang("ring.js", src, "zh")
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Run(context.Background(), Input{Now: time.Now(), Lang: "zh"})
	if err != nil {
		t.Fatal(err)
	}
	r := res.Chart.Ring
	if r == nil || r.Layout != "concentric" || r.Size != 96 || r.Thickness != 0.2 || r.Cap != "butt" ||
		r.Track != "none" || r.Fill != "solid" || r.Center != "value" || r.Align != "center" || r.VAlign != "middle" {
		t.Fatalf("ring %+v", r)
	}
	p := res.Chart.Progress[0]
	if p.Color != "#F43F5E" || p.To != "#FB923C" {
		t.Errorf("colors %+v", p)
	}
	if p.Raw() != 130 || p.Percent() != 100 {
		t.Errorf("raw %v, percent %v, want 130 and 100", p.Raw(), p.Percent())
	}
}
