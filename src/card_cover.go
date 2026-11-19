package main

import (
	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

// A card with pictures is the picture, edge to edge, when its layout says so
// (layout image: "cover", the default). Its text and buttons are over the
// picture, in panels that show while the pointer is on the card. A script that
// wants the picture among its other contents says layout image: "inline".

// cover is the picture a card is filled with: the one it is at, and the others.
type cover struct {
	bm    *ui.Bitmap
	shown []engine.Image
	idx   int
}

// coverOf is the picture to fill a card with, or nil: not allowed, no picture,
// or one that cannot be decoded (the card then shows it inline, with its note).
func (a *app) coverOf(sc store.Script, res engine.Result, allowed bool) *cover {
	if !allowed {
		return nil
	}
	shown, idx, ok := a.carouselPos(sc, res.Images)
	if !ok {
		return nil
	}
	bm := a.bitmapOf(shown[idx].Local)
	if bm == nil {
		return nil
	}
	return &cover{bm: bm, shown: shown, idx: idx}
}

// touchUI says whether the screen has no pointer to hover with: a phone.
func (a *app) touchUI() bool { return isAndroid() }

// coverCard fills the card with its picture, and puts its text and buttons over
// it in two panels, the buttons at the top and the title, summary and the
// picture's controls at the bottom. Hidden, the panels stay built, transparent
// and out of the pointer's way, so a click is the card's own.
func (a *app) coverCard(c *ui.Context, sc store.Script, cv *cover, res engine.Result, lay engine.Layout, title string, size int, show bool) {
	p := pal(c)
	ui.Image(c, cv.bm).Fit(ui.Cover).Absolute().Fill().Radius(radCard)
	over := ui.Column(c).Absolute().Fill().Padding(10).Gap(6)
	if !show {
		over = over.Opacity(0).IgnoreInput(true)
	}
	over.Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Spacer(c)
			ui.Row(c).AlignItems(ui.Center).Gap(2).Padding(2, 4).Radius(radControl).Background(p.card.Alpha(0.9)).Children(func() {
				a.refreshButton(c, sc, true)
				a.sizeButton(c, sc, size, true)
			})
		})
		ui.Spacer(c)
		ui.Column(c).Gap(4).Padding(10).Radius(radControl + 2).Background(p.card.Alpha(0.92)).Children(func() {
			ui.Text(c, title).FontSize(fsCaption).FontWeight(600).NoWrap()
			if lay.Summary != "hide" && res.Sub != "" {
				ui.Text(c, res.Sub).FontSize(fsSmall).TextColor(p.muted).NoWrap()
			}
			a.carouselRows(c, sc, cv.shown, cv.idx)
		})
	})
}

// sizeButton is the button that grows a card to the next size. show false hides
// it until the card is hovered, as the refresh button is.
func (a *app) sizeButton(c *ui.Context, sc store.Script, size int, show bool) {
	el := action.ActionIcon(c, icon("maximize-2"), action.ActionIconProps{Size: theme.Small, Title: tr("右键切换大小")})
	if !show {
		el = el.Opacity(0).IgnoreInput(true)
	}
	if el.Clicked() {
		next := (size + 1) % 3
		if size >= 3 {
			next = 0
		}
		a.setSize(sc.ID, next)
	}
}
