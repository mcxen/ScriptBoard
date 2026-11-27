package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

const (
	homeViewOverviewID = "overview"
	homeViewNetworkID  = "network"
)

var viewIconNames = []string{
	"layout-grid",
	"activity",
	"globe",
	"chart-line",
	"gauge",
	"wallet",
	"calendar-days",
	"bot",
	"zap",
}

type homeViewEditor struct {
	open       bool
	creating   bool
	viewID     string
	name       string
	iconIdx    int
	search     string
	selected   map[string]bool
	note       string
	confirmDel string
}

func clampSizePreset(mode int) int {
	if mode < 0 || mode > 2 {
		return 1
	}
	return mode
}

func sizePresetKey(mode int) string {
	switch clampSizePreset(mode) {
	case 0:
		return "small"
	case 2:
		return "large"
	default:
		return "medium"
	}
}

func sizePresetLabel(mode int) string {
	switch clampSizePreset(mode) {
	case 0:
		return tr("小")
	case 2:
		return tr("大")
	default:
		return tr("中")
	}
}

func cloneCardBoxes(src map[string]store.CardBox) map[string]store.CardBox {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]store.CardBox, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func homeViewTitle(v store.HomeView) string {
	switch v.ID {
	case homeViewOverviewID:
		if v.Name == "" || v.Name == "总览" {
			return tr("总览")
		}
	case homeViewNetworkID:
		if v.Name == "" || v.Name == "网络排查" {
			return tr("网络排查")
		}
	}
	return tr(v.Name)
}

func homeViewIcon(v store.HomeView) string {
	if v.Icon != "" {
		return v.Icon
	}
	if v.All {
		return "layout-grid"
	}
	if v.ID == homeViewNetworkID {
		return "activity"
	}
	return "layout-grid"
}

func defaultNetworkView() store.HomeView {
	return store.HomeView{
		ID:       homeViewNetworkID,
		Name:     "网络排查",
		Icon:     "activity",
		Builtin:  true,
		Scripts:  []string{"sb-ping", "ip-dashboard"},
		SizeMode: 1,
		SizeBoxes: map[string]map[string]store.CardBox{
			"small": {
				"sb-ping":      {Span: 4, Height: 224},
				"ip-dashboard": {Span: 8, Height: 224},
			},
			"medium": {
				"sb-ping":      {Span: 4, Height: 432},
				"ip-dashboard": {Span: 8, Height: 432},
			},
			"large": {
				"sb-ping":      {Span: 12, Height: 400},
				"ip-dashboard": {Span: 12, Height: 320},
			},
		},
	}
}

func (a *app) ensureHomeViews() {
	changed := false
	if len(a.set.HomeViews) == 0 {
		ov := store.HomeView{
			ID:       homeViewOverviewID,
			Name:     "总览",
			Icon:     "layout-grid",
			All:      true,
			Builtin:  true,
			SizeMode: 1,
			Order:    append([]string(nil), a.set.CardOrder...),
		}
		if len(a.set.CardBoxes) > 0 {
			ov.SizeBoxes = map[string]map[string]store.CardBox{
				"medium": cloneCardBoxes(a.set.CardBoxes),
			}
		}
		a.set.HomeViews = append(a.set.HomeViews, ov)
		changed = true
	}
	if !a.set.NetworkViewSeeded {
		hasNet := false
		for _, v := range a.set.HomeViews {
			if v.ID == homeViewNetworkID {
				hasNet = true
				break
			}
		}
		if !hasNet {
			a.set.HomeViews = append(a.set.HomeViews, defaultNetworkView())
		}
		a.set.NetworkViewSeeded = true
		changed = true
	}
	idx := a.homeViewIndexByID(a.set.ActiveHomeView)
	if idx < 0 {
		idx = 0
		a.set.ActiveHomeView = a.set.HomeViews[0].ID
		changed = true
	}
	a.cardSize = clampSizePreset(a.set.HomeViews[idx].SizeMode)
	if changed {
		if err := a.store.SetSettings(a.set); err != nil {
			a.set = a.store.Settings()
		}
	}
}

