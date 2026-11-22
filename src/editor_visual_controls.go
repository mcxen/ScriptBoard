package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"scriptboard/src/internal/engine"
)

// visualComponentContent renders only the data items editor for Tab 0 (内容).
func (a *app) visualComponentContent(c *ui.Context, compact bool) {
	p := pal(c)
	cfg := a.visual.cfg
	ui.Column(c).Gap(12).Padding(14).Radius(radControl + 2).BorderWidth(1).BorderColor(p.hairline).Children(func() {
		switch cfg.Type {
		case "progressBar":
			a.visualProgressItems(c, cfg, tr("进度条目"), tr("+ 添加进度项"))
		case "progressRing":
			a.visualProgressItems(c, cfg, tr("圆环条目"), tr("+ 添加圆环项"))
		case "donut":
			a.visualDonutItems(c, cfg)
		case "checklist":
			a.visualChecklistControls(c, cfg)
		case "kv":
			a.visualKVControls(c, cfg)
		case "countdown":
			a.visualCountdownControls(c, cfg)
		case "clock":
			a.visualClockZones(c, cfg)
		case "contrib":
			label(c, tr("打卡热力格数据"))
			hint(c, tr("当前展示近 12 周示例打卡数据；可在「样式」调整颜色与单位，或在「源码」接入真实数据"))
		case "table":
			a.visualTableItems(c, cfg)
		case "line":
			a.visualLineItems(c, cfg)
		}
	})
	_ = compact
}

// visualComponentStyle renders only the component-specific style controls for
// Tab 1 (样式). Components without extra style knobs (such as checklist, kv,
// countdown) skip this box and show only Card Display.
func (a *app) visualComponentStyle(c *ui.Context, compact bool) {
	p := pal(c)
	cfg := a.visual.cfg
	switch cfg.Type {
	case "checklist", "kv", "countdown":
		return
	}
	ui.Column(c).Gap(12).Padding(14).Radius(radControl + 2).BorderWidth(1).BorderColor(p.hairline).Children(func() {
		switch cfg.Type {
		case "progressBar":
			a.visualProgressBarStyle(c, cfg)
		case "progressRing":
			a.visualProgressRingStyle(c, cfg)
		case "donut":
			a.visualDonutStyle(c, cfg)
		case "clock":
			a.visualClockStyle(c, cfg)
		case "contrib":
			a.visualContribControls(c, cfg)
		case "table":
			a.visualTableStyle(c, cfg)
		case "line":
			a.visualLineStyle(c, cfg)
		}
	})
	_ = compact
}

func (a *app) colorSwatchRow(c *ui.Context, current *string, onChange func()) {
	p := pal(c)
	ui.Row(c).Gap(6).AlignItems(ui.Center).Wrap().Children(func() {
		for _, hex := range visualSwatches {
			hex := hex
			sel := strings.EqualFold(*current, hex)
			el := ui.ButtonBase(c).Size(18, 18).Radius(9).Background(ui.Hex(hex)).Cursor(ui.CursorPointer)
			if sel {
				el = el.BorderWidth(2).BorderColor(p.text)
			}
			if el.Clicked() {
				*current = hex
				onChange()
			}
		}
	})
}

