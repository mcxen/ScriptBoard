package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"scriptboard/src/internal/caps"
	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
	"scriptboard/src/internal/table"
)

// A data-table plugin has no script: it is rows the user types in, pastes or
// imports from a spreadsheet, shown as a table, with a chart and a card made
// from its numbers. A table can also be a view of a recorded series (weight,
// gold), and then editing it edits what the cards of that metric show.

const (
	tablePageSize = 40
	// boundTimeFormat is how a bound table writes the time of a record.
	boundTimeFormat = "2006-01-02 15:04"
)

// tableColors are the lines of a table's chart.
var tableColors = []string{"#4FC3F7", "#FFB74D", "#81C784", "#BA68C8"}

// boundInfo is what a metric-bound table shows: its icon and the label of its
// value column.
func (a *app) boundColumns(metric string) []string {
	if metric == "weight" {
		return []string{tr("时间"), tr("体重") + "（" + a.store.Settings().WeightUnit + "）"}
	}
	return []string{tr("时间"), tr("金价") + "（" + tr("元/克") + "）"}
}

func boundIcon(metric string) string {
	if metric == "weight" {
		return "scale"
	}
	return "coins"
}

// tableMeta is the meta of a data table: it needs nothing and runs on any
// device.
func tableMeta(sc store.Script) engine.Meta {
	icon := "file-spreadsheet"
	if sc.Table != nil && sc.Table.Metric != "" {
		icon = boundIcon(sc.Table.Metric)
	}
	return engine.Meta{
		Name:        sc.Name,
		Description: tr("数据表：可以编辑、导入 Excel 或 CSV"),
		Version:     "1.0.0",
		Platforms:   []string{caps.Desktop, caps.Mobile},
		Needs:       []string{},
		Icon:        icon,
	}
}

// boundRows are the records of a metric as rows of a table: the time and the
// value in the unit the settings use.
func (a *app) boundRows(metric string) [][]string {
	pts := a.store.Series(metric)
	rows := make([][]string, len(pts))
	for i, pt := range pts {
		rows[i] = []string{
			time.UnixMilli(pt.T).Format(boundTimeFormat),
			table.FormatNumber(round2(a.shownValue(metric, pt.V))),
		}
	}
	return rows
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }

// tableData is the columns and rows a table shows. A bound table that has no
// rows of its own (the stored one) reads them from the records.
func (a *app) tableData(sc store.Script) ([]string, [][]string) {
	t := sc.Table
	if t == nil {
		return nil, nil
	}
	if t.Metric == "" {
		return t.Columns, t.Rows
	}
	if t.Rows != nil {
		return a.boundColumns(t.Metric), t.Rows
	}
	return a.boundColumns(t.Metric), a.boundRows(t.Metric)
}

// loadBound puts the records of a bound table's metric into the draft, to edit.
func (a *app) loadBound(sc *store.Script) {
	if sc.Table == nil || sc.Table.Metric == "" {
		return
	}
	sc.Table.Columns = a.boundColumns(sc.Table.Metric)
	sc.Table.Rows = a.boundRows(sc.Table.Metric)
}

// column says what a column holds.
type column struct {
	numeric bool // most of its cells are numbers
	dates   bool // most of its cells are dates
}

func kindOfColumn(rows [][]string, j int) column {
	var n, nums, times int
	for _, r := range rows {
		if j >= len(r) || strings.TrimSpace(r[j]) == "" {
			continue
		}
		n++
		if _, ok := table.Number(r[j]); ok {
			nums++
		}
		if _, ok := table.Time(r[j]); ok {
			times++
		}
	}
	if n == 0 {
		return column{}
	}
	return column{numeric: float64(nums) >= 0.6*float64(n), dates: float64(times) >= 0.6*float64(n)}
}

