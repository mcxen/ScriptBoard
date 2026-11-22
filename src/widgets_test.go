package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/store"
)

// A standalone progressBar needs no chart wrapper, draws neon bars on home
// cards, and shows NO chart controls (样式 / 大图 / 折线 / 悬浮图表查看详情) in
// its full view.
func TestProgressBarNeedsNoChartAndNoFullViewChartControls(t *testing.T) {
	a := newTestApp(t)
	for _, sc := range a.store.Scripts() {
		sc.Home = false
		a.store.SaveScript(sc)
	}
	pbSrc := `export function meta() { return { name: "霓虹目标", icon: "zap", sources: {} }; }
export function render(ctx) {
  return {
    title: "霓虹目标",
    icon: "zap",
    big: "78%",
    sub: "本周达成度",
    lines: [],
    progressBar: {
      style: "neon",
      height: 11,
      track: "dark",
      glow: 1.1,
      items: [
        { name: "季度 OKR", value: 78, max: 100, unit: "%", color: "#00F5D4", to: "#38BDF8" },
        { name: "每日专注", value: 4.5, max: 6, unit: "h", color: "#FF2A85", to: "#FB923C" },
        { name: "年度阅读", value: 18, max: 24, unit: "本", color: "#A855F7", to: "#38BDF8" },
      ],
    },
  };
}`
	a.store.SaveScript(store.Script{ID: "neon-pb", Name: "霓虹目标", Home: true, Origin: store.OriginUser, Source: pbSrc})
	a.setSize("neon-pb", 2)

	tt := ui.NewTester(a.view, 1100, 760)
	settle(tt)
	settle(tt)
	requireText(t, tt, "季度 OKR")
	requireText(t, tt, "每日专注")
	requireText(t, tt, "年度阅读")
	absent(t, tt, "悬浮图表查看详情", "点按图表查看详情")
	savePNG(t, tt, "20-neon-progress-home")

	// Open full detail view: must NOT contain chart style picker, 大图, or chart detail.
	a.router.Push("/full/neon-pb")
	settle(tt)
	settle(tt)
	requireText(t, tt, "季度 OKR")
	requireText(t, tt, "年度阅读")
	absent(t, tt, "样式", "大图", "折线", "面积", "柱状", "悬浮图表查看详情", "点按图表查看详情", "最高", "最低", "平均")
	savePNG(t, tt, "21-neon-progress-full")
}

// All six new widget components render cleanly on Home and in Full Detail view
// without plot chart controls.
func TestSixNewWidgetComponentsRenderOnHomeAndFull(t *testing.T) {
	a := newTestApp(t)
	for _, sc := range a.store.Scripts() {
		sc.Home = false
		a.store.SaveScript(sc)
	}
	widgets := []struct {
		id     string
		tplKey string
		want   []string
		full   []string
	}{
		{"w-donut", "donut", []string{"餐饮", "居住", "¥6,420"}, []string{"合计", "最高占比"}},
		{"w-check", "checklist", []string{"已完成 2 / 4 · 50%", "晨间复盘与排期"}, []string{"已完成", "待完成", "完成率"}},
		{"w-kv", "kv", []string{"API 网关", "24 ms", "任务队列"}, []string{"API 网关", "128.4K"}},
		{"w-cd", "countdown", []string{"距离下一年", "距离目标", "天"}, []string{"距离下一年", "时", "分"}},
		{"w-clock", "clock", []string{"北京 / 上海"}, []string{"北京 / 上海"}},
		{"w-contrib", "contrib", []string{"连续 12 天", "少", "多"}, []string{"连续 12 天", "少", "多"}},
	}
	for _, w := range widgets {
		cfg := a.defaultVisualConfig(w.tplKey)
		if w.tplKey == "clock" {
			cfg.Clock.Style = "world"
		}
		a.store.SaveScript(store.Script{
			ID:     w.id,
			Name:   cfg.Name,
			Home:   true,
			Origin: store.OriginUser,
			Source: renderVisualScript(cfg),
		})
		a.setSize(w.id, 1)
	}

	tt := ui.NewTester(a.view, 1280, 1400)
	settle(tt)
	settle(tt)
	for _, w := range widgets {
		for _, want := range w.want {
			requireText(t, tt, want)
		}
	}
	absent(t, tt, "悬浮图表查看详情", "点按图表查看详情")
	savePNG(t, tt, "22-rich-widgets-home")

	// Also verify dark mode rendering of all 6 widgets.
	t.Cleanup(func() { setThemeMode(themeSystem) })
	setThemeMode(themeDark)
	settle(tt)
	settle(tt)
	savePNG(t, tt, "25-dark-widgets-home")
	setThemeMode(themeSystem)
	settle(tt)

	for _, w := range widgets {
		a.router.Replace("/full/" + w.id)
		settle(tt)
		settle(tt)
		for _, want := range w.full {
			requireText(t, tt, want)
		}
		absent(t, tt, "样式", "大图", "折线", "悬浮图表查看详情", "点按图表查看详情")
	}
}

