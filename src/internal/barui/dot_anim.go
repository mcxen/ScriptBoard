package barui

import (
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
)

// NormalizeDotAnimation canonicalizes an animation name for status dots and indicators.
func NormalizeDotAnimation(anim string, pulse bool) string {
	a := strings.ToLower(strings.TrimSpace(anim))
	switch a {
	case "none", "pulse", "ripple", "dots", "wave", "orbit", "blink", "matrix":
		return a
	case "":
		if pulse {
			return "pulse"
		}
		return "none"
	default:
		if pulse {
			return "pulse"
		}
		return "none"
	}
}

// DotAnimationBoxSize computes the bounding box size for an animated dot indicator.
func DotAnimationBoxSize(anim string, dotSize float32, dotCount int) (w, h float32) {
	if dotSize < 4 {
		dotSize = 7
	} else if dotSize > 16 {
		dotSize = 16
	}
	effAnim := NormalizeDotAnimation(anim, false)
	cnt := normalizeDotCount(effAnim, dotCount)
	switch effAnim {
	case "pulse":
		s := dotSize + 8
		return s, s
	case "ripple", "orbit":
		s := dotSize + 14
		return s, s
	case "dots", "wave":
		return float32(cnt)*(dotSize+3) + 2, dotSize + 6
	case "matrix":
		step := dotSize*0.85 + 3
		return float32(cnt)*step + 8, dotSize*1.8 + 8
	default:
		return dotSize + 2, dotSize + 2
	}
}

func normalizeDotCount(anim string, dotCount int) int {
	if dotCount <= 0 {
		switch anim {
		case "orbit":
			return 3
		case "matrix":
			return 5
		default:
			return 4
		}
	}
	if dotCount < 2 {
		return 2
	}
	if dotCount > 12 {
		return 12
	}
	return dotCount
}

