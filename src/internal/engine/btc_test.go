package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

const btcTicker = `{"symbol":"BTCUSDT","lastPrice":"63480.43000000","priceChangePercent":"-1.47","highPrice":"65000.00","lowPrice":"63000.00"}`

const btcFunding = `{"symbol":"BTCUSDT","markPrice":"63421.98","lastFundingRate":"0.00004150"}`

func btcKlines(n int, last float64) json.RawMessage {
	start := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	parts := make([]string, n)
	for i := 0; i < n; i++ {
		close := last - float64(n-1-i)
		open := close + 10
		tms := start.AddDate(0, 0, i).UnixMilli()
		parts[i] = fmt.Sprintf(`[%d,"%.2f","%.2f","%.2f","%.2f","1"]`, tms, open, close+20, close-20, close)
	}
	return json.RawMessage("[" + strings.Join(parts, ",") + "]")
}

func TestBtcDrawsDailyCandles(t *testing.T) {
	res, err := builtin(t, "btc").Run(context.Background(), Input{
		Now:  time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC),
		View: map[string]float64{"range": 30},
		Sources: map[string]json.RawMessage{
			"ticker":  json.RawMessage(btcTicker),
			"klines":  btcKlines(10, 63480.43),
			"funding": json.RawMessage(btcFunding),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Big != "63480.43" {
		t.Errorf("big %q, want ticker lastPrice", res.Big)
	}
	if res.Sub != "24h -1.47%" {
		t.Errorf("sub %q", res.Sub)
	}
	joined := strings.Join(res.Lines, " ")
	if !strings.Contains(joined, "63000") || !strings.Contains(joined, "资金费 +0.0042%") {
		t.Errorf("lines %q, want 24h range and funding", res.Lines)
	}
	if res.Chart == nil || res.Chart.Type != "candle" || len(res.Chart.Candles) != 10 {
		t.Fatalf("chart %+v, want 10 candles", res.Chart)
	}
	if got := res.Chart.Labels[0]; got != "11-16" {
		t.Errorf("first label %q, want 11-16", got)
	}
	if got := res.Chart.Labels[9]; got != "11-25" {
		t.Errorf("last label %q, want 11-25", got)
	}
}

func TestBtcRangeKeepsLastBars(t *testing.T) {
	res, err := builtin(t, "btc").Run(context.Background(), Input{
		Now:  time.Now(),
		View: map[string]float64{"range": 7},
		Sources: map[string]json.RawMessage{
			"ticker": json.RawMessage(btcTicker),
			"klines": btcKlines(10, 63480.43),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Chart == nil || len(res.Chart.Candles) != 7 {
		t.Fatalf("chart %+v, want 7 candles", res.Chart)
	}
}

func TestBtcTickerWithoutKlines(t *testing.T) {
	res, err := builtin(t, "btc").Run(context.Background(), Input{
		Now: time.Now(),
		Sources: map[string]json.RawMessage{
			"ticker": json.RawMessage(btcTicker),
			"klines": json.RawMessage(`{"error":"timeout"}`),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Big != "63480.43" {
		t.Errorf("big %q", res.Big)
	}
	if res.Chart != nil {
		t.Errorf("chart %+v, want none when klines failed", res.Chart)
	}
}

func TestBtcOfflineHasNoSample(t *testing.T) {
	s := builtin(t, "btc")
	if !strings.Contains(s.Meta.Sources["ticker"], "api.binance.com") {
		t.Errorf("ticker %q", s.Meta.Sources["ticker"])
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
		t.Errorf("offline btc still drew a chart")
	}
	if strings.Contains(strings.Join(res.Lines, " "), "示例") {
		t.Errorf("offline btc used sample copy: %q", res.Lines)
	}
}
