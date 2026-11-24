package main

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/spec"
	"scriptboard/src/internal/store"
)

// fullView is the full-screen view of one script: its chart in the style and
// with the dimensions the user picks, the statistics, and what the script
// says about itself. 大图 gives the chart the whole window.
func (a *app) fullView(c *ui.Context, id string, compact bool) {
	p := pal(c)
	sc, ok := a.store.Script(id)
	if !ok {
		page(c, compact, func() {
			subheader(c, a, tr("没有这个脚本"), "", nil)
		})
		return
	}
	meta, metaErr := a.metaOf(sc)
	idx, vals := a.viewOf(sc.ID, meta.Dims)
	st := &runState{err: metaErr}
	if metaErr == nil {
		st = a.ensure(a.runKeyOf(sc.ID, sc, meta, vals), sc, vals)
		a.watchLive(sc.ID, meta)
	}
	sel := a.selOf("full:" + sc.ID)
	pinned := a.pinnedSelOf(sc.ID)

	// The whole window, as the home page: the tiles and the chart are sized to it.
	pageEl := pageFull(c, compact, func() {
		comp, viewRes, subStyle := resolveComponent(st.res, a.styles[sc.ID])
		compSpec := engine.WidgetSpec{}
		if comp != nil {
			compSpec = comp.Spec()
		}
		// Apply interactive series visibility filter for multi-series plots.
		rawChart := viewRes.Chart
		if compSpec.Capabilities.Plot && rawChart != nil && len(rawChart.Series) >= 2 {
			viewRes.Chart = a.filterVisibleSeries(sc.ID, rawChart)
		}
		cal := isCalendar(st.res.Chart)
		_, hasPB := progressBarOf(viewRes)
		shown := shownImages(st.res.Images)
		gallery := len(shown) > 0 && !hasPB && st.res.Chart == nil
		plot := !hasPB && compSpec.Capabilities.Plot
		viewMode := a.fullModeOf(sc.ID)
		extractedTable := engine.ExtractTable(st.res, curLang())
		canTableMode := !cal && !isTableWidget(st.res.Chart) && extractedTable != nil
		// Keep selection synchronized with pinned index when pointer is outside.
		if plot && viewMode == 0 && *pinned >= 0 {
			if n := count(viewRes.Chart); *pinned >= n {
				*pinned = n - 1
			}
			if *sel < 0 && *pinned >= 0 {
				*sel = *pinned
			}
		}
		sub := st.res.Title
		if cal || sub == tr(sc.Name) {
			sub = ""
		} else if gallery {
			sub = fmt.Sprintf(tr("共 %d 张"), len(shown))
		}
		subheader(c, a, tr(sc.Name), sub, func() {
			if cal {
				if compact {
					if action.ActionIcon(c, icon("pencil-line"), action.ActionIconProps{Size: theme.Small, Title: tr("编辑")}).Clicked() {
						a.router.Push("/edit/" + sc.ID)
					}
					return
				}
				if btn(c, tr("编辑")).Clicked() {
					a.router.Push("/edit/" + sc.ID)
				}
				return
			}
			if compact {
				// A phone has room for icons, not for a row of buttons; 大图 gives
				// the chart the window's height, which a phone has none to spare.
				a.refreshButton(c, sc, true)
				a.resetIcon(c, sc)
				if action.ActionIcon(c, icon("pencil-line"), action.ActionIconProps{Size: theme.Small, Title: tr("编辑")}).Clicked() {
					a.router.Push("/edit/" + sc.ID)
				}
				return
			}
			if plot && viewMode == 0 {
				ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
					if ui.Text(c, tr("大图")).FontSize(fsCaption).Cursor(ui.CursorPointer).Clicked() {
						a.big = !a.big
					}
					sw(c, &a.big)
				})
			}
			if btn(c, tr("刷新")).Clicked() {
				a.refreshScript(sc)
			}
			a.resetButton(c, sc)
			if btn(c, tr("编辑")).Clicked() {
				a.router.Push("/edit/" + sc.ID)
			}
		})

		switch {
		case st.loading:
			card(c).Children(func() { empty(c, "activity", tr("加载中…"), "") })
			return
		case st.err != nil:
			card(c).Padding(16).Children(func() {
				ui.Text(c, st.err.Error()).TextColor(p.up)
			})
			return
		}
		res := st.res
		if isCalendar(res.Chart) {
			a.calendarPage(c, sel, compact)
			a.infoBlock(c, sc, meta, compact)
			return
		}

		// Data view mode switcher: Visual | Data Table | Raw Data Inspector.
		if !gallery || viewMode > 0 {
			a.fullViewModeBar(c, sc.ID, canTableMode, compact)
		}
		viewMode = a.fullModeOf(sc.ID)
		if !canTableMode && viewMode == 1 {
			viewMode = 0
		}

		if viewMode == 1 && extractedTable != nil {
			card(c).Padding(compactPad(compact)).Gap(gapItem).Children(func() {
				if len(meta.Dims) > 0 || len(meta.Vars) > 0 {
					ui.Row(c).AlignItems(ui.Center).Gap(18).Wrap().Children(func() {
						a.dimRow(c, idx, meta.Dims)
						a.varRow(c, sc, meta)
					})
				}
				a.tableWidgetView(c, "fullmode:"+sc.ID, extractedTable, true, 0)
			})
			a.infoBlock(c, sc, meta, compact)
			return
		}
		if viewMode == 2 {
			a.rawInspectorCard(c, sc, meta, res, compact)
			a.infoBlock(c, sc, meta, compact)
			return
		}

		if gallery {
			// Image-centric script (such as Bing wallpapers): photo-first
			// gallery layout with text as quiet secondary metadata and Save.
			a.imageGalleryFull(c, sc, res, compact)
			if len(res.Stats) > 0 {
				a.statsTiles(c, res.Stats)
			}
			if len(res.Lines) > 0 || len(res.Logs) > 0 {
				card(c).Padding(16).Gap(4).Children(func() {
					for _, line := range res.Lines {
						ui.Text(c, line).FontSize(fsCaption)
					}
					for _, l := range res.Logs {
						ui.Text(c, "log: "+l).FontSize(fsTiny).TextColor(p.muted)
					}
				})
			}
			a.infoBlock(c, sc, meta, compact)
			return
		}
		accent := accentOf(res, p.accent)

		// The hero: the headline number, the controls, and the component or chart.
		card(c).Padding(compactPad(compact)).Gap(gapItem).Children(func() {
			selfHero := isClock(viewRes.Chart) || (isCountdown(viewRes.Chart) && len(viewRes.Chart.Countdown) == 1) || isBentoHero(viewRes.Chart)
			showHeroHead := !selfHero && (res.Big != "" || res.Sub != "" || plot)
			if showHeroHead {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(14).Children(func() {
					ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
						if res.Big != "" || plot {
							ui.Text(c, tickText(c, "hero:"+sc.ID, heroText(viewRes, sel))).
								FontSize(46).
								FontWeight(300).
								LetterSpacing(-2).
								FontFeatures("tnum").
								ContentTransition(ui.ContentTransition{Numeric: true}).
								NoWrap()
						} else if res.Title != "" {
							ui.Text(c, res.Title).FontSize(fsHeading).FontWeight(650).NoWrap()
						}
						ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
							if plot {
								if cs := summarize(viewRes.Chart); cs.ok && len(res.Stats) == 0 {
									deltaChip(c, cs.pct)
								}
							}
							if plot {
								if i := hoveredIndex(viewRes.Chart, sel); i >= 0 {
									detailRow(c, viewRes.Chart, i)
								} else if res.Sub != "" {
									ui.Text(c, res.Sub).FontSize(fsCaption).TextColor(p.muted)
								}
							} else if res.Sub != "" {
								ui.Text(c, res.Sub).FontSize(fsCaption).TextColor(p.muted)
							}
						})
					})
					ui.Icon(c, icon(firstNonEmpty(res.Icon, meta.Icon))).FontSize(42).TextColor(accent)
				})
			}
			if a.status != "" {
				ui.Text(c, a.status).FontSize(fsTiny).TextColor(p.accent)
			}

			swaps := visualSwapOptions(res)
			plotOpts := []styleOpt(nil)
			if plot {
				plotOpts = stylesFor(viewRes.Chart)
			}
			showPlotStylePicker := len(plotOpts) > 1
			showBarStylePicker := hasPB
			showClockStylePicker := isClock(viewRes.Chart)
			showMatrixStylePicker := !plot && !showBarStylePicker && !showClockStylePicker && hasMatrixStyleOption(viewRes.Chart)
			if showPlotStylePicker || showBarStylePicker || showClockStylePicker || showMatrixStylePicker || len(swaps) > 1 || len(meta.Dims) > 0 || len(meta.Vars) > 0 {
				if showHeroHead {
					ui.Box(c).FillWidth().Height(1).Background(p.hairline)
				}
				ui.Row(c).AlignItems(ui.Center).Gap(18).Wrap().Children(func() {
					if len(swaps) > 1 {
						a.componentSwapPicker(c, sc.ID, swaps, compSpec.Type, compact)
					}
					if showPlotStylePicker {
						labels := make([]string, len(plotOpts))
						cur := 0
						want := resolveStyle(viewRes.Chart, subStyle)
						for i, o := range plotOpts {
							labels[i] = tr(o.label)
							if o.key == want {
								cur = i
							}
						}
						ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
							ui.Text(c, tr("样式")).FontSize(fsSmall).TextColor(p.muted)
							pick := choose
							if compact && len(labels) > 3 {
								pick = dropdown
							}
							if pick(c, &cur, labels...) {
								a.setCardStyle(sc.ID, plotOpts[cur].key)
							}
						})
					} else if showBarStylePicker {
						a.progressBarStylePicker(c, sc.ID, viewRes, subStyle)
					} else if showClockStylePicker {
						a.clockStylePicker(c, sc.ID, viewRes.Chart.Clock, subStyle)
					} else if showMatrixStylePicker {
						a.matrixWidgetStylePicker(c, sc.ID, viewRes.Chart, subStyle)
					}
					a.dimRow(c, idx, meta.Dims)
					a.varRow(c, sc, meta)
				})
			}

			h := float32(340)
			if a.big && plot {
				h = max(340, a.height-340)
			}
			if compact {
				h = min(h, 300)
			}
			if comp != nil {
				comp.RenderFull(c, ComponentRenderReq{
					App: a, Script: sc, Result: viewRes, Style: subStyle,
					Size: -1, Width: a.contentW() - 2*compactPad(compact), Height: h, Selected: sel, Pinned: pinned, Now: a.now(), Full: true,
				})
				if compSpec.Capabilities.HasDetail {
					ui.Row(c).AlignItems(ui.Center).Gap(10).Wrap().Children(func() {
						Detail(c, viewRes.Chart, sel)
						if plot && *pinned >= 0 {
							if btn(c.Key("unpin:"+sc.ID), tr("取消固定")).Clicked() {
								*pinned = -1
								*sel = -1
							}
						} else if plot && !compact && count(viewRes.Chart) > 1 {
							ui.Text(c, tr("←/→ 逐点查看 · 点击固定")).FontSize(fsTiny).TextColor(p.faint)
						}
					})
				}
				if plot && rawChart != nil && len(rawChart.Series) >= 2 {
					a.interactiveLegendRow(c, sc.ID, rawChart)
				}
			}
			if len(meta.Inputs) > 0 {
				ui.Box(c).Height(1).Shrink(0).Background(p.hairline)
				ui.Column(c).Gap(8).MaxWidth(420).Children(func() {
					for _, in := range meta.Inputs {
						a.quickEntry(c, sc.ID, in)
					}
				})
			}
		})

		// The pictures of the script, as a carousel, with the history cleared by the
		// button under them.
		if shown := shownImages(res.Images); len(shown) > 0 {
			card(c).Padding(compactPad(compact)).Gap(gapItem).Children(func() {
				a.imageCarousel(c, sc, res.Images, a.contentW()-2*compactPad(compact), 360)
				ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Spacer(c)
					if btn(c, tr("清空图片历史")).Clicked() {
						a.clearImages(sc)
					}
				})
			})
		}

		switch {
		case len(res.Stats) > 0:
			a.statsTiles(c, res.Stats)
		case comp != nil && comp.SummaryTiles(a, c, viewRes):
		case hasPB:
			// progressBar has no chart statistics tiles.
		case a.widgetTiles(c, viewRes.Chart):
		case !a.kindTiles(c, viewRes.Chart):
			a.statTiles(c, summarize(viewRes.Chart))
		}

		if len(res.Lines) > 0 || len(res.Logs) > 0 {
			card(c).Padding(16).Gap(4).Children(func() {
				for _, line := range res.Lines {
					ui.Text(c, line).FontSize(fsCaption)
				}
				for _, l := range res.Logs {
					ui.Text(c, "log: "+l).FontSize(fsTiny).TextColor(p.muted)
				}
			})
		}
		a.infoBlock(c, sc, meta, compact)
	})
	// Keyboard scrubbing for plot charts in Visual mode: Left/Right/Home/End.
	if a.fullModeOf(sc.ID) == 0 && st.err == nil && !st.loading {
		comp, viewRes, _ := resolveComponent(st.res, a.styles[sc.ID])
		if comp != nil && comp.Spec().Capabilities.Plot {
			if n := count(viewRes.Chart); n > 0 {
				stepTo := func(idx int) {
					idx = min(max(idx, 0), n-1)
					*pinned = idx
					*sel = idx
				}
				switch {
				case pageEl.OverlayShortcut(0, ui.KeyLeft):
					cur := *sel
					if cur < 0 {
						cur = n - 1
					} else {
						cur--
					}
					stepTo(cur)
				case pageEl.OverlayShortcut(0, ui.KeyRight):
					cur := *sel
					if cur < 0 {
						cur = 0
					} else {
						cur++
					}
					stepTo(cur)
				case pageEl.OverlayShortcut(0, ui.KeyHome):
					stepTo(0)
				case pageEl.OverlayShortcut(0, ui.KeyEnd):
					stepTo(n - 1)
				}
			}
		}
	}
}

