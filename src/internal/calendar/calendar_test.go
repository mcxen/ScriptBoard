package calendar

import (
	"testing"
	"time"
)

func civil(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 12, 0, 0, 0, time.UTC)
}

func TestSpringFestival(t *testing.T) {
	cases := []struct {
		solar               time.Time
		year, month, day    int
		gz, zodiac, holiday string
	}{
		{civil(2024, 2, 10), 2024, 1, 1, "甲辰", "龙", "spring-festival"},
		{civil(2025, 1, 29), 2025, 1, 1, "乙巳", "蛇", "spring-festival"},
		{civil(2026, 2, 17), 2026, 1, 1, "丙午", "马", "spring-festival"},
	}
	for _, c := range cases {
		got := Lookup(c.solar)
		if !got.Valid {
			t.Fatalf("%s: not valid", c.solar.Format("2006-01-02"))
		}
		if got.LunarYear != c.year || got.LunarMonth != c.month || got.LunarDay != c.day || got.Leap {
			t.Errorf("%s: lunar %d-%d-%d leap=%v, want %d-%d-%d", c.solar.Format("2006-01-02"),
				got.LunarYear, got.LunarMonth, got.LunarDay, got.Leap, c.year, c.month, c.day)
		}
		if gz := got.YearGZ("zh"); gz != c.gz {
			t.Errorf("%s: ganzhi %q, want %q", c.solar.Format("2006-01-02"), gz, c.gz)
		}
		if z := got.Zodiac("zh"); z != c.zodiac {
			t.Errorf("%s: zodiac %q, want %q", c.solar.Format("2006-01-02"), z, c.zodiac)
		}
		if !got.HasHoliday(c.holiday) {
			t.Errorf("%s: missing %s, have %+v", c.solar.Format("2006-01-02"), c.holiday, got.Holidays)
		}
	}
}

func TestLunarYearFollowsSpringFestival(t *testing.T) {
	eve := Lookup(civil(2026, 2, 16))
	if !eve.Valid || eve.YearGZ("zh") != "乙巳" || eve.Zodiac("zh") != "蛇" {
		t.Errorf("2026-02-16 ganzhi %s %s, want 乙巳 蛇", eve.YearGZ("zh"), eve.Zodiac("zh"))
	}
	if !eve.HasHoliday("lunar-eve") {
		t.Errorf("2026-02-16 missing 除夕: %+v", eve.Holidays)
	}
	day := Lookup(civil(2026, 2, 17))
	if day.YearGZ("zh") != "丙午" || day.Zodiac("zh") != "马" {
		t.Errorf("2026-02-17 ganzhi %s %s, want 丙午 马", day.YearGZ("zh"), day.Zodiac("zh"))
	}
}

func TestMidAutumnAndDragonBoat(t *testing.T) {
	mid := Lookup(civil(2024, 9, 17))
	if !mid.Valid || mid.LunarMonth != 8 || mid.LunarDay != 15 || !mid.HasHoliday("mid-autumn") {
		t.Errorf("2024-09-17 lunar %d-%d holidays %+v", mid.LunarMonth, mid.LunarDay, mid.Holidays)
	}
	duan := Lookup(civil(2024, 6, 10))
	if duan.LunarMonth != 5 || duan.LunarDay != 5 || !duan.HasHoliday("dragon-boat") {
		t.Errorf("2024-06-10 lunar %d-%d holidays %+v", duan.LunarMonth, duan.LunarDay, duan.Holidays)
	}
}

func TestQingming(t *testing.T) {
	cases := []time.Time{
		civil(2024, 4, 4),
		civil(2025, 4, 4),
		civil(2026, 4, 5),
		civil(1998, 4, 5),
	}
	for _, d := range cases {
		got := Lookup(d)
		if !got.HasHoliday("qingming") {
			t.Errorf("%s missing 清明: %+v", d.Format("2006-01-02"), got.Holidays)
		}
	}
	if Lookup(civil(2024, 4, 5)).HasHoliday("qingming") {
		t.Error("2024-04-05 is not 清明")
	}
}

func TestEasterAndThanksgiving(t *testing.T) {
	if !Lookup(civil(2024, 3, 31)).HasHoliday("easter") {
		t.Error("2024-03-31 is Easter")
	}
	if !Lookup(civil(2025, 4, 20)).HasHoliday("easter") {
		t.Error("2025-04-20 is Easter")
	}
	if !Lookup(civil(2026, 4, 5)).HasHoliday("easter") {
		t.Error("2026-04-05 is Easter")
	}
	if !Lookup(civil(2024, 11, 28)).HasHoliday("thanksgiving") {
		t.Error("2024-11-28 is Thanksgiving")
	}
	if !Lookup(civil(2025, 11, 27)).HasHoliday("thanksgiving") {
		t.Error("2025-11-27 is Thanksgiving")
	}
}

func TestQingmingAndEasterShare2026(t *testing.T) {
	d := Lookup(civil(2026, 4, 5))
	if !d.HasHoliday("qingming") || !d.HasHoliday("easter") {
		t.Errorf("2026-04-05 want 清明 and Easter, have %+v", d.Holidays)
	}
}

func TestEnglishHasNoCJK(t *testing.T) {
	d := Lookup(civil(2024, 2, 10))
	for _, s := range []string{
		d.YearGZ("en"), d.Zodiac("en"), d.LunarMonthName("en"), d.LunarDayName("en"),
		d.LunarLine("en"), d.Headline("en"), d.SolarLine("en"), d.WeekdayName("en"),
		d.CellCaption("en"), EmptyHoliday("en"),
	} {
		for _, r := range s {
			if r >= 0x4e00 && r <= 0x9fff {
				t.Errorf("CJK in English %q", s)
			}
		}
	}
	if d.Headline("en") == "" || d.Zodiac("en") != "Dragon" || d.YearGZ("en") != "Jia-Chen" {
		t.Errorf("english headline %q zodiac %q gz %q", d.Headline("en"), d.Zodiac("en"), d.YearGZ("en"))
	}
}

func TestChristmas(t *testing.T) {
	d := Lookup(civil(2026, 12, 25))
	if !d.HasHoliday("christmas") {
		t.Errorf("Christmas missing: %+v", d.Holidays)
	}
}

func TestSolarTerms(t *testing.T) {
	cases := []struct {
		t  time.Time
		id string
	}{
		{civil(2024, 2, 4), "lichun"},
		{civil(2024, 4, 4), "qingming"},
		{civil(2024, 6, 21), "xiazhi"},
		{civil(2024, 12, 21), "dongzhi"},
		{civil(2025, 2, 3), "lichun"},
		{civil(2026, 4, 5), "qingming"},
		{civil(1998, 4, 5), "qingming"},
	}
	for _, c := range cases {
		got := Lookup(c.t)
		if !got.HasHoliday(c.id) {
			t.Errorf("%s missing %s: %+v", c.t.Format("2006-01-02"), c.id, got.Holidays)
		}
	}
	if Lookup(civil(2024, 2, 5)).HasHoliday("lichun") {
		t.Error("2024-02-05 is not 立春")
	}
}

func TestMothersAndFathersDay(t *testing.T) {
	if !Lookup(civil(2024, 5, 12)).HasHoliday("mothers-day") {
		t.Error("2024-05-12 is Mother's Day")
	}
	if !Lookup(civil(2024, 6, 16)).HasHoliday("fathers-day") {
		t.Error("2024-06-16 is Father's Day")
	}
}
