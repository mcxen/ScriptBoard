package engine

import (
	"fmt"
	"math"
	"strings"
)

// WidgetCategory groups component types by their visual and data role.
type WidgetCategory string

const (
	CategoryPlot     WidgetCategory = "plot"
	CategoryProgress WidgetCategory = "progress"
	CategoryData     WidgetCategory = "data"
	CategoryTime     WidgetCategory = "time"
	CategoryLayout   WidgetCategory = "layout"
)

// TableColumn describes one column of a "table" component or extracted data view.
type TableColumn struct {
	Title  string `json:"title"`
	Align  string `json:"align,omitempty"`  // left (default) | right | center
	Format string `json:"format,omitempty"` // text | number | delta | badge | status
	Width  int    `json:"width,omitempty"`  // optional relative width weight (1..6)
}

// TableSpec configures a structured "table" widget component.
type TableSpec struct {
	Columns    []TableColumn `json:"columns"`
	Rows       [][]string    `json:"rows"`
	SortBy     int           `json:"sortBy,omitempty"`     // 1-based default sorted column; 0 none
	Desc       bool          `json:"desc,omitempty"`       // sort descending when SortBy > 0
	Searchable bool          `json:"searchable,omitempty"` // show filter input in full view
	Compact    int           `json:"compact,omitempty"`    // max rows on medium card (0 defaults to 4)
	Striped    bool          `json:"striped,omitempty"`    // subtle alternating row shading
}

// WidgetCapabilities declares the host behaviors supported by a component type.
type WidgetCapabilities struct {
	Plot          bool `json:"plot"`          // coordinate plot (supports line/bar/area/grid styles, 大图, crosshair)
	HasDetail     bool `json:"hasDetail"`     // renders Detail readout strip under component
	SelfHero      bool `json:"selfHero"`      // draws its own hero figure (skips duplicate Big header on cards/full)
	TopAligned    bool `json:"topAligned"`    // sits directly under summary on cards instead of bottom-pushed
	Interactive   bool `json:"interactive"`   // supports hover/click/selection interaction
	SupportsTable bool `json:"supportsTable"` // can be inspected in tabular view via ExtractTable
}

// WidgetSpec is the single source of truth describing one ScriptBoard component
// type for the engine, UI registry, visual editor, and MCP tools.
type WidgetSpec struct {
	Type         string             `json:"type"`
	Category     WidgetCategory     `json:"category"`
	Icon         string             `json:"icon"`
	ZhName       string             `json:"zhName"`
	EnName       string             `json:"enName"`
	DescZh       string             `json:"descZh"`
	DescEn       string             `json:"descEn"`
	DefaultColor string             `json:"defaultColor,omitempty"`
	Capabilities WidgetCapabilities `json:"capabilities"`
	Swappable    []string           `json:"swappable,omitempty"`
	SchemaHint   string             `json:"schemaHint,omitempty"`
}

