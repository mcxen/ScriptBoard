package engine

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

// MenuActions are the built-in static actions a menu entry may name. In
// addition, parameterized actions "set_var:<key>=<value>" and
// "set_dim:<key>=<value>" switch a script's variables or dimensions directly
// from the menu bar.
var MenuActions = []string{
	"refresh", "refresh_all", "open", "window", "settings", "toggle_offline", "copy_headline",
}

// MenuBarIcons are the supported built-in tray glyph shapes for menubar.icon.
var MenuBarIcons = []string{"bars", "dot", "ring", "up", "down", "pulse"}

// MenuBarMinRefreshSec is the shortest allowed custom menu bar refresh interval.
const MenuBarMinRefreshSec = 15

// MenuBarTitleMax is how many characters of a menu bar item's title are shown;
// more are cut, so that the bar keeps its width.
const MenuBarTitleMax = 12

var reMenuColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// ParseMenuAction parses a MenuEntry.Action string into its action kind and
// optional key/value parameters (for set_var:k=v and set_dim:k=v).
func ParseMenuAction(action string) (kind, key, val string, ok bool) {
	if slices.Contains(MenuActions, action) {
		return action, "", "", true
	}
	for _, prefix := range []string{"set_var:", "set_dim:"} {
		if strings.HasPrefix(action, prefix) {
			rest := strings.TrimPrefix(action, prefix)
			k, v, hasEq := strings.Cut(rest, "=")
			k = strings.TrimSpace(k)
			if hasEq && k != "" {
				return strings.TrimSuffix(prefix, ":"), k, v, true
			}
			return "", "", "", false
		}
	}
	return "", "", "", false
}

// MenuBarProblems says what is wrong with what a result gives the menu bar, in
// words for the script's author: a menu on a card that did not ask for the bar,
// a link that is not a web address, an action that does not exist, a title that
// will be cut, or invalid icon/color/refresh settings. Nothing is wrong when empty.
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
		if mb.Icon != "" && !slices.Contains(MenuBarIcons, mb.Icon) {
			out = append(out, fmt.Sprintf("menubar.icon 是 %q，可选值：%v", mb.Icon, MenuBarIcons))
		}
		if mb.Color != "" && !reMenuColor.MatchString(mb.Color) {
			out = append(out, fmt.Sprintf("menubar.color 是 %q，请用 #RRGGBB 格式", mb.Color))
		}
		if mb.RefreshSec > 0 && mb.RefreshSec < MenuBarMinRefreshSec {
			out = append(out, fmt.Sprintf("menubar.refreshSec 不能小于 %d 秒（当前 %d）", MenuBarMinRefreshSec, mb.RefreshSec))
		}
	}
	hasVar := func(key string) bool {
		for _, v := range meta.Vars {
			if v.Key == key {
				return true
			}
		}
		return false
	}
	hasDim := func(key string) bool {
		for _, d := range meta.Dims {
			if d.Key == key {
				return true
			}
		}
		return false
	}
	var walk func(entries []MenuEntry, depth int)
	walk = func(entries []MenuEntry, depth int) {
		for _, e := range entries {
			if e.URL != "" && !webURL(e.URL) {
				out = append(out, fmt.Sprintf("菜单项 %q 的链接只能是 http 或 https，别的不会打开", e.Label))
			}
			if e.Action != "" {
				kind, key, _, ok := ParseMenuAction(e.Action)
				if !ok {
					out = append(out, fmt.Sprintf("菜单项 %q 的 action 是 %q，只有 %v（或 set_var:k=v / set_dim:k=v）", e.Label, e.Action, MenuActions))
				} else if kind == "set_var" && !hasVar(key) {
					out = append(out, fmt.Sprintf("菜单项 %q 的 set_var 引用的变量 %q 未在 meta().vars 中声明", e.Label, key))
				} else if kind == "set_dim" && !hasDim(key) {
					out = append(out, fmt.Sprintf("菜单项 %q 的 set_dim 引用的维度 %q 未在 meta().dims 中声明", e.Label, key))
				}
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
