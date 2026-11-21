package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

// A card can show in the menu bar (macOS) or the notification area (Windows,
// Linux), as one icon of its own. Its script asks for it (meta menubar: true),
// and the user chooses which cards show there. On macOS the title, the
// headline, shows beside the icon; Windows and Linux show no title, so the
// headline is in the tooltip and at the top of the menu. The menu has the
// script's own items, then refresh, the window and quit.

// menuBarEvery is how often the cards in the bar run again. A result younger
// than a minute comes from the cache, so the network is asked about once a
// minute at most.
const menuBarEvery = time.Minute

// menuBarTitleMax is how many characters a title beside the icon may have, so
// that the bar keeps its width as the value changes.
const menuBarTitleMax = 12

// menuBarOffered says whether a card's right-click menu offers the menu bar:
// its script asked for it.
func menuBarOffered(m engine.Meta) bool { return m.Menubar }

// menuBarOn says whether the user has put the card in the menu bar.
func (a *app) menuBarOn(id string) bool {
	for _, x := range a.set.MenuBar {
		if x == id {
			return true
		}
	}
	return false
}

// menuBarActive says whether a card is in the bar. Then the app keeps running
// with its window hidden, so that the bar can show the window again.
func (a *app) menuBarActive() bool { return len(a.set.MenuBar) > 0 }

// toggleMenuBar puts a card in the menu bar, or takes it out, and keeps the
// choice. When the last card leaves, the window comes back.
func (a *app) toggleMenuBar(id string) {
	if a.menuBarOn(id) {
		kept := a.set.MenuBar[:0:0]
		for _, x := range a.set.MenuBar {
			if x != id {
				kept = append(kept, x)
			}
		}
		a.set.MenuBar = kept
	} else {
		a.set.MenuBar = append(a.set.MenuBar, id)
	}
	if err := a.store.SetSettings(a.set); err != nil {
		a.set = a.store.Settings()
	}
	a.syncMenuBar()
	if !a.menuBarActive() {
		a.showWindow()
	}
}

// addToMenuBar puts a card in the menu bar, if it is not there. The choice is
// kept, and the bar is brought up to date.
func (a *app) addToMenuBar(id string) {
	if a.menuBarOn(id) {
		return
	}
	a.set.MenuBar = append(a.set.MenuBar, id)
	if err := a.store.SetSettings(a.set); err != nil {
		a.set = a.store.Settings()
		return
	}
	a.syncMenuBar()
}

// menuBarCard finds the card in the bar: its script, and the run that gives its
// result, the one the home card shows. ok is false when the script is gone or
// its script no longer asks for the bar.
func (a *app) menuBarCard(id string) (store.Script, *runState, bool) {
	sc, ok := a.store.Script(id)
	if !ok {
		return store.Script{}, nil, false
	}
	meta, err := a.metaOf(sc)
	if err != nil || !meta.Menubar {
		return sc, nil, false
	}
	_, vals := a.viewOf(sc.ID, meta.Dims)
	return sc, a.ensure(a.runKeyOf(sc.ID, sc, meta, vals), sc, vals), true
}

// syncMenuBar shows exactly the cards the user chose: an icon for each, and no
// icon for a card taken out or no longer able to show there. It runs on the
// main thread, and does nothing before the window exists.
func (a *app) syncMenuBar() {
	if a.win == nil {
		return
	}
	if a.bars == nil {
		a.bars = map[string]*mygo.Tray{}
		a.barSig = map[string]string{}
	}
	type barCard struct {
		sc store.Script
		st *runState
	}
	var cards []barCard
	for _, id := range a.set.MenuBar {
		if sc, st, ok := a.menuBarCard(id); ok {
			cards = append(cards, barCard{sc, st})
		}
	}
	live := map[string]bool{}
	for _, c := range cards {
		live[c.sc.ID] = true
	}
	for id, t := range a.bars {
		if !live[id] {
			t.Destroy()
			delete(a.bars, id)
			delete(a.barSig, id)
		}
	}
	for _, c := range cards {
		a.showInBar(c.sc, c.st)
	}
}

