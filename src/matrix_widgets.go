package main

import (
	"fmt"
	"math"
	"time"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
)

// Shared LED dot-matrix rendering primitives for the Matrix LED component family:
// donut (ring proportion), gauge (quota dial), progressRing (completion rings),
// contrib (7-row activity matrix), and clock (5x7 pixel LED clock).

const (
	matrixPanelHex = "#05070B"
)

// paintMatrixLED paints a single lit circular LED dot with double soft outer
// glow halo, color-mixed body, and optional white-hot specular core.
func paintMatrixLED(pt *ui.Painter, cx, cy, dotR float32, col ui.Color, intensity, whiteMix, glow, shimmer float32) {
	white := ui.Hex("#FFFFFF")
	if glow > 0 && intensity > 0.30 {
		rOuter := dotR * (1.85 + 0.32*shimmer)
		aOuter := min(0.15*intensity*glow, 0.32)
		pt.Fill(ui.Rect{X: cx - rOuter, Y: cy - rOuter, W: rOuter * 2, H: rOuter * 2}, col.Alpha(aOuter), rOuter)

		rMid := dotR * (1.35 + 0.15*shimmer)
		aMid := min(0.30*intensity*glow, 0.54)
		pt.Fill(ui.Rect{X: cx - rMid, Y: cy - rMid, W: rMid * 2, H: rMid * 2}, col.Alpha(aMid), rMid)
	}
	dotCol := col.Mix(white, whiteMix*0.82).Alpha(min(intensity, 1.0))
	pt.Fill(ui.Rect{X: cx - dotR, Y: cy - dotR, W: dotR * 2, H: dotR * 2}, dotCol, dotR)
	if whiteMix > 0.36 {
		coreR := dotR * 0.56
		pt.Fill(ui.Rect{X: cx - coreR, Y: cy - coreR, W: coreR * 2, H: coreR * 2}, white.Alpha(min(whiteMix*0.95, 0.96)), coreR)
	}
}

func paintMatrixUnlit(pt *ui.Painter, cx, cy, dotR float32, alpha float32) {
	r := dotR * 0.85
	pt.Fill(ui.Rect{X: cx - r, Y: cy - r, W: r * 2, H: r * 2}, ui.Hex("#FFFFFF").Alpha(alpha), r)
}

func matrixTier(col, row int) (base, hot float64) {
	h := matrixDotHash(col, row)
	switch {
	case h < 0.28:
		return 0.28, 0
	case h < 0.66:
		return 0.66, 0
	case h < 0.89:
		return 0.94, 0.14
	default:
		return 1.0, 0.82
	}
}

func angularDist(a, b float64) float64 {
	d := math.Mod(a-b+3*math.Pi, 2*math.Pi) - math.Pi
	return math.Abs(d)
}

// ---------------------------------------------------------------------------
// 1. Matrix Donut (点阵环形占比)
// ---------------------------------------------------------------------------

