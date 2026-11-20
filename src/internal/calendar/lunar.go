// Package calendar converts a civil date to the Chinese lunar date and names
// the traditional and international holidays that fall on it.
package calendar

import "time"

// lunarInfo is the leap month and month lengths of each lunar year from 1900
// to 2100, from jjonline/calendar.js (the 2033 and 2057 corrections included).
//
// Bits 0–3 are the leap month (0 if the year has none). Bit 16 is 1 when that
// leap month has 30 days. Bits 4–15 are the 12 regular months, high bit first:
// 1 is 30 days, 0 is 29.
var lunarInfo = []uint32{
	0x04bd8, 0x04ae0, 0x0a570, 0x054d5, 0x0d260, 0x0d950, 0x16554, 0x056a0, 0x09ad0, 0x055d2, // 1900-1909
	0x04ae0, 0x0a5b6, 0x0a4d0, 0x0d250, 0x1d255, 0x0b540, 0x0d6a0, 0x0ada2, 0x095b0, 0x14977, // 1910-1919
	0x04970, 0x0a4b0, 0x0b4b5, 0x06a50, 0x06d40, 0x1ab54, 0x02b60, 0x09570, 0x052f2, 0x04970, // 1920-1929
	0x06566, 0x0d4a0, 0x0ea50, 0x16a95, 0x05ad0, 0x02b60, 0x186e3, 0x092e0, 0x1c8d7, 0x0c950, // 1930-1939
	0x0d4a0, 0x1d8a6, 0x0b550, 0x056a0, 0x1a5b4, 0x025d0, 0x092d0, 0x0d2b2, 0x0a950, 0x0b557, // 1940-1949
	0x06ca0, 0x0b550, 0x15355, 0x04da0, 0x0a5b0, 0x14573, 0x052b0, 0x0a9a8, 0x0e950, 0x06aa0, // 1950-1959
	0x0aea6, 0x0ab50, 0x04b60, 0x0aae4, 0x0a570, 0x05260, 0x0f263, 0x0d950, 0x05b57, 0x056a0, // 1960-1969
	0x096d0, 0x04dd5, 0x04ad0, 0x0a4d0, 0x0d4d4, 0x0d250, 0x0d558, 0x0b540, 0x0b6a0, 0x195a6, // 1970-1979
	0x095b0, 0x049b0, 0x0a974, 0x0a4b0, 0x0b27a, 0x06a50, 0x06d40, 0x0af46, 0x0ab60, 0x09570, // 1980-1989
	0x04af5, 0x04970, 0x064b0, 0x074a3, 0x0ea50, 0x06b58, 0x05ac0, 0x0ab60, 0x096d5, 0x092e0, // 1990-1999
	0x0c960, 0x0d954, 0x0d4a0, 0x0da50, 0x07552, 0x056a0, 0x0abb7, 0x025d0, 0x092d0, 0x0cab5, // 2000-2009
	0x0a950, 0x0b4a0, 0x0baa4, 0x0ad50, 0x055d9, 0x04ba0, 0x0a5b0, 0x15176, 0x052b0, 0x0a930, // 2010-2019
	0x07954, 0x06aa0, 0x0ad50, 0x05b52, 0x04b60, 0x0a6e6, 0x0a4e0, 0x0d260, 0x0ea65, 0x0d530, // 2020-2029
	0x05aa0, 0x076a3, 0x096d0, 0x04afb, 0x04ad0, 0x0a4d0, 0x1d0b6, 0x0d250, 0x0d520, 0x0dd45, // 2030-2039
	0x0b5a0, 0x056d0, 0x055b2, 0x049b0, 0x0a577, 0x0a4b0, 0x0aa50, 0x1b255, 0x06d20, 0x0ada0, // 2040-2049
	0x14b63, 0x09370, 0x049f8, 0x04970, 0x064b0, 0x168a6, 0x0ea50, 0x06aa0, 0x1a6c4, 0x0aae0, // 2050-2059
	0x092e0, 0x0d2e3, 0x0c960, 0x0d557, 0x0d4a0, 0x0da50, 0x05d55, 0x056a0, 0x0a6d0, 0x055d4, // 2060-2069
	0x052d0, 0x0a9b8, 0x0a950, 0x0b4a0, 0x0b6a6, 0x0ad50, 0x055a0, 0x0aba4, 0x0a5b0, 0x052b0, // 2070-2079
	0x0b273, 0x06930, 0x07337, 0x06aa0, 0x0ad50, 0x14b55, 0x04b60, 0x0a570, 0x054e4, 0x0d160, // 2080-2089
	0x0e968, 0x0d520, 0x0daa0, 0x16aa6, 0x056d0, 0x04ae0, 0x0a9d4, 0x0a2d0, 0x0d150, 0x0f252, // 2090-2099
	0x0d520, // 2100
}