// showInBar shows a card's icon with its title, tooltip and menu. An icon whose
// title, tooltip and menu have not changed is left alone. One whose click mode
// or title font changed is made again: a tray's font and its click are fixed
// when it is made.
func (a *app) showInBar(sc store.Script, st *runState) {
	title, tip := menuBarText(sc, st)
	click, mono := menuBarKind(st.res)
	iconShape, iconColor := menuBarIconSpec(st.res.MenuBar)
	kind := fmt.Sprintf("%s|%v|%s|%s\x00", click, mono, iconShape, iconColor)
	varSig := ""
	if meta, err := a.metaOf(sc); err == nil {
		_, vals := a.viewOf(sc.ID, meta.Dims)
		varSig = a.runKeyOf(sc.ID, sc, meta, vals)
	}
	sig := kind + fmt.Sprintf("%q|%q|%q|%q|%q|%v|%v", title, tip, st.res.Title, st.res.Sub, varSig, a.set.Offline, st.res.Menu)
	if t, ok := a.bars[sc.ID]; ok {
		if !strings.HasPrefix(a.barSig[sc.ID], kind) {
			t.Destroy()
			delete(a.bars, sc.ID)
			delete(a.barSig, sc.ID)
		} else {
			if a.barSig[sc.ID] == sig {
				return
			}
			t.SetTitle(title)
			t.SetToolTip(tip)
			if click == "menu" {
				t.SetMenu(a.menuBarMenu(sc, st))
			}
			a.barSig[sc.ID] = sig
			return
		}
	}
	iconBytes, isTemplate := menuBarIconFor(st.res.MenuBar)
	opts := mygo.TrayOptions{
		ID:              "card-" + sc.ID,
		Icon:            iconBytes,
		IconIsTemplate:  isTemplate,
		Title:           title,
		ToolTip:         tip,
		MonospacedTitle: mono,
	}
	if click == "menu" {
		opts.Menu = a.menuBarMenu(sc, st)
	}
	t, err := mygo.NewTray(opts)
	if err != nil {
		log.Printf("menu bar: %v", err)
		return
	}
	if click == "open" {
		// A click opens the card; the menu is on a right click.
		id := sc.ID
		t.OnClick(func() { a.openCard(id) })
		t.OnRightClick(func() {
			if card, run, ok := a.menuBarCard(id); ok {
				t.SetMenu(a.menuBarMenu(card, run))
				t.PopUpMenu()
				t.SetMenu(nil)
			}
		})
	}
	a.bars[sc.ID] = t
	a.barSig[sc.ID] = sig
}

// menuBarKind is how an item behaves, by what the script asked: what a click
// does ("menu" or "open") and whether its title is monospaced.
func menuBarKind(res engine.Result) (click string, mono bool) {
	click = "menu"
	if mb := res.MenuBar; mb != nil {
		if mb.Click == "open" {
			click = "open"
		}
		mono = mb.Mono
	}
	return click, mono
}

// openCard shows a card's full view in the window.
func (a *app) openCard(id string) {
	a.router.Push("/full/" + id)
	a.showWindow()
}

// menuBarText is the title beside a card's icon and its tooltip. The script may
// give its own (result.menubar). Otherwise the title is the headline, cut
// short, and the tooltip is the card's name with its summary.
func menuBarText(sc store.Script, st *runState) (title, tip string) {
	res := st.res
	name := sc.Name
	if res.Title != "" {
		name = res.Title
	}
	switch {
	case st.loading && res.Big == "":
		return "…", name + " · " + tr("加载中…")
	case res.Big == "" && st.err != nil:
		return "--", name + " · " + st.err.Error()
	}
	title, tip = res.Big, name
	if res.Sub != "" {
		tip += " · " + res.Sub
	}
	if mb := res.MenuBar; mb != nil {
		if mb.Title != "" {
			title = mb.Title
		}
		if mb.Tooltip != "" {
			tip = mb.Tooltip
		}
	}
	if title == "" {
		title = "--"
	}
	return cutTitle(title, menuBarTitleMax), tip
}

