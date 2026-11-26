package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"

	"scriptboard/src/internal/bar"
	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

func (a *app) ensureBuiltinBars() error {
	type builtinDef struct {
		src  string
		home bool
	}
	defs := []builtinDef{
		{src: bar.BuiltinWeatherBar, home: false},
		{src: bar.BuiltinPingBar, home: !isMobile()},
	}
	for _, d := range defs {
		spec, err := bar.Parse(d.src)
		if err != nil {
			return fmt.Errorf("builtin bar: %w", err)
		}
		sc, ok := a.store.Script(spec.ID)
		if ok {
			changed := false
			if sc.Kind != store.KindBar || sc.Origin == "" {
				sc.Kind = store.KindBar
				if sc.Origin == "" {
					sc.Origin = store.OriginBuiltin
				}
				changed = true
			}
			// Auto-upgrade unmodified shipped sb-ping to latest v1.2.0 ($metrix.command.resolve support).
			if sc.ID == "sb-ping" && sc.Origin == store.OriginBuiltin && !strings.Contains(sc.Source, "$metrix.command.resolve") && strings.Contains(sc.Source, `const CMD_ID = "net-ping";`) {
				sc.Source = d.src
				sc.Name = spec.Title
				changed = true
			}
			if changed {
				if err := a.store.SaveScript(sc); err != nil {
					return err
				}
			}
			continue
		}
		if err := a.store.SaveScript(store.Script{
			ID:     spec.ID,
			Name:   spec.Title,
			Source: d.src,
			Home:   d.home,
			Origin: store.OriginBuiltin,
			Kind:   store.KindBar,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (a *app) barScripts() []store.Script {
	var out []store.Script
	for _, sc := range a.store.Scripts() {
		if sc.IsBar() {
			out = append(out, sc)
		}
	}
	return out
}

// menuManageScripts returns all .bar components followed by regular scripts
// that declare menubar: true in meta().
func (a *app) menuManageScripts() []store.Script {
	var bars, scripts []store.Script
	for _, sc := range a.store.Scripts() {
		if sc.IsBar() {
			bars = append(bars, sc)
			continue
		}
		if sc.IsTable() {
			continue
		}
		if meta, err := a.metaOf(sc); err == nil && meta.Menubar {
			scripts = append(scripts, sc)
		}
	}
	return append(bars, scripts...)
}

func (a *app) scriptDockPlacement(id string) store.BarPlacement {
	if a.set.BarPlacements != nil {
		if pl, ok := a.set.BarPlacements[id]; ok {
			return pl
		}
	}
	return store.BarPlacement{
		Enabled:     true,
		MenuBar:     a.menuBarOn(id),
		MenuBarMode: "menu",
		ShortcutKey: "Alt+W",
	}
}

func (a *app) menuItemActive(sc store.Script) bool {
	if sc.IsBar() {
		pl := a.barPlacement(sc.ID)
		return pl.Enabled && (pl.MenuBar || pl.LeftSidebar || pl.RightSidebar || pl.BottomBar || pl.Shortcut)
	}
	pl := a.scriptDockPlacement(sc.ID)
	return a.menuBarOn(sc.ID) || (pl.Enabled && (pl.LeftSidebar || pl.RightSidebar || pl.BottomBar || pl.Shortcut))
}

func barIconName(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "lucide:")
	s = strings.ReplaceAll(s, ".", "-")
	switch s {
	case "", "cloud-sun":
		return "cloud-sun"
	case "cloud-moon", "moon":
		return "cloud-sun"
	case "cloud", "cloud-fog":
		return "cloud-sun"
	case "cloud-rain", "cloud-drizzle", "cloud-snow":
		return "cloud-rain"
	case "cloud-lightning":
		return "zap"
	default:
		if validIconName.MatchString(s) && loadIcon(s) != nil {
			return s
		}
		return "sparkles"
	}
}

func (a *app) barSpecOf(sc store.Script) (bar.Spec, error) {
	if a.barSourceSig != nil && a.barSourceSig[sc.ID] == sc.Source {
		if sp, ok := a.barSpecs[sc.ID]; ok && sp.Script != "" {
			return sp, nil
		}
	}
	sp, err := bar.Parse(sc.Source)
	if err == nil {
		if a.barSpecs == nil {
			a.barSpecs = map[string]bar.Spec{}
		}
		if a.barSourceSig == nil {
			a.barSourceSig = map[string]string{}
		}
		a.barSpecs[sc.ID] = sp
		a.barSourceSig[sc.ID] = sc.Source
	}
	return sp, err
}

func (a *app) barMeta(sc store.Script) engine.Meta {
	sp, err := a.barSpecOf(sc)
	if err != nil {
		name := sc.Name
		if name == "" {
			name = sc.ID
		}
		return engine.Meta{Name: name, Icon: "sparkles"}
	}
	name := sp.Title
	if name == "" {
		name = sc.Name
	}
	return engine.Meta{
		Name:        name,
		Icon:        barIconName(sp.Icon),
		Version:     sp.Version,
		Description: sp.Description,
	}
}

func (a *app) barPlacement(id string) store.BarPlacement {
	if a.set.BarPlacements != nil {
		if pl, ok := a.set.BarPlacements[id]; ok {
			if pl.MenuBarMode == "" {
				pl.MenuBarMode = "panel"
			}
			return pl
		}
	}
	// Default: enabled and shown in Menu Bar as a floating panel.
	return store.BarPlacement{
		Enabled:     true,
		MenuBar:     true,
		MenuBarMode: "panel",
		ShortcutKey: "Alt+W",
	}
}

func (a *app) setBarPlacement(id string, pl store.BarPlacement) {
	if pl.MenuBarMode == "" {
		pl.MenuBarMode = "panel"
	}
	if a.set.BarPlacements == nil {
		a.set.BarPlacements = map[string]store.BarPlacement{}
	}
	a.set.BarPlacements[id] = pl
	if err := a.store.SetSettings(a.set); err != nil {
		a.set = a.store.Settings()
	}
	if !pl.Enabled && a.barOverlayID == id {
		a.barOverlayID = ""
	}
	a.syncBarMenuBar()
}

func (a *app) barInputs(id string, spec bar.Spec) map[string]string {
	var saved map[string]string
	if a.set.ScriptVars != nil {
		saved = a.set.ScriptVars[id]
	}
	return bar.MergeInputs(spec, saved)
}

func (a *app) setBarInput(id, key, val string) {
	if a.set.ScriptVars == nil {
		a.set.ScriptVars = map[string]map[string]string{}
	}
	if a.set.ScriptVars[id] == nil {
		a.set.ScriptVars[id] = map[string]string{}
	}
	a.set.ScriptVars[id][key] = val
	if err := a.store.SetSettings(a.set); err != nil {
		a.set = a.store.Settings()
	}
	if sc, ok := a.store.Script(id); ok {
		if sp, err := a.barSpecOf(sc); err == nil {
			if inst, ok := a.barInsts[id]; ok {
				inst.UpdateInputs(a.barInputs(id, sp), curLang())
			}
		}
	}
}

func (a *app) ensureBarInstance(sc store.Script) (*bar.Instance, bar.Spec, error) {
	if inst, ok := a.barInsts[sc.ID]; ok {
		if a.barSourceSig != nil && a.barSourceSig[sc.ID] == sc.Source {
			if sp, ok2 := a.barSpecs[sc.ID]; ok2 {
				return inst, sp, nil
			}
		}
		inst.Close()
		delete(a.barInsts, sc.ID)
	}
	sp, err := a.barSpecOf(sc)
	if err != nil {
		return nil, bar.Spec{}, err
	}
	id := sc.ID
	themeName := a.effectiveThemeName()
	inst, err := bar.NewInstance(sp, bar.Options{
		Client:     a.client,
		Inputs:     a.barInputs(id, sp),
		Lang:       curLang(),
		Theme:      themeName,
		Offline:    a.set.Offline,
		DataDir:    bar.ComponentDataDir(a.dataDir, id),
		Commands:   appCommandBridge{a},
		OnCopyText: func(s string) { a.post(func() { copyText(s) }) },
		OnUpdate: func(snap bar.Snapshot) {
			a.post(func() {
				a.onBarUpdate(id, snap)
				a.syncBarMenuBar()
			})
		},
		OnOpenURL: func(rawURL string) {
			if safeURL(rawURL) {
				go func(u string) { _ = mygo.Shell.OpenExternal(u) }(rawURL)
			}
		},
	})
	if err != nil {
		return nil, sp, err
	}
	a.barInsts[id] = inst
	return inst, sp, nil
}

func (a *app) onBarUpdate(id string, snap bar.Snapshot) {
	if !snap.Refreshing && a.refreshing[id] {
		a.finishRefresh(id, snap.Error == "", snap.Error)
	}
}

func (a *app) refreshBar(id string) {
	if a.isRefreshing(id) {
		return
	}
	a.beginRefresh(id, 1)
	sc, ok := a.store.Script(id)
	if !ok || !sc.IsBar() {
		a.finishRefresh(id, false, tr("找不到组件"))
		return
	}
	inst, _, err := a.ensureBarInstance(sc)
	if err != nil {
		a.finishRefresh(id, false, err.Error())
		return
	}
	inst.SetOffline(a.set.Offline)
	inst.Refresh()
	if a.win == nil && a.set.Offline {
		inst.WaitIdle(5 * time.Second)
		a.onBarUpdate(id, inst.Snapshot())
	}
}

func (a *app) invokeBarAction(id, actionID string) {
	a.invokeBarActionWithArg(id, actionID, nil)
}

func (a *app) invokeBarActionWithArg(id, actionID string, arg any) {
	if actionID == "" {
		return
	}
	if inst, ok := a.barInsts[id]; ok {
		inst.InvokeActionWithArg(actionID, arg)
	}
}

func (a *app) barCookieCount(id string) int {
	if inst, ok := a.barInsts[id]; ok && inst.Cookies() != nil {
		return inst.Cookies().Count()
	}
	jar, err := bar.NewCookieJar(bar.CookiesPath(a.dataDir, id))
	if err != nil {
		return 0
	}
	return jar.Count()
}

func (a *app) importBarCookies(id, rawURL, rawInput string) (int, error) {
	text := strings.TrimSpace(rawInput)
	if text == "" {
		return 0, fmt.Errorf("%s", tr("请输入 curl 命令或 Cookie 字符串"))
	}
	var jar *bar.CookieJar
	var err error
	if inst, ok := a.barInsts[id]; ok && inst.Cookies() != nil {
		jar = inst.Cookies()
	} else {
		jar, err = bar.NewCookieJar(bar.CookiesPath(a.dataDir, id))
		if err != nil {
			return 0, err
		}
	}
	var count int
	if len(text) >= 5 && strings.EqualFold(text[:5], "curl ") {
		_, count, err = jar.ImportCurl(text)
	} else {
		count, err = jar.SetHeader(rawURL, text)
	}
	if err != nil {
		return 0, err
	}
	if inst, ok := a.barInsts[id]; ok {
		inst.SetOffline(a.set.Offline)
		inst.Refresh()
	}
	return count, nil
}

func (a *app) clearBarCookies(id string) error {
	var err error
	if inst, ok := a.barInsts[id]; ok && inst.Cookies() != nil {
		err = inst.Cookies().Clear("")
		if err == nil {
			inst.SetOffline(a.set.Offline)
			inst.Refresh()
		}
		return err
	}
	jar, err := bar.NewCookieJar(bar.CookiesPath(a.dataDir, id))
	if err != nil {
		return err
	}
	return jar.Clear("")
}

func (a *app) importBarSource(raw string) (store.Script, error) {
	sp, err := bar.Parse(raw)
	if err != nil {
		return store.Script{}, err
	}
	id := sp.ID
	// Avoid clobbering a non-.bar script or an existing .bar with the same ID.
	if existing, exists := a.store.Script(id); exists {
		if existing.Kind == store.KindBar && existing.Origin != store.OriginBuiltin {
			// Replace user-imported .bar with same ID or suffix if builtin.
		} else {
			id = fmt.Sprintf("%s-%d", sp.ID, a.now().UnixNano())
		}
	}
	sc := store.Script{
		ID:     id,
		Name:   sp.Title,
		Source: raw,
		Home:   false,
		Origin: store.OriginUser,
		Kind:   store.KindBar,
	}
	if err := a.store.SaveScript(sc); err != nil {
		return store.Script{}, err
	}
	// Close previous instance if updating an existing imported ID.
	if old, ok := a.barInsts[sc.ID]; ok {
		old.Close()
		delete(a.barInsts, sc.ID)
	}
	if a.barSpecs == nil {
		a.barSpecs = map[string]bar.Spec{}
	}
	if a.barSourceSig == nil {
		a.barSourceSig = map[string]string{}
	}
	a.barSpecs[sc.ID] = sp
	a.barSourceSig[sc.ID] = raw
	a.barSelectedID = sc.ID
	return sc, nil
}

func (a *app) deleteBarComponent(id string) error {
	if inst, ok := a.barInsts[id]; ok {
		inst.Close()
		delete(a.barInsts, id)
	}
	if f, ok := a.barFlyouts[id]; ok {
		f.Close()
		delete(a.barFlyouts, id)
	}
	if t, ok := a.barTrays[id]; ok {
		t.Destroy()
		delete(a.barTrays, id)
		delete(a.barTraySig, id)
	}
	delete(a.barSpecs, id)
	delete(a.barSourceSig, id)
	delete(a.barFlyoutActive, id)
	if a.barOverlayID == id {
		a.barOverlayID = ""
	}
	if a.set.BarPlacements != nil {
		delete(a.set.BarPlacements, id)
		if err := a.store.SetSettings(a.set); err != nil {
			a.set = a.store.Settings()
		}
	}
	if dir := bar.ComponentDataDir(a.dataDir, id); dir != "" {
		_ = os.RemoveAll(dir)
	}
	if err := a.store.DeleteScript(id); err != nil {
		return err
	}
	if a.barSelectedID == id {
		a.barSelectedID = ""
		if list := a.barScripts(); len(list) > 0 {
			a.barSelectedID = list[0].ID
		}
	}
	a.syncBarMenuBar()
	return nil
}

func (a *app) openBarFileDialog() {
	if a.win == nil {
		return
	}
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: a.win,
			Title:  tr("导入 .bar 组件"),
			Filters: []mygo.FileFilter{
				{Name: "Bar Component", Extensions: []string{"bar", "xml"}},
			},
		})
		if err != nil || len(paths) == 0 {
			return
		}
		data, readErr := os.ReadFile(paths[0])
		a.post(func() {
			if readErr != nil {
				a.barImportErr = readErr.Error()
				return
			}
			sc, impErr := a.importBarSource(string(data))
			if impErr != nil {
				a.barImportErr = fmt.Sprintf("%s (%s): %v", tr("导入失败"), filepath.Base(paths[0]), impErr)
				return
			}
			a.barImportErr = ""
			a.barImportOpen = false
			a.barImportText = ""
			a.router.Push("/bar/" + sc.ID)
			a.syncBarMenuBar()
		})
	}()
}

