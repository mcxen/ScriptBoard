package main

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"scriptboard/src/internal/engine"
)

// progressView draws a "progress" chart: one bar per item, each with its name,
// its value out of its maximum, and how full it is.
func progressView(c *ui.Context, items []engine.Progress, limit int) ui.Element {
	return progressBarView(c, &engine.ProgressBar{Items: items}, limit)
}

// progressBarOf extracts a ProgressBar from res: either its standalone
// res.ProgressBar field (preferred: no Chart object needed) or a Chart of type
// "progress" / "progressBar".
func progressBarOf(res engine.Result) (*engine.ProgressBar, bool) {
	if res.ProgressBar != nil {
		return res.ProgressBar, true
	}
	if res.Chart != nil && (res.Chart.Type == "progress" || res.Chart.Type == "progressBar") {
		pb := &engine.ProgressBar{Items: res.Chart.Progress}
		if res.Chart.Bar != nil {
			pb.BarStyle = *res.Chart.Bar
		}
		return pb, true
	}
	return nil, false
}

// neonPalette are vivid colors suited for neon progress bars when an item
// leaves its color unset.
var neonPalette = []ui.Color{
	ui.Hex("#00F5D4"), ui.Hex("#38BDF8"), ui.Hex("#FF2A85"), ui.Hex("#A855F7"), ui.Hex("#22C55E"), ui.Hex("#FB923C"),
}

// progressBarView draws a standalone progressBar component: solid, gradient,
// neon (glowing neon tube) or segmented blocks. It needs no chart wrapper or
// chart detail controls.
func progressBarView(c *ui.Context, pb *engine.ProgressBar, limit int) ui.Element {
	p := pal(c)
	if pb == nil {
		return ui.Text(c, tr("暂无进度")).FontSize(fsSmall).TextColor(p.muted)
	}
	items := pb.Items
	shown, more := items, 0
	if limit > 0 && len(items) > limit {
		shown, more = items[:limit], len(items)-limit
	}
	style := pb.Style
	if style == "" {
		style = "solid"
	}
	isNeon := style == "neon"
	barH := pb.Height
	if barH <= 0 {
		barH = 8
		if isNeon {
			barH = 10
		}
	}
	barH = clampF(barH, 4, 24)
	glow := pb.Glow
	if glow <= 0 {
		if isNeon {
			glow = 1.0
		}
	} else {
		glow = clampF(glow, 0.2, 1.5)
	}
	segs := pb.Segments
	if segs <= 0 {
		segs = 10
	}
	segs = min(max(segs, 4), 30)

	gap := float32(10)
	if isNeon {
		gap = 12
	}
	return ui.Column(c).Gap(gap).Children(func() {
		if len(items) == 0 {
			ui.Text(c, tr("暂无进度")).FontSize(fsSmall).TextColor(p.muted)
		}
		for i, it := range shown {
			i, it := i, it
			pct := it.Percent()
			frac := float32(pct / 100)
			base := p.accent
			if isNeon {
				base = neonPalette[i%len(neonPalette)]
			}
			from := parseColor(it.Color, base)
			to := from
			if hexColor.MatchString(it.To) {
				to = ui.Hex(it.To)
			} else if isNeon || style == "gradient" {
				to = from.Mix(ui.Hex("#FFFFFF"), 0.28)
			}
			trackColor, hasTrack := barTrackColor(p, pb.Track, from, isNeon)

			ui.Column(c).Gap(4).Children(func() {
				if it.Name != "" || !pb.HideLabel {
					ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
						ui.Text(c, it.Name).FontSize(fsSmall).TextColor(p.muted).Grow(1).MinWidth(0).NoWrap()
						if !pb.HideLabel {
							lbl := ui.Text(c, progressLabel(it, pct)).FontSize(fsSmall).FontWeight(600).NoWrap()
							if isNeon && p.dark {
								lbl.TextColor(from)
							}
						}
					})
				}
				padY := float32(0)
				if isNeon || glow > 0 {
					padY = 6
				}
				ui.Box(c).Height(barH + padY*2).Draw(func(pt *ui.Painter, r ui.Rect) {
					track := ui.Rect{X: r.X, Y: r.Y + padY, W: r.W, H: barH}
					rad := barH / 2
					switch style {
					case "segmented":
						paintSegmentedBar(pt, track, segs, frac, from, to, trackColor, hasTrack, glow)
					case "neon":
						paintNeonBar(pt, track, rad, frac, from, to, trackColor, hasTrack, glow)
					case "gradient":
						paintGradientBar(pt, track, rad, frac, from, to, trackColor, hasTrack)
					default:
						paintSolidBar(pt, track, rad, frac, from, trackColor, hasTrack)
					}
				})
			})
		}
		if more > 0 {
			ui.Text(c, fmt.Sprintf(tr("另外 %d 项"), more)).FontSize(fsTiny).TextColor(p.muted)
		}
	})
}

