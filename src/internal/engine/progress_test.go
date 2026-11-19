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
	if UnknownChartType("progress") {
		t.Error("progress is reported as unknown")
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
