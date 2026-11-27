// Package engine runs ScriptBoard scripts. A script is an ES module run by
// moejs, a pure-Go JavaScript runtime. It exports two functions:
//
//	export function meta() { return { name, sources } }
//	export function render(ctx) { return { title, big, sub, lines, chart } }
//
// meta names the data sources, which the host fetches (see Fetch). render gets
// everything as ctx and returns what the page or card shows. A script never
// touches the network or the UI itself.
package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/MoeForge/moejs"

	"scriptboard/src/internal/cmdrun"
	"scriptboard/src/internal/cred"
)

const (
	// RunTimeout stops a script that runs too long, as a loop that never ends.
	RunTimeout = 3 * time.Second
	// MemoryLimit caps what one run allocates, in bytes.
	MemoryLimit = 16 << 20
)

// Meta is what a script's meta() returns.
type Meta struct {
	Name string `json:"name"`
	// Description says in a line what the script shows.
	Description string `json:"description"`
	// Version is the script's own version, such as "1.0.0". The host does not
	// compare it; the editor and the community show it.
	Version string `json:"version"`
	// Platforms lists where the script can run: "desktop" and "mobile" (package
	// caps). Missing, it runs anywhere its other needs allow. A script that
	// reads the computer's own files is desktop only, whatever it says.
	Platforms []string `json:"platforms"`
	// Needs lists the platform features the script depends on: "network",
	// "files", "exec" and "key" (package caps). The host adds what it sees the
	// script use: its sources, feeds and usage providers.
	Needs []string `json:"needs"`
	// Icon names a Lucide icon shipped with the app, as "coins".
	Icon string `json:"icon"`
	// Dims are the dimensions the user can adjust, such as the time range.
	Dims []Dim `json:"dims"`
	// Inputs are the values the user types in by hand for this script, such as
	// a weight or a price. The home card shows a quick-entry row for each; a
	// value is stored as a record of its Metric and read back as
	// ctx.metrics[Metric].
	Inputs []ManualInput `json:"inputs"`
	// Usage names the AI usage providers the script reads, as ctx.usage[id]:
	// tokens or a balance, with a limit, windows and breakdowns.
	Usage []string `json:"usage"`
	// Feeds names the data sources the script reads, as ctx.feeds[name]. The
	// host polls them and aggregates their samples by hour and by day.
	Feeds []string `json:"feeds"`
	// Commands names the command tasks the script reads, as ctx.commands[id].
	Commands []string `json:"commands,omitempty"`
	// Credentials declares managed host credentials (cookie/bearer/token/basic/header)
	// with automatic WebView/Browser/Ego/Manual refresh on 401/403.
	Credentials []cred.Spec `json:"credentials,omitempty"`
	// Sources maps a name to where its data comes from: an address, or a
	// request of its own (request.go). {start} and {end} expand to the dates of
	// the last 30 days, as yyyy-mm-dd.
	Sources map[string]Source `json:"sources"`
	// SourceHeaders gives the request headers of a source, by its name: a
	// Cookie header is how a source that needs the user's login is read, as
	// {"api": {"Cookie": "session=…"}}. The values stay in the script.
	SourceHeaders map[string]map[string]string `json:"sourceHeaders"`
	// History names the sources whose past responses the script wants, as
	// ctx.history[name]: the host keeps them on disk, so a script can draw a
	// curve from an API that only says what the price is now.
	History []string `json:"history"`
	// HistoryLimit is how many of the newest it wants; 0 is 120.
	HistoryLimit int `json:"historyLimit"`
	// Vars are the values the user switches, which fill {key} in the sources
	// (see vars.go); render reads the chosen ones as ctx.vars.
	Vars []Var `json:"vars"`
	// Menubar says the card can show in the menu bar (macOS) or notification
	// area: the user chooses whether it does. What it shows there is the
	// result's MenuBar and Menu.
	Menubar bool `json:"menubar"`
	// Live names the sources that are asked for again every few seconds while
	// the card is on screen (see live.go). Their history is kept for the script.
	Live *Live `json:"live"`
	// NetProbe enables host-side network, latency, exit-IP, and risk probing
	// whose structured snapshot is injected into render(ctx) as ctx.netprobe.
	NetProbe *NetProbeConfig `json:"netprobe,omitempty"`
}

// NetProbeConfig configures host-side network probing for a script.
// In meta() it may be written as `netprobe: true` or `{ rounds, includeRisk, includeSplit }`.
type NetProbeConfig struct {
	Rounds       int   `json:"rounds,omitempty"`
	IncludeRisk  *bool `json:"includeRisk,omitempty"`
	IncludeSplit *bool `json:"includeSplit,omitempty"`
}

// WantRisk reports whether IP risk / ASN enrichment should be fetched (default true).
func (n NetProbeConfig) WantRisk() bool {
	if n.IncludeRisk == nil {
		return true
	}
	return *n.IncludeRisk
}

// WantSplit reports whether website split-routing exit IP tests should run (default true).
func (n NetProbeConfig) WantSplit() bool {
	if n.IncludeSplit == nil {
		return true
	}
	return *n.IncludeSplit
}

