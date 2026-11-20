package calendar

import "time"

// Holiday is one named day: a traditional Chinese festival, a solar term, or
// an international one. Short is what a calendar cell has room to write.
type Holiday struct {
	ID      string
	ZH, EN  string
	ShortZH string
	ShortEN string
	Term    bool // 节气, drawn quieter than a festival
}

// Name is the holiday in lang ("zh" or "en").
func (h Holiday) Name(lang string) string {
	if lang == "en" {
		return h.EN
	}
	return h.ZH
}

// Short is a cell label in lang, two or three characters when Chinese.
func (h Holiday) Short(lang string) string {
	if lang == "en" {
		if h.ShortEN != "" {
			return h.ShortEN
		}
		return h.EN
	}
	if h.ShortZH != "" {
		return h.ShortZH
	}
	return h.ZH
}

func hday(id, zh, en, shortZH, shortEN string) Holiday {
	return Holiday{ID: id, ZH: zh, EN: en, ShortZH: shortZH, ShortEN: shortEN}
}

var (
	newYear       = hday("new-year", "元旦", "New Year's Day", "元旦", "NY")
	valentines    = hday("valentines", "情人节", "Valentine's Day", "情人", "Val")
	womensDay     = hday("womens-day", "妇女节", "Women's Day", "妇女", "Women")
	arborDay      = hday("arbor-day", "植树节", "Arbor Day", "植树", "Arbor")
	aprilFools    = hday("april-fools", "愚人节", "April Fools' Day", "愚人", "Fools")
	labourDay     = hday("labour-day", "劳动节", "Labour Day", "劳动", "Labor")
	youthDay      = hday("youth-day", "青年节", "Youth Day", "青年", "Youth")
	childrensDay  = hday("childrens-day", "儿童节", "Children's Day", "儿童", "Kids")
	cpcDay        = hday("cpc-day", "建党节", "CPC Founding Day", "建党", "CPC")
	plaDay        = hday("pla-day", "建军节", "PLA Day", "建军", "PLA")
	teachersDay   = hday("teachers-day", "教师节", "Teachers' Day", "教师", "Teach")
	nationalDay   = hday("national-day", "国庆", "National Day", "国庆", "PRC")
	halloween     = hday("halloween", "万圣节", "Halloween", "万圣", "Hall")
	christmasEve  = hday("christmas-eve", "平安夜", "Christmas Eve", "平安夜", "Eve")
	christmas     = hday("christmas", "圣诞", "Christmas", "圣诞", "Xmas")
	boxingDay     = hday("boxing-day", "节礼日", "Boxing Day", "节礼", "Box")
	newYearsEve   = hday("new-years-eve", "跨年夜", "New Year's Eve", "跨年", "NYE")
	springFest    = hday("spring-festival", "春节", "Spring Festival", "春节", "Chun")
	lanternFest   = hday("lantern", "元宵", "Lantern Festival", "元宵", "Yuan")
	dragonHead    = hday("dragon-head", "龙抬头", "Dragon Head Raising", "龙抬头", "Long")
	dragonBoat    = hday("dragon-boat", "端午", "Dragon Boat Festival", "端午", "Duan")
	qixi          = hday("qixi", "七夕", "Qixi", "七夕", "Qixi")
	ghostFest     = hday("ghost", "中元", "Ghost Festival", "中元", "Ghost")
	midAutumn     = hday("mid-autumn", "中秋", "Mid-Autumn Festival", "中秋", "Moon")
	doubleNinth   = hday("double-ninth", "重阳", "Double Ninth", "重阳", "Chong")
	laba          = hday("laba", "腊八", "Laba Festival", "腊八", "Laba")
	littleNewYear = hday("little-new-year", "北方小年", "Little New Year", "小年", "Xiao")
	lunarEve      = hday("lunar-eve", "除夕", "Lunar New Year's Eve", "除夕", "Chuxi")
	qingming      = hday("qingming", "清明", "Qingming", "清明", "Qing")
	easterDay     = hday("easter", "复活节", "Easter", "复活", "Easter")
	thanksgiving  = hday("thanksgiving", "感恩节", "Thanksgiving", "感恩", "Thanks")
	mothersDay    = hday("mothers-day", "母亲节", "Mother's Day", "母亲", "Mom")
	fathersDay    = hday("fathers-day", "父亲节", "Father's Day", "父亲", "Dad")
)

