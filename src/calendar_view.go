package main

import (
	"strconv"
	"time"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/calendar"
	"scriptboard/src/internal/engine"
)

func isCalendar(ch *engine.Chart) bool { return ch != nil && ch.Type == "calendar" }

func ymdOf(t time.Time) int {
	y, m, d := t.Date()
	return y*10000 + int(m)*100 + d
}

func parseYMD(n int, loc *time.Location) (time.Time, bool) {
	if n < 19000131 || n > 21001231 {
		return time.Time{}, false
	}
	if loc == nil {
		loc = time.Local
	}
	y, m, d := n/10000, time.Month((n/100)%100), n%100
	t := time.Date(y, m, d, 0, 0, 0, 0, loc)
	if t.Year() != y || t.Month() != m || t.Day() != d {
		return time.Time{}, false
	}
	return t, true
}

func civilDay(t time.Time) time.Time {
	y, m, d := t.Date()
	loc := t.Location()
	if loc == nil {
		loc = time.Local
	}
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

func monthWeeks(month time.Time) int {
	start := int(month.Weekday())
	days := time.Date(month.Year(), month.Month()+1, 0, 0, 0, 0, 0, month.Location()).Day()
	n := (start + days + 6) / 7
	if n < 1 {
		return 1
	}
	return n
}

func chosenDay(sel *int, now time.Time) time.Time {
	today := civilDay(now)
	if sel != nil && *sel > 0 {
		if t, ok := parseYMD(*sel, today.Location()); ok {
			return t
		}
	}
	return today
}

func moveSel(sel *int, now time.Time, years, months, days int) {
	if sel == nil {
		return
	}
	n := chosenDay(sel, now).AddDate(years, months, days)
	if _, ok := parseYMD(ymdOf(n), n.Location()); ok {
		*sel = ymdOf(n)
	}
}

// calView is how a month is drawn: a card is a quiet grid, the full page
// turns days into buttons and shows the lunar line under each date.
type calView struct {
	Sel                 *int
	Now                 time.Time
	Height              float32
	Nav, Headers        bool
	Captions, Clickable bool
}

func calendarOpts(full bool, h float32) calView {
	return calView{
		Height:    h,
		Nav:       full,
		Headers:   full || h >= 200,
		Captions:  full || h >= 110,
		Clickable: full,
	}
}

func calendarMonth(c *ui.Context, v calView) ui.Element {
	now := v.Now
	if now.IsZero() {
		now = time.Now()
	}
	today := civilDay(now)
	loc := today.Location()
	chosen := chosenDay(v.Sel, now)
	view := chosen
	month := time.Date(view.Year(), view.Month(), 1, 0, 0, 0, 0, loc)
	lang := curLang()
	p := pal(c)
	h := v.Height

	c.Key("cal:" + month.Format("2006-01"))
	if v.Clickable && v.Sel != nil {
		c.OnWindowShortcut(0, ui.KeyLeft, func() { moveSel(v.Sel, now, 0, 0, -1) })
		c.OnWindowShortcut(0, ui.KeyRight, func() { moveSel(v.Sel, now, 0, 0, 1) })
		c.OnWindowShortcut(0, ui.KeyUp, func() { moveSel(v.Sel, now, 0, 0, -7) })
		c.OnWindowShortcut(0, ui.KeyDown, func() { moveSel(v.Sel, now, 0, 0, 7) })
		c.OnWindowShortcut(0, ui.KeyHome, func() { *v.Sel = -1 })
	}

	col := ui.Column(c).Gap(8)
	if h > 0 {
		col = col.Height(h)
	} else {
		col = col.Grow(1)
	}
	return col.Children(func() {
		if v.Nav {
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				if btn(c, tr("上月")).Clicked() && v.Sel != nil {
					moveSel(v.Sel, now, 0, -1, 0)
				}
				ui.Spacer(c)
				ui.Text(c, calendar.MonthTitle(month, lang)).FontSize(fsHeading).Bold().NoWrap()
				ui.Spacer(c)
				if btn(c, tr("今天")).Clicked() && v.Sel != nil {
					*v.Sel = -1
				}
				if btn(c, tr("下月")).Clicked() && v.Sel != nil {
					moveSel(v.Sel, now, 0, 1, 0)
				}
			})
		}
		weeks := 6
		gap := float32(2)
		if v.Nav || v.Headers {
			gap = 4
		}
		if !v.Nav {
			weeks = monthWeeks(month)
		}
		grid := ui.Grid(c).Columns(7).Gap(gap).Grow(1)
		if v.Headers {
			tracks := make([]ui.Track, 1+weeks)
			tracks[0] = ui.FitContent()
			for i := 1; i < len(tracks); i++ {
				tracks[i] = ui.Fr(1)
			}
			grid.RowTracks(tracks...)
		} else if h >= 80 || h <= 0 {
			tracks := make([]ui.Track, weeks)
			for i := range tracks {
				tracks[i] = ui.Fr(1)
			}
			grid.RowTracks(tracks...)
		}
		grid.Children(func() {
			if v.Headers {
				for _, name := range weekdays {
					ui.Text(c, tr(name)).FontSize(fsTiny).FontWeight(600).TextColor(p.muted).TextAlign(ui.Center).Padding(0, 0, 4, 0)
				}
			}
			start := int(month.Weekday())
			for r := 0; r < weeks; r++ {
				for col := 0; col < 7; col++ {
					dayNum := r*7 + col - start + 1
					cell := month.AddDate(0, 0, dayNum-1)
					calendarCell(c, cell, month, today, chosen, v)
				}
			}
		})
	})
}

