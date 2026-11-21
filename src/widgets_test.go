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
	requireText(t, tt, "组件类型模板")

	// Create a new script -> visual templates open automatically.
	click(t, tt, "+ 脚本")
	requireText(t, tt, "霓虹进度条")
	requireText(t, tt, "环形占比")
	requireText(t, tt, "待办清单")
	requireText(t, tt, "状态面板")
	requireText(t, tt, "倒计时")
	requireText(t, tt, "世界时钟")
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
	requireText(t, tt, "进度条外观（无图表组件）")
	requireText(t, tt, "季度 OKR")
	absent(t, tt, "悬浮图表查看详情")
	savePNG(t, tt, "23-visual-editor-neon")

	// Switch progress bar style to matrix and segmented via visual segmented control.
	click(t, tt, "点阵灯")
	if !strings.Contains(a.draft.Source, `"style": "matrix"`) {
		t.Fatalf("expected matrix style in source:\n%s", a.draft.Source)
	}
	requireText(t, tt, "呼吸流光")
	requireText(t, tt, "合并点阵")
	click(t, tt, "分段")
	if !strings.Contains(a.draft.Source, `"style": "segmented"`) {
		t.Fatalf("expected segmented style in source:\n%s", a.draft.Source)
	}

	// Open visual icon picker and pick an icon.
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
	requireText(t, tt, "进度条外观（无图表组件）")

	// Verify all other visual templates apply cleanly with zero preview errors.
	for _, tplName := range []string{"完成度环", "环形占比", "待办清单", "状态面板", "倒计时", "世界时钟", "打卡热力格", "折线图表"} {
		click(t, tt, "选择模板")
		click(t, tt, tplName)
		if !strings.Contains(a.draft.Source, visualMarker) {
			t.Errorf("template %s failed to generate visual source", tplName)
		}
		if _, err := a.metaOf(a.draft); err != nil {
			t.Errorf("template %s meta error: %v", tplName, err)
		}
	}

	// Verify English mode in visual editor with template picker & neon progress bar.
	t.Cleanup(func() { setLang(langZH) })
	a.set.Lang = langEN
	setLang(langEN)
	a.applyTemplate("progressBar")
	a.visual.tplOpen = true
	settle(tt)
	settle(tt)
	requireText(t, tt, "Visual")
	requireText(t, tt, "Progress bar style (no chart)")
	onlyEnglish(t, tt, "visual-editor-en")
}

