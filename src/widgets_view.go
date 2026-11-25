package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"scriptboard/src/internal/engine"
)

// Six rich non-plot component types:
//   - donut:     multi-slice share breakdown with center total and legend
//   - checklist: task / habit list with completion circles and badges
//   - kv:        key-value status panel with status dots and pills
//   - countdown: target date countdown / anniversary with optional progress bar
//   - clock:     analog dial (with optional neon glow), digital or world clock
//   - contrib:   7-row GitHub-style activity / habit contribution grid
//
// None of these (nor progressBar / progress / progressRing) show plot chart
// style pickers, 大图 toggles or hover crosshair readouts.

func isDonut(ch *engine.Chart) bool     { return ch != nil && ch.Type == "donut" }
func isChecklist(ch *engine.Chart) bool { return ch != nil && ch.Type == "checklist" }
func isKV(ch *engine.Chart) bool        { return ch != nil && ch.Type == "kv" }
func isCountdown(ch *engine.Chart) bool { return ch != nil && ch.Type == "countdown" }
func isClock(ch *engine.Chart) bool     { return ch != nil && ch.Type == "clock" }
func isContrib(ch *engine.Chart) bool   { return ch != nil && ch.Type == "contrib" }

// isWidget reports whether ch is one of the non-plot widget components drawn by
// this file.
func isWidget(ch *engine.Chart) bool {
	return isDonut(ch) || isChecklist(ch) || isKV(ch) || isCountdown(ch) || isClock(ch) || isContrib(ch)
}

// isPlotChart reports whether ch is a coordinate plot (line, dot, candle,
// combo, panels) that supports chart styles (折线/面积/柱状…), 大图 and
// hover crosshairs. progressBar and all widget components return false.
func isPlotChart(ch *engine.Chart) bool {
	if ch == nil {
		return false
	}
	switch ch.Type {
	case "", "line", "dot", "candle", "combo", "panels":
		return true
	}
	return false
}

// hasChartDetail reports whether Detail(...) should be shown under ch.
// progressBar, progress, progressRing and non-interactive widgets have none.
func hasChartDetail(ch *engine.Chart) bool {
	return isPlotChart(ch) || isCalendar(ch)
}

// readableTextColor keeps neon colors vivid in dark mode while deepening
// ultra-bright hues (like #00F5D4) on light backgrounds so text and chips stay
// crisp and high-contrast.
func readableTextColor(col ui.Color, dark bool) ui.Color {
	if dark {
		return col
	}
	lum := 0.299*float64(col.R) + 0.587*float64(col.G) + 0.114*float64(col.B)
	if lum > 0.62 {
		return col.Mix(ui.Hex("#0F172A"), 0.36)
	}
	return col
}

// widgetView draws any of the six widget components in this file.
func widgetView(c *ui.Context, ch *engine.Chart, h float32, sel *int, full bool, now time.Time, limit int) ui.Element {
	return widgetViewWith(c, nil, "", ch, "", 0, h, sel, full, now, limit)
}

// widgetViewWith draws a widget component with optional app context, script ID,
// and runtime sub-style override (for interactive checklist toggles, KV copy
// feedback, donut slice selection, and clock style switching).
func widgetViewWith(c *ui.Context, a *app, scriptID string, ch *engine.Chart, subStyle string, width, h float32, sel *int, full bool, now time.Time, limit int) ui.Element {
	if now.IsZero() {
		now = time.Now()
	}
	switch ch.Type {
	case "donut":
		if subStyle == "classic" || subStyle == "matrix" {
			cp := *ch
			var d engine.DonutStyle
			if ch.Donut != nil {
				d = *ch.Donut
			}
			if subStyle == "classic" {
				d.Style = ""
			} else {
				d.Style = subStyle
			}
			cp.Donut = &d
			ch = &cp
		}
		return donutWidgetView(c, ch, width, h, sel, full)
	case "checklist":
		return checklistViewWith(c, a, scriptID, ch.Checklist, limit, full)
	case "kv":
		return kvViewWith(c, a, ch.KV, width, limit, full)
	case "countdown":
		return countdownView(c, ch.Countdown, now, width, limit, full)
	case "clock":
		spec := ch.Clock
		if subStyle == "analog" || subStyle == "digital" || subStyle == "matrix" || subStyle == "world" {
			var cp engine.ClockSpec
			if spec != nil {
				cp = *spec
			}
			cp.Style = subStyle
			spec = &cp
		}
		return clockView(c, spec, now, width, h, full)
	case "contrib":
		spec := ch.Contrib
		if spec != nil && (subStyle == "classic" || subStyle == "matrix") {
			cp := *spec
			if subStyle == "classic" {
				cp.Style = ""
			} else {
				cp.Style = subStyle
			}
			spec = &cp
		}
		return contribView(c, spec, width, sel, full)
	}
	return ui.Text(c, "")
}

// ---------------------------------------------------------------------------
// 1. donut: multi-slice share breakdown
// ---------------------------------------------------------------------------

var donutPalette = []ui.Color{
	ui.Hex("#3B82F6"), ui.Hex("#10B981"), ui.Hex("#F59E0B"),
	ui.Hex("#F43F5E"), ui.Hex("#8B5CF6"), ui.Hex("#06B6D4"), ui.Hex("#EC4899"),
}

