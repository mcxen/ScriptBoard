package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// The phone layout: a 390-point wide window shows a bar of tabs instead of
// the sidebar, and the pages stack in one column.
func TestPhoneLayout(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 390, 844)
	settle(tt) // the first frame learns the width
	settle(tt)
	if a.width >= compactWidth {
		t.Fatalf("width %v, want a phone width", a.width)
	}
	requireTextStart(t, tt, "USD /")
	savePNG(t, tt, "4-phone-home")

	click(t, tt, "组件管理")
	requireText(t, tt, "脚本与数据表") // the list comes first
	savePNG(t, tt, "4-phone-editor")
	click(t, tt, "体重记录")
	click(t, tt, "预览")
	requireText(t, tt, "体重 · kg")

	click(t, tt, "社区")
	requireText(t, tt, "登录后可以上传脚本（下载不需要登录）")
	savePNG(t, tt, "4-phone-community")
}

// Desktop: the community page is one sidebar item and lists nothing offline.
func TestCommunityPageOffline(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 960, 640)
	click(t, tt, "社区")
	requireText(t, tt, "还没有设置社区地址：到「设置」里填写 Cloudflare Worker 的地址。")
	savePNG(t, tt, "5-community")
}

// Scripts can be imported as JSON from export, or as a bare module.
func TestImportExport(t *testing.T) {
	a := newTestApp(t)
	before := len(a.store.Scripts())

	src := "export function meta() { return { name: \"导入测试\", icon: \"coins\", sources: {} }; }\n" +
		"export function render(ctx) { return { title: \"t\", big: \"1\", sub: \"\", lines: [], chart: null }; }"
	if err := a.importText(src); err != nil {
		t.Fatalf("bare module: %v", err)
	}
	if got := len(a.store.Scripts()); got != before+1 {
		t.Fatalf("%d scripts after import, want %d", got, before+1)
	}

	doc := exportScript(a.draft)
	if !strings.Contains(doc, `"format": "scriptboard/1"`) {
		t.Fatalf("export lacks the format tag: %s", doc)
	}
	if err := a.importText(doc); err != nil {
		t.Fatalf("export round trip: %v", err)
	}

	for _, bad := range []string{"", "{not json", "export const x = 1;"} {
		if err := a.importText(bad); err == nil {
			t.Errorf("import of %q should fail", bad)
		}
	}
}

func TestPhoneHomeCardsDoNotOverflowOrCollide(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 390, 844)
	settle(tt)
	settle(tt)

	// 1. Weight card's quick-entry record button must stay strictly inside left column.
	rec, ok := tt.Access("记录")
	if !ok {
		t.Fatal("missing quick entry button 记录 on phone home")
	}
	if right := rec.Bounds.X + rec.Bounds.W; right > 195 {
		t.Errorf("weight record button overflows left column: right=%.1f > 195", right)
	}

	// 2. Countdown unit boxes (天, 时, 分) stay within right column and viewport (<= 376 / 390).
	for _, lbl := range []string{"天", "时", "分"} {
		info, ok := tt.Access(lbl)
		if !ok {
			t.Fatalf("missing countdown unit %q on phone home", lbl)
		}
		if right := info.Bounds.X + info.Bounds.W; right > 376 {
			t.Errorf("countdown unit %q overflows right column: right=%.1f > 376", lbl, right)
		}
	}

	// 3. World clock city label stays left of its formatted time readout (no text overlap).
	city, ok1 := tt.Access("北京 / 上海")
	if !ok1 {
		t.Fatal("missing world clock city 北京 / 上海 on phone home")
	}
	hhmm := a.now().UTC().Add(8 * time.Hour).Format("15:04")
	tm, ok2 := tt.Access(hhmm)
	if !ok2 {
		t.Fatalf("missing world clock time %q on phone home", hhmm)
	}
	if city.Bounds.W <= 0 || city.Bounds.X+city.Bounds.W > tm.Bounds.X+1 {
		t.Errorf("world clock city %+v overlaps time %+v", city.Bounds, tm.Bounds)
	}
	if right := tm.Bounds.X + tm.Bounds.W; right > 195 {
		t.Errorf("world clock time %q overflows left column: right=%.1f > 195", hhmm, right)
	}

	// 4. Calendar compact weekday headers render on a single horizontal line and stay inside right column.
	for _, wd := range []string{"日", "一", "二", "三", "四", "五", "六"} {
		info, ok := tt.Access(wd)
		if !ok {
			t.Fatalf("missing compact weekday header %q on phone home", wd)
		}
		if info.Bounds.H > 20 {
			t.Errorf("weekday header %q wrapped onto multiple lines: height=%.1f", wd, info.Bounds.H)
		}
		if right := info.Bounds.X + info.Bounds.W; right > 376 {
			t.Errorf("weekday header %q overflows right column: right=%.1f > 376", wd, right)
		}
	}
}
