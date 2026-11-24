package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// A source is an address as a string, or an object: both read, and the object
// keeps its method, headers and body.
func TestSourceAcceptsStringAndObject(t *testing.T) {
	src := `export function meta() { return { name: "x", sources: {
  a: "https://example.com/a",
  b: { url: "https://example.com/b", method: "post", headers: { Cookie: "s=1" }, body: '{"d":"{start}"}' } } }; }
export function render(ctx) { return { title: "x", big: "1", sub: "", lines: [], chart: null }; }`
	s, err := Compile("x.js", src)
	if err != nil {
		t.Fatal(err)
	}
	if s.Meta.Sources["a"].URL != "https://example.com/a" || s.Meta.Sources["a"].Method != "" {
		t.Errorf("string source: %+v", s.Meta.Sources["a"])
	}
	b := s.Meta.Sources["b"]
	if b.URL != "https://example.com/b" || b.Method != "post" || b.Headers["Cookie"] != "s=1" || !strings.Contains(b.Body, "{start}") {
		t.Errorf("object source: %+v", b)
	}
	r := b.Request(time.Date(2026, 11, 19, 0, 0, 0, 0, time.UTC), nil)
	if r.Method != "POST" || !strings.Contains(r.Body, "2026-10-20") {
		t.Errorf("request %+v", r)
	}
}

// A POST goes out with its method, body and headers, and its response is kept
// under a key of its own, apart from a GET of the same address.
func TestPostSendsItsBody(t *testing.T) {
	var gotMethod, gotBody, gotCookie string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotCookie = r.Header.Get("Cookie")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		fmt.Fprint(w, `{"ok": true}`)
	}))
	defer srv.Close()
	src := Source{URL: srv.URL, Method: "POST", Body: `{"q":1}`, Headers: map[string]string{"Cookie": "s=9"}}
	body, info := FetchRequests(context.Background(), srv.Client(), map[string]Request{"p": src.Request(time.Now(), nil)}, time.Now(), FetchOptions{})
	if gotMethod != "POST" || gotBody != `{"q":1}` || gotCookie != "s=9" {
		t.Errorf("server saw %s %q cookie %q", gotMethod, gotBody, gotCookie)
	}
	if info["p"].Error != "" || !strings.Contains(string(body["p"]), "ok") {
		t.Errorf("response %s %+v", body["p"], info["p"])
	}
	get := Request{Method: "GET", URL: srv.URL}
	if get.CacheKey() == (Request{Method: "POST", URL: srv.URL, Body: `{"q":1}`}).CacheKey() {
		t.Error("a POST and a GET of one address share a cache key")
	}
}

// Query parameters join the address, and a query on an address that already
// has one keeps it.
func TestQueryJoinsTheAddress(t *testing.T) {
	now := time.Now()
	r := Source{URL: "https://example.com/x?a=1", Query: map[string]string{"b": "2", "c": "{start}"}}.Request(now, nil)
	if !strings.HasPrefix(r.URL, "https://example.com/x?a=1&") || !strings.Contains(r.URL, "b=2") {
		t.Errorf("url %s", r.URL)
	}
	if strings.Contains(r.URL, "{start}") {
		t.Errorf("a query value was not expanded: %s", r.URL)
	}
}

// Paging reads the pages in order, joins their lists, and stops at a page with
// no items.
func TestPagingJoinsPages(t *testing.T) {
	var pages []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Query().Get("page")
		pages = append(pages, p)
		switch p {
		case "1":
			fmt.Fprint(w, `{"data":{"list":[{"v":1},{"v":2}]}}`)
		case "2":
			fmt.Fprint(w, `{"data":{"list":[{"v":3}]}}`)
		default:
			fmt.Fprint(w, `{"data":{"list":[]}}`)
		}
	}))
	defer srv.Close()
	src := Source{URL: srv.URL, Paging: &Paging{Param: "page", Start: 1, Pages: 5, Items: "data.list"}}
	body, info := FetchRequests(context.Background(), srv.Client(), map[string]Request{"l": src.Request(time.Now(), nil)}, time.Now(), FetchOptions{})
	if info["l"].Error != "" {
		t.Fatal(info["l"].Error)
	}
	var items []map[string]int
	if err := json.Unmarshal(body["l"], &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || items[2]["v"] != 3 {
		t.Errorf("joined items %v", items)
	}
	if strings.Join(pages, ",") != "1,2,3" {
		t.Errorf("pages asked %v, want 1,2,3 (the third is empty and stops it)", pages)
	}
}

