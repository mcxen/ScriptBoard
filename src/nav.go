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
	applySkin(c)
	compact := a.width > 0 && a.width < compactWidth
	a.compact = compact

	root := skinBackground(c, ui.Column(c).Fill())
	// Escape goes back one page, as in notes; a dialog that is open takes it
	// first, since it is built after the page.
	if root.OverlayShortcut(0, ui.KeyEscape) {
		a.router.Back()
	}
	root.Children(func() {
		p := pal(c)
		path := a.router.Path()
		switch {
		case isMac():
			macMenuBar(c, a)
		case !compact:
			titleBar(c, a, func() {})
		}
		pages := func() {
			skinSurface(c, ui.Column(c).Grow(1)).Children(func() {
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
	// The page's own size in DIPs. The root's bounds are not the window's on
	// Android, where they came out as a landscape rectangle.
	if w, h := c.Size(); w > 0 {
		a.width, a.height = float32(w), float32(h)
	}
}

// sidebar is the brand, the places to go and the state of the network. Like
// Kero's it sits on the desk's own color; the chosen row is a white tile. It
// folds into a rail of icons, which keeps the pages wide.
func (a *app) sidebar(c *ui.Context, p palette, path string) {
	if a.set.SidebarCollapsed {
		a.sidebarRail(c, p, path)
		return
	}
	ui.Column(c).Width(sidebarWidth).Background(p.sidebar).Padding(18, 12).Gap(2).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(10).Padding(2, 8, 18, 8).Children(func() {
			iconBadge(c, "chart-line", p.accent, 30)
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, appName()).FontSize(fsBodyLarge).FontWeight(650)
				ui.Text(c, tr("脚本小组件")).FontSize(fsTiny).TextColor(p.faint)
			})
			if sidebarToggle(c, "panel-left", tr("收起侧栏")).Clicked() {
				a.toggleSidebar()
			}
		})
		if isMac() {
			for _, n := range navItems {
				if macSidebarItem(c, tr(n.label), n.icon, n.active(path), sidebarWidth-24) {
					a.router.Push(n.path)
				}
			}
			a.sidebarStatus(c, p)
			return
		}
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
		a.sidebarStatus(c, p)
	})
}

// sidebarRail is the sidebar folded: the toggle, one icon for each place, and
// the network state as a dot, each with its name as a tooltip.
func (a *app) sidebarRail(c *ui.Context, p palette, path string) {
	ui.Column(c).Width(railWidth).Background(p.sidebar).Padding(10, 6).Gap(6).AlignItems(ui.Center).Children(func() {
		if sidebarToggle(c, "panel-left", tr("展开侧栏")).Clicked() {
			a.toggleSidebar()
		}
		for _, n := range navItems {
			if macOrRailItem(c, n, n.active(path), p) {
				a.router.Push(n.path)
			}
		}
		ui.Spacer(c)
		color := p.down
		if a.set.Offline {
			color = p.warn
		}
		ui.Row(c).Size(7, 7).Radius(3.5).Background(color).Padding(0, 0, 10, 0)
	})
}

// toggleSidebar folds the sidebar or opens it again, and keeps the choice.
func (a *app) toggleSidebar() {
	a.set.SidebarCollapsed = !a.set.SidebarCollapsed
	a.saveSettings()
}

// sidebarToggle is the button that folds the sidebar: a square with the
// panel's icon.
func sidebarToggle(c *ui.Context, iconName, title string) ui.Element {
	return ui.ButtonBase(c).Size(28, 28).Radius(6).AlignItems(ui.Center).Justify(ui.Center).Label(title).Tooltip(title).Children(func() {
		ui.Icon(c, icon(iconName)).FontSize(16).TextColor(pal(c).muted)
	})
}

// macOrRailItem is one place in the rail: its icon, chosen as a filled square.
func macOrRailItem(c *ui.Context, n navItem, chosen bool, p palette) bool {
	bg := ui.Color{}
	if chosen {
		bg = p.rowSel
	}
	el := ui.ButtonBase(c).Size(34, 34).Radius(8).AlignItems(ui.Center).Justify(ui.Center)
	if chosen {
		el = el.Background(bg)
	}
	return el.Children(func() {
		ui.Icon(c, icon(n.icon)).FontSize(18).TextColor(p.text)
	}).Clicked()
}

// sidebarStatus is the line at the foot of the sidebar: online, or offline.
func (a *app) sidebarStatus(c *ui.Context, p palette) {
	status, color := tr("在线"), p.down
	if a.set.Offline {
		status, color = tr("离线模式"), p.warn
	}
	ui.Row(c).AlignItems(ui.Center).Gap(8).Padding(8, 10).Children(func() {
		ui.Row(c).Size(7, 7).Radius(3.5).Background(color)
		ui.Text(c, status).FontSize(fsSmall).TextColor(p.muted)
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
