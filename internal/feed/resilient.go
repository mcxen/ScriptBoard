package feed

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"time"
)

// High availability for a source that lives on somebody else's server: a
// request is retried with growing, jittered waits when the failure is of the
// kind that passes (a timeout, a reset, a 5xx or 429); it is not retried when
// it cannot pass (a 4xx, a bad address, a rule that finds no number). A source
// can name fallback requests, which are tried in order when the main one
// fails. After a run of failures the runner stops asking at the normal pace
// (see Runner), and the last good reading stays in the store, marked stale.

// HTTPError is a response with a status that is not 2xx.
type HTTPError struct{ Status int }

func (e HTTPError) Error() string { return fmt.Sprintf("HTTP %d", e.Status) }

type permanentError struct{ err error }

func (e permanentError) Error() string { return e.err.Error() }
func (e permanentError) Unwrap() error { return e.err }

// permanent marks an error that another try cannot cure.
func permanent(err error) error { return permanentError{err} }

// IsRetryable says whether another try may succeed.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	var perm permanentError
	if errors.As(err, &perm) {
		return false
	}
	var he HTTPError
	if errors.As(err, &he) {
		return he.Status >= 500 || he.Status == 429 || he.Status == 408
	}
	var ne net.Error
	switch {
	case errors.As(err, &ne),
		errors.Is(err, io.ErrUnexpectedEOF),
		errors.Is(err, io.EOF),
		errors.Is(err, context.DeadlineExceeded):
		return true
	}
	return false
}

// Policy is how hard one request is tried.
type Policy struct {
	// Attempts is the number of tries of one request, at least 1.
	Attempts int
	// Timeout is the longest one try may take.
	Timeout time.Duration
	// Base and Max bound the wait between tries: Base, 2·Base, 4·Base … up to
	// Max, each shortened by up to half at random so that sources that fail
	// together do not retry together.
	Base, Max time.Duration

	// Sleep waits for d or until ctx ends; tests replace it.
	Sleep func(ctx context.Context, d time.Duration) error
	// Rand returns a number in [0, 1); tests replace it.
	Rand func() float64
}

// DefaultPolicy is three tries of up to ten seconds, waiting 0.5 s then 1 s.
var DefaultPolicy = Policy{Attempts: 3, Timeout: 10 * time.Second, Base: 500 * time.Millisecond, Max: 8 * time.Second}

// Wait is the wait after the n-th failed try (n starts at 1).
func (p Policy) Wait(n int) time.Duration {
	d := p.Base
	for i := 1; i < n && d < p.Max; i++ {
		d *= 2
	}
	if p.Max > 0 && d > p.Max {
		d = p.Max
	}
	r := rand.Float64
	if p.Rand != nil {
		r = p.Rand
	}
	return d/2 + time.Duration(r()*float64(d/2))
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Do runs try until it succeeds, fails for good, or the attempts are used up.
// Each try gets its own timeout. The error of the last try is returned.
func (p Policy) Do(ctx context.Context, try func(ctx context.Context) ([]byte, error)) ([]byte, error) {
	attempts := max(p.Attempts, 1)
	wait := p.Sleep
	if wait == nil {
		wait = sleep
	}
	var err error
	for n := 1; n <= attempts; n++ {
		tctx, cancel := ctx, context.CancelFunc(func() {})
		if p.Timeout > 0 {
			tctx, cancel = context.WithTimeout(ctx, p.Timeout)
		}
		var body []byte
		body, err = try(tctx)
		cancel()
		if err == nil {
			return body, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err() // the caller gave up
		}
		if n == attempts || !IsRetryable(err) {
			return nil, err
		}
		if werr := wait(ctx, p.Wait(n)); werr != nil {
			return nil, werr
		}
	}
	return nil, err
}