func paintMatrixDonut(pt *ui.Painter, r ui.Rect, vals []float64, cols []ui.Color, thick float32, activeIdx int) {
	if r.W <= 12 || r.H <= 12 || len(vals) == 0 {
		return
	}
	pt.AnimationFrameRate(30)
	sec := float64(pt.Now().UnixNano()) / 1e9
	sweepAngle := math.Mod(sec*1.05, 2*math.Pi)

	cx, cy := r.X+r.W/2, r.Y+r.H/2
	discR := min(r.W, r.H) / 2
	// Obsidian circular housing with subtle rim.
	pt.Fill(ui.Rect{X: cx - discR, Y: cy - discR, W: discR * 2, H: discR * 2}, ui.Hex(matrixPanelHex), discR)
	pt.StrokePath((&ui.Path{}).Circle(cx, cy, discR-0.5), 1, ui.Hex("#FFFFFF").Alpha(0.08))

	outer := discR - 5
	band := clampF(outer*max(thick, 0.30), 16, outer*0.52)
	inner := outer - band

	rings := 4
	if band < 24 {
		rings = 3
	}
	radialStep := band / float32(rings)
	dotR := clampF(radialStep*0.30, 1.35, 3.6)

	// Cumulative angular spans starting at 0 (12 o'clock) clockwise to 2*pi.
	total := 0.0
	for _, v := range vals {
		if v > 0 {
			total += v
		}
	}
	if total <= 0 {
		return
	}
	type sliceSpan struct {
		start, end float64
		col        ui.Color
		idx        int
	}
	spans := make([]sliceSpan, 0, len(vals))
	cur := 0.0
	for i, v := range vals {
		if v <= 0 {
			continue
		}
		span := (v / total) * 2 * math.Pi
		spans = append(spans, sliceSpan{start: cur, end: cur + span, col: cols[i], idx: i})
		cur += span
	}

	gapAng := 0.035
	if len(spans) <= 1 {
		gapAng = 0
	}

	for ring := 0; ring < rings; ring++ {
		rad := inner + (float32(ring)+0.5)*radialStep
		circ := 2 * math.Pi * float64(rad)
		nDots := int(math.Round(circ / float64(radialStep*1.05)))
		nDots = min(max(nDots, 18), 96)

		for k := 0; k < nDots; k++ {
			frac := float64(k) / float64(nDots)
			theta := frac * 2 * math.Pi // 0 = 12 o'clock, clockwise
			drawAng := theta - math.Pi/2
			dx := cx + rad*float32(math.Cos(drawAng))
			dy := cy + rad*float32(math.Sin(drawAng))

			var hit *sliceSpan
			for i := range spans {
				s := &spans[i]
				pad := min(gapAng/2, (s.end-s.start)*0.15)
				if theta >= s.start+pad && theta <= s.end-pad {
					hit = s
					break
				}
			}
			if hit == nil {
				paintMatrixUnlit(pt, dx, dy, dotR, 0.06)
				continue
			}

			base, hot := matrixTier(k, ring)
			phase := matrixDotPhase(k, ring) * 2 * math.Pi
			speed := 1.3 + 0.8*matrixDotPhase(ring+19, k+7)
			breath := 0.87 + 0.13*math.Sin(sec*speed+phase)

			dAng := angularDist(theta, sweepAngle)
			shimmer := math.Exp(-(dAng * dAng) / (2 * 0.38 * 0.38))

			dimMul := 1.0
			glow := float32(1.0)
			if activeIdx >= 0 {
				if hit.idx == activeIdx {
					dimMul = 1.15
					glow = 1.25
				} else {
					dimMul = 0.28
					glow = 0.35
					shimmer *= 0.25
				}
			}

			intensity := clampF(float32((base*breath+shimmer*0.30)*dimMul), 0.16, 1.18)
			whiteMix := clampF(float32((hot+shimmer*0.30)*min(dimMul, 1.0)), 0, 0.92)
			paintMatrixLED(pt, dx, dy, dotR, hit.col, intensity, whiteMix, glow, float32(shimmer))
		}
	}
}

// ---------------------------------------------------------------------------
// 2. Matrix Gauge (点阵额度仪表盘)
// ---------------------------------------------------------------------------

func paintMatrixGauge(pt *ui.Painter, r ui.Rect, frac float64, col ui.Color) {
	if r.W <= 12 || r.H <= 12 {
		return
	}
	pt.AnimationFrameRate(30)
	sec := float64(pt.Now().UnixNano()) / 1e9

	cx, cy := r.X+r.W/2, r.Y+r.H/2
	discR := min(r.W, r.H) / 2
	pt.Fill(ui.Rect{X: cx - discR, Y: cy - discR, W: discR * 2, H: discR * 2}, ui.Hex(matrixPanelHex), discR)
	pt.StrokePath((&ui.Path{}).Circle(cx, cy, discR-0.5), 1, ui.Hex("#FFFFFF").Alpha(0.08))

	outer := discR - 5
	band := clampF(outer*0.34, 14, outer*0.46)
	inner := outer - band
	rings := 4
	if band < 22 {
		rings = 3
	}
	radialStep := band / float32(rings)
	dotR := clampF(radialStep*0.30, 1.3, 3.4)

	// 270-degree dial sweep: starts at 135° (bottom-left) clockwise to 405° (bottom-right).
	const startDeg = 135.0
	const sweepDeg = 270.0
	wavePos := math.Mod(sec*0.30, 1.45) - 0.20

	clampedFrac := math.Max(0, math.Min(1, frac))

	for ring := 0; ring < rings; ring++ {
		rad := inner + (float32(ring)+0.5)*radialStep
		arcLen := (sweepDeg / 360.0) * 2 * math.Pi * float64(rad)
		nDots := int(math.Round(arcLen / float64(radialStep*1.08)))
		nDots = min(max(nDots, 16), 72)

		for k := 0; k < nDots; k++ {
			t := float64(k) / float64(max(nDots-1, 1))
			deg := startDeg + t*sweepDeg
			radAng := deg * math.Pi / 180.0
			dx := cx + rad*float32(math.Cos(radAng))
			dy := cy + rad*float32(math.Sin(radAng))

			if t > clampedFrac {
				paintMatrixUnlit(pt, dx, dy, dotR, 0.065)
				continue
			}

			base, hot := matrixTier(k, ring)
			phase := matrixDotPhase(k, ring) * 2 * math.Pi
			speed := 1.4 + 0.8*matrixDotPhase(ring+23, k+13)
			breath := 0.87 + 0.13*math.Sin(sec*speed+phase)

			d := t - wavePos
			shimmer := math.Exp(-(d * d) / (2 * 0.08 * 0.08))

			intensity := clampF(float32(base*breath+shimmer*0.32), 0.18, 1.18)
			whiteMix := clampF(float32(hot+shimmer*0.32), 0, 0.92)
			paintMatrixLED(pt, dx, dy, dotR, col, intensity, whiteMix, 1.05, float32(shimmer))
		}
	}
}

