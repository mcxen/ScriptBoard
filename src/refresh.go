package main

import (
	"context"
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"scriptboard/src/internal/barui"
	"scriptboard/src/internal/cmdrun"
	"scriptboard/src/internal/feed"
	"scriptboard/src/internal/store"
)

type refreshPhase uint8

const (
	refreshIdle refreshPhase = iota
	refreshRunning
	refreshOK
	refreshError
)

const (
	refreshOKDuration    = 2200 * time.Millisecond
	refreshErrorDuration = 3500 * time.Millisecond
)

type refreshState struct {
	phase     refreshPhase
	step      int
	total     int
	startedAt time.Time
	doneAt    time.Time
	errMsg    string
	feedErr   string
	gen       uint64
}

func (a *app) beginRefresh(id string, total int) {
	if a.refreshing == nil {
		a.refreshing = map[string]bool{}
	}
	if a.refreshStates == nil {
		a.refreshStates = map[string]refreshState{}
	}
	if total < 1 {
		total = 1
	}
	a.refreshGen++
	a.refreshing[id] = true
	a.refreshStates[id] = refreshState{
		phase:     refreshRunning,
		step:      0,
		total:     total,
		startedAt: a.now(),
		gen:       a.refreshGen,
	}
	a.redrawFlyouts()
}

func (a *app) advanceRefresh(id string, step int, feedErr string) {
	st, ok := a.refreshStates[id]
	if !ok || st.phase != refreshRunning {
		return
	}
	if step > st.total {
		step = st.total
	}
	st.step = step
	if feedErr != "" && st.feedErr == "" {
		st.feedErr = feedErr
	}
	a.refreshStates[id] = st
	a.redrawFlyouts()
}

func (a *app) finishRefresh(id string, ok bool, errMsg string) {
	delete(a.refreshing, id)
	if a.refreshStates == nil {
		a.refreshStates = map[string]refreshState{}
	}
	prev := a.refreshStates[id]
	a.refreshGen++
	gen := a.refreshGen
	phase := refreshOK
	dur := refreshOKDuration
	if !ok {
		phase = refreshError
		dur = refreshErrorDuration
	}
	total := prev.total
	if total < 1 {
		total = 1
	}
	a.refreshStates[id] = refreshState{
		phase:     phase,
		step:      total,
		total:     total,
		startedAt: prev.startedAt,
		doneAt:    a.now(),
		errMsg:    errMsg,
		gen:       gen,
	}
	a.redrawFlyouts()
	if a.win != nil {
		time.AfterFunc(dur, func() {
			a.post(func() { a.clearExpiredRefresh(id, gen) })
		})
	}
}

func (a *app) clearExpiredRefresh(id string, gen uint64) {
	if st, ok := a.refreshStates[id]; ok && st.gen == gen {
		delete(a.refreshStates, id)
		a.redrawFlyouts()
	}
}

func (a *app) refreshStateOf(id string) refreshState {
	st, ok := a.refreshStates[id]
	if !ok {
		return refreshState{}
	}
	switch st.phase {
	case refreshOK:
		if a.now().Sub(st.doneAt) >= refreshOKDuration {
			return refreshState{}
		}
	case refreshError:
		if a.now().Sub(st.doneAt) >= refreshErrorDuration {
			return refreshState{}
		}
	}
	return st
}

func (a *app) isRefreshing(id string) bool {
	return a.refreshing[id] || a.refreshStateOf(id).phase == refreshRunning
}

func (a *app) refreshActive(id string) bool {
	return a.refreshStateOf(id).phase != refreshIdle
}

func (a *app) redrawFlyouts() {
	for _, f := range a.barFlyouts {
		if f != nil {
			f.Update(func() {})
		}
	}
}

func refreshTooltip(baseLabel string, st refreshState) string {
	switch st.phase {
	case refreshRunning:
		if st.total > 1 {
			return fmt.Sprintf(tr("刷新中 %d/%d…"), st.step, st.total)
		}
		return tr("刷新中…")
	case refreshOK:
		return tr("已刷新")
	case refreshError:
		if st.errMsg != "" {
			return fmt.Sprintf(tr("刷新失败：%s"), st.errMsg)
		}
		return tr("刷新失败")
	default:
		return baseLabel
	}
}