// PaintDotAnimation draws a single-dot, multi-dot, radar-ripple, orbiting, or
// LED-matrix dot animation at 24fps inside r when animate is true.
func PaintDotAnimation(
	pt *ui.Painter,
	r ui.Rect,
	anim string,
	col ui.Color,
	dotSize float32,
	dotCount int,
	speed float64,
	animate bool,
) {
	if r.W <= 0 || r.H <= 0 {
		return
	}
	if dotSize < 4 {
		dotSize = 7
	} else if dotSize > 16 {
		dotSize = 16
	}
	if speed <= 0 {
		speed = 1.0
	} else if speed < 0.25 {
		speed = 0.25
	} else if speed > 4.0 {
		speed = 4.0
	}

	effAnim := NormalizeDotAnimation(anim, false)
	var sec float64
	if animate && effAnim != "none" {
		pt.AnimationFrameRate(24)
		sec = (float64(pt.Now().UnixNano()) / 1e9) * speed
	} else if !animate {
		// Still use a fixed pleasing phase for offline previews when requested?
		// Keep phase 0.35s so multi-dot/orbit/ripple states render distinctly in snapshots too.
		sec = 0.35
	}

	cx := r.X + r.W/2
	cy := r.Y + r.H/2
	rad := dotSize / 2

	switch effAnim {
	case "pulse":
		wave := 0.5 + 0.5*math.Sin(sec*3.2)
		haloR := rad + 2.0 + float32(2.6*wave)
		haloA := float32(0.16 + 0.22*(1.0-wave))
		pt.Fill(ui.Rect{X: cx - haloR, Y: cy - haloR, W: haloR * 2, H: haloR * 2}, col.Alpha(haloA), haloR)
		coreR := rad * float32(0.92+0.14*wave)
		pt.Fill(ui.Rect{X: cx - coreR, Y: cy - coreR, W: coreR * 2, H: coreR * 2}, col, coreR)

	case "ripple":
		maxR := min(r.W, r.H)/2 - 1
		if maxR < rad+2 {
			maxR = rad + 3
		}
		for waveIdx := 0; waveIdx < 2; waveIdx++ {
			phase := math.Mod(sec*0.75+float64(waveIdx)*0.5, 1.0)
			rr := rad + float32(phase)*(maxR-rad)
			alpha := float32((1.0 - phase) * 0.65)
			if alpha > 0.02 {
				pt.StrokePath((&ui.Path{}).Circle(cx, cy, rr), 1.4, col.Alpha(alpha))
			}
		}
		pt.Fill(ui.Rect{X: cx - rad*1.3, Y: cy - rad*1.3, W: rad * 2.6, H: rad * 2.6}, col.Alpha(0.22), rad*1.3)
		pt.Fill(ui.Rect{X: cx - rad, Y: cy - rad, W: rad * 2, H: rad * 2}, col, rad)

	case "dots":
		cnt := normalizeDotCount("dots", dotCount)
		step := r.W / float32(cnt)
		dotR := clampF(min(dotSize/2, (step-2)/2), 1.6, 5.5)
		bounceAmp := clampF((r.H-dotR*2)*0.42, 0, 3.5)
		for i := 0; i < cnt; i++ {
			dx := r.X + (float32(i)+0.5)*step
			phase := sec*4.2 - float64(i)*0.65
			s := math.Sin(phase)
			dy := cy - float32(s)*bounceAmp
			w := float32(0.5 + 0.5*s)
			alpha := 0.35 + 0.65*w
			rr := dotR * (0.82 + 0.24*w)
			if w > 0.55 {
				pt.Fill(ui.Rect{X: dx - rr*1.45, Y: dy - rr*1.45, W: rr * 2.9, H: rr * 2.9}, col.Alpha(0.18*w), rr*1.45)
			}
			pt.Fill(ui.Rect{X: dx - rr, Y: dy - rr, W: rr * 2, H: rr * 2}, col.Alpha(alpha), rr)
		}

	case "wave":
		cnt := normalizeDotCount("wave", dotCount)
		step := r.W / float32(cnt)
		dotR := clampF(min(dotSize/2, (step-2)/2), 1.5, 5.0)
		for i := 0; i < cnt; i++ {
			dx := r.X + (float32(i)+0.5)*step
			w := float32(0.5 + 0.5*math.Sin(sec*3.8-float64(i)*0.75))
			intensity := 0.40 + 0.60*w
			whiteMix := 0.14 + 0.36*w
			paintLED(pt, dx, cy, dotR*(0.85+0.20*w), col, intensity, whiteMix)
		}

	case "orbit":
		cnt := normalizeDotCount("orbit", dotCount)
		if cnt > 6 {
			cnt = 6
		}
		orbitR := min(r.W, r.H)/2 - 2.2
		if orbitR < rad+2 {
			orbitR = rad + 3
		}
		pt.StrokePath((&ui.Path{}).Circle(cx, cy, orbitR), 1, col.Alpha(0.16))
		coreR := clampF(rad*0.72, 1.8, 4.5)
		pt.Fill(ui.Rect{X: cx - coreR, Y: cy - coreR, W: coreR * 2, H: coreR * 2}, col.Alpha(0.72), coreR)
		satR := clampF(rad*0.48, 1.4, 3.2)
		for i := 0; i < cnt; i++ {
			ang := sec*2.6 + float64(i)*(2*math.Pi/float64(cnt))
			ox := cx + float32(math.Cos(ang))*orbitR
			oy := cy + float32(math.Sin(ang))*orbitR
			alpha := float32(0.55 + 0.45*(0.5+0.5*math.Sin(ang+sec)))
			pt.Fill(ui.Rect{X: ox - satR*1.4, Y: oy - satR*1.4, W: satR * 2.8, H: satR * 2.8}, col.Alpha(0.18*alpha), satR*1.4)
			pt.Fill(ui.Rect{X: ox - satR, Y: oy - satR, W: satR * 2, H: satR * 2}, col.Alpha(alpha), satR)
		}

	case "blink":
		// Sharp telemetry LED double-pulse cycle.
		cyc := math.Mod(sec*1.15, 1.0)
		on := cyc < 0.16 || (cyc >= 0.26 && cyc < 0.42)
		if !animate {
			on = true
		}
		if on {
			pt.Fill(ui.Rect{X: cx - rad*1.75, Y: cy - rad*1.75, W: rad * 3.5, H: rad * 3.5}, col.Alpha(0.28), rad*1.75)
			pt.Fill(ui.Rect{X: cx - rad, Y: cy - rad, W: rad * 2, H: rad * 2}, col, rad)
			coreR := rad * 0.42
			pt.Fill(ui.Rect{X: cx - coreR, Y: cy - coreR, W: coreR * 2, H: coreR * 2}, ui.Hex("#FFFFFF").Alpha(0.65), coreR)
		} else {
			pt.Fill(ui.Rect{X: cx - rad*0.9, Y: cy - rad*0.9, W: rad * 1.8, H: rad * 1.8}, col.Alpha(0.26), rad*0.9)
		}

	case "matrix":
		cnt := normalizeDotCount("matrix", dotCount)
		rows := 2
		padX, padY := float32(3), float32(2.5)
		radBox := min(r.H/2, 6)
		pt.Fill(r, ui.RGBA(12, 16, 24, 0.82), radBox)
		pt.Stroke(r, ui.RGBA(255, 255, 255, 0.09), radBox, 1)
		iw := r.W - padX*2
		ih := r.H - padY*2
		if iw <= 2 || ih <= 2 {
			return
		}
		colStep := iw / float32(cnt)
		rowStep := ih / float32(rows)
		dotR := clampF(min(colStep, rowStep)*0.30, 1.2, 2.8)
		for cxIdx := 0; cxIdx < cnt; cxIdx++ {
			dx := r.X + padX + (float32(cxIdx)+0.5)*colStep
			for ryIdx := 0; ryIdx < rows; ryIdx++ {
				dy := r.Y + padY + (float32(ryIdx)+0.5)*rowStep
				w := float32(0.5 + 0.5*math.Sin(sec*3.4-float64(cxIdx)*0.6-float64(ryIdx)*0.45))
				intensity := 0.55 + 0.45*w
				whiteMix := 0.16 + 0.36*w
				paintLED(pt, dx, dy, dotR, col, intensity, whiteMix)
			}
		}

	default:
		pt.Fill(ui.Rect{X: cx - rad, Y: cy - rad, W: rad * 2, H: rad * 2}, col, rad)
	}
}
