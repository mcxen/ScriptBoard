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

// matrixPalette are vibrant LED segment colors matching classic multi-zone LED
// dot matrices (blue -> green -> cyan -> orange -> violet -> rose).
var matrixPalette = []ui.Color{
	ui.Hex("#3B82F6"), ui.Hex("#22C55E"), ui.Hex("#06B6D4"), ui.Hex("#F97316"), ui.Hex("#A855F7"), ui.Hex("#F43F5E"),
}

// progressBarView draws a standalone progressBar component: solid, gradient,
// neon (glowing neon tube), segmented blocks, or matrix (glowing LED dot matrix).
// It needs no chart wrapper or chart detail controls.
func progressBarView(c *ui.Context, pb *engine.ProgressBar, limit int) ui.Element {
	p := pal(c)
	if pb == nil {
		return ui.Text(c, tr("暂无进度")).FontSize(fsSmall).TextColor(p.muted)
	}
	style := pb.Style
	if style == "" {
		style = "solid"
	}
	if style == "matrix" {
		return matrixProgressView(c, p, pb, limit)
	}
	items := pb.Items
	shown, more := items, 0
	if limit > 0 && len(items) > limit {
		shown, more = items[:limit], len(items)-limit
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

	compactCard := limit > 0
	gap := float32(10)
	if isNeon {
		gap = 12
	}
	if compactCard {
		gap = 7
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
					ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
						ui.Text(c, it.Name).FontSize(fsSmall).TextColor(p.muted).Grow(1).Shrink(1).MinWidth(0).SingleLine().Ellipsis("…")
						if !pb.HideLabel {
							lbl := ui.Text(c, progressLabel(it, pct)).FontSize(fsSmall).FontWeight(600).FontFeatures("tnum").NoWrap().Shrink(0)
							if isNeon && p.dark {
								lbl.TextColor(from)
							}
						}
					})
				}
				padY := float32(0)
				if isNeon || glow > 0 {
					padY = 6
					if compactCard {
						padY = 3
					}
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

type matrixSegment struct {
	color     ui.Color
	startFrac float32
	endFrac   float32
}

// matrixIsStacked reports whether a matrix progressBar should merge its items
// into a single multi-zone LED dot matrix strip (like a segmented quota/usage
// bar) or draw one LED matrix bar per item.
func matrixIsStacked(pb *engine.ProgressBar, items []engine.Progress) bool {
	if pb.Stacked != nil {
		return *pb.Stacked
	}
	if pb.Total > 0 {
		return true
	}
	if len(items) <= 1 {
		return false
	}
	unit := items[0].Unit
	commonMax := items[0].Max
	sum := 0.0
	for _, it := range items {
		if it.Value < 0 {
			return false
		}
		if it.Unit != unit {
			return false
		}
		if it.Max != commonMax {
			return false
		}
		sum += it.Value
	}
	if sum <= 0 {
		return false
	}
	if commonMax > 0 {
		return sum <= commonMax*1.001
	}
	return true
}

// matrixTotal computes the overall capacity for a stacked LED matrix bar.
func matrixTotal(pb *engine.ProgressBar, items []engine.Progress) float64 {
	sum := 0.0
	for _, it := range items {
		if it.Value > 0 {
			sum += it.Value
		}
	}
	if pb.Total > 0 {
		return max(pb.Total, sum)
	}
	if len(items) > 0 && items[0].Max > 0 {
		allSame := true
		for _, it := range items[1:] {
			if it.Max != items[0].Max {
				allSame = false
				break
			}
		}
		if allSame && sum <= items[0].Max*1.001 {
			return items[0].Max
		}
	}
	if sum > 0 && sum <= 100 && (len(items) == 0 || items[0].Max <= 0) {
		unit := ""
		if len(items) > 0 {
			unit = items[0].Unit
		}
		if unit == "" || unit == "%" {
			return 100
		}
	}
	if sum <= 0 {
		return 100
	}
	return sum
}

func matrixTrackStyle(p palette, track string, first ui.Color) (bg ui.Color, hasBg bool, darkBg bool) {
	switch track {
	case "none":
		return ui.Color{}, false, p.dark
	case "neutral":
		return p.chip, true, p.dark
	case "tint":
		return first.Alpha(0.10), true, p.dark
	default: // "dark" or ""
		return ui.Hex("#05070B"), true, true
	}
}

func matrixProgressView(c *ui.Context, p palette, pb *engine.ProgressBar, limit int) ui.Element {
	items := pb.Items
	if len(items) == 0 {
		return ui.Text(c, tr("暂无进度")).FontSize(fsSmall).TextColor(p.muted)
	}
	glow := pb.Glow
	if glow <= 0 {
		glow = 1.0
	} else {
		glow = clampF(glow, 0.2, 1.5)
	}
	animate := pb.Animate == nil || *pb.Animate
	cols := pb.Cols
	if cols > 0 {
		cols = min(max(cols, 12), 80)
	}

	if matrixIsStacked(pb, items) {
		rows := pb.Rows
		if rows <= 0 {
			rows = 7
		}
		rows = min(max(rows, 3), 10)
		barH := pb.Height
		if barH <= 0 {
			barH = float32(rows)*7.2 + 12
		}
		barH = clampF(barH, 28, 120)

		total := matrixTotal(pb, items)
		segs := make([]matrixSegment, 0, len(items))
		cursor := 0.0
		for i, it := range items {
			v := max(it.Value, 0)
			if v <= 0 {
				continue
			}
			col := parseColor(it.Color, matrixPalette[i%len(matrixPalette)])
			start := float32(cursor / total)
			cursor += v
			end := float32(min(cursor/total, 1.0))
			if end > start {
				segs = append(segs, matrixSegment{color: col, startFrac: start, endFrac: end})
			}
		}
		firstCol := matrixPalette[0]
		if len(segs) > 0 {
			firstCol = segs[0].color
		}
		bg, hasBg, darkBg := matrixTrackStyle(p, pb.Track, firstCol)

		shown, more := items, 0
		if limit > 0 && len(items) > limit {
			shown, more = items[:limit], len(items)-limit
		}

		return ui.Column(c).Gap(8).Children(func() {
			ui.Box(c).Height(barH).Draw(func(pt *ui.Painter, r ui.Rect) {
				paintMatrixBar(pt, r, rows, cols, segs, bg, hasBg, darkBg, glow, animate, p.muted)
			})
			if !pb.HideLabel {
				ui.Row(c).Wrap().Gap(12).AlignItems(ui.Center).Children(func() {
					for i, it := range shown {
						col := parseColor(it.Color, matrixPalette[i%len(matrixPalette)])
						valText := trimNum(it.Value) + it.Unit
						if it.Unit == "" && total == 100 && it.Max <= 0 {
							valText = fmt.Sprintf("%.0f%%", math.Round(it.Value))
						}
						ui.Row(c).AlignItems(ui.Center).Gap(5).Children(func() {
							ui.Box(c).Size(7, 7).Radius(3.5).Background(col).Shrink(0)
							if it.Name != "" {
								ui.Text(c, it.Name).FontSize(fsTiny).TextColor(p.muted).NoWrap()
							}
							ui.Text(c, valText).FontSize(fsTiny).FontWeight(600).NoWrap()
						})
					}
					if more > 0 {
						ui.Text(c, fmt.Sprintf(tr("另外 %d 项"), more)).FontSize(fsTiny).TextColor(p.muted)
					}
				})
			}
		})
	}

	// Single-item or per-item matrix progress bars.
	shown, more := items, 0
	if limit > 0 && len(items) > limit {
		shown, more = items[:limit], len(items)-limit
	}
	rows := pb.Rows
	if rows <= 0 {
		if len(shown) == 1 {
			rows = 7
		} else {
			rows = 5
		}
	}
	rows = min(max(rows, 3), 10)
	barH := pb.Height
	if barH <= 0 {
		if len(shown) == 1 {
			barH = float32(rows)*7.2 + 12
		} else {
			barH = float32(rows)*6.4 + 10
		}
	}
	barH = clampF(barH, 28, 120)

	return ui.Column(c).Gap(10).Children(func() {
		for i, it := range shown {
			i, it := i, it
			pct := it.Percent()
			frac := float32(pct / 100)
			col := parseColor(it.Color, matrixPalette[i%len(matrixPalette)])
			bg, hasBg, darkBg := matrixTrackStyle(p, pb.Track, col)
			var segs []matrixSegment
			if frac > 0 {
				segs = []matrixSegment{{color: col, startFrac: 0, endFrac: frac}}
			}
			ui.Column(c).Gap(4).Children(func() {
				if it.Name != "" || !pb.HideLabel {
					ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
						ui.Text(c, it.Name).FontSize(fsSmall).TextColor(p.muted).Grow(1).Shrink(1).MinWidth(0).SingleLine().Ellipsis("…")
						if !pb.HideLabel {
							lbl := ui.Text(c, progressLabel(it, pct)).FontSize(fsSmall).FontWeight(600).FontFeatures("tnum").NoWrap().Shrink(0)
							if p.dark {
								lbl.TextColor(col)
							}
						}
					})
				}
				ui.Box(c).Height(barH).Draw(func(pt *ui.Painter, r ui.Rect) {
					paintMatrixBar(pt, r, rows, cols, segs, bg, hasBg, darkBg, glow, animate, p.muted)
				})
			})
		}
		if more > 0 {
			ui.Text(c, fmt.Sprintf(tr("另外 %d 项"), more)).FontSize(fsTiny).TextColor(p.muted)
		}
	})
}