func refreshButtonLabel(st refreshState) string {
	switch st.phase {
	case refreshRunning:
		if st.total > 1 {
			return fmt.Sprintf("%d/%d", st.step, st.total)
		}
		return tr("刷新中…")
	case refreshOK:
		return tr("已刷新")
	case refreshError:
		return tr("刷新失败")
	default:
		return tr("刷新")
	}
}

func refreshTextButtonWidth() float32 {
	if curLang() == langEN {
		return 114
	}
	return 86
}

func refreshIconSVG(st refreshState) *ui.SVG {
	switch st.phase {
	case refreshOK:
		if s := barui.DefaultIcon("check"); s != nil {
			return s
		}
	case refreshError:
		if s := barui.DefaultIcon("alert-circle"); s != nil {
			return s
		}
	}
	return icon("refresh-cw")
}

func paintRefreshRing(pt *ui.Painter, r ui.Rect, st refreshState, col ui.Color) {
	if st.phase != refreshRunning || st.total <= 1 || r.W < 10 || r.H < 10 {
		return
	}
	cx := r.X + r.W/2
	cy := r.Y + r.H/2
	rad := min(r.W, r.H)/2 - 1.5
	if rad <= 3 {
		return
	}
	pt.StrokePath((&ui.Path{}).Circle(cx, cy, rad), 1.5, col.Alpha(0.20))
	frac := float32(st.step) / float32(st.total)
	if frac < 0.15 {
		frac = 0.15
	}
	if frac > 1 {
		frac = 1
	}
	if frac >= 0.999 {
		pt.StrokePath((&ui.Path{}).Circle(cx, cy, rad), 1.5, col)
		return
	}
	arc := circleFromTop(cx, cy, rad).Cap(ui.RoundCap).Trim(0, frac)
	pt.StrokePath(arc, 1.5, col)
}

func paintRefreshBar(pt *ui.Painter, r ui.Rect, st refreshState, col ui.Color) {
	if st.phase != refreshRunning || st.total <= 1 || r.W < 12 || r.H < 6 {
		return
	}
	frac := float32(st.step) / float32(st.total)
	if frac < 0.15 {
		frac = 0.15
	}
	if frac > 1 {
		frac = 1
	}
	barW := max(r.W-4, 0)
	barH := float32(2)
	barX := r.X + 2
	barY := r.Y + r.H - barH - 1
	pt.Fill(ui.Rect{X: barX, Y: barY, W: barW, H: barH}, col.Alpha(0.18), 1)
	pt.Fill(ui.Rect{X: barX, Y: barY, W: barW * frac, H: barH}, col, 1)
}

// refreshScript brings a card's data up to date now, instead of at the next
// interval: it polls the data sources the script reads (curl, tcp, file…),
// forgets the AI usage it has cached, and runs the script again, which asks
// its own sources (meta.sources, such as a custom endpoint) afresh. Offline,
// nothing is requested and the script runs again on what is stored.
func (a *app) refreshScript(sc store.Script) {
	if a.isRefreshing(sc.ID) {
		return
	}
	meta, metaErr := a.metaOf(sc)
	if metaErr != nil {
		a.beginRefresh(sc.ID, 1)
		a.finishRefresh(sc.ID, false, metaErr.Error())
		return
	}
	feeds := a.store.Feeds()
	var matched []feed.Source
	for _, id := range meta.Feeds {
		for _, src := range feeds {
			if src.ID == id {
				matched = append(matched, src)
			}
		}
	}
	var matchedCmds []cmdrun.Spec
	for _, cid := range meta.Commands {
		if csp, ok := a.store.Command(cid); ok {
			matchedCmds = append(matchedCmds, csp)
		}
	}
	offline := a.store.Settings().Offline
	total := len(matched) + len(matchedCmds) + 1
	a.beginRefresh(sc.ID, total)

	run := func() {
		var feedErr string
		step := 0
		if !offline {
			for _, src := range matched {
				if _, err := a.runner.Poll(context.Background(), src); err != nil && feedErr == "" {
					feedErr = err.Error()
				}
				step++
				curStep, fe := step, feedErr
				a.post(func() { a.advanceRefresh(sc.ID, curStep, fe) })
			}
			for _, csp := range matchedCmds {
				rec, err := a.cmdRunner.Run(context.Background(), csp, nil, "script")
				if err != nil && feedErr == "" {
					feedErr = err.Error()
				} else if err == nil && !rec.OK && feedErr == "" {
					feedErr = rec.Error
					if feedErr == "" {
						feedErr = fmt.Sprintf("command %s exited %d", csp.ID, rec.ExitCode)
					}
				}
				step++
				curStep, fe := step, feedErr
				a.post(func() { a.advanceRefresh(sc.ID, curStep, fe) })
			}
		}
		a.post(func() {
			if st, ok := a.refreshStates[sc.ID]; ok && feedErr != "" && st.feedErr == "" {
				st.feedErr = feedErr
				a.refreshStates[sc.ID] = st
			}
			for _, csp := range matchedCmds {
				a.refreshCommandConsumers(csp.ID)
			}
			a.forced[sc.ID] = true
			if len(meta.Usage) > 0 {
				a.usage.mu.Lock()
				a.usage.entries = nil
				a.usage.mu.Unlock()
			}
			if meta.NetProbe != nil {
				a.forgetNetProbe()
			}
			a.invalidate(sc.ID)
			_, vals := a.viewOf(sc.ID, meta.Dims)
			a.ensure(a.runKeyOf(sc.ID, sc, meta, vals), sc, vals)
		})
	}
	if a.win == nil {
		run()
		return
	}
	go run()
}

