package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

// A script with a variable: its source takes the city the user chose, and the
// server says which city it was asked for.
func varsApp(t *testing.T) (*app, store.Script, *atomic.Int32, string) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		city := strings.TrimPrefix(r.URL.Path, "/")
		fmt.Fprintf(w, `{"city": %q, "t": %d}`, city, hits.Load())
	}))
	t.Cleanup(srv.Close)
	a := newTestApp(t)
	st := a.store.Settings()
	st.Offline = false
	a.store.SetSettings(st)
	a.set = st
	src := `export function meta() { return {
  name: "城市天气", sources: { w: "` + srv.URL + `/{city}" },
  vars: [{ key: "city", label: "城市", default: "shanghai", options: [
    { value: "shanghai", label: "上海" }, { value: "beijing", label: "北京" }] }] }; }
export function render(ctx) { return { title: "x", big: ctx.sources.w.city, sub: "vars " + ctx.vars.city, lines: [], chart: null }; }`
	sc := store.Script{ID: "city-card", Name: "城市天气", Source: src, Home: true}
	if err := a.store.SaveScript(sc); err != nil {
		t.Fatal(err)
	}
	return a, sc, &hits, srv.URL + "/{city}"
}

// The chosen value fills the URL and reaches the script as ctx.vars; a value
// that is not an option falls back to the default.
func TestVariablesFillTheSourceURL(t *testing.T) {
	a, sc, _, _ := varsApp(t)
	res, err := a.execute(sc, nil)
	if err != nil || res.Big != "shanghai" || res.Sub != "vars shanghai" {
		t.Fatalf("default: %q %q %v", res.Big, res.Sub, err)
	}
	a.setVar(sc, "city", "beijing")
	res, err = a.execute(sc, nil)
	if err != nil || res.Big != "beijing" || res.Sub != "vars beijing" {
		t.Fatalf("beijing: %q %q %v", res.Big, res.Sub, err)
	}
	a.setVar(sc, "city", "atlantis")
	res, _ = a.execute(sc, nil)
	if res.Big != "shanghai" {
		t.Errorf("an unknown city gave %q, want the default", res.Big)
	}
}

// Each value is its own data: its own cached response, so switching cities
// never mixes one city's readings into the other's.
func TestEachValueHasItsOwnCache(t *testing.T) {
	a, sc, hits, tmpl := varsApp(t)
	a.execute(sc, nil)
	a.setVar(sc, "city", "beijing")
	a.execute(sc, nil)
	if hits.Load() != 2 {
		t.Fatalf("%d requests for two cities, want 2", hits.Load())
	}
	now := a.now()
	for _, city := range []string{"shanghai", "beijing"} {
		url := engine.ExpandVars(tmpl, now, map[string]string{"city": city})
		if _, ok := a.cache.Get(url); !ok {
			t.Errorf("no cached response for %s at %s", city, url)
		}
	}
}

// A new choice never shows the result of an old one.
func TestNewChoiceHasNoOldResult(t *testing.T) {
	a, sc, _, _ := varsApp(t)
	a.execute(sc, nil)
	res, _ := a.execute(sc, nil)
	a.setVar(sc, "city", "beijing")
	m, _ := a.metaOf(sc)
	key := a.runKeyOf(sc.ID, sc, m, nil)
	if _, ok := a.savedResult(sc, key, m); ok {
		t.Errorf("beijing shows a saved result before it was ever run (%q)", res.Big)
	}
}

// The reset forgets the data of every value, the saved results and the runs, so
// that the next run asks the network afresh.
func TestResetForgetsTheScriptsData(t *testing.T) {
	a, sc, hits, tmpl := varsApp(t)
	a.setVar(sc, "city", "beijing")
	a.execute(sc, nil)
	a.setVar(sc, "city", "shanghai")
	a.execute(sc, nil)
	if err := a.resetData(sc); err != nil {
		t.Fatal(err)
	}
	now := a.now()
	for _, city := range []string{"shanghai", "beijing"} {
		url := engine.ExpandVars(tmpl, now, map[string]string{"city": city})
		if _, ok := a.cache.Get(url); ok {
			t.Errorf("%s was kept by the reset", city)
		}
	}
	m, _ := a.metaOf(sc)
	if _, ok := a.savedResult(sc, a.runKeyOf(sc.ID, sc, m, nil), m); ok {
		t.Error("a saved result was kept by the reset")
	}
	before := hits.Load()
	res, _ := a.execute(sc, nil)
	if hits.Load() == before {
		t.Errorf("the next run after a reset did not ask the network (%q)", res.Big)
	}
}

// Combinations lists every choice of every variable, and stops at its bound.
func TestCombinations(t *testing.T) {
	vars := []engine.Var{
		{Key: "a", Options: []engine.VarOption{{Value: "1"}, {Value: "2"}}},
		{Key: "b", Options: []engine.VarOption{{Value: "x"}, {Value: "y"}, {Value: "z"}}},
	}
	if got := engine.Combinations(vars); len(got) != 6 {
		t.Errorf("%d combinations, want 6", len(got))
	}
	if got := engine.Combinations(nil); len(got) != 1 {
		t.Errorf("no variables: %d combinations, want one empty", len(got))
	}
}

// Every built-in script compiles, and each variable's default is one of its
// own options, so a fresh install always has a value to show.
func TestBuiltinVariablesAreWellFormed(t *testing.T) {
	for _, b := range engine.Builtins() {
		s, err := engine.Compile(b.ID+".js", b.Source)
		if err != nil {
			t.Errorf("%s: %v", b.ID, err)
			continue
		}
		for _, v := range s.Meta.Vars {
			if len(v.Options) == 0 {
				t.Errorf("%s: variable %s has no options", b.ID, v.Key)
			}
			ok := false
			for _, o := range v.Options {
				ok = ok || o.Value == v.Default
			}
			if !ok {
				t.Errorf("%s: default %q of %s is not one of its options", b.ID, v.Default, v.Key)
			}
		}
	}
}

// A built-in with a city switch shows the city the user chose.
func TestWeatherShowsTheChosenCity(t *testing.T) {
	a := newTestApp(t)
	sc, ok := a.store.Script("weather")
	if !ok {
		t.Fatal("no weather script")
	}
	res, err := a.execute(sc, nil)
	if err != nil || !strings.Contains(res.Title, "上海") {
		t.Fatalf("default title %q %v", res.Title, err)
	}
	a.setVar(sc, "city", "latitude=39.90&longitude=116.41")
	res, err = a.execute(sc, nil)
	if err != nil || !strings.Contains(res.Title, "北京") {
		t.Errorf("after switching: title %q %v", res.Title, err)
	}
}

func TestCnwxShowsTheChosenCity(t *testing.T) {
	a := newTestApp(t)
	sc, ok := a.store.Script("cnwx")
	if !ok {
		t.Fatal("no cnwx script")
	}
	res, err := a.execute(sc, nil)
	if err != nil || !strings.Contains(res.Title, "上海") {
		t.Fatalf("default title %q %v", res.Title, err)
	}
	a.setVar(sc, "city", "%E5%8C%97%E4%BA%AC")
	res, err = a.execute(sc, nil)
	if err != nil || !strings.Contains(res.Title, "北京") {
		t.Errorf("after switching: title %q %v", res.Title, err)
	}
}
