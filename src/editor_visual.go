package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/iconcat"
)

const visualMarker = "// @visual-config"

// visualConfig is the declarative JSON block embedded in a visual component
// script. Edits in the visual editor update this struct, serialize it back
// into the ES module source, and immediately refresh the live preview.
type visualConfig struct {
	Type        string          `json:"type"` // progressBar | progressRing | donut | checklist | kv | countdown | clock | contrib | table | line
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Icon        string          `json:"icon,omitempty"`
	Big         string          `json:"big,omitempty"`
	Sub         string          `json:"sub,omitempty"`
	Menubar     bool            `json:"menubar,omitempty"`
	MenuBarCfg  *engine.MenuBar `json:"menubarCfg,omitempty"`
	Platform    string          `json:"platform,omitempty"` // "" (all) | desktop | mobile
	Layout      engine.Layout   `json:"layout,omitempty"`

	Bar       engine.BarStyle        `json:"bar,omitempty"`
	Ring      engine.RingStyle       `json:"ring,omitempty"`
	Progress  []engine.Progress      `json:"progress,omitempty"`
	Donut     engine.DonutStyle      `json:"donut,omitempty"`
	Slices    []engine.Slice         `json:"slices,omitempty"`
	Checklist []engine.CheckItem     `json:"checklist,omitempty"`
	KV        []engine.KVItem        `json:"kv,omitempty"`
	Countdown []engine.CountdownItem `json:"countdown,omitempty"`
	Clock     engine.ClockSpec       `json:"clock,omitempty"`
	Contrib   engine.ContribSpec     `json:"contrib,omitempty"`
	Table     engine.TableSpec       `json:"table,omitempty"`
	LineLabel string                 `json:"lineLabel,omitempty"`
	LineColor string                 `json:"lineColor,omitempty"`
	LinePts   string                 `json:"linePts,omitempty"`
}

