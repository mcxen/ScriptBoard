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
	if got := chartReport(src("progress")); got != "" {
		t.Errorf("a warning for progress: %q", got)
	}
	if got := chartReport(src("calendar")); got != "" {
		t.Errorf("a warning for calendar: %q", got)
	}
}
