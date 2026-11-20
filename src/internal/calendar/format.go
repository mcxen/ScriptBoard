package calendar

import (
	"fmt"
	"strconv"
	"time"
)

var (
	ganZH = []string{"甲", "乙", "丙", "丁", "戊", "己", "庚", "辛", "壬", "癸"}
	zhiZH = []string{"子", "丑", "寅", "卯", "辰", "巳", "午", "未", "申", "酉", "戌", "亥"}
	ganEN = []string{"Jia", "Yi", "Bing", "Ding", "Wu", "Ji", "Geng", "Xin", "Ren", "Gui"}
	zhiEN = []string{"Zi", "Chou", "Yin", "Mao", "Chen", "Si", "Wu", "Wei", "Shen", "You", "Xu", "Hai"}

	zodiacZH = []string{"鼠", "牛", "虎", "兔", "龙", "蛇", "马", "羊", "猴", "鸡", "狗", "猪"}
	zodiacEN = []string{"Rat", "Ox", "Tiger", "Rabbit", "Dragon", "Snake", "Horse", "Goat", "Monkey", "Rooster", "Dog", "Pig"}

	monthZH = []string{"正", "二", "三", "四", "五", "六", "七", "八", "九", "十", "冬", "腊"}
	nStr1   = []string{"日", "一", "二", "三", "四", "五", "六", "七", "八", "九", "十"}
	nStr2   = []string{"初", "十", "廿", "卅"}

	weekZH = []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
)

func english(lang string) bool { return lang == "en" }

// YearGZ is the stem-branch of the lunar year: 甲辰 / Jia-Chen.
func (d Day) YearGZ(lang string) string {
	if !d.Valid {
		return ""
	}
	ganKey := (d.LunarYear - 3) % 10
	zhiKey := (d.LunarYear - 3) % 12
	if ganKey == 0 {
		ganKey = 10
	}
	if zhiKey == 0 {
		zhiKey = 12
	}
	if english(lang) {
		return ganEN[ganKey-1] + "-" + zhiEN[zhiKey-1]
	}
	return ganZH[ganKey-1] + zhiZH[zhiKey-1]
}

// Zodiac is the animal of the lunar year: 龙 / Dragon.
func (d Day) Zodiac(lang string) string {
	if !d.Valid {
		return ""
	}
	i := (d.LunarYear - 4) % 12
	if i < 0 {
		i += 12
	}
	if english(lang) {
		return zodiacEN[i]
	}
	return zodiacZH[i]
}

// LunarMonthName is 正月 / 1st month, with 闰 / leap when this day is in a leap month.
func (d Day) LunarMonthName(lang string) string {
	if !d.Valid || d.LunarMonth < 1 || d.LunarMonth > 12 {
		return ""
	}
	if english(lang) {
		name := ordinal(d.LunarMonth) + " month"
		if d.Leap {
			return "leap " + name
		}
		return name
	}
	name := monthZH[d.LunarMonth-1] + "月"
	if d.Leap {
		return "闰" + name
	}
	return name
}

// LunarDayName is 初一 / 1st.
func (d Day) LunarDayName(lang string) string {
	if !d.Valid {
		return ""
	}
	if english(lang) {
		return ordinal(d.LunarDay)
	}
	return chinaDay(d.LunarDay)
}

func chinaDay(d int) string {
	switch d {
	case 10:
		return "初十"
	case 20:
		return "二十"
	case 30:
		return "三十"
	default:
		return nStr2[d/10] + nStr1[d%10]
	}
}

func ordinal(n int) string {
	v := strconv.Itoa(n)
	if n%100 >= 11 && n%100 <= 13 {
		return v + "th"
	}
	switch n % 10 {
	case 1:
		return v + "st"
	case 2:
		return v + "nd"
	case 3:
		return v + "rd"
	default:
		return v + "th"
	}
}

// LunarDate is 正月初一 / 1st month 1st.
func (d Day) LunarDate(lang string) string {
	if !d.Valid {
		return ""
	}
	if english(lang) {
		return d.LunarMonthName(lang) + " " + d.LunarDayName(lang)
	}
	return d.LunarMonthName(lang) + d.LunarDayName(lang)
}

// LunarLine is 甲辰年正月初一 / Jia-Chen, 1st month 1st.
func (d Day) LunarLine(lang string) string {
	if !d.Valid {
		return ""
	}
	if english(lang) {
		return d.YearGZ(lang) + ", " + d.LunarDate(lang)
	}
	return d.YearGZ(lang) + "年" + d.LunarDate(lang)
}

// WeekdayName is 周六 / Sat.
func (d Day) WeekdayName(lang string) string {
	w := d.Time.Weekday()
	if english(lang) {
		s := w.String()
		if len(s) >= 3 {
			return s[:3]
		}
		return s
	}
	return weekZH[int(w)]
}

// SolarLine is 2024年2月10日 / 10 February 2024.
func (d Day) SolarLine(lang string) string {
	y, m, day := d.Time.Date()
	if english(lang) {
		return fmt.Sprintf("%d %s %d", day, m.String(), y)
	}
	return fmt.Sprintf("%d年%d月%d日", y, m, day)
}

// MonthTitle is 2024年2月 / February 2024.
func MonthTitle(t time.Time, lang string) string {
	if english(lang) {
		return t.Month().String() + " " + strconv.Itoa(t.Year())
	}
	return fmt.Sprintf("%d年%d月", t.Year(), t.Month())
}

// Headline is the card's sub line: a holiday when there is one, otherwise the
// lunar date and the weekday.
func (d Day) Headline(lang string) string {
	lunar := d.LunarDate(lang)
	if len(d.Holidays) > 0 {
		name := d.Holidays[0].Name(lang)
		if lunar != "" {
			return name + " · " + lunar
		}
		return name
	}
	if lunar == "" {
		return d.WeekdayName(lang)
	}
	if english(lang) {
		return "Lunar " + lunar + " · " + d.WeekdayName(lang)
	}
	return "农历" + lunar + " · " + d.WeekdayName(lang)
}

// CellCaption is the second line of a month cell: a holiday's short name, or
// the lunar day (the month's name on the first day). Empty when English and
// there is no holiday, so a compact grid stays a grid of numbers.
func (d Day) CellCaption(lang string) string {
	for _, h := range d.Holidays {
		if english(lang) && h.Term {
			continue
		}
		return h.Short(lang)
	}
	if !d.Valid {
		return ""
	}
	if english(lang) {
		return ""
	}
	if d.LunarDay == 1 {
		return d.LunarMonthName(lang)
	}
	return d.LunarDayName(lang)
}

// EmptyHoliday is the quiet line when the day has none.
func EmptyHoliday(lang string) string {
	if english(lang) {
		return "No holiday on this day"
	}
	return "这一天没有国际或传统节日"
}

// HasHoliday reports whether a holiday of this id falls on the day.
func (d Day) HasHoliday(id string) bool {
	for _, h := range d.Holidays {
		if h.ID == id {
			return true
		}
	}
	return false
}
