package main

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/feed"
	"scriptboard/src/internal/store"
)

// clockFetcher answers like a price API whose numbers follow the test clock:
// a price and a volume, in an items array.
type clockFetcher struct {
	now  func() time.Time
	base time.Time
}

func (f clockFetcher) Fetch(ctx context.Context, r feed.Request) ([]byte, error) {
	h := f.now().Sub(f.base).Hours()
	price := 630 + 5*math.Sin(h/3) + 0.05*h
	volume := 100 + 3*math.Cos(h)
	return []byte(fmt.Sprintf(`{"data":{"items":[{"v":%.2f},{"v":%.0f}]}}`, price, volume)), nil
}

const liveScript = `export function meta() {
  return {
    name: "实时金价",
    icon: "coins",
    feeds: ["gold-live"],
    dims: [{ key: "bucket", label: "粒度", options: [{ value: 1, label: "小时" }, { value: 24, label: "天" }] }],
    sources: {},
  };
}
export function render(ctx) {
  const f = ctx.feeds["gold-live"];
  if (f.error) return { title: "数据源", big: "--", sub: f.error, lines: [], chart: null };
  const daily = ctx.view.bucket === 24;
  const b = daily ? f.day : f.hour;
  const samples = b.counts.reduce((x, y) => x + y, 0);
  return {
    title: "金价 · " + (daily ? "按天" : "按小时"),
    icon: "coins",
    big: f.latest[0].toFixed(2),
    sub: "样本 " + samples + " 个，" + b.labels.length + " 个" + (daily ? "天" : "小时"),
    lines: [f.names.join(" / ")],
    chart: { type: "line", series: b.series, labels: b.labels, xLabels: [b.labels[0], b.labels[b.labels.length - 1]] },
  };
}`

// A source polled every 20 minutes for two days, read by a script at hour and
// at day granularity, with two numbers per sample.
func TestFeedSourceDrawsHourlyAndDailyCurves(t *testing.T) {
	a := newTestApp(t)
	base := time.Date(2026, 11, 16, 0, 0, 0, 0, time.Local)
	clock := base
	a.now = func() time.Time { return clock }
	a.runner.Now = func() time.Time { return clock }
	a.runner.Fetcher = clockFetcher{now: func() time.Time { return clock }, base: base}

	src := feed.Source{
		ID: "gold-live", Name: "金价实时", Every: 60,
		Request: feed.Request{URL: "https://api.example.com/price", Cookies: map[string]string{"session": "abc"}},
		Extract: feed.Extract{Kind: feed.KindJSON, Path: "data.items.#.v"},
		Names:   []string{"金价", "成交量"},
		Agg:     "avg",
	}
	if err := a.store.SaveFeed(src); err != nil {
		t.Fatal(err)
	}
	for ; clock.Before(base.Add(48 * time.Hour)); clock = clock.Add(20 * time.Minute) {
		if _, err := a.runner.Poll(context.Background(), src); err != nil {
			t.Fatal(err)
		}
	}
	clock = base.Add(48 * time.Hour)

	sc := store.Script{ID: "live-gold", Name: "实时金价", Source: liveScript, Home: true}
	if err := a.store.SaveScript(sc); err != nil {
		t.Fatal(err)
	}

	hourly, err := a.execute(sc, map[string]float64{"bucket": 1})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(hourly.Chart.Labels); got != 48 {
		t.Errorf("%d hourly buckets, want 48", got)
	}
	if len(hourly.Chart.Series) != 2 || hourly.Chart.Series[0].Name != "金价" || hourly.Chart.Series[1].Name != "成交量" {
		t.Errorf("series %+v, want the two named numbers", hourly.Chart.Series)
	}
	if !strings.Contains(hourly.Sub, "样本 144") {
		t.Errorf("sub %q, want all 144 samples counted", hourly.Sub)
	}

	daily, err := a.execute(sc, map[string]float64{"bucket": 24})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(daily.Chart.Labels); got != 2 {
		t.Errorf("%d daily buckets, want 2", got)
	}
	if daily.Title != "金价 · 按天" {
		t.Errorf("title %q", daily.Title)
	}

	// The same thing on the screen: the feeds page and the full view of the script.
	tt := ui.NewTester(a.view, 960, 900)
	a.router.Push("/feeds")
	settle(tt)
	requireText(t, tt, "金价实时")
	savePNG(t, tt, "8-feeds-page")

	a.router.Push("/full/live-gold")
	settle(tt)
	requireText(t, tt, "金价 · 按小时")
	savePNG(t, tt, "8-feed-hourly")
	click(t, tt, "天")
	requireText(t, tt, "金价 · 按天")
	savePNG(t, tt, "8-feed-daily")
}

