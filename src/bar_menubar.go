package main

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"log"
	"runtime"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/bar"
	"scriptboard/src/internal/store"
)

var (
	barTrayIconMu    sync.Mutex
	barTrayIconCache = map[string][]byte{}
)

func barTrayIconPNG(name string, template bool) []byte {
	ink := color.NRGBA{A: 255}
	if !template && runtime.GOOS != "darwin" {
		ink = color.NRGBA{R: 79, G: 124, B: 255, A: 255}
	}
	key := fmt.Sprintf("%s|%v|%02x%02x%02x", barIconName(name), template, ink.R, ink.G, ink.B)
	barTrayIconMu.Lock()
	if b, ok := barTrayIconCache[key]; ok {
		barTrayIconMu.Unlock()
		return b
	}
	barTrayIconMu.Unlock()

	svg := icon(barIconName(name))
	rgba := svg.Image(32, 32, ink)
	var buf bytes.Buffer
	if rgba != nil && png.Encode(&buf, rgba) == nil && buf.Len() > 0 {
		out := buf.Bytes()
		barTrayIconMu.Lock()
		barTrayIconCache[key] = out
		barTrayIconMu.Unlock()
		return out
	}
	return menuBarIcon()
}

func (a *app) barMenuBarActive() bool {
	for _, sc := range a.barScripts() {
		pl := a.barPlacement(sc.ID)
		if pl.Enabled && pl.MenuBar {
			return true
		}
	}
	return false
}

func (a *app) syncBarMenuBar() {
	if a.win == nil {
		return
	}
	if a.barTrays == nil {
		a.barTrays = map[string]*mygo.Tray{}
		a.barTraySig = map[string]string{}
	}
	if a.barFlyouts == nil {
		a.barFlyouts = map[string]*mygo.Window{}
	}

	live := map[string]bool{}
	for _, sc := range a.barScripts() {
		pl := a.barPlacement(sc.ID)
		if !pl.Enabled || !pl.MenuBar {
			continue
		}
		live[sc.ID] = true
		a.showBarInMenuBar(sc, pl)
	}

	for id, t := range a.barTrays {
		if !live[id] {
			t.Destroy()
			delete(a.barTrays, id)
			delete(a.barTraySig, id)
			if f, ok := a.barFlyouts[id]; ok {
				f.Close()
				delete(a.barFlyouts, id)
			}
		}
	}
}

func (a *app) showBarInMenuBar(sc store.Script, pl store.BarPlacement) {
	inst, sp, err := a.ensureBarInstance(sc)
	var snap bar.Snapshot
	if err == nil {
		snap = inst.Snapshot()
	}
	mode := pl.MenuBarMode
	if mode == "" {
		mode = "panel"
	}

	title := ""
	tip := sp.Title
	if tip == "" {
		tip = sc.Name
	}
	iconName := sp.Icon
	isTemplate := true
	var menuSig strings.Builder

	if snap.Tray != nil {
		title = cutTitle(snap.Tray.Title, menuBarTitleMax)
		if snap.Tray.Tooltip != "" {
			tip = snap.Tray.Tooltip
		}
		if snap.Tray.Icon != "" {
			iconName = snap.Tray.Icon
		}
		isTemplate = snap.Tray.IconTemplate
		for _, m := range snap.Tray.Menu {
			menuSig.WriteString(fmt.Sprintf("%v|%s|%v;", m.Separator, m.Label, m.Disabled))
		}
	}

	kind := mode + "\x00"
	sig := fmt.Sprintf("%s%q|%q|%q|%v|%s|%v", kind, title, tip, iconName, isTemplate, menuSig.String(), a.set.Offline)

	if t, ok := a.barTrays[sc.ID]; ok {
		if !strings.HasPrefix(a.barTraySig[sc.ID], kind) {
			t.Destroy()
			delete(a.barTrays, sc.ID)
			delete(a.barTraySig, sc.ID)
			if f, ok := a.barFlyouts[sc.ID]; ok {
				f.Close()
				delete(a.barFlyouts, sc.ID)
			}
		} else {
			if f, ok := a.barFlyouts[sc.ID]; ok && f.IsVisible() {
				f.Update(func() {})
			}
			if a.barTraySig[sc.ID] == sig {
				return
			}
			_ = t.SetIcon(barTrayIconPNG(iconName, isTemplate), isTemplate)
			t.SetTitle(title)
			t.SetToolTip(tip)
			if mode == "menu" || runtime.GOOS == "linux" {
				t.SetMenu(a.barTrayMenu(sc.ID, t, mode != "menu"))
			}
			a.barTraySig[sc.ID] = sig
			return
		}
	}

	opts := mygo.TrayOptions{
		ID:             "bar-" + sc.ID,
		Icon:           barTrayIconPNG(iconName, isTemplate),
		IconIsTemplate: isTemplate,
		Title:          title,
		ToolTip:        tip,
	}
	t, err := mygo.NewTray(opts)
	if err != nil {
		log.Printf("bar menu bar: %v", err)
		return
	}
	if mode == "menu" || runtime.GOOS == "linux" {
		// Linux AppIndicator only supports menus (no click callbacks).
		t.SetMenu(a.barTrayMenu(sc.ID, t, mode != "menu"))
	} else {
		id := sc.ID
		t.OnClick(func() { a.toggleBarFlyout(id, t) })
		t.OnRightClick(func() {
			if f, ok := a.barFlyouts[id]; ok {
				f.Close()
			}
			t.SetMenu(a.barTrayMenu(id, t, false))
			defer t.SetMenu(nil)
			t.PopUpMenu()
		})
	}
	a.barTrays[sc.ID] = t
	a.barTraySig[sc.ID] = sig
}

