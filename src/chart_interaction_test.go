package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/store"
)

// Note: pinnedSels must persist across frames when mutated via *pinned pointer!
func TestFullViewKeyboardScrubAndLegendToggle(t *testing.T) {
	a := newTestApp(t)
	src := `export function meta() { return { name: "双线对比", icon: "chart-line", sources: {} }; }
export function render(ctx) {
  return {
    title: "双线对比", big: "120", sub: "两项指标", lines: [],
    chart: {
      type: "line",
      labels: ["D1", "D2", "D3", "D4"],
      series: [
        { name: "收入", color: "#3B82F6", points: [100, 110, 105, 120] },
        { name: "支出", color: "#F97316", points: [60, 65, 62, 70] }
      ]
    }
  };
}`
	if err := a.store.SaveScript(store.Script{ID: "dual-line", Name: "双线对比", Source: src, Home: true, Origin: store.OriginUser}); err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(a.view, 1100, 760)
	a.router.Push("/full/dual-line")
	settle(tt)
	settle(tt)

	// Toggle "支出" series off via interactive legend.
	click(t, tt, "支出")
	if !a.hiddenSeries["dual-line"][1] {
		t.Fatalf("expected series 1 to be hidden after clicking legend")
	}
	requireText(t, tt, "全部显示")
	// Trying to hide the last visible series ("收入") must be refused.
	click(t, tt, "收入")
	if a.hiddenSeries["dual-line"][0] {
		t.Fatalf("must not allow hiding the last visible series")
	}
	// Restore all series.
	click(t, tt, "全部显示")
	if len(a.hiddenSeries["dual-line"]) != 0 {
		t.Fatalf("expected all series restored, got %v", a.hiddenSeries["dual-line"])
	}

	// Keyboard scrubbing: Left selects last point, Left again steps back.
	tt.Key(0, ui.KeyLeft)
	settle(tt)
	sel := a.selOf("full:dual-line")
	if *sel != 3 {
		t.Fatalf("expected KeyLeft to select index 3, got %d", *sel)
	}
	requireText(t, tt, "取消固定")
	tt.Key(0, ui.KeyLeft)
	settle(tt)
	if *sel != 2 {
		t.Fatalf("expected 2nd KeyLeft to select index 2, got %d", *sel)
	}
	tt.Key(0, ui.KeyHome)
	settle(tt)
	if *sel != 0 {
		t.Fatalf("expected KeyHome to select index 0, got %d", *sel)
	}
	savePNG(t, tt, "29-chart-interactive")
	click(t, tt, "取消固定")
	if *sel != -1 {
		t.Fatalf("expected unpin to clear selection, got %d", *sel)
	}
}

// Non-plot widgets support rich interactions: donut slice highlight, checklist
// live toggle, KV click-to-copy, and clock style switching.
func TestWidgetInteractions(t *testing.T) {
	a := newTestApp(t)
	var copied string
	oldCopy := copyText
	copyText = func(s string) { copied = s }
	defer func() { copyText = oldCopy }()

	// 1. Donut interactive legend selection updates center readout.
	donutSrc := `export function meta() { return { name: "占比", icon: "circle-dollar-sign", sources: {} }; }
export function render(ctx) {
  return {
    title: "占比", big: "", sub: "", lines: [],
    chart: { type: "donut", slices: [{ name: "餐饮", value: 250 }, { name: "房租", value: 750 }] }
  };
}`
	_ = a.store.SaveScript(store.Script{ID: "int-donut", Name: "占比", Source: donutSrc, Origin: store.OriginUser})
	tt := ui.NewTester(a.view, 1000, 720)
	a.router.Push("/full/int-donut")
	settle(tt)
	click(t, tt, "餐饮")
	if *a.selOf("full:int-donut") != 0 {
		t.Fatalf("expected donut slice 0 selected, got %d", *a.selOf("full:int-donut"))
	}
	requireText(t, tt, "25%")

	// 2. Checklist live toggle updates completion count.
	chkSrc := `export function meta() { return { name: "清单", icon: "activity", sources: {} }; }
export function render(ctx) {
  return {
    title: "清单", big: "", sub: "", lines: [],
    chart: { type: "checklist", checklist: [{ label: "写测试", done: false }, { label: "发版", done: true }] }
  };
}`
	_ = a.store.SaveScript(store.Script{ID: "int-chk", Name: "清单", Source: chkSrc, Origin: store.OriginUser})
	a.router.Push("/full/int-chk")
	settle(tt)
	requireText(t, tt, "已完成 1 / 2 · 50%")
	click(t, tt, "写测试")
	requireText(t, tt, "已完成 2 / 2 · 100%")

	// 3. KV click-to-copy copies value to clipboard.
	kvSrc := `export function meta() { return { name: "状态", icon: "activity", sources: {} }; }
export function render(ctx) {
  return {
    title: "状态", big: "", sub: "", lines: [],
    chart: { type: "kv", kv: [{ label: "网关延迟", value: "18 ms", status: "ok" }] }
  };
}`
	_ = a.store.SaveScript(store.Script{ID: "int-kv", Name: "状态", Source: kvSrc, Origin: store.OriginUser})
	a.router.Push("/full/int-kv")
	settle(tt)
	click(t, tt, "18 ms")
	if copied != "18 ms" {
		t.Fatalf("expected KV copy '18 ms', got %q", copied)
	}

	// 4. Clock runtime style switch (analog -> digital).
	a.router.Push("/full/world-clock")
	settle(tt)
	click(t, tt, "数字")
	if a.styles["world-clock"] != "digital" {
		t.Fatalf("expected clock style digital, got %q", a.styles["world-clock"])
	}
}