func donutWidgetView(c *ui.Context, ch *engine.Chart, width, h float32, sel *int, full bool) ui.Element {
	p := pal(c)
	var items []engine.Slice
	total := 0.0
	for _, sl := range ch.Slices {
		if sl.Value > 0 {
			items = append(items, sl)
			total += sl.Value
		}
	}
	if len(items) == 0 {
		return ui.Text(c, tr("暂无分布数据")).FontSize(fsSmall).TextColor(p.muted)
	}
	var st engine.DonutStyle
	if ch.Donut != nil {
		st = *ch.Donut
	}
	thick := st.Thickness
	if thick <= 0 {
		thick = 0.26
	}
	thick = clampF(thick, 0.12, 0.45)

	narrow := !full && width > 0 && width < 210
	d := clampF(h*0.86, 92, 210)
	if narrow {
		d = clampF(min(h*0.56, width*0.54), 58, 86)
	} else if !full {
		d = clampF(h*0.95, 92, 148)
	}
	activeIdx := -1
	if full && sel != nil && *sel >= 0 && *sel < len(items) {
		activeIdx = *sel
	}
	center := st.Center
	centerSub := st.Sub
	if activeIdx >= 0 {
		it := items[activeIdx]
		center = fmt.Sprintf("%.0f%%", math.Round(it.Value/total*100))
		centerSub = it.Name
	} else if center == "" {
		center = formatAmount(total)
	}
	centerFS := clampF(d*0.17, 11, 22)
	if n := len([]rune(center)); n >= 6 {
		centerFS = clampF(d*0.13, 10, 18)
	} else if n >= 5 {
		centerFS = clampF(d*0.145, 10.5, 20)
	}
	isMatrix := st.Style == "matrix"
	vals := make([]float64, len(items))
	cols := make([]ui.Color, len(items))
	for i, it := range items {
		vals[i] = it.Value
		defCol := donutPalette[i%len(donutPalette)]
		if isMatrix {
			defCol = matrixPalette[i%len(matrixPalette)]
		}
		cols[i] = parseColor(it.Color, defCol)
	}
	maxLegend := len(items)
	if narrow && maxLegend > 3 {
		maxLegend = 3
	} else if !full && maxLegend > 4 {
		maxLegend = 4
	}
	rowGap := float32(18)
	legendMinW := float32(110)
	if narrow {
		rowGap = 8
		legendMinW = 0
	}
	return ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Wrap().Children(func() {
		ui.Column(c).Size(d, d).Shrink(0).AlignItems(ui.Center).Justify(ui.Center).
			Draw(func(pt *ui.Painter, r ui.Rect) {
				if isMatrix {
					paintMatrixDonut(pt, r, vals, cols, thick, activeIdx)
					return
				}
				cx, cy := r.X+r.W/2, r.Y+r.H/2
				outer := min(r.W, r.H)/2 - 5
				inner := outer * (1 - thick)
				slices := chart.Pie(vals, 0.016)
				for i, sl := range slices {
					col := cols[i]
					outR := outer
					if activeIdx >= 0 {
						if i == activeIdx {
							outR = outer + 3.5
						} else {
							col = col.Alpha(0.38)
						}
					}
					if st.Neon || i == activeIdx {
						pt.FillPath(chart.Sector(cx, cy, inner-2, outR+2.5, sl), cols[i].Alpha(0.24))
					}
					pt.FillPath(chart.Sector(cx, cy, inner, outR, sl), col)
				}
			}).Children(func() {
			t := ui.Text(c, center).FontSize(centerFS).FontWeight(700).SingleLine().Ellipsis("…")
			if isMatrix {
				t.TextColor(ui.Hex("#F8FAFC"))
			}
			if centerSub != "" {
				sub := ui.Text(c, centerSub).FontSize(fsTiny).TextColor(p.muted).SingleLine().Ellipsis("…")
				if isMatrix {
					sub.TextColor(ui.Hex("#94A3B8"))
				}
			}
		})
		ui.Column(c).Gap(4).Grow(1).MinWidth(legendMinW).Children(func() {
			for i := 0; i < maxLegend; i++ {
				i := i
				it := items[i]
				pct := it.Value / total * 100
				rowEl := ui.Row(c).AlignItems(ui.Center).Gap(6).Padding(3, 4).Radius(6)
				if full && sel != nil {
					if i == activeIdx {
						rowEl = rowEl.Background(p.rowSel)
					} else if rowEl.Hovered() {
						rowEl = rowEl.Background(p.hover)
					}
					rowEl = rowEl.Cursor(ui.CursorPointer)
				}
				built := rowEl.Children(func() {
					ui.Box(c).Size(8, 8).Radius(4).Background(cols[i]).Shrink(0)
					ui.Text(c, it.Name).FontSize(fsSmall).TextColor(p.muted).Grow(1).Shrink(1).MinWidth(0).SingleLine().Ellipsis("…")
					valStr := trimNum(it.Value) + it.Unit
					ui.Text(c, fmt.Sprintf("%s · %.0f%%", valStr, math.Round(pct))).FontSize(fsSmall).FontWeight(600).NoWrap().Shrink(0)
				})
				if full && sel != nil && built.Clicked() {
					if *sel == i {
						*sel = -1
					} else {
						*sel = i
					}
				}
			}
			if len(items) > maxLegend {
				ui.Text(c, fmt.Sprintf(tr("另外 %d 项"), len(items)-maxLegend)).FontSize(fsTiny).TextColor(p.muted)
			}
		})
	})
}

// ---------------------------------------------------------------------------
// 2. checklist: task / habit check list
// ---------------------------------------------------------------------------

func checklistView(c *ui.Context, items []engine.CheckItem, limit int) ui.Element {
	return checklistViewWith(c, nil, "", items, limit, false)
}

