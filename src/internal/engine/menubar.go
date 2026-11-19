package engine

import (
	"fmt"
	"net/url"
	"slices"
	"unicode/utf8"
)

// MenuActions are the actions a menu entry may name.
var MenuActions = []string{"refresh", "open", "window"}

// MenuBarTitleMax is how many characters of a menu bar item's title are shown;
// more are cut, so that the bar keeps its width.
const MenuBarTitleMax = 12

// MenuBarProblems says what is wrong with what a result gives the menu bar, in
// words for the script's author: a menu on a card that did not ask for the bar,
// a link that is not a web address, an action that does not exist, a title that
// will be cut. Nothing is wrong when it is empty.
func MenuBarProblems(meta Meta, res Result) []string {
	var out []string
	asked := res.MenuBar != nil || len(res.Menu) > 0
	if asked && !meta.Menubar {
		out = append(out, "结果里有 menubar 或 menu，但 meta() 没有 menubar: true，卡片不能放进菜单栏")
	}
	if mb := res.MenuBar; mb != nil {
		if utf8.RuneCountInString(mb.Title) > MenuBarTitleMax {
			out = append(out, fmt.Sprintf("menubar.title 超过 %d 个字，会被截短", MenuBarTitleMax))
		}
		if mb.Click != "" && mb.Click != "menu" && mb.Click != "open" {
			out = append(out, fmt.Sprintf("menubar.click 是 %q，只有 menu 和 open", mb.Click))
		}
	}
	var walk func(entries []MenuEntry, depth int)
	walk = func(entries []MenuEntry, depth int) {
		for _, e := range entries {
			if e.URL != "" && !webURL(e.URL) {
				out = append(out, fmt.Sprintf("菜单项 %q 的链接只能是 http 或 https，别的不会打开", e.Label))
			}
			if e.Action != "" && !slices.Contains(MenuActions, e.Action) {
				out = append(out, fmt.Sprintf("菜单项 %q 的 action 是 %q，只有 %v", e.Label, e.Action, MenuActions))
			}
			if len(e.Items) > 0 {
				if depth >= 1 {
					out = append(out, fmt.Sprintf("菜单项 %q 的子菜单太深，只显示一层子菜单", e.Label))
					continue
				}
				walk(e.Items, depth+1)
			}
		}
	}
	walk(res.Menu, 0)
	return out
}

// webURL says whether s is an http or https address.
func webURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
