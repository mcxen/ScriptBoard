package main

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

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
					ui.Box(c).Height(8).Radius(4).Background(parseColor(it.Color, p.accent)).Grow(float32(max(pct, 0.0001))).Shrink(0)
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
		return progressCount(it) + " · " + pctText
	}
	return pctText
}

// progressCount is "3 / 10章": the value out of its maximum.
func progressCount(it engine.Progress) string {
	return fmt.Sprintf("%s / %s%s", trimNum(it.Value), trimNum(it.Max), it.Unit)
}

// isProgress says whether a chart is a completion chart, as bars or as rings.
func isProgress(ch *engine.Chart) bool {
	return ch != nil && (ch.Type == "progress" || ch.Type == "progressRing")
}

// ringPalette is the colors of the rings of a concentric chart, one each:
// distinct, bright hues, as Apple's activity rings have.
var ringPalette = []ui.Color{
	ui.Hex("#F43F5E"), ui.Hex("#84CC16"), ui.Hex("#22D3EE"), ui.Hex("#F59E0B"), ui.Hex("#8B5CF6"),
}

// ringAligns are the ways the rings can sit across the card.
var ringAligns = map[string]ui.Align{
	"left": ui.Start, "center": ui.Center, "right": ui.End,
	"between": ui.SpaceBetween, "around": ui.SpaceAround, "evenly": ui.SpaceEvenly,
}

// ringStyle is a chart's ring options, with the defaults and the limits applied.
type ringStyle struct {
	concentric  bool
	size        float32 // 0 fits the card
	thick       float32 // the stroke's width, as a share of the size
	butt, solid bool
	track       string // tint, neutral or none
	center      string // percent, value or none
	align       ui.Align
}

func ringStyleOf(ch *engine.Chart) ringStyle {
	var r engine.RingStyle
	if ch != nil && ch.Ring != nil {
		r = *ch.Ring
	}
	st := ringStyle{concentric: r.Layout == "concentric", butt: r.Cap == "butt", solid: r.Fill == "solid",
		track: r.Track, center: r.Center, align: ringAligns[r.Align]} // the zero value is left
	if st.track == "" {
		st.track = "tint"
	}
	if st.center == "" {
		st.center = "percent"
	}
	if r.Size > 0 {
		st.size = clampF(r.Size, 32, 200)
	}
	st.thick = 0.11
	if st.concentric {
		st.thick = 0.085
	}
	if r.Thickness > 0 {
		st.thick = clampF(r.Thickness, 0.04, 0.3)
	}
	return st
}

// ringVAlign is where a progressRing chart sits down its card: "top" (right
// under the summary), "middle" or "bottom". Any other chart is "".
func ringVAlign(ch *engine.Chart) string {
	if ch == nil || ch.Type != "progressRing" {
		return ""
	}
	if ch.Ring != nil && (ch.Ring.VAlign == "middle" || ch.Ring.VAlign == "bottom") {
		return ch.Ring.VAlign
	}
	return "top"
}

// ringColors are the colors an item's arc goes from and to. A concentric ring
// has a hue of its own, a done ring is green, and a script's color or its "to"
// wins over both. The arc deepens along its length, unless the fill is solid.
func ringColors(p palette, st ringStyle, it engine.Progress, i int) (from, to ui.Color) {
	base := p.accent
	switch {
	case st.concentric:
		base = ringPalette[i%len(ringPalette)]
	case it.Raw() >= 100:
		base = p.down
	}
	base = parseColor(it.Color, base)
	if st.solid {
		return base, base
	}
	if hexColor.MatchString(it.To) {
		return base, ui.Hex(it.To)
	}
	return base.Alpha(0.5), base
}

// ringTrack is the ring under an arc: the arc's color, faint, as Apple's rings
// have it; or neutral gray; or none.
func ringTrack(st ringStyle, tint, neutral ui.Color) (ui.Color, bool) {
	switch st.track {
	case "none":
		return ui.Color{}, false
	case "neutral":
		return neutral, true
	}
	return tint.Alpha(0.2), true
}