func checklistViewWith(c *ui.Context, a *app, scriptID string, items []engine.CheckItem, limit int, interactive bool) ui.Element {
	p := pal(c)
	if len(items) == 0 {
		return ui.Text(c, tr("暂无待办项")).FontSize(fsSmall).TextColor(p.muted)
	}
	isDone := func(idx int, orig bool) bool {
		if a != nil && scriptID != "" && a.checkOverrides != nil {
			if m := a.checkOverrides[scriptID]; m != nil {
				if v, ok := m[idx]; ok {
					return v
				}
			}
		}
		return orig
	}
	done := 0
	for i, it := range items {
		if isDone(i, it.Done) {
			done++
		}
	}
	shown, more := items, 0
	if limit > 0 && len(items) > limit {
		shown, more = items[:limit], len(items)-limit
	}
	return ui.Column(c).Gap(8).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			pct := float64(done) / float64(len(items)) * 100
			chip(c, fmt.Sprintf(tr("已完成 %d / %d · %.0f%%"), done, len(items), math.Round(pct)), p.accent, p.accentSoft)
		})
		for i, it := range shown {
			i, it := i, it
			checked := isDone(i, it.Done)
			col := parseColor(it.Color, p.down)
			rowEl := ui.Row(c).AlignItems(ui.Center).Gap(9).Padding(3, 4).Radius(6)
			if interactive && a != nil && scriptID != "" {
				if rowEl.Hovered() {
					rowEl = rowEl.Background(p.hover)
				}
				rowEl = rowEl.Cursor(ui.CursorPointer)
			}
			built := rowEl.Children(func() {
				if checked {
					ui.Row(c).Size(18, 18).Radius(9).Background(col).Shrink(0).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
						ui.Text(c, "✓").FontSize(11).Bold().TextColor(ui.Hex("#FFFFFF")).NoWrap()
					})
				} else {
					ui.Box(c).Size(18, 18).Radius(9).BorderWidth(1.5).BorderColor(p.border).Shrink(0)
				}
				ui.Column(c).Gap(1).Grow(1).MinWidth(0).ClipX().Children(func() {
					t := ui.Text(c, it.Label).FontSize(fsSmall).SingleLine().Ellipsis("…")
					if checked {
						t.TextColor(p.muted)
					} else {
						t.FontWeight(500)
					}
					if it.Note != "" {
						ui.Text(c, it.Note).FontSize(fsTiny).TextColor(p.faint).SingleLine().Ellipsis("…")
					}
				})
				if it.Badge != "" {
					bc := parseColor(it.Color, p.muted)
					chip(c, it.Badge, bc, bc.Alpha(0.13)).Shrink(0)
				}
			})
			if interactive && a != nil && scriptID != "" && built.Clicked() {
				if a.checkOverrides == nil {
					a.checkOverrides = map[string]map[int]bool{}
				}
				if a.checkOverrides[scriptID] == nil {
					a.checkOverrides[scriptID] = map[int]bool{}
				}
				a.checkOverrides[scriptID][i] = !checked
			}
		}
		if more > 0 {
			ui.Text(c, fmt.Sprintf(tr("另外 %d 项"), more)).FontSize(fsTiny).TextColor(p.muted)
		}
	})
}

// ---------------------------------------------------------------------------
// 3. kv: key-value status panel
// ---------------------------------------------------------------------------

func kvStatusColor(p palette, status, custom string) (ui.Color, bool) {
	if hexColor.MatchString(custom) {
		return ui.Hex(custom), true
	}
	switch status {
	case "ok":
		return p.down, true
	case "warn":
		return p.warn, true
	case "err":
		return p.up, true
	case "info":
		return p.accent, true
	}
	return ui.Color{}, false
}

func kvView(c *ui.Context, items []engine.KVItem, limit int, full bool) ui.Element {
	return kvViewWith(c, nil, items, 0, limit, full)
}

func kvViewWith(c *ui.Context, a *app, items []engine.KVItem, width float32, limit int, full bool) ui.Element {
	p := pal(c)
	if len(items) == 0 {
		return ui.Text(c, tr("暂无指标")).FontSize(fsSmall).TextColor(p.muted)
	}
	shown, more := items, 0
	if limit > 0 && len(items) > limit {
		shown, more = items[:limit], len(items)-limit
	}
	narrow := !full && width > 0 && width < 210
	rowPadY, rowPadX := float32(10), float32(6)
	rowGap := float32(10)
	valFS := fsBody
	if !full {
		rowPadY, rowPadX = 4.5, 2
	}
	if narrow {
		rowGap = 6
		valFS = 12
	}
	return ui.Column(c).Gap(0).Children(func() {
		for i, it := range shown {
			it := it
			row := ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Padding(rowPadY, rowPadX).Radius(6)
			if i > 0 {
				row = row.BorderWidth(1, 0, 0, 0).BorderColor(p.hairline)
			}
			if full {
				if row.Hovered() {
					row = row.Background(p.hover)
				}
				row = row.Cursor(ui.CursorPointer)
			}
			col, hasDot := kvStatusColor(p, it.Status, it.Color)
			built := row.Children(func() {
				if hasDot {
					ui.Box(c).Size(8, 8).Radius(4).Background(col).Shrink(0)
				}
				ui.Column(c).Gap(1).Grow(1).Shrink(1).MinWidth(0).ClipX().Children(func() {
					ui.Text(c, it.Label).FontSize(fsSmall).TextColor(p.muted).SingleLine().Ellipsis("…")
					if it.Sub != "" && (full || len(shown) <= 2) {
						ui.Text(c, it.Sub).FontSize(fsTiny).TextColor(p.faint).SingleLine().Ellipsis("…")
					}
				})
				if it.Badge != "" {
					bc := p.accent
					if hasDot {
						bc = col
					}
					chip(c, it.Badge, bc, bc.Alpha(0.13)).Shrink(0)
				}
				vt := ui.Text(c, it.Value).FontSize(valFS).FontWeight(650).FontFeatures("tnum").NoWrap().Shrink(0)
				if hasDot && (it.Status == "err" || it.Status == "warn") {
					vt.TextColor(col)
				}
			})
			if full && built.Clicked() {
				copyText(it.Value)
				if a != nil {
					a.status = tr("已复制：") + it.Value
				}
			}
		}
		if more > 0 {
			ui.Text(c, fmt.Sprintf(tr("另外 %d 项"), more)).FontSize(fsTiny).TextColor(p.muted).Padding(3, 0, 0, 0)
		}
	})
}

