package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"scriptboard/src/internal/store"
	"scriptboard/src/internal/table"
)

// The editor on a phone is two screens, as a phone's settings are: a list of
// what can be edited, and the thing itself. The thing is a script (source, or
// its preview) or a data table (rows as cards, or its preview), one at a time,
// with Save always at the top.

// phoneList is the list of scripts and data tables.
func (a *app) phoneList(c *ui.Context) {
	p := pal(c)
	page(c, true, func() {
		header(c, tr("编辑"), tr("脚本与数据表"), nil)
		ui.Row(c).AlignItems(ui.Center).Gap(8).Wrap().Children(func() {
			if primaryBtn(c, tr("+ 脚本")).Clicked() {
				a.newScript()
				a.router.Push("/edit/" + a.editorID)
			}
			if btn(c, tr("+ 数据表")).Clicked() {
				a.newTable()
				a.router.Push("/edit/" + a.editorID)
			}
			if btn(c, tr("导入脚本")).Clicked() {
				a.panel, a.transferText, a.transferErr = "import", "", ""
			}
		})
		a.transferPanel(c)

		var tables, scripts []store.Script
		for _, sc := range a.store.Scripts() {
			if sc.IsTable() {
				tables = append(tables, sc)
			} else {
				scripts = append(scripts, sc)
			}
		}
		section := func(title string, list []store.Script) {
			if len(list) == 0 {
				return
			}
			label(c, title)
			card(c).Padding(4, 0).Children(func() {
				for i, sc := range list {
					a.phoneRow(c, p, sc, i < len(list)-1)
				}
			})
		}
		section(tr("脚本"), scripts)
		section(tr("数据表"), tables)
	})
}

// phoneRow is one line of the list: its icon, name and where it is from.
func (a *app) phoneRow(c *ui.Context, p palette, sc store.Script, line bool) {
	meta, _ := a.metaOf(sc)
	row := ui.Row(c).AlignItems(ui.Center).Gap(12).Padding(11, 14).Cursor(ui.CursorPointer)
	if line {
		row.BorderWidth(0, 0, 1, 0).BorderColor(p.hairline)
	}
	row.Children(func() {
		iconBadge(c, meta.Icon, p.accent, 36)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(1).Children(func() {
			ui.Row(c).MinWidth(0).ClipX().Children(func() {
				ui.Text(c, tr(sc.Name)).FontSize(fsBody).NoWrap()
			})
			note := ui.Text(c, a.scriptNote(sc)).FontSize(fsTiny).TextColor(p.muted).NoWrap()
			if !a.supported(sc) {
				note.TextColor(p.warn)
			}
		})
		ui.Icon(c, icon("chevron-right")).FontSize(16).TextColor(p.faint)
	})
	if row.Clicked() {
		a.router.Push("/edit/" + sc.ID)
	}
}

// phoneEditor is the thing being edited: a title with Save, a switch between
// the editor and the preview, and one of them under it.
func (a *app) phoneEditor(c *ui.Context, preview func()) {
	p := pal(c)
	isTable := a.draft.IsTable()
	page(c, true, func() {
		subheader(c, a, tr(a.draft.Name), a.scriptNote(a.draft), func() {
			if primaryBtn(c, tr("保存")).Clicked() {
				if isTable {
					a.saveTable()
				} else {
					a.saveDraft()
				}
			}
		})
		if a.status != "" {
			ui.Text(c, a.status).FontSize(fsSmall).TextColor(p.muted)
		}
		first := tr("源码")
		if isTable {
			first = tr("表格")
		}
		before := a.editTab
		segmented(c, &a.editTab, first, tr("预览"))
		if a.editTab != before {
			a.invalidate(draftKey) // run what was just edited
		}
		switch {
		case a.editTab == 1:
			card(c).Padding(16).Gap(10).Children(preview)
		case isTable:
			card(c).Padding(14).Gap(gapItem).Children(func() { a.phoneTable(c) })
		default:
			card(c).Padding(14).Gap(gapItem).Children(func() { a.phoneScript(c) })
		}
	})
}

// phoneScript is the form of a script: its name, whether it is on the home
// page, its source, and the things that can be done with it.
func (a *app) phoneScript(c *ui.Context) {
	field(c, tr("名称"), "", func() { textInput(c, &a.draft.Name, tr("脚本名称")) })
	homeSwitch(c, &a.draft.Home)
	a.specInfo(c, a.draft)
	rows := int(clampF((a.height-360)/17, 9, 40))
	field(c, tr("源码"), tr("ES 模块：导出 meta() 与 render(ctx)；log(...) 的输出会显示在预览里"), func() {
		textArea(c, &a.draft.Source, "", rows).Font("monospace").FontSize(fsSmall)
	})
	ui.Row(c).AlignItems(ui.Center).Gap(8).Wrap().Children(func() {
		if btn(c, tr("复制")).Clicked() {
			a.duplicateDraft()
			a.router.Replace("/edit/" + a.editorID)
		}
		if btn(c, tr("导出")).Clicked() {
			a.panel, a.transferText, a.transferErr = "export", exportScript(a.draft), ""
		}
		if a.originOf(a.draft) == store.OriginBuiltin && btn(c, tr("还原")).Clicked() {
			a.restoreDraft()
		}
		a.deleteButton(c)
	})
	a.transferPanel(c)
}

// deleteButton is Delete, which asks for a second click, when the draft can be
// deleted.
func (a *app) deleteButton(c *ui.Context) {
	if !a.canDelete(a.draft) {
		return
	}
	label := tr("删除")
	if a.confirmDelete == a.draft.ID {
		label = tr("确认删除")
	}
	if btn(c, label).Clicked() {
		a.deleteDraft()
	}
}