func (a *app) barLoop() {
	last := map[string]time.Time{}
	for now := range time.Tick(15 * time.Second) {
		now := now
		a.post(func() { a.refreshBarsDue(now, last) })
	}
}

func (a *app) refreshBarsDue(now time.Time, last map[string]time.Time) {
	if a.set.Offline {
		return
	}
	for _, sc := range a.barScripts() {
		pl := a.barPlacement(sc.ID)
		if !pl.Enabled {
			continue
		}
		active := pl.MenuBar || pl.LeftSidebar || pl.RightSidebar || pl.BottomBar || pl.Shortcut || sc.Home
		if !active {
			continue
		}
		inst, sp, err := a.ensureBarInstance(sc)
		if err != nil {
			continue
		}
		every := bar.RefreshDuration(sp)
		if prev, ok := last[sc.ID]; !ok || now.Sub(prev) >= every {
			last[sc.ID] = now
			inst.Refresh()
		}
	}
}

func (a *app) closeBarInstances() {
	for id, f := range a.barFlyouts {
		f.Close()
		delete(a.barFlyouts, id)
	}
	for id, t := range a.barTrays {
		t.Destroy()
		delete(a.barTrays, id)
		delete(a.barTraySig, id)
	}
	for id, inst := range a.barInsts {
		inst.Close()
		delete(a.barInsts, id)
	}
}