// ---------------------------------------------------------------------------
// 4. countdown: target date countdown / anniversary with optional progress bar
// ---------------------------------------------------------------------------

func parseTargetTime(s string, loc *time.Location) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	if loc == nil {
		loc = time.Local
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		if n > 1e11 {
			return time.UnixMilli(n).In(loc), true
		}
		return time.Unix(n, 0).In(loc), true
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"} {
		if layout == time.RFC3339 {
			if t, err := time.Parse(layout, s); err == nil {
				return t.In(loc), true
			}
			continue
		}
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func countdownParts(now, target time.Time) (days int, hours int, mins int, past bool) {
	diff := target.Sub(now)
	if diff < 0 {
		past = true
		diff = -diff
	}
	totalMins := int(diff.Minutes())
	days = totalMins / (60 * 24)
	rem := totalMins % (60 * 24)
	hours = rem / 60
	mins = rem % 60
	return days, hours, mins, past
}

func shortDateRange(start, target string) string {
	parse := func(raw string) (time.Time, bool) {
		raw = strings.TrimSpace(raw)
		for _, layout := range []string{"2006-01-02", "2006-01-02 15:04:05", "2006-01-02 15:04"} {
			if t, err := time.Parse(layout, raw); err == nil {
				return t, true
			}
		}
		return time.Time{}, false
	}
	st, ok1 := parse(start)
	tg, ok2 := parse(target)
	if !ok1 || !ok2 {
		return start + " → " + target
	}
	if st.Month() == 1 && st.Day() == 1 && tg.Month() == 1 && tg.Day() == 1 {
		return fmt.Sprintf("%d → %d", st.Year(), tg.Year())
	}
	if st.Year() == tg.Year() {
		return st.Format("01-02") + " → " + tg.Format("01-02")
	}
	return st.Format("06.01") + " → " + tg.Format("06.01")
}

func countdownView(c *ui.Context, items []engine.CountdownItem, now time.Time, width float32, limit int, full bool) ui.Element {
	p := pal(c)
	if len(items) == 0 {
		return ui.Text(c, tr("暂无倒计时")).FontSize(fsSmall).TextColor(p.muted)
	}
	shown, more := items, 0
	if limit > 0 && len(items) > limit {
		shown, more = items[:limit], len(items)-limit
	}
	narrow := !full && width > 0 && width < 210
	// Single-item hero layout: large numeric boxes + optional neon progress bar.
	if len(shown) == 1 && more == 0 {
		it := shown[0]
		target, ok := parseTargetTime(it.Target, now.Location())
		if !ok {
			return ui.Text(c, tr("目标日期格式不对")).FontSize(fsSmall).TextColor(p.up)
		}
		days, hrs, mins, past := countdownParts(now, target)
		col := parseColor(it.Color, p.accent)
		txtCol := readableTextColor(col, p.dark)
		tag := tr("距离目标")
		if past {
			tag = tr("已过去")
		} else if days == 0 && hrs == 0 && mins == 0 {
			tag = tr("就是今天")
		}
		boxMinW := float32(52)
		boxPadY, boxPadX := float32(8), float32(12)
		boxFS := fsCount
		titleFS := fsBody
		topGap := float32(8)
		colGap := float32(12)
		if full {
			boxMinW = 88
			boxPadY, boxPadX = 14, 22
			boxFS = 34
			titleFS = fsHeading
		} else if narrow {
			titleFS = 12.5
			topGap = 6
			colGap = 10
		}
		return ui.Column(c).Gap(colGap).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(topGap).Children(func() {
				ui.Text(c, it.Label).FontSize(titleFS).FontWeight(650).Grow(1).Shrink(1).MinWidth(0).SingleLine().Ellipsis("…")
				if narrow {
					ui.Text(c, tag).FontSize(10).FontWeight(600).TextColor(txtCol).Padding(2, 6).Radius(6).Background(col.Alpha(0.14)).NoWrap().Shrink(0)
				} else {
					chip(c, tag, txtCol, col.Alpha(0.14)).Shrink(0)
				}
			})
			if narrow {
				ui.Row(c).FillWidth().Gap(5).AlignItems(ui.Center).Children(func() {
					unitBox := func(val int, unit string) {
						ui.Column(c).Grow(1).MinWidth(0).Padding(6, 2).Radius(8).Background(p.chip).AlignItems(ui.Center).Children(func() {
							ui.Text(c, fmt.Sprintf("%d", val)).FontSize(16).FontWeight(700).FontFeatures("tnum").TextColor(txtCol).SingleLine().Ellipsis("…")
							ui.Text(c, unit).FontSize(10).TextColor(p.muted).NoWrap()
						})
					}
					unitBox(days, tr("天"))
					unitBox(hrs, tr("时"))
					unitBox(mins, tr("分"))
				})
			} else {
				ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
					unitBox := func(val int, unit string) {
						ui.Column(c).MinWidth(boxMinW).Padding(boxPadY, boxPadX).Radius(radControl + 2).Background(p.chip).AlignItems(ui.Center).Children(func() {
							ui.Text(c, fmt.Sprintf("%d", val)).FontSize(boxFS).FontWeight(700).FontFeatures("tnum").TextColor(txtCol).NoWrap()
							ui.Text(c, unit).FontSize(fsTiny).TextColor(p.muted).NoWrap()
						})
					}
					unitBox(days, tr("天"))
					unitBox(hrs, tr("时"))
					unitBox(mins, tr("分"))
				})
			}
			if start, ok := parseTargetTime(it.Start, now.Location()); ok && target.After(start) {
				total := target.Sub(start).Seconds()
				elapsed := clampF(float32(now.Sub(start).Seconds()/total*100), 0, 100)
				st := it.Style
				if st == "" {
					st = "neon"
				}
				rangeName := it.Start + " → " + it.Target
				if narrow {
					rangeName = shortDateRange(it.Start, it.Target)
				}
				pb := &engine.ProgressBar{
					BarStyle: engine.BarStyle{Style: st, Height: 8},
					Items:    []engine.Progress{{Name: rangeName, Value: float64(elapsed), Max: 0, Color: it.Color, To: it.To}},
				}
				progressBarView(c, pb, 1)
			} else if it.Note != "" || full {
				sub := target.Format("2006-01-02")
				if it.Note != "" {
					sub += " · " + it.Note
				}
				ui.Text(c, sub).FontSize(fsTiny).TextColor(p.muted).SingleLine().Ellipsis("…")
			}
		})
	}

	// Multi-item list layout.
	return ui.Column(c).Gap(8).Children(func() {
		for _, it := range shown {
			target, ok := parseTargetTime(it.Target, now.Location())
			col := parseColor(it.Color, p.accent)
			badgeText := "—"
			if ok {
				days, hrs, _, past := countdownParts(now, target)
				switch {
				case !past && days == 0:
					badgeText = fmt.Sprintf(tr("%d 小时"), max(hrs, 1))
				case !past:
					badgeText = fmt.Sprintf(tr("剩 %d 天"), days)
				case days == 0:
					badgeText = tr("今天")
				default:
					badgeText = fmt.Sprintf(tr("已过 %d 天"), days)
				}
			}
			ui.Column(c).Gap(4).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Box(c).Size(6, 18).Radius(3).Background(col).Shrink(0)
					ui.Column(c).Gap(1).Grow(1).Shrink(1).MinWidth(0).ClipX().Children(func() {
						ui.Text(c, it.Label).FontSize(fsSmall).FontWeight(550).SingleLine().Ellipsis("…")
						sub := it.Target
						if it.Note != "" {
							sub += " · " + it.Note
						}
						ui.Text(c, sub).FontSize(fsTiny).TextColor(p.faint).SingleLine().Ellipsis("…")
					})
					chip(c, badgeText, readableTextColor(col, p.dark), col.Alpha(0.14)).Shrink(0)
				})
				if start, ok2 := parseTargetTime(it.Start, now.Location()); ok && ok2 && target.After(start) {
					elapsed := clampF(float32(now.Sub(start).Seconds()/target.Sub(start).Seconds()*100), 0, 100)
					st := it.Style
					if st == "" {
						st = "gradient"
					}
					pb := &engine.ProgressBar{
						BarStyle: engine.BarStyle{Style: st, Height: 6, HideLabel: true},
						Items:    []engine.Progress{{Value: float64(elapsed), Max: 100, Color: it.Color, To: it.To}},
					}
					progressBarView(c, pb, 1)
				}
			})
		}
		if more > 0 {
			ui.Text(c, fmt.Sprintf(tr("另外 %d 项"), more)).FontSize(fsTiny).TextColor(p.muted)
		}
	})
}

