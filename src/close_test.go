package main

import (
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Closing the main window minimizes it, and the app keeps running; once the
// app is quitting, the window closes.
func TestClosingTheMainWindowMinimizesIt(t *testing.T) {
	a := newTestApp(t)
	minimized := 0
	e := &mygo.CloseEvent{}
	a.onMainClose(e, func() { minimized++ })
	if !e.DefaultPrevented() || minimized != 1 {
		t.Errorf("prevented %v, minimized %d: closing must be cancelled and the window minimized", e.DefaultPrevented(), minimized)
	}

	a.quitting = true
	e = &mygo.CloseEvent{}
	a.onMainClose(e, func() { minimized++ })
	if e.DefaultPrevented() || minimized != 1 {
		t.Errorf("while quitting: prevented %v, minimized %d: the window must close", e.DefaultPrevented(), minimized)
	}
}

// The settings have a Quit, since closing the window no longer ends the app.
func TestSettingsHaveAQuit(t *testing.T) {
	a := newTestApp(t)
	quits := 0
	old := quitApp
	quitApp = func() { quits++ }
	defer func() { quitApp = old }()
	tt := ui.NewTester(a.settingsView, 700, 3000)
	settle(tt)
	settle(tt)
	click(t, tt, "退出应用")
	if quits != 1 {
		t.Errorf("quit %d times, want once", quits)
	}
}
