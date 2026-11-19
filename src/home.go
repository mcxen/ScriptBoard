package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

// cardSizes are the base widths and heights of the home cards: small shows
// the headline only, medium adds a curve, large a chart you can hover over.
var cardSizes = [4][2]float32{
	{170, 128},
	{300, 208},
	{620, 400},
	{0, 208}, // wide: the width is the whole row's
}

const cardGap = 16

var weekdays = []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}

// scale is the card scale the user chose, from 0.7 to 1.6.
func (a *app) scale() float32 {
	s := a.set.CardScale
	if s <= 0 {
		s = 1
	}
	return float32(math.Min(1.6, math.Max(0.7, s)))
}

// contentW is the width a page has for its content.
func (a *app) contentW() float32 {
	w := float32(960)
	if a.width > 0 {
		w = a.width
	}
	if !a.compact {
		w -= a.sidebarW()
	}
	return w - 2*pad(a.compact)
}

// sizeOf is the size of a home card: its own when the user gave it one, else
// the default.
func (a *app) sizeOf(id string) int {
	if n, ok := a.set.CardSizes[id]; ok && n >= 0 && n <= 3 {
		return n
	}
	return a.cardSize
}

// home is the overview: the cards on a grid, in the order the user arranged
// them. In edit mode a card moves by its body and is resized by its corner.
func (a *app) home(c *ui.Context, compact bool) {
	p := pal(c)
	now := a.now()
	scripts := a.orderedHome(a.homeScripts())
	subtitle := fmt.Sprintf(tr("%d月%d日 %s · %d 个组件"), int(now.Month()), now.Day(), tr(weekdays[now.Weekday()]), len(scripts))
	pageFull(c, compact, func() {
		// A phone has room for the size switch under the title, not beside it;
		// the scale slider is for a window with room to scale into.
		sizeControls := func() {
			if !compact {
				ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Icon(c, icon("sparkles")).FontSize(15).TextColor(p.muted)
					if slider(c, &a.set.CardScale, 0.7, 1.6).Width(104).Tooltip(tr("图标与卡片大小 ") + fmtScale(a.scale())).Changed() {
						a.set.CardScale = float64(a.scale())
						if err := a.store.SetSettings(a.set); err != nil {
							a.set = a.store.Settings()
						}
					}
				})
			}
			if choose(c, &a.cardSize, tr("小"), tr("中"), tr("大")) {
				a.setAllSizes(a.cardSize)
			}
		}
		editControls := func() {
			if a.editing {
				if primaryBtn(c, tr("完成")).Clicked() {
					a.editing = false
					a.rz = nil
				}
				return
			}
			if btn(c, tr("编辑布局")).Clicked() {
				a.editing = true
			}
		}
		if compact {
			header(c, tr("总览"), subtitle, nil)
			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				label(c, tr("卡片大小"))
				sizeControls()
			})
		} else {
			header(c, tr("总览"), subtitle, func() {
				sizeControls()
				editControls()
			})
		}
		if len(scripts) == 0 {
			card(c).Children(func() {
				empty(c, "layout-grid", tr("还没有组件"), tr("在「编辑」里新建脚本，或到「社区」下载别人写好的。"))
			})
			return
		}
		if a.editing {
			hint(c, tr("拖动卡片换位置；拖右下角的角调整大小。右键卡片也能选大小。"))
		}
		a.homeIDs = a.homeIDs[:0]
		items := make([]placed, 0, len(scripts))
		for _, sc := range scripts {
			a.homeIDs = append(a.homeIDs, sc.ID)
			items = append(items, placed{sc: sc, box: a.boxOf(sc.ID)})
		}
		a.cols = colsFor(a.contentW(), compact)
		a.homeGrid(c, items)
	})
}

// fmtScale shows a scale as a percentage, such as 120%.
func fmtScale(s float32) string {
	return fmt.Sprintf("%d%%", int(math.Round(float64(s)*100)))
}

