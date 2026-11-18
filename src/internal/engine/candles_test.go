package engine

import (
	"testing"
	"time"
)

// ms is a UTC time in milliseconds, for ticks.
func ms(y int, mo time.Month, d, h, mi, s int) int64 {
	return time.Date(y, mo, d, h, mi, s, 0, time.UTC).UnixMilli()
}

// Ticks are grouped into candles by period: open at the first tick, close at
// the last, high and low across the candle.
func TestCandlesByPeriod(t *testing.T) {
	ticks := []Point{
		{T: ms(2026, 11, 18, 9, 0, 1), V: 10},
		{T: ms(2026, 11, 18, 9, 0, 1) + 500, V: 12},
		{T: ms(2026, 11, 18, 9, 0, 2), V: 9},
		{T: ms(2026, 11, 18, 9, 0, 3), V: 11},
	}
	labels, cs := CandlesFrom(ticks, 1, 0)
	if len(cs) != 3 {
		t.Fatalf("seconds: %d candles, want 3: %v", len(cs), labels)
	}
	if cs[0] != (Candle{O: 10, H: 12, L: 10, C: 12}) {
		t.Errorf("first second %+v", cs[0])
	}
	if labels[0] != "09:00:01" || labels[2] != "09:00:03" {
		t.Errorf("second labels %v", labels)
	}

	// Minutes, hours and days.
	ticks = []Point{
		{T: ms(2026, 11, 18, 9, 5, 0), V: 1},
		{T: ms(2026, 11, 18, 9, 40, 0), V: 3},
		{T: ms(2026, 11, 18, 10, 1, 0), V: 2},
		{T: ms(2026, 11, 19, 8, 0, 0), V: 5},
	}
	if _, cs := CandlesFrom(ticks, 60, 0); len(cs) != 4 {
		t.Errorf("minutes: %d candles, want 4 (four different minutes)", len(cs))
	}
	labels, cs = CandlesFrom(ticks, 3600, 0)
	if len(cs) != 3 || labels[0] != "11-18 09:00" || labels[1] != "11-18 10:00" {
		t.Errorf("hours: %d candles, labels %v", len(cs), labels)
	}
	labels, cs = CandlesFrom(ticks, 86400, 0)
	if len(cs) != 2 || cs[0].H != 3 || cs[0].C != 2 || labels[1] != "11-19" {
		t.Errorf("days: %+v labels %v", cs, labels)
	}
}

// A week starts on Monday, and a month is a calendar month, whatever its length.
func TestCandlesWeekAndMonth(t *testing.T) {
	// 2026-11-16 is a Monday, 2026-11-22 a Sunday, 2026-11-23 the next Monday.
	ticks := []Point{
		{T: ms(2026, 11, 16, 12, 0, 0), V: 1},
		{T: ms(2026, 11, 22, 23, 0, 0), V: 2},
		{T: ms(2026, 11, 23, 1, 0, 0), V: 3},
	}
	labels, cs := CandlesFrom(ticks, 604800, 0)
	if len(cs) != 2 || labels[0] != "11-16" || labels[1] != "11-23" || cs[0].C != 2 {
		t.Errorf("weeks: %+v labels %v", cs, labels)
	}
	ticks = []Point{
		{T: ms(2026, 1, 31, 12, 0, 0), V: 1},
		{T: ms(2026, 2, 1, 0, 0, 0), V: 2},
		{T: ms(2026, 2, 28, 23, 0, 0), V: 4},
	}
	labels, cs = CandlesFrom(ticks, PeriodMonth, 0)
	if len(cs) != 2 || labels[0] != "2026-01" || labels[1] != "2026-02" || cs[1].H != 4 {
		t.Errorf("months: %+v labels %v", cs, labels)
	}
}

// The local time decides where a day starts: a tick at 23:30 UTC is the next
// day in Shanghai.
func TestCandlesUseLocalTime(t *testing.T) {
	ticks := []Point{{T: ms(2026, 11, 18, 23, 30, 0), V: 1}}
	labels, _ := CandlesFrom(ticks, 86400, 8*3600)
	if labels[0] != "11-19" {
		t.Errorf("local day label %q, want 11-19", labels[0])
	}
}

// A script's ticks become candles in Run, and the limit keeps the last ones.
func TestRunBuildsCandlesFromTicks(t *testing.T) {
	src := `export function meta() { return { name: "k", sources: {} }; }
export function render(ctx) {
  const ticks = [];
  for (let i = 0; i < 5; i++) ticks.push({ t: 1763424000000 + i * 86400000, v: 10 + i });
  return { title: "k", big: "1", sub: "", lines: [], chart: { type: "candle", ticks: ticks, period: 86400, limit: 3 } };
}`
	s, err := Compile("k.js", src)
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Run(t.Context(), Input{Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	ch := res.Chart
	if len(ch.Candles) != 3 || len(ch.Labels) != 3 || ch.Candles[2].C != 14 {
		t.Fatalf("candles %+v labels %v", ch.Candles, ch.Labels)
	}
}
