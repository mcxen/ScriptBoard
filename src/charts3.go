package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
)

// Two charts that are tables and stacks, not plots: a heatmap of values (a
// retention matrix, a calendar of activity) and a stack of panels that share
// one time axis (a price with its indicators). Both are interactive: a cell is
// chosen with a click, and in a stack the chosen point is marked in every panel.

// isHeat and isPanels report the chart types of this file.
func isHeat(ch *engine.Chart) bool   { return ch != nil && ch.Type == "heat" && ch.Grid != nil }
func isPanels(ch *engine.Chart) bool { return ch != nil && ch.Type == "panels" && len(ch.Panels) > 0 }

// heatColor shades a cell of the accent: the more the value, the stronger it is.
func heatColor(accent ui.Color, frac float64) ui.Color {
	return accent.Alpha(float32(0.12 + 0.88*frac))
}

// heatView draws the table. sel holds the chosen cell as row*columns+column, or
// -1 for none; a click on a cell chooses it, and a second click clears it.
func heatView(c *ui.Context, ch *engine.Chart, sel *int) ui.Element {
	p := pal(c)
	g := ch.Grid
	cols := len(g.Columns)
	chosen := -1
	if sel != nil {
		chosen = *sel
	}
	cell := func(r, col int, v float64, bold bool) {
		idx := r*cols + col
		if v < 0 {
			ui.Row(c).Grow(1).MinWidth(0).Height(26).Radius(3).Background(p.chip).Children(func() {})
			return
		}
		frac := math.Max(0, math.Min(1, v/100))
		fg := p.text
		if frac > 0.5 {
			fg = ui.Hex("#FFFFFF")
		}
		el := ui.ButtonBase(c).Grow(1).MinWidth(0).Height(26).Radius(3).Background(heatColor(p.accent, frac))
		if idx == chosen {
			el = el.BorderWidth(2).BorderColor(p.text)
		}
		if el.Clicked() && sel != nil {
			if *sel == idx {
				*sel = -1
			} else {
				*sel = idx
			}
		}
		el.Children(func() {
			t := ui.Text(c, fmt.Sprintf("%.0f", v)).FontSize(fsSmall).TextColor(fg).NoWrap()
			if bold {
				t.Bold()
			}
		})
	}
	return ui.Column(c).Gap(4).Children(func() {
		// The column names.
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "").Width(150)
			for _, name := range g.Columns {
				ui.Text(c, name).FontSize(fsTiny).TextColor(p.muted).Grow(1).MinWidth(0).NoWrap()
			}
		})
		for r, row := range g.Rows {
			r, row := r, row
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Width(150).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Text(c, row.Label).FontSize(fsSmall).NoWrap()
					if row.Note != "" {
						ui.Text(c, row.Note).FontSize(fsTiny).TextColor(p.muted).NoWrap()
					}
				})
				for col := 0; col < cols; col++ {
					v := -1.0
					if col < len(row.Cells) {
						v = row.Cells[col]
					}
					cell(r, col, v, false)
				}
			})
		}
		if len(g.Average) > 0 {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				ui.Text(c, tr("平均")).FontSize(fsSmall).Bold().Width(150)
				for col := 0; col < cols; col++ {
					v := -1.0
					if col < len(g.Average) {
						v = g.Average[col]
					}
					if v < 0 {
						ui.Text(c, "").Grow(1)
						continue
					}
					ui.Text(c, fmt.Sprintf("%.0f", v)).FontSize(fsSmall).Bold().Grow(1).MinWidth(0).TextAlign(ui.Center)
				}
			})
		}
		foot := g.Footer
		if chosen >= 0 && chosen < len(g.Rows)*cols {
			r, col := chosen/cols, chosen%cols
			row := g.Rows[r]
			v := -1.0
			if col < len(row.Cells) {
				v = row.Cells[col]
			}
			if v >= 0 {
				foot = fmt.Sprintf("%s · %s · %.1f%%", row.Label, g.Columns[col], v)
			}
		}
		if foot != "" {
			ui.Text(c, foot).FontSize(fsSmall).TextColor(p.muted).Padding(6, 0, 0, 0)
		}
	})
}

// panelsView stacks the panels of ch. Every panel has the same time axis, so
// the chosen point (sel) is marked in each of them: a crosshair across the
// stack. Each panel is titled and takes its share of h by its weight.
func panelsView(c *ui.Context, ch *engine.Chart, style string, h float32, sel *int, full bool) ui.Element {
	p := pal(c)
	total := 0.0
	for _, pn := range ch.Panels {
		total += math.Max(pn.Weight, 1)
	}
	return ui.Column(c).Gap(8).Children(func() {
		for _, pn := range ch.Panels {
			pn := pn
			share := math.Max(pn.Weight, 1) / total
			ph := float32(float64(h) * share)
			ui.Text(c, strings.ToUpper(pn.Title)).FontSize(fsTiny).TextColor(p.faint).NoWrap()
			sub := &engine.Chart{Type: "line", Series: pn.Series, Labels: ch.Labels, XLabels: ch.XLabels}
			st := pn.Style
			if st == "" {
				st = style
			}
			if full {
				chartWith(c, sub, st, ph-18, sel, true)
			} else {
				Spark(c, sub, ph-18)
			}
		}
	})
}
