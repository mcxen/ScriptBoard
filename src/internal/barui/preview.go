package barui

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"sync"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/bar"
	"scriptboard/src/internal/colortheme"
)

// PreviewOptions configures headless PNG rendering of a .bar Snapshot.
type PreviewOptions struct {
	Dark        bool
	Vibrancy    bool
	Width       int
	Height      int
	Scale       float32
	Material    string
	LoadingText string
	IconLookup  func(name string) *ui.SVG
}

// PreviewResult contains the PNG bytes and layout measurements from
// RenderPreview.
type PreviewResult struct {
	PNG            []byte   `json:"-"`
	Width          int      `json:"width"`
	Height         int      `json:"height"`
	MeasuredHeight int      `json:"measuredHeight"`
	PixelWidth     int      `json:"pixelWidth"`
	PixelHeight    int      `json:"pixelHeight"`
	Scale          float32  `json:"scale"`
	Texts          []string `json:"texts,omitempty"`
}

// VisualAudit summarizes structural, dark/light contrast, and sizing feedback
// for a rendered .bar Snapshot.
type VisualAudit struct {
	OK               bool     `json:"ok"`
	NodeCount        int      `json:"nodeCount"`
	MaxDepth         int      `json:"maxDepth"`
	ActionCount      int      `json:"actionCount"`
	TextCount        int      `json:"textCount"`
	HasVisualization bool     `json:"hasVisualization"`
	HasInteractive   bool     `json:"hasInteractive"`
	Warnings         []string `json:"warnings,omitempty"`
}

var previewMu sync.Mutex

// RenderPreview renders snap offscreen via ui.NewTester and returns a cleanly
// framed PNG suitable for visual inspection by MCP clients or tests.
func RenderPreview(snap bar.Snapshot, opts PreviewOptions) (PreviewResult, error) {
	previewMu.Lock()
	defer previewMu.Unlock()

	panelW := opts.Width
	if panelW <= 0 && snap.Panel != nil {
		if pw, ok := PropFloat(snap.Panel.Props, "preferredWidth"); ok && pw > 0 {
			panelW = int(pw)
		}
	}
	if panelW <= 0 {
		panelW = 300
	}
	if panelW < 220 {
		panelW = 220
	} else if panelW > 640 {
		panelW = 640
	}

	const pad = 16
	explicitH := opts.Height
	if explicitH <= 0 && snap.Panel != nil {
		if ph, ok := PropFloat(snap.Panel.Props, "preferredHeight"); ok && ph > 0 {
			explicitH = int(ph)
		}
	}
	canvasW := panelW + pad*2
	canvasH := 920
	if explicitH > 0 {
		if explicitH < 120 {
			explicitH = 120
		} else if explicitH > 960 {
			explicitH = 960
		}
		canvasH = explicitH + pad*2
	}

	scale := opts.Scale
	if scale <= 0 {
		scale = 2
	} else if scale > 3 {
		scale = 3
	}

	th := ThemeFor(opts.Dark, opts.Vibrancy)
	stageBG := ui.Hex("#EAEFF6")
	if opts.Dark {
		stageBG = ui.Hex("#0B0E14")
	}

	view := func(c *ui.Context) {
		ui.Column(c).Fill().Background(stageBG).Padding(pad).AlignItems(ui.Center).Children(func() {
			RenderPanel(c, snap, th, RenderOptions{
				Width:       float32(panelW),
				Material:    opts.Material,
				LoadingText: opts.LoadingText,
				IconLookup:  opts.IconLookup,
			})
		})
	}

	tt := ui.NewTester(view, canvasW, canvasH)
	tt.SetDark(opts.Dark)
	tt.SetScale(scale)
	tt.Frame()
	tt.Frame()

	img := tt.Image()
	if img == nil {
		return PreviewResult{}, fmt.Errorf("failed to render preview image")
	}

	cropped := img
	measuredLogicalH := explicitH
	if explicitH <= 0 {
		cropped, measuredLogicalH = cropStageImage(img, pad, scale)
	}
	b := cropped.Bounds()
	var buf bytes.Buffer
	if err := png.Encode(&buf, cropped); err != nil {
		return PreviewResult{}, err
	}

	logicalH := canvasH
	if explicitH <= 0 {
		logicalH = measuredLogicalH + pad*2
	}
	return PreviewResult{
		PNG:            buf.Bytes(),
		Width:          panelW,
		Height:         logicalH,
		MeasuredHeight: measuredLogicalH,
		PixelWidth:     b.Dx(),
		PixelHeight:    b.Dy(),
		Scale:          scale,
		Texts:          tt.Texts(),
	}, nil
}

