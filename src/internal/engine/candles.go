package engine

import (
	"sort"
	"time"
)

// A K-line is built from ticks: the host groups them into candles of one
// period, so every script gets the same periods. The period is in seconds:
// 1 a second, 60 a minute, 3600 an hour, 86400 a day, 604800 a week (weeks
// start on Monday), and PeriodMonth for calendar months.
const PeriodMonth = -1

// Periods are the periods a K-line can have, for the scripts' dimensions.
var Periods = map[string]int{
	"second": 1, "minute": 60, "hour": 3600, "day": 86400, "week": 604800, "month": PeriodMonth,
}

// finishChart fills in the candles of a K-line that the script gave as ticks,
// and keeps the last Limit of them. A chart the script built itself is left as
// it is.
func finishChart(ch *Chart, tzOffset int) {
	if ch == nil || ch.Type != "candle" || len(ch.Ticks) == 0 || len(ch.Candles) > 0 {
		return
	}
	labels, candles := CandlesFrom(ch.Ticks, ch.Period, tzOffset)
	if ch.Limit > 0 && len(candles) > ch.Limit {
		labels, candles = labels[len(labels)-ch.Limit:], candles[len(candles)-ch.Limit:]
	}
	ch.Candles, ch.Labels = candles, labels
	if len(labels) > 0 {
		ch.XLabels = []string{labels[0], labels[len(labels)-1]}
	}
}

// CandlesFrom groups ticks into candles of period seconds, in the local time
// of tzOffset (seconds east of UTC). Each candle opens at its first tick,
// closes at its last, and spans the highest and lowest. labels name the start
// of each candle. An unknown period (0) is a day.
func CandlesFrom(ticks []Point, period int, tzOffset int) (labels []string, candles []Candle) {
	if period == 0 {
		period = 86400
	}
	ts := append([]Point(nil), ticks...)
	sort.SliceStable(ts, func(i, j int) bool { return ts[i].T < ts[j].T })

	type bucket struct {
		key   int64
		start time.Time
		c     Candle
	}
	var out []bucket
	for _, p := range ts {
		key, start := bucketOf(p.T, period, tzOffset)
		if n := len(out); n > 0 && out[n-1].key == key {
			b := &out[n-1].c
			if p.V > b.H {
				b.H = p.V
			}
			if p.V < b.L {
				b.L = p.V
			}
			b.C = p.V
			continue
		}
		out = append(out, bucket{key: key, start: start, c: Candle{O: p.V, H: p.V, L: p.V, C: p.V}})
	}
	labels = make([]string, len(out))
	candles = make([]Candle, len(out))
	for i, b := range out {
		labels[i] = labelOf(b.start, period)
		candles[i] = b.c
	}
	return labels, candles
}

// bucketOf is the candle a tick falls in, and the local time that candle starts
// at.
func bucketOf(ms int64, period int, tzOffset int) (int64, time.Time) {
	sec := ms/1000 + int64(tzOffset)
	t := time.Unix(sec, 0).UTC()
	if period == PeriodMonth {
		key := int64(t.Year())*12 + int64(t.Month()) - 1
		start := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
		return key, start
	}
	p := int64(period)
	if period == 604800 {
		// Weeks start on Monday; 1970-01-01 was a Thursday.
		shift := int64(3 * 86400)
		key := (sec + shift) / p
		return key, time.Unix(key*p-shift, 0).UTC()
	}
	key := sec / p
	if sec < 0 && sec%p != 0 {
		key--
	}
	return key, time.Unix(key*p, 0).UTC()
}

// labelOf names a candle by its start: the time for a second or a minute, the
// day for a day or a week, the month for a month.
func labelOf(start time.Time, period int) string {
	switch {
	case period == PeriodMonth:
		return start.Format("2006-01")
	case period >= 86400:
		return start.Format("01-02")
	case period >= 3600:
		return start.Format("01-02 15:00")
	case period >= 60:
		return start.Format("01-02 15:04")
	}
	return start.Format("15:04:05")
}
