package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// The settings open in a window of their own on a computer: they are a page of
// rows, which is best at its own narrow width, not stretched over the main
// window, and the main window stays where it was. A phone has one screen, and
// there the settings are a page of it (/settings). The window is opened once
// and brought forward when it is asked for again.

// settingsWidth and settingsHeight are the window's first size.
const (
	settingsWidth  = 780
	settingsHeight = 640
)

// goTo is where a click on a place of the sidebar or the tab bar goes: the page,
// or, for the settings on a computer, their window.
func (a *app) goTo(path string) {
	if path == "/settings" && !isMobile() {
		if a.openSettingsHook != nil {
			a.openSettingsHook()
			return
		}
		if a.win != nil {
			a.openSettings()
			return
		}
	}
	a.router.Push(path)
}

// openSettings shows the settings window, opening it the first time.
func (a *app) openSettings() {
	if a.settingsWin != nil {
		a.settingsWin.Show()
		a.settingsWin.Focus()
		return
	}
	w := mygo.NewWindow(framedOptions(mygo.WindowOptions{
		Title:     tr("设置"),
		Width:     settingsWidth,
		Height:    settingsHeight,
		MinWidth:  560,
		MinHeight: 440,
		Vibrancy:  mygo.VibrancySidebar,
		// Opens where the user left it last time.
		StateKey: "settings",
		Content:  ui.View(a.settingsView),
	}))
	a.settingsWin = w
	a.post(func() {})
	w.OnClosed(func() {
		if a.settingsWin == w {
			a.settingsWin = nil
			a.post(func() {})
		}
	})
}

// settingsView is the content of the settings window: a title row, as the main
// window has, and the settings page.
func (a *app) settingsView(c *ui.Context) {
	applySkin(c)
	setPageWidth(a.set.ContentWidth)
	w := a.settingsWin
	root := skinBackground(c, ui.Column(c).Fill())
	if root.OverlayShortcut(0, ui.KeyEscape) && w != nil {
		w.Close()
	}
	if ownControls && root.OverlayShortcut(ui.Ctrl, ui.KeyQ) {
		quitApp()
	}
	root.Children(func() {
		switch {
		case isMac():
			macMenuBarIn(c, a, w)
		default:
			titleBarIn(c, a, w, func() {
				ui.Text(c, tr("设置")).FontSize(fsSmall).FontWeight(550).TextColor(pal(c).muted).Padding(0, 8)
			})
		}
		a.settingsBody(c, false, false)
	})
}

// openFeeds shows the data sources page, in the main window: the settings window
// has no pages of its own to go to.
func (a *app) openFeeds() {
	a.router.Push("/feeds")
	a.showWindow()
}

// openCommands shows the command tasks page in the main window.
func (a *app) openCommands() {
	a.router.Push("/commands")
	a.showWindow()
}