// UnmarshalJSON accepts `true` or an object `{rounds, includeRisk, includeSplit}`.
func (n *NetProbeConfig) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "true" {
		*n = NetProbeConfig{Rounds: 12}
		return nil
	}
	if s == "false" || s == "null" {
		*n = NetProbeConfig{}
		return nil
	}
	type plain NetProbeConfig
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	*n = NetProbeConfig(p)
	return nil
}

// ManualInput is a value typed in by hand.
type ManualInput struct {
	Metric string  `json:"metric"` // what the records are stored under
	Label  string  `json:"label"`
	Unit   string  `json:"unit"`
	Step   float64 `json:"step"` // how far the arrows move the value; 0 is 0.1
}

// Dim is a dimension the user adjusts with a row of options. render gets the
// value of the chosen option as ctx.view[Key].
type Dim struct {
	Key     string      `json:"key"`
	Label   string      `json:"label"`
	Options []DimOption `json:"options"`
}

// DimOption is one choice of a Dim.
type DimOption struct {
	Value float64 `json:"value"`
	Label string  `json:"label"`
}

// Point is one recorded value. T is Unix milliseconds.
type Point struct {
	T int64   `json:"t"`
	V float64 `json:"v"`
}

// Series is one line of a chart.
type Series struct {
	Name   string    `json:"name"`
	Color  string    `json:"color"`
	Points []float64 `json:"points"`
	// Offset is the index on the chart's axis of the first point. Only the
	// "combo" chart uses it, so that a curve can cover part of the axis: a
	// solid part from the start and a dashed projection after it.
	Offset int `json:"offset"`
	// Dashed draws the line dashed, and leaves it unfilled: a projection.
	Dashed bool `json:"dashed"`
}

// Grid is a heatmap table, such as a retention matrix: rows of cells with a
// value from 0 to 100 each, shaded darker as the value grows. A cell with no
// value is -1. Average, when set, is a row of its own under the table.
type Grid struct {
	Columns []string  `json:"columns"`
	Rows    []GridRow `json:"rows"`
	Average []float64 `json:"average"`
	Footer  string    `json:"footer"`
}

// GridRow is one row of a Grid: its label, a note after the label (such as the
// number of users), and its cells.
type GridRow struct {
	Label string    `json:"label"`
	Note  string    `json:"note"`
	Cells []float64 `json:"cells"`
}

// Panel is one panel of a "panels" chart: a titled chart of its own, stacked
// under the others. Style is the look of its series (line, area, dot, bar, or
// step). A reference line is a dashed series of one value repeated.
type Panel struct {
	Title  string   `json:"title"`
	Style  string   `json:"style"`
	Weight float64  `json:"weight"`
	Series []Series `json:"series"`
}

// Ring is one circle of a "combo" chart: a share of 100 percent, with a label
// under it. Text is what the circle shows; the percent when it is empty.
type Ring struct {
	Label   string  `json:"label"`
	Percent float64 `json:"percent"`
	Text    string  `json:"text"`
	Color   string  `json:"color"`
}

// Candle is the open, high, low and close of one period, as a K-line.
type Candle struct {
	O float64 `json:"o"`
	H float64 `json:"h"`
	L float64 `json:"l"`
	C float64 `json:"c"`
}

// Gauge is a quantity with a limit, drawn as a ring of what is left: an AI
// plan's tokens, a balance, a quota. Used and Limit are in Unit.
type Gauge struct {
	Used  float64 `json:"used"`
	Limit float64 `json:"limit"`
	Unit  string  `json:"unit"`
	Label string  `json:"label"`           // what the quantity is, such as "本月额度"
	Reset string  `json:"reset"`           // when it renews, such as "3 天后重置"
	Style string  `json:"style,omitempty"` // "" (classic ring) | "matrix" (LED dot-matrix dial)
}