func (a *app) homeViewIndexByID(id string) int {
	for i := range a.set.HomeViews {
		if a.set.HomeViews[i].ID == id {
			return i
		}
	}
	return -1
}

func (a *app) activeHomeViewIndex() int {
	if len(a.set.HomeViews) == 0 {
		a.ensureHomeViews()
	}
	if idx := a.homeViewIndexByID(a.set.ActiveHomeView); idx >= 0 {
		return idx
	}
	if len(a.set.HomeViews) > 0 {
		return 0
	}
	return -1
}

func (a *app) activeHomeView() *store.HomeView {
	idx := a.activeHomeViewIndex()
	if idx < 0 {
		return nil
	}
	return &a.set.HomeViews[idx]
}

// selectHomeView switches the active Home view tab and restores its remembered
// S/M/L size mode and per-component layout boxes.
func (a *app) selectHomeView(id string) {
	idx := a.homeViewIndexByID(id)
	if idx < 0 {
		return
	}
	a.set.ActiveHomeView = id
	v := a.set.HomeViews[idx]
	a.cardSize = clampSizePreset(v.SizeMode)
	preset := sizePresetKey(a.cardSize)
	if v.SizeBoxes != nil {
		a.set.CardBoxes = cloneCardBoxes(v.SizeBoxes[preset])
	} else {
		a.set.CardBoxes = nil
	}
	a.rz = nil
	a.viewEdit.open = false
	a.saveSettings()
}

// switchSizePreset switches the active view between S/M/L sub-views while
// preserving independent per-component CardBox sizes for each preset.
func (a *app) switchSizePreset(mode int) {
	mode = clampSizePreset(mode)
	a.cardSize = mode
	if idx := a.activeHomeViewIndex(); idx >= 0 {
		v := &a.set.HomeViews[idx]
		v.SizeMode = mode
		preset := sizePresetKey(mode)
		if v.SizeBoxes != nil {
			a.set.CardBoxes = cloneCardBoxes(v.SizeBoxes[preset])
		} else {
			a.set.CardBoxes = nil
		}
	}
	a.rz = nil
	a.saveSettings()
}

// resetActivePresetSizes clears custom per-card sizes in the current S/M/L preset only.
func (a *app) resetActivePresetSizes() {
	preset := sizePresetKey(a.cardSize)
	if idx := a.activeHomeViewIndex(); idx >= 0 {
		v := &a.set.HomeViews[idx]
		if v.SizeBoxes != nil {
			delete(v.SizeBoxes, preset)
		}
		if v.All {
			a.set.CardSizes = nil
		}
		a.set.CardBoxes = nil
	}
	a.rz = nil
	a.saveSettings()
	a.status = fmt.Sprintf(tr("已重置「%s」尺寸预设为默认布局"), sizePresetLabel(a.cardSize))
}

func (a *app) activeHomeScripts() []store.Script {
	v := a.activeHomeView()
	if v == nil || v.All {
		return a.orderedHome(a.homeScripts())
	}
	seen := make(map[string]bool, len(v.Scripts))
	out := make([]store.Script, 0, len(v.Scripts))
	for _, id := range v.Scripts {
		if seen[id] {
			continue
		}
		sc, ok := a.store.Script(id)
		if !ok || !a.supported(sc) {
			continue
		}
		seen[id] = true
		out = append(out, sc)
	}
	return out
}

func (a *app) viewTabMenuOpen(id string) *bool {
	if a.viewTabMenus == nil {
		a.viewTabMenus = map[string]*bool{}
	}
	b, ok := a.viewTabMenus[id]
	if !ok {
		b = new(bool)
		a.viewTabMenus[id] = b
	}
	return b
}

func (a *app) openNewHomeViewEditor() {
	a.rz = nil
	a.viewEdit = homeViewEditor{
		open:     true,
		creating: true,
		selected: map[string]bool{},
	}
}

