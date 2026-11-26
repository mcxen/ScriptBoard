package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/bar"
	"scriptboard/src/internal/barui"
	"scriptboard/src/internal/caps"
	"scriptboard/src/internal/cmdrun"
	"scriptboard/src/internal/colortheme"
	"scriptboard/src/internal/engine"
)

type cmdForm struct {
	name        string
	id          string
	desc        string
	shellIdx    int
	customShell string
	workDir     string
	command     string
	paramsText  string
	extractText string
	everyIdx    int
	timeoutText string
	note        string

	open         bool
	editing      string
	confirm      string
	testing      bool
	testResult   *cmdrun.Record
	runParams    map[string]map[string]*string
	selectIdx    map[string]map[string]*int
	expandedHist map[string]bool
	expandedRaw  map[string]bool
}

var (
	cmdShells   = []string{"sh", "bash", "zsh", "pwsh", "psh", "exec"}
	cmdEvery    = []int{0, 10, 30, 60, 300}
	cmdEveryTxt = []string{"仅手动", "10 秒", "30 秒", "1 分钟", "5 分钟"}
)

// ensureBuiltinCommands seeds shipped command tasks (such as net-ping) once,
// backfilling default display styles on unmodified legacy entries.
func (a *app) ensureBuiltinCommands() error {
	if sp, ok := a.store.Command(cmdrun.BuiltinPingID); ok {
		if sp.Builtin && sp.Display == nil && cmdrun.IsTrustedBuiltin(sp) {
			sp.Display = cmdrun.BuiltinPingSpec().Display.Clone()
			return a.store.SaveCommand(sp)
		}
		return nil
	}
	return a.store.SaveCommand(cmdrun.BuiltinPingSpec())
}

func (a *app) allowCommandExec(sp cmdrun.Spec) bool {
	if cmdrun.IsTrustedBuiltin(sp) {
		return true
	}
	return a.store.Settings().AllowExec
}

// commandInput gathers saved specs, latest runs, and recent histories for scripts naming meta().commands.
func (a *app) commandInput(ids []string) map[string]engine.CommandData {
	out := make(map[string]engine.CommandData, len(ids))
	for _, id := range ids {
		sp, ok := a.store.Command(id)
		if !ok {
			out[id] = engine.CommandData{
				ID:      id,
				Name:    id,
				History: []engine.CommandRun{},
				Error:   tr("没有命令任务 ") + id + tr("，请到「设置」→「数据源」→「命令任务」里添加"),
			}
			continue
		}
		recs, err := a.cmdHistory.Recent(id, 50)
		runs := make([]engine.CommandRun, 0, len(recs))
		for _, r := range recs {
			runs = append(runs, toEngineCommandRun(r))
		}
		cd := engine.CommandData{
			ID:      sp.ID,
			Name:    sp.Name,
			Display: sp.Display.Clone(),
			History: runs,
		}
		if len(runs) > 0 {
			latest := runs[0]
			cd.Latest = &latest
			cd.OK = latest.OK
			cd.Error = latest.Error
			rc := sp.Display.ResolveRecord(recs[0])
			cd.Resolved = &rc
		}
		if err != nil && cd.Error == "" {
			cd.Error = err.Error()
		}
		if e := a.cmdRunner.LastError(id); e != "" && cd.Error == "" {
			cd.Error = e
		}
		out[id] = cd
	}
	return out
}

func toEngineCommandRun(r cmdrun.Record) engine.CommandRun {
	return engine.CommandRun{
		ID:         r.ID,
		T:          r.T,
		Trigger:    r.Trigger,
		Params:     r.Params,
		OK:         r.OK,
		ExitCode:   r.ExitCode,
		DurationMs: r.DurationMs,
		Extracted:  r.Extracted,
		JSON:       r.JSON,
		Stdout:     r.Stdout,
		Stderr:     r.Stderr,
		Error:      r.Error,
	}
}

// commandLoop triggers scheduled desktop commands whose interval has elapsed.
func (a *app) commandLoop() {
	for range time.Tick(5 * time.Second) {
		if a.set.Offline || caps.Platform() == caps.Mobile {
			continue
		}
		a.cmdRunner.PollDue(context.Background(), a.store.Commands(), 4, func(sp cmdrun.Spec, _ cmdrun.Record) {
			id := sp.ID
			a.post(func() { a.refreshCommandConsumers(id) })
		})
	}
}

// refreshCommandConsumers invalidates scripts and updates .bar instances consuming command id.
func (a *app) refreshCommandConsumers(id string) {
	for _, sc := range a.store.Scripts() {
		if sc.IsTable() {
			continue
		}
		if sc.IsBar() {
			if sp, err := a.barSpecOf(sc); err == nil && bar.CommandAllowed(id, sp.CommandPerms) == nil {
				if inst, ok := a.barInsts[sc.ID]; ok && inst != nil {
					inst.RefreshRender()
				}
			}
			continue
		}
		meta, _ := a.metaOf(sc)
		if slices.Contains(meta.Commands, id) {
			a.invalidate(sc.ID)
		}
	}
	a.syncBarMenuBar()
	a.syncMenuBar()
	a.redrawFlyouts()
}

type appCommandBridge struct{ a *app }