// Chart is what a result draws. Type "gauge" draws Gauge as a ring of what is
// left; "barh" draws Series[0] as horizontal bars, one per label. Type is "line" (the default), "dot" for
// scattered points, or "candle" for a K-line, which reads Candles instead of
// Series. Type "calendar" is a month of days: the host fills the lunar date
// and the holidays, and a click on a day shows them. Labels, one per point, name each point in the detail readout;
// XLabels, when set, label the first and the last point along the x axis.
type Chart struct {
	Type    string   `json:"type"`
	Series  []Series `json:"series"`
	Candles []Candle `json:"candles"`
	Gauge   *Gauge   `json:"gauge"`
	Labels  []string `json:"labels"`
	XLabels []string `json:"xLabels"`
	// Ticks, for a K-line, are the prices in time order; the host groups them
	// into Candles of Period seconds (PeriodMonth for months), and keeps the
	// last Limit candles when it is set.
	Ticks  []Point `json:"ticks"`
	Period int     `json:"period"`
	Limit  int     `json:"limit"`
	// Rings are the circles of a "combo" chart, drawn beside its curve.
	Rings []Ring `json:"rings"`
	// Grid is the table of a "heat" chart.
	Grid *Grid `json:"grid"`
	// Panels are the stacked charts of a "panels" chart.
	Panels []Panel `json:"panels"`
	// Progress are the bars of a "progress" chart: each a value against its
	// maximum (a completion), shown as a percentage bar.
	Progress []Progress `json:"progress"`
	// Bar styles a progress / progressBar component (BarStyle).
	Bar *BarStyle `json:"bar,omitempty"`
	// Ring styles a progressRing chart (RingStyle).
	Ring *RingStyle `json:"ring,omitempty"`
	// Slices and Donut are a "donut" share breakdown component.
	Slices []Slice     `json:"slices,omitempty"`
	Donut  *DonutStyle `json:"donut,omitempty"`
	// Checklist is a "checklist" task/habit list component.
	Checklist []CheckItem `json:"checklist,omitempty"`
	// KV is a "kv" key-value status panel component.
	KV []KVItem `json:"kv,omitempty"`
	// Countdown is a "countdown" target-date component.
	Countdown []CountdownItem `json:"countdown,omitempty"`
	// Clock is a "clock" analog/digital/world clock component.
	Clock *ClockSpec `json:"clock,omitempty"`
	// Contrib is a "contrib" GitHub-style activity grid component.
	Contrib *ContribSpec `json:"contrib,omitempty"`
	// Table is a "table" structured data table component.
	Table *TableSpec `json:"table,omitempty"`
	// Bento is a "bento" nested dashboard component.
	Bento *BentoSpec `json:"bento,omitempty"`
}

// BentoSpec configures a nested "bento" dashboard component.
type BentoSpec struct {
	Theme       string           `json:"theme,omitempty"` // "" (inherit) | "dark" | "light"
	Maskable    bool             `json:"maskable,omitempty"`
	Hero        *BentoHero       `json:"hero,omitempty"`
	Probes      *BentoProbeGroup `json:"probes,omitempty"`
	Pills       *BentoPillGroup  `json:"pills,omitempty"`
	Cells       []BentoCell      `json:"cells,omitempty"`
	DetailTitle string           `json:"detailTitle,omitempty"`
	DetailDesc  string           `json:"detailDesc,omitempty"`
	DetailTable *TableSpec       `json:"detailTable,omitempty"`
}

// BentoHero is the primary identity / KPI block in a Bento dashboard.
type BentoHero struct {
	Label      string   `json:"label,omitempty"`
	Flag       string   `json:"flag,omitempty"`
	Value      string   `json:"value"`
	Masked     string   `json:"masked,omitempty"`
	Location   string   `json:"location,omitempty"`
	Meta       string   `json:"meta,omitempty"`
	Badge      string   `json:"badge,omitempty"`
	BadgeColor string   `json:"badgeColor,omitempty"`
	Copy       string   `json:"copy,omitempty"`
	Details    []KVItem `json:"details,omitempty"`
}

// BentoProbeGroup is a grid of connectivity / health sub-cards inside Bento.
type BentoProbeGroup struct {
	Title      string       `json:"title,omitempty"`
	ActionText string       `json:"actionText,omitempty"`
	ActionURL  string       `json:"actionUrl,omitempty"`
	Columns    int          `json:"columns,omitempty"`
	Items      []BentoProbe `json:"items,omitempty"`
}

// BentoProbe is one latency / connectivity sub-card inside BentoProbeGroup.
type BentoProbe struct {
	Name    string `json:"name"`
	Icon    string `json:"icon,omitempty"`
	Color   string `json:"color,omitempty"`
	Flag    string `json:"flag,omitempty"`
	Tag     string `json:"tag,omitempty"`
	Latency int    `json:"latency"` // ms; <0 means timeout/fail
	MinMs   int    `json:"minMs,omitempty"`
	MaxMs   int    `json:"maxMs,omitempty"`
	Success int    `json:"success,omitempty"`
	Total   int    `json:"total,omitempty"`
	Status  string `json:"status,omitempty"` // ok | warn | slow | err | pending
	Text    string `json:"text,omitempty"`   // display override e.g. "24ms" or "超时"
	Dots    []int  `json:"dots,omitempty"`
	URL     string `json:"url,omitempty"`
}

// BentoPillGroup is a bottom strip of clickable summary pill sub-cards inside Bento.
type BentoPillGroup struct {
	Title   string      `json:"title,omitempty"`
	Columns int         `json:"columns,omitempty"`
	Items   []BentoPill `json:"items,omitempty"`
}

// BentoPill is one clickable pill sub-card inside BentoPillGroup.
type BentoPill struct {
	Flag       string   `json:"flag,omitempty"`
	Value      string   `json:"value"`
	Masked     string   `json:"masked,omitempty"`
	Sub        string   `json:"sub,omitempty"`
	Badge      string   `json:"badge,omitempty"`
	BadgeColor string   `json:"badgeColor,omitempty"`
	Mono       bool     `json:"mono,omitempty"`
	Copy       string   `json:"copy,omitempty"`
	URL        string   `json:"url,omitempty"`
	Details    []KVItem `json:"details,omitempty"`
}

