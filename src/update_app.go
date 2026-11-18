package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"
)

// updateChoices are the update sources, in the order of the settings row:
// the key kept in Settings.UpdateSource and the label.
var updateChoices = []string{updateSourceAuto, updateSourceGitHub, updateSourceMirror}

// versionString is the version of the running build: what `mygo build` put
// in, or the source's own for a development build.
func versionString() string {
	if v := mygo.App.Version(); v != "" {
		return v
	}
	return appVersion
}

// updateLine says whether this build can update itself, and what the last
// check found.
func updateLine() string {
	if !mygo.Updater.Enabled() {
		return tr("这个版本不能自己更新：它是开发版，或由包管理器安装。")
	}
	if v := updater.AvailableVersion(); v != "" {
		return tr("有新版本 ") + v
	}
	if t := updater.LastCheck(); !t.IsZero() {
		return tr("上次检查：") + t.Local().Format("01-02 15:04")
	}
	return tr("还没有检查过。")
}

// updateGroup is the settings group for updates: the version, a check now,
// where updates come from, and the daily check.
func (a *app) updateGroup(c *ui.Context, compact bool) {
	rows := []row{
		{title: tr("检查更新"), note: updateLine() + " · " + tr("当前版本 ") + versionString(), control: func() {
			if btn(c, tr("立即检查")).Clicked() {
				updater.CheckForUpdates()
			}
		}},
	}
	if mygo.Updater.Enabled() {
		rows = append(rows,
			row{title: tr("更新来源"),
				note: tr("自动：系统语言是中文时走 tvv.tw 镜像，否则走 GitHub。镜像下载失败或被改动时自动改用 GitHub；更新都用内置的公钥验证签名。"),
				control: func() {
					names := make([]string, len(updateChoices))
					cur := 0
					for i, k := range updateChoices {
						names[i] = updateSourceName(k)
						if k == a.set.UpdateSource {
							cur = i
						}
					}
					if choose(c, &cur, names...) {
						a.set.UpdateSource = updateChoices[cur]
						a.saveSettings()
						applyUpdateSource(a.set.UpdateSource)
					}
				}},
			row{title: tr("自动检查更新"), note: tr("每天看一次，装之前会先问你。"), control: func() {
				on := updater.AutomaticChecks()
				if toggle(c, &on) {
					updater.SetAutomaticChecks(on)
				}
			}},
		)
	}
	group(c, tr("更新"), compact, rows...)
}
