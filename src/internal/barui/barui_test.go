package barui

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/bar"
	"scriptboard/src/internal/colortheme"
)

func TestColorParsing(t *testing.T) {
	th := ThemeFor(true, false)
	if got := th.Color("#00ff88", ui.Color{}); got != ui.Hex("#00ff88") {
		t.Fatalf("6-hex mismatch: %+v", got)
	}
	if got := th.Color("#0f8", ui.Color{}); got != ui.RGB(0, 255, 136) {
		t.Fatalf("3-hex mismatch: %+v", got)
	}
	if got := th.Color("#3b82f680", ui.Color{}); got.A == 0 || got == ui.RGB(0x3b, 0x82, 0xf6) {
		t.Fatalf("8-hex alpha mismatch: %+v", got)
	}
	if got := th.Color("rgba(10, 20, 30, 0.5)", ui.Color{}); got != ui.RGBA(10, 20, 30, 0.5) {
		t.Fatalf("rgba mismatch: %+v", got)
	}
	if got := th.Color("surfaceCard", ui.Color{}); got != th.SurfaceCard {
		t.Fatalf("semantic token surfaceCard mismatch: %+v", got)
	}
}

func TestTemplatesRenderPreviewLightDarkAndPassAudit(t *testing.T) {
	for _, tpl := range bar.Templates() {
		t.Run(tpl.ID, func(t *testing.T) {
			sp, err := bar.Parse(tpl.Source)
			if err != nil {
				t.Fatal(err)
			}
			inst, err := bar.NewInstance(sp, bar.Options{
				DataDir: t.TempDir(),
				Offline: true,
				Theme:   "dark",
			})
			if err != nil {
				t.Fatal(err)
			}
			if !inst.WaitIdle(5 * time.Second) {
				inst.Close()
				t.Fatal("WaitIdle timed out")
			}
			snap := inst.Snapshot()
			inst.Close()

			audit := AuditSnapshot(snap, sp)
			if !audit.OK || len(audit.Warnings) > 0 {
				t.Fatalf("template %s failed visual audit: %+v", tpl.ID, audit)
			}

			lightRes, err := RenderPreview(snap, PreviewOptions{
				Dark:     false,
				Scale:    1,
				Material: sp.Material,
			})
			if err != nil {
				t.Fatalf("light RenderPreview: %v", err)
			}
			darkRes, err := RenderPreview(snap, PreviewOptions{
				Dark:     true,
				Scale:    1,
				Material: sp.Material,
			})
			if err != nil {
				t.Fatalf("dark RenderPreview: %v", err)
			}

			lightImg, err := png.Decode(bytes.NewReader(lightRes.PNG))
			if err != nil {
				t.Fatalf("decode light PNG: %v", err)
			}
			darkImg, err := png.Decode(bytes.NewReader(darkRes.PNG))
			if err != nil {
				t.Fatalf("decode dark PNG: %v", err)
			}
			if lightImg.Bounds().Dx() < 280 || darkImg.Bounds().Dx() < 280 {
				t.Fatalf("unexpected preview dimensions: light=%v dark=%v", lightImg.Bounds(), darkImg.Bounds())
			}
			// Sample center backdrop/card pixel to verify Light and Dark actually differ visually.
			if bytes.Equal(lightRes.PNG, darkRes.PNG) {
				t.Fatalf("expected Light and Dark preview PNGs to differ visually")
			}
		})
	}
}