func (a *app) refreshIconButton(c *ui.Context, key, id, accLabel string, size, iconSize float32, show bool, bgHover, fgIdle, accent, okCol, errCol ui.Color, onClick func()) ui.Element {
	st := a.refreshStateOf(id)
	tip := refreshTooltip(accLabel, st)
	rad := float32(7)
	if isMac() {
		rad = 0
	}
	el := ui.ButtonBase(c.Key(key)).
		Size(size, size).
		Shrink(0).
		Radius(rad).
		AlignItems(ui.Center).
		Justify(ui.Center).
		Label(accLabel).
		Tooltip(tip)
	if !show {
		el = el.Opacity(0).IgnoreInput(true)
	} else {
		if st.phase != refreshRunning {
			el = el.Cursor(ui.CursorPointer)
		}
		fg := fgIdle
		var bg ui.Color
		hasBG := false
		switch st.phase {
		case refreshRunning:
			fg = accent
			bg = accent.Alpha(0.14)
			hasBG = true
		case refreshOK:
			fg = okCol
			bg = okCol.Alpha(0.14)
			hasBG = true
		case refreshError:
			fg = errCol
			bg = errCol.Alpha(0.14)
			hasBG = true
		default:
			if el.Hovered() {
				bg = bgHover
				hasBG = true
			}
		}
		if hasBG {
			el = el.Background(bg)
		}
		if isMac() {
			bc := ui.Transparent
			if st.phase != refreshIdle || el.Hovered() {
				bc = macShade
			}
			el = el.BorderWidth(1).BorderColor(bc)
		}
		if st.phase == refreshRunning && st.total > 1 {
			el = el.Draw(func(pt *ui.Painter, r ui.Rect) {
				paintRefreshRing(pt, r, st, accent)
			})
		}
		el.Children(func() {
			ic := ui.Icon(c, refreshIconSVG(st)).FontSize(iconSize).TextColor(fg)
			if st.phase == refreshRunning {
				deg := ic.Keyframes("spin", []ui.Keyframe{
					{At: 0, Value: 0},
					{At: 850 * time.Millisecond, Value: 360},
				}, ui.KeyframeOptions{Repeat: ui.RepeatForever})
				ic.Rotate(deg)
			}
		})
	}
	if el.Clicked() && st.phase != refreshRunning && onClick != nil {
		onClick()
	}
	return el
}

// refreshButton is the button that refreshes a script on home cards and covers:
// it spins while refreshing (with an outer ring when multi-step) and briefly
// displays check/error feedback upon completion.
// show false hides it until the card is hovered: it stays built, transparent and
// out of the pointer's way, so that assistive technology still finds it.
func (a *app) refreshButton(c *ui.Context, sc store.Script, show bool) {
	p := pal(c)
	a.refreshIconButton(c, "card-ref:"+sc.ID, sc.ID, tr("刷新数据"), 24, 14, show, p.hover, p.muted, p.accent, p.down, p.up, func() {
		a.refreshScript(sc)
	})
}

func (a *app) refreshGhostButton(c *ui.Context, key, id string, onClick func()) ui.Element {
	p := pal(c)
	return a.refreshIconButton(c, key, id, tr("刷新"), 26, 14, true, p.hover, p.muted, p.accent, p.down, p.up, onClick)
}