// tableResult makes the result of a data table: the card and the chart. The
// first column labels the points when it holds text or dates; every other
// column of numbers is a line.
func (a *app) tableResult(sc store.Script) engine.Result {
	cols, rows := a.tableData(sc)
	res := engine.Result{Title: tr(sc.Name), Icon: tableMeta(sc).Icon, Lines: []string{}}
	if len(rows) == 0 {
		res.Big, res.Sub = "—", tr("还没有数据")
		return res
	}
	width := len(cols)
	kinds := make([]column, width)
	for j := range kinds {
		kinds[j] = kindOfColumn(rows, j)
	}
	label := -1
	if width > 0 && (!kinds[0].numeric || kinds[0].dates) {
		label = 0
	}
	var vals []int
	for j := range kinds {
		if j != label && kinds[j].numeric && !kinds[j].dates && len(vals) < len(tableColors) {
			vals = append(vals, j)
		}
	}
	res.Sub = fmt.Sprintf(tr("%d 行 · %d 列"), len(rows), width)
	if len(vals) == 0 {
		res.Big = fmt.Sprintf("%d", len(rows))
		return res
	}

	// Dates in the first column put the rows in time order.
	ordered := rows
	if label == 0 && kinds[0].dates {
		ordered = append([][]string(nil), rows...)
		sort.SliceStable(ordered, func(i, k int) bool {
			ti, _ := table.Time(cell(ordered[i], 0))
			tk, _ := table.Time(cell(ordered[k], 0))
			return ti.Before(tk)
		})
	}
	const maxPoints = 240
	if len(ordered) > maxPoints {
		ordered = ordered[len(ordered)-maxPoints:]
	}
	ch := &engine.Chart{Type: "line"}
	prev := make([]float64, len(vals))
	for i, j := range vals {
		ch.Series = append(ch.Series, engine.Series{Name: cols[j], Color: tableColors[i%len(tableColors)]})
		for _, r := range ordered {
			if v, ok := table.Number(cell(r, j)); ok {
				prev[i] = v
				break
			}
		}
	}
	for n, r := range ordered {
		if label >= 0 {
			ch.Labels = append(ch.Labels, cell(r, label))
		} else {
			ch.Labels = append(ch.Labels, fmt.Sprintf("%d", n+1))
		}
		for i, j := range vals {
			if v, ok := table.Number(cell(r, j)); ok {
				prev[i] = v // a gap keeps the last value
			}
			ch.Series[i].Points = append(ch.Series[i].Points, prev[i])
		}
	}
	if len(ch.Labels) > 1 {
		ch.XLabels = []string{ch.Labels[0], ch.Labels[len(ch.Labels)-1]}
	}
	res.Chart = ch
	last := ch.Series[0].Points
	res.Big = table.FormatNumber(round2(last[len(last)-1]))
	if label >= 0 {
		res.Sub = ch.Labels[len(ch.Labels)-1] + " · " + res.Sub
	}
	return res
}

func cell(r []string, j int) string {
	if j < len(r) {
		return r[j]
	}
	return ""
}

// newTable starts a new data table in the editor. It is stored on Save.
func (a *app) newTable() {
	id := fmt.Sprintf("table-%d", a.now().UnixNano())
	a.editorID = id
	a.draft = store.Script{
		ID: id, Name: tr("新数据表"), Kind: store.KindTable, Origin: store.OriginUser, Home: true,
		Table: &store.Table{Columns: []string{tr("日期"), tr("数值")}, Rows: [][]string{{"", ""}}},
	}
	a.afterOpenTable()
	a.status = tr("未保存")
}

// afterOpenTable resets what belongs to the table that was open.
func (a *app) afterOpenTable() {
	a.tablePage = 0
	a.tableAppend = 0
	a.tableCols = false
	a.editTab = 0
	if a.draft.Table != nil && a.draft.Table.Metric != "" {
		a.tableAppend = 1 // what is imported joins the records
	}
	if a.panel == "paste" {
		a.panel = ""
	}
	delete(a.dims, draftKey)
	a.invalidate(draftKey)
}

// pointsOf reads a bound table's rows as records. Rows whose time or number
// cannot be read are left out, and counted.
func (a *app) pointsOf(t *store.Table) (pts []engine.Point, skipped int) {
	for _, r := range t.Rows {
		if strings.TrimSpace(cell(r, 0)) == "" && strings.TrimSpace(cell(r, 1)) == "" {
			continue
		}
		tm, ok1 := table.Time(cell(r, 0))
		v, ok2 := table.Number(cell(r, 1))
		if !ok1 || !ok2 {
			skipped++
			continue
		}
		if t.Metric == "weight" && a.store.Settings().WeightUnit == "斤" {
			v /= 2
		}
		pts = append(pts, engine.Point{T: tm.UnixMilli(), V: v})
	}
	sort.SliceStable(pts, func(i, k int) bool { return pts[i].T < pts[k].T })
	return pts, skipped
}

