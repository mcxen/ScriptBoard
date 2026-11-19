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

// supported reports whether the script can run on this device.
func (a *app) supported(sc store.Script) bool {
	return a.specOf(sc).Supports(caps.Platform())
}

// errUnsupported is why a script does not run on this device.
func errUnsupported(r spec.Resolved) error {
	switch {
	case r.DesktopOnly():
		return errors.New(tr("这个组件只能在电脑上使用：它要读电脑上的文件。"))
	case len(r.Platforms) == 1 && r.Platforms[0] == caps.Mobile:
		return errors.New(tr("这个组件只能在手机上使用。"))
	}
	return errors.New(tr("这个组件不能在这台设备上使用。"))
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

// platformLabel names where a script runs, or "" when it runs anywhere.
func platformLabel(r spec.Resolved) string {
	switch {
	case r.DesktopOnly():
		return tr("仅电脑")
	case len(r.Platforms) == 1 && r.Platforms[0] == caps.Mobile:
		return tr("仅手机")
	}
	return ""
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
	if !r.Supports(caps.Platform()) {
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
