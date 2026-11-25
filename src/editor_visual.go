package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/feedback"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/iconcat"
)

// visualEditor holds UI state for the script visual configuration editor.
type visualEditor struct {
	sourceSig     string
	cfg           *visualConfig
	tab           int // 0 内容, 1 样式, 2 更多 (for visual components)
	advOpen       bool
	filter        string // quick filter for the editor's left script list
	previewSize   int    // 0 中卡, 1 大卡
	iconOpen      bool
	iconCat       int
	tplOpen       bool
	metaDesc      string
	metaIcon      string
	metaMenubar   bool
	metaPlat      int // 0 all, 1 desktop, 2 mobile
	layoutTitle   int // 0 hide, 1 show
	layoutSummary int // 0 top, 1 hide
	layoutActions int // 0 hover, 1 always
	layoutLegend  int // 0 show, 1 hide
	layoutStats   int // 0 bottom, 1 hide
}

// syncVisualFromDraft keeps a.visual aligned with a.draft.Source whenever the
// user switches scripts or edits the raw source tab.
func (a *app) syncVisualFromDraft() {
	if a.visual.sourceSig == a.draft.Source {
		return
	}
	a.visual.sourceSig = a.draft.Source
	if cfg, ok := parseVisualConfig(a.draft.Source); ok {
		a.visual.cfg = cfg
		a.syncLayoutIndices(cfg.Layout)
		a.visual.metaIcon = cfg.Icon
		a.visual.metaDesc = cfg.Description
		a.visual.metaMenubar = cfg.Menubar
		switch cfg.Platform {
		case "desktop":
			a.visual.metaPlat = 1
		case "mobile":
			a.visual.metaPlat = 2
		default:
			a.visual.metaPlat = 0
		}
		return
	}
	a.visual.cfg = nil
	meta, err := a.metaOf(a.draft)
	if err == nil {
		a.visual.metaIcon = meta.Icon
		a.visual.metaDesc = meta.Description
		a.visual.metaMenubar = meta.Menubar
		a.visual.metaPlat = 0
		if len(meta.Platforms) == 1 && meta.Platforms[0] == "desktop" {
			a.visual.metaPlat = 1
		} else if len(meta.Platforms) == 1 && meta.Platforms[0] == "mobile" {
			a.visual.metaPlat = 2
		}
	}
	a.syncLayoutIndices(parseLayoutFromSource(a.draft.Source))
}

func (a *app) syncLayoutIndices(l engine.Layout) {
	a.visual.layoutTitle = boolIdx(l.Title == "show")
	a.visual.layoutSummary = boolIdx(l.Summary == "hide")
	a.visual.layoutActions = boolIdx(l.Actions == "always")
	a.visual.layoutLegend = boolIdx(l.Legend == "hide")
	a.visual.layoutStats = boolIdx(l.Stats == "hide")
}