// matrixDotHash returns a deterministic pseudo-random value in [0, 1) for an
// LED dot coordinate so each dot keeps an organic, stable luminance tier.
func matrixDotHash(col, row int) float64 {
	x := uint64(col*73856093^row*19349663) + 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	x ^= x >> 31
	return float64(x&0xFFFFFF) / float64(0x1000000)
}

func matrixDotPhase(col, row int) float64 {
	return matrixDotHash(col*3+11, row*7+29)
}

// paintMatrixBar draws a luminous LED dot-matrix panel with multi-tier dot
// brightness, white-hot sparkle cores, unlit hardware grid dots, and optional
// subtle breathing + flowing shimmer animation.
func paintMatrixBar(pt *ui.Painter, r ui.Rect, rows, wantCols int, segs []matrixSegment, bg ui.Color, hasBg, darkBg bool, glow float32, animate bool, muted ui.Color) {
	if r.W <= 4 || r.H <= 4 || rows <= 0 {
		return
	}
	const panelRad = float32(8)
	if hasBg {
		pt.Fill(r, bg, panelRad)
		if darkBg {
			pt.StrokePath(roundedRectPath(r, panelRad), 1, ui.Hex("#FFFFFF").Alpha(0.07))
		}
	}

	padX := float32(8)
	padY := float32(6)
	innerW := r.W - padX*2
	innerH := r.H - padY*2
	if innerW <= 4 || innerH <= 4 {
		return
	}

	pitchY := innerH / float32(rows)
	cols := wantCols
	if cols <= 0 {
		cols = int(math.Round(float64(innerW / pitchY)))
		cols = min(max(cols, 10), 72)
	}
	pitchX := innerW / float32(cols)
	pitch := min(pitchX, pitchY)
	dotR := clampF(pitch*0.29, 1.3, 4.5)

	gridW := pitchX * float32(cols)
	gridH := pitchY * float32(rows)
	originX := r.X + (r.W-gridW)/2
	originY := r.Y + (r.H-gridH)/2

	var sec, wavePos float64
	if animate && len(segs) > 0 {
		pt.AnimationFrameRate(30)
		sec = float64(pt.Now().UnixNano()) / 1e9
		wavePos = math.Mod(sec*0.26, 1.50) - 0.22
	}

	unlit := muted.Alpha(0.18)
	if darkBg {
		unlit = ui.Hex("#FFFFFF").Alpha(0.055)
	}
	white := ui.Hex("#FFFFFF")

	for col := 0; col < cols; col++ {
		colFrac := (float32(col) + 0.5) / float32(cols)
		var seg *matrixSegment
		for i := range segs {
			if colFrac >= segs[i].startFrac && colFrac <= segs[i].endFrac {
				seg = &segs[i]
				break
			}
		}
		cx := originX + (float32(col)+0.5)*pitchX
		nx := float64(col) / float64(max(cols-1, 1))

		for row := 0; row < rows; row++ {
			cy := originY + (float32(row)+0.5)*pitchY
			if seg == nil {
				pt.Fill(ui.Rect{X: cx - dotR*0.85, Y: cy - dotR*0.85, W: dotR * 1.7, H: dotR * 1.7}, unlit, dotR*0.85)
				continue
			}

			h := matrixDotHash(col, row)
			var base, hot float64
			switch {
			case h < 0.28:
				base, hot = 0.26, 0
			case h < 0.66:
				base, hot = 0.64, 0
			case h < 0.89:
				base, hot = 0.94, 0.14
			default:
				base, hot = 1.0, 0.82
			}

			breath := 1.0
			shimmer := 0.0
			if animate {
				phase := matrixDotPhase(col, row) * 2 * math.Pi
				speed := 1.3 + 0.9*matrixDotPhase(row+31, col+17)
				breath = 0.86 + 0.14*math.Sin(sec*speed+phase)

				ny := float64(row) / float64(max(rows-1, 1))
				d := nx - ny*0.10 - wavePos
				shimmer = math.Exp(-(d * d) / (2 * 0.065 * 0.065))
			}

			intensity := clampF(float32(base*breath+shimmer*0.30), 0.18, 1.18)
			whiteMix := clampF(float32(hot+shimmer*0.30), 0, 0.92)

			if glow > 0 && intensity > 0.32 {
				rOuter := dotR * (1.85 + 0.35*float32(shimmer))
				aOuter := min(0.15*intensity*glow, 0.32)
				pt.Fill(ui.Rect{X: cx - rOuter, Y: cy - rOuter, W: rOuter * 2, H: rOuter * 2}, seg.color.Alpha(aOuter), rOuter)

				rMid := dotR * (1.35 + 0.15*float32(shimmer))
				aMid := min(0.30*intensity*glow, 0.54)
				pt.Fill(ui.Rect{X: cx - rMid, Y: cy - rMid, W: rMid * 2, H: rMid * 2}, seg.color.Alpha(aMid), rMid)
			}

			dotCol := seg.color.Mix(white, whiteMix*0.82).Alpha(min(intensity, 1.0))
			pt.Fill(ui.Rect{X: cx - dotR, Y: cy - dotR, W: dotR * 2, H: dotR * 2}, dotCol, dotR)

			if whiteMix > 0.36 {
				coreR := dotR * 0.56
				pt.Fill(ui.Rect{X: cx - coreR, Y: cy - coreR, W: coreR * 2, H: coreR * 2}, white.Alpha(min(whiteMix*0.95, 0.96)), coreR)
			}
		}
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
	concentric          bool
	size                float32 // 0 fits the card
	thick               float32 // the stroke's width, as a share of the size
	butt, solid, matrix bool
	track               string // tint, neutral or none
	center              string // percent, value or none
	align               ui.Align
}

func ringStyleOf(ch *engine.Chart) ringStyle {
	var r engine.RingStyle
	if ch != nil && ch.Ring != nil {
		r = *ch.Ring
	}
	st := ringStyle{concentric: r.Layout == "concentric", butt: r.Cap == "butt", solid: r.Fill == "solid", matrix: r.Fill == "matrix",
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
func progressRingView(c *ui.Context, ch *engine.Chart, width float32, limit int, d float32) ui.Element {
	p := pal(c)
	st := ringStyleOf(ch)
	items := ch.Progress
	narrow := width > 0 && width < 210
	if narrow && !st.concentric {
		if limit == 0 || limit > 2 {
			limit = 2
		}
	}
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
		if narrow {
			d = min(d, 68)
		}
		return progressConcentricView(c, p, st, shown, more, d, narrow)
	}
	if st.size > 0 {
		d = st.size
	}
	if narrow {
		d = min(d, 38)
	}
	gap := float32(18)
	if narrow {
		gap = 10
	}
	return ui.Row(c).Wrap().Gap(gap).Justify(st.align).Children(func() {
		if len(items) == 0 {
			ui.Text(c, tr("暂无进度")).FontSize(fsSmall).TextColor(p.muted)
		}
		for i, it := range shown {
			i := i
			from, to := ringColors(p, st, it, i)
			track, hasTrack := ringTrack(st, to, p.chip)
			ui.Column(c).Gap(5).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Size(d, d).AlignItems(ui.Center).Justify(ui.Center).
					Draw(func(pt *ui.Painter, r ui.Rect) {
						cx, cy := r.X+r.W/2, r.Y+r.H/2
						if st.matrix {
							discR := min(r.W, r.H) / 2
							paintMatrixRingHousing(pt, cx, cy, discR)
							w := max(min(r.W, r.H)*max(st.thick, 0.16), 8)
							paintMatrixRingArc(pt, cx, cy, discR-3-w/2, w, it.Raw(), to, i)
							return
						}
						w := min(r.W, r.H) * st.thick
						ringArc(pt, cx, cy, (min(r.W, r.H)-w)/2, w, it.Raw(), from, to, track, hasTrack, st)
					}).Children(func() {
					if text := ringCenter(st, it); text != "" {
						t := ui.Text(c, text).FontSize(clampF(d*0.24, 10, 20)).FontWeight(650).NoWrap()
						if st.matrix {
							t.TextColor(ui.Hex("#F8FAFC"))
						}
					}
				})
				ui.Text(c, it.Name).FontSize(fsCaption).FontWeight(500).SingleLine().Ellipsis("…")
				if it.Max > 0 {
					ui.Text(c, progressCount(it)).FontSize(fsTiny).TextColor(p.muted).SingleLine().Ellipsis("…")
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
func progressConcentricView(c *ui.Context, p palette, st ringStyle, shown []engine.Progress, more int, d float32, narrow bool) ui.Element {
	gap := float32(18)
	if narrow {
		gap = 10
	}
	return ui.Row(c).AlignItems(ui.Center).Gap(gap).Justify(st.align).Children(func() {
		ui.Box(c).Size(d, d).Shrink(0).Draw(func(pt *ui.Painter, r ui.Rect) {
			cx, cy := r.X+r.W/2, r.Y+r.H/2
			big := min(r.W, r.H) / 2
			if st.matrix {
				paintMatrixRingHousing(pt, cx, cy, big)
				big -= 3
			}
			n := float32(max(len(shown), 1))
			// The rings share the outer part; the middle stays open.
			w := min(min(r.W, r.H)*st.thick, big*0.88/(n*1.2))
			gap := w * 0.2
			for i, it := range shown {
				from, to := ringColors(p, st, it, i)
				if st.matrix {
					paintMatrixRingArc(pt, cx, cy, big-w/2-float32(i)*(w+gap), w, it.Raw(), to, i)
					continue
				}
				track, hasTrack := ringTrack(st, to, p.chip)
				ringArc(pt, cx, cy, big-w/2-float32(i)*(w+gap), w, it.Raw(), from, to, track, hasTrack, st)
			}
		})
		ui.Column(c).Gap(6).Grow(1).Shrink(1).MinWidth(0).Children(func() {
			if len(shown) == 0 {
				ui.Text(c, tr("暂无进度")).FontSize(fsSmall).TextColor(p.muted)
			}
			for i, it := range shown {
				_, to := ringColors(p, st, it, i)
				figure := fmt.Sprintf("%.0f%%", math.Round(it.Raw()))
				if it.Max > 0 {
					figure = progressCount(it)
				}
				ui.Column(c).Gap(1).MinWidth(0).Children(func() {
					ui.Text(c, it.Name).FontSize(fsTiny).FontWeight(650).TextColor(to).SingleLine().Ellipsis("…")
					ui.Text(c, figure).FontSize(fsBody).FontWeight(650).SingleLine().Ellipsis("…")
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