const (
	minYear = 1900
	maxYear = 2100
)

// Day is one civil date with its lunar counterpart and the holidays on it.
type Day struct {
	Time time.Time

	LunarYear, LunarMonth, LunarDay int
	Leap                            bool
	Valid                           bool

	Holidays []Holiday
}

// Lookup reads the lunar date and holidays of t. Dates before 1900-01-31 or
// after 2100-12-31 have Valid false and only the civil date and solar holidays.
func Lookup(t time.Time) Day {
	y, m, d := t.Date()
	loc := t.Location()
	if loc == nil {
		loc = time.UTC
	}
	civil := time.Date(y, m, d, 0, 0, 0, 0, loc)
	out := Day{Time: civil}
	out.Holidays = holidaysOn(civil, Day{})
	if y < minYear || y > maxYear || (y == minYear && (m < time.January || (m == time.January && d < 31))) {
		return out
	}
	lunar, ok := solarToLunar(y, int(m), d)
	if !ok {
		return out
	}
	out.LunarYear, out.LunarMonth, out.LunarDay = lunar.year, lunar.month, lunar.day
	out.Leap, out.Valid = lunar.leap, true
	out.Holidays = holidaysOn(civil, out)
	return out
}

type lunarDate struct {
	year, month, day int
	leap             bool
}

func solarToLunar(y, m, d int) (lunarDate, bool) {
	offset := int(utcDate(y, m, d).Sub(utcDate(1900, 1, 31)) / (24 * time.Hour))
	if offset < 0 {
		return lunarDate{}, false
	}
	var temp int
	i := 1900
	for i < 2101 && offset > 0 {
		temp = yearDays(i)
		offset -= temp
		i++
	}
	if offset < 0 {
		offset += temp
		i--
	}
	year := i
	if year < minYear || year > maxYear {
		return lunarDate{}, false
	}
	leap := leapMonth(year)
	isLeap := false
	i = 1
	for i < 13 && offset > 0 {
		if leap > 0 && i == leap+1 && !isLeap {
			i--
			isLeap = true
			temp = leapDays(year)
		} else {
			temp = monthDays(year, i)
		}
		if isLeap && i == leap+1 {
			isLeap = false
		}
		offset -= temp
		i++
	}
	if offset == 0 && leap > 0 && i == leap+1 {
		if isLeap {
			isLeap = false
		} else {
			isLeap = true
			i--
		}
	}
	if offset < 0 {
		offset += temp
		i--
	}
	return lunarDate{year: year, month: i, day: offset + 1, leap: isLeap}, true
}

func utcDate(y, m, d int) time.Time {
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
}

func bitsOf(year int) uint32 {
	i := year - minYear
	if i < 0 || i >= len(lunarInfo) {
		return 0
	}
	return lunarInfo[i]
}

func leapMonth(year int) int { return int(bitsOf(year) & 0xf) }

func leapDays(year int) int {
	if leapMonth(year) == 0 {
		return 0
	}
	if bitsOf(year)&0x10000 != 0 {
		return 30
	}
	return 29
}

func monthDays(year, month int) int {
	if month < 1 || month > 12 {
		return -1
	}
	if bitsOf(year)&(0x10000>>uint(month)) != 0 {
		return 30
	}
	return 29
}

func yearDays(year int) int {
	sum := 348
	for i := uint32(0x8000); i > 0x8; i >>= 1 {
		if bitsOf(year)&i != 0 {
			sum++
		}
	}
	return sum + leapDays(year)
}

func lastLunarDay(year int) int { return monthDays(year, 12) }
