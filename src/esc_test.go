package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// Escape goes back to the page before, from the pages pushed on top of home.
func TestEscapeGoesBack(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 1000, 700)
	settle(tt)
	click(t, tt, "设置")
	if a.router.Path() != "/settings" {
		t.Fatalf("path %q after opening settings", a.router.Path())
	}
	tt.Key(0, ui.KeyEscape)
	settle(tt)
	if a.router.Path() != "/" {
		t.Errorf("path %q after Escape, want home", a.router.Path())
	}
}