// saveTable stores the draft table. A bound table writes its rows to the
// records and keeps none itself.
func (a *app) saveTable() {
	t := a.draft.Table
	if t == nil {
		return
	}
	if t.Metric == "" {
		if err := a.store.SaveScript(a.draft); err != nil {
			a.status = tr("保存失败：") + err.Error()
			return
		}
		a.invalidate(a.draft.ID)
		a.invalidate(draftKey)
		a.status = tr("已保存")
		return
	}
	pts, skipped := a.pointsOf(t)
	if err := a.store.SetSeries(t.Metric, pts); err != nil {
		a.status = tr("保存失败：") + err.Error()
		return
	}
	stored := a.draft
	stored.Table = &store.Table{Metric: t.Metric}
	if err := a.store.SaveScript(stored); err != nil {
		a.status = tr("保存失败：") + err.Error()
		return
	}
	a.loadBound(&a.draft)
	a.invalidateMetric(t.Metric)
	a.invalidate(a.draft.ID)
	a.status = tr("已保存")
	if skipped > 0 {
		a.status = fmt.Sprintf(tr("已保存，%d 行的时间或数值读不出，已跳过"), skipped)
	}
}

// importRows puts imported rows into the draft table: replacing what is there,
// or joining it (a bound table joins by time, so that a row typed again takes
// the new value). It says what happened.
func (a *app) importRows(rows [][]string) error {
	t := a.draft.Table
	if t == nil {
		return fmt.Errorf("%s", tr("这不是数据表"))
	}
	cols, body := table.Header(table.Rect(rows))
	if len(body) == 0 {
		return fmt.Errorf("%s", tr("没有数据行"))
	}
	if len(body) > table.MaxRows {
		body = body[:table.MaxRows]
	}
	if t.Metric != "" {
		return a.importBound(t, body)
	}
	if a.tableAppend == 1 && len(t.Columns) > 0 {
		if len(cols) != len(t.Columns) {
			return fmt.Errorf(tr("列数不一样：表有 %d 列，文件有 %d 列"), len(t.Columns), len(cols))
		}
		t.Rows = append(t.Rows, body...)
	} else {
		t.Columns, t.Rows = cols, body
	}
	a.tablePage = 0
	a.invalidate(draftKey)
	a.status = fmt.Sprintf(tr("已导入 %d 行，点「保存」才会存下"), len(body))
	return nil
}

func (a *app) importBound(t *store.Table, body [][]string) error {
	have := map[string]int{}
	rows := t.Rows
	if a.tableAppend == 0 {
		rows = nil
	}
	for i, r := range rows {
		if tm, ok := table.Time(cell(r, 0)); ok {
			have[tm.Format(boundTimeFormat)] = i
		}
	}
	added, skipped := 0, 0
	for _, r := range body {
		tm, ok1 := table.Time(cell(r, 0))
		v, ok2 := table.Number(cell(r, 1))
		if !ok1 || !ok2 {
			skipped++
			continue
		}
		row := []string{tm.Format(boundTimeFormat), table.FormatNumber(v)}
		if i, ok := have[row[0]]; ok {
			rows[i] = row
		} else {
			have[row[0]] = len(rows)
			rows = append(rows, row)
		}
		added++
	}
	if added == 0 {
		return fmt.Errorf("%s", tr("没有读出时间和数值：第一列要是日期，第二列要是数字"))
	}
	sort.SliceStable(rows, func(i, k int) bool { return cell(rows[i], 0) < cell(rows[k], 0) })
	t.Rows = rows
	a.tablePage = 0
	a.invalidate(draftKey)
	a.status = fmt.Sprintf(tr("已导入 %d 行，点「保存」才会存下"), added)
	if skipped > 0 {
		a.status = fmt.Sprintf(tr("已导入 %d 行，%d 行读不出，已跳过。点「保存」才会存下"), added, skipped)
	}
	return nil
}

// importFile reads a spreadsheet or CSV file into the draft table.
func (a *app) importFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	rows, err := table.Parse(filepath.Base(path), data)
	if err != nil {
		return err
	}
	return a.importRows(rows)
}

// pickFile asks for a spreadsheet and imports it. The dialog blocks, so it
// runs off the main thread.
func (a *app) pickFile() {
	a.background(func() (string, error) {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Title:   tr("导入表格"),
			Filters: []mygo.FileFilter{{Name: tr("表格"), Extensions: []string{"xlsx", "csv", "tsv", "txt"}}},
		})
		if err != nil || len(paths) == 0 {
			return "", err
		}
		return paths[0], nil
	}, func(path string, err error) {
		switch {
		case err != nil:
			a.status = err.Error()
		case path != "":
			if err := a.importFile(path); err != nil {
				a.status = err.Error()
			}
		}
	})
}

// csvOf is the draft table as CSV, in the unit and order it is shown.
func (a *app) csvOf() string {
	cols, rows := a.tableData(a.draft)
	return table.WriteCSV(cols, rows)
}