// ---------------------------------------------------------------------------
// 3. Matrix Progress Ring (点阵完成度圆环)
// ---------------------------------------------------------------------------

func paintMatrixRingHousing(pt *ui.Painter, cx, cy, discR float32) {
	pt.Fill(ui.Rect{X: cx - discR, Y: cy - discR, W: discR * 2, H: discR * 2}, ui.Hex(matrixPanelHex), discR)
	pt.StrokePath((&ui.Path{}).Circle(cx, cy, discR-0.5), 1, ui.Hex("#FFFFFF").Alpha(0.08))
}

func paintMatrixRingArc(pt *ui.Painter, cx, cy, rad, w float32, raw float64, col ui.Color, ringIdx int) {
	if rad <= 4 || w <= 2 {
		return
	}
	pt.AnimationFrameRate(30)
	sec := float64(pt.Now().UnixNano()) / 1e9
	sweepAngle := math.Mod(sec*1.15+float64(ringIdx)*0.8, 2*math.Pi)

	subRings := 2
	if w >= 12 {
		subRings = 3
	} else if w < 6 {
		subRings = 1
	}
	stepR := w / float32(subRings)
	dotR := clampF(stepR*0.32, 1.15, 3.0)

	f := math.Max(0, math.Min(1, raw/100))
	secondLap := math.Max(0, math.Min(1, (raw-100)/100))

	for sr := 0; sr < subRings; sr++ {
		rCur := rad - w/2 + (float32(sr)+0.5)*stepR
		if rCur <= 2 {
			continue
		}
		circ := 2 * math.Pi * float64(rCur)
		nDots := int(math.Round(circ / float64(max(stepR*1.12, 4.2))))
		nDots = min(max(nDots, 16), 84)

		for k := 0; k < nDots; k++ {
			t := float64(k) / float64(nDots)
			theta := t * 2 * math.Pi
			drawAng := theta - math.Pi/2
			dx := cx + rCur*float32(math.Cos(drawAng))
			dy := cy + rCur*float32(math.Sin(drawAng))

			if t > f || raw <= 0 {
				paintMatrixUnlit(pt, dx, dy, dotR, 0.065)
				continue
			}

			base, hot := matrixTier(k+ringIdx*17, sr+ringIdx*5)
			if secondLap > 0 && t <= secondLap {
				base = min(base+0.25, 1.0)
				hot = min(hot+0.35, 0.90)
			}
			phase := matrixDotPhase(k+ringIdx*7, sr) * 2 * math.Pi
			speed := 1.4 + 0.8*matrixDotPhase(sr+13, k+ringIdx)
			breath := 0.87 + 0.13*math.Sin(sec*speed+phase)

			dAng := angularDist(theta, sweepAngle)
			shimmer := math.Exp(-(dAng * dAng) / (2 * 0.42 * 0.42))

			intensity := clampF(float32(base*breath+shimmer*0.30), 0.20, 1.18)
			whiteMix := clampF(float32(hot+shimmer*0.30), 0, 0.92)
			paintMatrixLED(pt, dx, dy, dotR, col, intensity, whiteMix, 1.05, float32(shimmer))
		}
	}
}

// ---------------------------------------------------------------------------
// 4. Matrix Contrib (点阵打卡热力矩阵)
// ---------------------------------------------------------------------------