// BentoCell is a general-purpose nested sub-card in a Bento grid.
type BentoCell struct {
	Title  string   `json:"title,omitempty"`
	Value  string   `json:"value,omitempty"`
	Sub    string   `json:"sub,omitempty"`
	Icon   string   `json:"icon,omitempty"`
	Flag   string   `json:"flag,omitempty"`
	Badge  string   `json:"badge,omitempty"`
	Status string   `json:"status,omitempty"`
	Color  string   `json:"color,omitempty"`
	Span   int      `json:"span,omitempty"`
	Dots   []int    `json:"dots,omitempty"`
	KV     []KVItem `json:"kv,omitempty"`
}

// BarStyle styles a progressBar / progress component.
type BarStyle struct {
	// Style is "solid" (default), "gradient", "neon" (glowing neon tube),
	// "segmented" (discrete blocks), or "matrix" (glowing LED dot matrix).
	Style string `json:"style,omitempty"`
	// Height is the bar thickness in DIPs (4 to 24 for bars; 28 to 120 for matrix, 0 is auto).
	Height float32 `json:"height,omitempty"`
	// Track is "neutral" (default), "tint", "dark", or "none".
	Track string `json:"track,omitempty"`
	// Glow is neon/LED halo intensity (0.2 to 1.5; 0 defaults to 1.0 for neon/matrix).
	Glow float32 `json:"glow,omitempty"`
	// Segments is block count when Style == "segmented" (4 to 30; 0 is 10).
	Segments int `json:"segments,omitempty"`
	// Rows is the LED dot row count when Style == "matrix" (3 to 10; 0 is 7).
	Rows int `json:"rows,omitempty"`
	// Cols is the LED dot column count when Style == "matrix" (12 to 80; 0 fits width).
	Cols int `json:"cols,omitempty"`
	// Total is the optional overall capacity for stacked matrix mode.
	Total float64 `json:"total,omitempty"`
	// Stacked controls whether Style == "matrix" merges items into one multi-segment
	// LED matrix strip (nil = auto, true = merged, false = one matrix bar per item).
	Stacked *bool `json:"stacked,omitempty"`
	// Animate enables subtle LED breathing and flowing shimmer for Style == "matrix"
	// (nil defaults to true).
	Animate *bool `json:"animate,omitempty"`
	// HideLabel hides the "value / max · pct%" readout on the right.
	HideLabel bool `json:"hideLabel,omitempty"`
}

// ProgressBar is a standalone progress bar component directly on Result (no
// Chart object needed). It unmarshals from either an array of Progress items
// or an object {items, style, height, track, glow, segments, hideLabel}.
type ProgressBar struct {
	Items []Progress `json:"items"`
	BarStyle
}

// UnmarshalJSON accepts ProgressBar as an array of items or as a styled object.
func (p *ProgressBar) UnmarshalJSON(b []byte) error {
	trimmed := strings.TrimSpace(string(b))
	if strings.HasPrefix(trimmed, "[") {
		var items []Progress
		if err := json.Unmarshal(b, &items); err != nil {
			return err
		}
		*p = ProgressBar{Items: items}
		return nil
	}
	type raw struct {
		Items    []Progress `json:"items"`
		Progress []Progress `json:"progress"`
		BarStyle
	}
	var r raw
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	items := r.Items
	if len(items) == 0 && len(r.Progress) > 0 {
		items = r.Progress
	}
	*p = ProgressBar{Items: items, BarStyle: r.BarStyle}
	return nil
}

// Slice is one segment of a "donut" component.
type Slice struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Color string  `json:"color,omitempty"`
	Unit  string  `json:"unit,omitempty"`
}

// DonutStyle styles a "donut" component.
type DonutStyle struct {
	Center    string  `json:"center,omitempty"`    // text in the hole; empty shows total
	Sub       string  `json:"sub,omitempty"`       // caption under center
	Thickness float32 `json:"thickness,omitempty"` // 0.12 to 0.45; 0 is 0.26
	Neon      bool    `json:"neon,omitempty"`
	Style     string  `json:"style,omitempty"` // "" (classic) | "matrix" (LED dot-matrix ring)
}

// CheckItem is one row of a "checklist" component.
type CheckItem struct {
	Label string `json:"label"`
	Done  bool   `json:"done"`
	Note  string `json:"note,omitempty"`
	Badge string `json:"badge,omitempty"`
	Color string `json:"color,omitempty"`
}

// KVItem is one row of a "kv" key-value status component.
type KVItem struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Sub    string `json:"sub,omitempty"`
	Badge  string `json:"badge,omitempty"`
	Status string `json:"status,omitempty"` // ok | warn | err | info
	Color  string `json:"color,omitempty"`
}

// CountdownItem is one target date of a "countdown" component.
type CountdownItem struct {
	Label  string `json:"label"`
	Target string `json:"target"`          // YYYY-MM-DD, YYYY-MM-DD HH:MM, RFC3339, or unix timestamp
	Start  string `json:"start,omitempty"` // optional start date for progress bar
	Note   string `json:"note,omitempty"`
	Color  string `json:"color,omitempty"`
	To     string `json:"to,omitempty"`
	Style  string `json:"style,omitempty"` // solid | gradient | neon
}

