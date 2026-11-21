package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

// imageMaxSide is the longest side a picture is decoded at for cards and
// thumbnails; full-screen gallery views decode up to imageFullMaxSide.
const (
	imageMaxSide     = 960
	imageFullMaxSide = 1920
)

// bitmapOf is the decoded picture at path for cards, decoded once.
func (a *app) bitmapOf(path string) *ui.Bitmap {
	return a.bitmapOfMax(path, imageMaxSide)
}

// bitmapOfMax decodes path at up to maxSide pixels on its longest edge,
// caching the result per (maxSide, path). A file that does not decode is
// remembered as nil so it is not retried every frame.
func (a *app) bitmapOfMax(path string, maxSide int) *ui.Bitmap {
	if a.bitmaps == nil {
		a.bitmaps = map[string]*ui.Bitmap{}
	}
	key := path
	if maxSide != imageMaxSide {
		key = fmt.Sprintf("%d:%s", maxSide, path)
	}
	if bm, ok := a.bitmaps[key]; ok {
		return bm
	}
	var bm *ui.Bitmap
	if data, err := os.ReadFile(path); err == nil {
		if decoded, err := ui.DecodeBitmapScaled(data, maxSide); err == nil {
			bm = decoded
		}
	}
	a.bitmaps[key] = bm
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
	shown, idx, ok := a.carouselPos(sc, imgs)
	if !ok {
		return
	}
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
		a.carouselRows(c, sc, shown, idx)
	})
}

// carouselPos is the pictures a card can show, in order, and the one it is at
// (kept per script, and wrapped round). ok is false when there is none.
func (a *app) carouselPos(sc store.Script, imgs []engine.Image) (shown []engine.Image, idx int, ok bool) {
	shown = shownImages(imgs)
	if len(shown) == 0 {
		return nil, 0, false
	}
	if a.carouselIdx == nil {
		a.carouselIdx = map[string]int{}
	}
	idx = ((a.carouselIdx[sc.ID] % len(shown)) + len(shown)) % len(shown)
	a.carouselIdx[sc.ID] = idx
	return shown, idx, true
}

// carouselRows are the controls under a picture: the caption between the arrows
// that move through the pictures, and the picture's own actions, as QX's gallery
// has them: its source page, and a copy of its link.
func (a *app) carouselRows(c *ui.Context, sc store.Script, shown []engine.Image, idx int) {
	cur := shown[idx]
	p := pal(c)
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
	ui.Row(c).AlignItems(ui.Center).Gap(6).Wrap().Children(func() {
		if cur.Local != "" && btn(c, tr("保存")).Clicked() {
			a.saveImageFile(cur)
		}
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
}

// imageGalleryFull draws a photo-first gallery layout for image-centric
// scripts (such as Bing wallpapers) in fullView: a large full-bleed hero image
// as the centerpiece, subtle floating navigation arrows and counter, quiet
// secondary caption + Save/Copy/Source actions below, and a horizontal
// thumbnail filmstrip when multiple pictures are cached.
func (a *app) imageGalleryFull(c *ui.Context, sc store.Script, res engine.Result, compact bool) {
	shown, idx, ok := a.carouselPos(sc, res.Images)
	if !ok {
		return
	}
	cur := shown[idx]
	p := pal(c)

	// Size the cinema stage so the wallpaper is the unmistakable centerpiece
	// while keeping the caption, save button and thumbnail strip in view.
	contentW := a.contentW()
	stageH := clampF(contentW*9/16, 360, max(420, a.height-240))
	if compact {
		stageH = clampF(contentW*9/16, 220, 320)
	}

	card(c).Padding(0).Gap(0).Children(func() {
		// 1. Hero image stage (dark cinema backdrop, Contain fit so 16:9 or
		// portrait wallpapers are never cropped).
		stage := ui.Box(c).Height(stageH).Background(ui.Hex("#0B0F19")).Radius(radCard, radCard, 0, 0).Clip()
		stage.Children(func() {
			if bm := a.bitmapOfMax(cur.Local, imageFullMaxSide); bm != nil {
				ui.Image(c, bm).Fit(ui.Contain).Absolute().Fill()
			} else {
				ui.Column(c).Absolute().Fill().AlignItems(ui.Center).Justify(ui.Center).Children(func() {
					ui.Text(c, tr("图片不能显示")).FontSize(fsSmall).TextColor(ui.Hex("#94A3B8"))
				})
			}

			// Floating prev/next navigation pills over the left/right edges.
			if len(shown) > 1 {
				navY := max(8, (stageH-36)/2)
				navBtn := func(symbol string, delta int, right bool) {
					b := ui.ButtonBase(c).Absolute().Top(navY).Size(36, 36).Radius(18).
						Background(ui.Hex("#0F172A").Alpha(0.62)).
						BorderWidth(1).BorderColor(ui.Hex("#FFFFFF").Alpha(0.18)).
						AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
					if right {
						b = b.Right(14)
					} else {
						b = b.Left(14)
					}
					if b.Clicked() {
						a.carouselIdx[sc.ID] = idx + delta
					}
					b.Children(func() {
						ui.Text(c, symbol).FontSize(18).Bold().TextColor(ui.Hex("#FFFFFF")).NoWrap()
					})
				}
				navBtn("‹", -1, false)
				navBtn("›", 1, true)
			}

			// Quiet counter badge at the bottom-right corner of the photo.
			ui.Row(c).Absolute().Right(14).Bottom(12).Padding(4, 10).Radius(99).
				Background(ui.Hex("#0F172A").Alpha(0.68)).AlignItems(ui.Center).Children(func() {
				ui.Text(c, fmt.Sprintf("%d / %d", idx+1, len(shown))).FontSize(fsTiny).FontWeight(600).TextColor(ui.Hex("#FFFFFF")).NoWrap()
			})
		})

		// 2. Quiet secondary caption + primary Save action bar below the photo.
		barPadY, barPadX := float32(14), float32(18)
		if compact {
			barPadY, barPadX = 12, 14
		}
		ui.Row(c).AlignItems(ui.Center).Gap(14).Padding(barPadY, barPadX).Wrap().Children(func() {
			ui.Column(c).Gap(3).Grow(1).MinWidth(200).Children(func() {
				capText := cur.Caption
				if capText == "" {
					capText = res.Title
				}
				ui.Text(c, capText).FontSize(fsBody).FontWeight(550).LineHeight(1.35)
				note := res.Sub
				noteColor := p.muted
				if a.status != "" {
					note = a.status
					noteColor = p.accent
				}
				if note != "" {
					ui.Text(c, note).FontSize(fsTiny).TextColor(noteColor)
				}
			})
			ui.Row(c).AlignItems(ui.Center).Gap(8).Wrap().Shrink(0).Children(func() {
				if cur.Local != "" && primaryBtn(c, tr("保存图片")).Clicked() {
					a.saveImageFile(cur)
				}
				if btn(c, tr("复制链接")).Clicked() {
					copyText(cur.URL)
					a.status = tr("已复制")
				}
				if cur.Source != "" && btn(c, tr("打开来源")).Clicked() {
					if err := mygo.Shell.OpenExternal(cur.Source); err != nil {
						a.status = err.Error()
					}
				}
				if btn(c, tr("清空图片历史")).Clicked() {
					a.clearImages(sc)
				}
			})
		})

		// 3. Horizontal thumbnail filmstrip for quick browsing.
		if len(shown) > 1 {
			ui.Box(c).Height(1).Shrink(0).Background(p.hairline)
			ui.ScrollHorizontal(c).Padding(10, barPadX).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					for i, img := range shown {
						i, img := i, img
						sel := i == idx
						thumb := ui.ButtonBase(c.Key(fmt.Sprintf("thumb:%s:%d", sc.ID, i))).
							Size(88, 50).Radius(6).Clip().Shrink(0).Cursor(ui.CursorPointer)
						if sel {
							thumb = thumb.BorderWidth(2).BorderColor(p.accent)
						} else {
							thumb = thumb.Opacity(0.68).BorderWidth(1).BorderColor(p.hairline)
						}
						if thumb.Clicked() {
							a.carouselIdx[sc.ID] = i
						}
						thumb.Children(func() {
							if bm := a.bitmapOf(img.Local); bm != nil {
								ui.Image(c, bm).Fit(ui.Cover).Absolute().Fill().PassThrough()
							} else {
								ui.Box(c).Absolute().Fill().Background(p.chip).PassThrough()
							}
						})
					}
				})
			})
		}
	})
}