var solarFixed = map[[2]int]Holiday{
	{1, 1}:   newYear,
	{2, 14}:  valentines,
	{3, 8}:   womensDay,
	{3, 12}:  arborDay,
	{4, 1}:   aprilFools,
	{5, 1}:   labourDay,
	{5, 4}:   youthDay,
	{6, 1}:   childrensDay,
	{7, 1}:   cpcDay,
	{8, 1}:   plaDay,
	{9, 10}:  teachersDay,
	{10, 1}:  nationalDay,
	{10, 31}: halloween,
	{12, 24}: christmasEve,
	{12, 25}: christmas,
	{12, 26}: boxingDay,
	{12, 31}: newYearsEve,
}

var lunarFixed = map[[2]int]Holiday{
	{1, 1}:   springFest,
	{1, 15}:  lanternFest,
	{2, 2}:   dragonHead,
	{5, 5}:   dragonBoat,
	{7, 7}:   qixi,
	{7, 15}:  ghostFest,
	{8, 15}:  midAutumn,
	{9, 9}:   doubleNinth,
	{12, 8}:  laba,
	{12, 23}: littleNewYear,
}

func holidaysOn(civil time.Time, lunar Day) []Holiday {
	var out []Holiday
	y, m, d := civil.Date()
	if lunar.Valid && !lunar.Leap {
		if h, ok := lunarFixed[[2]int{lunar.LunarMonth, lunar.LunarDay}]; ok {
			out = append(out, h)
		}
		if lunar.LunarMonth == 12 && lunar.LunarDay == lastLunarDay(lunar.LunarYear) {
			out = append(out, lunarEve)
		}
	}
	if h, ok := solarFixed[[2]int{int(m), d}]; ok {
		out = append(out, h)
	}
	if h, ok := termOn(y, civil); ok && !hasID(out, h.ID) {
		out = append(out, h)
	}
	if sameDate(civil, easterDate(y)) {
		out = append(out, easterDay)
	}
	if sameDate(civil, thanksgivingDate(y)) {
		out = append(out, thanksgiving)
	}
	if sameDate(civil, nthWeekday(y, time.May, time.Sunday, 2)) {
		out = append(out, mothersDay)
	}
	if sameDate(civil, nthWeekday(y, time.June, time.Sunday, 3)) {
		out = append(out, fathersDay)
	}
	return out
}

func hasID(hs []Holiday, id string) bool {
	for _, h := range hs {
		if h.ID == id {
			return true
		}
	}
	return false
}

func sameDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

func nthWeekday(year int, month time.Month, weekday time.Weekday, n int) time.Time {
	d := utcDate(year, int(month), 1)
	offset := (int(weekday) - int(d.Weekday()) + 7) % 7
	return d.AddDate(0, 0, offset+7*(n-1))
}

// easterDate is Easter Sunday of year, Anonymous Gregorian algorithm.
func easterDate(year int) time.Time {
	a := year % 19
	b := year / 100
	c := year % 100
	dd := b / 4
	e := b % 4
	f := (b + 8) / 25
	g := (b - f + 1) / 3
	h := (19*a + b - dd - g + 15) % 30
	i := c / 4
	k := c % 4
	l := (32 + 2*e + 2*i - h - k) % 7
	m := (a + 11*h + 22*l) / 451
	month := (h + l - 7*m + 114) / 31
	day := (h+l-7*m+114)%31 + 1
	return utcDate(year, month, day)
}

func thanksgivingDate(year int) time.Time {
	d := utcDate(year, 11, 1)
	offset := (int(time.Thursday) - int(d.Weekday()) + 7) % 7
	return d.AddDate(0, 0, offset+21)
}