// ringCenter is the text inside a ring.
func ringCenter(st ringStyle, it engine.Progress) string {
	switch st.center {
	case "none":
		return ""
	case "value":
		return trimNum(it.Value)
	}
	return fmt.Sprintf("%.0f%%", math.Round(it.Raw()))
}

// progressRingView draws a "progressRing" chart: one ring for each item, its
// percentage inside, its name under it and, with a maximum, its count; or, with
// ring layout "concentric", the rings nested in one, with a legend. d is the
// ring's size, unless the chart sets one; limit is how many items show (0 is
// all), the rest counted.
func progressRingView(c *ui.Context, ch *engine.Chart, limit int, d float32) ui.Element {
	p := pal(c)
	st := ringStyleOf(ch)
	items := ch.Progress
	shown, more := items, 0
	if limit > 0 && len(items) > limit {
		shown, more = items[:limit], len(items)-limit
	}
	if st.concentric {
		if st.size > 0 {
			d = st.size
		} else {
			d *= 1.45
		}
		return progressConcentricView(c, p, st, shown, more, d)
	}
	if st.size > 0 {
		d = st.size
	}
	return ui.Row(c).Wrap().Gap(18).Justify(st.align).Children(func() {
		if len(items) == 0 {
			ui.Text(c, tr("暂无进度")).FontSize(fsSmall).TextColor(p.muted)
		}
		for i, it := range shown {
			from, to := ringColors(p, st, it, i)
			track, hasTrack := ringTrack(st, to, p.chip)
			ui.Column(c).Gap(5).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Size(d, d).AlignItems(ui.Center).Justify(ui.Center).
					Draw(func(pt *ui.Painter, r ui.Rect) {
						w := min(r.W, r.H) * st.thick
						ringArc(pt, r.X+r.W/2, r.Y+r.H/2, (min(r.W, r.H)-w)/2, w, it.Raw(), from, to, track, hasTrack, st)
					}).Children(func() {
					if text := ringCenter(st, it); text != "" {
						ui.Text(c, text).FontSize(clampF(d*0.24, 11, 20)).FontWeight(650).NoWrap()
					}
				})
				ui.Text(c, it.Name).FontSize(fsCaption).FontWeight(500).NoWrap()
				if it.Max > 0 {
					ui.Text(c, progressCount(it)).FontSize(fsTiny).TextColor(p.muted).NoWrap()
				}
			})
		}
		if more > 0 {
			ui.Text(c, fmt.Sprintf(tr("另外 %d 项"), more)).FontSize(fsTiny).TextColor(p.muted)
		}
	})
}

// progressConcentricView nests the items' rings in one, the first outermost, as
// Apple's activity rings are, with each item's name and figure beside it in the
// ring's color.
func progressConcentricView(c *ui.Context, p palette, st ringStyle, shown []engine.Progress, more int, d float32) ui.Element {
	return ui.Row(c).AlignItems(ui.Center).Gap(18).Justify(st.align).Children(func() {
		ui.Box(c).Size(d, d).Shrink(0).Draw(func(pt *ui.Painter, r ui.Rect) {
			cx, cy := r.X+r.W/2, r.Y+r.H/2
			big := min(r.W, r.H) / 2
			n := float32(max(len(shown), 1))
			// The rings share the outer part; the middle stays open.
			w := min(min(r.W, r.H)*st.thick, big*0.88/(n*1.2))
			gap := w * 0.2
			for i, it := range shown {
				from, to := ringColors(p, st, it, i)
				track, hasTrack := ringTrack(st, to, p.chip)
				ringArc(pt, cx, cy, big-w/2-float32(i)*(w+gap), w, it.Raw(), from, to, track, hasTrack, st)
			}
		})
		ui.Column(c).Gap(8).MinWidth(0).Children(func() {
			if len(shown) == 0 {
				ui.Text(c, tr("暂无进度")).FontSize(fsSmall).TextColor(p.muted)
			}
			for i, it := range shown {
				_, to := ringColors(p, st, it, i)
				figure := fmt.Sprintf("%.0f%%", math.Round(it.Raw()))
				if it.Max > 0 {
					figure = progressCount(it)
				}
				ui.Column(c).Gap(1).Children(func() {
					ui.Text(c, it.Name).FontSize(fsTiny).FontWeight(650).TextColor(to).NoWrap()
					ui.Text(c, figure).FontSize(fsBody).FontWeight(650).NoWrap()
				})
			}
			if more > 0 {
				ui.Text(c, fmt.Sprintf(tr("另外 %d 项"), more)).FontSize(fsTiny).TextColor(p.muted)
			}
		})
	})
}

