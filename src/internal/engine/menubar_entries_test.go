package engine

import (
	"context"
	"strings"
	"testing"
	"time"
)

// A card's menu bar item and its menu entries, with all their fields, arrive in
// the result.
func TestMenuEntriesAndItemOptionsFromScript(t *testing.T) {
	src := `export function meta() { return { name: "x", menubar: true, sources: {} }; }
export function render(ctx) { return { title: "x", big: "1", sub: "", lines: [], chart: null,
  menubar: { title: "1", mono: true, click: "open" },
  menu: [
    { label: "更多", items: [{ label: "复制", copy: "abc" }, { label: "刷新", action: "refresh" }] },
    { label: "今日", sub: "3 次", badge: "3", checked: true },
    { label: "停用", disabled: true },
    { separator: true },
  ] }; }`
	s, err := CompileLang("entries.js", src, "zh")
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Run(context.Background(), Input{Now: time.Now(), Lang: "zh"})
	if err != nil {
		t.Fatal(err)
	}
	if res.MenuBar == nil || !res.MenuBar.Mono || res.MenuBar.Click != "open" {
		t.Fatalf("menu bar %+v", res.MenuBar)
	}
	m := res.Menu
	if len(m) != 4 || len(m[0].Items) != 2 || m[0].Items[0].Copy != "abc" || m[0].Items[1].Action != "refresh" {
		t.Fatalf("menu %+v", m)
	}
	if m[1].Sub != "3 次" || m[1].Badge != "3" || !m[1].Checked || !m[2].Disabled || !m[3].Separator {
		t.Errorf("entries %+v", m)
	}
	if p := MenuBarProblems(s.Meta, res); len(p) != 0 {
		t.Errorf("problems with a good menu: %v", p)
	}
}

// What the app cannot use is named, for the script's author.
func TestMenuBarProblems(t *testing.T) {
	asked := Meta{Menubar: true}
	cases := []struct {
		name string
		meta Meta
		res  Result
		want string // a word in the problem; "" for none
	}{
		{"a clean menu", asked, Result{Menu: []MenuEntry{{Label: "打开", URL: "https://example.com"}}}, ""},
		{"a menu on a card that did not ask", Meta{}, Result{Menu: []MenuEntry{{Label: "x"}}}, "meta()"},
		{"a link that is not a web address", asked, Result{Menu: []MenuEntry{{Label: "坏", URL: "javascript:alert(1)"}}}, "http"},
		{"an action that does not exist", asked, Result{Menu: []MenuEntry{{Label: "x", Action: "format-disk"}}}, "action"},
		{"a submenu in a submenu", asked, Result{Menu: []MenuEntry{{Label: "a", Items: []MenuEntry{{Label: "b", Items: []MenuEntry{{Label: "c"}}}}}}}, "子菜单"},
		{"a title that will be cut", asked, Result{MenuBar: &MenuBar{Title: "一二三四五六七八九十十一十二十三"}}, "截短"},
		{"a click that does not exist", asked, Result{MenuBar: &MenuBar{Click: "double"}}, "click"},
	}
	for _, c := range cases {
		got := strings.Join(MenuBarProblems(c.meta, c.res), "；")
		if c.want == "" && got != "" {
			t.Errorf("%s: %q, want no problem", c.name, got)
		}
		if c.want != "" && !strings.Contains(got, c.want) {
			t.Errorf("%s: %q, want it to mention %q", c.name, got, c.want)
		}
	}
}
