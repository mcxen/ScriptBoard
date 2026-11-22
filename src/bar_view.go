package main

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"scriptboard/src/internal/bar"
	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

var (
	barThemeAutoSVG  = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="9"/><path d="M12 3v18"/><path d="M12 8l5-3"/><path d="M12 13l7-4"/><path d="M12 18l5-3"/></svg>`))
	barThemeLightSVG = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="4"/><path d="M12 2v2m0 16v2M4.93 4.93l1.41 1.41m11.32 11.32l1.41 1.41M2 12h2m16 0h2M6.34 17.66l-1.41 1.41M19.07 4.93l-1.41 1.41"/></svg>`))
	barThemeDarkSVG  = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20.985 12.486a9 9 0 1 1-9.473-9.472c.405-.022.617.46.402.803a6 6 0 0 0 8.268 8.268c.344-.215.825-.004.803.401"/></svg>`))
	barMoreSVG       = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/><circle cx="5" cy="12" r="1"/></svg>`))
	barDockLeftSVG   = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="18" height="18" x="3" y="3" rx="2"/><path d="M9 3v18"/></svg>`))
	barDockRightSVG  = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="18" height="18" x="3" y="3" rx="2"/><path d="M15 3v18"/></svg>`))
	barDockBottomSVG = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="18" height="18" x="3" y="3" rx="2"/><path d="M3 15h18"/></svg>`))
	barKeySVG        = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="20" height="14" x="2" y="5" rx="2"/><path d="M6 10h.01M10 10h.01M14 10h.01M18 10h.01M8 14h8"/></svg>`))
	barMenuBarSVG    = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="18" height="18" x="3" y="3" rx="2"/><path d="M3 8h18"/></svg>`))
	barDisplaySVG    = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect width="20" height="14" x="2" y="3" rx="2"/><path d="M8 21h8m-4-4v4"/></svg>`))
)

var barMoreOpen bool

func (a *app) selectBarComponent(id string) {
	a.barSelectedID = id
	barMoreOpen = false
}

func (a *app) barView(c *ui.Context, compact bool) {
	p := pal(c)
	all := a.menuManageScripts()
	if a.barSelectedID == "" && len(all) > 0 {
		a.barSelectedID = all[0].ID
	}
	var selected *store.Script
	for i := range all {
		if all[i].ID == a.barSelectedID {
			selected = &all[i]
			break
		}
	}
	if selected == nil && len(all) > 0 {
		selected = &all[0]
		a.barSelectedID = selected.ID
	}

	// Filter component list by search query.
	q := strings.ToLower(strings.TrimSpace(a.barSearch))
	var filtered []store.Script
	for _, sc := range all {
		if q == "" {
			filtered = append(filtered, sc)
			continue
		}
		meta, _ := a.metaOf(sc)
		hay := strings.ToLower(sc.Name + " " + tr(sc.Name) + " " + sc.ID + " " + meta.Name + " " + meta.Description)
		if sc.IsBar() {
			if sp, err := a.barSpecOf(sc); err == nil {
				hay += " " + strings.ToLower(sp.Title+" "+sp.Category+" "+sp.Description)
			}
		}
		if strings.Contains(hay, q) {
			filtered = append(filtered, sc)
		}
	}

	renderSelected := func(isCompact bool) {
		if selected == nil {
			return
		}
		if selected.IsBar() {
			a.barWorkspace(c, p, *selected, isCompact)
		} else {
			a.scriptMenuWorkspace(c, p, *selected, isCompact)
		}
	}

	if compact {
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).FillWidth().Padding(pad(true)).Gap(14).Children(func() {
				a.barRail(c, p, filtered, len(all), true)
				a.barImportBox(c, p)
				renderSelected(true)
			})
		})
		return
	}

	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		// Left component rail
		ui.Column(c).
			Width(220).
			FillHeight().
			Background(p.sidebar).
			BorderWidth(0, 1, 0, 0).
			BorderColor(p.border).
			Padding(14, 12).
			Gap(10).
			Children(func() {
				a.barRail(c, p, filtered, len(all), false)
			})

		// Main right workspace
		ui.Column(c).Grow(1).MinWidth(0).FillHeight().Children(func() {
			if a.barImportOpen {
				ui.Column(c).Padding(16, 20, 0, 20).Children(func() {
					a.barImportBox(c, p)
				})
			}
			if selected == nil {
				ui.Column(c).Grow(1).Center().Padding(24).Children(func() {
					empty(c, "sparkles", tr("还没有组件"), tr("点击左上角 + 导入 .bar 组件文件。"))
				})
				return
			}
			renderSelected(false)
		})
	})
}

func (a *app) barRail(c *ui.Context, p palette, filtered []store.Script, total int, compact bool) {
	ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
		textInput(c, &a.barSearch, tr("搜索")).Grow(1).MinWidth(0)
		if action.ActionIcon(c, icon("plus"), action.ActionIconProps{
			Size:  theme.Small,
			Title: tr("导入 .bar 组件"),
		}).Clicked() {
			a.barImportOpen = !a.barImportOpen
			a.barImportErr = ""
		}
	})

	ui.Row(c).AlignItems(ui.Center).Padding(4, 6).Gap(6).Children(func() {
		ui.Text(c, tr("概览")).FontSize(fsTiny).FontWeight(600).TextColor(p.muted)
		ui.Spacer(c)
		ui.Text(c, fmt.Sprintf("%d", total)).FontSize(fsTiny).TextColor(p.faint)
	})

	listBody := func() {
		for _, sc := range filtered {
			sc := sc
			meta, _ := a.metaOf(sc)
			active := sc.ID == a.barSelectedID
			iconName := meta.Icon
			title := tr(sc.Name)
			if sc.IsBar() {
				if sp, err := a.barSpecOf(sc); err == nil {
					if sp.Title != "" {
						title = sp.Title
					}
					iconName = barIconName(sp.Icon)
				}
			} else if meta.Name != "" {
				title = meta.Name
			}

			row := ui.Row(c.Key("bar-item:"+sc.ID)).
				FillWidth().
				AlignItems(ui.Center).
				Gap(8).
				Padding(8, 10).
				Radius(radControl).
				Cursor(ui.CursorPointer)
			switch {
			case active:
				row = row.Background(p.rowSel)
			case row.Hovered():
				row = row.Background(p.hover)
			}
			if row.Children(func() {
				ui.Icon(c, icon(iconName)).FontSize(15).TextColor(p.text)
				ui.Text(c, title).FontSize(fsSmall).FontWeight(500).SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
				dotCol := p.faint.Alpha(0.45)
				if a.menuItemActive(sc) {
					dotCol = p.accent
				}
				ui.Box(c).Size(7, 7).Radius(3.5).Background(dotCol)
			}).Clicked() {
				a.selectBarComponent(sc.ID)
				a.router.Push("/bar/" + sc.ID)
			}
		}
		if len(filtered) == 0 {
			ui.Text(c, tr("没有匹配的组件")).FontSize(fsTiny).TextColor(p.faint).Padding(8)
		}
	}

	if compact {
		ui.Column(c).FillWidth().Gap(4).Children(listBody)
		return
	}
	ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Children(func() {
		ui.Column(c).FillWidth().Gap(3).Children(listBody)
	})
}

func (a *app) barImportBox(c *ui.Context, p palette) {
	if !a.barImportOpen {
		return
	}
	card(c).FillWidth().Padding(14).Gap(10).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, tr("导入 .bar 组件")).FontSize(fsBody).FontWeight(600)
			ui.Spacer(c)
			if a.win != nil {
				if btn(c, tr("选择 .bar 文件")).Clicked() {
					a.openBarFileDialog()
				}
			}
		})
		hint(c, tr("粘贴 .bar XML 内容，或选择本地 .bar 文件导入。"))
		textArea(c, &a.barImportText, "<bar id=\"...\" title=\"...\">...</bar>", 6).Font("monospace").FontSize(fsSmall)
		if a.barImportErr != "" {
			ui.Text(c, a.barImportErr).FontSize(fsSmall).TextColor(p.up)
		}
		ui.Row(c).Gap(8).Children(func() {
			if primaryBtn(c, tr("导入")).Clicked() {
				sc, err := a.importBarSource(a.barImportText)
				if err != nil {
					a.barImportErr = err.Error()
				} else {
					a.barImportErr = ""
					a.barImportText = ""
					a.barImportOpen = false
					a.router.Push("/bar/" + sc.ID)
					a.syncBarMenuBar()
				}
			}
			if btn(c, tr("关闭")).Clicked() {
				a.barImportOpen = false
				a.barImportErr = ""
			}
		})
	})
}

func (a *app) barWorkspace(c *ui.Context, p palette, sc store.Script, compact bool) {
	inst, sp, instErr := a.ensureBarInstance(sc)
	var snap bar.Snapshot
	if instErr == nil && inst != nil {
		snap = inst.Snapshot()
	} else if instErr != nil {
		snap.Error = instErr.Error()
	}
	pl := a.barPlacement(sc.ID)

	title := sp.Title
	if title == "" {
		title = sc.Name
	}
	ver := sp.Version
	if ver == "" {
		ver = "1.0.0"
	}
	subParts := []string{}
	if cat := barCategoryLabel(sp.Category); cat != "" {
		subParts = append(subParts, cat)
	}
	subParts = append(subParts, "v"+ver)
	subParts = append(subParts, fmt.Sprintf(tr("更新于 %s"), barRelativeTime(snap.UpdatedAt, a.now())))
	subtitle := strings.Join(subParts, " · ")

	// Top header bar
	ui.Column(c).
		FillWidth().
		BorderWidth(0, 0, 1, 0).
		BorderColor(p.border).
		Padding(14, 20).
		Gap(8).
		Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Children(func() {
				iconBadge(c, barIconName(sp.Icon), p.accent, 34)
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, title).FontSize(fsBodyLarge).FontWeight(650).SingleLine().Ellipsis("…")
					ui.Text(c, subtitle).FontSize(fsTiny).TextColor(p.muted).SingleLine().Ellipsis("…")
				})

				enabled := pl.Enabled
				if sw(c, &enabled).Changed() {
					pl.Enabled = enabled
					a.setBarPlacement(sc.ID, pl)
				}
				if action.ActionIcon(c, icon("refresh-cw"), action.ActionIconProps{
					Size:  theme.Small,
					Title: tr("刷新"),
				}).Clicked() {
					a.refreshBar(sc.ID)
				}
				moreBtn := ui.Row(c).Size(28, 28).Radius(7).Center().Cursor(ui.CursorPointer).Tooltip(tr("更多操作"))
				if moreBtn.Hovered() || barMoreOpen {
					moreBtn = moreBtn.Background(p.hover)
				}
				if moreBtn.Children(func() {
					ui.Icon(c, barMoreSVG).FontSize(15).TextColor(p.muted)
				}).Clicked() {
					barMoreOpen = !barMoreOpen
				}
			})

			if barMoreOpen {
				ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(8).Padding(6, 0, 0, 0).Children(func() {
					if btn(c, tr("在组件管理中编辑")).Clicked() {
						barMoreOpen = false
						a.router.Push("/edit/" + sc.ID)
					}
					homeLbl := tr("显示在首页")
					if sc.Home {
						homeLbl = tr("从首页移除")
					}
					if btn(c, homeLbl).Clicked() {
						sc.Home = !sc.Home
						_ = a.store.SaveScript(sc)
						barMoreOpen = false
					}
					if btn(c, tr("导出 .bar 源码")).Clicked() {
						a.barImportOpen = true
						a.barImportText = sc.Source
						a.barImportErr = ""
						barMoreOpen = false
					}
					if a.originOf(sc) != store.OriginBuiltin {
						if btn(c, tr("删除组件")).Clicked() {
							_ = a.deleteBarComponent(sc.ID)
							barMoreOpen = false
						}
					}
				})
			}
		})

	// Workspace body: Preview stage + Right inspector
	previewStage := func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Text(c, tr("预览")).FontSize(fsSmall).FontWeight(500).TextColor(p.muted)
			ui.Spacer(c)
			a.barThemeSwitcher(c, p)
		})

		stageBG := ui.Hex("#0B1110")
		if !p.dark {
			stageBG = ui.Hex("#EEF1F5")
		}
		stage := ui.Column(c).
			FillWidth().
			Background(stageBG).
			Radius(12).
			BorderWidth(1).
			BorderColor(p.border).
			Padding(24).
			Center()
		if !compact {
			stage = stage.Grow(1).MinHeight(560)
		}
		stage.Children(func() {
			th := barThemeFor(c, a.barPreviewTheme)
			renderBarPanel(c, snap, th, 300, func(aid string) {
				a.invokeBarAction(sc.ID, aid)
			})
			if !pl.Enabled {
				ui.Text(c, tr("组件已停用，开启右上角开关后在对应位置显示")).
					FontSize(fsTiny).TextColor(p.muted).Padding(10, 0, 0, 0)
			}
		})
	}

	inspector := func() {
		a.barPlacementSection(c, p, sc.ID, pl)
		if len(sp.Inputs) > 0 {
			a.barInputsSection(c, p, sc.ID, sp)
		}
		a.barSourceSection(c, p, sc, sp)
	}

	if compact {
		ui.Column(c).FillWidth().Padding(14).Gap(16).Children(func() {
			previewStage()
			inspector()
		})
		return
	}

	ui.Row(c).Grow(1).MinHeight(0).FillWidth().Padding(16, 20).Gap(18).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).FillHeight().Gap(10).Children(previewStage)
		ui.Scroll(c).Width(292).FillHeight().Children(func() {
			ui.Column(c).FillWidth().Gap(14).Padding(0, 0, 16, 0).Children(inspector)
		})
	})
}

func (a *app) barThemeSwitcher(c *ui.Context, p palette) {
	items := []struct {
		mode int
		svg  *ui.SVG
		tip  string
	}{
		{barThemeAuto, barThemeAutoSVG, tr("跟随系统")},
		{barThemeLight, barThemeLightSVG, tr("浅色")},
		{barThemeDark, barThemeDarkSVG, tr("深色")},
	}
	ui.Row(c).
		Padding(2).
		Radius(14).
		Background(p.chip).
		BorderWidth(1).
		BorderColor(p.border).
		Gap(2).
		Children(func() {
			for _, it := range items {
				it := it
				sel := a.barPreviewTheme == it.mode
				b := ui.Row(c.Key("bar-theme-"+it.tip)).
					Size(24, 22).
					Radius(11).
					Center().
					Cursor(ui.CursorPointer).
					Tooltip(it.tip)
				fg := p.muted
				if sel {
					b = b.Background(p.rowSel)
					fg = p.text
				} else if b.Hovered() {
					b = b.Background(p.hover)
				}
				if b.Children(func() {
					ui.Icon(c, it.svg).FontSize(13).TextColor(fg)
				}).Clicked() {
					a.barPreviewTheme = it.mode
				}
			}
		})
}

func (a *app) barPlacementSection(c *ui.Context, p palette, id string, pl store.BarPlacement) {
	ui.Column(c).FillWidth().Gap(6).Children(func() {
		ui.Text(c, tr("显示位置")).FontSize(fsSmall).FontWeight(500).TextColor(p.muted)
		card(c).FillWidth().Children(func() {
			// Menu bar row with mode selector
			a.barPlacementRow(c, p, barMenuBarSVG, tr("菜单栏"), true, func() {
				modeIdx := 0
				if pl.MenuBarMode == "menu" {
					modeIdx = 1
				}
				if dropdown(c, &modeIdx, tr("浮动面板"), tr("下拉菜单")) {
					if modeIdx == 1 {
						pl.MenuBarMode = "menu"
					} else {
						pl.MenuBarMode = "panel"
					}
					a.setBarPlacement(id, pl)
				}
				on := pl.MenuBar
				if sw(c, &on).Changed() {
					pl.MenuBar = on
					a.setBarPlacement(id, pl)
				}
			})

			a.barDisplayRow(c, p, id, pl, nil)

			a.barPlacementRow(c, p, barDockLeftSVG, tr("左侧栏"), true, func() {
				on := pl.LeftSidebar
				if sw(c, &on).Changed() {
					pl.LeftSidebar = on
					a.setBarPlacement(id, pl)
				}
			})

			a.barPlacementRow(c, p, barDockRightSVG, tr("右侧栏"), true, func() {
				on := pl.RightSidebar
				if sw(c, &on).Changed() {
					pl.RightSidebar = on
					a.setBarPlacement(id, pl)
				}
			})

			a.barPlacementRow(c, p, barDockBottomSVG, tr("底部侧栏"), true, func() {
				on := pl.BottomBar
				if sw(c, &on).Changed() {
					pl.BottomBar = on
					a.setBarPlacement(id, pl)
				}
			})

			a.barPlacementRow(c, p, barKeySVG, tr("快捷键"), false, func() {
				chip(c, "Alt+W", p.muted, p.chip)
				on := pl.Shortcut
				if sw(c, &on).Changed() {
					pl.Shortcut = on
					a.setBarPlacement(id, pl)
				}
			})
		})
		a.barDisplayNote(c, p)
	})
}

func (a *app) barDisplayRow(c *ui.Context, p palette, id string, pl store.BarPlacement, extra func()) {
	dispIdx := 0
	switch pl.Display {
	case "internal":
		dispIdx = 1
	case "external":
		dispIdx = 2
	}
	a.barPlacementRow(c, p, barDisplaySVG, tr("显示器"), true, func() {
		if dropdown(c, &dispIdx, tr("跟随点击"), tr("内接显示器"), tr("外接显示器")) {
			switch dispIdx {
			case 1:
				pl.Display = "internal"
			case 2:
				pl.Display = "external"
			default:
				pl.Display = ""
			}
			a.setBarPlacement(id, pl)
			if extra != nil {
				extra()
			}
		}
	})
}

func (a *app) barDisplayNote(c *ui.Context, p palette) {
	if a.win == nil {
		return
	}
	if sum := displaysSummary(connectedDisplays()); sum != "" {
		ui.Text(c, sum).FontSize(fsTiny).TextColor(p.faint)
	}
}

func (a *app) barPlacementRow(c *ui.Context, p palette, svg *ui.SVG, lbl string, divider bool, right func()) {
	r := ui.Row(c).FillWidth().AlignItems(ui.Center).Padding(10, 12).Gap(8)
	if divider {
		r = r.BorderWidth(0, 0, 1, 0).BorderColor(p.hairline)
	}
	r.Children(func() {
		ui.Icon(c, svg).FontSize(15).TextColor(p.muted)
		ui.Text(c, lbl).FontSize(fsSmall).FontWeight(500)
		ui.Spacer(c)
		right()
	})
}

func (a *app) barInputsSection(c *ui.Context, p palette, id string, sp bar.Spec) {
	saved := a.barInputs(id, sp)
	if a.barInputDrafts == nil {
		a.barInputDrafts = map[string]map[string]string{}
	}
	if a.barInputDrafts[id] == nil {
		a.barInputDrafts[id] = map[string]string{}
	}
	drafts := a.barInputDrafts[id]

	ui.Column(c).FillWidth().Gap(6).Children(func() {
		ui.Text(c, tr("输入参数")).FontSize(fsSmall).FontWeight(500).TextColor(p.muted)
		card(c).FillWidth().Padding(12).Gap(12).Children(func() {
			for _, in := range sp.Inputs {
				in := in
				curVal := saved[in.Name]
				ui.Column(c.Key("bar-in:"+id+":"+in.Name)).FillWidth().Gap(5).Children(func() {
					ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(6).Children(func() {
						ui.Text(c, in.Label).FontSize(fsSmall).FontWeight(500)
						if in.Required {
							ui.Text(c, tr("必填")).FontSize(fsTiny).TextColor(p.faint)
						}
					})

					switch in.Type {
					case "select":
						if len(in.Options) > 0 {
							selIdx := 0
							labels := make([]string, len(in.Options))
							for i, opt := range in.Options {
								labels[i] = opt.Label
								if opt.Value == curVal {
									selIdx = i
								}
							}
							if dropdown(c, &selIdx, labels...) {
								a.setBarInput(id, in.Name, in.Options[selIdx].Value)
							}
						}
					case "boolean":
						on := strings.EqualFold(curVal, "true") || curVal == "1"
						if sw(c, &on).Changed() {
							v := "false"
							if on {
								v = "true"
							}
							a.setBarInput(id, in.Name, v)
						}
					default:
						if _, ok := drafts[in.Name]; !ok {
							drafts[in.Name] = curVal
						}
						dVal := drafts[in.Name]
						dirty := dVal != curVal
						ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(6).Children(func() {
							var ti ui.Element
							ui.Column(c).Grow(1).MinWidth(0).Children(func() {
								if in.Type == "password" {
									ti = passwordInput(c, &dVal, in.Default).FillWidth()
								} else {
									ti = textInput(c, &dVal, in.Default).FillWidth()
								}
							})
							drafts[in.Name] = dVal
							if ti.Submitted() {
								a.setBarInput(id, in.Name, dVal)
							}
							if dirty {
								if primaryBtn(c, tr("应用")).Clicked() {
									a.setBarInput(id, in.Name, dVal)
								}
							}
						})
					}

					if in.Help != "" {
						ui.Text(c, in.Help).FontSize(fsTiny).TextColor(p.faint)
					}
				})
			}
		})
	})
}

func (a *app) barSourceSection(c *ui.Context, p palette, sc store.Script, sp bar.Spec) {
	origin := originLabel(a.originOf(sc))
	ver := sp.Version
	if ver == "" {
		ver = "1.0.0"
	}

	ui.Column(c).FillWidth().Gap(6).Children(func() {
		ui.Text(c, tr("来源")).FontSize(fsSmall).FontWeight(500).TextColor(p.muted)
		card(c).FillWidth().Padding(12).Gap(8).Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(6).Children(func() {
				ui.Text(c, fmt.Sprintf("%s · v%s", origin, ver)).FontSize(fsSmall).FontWeight(500)
				ui.Spacer(c)
				if sp.License != "" {
					chip(c, sp.License, p.muted, p.chip)
				}
			})
			if sp.Description != "" {
				ui.Text(c, sp.Description).FontSize(fsTiny).TextColor(p.muted)
			}
			if len(sp.Permissions) > 0 {
				var hosts []string
				for _, perm := range sp.Permissions {
					if u, err := url.Parse(perm); err == nil && u.Host != "" {
						hosts = append(hosts, u.Host)
					} else {
						hosts = append(hosts, perm)
					}
				}
				ui.Text(c, tr("联网权限：")+strings.Join(hosts, ", ")).FontSize(fsTiny).TextColor(p.faint)
			}
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Padding(2, 0, 0, 0).Children(func() {
				ui.Spacer(c)
				if safeURL(sp.Homepage) {
					if action.ActionIcon(c, icon("globe"), action.ActionIconProps{
						Size:  theme.Small,
						Title: sp.Homepage,
					}).Clicked() {
						_ = mygo.Shell.OpenExternal(sp.Homepage)
					}
				}
				if action.ActionIcon(c, icon("refresh-cw"), action.ActionIconProps{
					Size:  theme.Small,
					Title: tr("刷新"),
				}).Clicked() {
					a.refreshBar(sc.ID)
				}
			})
		})
	})
}

func barCategoryLabel(cat string) string {
	switch strings.ToLower(strings.TrimSpace(cat)) {
	case "":
		return ""
	case "weather":
		return tr("天气")
	case "finance":
		return tr("财经")
	case "system":
		return tr("系统")
	case "developer", "dev":
		return tr("开发")
	case "productivity":
		return tr("效率")
	default:
		return cat
	}
}

func barRelativeTime(updated, now time.Time) string {
	if updated.IsZero() {
		return tr("未更新")
	}
	d := now.Sub(updated)
	if d < time.Minute {
		return tr("刚刚")
	}
	mins := int(d.Minutes())
	if mins < 60 {
		return fmt.Sprintf(tr("%d 分钟前"), mins)
	}
	hrs := int(d.Hours())
	return fmt.Sprintf(tr("%d 小时前"), hrs)
}

// dockedBarComponents returns enabled .bar components and menubar scripts whose
// placement matches pick.
func (a *app) dockedBarComponents(pick func(store.BarPlacement) bool) []store.Script {
	var out []store.Script
	for _, sc := range a.menuManageScripts() {
		var pl store.BarPlacement
		if sc.IsBar() {
			pl = a.barPlacement(sc.ID)
		} else {
			pl = a.scriptDockPlacement(sc.ID)
		}
		if pl.Enabled && pick(pl) {
			out = append(out, sc)
		}
	}
	return out
}

func (a *app) scriptMenuWorkspace(c *ui.Context, p palette, sc store.Script, compact bool) {
	meta, metaErr := a.metaOf(sc)
	idx, vals := a.viewOf(sc.ID, meta.Dims)
	st := &runState{err: metaErr}
	if metaErr == nil {
		st = a.ensure(a.runKeyOf(sc.ID, sc, meta, vals), sc, vals)
	}
	pl := a.scriptDockPlacement(sc.ID)
	inBar := a.menuBarOn(sc.ID)

	title := tr(sc.Name)
	if meta.Name != "" {
		title = meta.Name
	}
	ver := meta.Version
	if ver == "" {
		ver = "1.0"
	}
	subtitle := strings.Join([]string{tr("脚本"), "v" + ver, originLabel(a.originOf(sc))}, " · ")

	ui.Column(c).
		FillWidth().
		BorderWidth(0, 0, 1, 0).
		BorderColor(p.border).
		Padding(14, 20).
		Gap(8).
		Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Children(func() {
				iconBadge(c, meta.Icon, p.accent, 34)
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, title).FontSize(fsBodyLarge).FontWeight(650).SingleLine().Ellipsis("…")
					ui.Text(c, subtitle).FontSize(fsTiny).TextColor(p.muted).SingleLine().Ellipsis("…")
				})
				enabled := inBar
				if sw(c, &enabled).Changed() {
					a.toggleMenuBar(sc.ID)
				}
				if action.ActionIcon(c, icon("refresh-cw"), action.ActionIconProps{
					Size:  theme.Small,
					Title: tr("刷新"),
				}).Clicked() {
					a.forced[sc.ID] = true
					a.invalidate(sc.ID)
					a.syncMenuBar()
				}
				moreBtn := ui.Row(c).Size(28, 28).Radius(7).Center().Cursor(ui.CursorPointer).Tooltip(tr("更多操作"))
				if moreBtn.Hovered() || barMoreOpen {
					moreBtn = moreBtn.Background(p.hover)
				}
				if moreBtn.Children(func() {
					ui.Icon(c, barMoreSVG).FontSize(15).TextColor(p.muted)
				}).Clicked() {
					barMoreOpen = !barMoreOpen
				}
			})

			if barMoreOpen {
				ui.Row(c).FillWidth().Wrap().AlignItems(ui.Center).Gap(8).Padding(6, 0, 0, 0).Children(func() {
					if btn(c, tr("在组件管理中编辑")).Clicked() {
						barMoreOpen = false
						a.router.Push("/edit/" + sc.ID)
					}
					if btn(c, tr("查看全屏详情")).Clicked() {
						barMoreOpen = false
						a.router.Push("/full/" + sc.ID)
					}
					homeLbl := tr("显示在首页")
					if sc.Home {
						homeLbl = tr("从首页移除")
					}
					if btn(c, homeLbl).Clicked() {
						sc.Home = !sc.Home
						_ = a.store.SaveScript(sc)
						barMoreOpen = false
					}
				})
			}
		})

	previewStage := func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
			ui.Text(c, tr("预览")).FontSize(fsSmall).FontWeight(500).TextColor(p.muted)
			ui.Spacer(c)
			a.barThemeSwitcher(c, p)
		})

		stageBG := ui.Hex("#0B1110")
		if !p.dark {
			stageBG = ui.Hex("#EEF1F5")
		}
		stage := ui.Column(c).
			FillWidth().
			Background(stageBG).
			Radius(12).
			BorderWidth(1).
			BorderColor(p.border).
			Padding(24).
			Center()
		if !compact {
			stage = stage.Grow(1).MinHeight(560)
		}
		stage.Children(func() {
			th := barThemeFor(c, a.barPreviewTheme)
			a.renderScriptMenuCard(c, sc, st, meta, th, false)
		})
	}

	inspector := func() {
		// Display locations
		ui.Column(c).FillWidth().Gap(6).Children(func() {
			ui.Text(c, tr("显示位置")).FontSize(fsSmall).FontWeight(500).TextColor(p.muted)
			card(c).FillWidth().Children(func() {
				modeStr, _ := a.scriptMenuBarKind(sc, st.res)
				modeIdx := 0
				switch modeStr {
				case "menu":
					modeIdx = 1
				case "open":
					modeIdx = 2
				}
				a.barPlacementRow(c, p, barMenuBarSVG, tr("菜单栏"), true, func() {
					if dropdown(c, &modeIdx, tr("浮动面板"), tr("下拉菜单"), tr("打开详情")) {
						switch modeIdx {
						case 0:
							pl.MenuBarMode = "panel"
						case 1:
							pl.MenuBarMode = "menu"
						case 2:
							pl.MenuBarMode = "open"
						}
						a.setBarPlacement(sc.ID, pl)
						a.syncMenuBar()
					}
					on := inBar
					if sw(c, &on).Changed() {
						a.toggleMenuBar(sc.ID)
					}
				})
				a.barDisplayRow(c, p, sc.ID, pl, a.syncMenuBar)
				a.barPlacementRow(c, p, barDockLeftSVG, tr("左侧栏"), true, func() {
					on := pl.Enabled && pl.LeftSidebar
					if sw(c, &on).Changed() {
						pl.Enabled = true
						pl.LeftSidebar = on
						a.setBarPlacement(sc.ID, pl)
					}
				})
				a.barPlacementRow(c, p, barDockRightSVG, tr("右侧栏"), true, func() {
					on := pl.Enabled && pl.RightSidebar
					if sw(c, &on).Changed() {
						pl.Enabled = true
						pl.RightSidebar = on
						a.setBarPlacement(sc.ID, pl)
					}
				})
				a.barPlacementRow(c, p, barDockBottomSVG, tr("底部侧栏"), true, func() {
					on := pl.Enabled && pl.BottomBar
					if sw(c, &on).Changed() {
						pl.Enabled = true
						pl.BottomBar = on
						a.setBarPlacement(sc.ID, pl)
					}
				})
				a.barPlacementRow(c, p, barKeySVG, tr("快捷键"), false, func() {
					chip(c, "Alt+W", p.muted, p.chip)
					on := pl.Enabled && pl.Shortcut
					if sw(c, &on).Changed() {
						pl.Enabled = true
						pl.Shortcut = on
						a.setBarPlacement(sc.ID, pl)
					}
				})
			})
			a.barDisplayNote(c, p)
		})

		// Inputs: script vars and dimensions
		if len(meta.Vars) > 0 || len(meta.Dims) > 0 {
			ui.Column(c).FillWidth().Gap(6).Children(func() {
				ui.Text(c, tr("输入参数")).FontSize(fsSmall).FontWeight(500).TextColor(p.muted)
				card(c).FillWidth().Padding(12).Gap(10).Children(func() {
					if len(meta.Vars) > 0 {
						a.varRow(c, sc, meta)
					}
					if len(meta.Dims) > 0 {
						a.dimRow(c, idx, meta.Dims)
					}
				})
			})
		}

		// Source info
		ui.Column(c).FillWidth().Gap(6).Children(func() {
			ui.Text(c, tr("来源")).FontSize(fsSmall).FontWeight(500).TextColor(p.muted)
			card(c).FillWidth().Padding(12).Gap(8).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(6).Children(func() {
					ui.Text(c, fmt.Sprintf("%s · v%s", originLabel(a.originOf(sc)), ver)).FontSize(fsSmall).FontWeight(500)
					ui.Spacer(c)
					chip(c, tr("脚本"), p.muted, p.chip)
				})
				if meta.Description != "" {
					ui.Text(c, meta.Description).FontSize(fsTiny).TextColor(p.muted)
				}
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
					if btn(c, tr("在组件管理中编辑")).Clicked() {
						a.router.Push("/edit/" + sc.ID)
					}
					ui.Spacer(c)
					if action.ActionIcon(c, icon("refresh-cw"), action.ActionIconProps{
						Size:  theme.Small,
						Title: tr("刷新"),
					}).Clicked() {
						a.forced[sc.ID] = true
						a.invalidate(sc.ID)
						a.syncMenuBar()
					}
				})
			})
		})
	}

	if compact {
		ui.Column(c).FillWidth().Padding(14).Gap(16).Children(func() {
			previewStage()
			inspector()
		})
		return
	}

	ui.Row(c).Grow(1).MinHeight(0).FillWidth().Padding(16, 20).Gap(18).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).FillHeight().Gap(10).Children(previewStage)
		ui.Scroll(c).Width(292).FillHeight().Children(func() {
			ui.Column(c).FillWidth().Gap(14).Padding(0, 0, 16, 0).Children(inspector)
		})
	})
}

func (a *app) renderScriptMenuCard(c *ui.Context, sc store.Script, st *runState, meta engine.Meta, th barTheme, vibrant bool) {
	iconName := meta.Icon
	title := tr(sc.Name)
	if st.res.Icon != "" {
		iconName = st.res.Icon
	}
	if st.res.Title != "" {
		title = st.res.Title
	}
	cardEl := ui.Column(c).
		Width(300).
		Padding(16).
		Gap(12)
	if !vibrant {
		cardEl = cardEl.Background(th.surface).Radius(14).BorderWidth(1).BorderColor(th.border)
	}
	cardEl.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			iconBadge(c, iconName, th.accent, 26)
			ui.Text(c, title).FontSize(13).FontWeight(600).TextColor(th.text).SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
			if st.res.MenuBar != nil && st.res.MenuBar.Title != "" {
				chip(c, st.res.MenuBar.Title, th.accent, th.accent.Alpha(0.14))
			}
		})
		switch {
		case st.loading:
			ui.Text(c, tr("加载中…")).FontSize(12).TextColor(th.textMuted)
		case st.err != nil:
			ui.Text(c, st.err.Error()).FontSize(12).TextColor(th.danger)
		default:
			if st.res.Big != "" {
				ui.Text(c, st.res.Big).FontSize(34).FontWeight(650).TextColor(th.text).NoWrap()
			}
			if st.res.Sub != "" {
				ui.Text(c, st.res.Sub).FontSize(12).TextColor(th.textMuted)
			}
			comp, viewRes, subStyle := resolveComponent(st.res, a.styles[sc.ID])
			if comp != nil {
				comp.RenderCard(c, ComponentRenderReq{
					App: a, Script: sc, Result: viewRes, Style: subStyle,
					Size: 1, Height: 140, Selected: a.selOf(sc.ID), Now: a.now(),
				})
			}
			if len(st.res.Menu) > 0 {
				ui.Box(c).FillWidth().Height(1).Background(th.border)
				ui.Column(c).FillWidth().Gap(4).Children(func() {
					for i, m := range st.res.Menu {
						if i >= 6 {
							break
						}
						if m.Separator {
							ui.Box(c).FillWidth().Height(1).Background(th.border)
							continue
						}
						ui.Text(c, m.Label).FontSize(12).TextColor(th.textMuted).SingleLine().Ellipsis("…")
					}
				})
			} else if len(st.res.Lines) > 0 {
				ui.Box(c).FillWidth().Height(1).Background(th.border)
				ui.Column(c).FillWidth().Gap(3).Children(func() {
					for i, line := range st.res.Lines {
						if i >= 4 {
							break
						}
						ui.Text(c, line).FontSize(12).TextColor(th.textMuted).SingleLine().Ellipsis("…")
					}
				})
			}
		}
	})
}

func (a *app) renderDockedBarPanel(c *ui.Context, sc store.Script, p palette, onClose func()) {
	title := tr(sc.Name)
	iconName := "sparkles"
	var snap bar.Snapshot
	if sc.IsBar() {
		inst, sp, err := a.ensureBarInstance(sc)
		if err == nil && inst != nil {
			snap = inst.Snapshot()
		} else if err != nil {
			snap.Error = err.Error()
		}
		if sp.Title != "" {
			title = sp.Title
		}
		iconName = barIconName(sp.Icon)
	} else {
		meta, _ := a.metaOf(sc)
		iconName = meta.Icon
		if meta.Name != "" {
			title = meta.Name
		}
	}

	ui.Column(c.Key("dock:"+sc.ID)).Gap(6).Children(func() {
		ui.Row(c).Width(300).AlignItems(ui.Center).Gap(6).Children(func() {
			ui.Icon(c, icon(iconName)).FontSize(13).TextColor(p.muted)
			ui.Text(c, title).FontSize(fsTiny).FontWeight(600).TextColor(p.muted).SingleLine().Ellipsis("…")
			ui.Spacer(c)
			if action.ActionIcon(c, icon("maximize-2"), action.ActionIconProps{
				Size:  theme.Small,
				Title: tr("打开组件详情"),
			}).Clicked() {
				a.router.Push("/bar/" + sc.ID)
			}
			if action.ActionIcon(c, icon("x"), action.ActionIconProps{
				Size:  theme.Small,
				Title: tr("关闭"),
			}).Clicked() {
				onClose()
			}
		})
		if sc.IsBar() {
			renderBarPanel(c, snap, barThemeFor(c, barThemeAuto), 300, func(aid string) {
				a.invokeBarAction(sc.ID, aid)
			})
		} else {
			meta, metaErr := a.metaOf(sc)
			_, vals := a.viewOf(sc.ID, meta.Dims)
			st := &runState{err: metaErr}
			if metaErr == nil {
				st = a.ensure(a.runKeyOf(sc.ID, sc, meta, vals), sc, vals)
			}
			a.renderScriptMenuCard(c, sc, st, meta, barThemeFor(c, barThemeAuto), false)
		}
	})
}

func (a *app) barShortcutOverlay(c *ui.Context, p palette) {
	if a.barOverlayID == "" {
		return
	}
	sc, ok := a.store.Script(a.barOverlayID)
	if !ok {
		a.barOverlayID = ""
		return
	}
	var pl store.BarPlacement
	if sc.IsBar() {
		pl = a.barPlacement(sc.ID)
	} else {
		pl = a.scriptDockPlacement(sc.ID)
	}
	if !pl.Enabled {
		a.barOverlayID = ""
		return
	}

	backdrop := ui.Column(c.Key("bar-shortcut-overlay")).
		Absolute().
		Top(0).Right(0).Bottom(0).Left(0).
		Background(ui.Hex("#000000").Alpha(0.45)).
		Center()
	backdrop.Children(func() {
		box := ui.Column(c).Gap(8).AlignItems(ui.End).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				if btn(c, tr("打开组件详情")).Clicked() {
					a.barOverlayID = ""
					a.router.Push("/bar/" + sc.ID)
				}
				if btn(c, tr("关闭")).Clicked() {
					a.barOverlayID = ""
				}
			})
			if sc.IsBar() {
				inst, _, err := a.ensureBarInstance(sc)
				var snap bar.Snapshot
				if err == nil && inst != nil {
					snap = inst.Snapshot()
				} else if err != nil {
					snap.Error = err.Error()
				}
				renderBarPanel(c, snap, barThemeFor(c, barThemeAuto), 300, func(aid string) {
					a.invokeBarAction(sc.ID, aid)
				})
			} else {
				meta, metaErr := a.metaOf(sc)
				_, vals := a.viewOf(sc.ID, meta.Dims)
				st := &runState{err: metaErr}
				if metaErr == nil {
					st = a.ensure(a.runKeyOf(sc.ID, sc, meta, vals), sc, vals)
				}
				a.renderScriptMenuCard(c, sc, st, meta, barThemeFor(c, barThemeAuto), false)
			}
		})
		if backdrop.Clicked() && !box.Hovered() {
			a.barOverlayID = ""
		}
	})
}
