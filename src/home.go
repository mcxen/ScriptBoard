package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"scriptboard/src/internal/bar"
	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

// cardSizes are the base widths and heights of the home cards: small shows
// the headline only, medium adds a curve, large a chart you can hover over.
var cardSizes = [5][2]float32{
	{170, 128},
	{300, 208},
	{620, 400},
	{0, 208}, // wide: the width is the whole row's
	{0, 96},  // mini: only the number, the smallest card
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
	w -= 2 * pad(a.compact)
	// What the page itself is capped at (pagewidth.go).
	if capNow > 0 && w > capNow {
		w = capNow
	}
	return w
}

// sizeOf is the size of a home card: its own when the user gave it one, else
// the default.
func (a *app) sizeOf(id string) int {
	if n, ok := a.set.CardSizes[id]; ok && n >= 0 && n <= sizeMini {
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
	if sc.IsBar() {
		a.barHomeCard(c, sc, b, w, s)
		return
	}
	meta, metaErr := a.metaOf(sc)
	idx, vals := a.viewOf(sc.ID, meta.Dims)
	st := &runState{err: metaErr}
	if metaErr == nil {
		st = a.ensure(a.runKeyOf(sc.ID, sc, meta, vals), sc, vals)
		a.watchLive(sc.ID, meta)
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
	ready := st.err == nil && !st.loading
	var comp ComponentView
	var viewRes engine.Result
	var subStyle string
	if ready {
		comp, viewRes, subStyle = resolveComponent(st.res, a.styles[sc.ID])
	}
	pb, hasPB := progressBarOf(viewRes)
	hasPB = ready && hasPB
	// A card keeps the height its box gives it: a size is what the user chose.
	hasChart := ready && viewRes.Chart != nil && !hasPB
	compSpec := engine.WidgetSpec{}
	if comp != nil {
		compSpec = comp.Spec()
	}
	h := b.Height*s + float32(len(inputs))*44*s
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

	lay := layoutOf(st.res)
	showTitle := lay.Title == "show"
	// A wide card is a strip with a chart in it: the chart takes what the rest
	// of the content (the headline, the selectors, the detail line, the values to
	// type in) leaves, and the card grows when that is under 96, rather than the
	// content spilling out of it over the next card.
	wideChart := 120 * s
	if size == 3 && ready && !isCalendar(st.res.Chart) {
		need := 32*s + 36*s // its padding, and the gaps between its parts
		if showTitle {
			need += 36 * s
		}
		if st.res.Big != "" {
			need += bigSize * 1.3
		}
		if st.res.Sub != "" || hasChart {
			need += 22 * s
		}
		if len(meta.Dims) > 0 {
			need += 34 * s
		}
		if len(meta.Vars) > 0 {
			need += 34 * s
		}
		if hasChart && hasChartDetail(st.res.Chart) {
			need += 22 * s // the detail line under the chart
		}
		if len(st.res.Stats) > 0 {
			need += 60 * s
		}
		need += float32(len(inputs)) * 44 * s
		if hasChart {
			wideChart = max(h-need, 96*s)
			h = need + wideChart
		} else if hasPB && len(pb.Items) > 0 {
			barNeed := float32(len(pb.Items)) * 44 * s
			h = max(h, need+barNeed)
		}
	}
	base := card(c).Width(w)
	if h > 0 {
		base = base.Height(h)
	}
	// A card with a picture is the picture, edge to edge, unless its layout says
	// inline: its text and buttons are over it while the pointer is on it.
	cv := a.coverOf(sc, st.res, ready && size != sizeMini && !a.editing && lay.Image != "inline")
	padding := 16 * s
	if cv != nil {
		padding = 0
	}
	box := base.Padding(padding).Gap(6 * s).Cursor(ui.CursorPointer)
	// The buttons show while the pointer is on the card. A touch screen has no
	// pointer; a refresh under way and the edit layout show them too, and so does
	// a script that asks for them (layout actions: "always").
	showActions := box.Hovered() || lay.Actions == "always" || a.touchUI() || a.editing || a.refreshing[sc.ID]
	// A card is its content: its name is in a row of its own only when the layout
	// asks (title: "show"), and shows over it, with the buttons, on hover. The
	// height the row would take goes to the chart.
	lift := float32(0)
	if !showTitle {
		lift = 36 * s
	}
	// chip is the change of a curve, or what is left of a gauge: beside the
	// title, or on the summary line when there is no title (the hover panel is
	// at the top right, over the headline's line).
	chipOK := false
	if g := st.res.Chart; ready && g != nil && g.Type == "gauge" && g.Gauge != nil {
		chipOK = true
	} else if ready && size > 0 && len(st.res.Stats) == 0 {
		// A live result has no change to show as a percentage: its curve
		// is a day of readings, not a series to compare.
		chipOK = summarize(st.res.Chart).ok
	}
	chip := func() {
		if !chipOK {
			return
		}
		if g := st.res.Chart; g != nil && g.Type == "gauge" && g.Gauge != nil {
			_, frac := remaining(g.Gauge)
			display.Tag(c, fmt.Sprintf(tr("剩余 %.0f%%"), frac*100), display.TagProps{Color: tagFor(frac), Size: theme.Small}).Shrink(0)
		} else {
			deltaChip(c, summarize(st.res.Chart).pct).Shrink(0)
		}
	}
	el := box.Children(func() {
		if cv != nil {
			a.coverCard(c, sc, cv, st.res, lay, title, size, showActions)
			return
		}
		if size == sizeMini {
			// The smallest card: its number and nothing else. A click opens the
			// full view and the right-click menu still picks a size.
			if ready && isCalendar(st.res.Chart) {
				a.calendarHomeCard(c, size, s, b.Height)
			} else {
				ui.Spacer(c)
				ui.Text(c, tickText(c, "card:"+sc.ID, st.res.Big)).FontSize(clampF(44*s, 28, 60)).Bold().NoWrap()
			}
		} else {
			if showTitle {
				ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
					iconBadge(c, iconName, accent, clampF(30*s, 24, 44))
					// The title gives way, with an ellipsis, so the tag and the buttons
					// stay whole on a small card.
					ui.Row(c).Grow(1).Shrink(1).MinWidth(0).ClipX().Children(func() {
						ui.Text(c, title).FontSize(fsCaption).TextColor(p.muted).NoWrap()
					})
					chip()
					a.refreshButton(c, sc, showActions)
					a.sizeButton(c, sc, size, showActions)
				})
			}
			switch {
			case st.loading:
				ui.Spacer(c)
				ui.Text(c, tr("加载中…")).FontSize(fsSmall).TextColor(p.muted)
			case st.err != nil:
				ui.Text(c, st.err.Error()).FontSize(fsTiny).TextColor(p.up)
			default:
				if isCalendar(st.res.Chart) {
					a.calendarHomeCard(c, size, s, b.Height)
					break
				}
				// A card with no headline, or no summary, does not keep a line empty.
				// SelfHero widgets (clock, single-target countdown) already draw a hero
				// display on medium/large cards, so avoid duplicating Big above them.
				selfHero := size > 0 && hasChart && compSpec.Capabilities.SelfHero &&
					(!isCountdown(viewRes.Chart) || len(viewRes.Chart.Countdown) == 1)
				if st.res.Big != "" && !selfHero {
					ui.Text(c, tickText(c, "card:"+sc.ID, st.res.Big)).FontSize(bigSize).Bold().NoWrap()
				}
				hasSub := lay.Summary != "hide" && st.res.Sub != ""
				switch {
				case showTitle && hasSub:
					ui.Text(c, st.res.Sub).FontSize(fsSmall).TextColor(p.muted).NoWrap()
				case !showTitle && (hasSub || chipOK):
					// The summary, with the change beside it.
					ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
						if hasSub {
							ui.Row(c).Shrink(1).MinWidth(0).ClipX().Children(func() {
								ui.Text(c, st.res.Sub).FontSize(fsSmall).TextColor(p.muted).NoWrap()
							})
						}
						chip()
					})
				}
				showLegend := a.legendShown(sc.ID, lay)
				if size > 0 {
					a.freshness(c, st, size)
					if len(st.res.Images) > 0 {
						a.imageCarousel(c, sc, st.res.Images, w-32*s, 120*s)
					}
				}
				switch size {
				case 0:
					if comp != nil && hasPB && len(pb.Items) > 0 {
						comp.RenderCard(c, ComponentRenderReq{
							App: a, Script: sc, Result: viewRes, Style: subStyle,
							Size: 0, Limit: 1, Now: a.now(),
						})
					}
				case 1:
					if hasPB {
						comp.RenderCard(c, ComponentRenderReq{
							App: a, Script: sc, Result: viewRes, Style: subStyle,
							Size: 1, Limit: 2, Now: a.now(),
						})
						for _, in := range inputs {
							a.quickEntry(c, sc.ID, in)
						}
						break
					}
					if comp == nil || !hasChart {
						for _, in := range inputs {
							a.quickEntry(c, sc.ID, in)
						}
						break
					}
					if !compSpec.Capabilities.TopAligned || ringVAlign(viewRes.Chart) == "middle" || ringVAlign(viewRes.Chart) == "bottom" {
						ui.Spacer(c)
					}
					sparkH := max(54, b.Height-154)*s + lift
					if !compSpec.Capabilities.TopAligned && len(st.res.Stats) > 0 && !isCalendar(viewRes.Chart) {
						// The stat strip takes the room a curve would have had.
						sparkH = max(28, b.Height-210)*s + lift
					}
					comp.RenderCard(c, ComponentRenderReq{
						App: a, Script: sc, Result: viewRes, Style: subStyle,
						Size: 1, Height: sparkH, Limit: 3, Now: a.now(),
					})
					for _, in := range inputs {
						a.quickEntry(c, sc.ID, in)
					}
				case 2, 3:
					if hasPB {
						a.dimRow(c, idx, meta.Dims)
						a.varRow(c, sc, meta)
						comp.RenderCard(c, ComponentRenderReq{
							App: a, Script: sc, Result: viewRes, Style: subStyle,
							Size: size, Limit: 0, Now: a.now(),
						})
						for _, in := range inputs {
							a.quickEntry(c, sc.ID, in)
						}
						break
					}
					// A completion chart or widget sits right under the summary; a curve is
					// pushed down to fill the card.
					if !hasChart || !compSpec.Capabilities.TopAligned || ringVAlign(viewRes.Chart) == "middle" || ringVAlign(viewRes.Chart) == "bottom" {
						ui.Spacer(c)
					}
					a.dimRow(c, idx, meta.Dims)
					a.varRow(c, sc, meta)
					// A wide card is a strip: its chart is shorter than a large card's.
					chartH := max(120, b.Height-210)*s + lift
					if size == 3 {
						chartH = wideChart
					}
					if comp != nil && hasChart {
						sel := a.selOf("card:" + sc.ID)
						comp.RenderCard(c, ComponentRenderReq{
							App: a, Script: sc, Result: viewRes, Style: subStyle,
							Size: size, Height: chartH, Selected: sel, Now: a.now(), Legend: showLegend,
						})
						if compSpec.Capabilities.HasDetail && !isCalendar(viewRes.Chart) {
							Detail(c, viewRes.Chart, sel)
						}
						if showLegend && compSpec.Capabilities.Plot {
							legendRow(c, viewRes.Chart)
						}
						if isProgress(viewRes.Chart) && ringVAlign(viewRes.Chart) != "bottom" {
							ui.Spacer(c)
						}
					}
					for _, in := range inputs {
						a.quickEntry(c, sc.ID, in)
					}
				}
				// The stats are their own block at the foot of the card, when the
				// layout keeps them. A medium card with bars or widgets has no room for them;
				// its full view has them.
				barsOnMedium := size == 1 && (hasPB || (hasChart && compSpec.Capabilities.TopAligned))
				if size > 0 && lay.Stats != "hide" && !barsOnMedium {
					cardStats(c, st.res.Stats)
				}
			}
		}
		if !showTitle && !a.editing && size != sizeMini {
			a.hoverBar(c, sc, title, size, showActions, w)
		}
		if a.editing {
			a.removeRow(c, sc)
			a.resizeHandle(c, sc.ID, span, b)
		}
	})
	// Right click on this card: a menu in the window with its sizes, legend,
	// menu bar toggle, and compatible visual view swaps.
	a.cardContextMenu(c, sc, meta, st, viewRes, compSpec, el)
	if a.editing {
		el = el.Drag(cardRef{sc.ID}).Cursor(ui.CursorGrab).Label("card:" + sc.ID)
		if a.rz != nil && a.rz.id == sc.ID {
			el.Border(2, p.accent)
		} else {
			el.Border(1, accent.Alpha(0.55))
		}
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

func (a *app) cardContextMenu(c *ui.Context, sc store.Script, meta engine.Meta, st *runState, viewRes engine.Result, compSpec engine.WidgetSpec, el ui.Element) {
	p := pal(c)
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
			// The legend of this card: only offered when the chart actually has multiple series.
			if compSpec.Capabilities.Plot && viewRes.Chart != nil && len(viewRes.Chart.Series) > 1 {
				legend := tr("隐藏图例")
				if a.set.LegendHidden[sc.ID] {
					legend = tr("显示图例")
				}
				if btn(c, legend).Clicked() {
					a.toggleLegend(sc.ID)
					*open = false
				}
			}
			// The menu bar: offered when the script asks for it, and the user
			// chooses whether the card is there.
			if menuBarOffered(meta) {
				bar := tr("在菜单栏显示")
				if a.menuBarOn(sc.ID) {
					bar = tr("从菜单栏移除")
				}
				if btn(c, bar).Clicked() {
					a.toggleMenuBar(sc.ID)
					*open = false
				}
			}
			// Runtime component swapping directly from the card's context menu.
			if swaps := visualSwapOptions(st.res); len(swaps) > 1 {
				ui.Box(c).Height(1).Shrink(0).Background(p.hairline)
				baseType := engine.ResultWidgetType(st.res)
				for _, target := range swaps {
					target := target
					if target == compSpec.Type {
						continue
					}
					label := tr("切换为 ") + widgetDisplayName(target)
					if btn(c, label).Clicked() {
						if target == baseType {
							a.setCardStyle(sc.ID, "")
						} else {
							a.setCardStyle(sc.ID, "view:"+target)
						}
						*open = false
					}
				}
			}
		})
	})
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

