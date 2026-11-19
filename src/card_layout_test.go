package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

// hover moves the pointer onto the element with this text, so that the card
// around it is hovered: a card's buttons show while the pointer is on it.
func hover(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	info, ok := tt.Access(label)
	if !ok {
		t.Fatalf("no %q to hover", label)
	}
	tt.Move(info.Bounds.X+info.Bounds.W/2, info.Bounds.Y+info.Bounds.H/2)
	settle(tt)
}

// Without a layout a result shows its legend, its summary at the top and its
// stats at the bottom; a layout changes only the parts it names.
func TestLayoutDefaults(t *testing.T) {
	def := layoutOf(engine.Result{})
	if def != (engine.Layout{Legend: "show", Summary: "top", Stats: "bottom", Actions: "hover", Image: "cover"}) {
		t.Errorf("defaults %+v", def)
	}
	part := layoutOf(engine.Result{Layout: &engine.Layout{Stats: "hide"}})
	if part != (engine.Layout{Legend: "show", Summary: "top", Stats: "hide", Actions: "hover", Image: "cover"}) {
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

// A progressRing chart draws one ring per item, with its percentage, name and
// count. A medium card has room for three and says how many it leaves out; a
// large card shows them all.
func TestProgressRingDrawsItsRings(t *testing.T) {
	a := newTestApp(t)
	src := func(id string, names ...string) store.Script {
		bars := ""
		for i, n := range names {
			bars += `{ name: "` + n + `", value: ` + []string{"3", "20", "0", "5"}[i] + `, max: ` + []string{"10", "20", "8", "10"}[i] + `, unit: "章" }, `
		}
		return store.Script{ID: id, Name: "环", Home: true, Origin: store.OriginUser,
			Source: `export function meta() { return { name: "环", sources: {} }; }
export function render(ctx) { return { title: "环", big: "4", sub: "", lines: [],
  chart: { type: "progressRing", progress: [` + bars + `] } }; }`}
	}
	a.store.SaveScript(src("ring", "甲环", "乙环", "丙环", "丁环"))
	a.store.SaveScript(src("ring-large", "戊环", "己环", "庚环", "辛环"))
	a.setSize("ring-large", 2)
	tt := ui.NewTester(a.view, 1200, 2000)
	settle(tt)
	settle(tt)
	requireText(t, tt, "甲环")
	requireText(t, tt, "丙环")
	absent(t, tt, "丁环") // the medium card's fourth ring is counted, not drawn
	requireText(t, tt, "另外 1 项")
	requireText(t, tt, "3 / 10章") // a ring's count
	requireText(t, tt, "30%")     // and its percentage
	for _, n := range []string{"戊环", "己环", "庚环", "辛环"} {
		requireText(t, tt, n)
	}
}

// ringCard saves a large card with one progress ring, with the given ring
// options, and its own title and ring name.
func ringCard(a *app, id, title, ring string) {
	a.store.SaveScript(store.Script{ID: id, Name: title, Home: true, Origin: store.OriginUser,
		Source: `export function meta() { return { name: "` + title + `", sources: {} }; }
export function render(ctx) { return { title: "` + title + `", big: "", sub: "", lines: [],
  chart: { type: "progressRing", ring: { ` + ring + ` }, progress: [{ name: "` + id + `-name", value: 3, max: 10 }] } }; }`})
	a.setSize(id, 2)
}

// The rings sit where the script puts them across the card: left, center or
// right.
func TestProgressRingsAlignAcrossTheCard(t *testing.T) {
	a := newTestApp(t)
	ringCard(a, "ra-left", "标题甲", `align: "left"`)
	ringCard(a, "ra-center", "标题乙", `align: "center"`)
	ringCard(a, "ra-right", "标题丙", `align: "right"`)
	for _, sc := range a.store.Scripts() {
		sc.Home = len(sc.ID) > 3 && sc.ID[:3] == "ra-"
		a.store.SaveScript(sc)
	}
	tt := ui.NewTester(a.view, 1200, 1800)
	settle(tt)
	settle(tt)
	x := func(id string) float32 {
		info, ok := tt.Access(id + "-name")
		if !ok {
			t.Fatalf("no ring %s", id)
		}
		return info.Bounds.X + info.Bounds.W/2
	}
	l, c, r := x("ra-left"), x("ra-center"), x("ra-right")
	if !(l+150 < c && c+150 < r) {
		t.Errorf("left %v, center %v, right %v: want them well apart, in that order", l, c, r)
	}
}

// The rings sit where the script puts them down the card: right under the
// summary by default, in the middle, or at the bottom.
func TestProgressRingsAlignDownTheCard(t *testing.T) {
	a := newTestApp(t)
	ringCard(a, "rv-top", "标题甲", `valign: "top"`)
	ringCard(a, "rv-middle", "标题乙", `valign: "middle"`)
	ringCard(a, "rv-bottom", "标题丙", `valign: "bottom"`)
	for _, sc := range a.store.Scripts() {
		sc.Home = len(sc.ID) > 3 && sc.ID[:3] == "rv-"
		a.store.SaveScript(sc)
	}
	tt := ui.NewTester(a.view, 1200, 1800)
	settle(tt)
	settle(tt)
	below := func(id, title string) float32 { // how far under its card's title the ring's name is
		ring, ok1 := tt.Access(id + "-name")
		head, ok2 := tt.Access(title)
		if !ok1 || !ok2 {
			t.Fatalf("no ring or title for %s", id)
		}
		return ring.Bounds.Y - head.Bounds.Y
	}
	top, mid, bot := below("rv-top", "标题甲"), below("rv-middle", "标题乙"), below("rv-bottom", "标题丙")
	if !(top+60 < mid && mid+60 < bot) {
		t.Errorf("top %v, middle %v, bottom %v: want them apart, in that order", top, mid, bot)
	}
	// The ring's name is under the ring, which is under the header and the
	// headline: right under the summary is about 170 down, not half the card.
	if top > 230 {
		t.Errorf("the default is right under the summary, and the rings are %v down", top)
	}
}

// Concentric rings nest in one and say what each is in a legend beside it.
func TestConcentricRingsHaveALegend(t *testing.T) {
	a := newTestApp(t)
	a.store.SaveScript(store.Script{ID: "act", Name: "活动", Home: true, Origin: store.OriginUser,
		Source: `export function meta() { return { name: "活动", sources: {} }; }
export function render(ctx) { return { title: "活动", big: "", sub: "", lines: [],
  chart: { type: "progressRing", ring: { layout: "concentric" }, progress: [
    { name: "移动", value: 520, max: 600, unit: "千卡" }, { name: "锻炼", value: 41, max: 30, unit: "分钟" }] } }; }`})
	a.setSize("act", 2)
	tt := ui.NewTester(a.view, 1200, 1400)
	settle(tt)
	settle(tt)
	for _, want := range []string{"移动", "520 / 600千卡", "锻炼", "41 / 30分钟"} {
		requireText(t, tt, want)
	}
}
