package main

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
)

func calendarScreen(now time.Time, sel *int, full bool) func(c *ui.Context) {
	ch := &engine.Chart{Type: "calendar"}
	return func(c *ui.Context) {
		applySkin(c)
		ui.Column(c).Gap(12).Padding(16).Children(func() {
			Chart(c, ChartProps{Data: ch, Height: 360, Full: full, Selected: sel, Now: now})
			if full {
				calendarDetail(c, sel, now)
			}
		})
	}
}

func TestCalendarShowsLunarAndHoliday(t *testing.T) {
	now := time.Date(2024, 2, 10, 9, 0, 0, 0, time.Local)
	sel := -1
	tt := ui.NewTester(calendarScreen(now, &sel, true), 720, 640)
	settle(tt)
	requireText(t, tt, "春节")
	requireText(t, tt, "甲辰年正月初一")
	requireText(t, tt, "2024年2月")
	requireStackedCells(t, tt, "10春节", "14情人")
	savePNG(t, tt, "21-cal-spring")

	if err := tt.Click("2024-02-14"); err != nil {
		t.Fatalf("click Valentine's: %v", err)
	}
	settle(tt)
	requireText(t, tt, "情人节")
	if sel != 20240214 {
		t.Errorf("sel %d, want 20240214", sel)
	}
	requireStackedCells(t, tt, "10春节", "14情人")
	savePNG(t, tt, "21-cal-valentine")
}

func TestCalendarMonthNav(t *testing.T) {
	now := time.Date(2024, 2, 10, 9, 0, 0, 0, time.Local)
	sel := -1
	tt := ui.NewTester(calendarScreen(now, &sel, true), 720, 640)
	settle(tt)
	click(t, tt, "下月")
	requireText(t, tt, "2024年3月")
	click(t, tt, "今天")
	requireText(t, tt, "2024年2月")
	requireText(t, tt, "春节")
}

func TestCalendarEnglishHasNoCJK(t *testing.T) {
	t.Cleanup(func() { setLang(langZH) })
	setLang(langEN)
	now := time.Date(2024, 2, 10, 9, 0, 0, 0, time.Local)
	sel := -1
	tt := ui.NewTester(calendarScreen(now, &sel, true), 720, 640)
	settle(tt)
	requireText(t, tt, "Spring Festival")
	requireText(t, tt, "Jia-Chen")
	requireText(t, tt, "February 2024")
	requireStackedCells(t, tt, "10Chun", "14Val", "9Chuxi")
	for _, s := range tt.Texts() {
		if cjk.MatchString(s) {
			t.Errorf("CJK on the English calendar: %q", s)
		}
	}
	savePNG(t, tt, "21-cal-en")
}

func requireStackedCells(t *testing.T, tt *ui.Tester, glued ...string) {
	t.Helper()
	for _, s := range tt.Texts() {
		for _, g := range glued {
			if s == g {
				t.Errorf("day number and caption on one line: %q", s)
			}
		}
	}
}

func TestCalendarCardIsNotClickable(t *testing.T) {
	now := time.Date(2024, 2, 10, 9, 0, 0, 0, time.Local)
	sel := -1
	tt := ui.NewTester(calendarScreen(now, &sel, false), 320, 220)
	settle(tt)
	if err := tt.Click("2024-02-14"); err == nil {
		t.Error("a card cell was clickable")
	}
	if sel != -1 {
		t.Errorf("sel %d after a card click", sel)
	}
}

func TestCalendarQingmingAndEaster(t *testing.T) {
	now := time.Date(2026, 4, 5, 9, 0, 0, 0, time.Local)
	sel := -1
	tt := ui.NewTester(calendarScreen(now, &sel, true), 720, 640)
	settle(tt)
	texts := strings.Join(tt.Texts(), " | ")
	if !strings.Contains(texts, "清明") || !strings.Contains(texts, "复活节") {
		t.Errorf("2026-04-05 missing 清明 or 复活节: %s", texts)
	}
}

func TestCalendarHomeCard(t *testing.T) {
	a := newTestApp(t)
	a.now = func() time.Time { return time.Date(2024, 2, 10, 9, 0, 0, 0, time.Local) }
	a.invalidateAll()
	tt := ui.NewTester(a.view, 1200, 900)
	settle(tt)
	requireText(t, tt, "春节")
	requireStackedCells(t, tt, "10春节", "14情人")
	savePNG(t, tt, "21-cal-home")

	a.router.Push("/full/cal")
	settle(tt)
	requireText(t, tt, "甲辰年正月初一")
	requireText(t, tt, "上月")
	requireText(t, tt, "编辑")
	for _, label := range []string{"样式", "大图", "刷新", "重置数据"} {
		if _, ok := tt.Access(label); ok {
			t.Errorf("the calendar full view still has %q", label)
		}
	}
	savePNG(t, tt, "21-cal-full")
}

func TestCalendarKeyboardMovesDay(t *testing.T) {
	now := time.Date(2024, 2, 10, 9, 0, 0, 0, time.Local)
	sel := -1
	tt := ui.NewTester(calendarScreen(now, &sel, true), 720, 640)
	settle(tt)
	tt.Key(0, ui.KeyRight)
	settle(tt)
	if sel != 20240211 {
		t.Errorf("sel %d after Right, want 20240211", sel)
	}
	tt.Key(0, ui.KeyHome)
	settle(tt)
	if sel != -1 {
		t.Errorf("sel %d after Home, want -1 (today)", sel)
	}
}

func TestCalendarEditorPreview(t *testing.T) {
	a := newTestApp(t)
	a.now = func() time.Time { return time.Date(2024, 2, 10, 9, 0, 0, 0, time.Local) }
	a.invalidateAll()
	tt := ui.NewTester(a.view, 1200, 900)
	a.router.Push("/edit/cal")
	settle(tt)
	requireText(t, tt, "上月")
	requireText(t, tt, "甲辰年正月初一")
	requireText(t, tt, "春节")
}