// The visual script editor lets users pick component templates, edit visual
// styles/icons/layout, switch to raw source, and stays 100% English in English
// mode.
func TestVisualEditorTemplatesStyleIconAndEnglish(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 1280, 860)
	settle(tt)

	click(t, tt, "编辑")
	requireText(t, tt, "可视化")
	requireText(t, tt, "源码")
	requireText(t, tt, "卡片显示")
	requireText(t, tt, "高级设置")

	// Create a new script -> visual templates open automatically.
	click(t, tt, "+ 脚本")
	requireText(t, tt, "霓虹进度条")
	requireText(t, tt, "环形占比")
	requireText(t, tt, "待办清单")
	requireText(t, tt, "状态面板")
	requireText(t, tt, "倒计时")
	requireText(t, tt, "时钟表盘")
	requireText(t, tt, "打卡热力格")
	savePNG(t, tt, "26-visual-editor-templates")

	// Apply neon progress bar template -> generates standalone progressBar without chart.
	click(t, tt, "霓虹进度条")
	if !strings.Contains(a.draft.Source, visualMarker) {
		t.Fatalf("missing visual marker in generated source:\n%s", a.draft.Source)
	}
	if !strings.Contains(a.draft.Source, "progressBar:") || strings.Contains(a.draft.Source, "chart:") {
		t.Fatalf("progressBar template should output standalone progressBar without chart:\n%s", a.draft.Source)
	}
	requireText(t, tt, "进度条目")
	requireText(t, tt, "季度 OKR")
	absent(t, tt, "悬浮图表查看详情")
	savePNG(t, tt, "23-visual-editor-neon")

	// Switch to 样式 tab and switch progress bar style to segmented.
	click(t, tt, "样式")
	requireText(t, tt, "进度条样式")
	click(t, tt, "分段")
	if !strings.Contains(a.draft.Source, `"style": "segmented"`) {
		t.Fatalf("expected segmented style in source:\n%s", a.draft.Source)
	}

	// Switch back to 内容 tab, open visual icon picker and pick an icon.
	click(t, tt, "内容")
	click(t, tt, "更换图标")
	requireText(t, tt, "天气")
	requireText(t, tt, "财经")
	click(t, tt, "财经")
	click(t, tt, "金币")
	if !strings.Contains(a.draft.Source, `"icon": "coins"`) {
		t.Fatalf("expected icon coins in source:\n%s", a.draft.Source)
	}

	// Switch to raw source mode and back.
	click(t, tt, "源码")
	requireText(t, tt, "ES 模块：导出 meta() 与 render(ctx)；log(...) 的输出会显示在预览里")
	click(t, tt, "可视化")
	requireText(t, tt, "进度条目")

	// Verify all other visual templates apply cleanly with zero preview errors.
	for _, tplName := range []string{"完成度环", "环形占比", "待办清单", "状态面板", "倒计时", "时钟表盘", "打卡热力格", "折线图表"} {
		click(t, tt, "更多")
		click(t, tt, "选择模板")
		click(t, tt, tplName)
		if !strings.Contains(a.draft.Source, visualMarker) {
			t.Errorf("template %s failed to generate visual source", tplName)
		}
		if _, err := a.metaOf(a.draft); err != nil {
			t.Errorf("template %s meta error: %v", tplName, err)
		}
	}

	// Verify English mode in visual editor with Style tab & neon progress bar.
	t.Cleanup(func() { setLang(langZH) })
	a.set.Lang = langEN
	setLang(langEN)
	a.applyTemplate("progressBar")
	a.visual.tab = 1
	settle(tt)
	settle(tt)
	requireText(t, tt, "Visual")
	requireText(t, tt, "Progress bar style")
	onlyEnglish(t, tt, "visual-editor-en")
}

