package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/bar"
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
	win *mygo.Window // nil in tests, where runs happen inline
	// bars are the icons of the cards in the menu bar, by script ID, and barSig
	// what each shows, so that an unchanged icon is not drawn again.
	bars   map[string]*mygo.Tray
	barSig map[string]string
	// quitting is set once the app quits, so that the window really closes.
	quitting bool
	// live is the state of the cards whose sources are asked for again (live_app.go).
	live liveState
	// shown is what each run key last showed, kept when the run is dropped to run
	// again, so that its card keeps it meanwhile (see startState).
	shown map[string]engine.Result
	// settingsWin is the settings window, while it is open (settings_window.go).
	settingsWin *mygo.Window
	// openSettingsHook, when non-nil in tests, intercepts opening the separate
	// settings window so tests can assert desktop clicks do not route in-window.
	openSettingsHook func()
	router           *ui.Router
	store            *store.Store
	dataDir          string // the folder of data.json; plugins and icons live under it
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
	runs           map[string]*runState
	metas          map[string]metaEntry         // compiled meta of each script, by ID and source
	dims           map[string][]int             // selected option of each dimension, by key
	styles         map[string]string            // chart style chosen for each script, by ID
	sels           map[string]*int              // chart selection, by chart key; -1 is none
	pinnedSels     map[string]*int              // pinned chart index per full-view key; -1 when unpinned
	hiddenSeries   map[string]map[int]bool      // hidden series indices per script ID for interactive legends
	checkOverrides map[string]map[int]bool      // interactive local checklist toggle overrides per script
	tableViews     map[string]*tableWidgetState // per-view sort/search state for table widgets
	fullModes      map[string]int               // full-view mode per script: 0 visual, 1 table, 2 raw

	width    float32 // width of the window, from the last frame
	height   float32 // height of the window, from the last frame
	big      bool    // 大图: the full-screen chart takes the whole window
	compact  bool    // the phone layout is showing
	cardSize int     // 0 small, 1 medium, 2 large

	// Editor.
	editorID      string
	editorSearch  string
	editorKind    int // 0: all, 1: script, 2: table, 3: .bar
	confirmDelete string
	draft         store.Script
	tablePage     int    // the page of rows the table editor shows
	tableAppend   int    // an import: 0 replaces the rows, 1 joins them
	tableCols     bool   // the phone editor shows the columns to edit
	tableFilter   string // live row filter in the table editor
	tableSortCol  int    // sorted column in the table editor (-1 none)
	tableSortDesc bool   // sort direction in the table editor
	editTab       int    // the phone editor: 0 visual, 1 source, 2 preview (table: 0 table, 1 preview)
	editMode      int    // the desktop script editor: 0 visual config, 1 source
	visual        visualEditor
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
	set         store.Settings
	settingsTab string // active left-sidebar tab key in the settings UI
	unitIdx     int
	langIdx     int // the language row of the settings: 0 auto, 1 中文, 2 English
	skinIdx     int // the appearance row of the settings: 0 default, 1 Mac OS 9
	// cardMenus are the open states of the cards' right-click menus, by script.
	cardMenus map[string]*bool
	themeIdx  int // the light and dark row of the settings: 0 system, 1 light, 2 dark

	// The home page in edit mode: whether it is on, the card being resized, the
	// columns and the width of one column, and the cards in the order shown.
	editing bool
	rz      *resize
	cols    int
	unit    float32
	homeIDs []string
	// confirmReset is the script whose reset button was clicked once.
	confirmReset string
	// aboutOpen is whether the full view shows a script's details.
	aboutOpen bool
	// The newest Android release, as found by the last check (apk_update.go).
	apkRel     *apkRelease
	apkBusy    bool
	apkChecked bool
	apkErr     string
	// The decoded pictures, by their copy's path, and the carousel's place per
	// script (images_ui.go).
	bitmaps     map[string]*ui.Bitmap
	carouselIdx map[string]int

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

	// .bar component runtime, menu bar flyouts, and Component Studio state.
	barInsts        map[string]*bar.Instance
	barSpecs        map[string]bar.Spec
	barTrays        map[string]*mygo.Tray
	barTraySig      map[string]string
	barFlyouts         map[string]*mygo.Window
	barFlyoutActive    map[string]bool
	barFlyoutClosedAt  map[string]time.Time
	barPreviewTheme int
	barSearch       string
	barSelectedID   string
	barImportOpen   bool
	barImportText   string
	barImportErr    string
	barOverlayID    string
	barInputDrafts  map[string]map[string]string
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
		router:          ui.NewRouter("/"),
		store:           st,
		client:          &http.Client{Timeout: 10 * time.Second},
		now:             time.Now,
		runs:            map[string]*runState{},
		metas:           map[string]metaEntry{},
		dims:            map[string][]int{},
		styles:          map[string]string{},
		quick:           map[string]*float64{},
		refreshing:      map[string]bool{},
		usageForms:      map[string]*usageForm{},
		sels:            map[string]*int{},
		cardSize:        1, // medium cards to start with
		barInsts:        map[string]*bar.Instance{},
		barSpecs:        map[string]bar.Spec{},
		barTrays:        map[string]*mygo.Tray{},
		barTraySig:      map[string]string{},
		barFlyouts:        map[string]*mygo.Window{},
		barFlyoutActive:   map[string]bool{},
		barFlyoutClosedAt: map[string]time.Time{},
		barInputDrafts:  map[string]map[string]string{},
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
	if err := a.ensureBuiltinBars(); err != nil {
		return nil, err
	}
	a.syncPlugins()
	a.set = st.Settings()
	for id, style := range a.set.CardStyles {
		a.styles[id] = style
	}
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
		home := defaultHomeBuiltin(b.ID)
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
	go a.menuBarLoop()
	go a.barLoop()
	go a.liveLoop()
	a.applyMCP()
	a.syncMenuBar()
	a.syncBarMenuBar()
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
	if sc.IsBar() {
		return a.barMeta(sc), nil
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

// startState is what a card shows while its run is under way: its last saved
// result, marked as old and updating, when there is one, and a loading state
// when not. A refresh shows the same, so that the card keeps its pictures and
// figures while the new result comes in; the refresh only decides that the
// network is asked (ensure).
func (a *app) startState(sc store.Script, key string, meta engine.Meta) *runState {
	if res, ok := a.shown[key]; ok {
		return &runState{old: true, updating: true, res: res}
	}
	if res, ok := a.savedResult(sc, key, meta); ok {
		return &runState{old: true, updating: true, res: res}
	}
	return &runState{loading: true}
}

// ensure returns the run under key for sc, starting one if there is none yet.
// A script that reads the network shows its last result at once, then the new
// one when the run is done; if the run fails, the last result stays, marked as
// old.
func (a *app) ensure(key string, sc store.Script, vals map[string]float64) *runState {
	if st, ok := a.runs[key]; ok {
		return st
	}
	meta, _ := a.metaOf(sc)
	if meta.Live != nil {
		if a.live.at == nil {
			a.live.at = map[string]time.Time{}
		}
		a.live.at[sc.ID] = a.now()
	}
	st := a.startState(sc, key, meta)
	a.runs[key] = st
	// The run reads the table off the main thread, and the editor changes it.
	sc.Table = sc.Table.Clone()
	force := a.forced[sc.ID]
	delete(a.forced, sc.ID)
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
			if (err != nil || (len(meta.Sources) > 0 && !hasResult(res))) && st.old && hasResult(saved) {
				st.res, st.res.Stale = saved, true
				return
			}
			if !hasResult(res) {
				res.Stale, res.DataAt = false, 0
			}
			st.old = false
			st.res = res
			st.err = err
			a.menuBarRan(sc.ID)
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
	for k, st := range a.runs {
		if k == key || strings.HasPrefix(k, key+"|") {
			a.keepShown(k, st)
			delete(a.runs, k)
		}
	}
}

// keepShown remembers the result a run shows, before the run is dropped to run
// again. The editor's previews are not kept: a preview of one script is not a
// result of another.
func (a *app) keepShown(key string, st *runState) {
	if st == nil || strings.HasPrefix(key, draftKey) || !hasResult(st.res) {
		return
	}
	if a.shown == nil {
		a.shown = map[string]engine.Result{}
	}
	a.shown[key] = st.res
}

// hasResult says whether a result has real content worth keeping on screen or
// on disk, not an empty run or a "--" placeholder from a failed source.
func hasResult(r engine.Result) bool {
	return (r.Big != "" && r.Big != "--") || r.Chart != nil || r.ProgressBar != nil || len(r.Images) > 0 || len(r.Stats) > 0
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
func (a *app) invalidateAll() {
	for k, st := range a.runs {
		a.keepShown(k, st)
	}
	a.runs = map[string]*runState{}
}

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
	if sc.IsBar() {
		m := a.barMeta(sc)
		return engine.Result{Title: m.Name, Icon: m.Icon}, nil
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
	opt := engine.FetchOptions{TTL: sourceTTL, Offline: set.Offline, Force: force, Lang: curLang(), Headers: s.Meta.SourceHeaders}
	opt.LiveTTL = s.Meta.LiveTTL()
	if a.cache != nil {
		opt.Cache = cacheAdapter{a.cache}
	}
	// The variables the user chose fill the sources: each combination is its
	// own URL, so its own cache and history.
	vars := a.varsOf(sc, s.Meta)
	reqs := make(map[string]engine.Request, len(s.Meta.Sources))
	keys := make(map[string]string, len(s.Meta.Sources))
	for name, src := range s.Meta.Sources {
		r := src.Request(now, vars)
		for k, v := range s.Meta.SourceHeaders[name] {
			r.Headers[k] = v
		}
		reqs[name] = r
		keys[name] = r.CacheKey()
	}
	sources, info := engine.FetchRequests(context.Background(), a.client, reqs, now, opt)
	var history map[string][]engine.HistoryPoint
	if names, n := historyWanted(s.Meta); len(names) > 0 && opt.Cache != nil {
		history = engine.HistoryOf(opt.Cache, keys, names, n, now)
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
		Vars:       vars,
	})
	if err == nil {
		res.DataAt, res.Stale = dataTimes(info)
		res.Images = a.keepImages(sc, res.Images, now)
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
	// A setting may be changed in either window: redraw both.
	a.post(func() {})
	if a.settingsWin != nil {
		a.settingsWin.Update(func() {})
	}
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

// retiredBuiltins are old or duplicate built-in scripts that were removed in
// favor of cleaner modern replacements. Unmodified copies from earlier versions
// are cleaned up automatically in ensureBuiltins. Extra SHA-256 hashes listed
// here cover retired scripts that have no legacy testdata fixture.
var retiredBuiltins = map[string][]string{
	"ai-claude":     nil,
	"ai-deepseek":   nil,
	"ai-openrouter": nil,
	"fxcny":         nil,
	"fxcny-kv":      {"77adc2af5afeda8f208c02f447687804077da14f676accddecdf23fecc4cabed"},
	"gold":          nil,
	"goldacc":       nil,
	"cnwx":          nil,
	"aqi":           nil,
}

func isRetiredUnmodified(id, source string) bool {
	if refreshBuiltin(id, source) {
		return true
	}
	if fps := retiredBuiltins[id]; len(fps) > 0 {
		sum := sha256.Sum256([]byte(source))
		h := hex.EncodeToString(sum[:])
		for _, f := range fps {
			if h == f {
				return true
			}
		}
	}
	return false
}

// ensureBuiltins adds the built-in scripts a data file from an earlier version
// does not have yet, refreshes unmodified built-ins to the latest version, and
// removes unmodified retired built-ins that were replaced by modern ones.
func (a *app) ensureBuiltins() error {
	for id := range retiredBuiltins {
		sc, ok := a.store.Script(id)
		if !ok {
			continue
		}
		if (sc.Origin == store.OriginBuiltin || sc.Origin == "") && isRetiredUnmodified(id, sc.Source) {
			if err := a.store.DeleteScript(id); err != nil {
				return err
			}
		}
	}
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
		home := defaultHomeBuiltin(b.ID)
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
	if s, err := engine.Compile(id+".js", source); err == nil && s.Meta.Version == "" {
		return true
	}
	if fingerprints := previousBuiltins[id]; len(fingerprints) > 0 {
		sum := sha256.Sum256([]byte(source))
		for _, f := range fingerprints {
			if hex.EncodeToString(sum[:]) == f {
				return true
			}
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

// previousBuiltins are the SHA-256 fingerprints of the built-in scripts as they
// shipped in earlier versions. A stored copy with one of them was never edited,
// so it takes the new script on start; an edited copy is kept.
var previousBuiltins = map[string][]string{
	"ai-claude": {
		"01d43a22fa4a5468002ae210e564ca9478e71d3b0c30b904e5e26323ef7dcc3e",
		"57920d75fca8033b91f46c783d68ccdc23713b206433db1ae4897853ab87be9a",
		"b6dce9d3d583d884c8010e5c480dafd8bab228d7954ae5356789abc65289a6dc",
		"f6b1d04666479750bc1700fb62b8597c922bdd8156e2361283d07bdd2cfda411",
	},
	"ai-deepseek": {
		"17979bf307a1e9a5a21ffef0ce78e29736b08b54e5fae00dd308d73911c8a77e",
		"6af299d782e7d4bc9c18cd73499b7648477acca2e2a46304944d52c860732e4b",
		"f5f55d13f2a56ee57f03f35a5cd648fb925b988960744a7da992177c8ad3e0fa",
	},
	"ai-openrouter": {
		"9ca38d8df202545e5910a78387fe029bebaa7d6b00927823d4de3fc081775e2e",
		"c4df543586af278eb2c74bd32ab04ba81591ce713ca6dc3a2b0c20eaab2cad38",
		"d8471910d8aa99cd3d12526009842700b568b3fe9b0b5ce0e81cc9de4a6d1eb3",
	},
	"gold": {
		"0277a3e4520967925e85a84ab230fd42976bb96c83823e9d29ff0fcfbdf53d74",
		"18b7d852d6d4215d88c1dc50dded64bda84598f840c5992e01802dfc6944143c",
		"4b0ed00df43b50c9128e73e8e0a38c47883b96e1527f807fc4173fd307da8f48",
		"506f1b9ebf031bd163db5a441bcaa43ded005599faeac77d1185321380d8c83a",
		"5f7e420061fd66e6450d66cc0332f2da22fa747d7f356653996bd536c50f510e",
		"bf7b399b3581617ddcd24ee6f2670efc92842e3dc6524e8c4a7d2d512cdb3601",
	},
	"goldacc": {
		"59d9d236c5a0df22d1a36ebebbcff871e9c1c9de27763d996cad451915f72f90",
		"69995db57cbee0af1ef11c4a3265862385cf775f0bdc6920fd5f2751c1820755",
		"adfccf450de4d4b75a823080718b51c5371eae0a930367ca2b0aff453a6a674c",
		"bb29525848fc3436cdacc9bfdb0fac6d2355f30b58f2cca24a6c25b3c912fac1",
	},
	"goldk": {
		"21474d818d19cf6e0102b3845d3bbad43e45ab791fd10c9cf958c5643eb5a3ca",
		"aceb6870c1ef7a46ab6b0a8a844a7019a72c889703b414a16f8413f9dc3e387f",
		"bc34812902296c0800b6c995d7695447a1f3fc7dfc4150030fc4928837c53526",
		"d5822e515716e7897db2c56f25fb739e8e2ec8f75a7a7e2eed60af8e104eb7c7",
		"f680c5fe268f5fefd8f75094611dbfceedeb557302800fc8446327ab480abb0e",
	},
	"weight": {
		"0f43a5ce93da26669fb0c5b462257d093695fa31d9558afb9a2ad7062d3617af",
		"2dddafbc1938f5294f9d73c207ed5ff7e16c1f81d13a67687601809d162f8944",
		"4cc49d899f15ab6c805525f5d4330ca0524ce51cd828ff8fff65eb35f165e99e",
		"e8d2bcaa0e5f41badc0fe7b8f0f230df2bd5a090547fecd157a26f05f3e90578",
	},
	"fxcny": {
		"4889b8f502a3c31454a19124dd67d08662d4dab0b87081ed03de9543270d384e",
		"4efe2a6d4d67206c4b8b92d5d7eae023d9560e70bab67684fbbc6b6d8c03c35e",
		"bf9de9193b07055e571f89ad903f681fa2f9ad98a33910ef627d1b9e030167e9",
		"deaf30589c4cbdb5f65446f58f319cfae4c8e3aea697f50b74aa04f3163766b7",
	},
	"fx": {
		"0d878adca9d18e653d30d766289e01adeb8db6dac7f19f7f64caee3e53c57468",
		"bef1d20129a0faa4d8ee06c34319bdc80746996bc2b52b9fba60794b11b9fed6",
		"e1c51dfb3ea10b04344eafd0327876f56b5f0465e9d581a3db9849c06e1a6907",
		"ec33aecef15f8e5f418b0411b88ee65fcb17ab076386fa89c34aab3c0b727938",
	},
	"weather": {
		"0098ead3b6349df77d9ddb91495179ff02d1b07a6a460c9d25d3d484e355b56a",
		"61d32c7ea19e34e37492597deaabcc9e3e8701ea86240f0344fec04966abb30c",
		"bb2691362cba6f5a58efe1b72ec03fdf0b1a5c70037d33c1b689b98cfb2c2471",
		"df88f52041b64e51c335b67c86de9818178d13c4acde824ce6b799dc5b1abd84",
	},
	"aqi": {
		"8577e85043f4d9529b62ecb2abe1a28ec4de9d213a8afe77e4912ae671f84289",
		"dd3f05d81d39ca4e57c341e8035c38e6221d4f736bafb64c8f730c9b0fb00d30",
	},
	"btc": {
		"26b36f771479fdc59daadff7c70cdb6d86a10cc17ce935569cf4b51563d60e16",
		"8a04a645d916fe443f0b450518db27a4fcbeec5324122bae51da2c97cf117f6e",
		"abe59a2b07c31ef986f3af058828b88865afc623fcafb003519a3ccdaba5ec4d",
	},
	"cnwx": {
		"a2a145f8e9f4c9e4bbfe5c60b7366640e17b25c8cbadee83d8e5de28d37dac34",
		"d46fda97fcf2256357d352919bfbeb555b1f3e3a6fee491ddb4f169fe7bc53ee",
	},
	"hs300": {
		"30482a41f0ffca6f1becda31e97abd690ea0bc290ea0bc2963910c5370218ae0be6b9adb",
		"30482a41f0ffca6f1becda31e97abd690ea0bc2963910c5370218ae0be6b9adb",
		"d87f8755bd9cb8a05dcfc91a7c300b18f4c43e4641da24e4e83e316742d9c50d",
	},
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

// defaultHomeBuiltin reports whether a newly added built-in script starts on
// the home page. Scripts that require an API key or local AI logs, as well as
// optional companions, start off-home so users can enable them when needed.
func defaultHomeBuiltin(id string) bool {
	switch id {
	case "ai-claude-neon", "ai-balance-neon", "weight-contrib":
		return false
	}
	return true
}

// builtinIDs are the ids of the scripts that ship with the app.
func builtinIDs() map[string]bool {
	ids := map[string]bool{
		"sb-weather": true,
	}
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
