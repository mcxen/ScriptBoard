package main

import (
	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
)

// layoutOf is a result's layout with the defaults filled in: the legend shows,
// the summary sits at the top, the stats at the bottom, the buttons show on
// hover, and a picture fills its card.
func layoutOf(res engine.Result) engine.Layout {
	l := engine.Layout{Legend: "show", Summary: "top", Stats: "bottom", Actions: "hover", Image: "cover"}
	if res.Layout == nil {
		return l
	}
	if res.Layout.Legend != "" {
		l.Legend = res.Layout.Legend
	}
	if res.Layout.Summary != "" {
		l.Summary = res.Layout.Summary
	}
	if res.Layout.Stats != "" {
		l.Stats = res.Layout.Stats
	}
	if res.Layout.Actions != "" {
		l.Actions = res.Layout.Actions
	}
	if res.Layout.Image != "" {
		l.Image = res.Layout.Image
	}
	return l
}

// legendShown is whether a card's legend shows: the script's layout may hide
// it, and so may the user, from the card's menu.
func (a *app) legendShown(id string, lay engine.Layout) bool {
	return lay.Legend != "hide" && !a.set.LegendHidden[id]
}

// toggleLegend shows or hides a card's legend, and keeps the choice.
func (a *app) toggleLegend(id string) {
	if a.set.LegendHidden == nil {
		a.set.LegendHidden = map[string]bool{}
	}
	a.set.LegendHidden[id] = !a.set.LegendHidden[id]
	if err := a.store.SetSettings(a.set); err != nil {
		a.set = a.store.Settings()
	}
}

// cardStats is the card's figures in their own block under the chart, set off
// by a hairline, so that they never run into the headline or the chart.
func cardStats(c *ui.Context, stats []engine.Stat) {
	if len(stats) == 0 {
		return
	}
	p := pal(c)
	ui.Column(c).Gap(8).Children(func() {
		ui.Box(c).Height(1).Background(p.hairline)
		statStrip(c, stats)
	})
}