func TestAllCustomNodesPreviewAndAuditWarnings(t *testing.T) {
	// Generate a tiny 2x2 PNG data URL for testing Image nodes.
	im := image.NewRGBA(image.Rect(0, 0, 2, 2))
	im.Set(0, 0, color.RGBA{R: 255, G: 80, B: 80, A: 255})
	im.Set(1, 1, color.RGBA{R: 80, G: 180, B: 255, A: 255})
	var imgBuf bytes.Buffer
	_ = png.Encode(&imgBuf, im)
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(imgBuf.Bytes())

	snap := bar.Snapshot{
		Tray: &bar.TraySpec{Title: "ABCDEFGHIJKLMN"}, // 14 chars -> should warn
		Panel: &bar.Node{
			Type: "Column",
			Props: map[string]any{
				"padding": float64(14),
				"gap":     float64(8),
			},
			Children: []*bar.Node{
				{Type: "Text", Props: map[string]any{"fontSize": float64(8), "textColor": "#000000"}, Text: "Tiny hardcoded"},
				{Type: "Ring", Props: map[string]any{"value": float64(64), "style": "solid", "label": "64%"}},
				{Type: "Progress", Props: map[string]any{"value": float64(70), "style": "segmented", "segments": float64(8)}},
				{Type: "Progress", Props: map[string]any{"value": float64(70), "style": "matrix"}},
				{Type: "Badge", Props: map[string]any{"tone": "purple", "dot": true}, Text: "PurpleTag"},
				{Type: "StatusDot", Props: map[string]any{"color": "warning", "pulse": true}},
				{Type: "Switch", Props: map[string]any{"checked": true}},
				{Type: "Segmented", Props: map[string]any{"value": "a", "options": []any{"a", "b"}}},
				{Type: "ListRow", Props: map[string]any{"icon": "key", "title": "RowItem", "shortcut": "Cmd+K", "chevron": true}},
				{Type: "Image", Props: map[string]any{"src": dataURL, "width": float64(24), "height": float64(24), "radius": float64(6)}},
				{Type: "Input", Props: map[string]any{"value": "abc", "placeholder": "Search"}},
				{Type: "Chart", Props: map[string]any{
					"type":          "bar",
					"height":        float64(36),
					"referenceLine": float64(50),
					"series": []any{map[string]any{
						"color":  "accent",
						"points": []any{map[string]any{"x": float64(0), "y": float64(30)}, map[string]any{"x": float64(1), "y": float64(75)}},
					}},
				}},
			},
		},
	}

	audit := AuditSnapshot(snap, bar.Spec{ID: "audit-test"})
	if audit.OK || len(audit.Warnings) < 3 {
		t.Fatalf("expected audit warnings for long tray title, missing preferredWidth, tiny font, and hardcoded #000000, got %+v", audit)
	}
	if !audit.HasVisualization || !audit.HasInteractive {
		t.Fatalf("expected HasVisualization & HasInteractive true, got %+v", audit)
	}

	res, err := RenderPreview(snap, PreviewOptions{Dark: true, Scale: 1})
	if err != nil {
		t.Fatalf("RenderPreview custom nodes: %v", err)
	}
	if len(res.PNG) == 0 {
		t.Fatal("empty PNG bytes")
	}
}

func TestBuiltinIconsParseValid(t *testing.T) {
	if len(builtinIconPaths) == 0 {
		t.Fatal("builtinIconPaths must not be empty")
	}
	for name := range builtinIconPaths {
		if svg := DefaultIcon(name); svg == nil {
			t.Fatalf("DefaultIcon(%q) returned nil", name)
		}
	}
}

func TestWeatherStyleAndNewPropsRenderCleanly(t *testing.T) {
	snap := bar.Snapshot{
		Tray: &bar.TraySpec{Title: "24°"},
		Panel: &bar.Node{
			Type: "Column",
			Props: map[string]any{
				"preferredWidth": float64(300),
				"padding":        []any{float64(10), float64(16), float64(16), float64(16)},
				"gap":            float64(12),
				"safeArea":       true,
			},
			Children: []*bar.Node{
				{
					Type:  "Text",
					Props: map[string]any{"fontSize": float64(48), "fontWeight": float64(300), "fontFeatures": "tnum", "contentTransition": map[string]any{"numeric": true}},
					Text:  "24°",
				},
				{
					Type:  "ListRow",
					Props: map[string]any{"icon": "map-pin", "iconPlain": true, "title": "Tokyo-01", "value": "14ms"},
				},
			},
		},
	}
	res, err := RenderPreview(snap, PreviewOptions{Dark: true, Vibrancy: true, Scale: 1, Material: "glass"})
	if err != nil {
		t.Fatalf("RenderPreview new props: %v", err)
	}
	if len(res.PNG) == 0 {
		t.Fatal("empty PNG bytes")
	}

	sp, err := bar.Parse(bar.BuiltinWeatherBar)
	if err != nil {
		t.Fatal(err)
	}
	inst, err := bar.NewInstance(sp, bar.Options{DataDir: t.TempDir(), Offline: true, Theme: "dark"})
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close()
	if !inst.WaitIdle(3 * time.Second) {
		t.Fatal("WaitIdle timed out")
	}
	wRes, err := RenderPreview(inst.Snapshot(), PreviewOptions{Dark: true, Vibrancy: true, Scale: 1, Material: sp.Material})
	if err != nil || len(wRes.PNG) == 0 {
		t.Fatalf("BuiltinWeatherBar RenderPreview failed: err=%v len=%d", err, len(wRes.PNG))
	}
}

