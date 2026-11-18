package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/display"
)

// navItem is one place the app can go to.
type navItem struct {
	path, icon, label string
	// also lists the paths that belong to the same place, so that it stays
	// lit on them.
	also []string
}

var navItems = []navItem{
	{path: "/", icon: "layout-grid", label: "首页", also: []string{"/full/"}},
	{path: "/edit", icon: "pencil-line", label: "编辑"},
	{path: "/community", icon: "users", label: "社区"},
	{path: "/settings", icon: "settings", label: "设置", also: []string{"/feeds"}},
}

// active reports whether the page at path belongs to the item.
func (n navItem) active(path string) bool {
	if n.path == "/" {
		if path == "/" {
			return true
		}
	} else if path == n.path || strings.HasPrefix(path, n.path+"/") {
		return true
	}
	for _, p := range n.also {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// view builds the window. A wide window has a sidebar beside the page; a
// narrow one, the phone layout, has a bar of tabs under it.
func (a *app) view(c *ui.Context) {
	brand.Apply(c)
	compact := a.width > 0 && a.width < compactWidth
	a.compact = compact

	root := ui.Column(c).Fill().Background(pal(c).canvas).Children(func() {
		p := pal(c)
		path := a.router.Path()
		pages := func() {
			ui.Column(c).Grow(1).Background(p.canvas).Children(func() {
				a.router.View(c, func(r *ui.Route) {
					switch {
					case r.Match("/"):
						r.Title(tr("首页"))
						a.home(c, compact)
					case r.Match("/edit/{id}"):
						r.Title(tr("编辑"))
						a.openEditor(r.Param("id"))
						a.editor(c, compact)
					case r.Match("/edit"):
						r.Title(tr("编辑"))
						a.editor(c, compact)
					case r.Match("/full/{id}"):
						r.Title(tr("查看"))
						a.fullView(c, r.Param("id"), compact)
					case r.Match("/feeds"):
						r.Title(tr("数据源"))
						a.feedsPage(c, compact)
					case r.Match("/community"):
						r.Title(tr("社区"))
						a.community(c, compact)
					case r.Match("/settings"):
						r.Title(tr("设置"))
						a.settings(c, compact)
					default:
						ui.Text(c, tr("页面不存在"))
					}
				})
			})
		}
		if compact {
			ui.Column(c).Fill().Children(func() {
				pages()
				a.tabBar(c, p, path)
			})
			return
		}
		ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
			a.sidebar(c, p, path)
			pages()
		})
	})
	if a.win != nil || a.width == 0 {
		a.width = root.Bounds().W
		a.height = root.Bounds().H
	}
}

// sidebar is the brand, the places to go and the state of the network. Like
// Kero's it sits on the desk's own color; the chosen row is a white tile.
func (a *app) sidebar(c *ui.Context, p palette, path string) {
	ui.Column(c).Width(sidebarWidth).Background(p.sidebar).Padding(18, 12).Gap(2).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(10).Padding(2, 8, 18, 8).Children(func() {
			iconBadge(c, "chart-line", p.accent, 30)
			ui.Column(c).Children(func() {
				ui.Text(c, appName()).FontSize(fsBody + 1).FontWeight(650)
				ui.Text(c, tr("脚本小组件")).FontSize(fsTiny).TextColor(p.faint)
			})
		})
		items := make([]display.ListItem, len(navItems))
		active := ""
		for i, n := range navItems {
			items[i] = display.ListItem{Key: n.path, Label: tr(n.label), Icon: icon(n.icon)}
			if n.active(path) {
				active = n.path
			}
		}
		if key := display.List(c, display.ListProps{Items: items, Active: active}); key != "" {
			a.router.Push(key)
		}
		ui.Spacer(c)
		status, color := tr("在线"), p.down
		if a.set.Offline {
			status, color = tr("离线模式"), p.warn
		}
		ui.Row(c).AlignItems(ui.Center).Gap(8).Padding(8, 10).Children(func() {
			ui.Row(c).Size(7, 7).Radius(3.5).Background(color)
			ui.Text(c, status).FontSize(fsSmall).TextColor(p.muted)
		})
	})
}

// tabBar is the phone layout's navigation: an icon over a label for each
// place.
func (a *app) tabBar(c *ui.Context, p palette, path string) {
	ui.Row(c).Background(p.card).Padding(8, 6, 10, 6).BorderWidth(1, 0, 0, 0).BorderColor(p.border).Children(func() {
		for _, n := range navItems {
			n := n
			active := n.active(path)
			fg := p.muted
			if active {
				fg = p.accent
			}
			el := ui.Column(c).Grow(1).AlignItems(ui.Center).Gap(3).Padding(4, 0).Radius(radControl).Children(func() {
				ui.Icon(c, icon(n.icon)).FontSize(21).TextColor(fg)
				t := ui.Text(c, tr(n.label)).FontSize(fsTiny).TextColor(fg)
				if active {
					t.Bold()
				}
			})
			if el.Clicked() {
				a.router.Push(n.path)
			}
		}
	})
}