func compactPad(compact bool) float32 {
	if compact {
		return 14
	}
	return 20
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// infoBlock says what the script is: where it comes from and what it reads.
func (a *app) infoBlock(c *ui.Context, sc store.Script, meta engine.Meta, compact bool) {
	p := pal(c)
	val := func(text string) func() {
		return func() { ui.Text(c, text).FontSize(fsCaption).TextColor(p.muted) }
	}
	kind := originLabel(a.originOf(sc))
	if isPluginScript(sc.ID) {
		kind += " · " + sc.ID
	}
	if meta.Version != "" {
		kind += " · v" + meta.Version
	}
	r := spec.Resolve(meta)
	where := tr("电脑与手机")
	if pl := platformLabel(r); pl != "" {
		where = pl
	}
	needs := make([]string, len(r.Needs))
	for i, n := range r.Needs {
		needs[i] = needLabel(n)
	}
	rows := []row{
		{title: tr("来源"), control: val(kind)},
		{title: tr("平台"), control: val(where)},
		{title: tr("图标"), control: val(meta.Icon)},
		{title: tr("首页显示"), control: val(onOff(sc.Home))},
	}
	if len(needs) > 0 {
		rows = append(rows, row{title: tr("依赖"), control: val(joinNames(needs))})
	}
	if len(meta.Dims) > 0 {
		rows = append(rows, row{title: tr("可调维度"), control: val(dimNames(meta.Dims))})
	}
	if len(meta.Sources) > 0 {
		rows = append(rows, row{title: tr("数据来源"), control: val(sourceNames(meta.Sources))})
	}
	if len(meta.Feeds) > 0 {
		rows = append(rows, row{title: tr("数据源"), control: val(joinNames(meta.Feeds))})
	}
	if meta.NetProbe != nil {
		rounds := meta.NetProbe.Rounds
		if rounds <= 0 {
			rounds = 12
		}
		rows = append(rows, row{title: tr("网络探测"), control: val(fmt.Sprintf(tr("%d 轮连通性 + 网站分流"), rounds))})
	}
	// The details fold: one line with a summary, opened by its link. Closed,
	// they take a line at the foot of the page rather than a block.
	ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
		label(c, tr("关于"))
		ui.Text(c, kind+" · "+where).FontSize(fsSmall).TextColor(p.muted).NoWrap()
		ui.Spacer(c)
		toggle := tr("展开")
		if a.aboutOpen {
			toggle = tr("收起")
		}
		if ui.Text(c, toggle).FontSize(fsSmall).TextColor(p.accent).Cursor(ui.CursorPointer).Clicked() {
			a.aboutOpen = !a.aboutOpen
		}
	})
	if a.aboutOpen {
		rows = append(rows, row{title: tr("数据"), note: tr("清空这个脚本取过的数据和历史，下次打开重新取"), control: func() {
			a.resetButton(c, sc)
		}})
		group(c, "", compact, rows...)
	}
}

