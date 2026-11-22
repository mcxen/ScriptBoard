package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"scriptboard/src/internal/engine"
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
	Bento     engine.BentoSpec       `json:"bento,omitempty"`
	LineLabel string                 `json:"lineLabel,omitempty"`
	LineColor string                 `json:"lineColor,omitempty"`
	LinePts   string                 `json:"linePts,omitempty"`
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
	{"progressBar", "zap", "霓虹进度条", "Neon bar", "霓虹发光 / 点阵灯 / 渐变 / 分段", "#00F5D4"},
	{"progressRing", "gauge", "完成度环", "Progress rings", "并排或同心活动圆环", "#F43F5E"},
	{"donut", "circle-dollar-sign", "环形占比", "Donut share", "多分类占比环图与图例", "#3B82F6"},
	{"checklist", "activity", "待办清单", "Checklist", "勾选状态、徽章与完成进度", "#10B981"},
	{"kv", "file-spreadsheet", "状态面板", "Status KV", "键值指标与状态指示灯", "#38BDF8"},
	{"table", "file-spreadsheet", "数据表格", "Data table", "排序、筛选、徽章与涨跌着色", "#2563EB"},
	{"countdown", "bell", "倒计时", "Countdown", "目标日期倒数与进度条", "#A855F7"},
	{"clock", "calendar-days", "时钟表盘", "Clock dial", "指针表盘 / 数字 / 多时区", "#F59E0B"},
	{"contrib", "sparkles", "打卡热力格", "Activity grid", "7 行习惯打卡活跃矩阵", "#22C55E"},
	{"line", "chart-line", "折线图表", "Line chart", "经典数值走势曲线", "#3B82F6"},
	{"bento", "layout-grid", "Bento 看板", "Bento board", "主信息 + 探针点阵 + 汇总胶囊嵌套", "#22C55E"},
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
	case "bento":
		return visualConfig{
			Type:        "bento",
			Name:        pick("Bento 看板", "Bento dashboard"),
			Description: pick("主指标、探针点阵与汇总胶囊嵌套看板", "Nested Bento board with hero, probes, and pills"),
			Icon:        "layout-grid",
			Big:         "203.0.113.10",
			Sub:         pick("网络与节点状态", "Network & edge status"),
			Layout:      engine.Layout{Legend: "hide", Summary: "hide", Stats: "hide", Actions: "hover", Title: "hide"},
			Bento: engine.BentoSpec{
				Maskable: true,
				Hero: &engine.BentoHero{
					Label:    "IPV4",
					Flag:     "cn",
					Value:    "203.0.113.10",
					Location: pick("中国 上海 电信", "China Shanghai Telecom"),
					Meta:     pick("家庭宽带 (ISP)", "Residential (ISP)"),
				},
				Probes: &engine.BentoProbeGroup{
					Title:      pick("网络连通性", "Connectivity"),
					ActionText: pick("查看更多 ›", "More ›"),
					Columns:    3,
					Items: []engine.BentoProbe{
						{Name: "GitHub", Icon: "github", Latency: 48, Dots: []int{45, 49, 48, 52, 46, 50, 48, 47, 51, 49, 48, 50}},
						{Name: "Cloudflare", Icon: "cloudflare", Latency: 26, Dots: []int{25, 27, 26, 28, 24, 26, 25, 27, 26, 25, 28, 26}},
						{Name: "YouTube", Icon: "youtube", Latency: 168, Dots: []int{165, 172, 168, 170, 164, 175, 168, 169, 171, 166, 168, 170}},
					},
				},
				Pills: &engine.BentoPillGroup{
					Title:   pick("出口汇总", "Exit Summary"),
					Columns: 3,
					Items: []engine.BentoPill{
						{Flag: "cn", Value: "203.0.113.10", Badge: pick("直连", "Direct")},
						{Flag: "jp", Value: "198.51.100.24", Badge: pick("国际", "Global")},
					},
				},
			},
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
      height: (CONFIG.bar && CONFIG.bar.height) || 0,
      track: (CONFIG.bar && CONFIG.bar.track) || "dark",
      glow: (CONFIG.bar && CONFIG.bar.glow) || 1,
      segments: (CONFIG.bar && CONFIG.bar.segments) || 10,
      rows: (CONFIG.bar && CONFIG.bar.rows) || 0,
      cols: (CONFIG.bar && CONFIG.bar.cols) || 0,
      total: (CONFIG.bar && CONFIG.bar.total) || 0,
      stacked: CONFIG.bar ? CONFIG.bar.stacked : undefined,
      animate: CONFIG.bar ? CONFIG.bar.animate : undefined,
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
	case "bento":
		body = `  return {
    title: CONFIG.name,
    icon: CONFIG.icon || "layout-grid",
    big: (CONFIG.bento && CONFIG.bento.hero && CONFIG.bento.hero.value) || CONFIG.big || "",
    sub: (CONFIG.bento && CONFIG.bento.hero && CONFIG.bento.hero.location) || CONFIG.sub || "",
    lines: [],
    chart: { type: "bento", bento: CONFIG.bento },
    layout: CONFIG.layout,
    menubar: CONFIG.menubar ? CONFIG.menubarCfg : undefined,
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