func (b appCommandBridge) Run(ctx context.Context, id string, params map[string]string) (cmdrun.Record, error) {
	sp, ok := b.a.store.Command(id)
	if !ok {
		return cmdrun.Record{}, fmt.Errorf("%s: %s", tr("找不到命令"), id)
	}
	rec, err := b.a.cmdRunner.Run(ctx, sp, params, "bar")
	if err == nil {
		b.a.post(func() { b.a.refreshCommandConsumers(id) })
	}
	return rec, err
}

func (b appCommandBridge) Latest(id string) (cmdrun.Record, bool, error) {
	return b.a.cmdHistory.Latest(id)
}

func (b appCommandBridge) History(id string, limit int) ([]cmdrun.Record, error) {
	return b.a.cmdHistory.Recent(id, limit)
}

func (b appCommandBridge) Style(id string) (*cmdrun.DisplayStyle, bool) {
	sp, ok := b.a.store.Command(id)
	if !ok || sp.Display == nil {
		return nil, false
	}
	return sp.Display.Clone(), true
}

func (a *app) cmdRunParamPtr(id string, p cmdrun.Param) *string {
	if a.cmdForm.runParams == nil {
		a.cmdForm.runParams = map[string]map[string]*string{}
	}
	if a.cmdForm.runParams[id] == nil {
		a.cmdForm.runParams[id] = map[string]*string{}
	}
	if ptr, ok := a.cmdForm.runParams[id][p.Key]; ok {
		return ptr
	}
	v := p.Default
	a.cmdForm.runParams[id][p.Key] = &v
	return &v
}

func (a *app) cmdSelectIdxPtr(id string, p cmdrun.Param) *int {
	if a.cmdForm.selectIdx == nil {
		a.cmdForm.selectIdx = map[string]map[string]*int{}
	}
	if a.cmdForm.selectIdx[id] == nil {
		a.cmdForm.selectIdx[id] = map[string]*int{}
	}
	if ptr, ok := a.cmdForm.selectIdx[id][p.Key]; ok {
		return ptr
	}
	idx := 0
	for i, opt := range p.Options {
		if opt == p.Default {
			idx = i
			break
		}
	}
	a.cmdForm.selectIdx[id][p.Key] = &idx
	return &idx
}

func (a *app) collectCardRunParams(sp cmdrun.Spec) map[string]string {
	if len(sp.Params) == 0 {
		return nil
	}
	out := make(map[string]string, len(sp.Params))
	for _, p := range sp.Params {
		if p.Type == "select" && len(p.Options) > 0 {
			idx := *a.cmdSelectIdxPtr(sp.ID, p)
			if idx >= 0 && idx < len(p.Options) {
				out[p.Key] = p.Options[idx]
			}
			continue
		}
		out[p.Key] = *a.cmdRunParamPtr(sp.ID, p)
	}
	return out
}

func (a *app) runCommandInBackground(sp cmdrun.Spec, params map[string]string, trigger string) {
	refKey := "cmd:" + sp.ID
	if a.isRefreshing(refKey) {
		return
	}
	a.beginRefresh(refKey, 1)
	run := func() {
		rec, err := a.cmdRunner.Run(context.Background(), sp, params, trigger)
		a.post(func() {
			if err != nil {
				a.cmdForm.note = tr(sp.Name) + "：" + err.Error()
				a.finishRefresh(refKey, false, err.Error())
				return
			}
			a.refreshCommandConsumers(sp.ID)
			if rec.OK {
				a.cmdForm.note = fmt.Sprintf(tr("%s：执行成功（%d ms）"), tr(sp.Name), rec.DurationMs)
				a.finishRefresh(refKey, true, "")
			} else {
				errText := rec.Error
				if errText == "" {
					errText = fmt.Sprintf("exit %d", rec.ExitCode)
				}
				a.cmdForm.note = fmt.Sprintf(tr("%s：执行失败（%s）"), tr(sp.Name), errText)
				a.finishRefresh(refKey, false, errText)
			}
		})
	}
	if a.win == nil {
		run()
		return
	}
	go run()
}