func (a *app) visualProgressBarStyle(c *ui.Context, cfg *visualConfig) {
	label(c, tr("进度条样式"))
	styles := []string{"neon", "gradient", "solid", "segmented"}
	styleIdx := indexOf(styles, cfg.Bar.Style, 0)
	tracks := []string{"dark", "tint", "neutral", "none"}
	trackIdx := indexOf(tracks, cfg.Bar.Track, 0)
	changed := false

	ui.Row(c).Gap(14).Wrap().Children(func() {
		field(c, tr("进度条风格"), "", func() {
			if choose(c, &styleIdx, tr("霓虹灯"), tr("渐变"), tr("纯色"), tr("分段")) {
				cfg.Bar.Style = styles[styleIdx]
				if cfg.Bar.Style == "neon" && cfg.Bar.Track == "" {
					cfg.Bar.Track = "dark"
				}
				changed = true
			}
		})
		field(c, tr("底槽样式"), "", func() {
			if choose(c, &trackIdx, tr("深色"), tr("同色"), tr("浅灰"), tr("无")) {
				cfg.Bar.Track = tracks[trackIdx]
				changed = true
			}
		})
	})

	ui.Row(c).Gap(16).AlignItems(ui.Start).Wrap().Children(func() {
		h := float64(cfg.Bar.Height)
		if h <= 0 {
			h = 10
		}
		field(c, fmt.Sprintf(tr("高度 %d"), int(h)), "", func() {
			if slider(c, &h, 6, 20).Width(120).Changed() {
				cfg.Bar.Height = float32(h)
				changed = true
			}
		})
		if cfg.Bar.Style == "neon" || cfg.Bar.Style == "segmented" {
			g := float64(cfg.Bar.Glow)
			if g <= 0 {
				g = 1.0
			}
			field(c, fmt.Sprintf(tr("发光强度 %.1f"), g), "", func() {
				if slider(c, &g, 0.3, 1.5).Width(120).Changed() {
					cfg.Bar.Glow = float32(g)
					changed = true
				}
			})
		}
		if cfg.Bar.Style == "segmented" {
			seg := float64(cfg.Bar.Segments)
			if seg <= 0 {
				seg = 10
			}
			field(c, fmt.Sprintf(tr("分段数 %d"), int(seg)), "", func() {
				if slider(c, &seg, 4, 24).Width(110).Changed() {
					cfg.Bar.Segments = int(seg)
					changed = true
				}
			})
		}
		field(c, tr("隐藏数值标签"), "", func() {
			ui.Row(c).AlignItems(ui.Center).Height(26).Children(func() {
				if toggle(c, &cfg.Bar.HideLabel) {
					changed = true
				}
			})
		})
	})
	if changed {
		a.applyVisualConfig()
	}
}

func (a *app) visualProgressItems(c *ui.Context, cfg *visualConfig, heading, addLabel string) {
	p := pal(c)
	label(c, heading)
	changed := false
	delIdx := -1
	for i := range cfg.Progress {
		i := i
		it := &cfg.Progress[i]
		ui.Column(c).Key(fmt.Sprintf("pb-%d", i)).Gap(6).Padding(10).Radius(radControl).Background(p.chip).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
				ui.Row(c).Grow(1).MinWidth(110).Children(func() {
					if textInput(c, &it.Name, tr("名称")).Changed() {
						changed = true
					}
				})
				if numberInput(c, &it.Value, 0, 1e9, 1).Width(84).Changed() {
					changed = true
				}
				ui.Text(c, "/").FontSize(fsSmall).TextColor(p.muted)
				if numberInput(c, &it.Max, 0, 1e9, 1).Width(84).Changed() {
					changed = true
				}
				if textInput(c, &it.Unit, tr("单位")).Width(64).Changed() {
					changed = true
				}
				if len(cfg.Progress) > 1 && action.ActionIcon(c, icon("x"), action.ActionIconProps{Size: theme.Small, Title: tr("删除")}).Clicked() {
					delIdx = i
				}
			})
			a.colorSwatchRow(c, &it.Color, func() { changed = true })
		})
	}
	if delIdx >= 0 {
		cfg.Progress = append(cfg.Progress[:delIdx], cfg.Progress[delIdx+1:]...)
		changed = true
	}
	ui.Row(c).Children(func() {
		if btn(c, addLabel).Clicked() {
			col := visualSwatches[len(cfg.Progress)%len(visualSwatches)]
			cfg.Progress = append(cfg.Progress, engine.Progress{Name: fmt.Sprintf(tr("目标 %d"), len(cfg.Progress)+1), Value: 60, Max: 100, Color: col})
			changed = true
		}
	})
	if changed {
		a.applyVisualConfig()
	}
}