func boolIdx(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (a *app) currentVisualLayout() engine.Layout {
	l := engine.Layout{
		Title:   "hide",
		Summary: "top",
		Actions: "hover",
		Legend:  "show",
		Stats:   "bottom",
	}
	if a.visual.layoutTitle == 1 {
		l.Title = "show"
	}
	if a.visual.layoutSummary == 1 {
		l.Summary = "hide"
	}
	if a.visual.layoutActions == 1 {
		l.Actions = "always"
	}
	if a.visual.layoutLegend == 1 {
		l.Legend = "hide"
	}
	if a.visual.layoutStats == 1 {
		l.Stats = "hide"
	}
	return l
}

// applyVisualConfig writes a.visual.cfg back to a.draft.Source and refreshes preview.
func (a *app) applyVisualConfig() {
	if a.visual.cfg == nil {
		return
	}
	cfg := *a.visual.cfg
	cfg.Layout = a.currentVisualLayout()
	if strings.TrimSpace(cfg.Name) != "" {
		a.draft.Name = cfg.Name
	} else {
		cfg.Name = a.draft.Name
	}
	src := renderVisualScript(cfg)
	a.draft.Source = src
	a.visual.sourceSig = src
	a.visual.cfg = &cfg
	a.visual.metaMenubar = cfg.Menubar
	a.invalidate(draftKey)
}

// applyTemplate replaces a.draft.Source with a visual component template.
func (a *app) applyTemplate(key string) {
	cfg := a.defaultVisualConfig(key)
	a.visual.cfg = &cfg
	a.visual.tplOpen = false
	a.visual.metaIcon = cfg.Icon
	a.visual.metaDesc = cfg.Description
	a.visual.metaMenubar = cfg.Menubar
	a.visual.metaPlat = 0
	a.syncLayoutIndices(cfg.Layout)
	a.draft.Name = cfg.Name
	a.applyVisualConfig()
	a.status = tr("已应用模板，修改后点「保存」生效")
}

// visualScriptEditor draws the visual configuration & visual editing form for
// the currently selected script.
//   - Declarative visual components (@visual-config) get three focused sub-tabs:
//     内容 (basic info + data items), 样式 (component style + card display),
//     and 更多 (platform, menu bar, template replacement).
//   - Code / built-in scripts get a clean Basic Info + Card Display form with
//     Advanced settings (platform / menu bar / switch to template) tucked away.
func (a *app) visualScriptEditor(c *ui.Context, compact bool) {
	a.syncVisualFromDraft()
	p := pal(c)
	ro := isPluginScript(a.draft.ID)
	if ro {
		notice(c, tr("本地插件在插件文件夹里修改，此处仅供查看"), feedback.Info)
	}

	if a.visual.cfg != nil && !ro {
		a.visualComponentEditor(c, compact)
		return
	}

	// Code / built-in script: show template gallery only when explicitly opened
	// (e.g. immediately after "+ 脚本" or via Advanced settings).
	if !ro && a.visual.tplOpen {
		ui.Column(c).Gap(8).Padding(12).Radius(radControl + 2).Background(p.chip).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				label(c, tr("选择组件模板快速开始"))
				ui.Spacer(c)
				if btn(c, tr("收起模板")).Clicked() {
					a.visual.tplOpen = false
				}
			})
			a.visualTemplateGrid(c, compact)
		})
	}

	a.visualBasicFields(c, ro)

	if !ro {
		a.visualLayoutControls(c)
		a.visualCodeAdvancedSection(c, compact)
		hint(c, tr("数据获取与计算逻辑请在「源码」标签页中编辑"))
	}
}

// visualComponentEditor renders the 3-tab editor (内容 / 样式 / 更多) for a
// declarative @visual-config component.
func (a *app) visualComponentEditor(c *ui.Context, compact bool) {
	p := pal(c)
	cfg := a.visual.cfg

	// Compact component type bar + compatible in-place swap chips.
	ui.Column(c).Gap(8).Padding(10, 12).Radius(radControl + 2).Background(p.chip).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(8).Wrap().Children(func() {
			chip(c, templateLabel(cfg.Type), p.accent, p.accentSoft)
			if targets := visualSwapTargets(cfg.Type); len(targets) > 0 {
				ui.Text(c, tr("原地转换")).FontSize(fsTiny).TextColor(p.muted)
				for _, t := range targets {
					t := t
					if btn(c.Key("vswap:"+t), tr("转为 ")+templateLabel(t)).Clicked() {
						a.swapVisualType(t)
					}
				}
			}
		})
	})

	if a.visual.tab < 0 || a.visual.tab > 2 {
		a.visual.tab = 0
	}
	choose(c, &a.visual.tab, tr("内容"), tr("样式"), tr("更多"))

	switch a.visual.tab {
	case 0:
		a.visualBasicFields(c, false)
		// Only show Big/Sub inputs for components that actually display them
		// (clock, countdown, and bento draw their own hero display).
		if cfg.Type != "clock" && cfg.Type != "countdown" && cfg.Type != "bento" {
			ui.Row(c).Gap(10).Wrap().Children(func() {
				ui.Column(c).Grow(1).MinWidth(140).Children(func() {
					field(c, tr("主数值（可选）"), "", func() {
						if textInput(c, &cfg.Big, tr("例如 78%")).Changed() {
							a.applyVisualConfig()
						}
					})
				})
				ui.Column(c).Grow(1).MinWidth(140).Children(func() {
					field(c, tr("摘要文字（可选）"), "", func() {
						if textInput(c, &cfg.Sub, tr("例如 本周目标")).Changed() {
							a.applyVisualConfig()
						}
					})
				})
			})
		}
		a.visualComponentContent(c, compact)
	case 1:
		a.visualComponentStyle(c, compact)
		a.visualLayoutControls(c)
	case 2:
		a.visualPlatformAndMenubar(c)
		ui.Column(c).Gap(8).Padding(12).Radius(radControl + 2).BorderWidth(1).BorderColor(p.hairline).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				label(c, tr("更换为其他模板"))
				ui.Spacer(c)
				toggle := tr("选择模板")
				if a.visual.tplOpen {
					toggle = tr("收起模板")
				}
				if btn(c, toggle).Clicked() {
					a.visual.tplOpen = !a.visual.tplOpen
				}
			})
			if a.visual.tplOpen {
				a.visualTemplateGrid(c, compact)
			}
		})
	}
}

