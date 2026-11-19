package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/store"
)

// The mini card shows only its number: no title, no sub-line.
func TestMiniCardShowsOnlyTheNumber(t *testing.T) {
	a := newTestApp(t)
	a.store.SaveScript(store.Script{ID: "mini-t", Name: "迷你", Home: true, Origin: store.OriginUser,
		Source: `export function meta() { return { name: "迷你", sources: {} }; }
export function render(ctx) { return { title: "迷你", big: "42.5", sub: "subline-zz", lines: [], chart: null }; }`})
	a.setSize("mini-t", sizeMini)
	tt := ui.NewTester(a.view, 1200, 900)
	settle(tt)
	settle(tt)
	info, ok := tt.Access("42.5")
	if !ok {
		t.Fatal("the number of the mini card is not on the page")
	}
	if _, ok := tt.Access("subline-zz"); ok {
		t.Error("the mini card shows its sub-line")
	}
	if info.Bounds.H > 120 {
		t.Errorf("mini number box is %v tall", info.Bounds.H)
	}
}