func barTrackColor(p palette, track string, accent ui.Color, neon bool) (ui.Color, bool) {
	switch track {
	case "none":
		return ui.Color{}, false
	case "dark":
		return ui.Hex("#0F172A").Alpha(0.9), true
	case "tint":
		return accent.Alpha(0.16), true
	case "neutral":
		return p.chip, true
	}
	if neon {
		if p.dark {
			return ui.Hex("#0B1120").Alpha(0.95), true
		}
		return ui.Hex("#0F172A").Alpha(0.86), true
	}
	return p.chip, true
}

func roundedRectPath(r ui.Rect, rad float32) *ui.Path {
	rad = min(rad, min(r.W, r.H)/2)
	p := &ui.Path{}
	if r.W <= 0 || r.H <= 0 {
		return p
	}
	const k = 0.5522847498
	x0, y0, x1, y1 := r.X, r.Y, r.X+r.W, r.Y+r.H
	p.MoveTo(x0+rad, y0)
	p.LineTo(x1-rad, y0)
	p.CubeTo(x1-rad*(1-k), y0, x1, y0+rad*(1-k), x1, y0+rad)
	p.LineTo(x1, y1-rad)
	p.CubeTo(x1, y1-rad*(1-k), x1-rad*(1-k), y1, x1-rad, y1)
	p.LineTo(x0+rad, y1)
	p.CubeTo(x0+rad*(1-k), y1, x0, y1-rad*(1-k), x0, y1-rad)
	p.LineTo(x0, y0+rad)
	p.CubeTo(x0, y0+rad*(1-k), x0+rad*(1-k), y0, x0+rad, y0)
	p.Close()
	return p
}

func paintSolidBar(pt *ui.Painter, track ui.Rect, rad, frac float32, color, trackCol ui.Color, hasTrack bool) {
	if hasTrack {
		pt.Fill(track, trackCol, rad)
	}
	if frac <= 0 {
		return
	}
	w := max(track.W*frac, min(track.H, track.W))
	pt.Fill(ui.Rect{X: track.X, Y: track.Y, W: w, H: track.H}, color, rad)
}

func paintGradientBar(pt *ui.Painter, track ui.Rect, rad, frac float32, from, to, trackCol ui.Color, hasTrack bool) {
	if hasTrack {
		pt.Fill(track, trackCol, rad)
	}
	if frac <= 0 {
		return
	}
	w := max(track.W*frac, min(track.H, track.W))
	bar := ui.Rect{X: track.X, Y: track.Y, W: w, H: track.H}
	pt.FillPathGradient(roundedRectPath(bar, rad), ui.LinearGradient{From: from, To: to, Angle: 90})
}

