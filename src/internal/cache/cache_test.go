package cache

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC)

// A response is cached by its URL and comes back as it was.
func TestPutAndGet(t *testing.T) {
	s := Open(t.TempDir())
	if _, ok := s.Get("https://a.example/x"); ok {
		t.Fatal("an empty cache has an entry")
	}
	if err := s.Put("https://a.example/x", []byte(`{"p":1.5}`), t0); err != nil {
		t.Fatal(err)
	}
	e, ok := s.Get("https://a.example/x")
	if !ok || string(e.Body) != `{"p":1.5}` || !e.At.Equal(t0) {
		t.Fatalf("got %+v %v", e, ok)
	}
	if _, ok := s.Get("https://a.example/y"); ok {
		t.Errorf("another URL found an entry")
	}
	s.Put("https://a.example/x", []byte(`{"p":2}`), t0.Add(time.Minute))
	if e, _ := s.Get("https://a.example/x"); string(e.Body) != `{"p":2}` {
		t.Errorf("the latest response was not kept: %s", e.Body)
	}
}

// What is not JSON, or is too big, is not kept: the cache holds what a script
// can read.
func TestRefusesWhatItCannotUse(t *testing.T) {
	s := Open(t.TempDir())
	if err := s.Put("u", []byte("<html>"), t0); err == nil {
		t.Errorf("HTML was cached")
	}
	if err := s.Put("u", []byte(`"`+strings.Repeat("x", MaxBodyBytes)+`"`), t0); err == nil {
		t.Errorf("a response over the limit was cached")
	}
	if _, ok := s.Get("u"); ok {
		t.Errorf("a refused response left an entry")
	}
}

// The history keeps each response that differs, and an unchanged one only when
// enough time has passed.
func TestHistoryAddsWhatChanged(t *testing.T) {
	s := Open(t.TempDir())
	u := "https://a.example/price"
	s.Put(u, []byte(`{"p":1}`), t0)
	s.Put(u, []byte(`{"p":1}`), t0.Add(time.Minute))                  // the same, too soon
	s.Put(u, []byte(`{"p":2}`), t0.Add(2*time.Minute))                // changed
	s.Put(u, []byte(`{"p":2}`), t0.Add(2*time.Minute+HistoryEvery+1)) // the same, long after
	h := s.History(u, 0)
	if len(h) != 3 {
		t.Fatalf("%d entries, want 3: %+v", len(h), h)
	}
	if string(h[0].Body) != `{"p":1}` || string(h[2].Body) != `{"p":2}` || !h[0].At.Before(h[2].At) {
		t.Errorf("history not oldest first: %+v", h)
	}
	if got := s.History(u, 2); len(got) != 2 || string(got[1].Body) != `{"p":2}` {
		t.Errorf("limit 2: %+v", got)
	}
}

// The history is bounded by count, and the oldest entries go first.
func TestHistoryIsBounded(t *testing.T) {
	s := Open(t.TempDir())
	u := "https://a.example/n"
	for i := 0; i < MaxHistoryEntries+40; i++ {
		s.Put(u, []byte(fmt.Sprintf(`{"n":%d}`, i)), t0.Add(time.Duration(i)*time.Minute))
	}
	h := s.History(u, 0)
	if len(h) != MaxHistoryEntries {
		t.Fatalf("%d entries kept, want %d", len(h), MaxHistoryEntries)
	}
	if string(h[0].Body) != `{"n":40}` || string(h[len(h)-1].Body) != fmt.Sprintf(`{"n":%d}`, MaxHistoryEntries+39) {
		t.Errorf("the wrong end was dropped: first %s last %s", h[0].Body, h[len(h)-1].Body)
	}
}

// ... and by size: big responses crowd the history out sooner.
func TestHistoryIsBoundedBySize(t *testing.T) {
	s := Open(t.TempDir())
	u := "https://a.example/big"
	for i := 0; i < 40; i++ {
		body := fmt.Sprintf(`{"i":%d,"pad":"%s"}`, i, strings.Repeat("x", 40<<10))
		s.Put(u, []byte(body), t0.Add(time.Duration(i)*time.Minute))
	}
	h := s.History(u, 0)
	size := 0
	for _, e := range h {
		size += len(e.Body)
	}
	if size > MaxHistoryBytes+(40<<10) || len(h) < 2 || len(h) >= 40 {
		t.Errorf("%d entries, %d bytes kept", len(h), size)
	}
	if !strings.HasPrefix(string(h[len(h)-1].Body), `{"i":39,`) {
		t.Errorf("the newest entry was dropped: %.20s", h[len(h)-1].Body)
	}
}

// A card's last result is kept under its key and comes back with its time.
func TestResults(t *testing.T) {
	s := Open(t.TempDir())
	if _, _, ok := s.GetResult("card"); ok {
		t.Fatal("an empty cache has a result")
	}
	if err := s.PutResult("card", []byte(`{"big":"65.3"}`), t0); err != nil {
		t.Fatal(err)
	}
	data, at, ok := s.GetResult("card")
	if !ok || string(data) != `{"big":"65.3"}` || !at.Equal(t0) {
		t.Fatalf("got %s %v %v", data, at, ok)
	}
	if err := s.PutResult("card", []byte("nope"), t0); err == nil {
		t.Errorf("a result that is not JSON was cached")
	}
}

// The size is what is on disk, and Clear takes it all away.
func TestSizeAndClear(t *testing.T) {
	s := Open(t.TempDir())
	if s.Size() != 0 {
		t.Fatalf("empty cache has size %d", s.Size())
	}
	s.Put("u", []byte(`{"a":1}`), t0)
	s.PutResult("k", []byte(`{"b":2}`), t0)
	if s.Size() == 0 {
		t.Fatal("size is 0 after writes")
	}
	if err := s.Clear(); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get("u"); ok || s.Size() != 0 {
		t.Errorf("not cleared: size %d", s.Size())
	}
	// It works again after a clear.
	if err := s.Put("u", []byte(`{"a":1}`), t0); err != nil {
		t.Errorf("put after clear: %v", err)
	}
}

// When the cache is over its limit the oldest files go first.
func TestPruneDeletesTheOldest(t *testing.T) {
	s := Open(t.TempDir())
	for i := 0; i < 5; i++ {
		s.Put(fmt.Sprintf("u%d", i), []byte(fmt.Sprintf(`{"pad":"%s"}`, strings.Repeat("x", 10<<10))), t0)
		time.Sleep(15 * time.Millisecond) // distinct modification times
	}
	s.mu.Lock()
	s.pruneLocked(s.sizeUnlocked() / 2)
	s.mu.Unlock()
	if _, ok := s.Get("u4"); !ok {
		t.Errorf("the newest was pruned")
	}
	if _, ok := s.Get("u0"); ok {
		t.Errorf("the oldest survived the prune")
	}
}

func (s *Store) sizeUnlocked() int64 {
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.Size()
}

// Writers on several goroutines leave whole files behind.
func TestConcurrentPuts(t *testing.T) {
	s := Open(t.TempDir())
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				s.Put("shared", []byte(fmt.Sprintf(`{"w":%d,"j":%d}`, i, j)), t0.Add(time.Duration(j)*time.Hour))
			}
		}(i)
	}
	wg.Wait()
	if e, ok := s.Get("shared"); !ok || len(e.Body) == 0 {
		t.Fatalf("no whole entry after concurrent writes")
	}
	for _, e := range s.History("shared", 0) {
		if len(e.Body) == 0 {
			t.Fatalf("a torn history line")
		}
	}
}