func joinNames(names []string) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += "、"
		}
		out += n
	}
	return out
}

func onOff(b bool) string {
	if b {
		return tr("是")
	}
	return tr("否")
}

func dimNames(dims []engine.Dim) string {
	out := ""
	for i, d := range dims {
		if i > 0 {
			out += "、"
		}
		out += d.Label
	}
	return out
}

func sourceNames[V any](src map[string]V) string {
	out := ""
	for k := range src {
		if out != "" {
			out += "、"
		}
		out += k
	}
	return out
}

// hoveredIndex is the point the pointer is on, or -1 when none is chosen.
func hoveredIndex(ch *engine.Chart, sel *int) int {
	if ch == nil || sel == nil || *sel < 0 || *sel >= count(ch) {
		return -1
	}
	return *sel
}

// heroText is the headline figure of a full view: the newest value, or the
// value of the point under the pointer.
func heroText(res engine.Result, sel *int) string {
	ch := res.Chart
	i := hoveredIndex(ch, sel)
	switch {
	case i < 0:
		return res.Big
	case ch.Type == "candle" && i < len(ch.Candles):
		return fmt.Sprintf("%.2f", ch.Candles[i].C)
	case isPanels(ch):
		for _, sr := range ch.Panels[0].Series {
			if i < len(sr.Points) {
				return fmt.Sprintf("%.2f", sr.Points[i])
			}
		}
	}
	for _, sr := range ch.Series {
		if i < len(sr.Points) {
			return fmt.Sprintf("%.2f", sr.Points[i])
		}
	}
	return res.Big
}

