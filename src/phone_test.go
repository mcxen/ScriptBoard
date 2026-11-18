package main

import (
	"strings"
	"testing"

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
	requireTextStart(t, tt, "USD / CNY")
	savePNG(t, tt, "4-phone-home")

	click(t, tt, "编辑")
	requireText(t, tt, "体重 · kg")
	savePNG(t, tt, "4-phone-editor")

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
