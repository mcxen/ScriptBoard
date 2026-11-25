package colortheme

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestParseColors(t *testing.T) {
	cases := []struct {
		raw  string
		want ui.Color
		ok   bool
	}{
		{"#000", ui.RGB(0, 0, 0), true},
		{"#0f8", ui.RGB(0, 255, 136), true},
		{"#0f88", ui.RGBA(0, 255, 136, float32(0x88)/255), true},
		{"#111827", ui.Hex("#111827"), true},
		{"#3b82f680", ui.RGBA(0x3b, 0x82, 0xf6, float32(0x80)/255), true},
		{"rgb(10, 20, 30)", ui.RGB(10, 20, 30), true},
		{"rgba(10, 20, 30, 0.5)", ui.RGBA(10, 20, 30, 0.5), true},
		{"black", ui.RGB(0, 0, 0), true},
		{"White", ui.RGB(255, 255, 255), true},
		{"gray", ui.RGB(128, 128, 128), true},
		{"transparent", ui.Color{}, true},
		{"surfaceCard", ui.Color{}, false},
		{"", ui.Color{}, false},
	}
	for _, tc := range cases {
		got, ok := Parse(tc.raw)
		if ok != tc.ok {
			t.Fatalf("Parse(%q) ok=%v, want %v", tc.raw, ok, tc.ok)
		}
		if ok && got != tc.want {
			t.Fatalf("Parse(%q) = %+v, want %+v", tc.raw, got, tc.want)
		}
	}
}

func TestAdaptDarkModeRoles(t *testing.T) {
	darkPal := DefaultPalette(true)

	// Dark neutral and dark chromatic texts must reach >= 4.5 contrast on dark #1A1D24.
	textInputs := []string{"#000000", "#111827", "#333333", "black", "rgba(0,0,0,0.85)", "#1E3A8A"}
	for _, raw := range textInputs {
		col, _ := Parse(raw)
		got := Adapt(col, RoleText, darkPal)
		if cr := ContrastRatio(got, darkPal.Surface); cr < 4.5 {
			t.Fatalf("RoleText dark adapt(%q) -> %+v has contrast %.2f < 4.5", raw, got, cr)
		}
		if RelativeLuminance(got) <= RelativeLuminance(darkPal.Surface) {
			t.Fatalf("RoleText dark adapt(%q) should be lighter than dark surface, got %+v", raw, got)
		}
	}

	// Light neutral surfaces must invert to dark surfaces, keeping hierarchy.
	surfWhite := Adapt(ui.Hex("#FFFFFF"), RoleSurface, darkPal)
	surfGray := Adapt(ui.Hex("#F5F6F9"), RoleSurface, darkPal)
	surfGlass := Adapt(ui.RGBA(0, 0, 0, 0.05), RoleSurface, darkPal)
	if RelativeLuminance(surfWhite) >= 0.15 {
		t.Fatalf("#FFFFFF RoleSurface in dark mode stayed too bright: %+v (L=%.3f)", surfWhite, RelativeLuminance(surfWhite))
	}
	if RelativeLuminance(surfGray) >= 0.15 {
		t.Fatalf("#F5F6F9 RoleSurface in dark mode stayed too bright: %+v (L=%.3f)", surfGray, RelativeLuminance(surfGray))
	}
	if surfGlass.R < 200 || surfGlass.A == 0 || surfGlass.A > 80 {
		t.Fatalf("rgba(0,0,0,0.05) should become subtle white glass, got %+v", surfGlass)
	}

	// Pastel light badge background becomes dark same-hue surface.
	pastel := Adapt(ui.Hex("#EFF6FF"), RoleSurface, darkPal)
	if RelativeLuminance(pastel) >= 0.15 || pastel.B <= pastel.R {
		t.Fatalf("#EFF6FF pastel should adapt to dark blue-tinted surface, got %+v", pastel)
	}

	// Solid saturated fill (#2563EB) is preserved in dark mode.
	btnBG := ui.Hex("#2563EB")
	if got := Adapt(btnBG, RoleSurface, darkPal); got != btnBG {
		t.Fatalf("solid primary #2563EB should be preserved, got %+v", got)
	}

	// Borders become subtle white hairlines.
	bHex := Adapt(ui.Hex("#E5E7EB"), RoleBorder, darkPal)
	bRgba := Adapt(ui.RGBA(0, 0, 0, 0.10), RoleBorder, darkPal)
	if bHex.R < 200 || bHex.A == 0 || bHex.A > 70 {
		t.Fatalf("#E5E7EB border in dark mode unexpected: %+v", bHex)
	}
	if bRgba.R < 200 || bRgba.A == 0 || bRgba.A > 80 {
		t.Fatalf("rgba(0,0,0,0.10) border in dark mode unexpected: %+v", bRgba)
	}
}