func TestDarkModeAutoAdaptsHardcodedLightColors(t *testing.T) {
	snap := bar.Snapshot{
		Panel: &bar.Node{
			Type: "Column",
			Props: map[string]any{
				"preferredWidth": float64(300),
				"background":     "#FFFFFF",
				"padding":        float64(14),
				"gap":            float64(10),
			},
			Children: []*bar.Node{
				{
					Type:  "Column",
					Props: map[string]any{"background": "#F5F6F9", "borderWidth": float64(1), "borderColor": "#E5E7EB", "padding": float64(10), "gap": float64(6), "radius": float64(10)},
					Children: []*bar.Node{
						{Type: "Text", Props: map[string]any{"fontSize": float64(18), "fontWeight": float64(700), "textColor": "#000000"}, Text: "HARDCODED BLACK"},
						{Type: "Text", Props: map[string]any{"fontSize": float64(13), "textColor": "#111827"}, Text: "Dark Slate Text"},
						{Type: "Badge", Props: map[string]any{"background": "#EFF6FF", "textColor": "#1D4ED8"}, Text: "Pastel Tag"},
					},
				},
				{
					Type:  "Button",
					Props: map[string]any{"variant": "primary", "background": "#2563EB", "textColor": "#FFFFFF"},
					Text:  "Primary Action",
				},
				{
					Type:  "Row",
					Props: map[string]any{"rawColor": true, "background": "#FFFFFF", "padding": float64(8), "radius": float64(6)},
					Children: []*bar.Node{
						{Type: "Text", Props: map[string]any{"fontSize": float64(12), "textColor": "#000000"}, Text: "RAW WHITE BOX"},
					},
				},
			},
		},
	}

	res, err := RenderPreview(snap, PreviewOptions{Dark: true, Scale: 1, Height: 240})
	if err != nil {
		t.Fatalf("RenderPreview dark auto-adapt failed: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(res.PNG))
	if err != nil {
		t.Fatalf("decode preview PNG: %v", err)
	}

	b := img.Bounds()
	pxLum := func(c color.Color) float64 {
		r, g, bl, _ := c.RGBA()
		return colortheme.RelativeLuminance(ui.RGB(uint8(r>>8), uint8(g>>8), uint8(bl>>8)))
	}

	// Sample upper-half adapted card interior (adapted #FFFFFF -> dark surface, adapted #000000 -> light ink).
	var upperSum float64
	var upperMax float64
	upperCount := 0
	for y := b.Min.Y + 28; y < b.Min.Y+90 && y < b.Max.Y; y++ {
		for x := b.Min.X + 36; x < b.Max.X-36; x++ {
			l := pxLum(img.At(x, y))
			upperSum += l
			upperMax = max(upperMax, l)
			upperCount++
		}
	}
	if upperCount == 0 {
		t.Fatal("empty upper sample region")
	}
	if avgLum := upperSum / float64(upperCount); avgLum >= 0.25 {
		t.Fatalf("expected adapted dark card interior avg luminance < 0.25, got %.3f", avgLum)
	}
	if upperMax <= 0.45 {
		t.Fatalf("expected adapted black text in dark card to produce bright readable pixels (max lum > 0.45), got %.3f", upperMax)
	}

	// Sample lower region where the rawColor: true white box lives; it should keep white pixels (> 0.85).
	var lowerMax float64
	for y := b.Min.Y + 130; y < b.Max.Y-20; y++ {
		for x := b.Min.X + 36; x < b.Max.X-36; x++ {
			lowerMax = max(lowerMax, pxLum(img.At(x, y)))
		}
	}
	if lowerMax <= 0.85 {
		t.Fatalf("expected rawColor:true box to retain white pixels (max lum > 0.85), got %.3f", lowerMax)
	}

	// AuditSnapshot should warn on hardcoded #111827, but ignore nodes marked rawColor: true.
	auditWarnSnap := bar.Snapshot{
		Panel: &bar.Node{
			Type:  "Column",
			Props: map[string]any{"preferredWidth": float64(300)},
			Children: []*bar.Node{
				{Type: "Text", Props: map[string]any{"textColor": "#111827"}, Text: "Hardcoded"},
			},
		},
	}
	if audit := AuditSnapshot(auditWarnSnap, bar.Spec{ID: "hazard"}); len(audit.Warnings) == 0 {
		t.Fatalf("expected AuditSnapshot warning for hardcoded #111827")
	} else if !strings.Contains(audit.Warnings[0], "#111827") {
		t.Fatalf("unexpected audit warning: %v", audit.Warnings)
	}

	auditRawSnap := bar.Snapshot{
		Panel: &bar.Node{
			Type:  "Column",
			Props: map[string]any{"preferredWidth": float64(300)},
			Children: []*bar.Node{
				{Type: "Text", Props: map[string]any{"rawColor": true, "textColor": "#111827"}, Text: "RawOK"},
			},
		},
	}
	if audit := AuditSnapshot(auditRawSnap, bar.Spec{ID: "raw-ok"}); len(audit.Warnings) > 0 {
		t.Fatalf("expected no hardcoded warning when rawColor:true is set, got %v", audit.Warnings)
	}
}
