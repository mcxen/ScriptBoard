package main

import (
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// The window's title row, as notes and Kero draw it. On macOS the window keeps
// its traffic lights, inset into the row; on Windows and Linux the system
// buttons are hidden and the page draws minimize, maximize and close at the top
// right of the row. The row drags the window everywhere. Every skin uses the
// same height, so the lights stay where the window put them when a skin changes.

const (
	// titleRow is the height of the title row at the top of the window.
	titleRow = 38
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