func matrixContribView(c *ui.Context, spec *engine.ContribSpec, sel *int, full bool) ui.Element {
	p := pal(c)
	if spec == nil || len(spec.Cells) == 0 {
		return ui.Text(c, tr("暂无打卡记录")).FontSize(fsSmall).TextColor(p.muted)
	}
	customCol := hexColor.MatchString(spec.Color)
	baseCol := adaptAccentColor(parseColor(spec.Color, ui.Hex("#22C55E")), true)

	cells := spec.Cells
	maxWeeks := spec.Weeks
	if maxWeeks <= 0 {
		maxWeeks = (len(cells) + 6) / 7
	}
	if !full && maxWeeks > 20 {
		maxWeeks = 20
	}
	if maxWeeks > 52 {
		maxWeeks = 52
	}
	need := maxWeeks * 7
	if len(cells) > need {
		cells = cells[len(cells)-need:]
	}
	hi := spec.Max
	if hi <= 0 {
		for _, cell := range cells {
			if cell.Value > hi {
				hi = cell.Value
			}
		}
	}
	if hi <= 0 {
		hi = 1
	}

	cols := (len(cells) + 6) / 7
	chosen := -1
	if sel != nil {
		chosen = *sel
	}

	panelH := float32(84)
	if full {
		panelH = 128
	}

	return ui.Column(c).Gap(10).Children(func() {
		box := ui.Box(c).Height(panelH)
		if full && sel != nil {
			box = box.Cursor(ui.CursorPointer)
		}
		el := box.Draw(func(pt *ui.Painter, r ui.Rect) {
			if r.W <= 8 || r.H <= 8 || cols <= 0 {
				return
			}
			const panelRad = float32(10)
			pt.Fill(r, ui.Hex(matrixPanelHex), panelRad)
			pt.StrokePath(roundedRectPath(r, panelRad), 1, ui.Hex("#FFFFFF").Alpha(0.07))

			pt.AnimationFrameRate(30)
			sec := float64(pt.Now().UnixNano()) / 1e9
			wavePos := math.Mod(sec*0.24, 1.50) - 0.22

			padX, padY := float32(10), float32(8)
			innerW := r.W - padX*2
			innerH := r.H - padY*2
			if innerW <= 4 || innerH <= 4 {
				return
			}
			stepY := innerH / 7
			stepX := innerW / float32(cols)
			pitch := min(stepX, stepY)
			dotR := clampF(pitch*0.32, 1.6, 5.2)

			gridW := stepX * float32(cols)
			gridH := stepY * 7
			originX := r.X + (r.W-gridW)/2
			originY := r.Y + (r.H-gridH)/2

			for col := 0; col < cols; col++ {
				cx := originX + (float32(col)+0.5)*stepX
				nx := float64(col) / float64(max(cols-1, 1))
				// When no explicit color is specified, tint columns by 4-week zone like the multi-color matrix.
				zoneCol := baseCol
				if !customCol && cols >= 8 {
					zoneCol = matrixPalette[(col/4)%len(matrixPalette)]
				}
				for row := 0; row < 7; row++ {
					cy := originY + (float32(row)+0.5)*stepY
					idx := col*7 + row
					if idx >= len(cells) || cells[idx].Value <= 0 {
						paintMatrixUnlit(pt, cx, cy, dotR, 0.06)
						continue
					}
					frac := math.Max(0, math.Min(1, cells[idx].Value/hi))
					var base, hot float64
					switch {
					case frac <= 0.25:
						base, hot = 0.34, 0
					case frac <= 0.55:
						base, hot = 0.66, 0
					case frac <= 0.85:
						base, hot = 0.94, 0.16
					default:
						base, hot = 1.0, 0.78
					}
					phase := matrixDotPhase(col, row) * 2 * math.Pi
					speed := 1.2 + 0.8*matrixDotPhase(row+29, col+13)
					breath := 0.88 + 0.12*math.Sin(sec*speed+phase)

					ny := float64(row) / 6.0
					d := nx - ny*0.10 - wavePos
					shimmer := math.Exp(-(d * d) / (2 * 0.065 * 0.065))

					intensity := clampF(float32(base*breath+shimmer*0.26), 0.22, 1.18)
					whiteMix := clampF(float32(hot+shimmer*0.26), 0, 0.92)
					paintMatrixLED(pt, cx, cy, dotR, zoneCol, intensity, whiteMix, 1.05, float32(shimmer))

					if idx == chosen {
						pt.StrokePath((&ui.Path{}).Circle(cx, cy, dotR+2.2), 1.5, ui.Hex("#FFFFFF").Alpha(0.9))
					}
				}
			}
		})
		if full && sel != nil && el.Clicked() {
			b := el.Bounds()
			px, py, ok := el.PointerPosition()
			padX, padY := float32(10), float32(8)
			innerW := b.W - padX*2
			innerH := b.H - padY*2
			if ok && innerW > 4 && innerH > 4 {
				col := int((px - padX) / (innerW / float32(cols)))
				row := int((py - padY) / (innerH / 7))
				if col >= 0 && col < cols && row >= 0 && row < 7 {
					idx := col*7 + row
					if idx < len(cells) {
						if *sel == idx {
							*sel = -1
						} else {
							*sel = idx
						}
					}
				}
			}
		}

		// Legend & selected-day readout.
		ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
			if chosen >= 0 && chosen < len(cells) {
				ce := cells[chosen]
				lbl := ce.Label
				if lbl == "" {
					lbl = ce.Date
				}
				valText := tr("未记录")
				if ce.Value > 0 {
					valText = trimNum(ce.Value) + spec.Unit
				}
				ui.Text(c, fmt.Sprintf("%s · %s", lbl, valText)).FontSize(fsSmall).FontWeight(600).Grow(1).MinWidth(0).NoWrap()
			} else {
				active := 0
				for _, ce := range cells {
					if ce.Value > 0 {
						active++
					}
				}
				ui.Text(c, fmt.Sprintf(tr("活跃 %d / %d 天"), active, len(cells))).FontSize(fsTiny).TextColor(p.muted).Grow(1).MinWidth(0).NoWrap()
			}
			ui.Text(c, tr("少")).FontSize(fsTiny).TextColor(p.faint)
			for _, a := range []float32{0.18, 0.42, 0.72, 1.0} {
				ui.Box(c).Size(8, 8).Radius(4).Background(baseCol.Alpha(a))
			}
			ui.Text(c, tr("多")).FontSize(fsTiny).TextColor(p.faint)
		})
	})
}

