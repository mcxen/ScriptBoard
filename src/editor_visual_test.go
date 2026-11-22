package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
)

// Visual editor supports the native table template, in-place component swapping
// with data migration, and Tray menu bar display configuration.
func TestVisualEditorTableAndInPlaceSwap(t *testing.T) {
	a := newTestApp(t)
	a.newScript()
	a.applyTemplate("progressBar")
	if a.visual.cfg == nil || a.visual.cfg.Type != "progressBar" || len(a.visual.cfg.Progress) != 3 {
		t.Fatalf("progressBar template failed: %+v", a.visual.cfg)
	}
	a.visual.cfg.Progress[0].Name = "自定义目标"
	a.applyVisualConfig()

	// Swap in-place progressBar -> progressRing keeps existing items.
	a.swapVisualType("progressRing")
	if a.visual.cfg.Type != "progressRing" || a.visual.cfg.Progress[0].Name != "自定义目标" {
		t.Fatalf("swap to progressRing lost data: %+v", a.visual.cfg)
	}

	// Swap in-place progressRing -> table migrates progress rows into TableSpec.
	a.swapVisualType("table")
	if a.visual.cfg.Type != "table" || len(a.visual.cfg.Table.Rows) != 3 || a.visual.cfg.Table.Rows[0][0] != "自定义目标" {
		t.Fatalf("swap to table failed: %+v", a.visual.cfg.Table)
	}

	// Swap table -> kv preserves row labels and values.
	a.swapVisualType("kv")
	if a.visual.cfg.Type != "kv" || len(a.visual.cfg.KV) != 3 || a.visual.cfg.KV[0].Label != "自定义目标" {
		t.Fatalf("swap to kv failed: %+v", a.visual.cfg.KV)
	}

	// Verify table template renders cleanly in the visual editor UI and emits runnable JS.
	a.applyTemplate("table")
	a.visual.cfg.Menubar = true
	a.visual.cfg.MenuBarCfg = &engine.MenuBar{Title: "行情", Click: "open", Mono: true}
	a.applyVisualConfig()
	if !strings.Contains(a.draft.Source, `"type": "table"`) || !strings.Contains(a.draft.Source, `"menubarCfg"`) {
		t.Fatalf("generated source missing table/menubarCfg: %s", a.draft.Source)
	}
	tt := ui.NewTester(a.view, 1400, 900)
	a.router.Push("/edit/" + a.draft.ID)
	settle(tt)
	// Tab 0 (内容): data table config.
	requireText(t, tt, "数据表格配置")
	// Tab 1 (样式): table style + card display.
	click(t, tt, "样式")
	requireText(t, tt, "表格样式")
	requireText(t, tt, "卡片显示")
	// Tab 2 (更多): platform & menu bar display options.
	click(t, tt, "更多")
	requireText(t, tt, "菜单栏显示配置")
	savePNG(t, tt, "30-visual-editor-table")
}