// cutTitle shortens s to most characters, ending in an ellipsis when it is cut.
func cutTitle(s string, most int) string {
	if utf8.RuneCountInString(s) <= most {
		return s
	}
	return string([]rune(s)[:most-1]) + "…"
}

// menuBarMenu is a card's menu in the bar: its name and summary, the script's
// items, automatic variable/dimension switchers (when declared), then refresh,
// the window and quit.
func (a *app) menuBarMenu(sc store.Script, st *runState) *mygo.Menu {
	name := sc.Name
	if st.res.Title != "" {
		name = st.res.Title
	}
	items := []*mygo.MenuItem{{Label: name, Subtitle: st.res.Sub, Disabled: true}}
	items = append(items, a.menuEntryItemsWithState(sc, st, st.res.Menu, 0)...)
	if meta, err := a.metaOf(sc); err == nil {
		items = append(items, a.menuVarDimItems(sc, meta)...)
	}
	return mygo.NewMenu(append(items,
		mygo.Separator(),
		&mygo.MenuItem{Label: tr("刷新"), Click: func(*mygo.MenuItem, *mygo.Window) { a.refreshScript(sc) }},
		&mygo.MenuItem{Label: tr("打开盯盯板"), Click: func(*mygo.MenuItem, *mygo.Window) { a.showWindow() }},
		mygo.Separator(),
		&mygo.MenuItem{Role: mygo.RoleQuit},
	))
}

// menuVarDimItems adds quick-switch submenus for a script's declared vars and
// dims so users can switch city/currency/period straight from the system tray.
func (a *app) menuVarDimItems(sc store.Script, meta engine.Meta) []*mygo.MenuItem {
	var out []*mygo.MenuItem
	if len(meta.Vars) > 0 {
		curVars := a.varsOf(sc, meta)
		for _, v := range meta.Vars {
			v := v
			if len(v.Options) == 0 {
				continue
			}
			var sub []*mygo.MenuItem
			for _, o := range v.Options {
				o := o
				sub = append(sub, &mygo.MenuItem{
					Label:   o.Label,
					Type:    mygo.MenuItemCheckbox,
					Checked: curVars[v.Key] == o.Value,
					Click: func(*mygo.MenuItem, *mygo.Window) {
						a.setVar(sc, v.Key, o.Value)
						a.syncMenuBar()
					},
				})
			}
			lbl := v.Label
			if lbl == "" {
				lbl = v.Key
			}
			out = append(out, &mygo.MenuItem{Label: lbl, Submenu: sub})
		}
	}
	if len(meta.Dims) > 0 {
		idx, _ := a.viewOf(sc.ID, meta.Dims)
		for di, d := range meta.Dims {
			di, d := di, d
			if len(d.Options) == 0 {
				continue
			}
			var sub []*mygo.MenuItem
			for oi, o := range d.Options {
				oi, o := oi, o
				sub = append(sub, &mygo.MenuItem{
					Label:   o.Label,
					Type:    mygo.MenuItemCheckbox,
					Checked: di < len(idx) && idx[di] == oi,
					Click: func(*mygo.MenuItem, *mygo.Window) {
						a.setDimIndex(sc.ID, meta.Dims, di, oi)
						a.syncMenuBar()
					},
				})
			}
			lbl := d.Label
			if lbl == "" {
				lbl = d.Key
			}
			out = append(out, &mygo.MenuItem{Label: lbl, Submenu: sub})
		}
	}
	if len(out) > 0 {
		return append([]*mygo.MenuItem{mygo.Separator()}, out...)
	}
	return nil
}

