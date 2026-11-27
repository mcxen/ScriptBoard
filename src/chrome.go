package main

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// The window's title row, as notes and Kero draw it. On macOS the window keeps
// its traffic lights, inset into the row; on Windows and Linux the system
// buttons are hidden and the page draws minimize, maximize and close at the top
// right of the row. The row drags the window everywhere. Every skin uses the
// same height, so the lights stay where the window put them when a skin changes.

const (
	// titleRow is the height of the fused title/tool row at the top of the window.
	titleRow = 44
	// trafficInset is the room the macOS traffic lights take on the left.
	trafficInset = 78
	// captionWidth is the width of one Windows caption button.
	captionWidth = 46
)

// ownControls is true where the page draws the window's buttons itself.
var ownControls = runtime.GOOS != "darwin"

// framedOptions adapts the main window's options to the platform: the title
// bar is hidden, and on macOS the traffic lights sit in the title row.
func framedOptions(o mygo.WindowOptions) mygo.WindowOptions {
	if ownControls {
		o.TitleBarStyle = mygo.TitleBarHidden
		o.HideWindowButtons = true
		return o
	}
	o.TitleBarStyle = mygo.TitleBarHiddenInset
	o.TitleBarHeight = titleRow
	// The lights are about 16 high and MyGo raises them by 1: their center
	// lands on the row's middle.
	o.TrafficLightPosition = &mygo.Point{X: 14, Y: titleRow/2 - 7}
	return o
}

// titleBar draws the title row across the top of the window and fills it with
// body. The row keeps clear of the macOS lights and, on Windows and Linux, ends
// in the window's own buttons.
func titleBar(c *ui.Context, a *app, body func()) { titleBarIn(c, a, a.win, body) }

// titleBarIn is titleBar for the window w: its buttons act on that window, so
// the settings window's close does not close the main one.
func titleBarIn(c *ui.Context, a *app, w *mygo.Window, body func()) {
	bar := ui.Row(c).Height(titleRow).AlignItems(ui.Center).DragWindow()
	bar = skinTitleBar(c, bar)
	bar.Children(func() {
		if !ownControls {
			ui.Box(c).Width(trafficInset).Shrink(0)
		}
		body()
		ui.Spacer(c)
		if ownControls {
			a.windowButtons(c, w)
		}
	})
}

// desktopTopBar renders the fused Paper & Ink top bar across the main window:
// on desktop it merges the app wordmark + sidebar toggle beside the macOS
// traffic lights (when expanded) with active Home view tabs, S/M/L preset
// switcher, scale pill, refresh, and edit-layout controls on the right.
func desktopTopBar(c *ui.Context, a *app, p palette, path string) {
	if !a.set.SidebarCollapsed {
		ui.Row(c).Height(titleRow).FillWidth().AlignItems(ui.Center).Children(func() {
			sideBarTop := ui.Row(c).
				Width(sidebarWidth).
				Height(titleRow).
				Background(p.sidebar).
				BorderWidth(0, 1, 0, 0).
				BorderColor(p.hairline).
				AlignItems(ui.Center).
				Gap(8).
				DragWindow()
			if !ownControls {
				sideBarTop = sideBarTop.Padding(0, 10, 0, 0)
			} else {
				sideBarTop = sideBarTop.Padding(0, 10, 0, 14)
			}
			sideBarTop.Children(func() {
				if !ownControls {
					ui.Box(c).Width(trafficInset).Shrink(0)
				}
				ui.Box(c).Size(8, 8).Radius(2.5).Background(p.text).Shrink(0)
				ui.Text(c, appName()).FontSize(13.5).FontWeight(650).LetterSpacing(-0.3).TextColor(p.text).NoWrap()
				ui.Spacer(c)
				if sidebarToggle(c, "panel-left", tr("收起侧栏")).Clicked() {
					a.toggleSidebar()
				}
			})

			mainTop := ui.Row(c).
				Grow(1).
				MinWidth(0).
				Height(titleRow).
				Background(p.canvas).
				BorderWidth(0, 0, 1, 0).
				BorderColor(p.hairline).
				Padding(0, 14, 0, 16).
				Gap(10).
				AlignItems(ui.Center).
				DragWindow()
			mainTop.Children(func() {
				a.topBarMainContent(c, p, path)
				if ownControls {
					a.windowButtons(c, a.win)
				}
			})
		})
		return
	}

	bar := ui.Row(c).
		FillWidth().
		Height(titleRow).
		Background(p.canvas).
		BorderWidth(0, 0, 1, 0).
		BorderColor(p.hairline).
		AlignItems(ui.Center).
		Gap(10).
		DragWindow()
	if !ownControls {
		bar = bar.Padding(0, 14, 0, 0)
	} else {
		bar = bar.Padding(0, 14, 0, 10)
	}
	bar.Children(func() {
		if !ownControls {
			ui.Box(c).Width(trafficInset).Shrink(0)
		}
		if sidebarToggle(c, "panel-left", tr("展开侧栏")).Clicked() {
			a.toggleSidebar()
		}
		ui.Box(c).Width(1).Height(16).Background(p.hairline).Shrink(0)
		a.topBarMainContent(c, p, path)
		if ownControls {
			a.windowButtons(c, a.win)
		}
	})
}

