package main

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/feed"
	"scriptboard/src/internal/store"
)

// app is the state of the window. Its view builds the pages from it on the
// main thread. Scripts run off the main thread and post their results back
// with post, so the view never waits for the network.
type app struct {
	win     *mygo.Window // nil in tests, where runs happen inline
	router  *ui.Router
	store   *store.Store
	dataDir string // the folder of data.json; plugins and icons live under it
	// pluginNote says what could not be loaded from the plugin folder.
	pluginNote string
	client     *http.Client
	now        func() time.Time

	// runs holds the latest run of each card and of the editor's draft. A run
	// is keyed by script ID, then by the values of its dimensions, as
	// "id|range=30|period=1"; the draft uses draftKey. Main thread only.
	runs   map[string]*runState
	metas  map[string]metaEntry // compiled meta of each script, by ID and source
	dims   map[string][]int     // selected option of each dimension, by key
	styles map[string]string    // chart style chosen for each script, by ID
	sels   map[string]*int      // chart selection, by chart key; -1 is none

	width    float32 // width of the window, from the last frame
	height   float32 // height of the window, from the last frame
	big      bool    // 大图: the full-screen chart takes the whole window
	compact  bool    // the phone layout is showing
	cardSize int     // 0 small, 1 medium, 2 large

	// Editor.
	editorID  string
	draft     store.Script
	metricIdx int     // 0 weight, 1 gold
	entry     float64 // the value typed in the editor
	status    string  // one line of feedback: saved, recorded, errors

	// Quick entry on the home cards: the value typed for each input, and the
	// last line of feedback.
	quick      map[string]*float64
	refreshing map[string]bool // scripts whose refresh is running
	quickNote  string

	// Import and export of scripts, shown under the draft: panel is "",
	// "import" or "export".
	panel        string
	transferText string
	transferErr  string

	// Settings. set is the working copy; it is saved on every change.
	set     store.Settings
	unitIdx int
	langIdx int // the language row of the settings: 0 auto, 1 中文, 2 English
	skinIdx int // the appearance row of the settings: 0 default, 1 Mac OS 9
	// frameless is true when the window was opened without its system frame,
	// so the Mac OS 9 menu bar is the title bar and can close the window.
	frameless bool

	// AI usage: the reports kept for a moment, the forms in the settings, and
	// the folder of Claude Code logs (empty is the default one; tests set it).
	usage      usageCache
	usageForms map[string]*usageForm
	usageDir   string

	// The MCP server and what its settings rows show.
	mcp       mcpRunner
	mcpPort   string
	mcpPublic string
	mcpNote   string

	// Data sources: the samples of each source, the runner that takes them and
	// the form that adds a source.
	samples  *feed.Store
	runner   *feed.Runner
	feedForm feedForm

	// Community: the list, the login form and the upload description.
	comm       commState
	authUser   string
	authPass   string
	uploadDesc string
}

type runState struct {
	loading bool
	res     engine.Result
	err     error
}

type metaEntry struct {
	meta engine.Meta
	err  error
}

const draftKey = "draft"

// compactWidth is the window width below which the pages use the phone
// layout: a bottom bar in place of the sidebar, and cards stacked.
const compactWidth = 640

// sidebarWidth is the width of the sidebar on a wide window.
const sidebarWidth = 232

// newApp opens the data in dir. offline makes every run use sample data, with
// no network requests.
func newApp(dir string, offline bool) (*app, error) {
	st, err := store.Open(dir)
	if err != nil {
		return nil, err
	}
	a := &app{
		router:     ui.NewRouter("/"),
		store:      st,
		client:     &http.Client{Timeout: 10 * time.Second},
		now:        time.Now,
		runs:       map[string]*runState{},
		metas:      map[string]metaEntry{},
		dims:       map[string][]int{},
		styles:     map[string]string{},
		quick:      map[string]*float64{},
		refreshing: map[string]bool{},
		usageForms: map[string]*usageForm{},
		sels:       map[string]*int{},
		cardSize:   1, // medium cards to start with
	}
	a.dataDir = filepath.Dir(st.Path())
	a.samples = feed.NewStore(filepath.Join(a.dataDir, "feeds"))
	a.runner = feed.NewRunner(feed.Mux{
		HTTP:      feed.HTTPFetcher{Client: a.client},
		AllowExec: func() bool { return a.store.Settings().AllowExec },
	}, a.samples)
	if err := a.seed(); err != nil {
		return nil, err
	}
	if err := a.ensureBuiltins(); err != nil {
		return nil, err
	}
	a.syncPlugins()
	a.set = st.Settings()
	if offline {
		a.set.Offline = true
		if err := st.SetSettings(a.set); err != nil {
			return nil, err
		}
	}
	if a.set.WeightUnit == "斤" {
		a.unitIdx = 1
	}
	switch a.set.Lang {
	case langZH:
		a.langIdx = 1
	case langEN:
		a.langIdx = 2
	}
	setLang(a.set.Lang)
	setSkin(a.set.Skin)
	a.skinIdx = 0
	if curSkin() == skinMac9 {
		a.skinIdx = 1
	}
	return a, nil
}

