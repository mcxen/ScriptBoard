package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/cmdrun"
	"scriptboard/src/internal/store"
)

func TestFusedTopBarAndTrafficLightClearance(t *testing.T) {
	for _, width := range []int{1280, 960} {
		t.Run(fmt.Sprintf("w%d", width), func(t *testing.T) {
			a := newTestApp(t)
			tt := ui.NewTester(a.view, width, 760)
			settle(tt)
			settle(tt)

			topLabels := []string{"总览", "网络排查", "小", "中", "大", "编辑布局", "收起侧栏"}
			for _, lbl := range topLabels {
				info, ok := tt.Access(lbl)
				if !ok {
					t.Fatalf("missing top bar control %q at width %d", lbl, width)
				}
				if info.Bounds.Y < 0 || info.Bounds.Y+info.Bounds.H > titleRow+1 {
					t.Errorf("%q bounds %+v exceed fused top bar height %d", lbl, info.Bounds, titleRow)
				}
				if info.Bounds.X+info.Bounds.W > float32(width)+0.5 {
					t.Errorf("%q bounds %+v overflow right edge %d", lbl, info.Bounds, width)
				}
				if !ownControls && info.Bounds.X < trafficInset-1 {
					t.Errorf("%q overlaps macOS traffic lights zone (X=%.1f < %d)", lbl, info.Bounds.X, trafficInset)
				}
			}

			// Brand title sits right of macOS traffic lights inside the fused sidebar header.
			brand, ok := tt.Access("盯盯板")
			if !ok {
				t.Fatal("missing brand text 盯盯板")
			}
			if brand.Bounds.Y+brand.Bounds.H > titleRow+1 {
				t.Errorf("brand bounds %+v exceed fused top bar height %d", brand.Bounds, titleRow)
			}
			if !ownControls && brand.Bounds.X < trafficInset-1 {
				t.Errorf("brand overlaps traffic lights zone (X=%.1f < %d)", brand.Bounds.X, trafficInset)
			}

			// Collapse sidebar into rail and ensure expand button remains inside fused bar clear of traffic lights.
			click(t, tt, "收起侧栏")
			exp, ok := tt.Access("展开侧栏")
			if !ok {
				t.Fatal("missing 展开侧栏 in fused bar after collapse")
			}
			if exp.Bounds.Y+exp.Bounds.H > titleRow+1 {
				t.Errorf("展开侧栏 bounds %+v exceed fused top bar height %d", exp.Bounds, titleRow)
			}
			if !ownControls && exp.Bounds.X < trafficInset-1 {
				t.Errorf("展开侧栏 overlaps traffic lights zone (X=%.1f < %d)", exp.Bounds.X, trafficInset)
			}
			click(t, tt, "展开侧栏")
		})
	}
}

func TestNetworkDiagnosticsViewLightAndDark(t *testing.T) {
	t.Cleanup(func() { setThemeMode(themeSystem); setSkin(skinDefault); setLang(langZH) })
	a := newIPBentoTestApp(t)

	// Seed deterministic offline ping history so sb-ping renders rich RTT stats & sparkline.
	now := a.now().UnixMilli()
	for i, rtt := range []float64{18.4, 21.2, 16.9, 17.6} {
		_ = a.cmdHistory.Append(cmdrun.BuiltinPingID, cmdrun.Record{
			ID:        fmt.Sprintf("seed-ping-%d", i),
			CommandID: cmdrun.BuiltinPingID,
			T:         now - int64((3-i)*60_000),
			Trigger:   "schedule",
			Params:    map[string]string{"host": "223.5.5.5"},
			OK:        true,
			Extracted: map[string]any{"host": "223.5.5.5", "resolvedIp": "223.5.5.5", "avgMs": rtt, "minMs": rtt - 1.2, "maxMs": rtt + 1.5, "jitterMs": 1.1, "lossPct": 0.0},
		}, 100)
	}

	setThemeMode(themeLight)
	tt := ui.NewTester(a.view, 1280, 860)
	settle(tt)
	settle(tt)

	click(t, tt, "网络排查")
	settle(tt)
	settle(tt)
	if a.set.ActiveHomeView != homeViewNetworkID {
		t.Fatalf("ActiveHomeView = %q, want %q", a.set.ActiveHomeView, homeViewNetworkID)
	}
	requireText(t, tt, "网络 Ping")
	requireText(t, tt, "39.180.138.19")
	requireText(t, tt, "网络连通性")
	absent(t, tt, "体重 · kg", "USD / CNY")
	savePNG(t, tt, "35-network-view-light")

	setThemeMode(themeDark)
	settle(tt)
	settle(tt)
	requireText(t, tt, "网络 Ping")
	requireText(t, tt, "39.180.138.19")
	savePNG(t, tt, "35-network-view-dark")

	// Switching back to Overview restores full widget grid.
	click(t, tt, "总览")
	requireText(t, tt, "体重 · kg")
}

