package main

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
)

// stats is what a chart says about itself: its latest value, the range, the
// average and the change, with the labels of the points they happened at.
type stats struct {
	ok                    bool
	count                 int
	first, last           float64
	max, min, avg         float64
	maxLabel, minLabel    string
	firstLabel, lastLabel string
	change, pct           float64
}

// summarize reads the statistics of a chart. For a candle chart the values are
// the closes, and the high and low are the extremes of the candles.
func summarize(ch *engine.Chart) stats {
	var st stats
	if ch == nil {
		return st
	}
	var vals []float64
	hiV, loV := math.Inf(-1), math.Inf(1)
	var hiI, loI int
	consider := func(i int, high, low float64) {
		if high > hiV {
			hiV, hiI = high, i
		}
		if low < loV {
			loV, loI = low, i
		}
	}
	switch {
	case ch.Type == "candle":
		for i, cd := range ch.Candles {
			vals = append(vals, cd.C)
			consider(i, cd.H, cd.L)
		}
	case isPanels(ch) && len(ch.Panels[0].Series) > 0:
		for i, v := range ch.Panels[0].Series[0].Points {
			vals = append(vals, v)
			consider(i, v, v)
		}
	case len(ch.Series) > 0:
		for i, v := range ch.Series[0].Points {
			vals = append(vals, v)
			consider(i, v, v)
		}
	}
	if len(vals) == 0 {
		return st
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	st.ok = true
	st.count = len(vals)
	st.first, st.last = vals[0], vals[len(vals)-1]
	st.max, st.min = hiV, loV
	st.avg = sum / float64(len(vals))
	st.maxLabel = labelOf(ch, hiI)
	st.minLabel = labelOf(ch, loI)
	st.firstLabel = labelOf(ch, 0)
	st.lastLabel = labelOf(ch, len(vals)-1)
	st.change = st.last - st.first
	if st.first != 0 {
		st.pct = st.change / st.first * 100
	}
	return st
}

// labelOf is the label of point i, or its number when it has none.
func labelOf(ch *engine.Chart, i int) string {
	if i >= 0 && i < len(ch.Labels) && ch.Labels[i] != "" {
		return ch.Labels[i]
	}
	return fmt.Sprintf(tr("第 %d 个"), i+1)
}

// precision is how many decimals a value of this size is shown with.
func precision(v float64) int {
	if math.Abs(v) < 10 {
		return 4
	}
	return 2
}

// statTiles shows the statistics as a grid of tiles of equal width: two
// across on a phone, six on a wide window.
func (a *app) statTiles(c *ui.Context, st stats) {
	p := pal(c)
	if !st.ok {
		return
	}
	prec := precision(st.last)
	cols := 6
	if a.compact {
		cols = 2
	}
	trend := p.text
	if st.change > 0 {
		trend = p.up
	} else if st.change < 0 {
		trend = p.down
	}
	a.tileRow(c, cols, []tileData{
		{tr("最新"), fmt.Sprintf("%.*f", prec, st.last), st.lastLabel, p.text},
		{tr("涨跌"), fmt.Sprintf("%+.*f", prec, st.change), fmt.Sprintf(tr("%+.2f%% · 自 %s"), st.pct, st.firstLabel), trend},
		{tr("最高"), fmt.Sprintf("%.*f", prec, st.max), st.maxLabel, p.text},
		{tr("最低"), fmt.Sprintf("%.*f", prec, st.min), st.minLabel, p.text},
		{tr("平均"), fmt.Sprintf("%.*f", prec, st.avg), fmt.Sprintf(tr("共 %d 个点"), st.count), p.text},
		{tr("振幅"), fmt.Sprintf("%.*f", prec, st.max-st.min), tr("最高减最低"), p.text},
	})
}

// tileData is one tile of a row of statistics: a name, its value, a line under
// it, and the color of the value.
type tileData struct {
	name, value, sub string
	color            ui.Color
}

// tileRow draws tiles of equal width, cols across, wrapping to more rows. The
// statistics of a line chart and those of a gauge or bars are drawn by it.
func (a *app) tileRow(c *ui.Context, cols int, tiles []tileData) {
	p := pal(c)
	const gap = 12
	w := (a.contentW() - gap*float32(cols-1)) / float32(cols)
	ui.Row(c).Wrap().Gap(gap).Children(func() {
		for _, t := range tiles {
			card(c).Width(w).Padding(14).Gap(3).Children(func() {
				ui.Text(c, t.name).FontSize(fsTiny).TextColor(p.muted)
				ui.Text(c, t.value).FontSize(fsCount).FontWeight(650).TextColor(t.color).NoWrap()
				ui.Text(c, t.sub).FontSize(fsTiny).TextColor(p.muted).NoWrap()
			})
		}
	})
}

// statStrip is a result's figures in a row under its headline: each a small
// name over a bold value, with its line under that. A card shows the strip;
// the full view shows the same figures as tiles (statsTiles).
func statStrip(c *ui.Context, stats []engine.Stat) {
	if len(stats) == 0 {
		return
	}
	p := pal(c)
	ui.Row(c).Wrap().Gap(18).AlignItems(ui.Start).Children(func() {
		for _, st := range stats {
			ui.Column(c).Gap(1).Children(func() {
				ui.Text(c, st.Label).FontSize(fsTiny).TextColor(p.muted).NoWrap()
				ui.Text(c, st.Value).FontSize(fsBody).FontWeight(650).NoWrap()
				if st.Sub != "" {
					ui.Text(c, st.Sub).FontSize(fsTiny).TextColor(p.faint).NoWrap()
				}
			})
		}
	})
}

// statsTiles is a result's figures as tiles of equal width, two across on a
// phone and four on a window: the full view's statistics for a live result.
func (a *app) statsTiles(c *ui.Context, stats []engine.Stat) {
	cols := 4
	if a.compact {
		cols = 2
	}
	tiles := make([]tileData, 0, len(stats))
	p := pal(c)
	for _, st := range stats {
		tiles = append(tiles, tileData{name: st.Label, value: st.Value, sub: st.Sub, color: p.text})
	}
	a.tileRow(c, cols, tiles)
}
