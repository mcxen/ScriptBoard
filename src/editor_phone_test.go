package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func absent(t *testing.T, tt *ui.Tester, texts ...string) {
	t.Helper()
	for _, s := range texts {
		if _, ok := tt.Access(s); ok {
			t.Errorf("%q is on the screen", s)
		}
	}
}

// On a phone the editor is a list, and the thing you pick from it: a script as
// source with its preview a tab away, never the list and the form together.
func TestPhoneEditorIsListThenThing(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 390, 844)
	settle(tt)
	click(t, tt, "编辑")
	requireText(t, tt, "体重记录")
	requireText(t, tt, "汇率")
	requireText(t, tt, "七日天气")
	requireText(t, tt, "+ 数据表")
	absent(t, tt, "源码", "保存")
	savePNG(t, tt, "p0-list")

	click(t, tt, "体重记录")
	if a.router.Path() != "/edit/weight" {
		t.Fatalf("path %q", a.router.Path())
	}
	requireText(t, tt, "源码")
	requireText(t, tt, "保存")
	absent(t, tt, "汇率", "录入数据") // no list, no data-entry panel
	savePNG(t, tt, "p1-script")

	click(t, tt, "预览")
	requireText(t, tt, "体重 · kg")
	savePNG(t, tt, "p1b-preview")

	// Back to the list.
	a.router.Back()
	settle(tt)
	requireText(t, tt, "汇率")
}

// A data table on a phone: its rows are cards of fields, the columns are edited
// on request, and Save keeps what was typed.
func TestPhoneTableEditor(t *testing.T) {
	a := newTestApp(t)
	saveTable(t, a, "t1", []string{"日期", "收入", "支出"}, [][]string{{"2026-11-01", "10", "2"}, {"2026-11-02", "20", "3"}})
	tt := ui.NewTester(a.view, 390, 844)
	settle(tt)
	click(t, tt, "编辑")
	a.router.Push("/edit/t1") // the row is below the fold of a long list
	settle(tt)
	requireText(t, tt, "表格")
	requireText(t, tt, "第 1 行")
	requireText(t, tt, "第 2 行")
	requireText(t, tt, "+ 添加一行")
	absent(t, tt, "源码", "运行")
	savePNG(t, tt, "p2-table")

	click(t, tt, "编辑列")
	requireText(t, tt, "列 3")
	savePNG(t, tt, "p2b-cols")
	click(t, tt, "完成")

	click(t, tt, "+ 添加一行")
	if rows := len(a.draft.Table.Rows); rows != 3 {
		t.Fatalf("%d rows after adding one", rows)
	}
	requireText(t, tt, "第 3 行")
	click(t, tt, "保存")
	if st, _ := a.store.Script("t1"); len(st.Table.Rows) != 3 {
		t.Errorf("saved %d rows", len(st.Table.Rows))
	}
}

// New things start on the phone's second screen, and deleting one goes back to
// the list.
func TestPhoneNewAndDelete(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 390, 844)
	settle(tt)
	click(t, tt, "编辑")
	click(t, tt, "+ 数据表")
	requireText(t, tt, "表格")
	requireText(t, tt, "第 1 行")
	click(t, tt, "保存")
	id := a.editorID
	if _, ok := a.store.Script(id); !ok {
		t.Fatal("the new table was not saved")
	}
	click(t, tt, "删除")
	click(t, tt, "确认删除")
	if _, ok := a.store.Script(id); ok {
		t.Error("the table is still there")
	}
	if a.router.Path() != "/edit" {
		t.Errorf("after deleting, the path is %q, want the list", a.router.Path())
	}
	requireText(t, tt, "体重记录")
}