// ClockZone is one timezone in a "clock" component. Offset is hours east of
// UTC (e.g. 8, -5, 5.5); values with |offset| > 14 are treated as minutes.
type ClockZone struct {
	Label  string  `json:"label"`
	Offset float64 `json:"offset"`
}

// ClockSpec configures a "clock" component.
type ClockSpec struct {
	Style   string      `json:"style,omitempty"` // analog (default) | digital | matrix | world
	Zones   []ClockZone `json:"zones,omitempty"`
	Seconds bool        `json:"seconds,omitempty"`
	Color   string      `json:"color,omitempty"`
	Neon    bool        `json:"neon,omitempty"`
}

// ContribCell is one day in a "contrib" activity grid component.
type ContribCell struct {
	Date  string  `json:"date"`
	Value float64 `json:"value"`
	Label string  `json:"label,omitempty"`
}

// ContribSpec configures a "contrib" component.
type ContribSpec struct {
	Cells []ContribCell `json:"cells"`
	Color string        `json:"color,omitempty"`
	Max   float64       `json:"max,omitempty"`
	Weeks int           `json:"weeks,omitempty"`
	Unit  string        `json:"unit,omitempty"`
	Neon  bool          `json:"neon,omitempty"`
	Style string        `json:"style,omitempty"` // "" (classic squares) | "matrix" (LED dot matrix)
}

// Progress is one bar of a "progress" chart: Value out of Max. A Max of 0
// means Value is already a percentage (0 to 100).
type Progress struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Max   float64 `json:"max"`
	Unit  string  `json:"unit,omitempty"`
	// Color is the item's color, as "#RRGGBB"; without one a bar is the accent
	// and the rings of a concentric chart each have a color of their own. To is
	// the color a ring's arc deepens to, when it is not the color's own.
	Color string `json:"color,omitempty"`
	To    string `json:"to,omitempty"`
}

// Raw is how far the bar is full in percent, past 100 when the value is over
// its maximum (a ring goes round a second time).
func (p Progress) Raw() float64 {
	pct := p.Value
	if p.Max > 0 {
		pct = p.Value / p.Max * 100
	}
	if pct != pct || pct < 0 {
		return 0
	}
	return pct
}

// Percent is how far the bar is full, 0 to 100.
func (p Progress) Percent() float64 { return min(p.Raw(), 100) }

// RingStyle styles a progressRing chart. Every field is optional, and the zero
// value is the default ring.
type RingStyle struct {
	// Layout is "row" (the default: a ring for each item, side by side) or
	// "concentric" (the items' rings nested in one, as Apple's activity rings,
	// with a legend beside it).
	Layout string `json:"layout,omitempty"`
	// Size is a ring's size in DIPs, 32 to 200; 0 fits it to the card.
	Size float32 `json:"size,omitempty"`
	// Thickness is the width of the stroke as a share of the size, 0.04 to 0.3;
	// 0 is 0.11 (0.085 for concentric rings).
	Thickness float32 `json:"thickness,omitempty"`
	// Cap is how the arc ends: "round" (the default) or "butt".
	Cap string `json:"cap,omitempty"`
	// Track is the ring under the arc: "tint" (the default: the arc's color,
	// faint), "neutral" (gray) or "none".
	Track string `json:"track,omitempty"`
	// Fill is "gradient" (the default: the color deepens along the arc),
	// "solid", or "matrix" (glowing LED dot-matrix ring).
	Fill string `json:"fill,omitempty"`
	// Center is what a ring shows inside: "percent" (the default), "value" or
	// "none".
	Center string `json:"center,omitempty"`
	// Align is where the rings sit across the card: "left" (the default),
	// "center", "right", or spread over it: "between", "around", "evenly".
	Align string `json:"align,omitempty"`
	// VAlign is where they sit down the card: "top" (the default, right under the
	// summary), "middle" or "bottom".
	VAlign string `json:"valign,omitempty"`
}

// ChartTypes are the chart and component types a result can draw, derived from
// WidgetSpecs so the engine, UI registry, and MCP guide stay in sync.
var ChartTypes = widgetTypeNames()

// UnknownChartType reports whether t is not one of ChartTypes (an empty type is
// a line).
func UnknownChartType(t string) bool {
	if t == "" {
		return false
	}
	_, ok := WidgetSpecOf(t)
	return !ok
}

// Layout says how a result's card is laid out: whether its legend shows, where
// its summary and its stats go. The zero value is the default layout.
type Layout struct {
	// Legend is "show" (the default) or "hide": the names of a chart's series.
	Legend string `json:"legend,omitempty"`
	// Summary is "top" (the default) or "hide": the line under the headline.
	Summary string `json:"summary,omitempty"`
	// Stats is "bottom" (the default) or "hide": the figures, in their own
	// block under the chart.
	Stats string `json:"stats,omitempty"`
	// Actions is "hover" (the default) or "always": a card's buttons (refresh,
	// size) show while the pointer is on the card, or all the time. A touch
	// screen has no pointer and always shows them.
	Actions string `json:"actions,omitempty"`
	// Title is "hide" (the default) or "show": a card's name and icon in a row
	// of their own at its top. Hidden, the card is its content, and its name
	// shows over it, with the buttons, while the pointer is on the card.
	Title string `json:"title,omitempty"`
	// Image is "cover" (the default) or "inline": a card with pictures is the
	// picture, edge to edge, with its text and buttons over it while the pointer
	// is on it; or the picture is a block among the card's other contents.
	Image string `json:"image,omitempty"`
}