// detailRow is the readout of the point under the pointer, as a quote board
// shows it under the price: its label, then the open, high, low and close of
// a candle, or the value of each line.
func detailRow(c *ui.Context, ch *engine.Chart, i int) {
	p := pal(c)
	label := fmt.Sprintf(tr("第 %d 个"), i+1)
	if i < len(ch.Labels) && ch.Labels[i] != "" {
		label = ch.Labels[i]
	}
	field := func(name, value string, color ui.Color) {
		ui.Text(c, name).FontSize(fsSmall).TextColor(p.muted)
		ui.Text(c, value).FontSize(fsSmall).Bold().TextColor(color).NoWrap()
	}
	ui.Row(c).AlignItems(ui.Center).Gap(10).Wrap().Children(func() {
		ui.Text(c, label).FontSize(fsSmall).TextColor(p.muted).NoWrap()
		if ch.Type == "candle" && i < len(ch.Candles) {
			cd := ch.Candles[i]
			up := p.up
			if cd.C < cd.O {
				up = p.down
			}
			field("O", fmt.Sprintf("%.2f", cd.O), p.text)
			field("H", fmt.Sprintf("%.2f", cd.H), p.text)
			field("L", fmt.Sprintf("%.2f", cd.L), p.text)
			field("C", fmt.Sprintf("%.2f", cd.C), up)
			return
		}
		series := ch.Series
		if isPanels(ch) {
			series = ch.Panels[0].Series
		}
		for _, sr := range series {
			if i < len(sr.Points) {
				field(sr.Name, fmt.Sprintf("%.2f", sr.Points[i]), p.text)
			}
		}
	})
}

