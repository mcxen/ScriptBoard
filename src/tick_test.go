package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// A figure keeps its unit and decimals while it moves, is still on its way
// part of the time, and settles on its value.
func TestTickTextMovesThenSettles(t *testing.T) {
	ticks = map[string]*tickState{}
	t.Cleanup(func() { ticks = map[string]*tickState{} })
	val := "10.00 kg"
	view := func(c *ui.Context) { ui.Text(c, tickText(c, "tick-test", val)) }
	tt := ui.NewTester(view, 240, 60)
	settle(tt)
	requireText(t, tt, "10.00 kg")

	val = "20.00 kg"
	tt.Frame()
	tt.Advance(60 * time.Millisecond)
	tt.Frame()
	if tt.HasText("20.00 kg") {
		t.Errorf("the figure jumped to its new value at once")
	}
	settle(tt)
	requireText(t, tt, "20.00 kg")
}

// Text with no figure is shown as it is.
func TestTickTextWithoutFigure(t *testing.T) {
	ticks = map[string]*tickState{}
	t.Cleanup(func() { ticks = map[string]*tickState{} })
	view := func(c *ui.Context) { ui.Text(c, tickText(c, "no-figure", "未填写")) }
	tt := ui.NewTester(view, 240, 60)
	settle(tt)
	requireText(t, tt, "未填写")
}
