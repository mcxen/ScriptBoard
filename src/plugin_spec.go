package main

import (
	"errors"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/feedback"

	"scriptboard/src/internal/caps"
	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/spec"
	"scriptboard/src/internal/store"
)

// What the app says about a script's origin, platforms and needs: one place, so
// the editor, the home page and the full view agree.

// specOf is what a script's meta() and its uses add up to. A script that does
// not compile has no meta: it counts as runnable, so that its error is shown
// rather than hidden.
func (a *app) specOf(sc store.Script) spec.Resolved {
	m, err := a.metaOf(sc)
	if err != nil {
		return spec.Resolved{Platforms: caps.Platforms}
	}
	return spec.Resolve(m)
}

// supported reports whether the script can run on this system.
func (a *app) supported(sc store.Script) bool {
	return a.specOf(sc).SupportsOS(caps.OS())
}

// osName is how a system is written for the user.
func osName(system string) string {
	switch system {
	case caps.MacOS:
		return "macOS"
	case caps.Windows:
		return "Windows"
	case caps.Linux:
		return "Linux"
	case caps.Android:
		return "Android"
	case caps.IOS:
		return "iOS"
	}
	return system
}

// errUnsupported is why a script does not run on this system.
func errUnsupported(r spec.Resolved) error {
	if len(r.OSes) == 0 {
		return errors.New(tr("这个组件不能在任何设备上使用，它的 meta() 有矛盾。"))
	}
	if r.DesktopOnly() && r.PlatformDependent() && caps.Platform() == caps.Mobile {
		return errors.New(tr("这个组件只能在电脑上使用：它要读电脑上的文件。"))
	}
	names := make([]string, len(r.OSes))
	for i, s := range r.OSes {
		names[i] = osName(s)
	}
	return errors.New(tr("这个组件只能在这些系统上使用：") + strings.Join(names, "、"))
}

// originLabel names where a script came from.
func originLabel(origin string) string {
	switch origin {
	case store.OriginBuiltin:
		return tr("内置")
	case store.OriginCommunity:
		return tr("社区")
	case store.OriginPlugin:
		return tr("本地插件")
	}
	return tr("自建")
}

// platformLabel names where a script runs, or "" when it runs everywhere.
func platformLabel(r spec.Resolved) string {
	switch {
	case len(r.OSes) == len(caps.Systems):
		return ""
	case r.DesktopOnly() && len(r.OSes) == len(caps.Expand(caps.Desktop)):
		return tr("仅电脑")
	case r.MobileOnly() && len(r.OSes) == len(caps.Expand(caps.Mobile)):
		return tr("仅手机")
	}
	names := make([]string, len(r.OSes))
	for i, s := range r.OSes {
		names[i] = osName(s)
	}
	return tr("仅 ") + strings.Join(names, "、")
}

// needLabel names a platform feature a script depends on.
func needLabel(need string) string {
	switch need {
	case caps.Network:
		return tr("联网")
	case caps.Files:
		return tr("读本机文件")
	case caps.Exec:
		return tr("运行本机程序")
	case caps.Key:
		return tr("需要密钥")
	}
	return need
}

// scriptNote is the line under a script's name in a list: its origin, the
// platform if it is limited to one, and what it reads from this machine.
func (a *app) scriptNote(sc store.Script) string {
	r := a.specOf(sc)
	parts := []string{originLabel(a.originOf(sc))}
	if sc.IsTable() {
		return tr("数据表") + " · " + parts[0]
	}
	if sc.IsBar() {
		return tr("组件") + " · " + parts[0]
	}
	if p := platformLabel(r); p != "" {
		parts = append(parts, p)
	}
	if r.PlatformDependent() {
		for _, n := range r.Needs {
			if caps.DesktopOnly[n] {
				parts = append(parts, needLabel(n))
			}
		}
	}
	return strings.Join(parts, " · ")
}