func (a *app) visualTemplateGrid(c *ui.Context, compact bool) {
	p := pal(c)
	renderTile := func(tpl visualTemplate) {
		active := a.visual.cfg != nil && a.visual.cfg.Type == tpl.Key
		col := adaptAccentColor(parseColor(tpl.Color, p.accent), p.dark)
		tile := ui.ButtonBase(c).Grow(1).MinWidth(0).Padding(8, 10).Radius(radControl).Background(p.card).
			BorderWidth(1).BorderColor(p.hairline).Justify(ui.Start).AlignItems(ui.Center).Cursor(ui.CursorPointer)
		if active {
			tile = tile.BorderWidth(1.5).BorderColor(col).Background(col.Alpha(0.08))
		}
		if tile.Clicked() {
			a.applyTemplate(tpl.Key)
			a.visual.tab = 0
		}
		tile.Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(8).Grow(1).MinWidth(0).Children(func() {
				iconBadge(c, tpl.Icon, col, 26)
				ui.Column(c).Gap(1).Grow(1).MinWidth(0).Children(func() {
					name := tpl.Zh
					if curLang() == langEN {
						name = tpl.En
					}
					ui.Text(c, name).FontSize(fsSmall).FontWeight(600).NoWrap()
					if !compact {
						ui.Text(c, tr(tpl.Note)).FontSize(fsTiny).TextColor(p.muted).LineHeight(1.25)
					}
				})
			})
		})
	}
	cols := 2
	ui.Column(c).Gap(8).Children(func() {
		for i := 0; i < len(visualTemplates); i += cols {
			i := i
			ui.Row(c).Gap(8).AlignItems(ui.Stretch).Children(func() {
				for j := 0; j < cols; j++ {
					if i+j < len(visualTemplates) {
						renderTile(visualTemplates[i+j])
					} else {
						ui.Box(c).Grow(1).MinWidth(0)
					}
				}
			})
		}
	})
}

func (a *app) visualBasicFields(c *ui.Context, ro bool) {
	p := pal(c)
	ui.Row(c).AlignItems(ui.Start).Gap(12).Wrap().Children(func() {
		ui.Column(c).Grow(1).MinWidth(180).Children(func() {
			field(c, tr("名称"), "", func() {
				if textInput(c, &a.draft.Name, tr("脚本名称")).Changed() && !ro {
					if a.visual.cfg != nil {
						a.visual.cfg.Name = a.draft.Name
						a.applyVisualConfig()
					}
				}
			})
		})
		curIcon := a.visual.metaIcon
		if curIcon == "" {
			curIcon = "sparkles"
		}
		ui.Column(c).Gap(6).Shrink(0).Children(func() {
			label(c, tr("图标"))
			ui.Row(c).AlignItems(ui.Center).Gap(8).Height(32).Children(func() {
				iconBadge(c, curIcon, p.accent, 30)
				if !ro {
					lbl := tr("更换图标")
					if a.visual.iconOpen {
						lbl = tr("收起")
					}
					if btn(c, lbl).Clicked() {
						a.visual.iconOpen = !a.visual.iconOpen
					}
				}
			})
		})
	})

	if a.visual.iconOpen && !ro {
		a.iconPickerPanel(c)
	}

	homeSwitch(c, &a.draft.Home)
}

