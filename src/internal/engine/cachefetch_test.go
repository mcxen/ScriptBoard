package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// memCache is a SourceCache in memory, with its history.
type memCache struct {
	mu   sync.Mutex
	last map[string]CachedBody
	hist map[string][]CachedBody
}

func newMemCache() *memCache {
	return &memCache{last: map[string]CachedBody{}, hist: map[string][]CachedBody{}}
}

func (m *memCache) Get(url string) (CachedBody, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.last[url]
	return b, ok
}

func (m *memCache) Put(url string, body []byte, at time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e := CachedBody{At: at, Body: append([]byte(nil), body...)}
	m.last[url] = e
	m.hist[url] = append(m.hist[url], e)
}

func (m *memCache) History(url string, limit int) []CachedBody {
	m.mu.Lock()
	defer m.mu.Unlock()
	h := m.hist[url]
	if limit > 0 && len(h) > limit {
		h = h[len(h)-limit:]
	}
	return h
}

// server answers {"p": n} with a count of requests, or fails when told to.
type server struct {
	*httptest.Server
	hits atomic.Int32
	fail atomic.Bool
}

func newServer() *server {
	s := &server{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := s.hits.Add(1)
		if s.fail.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintf(w, `{"p": %d}`, n)
	}))
	return s
}

var now = time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC)