// WidgetSpecs is the authoritative registry of all built-in component types.
var WidgetSpecs = []WidgetSpec{
	{
		Type: "line", Category: CategoryPlot, Icon: "chart-line",
		ZhName: "折线图表", EnName: "Line chart",
		DescZh:       "经典数值走势曲线，支持折线/面积/柱状/阶梯/网格等多种样式",
		DescEn:       "Time-series trend chart with line, area, bar, step, and grid styles",
		DefaultColor: "#3B82F6",
		Capabilities: WidgetCapabilities{Plot: true, HasDetail: true, Interactive: true, SupportsTable: true},
		Swappable:    []string{"line", "dot", "barh", "table"},
		SchemaHint:   `chart: { type: "line", labels: [...], series: [{ name, color, points: [...] }] }`,
	},
	{
		Type: "dot", Category: CategoryPlot, Icon: "chart-scatter",
		ZhName: "散点图", EnName: "Dot chart",
		DescZh:       "离散数据点与连线分布图",
		DescEn:       "Discrete points and connected dot series",
		DefaultColor: "#3B82F6",
		Capabilities: WidgetCapabilities{Plot: true, HasDetail: true, Interactive: true, SupportsTable: true},
		Swappable:    []string{"dot", "line", "barh", "table"},
		SchemaHint:   `chart: { type: "dot", labels: [...], series: [{ name, color, points: [...] }] }`,
	},
	{
		Type: "candle", Category: CategoryPlot, Icon: "candlestick-chart",
		ZhName: "K 线图", EnName: "Candlestick",
		DescZh:       "金融 OHLC 蜡烛图、空心 K 线、美国线与收盘线",
		DescEn:       "Financial OHLC candles, hollow candles, OHLC bars, and close lines",
		DefaultColor: "#E5484D",
		Capabilities: WidgetCapabilities{Plot: true, HasDetail: true, Interactive: true, SupportsTable: true},
		Swappable:    []string{"candle", "table"},
		SchemaHint:   `chart: { type: "candle", labels: [...], candles: [{ o, h, l, c }] }`,
	},
	{
		Type: "gauge", Category: CategoryProgress, Icon: "gauge",
		ZhName: "额度仪表环", EnName: "Quota gauge",
		DescZh:       "已用与总额度环形仪表盘，显示剩余比例与重置周期",
		DescEn:       "Used vs limit ring gauge showing remaining share and reset cycle",
		DefaultColor: "#10B981",
		Capabilities: WidgetCapabilities{TopAligned: true, SupportsTable: true},
		Swappable:    []string{"gauge", "progressBar", "table"},
		SchemaHint:   `chart: { type: "gauge", gauge: { used, limit, unit, label, reset } }`,
	},
	{
		Type: "barh", Category: CategoryData, Icon: "chart-bar-big",
		ZhName: "横向柱状图", EnName: "Horizontal bars",
		DescZh:       "多类别数值横向排名对比条",
		DescEn:       "Horizontal comparison bars across categories",
		DefaultColor: "#3B82F6",
		Capabilities: WidgetCapabilities{Interactive: true, SupportsTable: true},
		Swappable:    []string{"barh", "donut", "table"},
		SchemaHint:   `chart: { type: "barh", labels: [...], series: [{ name, points: [...] }] }`,
	},
	{
		Type: "combo", Category: CategoryPlot, Icon: "activity",
		ZhName: "组合图表", EnName: "Combo chart",
		DescZh:       "走势曲线与侧边指标环组合展示",
		DescEn:       "Trend curve combined with side indicator rings",
		DefaultColor: "#3B82F6",
		Capabilities: WidgetCapabilities{Plot: true, HasDetail: true, Interactive: true, SupportsTable: true},
		Swappable:    []string{"combo", "line", "table"},
		SchemaHint:   `chart: { type: "combo", labels: [...], series: [...], rings: [{ label, percent, text, color }] }`,
	},
	{
		Type: "heat", Category: CategoryData, Icon: "layout-grid",
		ZhName: "热力表格", EnName: "Heat grid",
		DescZh:       "二维矩阵热力着色表，点击单元格查看详情",
		DescEn:       "2D color-coded heat matrix with clickable cell details",
		DefaultColor: "#F59E0B",
		Capabilities: WidgetCapabilities{Interactive: true, SupportsTable: true},
		Swappable:    []string{"heat", "table"},
		SchemaHint:   `chart: { type: "heat", grid: { columns: [...], rows: [{ label, note, cells: [...] }], average: [...] } }`,
	},
	{
		Type: "panels", Category: CategoryPlot, Icon: "chart-column",
		ZhName: "联动多面板", EnName: "Stacked panels",
		DescZh:       "上下叠放多个子图并共享十字准星联动读数",
		DescEn:       "Vertically stacked sub-charts sharing a synchronized crosshair",
		DefaultColor: "#3B82F6",
		Capabilities: WidgetCapabilities{Plot: true, HasDetail: true, Interactive: true, SupportsTable: true},
		Swappable:    []string{"panels", "table"},
		SchemaHint:   `chart: { type: "panels", labels: [...], panels: [{ title, style, series: [...] }] }`,
	},
	{
		Type: "progress", Category: CategoryProgress, Icon: "zap",
		ZhName: "进度条（兼容）", EnName: "Progress bars",
		DescZh:       "经典完成度进度条（推荐使用独立 progressBar 字段）",
		DescEn:       "Legacy progress bars (prefer standalone progressBar field)",
		DefaultColor: "#3B82F6",
		Capabilities: WidgetCapabilities{TopAligned: true, Interactive: true, SupportsTable: true},
		Swappable:    []string{"progressBar", "progressRing", "table"},
		SchemaHint:   `chart: { type: "progress", progress: [{ name, value, max, unit, color }] }`,
	},
	{
		Type: "progressBar", Category: CategoryProgress, Icon: "zap",
		ZhName: "霓虹进度条", EnName: "Neon progress bar",
		DescZh:       "独立进度条组件，支持霓虹发光 / 渐变 / 纯色 / 分段风格，无图表外壳",
		DescEn:       "Standalone progress bar with neon glow, gradient, solid, and segmented styles",
		DefaultColor: "#00F5D4",
		Capabilities: WidgetCapabilities{TopAligned: true, Interactive: true, SupportsTable: true},
		Swappable:    []string{"progressBar", "progressRing", "table"},
		SchemaHint:   `progressBar: { style: "neon"|"gradient"|"solid"|"segmented", height: 10, glow: 1, items: [{ name, value, max, unit, color, to }] }`,
	},
	{
		Type: "progressRing", Category: CategoryProgress, Icon: "gauge",
		ZhName: "完成度圆环", EnName: "Progress rings",
		DescZh:       "并排或同心圆环展示多项目标完成度",
		DescEn:       "Side-by-side or concentric activity completion rings",
		DefaultColor: "#F43F5E",
		Capabilities: WidgetCapabilities{TopAligned: true, Interactive: true, SupportsTable: true},
		Swappable:    []string{"progressRing", "progressBar", "table"},
		SchemaHint:   `chart: { type: "progressRing", ring: { layout: "row"|"concentric", ... }, progress: [...] }`,
	},
	{
		Type: "calendar", Category: CategoryTime, Icon: "calendar-days",
		ZhName: "农历日历", EnName: "Lunar calendar",
		DescZh:       "月历视图，内置农历、节气与节假日标记，支持点选查看",
		DescEn:       "Monthly calendar with lunar dates, solar terms, and holiday badges",
		DefaultColor: "#2563EB",
		Capabilities: WidgetCapabilities{HasDetail: true, TopAligned: true, Interactive: true, SupportsTable: false},
		Swappable:    []string{"calendar"},
		SchemaHint:   `chart: { type: "calendar" }`,
	},
	{
		Type: "donut", Category: CategoryData, Icon: "circle-dollar-sign",
		ZhName: "环形占比", EnName: "Donut share",
		DescZh:       "多分类占比圆环与互动图例，悬停/点击查看单项详情",
		DescEn:       "Multi-slice share breakdown donut with interactive legend highlight",
		DefaultColor: "#3B82F6",
		Capabilities: WidgetCapabilities{TopAligned: true, Interactive: true, SupportsTable: true},
		Swappable:    []string{"donut", "barh", "table"},
		SchemaHint:   `chart: { type: "donut", donut: { center, sub, thickness, neon }, slices: [{ name, value, color, unit }] }`,
	},
	{
		Type: "checklist", Category: CategoryLayout, Icon: "activity",
		ZhName: "待办清单", EnName: "Checklist",
		DescZh:       "带完成状态、徽章与整体进度条的任务/习惯列表",
		DescEn:       "Interactive task/habit checklist with badges and completion progress",
		DefaultColor: "#10B981",
		Capabilities: WidgetCapabilities{TopAligned: true, Interactive: true, SupportsTable: true},
		Swappable:    []string{"checklist", "table"},
		SchemaHint:   `chart: { type: "checklist", checklist: [{ label, done, note, badge, color }] }`,
	},
	{
		Type: "kv", Category: CategoryLayout, Icon: "file-spreadsheet",
		ZhName: "状态面板", EnName: "Status KV",
		DescZh:       "键值指标与健康状态点面板，支持点击复制数值",
		DescEn:       "Key-value status panel with status dots, pills, and click-to-copy",
		DefaultColor: "#38BDF8",
		Capabilities: WidgetCapabilities{TopAligned: true, Interactive: true, SupportsTable: true},
		Swappable:    []string{"kv", "table"},
		SchemaHint:   `chart: { type: "kv", kv: [{ label, value, sub, badge, status: "ok"|"warn"|"err"|"info", color }] }`,
	},
	{
		Type: "countdown", Category: CategoryTime, Icon: "bell",
		ZhName: "目标倒计时", EnName: "Countdown",
		DescZh:       "重要日期倒数/纪念日计时与可选时间进度条",
		DescEn:       "Target date countdown / anniversary cards with optional progress bar",
		DefaultColor: "#A855F7",
		Capabilities: WidgetCapabilities{SelfHero: true, TopAligned: true, SupportsTable: true},
		Swappable:    []string{"countdown", "table"},
		SchemaHint:   `chart: { type: "countdown", countdown: [{ label, target, start, note, color, to, style }] }`,
	},
	{
		Type: "clock", Category: CategoryTime, Icon: "calendar-days",
		ZhName: "世界时钟", EnName: "World clock",
		DescZh:       "指针表盘 / 数字 / 多时区世界时钟，支持界面切换制式与样式",
		DescEn:       "Analog dial, digital, and multi-timezone world clock",
		DefaultColor: "#F59E0B",
		Capabilities: WidgetCapabilities{SelfHero: true, TopAligned: true, Interactive: true, SupportsTable: true},
		Swappable:    []string{"clock", "table"},
		SchemaHint:   `chart: { type: "clock", clock: { style: "analog"|"digital"|"world", seconds, color, neon, zones: [{ label, offset }] } }`,
	},
	{
		Type: "contrib", Category: CategoryData, Icon: "sparkles",
		ZhName: "打卡热力格", EnName: "Activity grid",
		DescZh:       "7 行 GitHub 风格习惯打卡活跃矩阵，支持点选日期详情",
		DescEn:       "7-row GitHub-style activity heatmap with clickable day details",
		DefaultColor: "#22C55E",
		Capabilities: WidgetCapabilities{TopAligned: true, Interactive: true, SupportsTable: true},
		Swappable:    []string{"contrib", "table"},
		SchemaHint:   `chart: { type: "contrib", contrib: { cells: [{ date, value, label }], color, max, weeks, unit, neon } }`,
	},
	{
		Type: "table", Category: CategoryData, Icon: "file-spreadsheet",
		ZhName: "数据表格", EnName: "Data table",
		DescZh:       "结构化数据表组件，支持列对齐、徽章/涨跌着色、排序、搜索与 CSV 导出",
		DescEn:       "Structured data table with sorting, search, status/delta badges, and CSV export",
		DefaultColor: "#2563EB",
		Capabilities: WidgetCapabilities{TopAligned: true, Interactive: true, SupportsTable: true},
		Swappable:    []string{"table", "kv"},
		SchemaHint:   `chart: { type: "table", table: { columns: [{ title, align, format }], rows: [[...]], searchable: true, striped: true } }`,
	},
}

