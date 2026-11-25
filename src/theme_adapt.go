package main

import (
	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/colortheme"
	"scriptboard/src/internal/engine"
)

func paletteToColorTheme(p palette) colortheme.Palette {
	surf := p.card
	if surf.A == 0 || surf == ui.Transparent {
		surf = colortheme.DefaultPalette(p.dark).Surface
	} else if surf.A < 255 {
		surf = colortheme.CompositeOver(surf, colortheme.DefaultPalette(p.dark).Surface)
	}
	return colortheme.Palette{
		Dark:       p.dark,
		Surface:    surf,
		SurfaceAlt: p.canvas,
		Text:       p.text,
		TextMuted:  p.muted,
		Border:     p.border,
	}
}

func adaptColor(col ui.Color, role colortheme.Role, p palette) ui.Color {
	return colortheme.Adapt(col, role, paletteToColorTheme(p))
}

func adaptTextColor(col ui.Color, dark bool) ui.Color {
	return colortheme.Adapt(col, colortheme.RoleText, colortheme.DefaultPalette(dark))
}

func adaptAccentColor(col ui.Color, dark bool) ui.Color {
	return colortheme.Adapt(col, colortheme.RoleAccent, colortheme.DefaultPalette(dark))
}

func parseCustomColor(raw string) (ui.Color, bool) {
	col, ok := colortheme.Parse(raw)
	if !ok || col.A == 0 {
		return ui.Color{}, false
	}
	return col, true
}

func parseColorRole(raw string, role colortheme.Role, p palette, fallback ui.Color) ui.Color {
	col, ok := parseCustomColor(raw)
	if !ok {
		return fallback
	}
	return adaptColor(col, role, p)
}

func onColor(bg ui.Color) ui.Color {
	return colortheme.OnColor(bg)
}

func accentFor(res engine.Result, fallback ui.Color, dark bool) ui.Color {
	return adaptAccentColor(accentOf(res, fallback), dark)
}
