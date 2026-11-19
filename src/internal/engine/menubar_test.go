package engine

import (
	"context"
	"testing"
	"time"
)

// A script asks for the menu bar in meta, and its result gives the bar's title,
// tooltip and items.
func TestMenuBarFieldsFromScript(t *testing.T) {
	src := `export function meta() { return { name: "天气", menubar: true, sources: {} }; }
export function render(ctx) { return { title: "上海天气", big: "18°", sub: "多云", lines: [], chart: null,
  menubar: { title: "18° 多云", tooltip: "上海 · 今天 11°~18°" },
  menu: [{ label: "打开天气网", url: "https://example.com/weather" }, { separator: true }, { label: "今天 11° ~ 18°" }] }; }`
	s, err := CompileLang("weather.js", src, "zh")
	if err != nil {
		t.Fatal(err)
	}
	if !s.Meta.Menubar {
		t.Error("the script's menubar is not read")
	}
	res, err := s.Run(context.Background(), Input{Now: time.Now(), Lang: "zh"})
	if err != nil {
		t.Fatal(err)
	}
	if res.MenuBar == nil || res.MenuBar.Title != "18° 多云" || res.MenuBar.Tooltip != "上海 · 今天 11°~18°" {
		t.Fatalf("menu bar %+v", res.MenuBar)
	}
	if len(res.Menu) != 3 || res.Menu[0].URL != "https://example.com/weather" || !res.Menu[1].Separator || res.Menu[2].Label != "今天 11° ~ 18°" {
		t.Errorf("menu %+v", res.Menu)
	}
}

// A script that does not ask for the menu bar has none, and its result shows
// none.
func TestMenuBarIsOffUnlessTheScriptAsks(t *testing.T) {
	src := `export function meta() { return { name: "x", sources: {} }; }
export function render(ctx) { return { title: "x", big: "1", sub: "", lines: [], chart: null }; }`
	s, err := CompileLang("plain.js", src, "zh")
	if err != nil {
		t.Fatal(err)
	}
	if s.Meta.Menubar {
		t.Error("a script that did not ask has the menu bar")
	}
	res, err := s.Run(context.Background(), Input{Now: time.Now(), Lang: "zh"})
	if err != nil {
		t.Fatal(err)
	}
	if res.MenuBar != nil || len(res.Menu) != 0 {
		t.Errorf("menu bar %+v, menu %+v", res.MenuBar, res.Menu)
	}
}
