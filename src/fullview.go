package main

import (
	"fmt"

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
		st = a.ensure(runKey(sc.ID, meta.Dims, vals), sc, vals)
	}
	sel := a.selOf("full:" + sc.ID)

	page(c, compact, func() {
		subheader(c, a, tr(sc.Name), st.res.Title, func() {
			if compact {
				// A phone has room for icons, not for a row of buttons; 大图 gives
				// the chart the window's height, which a phone has none to spare.
				a.refreshButton(c, sc)
				if action.ActionIcon(c, icon("pencil-line"), action.ActionIconProps{Size: theme.Small, Title: tr("编辑")}).Clicked() {
					a.router.Push("/edit/" + sc.ID)
				}
				return
			}
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				if ui.Text(c, tr("大图")).FontSize(fsCaption).Cursor(ui.CursorPointer).Clicked() {
					a.big = !a.big
				}
				sw(c, &a.big)
			})
			if btn(c, tr("刷新")).Clicked() {
				a.refreshScript(sc)
			}
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
		accent := accentOf(res, p.accent)

		// The hero: the headline number, the controls, and the chart.
		card(c).Padding(compactPad(compact)).Gap(gapItem).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(14).Wrap().Children(func() {
				iconBadge(c, firstNonEmpty(res.Icon, meta.Icon), accent, 44)
				ui.Column(c).Gap(2).Children(func() {
					ui.Text(c, tickText(c, "hero:"+sc.ID, heroText(res, sel))).FontSize(36).Bold().NoWrap()
					ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
						if cs := summarize(res.Chart); cs.ok {
							deltaChip(c, cs.pct)
						}
						if i := hoveredIndex(res.Chart, sel); i >= 0 {
							detailRow(c, res.Chart, i)
						} else {
							ui.Text(c, res.Sub).FontSize(fsCaption).TextColor(p.muted)
						}
					})
				})
			})

			opts := stylesFor(res.Chart)
			labels := make([]string, len(opts))
			cur := 0
			want := resolveStyle(res.Chart, a.styles[sc.ID])
			for i, o := range opts {
				labels[i] = tr(o.label)
				if o.key == want {
					cur = i
				}
			}
			ui.Row(c).AlignItems(ui.Center).Gap(18).Wrap().Children(func() {
				if !isGaugeOrBars(res.Chart) {
					ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
						ui.Text(c, tr("样式")).FontSize(fsSmall).TextColor(p.muted)
						pick := choose
						if compact && len(labels) > 3 {
							pick = dropdown
						}
						if pick(c, &cur, labels...) {
							a.styles[sc.ID] = opts[cur].key
						}
					})
				}
				a.dimRow(c, idx, meta.Dims)
			})

			h := float32(340)
			if a.big {
				h = max(340, a.height-340)
			}
			if compact {
				h = min(h, 300)
			}
			Chart(c, ChartProps{Data: res.Chart, Style: a.styles[sc.ID], Height: h, Selected: sel, Full: true})
			Detail(c, res.Chart, sel)
		})

		if !a.kindTiles(c, res.Chart) {
			a.statTiles(c, summarize(res.Chart))
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
	group(c, tr("关于"), compact, rows...)
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

func sourceNames(src map[string]string) string {
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
		for _, sr := range ch.Series {
			if i < len(sr.Points) {
				field(sr.Name, fmt.Sprintf("%.2f", sr.Points[i]), p.text)
			}
		}
	})
}