func (a *app) commandsPage(c *ui.Context, compact bool) {
	p := pal(c)
	page(c, compact, func() {
		subheader(c, a, tr("命令任务"), tr("执行 macOS / Linux Shell 或 PSH 命令，提取字段、定时追踪并保存历史记录"), nil)
		hint(c, tr("配置有参或无参命令行任务，支持在页面、菜单栏组件和脚本卡片中按需触发或定时运行，并保留历史记录。"))

		if caps.Platform() == caps.Mobile {
			chip(c, tr("本机命令执行仅支持桌面端"), p.warn, p.warn.Alpha(0.14))
		}
		if a.set.Offline {
			chip(c, tr("现在是离线模式，定时命令不会自动执行"), p.warn, p.warn.Alpha(0.14))
		}

		// Quick desktop AllowExec safety toggle bar.
		card(c).Padding(14).Gap(6).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, tr("允许执行自定义本机命令")).FontWeight(600)
					ui.Text(c, tr("内置网络 Ping 始终可用；自定义 Shell / PSH 命令需开启此开关")).FontSize(fsTiny).TextColor(p.muted)
				})
				if toggle(c, &a.set.AllowExec) {
					a.saveSettings()
				}
			})
		})

		cmds := a.store.Commands()
		if len(cmds) == 0 {
			card(c).Children(func() {
				empty(c, "terminal", tr("还没有命令任务"), tr("点击下方「+ 新建命令」添加一条 Shell 或 PSH 命令。"))
			})
		}
		for _, sp := range cmds {
			a.cmdCard(c, sp, compact)
		}

		if a.cmdForm.note != "" {
			chip(c, a.cmdForm.note, p.accent, p.accentSoft)
		}
		if a.cmdForm.testResult != nil {
			a.cmdTestPreview(c, a.cmdForm.testResult)
		}

		if a.cmdForm.open {
			a.cmdAddForm(c, compact)
		} else {
			ui.Row(c).Children(func() {
				if primaryBtn(c, tr("+ 新建命令")).Clicked() {
					a.cmdForm.open = true
					a.cmdForm.editing = ""
					a.cmdForm.id = ""
					a.cmdForm.name = ""
					a.cmdForm.desc = ""
					a.cmdForm.shellIdx = 0
					a.cmdForm.customShell = ""
					a.cmdForm.workDir = ""
					a.cmdForm.command = ""
					a.cmdForm.paramsText = ""
					a.cmdForm.extractText = ""
					a.cmdForm.everyIdx = 0
					a.cmdForm.timeoutText = "15"
					a.cmdForm.testResult = nil
				}
			})
		}
	})
}

