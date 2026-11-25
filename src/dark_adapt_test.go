package main

import (
	"context"
	"image/color"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/colortheme"
	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

func TestReadableAndAccentColorAdaptation(t *testing.T) {
	darkBg := colortheme.DefaultPalette(true).Surface
	lightBg := colortheme.DefaultPalette(false).Surface

	for _, col := range []ui.Color{
		ui.Hex("#000000"),
		ui.Hex("#111827"),
		ui.Hex("#1E3A8A"),
	} {
		adaptedText := readableTextColor(col, true)
		if cr := colortheme.ContrastRatio(adaptedText, darkBg); cr < 4.5 {
			t.Fatalf("readableTextColor(%+v, dark) -> %+v contrast %.2f < 4.5", col, adaptedText, cr)
		}
		adaptedAccent := adaptAccentColor(col, true)
		if cr := colortheme.ContrastRatio(adaptedAccent, darkBg); cr < 3.0 {
			t.Fatalf("adaptAccentColor(%+v, dark) -> %+v contrast %.2f < 3.0", col, adaptedAccent, cr)
		}
	}

	for _, col := range []ui.Color{
		ui.Hex("#FFFFFF"),
		ui.Hex("#00F5D4"),
	} {
		adaptedText := readableTextColor(col, false)
		if cr := colortheme.ContrastRatio(adaptedText, lightBg); cr < 4.5 {
			t.Fatalf("readableTextColor(%+v, light) -> %+v contrast %.2f < 4.5", col, adaptedText, cr)
		}
	}
}

func TestMac9SkinStaysLightInDarkThemeMode(t *testing.T) {
	resetSkin(t)
	t.Cleanup(func() { setThemeMode(themeSystem) })

	setSkin(skinMac9)
	setThemeMode(themeDark)

	var gotDark bool
	tt := ui.NewTester(func(c *ui.Context) {
		applySkin(c)
		gotDark = pal(c).dark
	}, 240, 120)
	settle(tt)

	if gotDark {
		t.Fatalf("expected Mac OS 9 skin to stay light (pal.dark=false) under themeDark")
	}
}

func TestScriptWidgetsAdaptDarkModeHeadlessly(t *testing.T) {
	t.Cleanup(func() { setThemeMode(themeSystem); setSkin(skinDefault) })
	setSkin(skinDefault)
	setThemeMode(themeDark)

	a := newTestApp(t)
	now := time.Date(2026, 11, 25, 14, 30, 0, 0, time.Local)
	target := now.Add(48 * time.Hour).Format("2006-01-02 15:04")

	charts := []*engine.Chart{
		{
			Type: "kv",
			KV: []engine.KVItem{
				{Label: "Black", Value: "OK", Color: "#000000", Badge: "TAG"},
				{Label: "Slate", Value: "99ms", Color: "#111827", Status: "warn"},
				{Label: "Navy", Value: "UP", Color: "#1E3A8A"},
			},
		},
		{
			Type:      "countdown",
			Countdown: []engine.CountdownItem{{Label: "Release", Target: target, Color: "#111827"}},
		},
		{
			Type:      "checklist",
			Checklist: []engine.CheckItem{{Label: "Task", Done: true, Color: "#000000", Badge: "DONE"}},
		},
		{
			Type:     "progressBar",
			Bar:      &engine.BarStyle{Style: "gradient"},
			Progress: []engine.Progress{{Name: "Usage", Value: 65, Max: 100, Color: "#111827", To: "#1E3A8A"}},
		},
		{
			Type:  "clock",
			Clock: &engine.ClockSpec{Style: "digital", Color: "#111827", Neon: true},
		},
		{
			Type:   "donut",
			Slices: []engine.Slice{{Name: "A", Value: 40, Color: "#111827"}, {Name: "B", Value: 60, Color: "#1E3A8A"}},
		},
		{
			Type:   "line",
			Labels: []string{"1", "2", "3"},
			Series: []engine.Series{{Name: "DarkSeries", Color: "#000000", Points: []float64{10, 20, 15}}},
		},
	}

	for _, ch := range charts {
		ch := ch
		tt := ui.NewTester(func(c *ui.Context) {
			applySkin(c)
			p := pal(c)
			ui.Column(c).Fill().Background(p.card).Padding(12).Children(func() {
				if ch.Type == "line" {
					Chart(c, ChartProps{Data: ch, Height: 140, Full: true})
				} else {
					widgetView(c, ch, 140, nil, true, now, 0)
				}
			})
		}, 360, 220)
		settle(tt)
		img := tt.Image()
		if img == nil {
			t.Fatalf("nil image for widget type %s in dark mode", ch.Type)
		}
		if ch.Type == "kv" || ch.Type == "countdown" || ch.Type == "checklist" {
			b := img.Bounds()
			var maxLum float64
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					r, g, bl, _ := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA).RGBA()
					lum := colortheme.RelativeLuminance(ui.RGB(uint8(r>>8), uint8(g>>8), uint8(bl>>8)))
					if lum > maxLum {
						maxLum = lum
					}
				}
			}
			if maxLum < 0.40 {
				t.Fatalf("widget %s with dark hex colors had no bright readable pixels in dark mode (maxLum=%.3f)", ch.Type, maxLum)
			}
		}
	}

	// Bento widget explicitly requesting light theme inside dark window must render its own light surface.
	bentoSpec := &engine.BentoSpec{
		Theme: "light",
		Hero: &engine.BentoHero{
			Label:      "IP",
			Value:      "203.0.113.10",
			Location:   "Tokyo, JP",
			Badge:      "Direct",
			BadgeColor: "#111827",
		},
	}
	bentoTT := ui.NewTester(func(c *ui.Context) {
		applySkin(c)
		p := pal(c)
		ui.Column(c).Fill().Background(p.card).Padding(16).Children(func() {
			a.bentoWidgetView(c, "bento-light-in-dark", bentoSpec, 520, 2, true)
		})
	}, 560, 240)
	settle(bentoTT)
	bentoImg := bentoTT.Image()
	if bentoImg == nil {
		t.Fatal("nil bento image")
	}
	b := bentoImg.Bounds()
	lightPixels := 0
	for y := b.Min.Y + 24; y < b.Max.Y-24; y++ {
		for x := b.Min.X + 24; x < b.Max.X-24; x++ {
			c := bentoImg.RGBAAt(x, y)
			if colortheme.RelativeLuminance(ui.RGB(c.R, c.G, c.B)) > 0.80 {
				lightPixels++
			}
		}
	}
	if lightPixels < 100 {
		t.Fatalf("expected Bento with theme:\"light\" inside dark mode to paint its own light surface (got %d bright pixels)", lightPixels)
	}
}