// ---------------------------------------------------------------------------
// 5. Matrix Pixel Clock (5x7 LED 点阵数字时钟)
// ---------------------------------------------------------------------------

// matrixFont5x7 defines crisp 5-column x 7-row LED dot bitmaps for digits & colon.
var matrixFont5x7 = map[rune][7]uint8{
	'0': {0b01110, 0b10001, 0b10011, 0b10101, 0b11001, 0b10001, 0b01110},
	'1': {0b00100, 0b01100, 0b00100, 0b00100, 0b00100, 0b00100, 0b01110},
	'2': {0b01110, 0b10001, 0b00001, 0b00110, 0b01000, 0b10000, 0b11111},
	'3': {0b01110, 0b10001, 0b00001, 0b00110, 0b00001, 0b10001, 0b01110},
	'4': {0b00010, 0b00110, 0b01010, 0b10010, 0b11111, 0b00010, 0b00010},
	'5': {0b11111, 0b10000, 0b11110, 0b00001, 0b00001, 0b10001, 0b01110},
	'6': {0b00110, 0b01000, 0b10000, 0b11110, 0b10001, 0b10001, 0b01110},
	'7': {0b11111, 0b00001, 0b00010, 0b00100, 0b01000, 0b01000, 0b01000},
	'8': {0b01110, 0b10001, 0b10001, 0b01110, 0b10001, 0b10001, 0b01110},
	'9': {0b01110, 0b10001, 0b10001, 0b01111, 0b00001, 0b00010, 0b01100},
	':': {0b00000, 0b00100, 0b00100, 0b00000, 0b00100, 0b00100, 0b00000},
}

