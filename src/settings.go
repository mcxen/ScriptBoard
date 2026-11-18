package main

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/feed"
	"scriptboard/src/internal/plugins"
)

const (
	appVersion = "0.1.0"
	stackInfo  = "MyGo v0.10.0 · moejs v0.1.1"
)

var unitNames = []string{"kg", "斤"}

// settings is the settings page: groups of rows, each with a title, a line of
// explanation and its control. Every change is saved at once.
func (a *app) settings(c *ui.Context, compact bool) {
	p := pal(c)
	inputW := float32(260)
	if compact {
		inputW = 180
	}
	muted := func(text string) func() {
		return func() { ui.Text(c, text).FontSize(fsCaption).TextColor(p.muted) }
	}

	page(c, compact, func() {
		header(c, tr("设置"), "", nil)

		group(c, tr("外观"), compact,
			row{title: tr("风格"), note: tr("切换后所有页面立即换成所选的风格。"), control: func() {
				if choose(c, &a.skinIdx, tr("默认"), "Mac OS 9") {
					a.set.Skin = skinNames[a.skinIdx]
					setSkin(a.set.Skin)
					a.saveSettings()
				}
			}},
			row{title: tr("明暗"), note: tr("跟随系统，或固定为浅色或深色。"), control: func() {
				if choose(c, &a.themeIdx, tr("跟随系统"), tr("浅色"), tr("深色")) {
					a.set.ThemeMode = themeModes[a.themeIdx]
					setThemeMode(a.set.ThemeMode)
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
			row{title: tr("语言"), control: func() {
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
			row{title: tr("离线模式"), note: tr("不请求网络，卡片显示示例数据"), control: func() {
				if toggle(c, &a.set.Offline) {
					a.saveSettings()
				}
			}},
			row{title: tr("数据文件"), note: a.store.Path()},
		)

		group(c, tr("数据源"), compact,
			row{title: tr("允许 exec 协议"), note: tr("数据源可以运行本机程序取数（exec:）。只在你信任的数据源上开启"), control: func() {
				if toggle(c, &a.set.AllowExec) {
					a.saveSettings()
				}
			}},
			row{title: tr("curl 数据源"), note: tr("每隔一段时间请求接口，存下数字，画按小时或按天的曲线") + " · " + tr("支持的协议：") + strings.Join(feed.Schemes(), " "), control: func() {
				if btn(c, tr("管理")).Clicked() {
					a.router.Push("/feeds")
				}
			}},
		)

		a.updateGroup(c, compact)

		group(c, tr("提醒"), compact,
			row{title: tr("每日提醒时间"), note: tr("尚未接入系统通知，目前只保存这个时间"), control: func() {
				if textInput(c, &a.set.RemindAt, "HH:MM").Width(96).Changed() {
					a.saveSettings()
				}
			}},
		)

		group(c, tr("社区"), compact,
			row{title: tr("社区地址"), note: tr("Cloudflare Worker 的地址，留空则不用社区"), control: func() {
				if textInput(c, &a.set.CommunityURL, "https://…workers.dev").Width(inputW).Changed() {
					a.saveSettings()
				}
			}},
		)

		a.usageGroups(c, compact)

		a.mcpGroups(c, compact)

		pluginRows := []row{
			{title: tr("插件目录"), note: plugins.PluginDir(a.dataDir)},
			{title: tr("本地插件"), note: tr("放进这个目录，或用 MCP 工具 sbmcp 创建；不需要登录，也不经过审核，改动约一秒后生效")},
		}
		if a.pluginNote != "" {
			pluginRows = append(pluginRows, row{title: tr("有插件没能加载"), note: a.pluginNote})
		}
		group(c, tr("插件"), compact, pluginRows...)

		group(c, tr("关于"), compact,
			row{title: appName(), control: muted(appVersion)},
			row{title: tr("技术栈"), control: muted(stackInfo)},
		)
	})
}
