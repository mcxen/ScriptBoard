package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
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
	homeHeader := func(actions func()) {
		if isMac() {
			macHeader(c, tr("总览"), subtitle, actions)
			return
		}
		ui.Row(c).AlignItems(ui.End).Gap(gapItem).Wrap().Children(func() {
			ui.Column(c).Gap(3).Children(func() {
				ui.Text(c, subtitle).FontSize(11.5).FontWeight(500).FontFeatures("tnum").TextColor(p.muted)
				titleSize := float32(26)
				if compact {
					titleSize = 24
				}
				ui.Text(c, tr("总览")).FontSize(titleSize).FontWeight(600).LetterSpacing(-0.65).TextColor(p.text)
			})
			ui.Spacer(c)
			if actions != nil {
				ui.Row(c).AlignItems(ui.Center).Gap(8).Wrap().Children(actions)
			}
		})
	}
	scaleControl := func() {
		if compact {
			return
		}
		scaleTrigger := ui.ButtonBase(c).
			Height(30).
			Padding(0, 10).
			Radius(radControl + 1).
			AlignItems(ui.Center).
			Justify(ui.Center).
			Cursor(ui.CursorPointer).
			Tooltip(tr("缩放比例"))
		bg := p.card
		if a.scalePopoverOpen || scaleTrigger.Hovered() {
			bg = p.hover
		}
		scaleTrigger = scaleTrigger.Background(bg).BorderWidth(1).BorderColor(p.border).Children(func() {
			ui.Text(c, fmtScale(a.scale())).FontSize(12).FontWeight(500).FontFeatures("tnum").TextColor(p.muted).NoWrap()
		})
		if scaleTrigger.Clicked() {
			a.scalePopoverOpen = !a.scalePopoverOpen
		}
		ui.PopoverBase(c, scaleTrigger, &a.scalePopoverOpen, func(panel ui.Element) {
			pop := panel.Width(220).Padding(14).Gap(10).Radius(12).Background(p.card).BorderWidth(1).BorderColor(p.border)
			if !isMac() {
				sh := ui.RGBA(28, 25, 23, 0.12)
				if p.dark {
					sh = ui.RGBA(0, 0, 0, 0.36)
				}
				pop = pop.Shadow(0, 8, 24, 0, sh)
			}
			pop.Children(func() {
				ui.Row(c).AlignItems(ui.Center).Children(func() {
					ui.Text(c, tr("缩放比例")).FontSize(fsSmall).FontWeight(600).TextColor(p.text)
					ui.Spacer(c)
					ui.Text(c, fmtScale(a.scale())).FontSize(fsSmall).FontFeatures("tnum").TextColor(p.muted)
				})
				if slider(c, &a.set.CardScale, 0.7, 1.6).Width(192).Changed() {
					a.set.CardScale = float64(a.scale())
					if err := a.store.SetSettings(a.set); err != nil {
						a.set = a.store.Settings()
					}
				}
				if math.Abs(float64(a.scale())-1) > 0.01 {
					if paperOutlineBtn(c, tr("恢复默认"), false).FillWidth().Clicked() {
						a.set.CardScale = 1
						if err := a.store.SetSettings(a.set); err != nil {
							a.set = a.store.Settings()
						}
					}
				}
			})
		})
	}
	scrollEl := pageFull(c, compact, func() {
		sizeControls := func() {
			if paperSegmented(c, &a.cardSize, tr("小"), tr("中"), tr("大")) {
				a.setAllSizes(a.cardSize)
			}
			if !compact {
				ui.Box(c).Width(1).Height(16).Background(p.hairline)
				scaleControl()
			}
		}
		editControls := func() {
			if a.editing {
				if paperOutlineBtn(c, tr("完成"), true).Clicked() {
					a.editing = false
					a.rz = nil
				}
				return
			}
			if paperOutlineBtn(c, tr("编辑布局"), false).Clicked() {
				a.editing = true
			}
		}
		if compact {
			homeHeader(nil)
			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				sizeControls()
			})
		} else {
			homeHeader(func() {
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
			ui.Row(c).Padding(8, 12).Radius(radControl + 1).Background(p.chip).Children(func() {
				hint(c, tr("拖动卡片换位置；拖右下角的角调整大小。右键卡片也能选大小。"))
			})
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
	if a.homeRefresh.Refreshing && !a.anyHomeRefreshing(scripts) {
		a.homeRefresh.Refreshing = false
	}
	if scrollEl.PullToRefresh(&a.homeRefresh) {
		a.refreshAllHome(scripts)
		if !a.anyHomeRefreshing(scripts) {
			a.homeRefresh.Refreshing = false
		}
	}
	a.cardActionSheet(c)
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
		accent = accentFor(st.res, p.accent, p.dark)
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
	narrow := w > 0 && w < 220
	bigSize := clampF(34*s, 24, 46)
	if narrow {
		bigSize = clampF(28*s, 22, 34)
	}
	// What is left of the header for the title: the card less its padding, the
	// icon, optional buttons, a tag when there is one, and the gaps. On narrow
	// cards action buttons sit in a floating corner bar instead of the header row.
	hasTag := ready && (size > 0 || (st.res.Chart != nil && st.res.Chart.Type == "gauge"))
	padSide := 18 * s
	if narrow {
		padSide = 13 * s
	}
	titleW := w - 2*padSide - 13 - 10
	if !narrow {
		titleW -= 2*22 + 8
	}
	if hasTag {
		titleW -= 52
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
		if isBento(viewRes.Chart) {
			needBento := bentoWideHeight(viewRes.Chart.Bento, max(w-32*s, 200), s)
			if showTitle {
				needBento += 36 * s
			}
			if len(meta.Dims) > 0 && viewRes.Chart.Bento.DetailTable == nil {
				needBento += 34 * s
			}
			if len(meta.Vars) > 0 {
				needBento += 34 * s
			}
			needBento += float32(len(inputs)) * 44 * s
			h = max(h, needBento)
		} else {
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
	}
	base := card(c).Width(w).Clip()
	if h > 0 {
		base = base.Height(h)
	}
	// A card with a picture is the picture, edge to edge, unless its layout says
	// inline: its text and buttons are over it while the pointer is on it.
	cv := a.coverOf(sc, st.res, ready && size != sizeMini && !a.editing && lay.Image != "inline")
	padding := 18 * s
	if narrow {
		padding = 13 * s
	}
	if cv != nil {
		padding = 0
	}
	innerW := max(w-2*padding, 80)
	box := base.Padding(padding).Gap(6 * s).Cursor(ui.CursorPointer)
	// The buttons show while the pointer is on the card. A touch screen has no
	// pointer; a refresh under way and the edit layout show them too, and so does
	// a script that asks for them (layout actions: "always").
	showActions := box.Hovered() || lay.Actions == "always" || a.touchUI() || a.editing || a.refreshActive(sc.ID)
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
				a.calendarHomeCard(c, size, s, b.Height, innerW)
			} else {
				ui.Spacer(c)
				ui.Text(c, tickText(c, "card:"+sc.ID, st.res.Big)).
					FontSize(clampF(44*s, 28, 60)).
					FontWeight(350).
					LetterSpacing(-1.6).
					FontFeatures("tnum").
					TextColor(p.text).
					ContentTransition(ui.ContentTransition{Numeric: true}).
					SingleLine().
					Ellipsis("…")
			}
		} else {
			if showTitle {
				ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
					ui.Icon(c, icon(iconName)).FontSize(13).TextColor(p.muted)
					// The title gives way, with an ellipsis, so the tag and any buttons
					// stay whole on a small card.
					ui.Row(c).Grow(1).Shrink(1).MinWidth(0).ClipX().Children(func() {
						ui.Text(c, title).FontSize(12).FontWeight(550).TextColor(p.muted).NoWrap()
					})
					chip()
					if !narrow {
						a.refreshButton(c, sc, showActions)
						a.sizeButton(c, sc, size, showActions)
					}
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
					a.calendarHomeCard(c, size, s, b.Height, innerW)
					break
				}
				// A card with no headline, or no summary, does not keep a line empty.
				// SelfHero widgets (clock, single-target countdown, and hero Bento boards) already draw a hero
				// display, so avoid duplicating Big above them.
				selfHero := hasChart && ((size > 0 && compSpec.Capabilities.SelfHero &&
					(!isCountdown(viewRes.Chart) || len(viewRes.Chart.Countdown) == 1) && !isBento(viewRes.Chart)) ||
					isBentoHero(viewRes.Chart))
				if st.res.Big != "" && !selfHero {
					renderHeroBig(c, p, sc.ID, st.res.Big, bigSize, s)
				}
				hasSub := lay.Summary != "hide" && st.res.Sub != "" && !isBentoHero(viewRes.Chart)
				subColor := p.muted
				subFS := fsSmall
				if st.res.Big == "--" {
					subColor = p.faint
					subFS = fsTiny
				}
				switch {
				case showTitle && hasSub:
					subTxt := ui.Text(c, st.res.Sub).FontSize(subFS).TextColor(subColor)
					if st.res.Big == "--" {
						subTxt.LineHeight(1.45)
					} else {
						subTxt.SingleLine().Ellipsis("…")
					}
				case !showTitle && hasSub && !chipOK:
					subTxt := ui.Text(c, st.res.Sub).FontSize(subFS).TextColor(subColor)
					if st.res.Big == "--" {
						subTxt.LineHeight(1.45)
					}
				case !showTitle && (hasSub || chipOK):
					subGap := float32(8)
					if narrow {
						subGap = 6
					}
					// The summary, with the change beside it.
					ui.Row(c).AlignItems(ui.Center).Gap(subGap).Children(func() {
						if hasSub {
							ui.Row(c).Grow(1).Shrink(1).MinWidth(0).ClipX().Children(func() {
								ui.Text(c, st.res.Sub).FontSize(subFS).TextColor(subColor).SingleLine().Ellipsis("…")
							})
						}
						chip()
					})
				}
				showLegend := a.legendShown(sc.ID, lay)
				if size > 0 {
					a.freshness(c, st, size)
					if len(st.res.Images) > 0 {
						a.imageCarousel(c, sc, st.res.Images, innerW, 120*s)
					}
				}
				switch size {
				case 0:
					if comp != nil && ((hasPB && len(pb.Items) > 0) || isBento(viewRes.Chart)) {
						comp.RenderCard(c, ComponentRenderReq{
							App: a, Script: sc, Result: viewRes, Style: subStyle,
							Size: 0, Width: innerW, Limit: 1, Now: a.now(),
						})
					}
				case 1:
					if hasPB {
						comp.RenderCard(c, ComponentRenderReq{
							App: a, Script: sc, Result: viewRes, Style: subStyle,
							Size: 1, Width: innerW, Limit: 2, Now: a.now(),
						})
						for _, in := range inputs {
							a.quickEntryWidth(c, sc.ID, in, innerW)
						}
						break
					}
					if comp == nil || !hasChart {
						for _, in := range inputs {
							a.quickEntryWidth(c, sc.ID, in, innerW)
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
						Size: 1, Width: innerW, Height: sparkH, Limit: 3, Now: a.now(),
					})
					for _, in := range inputs {
						a.quickEntryWidth(c, sc.ID, in, innerW)
					}
				case 2, 3:
					if hasPB {
						a.dimRow(c, idx, meta.Dims)
						a.varRow(c, sc, meta)
						comp.RenderCard(c, ComponentRenderReq{
							App: a, Script: sc, Result: viewRes, Style: subStyle,
							Size: size, Width: innerW, Limit: 0, Now: a.now(),
						})
						for _, in := range inputs {
							a.quickEntryWidth(c, sc.ID, in, innerW)
						}
						break
					}
					// A completion chart or widget sits right under the summary; a curve is
					// pushed down to fill the card.
					if !hasChart || !compSpec.Capabilities.TopAligned || ringVAlign(viewRes.Chart) == "middle" || ringVAlign(viewRes.Chart) == "bottom" {
						ui.Spacer(c)
					}
					if !isBento(viewRes.Chart) || viewRes.Chart.Bento.DetailTable == nil {
						a.dimRow(c, idx, meta.Dims)
					}
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
							Size: size, Width: innerW, Height: chartH, Selected: sel, Now: a.now(), Legend: showLegend,
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
						a.quickEntryWidth(c, sc.ID, in, innerW)
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
		if !a.editing && size != sizeMini {
			if !showTitle {
				a.hoverBar(c, sc, title, size, showActions, w)
			} else if narrow {
				corner := ui.Row(c).Absolute().Top(6).Right(6).AlignItems(ui.Center).Gap(2).Padding(2, 4).Radius(8).Background(p.card.Alpha(0.94)).BorderWidth(1).BorderColor(p.border)
				if !showActions {
					corner = corner.Opacity(0).IgnoreInput(true)
				}
				corner.Children(func() {
					a.refreshButton(c, sc, true)
					a.sizeButton(c, sc, size, true)
				})
			}
		}
		if a.editing {
			a.removeRow(c, sc)
			a.resizeHandle(c, sc.ID, span, b)
		}
	})
	// Right click on this card: a menu in the window with its sizes, legend,
	// menu bar toggle, and compatible visual view swaps. On touch screens, a
	// long press opens a native bottom action sheet instead.
	a.cardContextMenu(c, sc, meta, st, viewRes, compSpec, el)
	if !a.editing && el.LongPressed() {
		a.hapticMedium()
		a.sheetScriptID = sc.ID
		a.sheetOpen = true
	}
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
		hb := ui.RGBA(28, 25, 23, 0.20)
		if p.dark {
			hb = ui.RGBA(255, 255, 255, 0.20)
		}
		el.Border(1, hb)
	}
	if el.Clicked() && !a.editing {
		a.router.Push("/full/" + sc.ID)
	}
}