func (a *app) cmdCard(c *ui.Context, sp cmdrun.Spec, compact bool) {
	p := pal(c)
	refKey := "cmd:" + sp.ID
	running := a.isRefreshing(refKey)

	card(c.Key("cmd-card:"+sp.ID)).Padding(16).Gap(10).Children(func() {
		// Header
		ui.Row(c).AlignItems(ui.Center).Gap(10).Wrap().Children(func() {
			iconName := "terminal"
			if sp.ID == cmdrun.BuiltinPingID {
				iconName = "activity"
			}
			iconBadge(c, iconName, p.accent, 34)
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(6).Wrap().Children(func() {
					ui.Text(c, tr(sp.Name)).Bold()
					if sp.Builtin {
						chip(c, tr("内置"), p.muted, p.chip)
					}
					chip(c, sp.Shell, p.accent, p.accentSoft)
					if lbl := cmdDisplaySummaryLabel(sp.Display); lbl != "" {
						chip(c, tr("样式：")+lbl, p.accent, p.accentSoft)
					}
					if sp.Every >= cmdrun.MinEvery {
						chip(c, tr("每 ")+everyText(sp.Every), p.muted, p.chip)
					} else {
						chip(c, tr("仅手动"), p.muted, p.chip)
					}
				})
				sub := "id " + sp.ID
				if sp.Description != "" {
					sub += " · " + tr(sp.Description)
				}
				ui.Text(c, sub).FontSize(fsTiny).TextColor(p.muted)
			})

			a.refreshIconButton(c, "cmd-stat:"+sp.ID, refKey, tr("执行状态"), 26, 14, true, p.hover, p.muted, p.accent, p.down, p.up, nil)
			runLbl := tr("立即执行")
			if running {
				runLbl = tr("执行中…")
			}
			if primaryBtn(c, runLbl).Clicked() && !running {
				params := a.collectCardRunParams(sp)
				a.runCommandInBackground(sp, params, "manual")
			}
			if btn(c, tr("编辑")).Clicked() {
				a.editCommand(sp)
			}
			if !sp.Builtin {
				delLbl := tr("删除")
				if a.cmdForm.confirm == sp.ID {
					delLbl = tr("确认删除")
				}
				if btn(c, delLbl).Clicked() {
					if a.cmdForm.confirm == sp.ID {
						a.deleteCommand(sp.ID)
					} else {
						a.cmdForm.confirm = sp.ID
					}
				}
			}
		})

		// Runtime parameters form row
		if len(sp.Params) > 0 {
			inW := float32(180)
			if compact {
				inW = 140
			}
			ui.Row(c).Gap(10).Wrap().AlignItems(ui.End).Children(func() {
				for _, pm := range sp.Params {
					pm := pm
					lbl := tr(pm.Label)
					if lbl == "" {
						lbl = pm.Key
					}
					ui.Column(c).Gap(3).Children(func() {
						ui.Text(c, lbl).FontSize(fsTiny).TextColor(p.muted)
						switch {
						case pm.Type == "select" && len(pm.Options) > 0:
							segmented(c, a.cmdSelectIdxPtr(sp.ID, pm), pm.Options...)
						case pm.Type == "password":
							passwordInput(c, a.cmdRunParamPtr(sp.ID, pm), pm.Placeholder).Width(inW)
						default:
							textInput(c, a.cmdRunParamPtr(sp.ID, pm), pm.Placeholder).Width(inW)
						}
					})
				}
			})
		}

		// Latest execution record
		latest, hasLatest, _ := a.cmdHistory.Latest(sp.ID)
		histCount := a.cmdHistory.Count(sp.ID)
		if !hasLatest {
			hint(c, tr("还没有执行记录"))
			return
		}

		statusTxt := fmt.Sprintf(tr("成功 · %d ms"), latest.DurationMs)
		statusFg := p.down
		if !latest.OK {
			statusTxt = fmt.Sprintf(tr("失败（exit %d）· %d ms"), latest.ExitCode, latest.DurationMs)
			statusFg = p.up
		}
		ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
			chip(c, statusTxt, statusFg, statusFg.Alpha(0.13))
			chip(c, cmdTriggerText(latest.Trigger), p.muted, p.chip)
			ui.Text(c, fmtTime(latest.T)).FontSize(fsTiny).TextColor(p.muted)
			if len(latest.Params) > 0 {
				ui.Text(c, formatParamSummary(latest.Params)).FontSize(fsTiny).Font("monospace").TextColor(p.faint)
			}
		})

		if len(latest.Extracted) > 0 {
			a.renderCommandExtracted(c, p, sp, latest)
		}
		if !latest.OK && latest.Error != "" {
			ui.Text(c, latest.Error).FontSize(fsSmall).TextColor(p.up)
		}

		// Expandable raw output & history controls
		ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
			rawLbl := tr("原始输出")
			if a.cmdForm.expandedRaw != nil && a.cmdForm.expandedRaw[sp.ID] {
				rawLbl = tr("收起输出")
			}
			if btn(c, rawLbl).Clicked() {
				if a.cmdForm.expandedRaw == nil {
					a.cmdForm.expandedRaw = map[string]bool{}
				}
				a.cmdForm.expandedRaw[sp.ID] = !a.cmdForm.expandedRaw[sp.ID]
			}

			histLbl := fmt.Sprintf(tr("历史记录 (%d)"), histCount)
			if a.cmdForm.expandedHist != nil && a.cmdForm.expandedHist[sp.ID] {
				histLbl = tr("收起历史")
			}
			if btn(c, histLbl).Clicked() {
				if a.cmdForm.expandedHist == nil {
					a.cmdForm.expandedHist = map[string]bool{}
				}
				a.cmdForm.expandedHist[sp.ID] = !a.cmdForm.expandedHist[sp.ID]
			}

			if histCount > 0 && btn(c, tr("清空历史")).Clicked() {
				if err := a.cmdHistory.Clear(sp.ID); err != nil {
					a.cmdForm.note = err.Error()
				} else {
					a.cmdForm.note = tr("已清空历史记录")
					a.refreshCommandConsumers(sp.ID)
				}
			}
		})

		if a.cmdForm.expandedRaw != nil && a.cmdForm.expandedRaw[sp.ID] {
			a.cmdRawOutputBlock(c, p, latest)
		}

		if a.cmdForm.expandedHist != nil && a.cmdForm.expandedHist[sp.ID] {
			recs, _ := a.cmdHistory.Recent(sp.ID, 20)
			if len(recs) == 0 {
				hint(c, tr("暂无历史记录"))
			} else {
				ui.Column(c).FillWidth().Padding(8).Radius(8).Background(p.chip).Gap(4).Children(func() {
					for i, rec := range recs {
						stLbl := "OK"
						stCol := p.down
						if !rec.OK {
							stLbl = fmt.Sprintf("ERR(%d)", rec.ExitCode)
							stCol = p.up
						}
						summary := formatExtractedInline(rec.Extracted)
						if summary == "" && !rec.OK {
							summary = rec.Error
						}
						ui.Row(c.Key(fmt.Sprintf("hist-%s-%d", sp.ID, i))).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Text(c, fmtTime(rec.T)).FontSize(fsTiny).Font("monospace").TextColor(p.muted).NoWrap()
							ui.Text(c, stLbl).FontSize(fsTiny).FontWeight(600).TextColor(stCol).NoWrap()
							ui.Text(c, fmt.Sprintf("%dms", rec.DurationMs)).FontSize(fsTiny).FontFeatures("tnum").TextColor(p.muted).NoWrap()
							ui.Text(c, cmdTriggerText(rec.Trigger)).FontSize(fsTiny).TextColor(p.muted).NoWrap()
							ui.Text(c, summary).FontSize(fsTiny).TextColor(p.text).SingleLine().Ellipsis("…").Grow(1).MinWidth(0)
						})
					}
				})
			}
		}
	})
}

func (a *app) cmdRawOutputBlock(c *ui.Context, p palette, rec cmdrun.Record) {
	raw := rec.Stdout
	if raw == "" && rec.Stderr != "" {
		raw = rec.Stderr
	} else if rec.Stderr != "" {
		raw += "\n[stderr]\n" + rec.Stderr
	}
	if raw == "" {
		raw = tr("（无输出）")
	}
	if len(raw) > 2400 {
		raw = raw[:2400] + "\n…"
	}
	ui.Column(c).FillWidth().Padding(10).Radius(8).Background(p.chip).Children(func() {
		ui.Text(c, raw).Font("monospace").FontSize(fsTiny).TextColor(p.text)
	})
}