// seed writes the built-in scripts and sample values into an empty store.
func (a *app) seed() error {
	var scripts []store.Script
	for _, b := range engine.Builtins() {
		s, err := engine.Compile(b.ID+".js", b.Source)
		if err != nil {
			return fmt.Errorf("builtin %s: %w", b.ID, err)
		}
		// Scripts that need a key start off the home page.
		home := b.ID == "ai-claude" || !strings.HasPrefix(b.ID, "ai-")
		scripts = append(scripts, store.Script{
			ID: b.ID, Name: s.Meta.Name, Source: b.Source, Home: home,
		})
	}
	now := time.Now()
	var weight, gold []engine.Point
	for i := 0; i < 21; i++ {
		t := now.AddDate(0, 0, -2*(20-i)).UnixMilli()
		weight = append(weight, engine.Point{T: t, V: 66.4 - 0.07*float64(i) + 0.3*math.Sin(float64(i))})
	}
	// Several prices a day for 30 days, so that the K-lines have a range.
	base := 628.0
	for d := 0; d < 30; d++ {
		day := now.AddDate(0, 0, -(29 - d))
		// Five prices a day with a swing in the middle, so that candles have
		// wicks and some close lower than they open.
		closeOff := 3 * math.Cos(float64(d)*1.7)
		offs := []float64{-2, 4, -3.5, 1.5, closeOff}
		for k, hour := range []int{9, 11, 13, 16, 20} {
			t := time.Date(day.Year(), day.Month(), day.Day(), hour, 0, 0, 0, day.Location())
			v := base + 5*math.Sin(float64(d)/3) + offs[k] + 0.3*float64(d)
			gold = append(gold, engine.Point{T: t.UnixMilli(), V: v})
		}
		base += 0.2
	}
	return a.store.Seed(scripts, map[string][]engine.Point{"weight": weight, "gold": gold})
}

// attach connects the app to its window, so that runs can post results, and
// starts watching the plugin folder, so that a change shows within a second.
func (a *app) attach(w *mygo.Window) {
	a.win = w
	go a.watchPlugins()
	go a.feedLoop()
	a.applyMCP()
}

// post runs fn on the main thread.
func (a *app) post(fn func()) {
	if a.win == nil {
		fn()
		return
	}
	a.win.Update(fn)
}

// metaOf returns the meta of sc, compiled once for each source.
func (a *app) metaOf(sc store.Script) (engine.Meta, error) {
	k := curLang() + "\x00" + sc.ID + "\x00" + sc.Source
	if e, ok := a.metas[k]; ok {
		return e.meta, e.err
	}
	var e metaEntry
	if s, err := engine.CompileLang(sc.Name, sc.Source, curLang()); err != nil {
		e.err = err
	} else {
		e.meta = s.Meta
	}
	a.metas[k] = e
	return e.meta, e.err
}

// viewOf returns the selected option index of each dimension under key, and
// the value of each selected option.
func (a *app) viewOf(key string, dims []engine.Dim) ([]int, map[string]float64) {
	idx := a.dims[key]
	if len(idx) != len(dims) {
		idx = make([]int, len(dims))
		a.dims[key] = idx
	}
	vals := make(map[string]float64, len(dims))
	for i, d := range dims {
		if len(d.Options) == 0 {
			continue
		}
		if idx[i] >= len(d.Options) {
			idx[i] = 0
		}
		vals[d.Key] = d.Options[idx[i]].Value
	}
	return idx, vals
}