// setDimIndex selects option oi of dimension di for script id.
func (a *app) setDimIndex(id string, dims []engine.Dim, di, oi int) {
	if di < 0 || di >= len(dims) || oi < 0 || oi >= len(dims[di].Options) {
		return
	}
	idx, _ := a.viewOf(id, dims)
	idx[di] = oi
	a.dims[id] = idx
	a.invalidate(id)
}

// menuEntryItems are the items a script's entries make: a rule, a submenu (one
// level), an action, a web link, text to copy, or a line of text.
func (a *app) menuEntryItems(sc store.Script, entries []engine.MenuEntry, depth int) []*mygo.MenuItem {
	return a.menuEntryItemsWithState(sc, nil, entries, depth)
}

func (a *app) menuEntryItemsWithState(sc store.Script, st *runState, entries []engine.MenuEntry, depth int) []*mygo.MenuItem {
	var out []*mygo.MenuItem
	for _, e := range entries {
		if e.Separator {
			out = append(out, mygo.Separator())
			continue
		}
		it := &mygo.MenuItem{Label: e.Label, Subtitle: e.Sub, Badge: e.Badge, Disabled: e.Disabled}
		if e.Checked {
			it.Type = mygo.MenuItemCheckbox
			it.Checked = true
		}
		switch {
		case len(e.Items) > 0 && depth < 1:
			it.Submenu = a.menuEntryItemsWithState(sc, st, e.Items, depth+1)
		case e.Action != "":
			if click := a.menuActionWithState(sc, st, e.Action); click != nil {
				it.Click = click
			} else {
				it.Disabled = true
			}
		case e.URL != "" && safeURL(e.URL):
			link := e.URL
			if it.Label == "" {
				it.Label = link
			}
			it.Click = func(*mygo.MenuItem, *mygo.Window) {
				if err := mygo.Shell.OpenExternal(link); err != nil {
					log.Printf("menu bar: %v", err)
				}
			}
		case e.Copy != "":
			text := e.Copy
			if it.Label == "" {
				it.Label = text
			}
			it.Click = func(*mygo.MenuItem, *mygo.Window) { copyText(text) }
		default:
			it.Disabled = true
		}
		out = append(out, it)
	}
	return out
}

// menuAction is what an entry's action does, or nil for one the app does not know.
func (a *app) menuAction(sc store.Script, action string) func(*mygo.MenuItem, *mygo.Window) {
	return a.menuActionWithState(sc, nil, action)
}

func (a *app) menuActionWithState(sc store.Script, st *runState, action string) func(*mygo.MenuItem, *mygo.Window) {
	kind, key, val, ok := engine.ParseMenuAction(action)
	if !ok {
		return nil
	}
	switch kind {
	case "refresh":
		return func(*mygo.MenuItem, *mygo.Window) { a.refreshScript(sc) }
	case "refresh_all":
		return func(*mygo.MenuItem, *mygo.Window) {
			a.invalidateAll()
			a.syncMenuBar()
		}
	case "open":
		return func(*mygo.MenuItem, *mygo.Window) { a.openCard(sc.ID) }
	case "window":
		return func(*mygo.MenuItem, *mygo.Window) { a.showWindow() }
	case "settings":
		return func(*mygo.MenuItem, *mygo.Window) { a.goTo("/settings") }
	case "toggle_offline":
		return func(*mygo.MenuItem, *mygo.Window) {
			a.set.Offline = !a.set.Offline
			a.saveSettings()
			a.syncMenuBar()
		}
	case "copy_headline":
		headline := ""
		if st != nil {
			headline = st.res.Big
			if headline == "" && st.res.MenuBar != nil {
				headline = st.res.MenuBar.Title
			}
		}
		return func(*mygo.MenuItem, *mygo.Window) {
			if headline != "" {
				copyText(headline)
			}
		}
	case "set_var":
		return func(*mygo.MenuItem, *mygo.Window) {
			a.setVar(sc, key, val)
			a.syncMenuBar()
		}
	case "set_dim":
		return func(*mygo.MenuItem, *mygo.Window) {
			meta, err := a.metaOf(sc)
			if err != nil {
				return
			}
			wantNum, numErr := strconv.ParseFloat(val, 64)
			for di, d := range meta.Dims {
				if d.Key != key {
					continue
				}
				for oi, o := range d.Options {
					if o.Label == val || (numErr == nil && math.Abs(o.Value-wantNum) < 1e-9) {
						a.setDimIndex(sc.ID, meta.Dims, di, oi)
						a.syncMenuBar()
						return
					}
				}
			}
		}
	}
	return nil
}