func cropStageImage(img *image.RGBA, pad int, scale float32) (*image.RGBA, int) {
	b := img.Bounds()
	bg := img.RGBAAt(b.Min.X, b.Min.Y)
	diff := func(a, c uint8) int {
		if a > c {
			return int(a - c)
		}
		return int(c - a)
	}
	lastY := b.Min.Y
	for y := b.Max.Y - 1; y >= b.Min.Y; y-- {
		found := false
		for x := b.Min.X; x < b.Max.X; x++ {
			px := img.RGBAAt(x, y)
			if diff(px.R, bg.R) > 3 || diff(px.G, bg.G) > 3 || diff(px.B, bg.B) > 3 {
				found = true
				break
			}
		}
		if found {
			lastY = y
			break
		}
	}
	padPx := int(float32(pad) * scale)
	bottomY := lastY + padPx + 2
	minH := padPx*2 + int(60*scale)
	if bottomY < b.Min.Y+minH {
		bottomY = b.Min.Y + minH
	}
	if bottomY > b.Max.Y {
		bottomY = b.Max.Y
	}
	sub, ok := img.SubImage(image.Rect(b.Min.X, b.Min.Y, b.Max.X, bottomY)).(*image.RGBA)
	if !ok {
		return img, int(float32(b.Dy())/scale) - pad*2
	}
	panelPx := (lastY - b.Min.Y + 1) - padPx
	if panelPx < 0 {
		panelPx = 0
	}
	measuredDIP := int(float32(panelPx) / scale)
	return sub, measuredDIP
}

// AuditSnapshot inspects a rendered Snapshot and Spec for visual layout,
// accessibility, and Light/Dark theme pitfalls.
func AuditSnapshot(snap bar.Snapshot, spec bar.Spec) VisualAudit {
	var a VisualAudit
	if snap.Error != "" {
		a.Warnings = append(a.Warnings, fmt.Sprintf("runtime error present: %s", snap.Error))
	}
	if snap.Tray != nil && utf8.RuneCountInString(snap.Tray.Title) > 12 {
		a.Warnings = append(a.Warnings, fmt.Sprintf("tray.title %q (%d chars) exceeds 12 chars and will truncate in macOS menu bar", snap.Tray.Title, utf8.RuneCountInString(snap.Tray.Title)))
	}
	if snap.Panel == nil {
		a.Warnings = append(a.Warnings, "panel is empty; component has no floating popover UI")
		a.OK = len(a.Warnings) == 0
		return a
	}

	if _, ok := PropFloat(snap.Panel.Props, "preferredWidth"); !ok {
		a.Warnings = append(a.Warnings, "root panel omits preferredWidth (defaults to 300px; set preferredWidth explicitly for predictable framing)")
	}

	hardcodedWarned := false
	tinyFontWarned := false
	borderedCards := 0
	actionsSeen := map[string]bool{}
	var walk func(n *bar.Node, depth int)
	walk = func(n *bar.Node, depth int) {
		if n == nil {
			return
		}
		a.NodeCount++
		if depth > a.MaxDepth {
			a.MaxDepth = depth
		}
		for _, id := range []string{n.OnClickID, n.OnChangeID, n.OnSubmitID} {
			if id != "" && !actionsSeen[id] {
				actionsSeen[id] = true
				a.ActionCount++
			}
		}
		switch n.Type {
		case "Text":
			a.TextCount++
			if fs, ok := PropFloat(n.Props, "fontSize"); ok && fs > 0 && fs < 10 && !tinyFontWarned {
				tinyFontWarned = true
				a.Warnings = append(a.Warnings, fmt.Sprintf("fontSize %.1f on Text %q is below 10px readability floor", fs, n.Text))
			}
		case "Ring", "Progress", "Chart":
			a.HasVisualization = true
		case "Button", "Segmented", "Switch", "ListRow", "Input":
			a.HasInteractive = true
		}

		if bg, ok := PropString(n.Props, "background"); ok && (bg == "surfaceCard" || bg == "card") {
			if bw, okB := PropFloatSlice(n.Props, "borderWidth"); okB {
				hasBorder := false
				for _, v := range bw {
					if v > 0 {
						hasBorder = true
						break
					}
				}
				if hasBorder {
					borderedCards++
				}
			}
		}

		isRaw := propBool(n.Props, "rawColor", false)
		if v, ok := n.Props["adaptColors"].(bool); ok && !v {
			isRaw = true
		}
		if !isRaw {
			for _, key := range []string{"background", "textColor", "color", "borderColor", "valueColor", "iconBg"} {
				if raw, ok := PropString(n.Props, key); ok {
					if colortheme.IsHardcodedAdaptiveHazard(raw) && !hardcodedWarned {
						hardcodedWarned = true
						a.Warnings = append(a.Warnings, fmt.Sprintf("hardcoded %q color %q breaks Light/Dark adaptation; prefer semantic tokens (surfaceCard, text, textMuted, accent)", key, raw))
					}
				}
			}
		}
		for _, ch := range n.Children {
			walk(ch, depth+1)
		}
	}
	walk(snap.Panel, 1)

	if snap.Tray != nil {
		for _, m := range snap.Tray.Menu {
			if m.ActionID != "" && !actionsSeen[m.ActionID] {
				actionsSeen[m.ActionID] = true
				a.ActionCount++
			}
		}
	}
	if a.MaxDepth > 12 {
		a.Warnings = append(a.Warnings, fmt.Sprintf("deep VDOM nesting (depth %d); consider extracting $ui.card / ListRow / $ui.statGrid", a.MaxDepth))
	}
	if borderedCards > 4 {
		a.Warnings = append(a.Warnings, fmt.Sprintf("panel contains %d bordered surfaceCard containers; prefer frameless spacing and $ui.divider() for native translucency", borderedCards))
	}
	_ = spec
	a.OK = len(a.Warnings) == 0
	return a
}