func (a *app) fullModeOf(id string) int {
	if a.fullModes == nil {
		return 0
	}
	return a.fullModes[id]
}

func (a *app) setFullMode(id string, mode int) {
	if a.fullModes == nil {
		a.fullModes = map[string]int{}
	}
	a.fullModes[id] = mode
}

func (a *app) setCardStyle(id, style string) {
	if a.styles == nil {
		a.styles = map[string]string{}
	}
	if style == "" {
		delete(a.styles, id)
	} else {
		a.styles[id] = style
	}
	if a.set.CardStyles == nil {
		a.set.CardStyles = map[string]string{}
	}
	if style == "" {
		delete(a.set.CardStyles, id)
	} else {
		a.set.CardStyles[id] = style
	}
	if err := a.store.SetSettings(a.set); err != nil {
		a.set = a.store.Settings()
	}
}

func (a *app) pinnedSelOf(id string) *int {
	if a.pinnedSels == nil {
		a.pinnedSels = map[string]*int{}
	}
	if p, ok := a.pinnedSels[id]; ok {
		return p
	}
	v := -1
	a.pinnedSels[id] = &v
	return &v
}

// filterVisibleSeries returns a shallow copy of ch containing only series not
// hidden by the user's interactive legend toggles.
func (a *app) filterVisibleSeries(id string, ch *engine.Chart) *engine.Chart {
	if ch == nil || len(ch.Series) < 2 || a.hiddenSeries == nil {
		return ch
	}
	hidden := a.hiddenSeries[id]
	if len(hidden) == 0 {
		return ch
	}
	var kept []engine.Series
	for i, sr := range ch.Series {
		if !hidden[i] {
			kept = append(kept, sr)
		}
	}
	if len(kept) == 0 {
		return ch
	}
	cp := *ch
	cp.Series = kept
	return &cp
}