func (a *app) cmdTestPreview(c *ui.Context, rec *cmdrun.Record) {
	p := pal(c)
	card(c).Padding(14).Gap(8).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, tr("测试运行结果")).Bold()
			stCol := p.down
			stLbl := fmt.Sprintf("exit %d · %d ms", rec.ExitCode, rec.DurationMs)
			if !rec.OK {
				stCol = p.up
			}
			chip(c, stLbl, stCol, stCol.Alpha(0.14))
			ui.Spacer(c)
			if btn(c, tr("关闭")).Clicked() {
				a.cmdForm.testResult = nil
			}
		})
		if len(rec.Extracted) > 0 {
			lookupID := rec.CommandID
			if lookupID == "" || lookupID == "inline-test" {
				lookupID = a.cmdForm.editing
			}
			sp, _ := a.store.Command(lookupID)
			a.renderCommandExtracted(c, p, sp, *rec)
			b, _ := json.MarshalIndent(rec.Extracted, "", "  ")
			ui.Text(c, tr("提取字段：")).FontSize(fsTiny).TextColor(p.muted)
			ui.Text(c, string(b)).Font("monospace").FontSize(fsTiny)
		}
		if !rec.OK && rec.Error != "" {
			ui.Text(c, rec.Error).FontSize(fsSmall).TextColor(p.up)
		}
		a.cmdRawOutputBlock(c, p, *rec)
	})
}

func (a *app) renderCommandExtracted(c *ui.Context, p palette, sp cmdrun.Spec, rec cmdrun.Record) {
	resolved := sp.Display.ResolveRecord(rec)
	if len(resolved.Order) == 0 {
		return
	}
	ui.Row(c).Gap(18).Wrap().AlignItems(ui.End).Children(func() {
		for _, k := range resolved.Order {
			res, exists := resolved.Fields[k]
			if !exists || res.Hidden {
				continue
			}
			col := cmdResolveColor(p, res.Color, p.text)
			lbl := tr(res.Label)
			if lbl == "" {
				lbl = k
			}
			ui.Column(c).Gap(3).Children(func() {
				ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
					ui.Text(c, lbl).FontSize(fsTiny).TextColor(p.muted)
					if res.StatusLabel != "" {
						stCol := cmdResolveColor(p, res.Color, p.accent)
						chip(c, tr(res.StatusLabel), stCol, stCol.Alpha(0.14))
					}
				})
				switch res.Variant {
				case cmdrun.StyleVariantBadge:
					pillCol := cmdResolveColor(p, res.Color, p.accent)
					ui.Row(c).Padding(4, 10).Radius(999).Background(pillCol.Alpha(0.14)).Gap(6).AlignItems(ui.Center).Children(func() {
						if res.Animation != "" && res.Animation != cmdrun.AnimNone {
							dw, dh := barui.DotAnimationBoxSize(res.Animation, float32(res.DotSize), res.DotCount)
							ui.Box(c).Size(dw, dh).Draw(func(pt *ui.Painter, r ui.Rect) {
								barui.PaintDotAnimation(pt, r, res.Animation, pillCol, float32(res.DotSize), res.DotCount, res.Speed, true)
							})
						}
						ui.Text(c, res.FormattedValue).FontSize(14).FontWeight(700).TextColor(pillCol).FontFeatures("tnum")
					})
				case cmdrun.StyleVariantMatrix:
					matCol := cmdResolveColor(p, res.Color, p.accent)
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						dw, dh := barui.DotAnimationBoxSize(cmdrun.AnimMatrix, float32(res.DotSize), res.DotCount)
						ui.Box(c).Size(dw, dh).Draw(func(pt *ui.Painter, r ui.Rect) {
							barui.PaintDotAnimation(pt, r, cmdrun.AnimMatrix, matCol, float32(res.DotSize), res.DotCount, res.Speed, true)
						})
						valCol := col
						if res.Color != "" {
							valCol = matCol
						}
						ui.Text(c, res.FormattedValue).FontSize(18).Bold().TextColor(valCol).FontFeatures("tnum")
					})
				case cmdrun.StyleVariantBar:
					barCol := cmdResolveColor(p, res.Color, p.accent)
					ui.Column(c).Gap(4).Children(func() {
						ui.Text(c, res.FormattedValue).FontSize(18).Bold().TextColor(barCol).FontFeatures("tnum")
						pct := float32(res.FillFrac * 100)
						ui.Box(c).Width(92).Height(5).Radius(2.5).Background(p.chip).Clip().Children(func() {
							ui.Box(c).Absolute().Top(0).Bottom(0).Left(0).WidthPercent(pct).Radius(2.5).Background(barCol)
						})
					})
				default:
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						if res.Animation != "" && res.Animation != cmdrun.AnimNone {
							dotCol := cmdResolveColor(p, res.Color, p.accent)
							dw, dh := barui.DotAnimationBoxSize(res.Animation, float32(res.DotSize), res.DotCount)
							ui.Box(c).Size(dw, dh).Draw(func(pt *ui.Painter, r ui.Rect) {
								barui.PaintDotAnimation(pt, r, res.Animation, dotCol, float32(res.DotSize), res.DotCount, res.Speed, true)
							})
						}
						ui.Text(c, res.FormattedValue).FontSize(18).Bold().TextColor(col).FontFeatures("tnum")
					})
				}
			})
		}
	})
}