// saveFile writes the draft table to a CSV file the user picks.
func (a *app) saveFile() {
	text := a.csvOf()
	name := strings.ReplaceAll(a.draft.Name, "/", "-") + ".csv"
	a.background(func() (string, error) {
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Title: tr("导出 CSV"), DefaultPath: name,
			Filters: []mygo.FileFilter{{Name: "CSV", Extensions: []string{"csv"}}},
		})
		if err != nil || path == "" {
			return "", err
		}
		// A byte-order mark makes Excel read the Chinese text as UTF-8.
		return path, os.WriteFile(path, []byte("\xef\xbb\xbf"+text), 0o644)
	}, func(path string, err error) {
		switch {
		case err != nil:
			a.status = err.Error()
		case path != "":
			a.status = tr("已导出到 ") + path
		}
	})
}

// tableForm is the editor of a data table: the name, the buttons that import
// and export, and the rows as a grid of inputs, a page at a time.
func (a *app) tableForm(c *ui.Context, compact bool, gridW float32) {
	p := pal(c)
	t := a.draft.Table
	if t == nil {
		return
	}
	bound := t.Metric != ""

	ui.Row(c).AlignItems(ui.Center).Gap(8).Wrap().Children(func() {
		ui.Column(c).Gap(2).Children(func() {
			ui.Text(c, tr("编辑数据表")).FontSize(fsEmphasis).Bold()
			if a.status != "" {
				ui.Text(c, a.status).FontSize(fsSmall).TextColor(p.muted)
			}
		})
		ui.Spacer(c)
		if a.canDelete(a.draft) {
			label := tr("删除")
			if a.confirmDelete == a.draft.ID {
				label = tr("确认删除")
			}
			if btn(c, label).Clicked() {
				a.deleteDraft()
			}
		}
		if primaryBtn(c, tr("保存")).Clicked() {
			a.saveTable()
		}
	})
	if !bound {
		field(c, tr("名称"), "", func() { textInput(c, &a.draft.Name, tr("数据表名称")) })
		ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
			sw(c, &a.draft.Home)
			ui.Text(c, tr("显示在首页")).FontSize(fsBody)
		})
	} else {
		hint(c, fmt.Sprintf(tr("这张表就是「%s」的记录：保存后，首页的卡片和图表都用这里的数据。"), tr(metricName(t.Metric))))
		ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
			sw(c, &a.draft.Home)
			ui.Text(c, tr("显示在首页")).FontSize(fsBody)
		})
	}
	a.specInfo(c, a.draft)

	a.importBar(c, t)
	a.pastePanel(c)

	a.tableGrid(c, t, bound, gridW)
}

// importBar is the buttons that import and export a table, and how an import
// joins the rows.
func (a *app) importBar(c *ui.Context, t *store.Table) {
	p := pal(c)
	desktop := caps.Platform() == caps.Desktop
	ui.Row(c).AlignItems(ui.Center).Gap(8).Wrap().Children(func() {
		if desktop {
			if btn(c, tr("导入 Excel / CSV")).Clicked() {
				a.pickFile()
			}
		}
		if btn(c, tr("粘贴导入")).Clicked() {
			if a.panel == "paste" {
				a.panel = ""
			} else {
				a.panel, a.transferText, a.transferErr = "paste", "", ""
			}
		}
		if desktop {
			if btn(c, tr("导出 CSV")).Clicked() {
				a.saveFile()
			}
		}
		if btn(c, tr("复制 CSV")).Clicked() {
			copyText(a.csvOf())
			a.status = tr("已复制")
		}
		ui.Spacer(c)
		if len(t.Columns) > 0 {
			ui.Text(c, tr("导入时")).FontSize(fsSmall).TextColor(p.muted)
			segmented(c, &a.tableAppend, tr("替换"), tr("追加"))
		}
	})
}

// pastePanel is the box that imports pasted cells, when it is open.
func (a *app) pastePanel(c *ui.Context) {
	p := pal(c)
	if a.panel == "paste" {
		ui.Column(c).Padding(14).Gap(10).Radius(radControl + 2).Background(p.chip).Children(func() {
			hint(c, tr("从 Excel 复制一块单元格，或粘贴 CSV；第一行是表头"))
			textArea(c, &a.transferText, "", 6).Font("monospace").FontSize(fsSmall)
			if a.transferErr != "" {
				ui.Text(c, a.transferErr).FontSize(fsSmall).TextColor(p.up)
			}
			ui.Row(c).Gap(8).Children(func() {
				if primaryBtn(c, tr("导入")).Clicked() {
					rows, err := table.ParseText(a.transferText)
					if err == nil {
						err = a.importRows(rows)
					}
					if err != nil {
						a.transferErr = err.Error()
					} else {
						a.transferErr, a.transferText, a.panel = "", "", ""
					}
				}
				if btn(c, tr("关闭")).Clicked() {
					a.panel, a.transferErr = "", ""
				}
			})
		})
	}
}