// ---------------------------------------------------------------------------
// 5. clock: analog dial / digital / world clock
// ---------------------------------------------------------------------------

func zoneDuration(offset float64) time.Duration {
	if math.Abs(offset) > 14 {
		return time.Duration(offset) * time.Minute
	}
	return time.Duration(math.Round(offset*60)) * time.Minute
}

func zoneOffsetLabel(offset float64) string {
	mins := int(math.Round(zoneDuration(offset).Minutes()))
	sign := "+"
	if mins < 0 {
		sign = "-"
		mins = -mins
	}
	if mins%60 == 0 {
		return fmt.Sprintf("UTC%s%d", sign, mins/60)
	}
	return fmt.Sprintf("UTC%s%d:%02d", sign, mins/60, mins%60)
}

func shortZoneOffsetLabel(offset float64) string {
	mins := int(math.Round(zoneDuration(offset).Minutes()))
	sign := "+"
	if mins < 0 {
		sign = "-"
		mins = -mins
	}
	if mins%60 == 0 {
		return fmt.Sprintf("%s%d", sign, mins/60)
	}
	return fmt.Sprintf("%s%d:%02d", sign, mins/60, mins%60)
}

func clockView(c *ui.Context, spec *engine.ClockSpec, now time.Time, width, h float32, full bool) ui.Element {
	p := pal(c)
	var st engine.ClockSpec
	if spec != nil {
		st = *spec
	}
	col := parseColor(st.Color, p.accent)
	if st.Neon && !hexColor.MatchString(st.Color) {
		col = ui.Hex("#00F5D4")
	}
	txtCol := readableTextColor(col, p.dark)
	timeFmt := "15:04"
	if st.Seconds {
		timeFmt = "15:04:05"
	}
	narrow := !full && width > 0 && width < 210

	if st.Style == "world" && len(st.Zones) > 0 {
		localDay := now.Format("2006-01-02")
		shown := st.Zones
		more := 0
		if !full && len(shown) > 4 {
			shown, more = shown[:4], len(shown)-4
		}
		rowPad := float32(10)
		rowGap := float32(10)
		rowTimeFmt := timeFmt
		labelFS := fsSmall
		subFS := fsTiny
		if !full {
			rowPad = 4
		}
		if narrow {
			rowPad = 3.5
			rowGap = 6
			rowTimeFmt = "15:04"
			labelFS = 11.5
			subFS = 10
		}
		return ui.Column(c).Gap(0).Children(func() {
			for i, z := range shown {
				zt := now.UTC().Add(zoneDuration(z.Offset))
				dayDiff := tr("今天")
				if zDay := zt.Format("2006-01-02"); zDay > localDay {
					dayDiff = tr("明天")
				} else if zDay < localDay {
					dayDiff = tr("昨天")
				}
				offsetLbl := zoneOffsetLabel(z.Offset)
				if narrow {
					offsetLbl = shortZoneOffsetLabel(z.Offset)
				}
				row := ui.Row(c).AlignItems(ui.Center).Gap(rowGap).Padding(rowPad, 0)
				if i > 0 {
					row = row.BorderWidth(1, 0, 0, 0).BorderColor(p.hairline)
				}
				row.Children(func() {
					ui.Column(c).Gap(1).Grow(1).Shrink(1).MinWidth(0).ClipX().Children(func() {
						ui.Text(c, z.Label).FontSize(labelFS).FontWeight(600).SingleLine().Ellipsis("…")
						ui.Text(c, dayDiff+" · "+offsetLbl).FontSize(subFS).TextColor(p.muted).SingleLine().Ellipsis("…")
					})
					var t ui.Element
					if narrow {
						t = ui.Text(c, zt.Format(rowTimeFmt)).FontSize(14).FontWeight(700).FontFeatures("tnum").NoWrap().Shrink(0)
					} else {
						t = ui.Text(c, zt.Format(rowTimeFmt)).Font("monospace").FontSize(fsCount).FontWeight(700).NoWrap().Shrink(0)
					}
					if st.Neon {
						t.TextColor(txtCol)
					}
				})
			}
			if more > 0 {
				ui.Text(c, fmt.Sprintf(tr("另外 %d 个时区"), more)).FontSize(fsTiny).TextColor(p.muted).Padding(2, 0, 0, 0)
			}
		})
	}

	if st.Style == "matrix" {
		return matrixClockView(c, st, now, width, h, full)
	}

	if st.Style == "digital" {
		padY := float32(0)
		if full {
			padY = 28
		}
		numFS := clampF(h*0.32, 28, 56)
		if narrow {
			numFS = clampF(min(h*0.30, width*0.18), 18, 32)
		}
		return ui.Column(c).Grow(1).Padding(padY, 0).Gap(6).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			t := ui.Text(c, now.Format(timeFmt)).Font("monospace").FontSize(numFS).FontWeight(700).SingleLine().Ellipsis("…")
			if st.Neon {
				t.TextColor(txtCol)
			}
			ui.Text(c, fmt.Sprintf("%s · %s", now.Format("2006-01-02"), tr(weekdays[now.Weekday()]))).FontSize(fsSmall).TextColor(p.muted).SingleLine().Ellipsis("…")
		})
	}

	dateStr := fmt.Sprintf("%d月%d日 %s", int(now.Month()), now.Day(), tr(weekdays[now.Weekday()]))
	if curLang() == langEN {
		dateStr = fmt.Sprintf("%s · %s", now.Format("Jan 2"), tr(weekdays[now.Weekday()]))
	}
	if narrow {
		d := clampF(min(h*0.52, width*0.56), 58, 86)
		return ui.Column(c).FillWidth().AlignItems(ui.Center).Gap(6).Children(func() {
			ui.Box(c).Size(d, d).Shrink(0).Draw(func(pt *ui.Painter, r ui.Rect) {
				paintAnalogClock(pt, r, now, p, col, st.Seconds, st.Neon)
			})
			t := ui.Text(c, now.Format(timeFmt)).FontSize(15).FontWeight(700).FontFeatures("tnum").SingleLine().Ellipsis("…")
			if st.Neon {
				t.TextColor(txtCol)
			}
			ui.Text(c, dateStr).FontSize(fsTiny).TextColor(p.muted).SingleLine().Ellipsis("…")
		})
	}

	// Analog dial + digital readout beside it.
	d := clampF(h*0.86, 80, 200)
	if !full {
		d = clampF(h*0.86, 76, 136)
	}
	return ui.Row(c).AlignItems(ui.Center).Gap(22).Wrap().Children(func() {
		ui.Box(c).Size(d, d).Shrink(0).Draw(func(pt *ui.Painter, r ui.Rect) {
			paintAnalogClock(pt, r, now, p, col, st.Seconds, st.Neon)
		})
		ui.Column(c).Gap(6).MinWidth(0).Children(func() {
			t := ui.Text(c, now.Format(timeFmt)).Font("monospace").FontSize(clampF(d*0.24, 20, 36)).FontWeight(700).NoWrap()
			if st.Neon {
				t.TextColor(txtCol)
			}
			ui.Text(c, dateStr).FontSize(fsSmall).TextColor(p.muted).SingleLine().Ellipsis("…")
			if len(st.Zones) > 0 {
				limitZones := 1
				if full {
					limitZones = min(len(st.Zones), 5)
				}
				ui.Row(c).Gap(10).Wrap().Padding(4, 0, 0, 0).Children(func() {
					for i := 0; i < limitZones; i++ {
						z := st.Zones[i]
						zt := now.UTC().Add(zoneDuration(z.Offset))
						chip(c, fmt.Sprintf("%s %s", z.Label, zt.Format("15:04")), p.muted, p.chip)
					}
				})
			}
		})
	})
}