func cmdResolveColor(p palette, name string, fallback ui.Color) ui.Color {
	s := strings.ToLower(strings.TrimSpace(name))
	if s == "" {
		return fallback
	}
	bth := barui.ThemeFor(p.dark, false)
	switch s {
	case "success", "down", "ok", "green":
		return p.down
	case "danger", "error", "up", "red":
		return p.up
	case "warning", "warn", "amber", "yellow":
		return p.warn
	case "accent", "primary", "blue":
		return p.accent
	case "cyan":
		return bth.Cyan
	case "purple":
		return bth.Purple
	case "muted", "neutral", "secondary":
		return p.muted
	case "text", "foreground":
		return p.text
	}
	return parseColorRole(name, colortheme.RoleText, p, fallback)
}

func cmdDisplaySummaryLabel(d *cmdrun.DisplayStyle) string {
	if d == nil {
		return ""
	}
	anim := d.Animation
	if anim == "" && d.PrimaryField != "" && d.Fields != nil {
		anim = d.Fields[d.PrimaryField].Animation
	}
	switch anim {
	case cmdrun.AnimPulse:
		return tr("点状脉冲")
	case cmdrun.AnimRipple:
		return tr("雷达波纹")
	case cmdrun.AnimDots:
		return tr("多点波浪")
	case cmdrun.AnimWave:
		return tr("流光波")
	case cmdrun.AnimOrbit:
		return tr("轨道点")
	case cmdrun.AnimBlink:
		return tr("心跳闪烁")
	case cmdrun.AnimMatrix:
		return tr("点阵流光")
	}
	if d.Variant != "" && d.Variant != cmdrun.StyleVariantDefault {
		return d.Variant
	}
	return ""
}

func (a *app) cmdAddForm(c *ui.Context, compact bool) {
	f := &a.cmdForm
	w := float32(400)
	if compact {
		w = 300
	}
	title := tr("新建命令任务")
	if f.editing != "" {
		title = tr("编辑命令任务")
	}
	sectionTitle(c, title)
	card(c).Padding(18).Gap(14).Children(func() {
		field(c, tr("名称"), "", func() {
			textInput(c, &f.name, tr("例如 数据库活动进度")).Width(w)
		})
		if f.editing != "" {
			field(c, "id", tr("id 创建后不能改，脚本或 .bar 组件用它引用命令"), func() {
				ui.Text(c, f.editing).Font("monospace").FontSize(fsSmall)
			})
		} else {
			field(c, "id", tr("可选，留空自动生成；小写字母、数字和连字符"), func() {
				textInput(c, &f.id, "db-activity").Width(w)
			})
		}
		field(c, tr("说明（可选）"), "", func() {
			textInput(c, &f.desc, tr("例如 查询活跃连接与事务进度")).Width(w)
		})
		field(c, tr("执行方式"), tr("macOS/Linux Shell、PowerShell (pwsh)、PSH 或直接 exec"), func() {
			segmented(c, &f.shellIdx, cmdShells...)
		})
		field(c, tr("命令 / 脚本"), tr("参数会通过环境变量 SB_PARAM_<KEY> 安全传入，禁止直接拼接未校验变量"), func() {
			textArea(c, &f.command, `printf '{"status":"ok","progress":85}'`, 5).Font("monospace").FontSize(fsSmall)
		})
		field(c, tr("参数定义（可选）"), tr("每行一个：key | 标签 | string/select/number/password | 默认值 | 选项1,选项2"), func() {
			textArea(c, &f.paramsText, tr("host | 目标主机 | string | 223.5.5.5\ncount | 发包数 | select | 3 | 1,3,5"), 3).Font("monospace").FontSize(fsSmall)
		})
		field(c, tr("字段提取规则（可选）"), tr("每行一个：字段名 | json/regex/line/raw | 规则 | string/number/bool"), func() {
			textArea(c, &f.extractText, "status | json | status | string\navgMs | regex | avg=([0-9.]+) | number", 3).Font("monospace").FontSize(fsSmall)
		})
		ui.Row(c).Gap(gapSection).Wrap().Children(func() {
			field(c, tr("定时触发"), "", func() {
				segmented(c, &f.everyIdx, trAll(cmdEveryTxt)...)
			})
			field(c, tr("超时（秒）"), "", func() {
				textInput(c, &f.timeoutText, "15").Width(96)
			})
		})
		ui.Row(c).Gap(8).Wrap().Children(func() {
			if primaryBtn(c, tr("保存")).Clicked() {
				a.saveCommandForm()
			}
			testLbl := tr("测试运行")
			if f.testing {
				testLbl = tr("测试中…")
			}
			if btn(c, testLbl).Clicked() && !f.testing {
				a.testCommandForm()
			}
			if btn(c, tr("取消")).Clicked() {
				note := f.note
				a.closeCommandForm()
				a.cmdForm.note = note
			}
		})
		hint(c, tr("「测试运行」只执行一次并展示提取字段和原始输出，不会保存或写入历史。"))
	})
}

