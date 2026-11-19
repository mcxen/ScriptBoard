package main

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
)

// progressView draws a "progress" chart: one bar per item, each with its name,
// its value out of its maximum, and how full it is. The bars share the card's
// width, so the same chart reads the same on a phone and on a window.
// limit is how many bars show (0 is all); a card too short for them all shows
// the first and counts the rest.
func progressView(c *ui.Context, items []engine.Progress, limit int) ui.Element {
	p := pal(c)
	shown, more := items, 0
	if limit > 0 && len(items) > limit {
		shown, more = items[:limit], len(items)-limit
	}
	return ui.Column(c).Gap(10).Children(func() {
		if len(items) == 0 {
			ui.Text(c, tr("暂无进度")).FontSize(fsSmall).TextColor(p.muted)
		}
		for _, it := range shown {
			pct := it.Percent()
			ui.Column(c).Gap(4).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Text(c, it.Name).FontSize(fsSmall).TextColor(p.muted).Grow(1).MinWidth(0).NoWrap()
					ui.Text(c, progressLabel(it, pct)).FontSize(fsSmall).FontWeight(600).NoWrap()
				})
				// The fill and the rest of the track share the row by their
				// percentages, so the bar is exactly as full as the value says.
				ui.Row(c).Height(8).Radius(4).Background(p.chip).Children(func() {
					ui.Box(c).Height(8).Radius(4).Background(p.accent).Grow(float32(max(pct, 0.0001))).Shrink(0)
					ui.Box(c).Height(8).Grow(float32(max(100-pct, 0.0001))).Shrink(0)
				})
			})
		}
		if more > 0 {
			ui.Text(c, fmt.Sprintf(tr("另外 %d 项"), more)).FontSize(fsTiny).TextColor(p.muted)
		}
	})
}

// progressLabel is "3 / 10 · 30%", or "30%" for a bar given as a percentage.
func progressLabel(it engine.Progress, pct float64) string {
	pctText := fmt.Sprintf("%.0f%%", math.Round(pct))
	if it.Max > 0 {
		return fmt.Sprintf("%s / %s%s · %s", trimNum(it.Value), trimNum(it.Max), it.Unit, pctText)
	}
	return pctText
}

// trimNum writes a number without trailing zeros: 3 for 3.0, 2.5 for 2.5.
func trimNum(v float64) string {
	if v == math.Trunc(v) {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.2f", v)
}

// legendRow names a multi-series chart's lines, each with its color: the
// legend. A chart with one series needs none; its readout already names it.
func legendRow(c *ui.Context, ch *engine.Chart) {
	if ch == nil || len(ch.Series) < 2 {
		return
	}
	p := pal(c)
	ui.Row(c).Wrap().Gap(12).AlignItems(ui.Center).Children(func() {
		for i, sr := range ch.Series {
			if sr.Name == "" {
				continue
			}
			color := parseColor(sr.Color, seriesPalette[i%len(seriesPalette)])
			ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
				ui.Box(c).Size(8, 8).Radius(4).Background(color).Shrink(0)
				ui.Text(c, sr.Name).FontSize(fsTiny).TextColor(p.muted).NoWrap()
			})
		}
	})
}