func (a *app) openEditHomeViewEditor(id string) {
	if id == "" {
		if v := a.activeHomeView(); v != nil {
			id = v.ID
		}
	}
	idx := a.homeViewIndexByID(id)
	if idx < 0 {
		return
	}
	v := a.set.HomeViews[idx]
	sel := map[string]bool{}
	if v.All {
		for _, sc := range a.store.Scripts() {
			if sc.Home && a.supported(sc) {
				sel[sc.ID] = true
			}
		}
	} else {
		for _, sid := range v.Scripts {
			sel[sid] = true
		}
	}
	icIdx := slices.Index(viewIconNames, homeViewIcon(v))
	if icIdx < 0 {
		icIdx = 0
	}
	a.rz = nil
	a.viewEdit = homeViewEditor{
		open:     true,
		creating: false,
		viewID:   v.ID,
		name:     v.Name,
		iconIdx:  icIdx,
		selected: sel,
	}
}

func (a *app) uniqueHomeViewID(name string) string {
	base := slugOf(name)
	if base == "" || base == homeViewOverviewID {
		base = "view"
	}
	if a.homeViewIndexByID(base) < 0 {
		return base
	}
	for n := 2; ; n++ {
		cand := base + "-" + strconv.Itoa(n)
		if a.homeViewIndexByID(cand) < 0 {
			return cand
		}
	}
}

func (a *app) saveHomeViewEditor() {
	e := &a.viewEdit
	name := strings.TrimSpace(e.name)
	if name == "" {
		e.note = tr("请填写视图名称")
		return
	}
	all := a.store.Scripts()
	icName := viewIconNames[clampIndex(e.iconIdx, len(viewIconNames))]

	if e.creating {
		id := a.uniqueHomeViewID(name)
		var picked []string
		for _, sc := range all {
			if a.supported(sc) && e.selected[sc.ID] {
				picked = append(picked, sc.ID)
			}
		}
		v := store.HomeView{
			ID:       id,
			Name:     name,
			Icon:     icName,
			Scripts:  picked,
			SizeMode: 1,
		}
		a.set.HomeViews = append(a.set.HomeViews, v)
		a.set.ActiveHomeView = id
		a.cardSize = 1
		a.set.CardBoxes = nil
		a.rz = nil
		a.saveSettings()
		e.open = false
		return
	}

	idx := a.homeViewIndexByID(e.viewID)
	if idx < 0 {
		e.open = false
		return
	}
	v := &a.set.HomeViews[idx]
	v.Name = name
	v.Icon = icName
	if v.All {
		for _, sc := range all {
			if !a.supported(sc) {
				continue
			}
			want := e.selected[sc.ID]
			if sc.Home != want {
				sc.Home = want
				_ = a.store.SaveScript(sc)
			}
		}
		a.saveSettings()
		e.open = false
		return
	}

	// Preserve existing card ordering for items that stay selected, then append new picks.
	var next []string
	added := map[string]bool{}
	for _, sid := range v.Scripts {
		if e.selected[sid] && !added[sid] {
			added[sid] = true
			next = append(next, sid)
		}
	}
	for _, sc := range all {
		if a.supported(sc) && e.selected[sc.ID] && !added[sc.ID] {
			added[sc.ID] = true
			next = append(next, sc.ID)
		}
	}
	v.Scripts = next
	a.saveSettings()
	e.open = false
}

func (a *app) deleteHomeView(id string) {
	idx := a.homeViewIndexByID(id)
	if idx < 0 || a.set.HomeViews[idx].All {
		return
	}
	a.set.HomeViews = append(a.set.HomeViews[:idx], a.set.HomeViews[idx+1:]...)
	if a.set.ActiveHomeView == id {
		if len(a.set.HomeViews) > 0 {
			a.set.ActiveHomeView = a.set.HomeViews[0].ID
			cur := a.set.HomeViews[0]
			a.cardSize = clampSizePreset(cur.SizeMode)
			a.set.CardBoxes = cloneCardBoxes(cur.SizeBoxes[sizePresetKey(a.cardSize)])
		} else {
			a.set.ActiveHomeView = ""
			a.set.CardBoxes = nil
		}
	}
	a.rz = nil
	a.viewEdit.open = false
	a.viewEdit.confirmDel = ""
	a.saveSettings()
}

