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

	hover(t, tt, "刷新数据") // the card's buttons show while the pointer is on it
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

func TestRefreshProgressAndSuccessFeedback(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"n": %d}`, hits.Add(1))
	}))
	defer srv.Close()

	a := newTestApp(t)
	now := time.Date(2026, 11, 26, 15, 0, 0, 0, time.UTC)
	a.now = func() time.Time { return now }
	st := a.store.Settings()
	st.Offline = false
	_ = a.store.SetSettings(st)
	a.set = st

	sc := store.Script{
		ID: "ref-ok", Name: "刷新成功测试", Home: true,
		Source: `export function meta() { return { name: "刷新成功测试", sources: { api: "` + srv.URL + `/n" } }; }
export function render(ctx) { return { title: "刷新成功测试", big: String(ctx.sources.api.n) }; }`,
	}
	_ = a.store.SaveScript(sc)
	showOnly(a, sc.ID)

	tt := ui.NewTester(a.view, 1100, 900)
	settle(tt)
	requireText(t, tt, "1")

	hover(t, tt, "刷新数据")
	click(t, tt, "刷新数据")

	if got := a.refreshStateOf(sc.ID); got.phase != refreshOK {
		t.Fatalf("expected refreshOK, got %+v", got)
	}
	if !a.refreshActive(sc.ID) || a.isRefreshing(sc.ID) {
		t.Fatalf("unexpected active=%v running=%v", a.refreshActive(sc.ID), a.isRefreshing(sc.ID))
	}
	if tip := refreshTooltip(tr("刷新数据"), a.refreshStateOf(sc.ID)); tip != tr("已刷新") {
		t.Fatalf("tooltip = %q, want %q", tip, tr("已刷新"))
	}

	a.router.Push("/full/" + sc.ID)
	settle(tt)
	requireText(t, tt, "已刷新")

	// Advancing past refreshOKDuration clears completion state back to idle.
	now = now.Add(refreshOKDuration + time.Millisecond)
	if got := a.refreshStateOf(sc.ID); got.phase != refreshIdle {
		t.Fatalf("expected refreshIdle after expiry, got %+v", got)
	}
	if a.refreshActive(sc.ID) {
		t.Fatal("expected refreshActive false after expiry")
	}
	settle(tt)
	requireText(t, tt, "刷新")
}

func TestRefreshErrorAndStaleFeedback(t *testing.T) {
	var fail atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail.Load() {
			http.Error(w, "upstream down", http.StatusInternalServerError)
			return
		}
		fmt.Fprint(w, `{"n": 42}`)
	}))
	defer srv.Close()

	a := newTestApp(t)
	now := time.Date(2026, 11, 26, 15, 0, 0, 0, time.UTC)
	a.now = func() time.Time { return now }
	st := a.store.Settings()
	st.Offline = false
	_ = a.store.SetSettings(st)
	a.set = st

	sc := store.Script{
		ID: "ref-err", Name: "刷新失败测试", Home: true,
		Source: `export function meta() { return { name: "刷新失败测试", sources: { api: "` + srv.URL + `/n" } }; }
