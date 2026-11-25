package colortheme

import (
	"math"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
)

// Role identifies the visual purpose of a color so dark/light adaptation
// preserves perceptual hierarchy (foreground ink, secondary text, container
// surface, hairline border, or vivid accent/series color).
type Role uint8

const (
	RoleText Role = iota
	RoleMuted
	RoleSurface
	RoleBorder
	RoleAccent
)

// Palette describes the active surface and polarity context used when adapting
// literal colors and semantic fallbacks.
type Palette struct {
	Dark       bool
	Surface    ui.Color
	SurfaceAlt ui.Color
	Text       ui.Color
	TextMuted  ui.Color
	Border     ui.Color
}

// DefaultPalette returns the base ScriptBoard dark or light palette.
func DefaultPalette(dark bool) Palette {
	if dark {
		return Palette{
			Dark:       true,
			Surface:    ui.Hex("#1A1D24"),
			SurfaceAlt: ui.Hex("#222631"),
			Text:       ui.Hex("#F2F4F8"),
			TextMuted:  ui.Hex("#9BA3B4"),
			Border:     ui.RGBA(255, 255, 255, 0.10),
		}
	}
	return Palette{
		Dark:       false,
		Surface:    ui.Hex("#FFFFFF"),
		SurfaceAlt: ui.Hex("#F5F6F9"),
		Text:       ui.Hex("#141821"),
		TextMuted:  ui.Hex("#646C7E"),
		Border:     ui.RGBA(15, 23, 42, 0.10),
	}
}

// WithSurface returns a derived Palette whose Surface is bg composited over the
// current surface. When the resulting effective surface flips polarity (for
// example a saturated #2563EB or #0F172A container inside light mode, or an
// amber #F59E0B badge inside dark mode), Text, TextMuted, Border, and
// SurfaceAlt follow the effective surface polarity.
func (p Palette) WithSurface(bg ui.Color) Palette {
	if bg.A == 0 {
		return p
	}
	base := p.Surface
	if base.A == 0 {
		base = DefaultPalette(p.Dark).Surface
	}
	eff := CompositeOver(bg, base)
	if eff.A < 255 {
		eff = CompositeOver(eff, DefaultPalette(p.Dark).Surface)
	}
	effDark := RelativeLuminance(eff) < 0.18
	if effDark != p.Dark {
		def := DefaultPalette(effDark)
		def.Surface = eff
		return def
	}
	p.Surface = eff
	return p
}

var namedColors = map[string]ui.Color{
	"black":       ui.RGB(0, 0, 0),
	"white":       ui.RGB(255, 255, 255),
	"gray":        ui.RGB(128, 128, 128),
	"grey":        ui.RGB(128, 128, 128),
	"darkgray":    ui.RGB(169, 169, 169),
	"darkgrey":    ui.RGB(169, 169, 169),
	"lightgray":   ui.RGB(211, 211, 211),
	"lightgrey":   ui.RGB(211, 211, 211),
	"dimgray":     ui.RGB(105, 105, 105),
	"dimgrey":     ui.RGB(105, 105, 105),
	"silver":      ui.RGB(192, 192, 192),
	"slategray":   ui.RGB(112, 128, 144),
	"slategrey":   ui.RGB(112, 128, 144),
	"whitesmoke":  ui.RGB(245, 245, 245),
	"gainsboro":   ui.RGB(220, 220, 220),
	"snow":        ui.RGB(255, 250, 250),
	"ivory":       ui.RGB(255, 255, 240),
	"red":         ui.RGB(239, 68, 68),
	"green":       ui.RGB(16, 185, 129),
	"blue":        ui.RGB(37, 99, 235),
	"yellow":      ui.RGB(234, 179, 8),
	"orange":      ui.RGB(249, 115, 22),
	"purple":      ui.RGB(139, 92, 246),
	"pink":        ui.RGB(236, 72, 153),
	"cyan":        ui.RGB(6, 182, 212),
	"aqua":        ui.RGB(0, 255, 255),
	"teal":        ui.RGB(20, 184, 166),
	"indigo":      ui.RGB(99, 102, 241),
	"violet":      ui.RGB(139, 92, 246),
	"navy":        ui.RGB(0, 0, 128),
	"maroon":      ui.RGB(128, 0, 0),
	"olive":       ui.RGB(128, 128, 0),
	"lime":        ui.RGB(34, 197, 94),
	"gold":        ui.RGB(245, 158, 11),
	"coral":       ui.RGB(255, 127, 80),
	"crimson":     ui.RGB(220, 20, 60),
	"brown":       ui.RGB(165, 42, 42),
	"amber":       ui.RGB(245, 158, 11),
	"emerald":     ui.RGB(16, 185, 129),
	"rose":        ui.RGB(244, 63, 94),
	"slate":       ui.RGB(100, 116, 139),
}