func calendarCell(c *ui.Context, cell, month, today, chosen time.Time, v calView) {
	p := pal(c)
	inMonth := cell.Month() == month.Month() && cell.Year() == month.Year()
	iso := cell.Format("2006-01-02")
	isToday := cell.Equal(today)
	isChosen := v.Clickable && cell.Equal(chosen)
	day := calendar.Lookup(cell)
	fg := p.text
	switch {
	case !inMonth:
		fg = p.faint
	case isToday:
		fg = p.accent
	case len(day.Holidays) > 0:
		fg = p.accent
	}

	box := ui.Column(c).Radius(8)
	if isToday {
		box = box.Background(p.accentSoft)
	} else if isChosen {
		box = box.Background(p.chip)
	}
	if isChosen {
		box = box.BorderWidth(1.5).BorderColor(p.accent)
	}
	box.Children(func() {
		face := func() {
			calendarCellFace(c, cell, day, fg, inMonth, isToday, v)
		}
		if v.Clickable && v.Sel != nil {
			c.Key("cal-day-" + iso)
			b := ui.ButtonBase(c).Fill().Radius(8).Cursor(ui.CursorPointer).Label(iso)
			if b.Clicked() {
				*v.Sel = ymdOf(cell)
			}
			if b.Hovered() && !isToday && !isChosen {
				b.Background(p.hover)
			}
			b.Children(face)
			return
		}
		face()
	})
}

func calendarCellFace(c *ui.Context, cell time.Time, day calendar.Day, fg ui.Color, inMonth, isToday bool, v calView) {
	p := pal(c)
	showCaption := inMonth && v.Captions
	numSize := fsTiny
	if v.Captions || v.Clickable {
		numSize = fsBody
	}
	caption := ""
	if showCaption {
		caption = day.CellCaption(curLang())
	}
	ui.Column(c).Grow(1).AlignItems(ui.Center).Justify(ui.Center).Gap(1).Padding(4, 2).Children(func() {
		n := ui.Text(c, strconv.Itoa(cell.Day())).FontSize(numSize).TextColor(fg).TextAlign(ui.Center).NoWrap()
		if isToday {
			n.Bold()
		}
		if caption == "" {
			return
		}
		capColor := p.faint
		if len(day.Holidays) > 0 && !day.Holidays[0].Term {
			capColor = p.accent
		} else if len(day.Holidays) > 0 {
			capColor = p.muted
		}
		if !inMonth {
			capColor = p.faint
		}
		cap := ui.Text(c, caption).FontSize(fsTiny).TextColor(capColor).TextAlign(ui.Center).NoWrap()
		if len(day.Holidays) > 0 {
			cap.FontWeight(600)
		}
	})
}