func paintAnalogClock(pt *ui.Painter, r ui.Rect, now time.Time, p palette, accent ui.Color, seconds, neon bool) {
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	rad := min(r.W, r.H)/2 - 4
	if rad <= 8 {
		return
	}
	face := p.chip
	if neon {
		face = ui.Hex("#0F172A").Alpha(0.92)
		pt.StrokePath((&ui.Path{}).Circle(cx, cy, rad+2), 5, accent.Alpha(0.18))
		pt.StrokePath((&ui.Path{}).Circle(cx, cy, rad), 2, accent)
	} else {
		pt.StrokePath((&ui.Path{}).Circle(cx, cy, rad), 1.5, p.border)
	}
	pt.FillPath((&ui.Path{}).Circle(cx, cy, rad-1), face)

	tickCol := p.muted
	handCol := p.text
	if neon {
		tickCol = accent.Alpha(0.55)
		handCol = ui.Hex("#FFFFFF")
	}
	strokeSeg := func(x0, y0, x1, y1, w float32, col ui.Color) {
		seg := &ui.Path{}
		seg.MoveTo(x0, y0)
		seg.LineTo(x1, y1)
		seg.Cap(ui.RoundCap)
		pt.StrokePath(seg, w, col)
	}
	for i := 0; i < 12; i++ {
		ang := float64(i)*math.Pi/6 - math.Pi/2
		inner := rad * 0.84
		w := float32(1.4)
		if i%3 == 0 {
			inner = rad * 0.76
			w = 2.2
		}
		cos, sin := float32(math.Cos(ang)), float32(math.Sin(ang))
		strokeSeg(cx+cos*inner, cy+sin*inner, cx+cos*(rad-4), cy+sin*(rad-4), w, tickCol)
	}
	hr := float64(now.Hour()%12) + float64(now.Minute())/60
	mn := float64(now.Minute()) + float64(now.Second())/60
	sc := float64(now.Second())

	drawHand := func(angle, length, tail, width float32, col ui.Color) {
		cos, sin := float32(math.Cos(float64(angle))), float32(math.Sin(float64(angle)))
		x0, y0 := cx-cos*tail, cy-sin*tail
		x1, y1 := cx+cos*length, cy+sin*length
		if neon {
			strokeSeg(x0, y0, x1, y1, width+3.5, col.Alpha(0.24))
		}
		strokeSeg(x0, y0, x1, y1, width, col)
	}
	drawHand(float32(hr*math.Pi/6-math.Pi/2), rad*0.50, rad*0.10, 3.6, handCol)
	drawHand(float32(mn*math.Pi/30-math.Pi/2), rad*0.72, rad*0.12, 2.5, handCol)
	if seconds {
		drawHand(float32(sc*math.Pi/30-math.Pi/2), rad*0.80, rad*0.16, 1.5, accent)
	}
	pt.Fill(ui.Rect{X: cx - 4, Y: cy - 4, W: 8, H: 8}, accent, 4)
	pt.Fill(ui.Rect{X: cx - 1.8, Y: cy - 1.8, W: 3.6, H: 3.6}, ui.Hex("#FFFFFF"), 1.8)
}