// Parse parses a CSS hex (#RGB, #RGBA, #RRGGBB, #RRGGBBAA), rgb()/rgba(),
// transparent, or CSS named color literal into a ui.Color without panicking.
func Parse(raw string) (ui.Color, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ui.Color{}, false
	}
	low := strings.ToLower(s)
	if low == "transparent" {
		return ui.Color{}, true
	}
	if strings.HasPrefix(s, "#") {
		return parseHexColor(s)
	}
	if c, ok := parseRGBFunc(s); ok {
		return c, true
	}
	if c, ok := namedColors[low]; ok {
		return c, true
	}
	return ui.Color{}, false
}

func parseHexColor(s string) (ui.Color, bool) {
	h := strings.TrimPrefix(s, "#")
	switch len(h) {
	case 3:
		r, ok1 := parseHexByte(h[0:1] + h[0:1])
		g, ok2 := parseHexByte(h[1:2] + h[1:2])
		b, ok3 := parseHexByte(h[2:3] + h[2:3])
		if ok1 && ok2 && ok3 {
			return ui.RGB(r, g, b), true
		}
	case 4:
		r, ok1 := parseHexByte(h[0:1] + h[0:1])
		g, ok2 := parseHexByte(h[1:2] + h[1:2])
		b, ok3 := parseHexByte(h[2:3] + h[2:3])
		a, ok4 := parseHexByte(h[3:4] + h[3:4])
		if ok1 && ok2 && ok3 && ok4 {
			return ui.RGBA(r, g, b, float32(a)/255), true
		}
	case 6:
		r, ok1 := parseHexByte(h[0:2])
		g, ok2 := parseHexByte(h[2:4])
		b, ok3 := parseHexByte(h[4:6])
		if ok1 && ok2 && ok3 {
			return ui.RGB(r, g, b), true
		}
	case 8:
		r, ok1 := parseHexByte(h[0:2])
		g, ok2 := parseHexByte(h[2:4])
		b, ok3 := parseHexByte(h[4:6])
		a, ok4 := parseHexByte(h[6:8])
		if ok1 && ok2 && ok3 && ok4 {
			return ui.RGBA(r, g, b, float32(a)/255), true
		}
	}
	return ui.Color{}, false
}

func parseHexByte(s string) (uint8, bool) {
	v, err := strconv.ParseUint(s, 16, 8)
	return uint8(v), err == nil
}

func parseRGBFunc(s string) (ui.Color, bool) {
	low := strings.ToLower(s)
	inner := ""
	isRGBA := false
	switch {
	case strings.HasPrefix(low, "rgba(") && strings.HasSuffix(low, ")"):
		inner = s[5 : len(s)-1]
		isRGBA = true
	case strings.HasPrefix(low, "rgb(") && strings.HasSuffix(low, ")"):
		inner = s[4 : len(s)-1]
	default:
		return ui.Color{}, false
	}
	parts := strings.Split(inner, ",")
	if len(parts) < 3 {
		return ui.Color{}, false
	}
	r, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	g, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	b, err3 := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return ui.Color{}, false
	}
	clampByte := func(v float64) uint8 {
		if v < 0 {
			return 0
		}
		if v > 255 {
			return 255
		}
		return uint8(v + 0.5)
	}
	if isRGBA && len(parts) >= 4 {
		a, err4 := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
		if err4 != nil {
			return ui.Color{}, false
		}
		a = clamp01(a)
		return ui.RGBA(clampByte(r), clampByte(g), clampByte(b), float32(a)), true
	}
	return ui.RGB(clampByte(r), clampByte(g), clampByte(b)), true
}