// phoneTable is the form of a data table: the name, the import and export
// buttons, the columns, and the rows, each a card of its own fields.
func (a *app) phoneTable(c *ui.Context) {
	p := pal(c)
	t := a.draft.Table
	if t == nil {
		return
	}
	bound := t.Metric != ""
	if !bound {
		field(c, tr("名称"), "", func() { textInput(c, &a.draft.Name, tr("数据表名称")) })
	} else {
		hint(c, fmt.Sprintf(tr("这张表就是「%s」的记录：保存后，首页的卡片和图表都用这里的数据。"), tr(metricName(t.Metric))))
	}
	homeSwitch(c, &a.draft.Home)
	a.importBar(c, t)
	a.pastePanel(c)

	cols := t.Columns
	if bound {
		cols = a.boundColumns(t.Metric)
	}
	changed := false

	// The columns: named in a line, edited on request.
	if !bound {
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(1).Children(func() {
				label(c, tr("列"))
				ui.Text(c, strings.Join(cols, " · ")).FontSize(fsSmall).TextColor(p.muted)
			})
			name := tr("编辑列")
			if a.tableCols {
				name = tr("完成")
			}
			if btn(c, name).Clicked() {
				a.tableCols = !a.tableCols
			}
		})
		if a.tableCols {
			ui.Column(c).Gap(8).Padding(12).Radius(radControl + 2).Background(p.chip).Children(func() {
				for j := range t.Columns {
					field(c, fmt.Sprintf(tr("列 %d"), j+1), "", func() {
						if textInput(c, &t.Columns[j], tr("列名")).Changed() {
							changed = true
						}
					})
				}
				ui.Row(c).Gap(8).Wrap().Children(func() {
					if btn(c, tr("+ 添加一列")).Clicked() && len(t.Columns) < table.MaxCols {
						t.Columns = append(t.Columns, fmt.Sprintf(tr("列 %d"), len(t.Columns)+1))
						changed = true
					}
					if len(t.Columns) > 1 && btn(c, tr("删除最后一列")).Clicked() {
						k := len(t.Columns) - 1
						t.Columns = t.Columns[:k]
						for i := range t.Rows {
							if len(t.Rows[i]) > k {
								t.Rows[i] = t.Rows[i][:k]
							}
						}
						changed = true
					}
				})
			})
		}
	}

	// The rows, a page at a time, newest page last.
	n := max(len(cols), 1)
	size := a.pageSize()
	pages := max(1, (len(t.Rows)+size-1)/size)
	a.tablePage = min(max(a.tablePage, 0), pages-1)
	from := a.tablePage * size
	to := min(len(t.Rows), from+size)
	ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
		label(c, fmt.Sprintf(tr("共 %d 行"), len(t.Rows)))
		ui.Spacer(c)
		if len(t.Rows) > 1 && kindOfColumn(t.Rows, 0).dates && btn(c, tr("按时间排序")).Clicked() {
			sort.SliceStable(t.Rows, func(i, k int) bool {
				ti, _ := table.Time(cell(t.Rows[i], 0))
				tk, _ := table.Time(cell(t.Rows[k], 0))
				return ti.Before(tk)
			})
			changed = true
		}
	})
	if len(t.Rows) == 0 {
		hint(c, tr("还没有数据：点「添加一行」，或粘贴 Excel / CSV 导入"))
	}
	for i := from; i < to; i++ {
		for len(t.Rows[i]) < n {
			t.Rows[i] = append(t.Rows[i], "")
		}
		ui.Column(c).Gap(8).Padding(12).Radius(radControl + 2).BorderWidth(1).BorderColor(p.hairline).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, fmt.Sprintf(tr("第 %d 行"), i+1)).FontSize(fsSmall).FontWeight(600).TextColor(p.muted)
				ui.Spacer(c)
				if action.ActionIcon(c, icon("x"), action.ActionIconProps{Size: theme.Small, Title: tr("删除这一行")}).Clicked() {
					t.Rows = append(t.Rows[:i], t.Rows[i+1:]...)
					changed = true
				}
			})
			if i >= len(t.Rows) {
				return
			}
			for j := 0; j < n; j++ {
				ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
					ui.Row(c).Width(86).MinWidth(0).ClipX().Children(func() {
						ui.Text(c, cols[j]).FontSize(fsSmall).TextColor(p.muted).NoWrap()
					})
					ui.Row(c).Grow(1).MinWidth(0).Children(func() {
						if textInput(c, &t.Rows[i][j], "").Changed() {
							changed = true
						}
					})
				})
			}
		})
	}
	ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
		if primaryBtn(c, tr("+ 添加一行")).Clicked() {
			row := make([]string, n)
			if bound {
				row[0] = a.now().Format(boundTimeFormat)
			}
			t.Rows = append(t.Rows, row)
			a.tablePage = (len(t.Rows) - 1) / size
			changed = true
		}
		ui.Spacer(c)
		if pages > 1 {
			if btn(c, tr("上一页")).Clicked() && a.tablePage > 0 {
				a.tablePage--
			}
			ui.Text(c, fmt.Sprintf("%d / %d", a.tablePage+1, pages)).FontSize(fsSmall).TextColor(p.muted)
			if btn(c, tr("下一页")).Clicked() && a.tablePage < pages-1 {
				a.tablePage++
			}
		}
	})
	ui.Row(c).Gap(8).Children(func() { a.deleteButton(c) })
	if changed {
		a.invalidate(draftKey)
	}
}