// A page that fails after the first keeps what the earlier pages gave.
func TestPagingKeepsWhatItHas(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "1" {
			fmt.Fprint(w, `[{"v":1}]`)
			return
		}
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	src := Source{URL: srv.URL, Paging: &Paging{Param: "page", Pages: 3}}
	body, info := FetchRequests(context.Background(), srv.Client(), map[string]Request{"l": src.Request(time.Now(), nil)}, time.Now(), FetchOptions{})
	if info["l"].Error != "" || string(body["l"]) != `[{"v":1}]` {
		t.Errorf("body %s info %+v", body["l"], info["l"])
	}
}

// A source with fallbacks tries them when the main address fails, returns
// non-JSON, or returns an API error wrapper, and caches the working response
// under the source's key. Both object fallbacks and array shorthand work.
func TestSourceFallbacks(t *testing.T) {
	badHTML := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<script src="/_guard/html.js"></script>`)
	}))
	defer badHTML.Close()
	badAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"code":500,"msg":"upstream busy"}`)
	}))
	defer badAPI.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		fmt.Fprintf(w, `{"city":%q,"method":%q,"hdr":%q,"body":%q,"p":42}`,
			r.URL.Query().Get("city"), r.Method, r.Header.Get("X-Token"), string(b))
	}))
	defer good.Close()

	// 1. Object with string fallback (inherits query & headers) and object fallback (own method/body).
	js := `export function meta() {
  return {
    name: "fb",
    sources: {
      a: {
        url: "` + badHTML.URL + `",
        headers: { "X-Token": "tok" },
        query: { city: "{city}" },
        fallbacks: [
          "` + badAPI.URL + `",
          { url: "` + good.URL + `", method: "POST", query: { city: "{city}" }, body: "hi-{city}" }
        ]
      },
      b: ["` + badHTML.URL + `?city={city}", "` + good.URL + `?city={city}"]
    }
  };
}
export function render(ctx) {
  return { title: "fb", big: ctx.sources.a.city + ":" + ctx.sourceInfo.a.used + ":" + ctx.sourceInfo.b.used, sub: ctx.sources.a.body, lines: [], chart: null };
}`
	s, err := Compile("fb.js", js)
	if err != nil {
		t.Fatal(err)
	}
	vals := map[string]string{"city": "shanghai"}
	reqs := map[string]Request{
		"a": s.Meta.Sources["a"].Request(time.Now(), vals),
		"b": s.Meta.Sources["b"].Request(time.Now(), vals),
	}
	c := newMemCache()
	bodies, info := FetchRequests(context.Background(), good.Client(), reqs, time.Now(), FetchOptions{Cache: c})
	if info["a"].Error != "" || info["a"].Stale || info["a"].Used != 2 {
		t.Fatalf("source a info: %+v", info["a"])
	}
	if info["b"].Error != "" || info["b"].Stale || info["b"].Used != 1 {
		t.Fatalf("source b info: %+v", info["b"])
	}
	if got, ok := c.Get(reqs["a"].CacheKey()); !ok || !strings.Contains(string(got.Body), "42") {
		t.Errorf("fallback response not cached under main key: %+v %v", got, ok)
	}
	res, err := s.Run(context.Background(), Input{Now: time.Now(), Sources: bodies, SourceInfo: info, Vars: vals})
	if err != nil || res.Big != "shanghai:2:1" || res.Sub != "hi-shanghai" {
		t.Errorf("render got big=%q sub=%q err=%v", res.Big, res.Sub, err)
	}
}

func TestRequestCacheKeyIncludesHeadersAndKeepsPlainGET(t *testing.T) {
	plain := Request{Method: "GET", URL: "https://api.example.com/me"}
	if got := plain.CacheKey(); got != "https://api.example.com/me" {
		t.Fatalf("plain GET CacheKey = %q, want %q", got, plain.URL)
	}

	r1 := Request{
		Method:  "GET",
		URL:     "https://api.example.com/me",
		Headers: map[string]string{"Cookie": "sid=secret-user-one", "X-Env": "prod"},
	}
	r2 := Request{
		Method:  "GET",
		URL:     "https://api.example.com/me",
		Headers: map[string]string{"Cookie": "sid=secret-user-two", "X-Env": "prod"},
	}
	k1, k2 := r1.CacheKey(), r2.CacheKey()
	if k1 == plain.CacheKey() || k2 == plain.CacheKey() || k1 == k2 {
		t.Fatalf("expected distinct header-aware cache keys: plain=%q k1=%q k2=%q", plain.CacheKey(), k1, k2)
	}
	if strings.Contains(k1, "secret-user-one") || strings.Contains(k2, "secret-user-two") {
		t.Fatalf("CacheKey must hash header secrets rather than embed raw values: k1=%q k2=%q", k1, k2)
	}
}
