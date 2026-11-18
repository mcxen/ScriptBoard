package engine

import (
	"context"
	"testing"
	"time"
)

func builtin(t *testing.T, id string) *Script {
	t.Helper()
	for _, b := range Builtins() {
		if b.ID == id {
			s, err := Compile(id+".js", b.Source)
			if err != nil {
				t.Fatalf("%s: %v", id, err)
			}
			return s
		}
	}
	t.Fatalf("no builtin %q", id)
	return nil
}

// Several gold records on one day make one candle: open is the first,
// close the last, high and low the extremes.
func TestGoldKAggregatesByLocalDay(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*3600)
	base := time.Date(2026, 11, 16, 0, 0, 0, 0, loc)
	at := func(day, hour int) int64 {
		return base.AddDate(0, 0, day).Add(time.Duration(hour) * time.Hour).UnixMilli()
	}
	pts := []Point{
		{T: at(0, 9), V: 630},
		{T: at(0, 13), V: 640},
		{T: at(0, 20), V: 620},
		{T: at(0, 22), V: 635},
		{T: at(1, 9), V: 645},
	}
	res, err := builtin(t, "goldk").Run(context.Background(), Input{
		Now:      base.AddDate(0, 0, 2),
		TZOffset: 8 * 3600,
		Metrics:  map[string][]Point{"gold": pts},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Chart == nil || res.Chart.Type != "candle" {
		t.Fatalf("chart %+v, want a candle chart", res.Chart)
	}
	if res.Icon != "candlestick-chart" {
		t.Errorf("icon %q", res.Icon)
	}
	c := res.Chart.Candles
	if len(c) != 2 {
		t.Fatalf("%d candles, want 2", len(c))
	}
	if c[0] != (Candle{O: 630, H: 640, L: 620, C: 635}) {
		t.Errorf("day 1 candle %+v", c[0])
	}
	if c[1] != (Candle{O: 645, H: 645, L: 645, C: 645}) {
		t.Errorf("day 2 candle %+v", c[1])
	}
	if l := res.Chart.Labels; len(l) != 2 || l[0] != "11-16" || l[1] != "11-17" {
		t.Errorf("labels %q", l)
	}
	if res.Big != "645.00" {
		t.Errorf("big %q", res.Big)
	}
}

// Weight is a dot chart whose labels carry the local date and time.
func TestWeightIsDotChartWithLocalLabels(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*3600)
	base := time.Date(2026, 11, 16, 0, 0, 0, 0, loc)
	pts := []Point{
		{T: base.Add(7 * time.Hour).UnixMilli(), V: 66.0},
		{T: base.AddDate(0, 0, 1).Add(21 * time.Hour).UnixMilli(), V: 65.5},
	}
	res, err := builtin(t, "weight").Run(context.Background(), Input{
		Now:      base.AddDate(0, 0, 1),
		TZOffset: 8 * 3600,
		Settings: map[string]any{"weightUnit": "kg"},
		Metrics:  map[string][]Point{"weight": pts},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Chart == nil || res.Chart.Type != "dot" {
		t.Fatalf("chart %+v, want a dot chart", res.Chart)
	}
	if got := res.Chart.Labels; len(got) != 2 || got[0] != "11-16 07:00" || got[1] != "11-17 21:00" {
		t.Errorf("labels %q", got)
	}
	if res.Icon != "scale" {
		t.Errorf("icon %q", res.Icon)
	}
}