func (a *app) toggleSeriesVisible(id string, idx, total int) {
	if a.hiddenSeries == nil {
		a.hiddenSeries = map[string]map[int]bool{}
	}
	m := a.hiddenSeries[id]
	if m == nil {
		m = map[int]bool{}
		a.hiddenSeries[id] = m
	}
	if m[idx] {
		delete(m, idx)
		return
	}
	// Never hide the last visible series.
	if len(m) >= total-1 {
		return
	}
	m[idx] = true
}

// interactiveLegendRow renders clickable series pills that toggle visibility on
// multi-series charts and auto-rescale the Y axis.
func (a *app) interactiveLegendRow(c *ui.Context, id string, ch *engine.Chart) {
	if ch == nil || len(ch.Series) < 2 {
		return
	}
	p := pal(c)
	hidden := map[int]bool(nil)
	if a.hiddenSeries != nil {
		hidden = a.hiddenSeries[id]
	}
	ui.Row(c).Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
		for i, sr := range ch.Series {
			i, sr := i, sr
			name := sr.Name
			if name == "" {
				name = fmt.Sprintf("#%d", i+1)
			}
			isHidden := hidden[i]
			col := parseColor(sr.Color, seriesPalette[i%len(seriesPalette)])
			dotCol := col
			txtCol := p.text
			bg := p.chip
			if isHidden {
				dotCol = p.faint
				txtCol = p.faint
				bg = ui.Color{}
			}
			el := ui.ButtonBase(c.Key(fmt.Sprintf("leg:%s:%d", id, i))).
				Padding(4, 9).Radius(99).Gap(6).AlignItems(ui.Center).
				BorderWidth(1).BorderColor(p.hairline).Cursor(ui.CursorPointer)
			if !isHidden {
				el = el.Background(bg)
			}
			if el.Children(func() {
				ui.Box(c).Size(8, 8).Radius(4).Background(dotCol).Shrink(0)
				ui.Text(c, name).FontSize(fsTiny).TextColor(txtCol).NoWrap()
			}).Clicked() {
				a.toggleSeriesVisible(id, i, len(ch.Series))
			}
		}
		if len(hidden) > 0 {
			if ui.Text(c, tr("全部显示")).FontSize(fsTiny).TextColor(p.accent).Cursor(ui.CursorPointer).Clicked() {
				delete(a.hiddenSeries, id)
			}
		}
	})
}

// fullViewModeBar lets the user inspect any script as Visual, Data Table, or Raw Data.
func (a *app) fullViewModeBar(c *ui.Context, id string, hasTable, compact bool) {
	mode := a.fullModeOf(id)
	labels := []string{tr("可视化")}
	modes := []int{0}
	if hasTable {
		labels = append(labels, tr("数据表"))
		modes = append(modes, 1)
	}
	labels = append(labels, tr("原始数据"))
	modes = append(modes, 2)
	cur := 0
	for i, m := range modes {
		if m == mode {
			cur = i
		}
	}
	ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
		if choose(c, &cur, labels...) {
			a.setFullMode(id, modes[cur])
		}
	})
}

// visualSwapOptions lists compatible non-table visual components for res.
func visualSwapOptions(res engine.Result) []string {
	base := engine.ResultWidgetType(res)
	if base == "" || base == "line" || base == "dot" || base == "candle" || base == "combo" || base == "panels" {
		// Plot charts already have their dedicated style switcher (折线/面积/柱状/网格…).
		return nil
	}
	all := engine.SwappableTypes(res)
	out := make([]string, 0, len(all))
	for _, t := range all {
		if t == "table" && base != "table" {
			// Table inspection already has its own top-level "数据表" mode tab.
			continue
		}
		out = append(out, t)
	}
	return out
}

