package engine

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func rawSources(m map[string]any) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(m))
	for k, v := range m {
		b, _ := json.Marshal(v)
		out[k] = b
	}
	return out
}

func TestProgressBarStandalone(t *testing.T) {
	// 1. Array shorthand directly on Result.progressBar (no chart field).
	arrSrc := `export function meta() { return { name: "数组进度", sources: {} }; }
export function render(ctx) {
  return {
    title: "数组进度",
    big: "50%",
    sub: "",
    lines: [],
    progressBar: [{ name: "阅读", value: 5, max: 10, unit: "章", color: "#00F5D4" }],
  };
}`
	s, err := CompileLang("pb-arr.js", arrSrc, "zh")
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Run(context.Background(), Input{Now: time.Now(), Lang: "zh"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Chart != nil {
		t.Fatalf("expected nil chart for standalone progressBar, got %+v", res.Chart)
	}
	if res.ProgressBar == nil || len(res.ProgressBar.Items) != 1 {
		t.Fatalf("progressBar %+v", res.ProgressBar)
	}
	if it := res.ProgressBar.Items[0]; it.Name != "阅读" || it.Percent() != 50 || it.Color != "#00F5D4" {
		t.Errorf("item %+v", it)
	}

	// 2. Full styled neon object directly on Result.progressBar.
	neonSrc := `export function meta() { return { name: "霓虹进度", sources: {} }; }
export function render(ctx) {
  return {
    title: "霓虹进度",
    big: "78%",
    sub: "本周目标",
    lines: [],
    progressBar: {
      style: "neon",
      height: 12,
      track: "dark",
      glow: 1.2,
      segments: 12,
      hideLabel: false,
      items: [
        { name: "季度 OKR", value: 78, max: 100, unit: "%", color: "#00F5D4", to: "#38BDF8" },
      ],
    },
  };
}`
	s2, err := CompileLang("pb-neon.js", neonSrc, "zh")
	if err != nil {
		t.Fatal(err)
	}
	res2, err := s2.Run(context.Background(), Input{Now: time.Now(), Lang: "zh"})
	if err != nil {
		t.Fatal(err)
	}
	if res2.Chart != nil {
		t.Fatalf("expected nil chart for neon progressBar, got %+v", res2.Chart)
	}
	pb := res2.ProgressBar
	if pb == nil || pb.Style != "neon" || pb.Height != 12 || pb.Track != "dark" || pb.Glow != 1.2 || pb.Segments != 12 || len(pb.Items) != 1 {
		t.Fatalf("neon progressBar %+v", pb)
	}
	if pb.Items[0].To != "#38BDF8" {
		t.Errorf("expected gradient target #38BDF8, got %q", pb.Items[0].To)
	}
}

func TestRichWidgetsFromScript(t *testing.T) {
	for _, typ := range []string{"progressBar", "donut", "checklist", "kv", "countdown", "clock", "contrib"} {
		if UnknownChartType(typ) {
			t.Errorf("%q reported as unknown chart type", typ)
		}
	}
	if !UnknownChartType("pie") {
		t.Error("pie should remain unknown")
	}

	cases := []struct {
		name  string
		body  string
		check func(t *testing.T, res Result)
	}{
		{
			name: "donut",
			body: `{ type: "donut", donut: { center: "¥6420", sub: "总计", thickness: 0.3, neon: true }, slices: [{ name: "餐饮", value: 2180, color: "#00F5D4", unit: "元" }] }`,
			check: func(t *testing.T, res Result) {
				if res.Chart == nil || res.Chart.Donut == nil || !res.Chart.Donut.Neon || len(res.Chart.Slices) != 1 || res.Chart.Slices[0].Value != 2180 {
					t.Errorf("donut %+v", res.Chart)
				}
			},
		},
		{
			name: "checklist",
			body: `{ type: "checklist", checklist: [{ label: "晨间复盘", done: true, note: "09:00", badge: "工作", color: "#10B981" }] }`,
			check: func(t *testing.T, res Result) {
				if res.Chart == nil || len(res.Chart.Checklist) != 1 || !res.Chart.Checklist[0].Done || res.Chart.Checklist[0].Badge != "工作" {
					t.Errorf("checklist %+v", res.Chart)
				}
			},
		},
		{
			name: "kv",
			body: `{ type: "kv", kv: [{ label: "API", value: "24ms", sub: "Tokyo", badge: "OK", status: "ok" }] }`,
			check: func(t *testing.T, res Result) {
				if res.Chart == nil || len(res.Chart.KV) != 1 || res.Chart.KV[0].Status != "ok" || res.Chart.KV[0].Value != "24ms" {
					t.Errorf("kv %+v", res.Chart)
				}
			},
		},
		{
			name: "countdown",
			body: `{ type: "countdown", countdown: [{ label: "新年", target: "2027-01-01", start: "2026-01-01", style: "neon", color: "#00F5D4" }] }`,
			check: func(t *testing.T, res Result) {
				if res.Chart == nil || len(res.Chart.Countdown) != 1 || res.Chart.Countdown[0].Style != "neon" {
					t.Errorf("countdown %+v", res.Chart)
				}
			},
		},
		{
			name: "clock",
			body: `{ type: "clock", clock: { style: "world", seconds: true, neon: true, color: "#00F5D4", zones: [{ label: "上海", offset: 8 }] } }`,
			check: func(t *testing.T, res Result) {
				if res.Chart == nil || res.Chart.Clock == nil || res.Chart.Clock.Style != "world" || !res.Chart.Clock.Neon || len(res.Chart.Clock.Zones) != 1 {
					t.Errorf("clock %+v", res.Chart)
				}
			},
		},
		{
			name: "contrib",
			body: `{ type: "contrib", contrib: { color: "#10B981", max: 4, weeks: 12, unit: "次", neon: true, cells: [{ date: "2026-11-20", value: 3 }] } }`,
			check: func(t *testing.T, res Result) {
				if res.Chart == nil || res.Chart.Contrib == nil || res.Chart.Contrib.Weeks != 12 || len(res.Chart.Contrib.Cells) != 1 || res.Chart.Contrib.Cells[0].Value != 3 {
					t.Errorf("contrib %+v", res.Chart)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := `export function meta() { return { name: "` + tc.name + `", sources: {} }; }
export function render(ctx) { return { title: "` + tc.name + `", big: "", sub: "", lines: [], chart: ` + tc.body + ` }; }`
			s, err := CompileLang(tc.name+".js", src, "zh")
			if err != nil {
				t.Fatal(err)
			}
			res, err := s.Run(context.Background(), Input{Now: time.Now(), Lang: "zh"})
			if err != nil {
				t.Fatal(err)
			}
			tc.check(t, res)
		})
	}
}

func TestNewStyleBuiltins(t *testing.T) {
	now := time.Date(2026, 11, 21, 9, 30, 0, 0, time.FixedZone("CST", 8*3600))
	byID := map[string]string{}
	for _, b := range Builtins() {
		byID[b.ID] = b.Source
	}

	run := func(t *testing.T, id, lang string, in Input) Result {
		t.Helper()
		src, ok := byID[id]
		if !ok {
			t.Fatalf("missing builtin %q", id)
		}
		s, err := CompileLang(id+".js", src, lang)
		if err != nil {
			t.Fatalf("%s (%s) compile: %v", id, lang, err)
		}
		in.Now = now
		in.Lang = lang
		res, err := s.Run(context.Background(), in)
		if err != nil {
			t.Fatalf("%s (%s) run: %v", id, lang, err)
		}
		if res.Title == "" || res.Big == "" {
			t.Errorf("%s (%s): empty title or big: %+v", id, lang, res)
		}
		return res
	}

	// 1. ai-claude-neon: neon progressBar (view=0), donut (view=1), contrib (view=2)
	claudeUsage := map[string]any{
		"claude-code": map[string]any{
			"limit":      1000000.0,
			"blockLimit": 200000.0,
			"windows":    map[string]any{"today": 50000.0, "week": 240000.0, "month": 620000.0, "block": 80000.0},
			"models": []any{
				map[string]any{"name": "claude-sonnet-4", "value": 420000.0},
				map[string]any{"name": "claude-opus-4", "value": 200000.0},
			},
			"daily": []any{
				map[string]any{"name": "2026-11-20", "value": 45000.0},
				map[string]any{"name": "2026-11-21", "value": 50000.0},
			},
		},
	}
	r0 := run(t, "ai-claude-neon", "zh", Input{Usage: claudeUsage, View: map[string]float64{"view": 0}})
	if r0.Chart != nil || r0.ProgressBar == nil || r0.ProgressBar.Style != "neon" || len(r0.ProgressBar.Items) < 2 {
		t.Errorf("ai-claude-neon view=0: chart=%+v pb=%+v", r0.Chart, r0.ProgressBar)
	}
	r1 := run(t, "ai-claude-neon", "zh", Input{Usage: claudeUsage, View: map[string]float64{"view": 1}})
	if r1.Chart == nil || r1.Chart.Type != "donut" || len(r1.Chart.Slices) != 2 {
		t.Errorf("ai-claude-neon view=1: %+v", r1.Chart)
	}
	r2 := run(t, "ai-claude-neon", "en", Input{Usage: claudeUsage, View: map[string]float64{"view": 2}})
	if r2.Chart == nil || r2.Chart.Type != "contrib" || r2.Chart.Contrib == nil || len(r2.Chart.Contrib.Cells) != 2 {
		t.Errorf("ai-claude-neon view=2: %+v", r2.Chart)
	}

	// 2. ai-balance-neon: standalone progressBar
	balUsage := map[string]any{
		"deepseek-balance":   map[string]any{"balance": 42.5, "limit": 100.0, "used": 57.5, "unit": "CNY"},
		"openrouter-credits": map[string]any{"balance": 12.0, "limit": 50.0, "used": 38.0, "unit": "USD"},
	}
	rb := run(t, "ai-balance-neon", "zh", Input{Usage: balUsage})
	if rb.Chart != nil || rb.ProgressBar == nil || rb.ProgressBar.Style != "neon" || len(rb.ProgressBar.Items) != 2 {
		t.Errorf("ai-balance-neon: chart=%+v pb=%+v", rb.Chart, rb.ProgressBar)
	}

	// 3. goldacc-kv: kv board with change badge
	rga := run(t, "goldacc-kv", "zh", Input{Sources: rawSources(map[string]any{
		"icbc": map[string]any{"data": map[string]any{"price": 648.5, "change": 2.4, "change_pct": 0.37, "update_time": "09:30"}},
		"ms":   map[string]any{"data": map[string]any{"price": 647.2, "change": -1.1, "change_pct": -0.17}},
	})})
	if rga.Chart == nil || rga.Chart.Type != "kv" || len(rga.Chart.KV) != 2 || rga.Chart.KV[0].Badge != "+0.37%" {
		t.Errorf("goldacc-kv: %+v", rga.Chart)
	}

	// 4. gold-kv: kv store gold board
	rg := run(t, "gold-kv", "zh", Input{Sources: rawSources(map[string]any{
		"gold": map[string]any{
			"date":   "2026-11-21",
			"stores": []any{map[string]any{"brand": "周大福", "price": 815}, map[string]any{"brand": "老凤祥", "price": 810}},
			"metals": []any{map[string]any{"name": "今日金价", "unit": "元/克", "sell_price": 640}},
		},
	})})
	if rg.Chart == nil || rg.Chart.Type != "kv" || len(rg.Chart.KV) != 2 || rg.Chart.KV[0].Badge != "+175" {
		t.Errorf("gold-kv: %+v", rg.Chart)
	}

	// 6. aqi-neon: standalone neon progressBar (no chart)
	raqi := run(t, "aqi-neon", "zh", Input{Sources: rawSources(map[string]any{
		"air": map[string]any{"current": map[string]any{"us_aqi": 42, "pm2_5": 14.5, "pm10": 28.0, "time": "2026-11-21T09:00"}},
	})})
	if raqi.Chart != nil || raqi.ProgressBar == nil || raqi.ProgressBar.Style != "neon" || len(raqi.ProgressBar.Items) != 3 {
		t.Errorf("aqi-neon: chart=%+v pb=%+v", raqi.Chart, raqi.ProgressBar)
	}

	// 7. cnwx-kv: kv weather status panel
	rcn := run(t, "cnwx-kv", "zh", Input{Sources: rawSources(map[string]any{
		"now": map[string]any{
			"code": 200,
			"data": map[string]any{
				"weather":     map[string]any{"temperature": 18, "condition_code": "00", "wind_direction": "东南风", "wind_power": "3", "humidity": 58, "precipitation": 0, "pressure": 1018},
				"air_quality": map[string]any{"aqi": 38},
				"sunrise":     map[string]any{"sunrise_desc": "06:24", "sunset_desc": "16:56"},
			},
		},
	})})
	if rcn.Chart == nil || rcn.Chart.Type != "kv" || len(rcn.Chart.KV) < 5 {
		t.Errorf("cnwx-kv: %+v", rcn.Chart)
	}

	// 8. weight-contrib: contrib (view=0) & neon progressBar (view=1)
	rw0 := run(t, "weight-contrib", "zh", Input{Metrics: sampleMetrics(now), Settings: map[string]any{"weightUnit": "kg"}, View: map[string]float64{"view": 0}})
	if rw0.Chart == nil || rw0.Chart.Type != "contrib" || rw0.Chart.Contrib == nil || len(rw0.Chart.Contrib.Cells) != 84 {
		t.Errorf("weight-contrib view=0: %+v", rw0.Chart)
	}
	rw1 := run(t, "weight-contrib", "en", Input{Metrics: sampleMetrics(now), Settings: map[string]any{"weightUnit": "kg"}, View: map[string]float64{"view": 1}})
	if rw1.Chart != nil || rw1.ProgressBar == nil || rw1.ProgressBar.Style != "neon" || len(rw1.ProgressBar.Items) != 3 {
		t.Errorf("weight-contrib view=1: chart=%+v pb=%+v", rw1.Chart, rw1.ProgressBar)
	}

	// 9. year-countdown: single hero (view=0) & milestones list (view=1)
	ry0 := run(t, "year-countdown", "zh", Input{View: map[string]float64{"view": 0}})
	if ry0.Chart == nil || ry0.Chart.Type != "countdown" || len(ry0.Chart.Countdown) != 1 {
		t.Errorf("year-countdown view=0: %+v", ry0.Chart)
	}
	ry1 := run(t, "year-countdown", "en", Input{View: map[string]float64{"view": 1}})
	if ry1.Chart == nil || ry1.Chart.Type != "countdown" || len(ry1.Chart.Countdown) != 4 {
		t.Errorf("year-countdown view=1: %+v", ry1.Chart)
	}

	// 10. world-clock: world clock + menubar (style switching is handled by clockStylePicker)
	rc := run(t, "world-clock", "zh", Input{})
	if rc.Chart == nil || rc.Chart.Type != "clock" || rc.Chart.Clock == nil || rc.Chart.Clock.Style != "world" {
		t.Errorf("world-clock: %+v", rc.Chart)
	}
	if rc.MenuBar == nil || rc.MenuBar.Title == "" {
		t.Errorf("world-clock missing menubar: %+v", rc.MenuBar)
	}

	// 11. fxcny-table: native interactive table + fallback + menubar
	rf := run(t, "fxcny-table", "zh", Input{
		Sources: rawSources(map[string]any{
			"rates": map[string]any{
				"date":  "2026-11-21",
				"rates": map[string]any{"USD": 0.138, "EUR": 0.129, "JPY": 21.4},
			},
		}),
	})
	if rf.Chart == nil || rf.Chart.Type != "table" || rf.Chart.Table == nil || len(rf.Chart.Table.Rows) != 3 {
		t.Fatalf("fxcny-table failed: %+v", rf.Chart)
	}
	if rf.MenuBar == nil || len(rf.Menu) == 0 {
		t.Errorf("fxcny-table missing menubar/menu: %+v %+v", rf.MenuBar, rf.Menu)
	}
	rfEn := run(t, "fxcny-table", "en", Input{
		Sources: rawSources(map[string]any{
			"rates": map[string]any{
				"date":  "2026-11-21",
				"rates": map[string]any{"USD": 0.138},
			},
		}),
	})
	if rfEn.Chart == nil || rfEn.Chart.Table == nil || rfEn.Chart.Table.Columns[0].Title != "Currency" {
		t.Errorf("fxcny-table en columns: %+v", rfEn.Chart)
	}
}
