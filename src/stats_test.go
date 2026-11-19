package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/store"
)

// The card shows a result's figures under its headline, and the full view shows
// them as tiles.
func TestStatsAreDrawnOnCardAndFullView(t *testing.T) {
	a := newTestApp(t)
	a.store.SaveScript(store.Script{ID: "live-t", Name: "实况", Home: true, Origin: store.OriginUser,
		Source: `export function meta() { return { name: "实况", sources: {} }; }
export function render(ctx) { return { title: "实况", big: "18°", sub: "多云", lines: [], chart: null,
  stats: [{ label: "体感", value: "17°" }, { label: "湿度", value: "63%" }] }; }`})
	tt := ui.NewTester(a.view, 1200, 900)
	a.router.Push("/full/live-t")
	settle(tt)
	requireText(t, tt, "体感")
	requireText(t, tt, "63%")
	a.router.Push("/")
	settle(tt)
	requireText(t, tt, "湿度")
}