// imageFilename turns an image's caption and raw bytes into a safe default
// file name with the matching image extension.
func imageFilename(img engine.Image, data []byte) string {
	ext := ".jpg"
	switch http.DetectContentType(data) {
	case "image/png":
		ext = ".png"
	case "image/webp":
		ext = ".webp"
	case "image/gif":
		ext = ".gif"
	case "image/jpeg":
		ext = ".jpg"
	default:
		if uExt := strings.ToLower(filepath.Ext(strings.SplitN(img.URL, "?", 2)[0])); uExt == ".png" || uExt == ".jpg" || uExt == ".jpeg" || uExt == ".webp" {
			ext = uExt
		}
	}
	base := strings.TrimSpace(img.Caption)
	if base == "" {
		base = "wallpaper"
	}
	repl := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "*", "", "?", "", "\"", "", "<", "", ">", "", "|", "", " · ", "-")
	base = strings.TrimSpace(repl.Replace(base))
	if r := []rune(base); len(r) > 64 {
		base = string(r[:64])
	}
	if base == "" {
		base = "wallpaper"
	}
	return base + ext
}

// writeImageCopy copies the cached full-resolution image file at srcPath to
// dstPath, preserving the original downloaded bytes.
func writeImageCopy(srcPath, dstPath string) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}
	return os.WriteFile(dstPath, data, 0o644)
}

// saveImageFile opens a native save dialog and writes the original cached
// wallpaper bytes to the chosen destination.
func (a *app) saveImageFile(img engine.Image) {
	if img.Local == "" {
		a.status = tr("图片不能显示")
		return
	}
	data, err := os.ReadFile(img.Local)
	if err != nil {
		a.status = tr("保存失败：") + err.Error()
		return
	}
	defName := imageFilename(img, data)
	srcPath := img.Local
	a.background(func() (string, error) {
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Title:       tr("保存图片"),
			DefaultPath: defName,
			Filters:     []mygo.FileFilter{{Name: tr("图片文件"), Extensions: []string{"jpg", "jpeg", "png", "webp"}}},
		})
		if err != nil || path == "" {
			return "", err
		}
		return path, writeImageCopy(srcPath, path)
	}, func(path string, err error) {
		switch {
		case err != nil:
			a.status = tr("保存失败：") + err.Error()
		case path != "":
			a.status = tr("已保存到 ") + path
		}
	})
}
