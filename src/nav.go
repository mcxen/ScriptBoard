package main

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/caps"
	"scriptboard/src/internal/store"
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
	{path: "/edit", icon: "pencil-line", label: "组件管理", also: []string{"/edit/"}},
	{path: "/bar", icon: "sparkles", label: "菜单管理", also: []string{"/bar/"}},
	{path: "/community", icon: "users", label: "社区"},
	{path: "/settings", icon: "settings", label: "设置", also: []string{"/feeds"}},
}

// mainNavItems are the pages that live inside the main window; settings opens
// in its own window on desktop and sits apart in the sidebar footer.
var (
	mainNavItems    = navItems[:4]
	settingsNavItem = navItems[4]
)

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
	// The page's own size in DIPs, read first so that the very first frame is
	// laid out for it. (The root's bounds are not the window's on Android, where
	// they came out as a landscape rectangle.) Before any size is known, a
	// phone is assumed to be a phone: it must not flash the desktop layout.
	if w, h := c.Size(); w > 0 {
		a.width, a.height = float32(w), float32(h)
	}
	compact := a.width < compactWidth
	if a.width == 0 {
		compact = caps.Platform() == caps.Mobile
	}
	a.compact = compact
	// Android has no updater: the app looks for a newer APK once, when it starts.
	if isAndroid() && a.win != nil && !a.apkChecked && !a.apkBusy {
		a.checkAPK()
	}

	setPageWidth(a.set.ContentWidth)
	root := skinBackground(c, ui.Column(c).Fill())
	// Escape closes shortcut overlay first, or goes back one page.
	if root.OverlayShortcut(0, ui.KeyEscape) {
		if a.barOverlayID != "" {
			a.barOverlayID = ""
		} else {
			a.router.Back()
		}
	}
	shortcutBars := a.dockedBarComponents(func(pl store.BarPlacement) bool { return pl.Shortcut })
	if len(shortcutBars) > 0 && root.OverlayShortcut(ui.Alt, ui.KeyW) {
		if a.barOverlayID != "" {
			a.barOverlayID = ""
		} else {
			a.barOverlayID = shortcutBars[0].ID
		}
	}
	// Cmd+, (macOS) or Ctrl+, (Windows/Linux) opens the settings window.
	if !isMobile() && root.OverlayShortcut(ui.Cmd, ui.KeyComma) {
		a.goTo(settingsNavItem.path)
	}
	// Closing the window only minimizes it, and Windows and Linux have no menu
	// with a Quit: Ctrl+Q quits there, as Cmd-Q does on a Mac.
	if ownControls && root.OverlayShortcut(ui.Ctrl, ui.KeyQ) {
		quitApp()
	}
	root.Children(func() {
		p := pal(c)
		a.syncSystemBars(p.dark)
		path := a.router.Path()
		switch {
		// A phone has no menu bar and no window buttons, whatever the skin.
		case isMac() && !compact:
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
					case r.Match("/bar/{id}"):
						r.Title(tr("菜单管理"))
						a.selectBarComponent(r.Param("id"))
						a.barView(c, compact)
					case r.Match("/bar"):
						r.Title(tr("菜单管理"))
						a.barView(c, compact)
					case r.Match("/edit/{id}"):
						r.Title(tr("组件管理"))
						a.openEditor(r.Param("id"))
						a.editor(c, compact)
					case r.Match("/edit"):
						r.Title(tr("组件管理"))
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
			ui.Column(c).Grow(1).Children(func() {
				pages()
				a.tabBar(c, p, path)
			})
			a.barShortcutOverlay(c, p)
			return
		}
		leftDocks := a.dockedBarComponents(func(pl store.BarPlacement) bool { return pl.LeftSidebar })
		rightDocks := a.dockedBarComponents(func(pl store.BarPlacement) bool { return pl.RightSidebar })
		bottomDocks := a.dockedBarComponents(func(pl store.BarPlacement) bool { return pl.BottomBar })

		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			a.sidebar(c, p, path)
			if len(leftDocks) > 0 {
				a.renderBarDockSide(c, p, leftDocks, false)
			}
			ui.Column(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
				ui.Column(c).Grow(1).MinHeight(0).FillWidth().Children(pages)
				if len(bottomDocks) > 0 {
					a.renderBarDockBottom(c, p, bottomDocks)
				}
			})
			if len(rightDocks) > 0 {
				a.renderBarDockSide(c, p, rightDocks, true)
			}
		})
		a.barShortcutOverlay(c, p)
	})
}