// ringArc paints one ring at radius rad and stroke width w: its track, and over
// it an arc from twelve o'clock as long as raw percent of the circle. Past 100
// the ring goes round a second time, with a soft shadow behind the head so that
// it reads over the first lap, as Apple's rings do.
func ringArc(pt *ui.Painter, cx, cy, rad, w float32, raw float64, from, to, track ui.Color, hasTrack bool, st ringStyle) {
	if hasTrack {
		pt.StrokePath((&ui.Path{}).Circle(cx, cy, rad), w, track)
	}
	if raw <= 0 {
		return
	}
	ringLap(pt, cx, cy, rad, w, float32(min(raw, 100)/100), from, to, st, false)
	if raw > 100 {
		ringLap(pt, cx, cy, rad, w, float32(min(raw-100, 100)/100), from, to, st, true)
	}
}

// ringLap paints one lap of an arc, f of the circle; shadow puts a soft dark
// patch behind its head.
func ringLap(pt *ui.Painter, cx, cy, rad, w, f float32, from, to ui.Color, st ringStyle, shadow bool) {
	if f <= 0 {
		return
	}
	cap := ui.RoundCap
	if st.butt {
		cap = ui.ButtCap
	}
	ring := func() *ui.Path { return circleFromTop(cx, cy, rad).Cap(cap) }
	if shadow {
		pt.StrokePath(ring().Trim(max(f-0.06, 0), f), w*1.12, ui.Hex("#000000").Alpha(0.2))
	}
	if f >= 0.999 {
		pt.StrokePath((&ui.Path{}).Circle(cx, cy, rad), w, to)
		return
	}
	stops := []ui.GradientStop{
		{Color: from, At: 0},
		{Color: to, At: f},
		// The start's round cap reaches back past twelve o'clock, to the end of
		// the turn: it takes the start's color, not the arc's last.
		{Color: from, At: 1},
	}
	if st.butt {
		// Flat ends are a filled sector: a stroke of the curve shows seams where
		// its pieces meet, and a sector has none.
		slice := chart.Pie([]float64{float64(f), float64(1 - f)}, 0)[0]
		sector := chart.Sector(cx, cy, rad-w/2, rad+w/2, slice)
		if st.solid {
			pt.FillPath(sector, to)
		} else {
			pt.FillPathConicGradient(sector, ui.ConicGradient{Stops: stops})
		}
		return
	}
	arc := ring().Trim(0, f)
	if st.solid {
		pt.StrokePath(arc, w, to)
		return
	}
	pt.StrokePathConicGradient(arc, w, ui.ConicGradient{Stops: stops})
}

// circleFromTop is a circle that starts at twelve o'clock and goes clockwise, so
// that a trimmed part of it is an arc of a progress ring.
func circleFromTop(cx, cy, r float32) *ui.Path {
	const k = 0.5522847498
	p := &ui.Path{}
	p.MoveTo(cx, cy-r)
	p.CubeTo(cx+r*k, cy-r, cx+r, cy-r*k, cx+r, cy)
	p.CubeTo(cx+r, cy+r*k, cx+r*k, cy+r, cx, cy+r)
	p.CubeTo(cx-r*k, cy+r, cx-r, cy+r*k, cx-r, cy)
	p.CubeTo(cx-r, cy-r*k, cx-r*k, cy-r, cx, cy-r)
	return p
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
