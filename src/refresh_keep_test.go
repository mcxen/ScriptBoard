package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

// A refresh keeps the card's last result: a card with a saved result shows it,
// marked as updating, while the new one is under way, and only a card with
// nothing saved is loading.
func TestRefreshKeepsTheLastResult(t *testing.T) {
	a := newTestApp(t)
	sc := store.Script{ID: "pics", Name: "图片", Home: true, Origin: store.OriginUser,
		Source: `export function meta() { return { name: "图片", sources: { api: "https://example.com/x" } }; }
export function render(ctx) { return { title: "图片", big: "2", sub: "", lines: [], chart: null }; }`}
	a.store.SaveScript(sc)
	meta, _ := a.metaOf(sc)
	_, vals := a.viewOf(sc.ID, meta.Dims)
	key := a.runKeyOf(sc.ID, sc, meta, vals)

	if st := a.startState(sc, key, meta); !st.loading {
		t.Fatalf("nothing is saved yet, the card should load: %+v", st)
	}
	saved := engine.Result{Title: "图片", Big: "2", Images: []engine.Image{{URL: "http://x/1.png", Caption: "one", Local: "/tmp/one.img"}}}
	a.saveResult(sc, key, meta, saved)
	st := a.startState(sc, key, meta)
	if st.loading || !st.old || !st.updating || len(st.res.Images) != 1 || st.res.Big != "2" {
		t.Fatalf("a refresh shows %+v, want the saved result, updating", st)
	}
}

// On the home page the pictures of a refreshing card stay on screen.
func TestRefreshingCardKeepsItsPictures(t *testing.T) {
	a := newTestApp(t)
	sc := store.Script{ID: "pics-home", Name: "图片", Home: true, Origin: store.OriginUser,
		Source: `export function meta() { return { name: "图片", sources: { api: "https://example.com/x" } }; }
export function render(ctx) { return { title: "图片", big: "2", sub: "", lines: [], chart: null }; }`}
	a.store.SaveScript(sc)
	meta, _ := a.metaOf(sc)
	_, vals := a.viewOf(sc.ID, meta.Dims)
	key := a.runKeyOf(sc.ID, sc, meta, vals)
	a.saveResult(sc, key, meta, engine.Result{Title: "图片", Big: "2",
		Images: []engine.Image{{URL: "http://x/1.png", Caption: "shown-while-refreshing", Local: "/tmp/one.img"}}})
	// The refresh's run is under way: the card has its saved result, not a loading state.
	a.runs[key] = a.startState(sc, key, meta)

	tt := ui.NewTester(a.view, 1200, 1400)
	settle(tt)
	settle(tt)
	requireText(t, tt, "shown-while-refreshing")
}