// Image is a picture of a result: its address, a caption under it, and, once the
// host has kept a copy, the copy's path on this computer.
type Image struct {
	URL     string `json:"url"`
	Caption string `json:"caption,omitempty"`
	// Source is the page the picture comes from, opened by its action.
	Source string `json:"source,omitempty"`
	Local  string `json:"local,omitempty"`
}

// Stat is one figure of a result: its name, its value, and a line under it.
type Stat struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Sub   string `json:"sub"`
}

// MenuBar is a card's item in the menu bar: its title and tooltip, and how the
// item behaves.
type MenuBar struct {
	Title   string `json:"title"`
	Tooltip string `json:"tooltip"`
	// Mono draws the title in a monospaced font (macOS), so that a changing
	// number keeps the item's width.
	Mono bool `json:"mono,omitempty"`
	// Click is what a click on the item does: "menu" (the default) opens its
	// menu, "open" opens the card in the window, and the menu is on a right
	// click.
	Click string `json:"click,omitempty"`
	// Icon picks a tray icon glyph: "bars" (default), "dot", "ring", "up",
	// "down", or "pulse".
	Icon string `json:"icon,omitempty"`
	// Color sets an optional "#RRGGBB" tint for the tray icon (turning off
	// monochrome template mode so status colors show in the bar).
	Color string `json:"color,omitempty"`
	// RefreshSec requests a custom menu-bar refresh cadence in seconds (min 15;
	// 0 defaults to 60s).
	RefreshSec int `json:"refreshSec,omitempty"`
}

