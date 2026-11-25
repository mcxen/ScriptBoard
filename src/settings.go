package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/feed"
	"scriptboard/src/internal/plugins"
)

// appVersion is what a build that did not get a version reports (versionString).
const appVersion = "dev"

// settingsNavWidth is the width of the left tab list in the settings UI.
const settingsNavWidth = 184

var unitNames = []string{"kg", "斤"}

type settingsTabItem struct {
	key, icon, label string
}

// settingsTabs are the sections shown in the left tab list of Settings.
var settingsTabs = []settingsTabItem{
	{key: "appearance", icon: "sun", label: "外观"},
	{key: "data", icon: "file-spreadsheet", label: "数据"},
	{key: "usage", icon: "sparkles", label: "AI 用量"},
	{key: "feeds", icon: "globe", label: "数据源"},
	{key: "mcp", icon: "bot", label: "MCP 服务器"},
	{key: "community", icon: "users", label: "社区"},
	{key: "about", icon: "info", label: "关于"},
}

func settingsTabByKey(key string) settingsTabItem {
	for _, t := range settingsTabs {
		if t.key == key {
			return t
		}
	}
	return settingsTabs[0]
}

// settings is the settings page: a left list of tabs beside the active tab's
// groups of rows (or a horizontal tab strip on phones). Every change is saved
// at once.
func (a *app) settings(c *ui.Context, compact bool) { a.settingsBody(c, compact, true) }

// settingsBody builds the settings UI; titled says whether it renders its own
// page heading (the standalone settings window puts "设置" in its title row).
func (a *app) settingsBody(c *ui.Context, compact, titled bool) {
	p := pal(c)
	tab := settingsTabByKey(a.settingsTab)
	a.settingsTab = tab.key

	winW, _ := c.Size()
	sideNav := !compact && (winW == 0 || winW >= 540)

	if sideNav {
		// Inside the main window (tests/fallback), the main sidebar already
		// uses sidebarWidth, so estimate the pane width from what remains.
		availW := float32(winW)
		if titled && availW > sidebarWidth {
			availW -= sidebarWidth
		}
		paneCompact := availW > 0 && (availW-settingsNavWidth) < 520

		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			a.settingsNav(c, p, tab.key)
			skinSurface(c, ui.Column(c).Grow(1).MinWidth(0)).Children(func() {
				page(c.Key("settings-pane:"+tab.key), paneCompact, func() {
					if titled {
						header(c, tr("设置"), "", nil)
					}
					a.settingsTabContent(c, tab, paneCompact)
				})
			})
		})
		return
	}

	// Narrow window or phone: horizontal scrollable tab bar over the active tab.
	skinSurface(c, ui.Column(c).Grow(1)).Children(func() {
		if titled {
			ui.Column(c).Padding(padPhone, padPhone, 6, padPhone).Children(func() {
				header(c, tr("设置"), "", nil)
			})
		}
		a.settingsTabStrip(c, p, tab.key)
		page(c.Key("settings-pane:"+tab.key), true, func() {
			a.settingsTabContent(c, tab, true)
		})
	})
}

// settingsNav is the left tab sidebar of Settings on desktop.
func (a *app) settingsNav(c *ui.Context, p palette, activeKey string) {
	borderColor := p.hairline
	if isMac() {
		borderColor = macBlack
	}
	ui.Column(c).
		Width(settingsNavWidth).
		Shrink(0).
		Background(p.sidebar).
		BorderWidth(0, 1, 0, 0).
		BorderColor(borderColor).
		Padding(14, 10).
		Gap(2).
		Children(func() {
			if isMac() {
				for _, t := range settingsTabs {
					if macSidebarItem(c, tr(t.label), t.icon, t.key == activeKey, settingsNavWidth-20) {
						a.settingsTab = t.key
					}
				}
				return
			}
			for _, t := range settingsTabs {
				t := t
				if navRow(c, "stab:"+t.key, t.icon, tr(t.label), t.key == activeKey, settingsNavWidth-20, nil) {
					a.settingsTab = t.key
				}
			}
		})
}

// settingsTabStrip is the horizontal tab bar used on phones and narrow windows.
func (a *app) settingsTabStrip(c *ui.Context, p palette, activeKey string) {
	ui.ScrollHorizontal(c).
		Shrink(0).
		BorderWidth(0, 0, 1, 0).
		BorderColor(p.hairline).
		Padding(8, padPhone).
		Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				for _, t := range settingsTabs {
					t := t
					chosen := t.key == activeKey
					fg, icFg := p.muted, p.muted
					if chosen {
						fg, icFg = p.text, p.accent
					}
					el := ui.ButtonBase(c.Key("stab:"+t.key)).
						Padding(6, 11).
						Radius(radControl).
						Gap(6).
						AlignItems(ui.Center).
						Shrink(0)
					if chosen {
						el = el.Background(p.rowSel)
						if !p.dark {
							el = el.BorderWidth(1).BorderColor(p.hairline)
						}
					} else if el.Hovered() {
						el = el.Background(p.hover)
					}
					if el.Children(func() {
						ui.Icon(c, icon(t.icon)).FontSize(14).TextColor(icFg)
						txt := ui.Text(c, tr(t.label)).FontSize(fsSmall).TextColor(fg).NoWrap()
						if chosen {
							txt.FontWeight(600)
						}
					}).Clicked() {
						a.settingsTab = t.key
					}
				}
			})
		})
}