func widgetDisplayName(typ string) string {
	s, ok := engine.WidgetSpecOf(typ)
	if !ok {
		return typ
	}
	if curLang() == langEN {
		return s.EnName
	}
	return s.ZhName
}

// componentSwapPicker lets the user swap between compatible component views at runtime.
func (a *app) componentSwapPicker(c *ui.Context, id string, swaps []string, activeType string, compact bool) {
	p := pal(c)
	labels := make([]string, len(swaps))
	cur := 0
	for i, t := range swaps {
		labels[i] = widgetDisplayName(t)
		if t == activeType {
			cur = i
		}
	}
	ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
		ui.Text(c, tr("视图")).FontSize(fsSmall).TextColor(p.muted)
		pick := choose
		if compact && len(labels) > 3 {
			pick = dropdown
		}
		if pick(c, &cur, labels...) {
			chosen := swaps[cur]
			if cur == 0 {
				a.setCardStyle(id, "")
			} else {
				a.setCardStyle(id, "view:"+chosen)
			}
		}
	})
}

// progressBarStylePicker switches a progressBar between neon / matrix / gradient / solid / segmented.
func (a *app) progressBarStylePicker(c *ui.Context, id string, res engine.Result, subStyle string) {
	p := pal(c)
	pb, ok := progressBarOf(res)
	if !ok {
		return
	}
	opts := []struct{ key, label string }{
		{"neon", "霓虹灯"},
		{"matrix", "点阵灯"},
		{"gradient", "渐变"},
		{"solid", "纯色"},
		{"segmented", "分段"},
	}
	curStyle := subStyle
	if curStyle == "" {
		curStyle = pb.Style
	}
	if curStyle == "" {
		curStyle = "solid"
	}
	cur := 0
	labels := make([]string, len(opts))
	for i, o := range opts {
		labels[i] = tr(o.label)
		if o.key == curStyle {
			cur = i
		}
	}
	ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
		ui.Text(c, tr("进度条风格")).FontSize(fsSmall).TextColor(p.muted)
		if choose(c, &cur, labels...) {
			swap, _ := parseViewOverride(a.styles[id])
			if swap != "" {
				a.setCardStyle(id, "view:"+swap+":"+opts[cur].key)
			} else {
				a.setCardStyle(id, opts[cur].key)
			}
		}
	})
}

// clockStylePicker switches a clock widget between analog, digital, matrix, and world styles.
func (a *app) clockStylePicker(c *ui.Context, id string, spec *engine.ClockSpec, subStyle string) {
	p := pal(c)
	opts := []struct{ key, label string }{
		{"analog", "指针表盘"},
		{"digital", "数字"},
		{"matrix", "点阵灯"},
	}
	if spec != nil && len(spec.Zones) > 0 {
		opts = append(opts, struct{ key, label string }{"world", "世界时钟"})
	}
	curStyle := subStyle
	if curStyle == "" && spec != nil {
		curStyle = spec.Style
	}
	if curStyle == "" {
		curStyle = "analog"
	}
	cur := 0
	labels := make([]string, len(opts))
	for i, o := range opts {
		labels[i] = tr(o.label)
		if o.key == curStyle {
			cur = i
		}
	}
	ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
		ui.Text(c, tr("时钟样式")).FontSize(fsSmall).TextColor(p.muted)
		if choose(c, &cur, labels...) {
			a.setCardStyle(id, opts[cur].key)
		}
	})
}

func hasMatrixStyleOption(ch *engine.Chart) bool {
	if ch == nil {
		return false
	}
	switch ch.Type {
	case "donut", "progressRing", "gauge", "contrib":
		return true
	}
	return false
}