// Adding a source from a curl command on the page: the cookie goes with the
// request, the first reading is taken, and the source can be deleted.
func TestAddFeedFromCurl(t *testing.T) {
	a := newTestApp(t)
	// The test app is offline; this test really makes requests, to a local server.
	a.set.Offline = false
	if err := a.store.SetSettings(a.set); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("session")
		if err != nil || c.Value != "s3cret" {
			http.Error(w, "login required", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"price":3210.5}}`))
	}))
	defer srv.Close()

	a.feedForm = feedForm{
		name: "测试价格",
		curl: "curl " + srv.URL + "/price -b 'session=s3cret'",
		rule: "data.price",
		id:   "test-price",
	}
	a.addFeed()
	if a.feedForm.note == "" || strings.Contains(a.feedForm.note, "失败") {
		t.Fatalf("note %q", a.feedForm.note)
	}
	src, ok := a.store.Feed("test-price")
	if !ok {
		t.Fatal("source not stored")
	}
	if src.Request.Cookies["session"] != "s3cret" || src.Every != 60 {
		t.Errorf("stored source %+v", src)
	}
	if latest, ok, _ := a.samples.Latest("test-price"); !ok || latest.Values[0] != 3210.5 {
		t.Errorf("first reading %v %v", latest, ok)
	}

	a.feedForm = feedForm{name: "没有 cookie", curl: "curl " + srv.URL + "/price", rule: "data.price", id: "no-cookie"}
	a.addFeed()
	if _, ok := a.store.Feed("no-cookie"); !ok {
		t.Fatal("the source is stored even when its first reading fails")
	}
	if a.runner.LastError("no-cookie") == "" {
		t.Error("the failing first reading was not remembered")
	}

	a.deleteFeed("test-price")
	if _, ok := a.store.Feed("test-price"); ok {
		t.Error("deleted source still stored")
	}
}

func TestSlugOf(t *testing.T) {
	cases := map[string]string{"金价 实时": "", "Gold Price (AU)": "gold-price-au", "  X  ": ""}
	for in, want := range cases {
		got := slugOf(in)
		if want != "" && got != want {
			t.Errorf("slugOf(%q) = %q, want %q", in, got, want)
		}
		if !feed.ValidID(got) {
			t.Errorf("slugOf(%q) = %q is not a valid id", in, got)
		}
	}
}

func TestRequestTextRoundTrip(t *testing.T) {
	reqs := []feed.Request{
		{Method: "GET", URL: "https://example.com/p?a=1"},
		{Method: "POST", URL: "https://example.com/p", Headers: map[string]string{"X-Key": "it's"}, Cookies: map[string]string{"a": "1", "b": "2"}, Body: `{"q":1}`},
		{URL: "tcp://127.0.0.1:9000"},
	}
	for _, r := range reqs {
		got, err := feed.ParseRequest(requestText(r))
		if err != nil {
			t.Fatalf("%q: %v", requestText(r), err)
		}
		if got.URL != r.URL || got.Body != r.Body || got.Cookies["b"] != r.Cookies["b"] || got.Headers["X-Key"] != r.Headers["X-Key"] {
			t.Errorf("%q gave %+v, want %+v", requestText(r), got, r)
		}
	}
}

func TestEditAndTestFeed(t *testing.T) {
	a := newTestApp(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"price":12.5}}`))
	}))
	defer srv.Close()

	a.set.Offline = false
	a.feedForm = feedForm{open: true, name: "编辑测试", curl: srv.URL, rule: "data.price", id: "edit-me", names: "价"}
	a.testFeed()
	if !strings.Contains(a.feedForm.note, "价 = 12.5") {
		t.Fatalf("test note %q", a.feedForm.note)
	}
	if _, ok := a.store.Feed("edit-me"); ok {
		t.Fatal("a test must not store the source")
	}
	a.addFeed()
	src, ok := a.store.Feed("edit-me")
	if !ok || a.feedForm.open {
		t.Fatalf("stored %v, form open %v", ok, a.feedForm.open)
	}
	src.Timeout = 30
	if err := a.store.SaveFeed(src); err != nil {
		t.Fatal(err)
	}

	a.editFeed(src)
	if a.feedForm.curl != srv.URL || a.feedForm.rule != "data.price" || a.feedForm.editing != "edit-me" {
		t.Fatalf("form %+v", a.feedForm)
	}
	a.feedForm.name, a.feedForm.every = "改过的名字", 1
	a.addFeed()
	got, _ := a.store.Feed("edit-me")
	if got.Name != "改过的名字" || got.Every != 300 || got.Timeout != 30 {
		t.Errorf("edited source %+v", got)
	}
	if n := len(a.store.Feeds()); n != 1 {
		t.Errorf("%d sources after an edit, want 1", n)
	}
}
