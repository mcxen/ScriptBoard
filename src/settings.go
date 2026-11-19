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

var unitNames = []string{"kg", "斤"}

// settings is the settings page: groups of rows, each with a title, a line of
// explanation and its control. Every change is saved at once.
func (a *app) settings(c *ui.Context, compact bool) { a.settingsBody(c, compact, true) }

// settingsBody is the settings page; titled says whether it has a title of its
// own (the settings window has its title in its title row).
func (a *app) settingsBody(c *ui.Context, compact, titled bool) {
	p := pal(c)
	inputW := float32(260)
	if compact {
		inputW = 180
	}
	muted := func(text string) func() {
		return func() { ui.Text(c, text).FontSize(fsCaption).TextColor(p.muted) }
	}

	page(c, compact, func() {
		if titled {
			header(c, tr("设置"), "", nil)
		}

		group(c, tr("外观"), compact,
			row{stack: true, title: tr("风格"), note: tr("切换后所有页面立即换成所选的风格。"), control: func() {
				if choose(c, &a.skinIdx, tr("默认"), "Mac OS 9") {
					a.set.Skin = skinNames[a.skinIdx]
					setSkin(a.set.Skin)
					a.saveSettings()
				}
			}},
			row{stack: true, title: tr("明暗"), note: tr("跟随系统，或固定为浅色或深色。"), control: func() {
				if choose(c, &a.themeIdx, tr("跟随系统"), tr("浅色"), tr("深色")) {
					a.set.ThemeMode = themeModes[a.themeIdx]
					setThemeMode(a.set.ThemeMode)
					a.saveSettings()
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

		group(c, tr("数据"), compact,
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

		a.usageGroups(c, compact)

		group(c, tr("数据源"), compact,
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

		a.updateGroup(c, compact)

		a.mcpGroups(c, compact)

		group(c, tr("社区"), compact,
			row{title: tr("社区地址"), note: tr("Cloudflare Worker 的地址，留空则不用社区"), control: func() {
				if textInput(c, &a.set.CommunityURL, "https://…workers.dev").Width(inputW).Changed() {
					a.saveSettings()
				}
			}},
		)

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
		group(c, tr("关于"), compact, aboutRows...)
	})
}