func (a *app) formCommandSpec(test bool) (cmdrun.Spec, error) {
	f := &a.cmdForm
	name := strings.TrimSpace(f.name)
	if name == "" {
		name = tr("命令任务")
	}
	id := strings.TrimSpace(f.id)
	if f.editing != "" {
		id = f.editing
	} else if id == "" {
		id = slugOf(name)
	}
	if !cmdrun.ValidID(id) {
		return cmdrun.Spec{}, errors.New(tr("id 只能用小写字母、数字和连字符，2 到 40 个字符"))
	}
	if f.editing == "" && !test {
		if _, exists := a.store.Command(id); exists {
			return cmdrun.Spec{}, errors.New(tr("已经有 id 为 ") + id + tr(" 的命令了"))
		}
	}
	shell := cmdShells[0]
	if f.shellIdx >= 0 && f.shellIdx < len(cmdShells) {
		shell = cmdShells[f.shellIdx]
	} else if strings.TrimSpace(f.customShell) != "" {
		shell = f.customShell
	}
	params, err := parseParamsSpec(f.paramsText)
	if err != nil {
		return cmdrun.Spec{}, err
	}
	extracts, err := parseExtractSpec(f.extractText)
	if err != nil {
		return cmdrun.Spec{}, err
	}
	every := 0
	if f.everyIdx >= 0 && f.everyIdx < len(cmdEvery) {
		every = cmdEvery[f.everyIdx]
	}
	timeout := cmdrun.DefaultTimeout
	if s := strings.TrimSpace(f.timeoutText); s != "" {
		n, perr := strconv.Atoi(s)
		if perr != nil || n < 1 || n > cmdrun.MaxTimeout {
			return cmdrun.Spec{}, fmt.Errorf(tr("超时需在 1 到 %d 秒之间"), cmdrun.MaxTimeout)
		}
		timeout = n
	}

	sp := cmdrun.Spec{}
	if old, ok := a.store.Command(f.editing); ok && f.editing != "" {
		sp = old
	}
	sp.ID = id
	sp.Name = name
	sp.Description = strings.TrimSpace(f.desc)
	sp.Shell = shell
	sp.Command = f.command
	sp.WorkDir = strings.TrimSpace(f.workDir)
	sp.Params = params
	sp.Extract = extracts
	sp.Every = every
	sp.Timeout = timeout
	if sp.MaxHistory <= 0 {
		sp.MaxHistory = cmdrun.DefaultMaxHistory
	}
	// If the user edits the built-in net-ping command text/shell cleanly, preserve ID while Builtin stays true only when unchanged.
	if sp.ID == cmdrun.BuiltinPingID && !cmdrun.IsTrustedBuiltin(sp) {
		sp.Builtin = false
	}
	if err := sp.Validate(); err != nil {
		return cmdrun.Spec{}, err
	}
	return sp, nil
}

func (a *app) saveCommandForm() {
	f := &a.cmdForm
	sp, err := a.formCommandSpec(false)
	if err != nil {
		f.note = err.Error()
		return
	}
	if err := a.store.SaveCommand(sp); err != nil {
		f.note = err.Error()
		return
	}
	// Clear cached run parameter pointers for this command so updated defaults take effect.
	if a.cmdForm.runParams != nil {
		delete(a.cmdForm.runParams, sp.ID)
	}
	if a.cmdForm.selectIdx != nil {
		delete(a.cmdForm.selectIdx, sp.ID)
	}
	f.note = tr("已保存命令 ") + sp.Name + " (" + sp.ID + ")"
	a.closeCommandForm()
	a.refreshCommandConsumers(sp.ID)
}

func (a *app) testCommandForm() {
	f := &a.cmdForm
	sp, err := a.formCommandSpec(true)
	if err != nil {
		f.note = err.Error()
		return
	}
	if f.testing {
		return
	}
	f.testing = true
	run := func() {
		rec, err := a.cmdRunner.Test(context.Background(), sp, nil)
		a.post(func() {
			a.cmdForm.testing = false
			if err != nil {
				a.cmdForm.note = err.Error()
				return
			}
			cp := rec
			a.cmdForm.testResult = &cp
			if rec.OK {
				a.cmdForm.note = fmt.Sprintf(tr("测试完成（%d ms）"), rec.DurationMs)
			} else {
				a.cmdForm.note = tr("测试返回错误：") + rec.Error
			}
		})
	}
	if a.win == nil {
		run()
		return
	}
	go run()
}

func (a *app) closeCommandForm() {
	note := a.cmdForm.note
	runP := a.cmdForm.runParams
	selI := a.cmdForm.selectIdx
	expH := a.cmdForm.expandedHist
	expR := a.cmdForm.expandedRaw
	a.cmdForm = cmdForm{
		note:         note,
		runParams:    runP,
		selectIdx:    selI,
		expandedHist: expH,
		expandedRaw:  expR,
	}
}