// ---------------------------------------------------------------------------
// 6. contrib: 7-row GitHub-style activity / habit contribution grid
// ---------------------------------------------------------------------------

func contribView(c *ui.Context, spec *engine.ContribSpec, width float32, sel *int, full bool) ui.Element {
	p := pal(c)
	if spec == nil || len(spec.Cells) == 0 {
		return ui.Text(c, tr("暂无打卡记录")).FontSize(fsSmall).TextColor(p.muted)
	}
	if spec.Style == "matrix" {
		return matrixContribView(c, spec, sel, full)
	}
	narrow := !full && width > 0 && width < 210
	base := parseColor(spec.Color, ui.Hex("#10B981"))
	cells := spec.Cells
	maxWeeks := spec.Weeks
	if maxWeeks <= 0 {
		maxWeeks = (len(cells) + 6) / 7
	}
	if !full && maxWeeks > 16 {
		maxWeeks = 16
	}
	if maxWeeks > 52 {
		maxWeeks = 52
	}
	if narrow {
		const bSize, gap = float32(8), float32(2)
		fitWeeks := max(6, int((width+gap)/(bSize+gap)))
		if maxWeeks > fitWeeks {
			maxWeeks = fitWeeks
		}
	}
	need := maxWeeks * 7
	if len(cells) > need {
		cells = cells[len(cells)-need:]
	}
	hi := spec.Max
	if hi <= 0 {
		for _, cell := range cells {
			if cell.Value > hi {
				hi = cell.Value
			}
		}
	}
	if hi <= 0 {
		hi = 1
	}
	levelColor := func(v float64) ui.Color {
		if v <= 0 {
			return p.chip
		}
		f := v / hi
		switch {
		case f <= 0.25:
			return base.Alpha(0.28)
		case f <= 0.50:
			return base.Alpha(0.52)
		case f <= 0.75:
			return base.Alpha(0.76)
		default:
			return base
		}
	}
	chosen := -1
	if sel != nil {
		chosen = *sel
	}
	cols := (len(cells) + 6) / 7
	boxSize := float32(12)
	gap := float32(3)
	rad := float32(2.5)
	if full {
		boxSize = 16
		gap = 4
		rad = 3.5
		if cols > 28 {
			boxSize = 13
			gap = 3
		}
	} else if narrow {
		boxSize = 8
		gap = 2
		rad = 2
	} else if cols > 12 {
		boxSize = 10
		gap = 2.5
	}
	grid := func() {
		ui.Row(c).Gap(gap).AlignItems(ui.Start).Children(func() {
			for col := 0; col < cols; col++ {
				col := col
				ui.Column(c).Gap(gap).Children(func() {
					for row := 0; row < 7; row++ {
						idx := col*7 + row
						if idx >= len(cells) {
							ui.Box(c).Size(boxSize, boxSize)
							continue
						}
						cell := cells[idx]
						colBg := levelColor(cell.Value)
						if full && sel != nil {
							btnEl := ui.ButtonBase(c.Key(fmt.Sprintf("contrib-%d", idx))).Size(boxSize, boxSize).Radius(rad).Background(colBg).Cursor(ui.CursorPointer)
							if idx == chosen {
								btnEl = btnEl.BorderWidth(1.5).BorderColor(p.text)
							}
							if btnEl.Clicked() {
								if *sel == idx {
									*sel = -1
								} else {
									*sel = idx
								}
							}
						} else {
							el := ui.Box(c).Size(boxSize, boxSize).Radius(rad).Background(colBg)
							if spec.Neon && cell.Value >= hi*0.75 {
								el = el.BorderWidth(1).BorderColor(base.Mix(ui.Hex("#FFFFFF"), 0.35))
							}
						}
					}
				})
			}
		})
	}
	return ui.Column(c).Gap(10).Children(func() {
		if full && cols > 20 {
			ui.ScrollHorizontal(c).Children(grid)
		} else {
			grid()
		}
		ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
			if chosen >= 0 && chosen < len(cells) {
				cell := cells[chosen]
				lbl := cell.Date
				if cell.Label != "" {
					lbl = cell.Label
				}
				ui.Text(c, fmt.Sprintf("%s · %s%s", lbl, trimNum(cell.Value), spec.Unit)).FontSize(fsTiny).TextColor(p.muted).Grow(1).Shrink(1).MinWidth(0).SingleLine().Ellipsis("…")
			} else {
				active := 0
				for _, cell := range cells {
					if cell.Value > 0 {
						active++
					}
				}
				msg := fmt.Sprintf(tr("活跃 %d / %d 天"), active, len(cells))
				if full {
					msg = fmt.Sprintf(tr("活跃 %d / %d 天 · 点选格子查看当天"), active, len(cells))
				}
				ui.Text(c, msg).FontSize(fsTiny).TextColor(p.muted).Grow(1).Shrink(1).MinWidth(0).SingleLine().Ellipsis("…")
			}
			if !narrow {
				ui.Spacer(c)
				ui.Text(c, tr("少")).FontSize(fsTiny).TextColor(p.faint)
				for _, f := range []float64{0, 0.25, 0.5, 0.75, 1.0} {
					ui.Box(c).Size(9, 9).Radius(2).Background(levelColor(f * hi))
				}
				ui.Text(c, tr("多")).FontSize(fsTiny).TextColor(p.faint)
			}
		})
	})
}

