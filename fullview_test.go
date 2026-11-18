package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The full-screen view shows the chart in the style the user picks, the
// statistics and what the script says about itself.
func TestFullViewStylesAndStats(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 960, 900)
	a.router.Push("/full/goldk")
	settle(tt)
	requireText(t, tt, "最新")
	requireText(t, tt, "最高")
	requireText(t, tt, "振幅")
	savePNG(t, tt, "7-full-candle")

	click(t, tt, "收盘线")
	if got := a.styles["goldk"]; got != "close" {
		t.Errorf("style after choosing 收盘线 is %q", got)
	}
	savePNG(t, tt, "7-full-close")

	click(t, tt, "大图")
	if !a.big {
		t.Error("大图 did not switch on")
	}
	savePNG(t, tt, "7-full-big")

	a.router.Push("/full/weight")
	settle(tt)
	click(t, tt, "面积")
	if got := a.styles["weight"]; got != "area" {
		t.Errorf("weight style %q, want area", got)
	}
	requireText(t, tt, "关于")
	savePNG(t, tt, "7-full-area")
}

// Moving the pointer over the chart picks the nearest point: the readout
// names it, and the hint to hover goes away.
func TestFullViewHoverReadout(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 960, 900)
	a.router.Push("/full/weight")
	settle(tt)
	const hint = "悬浮图表查看详情"
	hasHint := func() bool {
		for _, s := range tt.Texts() {
			if strings.Contains(s, hint) {
				return true
			}
		}
		return false
	}
	if !hasHint() {
		t.Fatalf("no readout hint on the page; texts: %s", strings.Join(tt.Texts(), " | "))
	}
	// The style row sits just above the chart.
	row, ok := tt.Find("样式")
	if !ok {
		t.Fatal("no style row")
	}
	tt.Move(row.X+200, row.Y+160)
	settle(tt)
	if hasHint() {
		t.Error("the hover hint is still there after the pointer moved over the chart")
	}
	savePNG(t, tt, "7-full-hover")
}

// On a phone the full-screen view stacks: the chart, then the statistics.
func TestFullViewPhone(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 390, 844)
	settle(tt)
	a.router.Push("/full/goldk")
	settle(tt)
	settle(tt)
	requireText(t, tt, "最新")
	savePNG(t, tt, "7-full-phone")
}