func (a *app) editCommand(sp cmdrun.Spec) {
	f := &a.cmdForm
	runP, selI, expH, expR := f.runParams, f.selectIdx, f.expandedHist, f.expandedRaw
	*f = cmdForm{
		open:         true,
		editing:      sp.ID,
		id:           sp.ID,
		name:         sp.Name,
		desc:         sp.Description,
		workDir:      sp.WorkDir,
		command:      sp.Command,
		paramsText:   formatParamsSpec(sp.Params),
		extractText:  formatExtractSpec(sp.Extract),
		timeoutText:  strconv.Itoa(int(sp.TimeoutDuration().Seconds())),
		runParams:    runP,
		selectIdx:    selI,
		expandedHist: expH,
		expandedRaw:  expR,
	}
	f.shellIdx = -1
	for i, s := range cmdShells {
		if strings.EqualFold(s, sp.Shell) || (sp.Shell == "" && s == "sh") {
			f.shellIdx = i
			break
		}
	}
	if f.shellIdx < 0 {
		f.shellIdx = 0
		f.customShell = sp.Shell
	}
	for i, e := range cmdEvery {
		if e == sp.Every {
			f.everyIdx = i
			break
		}
	}
}

func (a *app) deleteCommand(id string) {
	if err := a.store.DeleteCommand(id); err != nil {
		a.cmdForm.note = err.Error()
		return
	}
	_ = a.cmdHistory.Delete(id)
	a.cmdForm.confirm = ""
	a.cmdForm.note = tr("已删除命令 ") + id
	a.refreshCommandConsumers(id)
}

func parseParamsSpec(raw string) ([]cmdrun.Param, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if strings.HasPrefix(raw, "[") {
		var out []cmdrun.Param
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			return nil, fmt.Errorf(tr("参数 JSON 解析失败：%w"), err)
		}
		return out, nil
	}
	var out []cmdrun.Param
	for _, ln := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		parts := strings.Split(ln, "|")
		p := cmdrun.Param{Key: strings.TrimSpace(parts[0])}
		if len(parts) > 1 {
			p.Label = strings.TrimSpace(parts[1])
		}
		if len(parts) > 2 {
			p.Type = strings.TrimSpace(parts[2])
		}
		if len(parts) > 3 {
			p.Default = strings.TrimSpace(parts[3])
		}
		if len(parts) > 4 {
			for _, opt := range strings.Split(parts[4], ",") {
				if opt = strings.TrimSpace(opt); opt != "" {
					p.Options = append(p.Options, opt)
				}
			}
		}
		out = append(out, p)
	}
	return out, nil
}

func formatParamsSpec(params []cmdrun.Param) string {
	var lines []string
	for _, p := range params {
		typ := p.Type
		if typ == "" {
			typ = "string"
		}
		parts := []string{p.Key, p.Label, typ, p.Default}
		if len(p.Options) > 0 {
			parts = append(parts, strings.Join(p.Options, ","))
		}
		lines = append(lines, strings.Join(parts, " | "))
	}
	return strings.Join(lines, "\n")
}

func parseExtractSpec(raw string) ([]cmdrun.ExtractRule, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if strings.HasPrefix(raw, "[") {
		var out []cmdrun.ExtractRule
		if err := json.Unmarshal([]byte(raw), &out); err != nil {
			return nil, fmt.Errorf(tr("提取规则 JSON 解析失败：%w"), err)
		}
		return out, nil
	}
	var out []cmdrun.ExtractRule
	for _, ln := range strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		parts := strings.Split(ln, "|")
		if len(parts) < 2 {
			return nil, fmt.Errorf(tr("提取规则格式应为「字段名 | json/regex/line/raw | 规则 | 类型」：%s"), ln)
		}
		r := cmdrun.ExtractRule{
			Name: strings.TrimSpace(parts[0]),
			Kind: strings.ToLower(strings.TrimSpace(parts[1])),
		}
		expr := ""
		if len(parts) > 2 {
			expr = strings.TrimSpace(parts[2])
		}
		switch r.Kind {
		case cmdrun.KindJSON:
			r.Path = expr
		case cmdrun.KindRegex:
			r.Pattern = expr
		case cmdrun.KindLine:
			n, err := strconv.Atoi(expr)
			if err != nil {
				return nil, fmt.Errorf(tr("line 提取规则的行号需为整数：%s"), expr)
			}
			r.Line = n
		}
		if len(parts) > 3 {
			r.Type = strings.TrimSpace(parts[3])
		}
		out = append(out, r)
	}
	return out, nil
}

func formatExtractSpec(rules []cmdrun.ExtractRule) string {
	var lines []string
	for _, r := range rules {
		expr := ""
		switch r.Kind {
		case cmdrun.KindJSON:
			expr = r.Path
		case cmdrun.KindRegex:
			expr = r.Pattern
		case cmdrun.KindLine:
			expr = strconv.Itoa(r.Line)
		}
		parts := []string{r.Name, r.Kind, expr}
		if r.Type != "" {
			parts = append(parts, r.Type)
		}
		lines = append(lines, strings.Join(parts, " | "))
	}
	return strings.Join(lines, "\n")
}

func cmdTriggerText(trig string) string {
	switch trig {
	case "schedule":
		return tr("定时")
	case "bar":
		return tr("菜单栏")
	case "script":
		return tr("脚本组件")
	case "mcp":
		return "MCP"
	case "test":
		return tr("测试")
	default:
		return tr("手动")
	}
}

func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func formatParamSummary(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	return strings.Join(parts, " ")
}

func formatExtractedInline(ext map[string]any) string {
	if len(ext) == 0 {
		return ""
	}
	keys := sortedMapKeys(ext)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, ext[k]))
	}
	return strings.Join(parts, " · ")
}