var widgetSpecIndex = func() map[string]WidgetSpec {
	m := make(map[string]WidgetSpec, len(WidgetSpecs))
	for _, s := range WidgetSpecs {
		m[s.Type] = s
	}
	return m
}()

func widgetTypeNames() []string {
	out := make([]string, len(WidgetSpecs))
	for i, s := range WidgetSpecs {
		out[i] = s.Type
	}
	return out
}

// WidgetSpecOf returns the specification for component type t.
func WidgetSpecOf(t string) (WidgetSpec, bool) {
	s, ok := widgetSpecIndex[t]
	return s, ok
}

// ResultWidgetType returns the normalized component type of res, or "" if res
// has no chart/progressBar component.
func ResultWidgetType(res Result) string {
	if res.ProgressBar != nil {
		return "progressBar"
	}
	if res.Chart == nil {
		return ""
	}
	if res.Chart.Type == "" {
		return "line"
	}
	if res.Chart.Type == "progress" {
		return "progressBar"
	}
	return res.Chart.Type
}

// SwappableTypes returns the runtime-compatible component types that can render
// res's existing data without changing the script.
func SwappableTypes(res Result) []string {
	typ := ResultWidgetType(res)
	if typ == "" {
		return nil
	}
	spec, ok := WidgetSpecOf(typ)
	if !ok {
		return []string{typ}
	}
	out := make([]string, 0, len(spec.Swappable))
	for _, cand := range spec.Swappable {
		if canSwapResult(res, typ, cand) {
			out = append(out, cand)
		}
	}
	if len(out) == 0 {
		return []string{typ}
	}
	return out
}