export function render(ctx) { return { title: "刷新失败测试", big: ctx.sources.api && ctx.sources.api.n ? String(ctx.sources.api.n) : "" }; }`,
	}
	_ = a.store.SaveScript(sc)
	showOnly(a, sc.ID)

	tt := ui.NewTester(a.view, 1100, 900)
	settle(tt)
	requireText(t, tt, "42")

	fail.Store(true)
	hover(t, tt, "刷新数据")
	click(t, tt, "刷新数据")

	got := a.refreshStateOf(sc.ID)
	if got.phase != refreshError {
		t.Fatalf("expected refreshError on failed HTTP refresh, got %+v", got)
	}
	if tip := refreshTooltip(tr("刷新数据"), got); !strings.Contains(tip, tr("刷新失败")) {
		t.Fatalf("expected failure tooltip, got %q", tip)
	}

	a.router.Push("/full/" + sc.ID)
	settle(tt)
	requireText(t, tt, "刷新失败")

	now = now.Add(refreshErrorDuration + time.Millisecond)
	if a.refreshStateOf(sc.ID).phase != refreshIdle {
		t.Fatal("expected refreshIdle after error expiry")
	}
}

func TestRefreshDeterminateStepProgress(t *testing.T) {
	a := newTestApp(t)
	a.beginRefresh("multi", 3)
	a.advanceRefresh("multi", 1, "")

	st := a.refreshStateOf("multi")
	if st.phase != refreshRunning || st.step != 1 || st.total != 3 {
		t.Fatalf("unexpected step state: %+v", st)
	}
	if tip := refreshTooltip(tr("刷新数据"), st); tip != "刷新中 1/3…" {
		t.Fatalf("step tooltip = %q, want %q", tip, "刷新中 1/3…")
	}

	for _, dark := range []bool{false, true} {
		dark := dark
		tt := ui.NewTester(func(c *ui.Context) {
			if dark {
				setThemeMode(themeDark)
			} else {
				setThemeMode(themeLight)
			}
			applySkin(c)
			ui.Row(c).Gap(8).Padding(12).Children(func() {
				a.refreshButton(c, store.Script{ID: "multi"}, true)
				a.refreshTextButton(c, "multi", func() {})
			})
		}, 280, 80)
		tt.Frame()
		tt.Advance(80 * time.Millisecond)
		tt.Frame()
		requireText(t, tt, "1/3")
	}
	setThemeMode(themeSystem)
}

func TestRefreshBarTracksLifecycleAndOutcome(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.closeBarInstances)
	st := a.store.Settings()
	st.Offline = false
	_ = a.store.SetSettings(st)
	a.set = st

	var fail atomic.Bool
	a.client = &http.Client{
		Transport: testRoundTrip(func(req *http.Request) (*http.Response, error) {
			if fail.Load() {
				return &http.Response{StatusCode: 503, Header: make(http.Header), Body: http.NoBody}, nil
			}
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       http.NoBody,
			}, nil
		}),
	}

	raw := `<bar protocol-version="0" id="bar-ref-test" title="BarRefTest" version="1.0.0">
  <permissions><network origin="https://example.com" /></permissions>
  <script><![CDATA[
    export default {
      async scheduled() {
        const resp = await fetch("https://example.com/ping");
        if (!resp.ok) throw new Error("HTTP " + resp.status);
      },
      render(env) {
        return { panel: h("Column", { preferredWidth: 260 }, h("Text", {}, env.refreshing ? "loading" : "ready")) };
      }
    };
  ]]></script>
