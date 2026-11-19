package main

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
)

// The size guide: after a card changes size, the four sizes are drawn to scale
// as SVG outlines for a few seconds, dashed, with the chosen one solid, so that
// the user sees what the size means. Widths and heights are the cards' own, at
// guideScale.

const (
	// guideScale turns a card's DIPs into the guide's.
	guideScale = 0.2
	// guideDuration is how long the guide stays after a size is chosen.
	guideDuration = 4 * time.Second
)

// sizeNames are the sizes in the order of the card sizes: small, medium, large
// and wide, which spans a whole row.
func sizeNames() []string { return []string{tr("小"), tr("中"), tr("大"), tr("宽")} }

// sizeDims is a size's box in the guide, in DIPs.
func sizeDims(size int) (w, h float64) {
	if size == 3 {
		// Wide spans a row: two large cards' widths, the medium card's height.
		return 2 * float64(cardSizes[2][0]) * guideScale, float64(cardSizes[1][1]) * guideScale
	}
	return float64(cardSizes[size][0]) * guideScale, float64(cardSizes[size][1]) * guideScale
}

// sizeGuideSVG is the outline of one size as SVG: a dashed rounded rectangle,
// or a solid one tinted with the accent when it is the chosen size.
func sizeGuideSVG(size int, chosen bool) []byte {
	w, h := sizeDims(size)
	stroke, fill, opacity, dash, sw := "#888888", "none", "1", `stroke-dasharray="4 3"`, 1.0
	if chosen {
		stroke, fill, opacity, dash, sw = "#3D4E80", "#3D4E80", "0.14", "", 2.0
	}
	return []byte(fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.1f %.1f" width="%.1f" height="%.1f">`+
			`<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" rx="3" fill="%s" fill-opacity="%s" stroke="%s" stroke-width="%.1f" %s/>`+
			`</svg>`,
		w, h, w, h, sw/2, sw/2, w-sw, h-sw, fill, opacity, stroke, sw, dash))
}

// showSizeGuide shows the guide for the size just chosen.
func (a *app) showSizeGuide(size int) {
	a.guideSize = size
	a.guideUntil = a.now().Add(guideDuration)
}

// sizeGuide draws the guide above the cards while it is up, and asks for new
// frames until it goes, so that it disappears on its own.
func (a *app) sizeGuide(c *ui.Context) {
	if a.guideUntil.IsZero() || !a.now().Before(a.guideUntil) {
		return
	}
	c.Invalidate()
	p := pal(c)
	names := sizeNames()
	ui.Row(c).AlignItems(ui.End).Gap(18).Wrap().Children(func() {
		for i := range names {
			i := i
			if a.guideSVG[i] == nil {
				a.guideSVG[i] = ui.MustParseSVG(sizeGuideSVG(i, false))
			}
			chosen := i == a.guideSize
			svg := a.guideSVG[i]
			if chosen {
				svg = ui.MustParseSVG(sizeGuideSVG(i, true))
			}
			w, h := sizeDims(i)
			ui.Column(c).AlignItems(ui.Center).Gap(4).Children(func() {
				ui.Icon(c, svg).Size(float32(w), float32(h))
				ui.Text(c, names[i]).FontSize(fsTiny).TextColor(map[bool]ui.Color{true: p.accent, false: p.muted}[chosen])
			})
		}
	})
}