func canSwapResult(res Result, base, target string) bool {
	if target == base {
		return true
	}
	switch target {
	case "table":
		return ExtractTable(res, "zh") != nil
	case "progressBar", "progressRing":
		if res.ProgressBar != nil && len(res.ProgressBar.Items) > 0 {
			return true
		}
		if res.Chart != nil && len(res.Chart.Progress) > 0 {
			return true
		}
		if res.Chart != nil && res.Chart.Gauge != nil && res.Chart.Gauge.Limit > 0 {
			return true
		}
		return false
	case "barh":
		if res.ProgressBar != nil && len(res.ProgressBar.Items) > 0 {
			return true
		}
		if res.Chart != nil && (len(res.Chart.Progress) > 0 || len(res.Chart.Slices) > 0 || len(res.Chart.Series) > 0) {
			return true
		}
		return false
	case "donut":
		if res.Chart == nil {
			return false
		}
		if len(res.Chart.Slices) > 0 {
			return true
		}
		if res.Chart.Type == "barh" && len(res.Chart.Series) > 0 && len(res.Chart.Series[0].Points) > 0 {
			for _, v := range res.Chart.Series[0].Points {
				if v > 0 {
					return true
				}
			}
		}
		return false
	case "kv":
		if res.Chart == nil {
			return false
		}
		return len(res.Chart.KV) > 0 || len(res.Chart.Slices) > 0 || len(res.Chart.Checklist) > 0 ||
			len(res.Chart.Countdown) > 0 ||
			(res.Chart.Type == "barh" && len(res.Chart.Series) > 0) ||
			(res.Chart.Table != nil && len(res.Chart.Table.Columns) >= 2 && len(res.Chart.Table.Rows) > 0)
	case "line", "dot":
		return res.Chart != nil && len(res.Chart.Series) > 0
	}
	return false
}