// matrixWidgetStylePicker lets donut, progressRing, gauge, and contrib toggle
// between their classic appearance and the Matrix LED dot style at runtime.
func (a *app) matrixWidgetStylePicker(c *ui.Context, id string, ch *engine.Chart, subStyle string) {
	if ch == nil {
		return
	}
	p := pal(c)
	var opts []struct{ key, label string }
	curStyle := subStyle
	switch ch.Type {
	case "donut":
		opts = []struct{ key, label string }{{"classic", "经典"}, {"matrix", "点阵灯"}}
		if curStyle == "" {
			if ch.Donut != nil && ch.Donut.Style == "matrix" {
				curStyle = "matrix"
			} else {
				curStyle = "classic"
			}
		}
	case "progressRing":
		opts = []struct{ key, label string }{{"gradient", "渐变"}, {"solid", "纯色"}, {"matrix", "点阵灯"}}
		if curStyle == "" {
			if ch.Ring != nil && ch.Ring.Fill != "" {
				curStyle = ch.Ring.Fill
			} else {
				curStyle = "gradient"
			}
		}
	case "gauge":
		opts = []struct{ key, label string }{{"classic", "经典"}, {"matrix", "点阵灯"}}
		if curStyle == "" {
			if ch.Gauge != nil && ch.Gauge.Style == "matrix" {
				curStyle = "matrix"
			} else {
				curStyle = "classic"
			}
		}
	case "contrib":
		opts = []struct{ key, label string }{{"classic", "方块"}, {"matrix", "点阵灯"}}
		if curStyle == "" {
			if ch.Contrib != nil && ch.Contrib.Style == "matrix" {
				curStyle = "matrix"
			} else {
				curStyle = "classic"
			}
		}
	default:
		return
	}
	cur := 0
	labels := make([]string, len(opts))
	for i, o := range opts {
		labels[i] = tr(o.label)
		if o.key == curStyle {
			cur = i
		}
	}
	ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
		ui.Text(c, tr("风格")).FontSize(fsSmall).TextColor(p.muted)
		if choose(c, &cur, labels...) {
			swap, _ := parseViewOverride(a.styles[id])
			if swap != "" {
				a.setCardStyle(id, "view:"+swap+":"+opts[cur].key)
			} else {
				a.setCardStyle(id, opts[cur].key)
			}
		}
	})
}

// rawInspectorCard displays structured diagnostic metadata, source fallback
// chains, and formatted JSON output for a script's latest result.
func (a *app) rawInspectorCard(c *ui.Context, sc store.Script, meta engine.Meta, res engine.Result, compact bool) {
	p := pal(c)
	rawBytes, _ := json.MarshalIndent(res, "", "  ")
	rawJSON := string(rawBytes)

	typ := engine.ResultWidgetType(res)
	if typ == "" {
		typ = "—"
	} else {
		typ = fmt.Sprintf("%s (%s)", widgetDisplayName(typ), typ)
	}
	dataTime := tr("未记录")
	if res.DataAt > 0 {
		dataTime = fmtTime(res.DataAt)
	}
	cacheState := tr("实时数据")
	if res.Stale {
		cacheState = tr("缓存回退")
	}

	card(c).Padding(compactPad(compact)).Gap(gapItem).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(10).Wrap().Children(func() {
			ui.Text(c, tr("原始数据")).FontSize(fsHeading).FontWeight(650)
			chip(c, typ, p.accent, p.accent.Alpha(0.14))
			chip(c, cacheState, p.muted, p.chip)
			ui.Text(c, tr("数据时间")+" · "+dataTime).FontSize(fsTiny).TextColor(p.muted)
			ui.Spacer(c)
			if btn(c, tr("复制 JSON")).Clicked() {
				copyText(rawJSON)
				a.status = tr("已复制 JSON")
			}
		})
		if a.status != "" {
			ui.Text(c, a.status).FontSize(fsTiny).TextColor(p.accent)
		}
		if len(meta.Sources) > 0 {
			keys := make([]string, 0, len(meta.Sources))
			for k := range meta.Sources {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			ui.Column(c).Gap(4).Padding(10, 12).Radius(radControl).Background(p.chip).Children(func() {
				ui.Text(c, tr("数据来源")).FontSize(fsTiny).FontWeight(650).TextColor(p.muted)
				for _, k := range keys {
					src := meta.Sources[k]
					line := fmt.Sprintf("%s: %s", k, src.URL)
					if len(src.Fallbacks) > 0 {
						line += fmt.Sprintf(tr("（+%d 个备选源）"), len(src.Fallbacks))
					}
					ui.Text(c, line).FontSize(fsTiny).TextColor(p.text)
				}
			})
		}
		ui.Scroll(c.Key("raw-json:" + sc.ID)).Height(360).Padding(12).Radius(radControl).Background(p.chip).Children(func() {
			ui.Text(c, rawJSON).Font("monospace").FontSize(fsTiny).TextColor(p.text)
		})
	})
}
