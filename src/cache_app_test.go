package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

type cacheFixture struct {
	a    *app
	srv  *httptest.Server
	hits *atomic.Int32
	down *atomic.Bool
	sc   store.Script
}

// cacheApp is an app with one script that reads a server counting its hits.
func cacheApp(t *testing.T, extraMeta string) cacheFixture {
	t.Helper()
	var hits atomic.Int32
	var down atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if down.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintf(w, `{"n": %d}`, 100+n)
	}))
	t.Cleanup(srv.Close)

	a := newTestApp(t)
	st := a.store.Settings()
	st.Offline = false
	a.store.SetSettings(st)
	a.set = st
	for _, sc := range a.store.Scripts() {
		sc.Home = false
		a.store.SaveScript(sc)
	}
	src := `export function meta() { return { name: "缓存", ` + extraMeta + ` sources: { api: "` + srv.URL + `/n" } }; }
export function render(ctx) {
  const i = ctx.sourceInfo.api, h = (ctx.history && ctx.history.api) || [];
  const err = ctx.sources.api.error;
  return { title: "缓存", big: err ? "ERR" : String(ctx.sources.api.n), sub: h.length + "/" + i.cached + "/" + i.stale, lines: [], chart: null };
}`
	sc := store.Script{ID: "cache-card", Name: "缓存", Source: src, Home: true}
	if err := a.store.SaveScript(sc); err != nil {
		t.Fatal(err)
	}
	return cacheFixture{a, srv, &hits, &down, sc}
}

// Opening a card again within the TTL does not ask the network; the refresh
// button does.
func TestCachedSourceSkipsTheNetworkWithinTheTTL(t *testing.T) {
	f := cacheApp(t, "")
	res, err := f.a.execute(f.sc, nil)
	if err != nil || res.Big != "101" || res.Stale {
		t.Fatalf("first run %q %v stale %v", res.Big, err, res.Stale)
	}
	res, _ = f.a.execute(f.sc, nil)
	if res.Big != "101" || f.hits.Load() != 1 || res.Sub != "0/true/false" {
		t.Fatalf("second run %q %q hits %d", res.Big, res.Sub, f.hits.Load())
	}
	res, _ = f.a.executeOpt(f.sc, nil, true)
	if res.Big != "102" || f.hits.Load() != 2 {
		t.Fatalf("forced run %q hits %d", res.Big, f.hits.Load())
	}
	if res.DataAt == 0 {
		t.Error("the result does not say when its data is from")
	}
}

// When the network is down the last response stands in, and the result says so;
// offline mode makes no request at all and does the same.
func TestOfflineAndFailureServeTheCache(t *testing.T) {
	f := cacheApp(t, "")
	f.a.execute(f.sc, nil)
	f.down.Store(true)
	res, err := f.a.executeOpt(f.sc, nil, true)
	if err != nil || res.Big != "101" || !res.Stale || res.DataAt == 0 {
		t.Fatalf("server down: %q %v stale %v at %d", res.Big, err, res.Stale, res.DataAt)
	}

	hits := f.hits.Load()
	st := f.a.set
	st.Offline = true
	f.a.store.SetSettings(st)
	res, err = f.a.executeOpt(f.sc, nil, true)
	if err != nil || res.Big != "101" || !res.Stale || f.hits.Load() != hits {
		t.Fatalf("offline: %q %v stale %v hits %d→%d", res.Big, err, res.Stale, hits, f.hits.Load())
	}
}

// A script that asks for history gets the responses of its source, oldest
// first, and not when it does not ask.
func TestScriptsGetTheHistoryTheyAskFor(t *testing.T) {
	f := cacheApp(t, `history: ["api"],`)
	for i := 0; i < 3; i++ {
		if _, err := f.a.executeOpt(f.sc, nil, true); err != nil {
			t.Fatal(err)
		}
	}
	res, _ := f.a.execute(f.sc, nil)
	if !strings.HasPrefix(res.Sub, "3/") {
		t.Errorf("history %q, want three responses", res.Sub)
	}
	plain := cacheApp(t, "")
	plain.a.executeOpt(plain.sc, nil, true)
	if res, _ := plain.a.execute(plain.sc, nil); !strings.HasPrefix(res.Sub, "0/") {
		t.Errorf("a script that did not ask got %q", res.Sub)
	}
}

// A card opens at once from its last result, and when the network is down it
// keeps it, saying how old it is.
func TestCardOpensFromItsSavedResult(t *testing.T) {
	f := cacheApp(t, "")
	tt := ui.NewTester(f.a.view, 1100, 900)
	settle(tt)
	requireText(t, tt, "101")

	// A new run of the app: the result of the last one is on disk.
	f.a.runs = map[string]*runState{}
	f.down.Store(true)
	meta, _ := f.a.metaOf(f.sc)
	res, ok := f.a.savedResult(f.sc, runKey(f.sc.ID, meta.Dims, nil), meta)
	if !ok || res.Big != "101" || res.DataAt == 0 {
		t.Fatalf("saved result %+v %v", res, ok)
	}
	// The result of an edited script is not shown for the old one.
	other := f.sc
	other.Source += "\n// edited"
	if _, ok := f.a.savedResult(other, runKey(other.ID, meta.Dims, nil), meta); ok {
		t.Error("a changed script was shown the result of the old one")
	}

	// Failing run: the card keeps the saved result and marks it old.
	f.a.now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	f.a.invalidateAll()
	f.a.forced[f.sc.ID] = true
	tt2 := ui.NewTester(f.a.view, 1100, 900)
	settle(tt2)
	requireText(t, tt2, "101")
	requireText(t, tt2, "小时前")
}

// The settings show what the cache takes, and clearing it forgets it all.
func TestClearCache(t *testing.T) {
	f := cacheApp(t, "")
	f.a.execute(f.sc, nil)
	if f.a.cache.Size() == 0 {
		t.Fatal("nothing was cached")
	}
	if err := f.a.clearCache(); err != nil {
		t.Fatal(err)
	}
	if f.a.cache.Size() != 0 {
		t.Errorf("%d bytes left", f.a.cache.Size())
	}
	f.down.Store(true)
	res, _ := f.a.execute(f.sc, nil)
	if res.Big != "ERR" {
		t.Errorf("after clearing, a down server gave %q", res.Big)
	}
}

// A failed run that returns "--" must not be saved as a cached result or
// overwrite a valid saved result.
func TestFailedPlaceholderIsNotSavedAsCachedResult(t *testing.T) {
	f := cacheApp(t, "")
	meta, _ := f.a.metaOf(f.sc)
	key := runKey(f.sc.ID, meta.Dims, nil)
	f.a.saveResult(f.sc, key, meta, engine.Result{Title: "缓存", Big: "--", Sub: "failed"})
	if _, ok := f.a.savedResult(f.sc, key, meta); ok {
		t.Fatal("a '--' placeholder was kept as a saved result")
	}
}
