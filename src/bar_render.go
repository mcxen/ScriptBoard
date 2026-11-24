package main

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/bar"
	"scriptboard/src/internal/barui"
)

// barTheme carries resolved colors for rendering a .bar virtual DOM tree in
// Auto, Light, or Dark preview/display mode.
type barTheme struct {
	dark           bool
	surface        ui.Color
	surfaceHover   ui.Color
	surfacePressed ui.Color
	text           ui.Color
	textMuted      ui.Color
	accent         ui.Color
	warning        ui.Color
	danger         ui.Color
	success        ui.Color
	border         ui.Color
}

const (
	barThemeAuto  = 0
	barThemeLight = 1
	barThemeDark  = 2
)

func barThemeFor(c *ui.Context, mode int) barTheme {
	dark := pal(c).dark
	switch mode {
	case barThemeLight:
		dark = false
	case barThemeDark:
		dark = true
	}
	t := barui.ThemeFor(dark, false)
	return barTheme{
		dark:           t.Dark,
		surface:        t.Surface,
		surfaceHover:   t.SurfaceHover,
		surfacePressed: t.SurfacePressed,
		text:           t.Text,
		textMuted:      t.TextMuted,
		accent:         t.Accent,
		warning:        t.Warning,
		danger:         t.Danger,
		success:        t.Success,
		border:         t.Border,
	}
}

func (th barTheme) toBarUI(vibrancy bool) barui.Theme {
	out := barui.ThemeFor(th.dark, vibrancy)
	out.Surface = th.surface
	if !vibrancy {
		out.SurfaceHover = th.surfaceHover
		out.SurfacePressed = th.surfacePressed
		out.Border = th.border
	}
	out.Text = th.text
	out.TextMuted = th.textMuted
	out.Accent = th.accent
	out.Warning = th.warning
	out.Danger = th.danger
	out.Success = th.success
	return out
}

func (th barTheme) color(name string, fallback ui.Color) ui.Color {
	return th.toBarUI(false).Color(name, fallback)
}

// renderBarPanel paints a .bar Snapshot panel inside a rounded card matching
// the Metrix.Bar floating panel / preview appearance.
func renderBarPanel(c *ui.Context, snap bar.Snapshot, th barTheme, width float32, onAction func(string)) ui.Element {
	return renderBarPanelInteractive(c, snap, th, width, "", onAction, nil)
}

func renderBarPanelInteractive(c *ui.Context, snap bar.Snapshot, th barTheme, width float32, material string, onAction func(string), onActionArg func(string, any)) ui.Element {
	reqW := float32(0)
	if width != 300 && width > 0 {
		reqW = width
	}
	return barui.RenderPanel(c, snap, th.toBarUI(false), barui.RenderOptions{
		Width:       reqW,
		Material:    material,
		LoadingText: tr("加载中…"),
		IconLookup:  icon,
		OnAction:    onAction,
		OnActionArg: onActionArg,
	})
}

func renderBarNode(c *ui.Context, n *bar.Node, th barTheme, onAction func(string)) {
	renderBarNodeInteractive(c, n, th, false, onAction, nil)
}

func renderBarNodeInteractive(c *ui.Context, n *bar.Node, th barTheme, vibrancy bool, onAction func(string), onActionArg func(string, any)) {
	barui.RenderNode(c, n, th.toBarUI(vibrancy), barui.RenderOptions{
		LoadingText: tr("加载中…"),
		IconLookup:  icon,
		OnAction:    onAction,
		OnActionArg: onActionArg,
	})
}

func propFloat(m map[string]any, key string) (float32, bool) {
	return barui.PropFloat(m, key)
}

func propFloatSlice(m map[string]any, key string) ([]float32, bool) {
	return barui.PropFloatSlice(m, key)
}

func propString(m map[string]any, key string) (string, bool) {
	return barui.PropString(m, key)
}

func parseShortcutSpec(s string) (ui.Modifiers, ui.Key, bool) {
	parts := strings.Split(s, "+")
	if len(parts) == 0 {
		return 0, 0, false
	}
	var mod ui.Modifiers
	for i := 0; i < len(parts)-1; i++ {
		switch strings.ToLower(strings.TrimSpace(parts[i])) {
		case "cmdorctrl", "cmd", "ctrl", "meta", "super":
			mod |= ui.Cmd
		case "alt", "opt", "option":
			mod |= ui.Alt
		case "shift":
			mod |= ui.Shift
		}
	}
	k := strings.ToUpper(strings.TrimSpace(parts[len(parts)-1]))
	if len(k) == 1 && k[0] >= 'A' && k[0] <= 'Z' {
		return mod, ui.KeyA + ui.Key(k[0]-'A'), true
	}
	return 0, 0, false
}

func formatShortcutDisplay(s string) string {
	_ = strconv.Itoa(0)
	return s
}