// ExtractTable converts any widget Result into a clean tabular TableSpec so
// every component can be viewed, sorted, searched, and exported as CSV.
func ExtractTable(res Result, lang string) *TableSpec {
	en := lang == "en"
	t := func(zh, e string) string {
		if en {
			return e
		}
		return zh
	}
	fmtNum := func(v float64) string {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return "—"
		}
		if v == math.Trunc(v) && math.Abs(v) < 1e12 {
			return fmt.Sprintf("%.0f", v)
		}
		if math.Abs(v) < 10 && v != 0 {
			return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", v), "0"), ".")
		}
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", v), "0"), ".")
	}

	// Standalone progressBar.
	if res.ProgressBar != nil && len(res.ProgressBar.Items) > 0 {
		return progressTable(res.ProgressBar.Items, t, fmtNum)
	}

	if ch := res.Chart; ch != nil {
		switch ch.Type {
		case "table":
			if ch.Table != nil && len(ch.Table.Columns) > 0 {
				cp := *ch.Table
				cp.Searchable = true
				return &cp
			}
		case "candle":
			if len(ch.Candles) > 0 {
				cols := []TableColumn{
					{Title: t("时间", "Time")},
					{Title: t("开盘", "Open"), Align: "right", Format: "number"},
					{Title: t("最高", "High"), Align: "right", Format: "number"},
					{Title: t("最低", "Low"), Align: "right", Format: "number"},
					{Title: t("收盘", "Close"), Align: "right", Format: "number"},
					{Title: t("涨跌幅", "Change"), Align: "right", Format: "delta"},
				}
				rows := make([][]string, len(ch.Candles))
				for i, cd := range ch.Candles {
					lbl := fmt.Sprintf("#%d", i+1)
					if i < len(ch.Labels) && ch.Labels[i] != "" {
						lbl = ch.Labels[i]
					}
					chg := "0.00%"
					if cd.O != 0 {
						chg = fmt.Sprintf("%+.2f%%", (cd.C-cd.O)/cd.O*100)
					}
					rows[i] = []string{lbl, fmtNum(cd.O), fmtNum(cd.H), fmtNum(cd.L), fmtNum(cd.C), chg}
				}
				return &TableSpec{Columns: cols, Rows: rows, Searchable: true, Striped: true}
			}
		case "panels":
			if len(ch.Panels) > 0 {
				cols := []TableColumn{{Title: t("标签", "Label")}}
				type pSeries struct {
					pts []float64
				}
				var flat []pSeries
				maxN := 0
				for _, pn := range ch.Panels {
					for _, sr := range pn.Series {
						name := sr.Name
						if pn.Title != "" && name != "" && pn.Title != name {
							name = pn.Title + " · " + name
						} else if name == "" {
							name = pn.Title
						}
						cols = append(cols, TableColumn{Title: name, Align: "right", Format: "number"})
						flat = append(flat, pSeries{pts: sr.Points})
						if len(sr.Points) > maxN {
							maxN = len(sr.Points)
						}
					}
				}
				if maxN > 0 {
					rows := make([][]string, maxN)
					for i := 0; i < maxN; i++ {
						r := make([]string, len(cols))
						if i < len(ch.Labels) && ch.Labels[i] != "" {
							r[0] = ch.Labels[i]
						} else {
							r[0] = fmt.Sprintf("#%d", i+1)
						}
						for k, ps := range flat {
							if i < len(ps.pts) {
								r[k+1] = fmtNum(ps.pts[i])
							}
						}
						rows[i] = r
					}
					return &TableSpec{Columns: cols, Rows: rows, Searchable: true, Striped: true}
				}
			}
		case "heat":
			if ch.Grid != nil && len(ch.Grid.Rows) > 0 {
				cols := []TableColumn{{Title: t("项目", "Row")}}
				for _, col := range ch.Grid.Columns {
					cols = append(cols, TableColumn{Title: col, Align: "right", Format: "number"})
				}
				rows := make([][]string, 0, len(ch.Grid.Rows)+1)
				for _, gr := range ch.Grid.Rows {
					r := make([]string, len(cols))
					r[0] = gr.Label
					if gr.Note != "" {
						r[0] += " (" + gr.Note + ")"
					}
					for j, v := range gr.Cells {
						if j+1 >= len(r) {
							break
						}
						if v < 0 {
							r[j+1] = "—"
						} else {
							r[j+1] = fmtNum(v)
						}
					}
					rows = append(rows, r)
				}
				if len(ch.Grid.Average) > 0 {
					r := make([]string, len(cols))
					r[0] = t("平均", "Average")
					for j, v := range ch.Grid.Average {
						if j+1 >= len(r) {
							break
						}
						if v >= 0 {
							r[j+1] = fmtNum(v)
						}
					}
					rows = append(rows, r)
				}
				return &TableSpec{Columns: cols, Rows: rows, Searchable: true, Striped: true}
			}
		case "progress", "progressBar", "progressRing":
			if len(ch.Progress) > 0 {
				return progressTable(ch.Progress, t, fmtNum)
			}
		case "gauge":
			if g := ch.Gauge; g != nil {
				rem := math.Max(0, g.Limit-g.Used)
				pct := 0.0
				if g.Limit > 0 {
					pct = rem / g.Limit * 100
				}
				return &TableSpec{
					Columns: []TableColumn{
						{Title: t("项目", "Item")},
						{Title: t("已用", "Used"), Align: "right", Format: "number"},
						{Title: t("限额", "Limit"), Align: "right", Format: "number"},
						{Title: t("剩余", "Left"), Align: "right", Format: "number"},
						{Title: t("剩余比例", "Share"), Align: "right", Format: "badge"},
						{Title: t("重置", "Reset")},
					},
					Rows: [][]string{{
						g.Label,
						fmtNum(g.Used) + g.Unit,
						fmtNum(g.Limit) + g.Unit,
						fmtNum(rem) + g.Unit,
						fmt.Sprintf("%.0f%%", math.Round(pct)),
						g.Reset,
					}},
					Striped: true,
				}
			}
		case "donut":
			if len(ch.Slices) > 0 {
				total := 0.0
				for _, s := range ch.Slices {
					if s.Value > 0 {
						total += s.Value
					}
				}
				rows := make([][]string, 0, len(ch.Slices))
				for _, s := range ch.Slices {
					pct := "0%"
					if total > 0 && s.Value > 0 {
						pct = fmt.Sprintf("%.1f%%", s.Value/total*100)
					}
					rows = append(rows, []string{s.Name, fmtNum(s.Value), s.Unit, pct})
				}
				return &TableSpec{
					Columns: []TableColumn{
						{Title: t("分类", "Category")},
						{Title: t("数值", "Value"), Align: "right", Format: "number"},
						{Title: t("单位", "Unit"), Align: "center"},
						{Title: t("占比", "Share"), Align: "right", Format: "badge"},
					},
					Rows:       rows,
					Searchable: true,
					Striped:    true,
				}
			}
		case "checklist":
			if len(ch.Checklist) > 0 {
				rows := make([][]string, len(ch.Checklist))
				for i, it := range ch.Checklist {
					st := t("未完成", "Todo")
					if it.Done {
						st = t("已完成", "Done")
					}
					rows[i] = []string{st, it.Label, it.Badge, it.Note}
				}
				return &TableSpec{
					Columns: []TableColumn{
						{Title: t("状态", "Status"), Format: "badge"},
						{Title: t("项目", "Task")},
						{Title: t("标签", "Tag"), Format: "badge"},
						{Title: t("备注", "Note")},
					},
					Rows:       rows,
					Searchable: true,
					Striped:    true,
				}
			}
		case "kv":
			if len(ch.KV) > 0 {
				rows := make([][]string, len(ch.KV))
				for i, it := range ch.KV {
					rows[i] = []string{it.Label, it.Value, it.Status, it.Badge, it.Sub}
				}
				return &TableSpec{
					Columns: []TableColumn{
						{Title: t("指标", "Metric")},
						{Title: t("数值", "Value"), Align: "right"},
						{Title: t("状态", "Status"), Format: "status"},
						{Title: t("标签", "Badge"), Format: "badge"},
						{Title: t("说明", "Note")},
					},
					Rows:       rows,
					Searchable: true,
					Striped:    true,
				}
			}
		case "countdown":
			if len(ch.Countdown) > 0 {
				rows := make([][]string, len(ch.Countdown))
				for i, it := range ch.Countdown {
					rows[i] = []string{it.Label, it.Target, it.Start, it.Note}
				}
				return &TableSpec{
					Columns: []TableColumn{
						{Title: t("事件", "Event")},
						{Title: t("目标日期", "Target")},
						{Title: t("起始日期", "Start")},
						{Title: t("说明", "Note")},
					},
					Rows:    rows,
					Striped: true,
				}
			}
		case "clock":
			if ch.Clock != nil && len(ch.Clock.Zones) > 0 {
				rows := make([][]string, len(ch.Clock.Zones))
				for i, z := range ch.Clock.Zones {
					off := z.Offset
					if math.Abs(off) > 14 {
						off = off / 60
					}
					rows[i] = []string{z.Label, fmt.Sprintf("UTC%+.1f", off)}
				}
				return &TableSpec{
					Columns: []TableColumn{
						{Title: t("地区", "Region")},
						{Title: t("时区偏移", "UTC Offset"), Align: "right"},
					},
					Rows:    rows,
					Striped: true,
				}
			}
		case "contrib":
			if ch.Contrib != nil && len(ch.Contrib.Cells) > 0 {
				rows := make([][]string, len(ch.Contrib.Cells))
				for i, cell := range ch.Contrib.Cells {
					rows[i] = []string{cell.Date, fmtNum(cell.Value) + ch.Contrib.Unit, cell.Label}
				}
				return &TableSpec{
					Columns: []TableColumn{
						{Title: t("日期", "Date")},
						{Title: t("数值", "Value"), Align: "right", Format: "number"},
						{Title: t("说明", "Note")},
					},
					Rows:       rows,
					Searchable: true,
					Striped:    true,
				}
			}
		default:
			// line, dot, barh, combo, or empty type with Series.
			if len(ch.Series) > 0 {
				maxN := 0
				cols := []TableColumn{{Title: t("标签", "Label")}}
				for i, sr := range ch.Series {
					name := sr.Name
					if name == "" {
						name = fmt.Sprintf(t("序列 %d", "Series %d"), i+1)
					}
					cols = append(cols, TableColumn{Title: name, Align: "right", Format: "number"})
					if len(sr.Points) > maxN {
						maxN = len(sr.Points)
					}
				}
				if maxN > 0 {
					rows := make([][]string, maxN)
					for i := 0; i < maxN; i++ {
						r := make([]string, len(cols))
						if i < len(ch.Labels) && ch.Labels[i] != "" {
							r[0] = ch.Labels[i]
						} else {
							r[0] = fmt.Sprintf("#%d", i+1)
						}
						for k, sr := range ch.Series {
							idx := i - sr.Offset
							if idx >= 0 && idx < len(sr.Points) {
								r[k+1] = fmtNum(sr.Points[idx])
							}
						}
						rows[i] = r
					}
					return &TableSpec{Columns: cols, Rows: rows, Searchable: true, Striped: true}
				}
			}
		}
	}

	if len(res.Stats) > 0 {
		rows := make([][]string, len(res.Stats))
		for i, s := range res.Stats {
			rows[i] = []string{s.Label, s.Value, s.Sub}
		}
		return &TableSpec{
			Columns: []TableColumn{
				{Title: t("指标", "Metric")},
				{Title: t("数值", "Value"), Align: "right"},
				{Title: t("说明", "Note")},
			},
			Rows:    rows,
			Striped: true,
		}
	}

	if len(res.Images) > 0 {
		rows := make([][]string, len(res.Images))
		for i, im := range res.Images {
			rows[i] = []string{fmt.Sprintf("%d", i+1), im.Caption, im.Source, im.URL}
		}
		return &TableSpec{
			Columns: []TableColumn{
				{Title: "#"},
				{Title: t("说明", "Caption")},
				{Title: t("来源", "Source")},
				{Title: t("图片地址", "URL")},
			},
			Rows:    rows,
			Striped: true,
		}
	}
	return nil
}

func progressTable(items []Progress, t func(string, string) string, fmtNum func(float64) string) *TableSpec {
	rows := make([][]string, len(items))
	for i, it := range items {
		maxStr := "100%"
		if it.Max > 0 {
			maxStr = fmtNum(it.Max) + it.Unit
		}
		rows[i] = []string{
			it.Name,
			fmtNum(it.Value) + it.Unit,
			maxStr,
			fmt.Sprintf("%.0f%%", math.Round(it.Raw())),
		}
	}
	return &TableSpec{
		Columns: []TableColumn{
			{Title: t("项目", "Item")},
			{Title: t("当前", "Current"), Align: "right", Format: "number"},
			{Title: t("目标", "Target"), Align: "right", Format: "number"},
			{Title: t("完成度", "Progress"), Align: "right", Format: "badge"},
		},
		Rows:       rows,
		Searchable: true,
		Striped:    true,
	}
}
