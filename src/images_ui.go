package main

import (
	"fmt"
	"os"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

// imageMaxSide is the longest side a picture is decoded at: enough for the full
// view, and small enough to keep a long history light.
const imageMaxSide = 960

// bitmapOf is the decoded picture at path, decoded once. A file that does not
// decode is remembered as nil, so that it is not tried on every frame.
func (a *app) bitmapOf(path string) *ui.Bitmap {
	if a.bitmaps == nil {
		a.bitmaps = map[string]*ui.Bitmap{}
	}
	if bm, ok := a.bitmaps[path]; ok {
		return bm
	}
	var bm *ui.Bitmap
	if data, err := os.ReadFile(path); err == nil {
		if decoded, err := ui.DecodeBitmapScaled(data, imageMaxSide); err == nil {
			bm = decoded
		}
	}
	a.bitmaps[path] = bm
	return bm
}

// shownImages are the pictures that have a copy on this computer, in order.
func shownImages(imgs []engine.Image) []engine.Image {
	var out []engine.Image
	for _, img := range imgs {
		if img.Local != "" {
			out = append(out, img)
		}
	}
	return out
}

// imageCarousel shows one of a script's pictures, at width w and height h, with
// the arrows that step along them, the caption under it, and its place in the
// list. The position is kept per script.
func (a *app) imageCarousel(c *ui.Context, sc store.Script, imgs []engine.Image, w, h float32) {
	shown := shownImages(imgs)
	if len(shown) == 0 {
		return
	}
	if a.carouselIdx == nil {
		a.carouselIdx = map[string]int{}
	}
	idx := a.carouselIdx[sc.ID]
	idx = ((idx % len(shown)) + len(shown)) % len(shown)
	a.carouselIdx[sc.ID] = idx
	cur := shown[idx]
	p := pal(c)
	ui.Column(c).Gap(6).Children(func() {
		if bm := a.bitmapOf(cur.Local); bm != nil {
			ui.Image(c, bm).Fit(ui.Cover).Width(w).Height(h).Radius(radControl)
		} else {
			ui.Column(c).Width(w).Height(h).Radius(radControl).Background(p.chip).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
				ui.Text(c, tr("图片不能显示")).FontSize(fsSmall).TextColor(p.muted)
			})
		}
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			if btn(c, "‹").Clicked() {
				a.carouselIdx[sc.ID] = idx - 1
			}
			ui.Text(c, cur.Caption).FontSize(fsSmall).TextColor(p.muted).Grow(1).MinWidth(0).NoWrap()
			ui.Text(c, fmt.Sprintf("%d / %d", idx+1, len(shown))).FontSize(fsTiny).TextColor(p.faint)
			if btn(c, "›").Clicked() {
				a.carouselIdx[sc.ID] = idx + 1
			}
		})
		// The picture's own actions, as QX's gallery has them: its source page, and
		// a copy of its link.
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			if cur.Source != "" && btn(c, tr("打开来源")).Clicked() {
				if err := mygo.Shell.OpenExternal(cur.Source); err != nil {
					a.status = err.Error()
				}
			}
			if btn(c, tr("复制链接")).Clicked() {
				copyText(cur.URL)
				a.status = tr("已复制")
			}
		})
	})
}
