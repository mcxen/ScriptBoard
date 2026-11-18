package feed

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// Unit is the width of a bucket.
type Unit string

// The buckets a chart can be grouped by.
const (
	Hour Unit = "hour"
	Day  Unit = "day"
)

// Buckets is a series of samples grouped by hour or by day. Series[k][i] is
// the k-th number of bucket i, summarised by the source's Agg; Counts[i] is how
// many samples the bucket holds. Buckets with no samples are left out, so a
// gap in polling shows as a gap in the curve's labels rather than as zeros.
type Buckets struct {
	Labels []string
	Starts []int64 // start of each bucket, Unix milliseconds
	Series [][]float64
	Counts []int
}

// Aggregate groups the samples taken in [from, to] by unit, in the time zone
// loc, and summarises each number of a bucket with agg: last, avg, max or min.
// An empty agg means last.
func Aggregate(samples []Sample, unit Unit, agg string, loc *time.Location, from, to time.Time) Buckets {
	if loc == nil {
		loc = time.Local
	}
	if agg == "" {
		agg = "last"
	}
	sorted := append([]Sample(nil), samples...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].T < sorted[j].T })

	type group struct {
		start   int64
		samples []Sample
	}
	byStart := map[int64]*group{}
	for _, smp := range sorted {
		t := time.UnixMilli(smp.T)
		if t.Before(from) || t.After(to) {
			continue
		}
		st := bucketStart(t.In(loc), unit).UnixMilli()
		g := byStart[st]
		if g == nil {
			g = &group{start: st}
			byStart[st] = g
		}
		g.samples = append(g.samples, smp)
	}

	starts := make([]int64, 0, len(byStart))
	for st := range byStart {
		starts = append(starts, st)
	}
	sort.Slice(starts, func(i, j int) bool { return starts[i] < starts[j] })

	var out Buckets
	width := 0
	for _, st := range starts {
		width = max(width, maxWidth(byStart[st].samples))
	}
	out.Series = make([][]float64, width)
	for _, st := range starts {
		g := byStart[st]
		out.Labels = append(out.Labels, labelOf(time.UnixMilli(st).In(loc), unit))
		out.Starts = append(out.Starts, st)
		out.Counts = append(out.Counts, len(g.samples))
		for k := 0; k < width; k++ {
			out.Series[k] = append(out.Series[k], summarise(g.samples, k, agg))
		}
	}
	return out
}

func maxWidth(samples []Sample) int {
	n := 0
	for _, s := range samples {
		n = max(n, len(s.Values))
	}
	return n
}

// summarise reduces the k-th number of the samples with agg. NaN marks a
// bucket where no sample had that number.
func summarise(samples []Sample, k int, agg string) float64 {
	var vals []float64
	for _, s := range samples {
		if k < len(s.Values) {
			vals = append(vals, s.Values[k])
		}
	}
	if len(vals) == 0 {
		return math.NaN()
	}
	switch agg {
	case "avg":
		sum := 0.0
		for _, v := range vals {
			sum += v
		}
		return sum / float64(len(vals))
	case "max":
		m := vals[0]
		for _, v := range vals[1:] {
			m = math.Max(m, v)
		}
		return m
	case "min":
		m := vals[0]
		for _, v := range vals[1:] {
			m = math.Min(m, v)
		}
		return m
	}
	return vals[len(vals)-1] // last
}

// bucketStart is the first instant of the hour or the day that holds t.
func bucketStart(t time.Time, unit Unit) time.Time {
	y, m, d := t.Date()
	if unit == Day {
		return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
	}
	return time.Date(y, m, d, t.Hour(), 0, 0, 0, t.Location())
}

func labelOf(t time.Time, unit Unit) string {
	if unit == Day {
		return t.Format("01-02")
	}
	return fmt.Sprintf("%s %02d:00", t.Format("01-02"), t.Hour())
}
