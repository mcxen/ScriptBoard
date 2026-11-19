package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func hs300Source(n int, last float64) json.RawMessage {
	start := time.Date(2026, 11, 15, 16, 0, 0, 0, time.UTC) // 11-16 00:00 CST
	ts := make([]string, n)
	o := make([]string, n)
	h := make([]string, n)
	l := make([]string, n)
	c := make([]string, n)
	for i := 0; i < n; i++ {
		ts[i] = fmt.Sprintf("%d", start.AddDate(0, 0, i).UnixMilli())
		close := last - float64(n-1-i)
		o[i] = fmt.Sprintf("%.2f", close-1)
		h[i] = fmt.Sprintf("%.2f", close+2)
		l[i] = fmt.Sprintf("%.2f", close-2)
		c[i] = fmt.Sprintf("%.2f", close)
	}
	body := fmt.Sprintf(
		`{"data":{"timestamp":[%s],"open":[%s],"high":[%s],"low":[%s],"close":[%s]}}`,
		strings.Join(ts, ","), strings.Join(o, ","), strings.Join(h, ","), strings.Join(l, ","), strings.Join(c, ","),
	)
	return json.RawMessage(body)
}

func TestHs300DrawsDailyCandles(t *testing.T) {
	res, err := builtin(t, "hs300").Run(context.Background(), Input{
		Now:     time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC),
		View:    map[string]float64{"range": 30},
		Sources: map[string]json.RawMessage{"k": hs300Source(10, 3983.77)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Big != "3983.77" {
		t.Errorf("big %q, want last close 3983.77", res.Big)
	}
	if res.Chart == nil || res.Chart.Type != "candle" {
		t.Fatalf("chart %+v, want candle", res.Chart)
	}
	if n := len(res.Chart.Candles); n != 10 {
		t.Errorf("%d candles, want 10", n)
	}
	last := res.Chart.Candles[9]
	if last.C != 3983.77 || last.H != 3985.77 || last.L != 3981.77 {
		t.Errorf("last candle %+v", last)
	}
	if got := res.Chart.Labels[9]; got != "11-25" {
		t.Errorf("last label %q, want 11-25", got)
	}
	if !strings.Contains(res.Sub, "%") {
		t.Errorf("sub %q, want a percent", res.Sub)
	}
}

func TestHs300RangeKeepsLastBars(t *testing.T) {
	res, err := builtin(t, "hs300").Run(context.Background(), Input{
		Now:     time.Now(),
		View:    map[string]float64{"range": 7},
		Sources: map[string]json.RawMessage{"k": hs300Source(10, 3983.77)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Chart == nil || len(res.Chart.Candles) != 7 {
		t.Fatalf("chart %+v, want 7 candles", res.Chart)
	}
}

func TestHs300OfflineHasNoSample(t *testing.T) {
	s := builtin(t, "hs300")
	if !strings.Contains(s.Meta.Sources["k"], "free-api.tickflow.org") {
		t.Errorf("source %q, want TickFlow", s.Meta.Sources["k"])
	}
	res, err := s.Run(context.Background(), Input{
		Now:     time.Now(),
		Sources: Offline(s.Meta.Sources),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Big != "--" {
		t.Errorf("big %q, want --", res.Big)
	}
	if res.Chart != nil {
		t.Errorf("offline hs300 still drew a chart")
	}
	if strings.Contains(strings.Join(res.Lines, " "), "示例") {
		t.Errorf("offline hs300 used sample copy: %q", res.Lines)
	}
}

func TestHs300EnglishMeta(t *testing.T) {
	var src string
	for _, b := range Builtins() {
		if b.ID == "hs300" {
			src = b.Source
		}
	}
	s, err := CompileLang("hs300.js", src, "en")
	if err != nil {
		t.Fatal(err)
	}
	if s.Meta.Name != "CSI 300" {
		t.Errorf("name %q, want CSI 300", s.Meta.Name)
	}
}