func (a *app) visualProgressRingStyle(c *ui.Context, cfg *visualConfig) {
	label(c, tr("完成度环样式"))
	layouts := []string{"row", "concentric"}
	layIdx := indexOf(layouts, cfg.Ring.Layout, 0)
	fills := []string{"gradient", "solid"}
	fillIdx := indexOf(fills, cfg.Ring.Fill, 0)
	changed := false
	ui.Row(c).Gap(14).Wrap().Children(func() {
		field(c, tr("排列方式"), "", func() {
			if choose(c, &layIdx, tr("并排"), tr("同心圆")) {
				cfg.Ring.Layout = layouts[layIdx]
				changed = true
			}
		})
		field(c, tr("填充方式"), "", func() {
			if choose(c, &fillIdx, tr("渐变"), tr("纯色")) {
				cfg.Ring.Fill = fills[fillIdx]
				changed = true
			}
		})
	})
	if changed {
		a.applyVisualConfig()
	}
}

func (a *app) visualDonutStyle(c *ui.Context, cfg *visualConfig) {
	label(c, tr("环形占比样式"))
	changed := false
	ui.Row(c).Gap(10).Wrap().AlignItems(ui.Start).Children(func() {
		ui.Column(c).Grow(1).MinWidth(120).Children(func() {
			field(c, tr("中心文字"), "", func() {
				if textInput(c, &cfg.Donut.Center, tr("留空显示合计")).Changed() {
					changed = true
				}
			})
		})
		ui.Column(c).Grow(1).MinWidth(120).Children(func() {
			field(c, tr("中心说明"), "", func() {
				if textInput(c, &cfg.Donut.Sub, tr("例如 总计")).Changed() {
					changed = true
				}
			})
		})
		field(c, tr("霓虹微光"), "", func() {
			ui.Row(c).AlignItems(ui.Center).Height(32).Children(func() {
				if toggle(c, &cfg.Donut.Neon) {
					changed = true
				}
			})
		})
	})
	if changed {
		a.applyVisualConfig()
	}
}

func (a *app) visualDonutItems(c *ui.Context, cfg *visualConfig) {
	p := pal(c)
	label(c, tr("分类条目"))
	changed := false
	delIdx := -1
	for i := range cfg.Slices {
		i := i
		sl := &cfg.Slices[i]
		ui.Column(c).Key(fmt.Sprintf("dn-%d", i)).Gap(6).Padding(10).Radius(radControl).Background(p.chip).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
				ui.Row(c).Grow(1).MinWidth(100).Children(func() {
					if textInput(c, &sl.Name, tr("分类名")).Changed() {
						changed = true
					}
				})
				if numberInput(c, &sl.Value, 0, 1e9, 10).Width(90).Changed() {
					changed = true
				}
				if textInput(c, &sl.Unit, tr("单位")).Width(60).Changed() {
					changed = true
				}
				if len(cfg.Slices) > 1 && action.ActionIcon(c, icon("x"), action.ActionIconProps{Size: theme.Small, Title: tr("删除")}).Clicked() {
					delIdx = i
				}
			})
			a.colorSwatchRow(c, &sl.Color, func() { changed = true })
		})
	}
	if delIdx >= 0 {
		cfg.Slices = append(cfg.Slices[:delIdx], cfg.Slices[delIdx+1:]...)
		changed = true
	}
	if btn(c, tr("+ 添加分类")).Clicked() {
		col := visualSwatches[len(cfg.Slices)%len(visualSwatches)]
		cfg.Slices = append(cfg.Slices, engine.Slice{Name: fmt.Sprintf(tr("分类 %d"), len(cfg.Slices)+1), Value: 100, Color: col})
		changed = true
	}
	if changed {
		a.applyVisualConfig()
	}
}

func (a *app) visualChecklistControls(c *ui.Context, cfg *visualConfig) {
	p := pal(c)
	label(c, tr("待办清单条目"))
	changed := false
	delIdx := -1
	for i := range cfg.Checklist {
		i := i
		it := &cfg.Checklist[i]
		ui.Column(c).Key(fmt.Sprintf("cl-%d", i)).Gap(6).Padding(10).Radius(radControl).Background(p.chip).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
				if toggle(c, &it.Done) {
					changed = true
				}
				ui.Row(c).Grow(1).MinWidth(120).Children(func() {
					if textInput(c, &it.Label, tr("待办内容")).Changed() {
						changed = true
					}
				})
				if textInput(c, &it.Badge, tr("标签")).Width(80).Changed() {
					changed = true
				}
				if len(cfg.Checklist) > 1 && action.ActionIcon(c, icon("x"), action.ActionIconProps{Size: theme.Small, Title: tr("删除")}).Clicked() {
					delIdx = i
				}
			})
		})
	}
	if delIdx >= 0 {
		cfg.Checklist = append(cfg.Checklist[:delIdx], cfg.Checklist[delIdx+1:]...)
		changed = true
	}
	if btn(c, tr("+ 添加待办")).Clicked() {
		cfg.Checklist = append(cfg.Checklist, engine.CheckItem{Label: tr("新待办事项")})
		changed = true
	}
	if changed {
		a.applyVisualConfig()
	}
}

