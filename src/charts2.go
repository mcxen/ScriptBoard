package main

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"scriptboard/src/internal/engine"
)

// Two charts that are not plots: a gauge, which shows how much of a quantity
// is left, and horizontal bars, which compare a few named things.

// isGaugeOrBars reports whether a chart is drawn by this file.
// isCombo reports whether ch is a curve with rings beside it.
func isCombo(ch *engine.Chart) bool { return ch != nil && ch.Type == "combo" }

func isGaugeOrBars(ch *engine.Chart) bool {
	return ch != nil && (ch.Type == "gauge" || ch.Type == "barh")
}

// remaining is how much of a gauge's limit is left, and its share of it.
func remaining(g *engine.Gauge) (left, frac float64) {
	if g == nil || g.Limit <= 0 {
		return 0, 0
	}
	left = g.Limit - g.Used
	frac = math.Min(1, math.Max(0, left/g.Limit))
	return left, frac
}

// gaugeColor is green while plenty is left, amber when it runs low and red
// when it is nearly gone.
func gaugeColor(p palette, frac float64) ui.Color {
	switch {
	case frac <= 0.10:
		return p.up
	case frac <= 0.30:
		return p.warn
	}
	return p.down
}

// formatAmount prints an amount in a short form: 1.2K, 3.4M, 12.5 or 640.
func formatAmount(v float64) string {
	a := math.Abs(v)
	switch {
	case a >= 1e9:
		return fmt.Sprintf("%.1fB", v/1e9)
	case a >= 1e6:
		return fmt.Sprintf("%.1fM", v/1e6)
	case a >= 1e4:
		return fmt.Sprintf("%.1fK", v/1e3)
	case a >= 100:
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.2f", v)
}

// donut paints the ring: what is left in color from twelve o'clock, what is
// used on the track.
func donut(p *ui.Painter, r ui.Rect, frac float64, color, track ui.Color) {
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	outer := min(r.W, r.H) / 2
	inner := outer * 0.76
	switch {
	case frac >= 0.999:
		for _, s := range chart.Pie([]float64{1, 1}, 0) {
			p.FillPath(chart.Sector(cx, cy, inner, outer, s), color)
		}
	case frac <= 0.001:
		for _, s := range chart.Pie([]float64{1, 1}, 0) {
			p.FillPath(chart.Sector(cx, cy, inner, outer, s), track)
		}
	default:
		slices := chart.Pie([]float64{frac, 1 - frac}, 0)
		p.FillPath(chart.Sector(cx, cy, inner, outer, slices[0]), color)
		p.FillPath(chart.Sector(cx, cy, inner, outer, slices[1]), track)
	}
}

// gaugeView draws a gauge: the ring, and beside it how much is left, how much
// is used and when it renews. h is the ring's size.
func gaugeView(c *ui.Context, ch *engine.Chart, h float32) ui.Element {
	return gaugeViewWith(c, ch, "", h)
}

func gaugeViewWith(c *ui.Context, ch *engine.Chart, style string, h float32) ui.Element {
	p := pal(c)
	g := ch.Gauge
	if g == nil {
		return ui.Text(c, tr("没有额度数据")).TextColor(p.muted)
	}
	left, frac := remaining(g)
	color := gaugeColor(p, frac)
	size := clampF(h*0.62, 64, 200)
	isMatrix := style == "matrix" || (style == "" && g.Style == "matrix")
	if isMatrix {
		size = clampF(h*0.72, 88, 210)
	}
	return ui.Row(c).AlignItems(ui.Center).Gap(18).Children(func() {
		if isMatrix {
			ui.Column(c).Size(size, size).Shrink(0).AlignItems(ui.Center).Justify(ui.Center).
				Draw(func(pt *ui.Painter, r ui.Rect) {
					paintMatrixGauge(pt, r, frac, color)
				}).Children(func() {
				ui.Text(c, fmt.Sprintf("%.0f%%", math.Round(frac*100))).FontSize(clampF(size*0.21, 13, 24)).FontWeight(700).TextColor(ui.Hex("#F8FAFC")).NoWrap()
			})
		} else {
			ui.Box(c).Size(size, size).Shrink(0).Draw(func(pt *ui.Painter, r ui.Rect) {
				donut(pt, r, frac, color, p.chip)
			})
		}
		ui.Column(c).Gap(3).MinWidth(0).Children(func() {
			ui.Text(c, fmt.Sprintf(tr("剩余 %.0f%%"), frac*100)).FontSize(fsCount).FontWeight(650).TextColor(color).NoWrap()
			ui.Text(c, fmt.Sprintf("%s %s", formatAmount(left), g.Unit)).FontSize(fsBody).NoWrap()
			ui.Text(c, fmt.Sprintf(tr("已用 %s / %s"), formatAmount(g.Used), formatAmount(g.Limit))).FontSize(fsSmall).TextColor(p.muted).NoWrap()
			if g.Reset != "" {
				ui.Text(c, g.Reset).FontSize(fsTiny).TextColor(p.faint).NoWrap()
			}
		})
	})
}

// barhView draws Series[0] as horizontal bars: a label, a bar as long as its
// share of the largest value, and the value. As many rows show as fit h.
func barhView(c *ui.Context, ch *engine.Chart, h float32) ui.Element {
	p := pal(c)
	if len(ch.Series) == 0 || len(ch.Series[0].Points) == 0 {
		return ui.Text(c, tr("暂无数据")).TextColor(p.muted)
	}
	s := ch.Series[0]
	n := len(s.Points)
	if h < 240 {
		rows := max(1, int(h/26))
		n = min(n, rows)
	}
	hi := 0.0
	for _, v := range s.Points[:n] {
		hi = math.Max(hi, v)
	}
	if hi <= 0 {
		hi = 1
	}
	return ui.Column(c).Gap(6).Children(func() {
		for i := 0; i < n; i++ {
			i := i
			name := fmt.Sprintf("#%d", i+1)
			if i < len(ch.Labels) {
				name = ch.Labels[i]
			}
			color := parseColor(s.Color, seriesPalette[i%len(seriesPalette)])
			frac := float32(s.Points[i] / hi)
			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				ui.Row(c).Width(96).MinWidth(0).ClipX().Children(func() {
					ui.Text(c, name).FontSize(fsSmall).TextColor(p.muted).NoWrap()
				})
				ui.Box(c).Grow(1).Height(10).Radius(5).Background(p.chip).Draw(func(pt *ui.Painter, r ui.Rect) {
					pt.Fill(ui.Rect{X: r.X, Y: r.Y, W: max(r.W*frac, 4), H: r.H}, color, 5)
				})
				ui.Text(c, formatAmount(s.Points[i])).FontSize(fsSmall).FontWeight(600).Width(64).TextAlign(ui.End).NoWrap()
			})
		}
	})
}

