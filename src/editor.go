package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/display"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

var metricLabels = []string{"体重", "金价"}

// metricKey is the name the selected metric is stored and read under.
func (a *app) metricKey() string {
	if a.metricIdx == 1 {
		return "gold"
	}
	return "weight"
}

// openEditor selects script id in the editor. It does nothing when id is
// already selected, so the view may call it every frame.
func (a *app) openEditor(id string) {
	if id == "" || id == a.editorID {
		return
	}
	sc, ok := a.store.Script(id)
	if !ok {
		return
	}
	a.editorID = id
	a.draft = sc
	a.status = ""
	delete(a.dims, draftKey)
	a.invalidate(draftKey)
}

// newScript starts a new script in the editor. It is stored on Save.
func (a *app) newScript() {
	id := fmt.Sprintf("script-%d", a.now().UnixNano())
	a.editorID = id
	a.draft = store.Script{ID: id, Name: tr("新脚本"), Source: engine.DefaultSourceFor(curLang())}
	a.status = tr("未保存")
	delete(a.dims, draftKey)
	a.invalidate(draftKey)
}

// duplicateDraft saves a copy of the draft as a new script, on the home page,
// and opens it: a copy of the weather with another place is one edit away.
func (a *app) duplicateDraft() {
	sc := store.Script{
		ID:     fmt.Sprintf("script-%d", a.now().UnixNano()),
		Name:   a.draft.Name + " " + tr("副本"),
		Source: a.draft.Source,
		Home:   true,
	}
	if err := a.store.SaveScript(sc); err != nil {
		a.status = tr("复制失败：") + err.Error()
		return
	}
	a.editorID = sc.ID
	a.draft = sc
	delete(a.dims, draftKey)
	a.invalidate(draftKey)
	a.invalidateAll()
	a.status = tr("已复制，可以修改后保存")
}

// saveDraft stores the draft and drops the cards' runs of it. A plugin's
// script is changed in its folder, or with the MCP tools, not here.
func (a *app) saveDraft() {
	if isPluginScript(a.draft.ID) {
		a.status = tr("插件的脚本请在插件文件夹里修改，或用 MCP 工具修改")
		return
	}
	if err := a.store.SaveScript(a.draft); err != nil {
		a.status = tr("保存失败：") + err.Error()
		return
	}
	a.invalidate(a.draft.ID)
	a.invalidate(draftKey)
	a.status = tr("已保存")
}

// record stores the value typed in the editor for the selected metric. Weight
// is typed in the unit of the settings and stored in kg.
func (a *app) record() {
	if err := a.recordMetric(a.metricKey(), a.entry); err != nil {
		a.status = tr("记录失败：") + err.Error()
		return
	}
	a.status = tr("已记录 ") + tr(metricLabels[a.metricIdx])
}

// recordMetric stores a value typed in by hand under metric, now, and drops
// every run so that cards show it. Weight is typed in the unit of the
// settings and stored in kg.
func (a *app) recordMetric(metric string, v float64) error {
	if metric == "weight" && a.store.Settings().WeightUnit == "斤" {
		v /= 2
	}
	if err := a.store.Append(metric, engine.Point{T: a.now().UnixMilli(), V: v}); err != nil {
		return err
	}
	a.invalidateAll()
	return nil
}

// shownValue converts a stored value of metric key to the unit the editor
// shows it in.
func (a *app) shownValue(key string, v float64) float64 {
	if key == "weight" && a.store.Settings().WeightUnit == "斤" {
		return v * 2
	}
	return v
}