func (a *app) visualPlatformAndMenubar(c *ui.Context) {
	p := pal(c)
	ui.Row(c).AlignItems(ui.Start).Gap(16).Wrap().Children(func() {
		field(c, tr("适用平台"), "", func() {
			if choose(c, &a.visual.metaPlat, tr("全部设备"), tr("仅电脑"), tr("仅手机")) {
				a.applyPlatformChange()
			}
		})
		field(c, tr("允许放进菜单栏"), "", func() {
			ui.Row(c).AlignItems(ui.Center).Height(28).Children(func() {
				if toggle(c, &a.visual.metaMenubar) {
					a.applyMenubarChange()
				}
			})
		})
	})
	if cfg := a.visual.cfg; cfg != nil && cfg.Menubar {
		if cfg.MenuBarCfg == nil {
			cfg.MenuBarCfg = &engine.MenuBar{}
		}
		mb := cfg.MenuBarCfg
		mbChanged := false
		clickModes := []string{"menu", "open"}
		clickIdx := indexOf(clickModes, mb.Click, 0)
		ui.Column(c).Gap(8).Padding(10, 12).Radius(radControl).Background(p.chip).Children(func() {
			label(c, tr("菜单栏显示配置"))
			ui.Row(c).Gap(12).Wrap().AlignItems(ui.Start).Children(func() {
				ui.Column(c).Grow(1).MinWidth(140).Children(func() {
					field(c, tr("菜单栏短标题"), "", func() {
						if textInput(c, &mb.Title, tr("最多 12 字")).Changed() {
							mbChanged = true
						}
					})
				})
				field(c, tr("点击菜单栏图标"), "", func() {
					if choose(c, &clickIdx, tr("展开菜单"), tr("打开详情")) {
						mb.Click = clickModes[clickIdx]
						mbChanged = true
					}
				})
				field(c, tr("等宽数字"), "", func() {
					ui.Row(c).AlignItems(ui.Center).Height(28).Children(func() {
						if toggle(c, &mb.Mono) {
							mbChanged = true
						}
					})
				})
			})
		})
		if mbChanged {
			a.applyVisualConfig()
		}
	}
}

func (a *app) visualCodeAdvancedSection(c *ui.Context, compact bool) {
	p := pal(c)
	ui.Column(c).Gap(10).Padding(12).Radius(radControl + 2).BorderWidth(1).BorderColor(p.hairline).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			label(c, tr("高级设置"))
			ui.Text(c, tr("平台 / 菜单栏")).FontSize(fsTiny).TextColor(p.muted)
			ui.Spacer(c)
			lbl := tr("展开")
			if a.visual.advOpen {
				lbl = tr("收起")
			}
			if ui.Text(c, lbl).FontSize(fsSmall).TextColor(p.accent).Cursor(ui.CursorPointer).Clicked() {
				a.visual.advOpen = !a.visual.advOpen
			}
		})
		if a.visual.advOpen {
			a.visualPlatformAndMenubar(c)
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				if btn(c, tr("改用可视化模板…")).Clicked() {
					a.visual.tplOpen = !a.visual.tplOpen
				}
			})
		}
	})
	_ = compact
}

