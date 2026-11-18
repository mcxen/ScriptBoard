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
var cardSizes = [3][2]float32{
	{170, 128},
	{300, 208},
	{620, 400},
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
		w -= sidebarWidth
	}
	return w - 2*pad(a.compact)
}

// gridFor is the grid the home page lays its cards on, by the width it has: 12
// columns on a wide window, 6 on a middling one, 2 on a phone; and how many
// columns a small, a medium and a large card span in it. Cards of different
// sizes share the grid, so a row can hold a large card beside a medium one.
func gridFor(width float32) (cols int, spans [3]int) {
	switch {
	case width >= 900:
		return 12, [3]int{3, 4, 8}
	case width >= 520:
		return 6, [3]int{2, 3, 6}
	}
	return 2, [3]int{1, 2, 2}
}

// sizeOf is the size of a home card: its own when the user gave it one, else
// the default.
func (a *app) sizeOf(id string) int {
	if n, ok := a.set.CardSizes[id]; ok && n >= 0 && n <= 2 {
		return n
	}
	return a.cardSize
}

// setSize gives a card its own size.
func (a *app) setSize(id string, size int) {
	if a.set.CardSizes == nil {
		a.set.CardSizes = map[string]int{}
	}
	a.set.CardSizes[id] = size
	if err := a.store.SetSettings(a.set); err != nil {
		a.set = a.store.Settings()
	}
}

// setAllSizes gives every card the same size again.
func (a *app) setAllSizes(size int) {
	a.cardSize = size
	a.set.CardSizes = nil
	if err := a.store.SetSettings(a.set); err != nil {
		a.set = a.store.Settings()
	}
}

// cardLayout works out the size of a card of the given size from the width
// the page has and the scale: its width is its columns of the grid, its height
// the size's height times the scale. cols is how many such cards fit a row.
func (a *app) cardLayout(size int) (w, h float32, cols int) {
	s := a.scale()
	n, spans := gridFor(a.contentW())
	unit := (a.contentW() - cardGap*float32(n-1)) / float32(n)
	span := spans[size]
	w = unit*float32(span) + cardGap*float32(span-1)
	h = cardSizes[size][1] * s
	return w, h, max(1, n/span)
}

// home is the overview: how big the cards are, then one card per script marked
// for it.
func (a *app) home(c *ui.Context, compact bool) {
	p := pal(c)
	now := a.now()
	scripts := a.homeScripts()
	subtitle := fmt.Sprintf(tr("%d月%d日 %s · %d 个组件"), int(now.Month()), now.Day(), tr(weekdays[now.Weekday()]), len(scripts))
	page(c, compact, func() {
		header(c, tr("总览"), subtitle, func() {
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Icon(c, icon("sparkles")).FontSize(15).TextColor(p.muted)
				if slider(c, &a.set.CardScale, 0.7, 1.6).Width(104).Tooltip(tr("图标与卡片大小 ") + fmtScale(a.scale())).Changed() {
					a.set.CardScale = float64(a.scale())
					if err := a.store.SetSettings(a.set); err != nil {
						a.set = a.store.Settings()
					}
				}
			})
			if choose(c, &a.cardSize, tr("小"), tr("中"), tr("大")) {
				a.setAllSizes(a.cardSize)
			}
		})
		if len(scripts) == 0 {
			card(c).Children(func() {
				empty(c, "layout-grid", tr("还没有组件"), tr("在「编辑」里新建脚本，或到「社区」下载别人写好的。"))
			})
			return
		}
		// Cards of different sizes line up at their tops.
		ui.Row(c).Wrap().Gap(cardGap).AlignItems(ui.Start).Children(func() {
			for _, sc := range scripts {
				a.card(c, sc)
			}
		})
	})
}

// inputRows is the most quick-entry rows any home card of this size shows, so
// that the cards of a size stay as tall as each other.
func (a *app) inputRows(size int) int {
	if size == 0 {
		return 0
	}
	most := 0
	for _, sc := range a.homeScripts() {
		if a.sizeOf(sc.ID) != size {
			continue
		}
		meta, _ := a.metaOf(sc)
		n := len(meta.Inputs)
		if size == 1 {
			n = min(n, 1)
		}
		most = max(most, n)
	}
	return most
}

// fmtScale shows a scale as a percentage, such as 120%.
func fmtScale(s float32) string {
	return fmt.Sprintf("%d%%", int(math.Round(float64(s)*100)))
}

// card draws one script as a widget, sized by cardLayout.
func (a *app) card(c *ui.Context, sc store.Script) {
	p := pal(c)
	s := a.scale()
	meta, metaErr := a.metaOf(sc)
	idx, vals := a.viewOf(sc.ID, meta.Dims)
	st := &runState{err: metaErr}
	if metaErr == nil {
		st = a.ensure(runKey(sc.ID, meta.Dims, vals), sc, vals)
	}

	size := a.sizeOf(sc.ID)
	w, h, _ := a.cardLayout(size)
	// A card with values to type in has room for them.
	inputs := meta.Inputs
	if size == 1 && len(inputs) > 1 {
		inputs = inputs[:1]
	}
	if size == 0 {
		inputs = nil
	}
	// Every card in the grid is as tall as the one with the most rows, so the
	// rows of cards stay even.
	h += float32(a.inputRows(size)) * 44 * s
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
			if action.ActionIcon(c, icon("maximize-2"), action.ActionIconProps{Size: theme.Small, Title: tr("切换这张卡片的大小")}).Clicked() {
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
			switch size {
			case 1:
				ui.Spacer(c)
				SparkStyled(c, st.res.Chart, a.styles[sc.ID], 54*s)
				for _, in := range inputs {
					a.quickEntry(c, sc.ID, in)
				}
			case 2:
				ui.Spacer(c)
				a.dimRow(c, idx, meta.Dims)
				Chart(c, ChartProps{Data: st.res.Chart, Style: a.styles[sc.ID], Height: 190 * s, Selected: a.selOf("card:" + sc.ID)})
				Detail(c, st.res.Chart, a.selOf("card:"+sc.ID))
				for _, in := range inputs {
					a.quickEntry(c, sc.ID, in)
				}
			}
		}
	})
	if el.Hovered() {
		el.Border(1, accent.Alpha(0.55))
	}
	if el.Clicked() {
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
