package main

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
)

// resetSkin puts the default skin and language back after a test.
func resetSkin(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		setSkin(skinDefault)
		setLang(langZH)
	})
}

// The skin is saved with the settings and comes back with them.
func TestSkinIsSaved(t *testing.T) {
	resetSkin(t)
	a := newTestApp(t)
	a.set.Skin = skinMac9
	a.saveSettings()
	if got := a.store.Settings().Skin; got != skinMac9 {
		t.Fatalf("stored skin %q, want mac9", got)
	}
	setSkin(skinDefault)
	setSkin(a.store.Settings().Skin)
	if curSkin() != skinMac9 {
		t.Fatalf("skin %q after reload", curSkin())
	}
	setSkin("no-such-skin")
	if curSkin() != skinDefault {
		t.Errorf("an unknown skin gave %q, want the default", curSkin())
	}
}

// The Mac OS 9 skin is square and light; the default keeps its rounded look.
func TestSkinTokens(t *testing.T) {
	def := brandFor(skinDefault).Tokens(false)
	mac := brandFor(skinMac9).Tokens(false)
	if mac.BorderRadius != 0 || mac.BorderRadiusLG != 0 {
		t.Errorf("mac9 radius %v/%v, want 0", mac.BorderRadius, mac.BorderRadiusLG)
	}
	if def.BorderRadius == 0 {
		t.Errorf("the default skin lost its rounded corners")
	}
	if mac.Primary == def.Primary {
		t.Errorf("mac9 keeps the default primary color")
	}
	if brandFor(skinMac9).Tokens(true).BgLayout != mac.BgLayout {
		t.Errorf("mac9 dark differs from its light: it should stay light")
	}
}

// Mac OS 9 shows on the home page and the settings page with its own look.
func TestMac9Pages(t *testing.T) {
	resetSkin(t)
	a := newTestApp(t)
	setSkin(skinMac9)
	tt := ui.NewTester(a.view, 960, 640)
	settle(tt)
	requireText(t, tt, "首页")
	savePNG(t, tt, "16-mac9-home")

	click(t, tt, "设置")
	requireText(t, tt, "外观")
	savePNG(t, tt, "16-mac9-settings")
}

// Switching the appearance on the settings page changes every page at once.
func TestSwitchSkinOnSettings(t *testing.T) {
	resetSkin(t)
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 960, 900)
	click(t, tt, "设置")
	click(t, tt, "Mac OS 9")
	if curSkin() != skinMac9 || a.store.Settings().Skin != skinMac9 {
		t.Fatalf("skin %q, stored %q", curSkin(), a.store.Settings().Skin)
	}
	click(t, tt, "首页")
	requireText(t, tt, "体重 · kg")
}

// The retro chart styles draw on any series chart, in either skin.
func TestRetroChartStyles(t *testing.T) {
	resetSkin(t)
	ch := &engine.Chart{
		Type:   "line",
		Labels: []string{"a", "b", "c", "d", "e", "f"},
		Series: []engine.Series{
			{Name: "x", Color: "#4FC3F7", Points: []float64{3, 5, 4, 7, 6, 8}},
			{Name: "y", Color: "#FFB74D", Points: []float64{2, 3, 6, 5, 7, 6}},
		},
	}
	for _, skinName := range []string{skinDefault, skinMac9} {
		setSkin(skinName)
		for _, style := range []string{"stripe", "pixel", "grid"} {
			view := func(c *ui.Context) {
				Chart(c, ChartProps{Data: ch, Style: style, Height: 220, Full: true})
			}
			tt := ui.NewTester(view, 520, 280)
			settle(tt)
			if got := resolveStyle(ch, style); got != style {
				t.Errorf("%s: style %q resolved to %q", skinName, style, got)
			}
			savePNG(t, tt, fmt.Sprintf("17-retro-%s-%s", orDefault(skinName), style))
		}
	}
	// Also capture compact card (spark) grid line charts for sparse & dense series.
	setSkin(skinDefault)
	a := newTestApp(t)
	a.styles["weight"] = "grid"
	a.styles["fx"] = "grid"
	a.styles["weather"] = "grid"
	for _, sc := range a.store.Scripts() {
		sc.Home = sc.ID == "weight" || sc.ID == "fx" || sc.ID == "weather"
		a.store.SaveScript(sc)
	}
	tt := ui.NewTester(a.view, 1200, 520)
	settle(tt)
	settle(tt)
	savePNG(t, tt, "33-grid-spark-cards")
}

func orDefault(s string) string {
	if s == "" {
		return "default"
	}
	return s
}

// The new settings and chart labels are translated.
func TestRetroPagesInEnglish(t *testing.T) {
	resetSkin(t)
	a := newTestApp(t)
	setLang(langEN)
	tt := ui.NewTester(a.view, 960, 900)
	click(t, tt, "Settings")
	requireText(t, tt, "Appearance")
	onlyEnglish(t, tt, "settings")
}
