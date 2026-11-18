package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
)

// runExample compiles and renders a file of docs/examples.
func runExample(t *testing.T, name string) *engine.Chart {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "docs", "examples", name))
	if err != nil {
		t.Fatal(err)
	}
	s, err := engine.Compile(name, string(src))
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Run(context.Background(), engine.Input{Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	return res.Chart
}

// The example scripts draw their tables and stacks.
func TestExamplesRender(t *testing.T) {
	heat := runExample(t, "retention-heat.js")
	if !isHeat(heat) || len(heat.Grid.Rows) != 12 || len(heat.Grid.Columns) != 12 {
		t.Fatalf("heat %+v", heat)
	}
	panels := runExample(t, "price-panels.js")
	if !isPanels(panels) || len(panels.Panels) != 4 || len(panels.Panels[0].Series[0].Points) != 90 {
		t.Fatalf("panels %+v", panels)
	}

	// The full views of both, and a card for each.
	for _, tc := range []struct {
		name string
		ch   *engine.Chart
	}{{"heat", heat}, {"panels", panels}} {
		tc := tc
		view := func(c *ui.Context) {
			applySkin(c)
			sel := -1
			Chart(c, ChartProps{Data: tc.ch, Height: 300, Full: true, Selected: &sel})
		}
		tt := ui.NewTester(view, 760, 620)
		settle(tt)
		savePNG(t, tt, "20-example-"+tc.name)
	}
}
