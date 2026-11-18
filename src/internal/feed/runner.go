package feed

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"
)

// Runner polls sources and keeps the samples. It does not own a clock or a
// goroutine: the host asks Due which sources are ready, and polls them, so
// that the same code runs in the app, in tests and in any other host.
//
// A source that keeps failing is not asked at the normal pace. After
// BreakAfter failures in a row its circuit is open: the next try waits twice as
// long as the one before, up to MaxPause, and one success closes the circuit.
// A try the user asks for (Poll) is always made. The last good samples stay in
// the store the whole time.
type Runner struct {
	Fetcher Fetcher
	Store   *Store
	Now     func() time.Time
	// BreakAfter is the number of failures in a row that open the circuit;
	// 3 when zero.
	BreakAfter int
	// MaxPause is the longest wait while the circuit is open; an hour when
	// zero.
	MaxPause time.Duration
	// Sleep replaces the wait between tries (tests).
	Sleep func(ctx context.Context, d time.Duration) error

	mu     sync.Mutex
	health map[string]*Health
	busy   map[string]bool
}

// State is how a source is doing.
type State int

const (
	// Idle: not tried yet.
	Idle State = iota
	// OK: the last try worked.
	OK
	// Retrying: the last try failed; the source is still asked at its pace.
	Retrying
	// Open: failed BreakAfter times in a row; asked less and less often.
	Open
)

// Health is what the runner knows about a source.
type Health struct {
	State State
	// Fails is the number of failed tries in a row.
	Fails int
	// Err is the error of the last failed try; "" when the last try worked.
	Err string
	// LastTry, LastOK and Next are when it was last tried, when it last
	// worked, and when it is next due. Zero when unknown.
	LastTry, LastOK, Next time.Time
	// Used is the index of the request that answered the last time:
	// 0 is the main one, 1 the first fallback, and so on.
	Used int
}

// ErrBusy is returned by Poll when the source is already being polled.
var ErrBusy = errors.New("这个数据源正在获取")

// NewRunner returns a runner that uses f and keeps samples in store.
func NewRunner(f Fetcher, store *Store) *Runner {
	return &Runner{Fetcher: f, Store: store, Now: time.Now, health: map[string]*Health{}, busy: map[string]bool{}}
}

func (r *Runner) breakAfter() int {
	if r.BreakAfter > 0 {
		return r.BreakAfter
	}
	return 3
}

func (r *Runner) maxPause() time.Duration {
	if r.MaxPause > 0 {
		return r.MaxPause
	}
	return time.Hour
}

// Due returns the sources that are ready: never tried, or past the time the
// runner set for the next try.
func (r *Runner) Due(sources []Source) []Source {
	now := r.Now()
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Source
	for _, s := range sources {
		h, ok := r.health[s.ID]
		if !ok || !now.Before(h.Next) {
			if !r.busy[s.ID] {
				out = append(out, s)
			}
		}
	}
	return out
}

// Poll tries a source once: for each of its requests in turn it fetches (with
// retries, see Source.Policy), reads the numbers and stores them as a sample.
// A request that fails, or whose response holds no number, hands over to the
// next one; the error of the last is returned when none worked.
func (r *Runner) Poll(ctx context.Context, s Source) (Sample, error) {
	now := r.Now()
	if err := s.Validate(); err != nil {
		r.record(s, now, 0, err)
		return Sample{}, err
	}
	if !r.begin(s.ID) {
		return Sample{}, ErrBusy
	}
	defer r.end(s.ID)

	pol := s.Policy()
	if r.Sleep != nil {
		pol.Sleep = r.Sleep
	}
	var lastErr error
	for i, req := range s.Requests() {
		body, err := pol.Do(ctx, func(ctx context.Context) ([]byte, error) { return r.Fetcher.Fetch(ctx, req) })
		if err == nil {
			var nums []float64
			if nums, err = s.Extract.Numbers(body); err == nil {
				smp := Sample{T: now.UnixMilli(), Values: nums}
				if err := r.Store.Append(s.ID, smp); err != nil {
					r.record(s, now, i, err)
					return Sample{}, err
				}
				r.record(s, now, i, nil)
				return smp, nil
			}
		}
		lastErr = err
		if ctx.Err() != nil {
			break
		}
		if i > 0 {
			lastErr = fmt.Errorf("备用请求 %d：%w", i, err)
		}
	}
	r.record(s, now, 0, lastErr)
	return Sample{}, lastErr
}

func (r *Runner) begin(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.busy[id] {
		return false
	}
	r.busy[id] = true
	return true
}

func (r *Runner) end(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.busy, id)
}

// record remembers the result of a try and sets when the source is next due.
func (r *Runner) record(s Source, at time.Time, used int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	h := r.health[s.ID]
	if h == nil {
		h = &Health{}
		r.health[s.ID] = h
	}
	h.LastTry = at
	every := time.Duration(s.Interval()) * time.Second
	if err == nil {
		h.State, h.Fails, h.Err, h.LastOK, h.Used = OK, 0, "", at, used
		h.Next = at.Add(every)
		return
	}
	h.Fails++
	h.Err = err.Error()
	h.State = Retrying
	pause := every
	if h.Fails >= r.breakAfter() {
		h.State = Open
		// Twice as long after each failure past the limit, up to MaxPause,
		// with up to a tenth added at random so sources do not line up.
		for i := r.breakAfter(); i < h.Fails && pause < r.maxPause(); i++ {
			pause *= 2
		}
		pause = min(pause, r.maxPause())
		pause += time.Duration(rand.Float64() * 0.1 * float64(pause))
	}
	h.Next = at.Add(pause)
}

// Health is what the runner knows about a source.
func (r *Runner) Health(id string) Health {
	r.mu.Lock()
	defer r.mu.Unlock()
	if h := r.health[id]; h != nil {
		return *h
	}
	return Health{}
}

// LastError is the error of the last try of a source, or "".
func (r *Runner) LastError(id string) string { return r.Health(id).Err }

// Forget drops what the runner knows about a source, after it is deleted.
func (r *Runner) Forget(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.health, id)
	delete(r.busy, id)
}

// PollDue polls the due sources, at most parallel at a time, so that one slow
// endpoint does not hold up the others. done is called after each, from the
// polling goroutine. It returns when all are finished.
func (r *Runner) PollDue(ctx context.Context, sources []Source, parallel int, done func(Source, Sample, error)) {
	due := r.Due(sources)
	sem := make(chan struct{}, max(parallel, 1))
	var wg sync.WaitGroup
	for _, s := range due {
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer func() { <-sem; wg.Done() }()
			smp, err := r.Poll(ctx, s)
			if done != nil {
				done(s, smp, err)
			}
		}()
	}
	wg.Wait()
}

// Stale reports whether a source's latest sample is older than three of its
// intervals, or its last try failed: what a chart should show as old data.
func (r *Runner) Stale(s Source, latest int64) bool {
	h := r.Health(s.ID)
	if h.Fails > 0 {
		return true
	}
	if latest == 0 {
		return false
	}
	return r.Now().Sub(time.UnixMilli(latest)) > 3*time.Duration(s.Interval())*time.Second
}