func (a *app) topBarMainContent(c *ui.Context, p palette, path string) {
	if path == "/" {
		scripts := a.activeHomeScripts()
		ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Center).Gap(10).Children(func() {
			a.homeViewTabs(c, p, true)
			if a.width >= 1160 {
				ui.Text(c, fmt.Sprintf(tr("%d 个组件"), len(scripts))).
					FontSize(11.5).FontWeight(500).FontFeatures("tnum").TextColor(p.muted).NoWrap().Shrink(0)
			}
			ui.Spacer(c)
		})
		ui.Row(c).Shrink(0).AlignItems(ui.Center).Gap(8).Children(func() {
			if paperSegmented(c, &a.cardSize, tr("小"), tr("中"), tr("大")) {
				a.switchSizePreset(a.cardSize)
			}
			ui.Box(c).Width(1).Height(16).Background(p.hairline)
			a.homeScaleControl(c, false)
			a.refreshIconButton(c, "home-bar-ref", "__home_view__", tr("刷新当前视图"), 28, 14, true, p.hover, p.muted, p.accent, p.down, p.up, func() {
				a.refreshAllHome(scripts)
			})
			if a.editing {
				if paperToolbarBtn(c, tr("完成"), true).Clicked() {
					a.editing = false
					a.rz = nil
				}
			} else if paperToolbarBtn(c, tr("编辑布局"), false).Clicked() {
				a.editing = true
			}
		})
		return
	}

	ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Center).Gap(8).Children(func() {
		switch {
		case strings.HasPrefix(path, "/full/"):
			id := strings.TrimPrefix(path, "/full/")
			ui.BackButton(c, a.router)
			title := id
			if sc, ok := a.store.Script(id); ok {
				title = a.scriptDisplayName(sc)
			}
			ui.Text(c, title).FontSize(13.5).FontWeight(600).TextColor(p.text).SingleLine().Ellipsis("…")
		case path == "/edit" || strings.HasPrefix(path, "/edit/"):
			ui.Text(c, tr("组件管理")).FontSize(13.5).FontWeight(600).TextColor(p.text).NoWrap()
			if a.draft.Name != "" {
				ui.Text(c, "· "+a.scriptDisplayName(a.draft)).FontSize(12.5).TextColor(p.muted).SingleLine().Ellipsis("…")
			}
		case path == "/bar" || strings.HasPrefix(path, "/bar/"):
			ui.Text(c, tr("菜单管理")).FontSize(13.5).FontWeight(600).TextColor(p.text).NoWrap()
			if a.barSelectedID != "" {
				if sc, ok := a.store.Script(a.barSelectedID); ok {
					ui.Text(c, "· "+a.scriptDisplayName(sc)).FontSize(12.5).TextColor(p.muted).SingleLine().Ellipsis("…")
				}
			}
		case path == "/community":
			ui.Text(c, tr("社区")).FontSize(13.5).FontWeight(600).TextColor(p.text).NoWrap()
		case path == "/feeds":
			ui.BackButton(c, a.router)
			ui.Text(c, tr("数据源")).FontSize(13.5).FontWeight(600).TextColor(p.text).NoWrap()
		case path == "/commands":
			ui.BackButton(c, a.router)
			ui.Text(c, tr("命令任务")).FontSize(13.5).FontWeight(600).TextColor(p.text).NoWrap()
		case path == "/credentials":
			ui.BackButton(c, a.router)
			ui.Text(c, tr("凭据管理")).FontSize(13.5).FontWeight(600).TextColor(p.text).NoWrap()
		case path == "/settings":
			ui.Text(c, tr("设置")).FontSize(13.5).FontWeight(600).TextColor(p.text).NoWrap()
		default:
			ui.Text(c, appName()).FontSize(13.5).FontWeight(600).TextColor(p.text).NoWrap()
		}
		ui.Spacer(c)
	})
}

func paperToolbarBtn(c *ui.Context, text string, solidInk bool) ui.Element {
	if isMac() {
		return macButton(c, text, solidInk)
	}
	p := pal(c)
	b := ui.ButtonBase(c).
		Height(28).
		Padding(0, 11).
		Radius(radControl).
		AlignItems(ui.Center).
		Justify(ui.Center).
		Cursor(ui.CursorPointer).
		Label(text)
	fg, weight := p.text, 500
	if solidInk {
		fg, weight = p.card, 600
		bg := p.text
		if b.Hovered() {
			bg = p.text.Alpha(0.88)
		}
		b = b.Background(bg)
	} else {
		bg := p.card
		if b.Hovered() {
			bg = p.hover
		}
		b = b.Background(bg).BorderWidth(1).BorderColor(p.border)
	}
	return b.Children(func() {
		ui.Text(c, text).FontSize(12).FontWeight(weight).TextColor(fg).NoWrap()
	})
}

// windowButtons are the minimize, maximize and close buttons of Windows and
// Linux. Close turns red on hover, as the system's does.
func (a *app) windowButtons(c *ui.Context, w *mygo.Window) {
	p := pal(c)
	ui.Row(c).Shrink(0).Height(titleRow).Children(func() {
		if captionButton(c, "minus", p.text, p.hover, false).Clicked() && w != nil {
			w.Minimize()
		}
		maxIcon := "square"
		if w != nil && w.IsMaximized() {
			maxIcon = "copy" // two squares: restore
		}
		if captionButton(c, maxIcon, p.text, p.hover, false).Clicked() && w != nil {
			if w.IsMaximized() {
				w.Restore()
			} else {
				w.Maximize()
			}
		}
		if captionButton(c, "x", p.text, ui.Hex("#E81123"), true).Clicked() && w != nil {
			w.Close()
		}
	})
}

// captionButton is one caption button: an icon on a square that fills on
// hover. The close button's hover is red, and its icon turns white.
func captionButton(c *ui.Context, name string, fg, hover ui.Color, closeKind bool) ui.Element {
	b := ui.ButtonBase(c).Size(captionWidth, titleRow)
	if b.Hovered() {
		b = b.Background(hover)
		if closeKind {
			fg = ui.Hex("#FFFFFF")
		}
	}
	return b.Children(func() {
		ui.Icon(c, icon(name)).FontSize(16).TextColor(fg)
	})
}