// removeRow is the × of a card in edit mode: it takes the card off the home
// page. The script stays in 编辑, where it can be put back with 显示在首页.
func (a *app) removeRow(c *ui.Context, sc store.Script) {
	p := pal(c)
	ui.Row(c.Key("remove:" + sc.ID)).Absolute().Top(6).Right(6).Padding(2).Radius(radControl).
		Background(p.card.Alpha(0.92)).BorderWidth(1).BorderColor(p.hairline).Children(func() {
		if action.ActionIcon(c, icon("x"), action.ActionIconProps{Size: theme.Small, Title: tr("从首页移除")}).Clicked() {
			a.removeFromHome(sc)
		}
	})
}

// removeFromHome takes a script off the home page.
func (a *app) removeFromHome(sc store.Script) {
	sc.Home = false
	if err := a.store.SaveScript(sc); err != nil {
		a.status = tr("移除失败：") + err.Error()
		return
	}
	a.status = tr("已从首页移除，在「编辑」里可以再显示")
}

func (a *app) barHomeCard(c *ui.Context, sc store.Script, b store.CardBox, w, s float32) {
	p := pal(c)
	inst, _, err := a.ensureBarInstance(sc)
	var snap bar.Snapshot
	if err == nil && inst != nil {
		snap = inst.Snapshot()
	} else if err != nil {
		snap.Error = err.Error()
	}
	h := max(b.Height*s, 220*s)
	el := card(c).Width(w).Height(h).Clip().Cursor(ui.CursorPointer)
	el.Children(func() {
		if a.editing {
			a.removeRow(c, sc)
		}
		th := barThemeFor(c, barThemeAuto)
		ui.Scroll(c).Fill().Children(func() {
			if snap.Panel != nil {
				renderBarNode(c, snap.Panel, th, func(aid string) {
					a.invokeBarAction(sc.ID, aid)
				})
			} else if snap.Error != "" {
				ui.Column(c).Padding(16).Children(func() {
					ui.Text(c, snap.Error).FontSize(fsSmall).TextColor(p.up)
				})
			}
		})
	})
	if el.Clicked() && !a.editing {
		a.router.Push("/bar/" + sc.ID)
	}
}