func (a *app) iconPickerPanel(c *ui.Context) {
	p := pal(c)
	cats := iconcat.Categories
	catLabels := make([]string, len(cats))
	for i, g := range cats {
		if curLang() == langEN {
			catLabels[i] = g.En
		} else {
			catLabels[i] = g.Zh
		}
	}
	if a.visual.iconCat < 0 || a.visual.iconCat >= len(cats) {
		a.visual.iconCat = 0
	}
	ui.Column(c).Gap(10).Padding(12).Radius(radControl + 2).Background(p.chip).Children(func() {
		ui.Row(c).Wrap().Gap(6).Children(func() {
			for i, lbl := range catLabels {
				i := i
				b := ui.ButtonBase(c).Padding(4, 10).Radius(99).Cursor(ui.CursorPointer)
				if i == a.visual.iconCat {
					b = b.Background(p.accent)
				} else {
					b = b.Background(p.card)
				}
				if b.Clicked() {
					a.visual.iconCat = i
				}
				b.Children(func() {
					t := ui.Text(c, lbl).FontSize(fsTiny).NoWrap()
					if i == a.visual.iconCat {
						t.TextColor(ui.Hex("#FFFFFF")).FontWeight(600)
					} else {
						t.TextColor(p.muted)
					}
				})
			}
		})
		ui.Row(c).Wrap().Gap(6).Children(func() {
			for _, it := range cats[a.visual.iconCat].Items {
				it := it
				sel := a.visual.metaIcon == it.Name
				b := ui.ButtonBase(c).Padding(6, 9).Radius(radControl).Background(p.card).BorderWidth(1).BorderColor(p.hairline).Cursor(ui.CursorPointer)
				if sel {
					b = b.BorderColor(p.accent).Background(p.accentSoft)
				}
				if b.Clicked() {
					a.applyIconChange(it.Name)
					a.visual.iconOpen = false
				}
				b.Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
						ui.Icon(c, icon(it.Name)).FontSize(15).TextColor(p.text)
						lbl := it.Zh
						if curLang() == langEN {
							lbl = it.En
						}
						ui.Text(c, lbl).FontSize(fsTiny).NoWrap()
					})
				})
			}
		})
	})
}

// draftWidgetType returns the component type of the currently edited draft,
// either from its visual config or from the latest preview run.
func (a *app) draftWidgetType() (typ string, hasStats bool) {
	if a.visual.cfg != nil {
		return a.visual.cfg.Type, false
	}
	meta, err := a.metaOf(a.draft)
	if err != nil {
		return "", false
	}
	_, vals := a.viewOf(draftKey, meta.Dims)
	if st, ok := a.runs[a.runKeyOf(draftKey, a.draft, meta, vals)]; ok && st != nil && !st.loading && st.err == nil {
		return engine.ResultWidgetType(st.res), len(st.res.Stats) > 0
	}
	return "", false
}

func (a *app) visualLayoutControls(c *ui.Context) {
	p := pal(c)
	wType, hasStats := a.draftWidgetType()
	showSummary := wType != "clock" && wType != "countdown" && wType != "calendar"
	showStats := hasStats

	ui.Column(c).Gap(10).Padding(12).Radius(radControl + 2).BorderWidth(1).BorderColor(p.hairline).Children(func() {
		label(c, tr("卡片显示"))
		changed := false
		ui.Row(c).Gap(14).Wrap().Children(func() {
			field(c, tr("卡片标题行"), "", func() {
				if choose(c, &a.visual.layoutTitle, tr("悬浮显示"), tr("固定显示")) {
					changed = true
				}
			})
			field(c, tr("操作按钮"), "", func() {
				if choose(c, &a.visual.layoutActions, tr("悬浮显示"), tr("常显")) {
					changed = true
				}
			})
			if showSummary {
				field(c, tr("摘要行"), "", func() {
					if choose(c, &a.visual.layoutSummary, tr("显示"), tr("隐藏")) {
						changed = true
					}
				})
			}
			if showStats {
				field(c, tr("统计块"), "", func() {
					if choose(c, &a.visual.layoutStats, tr("显示"), tr("隐藏")) {
						changed = true
					}
				})
			}
		})
		if changed {
			if a.visual.cfg != nil {
				a.applyVisualConfig()
			} else {
				a.draft.Source = patchScriptLayout(a.draft.Source, a.currentVisualLayout())
				a.visual.sourceSig = a.draft.Source
				a.invalidate(draftKey)
			}
		}
	})
}