func TestAdaptLightModeAndNestedSurfaces(t *testing.T) {
	lightPal := DefaultPalette(false)

	// White text and ultra-bright neon ink deepen on white card.
	invWhite := Adapt(ui.Hex("#FFFFFF"), RoleText, lightPal)
	if cr := ContrastRatio(invWhite, lightPal.Surface); cr < 4.5 {
		t.Fatalf("#FFFFFF text in light mode contrast %.2f < 4.5", cr)
	}
	deepNeon := Adapt(ui.Hex("#00F5D4"), RoleText, lightPal)
	if cr := ContrastRatio(deepNeon, lightPal.Surface); cr < 4.5 {
		t.Fatalf("#00F5D4 text in light mode contrast %.2f < 4.5", cr)
	}

	// Dark-mode subtle white overlay on light mode inverts to slate tint.
	whiteTint := Adapt(ui.RGBA(255, 255, 255, 0.06), RoleSurface, lightPal)
	if whiteTint.R > 60 || whiteTint.A == 0 {
		t.Fatalf("white tint in light mode should invert to dark slate tint, got %+v", whiteTint)
	}

	// Saturated blue accent preserved in light mode.
	if got := Adapt(ui.Hex("#2563EB"), RoleAccent, lightPal); got != ui.Hex("#2563EB") {
		t.Fatalf("#2563EB accent in light mode changed to %+v", got)
	}

	// Nested context: white text inside #2563EB or #0F172A stays white in both light and dark modes!
	for _, pal := range []Palette{DefaultPalette(false), DefaultPalette(true)} {
		for _, bg := range []ui.Color{ui.Hex("#2563EB"), ui.Hex("#0F172A")} {
			nested := pal.WithSurface(bg)
			if !nested.Dark {
				t.Fatalf("expected %+v nested surface to have Dark=true", bg)
			}
			if got := Adapt(ui.Hex("#FFFFFF"), RoleText, nested); got != ui.Hex("#FFFFFF") {
				t.Fatalf("white text on %+v (outer dark=%v) should stay white, got %+v", bg, pal.Dark, got)
			}
		}
	}

	// OnColor picks dark ink on bright yellow/amber and white on navy/blue.
	if OnColor(ui.Hex("#2563EB")) != ui.Hex("#FFFFFF") {
		t.Fatalf("expected white OnColor for #2563EB")
	}
	if OnColor(ui.Hex("#FACC15")) != ui.Hex("#141821") {
		t.Fatalf("expected dark ink OnColor for bright yellow")
	}
}

func TestIsHardcodedAdaptiveHazard(t *testing.T) {
	for _, hazard := range []string{"#000", "#000000", "#111827", "#FFFFFF", "black", "white", "rgba(0,0,0,0.85)"} {
		if !IsHardcodedAdaptiveHazard(hazard) {
			t.Fatalf("expected %q to be flagged as adaptive hazard", hazard)
		}
	}
	for _, safe := range []string{"", "transparent", "surfaceCard", "text", "#2563EB", "#10B981"} {
		if IsHardcodedAdaptiveHazard(safe) {
			t.Fatalf("expected %q NOT to be flagged as adaptive hazard", safe)
		}
	}
}