func TestCustomMultiViewCreateEditReorderPersistAndDelete(t *testing.T) {
	dir := t.TempDir()
	a, err := newApp(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.closeBarInstances)

	tt := ui.NewTester(a.view, 1280, 860)
	settle(tt)

	// 1. Open New View editor from top bar "+", set name & pick weight + fx only.
	click(t, tt, "新建视图")
	requireText(t, tt, "包含组件（已选 0 个）")
	a.viewEdit.name = "桌面精选"
	click(t, tt, "体重记录")
	click(t, tt, "汇率")
	settle(tt)
	savePNG(t, tt, "35-home-view-editor")
	click(t, tt, "保存视图")

	if a.set.ActiveHomeView == homeViewOverviewID || a.set.ActiveHomeView == homeViewNetworkID {
		t.Fatalf("expected active view switched to newly created custom view, got %q", a.set.ActiveHomeView)
	}
	requireText(t, tt, "体重 · kg")
	requireTextStart(t, tt, "USD / CNY")
	absent(t, tt, "世界时钟")

	// 2. Reorder cards on the custom view without affecting Overview order.
	customID := a.set.ActiveHomeView
	a.homeIDs = []string{"weight", "fx"}
	a.moveCard("fx", "weight")
	if got := a.activeHomeScripts(); len(got) != 2 || got[0].ID != "fx" || got[1].ID != "weight" {
		t.Fatalf("unexpected custom view order: %+v", got)
	}

	// 3. Remove the first card (fx) from the custom view in Edit mode -> fx stays Home=true on Overview.
	click(t, tt, "编辑布局")
	click(t, tt, "从当前视图移除")
	click(t, tt, "完成")
	if got := a.activeHomeScripts(); len(got) != 1 || got[0].ID != "weight" {
		t.Fatalf("expected only weight remaining after remove-from-view, got %+v", got)
	}
	if fx, ok := a.store.Script("fx"); !ok || !fx.Home {
		t.Fatalf("removing fx from custom view should keep Overview sc.Home=true")
	}

	click(t, tt, "总览")
	requireText(t, tt, "体重 · kg")
	requireText(t, tt, "世界时钟")
	requireTextStart(t, tt, "USD / CNY")

	// 4. Reload app from disk and verify custom view & network view persist cleanly.
	a2, err := newApp(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a2.closeBarInstances)
	if a2.homeViewIndexByID(customID) < 0 || a2.homeViewIndexByID(homeViewNetworkID) < 0 {
		t.Fatalf("expected views to persist on restart, got %+v", a2.set.HomeViews)
	}

	// 5. Delete custom view AND builtin Network view -> Network is not re-seeded on next startup.
	a2.deleteHomeView(customID)
	a2.deleteHomeView(homeViewNetworkID)
	a3, err := newApp(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a3.closeBarInstances)
	if a3.homeViewIndexByID(homeViewNetworkID) >= 0 {
		t.Fatalf("deleted Network view unexpectedly re-seeded on restart")
	}
	if a3.homeViewIndexByID(customID) >= 0 {
		t.Fatalf("deleted custom view still present after restart")
	}
}