// safeURL says whether a script's link may open: web addresses only.
func safeURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// showWindow brings the window back: out of the Dock or the taskbar, where
// closing it put it, or from where the menu bar's item hid it.
func (a *app) showWindow() {
	if a.win == nil {
		return
	}
	mygo.App.Focus()
	if a.win.IsMinimized() {
		a.win.Restore()
	}
	a.win.Show()
	a.win.Focus()
}

// keepRunning makes closing the main window minimize it, so that the app keeps
// running, with its menu bar items and its cards' runs. Only a quit ends it: the
// menu's Quit, Cmd-Q, the Quit of the menu bar items or of the settings, which
// all go through OnBeforeQuit.
func (a *app) keepRunning(w *mygo.Window) {
	w.OnClose(func(e *mygo.CloseEvent) { a.onMainClose(e, w.Minimize) })
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { a.quitting = true })
}

// onMainClose is what closing the main window does: the close is cancelled and
// the window minimized, unless the app is quitting, when the window closes.
func (a *app) onMainClose(e *mygo.CloseEvent, minimize func()) {
	if a.quitting {
		return
	}
	e.PreventDefault()
	minimize()
}

// quitApp ends the app: a variable, so that a test can stand in for it.
var quitApp = func() { mygo.App.Quit() }

// menuBarRan shows a card's new result in the bar, when the card is there.
func (a *app) menuBarRan(id string) {
	if a.win != nil && a.menuBarOn(id) {
		a.syncMenuBar()
	}
}

// menuBarLoop ticks every 15s and refreshes pinned cards whose custom or
// default refresh cadence has elapsed.
func (a *app) menuBarLoop() {
	last := map[string]time.Time{}
	for now := range time.Tick(15 * time.Second) {
		now := now
		a.post(func() { a.refreshMenuBarDue(now, last) })
	}
}

func (a *app) refreshMenuBarDue(now time.Time, last map[string]time.Time) {
	any := false
	for _, id := range a.set.MenuBar {
		every := menuBarEvery
		if sc, st, ok := a.menuBarCard(id); ok && st.res.MenuBar != nil && st.res.MenuBar.RefreshSec >= engine.MenuBarMinRefreshSec {
			every = time.Duration(st.res.MenuBar.RefreshSec) * time.Second
			_ = sc
		}
		if prev, ok := last[id]; !ok || now.Sub(prev) >= every {
			last[id] = now
			a.invalidate(id)
			any = true
		}
	}
	if any {
		a.syncMenuBar()
	}
}

// refreshMenuBar drops the bar's runs, so that each card runs again (its cache
// answers within a minute), and shows what it has.
func (a *app) refreshMenuBar() {
	for _, id := range a.set.MenuBar {
		a.invalidate(id)
	}
	a.syncMenuBar()
}

func menuBarIconSpec(mb *engine.MenuBar) (shape, hex string) {
	shape = "bars"
	if mb == nil {
		return shape, ""
	}
	switch mb.Icon {
	case "bars", "dot", "ring", "up", "down", "pulse":
		shape = mb.Icon
	}
	if hexColor.MatchString(mb.Color) {
		hex = strings.ToUpper(mb.Color)
	}
	return shape, hex
}

