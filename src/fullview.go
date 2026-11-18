package main

import (
	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
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
					ui.Text(c, res.Big).FontSize(36).Bold().NoWrap()
					ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
						if cs := summarize(res.Chart); cs.ok {
							deltaChip(c, cs.pct)
						}
						ui.Text(c, res.Sub).FontSize(fsCaption).TextColor(p.muted)
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
						if choose(c, &cur, labels...) {
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
		infoBlock(c, sc, meta, compact)
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
func infoBlock(c *ui.Context, sc store.Script, meta engine.Meta, compact bool) {
	p := pal(c)
	val := func(text string) func() {
		return func() { ui.Text(c, text).FontSize(fsCaption).TextColor(p.muted) }
	}
	kind := tr("内置脚本")
	if isPluginScript(sc.ID) {
		kind = tr("本地插件 · ") + sc.ID
	}
	rows := []row{
		{title: tr("来源"), control: val(kind)},
		{title: tr("图标"), control: val(meta.Icon)},
		{title: tr("首页显示"), control: val(onOff(sc.Home))},
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
