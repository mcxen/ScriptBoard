package engine

import (
	"testing"
	"time"
)

// The gold K-line script draws every period: sample prices when nothing is
// recorded, and the recorded ones otherwise, at most 60 candles.
func TestGoldKLineEveryPeriod(t *testing.T) {
	var src string
	for _, b := range Builtins() {
		if b.ID == "goldk" {
			src = b.Source
		}
	}
	s, err := Compile("goldk.js", src)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	var recorded []Point
	for i := 0; i < 96; i++ {
		recorded = append(recorded, Point{T: now.Add(-time.Duration(i) * 15 * time.Minute).UnixMilli(), V: 640 + float64(i%7)})
	}
	for _, period := range []float64{86400, 604800, -1, 3600, 60, 1} {
		for _, metrics := range []map[string][]Point{nil, {"gold": recorded}} {
			res, err := s.Run(t.Context(), Input{
				Now: now, TZOffset: 0,
				View:    map[string]float64{"period": period, "range": 0},
				Metrics: metrics,
			})
			if err != nil {
				t.Fatalf("period %v: %v", period, err)
			}
			ch := res.Chart
			if ch == nil || ch.Type != "candle" {
				t.Fatalf("period %v: chart %+v", period, ch)
			}
			if n := len(ch.Candles); n == 0 || n > 60 || len(ch.Labels) != n {
				t.Errorf("period %v: %d candles, %d labels", period, n, len(ch.Labels))
			}
		}
	}
}
