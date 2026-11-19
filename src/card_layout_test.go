package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

// Without a layout a result shows its legend, its summary at the top and its
// stats at the bottom; a layout changes only the parts it names.
func TestLayoutDefaults(t *testing.T) {
	def := layoutOf(engine.Result{})
	if def != (engine.Layout{Legend: "show", Summary: "top", Stats: "bottom"}) {
		t.Errorf("defaults %+v", def)
	}
	part := layoutOf(engine.Result{Layout: &engine.Layout{Stats: "hide"}})
	if part != (engine.Layout{Legend: "show", Summary: "top", Stats: "hide"}) {
		t.Errorf("a layout that names only stats: %+v", part)
	}
}

// The legend shows unless the script hides it or the user has, and the user's
// choice is kept.
func TestLegendHiddenByScriptOrUser(t *testing.T) {
	a := newTestApp(t)
	shown := engine.Layout{Legend: "show"}
	if !a.legendShown("card", shown) {
		t.Fatal("the legend is hidden by default")
	}
	if a.legendShown("card", engine.Layout{Legend: "hide"}) {
		t.Error("the script's hidden legend shows")
	}
	a.toggleLegend("card")
	if a.legendShown("card", shown) {
		t.Error("the user's hidden legend shows")
	}
	if a.legendShown("other", shown) != true {
		t.Error("hiding one card's legend hid another's")
	}
	a.toggleLegend("card")
	if !a.legendShown("card", shown) {
		t.Error("toggling back did not show the legend")
	}
}

// A script's summary and stats are their own blocks: the summary sits under the
// headline, the stats under the chart, and a layout can hide either.
func TestSummaryAndStatsAreSeparateBlocks(t *testing.T) {
	a := newTestApp(t)
	a.store.SaveScript(store.Script{ID: "blocks", Name: "分块", Home: true, Origin: store.OriginUser,
		Source: `export function meta() { return { name: "分块", sources: {} }; }
export function render(ctx) { return { title: "分块", big: "7", sub: "副标题zz", lines: [], chart: null,
  stats: [{ label: "今日统计", value: "12", sub: "" }] }; }`})
	tt := ui.NewTester(a.view, 1200, 900)
	settle(tt)
	settle(tt)
	requireText(t, tt, "副标题zz")
	requireText(t, tt, "今日统计")

	// The same figures, with a layout that hides the summary and the stats.
	a.store.SaveScript(store.Script{ID: "blocks-off", Name: "分块关", Home: true, Origin: store.OriginUser,
		Source: `export function meta() { return { name: "分块关", sources: {} }; }
export function render(ctx) { return { title: "分块关", big: "8", sub: "隐藏副标题", lines: [], chart: null,
  stats: [{ label: "隐藏统计", value: "13", sub: "" }],
  layout: { summary: "hide", stats: "hide" } }; }`})
	tt = ui.NewTester(a.view, 1200, 900)
	settle(tt)
	settle(tt)
	requireText(t, tt, "8")
	absent(t, tt, "隐藏副标题", "隐藏统计")
}

// A progress chart draws its bars on the card, with the value out of its
// maximum and the percentage.
func TestProgressChartDrawsItsBars(t *testing.T) {
	a := newTestApp(t)
	src := func(id string, names ...string) store.Script {
		bars := ""
		for i, n := range names {
			bars += `{ name: "` + n + `", value: ` + []string{"3", "20", "0"}[i] + `, max: ` + []string{"10", "20", "8"}[i] + `, unit: "章" }, `
		}
		return store.Script{ID: id, Name: "阅读进度", Home: true, Origin: store.OriginUser,
			Source: `export function meta() { return { name: "阅读进度", sources: {} }; }
export function render(ctx) { return { title: "阅读进度", big: "3", sub: "", lines: [],
  chart: { type: "progress", progress: [` + bars + `] } }; }`}
	}
	// A medium card has room for two bars, and says how many it leaves out; a
	// large card shows them all. The two cards have their own bar names.
	a.store.SaveScript(src("reading", "甲书", "乙书", "丙书"))
	a.store.SaveScript(src("reading-large", "丁书", "戊书", "己书"))
	a.setSize("reading-large", 2)
	tt := ui.NewTester(a.view, 1200, 2000)
	settle(tt)
	settle(tt)
	requireText(t, tt, "甲书")
	requireText(t, tt, "乙书")
	absent(t, tt, "丙书") // the medium card's third bar is counted, not drawn
	requireText(t, tt, "另外 1 项")
	requireText(t, tt, "丁书")
	requireText(t, tt, "戊书")
	requireText(t, tt, "己书")
}

// A card's legend names a multi-series chart's lines, and goes when the script
// or the user hides it.
func TestLegendFollowsLayoutAndUser(t *testing.T) {
	a := newTestApp(t)
	src := func(id, layout, first, second string) store.Script {
		return store.Script{ID: id, Name: "图例", Home: true, Origin: store.OriginUser,
			Source: `export function meta() { return { name: "图例", sources: {} }; }
export function render(ctx) { return { title: "图例", big: "2", sub: "", lines: [],
  chart: { type: "line", labels: ["一", "二", "三"], series: [{ name: "` + first + `", points: [1, 2, 3] }, { name: "` + second + `", points: [3, 2, 1] }] }` + layout + ` }; }`}
	}
	// Large cards draw their chart and the legend under it. The script that
	// hides its legend uses other names, so the two cards can be told apart.
	const large = 2
	a.store.SaveScript(src("legend", "", "甲线", "乙线"))
	a.setSize("legend", large)
	a.store.SaveScript(src("legend-off", `, layout: { legend: "hide" }`, "丙线", "丁线"))
	a.setSize("legend-off", large)
	tt := ui.NewTester(a.view, 1200, 900)
	settle(tt)
	settle(tt)
	requireText(t, tt, "甲线")
	requireText(t, tt, "乙线")
	absent(t, tt, "丙线", "丁线") // the script hides its legend

	// The user's choice hides it, and shows it again.
	a.toggleLegend("legend")
	settle(tt)
	settle(tt)
	absent(t, tt, "甲线", "乙线")
	a.toggleLegend("legend")
	settle(tt)
	settle(tt)
	requireText(t, tt, "甲线")
}

// A plugin's home position puts an unarranged card there; the user's own
// arrangement wins over it.
func TestPluginPositionPlacesTheCard(t *testing.T) {
	a := newTestApp(t)
	join := func(list []store.Script) string {
		ids := make([]string, len(list))
		for i, sc := range list {
			ids[i] = sc.ID
		}
		return strings.Join(ids, ",")
	}
	first := []store.Script{{ID: "a"}, {ID: "b"}, {ID: "c", Position: 1}}
	if got := join(a.orderedHome(first)); got != "c,a,b" {
		t.Errorf("position 1: %s, want c,a,b", got)
	}
	past := []store.Script{{ID: "a"}, {ID: "b"}, {ID: "c", Position: 9}}
	if got := join(a.orderedHome(past)); got != "a,b,c" {
		t.Errorf("a position past the end: %s, want a,b,c", got)
	}
	a.set.CardOrder = []string{"b", "a", "c"}
	if got := join(a.orderedHome(first)); got != "b,a,c" {
		t.Errorf("arranged by the user: %s, want b,a,c", got)
	}
}