func TestScriptEngineReceivesThemeContext(t *testing.T) {
	src := `export function meta() { return { name: "theme-echo" }; }
export function render(ctx) {
  return { big: ctx.theme + ":" + String(ctx.dark) };
}`
	sc, err := engine.Compile("theme-echo.js", src)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	darkRes, err := sc.Run(context.Background(), engine.Input{Now: time.Now(), Theme: "dark"})
	if err != nil {
		t.Fatalf("Run dark: %v", err)
	}
	if darkRes.Big != "dark:true" {
		t.Fatalf("expected dark:true, got %q", darkRes.Big)
	}
	lightRes, err := sc.Run(context.Background(), engine.Input{Now: time.Now(), Theme: "light"})
	if err != nil {
		t.Fatalf("Run light: %v", err)
	}
	if lightRes.Big != "light:false" {
		t.Fatalf("expected light:false, got %q", lightRes.Big)
	}
}

func TestSyncAppearanceUpdatesActiveBarInstances(t *testing.T) {
	t.Cleanup(func() { setThemeMode(themeSystem); setSkin(skinDefault) })
	a := newTestApp(t)
	t.Cleanup(a.closeBarInstances)

	src := `<bar protocol-version="0" id="theme-sync" title="ThemeSync" version="1.0.0">
  <script><![CDATA[
    export default {
      render(env) {
        return {
          panel: h("Column", { preferredWidth: 260 },
            h("Text", {}, String(env.theme || ""))
          )
        };
      }
    };
  ]]></script>
</bar>`
	sc := store.Script{ID: "theme-sync", Name: "ThemeSync", Source: src, Kind: store.KindBar}
	if err := a.store.SaveScript(sc); err != nil {
		t.Fatalf("SaveScript: %v", err)
	}

	setThemeMode(themeDark)
	inst, _, err := a.ensureBarInstance(sc)
	if err != nil {
		t.Fatalf("ensureBarInstance: %v", err)
	}
	if !inst.WaitIdle(3 * time.Second) {
		t.Fatal("initial WaitIdle timed out")
	}
	if txt := inst.Snapshot().Panel.Children[0].Text; txt != "dark" {
		t.Fatalf("expected initial theme dark, got %q", txt)
	}

	setThemeMode(themeLight)
	a.syncAppearance()
	if !inst.WaitIdle(3 * time.Second) {
		t.Fatal("after syncAppearance WaitIdle timed out")
	}
	if txt := inst.Snapshot().Panel.Children[0].Text; txt != "light" {
		t.Fatalf("expected updated theme light after syncAppearance, got %q", txt)
	}
}