func metricName(metric string) string {
	if metric == "weight" {
		return "体重"
	}
	return "金价"
}

// tableGrid draws the rows as inputs, a page at a time.
func (a *app) tableGrid(c *ui.Context, t *store.Table, bound bool, gridW float32) {
	p := pal(c)
	cols := t.Columns
	if bound {
		cols = a.boundColumns(t.Metric)
	}
	n := max(len(cols), 1)
	colW := clampF((gridW-92)/float32(n), 96, 240)
	size := a.pageSize()
	pages := max(1, (len(t.Rows)+size-1)/size)
	a.tablePage = min(max(a.tablePage, 0), pages-1)
	changed := false

	ui.Column(c).Gap(4).Children(func() {
		// Column names.
		ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
			ui.Text(c, "#").FontSize(fsTiny).TextColor(p.muted).Width(34)
			for j := range cols {
				if bound {
					ui.Text(c, cols[j]).FontSize(fsSmall).Bold().Width(colW)
					continue
				}
				if textInput(c, &t.Columns[j], tr("列名")).Width(colW).Changed() {
					changed = true
				}
			}
			ui.Spacer(c).Width(28)
		})
		from := a.tablePage * size
		to := min(len(t.Rows), from+size)
		for i := from; i < to; i++ {
			ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
				ui.Text(c, fmt.Sprintf("%d", i+1)).FontSize(fsTiny).TextColor(p.muted).Width(34)
				for len(t.Rows[i]) < n {
					t.Rows[i] = append(t.Rows[i], "")
				}
				for j := 0; j < n; j++ {
					if textInput(c, &t.Rows[i][j], "").Width(colW).Changed() {
						changed = true
					}
				}
				if action.ActionIcon(c, icon("x"), action.ActionIconProps{Size: theme.Small, Title: tr("删除这一行")}).Clicked() {
					t.Rows = append(t.Rows[:i], t.Rows[i+1:]...)
					changed = true
				}
			})
		}
		if len(t.Rows) == 0 {
			hint(c, tr("还没有数据：点「添加一行」，或导入 Excel / CSV"))
		}
	})
	ui.Row(c).AlignItems(ui.Center).Gap(8).Wrap().Children(func() {
		if btn(c, tr("+ 添加一行")).Clicked() {
			row := make([]string, n)
			if bound {
				row[0] = a.now().Format(boundTimeFormat)
			}
			t.Rows = append(t.Rows, row)
			a.tablePage = (len(t.Rows) - 1) / size
			changed = true
		}
		if !bound {
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
		}
		if len(t.Rows) > 1 && kindOfColumn(t.Rows, 0).dates && btn(c, tr("按时间排序")).Clicked() {
			sort.SliceStable(t.Rows, func(i, k int) bool {
				ti, _ := table.Time(cell(t.Rows[i], 0))
				tk, _ := table.Time(cell(t.Rows[k], 0))
				return ti.Before(tk)
			})
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
		ui.Text(c, fmt.Sprintf(tr("共 %d 行"), len(t.Rows))).FontSize(fsSmall).TextColor(p.muted)
	})
	if changed {
		a.invalidate(draftKey)
	}
}

// builtinTables are the data tables that ship with the app: the records of the
// two metrics the home cards read, as tables to edit and import into.
var builtinTables = []struct{ id, name, metric string }{
	{"weight-data", "体重数据", "weight"},
	{"gold-data", "金价数据", "gold"},
}

// ensureBuiltinTables adds the built-in tables a data file does not have yet.
// They are not on the home page: the cards of their metrics already are.
func (a *app) ensureBuiltinTables() error {
	for _, b := range builtinTables {
		if _, ok := a.store.Script(b.id); ok {
			continue
		}
		err := a.store.SaveScript(store.Script{
			ID: b.id, Name: b.name, Kind: store.KindTable, Origin: store.OriginBuiltin,
			Table: &store.Table{Metric: b.metric},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// pageSize is how many rows of a table show at once: fewer on a phone, where
// each row is a card.
func (a *app) pageSize() int {
	if a.compact {
		return 8
	}
	return tablePageSize
}