func (a *app) visualKVControls(c *ui.Context, cfg *visualConfig) {
	p := pal(c)
	label(c, tr("状态面板指标"))
	statuses := []string{"ok", "warn", "err", "info"}
	changed := false
	delIdx := -1
	for i := range cfg.KV {
		i := i
		it := &cfg.KV[i]
		stIdx := indexOf(statuses, it.Status, 0)
		ui.Column(c).Key(fmt.Sprintf("kv-%d", i)).Gap(6).Padding(10).Radius(radControl).Background(p.chip).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
				ui.Row(c).Grow(1).MinWidth(100).Children(func() {
					if textInput(c, &it.Label, tr("名称")).Changed() {
						changed = true
					}
				})
				if textInput(c, &it.Value, tr("数值")).Width(90).Changed() {
					changed = true
				}
				if textInput(c, &it.Badge, tr("标签")).Width(72).Changed() {
					changed = true
				}
				if len(cfg.KV) > 1 && action.ActionIcon(c, icon("x"), action.ActionIconProps{Size: theme.Small, Title: tr("删除")}).Clicked() {
					delIdx = i
				}
			})
			if choose(c, &stIdx, tr("正常"), tr("警告"), tr("异常"), tr("信息")) {
				it.Status = statuses[stIdx]
				changed = true
			}
		})
	}
	if delIdx >= 0 {
		cfg.KV = append(cfg.KV[:delIdx], cfg.KV[delIdx+1:]...)
		changed = true
	}
	if btn(c, tr("+ 添加指标")).Clicked() {
		cfg.KV = append(cfg.KV, engine.KVItem{Label: tr("新指标"), Value: "OK", Status: "ok"})
		changed = true
	}
	if changed {
		a.applyVisualConfig()
	}
}

func (a *app) visualCountdownControls(c *ui.Context, cfg *visualConfig) {
	p := pal(c)
	label(c, tr("倒计时目标"))
	styles := []string{"neon", "gradient", "solid"}
	changed := false
	delIdx := -1
	for i := range cfg.Countdown {
		i := i
		it := &cfg.Countdown[i]
		stIdx := indexOf(styles, it.Style, 0)
		ui.Column(c).Key(fmt.Sprintf("cd-%d", i)).Gap(6).Padding(10).Radius(radControl).Background(p.chip).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
				ui.Row(c).Grow(1).MinWidth(110).Children(func() {
					if textInput(c, &it.Label, tr("事件名")).Changed() {
						changed = true
					}
				})
				if textInput(c, &it.Target, "YYYY-MM-DD").Width(120).Changed() {
					changed = true
				}
				if textInput(c, &it.Start, tr("开始日期(可选)")).Width(120).Changed() {
					changed = true
				}
				if len(cfg.Countdown) > 1 && action.ActionIcon(c, icon("x"), action.ActionIconProps{Size: theme.Small, Title: tr("删除")}).Clicked() {
					delIdx = i
				}
			})
			ui.Row(c).Gap(10).AlignItems(ui.Center).Wrap().Children(func() {
				if choose(c, &stIdx, tr("霓虹灯"), tr("渐变"), tr("纯色")) {
					it.Style = styles[stIdx]
					changed = true
				}
				a.colorSwatchRow(c, &it.Color, func() { changed = true })
			})
		})
	}
	if delIdx >= 0 {
		cfg.Countdown = append(cfg.Countdown[:delIdx], cfg.Countdown[delIdx+1:]...)
		changed = true
	}
	if btn(c, tr("+ 添加倒计时")).Clicked() {
		col := visualSwatches[len(cfg.Countdown)%len(visualSwatches)]
		cfg.Countdown = append(cfg.Countdown, engine.CountdownItem{Label: tr("新纪念日"), Target: fmt.Sprintf("%d-12-31", a.now().Year()), Color: col, Style: "neon"})
		changed = true
	}
	if changed {
		a.applyVisualConfig()
	}
}