func renderHeroBig(c *ui.Context, p palette, scriptID, raw string, bigSize, s float32) {
	if raw == "--" {
		ui.Text(c, raw).
			FontSize(clampF(26*s, 20, 32)).
			FontWeight(350).
			TextColor(p.faint).
			SingleLine()
		return
	}
	if num, unit, ok := splitHeroUnit(raw); ok {
		unitFS := clampF(bigSize*0.44, 12, 16)
		ui.Row(c).AlignItems(ui.Baseline).Gap(4).Label(raw).Children(func() {
			ui.Text(c, tickText(c, "card:"+scriptID, num)).
				FontSize(bigSize).
				FontWeight(350).
				LetterSpacing(-1.3).
				FontFeatures("tnum").
				TextColor(p.text).
				ContentTransition(ui.ContentTransition{Numeric: true}).
				SingleLine()
			ui.Text(c, unit).
				FontSize(unitFS).
				FontWeight(450).
				TextColor(p.muted).
				NoWrap()
		})
		return
	}
	ui.Text(c, tickText(c, "card:"+scriptID, raw)).
		FontSize(bigSize).
		FontWeight(350).
		LetterSpacing(-1.3).
		FontFeatures("tnum").
		TextColor(p.text).
		ContentTransition(ui.ContentTransition{Numeric: true}).
		SingleLine().
		Ellipsis("…")
}