func matrixClockView(c *ui.Context, st engine.ClockSpec, now time.Time, width, h float32, full bool) ui.Element {
	p := pal(c)
	col := adaptAccentColor(parseColor(st.Color, ui.Hex("#38BDF8")), true)
	if !hexColor.MatchString(st.Color) && st.Neon {
		col = ui.Hex("#00F5D4")
	}
	timeFmt := "15:04"
	if st.Seconds {
		timeFmt = "15:04:05"
	}
	text := now.Format(timeFmt)

	// Build bitmap columns: 1 empty border column on each side, 5 cols per digit, 3 cols per colon, 1 col gap.
	var gridCols []uint8 // each uint8 holds 7 row bits (bit 0 = top row)
	var isColon []bool
	gridCols = append(gridCols, 0)
	isColon = append(isColon, false)
	for ci, ch := range text {
		if ci > 0 {
			gridCols = append(gridCols, 0)
			isColon = append(isColon, false)
		}
		bmp := matrixFont5x7[ch]
		if ch == ':' {
			var mask uint8
			for row := 0; row < 7; row++ {
				if (bmp[row] & 0b00100) != 0 {
					mask |= 1 << row
				}
			}
			gridCols = append(gridCols, mask)
			isColon = append(isColon, true)
			continue
		}
		for bit := 4; bit >= 0; bit-- {
			var mask uint8
			for row := 0; row < 7; row++ {
				if (bmp[row] & (1 << bit)) != 0 {
					mask |= 1 << row
				}
			}
			gridCols = append(gridCols, mask)
			isColon = append(isColon, false)
		}
	}
	gridCols = append(gridCols, 0)
	isColon = append(isColon, false)

	panelH := clampF(h*0.58, 72, 128)
	if full {
		panelH = 128
	}
	// Match the panel aspect ratio to the 5x7 dot-matrix text width so the clock
	// panel stays centered with crisp square LED spacing.
	const totalRows = 9 // 1 unlit top + 7 digit rows + 1 unlit bottom
	idealPitch := (panelH - 20) / float32(totalRows)
	panelW := clampF(float32(len(gridCols))*idealPitch+24, 180, 540)
	if !full && width > 0 && panelW > width {
		panelW = max(width, 96)
		panelH = clampF((panelW-24)/float32(max(len(gridCols), 1))*float32(totalRows)+20, 48, panelH)
	}
	padY := float32(0)
	if full {
		padY = 18
	}

	return ui.Column(c).Grow(1).Padding(padY, 0).Gap(10).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		ui.Box(c).Size(panelW, panelH).Draw(func(pt *ui.Painter, r ui.Rect) {
			if r.W <= 12 || r.H <= 12 {
				return
			}
			const panelRad = float32(10)
			pt.Fill(r, ui.Hex(matrixPanelHex), panelRad)
			pt.StrokePath(roundedRectPath(r, panelRad), 1, ui.Hex("#FFFFFF").Alpha(0.08))

			pt.AnimationFrameRate(30)
			sec := float64(pt.Now().UnixNano()) / 1e9
			wavePos := math.Mod(sec*0.28, 1.55) - 0.25
			colonPulse := 0.52 + 0.48*math.Sin(sec*math.Pi) // gentle 2s breathing colon

			totalCols := len(gridCols)

			padX, padY := float32(12), float32(10)
			innerW := r.W - padX*2
			innerH := r.H - padY*2
			if innerW <= 4 || innerH <= 4 {
				return
			}
			pitch := min(innerW/float32(totalCols), innerH/float32(totalRows))
			dotR := clampF(pitch*0.33, 1.4, 4.8)

			gridW := pitch * float32(totalCols)
			gridH := pitch * float32(totalRows)
			originX := r.X + (r.W-gridW)/2
			originY := r.Y + (r.H-gridH)/2

			for gc := 0; gc < totalCols; gc++ {
				cx := originX + (float32(gc)+0.5)*pitch
				nx := float64(gc) / float64(max(totalCols-1, 1))
				mask := gridCols[gc]
				for gr := 0; gr < totalRows; gr++ {
					cy := originY + (float32(gr)+0.5)*pitch
					lit := gr >= 1 && gr <= 7 && (mask&(1<<(gr-1))) != 0
					if !lit {
						paintMatrixUnlit(pt, cx, cy, dotR, 0.055)
						continue
					}
					phase := matrixDotPhase(gc, gr) * 2 * math.Pi
					breath := 0.90 + 0.10*math.Sin(sec*1.5+phase)
					d := nx - wavePos
					shimmer := math.Exp(-(d * d) / (2 * 0.075 * 0.075))

					base := 0.92
					hot := 0.24
					if matrixDotHash(gc, gr) > 0.78 {
						hot = 0.62
					}
					if isColon[gc] {
						base *= colonPulse
					}
					intensity := clampF(float32(base*breath+shimmer*0.24), 0.25, 1.18)
					whiteMix := clampF(float32(hot+shimmer*0.28), 0, 0.90)
					paintMatrixLED(pt, cx, cy, dotR, col, intensity, whiteMix, 1.1, float32(shimmer))
				}
			}
		})
		ui.Text(c, fmt.Sprintf("%s · %s", now.Format("2006-01-02"), tr(weekdays[now.Weekday()]))).FontSize(fsSmall).TextColor(p.muted).SingleLine().Ellipsis("…")
	})
}