// card draws one script as a widget in its box: span is its columns on the
// page, w its width in DIPs.
func (a *app) card(c *ui.Context, sc store.Script, b store.CardBox, span int, w float32) {
	p := pal(c)
	s := a.scale()
	meta, metaErr := a.metaOf(sc)
	idx, vals := a.viewOf(sc.ID, meta.Dims)
	st := &runState{err: metaErr}
	if metaErr == nil {
		st = a.ensure(a.runKeyOf(sc.ID, sc, meta, vals), sc, vals)
	}

	size := classOf(b)
	// A card with values to type in has room for them.
	inputs := meta.Inputs
	if size == 1 && len(inputs) > 1 {
		inputs = inputs[:1]
	}
	if size == 0 {
		inputs = nil
	}
	h := b.Height*s + float32(len(inputs))*44*s
	ready := st.err == nil && !st.loading
	accent := p.accent
	if ready {
		accent = accentOf(st.res, p.accent)
	}
	title, iconName := tr(sc.Name), meta.Icon
	if ready {
		if st.res.Title != "" {
			title = st.res.Title
		}
		if st.res.Icon != "" {
			iconName = st.res.Icon
		}
	}
	bigSize := clampF(30*s, 22, 44)
	// What is left of the header for the title: the card less its padding, the
	// badge, the two buttons, a tag when there is one, and the gaps.
	hasTag := ready && (size > 0 || (st.res.Chart != nil && st.res.Chart.Type == "gauge"))
	titleW := w - 32*s - clampF(30*s, 24, 44) - 2*24 - 10*3 + 12
	if hasTag {
		titleW -= 56 + 10
	}
	title = fitText(title, max(titleW, 40), fsCaption)

	el := card(c).Width(w).Height(h).Padding(16 * s).Gap(6 * s).Cursor(ui.CursorPointer).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
			iconBadge(c, iconName, accent, clampF(30*s, 24, 44))
			// The title gives way, with an ellipsis, so the tag and the buttons
			// stay whole on a small card.
			ui.Row(c).Grow(1).Shrink(1).MinWidth(0).ClipX().Children(func() {
				ui.Text(c, title).FontSize(fsCaption).TextColor(p.muted).NoWrap()
			})
			if g := st.res.Chart; ready && g != nil && g.Type == "gauge" && g.Gauge != nil {
				_, frac := remaining(g.Gauge)
				display.Tag(c, fmt.Sprintf(tr("剩余 %.0f%%"), frac*100), display.TagProps{Color: tagFor(frac), Size: theme.Small}).Shrink(0)
			} else if ready && size > 0 {
				if cs := summarize(st.res.Chart); cs.ok {
					deltaChip(c, cs.pct).Shrink(0)
				}
			}
			a.refreshButton(c, sc)
			if action.ActionIcon(c, icon("maximize-2"), action.ActionIconProps{Size: theme.Small, Title: tr("右键切换大小")}).Clicked() {
				a.setSize(sc.ID, (size+1)%3)
			}
		})
		switch {
		case st.loading:
			ui.Spacer(c)
			ui.Text(c, tr("加载中…")).FontSize(fsSmall).TextColor(p.muted)
		case st.err != nil:
			ui.Text(c, st.err.Error()).FontSize(fsTiny).TextColor(p.up)
		default:
			ui.Text(c, tickText(c, "card:"+sc.ID, st.res.Big)).FontSize(bigSize).Bold().NoWrap()
			ui.Text(c, st.res.Sub).FontSize(fsSmall).TextColor(p.muted).NoWrap()
			if size > 0 {
				a.freshness(c, st, size)
			}
			switch size {
			case 1:
				ui.Spacer(c)
				SparkStyled(c, st.res.Chart, a.styles[sc.ID], max(54, b.Height-154)*s)
				for _, in := range inputs {
					a.quickEntry(c, sc.ID, in)
				}
			case 2, 3:
				ui.Spacer(c)
				a.dimRow(c, idx, meta.Dims)
				a.varRow(c, sc, meta)
				// A wide card is a strip: its chart is shorter than a large card's.
				chartH := max(120, b.Height-210) * s
				if size == 3 {
					chartH = 120 * s
				}
				Chart(c, ChartProps{Data: st.res.Chart, Style: a.styles[sc.ID], Height: chartH, Selected: a.selOf("card:" + sc.ID)})
				Detail(c, st.res.Chart, a.selOf("card:"+sc.ID))
				for _, in := range inputs {
					a.quickEntry(c, sc.ID, in)
				}
			}
		}
		if a.editing {
			ui.Row(c).Shrink(0).AlignItems(ui.End).Children(func() {
				ui.Spacer(c)
				a.resizeHandle(c, sc.ID, span, b)
			})
		}
	})
	// Right click on this card: a menu in the window with its four sizes. It
	// changes this card only.
	open := a.cardMenuOpen(sc.ID)
	ui.ContextMenuBase(c, el, open).Popup(func(panel ui.Element) {
		panel.Padding(6).Gap(2).Background(p.card).Radius(10).BorderWidth(1).BorderColor(p.border).Children(func() {
			for i, name := range sizeNames() {
				i := i
				if btn(c, name).Clicked() {
					a.setSize(sc.ID, i)
					*open = false
				}
			}
		})
	})
	if a.editing {
		el = el.Drag(cardRef{sc.ID}).Cursor(ui.CursorGrab).Label("card:" + sc.ID)
		el.Border(1, accent.Alpha(0.55))
		if ref, ok := ui.Drop[cardRef](el); ok && ref.id != sc.ID {
			a.moveCard(ref.id, sc.ID)
		}
	}
	if el.Hovered() {
		el.Border(1, accent.Alpha(0.55))
	}
	if el.Clicked() && !a.editing {
		a.router.Push("/full/" + sc.ID)
	}
}

// dimRow shows a row of options for each dimension, such as the range of
// days. The chosen options are kept in idx.
func (a *app) dimRow(c *ui.Context, idx []int, dims []engine.Dim) {
	p := pal(c)
	if len(dims) == 0 {
		return
	}
	ui.Row(c).AlignItems(ui.Center).Gap(14).Wrap().Children(func() {
		for i, d := range dims {
			i, d := i, d
			labels := make([]string, len(d.Options))
			for j, o := range d.Options {
				labels[j] = o.Label
			}
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Text(c, d.Label).FontSize(fsSmall).TextColor(p.muted)
				segmented(c, &idx[i], labels...)
			})
		}
	})
}

func clampF(v, lo, hi float32) float32 {
	return float32(math.Min(float64(hi), math.Max(float64(lo), float64(v))))
}

// tagFor colors a "remaining" tag: green while plenty is left, amber, then red.
func tagFor(frac float64) display.TagColor {
	switch {
	case frac <= 0.10:
		return display.TagError
	case frac <= 0.30:
		return display.TagWarning
	}
	return display.TagSuccess
}

// fitText shortens s with an ellipsis so that it is about width DIPs wide at
// font size fs: a wide (Chinese) character counts as fs, any other as 0.5 fs.
func fitText(s string, width, fs float32) string {
	var used float32
	for i, r := range s {
		w := fs * 0.48
		if r >= 0x2E80 {
			w = fs
		}
		if used+w > width {
			if width-used < fs { // no room for the ellipsis either
				return strings.TrimRight(s[:i], " ·") + "…"
			}
			return s[:i] + "…"
		}
		used += w
	}
	return s
}