// widgetTiles shows summary tiles in fullView for widgets that benefit from
// them (donut, checklist), and returns true for all non-plot widgets so
// fullView never falls back to plot chart statTiles.
func (a *app) widgetTiles(c *ui.Context, ch *engine.Chart) bool {
	if ch == nil {
		return false
	}
	if isProgress(ch) || isKV(ch) || isCountdown(ch) || isClock(ch) {
		return true
	}
	p := pal(c)
	cols := 3
	if a.compact {
		cols = 2
	}
	switch ch.Type {
	case "donut":
		total, topV, topName, unit := 0.0, 0.0, "", ""
		countPos := 0
		for _, sl := range ch.Slices {
			if sl.Value <= 0 {
				continue
			}
			countPos++
			total += sl.Value
			if sl.Value > topV {
				topV, topName, unit = sl.Value, sl.Name, sl.Unit
			}
		}
		if countPos == 0 {
			return true
		}
		a.tileRow(c, cols, []tileData{
			{tr("合计"), formatAmount(total) + unit, fmt.Sprintf(tr("%d 项"), countPos), p.text},
			{tr("最大项"), formatAmount(topV) + unit, topName, p.accent},
			{tr("最高占比"), fmt.Sprintf("%.1f%%", topV/total*100), topName, p.text},
		})
		return true
	case "checklist":
		if len(ch.Checklist) == 0 {
			return true
		}
		done := 0
		for _, it := range ch.Checklist {
			if it.Done {
				done++
			}
		}
		left := len(ch.Checklist) - done
		pct := float64(done) / float64(len(ch.Checklist)) * 100
		a.tileRow(c, cols, []tileData{
			{tr("已完成"), fmt.Sprintf("%d", done), fmt.Sprintf(tr("共 %d 项"), len(ch.Checklist)), p.down},
			{tr("待完成"), fmt.Sprintf("%d", left), "", p.text},
			{tr("完成率"), fmt.Sprintf("%.0f%%", math.Round(pct)), "", p.accent},
		})
		return true
	case "contrib":
		if ch.Contrib == nil || len(ch.Contrib.Cells) == 0 {
			return true
		}
		cells := ch.Contrib.Cells
		active, curStreak, bestStreak, run := 0, 0, 0, 0
		peakV, peakDate := 0.0, ""
		for _, cell := range cells {
			if cell.Value > 0 {
				active++
				run++
				if run > bestStreak {
					bestStreak = run
				}
				if cell.Value > peakV {
					peakV = cell.Value
					peakDate = cell.Date
					if cell.Label != "" {
						peakDate = cell.Label
					}
				}
			} else {
				run = 0
			}
		}
		for i := len(cells) - 1; i >= 0; i-- {
			if cells[i].Value > 0 {
				curStreak++
			} else if i < len(cells)-1 {
				break
			}
		}
		a.tileRow(c, cols, []tileData{
			{tr("活跃天数"), fmt.Sprintf("%d / %d", active, len(cells)), fmt.Sprintf("%.0f%%", float64(active)/float64(len(cells))*100), p.accent},
			{tr("当前连续"), fmt.Sprintf(tr("%d 天"), curStreak), fmt.Sprintf(tr("最长 %d 天"), bestStreak), p.down},
			{tr("最高单日"), trimNum(peakV) + ch.Contrib.Unit, peakDate, p.text},
		})
		return true
	}
	return false
}