// Modern built-in scripts are present in the store, retired legacy duplicates
// are absent, and they render cleanly on Home and in Full Detail view without
// unnecessary plot chart controls.
func TestNewStyleBuiltinsRegisteredAndRenderCleanly(t *testing.T) {
	a := newTestApp(t)
	wantPresent := []string{
		"ai-claude-neon", "ai-balance-neon",
		"goldacc-kv", "gold-kv", "fxcny-table",
		"aqi-neon", "cnwx-kv",
		"weight-contrib", "year-countdown", "world-clock",
	}
	for _, id := range wantPresent {
		if _, ok := a.store.Script(id); !ok {
			t.Fatalf("missing builtin %q", id)
		}
	}
	for id := range retiredBuiltins {
		if _, ok := a.store.Script(id); ok {
			t.Errorf("retired builtin %q should not ship anymore", id)
		}
	}
	for _, offHome := range []string{"ai-claude-neon", "ai-balance-neon", "weight-contrib"} {
		if sc, ok := a.store.Script(offHome); ok && sc.Home {
			t.Errorf("builtin %q should default to Home=false", offHome)
		}
	}

	// Enable the offline-ready ones on Home and verify rendering + screenshot.
	for _, sc := range a.store.Scripts() {
		sc.Home = sc.ID == "weight-contrib" || sc.ID == "year-countdown" || sc.ID == "world-clock"
		a.store.SaveScript(sc)
	}
	tt := ui.NewTester(a.view, 1200, 760)
	settle(tt)
	settle(tt)
	requireText(t, tt, "连续")
	requireText(t, tt, "距离 2027 年")
	requireText(t, tt, "北京 / 上海")
	absent(t, tt, "悬浮图表查看详情", "点按图表查看详情")
	savePNG(t, tt, "24-new-builtins-home")

	for _, id := range []string{"weight-contrib", "year-countdown", "world-clock"} {
		a.router.Replace("/full/" + id)
		settle(tt)
		settle(tt)
		absent(t, tt, "大图", "折线", "悬浮图表查看详情", "点按图表查看详情")
	}
}

