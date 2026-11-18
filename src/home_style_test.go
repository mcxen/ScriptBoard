package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// A home card draws its chart in the style chosen in the full view: choosing
// a style there changes the card too.
func TestHomeCardFollowsChosenStyle(t *testing.T) {
	a := newTestApp(t)
	a.styles["fx"] = "bar"
	tt := ui.NewTester(func(c *ui.Context) {
		applySkin(c)
		a.home(c, false)
	}, 960, 640)
	settle(tt)
	requireText(t, tt, "USD / CNY · 近 30 天")
	savePNG(t, tt, "22-home-chosen-style")
}