// editor is the editing page: the scripts, the draft with its source, a
// preview of its output and the form that records values. On a phone the
// blocks stack in one column.
func (a *app) editor(c *ui.Context, compact bool) {
	p := pal(c)
	if a.editorID == "" {
		if list := a.store.Scripts(); len(list) > 0 {
			a.openEditor(list[0].ID)
		}
	}
	if a.draft.ID == "" {
		page(c, compact, func() {
			header(c, tr("编辑"), "", nil)
			card(c).Children(func() { empty(c, "pencil-line", tr("还没有脚本"), tr("点「新建」写第一个脚本。")) })
		})
		return
	}
	if a.entry == 0 {
		if pts := a.store.Series(a.metricKey()); len(pts) > 0 {
			a.entry = a.shownValue(a.metricKey(), pts[len(pts)-1].V)
		}
	}

	// Three columns need room; on a narrower window the scripts become a row of
	// chips above two columns, and on a phone everything stacks.
	avail := a.width - a.sidebarW() - 32
	wide := !compact && avail >= 1000
	listRow := !wide

	scripts := func() {
		ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
			label(c, tr("脚本"))
			ui.Spacer(c)
			if btn(c, tr("导入")).Clicked() {
				a.panel, a.transferText, a.transferErr = "import", "", ""
			}
			if btn(c, tr("+ 新建")).Clicked() {
				a.newScript()
			}
		})
		if !listRow {
			items := []display.ListItem{}
			for _, sc := range a.store.Scripts() {
				meta, _ := a.metaOf(sc)
				it := display.ListItem{Key: sc.ID, Label: tr(sc.Name), Icon: icon(meta.Icon)}
				if isPluginScript(sc.ID) {
					it.Description = tr("本地插件")
				}
				items = append(items, it)
			}
			if key := display.List(c, display.ListProps{Items: items, Active: a.editorID}); key != "" {
				a.openEditor(key)
			}
			return
		}
		list := func() {
			for _, sc := range a.store.Scripts() {
				sc := sc
				meta, _ := a.metaOf(sc)
				active := sc.ID == a.editorID
				el := ui.Row(c).AlignItems(ui.Center).Gap(10).Padding(8, 10).Radius(radControl).Cursor(ui.CursorPointer).
					Children(func() {
						iconBadge(c, meta.Icon, p.accent, 28)
						ui.Column(c).Children(func() {
							t := ui.Text(c, tr(sc.Name)).FontSize(fsBody).NoWrap()
							if active {
								t.Bold()
							}
							if isPluginScript(sc.ID) {
								ui.Text(c, tr("本地插件")).FontSize(fsTiny).TextColor(p.muted)
							}
						})
					})
				switch {
				case active:
					el.Background(p.chip)
				case el.Hovered():
					el.Background(p.hover)
				}
				if el.Clicked() {
					a.openEditor(sc.ID)
				}
			}
		}
		if listRow {
			ui.Row(c).Wrap().Gap(6).Children(list)
		} else {
			ui.Column(c).Gap(2).Children(list)
		}
	}

	sourceH := max(220, a.height-440)
	if compact {
		sourceH = 260
	}
	draft := func() {
		ui.Row(c).AlignItems(ui.Center).Gap(8).Wrap().Children(func() {
			ui.Column(c).Gap(2).Children(func() {
				ui.Text(c, tr("编辑脚本")).FontSize(fsEmphasis).Bold()
				if a.status != "" {
					ui.Text(c, a.status).FontSize(fsSmall).TextColor(p.muted)
				}
			})
			ui.Spacer(c)
			if btn(c, tr("复制")).Clicked() {
				a.duplicateDraft()
			}
			if btn(c, tr("导出")).Clicked() {
				a.panel, a.transferText, a.transferErr = "export", exportScript(a.draft), ""
			}
			if btn(c, tr("运行")).Clicked() {
				a.invalidate(draftKey)
			}
			if primaryBtn(c, tr("保存")).Clicked() {
				a.saveDraft()
			}
		})
		a.transferPanel(c)
		field(c, tr("名称"), "", func() {
			textInput(c, &a.draft.Name, tr("脚本名称"))
		})
		ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
			sw(c, &a.draft.Home)
			ui.Text(c, tr("显示在首页")).FontSize(fsBody)
		})
		field(c, tr("源码"), tr("ES 模块：导出 meta() 与 render(ctx)；log(...) 的输出会显示在预览里"), func() {
			textArea(c, &a.draft.Source, "", int(sourceH/17)).Font("monospace").FontSize(fsSmall)
		})
	}

	preview := func() {
		meta, err := a.metaOf(a.draft)
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, tr("预览")).FontSize(fsHeading).Bold()
			ui.Spacer(c)
		})
		if err != nil {
			ui.Text(c, err.Error()).FontSize(fsSmall).TextColor(p.up)
			return
		}
		idx, vals := a.viewOf(draftKey, meta.Dims)
		st := a.ensure(runKey(draftKey, meta.Dims, vals), a.draft, vals)
		a.dimRow(c, idx, meta.Dims)
		switch {
		case st.loading:
			ui.Text(c, tr("运行中…")).TextColor(p.muted)
		case st.err != nil:
			ui.Text(c, st.err.Error()).FontSize(fsSmall).TextColor(p.up)
		default:
			ui.Text(c, st.res.Title).FontSize(fsSmall).TextColor(p.muted)
			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				ui.Text(c, st.res.Big).FontSize(30).Bold()
				if cs := summarize(st.res.Chart); cs.ok {
					deltaChip(c, cs.pct)
				}
			})
			ui.Text(c, st.res.Sub).FontSize(fsSmall).TextColor(p.muted)
			Chart(c, ChartProps{Data: st.res.Chart, Height: 170, Selected: a.selOf(draftKey)})
			Detail(c, st.res.Chart, a.selOf(draftKey))
			for _, line := range st.res.Lines {
				ui.Text(c, line).FontSize(fsTiny).TextColor(p.muted)
			}
			for _, l := range st.res.Logs {
				ui.Text(c, "log: "+l).FontSize(fsTiny).TextColor(p.muted)
			}
		}
	}

	record := func() {
		ui.Text(c, tr("录入数据")).FontSize(fsHeading).Bold()
		segmented(c, &a.metricIdx, trAll(metricLabels)...)
		name := tr("数值（元/克）")
		if a.metricKey() == "weight" {
			name = tr("数值（") + a.store.Settings().WeightUnit + "）"
		}
		field(c, name, "", func() {
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				numberInput(c, &a.entry, 0, 100000, 0.1).Width(170)
				if primaryBtn(c, tr("记录")).Clicked() {
					a.record()
				}
			})
		})
		a.recent(c)
	}

	if compact {
		page(c, compact, func() {
			header(c, tr("编辑"), "", nil)
			card(c).Padding(14).Gap(10).Children(scripts)
			card(c).Padding(16).Gap(gapItem).Children(draft)
			card(c).Padding(16).Gap(10).Children(preview)
			card(c).Padding(16).Gap(10).Children(record)
		})
		return
	}
	if !wide {
		ui.Column(c).Grow(1).Padding(16).Gap(16).Children(func() {
			card(c).Padding(14).Gap(10).Children(scripts)
			ui.Row(c).Grow(1).AlignItems(ui.Stretch).Gap(16).Children(func() {
				ui.Scroll(c).Grow(1).Children(func() {
					card(c).Padding(20).Gap(gapItem).Children(draft)
				})
				ui.Scroll(c).Width(330).Children(func() {
					ui.Column(c).Gap(16).Children(func() {
						card(c).Padding(18).Gap(10).Children(preview)
						card(c).Padding(18).Gap(10).Children(record)
					})
				})
			})
		})
		return
	}
	ui.Row(c).Grow(1).AlignItems(ui.Stretch).Padding(16).Gap(16).Children(func() {
		card(c).Width(248).Padding(14).Gap(10).Children(scripts)
		ui.Scroll(c).Grow(1).Children(func() {
			card(c).Padding(20).Gap(gapItem).Children(draft)
		})
		ui.Scroll(c).Width(372).Children(func() {
			ui.Column(c).Gap(16).Children(func() {
				card(c).Padding(18).Gap(10).Children(preview)
				card(c).Padding(18).Gap(10).Children(record)
			})
		})
	})
}

// recent lists the latest records of the selected metric.
func (a *app) recent(c *ui.Context) {
	p := pal(c)
	key := a.metricKey()
	pts := a.store.Series(key)
	if len(pts) == 0 {
		ui.Text(c, tr("还没有记录")).FontSize(fsSmall).TextColor(p.muted)
		return
	}
	label(c, tr("最近记录"))
	from := max(0, len(pts)-4)
	for i := len(pts) - 1; i >= from; i-- {
		pt := pts[i]
		pt.V = a.shownValue(key, pt.V)
		ui.Row(c).AlignItems(ui.Center).Padding(6, 0).BorderWidth(0, 0, 1, 0).BorderColor(p.hairline).Children(func() {
			ui.Text(c, fmtTime(pt.T)).FontSize(fsSmall).TextColor(p.muted)
			ui.Spacer(c)
			ui.Text(c, fmt.Sprintf("%.2f", pt.V)).FontSize(fsBody).Bold()
		})
	}
}

// valueText is a short label of a recorded value, for the record list.
func valueText(p engine.Point) string {
	return fmtTime(p.T) + "  " + fmt.Sprintf("%.2f", p.V)
}