// calendarDetail is the panel of the chosen day: its lunar date and holidays.
func calendarDetail(c *ui.Context, sel *int, now time.Time) ui.Element {
	if now.IsZero() {
		now = time.Now()
	}
	chosen := chosenDay(sel, now)
	lang := curLang()
	day := calendar.Lookup(chosen)
	p := pal(c)
	return ui.Column(c).Gap(12).Padding(20).Radius(12).Background(p.chip).Children(func() {
		ui.Row(c).AlignItems(ui.End).Gap(12).Children(func() {
			n := ui.Text(c, strconv.Itoa(chosen.Day())).FontSize(44).Bold().NoWrap()
			if len(day.Holidays) > 0 {
				n.TextColor(p.accent)
			}
			ui.Column(c).Gap(2).Children(func() {
				ui.Text(c, day.WeekdayName(lang)).FontSize(fsSmall).FontWeight(600).TextColor(p.muted).NoWrap()
				ui.Text(c, calendar.MonthTitle(chosen, lang)).FontSize(fsSmall).TextColor(p.muted).NoWrap()
			})
		})
		ui.Box(c).Height(1).FillWidth().Background(p.hairline)
		if day.Valid {
			ui.Text(c, day.LunarLine(lang)).FontSize(fsBody)
			if z := zodiacLine(day, lang); z != "" {
				ui.Text(c, z).FontSize(fsSmall).TextColor(p.muted)
			}
		}
		if len(day.Holidays) == 0 {
			ui.Text(c, calendar.EmptyHoliday(lang)).FontSize(fsSmall).TextColor(p.faint)
			return
		}
		ui.Row(c).Gap(8).Wrap().Children(func() {
			for _, h := range day.Holidays {
				fg, bg := p.accent, p.accentSoft
				if h.Term {
					fg, bg = p.text, p.hover
				}
				ui.Text(c, h.Name(lang)).FontSize(fsSmall).FontWeight(600).TextColor(fg).Padding(5, 10).Radius(8).Background(bg)
			}
		})
	})
}

func zodiacLine(day calendar.Day, lang string) string {
	z := day.Zodiac(lang)
	if z == "" {
		return ""
	}
	gz := day.YearGZ(lang)
	if lang == "en" {
		if gz != "" {
			return gz + " · Year of the " + z
		}
		return "Year of the " + z
	}
	if gz != "" {
		return gz + "年 · 生肖" + z
	}
	return "生肖 · " + z
}

func calFullH(a *app, compact bool) float32 {
	if compact {
		return 300
	}
	h := float32(420)
	if a != nil && a.height > 0 {
		h = max(360, a.height-240)
	}
	return min(h, 560)
}

// calendarPage is the full-screen calendar: the month on the left, the
// chosen day on the right, with no chart chrome.
func (a *app) calendarPage(c *ui.Context, sel *int, compact bool) {
	now := a.now()
	side := !compact && a.contentW() >= 760
	h := calFullH(a, compact)
	v := calView{Sel: sel, Now: now, Height: h, Nav: true, Headers: true, Captions: true, Clickable: true}
	card(c).Padding(compactPad(compact)).Gap(gapItem).Children(func() {
		if side {
			ui.Row(c).AlignItems(ui.Stretch).Gap(20).Children(func() {
				calendarMonth(c, v).Grow(1).MinWidth(0)
				calendarDetail(c, sel, now).Width(300).Shrink(0)
			})
			return
		}
		calendarMonth(c, v)
		calendarDetail(c, sel, now)
	})
}

// calendarHomeCard draws the month itself on a home card, not a headline
// number with a spark under it.
func (a *app) calendarHomeCard(c *ui.Context, size int, s, cardH float32) {
	now := a.now()
	lang := curLang()
	day := calendar.Lookup(now)
	p := pal(c)
	if size == sizeMini {
		ui.Spacer(c)
		ui.Text(c, strconv.Itoa(day.Time.Day())).FontSize(clampF(44*s, 28, 60)).Bold().NoWrap()
		return
	}
	if size == 0 {
		ui.Text(c, strconv.Itoa(day.Time.Day())).FontSize(clampF(30*s, 22, 44)).Bold().NoWrap()
		ui.Text(c, day.Headline(lang)).FontSize(fsSmall).TextColor(p.muted).NoWrap()
		return
	}
	ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
		ui.Text(c, calendar.MonthTitle(now, lang)).FontSize(fsSmall).FontWeight(600).NoWrap().Shrink(0)
		ui.Spacer(c)
		ui.Row(c).Shrink(1).MinWidth(0).ClipX().Children(func() {
			ui.Text(c, day.Headline(lang)).FontSize(fsTiny).TextColor(p.muted).NoWrap()
		})
	})
	gridH := max(96, cardH*s-64*s)
	calendarMonth(c, calView{
		Now:      now,
		Height:   gridH,
		Headers:  true,
		Captions: true,
	})
}

// calendarPreview is the editor's live month: days are buttons, and the
// chosen day has its lunar line and holidays under the grid.
func (a *app) calendarPreview(c *ui.Context, sel *int, h float32) {
	now := a.now()
	calendarMonth(c, calView{
		Sel: sel, Now: now, Height: h,
		Nav: true, Headers: true, Captions: true, Clickable: true,
	})
	calendarDetail(c, sel, now)
}
