package mcpsvc

import (
	"strings"
	"testing"
)

// script_check names a chart type the host does not draw, and says which ones
// it draws; progress is one of them.
func TestChartReportNamesAnUnknownType(t *testing.T) {
	src := func(typ string) string {
		return `export function meta() { return { name: "x", sources: {} }; }
export function render(ctx) { return { title: "x", big: "1", sub: "", lines: [],
  chart: { type: "` + typ + `", progress: [{ name: "a", value: 1, max: 2 }] } }; }`
	}
	got := chartReport(src("pie"))
	if !strings.Contains(got, "图表类型 pie 不认识") || !strings.Contains(got, "progress") {
		t.Errorf("no warning for pie: %q", got)
	}
	for _, okType := range []string{"progress", "progressBar", "progressRing", "calendar", "donut", "checklist", "kv", "countdown", "clock", "contrib", "table"} {
		if got := chartReport(src(okType)); got != "" {
			t.Errorf("a warning for %s: %q", okType, got)
		}
	}
	standalonePB := `export function meta() { return { name: "pb", sources: {} }; }
export function render(ctx) { return { title: "pb", big: "78%", sub: "", lines: [],
  progressBar: { style: "neon", items: [{ name: "a", value: 78, max: 100 }] } }; }`
	if got := chartReport(standalonePB); got != "" {
		t.Errorf("a warning for standalone progressBar: %q", got)
	}
}