// Systematic audit across all component types in fullView: plot charts keep
// style/Big/hover controls, non-plot components never show them, weight &
// weight-contrib support recording right in fullView, contrib shows streak
// summary tiles, and panels shows stats & date labels.
func TestAllComponentTypesFullViewAudit(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 1180, 820)

	// 1. weight (dot plot + quick entry input)
	a.router.Replace("/full/weight")
	settle(tt)
	settle(tt)
	requireText(t, tt, "样式")
	requireText(t, tt, "大图")
	requireText(t, tt, "记录")
	requireText(t, tt, "最新")
	savePNG(t, tt, "32-full-weight")

	// 2. weight-contrib (contrib grid + streak tiles + quick entry input, NO plot controls)
	a.router.Replace("/full/weight-contrib")
	settle(tt)
	settle(tt)
	requireText(t, tt, "活跃天数")
	requireText(t, tt, "当前连续")
	requireText(t, tt, "最高单日")
	requireText(t, tt, "记录")
	absent(t, tt, "大图", "折线", "悬浮图表查看详情")
	savePNG(t, tt, "29-full-contrib")

	// 3. year-countdown (hero countdown + neon bar, NO plot controls, NO duplicate title)
	a.router.Replace("/full/year-countdown")
	settle(tt)
	settle(tt)
	requireText(t, tt, "距离 2027 年")
	requireText(t, tt, "天")
	absent(t, tt, "样式", "大图", "折线", "悬浮图表查看详情")
	savePNG(t, tt, "30-full-countdown")

	// 4. world-clock (clock world & analog modes via single clockStylePicker, NO plot controls)
	a.router.Replace("/full/world-clock")
	settle(tt)
	settle(tt)
	requireText(t, tt, "北京 / 上海")
	absent(t, tt, "大图", "折线", "悬浮图表查看详情")
	click(t, tt, "指针表盘")
	settle(tt)
	savePNG(t, tt, "31-full-clock")

	// 5. panels chart (plot with multiple stacked panels: keeps stats & x-axis labels)
	panelsSrc := `export function meta() { return { name: "多面板", sources: {} }; }
export function render(ctx) {
  return {
    title: "多面板",
    big: "105.00",
    sub: "价格与动量",
    lines: [],
    chart: {
      type: "panels",
      labels: ["11-17", "11-18", "11-19", "11-20", "11-21"],
      panels: [
        { title: "价格", weight: 2, series: [{ name: "收盘", color: "#3B82F6", points: [100, 102, 101, 104, 105] }] },
        { title: "RSI", weight: 1, series: [{ name: "RSI", color: "#F59E0B", points: [48, 55, 52, 61, 64] }] },
      ],
    },
  };
}`
	a.store.SaveScript(store.Script{ID: "audit-panels", Name: "多面板", Origin: store.OriginUser, Source: panelsSrc})
	a.router.Replace("/full/audit-panels")
	settle(tt)
	settle(tt)
	requireText(t, tt, "最新")
	requireText(t, tt, "最高")
	requireText(t, tt, "11-21")

	// 6. heat & gauge & barh & progressRing: verify NONE show plot style picker or 大图.
	nonPlots := map[string]string{
		"audit-heat": `export function meta() { return { name: "热力", sources: {} }; }
export function render() { return { title: "热力", big: "42%", sub: "", lines: [], chart: { type: "heat", grid: { columns: ["D1","D7"], rows: [{ label: "W1", cells: [60, 42] }] } } }; }`,
		"audit-gauge": `export function meta() { return { name: "额度", sources: {} }; }
export function render() { return { title: "额度", big: "70%", sub: "", lines: [], chart: { type: "gauge", gauge: { used: 30, limit: 100, unit: "USD", label: "月额度" } } }; }`,
		"audit-barh": `export function meta() { return { name: "排行", sources: {} }; }
export function render() { return { title: "排行", big: "100", sub: "", lines: [], chart: { type: "barh", labels: ["甲","乙","丙"], series: [{ name: "分", points: [100, 80, 60] }] } }; }`,
		"audit-ring": `export function meta() { return { name: "圆环", sources: {} }; }
export function render() { return { title: "圆环", big: "", sub: "", lines: [], chart: { type: "progressRing", progress: [{ name: "跑步", value: 4, max: 5, unit: "km" }] } }; }`,
	}
	for id, src := range nonPlots {
		a.store.SaveScript(store.Script{ID: id, Name: id, Origin: store.OriginUser, Source: src})
		a.router.Replace("/full/" + id)
		settle(tt)
		settle(tt)
		absent(t, tt, "样式", "大图", "折线", "悬浮图表查看详情")
	}
}
