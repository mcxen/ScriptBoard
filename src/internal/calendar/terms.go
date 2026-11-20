package calendar

import "time"

// jieqi is the 24 solar terms, in order from 小寒. 清明 is the public holiday
// already named qingming; the others are quieter 节气.
var jieqi = []Holiday{
	term("xiaohan", "小寒", "Minor Cold", "小寒", "Cold"),
	term("dahan", "大寒", "Major Cold", "大寒", "Cold"),
	term("lichun", "立春", "Start of Spring", "立春", "Spring"),
	term("yushui", "雨水", "Rain Water", "雨水", "Rain"),
	term("jingzhe", "惊蛰", "Awakening of Insects", "惊蛰", "Bugs"),
	term("chunfen", "春分", "Spring Equinox", "春分", "Equi"),
	qingming,
	term("guyu", "谷雨", "Grain Rain", "谷雨", "Grain"),
	term("lixia", "立夏", "Start of Summer", "立夏", "Sum"),
	term("xiaoman", "小满", "Grain Buds", "小满", "Buds"),
	term("mangzhong", "芒种", "Grain in Ear", "芒种", "Ear"),
	term("xiazhi", "夏至", "Summer Solstice", "夏至", "Sol"),
	term("xiaoshu", "小暑", "Minor Heat", "小暑", "Heat"),
	term("dashu", "大暑", "Major Heat", "大暑", "Heat"),
	term("liqiu", "立秋", "Start of Autumn", "立秋", "Aut"),
	term("chushu", "处暑", "End of Heat", "处暑", "Heat"),
	term("bailu", "白露", "White Dew", "白露", "Dew"),
	term("qiufen", "秋分", "Autumn Equinox", "秋分", "Equi"),
	term("hanlu", "寒露", "Cold Dew", "寒露", "Dew"),
	term("shuangjiang", "霜降", "Frost", "霜降", "Frost"),
	term("lidong", "立冬", "Start of Winter", "立冬", "Win"),
	term("xiaoxue", "小雪", "Minor Snow", "小雪", "Snow"),
	term("daxue", "大雪", "Major Snow", "大雪", "Snow"),
	term("dongzhi", "冬至", "Winter Solstice", "冬至", "Sol"),
}

func term(id, zh, en, shortZH, shortEN string) Holiday {
	h := hday(id, zh, en, shortZH, shortEN)
	h.Term = true
	return h
}

// sTermInfo is minutes from 1900 小寒 for each term, from jjonline/calendar.js.
var sTermInfo = []int64{
	0, 21208, 42467, 63836, 85337, 107014, 128867, 150921,
	173149, 195551, 218072, 240693, 263343, 285989, 308563, 331033,
	353350, 375494, 397447, 419210, 440795, 462224, 483532, 504758,
}

func termOn(year int, civil time.Time) (Holiday, bool) {
	if year < minYear || year > maxYear {
		return Holiday{}, false
	}
	for n, h := range jieqi {
		if sameDate(civil, solarTermDate(year, n)) {
			return h, true
		}
	}
	return Holiday{}, false
}

// solarTermDate is term n (0 = 小寒) of year, UTC civil date.
func solarTermDate(year, n int) time.Time {
	if n < 0 || n >= len(sTermInfo) {
		return time.Time{}
	}
	const yearMs = 31556925974.7
	base := time.Date(1900, 1, 6, 2, 5, 0, 0, time.UTC).UnixMilli()
	ms := int64(yearMs*float64(year-1900)) + sTermInfo[n]*60000 + base
	t := time.UnixMilli(ms).UTC()
	return utcDate(t.Year(), int(t.Month()), t.Day())
}
