package engine

import (
	"embed"
	"strings"
)

//go:embed scripts/*.js
var builtinFS embed.FS

// Builtin is a script that ships with the app. ID is its file name without
// the extension, which is also its ID in the store.
type Builtin struct {
	ID     string
	Source string
}

// Builtins returns the scripts that ship with the app, in a fixed order.
func Builtins() []Builtin {
	order := []string{
		"weight", "cal", "world-clock", "year-countdown",
		"fx", "fxcny-table", "bing", "weather", "cnwx-kv", "aqi-neon",
		"gold-kv", "goldacc-kv", "goldk", "hs300", "btc",
		"ai-claude-neon", "ai-balance-neon", "weight-contrib", "ip-dashboard",
	}
	out := make([]Builtin, 0, len(order))
	for _, id := range order {
		b, err := builtinFS.ReadFile("scripts/" + id + ".js")
		if err != nil {
			panic("missing builtin script " + id + ": " + err.Error())
		}
		out = append(out, Builtin{ID: strings.TrimSpace(id), Source: string(b)})
	}
	return out
}

// DefaultSource is what a new script starts with.
const DefaultSource = `// 新脚本：render(ctx) 返回要显示的内容。
export function meta() {
  return { name: "新脚本", sources: {} };
}

export function render(ctx) {
  const pts = ctx.metrics.weight ?? [];
  return {
    title: "新脚本",
    big: String(pts.length),
    sub: "条体重记录",
    lines: [],
    chart: null,
  };
}
`

const defaultSourceEN = `// New script: render(ctx) returns what to show.
// ctx.lang is "zh" or "en", the language of the user.
export function meta() {
  return { name: "New script", sources: {} };
}

export function render(ctx) {
  const pts = ctx.metrics.weight ?? [];
  return {
    title: "New script",
    big: String(pts.length),
    sub: "weight records",
    lines: [],
    chart: null,
  };
}
`

// DefaultSourceFor is DefaultSource in the language of the user ("zh", "en").
func DefaultSourceFor(lang string) string {
	if lang == "en" {
		return defaultSourceEN
	}
	return DefaultSource
}