var (
	menuBarIconCacheMu sync.Mutex
	menuBarIconCache   = map[string][]byte{}
)

// menuBarIconFor renders a 32x32 PNG tray icon for mb. When mb has no custom
// icon or color, it returns the default black template icon.
func menuBarIconFor(mb *engine.MenuBar) ([]byte, bool) {
	shape, hex := menuBarIconSpec(mb)
	if shape == "bars" && hex == "" {
		return menuBarIcon(), true
	}
	key := shape + "|" + hex
	menuBarIconCacheMu.Lock()
	if b, ok := menuBarIconCache[key]; ok {
		menuBarIconCacheMu.Unlock()
		return b, hex == ""
	}
	menuBarIconCacheMu.Unlock()

	ink := color.NRGBA{A: 255}
	if hex != "" {
		r, _ := strconv.ParseUint(hex[1:3], 16, 8)
		g, _ := strconv.ParseUint(hex[3:5], 16, 8)
		b, _ := strconv.ParseUint(hex[5:7], 16, 8)
		ink = color.NRGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
	}
	const n = 32
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	set := func(x, y int) {
		if x >= 0 && x < n && y >= 0 && y < n {
			img.SetNRGBA(x, y, ink)
		}
	}
	switch shape {
	case "dot":
		for y := 0; y < n; y++ {
			for x := 0; x < n; x++ {
				dx, dy := float64(x)-15.5, float64(y)-15.5
				if dx*dx+dy*dy <= 9.5*9.5 {
					set(x, y)
				}
			}
		}
	case "ring":
		for y := 0; y < n; y++ {
			for x := 0; x < n; x++ {
				dx, dy := float64(x)-15.5, float64(y)-15.5
				d := math.Hypot(dx, dy)
				if d >= 7.0 && d <= 11.5 {
					set(x, y)
				}
			}
		}
	case "up":
		for y := 6; y <= 24; y++ {
			half := (y - 6) * 10 / 18
			for x := 16 - half; x <= 16+half; x++ {
				set(x, y)
			}
		}
	case "down":
		for y := 8; y <= 26; y++ {
			half := (26 - y) * 10 / 18
			for x := 16 - half; x <= 16+half; x++ {
				set(x, y)
			}
		}
	case "pulse":
		pts := [][2]int{{3, 18}, {9, 18}, {13, 8}, {18, 24}, {22, 14}, {28, 14}}
		for i := 0; i+1 < len(pts); i++ {
			x0, y0 := pts[i][0], pts[i][1]
			x1, y1 := pts[i+1][0], pts[i+1][1]
			steps := max(absInt(x1-x0), absInt(y1-y0)) * 2
			for s := 0; s <= steps; s++ {
				fx := x0 + (x1-x0)*s/max(steps, 1)
				fy := y0 + (y1-y0)*s/max(steps, 1)
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						set(fx+dx, fy+dy)
					}
				}
			}
		}
	default: // bars
		for i, h := range []int{12, 20, 16} {
			x0 := 5 + i*9
			for y := n - 4 - h; y < n-4; y++ {
				for x := x0; x < x0+6; x++ {
					set(x, y)
				}
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	out := buf.Bytes()
	menuBarIconCacheMu.Lock()
	menuBarIconCache[key] = out
	menuBarIconCacheMu.Unlock()
	return out, hex == ""
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// menuBarIconPNG is the icon of the cards in the bar: three bars, black on
// transparent, which the system tints for the bar it is on.
var menuBarIconPNG = sync.OnceValue(func() []byte {
	const n = 32
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	ink := color.NRGBA{A: 255}
	for i, h := range []int{12, 20, 16} {
		x0 := 5 + i*9
		for y := n - 4 - h; y < n-4; y++ {
			for x := x0; x < x0+6; x++ {
				img.SetNRGBA(x, y, ink)
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
})

// menuBarIcon is the PNG of a card's icon in the bar.
func menuBarIcon() []byte { return menuBarIconPNG() }
