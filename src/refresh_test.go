package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/feed"
	"scriptboard/src/internal/store"
)

// A card has a refresh button: it asks the card's own endpoint and its data
// sources again, now.
func TestRefreshButtonAsksAgain(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"n": %d}`, 100+hits.Add(1))
	}))
	defer srv.Close()

	a := newTestApp(t)
	st := a.store.Settings()
	st.Offline = false
	if err := a.store.SetSettings(st); err != nil {
		t.Fatal(err)
	}
	a.set = st

	for _, sc := range a.store.Scripts() { // only the new card, so there is one button
		sc.Home = false
		a.store.SaveScript(sc)
	}
	src := `export function meta() { return { name: "我的接口", sources: { api: "` + srv.URL + `/n" } }; }
export function render(ctx) { return { title: "我的接口", big: String(ctx.sources.api.n), sub: "", lines: [], chart: null }; }`
	if err := a.store.SaveScript(store.Script{ID: "my-api", Name: "我的接口", Source: src, Home: true}); err != nil {
		t.Fatal(err)
	}

	tt := ui.NewTester(a.view, 1100, 900)
	settle(tt)
	requireText(t, tt, "101")

	click(t, tt, "刷新数据")
	if hits.Load() < 2 {
		t.Fatalf("the endpoint was asked %d times, want a second time after the refresh", hits.Load())
	}
	requireText(t, tt, "102")
}

func TestRefreshPollsTheFeedsOfTheScript(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"v": %d}`, hits.Add(1))
	}))
	defer srv.Close()

	a := newTestApp(t)
	st := a.store.Settings()
	st.Offline = false
	a.store.SetSettings(st)
	a.set = st
	if err := a.store.SaveFeed(feed.Source{ID: "live", Name: "live", Every: 3600,
		Request: feed.Request{URL: srv.URL + "/v"}, Extract: feed.Extract{Kind: feed.KindJSON, Path: "v"}}); err != nil {
		t.Fatal(err)
	}
	src := `export function meta() { return { name: "实时", feeds: ["live"], sources: {} }; }
export function render(ctx) { return { title: "实时", big: String(ctx.feeds.live.latest[0] ?? "-"), sub: "", lines: [], chart: null }; }`
	sc := store.Script{ID: "live-card", Name: "实时", Source: src, Home: true}
	if err := a.store.SaveScript(sc); err != nil {
		t.Fatal(err)
	}
	a.refreshScript(sc) // a.win is nil in tests, so it runs at once
	if hits.Load() != 1 {
		t.Fatalf("feed polled %d times, want 1", hits.Load())
	}
	res, err := a.execute(sc, nil)
	if err != nil || res.Big != "1" {
		t.Fatalf("after refresh: %q %v", res.Big, err)
	}
	if a.refreshing[sc.ID] {
		t.Error("still marked as refreshing")
	}

	// Offline: nothing is requested.
	st.Offline = true
	a.store.SetSettings(st)
	a.refreshScript(sc)
	if hits.Load() != 1 {
		t.Errorf("offline refresh made a request (%d)", hits.Load())
	}
}

// The data sources page shows how each source is doing, and which protocol
// and fallback it uses.
func TestFeedHealthOnThePage(t *testing.T) {
	main := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusUnauthorized) // not retried: the fallback answers
	}))
	defer main.Close()
	backup := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"p": 643.5, "q": 12}`)
	}))
	defer backup.Close()

	a := newTestApp(t)
	st := a.store.Settings()
	st.Offline = false
	a.store.SetSettings(st)
	a.set = st
	src := feed.Source{ID: "gold-live", Name: "Live gold", Every: 60, Names: []string{"Gold", "Volume"},
		Request:   feed.Request{URL: main.URL + "/p"},
		Fallbacks: []feed.Request{{URL: backup.URL + "/p"}},
		Extract:   feed.Extract{Kind: feed.KindJSON, Path: "p"}}
	if err := a.store.SaveFeed(src); err != nil {
		t.Fatal(err)
	}
	// A second source that is down.
	dead := feed.Source{ID: "dead-api", Name: "Dead API", Every: 60, Retries: -1,
		Request: feed.Request{URL: main.URL + "/p"}, Extract: feed.Extract{Kind: feed.KindJSON, Path: "p"}}
	a.store.SaveFeed(dead)
	if _, err := a.runner.Poll(t.Context(), src); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		a.runner.Poll(t.Context(), dead)
	}
	if h := a.runner.Health("gold-live"); h.Used != 1 {
		t.Fatalf("health %+v", h)
	}
	a.router.Push("/feeds")
	tt := ui.NewTester(a.view, 1100, 900)
	settle(tt)
	requireText(t, tt, "正常 · 由备用请求 1 提供")
	requireText(t, tt, "已暂停")
	savePNG(t, tt, "15-feeds-health")

	setLang(langEN)
	defer setLang(langZH)
	a.invalidateAll()
	settle(tt)
	requireText(t, tt, "Healthy · served by fallback 1")
	requireText(t, tt, "Data sources")
	onlyEnglish(t, tt, "feeds")
	savePNG(t, tt, "15-feeds-health-en")
}