func price(t *testing.T, body json.RawMessage) float64 {
	t.Helper()
	var v struct {
		P float64 `json:"p"`
		E string  `json:"error"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatal(err)
	}
	if v.E != "" {
		t.Fatalf("source error %q", v.E)
	}
	return v.P
}

// A response is fetched once, cached, and served from the cache while it is
// younger than the TTL; a forced fetch asks again.
func TestFetchCachedUsesTheTTL(t *testing.T) {
	srv := newServer()
	defer srv.Close()
	src := map[string]string{"a": srv.URL}
	c := newMemCache()
	opt := FetchOptions{Cache: c, TTL: time.Minute}

	body, info := FetchCached(context.Background(), srv.Client(), src, now, opt)
	if price(t, body["a"]) != 1 || info["a"].Cached || info["a"].Stale {
		t.Fatalf("first fetch: %s %+v", body["a"], info["a"])
	}
	body, info = FetchCached(context.Background(), srv.Client(), src, now.Add(20*time.Second), opt)
	if price(t, body["a"]) != 1 || !info["a"].Cached || info["a"].Stale || srv.hits.Load() != 1 {
		t.Fatalf("within the TTL: %s %+v hits %d", body["a"], info["a"], srv.hits.Load())
	}
	body, _ = FetchCached(context.Background(), srv.Client(), src, now.Add(2*time.Minute), opt)
	if price(t, body["a"]) != 2 || srv.hits.Load() != 2 {
		t.Fatalf("after the TTL: %s hits %d", body["a"], srv.hits.Load())
	}
	forced := opt
	forced.Force = true
	body, _ = FetchCached(context.Background(), srv.Client(), src, now.Add(2*time.Minute+time.Second), forced)
	if price(t, body["a"]) != 3 || srv.hits.Load() != 3 {
		t.Fatalf("forced: %s hits %d", body["a"], srv.hits.Load())
	}
}

// When a request fails, the cached response stands in, marked stale, with the
// error; with nothing cached the source has the error, as before.
func TestFetchCachedFallsBackWhenTheRequestFails(t *testing.T) {
	srv := newServer()
	defer srv.Close()
	src := map[string]string{"a": srv.URL}
	c := newMemCache()
	opt := FetchOptions{Cache: c} // no TTL: always ask

	srv.fail.Store(true)
	body, info := FetchCached(context.Background(), srv.Client(), src, now, opt)
	var e struct{ Error string }
	json.Unmarshal(body["a"], &e)
	if e.Error == "" || info["a"].Error == "" || info["a"].Cached {
		t.Fatalf("down with nothing cached: %s %+v", body["a"], info["a"])
	}

	srv.fail.Store(false)
	FetchCached(context.Background(), srv.Client(), src, now, opt) // caches {"p": 2}
	srv.fail.Store(true)
	body, info = FetchCached(context.Background(), srv.Client(), src, now.Add(time.Hour), opt)
	if price(t, body["a"]) != 2 || !info["a"].Cached || !info["a"].Stale || info["a"].Error == "" {
		t.Fatalf("down with a cached copy: %s %+v", body["a"], info["a"])
	}
	if info["a"].At != now.UnixMilli() {
		t.Errorf("the data's time is %d, want when it was fetched (%d)", info["a"].At, now.UnixMilli())
	}
}

// Offline makes no request: the cache answers, marked stale, or there is an error.
func TestFetchCachedOffline(t *testing.T) {
	srv := newServer()
	defer srv.Close()
	src := map[string]string{"a": srv.URL, "b": srv.URL + "/other"}
	c := newMemCache()
	FetchCached(context.Background(), srv.Client(), map[string]string{"a": srv.URL}, now, FetchOptions{Cache: c})
	before := srv.hits.Load()

	body, info := FetchCached(context.Background(), srv.Client(), src, now.Add(time.Hour), FetchOptions{Cache: c, Offline: true})
	if srv.hits.Load() != before {
		t.Errorf("a request was made offline")
	}
	if price(t, body["a"]) != 1 || !info["a"].Cached || !info["a"].Stale {
		t.Errorf("cached source offline: %s %+v", body["a"], info["a"])
	}
	var e struct{ Error string }
	json.Unmarshal(body["b"], &e)
	if e.Error == "" || info["b"].Error == "" {
		t.Errorf("uncached source offline: %s %+v", body["b"], info["b"])
	}
}

// A response that is not JSON is a failure, and does not replace the cache.
func TestFetchCachedKeepsTheCacheOnBadResponses(t *testing.T) {
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "<html>maintenance</html>") }))
	defer bad.Close()
	c := newMemCache()
	c.Put(bad.URL, []byte(`{"p": 7}`), now)
	body, info := FetchCached(context.Background(), bad.Client(), map[string]string{"a": bad.URL}, now.Add(time.Hour), FetchOptions{Cache: c})
	if price(t, body["a"]) != 7 || !info["a"].Stale {
		t.Fatalf("%s %+v", body["a"], info["a"])
	}
	if got, _ := c.Get(bad.URL); string(got.Body) != `{"p": 7}` {
		t.Errorf("the cache was replaced by %s", got.Body)
	}
}

// A script asks for the past responses of a source and gets them, oldest first.
func TestHistoryOf(t *testing.T) {
	c := newMemCache()
	u := "https://a.example/p"
	for i := 1; i <= 4; i++ {
		c.Put(u, []byte(fmt.Sprintf(`{"p": %d}`, i)), now.Add(time.Duration(i)*time.Hour))
	}
	h := HistoryOf(c, map[string]string{"a": u, "b": "https://b.example"}, []string{"a", "zzz"}, 3, now)
	if len(h) != 1 || len(h["a"]) != 3 {
		t.Fatalf("history %+v", h)
	}
	if price(t, h["a"][0].V) != 2 || price(t, h["a"][2].V) != 4 || h["a"][0].T >= h["a"][2].T {
		t.Errorf("not the newest three, oldest first: %+v", h["a"])
	}
	if len(HistoryOf(nil, nil, []string{"a"}, 3, now)) != 0 {
		t.Errorf("history without a cache")
	}
}

// A script sees where its data came from and the history it asked for.
func TestScriptSeesSourceInfoAndHistory(t *testing.T) {
	src := `export function meta() { return { name: "h", sources: { a: "https://a.example/p" }, history: ["a"] }; }
export function render(ctx) {
  const i = ctx.sourceInfo.a, h = ctx.history.a;
  return { title: "x", big: String(h.length) + "/" + i.cached + "/" + i.stale + "/" + h[h.length - 1].v.p, sub: "", lines: [], chart: null };
}`
	s, err := Compile("h.js", src)
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Run(context.Background(), Input{
		Now:        now,
		Sources:    map[string]json.RawMessage{"a": json.RawMessage(`{"p": 9}`)},
		SourceInfo: map[string]SourceInfo{"a": {At: now.UnixMilli(), Cached: true, Stale: true}},
		History: map[string][]HistoryPoint{"a": {
			{T: 1, V: json.RawMessage(`{"p": 8}`)}, {T: 2, V: json.RawMessage(`{"p": 9}`)}}},
	})
	if err != nil || res.Big != "2/true/true/9" {
		t.Fatalf("big %q err %v", res.Big, err)
	}
	// A script that does not ask still runs: the objects are there, empty.
	s2, _ := Compile("n.js", `export function meta() { return { name: "n", sources: {} }; }
export function render(ctx) { return { title: "x", big: Object.keys(ctx.history).length + "," + Object.keys(ctx.sourceInfo).length, sub: "", lines: [], chart: null }; }`)
	if res, err := s2.Run(context.Background(), Input{Now: now}); err != nil || res.Big != "0,0" {
		t.Errorf("empty objects: %q %v", res.Big, err)
	}
}

// A source that needs the user's login gets the headers its script names, and
// the script's meta() carries them.
func TestSourceHeadersCarryACookie(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Cookie")
		fmt.Fprint(w, `{"p": 1}`)
	}))
	defer srv.Close()

	src := `export function meta() { return { name: "登录", sources: { api: "` + srv.URL + `" }, sourceHeaders: { api: { Cookie: "session=abc" } } }; }
export function render(ctx) { return { title: "x", big: "1", sub: "", lines: [], chart: null }; }`
	s, err := Compile("login.js", src)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Meta.SourceHeaders["api"]["Cookie"]; got != "session=abc" {
		t.Fatalf("meta lost the header: %+v", s.Meta.SourceHeaders)
	}
	body, info := FetchCached(context.Background(), srv.Client(), s.Meta.Sources, now,
		FetchOptions{Headers: s.Meta.SourceHeaders})
	if seen != "session=abc" {
		t.Fatalf("server got Cookie %q", seen)
	}
	if price(t, body["api"]) != 1 || info["api"].Error != "" {
		t.Fatalf("%s %+v", body["api"], info["api"])
	}
}
