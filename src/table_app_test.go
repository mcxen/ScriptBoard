package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/store"
)

func saveTable(t *testing.T, a *app, id string, cols []string, rows [][]string) store.Script {
	t.Helper()
	sc := store.Script{ID: id, Name: "我的表", Kind: store.KindTable, Origin: store.OriginUser, Home: true,
		Table: &store.Table{Columns: cols, Rows: rows}}
	if err := a.store.SaveScript(sc); err != nil {
		t.Fatal(err)
	}
	return sc
}

// A table makes a card and a chart from its numbers: the first column labels
// the points, dates put the rows in order, every column of numbers is a line.
func TestTableMakesAResult(t *testing.T) {
	a := newTestApp(t)
	sc := saveTable(t, a, "t1", []string{"日期", "收入", "支出", "备注"}, [][]string{
		{"2026-11-03", "30", "5", "c"},
		{"2026-11-01", "10", "2", "a"},
		{"2026-11-02", "20", "", "b"}, // a gap keeps the last value
	})
	res, err := a.execute(sc, nil)
	if err != nil {
		t.Fatal(err)
	}
	ch := res.Chart
	if ch == nil || len(ch.Series) != 2 || ch.Series[0].Name != "收入" || ch.Series[1].Name != "支出" {
		t.Fatalf("chart %+v (the note column is text, not a line)", ch)
	}
	if got := ch.Series[0].Points; got[0] != 10 || got[1] != 20 || got[2] != 30 {
		t.Errorf("rows not in date order: %v", got)
	}
	if got := ch.Series[1].Points; got[0] != 2 || got[1] != 2 || got[2] != 5 {
		t.Errorf("a gap should keep the last value: %v", got)
	}
	if res.Big != "30" || !strings.Contains(res.Sub, "2026-11-03") || !strings.Contains(res.Sub, "3 行") {
		t.Errorf("big %q sub %q", res.Big, res.Sub)
	}

	// Text only: no chart, the number of rows.
	text := saveTable(t, a, "t2", []string{"名", "说明"}, [][]string{{"a", "x"}, {"b", "y"}})
	if res, _ := a.execute(text, nil); res.Chart != nil || res.Big != "2" {
		t.Errorf("text table: chart %v big %q", res.Chart, res.Big)
	}
	empty := saveTable(t, a, "t3", []string{"日期", "值"}, nil)
	if res, _ := a.execute(empty, nil); res.Big != "—" {
		t.Errorf("empty table: %q", res.Big)
	}
	// A table runs anywhere and needs nothing.
	if !a.supported(sc) || len(a.needsOf(sc)) != 0 {
		t.Errorf("a table should run everywhere and need nothing")
	}
}

// Importing replaces the rows or joins them; the header comes from the file.
func TestImportIntoATable(t *testing.T) {
	a := newTestApp(t)
	saveTable(t, a, "t1", []string{"日期", "值"}, [][]string{{"2026-11-01", "1"}})
	a.openEditor("t1")

	dir := t.TempDir()
	file := filepath.Join(dir, "x.csv")
	os.WriteFile(file, []byte("day,amount\n2026-11-02,2\n2026-11-03,3\n"), 0o644)
	if err := a.importFile(file); err != nil {
		t.Fatal(err)
	}
	tb := a.draft.Table
	if len(tb.Rows) != 2 || tb.Columns[0] != "day" || tb.Rows[1][1] != "3" {
		t.Fatalf("replace: %+v", tb)
	}
	if st, _ := a.store.Script("t1"); len(st.Table.Rows) != 1 {
		t.Error("an import reached the store before Save")
	}

	a.tableAppend = 1
	if err := a.importFile(file); err != nil || len(a.draft.Table.Rows) != 4 {
		t.Fatalf("append: %v %+v", err, a.draft.Table)
	}
	os.WriteFile(file, []byte("a,b,c\n1,2,3\n"), 0o644)
	if err := a.importFile(file); err == nil || !strings.Contains(err.Error(), "列数") {
		t.Errorf("a file with other columns joined the table: %v", err)
	}
	if err := a.importFile(filepath.Join(dir, "missing.csv")); err == nil {
		t.Error("a missing file was imported")
	}
	a.saveTable()
	if st, _ := a.store.Script("t1"); len(st.Table.Rows) != 4 {
		t.Errorf("saved %d rows, want 4", len(st.Table.Rows))
	}
}