func (a *app) renderBarDockSide(c *ui.Context, p palette, list []store.Script, right bool) {
	col := ui.Column(c).
		Width(324).
		FillHeight().
		Background(p.sidebar).
		BorderColor(p.hairline).
		Padding(14, 12)
	if right {
		col = col.BorderWidth(0, 0, 0, 1)
	} else {
		col = col.BorderWidth(0, 1, 0, 0)
	}
	col.Children(func() {
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).FillWidth().Gap(14).Children(func() {
				for _, sc := range list {
					sc := sc
					a.renderDockedBarPanel(c, sc, p, func() {
						pl := a.barPlacement(sc.ID)
						if right {
							pl.RightSidebar = false
						} else {
							pl.LeftSidebar = false
						}
						a.setBarPlacement(sc.ID, pl)
					})
				}
			})
		})
	})
}

func (a *app) renderBarDockBottom(c *ui.Context, p palette, list []store.Script) {
	ui.Column(c).
		FillWidth().
		Height(250).
		Background(p.sidebar).
		BorderWidth(1, 0, 0, 0).
		BorderColor(p.hairline).
		Padding(12, 16).
		Children(func() {
			ui.Scroll(c).Fill().Children(func() {
				ui.Row(c).Wrap().Gap(16).Children(func() {
					for _, sc := range list {
						sc := sc
						a.renderDockedBarPanel(c, sc, p, func() {
							pl := a.barPlacement(sc.ID)
							pl.BottomBar = false
							a.setBarPlacement(sc.ID, pl)
						})
					}
				})
			})
		})
}

// sidebar is the brand, the places to go and the state of the network. Like
// Kero's it sits on the desk's own color; the chosen row is a white tile. It
// folds into a rail of icons, which keeps the pages wide.
func (a *app) sidebar(c *ui.Context, p palette, path string) {
	if a.set.SidebarCollapsed {
		a.sidebarRail(c, p, path)
		return
	}
	borderCol := p.hairline
	if isMac() {
		borderCol = macBlack
	}
	ui.Column(c).Width(sidebarWidth).Background(p.sidebar).BorderWidth(0, 1, 0, 0).BorderColor(borderCol).Padding(12, 10, 14, 10).Gap(2).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(9).Padding(2, 8, 14, 8).Children(func() {
			iconBadge(c, "chart-line", p.accent, 26)
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, appName()).FontSize(fsBodyLarge).FontWeight(650).LetterSpacing(-0.2)
				ui.Text(c, tr("脚本小组件")).FontSize(fsTiny).TextColor(p.faint)
			})
			if sidebarToggle(c, "panel-left", tr("收起侧栏")).Clicked() {
				a.toggleSidebar()
			}
		})
		if isMac() {
			for _, n := range mainNavItems {
				if macSidebarItem(c, tr(n.label), n.icon, n.active(path), sidebarWidth-20) {
					a.goTo(n.path)
				}
			}
			ui.Spacer(c)
			ui.Column(c).BorderWidth(1, 0, 0, 0).BorderColor(p.border).Padding(8, 0, 0, 0).Gap(2).Children(func() {
				if macSidebarItem(c, tr(settingsNavItem.label), settingsNavItem.icon, settingsNavItem.active(path) || a.settingsWin != nil, sidebarWidth-20) {
					a.goTo(settingsNavItem.path)
				}
				a.sidebarStatus(c, p)
			})
			return
		}
		for _, n := range mainNavItems {
			n := n
			if navRow(c, "nav:"+n.path, n.icon, tr(n.label), n.active(path), sidebarWidth-20, nil) {
				a.goTo(n.path)
			}
		}
		ui.Spacer(c)
		ui.Column(c).BorderWidth(1, 0, 0, 0).BorderColor(p.hairline).Padding(8, 0, 0, 0).Gap(2).Children(func() {
			a.sidebarSettingsButton(c, p, settingsNavItem.active(path))
			a.sidebarStatus(c, p)
		})
	})
}

// sidebarSettingsButton opens the standalone settings window on desktop (or the
// /settings page in headless tests / mobile fallback). It lives in the sidebar
// footer so it is visually distinct from main-window navigation tabs.
func (a *app) sidebarSettingsButton(c *ui.Context, p palette, active bool) {
	label := tr(settingsNavItem.label)
	shortcut := "⌘,"
	if ownControls {
		shortcut = "Ctrl+,"
	}
	lit := active || a.settingsWin != nil
	if navRow(c, "nav:settings", settingsNavItem.icon, label, lit, sidebarWidth-20, func() {
		if !isMobile() {
			ui.Text(c, shortcut).FontSize(fsTiny).TextColor(p.faint)
		}
	}) {
		a.goTo(settingsNavItem.path)
	}
}