// paintNeonBar draws a luminous neon progress tube: a deep housing track,
// multi-layered outer neon halo, gradient tube body, bright inner core
// filament, and a leading hotspot spark.
func paintNeonBar(pt *ui.Painter, track ui.Rect, rad, frac float32, from, to, trackCol ui.Color, hasTrack bool, glow float32) {
	if hasTrack {
		pt.Fill(track, trackCol, rad)
		pt.StrokePath(roundedRectPath(track, rad), 1, from.Alpha(0.24))
	}
	if frac <= 0 {
		return
	}
	w := max(track.W*frac, min(track.H, track.W))
	bar := ui.Rect{X: track.X, Y: track.Y, W: w, H: track.H}

	// Concentric outer neon glow layers.
	layers := [4][2]float32{
		{5.5, 0.08},
		{3.6, 0.16},
		{2.0, 0.28},
		{0.9, 0.44},
	}
	glowColor := from.Mix(to, 0.5)
	for _, l := range layers {
		spread := l[0] * glow
		alpha := min(l[1]*glow, 0.72)
		halo := ui.Rect{
			X: bar.X - spread*0.6,
			Y: bar.Y - spread,
			W: bar.W + spread*1.2,
			H: bar.H + spread*2,
		}
		pt.Fill(halo, glowColor.Alpha(alpha), rad+spread)
	}

	// Vibrant neon tube body.
	pt.FillPathGradient(roundedRectPath(bar, rad), ui.LinearGradient{From: from, To: to, Angle: 90})

	// Incandescent white-hot inner filament along the tube center.
	coreH := clampF(bar.H*0.28, 1.5, 4)
	coreInset := min(rad*0.45, bar.W*0.2)
	if bar.W > coreInset*2+2 {
		core := ui.Rect{
			X: bar.X + coreInset,
			Y: bar.Y + (bar.H-coreH)/2,
			W: bar.W - coreInset*2,
			H: coreH,
		}
		pt.Fill(core, ui.Hex("#FFFFFF").Alpha(0.72), coreH/2)
	}

	// Leading tip flare when in progress.
	if frac > 0.02 && frac < 0.995 {
		tipX := bar.X + bar.W - rad*0.5
		tipY := bar.Y + bar.H/2
		tipR := rad * 0.9
		pt.Fill(ui.Rect{X: tipX - tipR*1.6, Y: tipY - tipR*1.6, W: tipR * 3.2, H: tipR * 3.2}, to.Alpha(0.45*glow), tipR*1.6)
		pt.Fill(ui.Rect{X: tipX - tipR*0.55, Y: tipY - tipR*0.55, W: tipR * 1.1, H: tipR * 1.1}, ui.Hex("#FFFFFF").Alpha(0.92), tipR*0.55)
	}
}

func paintSegmentedBar(pt *ui.Painter, track ui.Rect, segs int, frac float32, from, to, trackCol ui.Color, hasTrack bool, glow float32) {
	gap := float32(3)
	if segs > 18 {
		gap = 2
	}
	w := (track.W - gap*float32(segs-1)) / float32(segs)
	if w < 2 {
		w = 2
	}
	rad := min(track.H*0.35, w*0.35)
	lit := frac * float32(segs)
	for i := 0; i < segs; i++ {
		cell := ui.Rect{X: track.X + float32(i)*(w+gap), Y: track.Y, W: w, H: track.H}
		if hasTrack {
			pt.Fill(cell, trackCol, rad)
		}
		fill := clampF(lit-float32(i), 0, 1)
		if fill <= 0 {
			continue
		}
		t := float32(i) / float32(max(segs-1, 1))
		col := from.Mix(to, t)
		if glow > 0 && fill > 0.2 {
			sp := 2.5 * glow
			pt.Fill(ui.Rect{X: cell.X - sp*0.5, Y: cell.Y - sp, W: cell.W + sp, H: cell.H + sp*2}, col.Alpha(0.28*fill*glow), rad+sp)
		}
		pt.Fill(cell, col.Alpha(0.25+0.75*fill), rad)
	}
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

// isProgress says whether a chart is a completion component, as bars or rings.
func isProgress(ch *engine.Chart) bool {
	return ch != nil && (ch.Type == "progress" || ch.Type == "progressBar" || ch.Type == "progressRing")
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