// The weight table is the records of the weight: editing it edits them, in the
// unit of the settings, and what is typed wrong is skipped.
func TestBoundTableEditsTheRecords(t *testing.T) {
	a := newTestApp(t)
	sc, ok := a.store.Script("weight-data")
	if !ok || !sc.IsTable() || sc.Table.Metric != "weight" || sc.Home {
		t.Fatalf("builtin weight table: %+v %v", sc, ok)
	}
	before := len(a.store.Series("weight"))
	a.openEditor("weight-data")
	if len(a.draft.Table.Rows) != before || !strings.Contains(a.draft.Table.Columns[1], "kg") {
		t.Fatalf("rows %d, want %d; columns %v", len(a.draft.Table.Rows), before, a.draft.Table.Columns)
	}

	a.draft.Table.Rows = append(a.draft.Table.Rows,
		[]string{"2030-01-01 08:00", "68.5"},
		[]string{"not a time", "1"},
		[]string{"2030-01-02 08:00", "x"},
	)
	a.saveTable()
	pts := a.store.Series("weight")
	if len(pts) != before+1 || pts[len(pts)-1].V != 68.5 {
		t.Fatalf("records %d (want %d), last %+v", len(pts), before+1, pts[len(pts)-1])
	}
	if !strings.Contains(a.status, "2 行") {
		t.Errorf("status %q does not say two rows were skipped", a.status)
	}
	if st, _ := a.store.Script("weight-data"); st.Table.Rows != nil {
		t.Error("a bound table keeps rows of its own")
	}

	// In 斤 the table shows and takes 斤, and the records stay in kg.
	set := a.store.Settings()
	set.WeightUnit = "斤"
	a.store.SetSettings(set)
	a.editorID = ""
	a.openEditor("weight-data")
	last := a.draft.Table.Rows[len(a.draft.Table.Rows)-1]
	if last[1] != "137" || !strings.Contains(a.draft.Table.Columns[1], "斤") {
		t.Errorf("in 斤: %v %v", last, a.draft.Table.Columns)
	}
	last[1] = "140"
	a.saveTable()
	if got := a.store.Series("weight"); got[len(got)-1].V != 70 {
		t.Errorf("140 斤 stored as %v kg, want 70", got[len(got)-1].V)
	}

	// A joined import takes the new value for a time that is there.
	a.tableAppend = 1
	if err := a.importRows([][]string{{"时间", "体重"}, {"2030-01-01 08:00", "66"}, {"2030-02-01 08:00", "65"}}); err != nil {
		t.Fatal(err)
	}
	rows := a.draft.Table.Rows
	if len(rows) != before+2 || rows[len(rows)-1][0] != "2030-02-01 08:00" {
		t.Errorf("joined rows %d: %v", len(rows), rows[len(rows)-2:])
	}
	if err := a.importRows([][]string{{"a", "b"}, {"x", "y"}}); err == nil {
		t.Error("rows without times and numbers were taken")
	}
}

// A script is edited as source and a table as rows: the editor shows one or
// the other, never both.
func TestEditorSplitsScriptsFromTables(t *testing.T) {
	a := newTestApp(t)
	saveTable(t, a, "t1", []string{"日期", "值"}, [][]string{{"2026-11-01", "1"}, {"2026-11-02", "2"}})
	tt := ui.NewTester(a.view, 1500, 900)
	a.router.Push("/edit/t1")
	settle(tt)
	requireText(t, tt, "编辑数据表")
	requireText(t, tt, "导入 Excel / CSV")
	requireText(t, tt, "+ 添加一行")
	for _, no := range []string{"源码", "录入数据", "运行"} {
		if _, ok := tt.Access(no); ok {
			t.Errorf("the table editor shows %q", no)
		}
	}
	savePNG(t, tt, "9-table-editor")

	a.router.Push("/edit/weight")
	settle(tt)
	requireText(t, tt, "源码")
	for _, no := range []string{"编辑数据表", "导入 Excel / CSV", "+ 添加一行"} {
		if _, ok := tt.Access(no); ok {
			t.Errorf("the script editor shows %q", no)
		}
	}
}

// A table on the home page is a card like any other.
func TestTableCardOnHome(t *testing.T) {
	a := newTestApp(t)
	saveTable(t, a, "t1", []string{"日期", "值"}, [][]string{{"2026-11-01", "41"}, {"2026-11-02", "42.5"}})
	tt := ui.NewTester(a.view, 1280, 900)
	settle(tt)
	requireText(t, tt, "我的表")
	requireText(t, tt, "42.5")
}

// Copying a table makes one the user owns, with rows of its own.
func TestDuplicateATable(t *testing.T) {
	a := newTestApp(t)
	saveTable(t, a, "t1", []string{"日期", "值"}, [][]string{{"2026-11-01", "1"}})
	a.openEditor("t1")
	a.duplicateDraft()
	if a.draft.ID == "t1" || !a.draft.IsTable() || len(a.draft.Table.Rows) != 1 {
		t.Fatalf("copy %+v", a.draft)
	}
	a.draft.Table.Rows[0][1] = "99"
	if orig, _ := a.store.Script("t1"); orig.Table.Rows[0][1] != "1" {
		t.Error("editing the copy changed the original")
	}
}