// runKey names the run of script id with the values of its dimensions.
func runKey(id string, dims []engine.Dim, vals map[string]float64) string {
	parts := []string{id}
	for _, d := range dims {
		parts = append(parts, fmt.Sprintf("%s=%g", d.Key, vals[d.Key]))
	}
	return strings.Join(parts, "|")
}

// selOf returns the selected point of the chart under key, -1 for none.
func (a *app) selOf(key string) *int {
	if s, ok := a.sels[key]; ok {
		return s
	}
	v := -1
	a.sels[key] = &v
	return &v
}

// ensure returns the run under key for sc, starting one if there is none yet.
func (a *app) ensure(key string, sc store.Script, vals map[string]float64) *runState {
	if st, ok := a.runs[key]; ok {
		return st
	}
	st := &runState{loading: true}
	a.runs[key] = st
	run := func() {
		res, err := a.execute(sc, vals)
		a.post(func() {
			if a.runs[key] != st {
				return // superseded by a newer run
			}
			st.loading = false
			st.res = res
			st.err = err
		})
	}
	if a.win == nil {
		run()
	} else {
		go run()
	}
	return st
}

// invalidate drops every run of key, and of the dimensions under it, so that
// they start again on the next frame.
func (a *app) invalidate(key string) {
	for k := range a.runs {
		if k == key || strings.HasPrefix(k, key+"|") {
			delete(a.runs, k)
		}
	}
}

// invalidateAll drops every run.
func (a *app) invalidateAll() { a.runs = map[string]*runState{} }

// execute compiles sc, fetches its sources and renders it with the values
// of its dimensions. It does not touch the UI, so it may run on any goroutine.
func (a *app) execute(sc store.Script, vals map[string]float64) (engine.Result, error) {
	s, err := engine.CompileLang(sc.Name, sc.Source, curLang())
	if err != nil {
		return engine.Result{}, err
	}
	now := a.now()
	_, tz := now.Zone()
	set := a.store.Settings()
	sources := engine.Offline(s.Meta.Sources)
	if !set.Offline {
		sources = engine.Fetch(context.Background(), a.client, s.Meta.Sources, now)
	}
	return s.Run(context.Background(), engine.Input{
		Now:      now,
		Lang:     curLang(),
		TZOffset: tz,
		View:     vals,
		Metrics:  a.store.Metrics(),
		Feeds:    a.feedInput(s.Meta.Feeds, now),
		Usage:    a.usageInput(s.Meta.Usage, now),
		Settings: map[string]any{
			"weightUnit": set.WeightUnit,
			"offline":    set.Offline,
		},
		Sources: sources,
	})
}

// saveSettings writes the working copy and drops every run, since the
// settings can change what scripts show.
func (a *app) saveSettings() {
	if err := a.store.SetSettings(a.set); err != nil {
		a.set = a.store.Settings()
	}
	a.invalidateAll()
}

// fmtTime formats a Unix millisecond time for lists: local month, day, hour
// and minute.
func fmtTime(ms int64) string {
	return time.UnixMilli(ms).Format("01-02 15:04")
}

// homeScripts are the scripts shown as cards, in store order.
func (a *app) homeScripts() []store.Script {
	var out []store.Script
	for _, sc := range a.store.Scripts() {
		if sc.Home {
			out = append(out, sc)
		}
	}
	return out
}

// ensureBuiltins adds the built-in scripts a data file from an earlier version
// does not have yet. Scripts that need a key start off the home page.
func (a *app) ensureBuiltins() error {
	for _, b := range engine.Builtins() {
		if _, ok := a.store.Script(b.ID); ok {
			continue
		}
		s, err := engine.Compile(b.ID+".js", b.Source)
		if err != nil {
			return fmt.Errorf("builtin %s: %w", b.ID, err)
		}
		home := b.ID == "ai-claude" || !strings.HasPrefix(b.ID, "ai-")
		if err := a.store.SaveScript(store.Script{ID: b.ID, Name: s.Meta.Name, Source: b.Source, Home: home}); err != nil {
			return err
		}
	}
	return nil
}
