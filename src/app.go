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

	"scriptboard/src/internal/cache"
	"scriptboard/src/internal/caps"
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
	// cache keeps the responses of the sources, their history and the last
	// result of each card, for offline use and for opening quickly.
	cache *cache.Store
	// forced holds the scripts whose next run must ask the network again.
	forced map[string]bool
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
	editorID string
	// confirmDelete is the script whose delete button was clicked once.
	confirmDelete string
	draft         store.Script
	tablePage     int    // the page of rows the table editor shows
	tableAppend   int    // an import: 0 replaces the rows, 1 joins them
	tableCols     bool   // the phone editor shows the columns to edit
	editTab       int    // the phone editor: 0 the editor, 1 the preview
	status        string // one line of feedback: saved, recorded, errors

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
	// The size guide: the size just chosen, until when it shows, and its SVGs.
	guideSize  int
	guideUntil time.Time
	guideSVG   [4]*ui.SVG
	// cardMenus are the open states of the cards' right-click menus, by script.
	cardMenus map[string]*bool
	themeIdx  int // the light and dark row of the settings: 0 system, 1 light, 2 dark

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
	// old: res is the result saved from an earlier run, shown until the
	// run that is under way gives a new one.
	old bool
	// updating: the run that gives the new result has not finished.
	updating bool
	res      engine.Result
	err      error
}

type metaEntry struct {
	meta engine.Meta
	err  error
}

const draftKey = "draft"

// compactWidth is the window width below which the pages use the phone
// layout: a bottom bar in place of the sidebar, and cards stacked.
const compactWidth = 640

// sidebarWidth is the width of the sidebar on a wide window; railWidth is the
// width of the sidebar folded into its rail of icons (notes' 46).
const (
	sidebarWidth = 232
	railWidth    = 46
)