</bar>`
	sc := store.Script{ID: "bar-ref-test", Name: "BarRefTest", Source: raw, Kind: store.KindBar}
	_ = a.store.SaveScript(sc)

	inst, _, err := a.ensureBarInstance(sc)
	if err != nil {
		t.Fatalf("ensureBarInstance: %v", err)
	}
	if !inst.WaitIdle(3 * time.Second) {
		t.Fatal("initial WaitIdle timed out")
	}

	a.refreshBar(sc.ID)
	if !inst.WaitIdle(3 * time.Second) {
		t.Fatal("ok refresh WaitIdle timed out")
	}
	a.onBarUpdate(sc.ID, inst.Snapshot())
	if got := a.refreshStateOf(sc.ID); got.phase != refreshOK {
		t.Fatalf("expected bar refreshOK, got %+v", got)
	}

	// Clear state and test failed refresh.
	delete(a.refreshStates, sc.ID)
	fail.Store(true)
	a.refreshBar(sc.ID)
	if !inst.WaitIdle(3 * time.Second) {
		t.Fatal("err refresh WaitIdle timed out")
	}
	a.onBarUpdate(sc.ID, inst.Snapshot())
	if got := a.refreshStateOf(sc.ID); got.phase != refreshError || !strings.Contains(got.errMsg, "HTTP 503") {
		t.Fatalf("expected bar refreshError with HTTP 503, got %+v", got)
	}
}

func TestRefreshFeedbackEnglishI18N(t *testing.T) {
	a := newTestApp(t)
	setLang(langEN)
	defer setLang(langZH)
	tt := ui.NewTester(func(c *ui.Context) {
		applySkin(c)
		ui.Column(c).Gap(8).Padding(12).Children(func() {
			a.refreshTextButton(c, "idle", func() {})
			a.refreshTextButton(c, "run-single", func() {})
			a.refreshTextButton(c, "run", func() {})
			a.refreshTextButton(c, "ok", func() {})
			a.refreshTextButton(c, "err", func() {})
		})
	}, 300, 240)

	a.beginRefresh("run-single", 1)
	a.beginRefresh("run", 2)
	a.advanceRefresh("run", 1, "")
	a.beginRefresh("ok", 1)
	a.finishRefresh("ok", true, "")
	a.beginRefresh("err", 1)
	a.finishRefresh("err", false, "boom")

	if tip := refreshTooltip(tr("刷新数据"), a.refreshStateOf("run")); tip != "Refreshing 1/2…" {
		t.Fatalf("multi-step tooltip = %q, want %q", tip, "Refreshing 1/2…")
	}

	tt.Frame()
	tt.Advance(40 * time.Millisecond)
	tt.Frame()
	requireText(t, tt, "Refresh")
	requireText(t, tt, "Refreshing…")
	requireText(t, tt, "1/2")
	requireText(t, tt, "Refreshed")
	requireText(t, tt, "Refresh failed")
	onlyEnglish(t, tt, "refresh-feedback")
}

func TestRefreshZeroLayoutShift(t *testing.T) {
	approx := func(a, b float32) bool {
		d := a - b
		if d < 0 {
			d = -d
		}
		return d <= 0.01
	}
	assertSameBounds := func(t *testing.T, label string, base, got ui.Rect) {
		t.Helper()
		if !approx(base.X, got.X) || !approx(base.Y, got.Y) || !approx(base.W, got.W) || !approx(base.H, got.H) {
			t.Fatalf("%s layout shifted: want %+v, got %+v", label, base, got)
		}
	}

	// 1. Full-view action bar invariance across all phases, languages, and skins.
	defer setLang(langZH)
	defer setSkin(skinDefault)
	for _, lang := range []string{langZH, langEN} {
		for _, sk := range []string{skinDefault, skinMac9} {
			lang, sk := lang, sk
			a := newTestApp(t)
			setLang(lang)
			setSkin(sk)
			tt := ui.NewTester(func(c *ui.Context) {
				applySkin(c)
				ui.Row(c).Padding(12).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, tr("大图")).FontSize(fsCaption).NoWrap()
					a.refreshTextButton(c, "btn", func() {})
					btn(c, tr("编辑"))
				})
			}, 420, 80)

			tt.Frame()
			bigInfo, ok := tt.Find(tr("大图"))
			if !ok {
				t.Fatalf("%s/%s: missing 大图", lang, sk)
			}
			refInfo, ok := tt.Access(tr("刷新数据"))
			if !ok {
				t.Fatalf("%s/%s: missing 刷新数据 button", lang, sk)
			}
			editInfo, ok := tt.Find(tr("编辑"))
			if !ok {
				t.Fatalf("%s/%s: missing 编辑", lang, sk)
			}

			applyPhase := func(step int) {
				switch step {
				case 0: // running (1 step)
					a.beginRefresh("btn", 1)
				case 1: // running (1/3)
					a.beginRefresh("btn", 3)
					a.advanceRefresh("btn", 1, "")
				case 2: // ok
					a.finishRefresh("btn", true, "")
				case 3: // error
					a.finishRefresh("btn", false, "network down")
				case 4: // back to idle
					delete(a.refreshStates, "btn")
				}
				tt.Frame()
				tt.Advance(40 * time.Millisecond)
				tt.Frame()
			}

			for step := 0; step < 5; step++ {
				applyPhase(step)
				gBig, ok := tt.Find(tr("大图"))
				if !ok {
					t.Fatalf("%s/%s step %d: missing 大图", lang, sk, step)
				}
				gRef, ok := tt.Access(tr("刷新数据"))
				if !ok {
					t.Fatalf("%s/%s step %d: missing 刷新数据", lang, sk, step)
				}
				gEdit, ok := tt.Find(tr("编辑"))
				if !ok {
					t.Fatalf("%s/%s step %d: missing 编辑", lang, sk, step)
				}
				assertSameBounds(t, fmt.Sprintf("%s/%s/step%d 大图", lang, sk, step), bigInfo, gBig)
				assertSameBounds(t, fmt.Sprintf("%s/%s/step%d 刷新数据", lang, sk, step), refInfo.Bounds, gRef.Bounds)
				assertSameBounds(t, fmt.Sprintf("%s/%s/step%d 编辑", lang, sk, step), editInfo, gEdit)
			}
		}
	}

	// 2. Home card internal stability: no transient "更新中…" row pushing content down.
	setLang(langZH)
	setSkin(skinDefault)
	a := newTestApp(t)
	sc := store.Script{
		ID: "zero-shift-home", Name: "稳定性", Home: true, Origin: store.OriginUser,
		Source: `export function meta() { return { name: "稳定性", sources: {} }; }