// visualEditor holds UI state for the script visual configuration editor.
type visualEditor struct {
	sourceSig     string
	cfg           *visualConfig
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

type visualTemplate struct {
	Key   string
	Icon  string
	Zh    string
	En    string
	Note  string
	Color string
}

var visualTemplates = []visualTemplate{
	{"progressBar", "zap", "霓虹进度条", "Neon bar", "霓虹发光 / 渐变 / 分段", "#00F5D4"},
	{"progressRing", "gauge", "完成度环", "Progress rings", "并排或同心活动圆环", "#F43F5E"},
	{"donut", "circle-dollar-sign", "环形占比", "Donut share", "多分类占比环图与图例", "#3B82F6"},
	{"checklist", "activity", "待办清单", "Checklist", "勾选状态、徽章与完成进度", "#10B981"},
	{"kv", "file-spreadsheet", "状态面板", "Status KV", "键值指标与状态指示灯", "#38BDF8"},
	{"table", "file-spreadsheet", "数据表格", "Data table", "排序、筛选、徽章与涨跌着色", "#2563EB"},
	{"countdown", "bell", "倒计时", "Countdown", "目标日期倒数与进度条", "#A855F7"},
	{"clock", "calendar-days", "世界时钟", "World clock", "指针表盘 / 数字 / 多时区", "#F59E0B"},
	{"contrib", "sparkles", "打卡热力格", "Activity grid", "7 行习惯打卡活跃矩阵", "#22C55E"},
	{"line", "chart-line", "折线图表", "Line chart", "经典数值走势曲线", "#3B82F6"},
}

var visualSwatches = []string{
	"#00F5D4", "#38BDF8", "#FF2A85", "#A855F7", "#10B981", "#FB923C", "#F59E0B", "#3B82F6",
}

// defaultVisualConfig returns a ready-to-preview config for template key.
func (a *app) defaultVisualConfig(key string) visualConfig {
	en := curLang() == langEN
	pick := func(zh, e string) string {
		if en {
			return e
		}
		return zh
	}
	today := a.now()
	nextYear := fmt.Sprintf("%d-01-01", today.Year()+1)
	thisYearStart := fmt.Sprintf("%d-01-01", today.Year())

	switch key {
	case "progressBar":
		return visualConfig{
			Type:        "progressBar",
			Name:        pick("霓虹进度条", "Neon progress"),
			Description: pick("霓虹灯风格目标进度展示", "Glowing neon goal progress bars"),
			Icon:        "zap",
			Big:         "78%",
			Sub:         pick("本周目标达成", "Weekly goals"),
			Layout:      engine.Layout{Legend: "hide", Summary: "top", Stats: "hide", Actions: "hover", Title: "hide"},
			Bar:         engine.BarStyle{Style: "neon", Height: 10, Track: "dark", Glow: 1.0, Segments: 10},
			Progress: []engine.Progress{
				{Name: pick("季度 OKR", "Quarterly OKR"), Value: 78, Max: 100, Unit: "%", Color: "#00F5D4", To: "#38BDF8"},
				{Name: pick("每日专注", "Daily focus"), Value: 4.5, Max: 6, Unit: "h", Color: "#FF2A85", To: "#FB923C"},
				{Name: pick("年度阅读", "Reading"), Value: 18, Max: 24, Unit: pick("本", " books"), Color: "#A855F7", To: "#38BDF8"},
			},
		}
	case "progressRing":
		return visualConfig{
			Type:        "progressRing",
			Name:        pick("活动圆环", "Activity rings"),
			Description: pick("同心或并排展示多项目标完成度", "Concentric or side-by-side completion rings"),
			Icon:        "gauge",
			Big:         "",
			Sub:         pick("今日运动与习惯", "Today's activity"),
			Layout:      engine.Layout{Legend: "hide", Summary: "top", Stats: "hide", Actions: "hover", Title: "hide"},
			Ring:        engine.RingStyle{Layout: "concentric", Track: "tint", Fill: "gradient", Cap: "round", Center: "percent", Align: "center", VAlign: "middle"},
			Progress: []engine.Progress{
				{Name: pick("活动", "Move"), Value: 480, Max: 600, Unit: " kcal", Color: "#F43F5E", To: "#FB923C"},
				{Name: pick("锻炼", "Exercise"), Value: 36, Max: 30, Unit: " min", Color: "#84CC16", To: "#22C55E"},
				{Name: pick("专注", "Focus"), Value: 5, Max: 6, Unit: "h", Color: "#22D3EE", To: "#3B82F6"},
			},
		}
	case "donut":
		return visualConfig{
			Type:        "donut",
			Name:        pick("支出分布", "Spending split"),
			Description: pick("分类占比环形图", "Category breakdown donut"),
			Icon:        "circle-dollar-sign",
			Big:         "",
			Sub:         pick("本月分类汇总", "Monthly breakdown"),
			Layout:      engine.Layout{Legend: "hide", Summary: "top", Stats: "hide", Actions: "hover", Title: "hide"},
			Donut:       engine.DonutStyle{Center: pick("¥6,420", "$6,420"), Sub: pick("总支出", "Total"), Thickness: 0.28, Neon: true},
			Slices: []engine.Slice{
				{Name: pick("餐饮", "Food"), Value: 2180, Color: "#00F5D4", Unit: pick("元", "")},
				{Name: pick("居住", "Housing"), Value: 2400, Color: "#38BDF8", Unit: pick("元", "")},
				{Name: pick("交通", "Transit"), Value: 860, Color: "#A855F7", Unit: pick("元", "")},
				{Name: pick("娱乐", "Leisure"), Value: 980, Color: "#FF2A85", Unit: pick("元", "")},
			},
		}
	case "checklist":
		return visualConfig{
			Type:        "checklist",
			Name:        pick("今日待办", "Today's tasks"),
			Description: pick("待办事项与完成进度清单", "Task checklist with status badges"),
			Icon:        "activity",
			Big:         "",
			Sub:         pick("每日核心清单", "Daily essentials"),
			Layout:      engine.Layout{Legend: "hide", Summary: "top", Stats: "hide", Actions: "hover", Title: "hide"},
			Checklist: []engine.CheckItem{
				{Label: pick("晨间复盘与排期", "Morning planning"), Done: true, Badge: "09:00", Color: "#10B981"},
				{Label: pick("完成核心需求评审", "Finish design review"), Done: true, Badge: pick("工作", "Work"), Color: "#38BDF8"},
				{Label: pick("30 分钟有氧运动", "30-min cardio"), Done: false, Note: pick("晚饭后", "After dinner"), Badge: pick("健康", "Health"), Color: "#F59E0B"},
				{Label: pick("阅读 20 页书籍", "Read 20 pages"), Done: false, Badge: pick("习惯", "Habit"), Color: "#A855F7"},
			},
		}
	case "kv":
		return visualConfig{
			Type:        "kv",
			Name:        pick("服务状态", "Service status"),
			Description: pick("核心指标与服务健康键值面板", "Key-value metrics and service health"),
			Icon:        "file-spreadsheet",
			Big:         "99.98%",
			Sub:         pick("全部核心服务在线", "All core services online"),
			Layout:      engine.Layout{Legend: "hide", Summary: "top", Stats: "hide", Actions: "hover", Title: "hide"},
			KV: []engine.KVItem{
				{Label: pick("API 网关", "API Gateway"), Value: "24 ms", Sub: "Tokyo · edge", Badge: "OK", Status: "ok"},
				{Label: pick("主数据库", "Primary DB"), Value: "42%", Sub: pick("连接池占用", "Pool usage"), Status: "ok"},
				{Label: pick("任务队列", "Job Queue"), Value: "18", Sub: pick("待处理任务", "Pending jobs"), Badge: pick("偏高", "Warm"), Status: "warn"},
				{Label: pick("今日请求", "Requests today"), Value: "128.4K", Status: "info"},
			},
		}
	case "table":
		return visualConfig{
			Type:        "table",
			Name:        pick("行情数据表", "Market table"),
			Description: pick("带排序、筛选与涨跌着色的结构化表格", "Sortable, searchable data table with status and delta badges"),
			Icon:        "file-spreadsheet",
			Big:         pick("3 项", "3 items"),
			Sub:         pick("点击表头排序，点单元格复制", "Click header to sort, cell to copy"),
			Layout:      engine.Layout{Legend: "hide", Summary: "top", Stats: "hide", Actions: "hover", Title: "hide"},
			Table: engine.TableSpec{
				Searchable: true,
				Striped:    true,
				Compact:    4,
				Columns: []engine.TableColumn{
					{Title: pick("名称", "Name")},
					{Title: pick("数值", "Value"), Align: "right", Format: "number"},
					{Title: pick("涨跌", "Change"), Align: "right", Format: "delta"},
					{Title: pick("状态", "Status"), Format: "status"},
				},
				Rows: [][]string{
					{pick("黄金", "Gold"), "638.5", "+1.24%", "ok"},
					{pick("纳斯达克", "Nasdaq"), "19240", "+0.85%", "ok"},
					{pick("美元/人民币", "USD/CNY"), "7.23", "-0.18%", "info"},
				},
			},
		}
	case "countdown":
		return visualConfig{
			Type:        "countdown",
			Name:        pick("新年倒计时", "New Year countdown"),
			Description: pick("重要日程与纪念日倒计时", "Countdown to target dates with progress"),
			Icon:        "bell",
			Big:         "",
			Sub:         "",
			Layout:      engine.Layout{Legend: "hide", Summary: "hide", Stats: "hide", Actions: "hover", Title: "hide"},
			Countdown: []engine.CountdownItem{
				{Label: pick("距离下一年", "Until next year"), Target: nextYear, Start: thisYearStart, Note: pick("时光进度", "Year progress"), Color: "#00F5D4", To: "#38BDF8", Style: "neon"},
			},
		}
	case "clock":
		return visualConfig{
			Type:        "clock",
			Name:        pick("世界时钟", "World clock"),
			Description: pick("模拟表盘、数字或世界时区时钟", "Analog dial, digital or multi-timezone clock"),
			Icon:        "calendar-days",
			Big:         "",
			Sub:         "",
			Layout:      engine.Layout{Legend: "hide", Summary: "hide", Stats: "hide", Actions: "hover", Title: "hide"},
			Clock: engine.ClockSpec{
				Style:   "analog",
				Seconds: true,
				Color:   "#00F5D4",
				Neon:    true,
				Zones: []engine.ClockZone{
					{Label: pick("北京 / 上海", "Shanghai"), Offset: 8},
					{Label: pick("东京", "Tokyo"), Offset: 9},
					{Label: pick("伦敦", "London"), Offset: 0},
					{Label: pick("纽约", "New York"), Offset: -5},
				},
			},
		}
	case "contrib":
		cells := make([]engine.ContribCell, 0, 84)
		for i := 83; i >= 0; i-- {
			d := today.AddDate(0, 0, -i)
			v := float64((i*7 + 3) % 5)
			if i%6 == 0 {
				v = 0
			}
			cells = append(cells, engine.ContribCell{Date: d.Format("2006-01-02"), Value: v})
		}
		return visualConfig{
			Type:        "contrib",
			Name:        pick("习惯打卡", "Habit tracker"),
			Description: pick("近 12 周每日打卡活跃热力格", "12-week daily habit contribution grid"),
			Icon:        "sparkles",
			Big:         pick("连续 12 天", "12-day streak"),
			Sub:         pick("每日专注与运动", "Daily focus & exercise"),
			Layout:      engine.Layout{Legend: "hide", Summary: "top", Stats: "hide", Actions: "hover", Title: "hide"},
			Contrib:     engine.ContribSpec{Cells: cells, Color: "#00F5D4", Max: 4, Weeks: 12, Unit: pick(" 次", "x"), Neon: true},
		}
	default: // line
		return visualConfig{
			Type:        "line",
			Name:        pick("趋势曲线", "Trend curve"),
			Description: pick("自定义数值走势折线图", "Custom trend line chart"),
			Icon:        "chart-line",
			Big:         "86.4",
			Sub:         pick("近 7 期变化", "Last 7 points"),
			Layout:      engine.Layout{Legend: "hide", Summary: "top", Stats: "hide", Actions: "hover", Title: "hide"},
			LineLabel:   pick("指标", "Metric"),
			LineColor:   "#3B82F6",
			LinePts:     "72, 75.5, 74.2, 79, 82.5, 84.1, 86.4",
		}
	}
}

// parseVisualConfig reads the embedded CONFIG JSON from a visual script.
func parseVisualConfig(src string) (*visualConfig, bool) {
	if !strings.Contains(src, visualMarker) {
		return nil, false
	}
	const prefix = "const CONFIG = "
	idx := strings.Index(src, prefix)
	if idx < 0 {
		return nil, false
	}
	rest := src[idx+len(prefix):]
	end := strings.Index(rest, "\n};\n")
	var raw string
	if end >= 0 {
		raw = rest[:end+2]
	} else if end2 := strings.Index(rest, "};"); end2 >= 0 {
		raw = rest[:end2+1]
	} else {
		return nil, false
	}
	var cfg visualConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil || cfg.Type == "" {
		return nil, false
	}
	return &cfg, true
}

// renderVisualScript turns cfg into a clean, self-contained ES module script.
func renderVisualScript(cfg visualConfig) string {
	b, _ := json.MarshalIndent(cfg, "", "  ")
	plats := `["desktop", "mobile"]`
	switch cfg.Platform {
	case "desktop":
		plats = `["desktop"]`
	case "mobile":
		plats = `["mobile"]`
	}
	var body string
	switch cfg.Type {
	case "progressBar":
		// progressBar is returned directly on Result with NO chart object.
		body = `  return {
    title: CONFIG.name,
    icon: CONFIG.icon || "zap",
    big: CONFIG.big || "",
    sub: CONFIG.sub || "",
    lines: [],
    progressBar: {
      style: (CONFIG.bar && CONFIG.bar.style) || "neon",
      height: (CONFIG.bar && CONFIG.bar.height) || 10,
      track: (CONFIG.bar && CONFIG.bar.track) || "dark",
      glow: (CONFIG.bar && CONFIG.bar.glow) || 1,
      segments: (CONFIG.bar && CONFIG.bar.segments) || 10,
      hideLabel: !!(CONFIG.bar && CONFIG.bar.hideLabel),
      items: CONFIG.progress || [],
    },
    layout: CONFIG.layout,
  };`
	case "progressRing":
		body = `  return {
    title: CONFIG.name,
    icon: CONFIG.icon || "gauge",
    big: CONFIG.big || "",
    sub: CONFIG.sub || "",
    lines: [],
    chart: { type: "progressRing", ring: CONFIG.ring, progress: CONFIG.progress || [] },
    layout: CONFIG.layout,
  };`
	case "donut":
		body = `  return {
    title: CONFIG.name,
    icon: CONFIG.icon || "circle-dollar-sign",
    big: CONFIG.big || "",
    sub: CONFIG.sub || "",
    lines: [],
    chart: { type: "donut", donut: CONFIG.donut, slices: CONFIG.slices || [] },
    layout: CONFIG.layout,
  };`
	case "checklist":
		body = `  return {
    title: CONFIG.name,
    icon: CONFIG.icon || "activity",
    big: CONFIG.big || "",
    sub: CONFIG.sub || "",
    lines: [],
    chart: { type: "checklist", checklist: CONFIG.checklist || [] },
    layout: CONFIG.layout,
  };`
	case "kv":
		body = `  return {
    title: CONFIG.name,
    icon: CONFIG.icon || "file-spreadsheet",
    big: CONFIG.big || "",
    sub: CONFIG.sub || "",
    lines: [],
    chart: { type: "kv", kv: CONFIG.kv || [] },
    layout: CONFIG.layout,
    menubar: CONFIG.menubar ? CONFIG.menubarCfg : undefined,
  };`
	case "table":
		body = `  return {
    title: CONFIG.name,
    icon: CONFIG.icon || "file-spreadsheet",
    big: CONFIG.big || "",
    sub: CONFIG.sub || "",
    lines: [],
    chart: { type: "table", table: CONFIG.table },
    layout: CONFIG.layout,
    menubar: CONFIG.menubar ? CONFIG.menubarCfg : undefined,
  };`
	case "countdown":
		body = `  return {
    title: CONFIG.name,
    icon: CONFIG.icon || "bell",
    big: CONFIG.big || "",
    sub: CONFIG.sub || "",
    lines: [],
    chart: { type: "countdown", countdown: CONFIG.countdown || [] },
    layout: CONFIG.layout,
  };`
	case "clock":
		body = `  return {
    title: CONFIG.name,
    icon: CONFIG.icon || "calendar-days",
    big: CONFIG.big || "",
    sub: CONFIG.sub || "",
    lines: [],
    chart: { type: "clock", clock: CONFIG.clock },
    layout: CONFIG.layout,
  };`
	case "contrib":
		body = `  return {
    title: CONFIG.name,
    icon: CONFIG.icon || "sparkles",
    big: CONFIG.big || "",
    sub: CONFIG.sub || "",
    lines: [],
    chart: { type: "contrib", contrib: CONFIG.contrib },
    layout: CONFIG.layout,
  };`
	default: // line
		body = `  const pts = String(CONFIG.linePts || "")
    .split(/[,，\s]+/)
    .map((s) => parseFloat(s))
    .filter((n) => n === n);
  const labels = pts.map((_, i) => "#" + (i + 1));
  return {
    title: CONFIG.name,
    icon: CONFIG.icon || "chart-line",
    big: CONFIG.big || (pts.length ? String(pts[pts.length - 1]) : "--"),
    sub: CONFIG.sub || "",
    lines: [],
    chart: pts.length ? {
      type: "line",
      series: [{ name: CONFIG.lineLabel || CONFIG.name, color: CONFIG.lineColor || "#3B82F6", points: pts }],
      labels: labels,
      xLabels: [labels[0], labels[labels.length - 1]],
    } : null,
    layout: CONFIG.layout,
  };`
	}
	return fmt.Sprintf(`%s
const CONFIG = %s;

export function meta() {
  return {
    name: CONFIG.name,
    description: CONFIG.description || CONFIG.name,
    version: "1.0.0",
    platforms: %s,
    needs: [],
    icon: CONFIG.icon || "sparkles",
    menubar: !!CONFIG.menubar,
    sources: {},
  };
}

export function render(ctx) {
%s
}
`, visualMarker, string(b), plats, body)
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

// visualSwapTargets returns compatible component types that can replace cfg.Type
// in place while migrating existing data items.
func visualSwapTargets(typ string) []string {
	switch typ {
	case "progressBar":
		return []string{"progressRing", "table"}
	case "progressRing":
		return []string{"progressBar", "table"}
	case "donut":
		return []string{"kv", "table"}
	case "checklist":
		return []string{"kv", "table"}
	case "kv":
		return []string{"table", "checklist"}
	case "table":
		return []string{"kv"}
	}
	return nil
}

// swapVisualType converts a.visual.cfg to target while preserving metadata and
// migrating compatible items instead of resetting to blank defaults.
func (a *app) swapVisualType(target string) {
	if a.visual.cfg == nil || a.visual.cfg.Type == target {
		return
	}
	cur := *a.visual.cfg
	next := a.defaultVisualConfig(target)
	next.Name = cur.Name
	next.Description = cur.Description
	next.Icon = cur.Icon
	next.Big = cur.Big
	next.Sub = cur.Sub
	next.Menubar = cur.Menubar
	next.MenuBarCfg = cur.MenuBarCfg
	next.Platform = cur.Platform
	next.Layout = cur.Layout

	switch {
	case (cur.Type == "progressBar" && target == "progressRing") || (cur.Type == "progressRing" && target == "progressBar"):
		if len(cur.Progress) > 0 {
			next.Progress = append([]engine.Progress(nil), cur.Progress...)
		}
	case (cur.Type == "progressBar" || cur.Type == "progressRing") && target == "table":
		rows := make([][]string, 0, len(cur.Progress))
		for _, it := range cur.Progress {
			rows = append(rows, []string{it.Name, trimNum(it.Value) + it.Unit, trimNum(it.Max) + it.Unit, fmt.Sprintf("%.0f%%", it.Raw())})
		}
		if len(rows) > 0 {
			next.Table = engine.TableSpec{
				Searchable: true, Striped: true,
				Columns: []engine.TableColumn{
					{Title: tr("项目")},
					{Title: tr("当前"), Align: "right", Format: "number"},
					{Title: tr("目标"), Align: "right", Format: "number"},
					{Title: tr("完成度"), Align: "right", Format: "badge"},
				},
				Rows: rows,
			}
		}
	case cur.Type == "donut" && target == "kv":
		var items []engine.KVItem
		for _, sl := range cur.Slices {
			items = append(items, engine.KVItem{Label: sl.Name, Value: trimNum(sl.Value) + sl.Unit, Color: sl.Color})
		}
		if len(items) > 0 {
			next.KV = items
		}
	case cur.Type == "donut" && target == "table":
		rows := make([][]string, 0, len(cur.Slices))
		for _, sl := range cur.Slices {
			rows = append(rows, []string{sl.Name, trimNum(sl.Value), sl.Unit})
		}
		if len(rows) > 0 {
			next.Table = engine.TableSpec{
				Searchable: true, Striped: true,
				Columns: []engine.TableColumn{{Title: tr("分类")}, {Title: tr("数值"), Align: "right", Format: "number"}, {Title: tr("单位"), Align: "center"}},
				Rows:    rows,
			}
		}
	case cur.Type == "checklist" && target == "kv":
		var items []engine.KVItem
		for _, it := range cur.Checklist {
			st, val := "warn", tr("未完成")
			if it.Done {
				st, val = "ok", tr("已完成")
			}
			items = append(items, engine.KVItem{Label: it.Label, Value: val, Sub: it.Note, Badge: it.Badge, Status: st, Color: it.Color})
		}
		if len(items) > 0 {
			next.KV = items
		}
	case cur.Type == "checklist" && target == "table":
		rows := make([][]string, 0, len(cur.Checklist))
		for _, it := range cur.Checklist {
			st := tr("未完成")
			if it.Done {
				st = tr("已完成")
			}
			rows = append(rows, []string{st, it.Label, it.Badge, it.Note})
		}
		if len(rows) > 0 {
			next.Table = engine.TableSpec{
				Searchable: true, Striped: true,
				Columns: []engine.TableColumn{{Title: tr("状态"), Format: "badge"}, {Title: tr("项目")}, {Title: tr("标签"), Format: "badge"}, {Title: tr("备注")}},
				Rows:    rows,
			}
		}
	case cur.Type == "kv" && target == "table":
		rows := make([][]string, 0, len(cur.KV))
		for _, it := range cur.KV {
			rows = append(rows, []string{it.Label, it.Value, it.Status, it.Badge, it.Sub})
		}
		if len(rows) > 0 {
			next.Table = engine.TableSpec{
				Searchable: true, Striped: true,
				Columns: []engine.TableColumn{
					{Title: tr("指标")},
					{Title: tr("数值"), Align: "right"},
					{Title: tr("状态"), Format: "status"},
					{Title: tr("标签"), Format: "badge"},
					{Title: tr("说明")},
				},
				Rows: rows,
			}
		}
	case cur.Type == "kv" && target == "checklist":
		var items []engine.CheckItem
		for _, it := range cur.KV {
			items = append(items, engine.CheckItem{Label: it.Label, Done: it.Status == "ok", Note: it.Value, Badge: it.Badge, Color: it.Color})
		}
		if len(items) > 0 {
			next.Checklist = items
		}
	case cur.Type == "table" && target == "kv":
		var items []engine.KVItem
		for _, r := range cur.Table.Rows {
			lbl, val, sub := "", "", ""
			if len(r) > 0 {
				lbl = r[0]
			}
			if len(r) > 1 {
				val = r[1]
			}
			if len(r) > 2 {
				sub = strings.Join(r[2:], " · ")
			}
			items = append(items, engine.KVItem{Label: lbl, Value: val, Sub: sub})
		}
		if len(items) > 0 {
			next.KV = items
		}
	}

	a.visual.cfg = &next
	a.applyVisualConfig()
	a.status = tr("已切换组件并保留已有数据")
}

// visualScriptEditor draws the visual configuration & visual editing form for
// the currently selected script. Works for both declarative visual scripts
// (full visual item/style editing) and arbitrary hand-written/built-in scripts
// (template picker, icon picker, meta/platform/menubar/layout visual editing).
func (a *app) visualScriptEditor(c *ui.Context, compact bool) {
	a.syncVisualFromDraft()
	p := pal(c)
	ro := isPluginScript(a.draft.ID)
	if ro {
		notice(c, tr("本地插件在插件文件夹里修改，此处仅供查看"), feedback.Info)
	}

	// 1. Visual component template selector (quick build / switch type).
	if !ro {
		ui.Column(c).Gap(8).Padding(12).Radius(radControl + 2).Background(p.chip).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				label(c, tr("组件类型模板"))
				if a.visual.cfg != nil {
					chip(c, templateLabel(a.visual.cfg.Type), p.accent, p.accentSoft)
				}
				ui.Spacer(c)
				toggle := tr("选择模板")
				if a.visual.tplOpen {
					toggle = tr("收起模板")
				}
				if btn(c, toggle).Clicked() {
					a.visual.tplOpen = !a.visual.tplOpen
				}
			})
			if a.visual.cfg != nil {
				if targets := visualSwapTargets(a.visual.cfg.Type); len(targets) > 0 {
					ui.Row(c).AlignItems(ui.Center).Gap(6).Wrap().Children(func() {
						ui.Text(c, tr("原地替换（保留数据）")).FontSize(fsTiny).TextColor(p.muted)
						for _, t := range targets {
							t := t
							if btn(c.Key("vswap:"+t), tr("转为 ")+templateLabel(t)).Clicked() {
								a.swapVisualType(t)
							}
						}
					})
				}
			}
			if a.visual.tplOpen {
				renderTile := func(tpl visualTemplate) {
					active := a.visual.cfg != nil && a.visual.cfg.Type == tpl.Key
					col := parseColor(tpl.Color, p.accent)
					tile := ui.ButtonBase(c).Grow(1).MinWidth(0).Padding(8, 10).Radius(radControl).Background(p.card).
						BorderWidth(1).BorderColor(p.hairline).Justify(ui.Start).AlignItems(ui.Center).Cursor(ui.CursorPointer)
					if active {
						tile = tile.BorderWidth(1.5).BorderColor(col).Background(col.Alpha(0.08))
					}
					if tile.Clicked() {
						a.applyTemplate(tpl.Key)
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
				if compact {
					cols = 2
				}
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
		})
	}

	// 2. Basic info + visual icon picker.
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

	if !ro {
		// Headline & subtitle for visual configs, or description for general scripts.
		if cfg := a.visual.cfg; cfg != nil {
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

		// Platform & menubar visual toggles.
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

	// 3. Component-specific visual property & item editors (for @visual-config scripts).
	if a.visual.cfg != nil && !ro {
		a.visualComponentControls(c, compact)
	}

	// 4. Card appearance / layout visual configuration (works for ANY script).
	if !ro {
		a.visualLayoutControls(c)
	}
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

func (a *app) visualLayoutControls(c *ui.Context) {
	p := pal(c)
	ui.Column(c).Gap(10).Padding(12).Radius(radControl + 2).BorderWidth(1).BorderColor(p.hairline).Children(func() {
		label(c, tr("卡片外观布局"))
		changed := false
		ui.Row(c).Gap(14).Wrap().Children(func() {
			field(c, tr("卡片标题行"), "", func() {
				if choose(c, &a.visual.layoutTitle, tr("悬浮显示"), tr("固定显示")) {
					changed = true
				}
			})
			field(c, tr("摘要行"), "", func() {
				if choose(c, &a.visual.layoutSummary, tr("显示"), tr("隐藏")) {
					changed = true
				}
			})
			field(c, tr("操作按钮"), "", func() {
				if choose(c, &a.visual.layoutActions, tr("悬浮显示"), tr("常显")) {
					changed = true
				}
			})
			field(c, tr("统计块"), "", func() {
				if choose(c, &a.visual.layoutStats, tr("显示"), tr("隐藏")) {
					changed = true
				}
			})
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

func (a *app) visualComponentControls(c *ui.Context, compact bool) {
	p := pal(c)
	cfg := a.visual.cfg
	ui.Column(c).Gap(12).Padding(14).Radius(radControl + 2).BorderWidth(1).BorderColor(p.hairline).Children(func() {
		switch cfg.Type {
		case "progressBar":
			a.visualProgressBarControls(c, cfg)
		case "progressRing":
			a.visualProgressRingControls(c, cfg)
		case "donut":
			a.visualDonutControls(c, cfg)
		case "checklist":
			a.visualChecklistControls(c, cfg)
		case "kv":
			a.visualKVControls(c, cfg)
		case "countdown":
			a.visualCountdownControls(c, cfg)
		case "clock":
			a.visualClockControls(c, cfg)
		case "contrib":
			a.visualContribControls(c, cfg)
		case "table":
			a.visualTableControls(c, cfg)
		case "line":
			a.visualLineControls(c, cfg)
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

func (a *app) visualProgressBarControls(c *ui.Context, cfg *visualConfig) {
	p := pal(c)
	label(c, tr("进度条外观（无图表组件）"))
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

	label(c, tr("进度条目"))
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
		if btn(c, tr("+ 添加进度项")).Clicked() {
			col := visualSwatches[len(cfg.Progress)%len(visualSwatches)]
			cfg.Progress = append(cfg.Progress, engine.Progress{Name: fmt.Sprintf(tr("目标 %d"), len(cfg.Progress)+1), Value: 60, Max: 100, Color: col})
			changed = true
		}
	})
	if changed {
		a.applyVisualConfig()
	}
}

func (a *app) visualProgressRingControls(c *ui.Context, cfg *visualConfig) {
	p := pal(c)
	label(c, tr("完成度环配置"))
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
	delIdx := -1
	for i := range cfg.Progress {
		i := i
		it := &cfg.Progress[i]
		ui.Column(c).Key(fmt.Sprintf("pr-%d", i)).Gap(6).Padding(10).Radius(radControl).Background(p.chip).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
				ui.Row(c).Grow(1).MinWidth(100).Children(func() {
					if textInput(c, &it.Name, tr("名称")).Changed() {
						changed = true
					}
				})
				if numberInput(c, &it.Value, 0, 1e9, 1).Width(80).Changed() {
					changed = true
				}
				ui.Text(c, "/").FontSize(fsSmall).TextColor(p.muted)
				if numberInput(c, &it.Max, 0, 1e9, 1).Width(80).Changed() {
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
	if btn(c, tr("+ 添加圆环项")).Clicked() {
		col := visualSwatches[len(cfg.Progress)%len(visualSwatches)]
		cfg.Progress = append(cfg.Progress, engine.Progress{Name: fmt.Sprintf(tr("项目 %d"), len(cfg.Progress)+1), Value: 50, Max: 100, Color: col})
		changed = true
	}
	if changed {
		a.applyVisualConfig()
	}
}

func (a *app) visualDonutControls(c *ui.Context, cfg *visualConfig) {
	p := pal(c)
	label(c, tr("环形占比配置"))
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

func (a *app) visualClockControls(c *ui.Context, cfg *visualConfig) {
	p := pal(c)
	label(c, tr("时钟外观与时区"))
	styles := []string{"analog", "digital", "world"}
	stIdx := indexOf(styles, cfg.Clock.Style, 0)
	changed := false
	ui.Row(c).Gap(14).AlignItems(ui.Start).Wrap().Children(func() {
		field(c, tr("时钟样式"), "", func() {
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
	if cfg.Clock.Style == "world" {
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
	}
	if changed {
		a.applyVisualConfig()
	}
}

func (a *app) visualContribControls(c *ui.Context, cfg *visualConfig) {
	label(c, tr("打卡热力格配置"))
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

func (a *app) visualLineControls(c *ui.Context, cfg *visualConfig) {
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
	field(c, tr("线条颜色"), "", func() {
		a.colorSwatchRow(c, &cfg.LineColor, func() { changed = true })
	})
	if changed {
		a.applyVisualConfig()
	}
}

func (a *app) visualTableControls(c *ui.Context, cfg *visualConfig) {
	p := pal(c)
	label(c, tr("数据表格配置"))
	changed := false
	if len(cfg.Table.Columns) == 0 {
		cfg.Table.Columns = []engine.TableColumn{{Title: tr("名称")}, {Title: tr("数值"), Align: "right", Format: "number"}}
		changed = true
	}
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

// ---------------------------------------------------------------------------
// Source patching helpers for non-visual (custom / built-in) scripts
// ---------------------------------------------------------------------------

var (
	reIconProp      = regexp.MustCompile(`(\bicon\s*:\s*)(["'])[^"']*(["'])`)
	reMenubarProp   = regexp.MustCompile(`(\bmenubar\s*:\s*)(true|false)`)
	rePlatformsProp = regexp.MustCompile(`(\bplatforms\s*:\s*)\[[^\]]*\]`)
	reLayoutBlock   = regexp.MustCompile(`\blayout\s*:\s*\{[^}]*\}`)
)

func (a *app) applyIconChange(iconName string) {
	a.visual.metaIcon = iconName
	if a.visual.cfg != nil {
		a.visual.cfg.Icon = iconName
		a.applyVisualConfig()
		return
	}
	if reIconProp.MatchString(a.draft.Source) {
		a.draft.Source = reIconProp.ReplaceAllString(a.draft.Source, `${1}"`+iconName+`"`)
	} else {
		a.draft.Source = insertMetaField(a.draft.Source, fmt.Sprintf(`icon: %q`, iconName))
	}
	a.visual.sourceSig = a.draft.Source
	a.invalidate(draftKey)
}

func (a *app) applyMenubarChange() {
	if a.visual.cfg != nil {
		a.visual.cfg.Menubar = a.visual.metaMenubar
		a.applyVisualConfig()
		return
	}
	val := strconv.FormatBool(a.visual.metaMenubar)
	if reMenubarProp.MatchString(a.draft.Source) {
		a.draft.Source = reMenubarProp.ReplaceAllString(a.draft.Source, `${1}`+val)
	} else {
		a.draft.Source = insertMetaField(a.draft.Source, "menubar: "+val)
	}
	a.visual.sourceSig = a.draft.Source
	a.invalidate(draftKey)
}

func (a *app) applyPlatformChange() {
	plats := []string{"", "desktop", "mobile"}
	p := plats[a.visual.metaPlat]
	if a.visual.cfg != nil {
		a.visual.cfg.Platform = p
		a.applyVisualConfig()
		return
	}
	arr := `["desktop", "mobile"]`
	if p != "" {
		arr = fmt.Sprintf(`[%q]`, p)
	}
	if rePlatformsProp.MatchString(a.draft.Source) {
		a.draft.Source = rePlatformsProp.ReplaceAllString(a.draft.Source, `${1}`+arr)
	} else {
		a.draft.Source = insertMetaField(a.draft.Source, "platforms: "+arr)
	}
	a.visual.sourceSig = a.draft.Source
	a.invalidate(draftKey)
}

func insertMetaField(src, prop string) string {
	idx := strings.Index(src, "export function meta")
	if idx < 0 {
		return src
	}
	ret := strings.Index(src[idx:], "return {")
	if ret < 0 {
		return src
	}
	pos := idx + ret + len("return {")
	return src[:pos] + "\n    " + prop + "," + src[pos:]
}

func parseLayoutFromSource(src string) engine.Layout {
	m := reLayoutBlock.FindString(src)
	if m == "" {
		return engine.Layout{}
	}
	var l engine.Layout
	grab := func(k string) string {
		re := regexp.MustCompile(`\b` + k + `\s*:\s*["']([^"']+)["']`)
		if sub := re.FindStringSubmatch(m); len(sub) == 2 {
			return sub[1]
		}
		return ""
	}
	l.Title = grab("title")
	l.Summary = grab("summary")
	l.Actions = grab("actions")
	l.Legend = grab("legend")
	l.Stats = grab("stats")
	return l
}

func patchScriptLayout(src string, l engine.Layout) string {
	block := fmt.Sprintf(`layout: { title: %q, summary: %q, actions: %q, legend: %q, stats: %q }`,
		l.Title, l.Summary, l.Actions, l.Legend, l.Stats)
	if reLayoutBlock.MatchString(src) {
		return reLayoutBlock.ReplaceAllString(src, block)
	}
	idx := strings.LastIndex(src, "return {")
	if idx < 0 {
		return src
	}
	pos := idx + len("return {")
	return src[:pos] + "\n    " + block + "," + src[pos:]
}

func indexOf(list []string, want string, def int) int {
	for i, s := range list {
		if s == want {
			return i
		}
	}
	return def
}

func templateLabel(key string) string {
	for _, t := range visualTemplates {
		if t.Key == key {
			if curLang() == langEN {
				return t.En
			}
			return t.Zh
		}
	}
	return key
}
