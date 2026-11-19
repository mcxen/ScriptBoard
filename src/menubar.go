package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"net/url"
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
// title, tooltip and menu have not changed is left alone.
func (a *app) showInBar(sc store.Script, st *runState) {
	title, tip := menuBarText(sc, st)
	sig := fmt.Sprintf("%q|%q|%q|%q|%v", title, tip, st.res.Title, st.res.Sub, st.res.Menu)
	if t, ok := a.bars[sc.ID]; ok {
		if a.barSig[sc.ID] == sig {
			return
		}
		t.SetTitle(title)
		t.SetToolTip(tip)
		t.SetMenu(a.menuBarMenu(sc, st))
		a.barSig[sc.ID] = sig
		return
	}
	t, err := mygo.NewTray(mygo.TrayOptions{
		ID:             "card-" + sc.ID,
		Icon:           menuBarIcon(),
		IconIsTemplate: true,
		Title:          title,
		ToolTip:        tip,
		Menu:           a.menuBarMenu(sc, st),
	})
	if err != nil {
		log.Printf("menu bar: %v", err)
		return
	}
	a.bars[sc.ID] = t
	a.barSig[sc.ID] = sig
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
// items, then refresh, the window and quit.
func (a *app) menuBarMenu(sc store.Script, st *runState) *mygo.Menu {
	name := sc.Name
	if st.res.Title != "" {
		name = st.res.Title
	}
	items := []*mygo.MenuItem{{Label: name, Subtitle: st.res.Sub, Disabled: true}}
	for _, e := range st.res.Menu {
		switch {
		case e.Separator:
			items = append(items, mygo.Separator())
		case e.URL != "" && safeURL(e.URL):
			link, label := e.URL, e.Label
			if label == "" {
				label = link
			}
			items = append(items, &mygo.MenuItem{Label: label, Click: func(*mygo.MenuItem, *mygo.Window) {
				if err := mygo.Shell.OpenExternal(link); err != nil {
					log.Printf("menu bar: %v", err)
				}
			}})
		default:
			items = append(items, &mygo.MenuItem{Label: e.Label, Disabled: true})
		}
	}
	return mygo.NewMenu(append(items,
		mygo.Separator(),
		&mygo.MenuItem{Label: tr("刷新"), Click: func(*mygo.MenuItem, *mygo.Window) { a.refreshScript(sc) }},
		&mygo.MenuItem{Label: tr("打开盯盯板"), Click: func(*mygo.MenuItem, *mygo.Window) { a.showWindow() }},
		mygo.Separator(),
		&mygo.MenuItem{Role: mygo.RoleQuit},
	))
}

// safeURL says whether a script's link may open: web addresses only.
func safeURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// showWindow brings the window back after keepInBar hid it.
func (a *app) showWindow() {
	if a.win == nil {
		return
	}
	mygo.App.Focus()
	a.win.Show()
	a.win.Focus()
}

// keepInBar hides the window instead of closing it while cards are in the menu
// bar, so that the app keeps running and the bar can show the window again.
// Quitting still closes it.
func (a *app) keepInBar(w *mygo.Window) {
	w.OnClose(func(e *mygo.CloseEvent) {
		if a.quitting || !a.menuBarActive() {
			return
		}
		e.PreventDefault()
		w.Hide()
	})
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { a.quitting = true })
}

// menuBarRan shows a card's new result in the bar, when the card is there.
func (a *app) menuBarRan(id string) {
	if a.win != nil && a.menuBarOn(id) {
		a.syncMenuBar()
	}
}

// menuBarLoop asks the cards in the bar for fresh data once a minute, for the
// life of the app.
func (a *app) menuBarLoop() {
	for range time.Tick(menuBarEvery) {
		a.post(a.refreshMenuBar)
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
