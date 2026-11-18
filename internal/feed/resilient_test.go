package feed

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func noSleep(ctx context.Context, d time.Duration) error { return nil }

func TestIsRetryable(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{HTTPError{503}, true}, {HTTPError{500}, true}, {HTTPError{429}, true}, {HTTPError{408}, true},
		{HTTPError{404}, false}, {HTTPError{401}, false}, {HTTPError{403}, false},
		{permanent(errors.New("bad")), false}, {context.DeadlineExceeded, true},
		{fmt.Errorf("请求失败：%w", HTTPError{502}), true}, {errors.New("something else"), false}, {nil, false},
	}
	for _, c := range cases {
		if got := IsRetryable(c.err); got != c.want {
			t.Errorf("IsRetryable(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}

func TestPolicyRetriesThenGivesUp(t *testing.T) {
	var waits []time.Duration
	p := Policy{Attempts: 4, Base: time.Second, Max: 3 * time.Second,
		Rand:  func() float64 { return 0.999 },
		Sleep: func(ctx context.Context, d time.Duration) error { waits = append(waits, d); return nil }}
	calls := 0
	_, err := p.Do(context.Background(), func(ctx context.Context) ([]byte, error) { calls++; return nil, HTTPError{503} })
	if calls != 4 || err == nil {
		t.Fatalf("calls %d err %v, want 4 tries and an error", calls, err)
	}
	// 1 s, 2 s, then capped at 3 s; each is between half and the whole wait.
	want := []time.Duration{time.Second, 2 * time.Second, 3 * time.Second}
	if len(waits) != 3 {
		t.Fatalf("waits %v", waits)
	}
	for i, w := range waits {
		if w > want[i] || w < want[i]/2 {
			t.Errorf("wait %d = %v, want within [%v, %v]", i, w, want[i]/2, want[i])
		}
	}
}

func TestPolicyStopsOnPermanentError(t *testing.T) {
	p := Policy{Attempts: 5, Sleep: noSleep}
	calls := 0
	_, err := p.Do(context.Background(), func(ctx context.Context) ([]byte, error) { calls++; return nil, HTTPError{401} })
	if calls != 1 || err == nil {
		t.Errorf("calls %d: a 401 was retried", calls)
	}
}

func TestPolicySucceedsAfterFailures(t *testing.T) {
	p := Policy{Attempts: 3, Sleep: noSleep}
	calls := 0
	body, err := p.Do(context.Background(), func(ctx context.Context) ([]byte, error) {
		calls++
		if calls < 3 {
			return nil, HTTPError{502}
		}
		return []byte("ok"), nil
	})
	if err != nil || string(body) != "ok" || calls != 3 {
		t.Errorf("body %q err %v calls %d", body, err, calls)
	}
}

func TestPolicyTimeoutPerTry(t *testing.T) {
	p := Policy{Attempts: 2, Timeout: 30 * time.Millisecond, Sleep: noSleep}
	calls := 0
	_, err := p.Do(context.Background(), func(ctx context.Context) ([]byte, error) {
		calls++
		<-ctx.Done()
		return nil, ctx.Err()
	})
	if calls != 2 || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("calls %d err %v", calls, err)
	}
}

func TestPolicyStopsWhenCallerCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p := Policy{Attempts: 5, Sleep: noSleep}
	calls := 0
	_, err := p.Do(ctx, func(ctx context.Context) ([]byte, error) { calls++; cancel(); return nil, HTTPError{503} })
	if calls != 1 || !errors.Is(err, context.Canceled) {
		t.Errorf("calls %d err %v", calls, err)
	}
}

// scripted returns what the test says for each address, and counts the calls.
type scripted struct {
	mu    sync.Mutex
	calls map[string]int
	reply func(url string, n int) ([]byte, error)
}

func (s *scripted) Fetch(ctx context.Context, r Request) ([]byte, error) {
	s.mu.Lock()
	if s.calls == nil {
		s.calls = map[string]int{}
	}
	s.calls[r.URL]++
	n := s.calls[r.URL]
	s.mu.Unlock()
	return s.reply(r.URL, n)
}

func (s *scripted) count(url string) int { s.mu.Lock(); defer s.mu.Unlock(); return s.calls[url] }

func haSource() Source {
	return Source{
		ID: "ha", Name: "ha", Every: 60, Retries: 1,
		Request:   Request{URL: "https://main.example/p"},
		Fallbacks: []Request{{URL: "https://backup.example/p"}},
		Extract:   Extract{Kind: KindJSON, Path: "p"},
	}
}

func TestFallbackAnswersWhenMainFails(t *testing.T) {
	f := &scripted{reply: func(u string, n int) ([]byte, error) {
		if u == "https://main.example/p" {
			return nil, HTTPError{503}
		}
		return []byte(`{"p": 9.5}`), nil
	}}
	r := NewRunner(f, NewStore(t.TempDir()))
	r.Sleep = noSleep
	smp, err := r.Poll(context.Background(), haSource())
	if err != nil || smp.Values[0] != 9.5 {
		t.Fatalf("poll: %v %v", smp, err)
	}
	if f.count("https://main.example/p") != 2 {
		t.Errorf("main tried %d times, want 2 (one retry)", f.count("https://main.example/p"))
	}
	h := r.Health("ha")
	if h.State != OK || h.Used != 1 || h.Fails != 0 {
		t.Errorf("health %+v, want OK served by the fallback", h)
	}
}

func TestFallbackWhenMainResponseHasNoNumber(t *testing.T) {
	f := &scripted{reply: func(u string, n int) ([]byte, error) {
		if u == "https://main.example/p" {
			return []byte(`{"error":"maintenance"}`), nil
		}
		return []byte(`{"p": 1}`), nil
	}}
	r := NewRunner(f, NewStore(t.TempDir()))
	r.Sleep = noSleep
	smp, err := r.Poll(context.Background(), haSource())
	if err != nil || smp.Values[0] != 1 {
		t.Fatalf("poll: %v %v", smp, err)
	}
	if f.count("https://main.example/p") != 1 {
		t.Errorf("a page without the number was retried %d times", f.count("https://main.example/p"))
	}
}

func TestAllFailReportsLastError(t *testing.T) {
	f := &scripted{reply: func(u string, n int) ([]byte, error) { return nil, HTTPError{500} }}
	r := NewRunner(f, NewStore(t.TempDir()))
	r.Sleep = noSleep
	_, err := r.Poll(context.Background(), haSource())
	if err == nil {
		t.Fatal("no error")
	}
	if h := r.Health("ha"); h.State != Retrying || h.Fails != 1 || h.Err == "" {
		t.Errorf("health %+v", h)
	}
}

func TestCircuitBreakerBacksOffAndRecovers(t *testing.T) {
	now := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
	var fail atomic.Bool
	fail.Store(true)
	f := &scripted{reply: func(u string, n int) ([]byte, error) {
		if fail.Load() {
			return nil, HTTPError{503}
		}
		return []byte(`{"p": 2}`), nil
	}}
	st := NewStore(t.TempDir())
	r := NewRunner(f, st)
	r.Sleep = noSleep
	r.Now = func() time.Time { return now }
	src := Source{ID: "cb", Name: "cb", Every: 60, Retries: -1,
		Request: Request{URL: "https://x.example/p"}, Extract: Extract{Kind: KindJSON, Path: "p"}}

	// Failures one and two: still asked once a minute.
	for i := 1; i <= 2; i++ {
		r.Poll(context.Background(), src)
		if h := r.Health("cb"); h.State != Retrying || h.Next.Sub(now) != time.Minute {
			t.Fatalf("fail %d: %+v", i, h)
		}
		now = now.Add(time.Minute)
	}
	// The third opens the circuit; the pause is longer than a minute.
	r.Poll(context.Background(), src)
	h := r.Health("cb")
	if h.State != Open || h.Next.Sub(now) < time.Minute {
		t.Fatalf("after 3 failures: %+v", h)
	}
	// Each further failure doubles it, never past MaxPause.
	prev := h.Next.Sub(now)
	for i := 0; i < 12; i++ {
		now = now.Add(2 * time.Hour)
		r.Poll(context.Background(), src)
		d := r.Health("cb").Next.Sub(now)
		if d > time.Hour+time.Hour/10 {
			t.Fatalf("pause %v is longer than MaxPause plus jitter", d)
		}
		prev = d
	}
	if prev < time.Hour {
		t.Errorf("pause settled at %v, want the one hour limit", prev)
	}
	// Not due while open, due again after the pause.
	if len(r.Due([]Source{src})) != 0 {
		t.Error("an open circuit is due at once")
	}
	now = now.Add(2 * time.Hour)
	if len(r.Due([]Source{src})) != 1 {
		t.Error("not due after the pause")
	}
	// The user can always try; a success closes the circuit and keeps the pace.
	fail.Store(false)
	if _, err := r.Poll(context.Background(), src); err != nil {
		t.Fatal(err)
	}
	if h := r.Health("cb"); h.State != OK || h.Fails != 0 || h.Next.Sub(now) != time.Minute {
		t.Errorf("after recovery: %+v", h)
	}
}

func TestStaleKeepsLastGoodSample(t *testing.T) {
	now := time.Date(2026, 11, 18, 9, 0, 0, 0, time.UTC)
	var fail atomic.Bool
	f := &scripted{reply: func(u string, n int) ([]byte, error) {
		if fail.Load() {
			return nil, HTTPError{500}
		}
		return []byte(`{"p": 4}`), nil
	}}
	st := NewStore(t.TempDir())
	r := NewRunner(f, st)
	r.Sleep = noSleep
	r.Now = func() time.Time { return now }
	src := Source{ID: "st", Name: "st", Every: 60, Retries: -1,
		Request: Request{URL: "https://x.example/p"}, Extract: Extract{Kind: KindJSON, Path: "p"}}
	r.Poll(context.Background(), src)
	last, _, _ := st.Latest("st")
	if r.Stale(src, last.T) {
		t.Error("fresh data reported stale")
	}
	fail.Store(true)
	now = now.Add(time.Minute)
	r.Poll(context.Background(), src)
	if !r.Stale(src, last.T) {
		t.Error("data after a failed try not reported stale")
	}
	if got, ok, _ := st.Latest("st"); !ok || got.Values[0] != 4 {
		t.Errorf("last good sample lost: %v %v", got, ok)
	}
}

func TestPollDueRunsInParallelAndNeverTwice(t *testing.T) {
	release := make(chan struct{})
	var running, peak atomic.Int32
	f := &scripted{reply: func(u string, n int) ([]byte, error) {
		cur := running.Add(1)
		for {
			p := peak.Load()
			if cur <= p || peak.CompareAndSwap(p, cur) {
				break
			}
		}
		<-release
		running.Add(-1)
		return []byte(`{"p": 1}`), nil
	}}
	r := NewRunner(f, NewStore(t.TempDir()))
	var srcs []Source
	for i := 0; i < 4; i++ {
		srcs = append(srcs, Source{ID: fmt.Sprintf("s%d", i), Name: "s", Every: 60,
			Request: Request{URL: fmt.Sprintf("https://x%d.example/p", i)}, Extract: Extract{Kind: KindJSON, Path: "p"}})
	}
	finished := make(chan struct{})
	var done atomic.Int32
	go func() {
		r.PollDue(context.Background(), srcs, 4, func(Source, Sample, error) { done.Add(1) })
		close(finished)
	}()
	deadline := time.Now().Add(3 * time.Second)
	for running.Load() < 4 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if running.Load() != 4 {
		t.Fatalf("%d sources running at once, want 4", running.Load())
	}
	// A source being polled is neither due nor pollable.
	if len(r.Due(srcs)) != 0 {
		t.Error("busy sources reported due")
	}
	if _, err := r.Poll(context.Background(), srcs[0]); !errors.Is(err, ErrBusy) {
		t.Errorf("second poll of a busy source: %v", err)
	}
	close(release)
	<-finished
	if done.Load() != 4 {
		t.Errorf("%d done", done.Load())
	}
}

func TestSourceValidationOfFallbacksAndLimits(t *testing.T) {
	s := haSource()
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	s.Fallbacks = []Request{{URL: "gopher://x"}}
	if err := s.Validate(); err == nil {
		t.Error("bad fallback accepted")
	}
	s = haSource()
	s.Timeout = 500
	if err := s.Validate(); err == nil {
		t.Error("500 s timeout accepted")
	}
	if got := (Source{Retries: -1}).Policy().Attempts; got != 1 {
		t.Errorf("Retries -1 gives %d attempts", got)
	}
	if got := (Source{}).Policy().Attempts; got != 3 {
		t.Errorf("default attempts %d", got)
	}
}