func (a *app) visualClockStyle(c *ui.Context, cfg *visualConfig) {
	label(c, tr("时钟样式"))
	styles := []string{"analog", "digital", "world"}
	stIdx := indexOf(styles, cfg.Clock.Style, 0)
	changed := false
	ui.Row(c).Gap(14).AlignItems(ui.Start).Wrap().Children(func() {
		field(c, tr("表盘风格"), "", func() {
			if choose(c, &stIdx, tr("指针表盘"), tr("数字"), tr("世界时钟")) {
				cfg.Clock.Style = styles[stIdx]
				changed = true
			}
		})
		field(c, tr("霓虹发光"), "", func() {
			ui.Row(c).AlignItems(ui.Center).Height(28).Children(func() {
				if toggle(c, &cfg.Clock.Neon) {
					changed = true
				}
			})
		})
		field(c, tr("显示秒"), "", func() {
			ui.Row(c).AlignItems(ui.Center).Height(28).Children(func() {
				if toggle(c, &cfg.Clock.Seconds) {
					changed = true
				}
			})
		})
	})
	field(c, tr("主题色"), "", func() {
		a.colorSwatchRow(c, &cfg.Clock.Color, func() { changed = true })
	})
	if changed {
		a.applyVisualConfig()
	}
}

func (a *app) visualClockZones(c *ui.Context, cfg *visualConfig) {
	p := pal(c)
	label(c, tr("世界时区列表"))
	if cfg.Clock.Style != "world" {
		hint(c, tr("当前为单表盘模式；可在「样式」切换为「世界时钟」以展示下方多时区"))
	}
	changed := false
	delIdx := -1
	for i := range cfg.Clock.Zones {
		i := i
		z := &cfg.Clock.Zones[i]
		ui.Row(c).Key(fmt.Sprintf("tz-%d", i)).Gap(8).AlignItems(ui.Center).Padding(6, 10).Radius(radControl).Background(p.chip).Children(func() {
			ui.Row(c).Grow(1).MinWidth(100).Children(func() {
				if textInput(c, &z.Label, tr("城市名")).Changed() {
					changed = true
				}
			})
			ui.Text(c, "UTC").FontSize(fsTiny).TextColor(p.muted)
			if numberInput(c, &z.Offset, -12, 14, 1).Width(76).Changed() {
				changed = true
			}
			if len(cfg.Clock.Zones) > 1 && action.ActionIcon(c, icon("x"), action.ActionIconProps{Size: theme.Small, Title: tr("删除")}).Clicked() {
				delIdx = i
			}
		})
	}
	if delIdx >= 0 {
		cfg.Clock.Zones = append(cfg.Clock.Zones[:delIdx], cfg.Clock.Zones[delIdx+1:]...)
		changed = true
	}
	if btn(c, tr("+ 添加时区")).Clicked() {
		cfg.Clock.Zones = append(cfg.Clock.Zones, engine.ClockZone{Label: tr("新加坡"), Offset: 8})
		changed = true
	}
	if changed {
		a.applyVisualConfig()
	}
}

func (a *app) visualContribControls(c *ui.Context, cfg *visualConfig) {
	label(c, tr("打卡热力格样式"))
	changed := false
	ui.Row(c).Gap(14).AlignItems(ui.Start).Wrap().Children(func() {
		field(c, tr("主题色"), "", func() {
			ui.Row(c).AlignItems(ui.Center).Height(32).Children(func() {
				a.colorSwatchRow(c, &cfg.Contrib.Color, func() { changed = true })
			})
		})
		field(c, tr("高亮边框"), "", func() {
			ui.Row(c).AlignItems(ui.Center).Height(32).Children(func() {
				if toggle(c, &cfg.Contrib.Neon) {
					changed = true
				}
			})
		})
		field(c, tr("单位"), "", func() {
			if textInput(c, &cfg.Contrib.Unit, tr("次")).Width(72).Changed() {
				changed = true
			}
		})
	})
	if changed {
		a.applyVisualConfig()
	}
}