// homeViewTabs draws the switchable view pills plus quick "+" and "customize"
// actions, reused seamlessly in the fused desktop top bar and compact header.
func (a *app) homeViewTabs(c *ui.Context, p palette, inTitleBar bool) {
	activeID := ""
	if v := a.activeHomeView(); v != nil {
		activeID = v.ID
	}
	tabH := float32(28)
	if !inTitleBar {
		tabH = 30
	}
	iconBtnSize := tabH

	renderTabsRow := func() {
		for _, v := range a.set.HomeViews {
			v := v
			active := v.ID == activeID
			title := homeViewTitle(v)
			tab := ui.ButtonBase(c.Key("hview:"+v.ID)).
				Height(tabH).
				Shrink(0).
				Padding(0, 10).
				Gap(6).
				Radius(radControl).
				AlignItems(ui.Center).
				Justify(ui.Center).
				Cursor(ui.CursorPointer).
				Label(title)
			fg, weight := p.muted, 500
			if isMac() {
				bg, txtFg := macFace, macBlack
				if active {
					bg, txtFg = macBlack, macWhite
				}
				tab = macBevel(tab.Background(bg), active).Radius(0)
				fg = txtFg
				if active {
					weight = 650
				}
			} else if active {
				activeBG := p.card
				if p.dark {
					activeBG = ui.Hex("#26262A")
				}
				tab = tab.Background(activeBG).BorderWidth(1).BorderColor(p.border)
				fg, weight = p.text, 600
			} else if tab.Hovered() {
				tab = tab.Background(p.hover)
				fg = p.text
			}
			tab.Children(func() {
				ui.Icon(c, icon(homeViewIcon(v))).FontSize(13).TextColor(fg)
				ui.Text(c, title).FontSize(12.5).FontWeight(weight).TextColor(fg).NoWrap()
			})
			if tab.Clicked() && !active {
				a.selectHomeView(v.ID)
			}
			open := a.viewTabMenuOpen(v.ID)
			ui.ContextMenuBase(c, tab, open).Popup(func(panel ui.Element) {
				pop := panel.MinWidth(160).Padding(6).Gap(2).Background(p.card).Radius(12).BorderWidth(1).BorderColor(p.border)
				if !isMac() {
					pop = pop.Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.24))
				}
				pop.Children(func() {
					if popoverMenuRow(c, tr("编辑视图"), false, false).Clicked() {
						*open = false
						a.openEditHomeViewEditor(v.ID)
					}
					if popoverMenuRow(c, tr("新建视图"), false, false).Clicked() {
						*open = false
						a.openNewHomeViewEditor()
					}
					if !v.All {
						ui.Box(c).Height(1).Shrink(0).Background(p.hairline)
						if popoverMenuRow(c, tr("删除视图"), false, true).Clicked() {
							*open = false
							a.deleteHomeView(v.ID)
						}
					}
				})
			})
		}
	}

	ui.Row(c).AlignItems(ui.Center).Gap(6).MinWidth(0).Shrink(1).Children(func() {
		if inTitleBar {
			ui.ScrollHorizontal(c).Scrollbars(false).MinWidth(0).Shrink(1).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(4).Children(renderTabsRow)
			})
		} else {
			ui.Row(c).AlignItems(ui.Center).Gap(4).Wrap().Children(renderTabsRow)
		}

		addBtn := ui.ButtonBase(c.Key("hview-add")).
			Size(iconBtnSize, iconBtnSize).
			Shrink(0).
			Radius(radControl).
			AlignItems(ui.Center).
			Justify(ui.Center).
			Cursor(ui.CursorPointer).
			Label(tr("新建视图")).
			Tooltip(tr("新建视图"))
		if isMac() {
			addBtn = macBevel(addBtn.Background(macFace), a.viewEdit.open && a.viewEdit.creating).Radius(0)
		} else if a.viewEdit.open && a.viewEdit.creating {
			addBtn = addBtn.Background(p.card).BorderWidth(1).BorderColor(p.accent)
		} else if addBtn.Hovered() {
			addBtn = addBtn.Background(p.hover)
		}
		if addBtn.Children(func() {
			fg := p.muted
			if a.viewEdit.open && a.viewEdit.creating {
				fg = p.accent
			}
			ui.Icon(c, icon("plus")).FontSize(14).TextColor(fg)
		}).Clicked() {
			if a.viewEdit.open && a.viewEdit.creating {
				a.viewEdit.open = false
			} else {
				a.openNewHomeViewEditor()
			}
		}

		editActive := a.viewEdit.open && !a.viewEdit.creating
		cfgBtn := ui.ButtonBase(c.Key("hview-edit")).
			Size(iconBtnSize, iconBtnSize).
			Shrink(0).
			Radius(radControl).
			AlignItems(ui.Center).
			Justify(ui.Center).
			Cursor(ui.CursorPointer).
			Label(tr("管理组件")).
			Tooltip(tr("自定义当前视图组件"))
		if isMac() {
			cfgBtn = macBevel(cfgBtn.Background(macFace), editActive).Radius(0)
		} else if editActive {
			cfgBtn = cfgBtn.Background(p.card).BorderWidth(1).BorderColor(p.accent)
		} else if cfgBtn.Hovered() {
			cfgBtn = cfgBtn.Background(p.hover)
		}
		if cfgBtn.Children(func() {
			fg := p.muted
			if editActive {
				fg = p.accent
			}
			ui.Icon(c, icon("pencil-line")).FontSize(13.5).TextColor(fg)
		}).Clicked() {
			if editActive {
				a.viewEdit.open = false
			} else {
				a.openEditHomeViewEditor(activeID)
			}
		}
	})
}

