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
	// Sources maps a name to a URL. {start} and {end} expand to the dates of
	// the last 30 days, as yyyy-mm-dd.
	Sources map[string]string `json:"sources"`
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
	Label string  `json:"label"` // what the quantity is, such as "本月额度"
	Reset string  `json:"reset"` // when it renews, such as "3 天后重置"
}

// Chart is what a result draws. Type "gauge" draws Gauge as a ring of what is
// left; "barh" draws Series[0] as horizontal bars, one per label. Type is "line" (the default), "dot" for
// scattered points, or "candle" for a K-line, which reads Candles instead of
// Series. Labels, one per point, name each point in the detail readout;
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
}

// Result is what render returns, plus the lines the script logged.
type Result struct {
	Title string   `json:"title"`
	Icon  string   `json:"icon"`
	Big   string   `json:"big"`
	Sub   string   `json:"sub"`
	Lines []string `json:"lines"`
	Chart *Chart   `json:"chart"`
	Logs  []string `json:"-"`
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

// Input is what a run gets besides the script.
type Input struct {
	Now time.Time
	// Lang is the language of the user: "zh" or "en".
	Lang     string
	TZOffset int // seconds east of UTC, so scripts can group values by local day
	// View holds the value of each adjusted dimension, by its key.
	View map[string]float64
	// Usage holds the report of each provider the script names in
	// meta().usage, or {"error": "..."} for one that could not be read.
	Usage map[string]any
	// Feeds holds the data of each source the script names in meta().feeds.
	Feeds    map[string]FeedData
	Metrics  map[string][]Point
	Settings map[string]any
	Sources  map[string]json.RawMessage
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

// Run calls render with in and returns its result.
func (s *Script) Run(ctx context.Context, in Input) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, RunTimeout)
	defer cancel()
	arg, err := json.Marshal(map[string]any{
		"now":      in.Now.UnixMilli(),
		"lang":     langOr(in.Lang),
		"tzOffset": in.TZOffset,
		"view":     nonNilView(in.View),
		"feeds":    nonNilFeeds(in.Feeds),
		"usage":    nonNilUsage(in.Usage),
		"metrics":  nonNilMetrics(in.Metrics),
		"settings": nonNilSettings(in.Settings),
		"sources":  nonNilSources(in.Sources),
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