func (a *app) visualLineItems(c *ui.Context, cfg *visualConfig) {
	label(c, tr("折线数据配置"))
	changed := false
	field(c, tr("系列名"), "", func() {
		if textInput(c, &cfg.LineLabel, tr("例如 评分")).Changed() {
			changed = true
		}
	})
	field(c, tr("数值序列（逗号分隔）"), "", func() {
		if textInput(c, &cfg.LinePts, "10, 20, 18, 25").Changed() {
			changed = true
		}
	})
	if changed {
		a.applyVisualConfig()
	}
}

func (a *app) visualLineStyle(c *ui.Context, cfg *visualConfig) {
	label(c, tr("折线样式"))
	changed := false
	field(c, tr("线条颜色"), "", func() {
		a.colorSwatchRow(c, &cfg.LineColor, func() { changed = true })
	})
	if changed {
		a.applyVisualConfig()
	}
}

func (a *app) visualTableStyle(c *ui.Context, cfg *visualConfig) {
	p := pal(c)
	label(c, tr("表格样式"))
	changed := false
	ui.Row(c).Gap(14).AlignItems(ui.Center).Wrap().Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
			ui.Text(c, tr("允许搜索")).FontSize(fsSmall).TextColor(p.muted)
			if toggle(c, &cfg.Table.Searchable) {
				changed = true
			}
		})
		ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
			ui.Text(c, tr("斑马纹")).FontSize(fsSmall).TextColor(p.muted)
			if toggle(c, &cfg.Table.Striped) {
				changed = true
			}
		})
	})
	if changed {
		a.applyVisualConfig()
	}
}

func (a *app) visualTableItems(c *ui.Context, cfg *visualConfig) {
	label(c, tr("数据表格配置"))
	changed := false
	if len(cfg.Table.Columns) == 0 {
		cfg.Table.Columns = []engine.TableColumn{{Title: tr("名称")}, {Title: tr("数值"), Align: "right", Format: "number"}}
		changed = true
	}
	n := len(cfg.Table.Columns)
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		for j := range cfg.Table.Columns {
			j := j
			if textInput(c.Key(fmt.Sprintf("vtcol-%d", j)), &cfg.Table.Columns[j].Title, tr("列名")).Grow(1).MinWidth(70).Changed() {
				changed = true
			}
		}
		ui.Spacer(c).Width(28)
	})
	delIdx := -1
	for i := range cfg.Table.Rows {
		i := i
		for len(cfg.Table.Rows[i]) < n {
			cfg.Table.Rows[i] = append(cfg.Table.Rows[i], "")
		}
		ui.Row(c.Key(fmt.Sprintf("vtrow-%d", i))).Gap(6).AlignItems(ui.Center).Children(func() {
			for j := 0; j < n; j++ {
				j := j
				if textInput(c.Key(fmt.Sprintf("vtcell-%d-%d", i, j)), &cfg.Table.Rows[i][j], "").Grow(1).MinWidth(70).Changed() {
					changed = true
				}
			}
			if len(cfg.Table.Rows) > 1 && action.ActionIcon(c, icon("x"), action.ActionIconProps{Size: theme.Small, Title: tr("删除")}).Clicked() {
				delIdx = i
			}
		})
	}
	if delIdx >= 0 {
		cfg.Table.Rows = append(cfg.Table.Rows[:delIdx], cfg.Table.Rows[delIdx+1:]...)
		changed = true
	}
	ui.Row(c).Gap(8).Wrap().Children(func() {
		if btn(c, tr("+ 添加一行")).Clicked() {
			cfg.Table.Rows = append(cfg.Table.Rows, make([]string, n))
			changed = true
		}
		if len(cfg.Table.Columns) < 6 && btn(c, tr("+ 添加一列")).Clicked() {
			cfg.Table.Columns = append(cfg.Table.Columns, engine.TableColumn{Title: fmt.Sprintf(tr("列 %d"), len(cfg.Table.Columns)+1)})
			changed = true
		}
	})
	if changed {
		a.applyVisualConfig()
	}
}
