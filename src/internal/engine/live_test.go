package engine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// A live source counts as fresh for its interval less a second, and the other
// sources keep the cache's TTL; names that are not sources are left out.
func TestLiveMetaClampsAndTTL(t *testing.T) {
	m := Meta{
		Sources: map[string]Source{"ticker": {URL: "https://example.com/t"}, "klines": {URL: "https://example.com/k"}},
		Live:    &Live{Sources: []string{"ticker", "nope"}, Every: 2},
	}
	if got := m.LiveEvery(); got != LiveMinEvery*time.Second {
		t.Errorf("every %v, want the minimum %v", got, LiveMinEvery*time.Second)
	}
	ttl := m.LiveTTL()
	if len(ttl) != 1 || ttl["ticker"] != 4*time.Second {
		t.Errorf("ttl %v, want ticker: 4s only", ttl)
	}
	if (Meta{}).LiveTTL() != nil || (Meta{}).LiveEvery() != 0 {
		t.Error("a script without live sources has a ttl or an interval")
	}
}

// Each run asks a live source again once its cached answer is older than its
// TTL, and answers the other sources from the cache.
func TestLiveSourceIsAskedAgainOthersAreCached(t *testing.T) {
	s := newServer()
	defer s.Close()
	reqs := map[string]Request{
		"ticker": {Method: "GET", URL: s.URL + "/ticker"},
		"klines": {Method: "GET", URL: s.URL + "/klines"},
	}
	opt := FetchOptions{Cache: newMemCache(), TTL: time.Minute, LiveTTL: map[string]time.Duration{"ticker": time.Second}}
	t0 := time.Now()
	FetchRequests(context.Background(), nil, reqs, t0, opt)
	if n := s.hits.Load(); n != 2 {
		t.Fatalf("first run made %d requests, want 2", n)
	}
	// Two seconds later: the ticker is older than its one-second TTL, the klines are not.
	FetchRequests(context.Background(), nil, reqs, t0.Add(2*time.Second), opt)
	if n := s.hits.Load(); n != 3 {
		t.Errorf("second run made %d requests in all, want 3 (the ticker only)", n)
	}
}

// The line of the BTC card is the prices of its ticker's history, oldest first,
// with their clock times, and a card with two or more answers draws it.
func TestBtcLiveLineFromTheTickerHistory(t *testing.T) {
	at := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	hist := []HistoryPoint{
		{T: at.UnixMilli(), V: json.RawMessage(`{"lastPrice":"63000.00"}`)},
		{T: at.Add(5 * time.Second).UnixMilli(), V: json.RawMessage(`{"lastPrice":"63010.50"}`)},
		{T: at.Add(10 * time.Second).UnixMilli(), V: json.RawMessage(`{"lastPrice":"62990.00"}`)},
	}
	res, err := builtin(t, "btc").Run(context.Background(), Input{
		Now:     at.Add(10 * time.Second),
		View:    map[string]float64{"range": 0},
		Sources: map[string]json.RawMessage{"ticker": json.RawMessage(btcTicker), "klines": btcKlines(10, 63480.43), "funding": json.RawMessage(btcFunding)},
		History: map[string][]HistoryPoint{"ticker": hist},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Chart == nil || res.Chart.Type != "line" || len(res.Chart.Series) != 1 {
		t.Fatalf("chart %+v, want one line", res.Chart)
	}
	pts := res.Chart.Series[0].Points
	if len(pts) != 3 || pts[1] != 63010.5 {
		t.Errorf("points %v", pts)
	}
	if got := strings.Join(res.Chart.Labels, ","); got != "12:00:00,12:00:05,12:00:10" {
		t.Errorf("labels %q", got)
	}
	if res.Big != "63480.43" {
		t.Errorf("the headline is not the ticker's price: %q", res.Big)
	}
}

// Before two answers have come in, the live view says it is recording, and draws
// no line.
func TestBtcLiveViewWaitsForTwoAnswers(t *testing.T) {
	res, err := builtin(t, "btc").Run(context.Background(), Input{
		Now:     time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC),
		View:    map[string]float64{"range": 0},
		Sources: map[string]json.RawMessage{"ticker": json.RawMessage(btcTicker), "klines": btcKlines(10, 63480.43)},
		History: map[string][]HistoryPoint{"ticker": {{T: 1, V: json.RawMessage(`{"lastPrice":"63000"}`)}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Chart != nil {
		t.Errorf("a chart with one answer: %+v", res.Chart)
	}
	if !strings.Contains(strings.Join(res.Lines, " "), "记录") {
		t.Errorf("lines %q, want the recording note", res.Lines)
	}
}