// RelativeLuminance computes the WCAG 2.1 relative luminance (0..1) of an sRGB color.
func RelativeLuminance(c ui.Color) float64 {
	linear := func(channel uint8) float64 {
		v := float64(channel) / 255.0
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(c.R) + 0.7152*linear(c.G) + 0.0722*linear(c.B)
}

// CompositeOver blends straight-alpha fg over bg using Porter-Duff source-over.
func CompositeOver(fg, bg ui.Color) ui.Color {
	fa := float64(fg.A) / 255.0
	if fa >= 1 {
		return fg
	}
	if fa <= 0 {
		return bg
	}
	ba := float64(bg.A) / 255.0
	outA := fa + ba*(1-fa)
	if outA <= 0 {
		return ui.Color{}
	}
	blend := func(fc, bc uint8) uint8 {
		v := (float64(fc)*fa + float64(bc)*ba*(1-fa)) / outA
		return floatToByte(v / 255.0)
	}
	return ui.Color{
		R: blend(fg.R, bg.R),
		G: blend(fg.G, bg.G),
		B: blend(fg.B, bg.B),
		A: floatToByte(outA),
	}
}

// ContrastRatio returns the WCAG 2.1 contrast ratio between fg composited over
// bg and bg itself (1.0 .. 21.0).
func ContrastRatio(fg, bg ui.Color) float64 {
	effBG := bg
	if effBG.A == 0 {
		effBG = ui.RGB(bg.R, bg.G, bg.B)
	} else if effBG.A < 255 {
		effBG = ui.Color{R: bg.R, G: bg.G, B: bg.B, A: 255}
	}
	comp := CompositeOver(fg, effBG)
	l1 := RelativeLuminance(comp)
	l2 := RelativeLuminance(effBG)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

// ToHSL converts sRGB bytes to HSL where h is degrees [0..360) and s, l are [0..1].
func ToHSL(c ui.Color) (h, s, l float64) {
	r := float64(c.R) / 255.0
	g := float64(c.G) / 255.0
	b := float64(c.B) / 255.0

	maxC := math.Max(r, math.Max(g, b))
	minC := math.Min(r, math.Min(g, b))
	l = (maxC + minC) / 2.0
	d := maxC - minC
	if d == 0 {
		return 0, 0, l
	}
	if l > 0.5 {
		s = d / (2.0 - maxC - minC)
	} else {
		s = d / (maxC + minC)
	}
	switch maxC {
	case r:
		h = (g - b) / d
		if g < b {
			h += 6
		}
	case g:
		h = (b-r)/d + 2
	case b:
		h = (r-g)/d + 4
	}
	h *= 60
	return h, s, l
}

// FromHSL converts HSL (h in degrees, s, l in [0..1]) and byte alpha into ui.Color.
func FromHSL(h, s, l float64, a uint8) ui.Color {
	s = clamp01(s)
	l = clamp01(l)
	if s == 0 {
		v := floatToByte(l)
		return ui.Color{R: v, G: v, B: v, A: a}
	}
	hue := math.Mod(h, 360)
	if hue < 0 {
		hue += 360
	}
	hNorm := hue / 360.0
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	hue2rgb := func(t float64) float64 {
		if t < 0 {
			t += 1
		}
		if t > 1 {
			t -= 1
		}
		if t < 1.0/6.0 {
			return p + (q-p)*6*t
		}
		if t < 1.0/2.0 {
			return q
		}
		if t < 2.0/3.0 {
			return p + (q-p)*(2.0/3.0-t)*6
		}
		return p
	}
	return ui.Color{
		R: floatToByte(hue2rgb(hNorm + 1.0/3.0)),
		G: floatToByte(hue2rgb(hNorm)),
		B: floatToByte(hue2rgb(hNorm - 1.0/3.0)),
		A: a,
	}
}

// IsNeutral reports whether c is achromatic or cool/warm neutral (low RGB
// chroma <= 0.16), distinguishing grayscale/slate inks and surfaces from
// saturated chromatic hues.
func IsNeutral(c ui.Color) bool {
	maxC := max(c.R, max(c.G, c.B))
	minC := min(c.R, min(c.G, c.B))
	return float64(maxC-minC)/255.0 <= 0.16
}

// OnColor returns high-contrast foreground ink (#FFFFFF or #141821) for bg.
func OnColor(bg ui.Color) ui.Color {
	white := ui.Hex("#FFFFFF")
	darkInk := ui.Hex("#141821")
	if ContrastRatio(white, bg) >= ContrastRatio(darkInk, bg) {
		return white
	}
	return darkInk
}

// EnsureContrast adapts fg when needed so ContrastRatio(result, bg) >= minCR.
// Colors that already meet minCR are returned untouched.
func EnsureContrast(fg, bg ui.Color, minCR float64) ui.Color {
	if fg.A == 0 || ContrastRatio(fg, bg) >= minCR {
		return fg
	}
	bgDark := RelativeLuminance(bg) < 0.18
	h, s, l := ToHSL(fg)
	neutral := IsNeutral(fg)

	var cand ui.Color
	if neutral {
		if bgDark {
			invL := clampRange(1.0-l, 0.62, 0.96)
			targetS := s
			targetH := h
			if s <= 0.18 {
				targetH = 220
				targetS = 0.14
			}
			cand = FromHSL(targetH, targetS, invL, fg.A)
			opaque := FromHSL(targetH, targetS, invL, 255)
			if ContrastRatio(opaque, bg) < minCR {
				lo, hi := invL, 0.98
				for i := 0; i < 16; i++ {
					mid := (lo + hi) / 2
					if ContrastRatio(FromHSL(targetH, targetS, mid, 255), bg) >= minCR {
						hi = mid
					} else {
						lo = mid
					}
				}
				cand = FromHSL(targetH, targetS, hi, fg.A)
			}
		} else {
			invL := clampRange(1.0-l, 0.08, 0.44)
			targetS := s
			targetH := h
			if s <= 0.18 {
				targetH = 220
				targetS = 0.16
			}
			cand = FromHSL(targetH, targetS, invL, fg.A)
			opaque := FromHSL(targetH, targetS, invL, 255)
			if ContrastRatio(opaque, bg) < minCR {
				lo, hi := 0.04, invL
				for i := 0; i < 16; i++ {
					mid := (lo + hi) / 2
					if ContrastRatio(FromHSL(targetH, targetS, mid, 255), bg) >= minCR {
						lo = mid
					} else {
						hi = mid
					}
				}
				cand = FromHSL(targetH, targetS, lo, fg.A)
			}
		}
	} else {
		if bgDark {
			adjS := s
			if adjS > 0.85 {
				adjS = 0.85
			}
			lo, hi := l, 0.92
			if lo > hi {
				lo = hi
			}
			bestL := hi
			for i := 0; i < 16; i++ {
				mid := (lo + hi) / 2
				if ContrastRatio(FromHSL(h, adjS, mid, 255), bg) >= minCR {
					bestL = mid
					hi = mid
				} else {
					lo = mid
				}
			}
			cand = FromHSL(h, adjS, bestL, fg.A)
		} else {
			lo, hi := 0.12, l
			if hi < lo {
				hi = lo
			}
			bestL := lo
			for i := 0; i < 16; i++ {
				mid := (lo + hi) / 2
				if ContrastRatio(FromHSL(h, s, mid, 255), bg) >= minCR {
					bestL = mid
					lo = mid
				} else {
					hi = mid
				}
			}
			cand = FromHSL(h, s, bestL, fg.A)
		}
	}

	if cand.A < 255 && ContrastRatio(cand, bg) < minCR {
		loA, hiA := int(cand.A), 255
		for loA < hiA {
			midA := (loA + hiA) / 2
			testCol := ui.Color{R: cand.R, G: cand.G, B: cand.B, A: uint8(midA)}
			if ContrastRatio(testCol, bg) >= minCR {
				hiA = midA
			} else {
				loA = midA + 1
			}
		}
		cand.A = uint8(hiA)
	}
	return cand
}

// Adapt converts col for the given visual Role and target Palette.
func Adapt(col ui.Color, role Role, pal Palette) ui.Color {
	if col.A == 0 {
		return col
	}
	surface := pal.Surface
	if surface.A == 0 {
		surface = DefaultPalette(pal.Dark).Surface
	}
	switch role {
	case RoleText:
		return EnsureContrast(col, surface, 4.5)
	case RoleMuted:
		return EnsureContrast(col, surface, 3.2)
	case RoleAccent:
		return EnsureContrast(col, surface, 3.0)
	case RoleSurface:
		return adaptSurface(col, pal)
	case RoleBorder:
		return adaptBorder(col, pal)
	default:
		return col
	}
}

func adaptSurface(col ui.Color, pal Palette) ui.Color {
	h, s, l := ToHSL(col)
	a := float64(col.A) / 255.0
	neutral := IsNeutral(col)

	if pal.Dark {
		if neutral && l >= 0.70 && a >= 0.45 {
			t := clamp01((l - 0.70) / 0.30)
			darkL := 0.22 - t*(0.22-0.13)
			return FromHSL(222, 0.16, darkL, col.A)
		}
		if neutral && l <= 0.35 && a < 0.75 {
			return ui.RGBA(255, 255, 255, float32(clampRange(a*1.35, 0.05, 0.24)))
		}
		if !neutral && l >= 0.80 && a >= 0.75 {
			base := pal.Surface
			if base.A == 0 {
				base = DefaultPalette(true).Surface
			}
			tint := FromHSL(h, clampRange(s, 0.45, 0.78), 0.56, 255)
			mixed := base.Mix(tint, 0.20)
			mixed.A = col.A
			return mixed
		}
		return col
	}

	if neutral && l >= 0.85 && a > 0 && a <= 0.25 {
		return ui.RGBA(15, 23, 42, float32(clampRange(a*0.85, 0.035, 0.14)))
	}
	return col
}

func adaptBorder(col ui.Color, pal Palette) ui.Color {
	_, _, l := ToHSL(col)
	a := float64(col.A) / 255.0
	neutral := IsNeutral(col)

	if pal.Dark {
		if neutral && l >= 0.65 && a >= 0.45 {
			return ui.RGBA(255, 255, 255, float32(clampRange(0.10+(1.0-l)*0.25, 0.09, 0.22)))
		}
		if neutral && l <= 0.35 {
			return ui.RGBA(255, 255, 255, float32(clampRange(math.Max(a*1.2, 0.10), 0.08, 0.26)))
		}
		surface := pal.Surface
		if surface.A == 0 {
			surface = DefaultPalette(true).Surface
		}
		return EnsureContrast(col, surface, 1.8)
	}

	if neutral && l >= 0.85 && a <= 0.35 {
		return ui.RGBA(15, 23, 42, float32(clampRange(a*0.85, 0.07, 0.20)))
	}
	return col
}

// IsHardcodedAdaptiveHazard reports whether raw is a hardcoded neutral color
// literal (such as #000000, #111827, #FFFFFF, white, black, or rgba(0,0,0,0.85))
// that would break Light/Dark adaptation if left unadapted. Semantic tokens
// and transparent values return false.
func IsHardcodedAdaptiveHazard(raw string) bool {
	s := strings.TrimSpace(raw)
	if s == "" || strings.EqualFold(s, "transparent") {
		return false
	}
	col, ok := Parse(s)
	if !ok || col.A == 0 {
		return false
	}
	return IsNeutral(col)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func clampRange(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func floatToByte(v float64) uint8 {
	v = clamp01(v)
	return uint8(v*255.0 + 0.5)
}