// kindTiles are the statistics of a gauge or of bars; it reports whether it
// drew any.
func (a *app) kindTiles(c *ui.Context, ch *engine.Chart) bool {
	if !isGaugeOrBars(ch) {
		return false
	}
	p := pal(c)
	cols := 4
	if a.compact {
		cols = 2
	}
	var tiles []tileData
	switch {
	case ch.Type == "gauge" && ch.Gauge != nil:
		g := ch.Gauge
		left, frac := remaining(g)
		tiles = []tileData{
			{tr("剩余"), formatAmount(left) + " " + g.Unit, fmt.Sprintf("%.1f%%", frac*100), gaugeColor(p, frac)},
			{tr("已用"), formatAmount(g.Used) + " " + g.Unit, fmt.Sprintf("%.1f%%", 100-frac*100), p.text},
			{tr("上限"), formatAmount(g.Limit) + " " + g.Unit, g.Label, p.text},
		}
		if g.Reset != "" {
			tiles = append(tiles, tileData{tr("重置"), g.Reset, "", p.text})
		}
	case len(ch.Series) > 0 && len(ch.Series[0].Points) > 0:
		sum, top, topI, low, lowI := 0.0, math.Inf(-1), 0, math.Inf(1), 0
		for i, v := range ch.Series[0].Points {
			sum += v
			if v > top {
				top, topI = v, i
			}
			if v < low {
				low, lowI = v, i
			}
		}
		topName, lowName := "", ""
		if topI < len(ch.Labels) {
			topName = ch.Labels[topI]
		}
		if lowI < len(ch.Labels) {
			lowName = ch.Labels[lowI]
		}
		tiles = []tileData{
			{tr("合计"), formatAmount(sum), fmt.Sprintf(tr("%d 项"), len(ch.Series[0].Points)), p.text},
			{tr("最大项"), formatAmount(top), topName, p.accent},
			{tr("最小项"), formatAmount(low), lowName, p.text},
		}
	}
	a.tileRow(c, cols, tiles)
	return true
}

// comboView is a curve on the left and rings on the right. The curve is drawn
// as a chart is (a bare curve on a card, the full chart in full view); each
// ring is a share of 100 percent, with its label under it.
func comboView(c *ui.Context, ch *engine.Chart, style string, h float32, sel *int, pinned *int, full bool) ui.Element {
	ringSize := clampF(h*0.42, 40, 64)
	return ui.Row(c).AlignItems(ui.Center).Gap(16).Children(func() {
		if len(ch.Series) > 0 {
			sub := &engine.Chart{Type: "line", Series: ch.Series, Labels: ch.Labels, XLabels: ch.XLabels}
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				if full {
					chartWith(c, sub, style, h, sel, pinned, true)
				} else {
					Spark(c, sub, h)
				}
			})
		}
		if len(ch.Rings) > 0 {
			ui.Row(c).Shrink(0).Gap(14).AlignItems(ui.Center).Children(func() {
				ringsView(c, ch.Rings, ringSize)
			})
		}
	})
}

// ringsView draws each ring: a donut of its share, the percent or text in it,
// and its label under it.
func ringsView(c *ui.Context, rings []engine.Ring, d float32) {
	p := pal(c)
	for _, rg := range rings {
		rg := rg
		frac := math.Max(0, math.Min(1, rg.Percent/100))
		color := parseColor(rg.Color, p.accent)
		text := rg.Text
		if text == "" {
			text = fmt.Sprintf("%.0f%%", rg.Percent)
		}
		ui.Column(c).Gap(4).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Size(d, d).AlignItems(ui.Center).Justify(ui.Center).
				Draw(func(pt *ui.Painter, r ui.Rect) { donut(pt, r, frac, color, p.chip) }).Children(func() {
				ui.Text(c, text).FontSize(fsTiny).FontWeight(650).TextColor(p.text).NoWrap()
			})
			ui.Text(c, rg.Label).FontSize(fsTiny).TextColor(p.muted).NoWrap()
		})
	}
}
