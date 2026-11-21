package main

import (
	"testing"
	"time"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

// A live card is due when it was drawn lately and its last run is an interval
// old (or it has none yet). A card not drawn lately is not run.
func TestLiveDue(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	every := 5 * time.Second
	cases := []struct {
		name       string
		seen, last time.Time
		want       bool
	}{
		{"drawn, never run", now, time.Time{}, true},
		{"drawn, run too recently", now, now.Add(-2 * time.Second), false},
		{"drawn, run an interval ago", now, now.Add(-5 * time.Second), true},
		{"not drawn lately", now.Add(-10 * time.Second), now.Add(-20 * time.Second), false},
		{"never drawn", time.Time{}, time.Time{}, false},
	}
	for _, c := range cases {
		if got := liveDue(now, c.seen, c.last, every); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}

// The cache's history of a script keeps the sources its live line is drawn from,
// even when the script did not name them.
func TestHistoryWantedIncludesLiveSources(t *testing.T) {
	names, n := historyWanted(engine.Meta{History: []string{"klines"}, Live: &engine.Live{Sources: []string{"ticker", "klines"}}, HistoryLimit: 180})
	if len(names) != 2 || names[0] != "klines" || names[1] != "ticker" || n != 180 {
		t.Errorf("names %v, limit %d", names, n)
	}
	names, n = historyWanted(engine.Meta{Live: &engine.Live{Sources: []string{"ticker"}}})
	if len(names) != 1 || names[0] != "ticker" || n != defaultHistory {
		t.Errorf("live only: names %v, limit %d", names, n)
	}
	if names, _ := historyWanted(engine.Meta{}); names != nil {
		t.Errorf("a script with no history wants %v", names)
	}
}

// A due live card runs once, and not again before its interval has passed.
func TestLiveCheckRunsADueCardOnceAnInterval(t *testing.T) {
	a := newTestApp(t)
	a.set.Offline = false
	sc := store.Script{ID: "live-t", Name: "实时", Home: true, Origin: store.OriginUser,
		Source: `export function meta() { return { name: "实时", live: { every: 5 }, sources: {} }; }
export function render(ctx) { return { title: "实时", big: "7", sub: "", lines: [], chart: null }; }`}
	a.store.SaveScript(sc)
	meta, err := a.metaOf(sc)
	if err != nil || meta.Live == nil {
		t.Fatalf("meta %+v, %v", meta, err)
	}
	_, vals := a.viewOf(sc.ID, meta.Dims)
	key := a.runKeyOf(sc.ID, sc, meta, vals)

	a.watchLive(sc.ID, meta)
	a.liveCheck()
	first, ok := a.runs[key]
	if !ok || first.res.Big != "7" {
		t.Fatalf("the due card did not run: %+v", first)
	}
	a.watchLive(sc.ID, meta)
	a.liveCheck()
	if a.runs[key] != first {
		t.Error("the card ran again before its interval")
	}

	// While a run is updating (with its previous result kept on screen), a due
	// tick must wait for it rather than dropping the in-flight run.
	first.updating = true
	a.live.at[sc.ID] = a.now().Add(-10 * time.Second)
	a.watchLive(sc.ID, meta)
	a.liveCheck()
	if a.runs[key] != first {
		t.Error("liveCheck dropped an updating run before it finished")
	}
}