// settingsTabContent renders the groups belonging to tab.
func (a *app) settingsTabContent(c *ui.Context, tab settingsTabItem, compact bool) {
	p := pal(c)
	inputW := float32(260)
	if compact {
		inputW = 180
	}
	muted := func(text string) func() {
		return func() { ui.Text(c, text).FontSize(fsCaption).TextColor(p.muted) }
	}

	// Default skin shows a crisp tab heading once at the top of the right pane
	// so single-group tabs do not repeat the label directly underneath it.
	// Mac OS 9 keeps its classic category bar on every group.
	groupTitled := isMac()
	if !isMac() {
		ui.Text(c, tr(tab.label)).FontSize(18).FontWeight(650).LetterSpacing(-0.3).Padding(0, 4)
	}
	singleTitle := func(name string) string {
		if groupTitled {
			return tr(name)
		}
		return ""
	}

	switch tab.key {
	case "appearance":
		group(c, singleTitle("外观"), compact,
			row{stack: true, title: tr("风格"), note: tr("切换后所有页面立即换成所选的风格。"), control: func() {
				if choose(c, &a.skinIdx, tr("默认"), "Mac OS 9") {
					a.set.Skin = skinNames[a.skinIdx]
					setSkin(a.set.Skin)
					a.saveSettings()
					a.syncAppearance()
				}
			}},
			row{stack: true, title: tr("明暗"), note: tr("跟随系统，或固定为浅色或深色。"), control: func() {
				if choose(c, &a.themeIdx, tr("跟随系统"), tr("浅色"), tr("深色")) {
					a.set.ThemeMode = themeModes[a.themeIdx]
					setThemeMode(a.set.ThemeMode)
					a.saveSettings()
					a.syncAppearance()
				}
			}},
			row{stack: true, title: tr("语言"), control: func() {
				if choose(c, &a.langIdx, tr("自动"), "中文", "English") {
					a.set.Lang = []string{"", langZH, langEN}[a.langIdx]
					setLang(a.set.Lang)
					a.saveSettings()
					a.invalidateAll()
					if a.win != nil {
						a.win.SetTitle(appName())
					}
					if a.settingsWin != nil {
						a.settingsWin.SetTitle(tr("设置"))
					}
				}
			}},
			row{stack: true, title: tr("内容宽度"), note: tr("自动：首页和详情页铺满窗口，设置等文字页保持易读的宽度；也可以固定为舒适、宽，或始终铺满。"), control: func() {
				idx := widthIndex(a.set.ContentWidth)
				if choose(c, &idx, tr("自动"), tr("舒适"), tr("宽"), tr("铺满")) {
					a.set.ContentWidth = widthModes[idx]
					a.saveSettings()
				}
			}},
		)

	case "data":
		group(c, singleTitle("数据"), compact,
			row{title: tr("体重单位"), note: tr("首页和图表按这个单位显示"), control: func() {
				if choose(c, &a.unitIdx, unitNames...) {
					a.set.WeightUnit = unitNames[a.unitIdx]
					a.saveSettings()
				}
			}},
			row{title: tr("离线模式"), note: tr("不请求网络，用缓存的数据，没有缓存就显示示例"), control: func() {
				if toggle(c, &a.set.Offline) {
					a.saveSettings()
				}
			}},
			row{title: tr("本地缓存"), note: fmt.Sprintf(tr("脚本请求过的数据、历史和上次的结果，保存在本机，离线时也能打开；占用 %s"), a.cacheSize()), control: func() {
				if btn(c, tr("清除缓存")).Clicked() {
					if err := a.clearCache(); err != nil {
						a.status = err.Error()
					} else {
						a.status = tr("已清除")
					}
				}
			}},
		)

	case "usage":
		a.usageGroups(c, compact, groupTitled)

	case "feeds":
		group(c, singleTitle("数据源"), compact,
			row{title: tr("允许 exec 协议"), note: tr("数据源可以运行本机程序取数（exec:）。只在你信任的数据源上开启"), control: func() {
				if toggle(c, &a.set.AllowExec) {
					a.saveSettings()
				}
			}},
			row{title: tr("curl 数据源"), note: tr("每隔一段时间请求接口，存下数字，画按小时或按天的曲线") + " · " + tr("支持的协议：") + strings.Join(feed.Schemes(), " "), control: func() {
				if btn(c, tr("管理")).Clicked() {
					a.openFeeds()
				}
			}},
		)

	case "mcp":
		a.mcpGroups(c, compact, groupTitled)

	case "community":
		group(c, singleTitle("社区"), compact,
			row{title: tr("社区地址"), note: tr("Cloudflare Worker 的地址，留空则不用社区"), control: func() {
				if textInput(c, &a.set.CommunityURL, "https://…workers.dev").Width(inputW).
					Keyboard(ui.KeyboardURL).
					Capitalize(ui.CapitalizeNone).
					Autocorrect(false).
					ReturnKey(ui.ReturnDone).
					Changed() {
					a.saveSettings()
				}
			}},
		)

	case "about":
		aboutRows := []row{
			{title: appName(), control: muted(versionString())},
			{title: tr("数据文件"), note: a.store.Path()},
			{title: tr("插件目录"), note: plugins.PluginDir(a.dataDir)},
		}
		if a.pluginNote != "" {
			aboutRows = append(aboutRows, row{title: tr("有插件没能加载"), note: a.pluginNote})
		}
		aboutRows = append(aboutRows, row{title: tr("退出"), note: tr("关闭窗口只是最小化，应用继续在后台运行；从这里、菜单栏的「退出」或快捷键才会真的退出。"), control: func() {
			if btn(c, tr("退出应用")).Clicked() {
				quitApp()
			}
		}})
		group(c, singleTitle("关于"), compact, aboutRows...)
		a.updateGroup(c, compact)
	}
}