// sidebarRail is the sidebar folded: the toggle, one icon for each place, and
// the network state as a dot, each with its name as a tooltip.
func (a *app) sidebarRail(c *ui.Context, p palette, path string) {
	borderCol := p.hairline
	if isMac() {
		borderCol = macBlack
	}
	ui.Column(c).Width(railWidth).Background(p.sidebar).BorderWidth(0, 1, 0, 0).BorderColor(borderCol).Padding(10, 6, 14, 6).Gap(6).AlignItems(ui.Center).Children(func() {
		if sidebarToggle(c, "panel-left", tr("展开侧栏")).Clicked() {
			a.toggleSidebar()
		}
		for _, n := range mainNavItems {
			if macOrRailItem(c, n, n.active(path), p) {
				a.goTo(n.path)
			}
		}
		ui.Spacer(c)
		ui.Box(c).Width(24).Height(1).Background(p.hairline)
		if macOrRailItem(c, settingsNavItem, settingsNavItem.active(path) || a.settingsWin != nil, p) {
			a.goTo(settingsNavItem.path)
		}
		color := p.down
		if a.set.Offline {
			color = p.warn
		}
		ui.Row(c).Size(6, 6).Radius(3).Background(color)
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
	p := pal(c)
	b := ui.ButtonBase(c).Size(26, 26).Radius(7).AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer).Label(title).Tooltip(title)
	if b.Hovered() {
		b = b.Background(p.hover)
	}
	return b.Children(func() {
		ui.Icon(c, icon(iconName)).FontSize(15).TextColor(p.muted)
	})
}

// macOrRailItem is one place in the rail: its icon, chosen as a filled square.
func macOrRailItem(c *ui.Context, n navItem, chosen bool, p palette) bool {
	label := tr(n.label)
	el := ui.ButtonBase(c).Size(34, 34).Radius(8).AlignItems(ui.Center).Justify(ui.Center).Label(label).Tooltip(label)
	fg := p.muted
	if isMac() {
		fg = p.text
	} else if chosen {
		fg = p.accent
	}
	if chosen {
		el = el.Background(p.rowSel)
		if !isMac() && !p.dark {
			el = el.BorderWidth(1).BorderColor(p.hairline)
		}
	} else if el.Hovered() {
		el = el.Background(p.hover)
	}
	return el.Children(func() {
		ui.Icon(c, icon(n.icon)).FontSize(17).TextColor(fg)
	}).Clicked()
}

// sidebarStatus is the line at the foot of the sidebar: online, or offline.
func (a *app) sidebarStatus(c *ui.Context, p palette) {
	status, color := tr("在线"), p.down
	if a.set.Offline {
		status, color = tr("离线模式"), p.warn
	}
	ui.Row(c).AlignItems(ui.Center).Gap(7).Padding(6, 10).Children(func() {
		ui.Row(c).Size(6, 6).Radius(3).Background(color)
		ui.Text(c, status).FontSize(fsTiny).TextColor(p.muted)
	})
}

// tabBar is the phone layout's navigation: an icon over a label for each
// place. It hides while the on-screen keyboard is open and fills the bottom
// safe-area inset when at rest.
func (a *app) tabBar(c *ui.Context, p palette, path string) {
	_, _, bottom, _ := c.SafeArea()
	if bottom > 120 {
		return
	}
	ui.Row(c).Background(p.card).Padding(7, 6, 9+bottom, 6).Margin(0, 0, -bottom, 0).BorderWidth(1, 0, 0, 0).BorderColor(p.hairline).Children(func() {
		for _, n := range navItems {
			n := n
			active := n.active(path)
			fg := p.muted
			if active {
				fg = p.accent
			}
			el := ui.Column(c).Grow(1).AlignItems(ui.Center).Gap(3).Padding(4, 0).Radius(radControl)
			if active {
				el = el.Background(p.accentSoft)
			}
			el.Children(func() {
				ui.Icon(c, icon(n.icon)).FontSize(20).TextColor(fg)
				t := ui.Text(c, tr(n.label)).FontSize(fsTiny).TextColor(fg)
				if active {
					t.Bold()
				}
			})
			if el.Clicked() {
				if !active {
					a.hapticSelection()
				}
				a.goTo(n.path)
			}
		}
	})
}
