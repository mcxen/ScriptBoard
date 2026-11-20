package engine

import (
	"strconv"

	"scriptboard/src/internal/calendar"
)

func finishCalendar(res *Result, in Input) {
	if res == nil || res.Chart == nil || res.Chart.Type != "calendar" {
		return
	}
	lang := in.Lang
	if lang != "en" {
		lang = "zh"
	}
	day := calendar.Lookup(in.Now)
	if res.Title == "" {
		if lang == "en" {
			res.Title = "Calendar"
		} else {
			res.Title = "日历"
		}
	}
	res.Big = strconv.Itoa(day.Time.Day())
	res.Sub = day.Headline(lang)
	res.Stats = calendarStats(day, lang)
	if res.Layout == nil {
		res.Layout = &Layout{Legend: "hide"}
	}
}

func calendarStats(day calendar.Day, lang string) []Stat {
	holiday := "—"
	holidaySub := ""
	if len(day.Holidays) > 0 {
		holiday = day.Holidays[0].Name(lang)
		if len(day.Holidays) > 1 {
			holidaySub = day.Holidays[1].Name(lang)
		}
	}
	lunarLabel, zodiacLabel, holidayLabel := "农历", "生肖", "节日"
	if lang == "en" {
		lunarLabel, zodiacLabel, holidayLabel = "Lunar", "Zodiac", "Holiday"
	}
	lunar := day.LunarDate(lang)
	if lunar == "" {
		lunar = "—"
	}
	zodiac := day.Zodiac(lang)
	if zodiac == "" {
		zodiac = "—"
	}
	return []Stat{
		{Label: lunarLabel, Value: lunar, Sub: day.YearGZ(lang)},
		{Label: zodiacLabel, Value: zodiac},
		{Label: holidayLabel, Value: holiday, Sub: holidaySub},
	}
}
