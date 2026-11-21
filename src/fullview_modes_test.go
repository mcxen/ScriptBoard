package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/store"
)

// Every chart/widget detail page supports switching between Visual, Data Table,
// and Raw Data inspection modes, with CSV and JSON copy.
func TestFullViewDataModes(t *testing.T) {
	a := newTestApp(t)
	var copied string
	oldCopy := copyText
	copyText = func(s string) { copied = s }
	defer func() { copyText = oldCopy }()

	tt := ui.NewTester(a.view, 1100, 760)
	a.router.Push("/full/weight")
	settle(tt)
	settle(tt)
	requireText(t, tt, "可视化")
	requireText(t, tt, "数据表")
	requireText(t, tt, "原始数据")

	// Switch to Data Table mode and verify extracted table rows + CSV copy.
	click(t, tt, "数据表")
	requireText(t, tt, "复制 CSV")
	click(t, tt, "复制 CSV")
	if !strings.Contains(copied, "标签") {
		t.Fatalf("expected CSV header in copy, got %q", copied)
	}
	savePNG(t, tt, "27-fullview-table-mode")

	// Switch to Raw Data mode and copy JSON.
	click(t, tt, "原始数据")
	requireText(t, tt, "复制 JSON")
	click(t, tt, "复制 JSON")
	if !strings.Contains(copied, `"chart"`) {
		t.Fatalf("expected JSON payload in copy, got %q", copied)
	}
	savePNG(t, tt, "27-fullview-raw-mode")

	// Switch back to Visual mode cleanly.
	click(t, tt, "可视化")
	requireText(t, tt, "样式")
}

// Runtime component swapping lets a progressBar switch to progressRing and barh
// without editing script source.
func TestFullViewRuntimeComponentSwap(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 1100, 760)
	a.router.Push("/full/ai-claude-neon")
	settle(tt)
	settle(tt)

	// Swap progressBar style to segmented.
	click(t, tt, "分段")
	if a.styles["ai-claude-neon"] != "segmented" {
		t.Fatalf("expected segmented style, got %q", a.styles["ai-claude-neon"])
	}

	// Swap component type from progressBar to progressRing.
	click(t, tt, "完成度圆环")
	if a.styles["ai-claude-neon"] != "view:progressRing" {
		t.Fatalf("expected view:progressRing swap, got %q", a.styles["ai-claude-neon"])
	}
	savePNG(t, tt, "27-fullview-swapped-ring")
}

// A script returning chart.type = "table" renders on both home cards and full view.
func TestNativeTableScriptCardAndFullView(t *testing.T) {
	a := newTestApp(t)
	src := `export function meta() { return { name: "资产表", icon: "file-spreadsheet", sources: {} }; }
export function render(ctx) {
  return {
    title: "资产表", big: "¥128,400", sub: "3 项持仓", lines: [],
    chart: {
      type: "table",
      table: {
        searchable: true, striped: true,
        columns: [
          { title: "标的" },
          { title: "现价", align: "right", format: "number" },
          { title: "涨跌", align: "right", format: "delta" },
          { title: "状态", format: "status" }
        ],
        rows: [
          ["黄金 ETF", "5.82", "+1.45%", "ok"],
          ["纳指 100", "2.18", "+0.82%", "ok"],
          ["恒生科技", "0.64", "-0.95%", "warn"]
        ]
      }
    }
  };
}`
	if err := a.store.SaveScript(store.Script{ID: "asset-table", Name: "资产表", Source: src, Home: true, Origin: store.OriginUser}); err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(a.view, 1100, 760)
	a.router.Push("/full/asset-table")
	settle(tt)
	settle(tt)
	requireText(t, tt, "黄金 ETF")
	requireText(t, tt, "纳指 100")
	requireText(t, tt, "+1.45%")
	// Native table can also swap to KV status view!
	click(t, tt, "状态面板")
	requireText(t, tt, "黄金 ETF")
}