// New style built-in scripts are present in the store, start off-home by
// default, and render cleanly on Home and in Full Detail view without plot
// chart controls.
func TestNewStyleBuiltinsRegisteredAndRenderCleanly(t *testing.T) {
	a := newTestApp(t)
	newIDs := []string{
		"ai-claude-neon", "ai-balance-neon",
		"goldacc-kv", "fxcny-kv", "gold-kv",
		"aqi-neon", "cnwx-kv",
		"weight-contrib", "year-countdown", "world-clock",
	}
	for _, id := range newIDs {
		sc, ok := a.store.Script(id)
		if !ok {
			t.Fatalf("missing new builtin %q", id)
		}
		if sc.Home {
			t.Errorf("new builtin %q should default to Home=false", id)
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

	// 4. world-clock (clock world & analog modes, NO plot controls)
	a.router.Replace("/full/world-clock")
	settle(tt)
	settle(tt)
	requireText(t, tt, "北京 / 上海")
	absent(t, tt, "大图", "折线", "悬浮图表查看详情")
	click(t, tt, "霓虹表盘")
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

func TestMatrixProgressBarVisualAndBehavior(t *testing.T) {
	a := newTestApp(t)
	for _, sc := range a.store.Scripts() {
		sc.Home = false
		a.store.SaveScript(sc)
	}
	matrixSrc := `export function meta() { return { name: "点阵配额", icon: "sparkles", sources: {} }; }
export function render() {
  return {
    title: "点阵配额",
    icon: "sparkles",
    big: "84%",
    sub: "本月 Token 分配与剩余额度",
    lines: [],
    progressBar: {
      style: "matrix",
      rows: 7,
      total: 100,
      glow: 1.1,
      track: "dark",
      animate: true,
      items: [
        { name: "Sonnet", value: 24, unit: "%", color: "#3B82F6" },
        { name: "Opus", value: 16, unit: "%", color: "#22C55E" },
        { name: "Haiku", value: 28, unit: "%", color: "#06B6D4" },
        { name: "Cache", value: 16, unit: "%", color: "#F97316" },
      ],
    },
    layout: { summary: "top", stats: "hide", legend: "hide" },
  };
}`
	perItemSrc := `export function meta() { return { name: "点阵进度", icon: "zap", sources: {} }; }
export function render() {
  return {
    title: "点阵进度",
    icon: "zap",
    big: "72%",
    sub: "独立目标完成度",
    lines: [],
    progressBar: {
      style: "matrix",
      stacked: false,
      rows: 5,
      track: "dark",
      items: [
        { name: "本月额度", value: 72, max: 100, unit: "%", color: "#3B82F6" },
        { name: "周任务", value: 14, max: 20, unit: "项", color: "#22C55E" },
      ],
    },
    layout: { summary: "top", stats: "hide", legend: "hide" },
  };
}`
	a.store.SaveScript(store.Script{ID: "matrix-quota", Name: "点阵配额", Home: true, Origin: store.OriginUser, Source: matrixSrc})
	a.store.SaveScript(store.Script{ID: "matrix-goals", Name: "点阵进度", Home: true, Origin: store.OriginUser, Source: perItemSrc})
	a.setSize("matrix-quota", 2)
	a.setSize("matrix-goals", 2)
	tt := ui.NewTester(a.view, 1100, 760)
	settle(tt)
	settle(tt)

	requireText(t, tt, "点阵配额")
	requireText(t, tt, "Sonnet")
	requireText(t, tt, "Opus")
	requireText(t, tt, "Haiku")
	requireText(t, tt, "Cache")
	requireText(t, tt, "点阵进度")
	requireText(t, tt, "本月额度")
	savePNG(t, tt, "26-matrix-progress-home")

	a.router.Push("/full/matrix-quota")
	settle(tt)
	settle(tt)
	requireText(t, tt, "点阵配额")
	absent(t, tt, "折线", "大图", "悬浮图表查看详情")
	savePNG(t, tt, "27-matrix-progress-full")
}

func TestMatrixFamilyShowcase(t *testing.T) {
	a := newTestApp(t)
	for _, sc := range a.store.Scripts() {
		sc.Home = false
		a.store.SaveScript(sc)
	}
	donutSrc := `export function meta() { return { name: "点阵占比", icon: "circle-dollar-sign", sources: {} }; }
export function render() {
  return {
    title: "点阵占比",
    icon: "circle-dollar-sign",
    big: "",
    sub: "本月 Token 模型分布",
    lines: [],
    chart: {
      type: "donut",
      donut: { center: "84.2M", sub: "本月总计", thickness: 0.34, style: "matrix" },
      slices: [
        { name: "Sonnet", value: 32, unit: "M", color: "#3B82F6" },
        { name: "Opus", value: 18, unit: "M", color: "#22C55E" },
        { name: "Haiku", value: 24, unit: "M", color: "#06B6D4" },
        { name: "Cache", value: 10.2, unit: "M", color: "#F97316" },
      ],
    },
    layout: { legend: "hide", summary: "top", stats: "hide" },
  };
}`
	ringSrc := `export function meta() { return { name: "点阵圆环", icon: "gauge", sources: {} }; }
export function render() {
  return {
    title: "点阵圆环",
    icon: "gauge",
    big: "",
    sub: "今日活动完成度",
    lines: [],
    chart: {
      type: "progressRing",
      ring: { layout: "concentric", fill: "matrix", size: 120 },
      progress: [
        { name: "专注", value: 82, max: 100, unit: "%", color: "#3B82F6" },
        { name: "运动", value: 68, max: 100, unit: "%", color: "#22C55E" },
        { name: "阅读", value: 94, max: 100, unit: "%", color: "#F97316" },
      ],
    },
    layout: { legend: "hide", summary: "top", stats: "hide" },
  };
}`
	gaugeSrc := `export function meta() { return { name: "点阵仪表", icon: "zap", sources: {} }; }
export function render() {
  return {
    title: "点阵仪表",
    icon: "zap",
    big: "",
    sub: "API 月度剩余配额",
    lines: [],
    chart: {
      type: "gauge",
      gauge: { used: 36, limit: 120, unit: "USD", label: "月度额度", reset: "9 天后重置", style: "matrix" },
    },
    layout: { legend: "hide", summary: "top", stats: "hide" },
  };
}`
	contribSrc := `export function meta() { return { name: "点阵热力", icon: "sparkles", sources: {} }; }
export function render() {
  const cells = [];
  for (let i = 0; i < 112; i++) {
    const v = (i * 7 + (i % 5) * 3) % 10;
    cells.push({ date: "2026-10-" + String((i % 28) + 1).padStart(2, "0"), value: v > 2 ? v : 0 });
  }
  return {
    title: "点阵热力",
    icon: "sparkles",
    big: "",
    sub: "近 16 周活跃灯阵",
    lines: [],
    chart: {
      type: "contrib",
      contrib: { weeks: 16, unit: "次", style: "matrix", cells },
    },
    layout: { legend: "hide", summary: "top", stats: "hide" },
  };
}`
	clockSrc := `export function meta() { return { name: "点阵时钟", icon: "calendar-days", sources: {} }; }
export function render() {
  return {
    title: "点阵时钟",
    icon: "calendar-days",
    big: "",
    sub: "",
    lines: [],
    chart: {
      type: "clock",
      clock: { style: "matrix", seconds: true, color: "#00F5D4" },
    },
    layout: { legend: "hide", summary: "hide", stats: "hide" },
  };
}`
	scripts := []struct {
		id, name, src string
		size          int
	}{
		{"mx-donut", "点阵占比", donutSrc, 2},
		{"mx-ring", "点阵圆环", ringSrc, 2},
		{"mx-gauge", "点阵仪表", gaugeSrc, 2},
		{"mx-contrib", "点阵热力", contribSrc, 2},
		{"mx-clock", "点阵时钟", clockSrc, 2},
	}
	for _, s := range scripts {
		a.store.SaveScript(store.Script{ID: s.id, Name: s.name, Home: true, Origin: store.OriginUser, Source: s.src})
		a.setSize(s.id, s.size)
	}

	tt := ui.NewTester(a.view, 1180, 900)
	settle(tt)
	settle(tt)
	requireText(t, tt, "84.2M")
	requireText(t, tt, "Sonnet")
	requireText(t, tt, "专注")
	requireText(t, tt, "剩余 70%")
	savePNG(t, tt, "28-matrix-family-home")

	// Open donut detail view and verify matrix style picker works.
	a.router.Push("/full/mx-donut")
	settle(tt)
	settle(tt)
	requireText(t, tt, "点阵灯")
	requireText(t, tt, "经典")
	savePNG(t, tt, "29-matrix-donut-full")

	// Open clock detail view and verify matrix clock style picker works.
	a.router.Replace("/full/mx-clock")
	settle(tt)
	settle(tt)
	requireText(t, tt, "点阵灯")
	requireText(t, tt, "时钟样式")
	savePNG(t, tt, "30-matrix-clock-full")

	// Open gauge & contrib detail views and save visual snapshots.
	a.router.Replace("/full/mx-gauge")
	settle(tt)
	settle(tt)
	requireText(t, tt, "点阵灯")
	savePNG(t, tt, "31-matrix-gauge-full")

	a.router.Replace("/full/mx-contrib")
	settle(tt)
	settle(tt)
	requireText(t, tt, "点阵灯")
	savePNG(t, tt, "32-matrix-contrib-full")
}