// MenuEntry is one item of a card's menu in the menu bar. What it does, first
// that applies: Items makes it a submenu; Action runs one of MenuActions; URL
// opens a web address; Copy puts text on the clipboard. With none of them it is
// a line of text.
type MenuEntry struct {
	Label     string `json:"label"`
	URL       string `json:"url,omitempty"`
	Separator bool   `json:"separator,omitempty"`
	// Copy is text to put on the clipboard when the item is chosen.
	Copy string `json:"copy,omitempty"`
	// Action is "refresh" (the card runs again), "open" (its full view, in the
	// window) or "window" (the window comes forward).
	Action string `json:"action,omitempty"`
	// Sub is a second, smaller line under the label.
	Sub string `json:"sub,omitempty"`
	// Badge is short text at the end of the item, such as a count.
	Badge    string `json:"badge,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
	// Checked puts a check mark before the label.
	Checked bool `json:"checked,omitempty"`
	// Items are a submenu's items; one level of submenus is shown.
	Items []MenuEntry `json:"items,omitempty"`
}

// Result is what render returns, plus the lines the script logged.
type Result struct {
	Title string   `json:"title"`
	Icon  string   `json:"icon"`
	Big   string   `json:"big"`
	Sub   string   `json:"sub"`
	Lines []string `json:"lines"`
	Chart *Chart   `json:"chart"`
	// ProgressBar is a standalone progress bar component (including neon bars),
	// rendered without a chart container or chart detail controls.
	ProgressBar *ProgressBar `json:"progressBar,omitempty"`
	// Stats are short figures a result shows as a row under its headline, and
	// as tiles in the full view: a live weather's feels-like, humidity, wind.
	Stats []Stat `json:"stats"`
	// Images are pictures a result shows as a carousel (inspiration, wallpapers).
	// The host keeps a copy of each, as Local, and the history of the script's
	// pictures; the carousel shows that history.
	Images []Image `json:"images"`
	// Layout says how the card of this result is laid out (see Layout).
	Layout *Layout `json:"layout,omitempty"`
	// MenuBar is the card's icon in the menu bar, when the user has put it
	// there: a short title beside the icon, and a tooltip. Without it, the
	// title is the headline and the tooltip the headline with the summary.
	MenuBar *MenuBar `json:"menubar,omitempty"`
	// Menu lists the script's own items in the menu bar menu, before the app's:
	// an item with a URL opens it (web addresses only), one without is a line
	// of text, and {separator: true} is a rule.
	Menu []MenuEntry `json:"menu,omitempty"`
	Logs []string    `json:"-"`
	// DataAt and Stale are filled by the host, not the script: when the oldest
	// data behind the result was fetched (Unix milliseconds), and whether any
	// of it came from the cache because a request failed or the app is offline.
	DataAt int64 `json:"dataAt"`
	Stale  bool  `json:"stale"`
}

// FeedBuckets is the samples of one data source grouped by hour or by day.
// Series[k] is the k-th number of the source, one value per bucket; Labels
// name the buckets; Counts say how many samples each bucket holds.
type FeedBuckets struct {
	Labels []string `json:"labels"`
	Series []Series `json:"series"`
	Counts []int    `json:"counts"`
}

// FeedData is what a script gets about one data source. Error says why the
// source has no data, for example when its last request failed.
type FeedData struct {
	Name     string      `json:"name"`
	Names    []string    `json:"names"`
	Latest   []float64   `json:"latest"`
	LatestAt int64       `json:"latestAt"` // Unix milliseconds; 0 when there is no sample
	Hour     FeedBuckets `json:"hour"`
	Day      FeedBuckets `json:"day"`
	Error    string      `json:"error,omitempty"`
	// Stale is true when the latest reading is old or the last try failed,
	// so a script can say that what it shows is not current.
	Stale bool `json:"stale"`
	// Fails is the number of failed tries in a row.
	Fails int `json:"fails"`
}

// CommandRun holds one execution record exposed to scripts via ctx.commands[id].
type CommandRun struct {
	ID         string            `json:"id"`
	T          int64             `json:"t"`
	Trigger    string            `json:"trigger"`
	Params     map[string]string `json:"params,omitempty"`
	OK         bool              `json:"ok"`
	ExitCode   int               `json:"exitCode"`
	DurationMs int64             `json:"durationMs"`
	Extracted  map[string]any    `json:"extracted,omitempty"`
	JSON       any               `json:"json,omitempty"`
	Stdout     string            `json:"stdout,omitempty"`
	Stderr     string            `json:"stderr,omitempty"`
	Error      string            `json:"error,omitempty"`
}

// CommandData holds the latest run and history for one command named in meta().commands.
type CommandData struct {
	ID       string                  `json:"id"`
	Name     string                  `json:"name"`
	Display  *cmdrun.DisplayStyle    `json:"display,omitempty"`
	Resolved *cmdrun.ResolvedCommand `json:"resolved,omitempty"`
	Latest   *CommandRun             `json:"latest,omitempty"`
	History  []CommandRun            `json:"history"`
	OK       bool                    `json:"ok"`
	Error    string                  `json:"error,omitempty"`
}

// Input is what a run gets besides the script.
type Input struct {
	Now time.Time
	// Lang is the language of the user: "zh" or "en".
	Lang string
	// Theme is the active UI polarity: "dark" or "light".
	Theme string
	// OS is the system the app runs on ("macos", "windows", "linux", "android",
	// "ios") and Kind its group ("desktop" or "mobile"): ctx.platform. A script
	// asks the host for what is system specific, and rarely needs these.
	OS, Kind string
	// Granted are the sensitive features the script may use: ctx.granted[need]
	// is true for each.
	Granted  []string
	TZOffset int // seconds east of UTC, so scripts can group values by local day
	// View holds the value of each adjusted dimension, by its key.
	View map[string]float64
	// Usage holds the report of each provider the script names in
	// meta().usage, or {"error": "..."} for one that could not be read.
	Usage map[string]any
	// Feeds holds the data of each source the script names in meta().feeds.
	Feeds map[string]FeedData
	// Commands holds the latest run and history of each command named in meta().commands.
	Commands map[string]CommandData
	// Credentials holds redacted credential readiness metadata (ctx.credentials).
	Credentials map[string]cred.Status
	Metrics     map[string][]Point
	Settings    map[string]any
	Sources     map[string]json.RawMessage
	// SourceInfo says where each source's data came from (ctx.sourceInfo), and
	// History holds the past responses of the sources the script asked for
	// (ctx.history).
	SourceInfo map[string]SourceInfo
	History    map[string][]HistoryPoint
	// Vars are the chosen value of each variable (ctx.vars).
	Vars map[string]string
	// NetProbe holds the host-side network probe report when meta().netprobe is configured.
	NetProbe any
}

// Script is a compiled script. Runs share the compiled module but each gets a
// runtime of its own, so runs can happen on any goroutine.
type Script struct {
	Meta Meta
	mod  *moejs.Module
}

// Compile compiles source and reads its meta(). Errors carry the syntax or
// runtime error from moejs.
func Compile(name, source string) (*Script, error) { return CompileLang(name, source, "") }

// CompileLang is Compile for a user who reads lang ("zh", "en"): meta() is
// called with {lang}, so a script can name its dimensions in that language.
func CompileLang(name, source, lang string) (*Script, error) {
	mod, err := moejs.Compile(name, source)
	if err != nil {
		return nil, err
	}
	s := &Script{mod: mod}
	var meta Meta
	arg, _ := json.Marshal(map[string]string{"lang": lang})
	if _, err := s.invoke(context.Background(), "meta", string(arg), &meta); err != nil {
		return nil, fmt.Errorf("meta(): %w", err)
	}
	s.Meta = meta
	return s, nil
}

// HasRender reports whether the script exports render.
func (s *Script) HasRender() bool {
	_, err := s.mod.Hook("render")
	return err == nil
}

func themeOr(s string) string {
	if s == "dark" {
		return "dark"
	}
	return "light"
}

// Run calls render with in and returns its result.
func (s *Script) Run(ctx context.Context, in Input) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, RunTimeout)
	defer cancel()
	themeName := themeOr(in.Theme)
	arg, err := json.Marshal(map[string]any{
		"now":         in.Now.UnixMilli(),
		"lang":        langOr(in.Lang),
		"theme":       themeName,
		"dark":        themeName == "dark",
		"platform":    map[string]string{"os": in.OS, "kind": in.Kind},
		"granted":     grantedSet(in.Granted),
		"tzOffset":    in.TZOffset,
		"view":        nonNilView(in.View),
		"feeds":       nonNilFeeds(in.Feeds),
		"commands":    nonNilCommands(in.Commands),
		"credentials": nonNilCredentials(in.Credentials),
		"usage":       nonNilUsage(in.Usage),
		"metrics":     nonNilMetrics(in.Metrics),
		"settings":    nonNilSettings(in.Settings),
		"sources":     nonNilSources(in.Sources),
		"sourceInfo":  nonNilInfo(in.SourceInfo),
		"history":     nonNilHistory(in.History),
		"vars":        nonNilVars(in.Vars),
		"netprobe":    nonNilNetProbe(in.NetProbe),
	})
	if err != nil {
		return Result{}, err
	}
	var res Result
	logs, err := s.invoke(ctx, "render", string(arg), &res)
	res.Logs = logs
	if err != nil {
		return res, err
	}
	finishChart(res.Chart, in.TZOffset)
	finishCalendar(&res, in)
	return res, nil
}

// invoke calls the exported function fn with arg (JSON text) in a fresh
// runtime and decodes its return value into target. Lines logged with the
// host's log(...) come back as well.
func (s *Script) invoke(ctx context.Context, fn, arg string, target any) ([]string, error) {
	rt := moejs.NewRuntime(moejs.Options{MemoryLimit: MemoryLimit})
	var logs []string
	logFn := func(r *moejs.Realm, _ moejs.Value, args []moejs.Value) (moejs.Value, error) {
		parts := make([]string, 0, len(args))
		for i := range args {
			str, err := r.ToString(moejs.Arg(args, i))
			if err != nil {
				return moejs.Undefined(), err
			}
			parts = append(parts, str.GoString())
		}
		logs = append(logs, strings.Join(parts, " "))
		return moejs.Undefined(), nil
	}
	if err := rt.SetGlobal("log", rt.Function("log", 0, logFn)); err != nil {
		return nil, err
	}
	if err := rt.Load(s.mod); err != nil {
		return logs, err
	}
	hook, err := s.mod.Hook(fn)
	if err != nil {
		return logs, err
	}
	argv, err := rt.ParseJSONString(arg)
	if err != nil {
		return logs, err
	}

	// Interrupt stops the script when ctx ends, even inside an endless loop.
	stop := context.AfterFunc(ctx, func() {
		rt.Interrupt(context.Cause(ctx))
	})
	defer stop()

	res, err := rt.Call(hook, argv)
	if err != nil {
		return logs, err
	}
	if err := rt.Unmarshal(res, target); err != nil {
		return logs, fmt.Errorf("%s() returned an unexpected value: %w", fn, err)
	}
	return logs, nil
}

func nonNilMetrics(m map[string][]Point) map[string][]Point {
	if m == nil {
		return map[string][]Point{}
	}
	return m
}

func nonNilSettings(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

func nonNilUsage(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	return m
}

func nonNilFeeds(m map[string]FeedData) map[string]FeedData {
	if m == nil {
		return map[string]FeedData{}
	}
	return m
}

func nonNilCommands(m map[string]CommandData) map[string]CommandData {
	if m == nil {
		return map[string]CommandData{}
	}
	return m
}

func nonNilCredentials(m map[string]cred.Status) map[string]cred.Status {
	if m == nil {
		return map[string]cred.Status{}
	}
	return m
}

func nonNilView(m map[string]float64) map[string]float64 {
	if m == nil {
		return map[string]float64{}
	}
	return m
}

func nonNilSources(m map[string]json.RawMessage) map[string]json.RawMessage {
	if m == nil {
		return map[string]json.RawMessage{}
	}
	return m
}

func langOr(l string) string {
	if l == "" {
		return "zh"
	}
	return l
}

// grantedSet is the granted features as the object ctx.granted: a feature that
// was not granted is absent, so ctx.granted.files is undefined, not an error.
func grantedSet(list []string) map[string]bool {
	out := make(map[string]bool, len(list))
	for _, n := range list {
		out[n] = true
	}
	return out
}

func nonNilInfo(m map[string]SourceInfo) map[string]SourceInfo {
	if m == nil {
		return map[string]SourceInfo{}
	}
	return m
}

func nonNilHistory(m map[string][]HistoryPoint) map[string][]HistoryPoint {
	if m == nil {
		return map[string][]HistoryPoint{}
	}
	return m
}

// nonNilVars is the chosen variables, never nil, so that ctx.vars is an object.
func nonNilVars(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	return m
}

func nonNilNetProbe(v any) any {
	if v == nil {
		return map[string]any{}
	}
	return v
}