func (a *app) barTrayMenu(id string, t *mygo.Tray, includeOpenPanel bool) *mygo.Menu {
	var items []*mygo.MenuItem
	sc, ok := a.store.Script(id)
	if !ok {
		return mygo.NewMenu(nil)
	}
	sp, _ := a.barSpecOf(sc)
	heading := sp.Title
	if heading == "" {
		heading = sc.Name
	}
	if inst, ok := a.barInsts[id]; ok {
		snap := inst.Snapshot()
		if snap.Tray != nil && snap.Tray.Tooltip != "" {
			heading = snap.Tray.Tooltip
		}
		items = append(items, &mygo.MenuItem{Label: heading, Disabled: true})
		if includeOpenPanel && t != nil {
			items = append(items, &mygo.MenuItem{
				Label: tr("打开浮动面板"),
				Click: func(*mygo.MenuItem, *mygo.Window) {
					a.toggleBarFlyout(id, t)
				},
			})
		}
		if snap.Tray != nil && len(snap.Tray.Menu) > 0 {
			items = append(items, mygo.Separator())
			for _, m := range snap.Tray.Menu {
				if m.Separator {
					items = append(items, mygo.Separator())
					continue
				}
				aid := m.ActionID
				items = append(items, &mygo.MenuItem{
					Label:    m.Label,
					Disabled: m.Disabled || aid == "",
					Click: func(*mygo.MenuItem, *mygo.Window) {
						a.invokeBarAction(id, aid)
					},
				})
			}
		}
	} else {
		items = append(items, &mygo.MenuItem{Label: heading, Disabled: true})
		if includeOpenPanel && t != nil {
			items = append(items, &mygo.MenuItem{
				Label: tr("打开浮动面板"),
				Click: func(*mygo.MenuItem, *mygo.Window) {
					a.toggleBarFlyout(id, t)
				},
			})
		}
	}

	items = append(items,
		mygo.Separator(),
		&mygo.MenuItem{
			Label: tr("刷新"),
			Click: func(*mygo.MenuItem, *mygo.Window) {
				a.refreshBar(id)
			},
		},
		&mygo.MenuItem{
			Label: tr("打开组件详情"),
			Click: func(*mygo.MenuItem, *mygo.Window) {
				a.router.Push("/bar/" + id)
				a.showWindow()
			},
		},
		&mygo.MenuItem{
			Label: tr("显示主窗口"),
			Click: func(*mygo.MenuItem, *mygo.Window) {
				a.showWindow()
			},
		},
		mygo.Separator(),
		&mygo.MenuItem{
			Label: tr("退出"),
			Click: func(*mygo.MenuItem, *mygo.Window) {
				quitApp()
			},
		},
	)
	return mygo.NewMenu(items)
}

func (a *app) toggleBarFlyout(id string, t *mygo.Tray) {
	if a.barFlyouts == nil {
		a.barFlyouts = map[string]*mygo.Window{}
	}
	if f, ok := a.barFlyouts[id]; ok {
		f.Close()
		return
	}
	if a.recentlyClosedFlyout(id) {
		return
	}

	panelW, panelH := 300, 512
	material := ""
	if inst, ok := a.barInsts[id]; ok {
		material = inst.Spec().Material
		if p := inst.Snapshot().Panel; p != nil {
			if w, ok := propFloat(p.Props, "preferredWidth"); ok && w > 0 {
				panelW = int(w)
			}
			if h, ok := propFloat(p.Props, "preferredHeight"); ok && h > 0 {
				panelH = int(h)
			}
		}
	}

	pl := a.barPlacement(id)
	a.barFlyouts[id] = a.openTrayPanelWindowWithMaterial(id, t, pl.Display, panelW, panelH, material, func(c *ui.Context) {
		applySkin(c)
		c.Root().Background(ui.Transparent)
		sc, ok := a.store.Script(id)
		if !ok {
			return
		}
		inst, sp, err := a.ensureBarInstance(sc)
		if err != nil {
			return
		}
		th := barThemeFor(c, barThemeAuto)
		themeStr := "dark"
		if !th.dark {
			themeStr = "light"
		}
		inst.SetAppearance(themeStr, c.Vibrancy())
		snap := inst.Snapshot()
		onAct := func(aid string) { a.invokeBarAction(id, aid) }
		onActArg := func(aid string, arg any) { a.invokeBarActionWithArg(id, aid, arg) }
		if c.Vibrancy() && snap.Panel != nil && !strings.EqualFold(sp.Material, "solid") {
			ui.Column(c).Fill().Children(func() {
				renderBarNodeInteractive(c, snap.Panel, th, true, onAct, onActArg)
			})
			return
		}
		renderBarPanelInteractive(c, snap, th, float32(panelW), sp.Material, onAct, onActArg)
	})
}