// specInfo is the block under the draft's name: its origin, version, platforms
// and needs as chips, its description, and what is wrong with its meta().
func (a *app) specInfo(c *ui.Context, sc store.Script) {
	p := pal(c)
	m, err := a.metaOf(sc)
	if err != nil {
		return
	}
	r := spec.Resolve(m)
	ui.Row(c).AlignItems(ui.Center).Gap(6).Wrap().Children(func() {
		chip(c, originLabel(a.originOf(sc)), p.accent, p.accentSoft)
		if m.Version != "" {
			chip(c, "v"+m.Version, p.muted, p.chip)
		}
		if pl := platformLabel(r); pl != "" {
			chip(c, pl, p.warn, p.warn.Alpha(0.14))
		} else {
			chip(c, tr("电脑与手机"), p.muted, p.chip)
		}
		for _, n := range r.Needs {
			if caps.DesktopOnly[n] {
				chip(c, needLabel(n), p.warn, p.warn.Alpha(0.14))
			} else {
				chip(c, needLabel(n), p.muted, p.chip)
			}
		}
	})
	if m.Description != "" {
		hint(c, m.Description)
	}
	a.grantRows(c, sc)
	if !r.SupportsOS(caps.OS()) {
		notice(c, errUnsupported(r).Error(), feedback.Warning)
	}
	for _, problem := range r.Problems {
		ui.Text(c, "⚠ "+problem).FontSize(fsSmall).TextColor(p.warn)
	}
}

// canDelete reports whether a script can be removed from the editor: the
// user's own and the community's. A built-in script is restored instead, and a
// plugin lives in its folder.
func (a *app) canDelete(sc store.Script) bool {
	o := a.originOf(sc)
	return o == store.OriginUser || o == store.OriginCommunity
}

// deleteDraft removes the draft's script, after a second click to confirm.
func (a *app) deleteDraft() {
	if !a.canDelete(a.draft) {
		return
	}
	if a.confirmDelete != a.draft.ID {
		a.confirmDelete = a.draft.ID
		a.status = tr("再点一次「确认删除」才会删除")
		return
	}
	id := a.draft.ID
	a.confirmDelete = ""
	if err := a.store.DeleteScript(id); err != nil {
		a.status = tr("删除失败：") + err.Error()
		return
	}
	a.invalidate(id)
	a.editorID = ""
	if a.compact {
		// Back to the list: nothing is open.
		a.draft = store.Script{}
		a.router.Replace("/edit")
		a.status = ""
		return
	}
	if list := a.store.Scripts(); len(list) > 0 {
		a.openEditor(list[0].ID)
	} else {
		a.newScript()
	}
	a.status = tr("已删除")
}

// restoreDraft puts a built-in script back to the source that shipped with the
// app, after the user changed it.
func (a *app) restoreDraft() {
	for _, b := range engine.Builtins() {
		if b.ID != a.draft.ID {
			continue
		}
		a.draft.Source = b.Source
		a.saveDraft()
		a.status = tr("已还原为内置版本")
		return
	}
}

// grantRows lists the sensitive features a script that is not built in uses
// (the computer's files, programs, a service key), whether the user allowed
// each, and the button that allows or takes it back. The script does not run
// without them.
func (a *app) grantRows(c *ui.Context, sc store.Script) {
	if a.trusted(sc) {
		return
	}
	p := pal(c)
	for _, n := range a.needsOf(sc) {
		if !caps.Sensitive[n] {
			continue
		}
		n := n
		on := a.granted(sc, n)
		ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
			state, color := tr("未授权"), p.warn
			if on {
				state, color = tr("已授权"), p.down
			}
			ui.Text(c, needLabel(n)).FontSize(fsSmall)
			chip(c, state, color, color.Alpha(0.14))
			label := tr("授权")
			if on {
				label = tr("撤销")
			}
			if btn(c, label).Clicked() {
				a.setGrant(sc.ID, n, !on)
			}
		})
	}
}