export function render() {
  return {
    title: "稳定性",
    big: "98.6%",
    sub: "副标题行",
    stats: [{ label: "延迟", value: "14ms" }]
  };
}`,
	}
	if err := a.store.SaveScript(sc); err != nil {
		t.Fatal(err)
	}
	showOnly(a, sc.ID)

	tt := ui.NewTester(a.view, 1100, 900)
	settle(tt)

	refBase, ok := tt.Access("刷新数据")
	if !ok {
		t.Fatal("home card missing 刷新数据")
	}
	statBase, ok := tt.Find("延迟")
	if !ok {
		t.Fatal("home card missing stat 延迟")
	}

	meta, _ := a.metaOf(sc)
	_, vals := a.viewOf(sc.ID, meta.Dims)
	runKey := a.runKeyOf(sc.ID, sc, meta, vals)

	// Enter refreshing + updating state.
	a.beginRefresh(sc.ID, 2)
	a.advanceRefresh(sc.ID, 1, "")
	if rs := a.runs[runKey]; rs != nil {
		rs.old = true
		rs.updating = true
	}
	tt.Frame()
	tt.Advance(40 * time.Millisecond)
	tt.Frame()

	refRun, ok1 := tt.Access("刷新数据")
	statRun, ok2 := tt.Find("延迟")
	if !ok1 || !ok2 {
		t.Fatalf("home running missing elements: ref=%v stat=%v", ok1, ok2)
	}
	assertSameBounds(t, "home running ref", refBase.Bounds, refRun.Bounds)
	assertSameBounds(t, "home running stat", statBase, statRun)

	// Finish OK.
	if rs := a.runs[runKey]; rs != nil {
		rs.old = false
		rs.updating = false
	}
	a.finishRefresh(sc.ID, true, "")
	tt.Frame()
	tt.Advance(40 * time.Millisecond)
	tt.Frame()

	refOK, ok1 := tt.Access("刷新数据")
	statOK, ok2 := tt.Find("延迟")
	if !ok1 || !ok2 {
		t.Fatalf("home ok missing elements: ref=%v stat=%v", ok1, ok2)
	}
	assertSameBounds(t, "home ok ref", refBase.Bounds, refOK.Bounds)
	assertSameBounds(t, "home ok stat", statBase, statOK)

	// Finish Error.
	a.finishRefresh(sc.ID, false, "timeout")
	tt.Frame()
	tt.Advance(40 * time.Millisecond)
	tt.Frame()

	refErr, ok1 := tt.Access("刷新数据")
	statErr, ok2 := tt.Find("延迟")
	if !ok1 || !ok2 {
		t.Fatalf("home error missing elements: ref=%v stat=%v", ok1, ok2)
	}
	assertSameBounds(t, "home error ref", refBase.Bounds, refErr.Bounds)
	assertSameBounds(t, "home error stat", statBase, statErr)
}