// sidebarW is the width the sidebar takes now: the full sidebar, or the rail.
func (a *app) sidebarW() float32 {
	if a.set.SidebarCollapsed {
		return railWidth
	}
	return sidebarWidth
}

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
	a.cache = cache.Open(filepath.Join(a.dataDir, "cache"))
	a.forced = map[string]bool{}
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
	if err := a.ensureBuiltinTables(); err != nil {
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
	setThemeMode(a.set.ThemeMode)
	a.themeIdx = 0
	switch a.set.ThemeMode {
	case themeLight:
		a.themeIdx = 1
	case themeDark:
		a.themeIdx = 2
	}
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
			ID: b.ID, Name: s.Meta.Name, Source: b.Source, Home: home, Origin: store.OriginBuiltin,
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
	if sc.IsTable() {
		return tableMeta(sc), nil
	}
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
// A script that reads the network shows its last result at once, then the new
// one when the run is done; if the run fails, the last result stays, marked as
// old.
func (a *app) ensure(key string, sc store.Script, vals map[string]float64) *runState {
	if st, ok := a.runs[key]; ok {
		return st
	}
	st := &runState{loading: true}
	a.runs[key] = st
	// The run reads the table off the main thread, and the editor changes it.
	sc.Table = sc.Table.Clone()
	force := a.forced[sc.ID]
	delete(a.forced, sc.ID)
	meta, _ := a.metaOf(sc)
	if !force {
		if res, ok := a.savedResult(sc, key, meta); ok {
			st.loading, st.old, st.updating, st.res = false, true, true, res
		}
	}
	saved := st.res
	run := func() {
		res, err := a.executeOpt(sc, vals, force)
		if err == nil {
			a.saveResult(sc, key, meta, res)
		}
		a.post(func() {
			if a.runs[key] != st {
				return // superseded by a newer run
			}
			st.loading, st.updating = false, false
			if err != nil && st.old {
				st.res, st.res.Stale = saved, true
				return
			}
			st.old = false
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

// cardMenuOpen is the open state of a card's right-click menu.
func (a *app) cardMenuOpen(id string) *bool {
	if a.cardMenus == nil {
		a.cardMenus = map[string]*bool{}
	}
	if open, ok := a.cardMenus[id]; ok {
		return open
	}
	open := new(bool)
	a.cardMenus[id] = open
	return open
}

// invalidateWhere drops the runs of the scripts that pick reports for, and
// leaves every other card as it is.
func (a *app) invalidateWhere(pick func(sc store.Script, m engine.Meta) bool) {
	for _, sc := range a.store.Scripts() {
		m, err := a.metaOf(sc)
		if err != nil {
			continue
		}
		if pick(sc, m) {
			a.invalidate(sc.ID)
		}
	}
}

// invalidateMetric drops the runs of the scripts that read metric as an input.
func (a *app) invalidateMetric(metric string) {
	a.invalidateWhere(func(_ store.Script, m engine.Meta) bool {
		for _, in := range m.Inputs {
			if in.Metric == metric {
				return true
			}
		}
		return false
	})
	a.invalidate(draftKey)
}

// invalidateAll drops every run.
func (a *app) invalidateAll() { a.runs = map[string]*runState{} }

// execute compiles sc, fetches its sources and renders it with the values
// of its dimensions. It does not touch the UI, so it may run on any goroutine.
func (a *app) execute(sc store.Script, vals map[string]float64) (engine.Result, error) {
	return a.executeOpt(sc, vals, false)
}

// executeOpt is execute; force asks the network for every source, whatever the
// cache holds (the user pressed refresh).
func (a *app) executeOpt(sc store.Script, vals map[string]float64, force bool) (engine.Result, error) {
	if sc.IsTable() {
		return a.tableResult(sc), nil
	}
	// The system first, then the user's permission: a script that cannot run
	// here, or that uses what it was not allowed, does not start.
	if r := a.specOf(sc); !r.SupportsOS(caps.OS()) {
		return engine.Result{}, errUnsupported(r)
	}
	if missing := a.missingGrants(sc); len(missing) > 0 {
		return engine.Result{}, errNeedsGrant(missing)
	}
	s, err := engine.CompileLang(sc.Name, sc.Source, curLang())
	if err != nil {
		return engine.Result{}, err
	}
	now := a.now()
	_, tz := now.Zone()
	set := a.store.Settings()
	opt := engine.FetchOptions{TTL: sourceTTL, Offline: set.Offline, Force: force}
	if a.cache != nil {
		opt.Cache = cacheAdapter{a.cache}
	}
	sources, info := engine.FetchCached(context.Background(), a.client, s.Meta.Sources, now, opt)
	var history map[string][]engine.HistoryPoint
	if names, n := historyWanted(s.Meta); len(names) > 0 && opt.Cache != nil {
		history = engine.HistoryOf(opt.Cache, s.Meta.Sources, names, n, now)
	}
	res, err := s.Run(context.Background(), engine.Input{
		Now:      now,
		Lang:     curLang(),
		TZOffset: tz,
		OS:       caps.OS(),
		Kind:     caps.Platform(),
		Granted:  a.grantedList(sc),
		View:     vals,
		Metrics:  a.store.Metrics(),
		Feeds:    a.feedInput(s.Meta.Feeds, now),
		Usage:    a.usageInput(s.Meta.Usage, now),
		Settings: map[string]any{
			"weightUnit": set.WeightUnit,
			"offline":    set.Offline,
		},
		Sources:    sources,
		SourceInfo: info,
		History:    history,
	})
	if err == nil {
		res.DataAt, res.Stale = dataTimes(info)
	}
	return res, err
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
		// A script this device cannot run has no card on it.
		if sc.Home && a.supported(sc) {
			out = append(out, sc)
		}
	}
	return out
}

// ensureBuiltins adds the built-in scripts a data file from an earlier version
// does not have yet (including goldacc, aqi, hs300, btc), and refreshes gold /
// goldk when they still use the old mock or hand-entered source. Scripts that
// need a key start off the home page.
func (a *app) ensureBuiltins() error {
	for _, b := range engine.Builtins() {
		sc, ok := a.store.Script(b.ID)
		if ok && !refreshBuiltin(b.ID, sc.Source) {
			// A script saved before origins existed learns that it is built in.
			if sc.Origin == "" {
				sc.Origin = store.OriginBuiltin
				if err := a.store.SaveScript(sc); err != nil {
					return err
				}
			}
			continue
		}
		s, err := engine.Compile(b.ID+".js", b.Source)
		if err != nil {
			return fmt.Errorf("builtin %s: %w", b.ID, err)
		}
		home := b.ID == "ai-claude" || !strings.HasPrefix(b.ID, "ai-")
		if ok {
			home = sc.Home
		}
		if err := a.store.SaveScript(store.Script{ID: b.ID, Name: s.Meta.Name, Source: b.Source, Home: home, Origin: store.OriginBuiltin}); err != nil {
			return err
		}
	}
	return nil
}

// refreshBuiltin is true when a stored builtin is an old version of the one
// that ships, and was never edited: the old gold script (hand records, no
// Tmini), the old gold K-line (sample candles), or a script with the chart
// colors of before the palette was refined.
func refreshBuiltin(id, source string) bool {
	switch id {
	case "gold":
		if !strings.Contains(source, "tmini.net/api/gold-price") {
			return true
		}
	case "goldk":
		if strings.Contains(source, "示例数据：还没有金价录入") {
			return true
		}
	}
	for _, b := range engine.Builtins() {
		if b.ID == id {
			old := legacyColors(b.Source)
			return old != b.Source && source == old
		}
	}
	return false
}

// legacyColors writes a built-in script with the pastel chart colors it had
// before 0.1.3, to tell a stored copy that is just the old version from one the
// user changed.
func legacyColors(src string) string {
	for _, c := range [][2]string{
		{"#3B82F6", "#4FC3F7"}, {"#F97316", "#FF8A65"}, {"#10B981", "#81C784"},
		{"#EAB308", "#FFD54F"}, {"#F59E0B", "#FFB74D"}, {"#8B5CF6", "#BA68C8"},
	} {
		src = strings.ReplaceAll(src, c[0], c[1])
	}
	return src
}

// builtinIDs are the ids of the scripts that ship with the app.
func builtinIDs() map[string]bool {
	ids := map[string]bool{}
	for _, b := range engine.Builtins() {
		ids[b.ID] = true
	}
	return ids
}

// originOf is where a script came from. A script saved before origins existed
// is a plugin by its id, built in if it has a built-in's id, and the user's
// otherwise.
func (a *app) originOf(sc store.Script) string {
	switch {
	case isPluginScript(sc.ID):
		return store.OriginPlugin
	case sc.Origin != "":
		return sc.Origin
	case builtinIDs()[sc.ID]:
		return store.OriginBuiltin
	}
	return store.OriginUser
}
