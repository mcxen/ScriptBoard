package feed

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseCurlGetWithCookies(t *testing.T) {
	cmd := `curl 'https://api.example.com/v1/price?sym=AU' -H 'accept: application/json' -b 'session=abc; theme=dark' -X GET --compressed -s`
	r, err := ParseCurl(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if r.URL != "https://api.example.com/v1/price?sym=AU" || r.Method != "GET" {
		t.Errorf("url %q method %q", r.URL, r.Method)
	}
	if r.Headers["accept"] != "application/json" {
		t.Errorf("headers %v", r.Headers)
	}
	if r.Cookies["session"] != "abc" || r.Cookies["theme"] != "dark" {
		t.Errorf("cookies %v", r.Cookies)
	}
}

func TestParseCurlPostDataAndLineContinuation(t *testing.T) {
	cmd := "curl -X POST https://api.example.com/q \\\n  -H \"Content-Type: application/json\" \\\n  --data-raw '{\"ask\":\"price\"}' \\\n  -H 'Cookie: token=xyz'"
	r, err := ParseCurl(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if r.Method != "POST" || r.Body != `{"ask":"price"}` {
		t.Errorf("method %q body %q", r.Method, r.Body)
	}
	if r.Cookies["token"] != "xyz" {
		t.Errorf("a Cookie header should become a cookie: %v", r.Cookies)
	}
	if _, ok := r.Headers["Cookie"]; ok {
		t.Error("the Cookie header is still in the headers")
	}
	if r.Headers["Content-Type"] != "application/json" {
		t.Errorf("headers %v", r.Headers)
	}
}

func TestParseCurlDataImpliesPost(t *testing.T) {
	r, err := ParseCurl(`curl https://x.example.com/a -d a=1 -d b=2`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Method != "POST" || r.Body != "a=1&b=2" {
		t.Errorf("method %q body %q", r.Method, r.Body)
	}
}

func TestParseCurlRefusesWhatItCannotReproduce(t *testing.T) {
	for _, cmd := range []string{
		`curl https://x.example.com -d @body.json`,
		`curl https://x.example.com -b cookies.txt`,
		`curl https://x.example.com --proxy http://p:8080`,
		`curl -H 'no-colon-here' https://x.example.com`,
		`curl 'https://x.example.com`,
		`curl`,
	} {
		if _, err := ParseCurl(cmd); err == nil {
			t.Errorf("accepted %q", cmd)
		}
	}
}

func TestSourceValidation(t *testing.T) {
	good := Source{
		ID: "gold-now", Name: "金价", Every: 60,
		Request: Request{URL: "https://api.example.com/p"},
		Extract: Extract{Kind: KindJSON, Path: "data.price"},
	}
	if err := good.Validate(); err != nil {
		t.Fatalf("good source refused: %v", err)
	}
	bad := map[string]func(*Source){
		"id":       func(s *Source) { s.ID = "Bad_ID" },
		"interval": func(s *Source) { s.Every = 30 },
		"scheme":   func(s *Source) { s.Request.URL = "gopher://example.com/x" },
		"method":   func(s *Source) { s.Request.Method = "TRACE" },
		"path":     func(s *Source) { s.Extract.Path = "" },
		"regex":    func(s *Source) { s.Extract = Extract{Kind: KindRegex, Pattern: "("} },
		"agg":      func(s *Source) { s.Agg = "median" },
	}
	for name, mutate := range bad {
		s := good
		mutate(&s)
		if err := s.Validate(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestExtractJSONAndRegex(t *testing.T) {
	body := []byte(`{"data":{"price":3210.5,"items":[{"v":"1,234.5"},{"v":2},{"x":1}]}}`)
	n, err := Extract{Kind: KindJSON, Path: "data.price"}.Numbers(body)
	if err != nil || len(n) != 1 || n[0] != 3210.5 {
		t.Fatalf("price: %v %v", n, err)
	}
	n, err = Extract{Kind: KindJSON, Path: "data.items.#.v"}.Numbers(body)
	if err != nil || len(n) != 2 || n[0] != 1234.5 || n[1] != 2 {
		t.Fatalf("items: %v %v", n, err)
	}
	if _, err := (Extract{Kind: KindJSON, Path: "data.missing"}).Numbers(body); err == nil {
		t.Error("a missing path should be an error")
	}

	page := []byte("当前 3,210.5 元，涨幅 4.5%，成交 12 手")
	n, err = Extract{Kind: KindRegex, Pattern: `当前 ([0-9,]+\.?[0-9]*) 元`}.Numbers(page)
	if err != nil || len(n) != 1 || n[0] != 3210.5 {
		t.Fatalf("regex group: %v %v", n, err)
	}
	n, err = Extract{Kind: KindRegex, Pattern: `[0-9][0-9,]*(?:\.[0-9]+)?`}.Numbers(page)
	if err != nil || len(n) != 3 || n[0] != 3210.5 || n[2] != 12 {
		t.Fatalf("regex whole match: %v %v", n, err)
	}
	if _, err := (Extract{Kind: KindRegex, Pattern: `zzz(\d+)`}).Numbers(page); err == nil {
		t.Error("no match should be an error")
	}
}

func TestAggregateHoursAndDays(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*3600)
	base := time.Date(2026, 11, 17, 22, 0, 0, 0, loc)
	var samples []Sample
	// Every 20 minutes for 30 hours: 3 samples an hour, values rising by hour.
	for i := 0; i < 90; i++ {
		at := base.Add(time.Duration(i) * 20 * time.Minute)
		samples = append(samples, Sample{T: at.UnixMilli(), Values: []float64{float64(i), 10 + float64(i)/10}})
	}
	from, to := base, base.Add(40*time.Hour)

	h := Aggregate(samples, Hour, "last", loc, from, to)
	if len(h.Labels) != 30 {
		t.Fatalf("%d hour buckets, want 30", len(h.Labels))
	}
	if h.Labels[0] != "11-17 22:00" || h.Counts[0] != 3 {
		t.Errorf("first hour %q with %d samples", h.Labels[0], h.Counts[0])
	}
	if h.Series[0][0] != 2 {
		t.Errorf("last of the first hour = %v, want 2", h.Series[0][0])
	}

	d := Aggregate(samples, Day, "avg", loc, from, to)
	if len(d.Labels) != 3 || d.Labels[0] != "11-17" || d.Labels[1] != "11-18" || d.Labels[2] != "11-19" {
		t.Fatalf("day labels %q", d.Labels)
	}
	// The 17th holds i=0..5 (22:00 to 23:40); its average is 2.5.
	if math.Abs(d.Series[0][0]-2.5) > 1e-9 {
		t.Errorf("day avg %v, want 2.5", d.Series[0][0])
	}

	// The 18th holds i=6..77, so its maximum is 77; the 19th ends at i=89.
	mx := Aggregate(samples, Day, "max", loc, from, to)
	if mx.Series[0][1] != 77 || mx.Series[0][2] != 89 {
		t.Errorf("day max %v and %v, want 77 and 89", mx.Series[0][1], mx.Series[0][2])
	}
}

func TestAggregateLeavesGapsOut(t *testing.T) {
	loc := time.UTC
	base := time.Date(2026, 11, 17, 0, 0, 0, 0, loc)
	samples := []Sample{
		{T: base.UnixMilli(), Values: []float64{1}},
		{T: base.Add(5 * time.Hour).UnixMilli(), Values: []float64{2}},
	}
	b := Aggregate(samples, Hour, "last", loc, base, base.Add(10*time.Hour))
	if len(b.Labels) != 2 {
		t.Errorf("%d buckets for 2 hours with samples, want 2", len(b.Labels))
	}
}

func TestStoreAppendReadPrune(t *testing.T) {
	st := NewStore(t.TempDir())
	base := time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		if err := st.Append("gold-now", Sample{T: base.Add(time.Duration(i) * time.Hour).UnixMilli(), Values: []float64{float64(i)}}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := st.Read("gold-now", base.Add(2*time.Hour))
	if err != nil || len(got) != 3 || got[0].Values[0] != 2 {
		t.Fatalf("read since 2h: %v %v", got, err)
	}
	last, ok, _ := st.Latest("gold-now")
	if !ok || last.Values[0] != 4 {
		t.Errorf("latest %v %v", last, ok)
	}
	if err := st.Prune("gold-now", base.Add(3*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got, _ := st.Read("gold-now", base); len(got) != 2 {
		t.Errorf("%d samples after prune, want 2", len(got))
	}
	if err := st.Append("../escape", Sample{}); err == nil {
		t.Error("an id that is not a valid source id was accepted")
	}
}

// fakeFetcher returns a body whose number the test controls.
type fakeFetcher struct {
	value float64
	fail  bool
}

func (f *fakeFetcher) Fetch(ctx context.Context, r Request) ([]byte, error) {
	if f.fail {
		return nil, errors.New("HTTP 503")
	}
	return []byte(fmt.Sprintf(`{"data":{"price":%g}}`, f.value)), nil
}

func TestRunnerPollsOnInterval(t *testing.T) {
	now := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
	f := &fakeFetcher{value: 3.5}
	r := NewRunner(f, NewStore(t.TempDir()))
	r.Now = func() time.Time { return now }
	src := Source{ID: "gold-now", Name: "金价", Every: 60,
		Request: Request{URL: "https://api.example.com/p"},
		Extract: Extract{Kind: KindJSON, Path: "data.price"}}

	if due := r.Due([]Source{src}); len(due) != 1 {
		t.Fatalf("a new source should be due, got %d", len(due))
	}
	if _, err := r.Poll(context.Background(), src); err != nil {
		t.Fatal(err)
	}
	if due := r.Due([]Source{src}); len(due) != 0 {
		t.Error("a source polled a moment ago is due again")
	}
	now = now.Add(61 * time.Second)
	if due := r.Due([]Source{src}); len(due) != 1 {
		t.Error("a source is not due after its interval")
	}

	f.fail = true
	if _, err := r.Poll(context.Background(), src); err == nil {
		t.Fatal("expected the failure")
	}
	if r.LastError("gold-now") == "" {
		t.Error("the failure was not remembered")
	}
	f.fail = false
	if _, err := r.Poll(context.Background(), src); err != nil {
		t.Fatal(err)
	}
	if r.LastError("gold-now") != "" {
		t.Error("a good poll should clear the error")
	}
	samples, _ := r.Store.Read("gold-now", now.Add(-time.Hour))
	if len(samples) != 2 {
		t.Errorf("%d samples stored, want 2 (the failed poll stores nothing)", len(samples))
	}
}

// The HTTP fetcher sends the cookies and headers of a parsed curl command.
func TestHTTPFetcherSendsCookies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("session")
		if err != nil || c.Value != "abc" || r.Header.Get("X-Token") != "t1" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(`{"n":42}`))
	}))
	defer srv.Close()

	r, err := ParseCurl("curl " + srv.URL + "/n -b 'session=abc' -H 'X-Token: t1'")
	if err != nil {
		t.Fatal(err)
	}
	body, err := HTTPFetcher{Client: srv.Client()}.Fetch(context.Background(), r)
	if err != nil || string(body) != `{"n":42}` {
		t.Fatalf("body %q, err %v", body, err)
	}
	r.Cookies = nil
	if _, err := (HTTPFetcher{Client: srv.Client()}).Fetch(context.Background(), r); err == nil {
		t.Error("without the cookie the server should refuse")
	}
}