func (a *app) homeViewEditorPanel(c *ui.Context, compact bool) {
	p := pal(c)
	e := &a.viewEdit
	if e.selected == nil {
		e.selected = map[string]bool{}
	}
	isAll := false
	title := tr("新建视图")
	if !e.creating {
		if idx := a.homeViewIndexByID(e.viewID); idx >= 0 {
			v := a.set.HomeViews[idx]
			isAll = v.All
			title = tr("编辑视图") + " · " + homeViewTitle(v)
		}
	}

	card(c.Key("home-view-editor")).Padding(18).Gap(14).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, title).FontSize(fsHeading).FontWeight(600).TextColor(p.text)
			ui.Spacer(c)
			if ghostIconBtn(c, icon("x"), tr("取消"), false).Clicked() {
				e.open = false
				e.confirmDel = ""
			}
		})
		if isAll {
			hint(c, tr("内置默认视图「总览」显示所有开启「显示在首页」的组件，可直接勾选调整。"))
		}

		ui.Row(c).AlignItems(ui.End).Gap(16).Wrap().Children(func() {
			ui.Column(c).Gap(6).Grow(1).MinWidth(200).Children(func() {
				label(c, tr("视图名称"))
				textInput(c.Key("hv-name-in"), &e.name, tr("例如：网络排查、工作台")).FillWidth()
			})
			ui.Column(c).Gap(6).Children(func() {
				label(c, tr("视图图标"))
				ui.Row(c).AlignItems(ui.Center).Gap(4).Wrap().Children(func() {
					for i, icName := range viewIconNames {
						i, icName := i, icName
						active := e.iconIdx == i
						b := ui.ButtonBase(c.Key("hv-ic:"+icName)).
							Size(30, 30).
							Radius(radControl).
							AlignItems(ui.Center).
							Justify(ui.Center).
							Cursor(ui.CursorPointer).
							Label(icName)
						fg := p.muted
						if active {
							b = b.Background(p.card).BorderWidth(1).BorderColor(p.accent)
							fg = p.accent
						} else if b.Hovered() {
							b = b.Background(p.hover)
							fg = p.text
						} else {
							b = b.Background(p.chip)
						}
						if b.Children(func() {
							ui.Icon(c, icon(icName)).FontSize(14).TextColor(fg)
						}).Clicked() {
							e.iconIdx = i
						}
					}
				})
			})
		})

		ui.Box(c).Height(1).FillWidth().Background(p.hairline)

		allScripts := a.store.Scripts()
		var supported []store.Script
		selectedCount := 0
		for _, sc := range allScripts {
			if a.supported(sc) {
				supported = append(supported, sc)
				if e.selected[sc.ID] {
					selectedCount++
				}
			}
		}

		ui.Row(c).AlignItems(ui.Center).Gap(8).Wrap().Children(func() {
			ui.Text(c, fmt.Sprintf(tr("包含组件（已选 %d 个）"), selectedCount)).
				FontSize(fsSmall).FontWeight(600).FontFeatures("tnum").TextColor(p.muted)
			ui.Spacer(c)
			textInput(c.Key("hv-search-in"), &e.search, tr("筛选组件名称或 ID")).Width(180)
			if !compact {
				if btn(c, tr("全选")).Clicked() {
					for _, sc := range supported {
						e.selected[sc.ID] = true
					}
				}
				if btn(c, tr("清空")).Clicked() {
					e.selected = map[string]bool{}
				}
			}
		})

		q := strings.ToLower(strings.TrimSpace(e.search))
		ui.Row(c).Gap(8).Wrap().Children(func() {
			for _, sc := range supported {
				sc := sc
				disp := a.scriptDisplayName(sc)
				if q != "" {
					hay := strings.ToLower(sc.ID + " " + sc.Name + " " + disp)
					if !strings.Contains(hay, q) {
						continue
					}
				}
				checked := e.selected[sc.ID]
				item := ui.ButtonBase(c.Key("hv-pick:"+sc.ID)).
					Height(34).
					Padding(0, 10).
					Gap(8).
					Radius(radControl + 1).
					AlignItems(ui.Center).
					Cursor(ui.CursorPointer).
					Label(disp)
				fg := p.text
				if checked {
					item = item.Background(p.accentSoft).BorderWidth(1).BorderColor(p.accent)
				} else if item.Hovered() {
					item = item.Background(p.hover).BorderWidth(1).BorderColor(p.border)
				} else {
					item = item.Background(p.card).BorderWidth(1).BorderColor(p.hairline)
					fg = p.muted
				}
				if item.Children(func() {
					dotCol := p.hairline
					if checked {
						dotCol = p.accent
					}
					ui.Box(c).Size(8, 8).Radius(4).Background(dotCol).Shrink(0)
					ui.Text(c, disp).FontSize(12.5).FontWeight(550).TextColor(fg).NoWrap()
					if sc.IsBar() {
						ui.Text(c, "BAR").FontSize(10).FontWeight(600).TextColor(p.faint).NoWrap()
					}
				}).Clicked() {
					e.selected[sc.ID] = !checked
				}
			}
		})

		if e.note != "" {
			ui.Text(c, e.note).FontSize(fsSmall).TextColor(p.up)
		}

		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			if !e.creating && !isAll {
				delLbl := tr("删除视图")
				if e.confirmDel == e.viewID {
					delLbl = tr("确认删除")
				}
				if btn(c, delLbl).Clicked() {
					if e.confirmDel == e.viewID {
						a.deleteHomeView(e.viewID)
						return
					}
					e.confirmDel = e.viewID
				}
			}
			ui.Spacer(c)
			if btn(c, tr("取消")).Clicked() {
				e.open = false
				e.confirmDel = ""
			}
			if paperOutlineBtn(c, tr("保存视图"), true).Clicked() {
				a.saveHomeViewEditor()
			}
		})
	})
}

func (a *app) scriptDisplayName(sc store.Script) string {
	if sc.Name != "" {
		return tr(sc.Name)
	}
	if sc.IsBar() {
		if sp, err := a.barSpecOf(sc); err == nil && sp.Title != "" {
			return tr(sp.Title)
		}
	} else if meta, err := a.metaOf(sc); err == nil && meta.Name != "" {
		return tr(meta.Name)
	}
	return sc.ID
}

// engineMetaCountString formats a quiet tabular-numeral widget count summary.
func formatWidgetCount(n int) string {
	return fmt.Sprintf(tr("%d 个组件"), n)
}

func unusedEngineImportGuard(_ engine.Meta) {}
