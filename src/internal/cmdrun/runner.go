package cmdrun

import (
	"context"
	"sync"
	"time"
)

// Runner coordinates gated command execution, scheduling, and history persistence.
type Runner struct {
	History   *HistoryStore
	AllowExec func(Spec) bool
	Now       func() time.Time

	mu      sync.Mutex
	busy    map[string]bool
	nextRun map[string]time.Time
	lastErr map[string]string
}

// NewRunner creates a Runner backed by history and permission callback allowExec.
func NewRunner(history *HistoryStore, allowExec func(Spec) bool) *Runner {
	return &Runner{
		History:   history,
		AllowExec: allowExec,
		Now:       time.Now,
		busy:      map[string]bool{},
		nextRun:   map[string]time.Time{},
		lastErr:   map[string]string{},
	}
}

func (r *Runner) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *Runner) allowed(spec Spec) bool {
	if r.AllowExec != nil {
		return r.AllowExec(spec)
	}
	return IsTrustedBuiltin(spec)
}

// Run validates spec, enforces execution permission, executes spec with params,
// and appends the resulting Record to History.
func (r *Runner) Run(ctx context.Context, spec Spec, params map[string]string, trigger string) (Record, error) {
	if err := spec.Validate(); err != nil {
		return Record{}, err
	}
	if !r.allowed(spec) {
		return Record{}, ErrExecDisabled
	}
	if !r.begin(spec.ID) {
		return Record{}, context.DeadlineExceeded
	}
	defer r.end(spec.ID)

	now := r.now()
	rec := Execute(ctx, spec, params, trigger, now)
	if r.History != nil && trigger != "test" {
		_ = r.History.Append(spec.ID, rec, spec.MaxHistoryOrDefault())
	}
	r.recordOutcome(spec, now, rec)
	return rec, nil
}

// Test executes spec once without persisting history.
func (r *Runner) Test(ctx context.Context, spec Spec, params map[string]string) (Record, error) {
	if err := spec.Validate(); err != nil {
		return Record{}, err
	}
	if !r.allowed(spec) {
		return Record{}, ErrExecDisabled
	}
	return Execute(ctx, spec, params, "test", r.now()), nil
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
	delete(r.busy, id)
	r.mu.Unlock()
}

func (r *Runner) recordOutcome(spec Spec, now time.Time, rec Record) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if spec.Every >= MinEvery {
		r.nextRun[spec.ID] = now.Add(time.Duration(spec.Every) * time.Second)
	}
	if rec.OK {
		delete(r.lastErr, spec.ID)
	} else {
		r.lastErr[spec.ID] = rec.Error
	}
}

// IsBusy reports whether command id currently has an active Run call.
func (r *Runner) IsBusy(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.busy[id]
}

// LastError returns the most recent non-OK error message recorded by Runner.
func (r *Runner) LastError(id string) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastErr[id]
}

// NextRun returns the next scheduled timestamp for id (zero if unscheduled/due immediately).
func (r *Runner) NextRun(id string) time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.nextRun[id]
}

// Due returns all scheduled specs (Every >= MinEvery) that are not busy and are ready to run.
func (r *Runner) Due(specs []Spec) []Spec {
	now := r.now()
	r.mu.Lock()
	defer r.mu.Unlock()
	var due []Spec
	for _, sp := range specs {
		if sp.Every < MinEvery || r.busy[sp.ID] {
			continue
		}
		if !r.allowed(sp) {
			continue
		}
		if next, ok := r.nextRun[sp.ID]; !ok || !now.Before(next) {
			due = append(due, sp)
		}
	}
	return due
}

// PollDue executes all due scheduled specs concurrently with up to workers goroutines.
func (r *Runner) PollDue(ctx context.Context, specs []Spec, workers int, onDone func(Spec, Record)) {
	due := r.Due(specs)
	if len(due) == 0 {
		return
	}
	if workers <= 0 {
		workers = 2
	}
	ch := make(chan Spec)
	var wg sync.WaitGroup
	for i := 0; i < min(workers, len(due)); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for sp := range ch {
				rec, err := r.Run(ctx, sp, nil, "schedule")
				if err == nil && onDone != nil {
					onDone(sp, rec)
				}
			}
		}()
	}
	for _, sp := range due {
		select {
		case <-ctx.Done():
			close(ch)
			wg.Wait()
			return
		case ch <- sp:
		}
	}
	close(ch)
	wg.Wait()
}