func TestSubViewSMLSizeMemoryAndPersistence(t *testing.T) {
	dir := t.TempDir()
	a, err := newApp(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.closeBarInstances)

	tt := ui.NewTester(a.view, 1280, 860)
	settle(tt)

	// Switch to Network view (defaults to Medium preset with sb-ping=4x432).
	click(t, tt, "网络排查")
	if got := a.boxOf("sb-ping"); got.Span != 4 || got.Height != 432 {
		t.Fatalf("initial Network Medium sb-ping box = %+v, want {Span:4 Height:432}", got)
	}

	// Customize sb-ping inside Medium preset.
	a.setCardBoxForActivePreset("sb-ping", store.CardBox{Span: 6, Height: 384})
	a.saveSettings()

	// Switch to Small preset: gets Small box, customize it independently.
	click(t, tt, "小")
	if got := a.boxOf("sb-ping"); got.Span != 4 || got.Height != 224 {
		t.Fatalf("Network Small initial sb-ping box = %+v, want {Span:4 Height:224}", got)
	}
	a.setCardBoxForActivePreset("sb-ping", store.CardBox{Span: 3, Height: 256})
	a.saveSettings()

	// Switch to Large preset: gets Large box.
	click(t, tt, "大")
	if got := a.boxOf("sb-ping"); got.Span != 12 || got.Height != 400 {
		t.Fatalf("Network Large initial sb-ping box = %+v, want {Span:12 Height:400}", got)
	}

	// Return to Medium and Small: each preset remembers its own independent box!
	click(t, tt, "中")
	if got := a.boxOf("sb-ping"); got.Span != 6 || got.Height != 384 {
		t.Fatalf("Network Medium forgot custom box: got %+v, want {Span:6 Height:384}", got)
	}
	click(t, tt, "小")
	if got := a.boxOf("sb-ping"); got.Span != 3 || got.Height != 256 {
		t.Fatalf("Network Small forgot custom box: got %+v, want {Span:3 Height:256}", got)
	}

	// Edit mode drag-resize on sb-ping .bar card updates active preset height/span cleanly.
	click(t, tt, "编辑布局")
	h, ok := tt.Access("resize:sb-ping")
	if !ok {
		t.Fatal("missing corner resize handle on sb-ping .bar card")
	}
	x, y := h.Bounds.X+h.Bounds.W/2, h.Bounds.Y+h.Bounds.H/2
	tt.Press(x, y)
	settle(tt)
	for i := 1; i <= 6; i++ {
		tt.Move(x+float32(i)*24, y+float32(i)*12)
		settle(tt)
	}
	tt.Release(x+144, y+72)
	settle(tt)
	settle(tt)
	if got := a.boxOf("sb-ping"); got.Height <= 256 {
		t.Fatalf("expected sb-ping height > 256 after drag-resize in Small preset, got %+v", got)
	}
	draggedSmall := a.boxOf("sb-ping")

	// Switch to Overview, customize Medium box for weight, switch S/M/L and verify separation.
	click(t, tt, "总览")
	a.setSize("weight", 2) // Large box saved under Overview Medium
	click(t, tt, "小")
	if got := a.boxOf("weight"); got.Span != gridSpans[0] {
		t.Fatalf("Overview Small weight span = %d, want small %d", got.Span, gridSpans[0])
	}
	click(t, tt, "中")
	if got := a.boxOf("weight"); got.Span != gridSpans[2] {
		t.Fatalf("Overview Medium weight span = %d, want remembered %d", got.Span, gridSpans[2])
	}

	// Reload from disk and verify remembered preset boxes survive restart.
	a2, err := newApp(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a2.closeBarInstances)
	if got := a2.boxOf("weight"); got.Span != gridSpans[2] {
		t.Fatalf("reloaded Overview Medium weight span = %d, want %d", got.Span, gridSpans[2])
	}
	a2.selectHomeView(homeViewNetworkID)
	a2.switchSizePreset(0)
	if got := a2.boxOf("sb-ping"); got != draggedSmall {
		t.Fatalf("reloaded Network Small sb-ping = %+v, want %+v", got, draggedSmall)
	}
	a2.switchSizePreset(1)
	if got := a2.boxOf("sb-ping"); got.Span != 6 || got.Height != 384 {
		t.Fatalf("reloaded Network Medium sb-ping = %+v, want {Span:6 Height:384}", got)
	}
}

func TestHomeViewsEnglishParity(t *testing.T) {
	t.Cleanup(func() { setLang(langZH) })
	a := newIPBentoTestApp(t)
	a.set.Lang = langEN
	setLang(langEN)

	tt := ui.NewTester(a.view, 1280, 860)
	settle(tt)
	settle(tt)

	requireText(t, tt, "Overview")
	requireText(t, tt, "Network")
	requireText(t, tt, "Edit layout")
	onlyEnglish(t, tt, "home-views-overview-en")

	click(t, tt, "Network")
	settle(tt)
	requireText(t, tt, "Ping Monitor")
	requireText(t, tt, "39.180.138.19")
	onlyEnglish(t, tt, "home-views-network-en")

	// Wait a beat cleanly & open Customize widgets panel in English.
	_ = time.Millisecond
	click(t, tt, "Customize widgets")
	settle(tt)
	requireText(t, tt, "Edit view")
	requireText(t, tt, "Save view")
	onlyEnglish(t, tt, "home-views-editor-en")
}