func (a *app) refreshTextButton(c *ui.Context, id string, onClick func()) ui.Element {
	p := pal(c)
	st := a.refreshStateOf(id)
	lbl := refreshButtonLabel(st)
	tip := refreshTooltip(tr("刷新数据"), st)
	w := refreshTextButtonWidth()

	if isMac() {
		face, fg := macFace, macBlack
		switch st.phase {
		case refreshRunning:
			fg = macAccent
		case refreshOK:
			fg = p.down
		case refreshError:
			fg = p.up
		}
		b := macBevel(ui.ButtonBase(c.Key("full-ref:"+id)).Width(w).Height(24).Shrink(0).Padding(0, 6).Gap(5).Radius(0).Background(face).AlignItems(ui.Center).Justify(ui.Center).Label(tr("刷新数据")).Tooltip(tip), st.phase == refreshRunning)
		if st.phase != refreshRunning {
			b = b.Cursor(ui.CursorPointer)
		}
		if st.phase == refreshRunning && st.total > 1 {
			b = b.Draw(func(pt *ui.Painter, r ui.Rect) {
				paintRefreshBar(pt, r, st, macAccent)
			})
		}
		b.Children(func() {
			ic := ui.Icon(c, refreshIconSVG(st)).FontSize(12).TextColor(fg)
			if st.phase == refreshRunning {
				deg := ic.Keyframes("spin", []ui.Keyframe{
					{At: 0, Value: 0},
					{At: 850 * time.Millisecond, Value: 360},
				}, ui.KeyframeOptions{Repeat: ui.RepeatForever})
				ic.Rotate(deg)
			}
			ui.Text(c, lbl).FontSize(fsCaption).TextColor(fg).FontFeatures("tnum").SingleLine().Ellipsis("…")
		})
		if b.Clicked() && st.phase != refreshRunning && onClick != nil {
			onClick()
		}
		return b
	}

	tk := theme.TokensOf(c)
	fg := p.text
	bg := p.card
	border := tk.Border
	switch st.phase {
	case refreshRunning:
		fg = p.accent
		bg = p.accent.Alpha(0.10)
		border = p.accent.Alpha(0.32)
	case refreshOK:
		fg = p.down
		bg = p.down.Alpha(0.10)
		border = p.down.Alpha(0.32)
	case refreshError:
		fg = p.up
		bg = p.up.Alpha(0.10)
		border = p.up.Alpha(0.32)
	}
	b := ui.ButtonBase(c.Key("full-ref:"+id)).
		Width(w).
		Height(24).
		Shrink(0).
		Padding(0, 6).
		Gap(5).
		Radius(tk.BorderRadiusSM).
		AlignItems(ui.Center).
		Justify(ui.Center).
		Label(tr("刷新数据")).
		Tooltip(tip)
	if st.phase == refreshIdle && b.Hovered() {
		bg = p.card.Mix(p.chip, 0.7)
	}
	if st.phase != refreshRunning {
		b = b.Cursor(ui.CursorPointer)
	}
	b = b.Background(bg).Border(1, border)
	if st.phase == refreshRunning && st.total > 1 {
		b = b.Draw(func(pt *ui.Painter, r ui.Rect) {
			paintRefreshBar(pt, r, st, p.accent)
		})
	}
	b.Children(func() {
		ic := ui.Icon(c, refreshIconSVG(st)).FontSize(12.5).TextColor(fg)
		if st.phase == refreshRunning {
			deg := ic.Keyframes("spin", []ui.Keyframe{
				{At: 0, Value: 0},
				{At: 850 * time.Millisecond, Value: 360},
			}, ui.KeyframeOptions{Repeat: ui.RepeatForever})
			ic.Rotate(deg)
		}
		ui.Text(c, lbl).FontSize(12).FontWeight(500).TextColor(fg).FontFeatures("tnum").SingleLine().Ellipsis("…")
	})
	if b.Clicked() && st.phase != refreshRunning && onClick != nil {
		onClick()
	}
	return b
}

// refreshAllHome starts a refresh for every script and .bar widget on the home grid.
func (a *app) refreshAllHome(scripts []store.Script) {
	for _, sc := range scripts {
		if sc.IsBar() {
			a.refreshBar(sc.ID)
			continue
		}
		a.refreshScript(sc)
	}
}

// anyHomeRefreshing reports whether any home script or .bar widget still has an in-flight refresh.
func (a *app) anyHomeRefreshing(scripts []store.Script) bool {
	for _, sc := range scripts {
		if a.isRefreshing(sc.ID) {
			return true
		}
	}
	return false
}