func splitHeroUnit(raw string) (num, unit string, ok bool) {
	s := strings.TrimSpace(raw)
	idx := strings.LastIndexByte(s, ' ')
	if idx <= 0 || idx >= len(s)-1 {
		return "", "", false
	}
	num = strings.TrimSpace(s[:idx])
	unit = strings.TrimSpace(s[idx+1:])
	if num == "" || unit == "" {
		return "", "", false
	}
	first := num[0]
	if (first < '0' || first > '9') && first != '+' && first != '-' {
		return "", "", false
	}
	runes := 0
	for _, r := range unit {
		runes++
		if runes > 6 || (r >= '0' && r <= '9') || r == ':' || r == '/' {
			return "", "", false
		}
	}
	return num, unit, true
}

func (a *app) cardContextMenu(c *ui.Context, sc store.Script, meta engine.Meta, st *runState, viewRes engine.Result, compSpec engine.WidgetSpec, el ui.Element) {
	p := pal(c)
	open := a.cardMenuOpen(sc.ID)
	curSize := classOf(a.boxOf(sc.ID))
	item := func(label string, checked bool) bool {
		if isMac() {
			return btn(c, label).Clicked()
		}
		return popoverMenuRow(c, label, checked, false).Clicked()
	}
	divider := func() {
		ui.Box(c).Height(1).Shrink(0).Background(p.hairline)
	}
	ui.ContextMenuBase(c, el, open).Popup(func(panel ui.Element) {
		pop := panel.MinWidth(172).Padding(6).Gap(2).Background(p.card).Radius(12).BorderWidth(1).BorderColor(p.border)
		if !isMac() {
			pop = pop.Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.24))
		}
		pop.Children(func() {
			for i, name := range sizeNames() {
				i := i
				if item(name, i == curSize) {
					a.setSize(sc.ID, i)
					*open = false
				}
			}
			hasLegend := compSpec.Capabilities.Plot && viewRes.Chart != nil && len(viewRes.Chart.Series) > 1
			hasBar := menuBarOffered(meta)
			if hasLegend || hasBar {
				divider()
			}
			// The legend of this card: only offered when the chart actually has multiple series.
			if hasLegend {
				legend := tr("隐藏图例")
				if a.set.LegendHidden[sc.ID] {
					legend = tr("显示图例")
				}
				if item(legend, false) {
					a.toggleLegend(sc.ID)
					*open = false
				}
			}
			// The menu bar: offered when the script asks for it, and the user
			// chooses whether the card is there.
			if hasBar {
				barLbl := tr("在菜单栏显示")
				if a.menuBarOn(sc.ID) {
					barLbl = tr("从菜单栏移除")
				}
				if item(barLbl, a.menuBarOn(sc.ID)) {
					a.toggleMenuBar(sc.ID)
					*open = false
				}
			}
			// Runtime component swapping directly from the card's context menu.
			if swaps := visualSwapOptions(st.res); len(swaps) > 1 {
				divider()
				baseType := engine.ResultWidgetType(st.res)
				for _, target := range swaps {
					target := target
					if target == compSpec.Type {
						continue
					}
					label := tr("切换为 ") + widgetDisplayName(target)
					if item(label, false) {
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
	th := barThemeFor(c, barThemeAuto)
	themeStr := "dark"
	if !th.dark {
		themeStr = "light"
	}
	var snap bar.Snapshot
	if err == nil && inst != nil {
		inst.SetAppearance(themeStr, false)
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
		ui.Scroll(c).Fill().Children(func() {
			if snap.Panel != nil {
				renderBarNodeInteractive(c, snap.Panel, th, false, func(aid string) {
					a.invokeBarAction(sc.ID, aid)
				}, func(aid string, arg any) {
					a.invokeBarActionWithArg(sc.ID, aid, arg)
				})
			} else if snap.Error != "" {
				ui.Column(c).Padding(16).Children(func() {
					ui.Text(c, snap.Error).FontSize(fsSmall).TextColor(p.up)
				})
			}
		})
	})
	if !a.editing && el.LongPressed() {
		a.hapticMedium()
		a.sheetScriptID = sc.ID
		a.sheetOpen = true
	}
	if el.Clicked() && !a.editing {
		a.router.Push("/bar/" + sc.ID)
	}
}

func (a *app) cardActionSheet(c *ui.Context) {
	sc, ok := a.store.Script(a.sheetScriptID)
	if !ok {
		a.sheetOpen = false
		return
	}
	var actions []mobile.Action
	if sc.IsBar() {
		actions = append(actions,
			mobile.Action{Label: tr("刷新数据"), OnTap: func() { a.refreshBar(sc.ID) }},
			mobile.Action{Label: tr("编辑"), OnTap: func() { a.router.Push("/bar/" + sc.ID) }},
		)
		if isMobile() {
			actions = append(actions, mobile.Action{Label: tr("分享脚本"), OnTap: func() { a.shareScript(sc) }})
		}
		actions = append(actions, mobile.Action{
			Label:       tr("从首页移除"),
			Destructive: true,
			OnTap:       func() { a.removeFromHome(sc) },
		})
		mobile.ActionSheet(c, &a.sheetOpen, mobile.ActionSheetOptions{Title: tr(sc.Name), Cancel: tr("取消")}, actions...)
		return
	}

	meta, metaErr := a.metaOf(sc)
	_, vals := a.viewOf(sc.ID, meta.Dims)
	var st *runState
	if metaErr == nil {
		st = a.runs[a.runKeyOf(sc.ID, sc, meta, vals)]
	}
	curSize := classOf(a.boxOf(sc.ID))
	names := sizeNames()
	nextSize := (curSize + 1) % len(names)
	actions = append(actions,
		mobile.Action{Label: tr("刷新数据"), OnTap: func() { a.refreshScript(sc) }},
		mobile.Action{Label: tr("切换大小：") + names[nextSize], OnTap: func() { a.setSize(sc.ID, nextSize) }},
	)
	if st != nil && st.err == nil && !st.loading {
		comp, viewRes, _ := resolveComponent(st.res, a.styles[sc.ID])
		if comp != nil && comp.Spec().Capabilities.Plot && viewRes.Chart != nil && len(viewRes.Chart.Series) > 1 {
			legendLbl := tr("隐藏图例")
			if a.set.LegendHidden[sc.ID] {
				legendLbl = tr("显示图例")
			}
			actions = append(actions, mobile.Action{Label: legendLbl, OnTap: func() { a.toggleLegend(sc.ID) }})
		}
		if swaps := visualSwapOptions(st.res); len(swaps) > 1 && comp != nil {
			curType := comp.Spec().Type
			nextTarget := swaps[0]
			for i, target := range swaps {
				if target == curType {
					nextTarget = swaps[(i+1)%len(swaps)]
					break
				}
			}
			if nextTarget != curType {
				baseType := engine.ResultWidgetType(st.res)
				actions = append(actions, mobile.Action{
					Label: tr("切换为 ") + widgetDisplayName(nextTarget),
					OnTap: func() {
						if nextTarget == baseType {
							a.setCardStyle(sc.ID, "")
						} else {
							a.setCardStyle(sc.ID, "view:"+nextTarget)
						}
					},
				})
			}
		}
	}
	actions = append(actions, mobile.Action{Label: tr("编辑"), OnTap: func() { a.router.Push("/edit/" + sc.ID) }})
	if isMobile() {
		actions = append(actions, mobile.Action{Label: tr("分享脚本"), OnTap: func() { a.shareScript(sc) }})
	}
	actions = append(actions, mobile.Action{
		Label:       tr("从首页移除"),
		Destructive: true,
		OnTap:       func() { a.removeFromHome(sc) },
	})
	mobile.ActionSheet(c, &a.sheetOpen, mobile.ActionSheetOptions{Title: tr(sc.Name), Cancel: tr("取消")}, actions...)
}
