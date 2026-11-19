// Package cache keeps on disk what the app fetched and computed, so that a card
// opens at once and the app works with no network:
//
//	src/   the latest response of each source URL
//	hist/  a bounded history of those responses, one JSON line each
//	res/   the last result of each card
//
// Every file is written through a temporary file and renamed, so a crash leaves
// the old copy, never half of a new one. The cache is a copy: deleting it
// loses nothing but speed.
package cache

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Limits of the history of one source: how many entries and how many bytes it
// keeps (the oldest go first), and how often it adds one.
const (
	MaxHistoryEntries = 500
	MaxHistoryBytes   = 512 << 10
	// HistoryEvery is the shortest time between two history entries of one
	// source whose body did not change; a changed body is always added.
	HistoryEvery = 15 * time.Minute
	// MaxBodyBytes is the largest response kept; a bigger one is not cached.
	MaxBodyBytes = 2 << 20
	// MaxTotalBytes is what the cache may grow to before the oldest files go.
	MaxTotalBytes = 64 << 20
)

// Entry is one cached response: when it was fetched and what it said.
type Entry struct {
	At   time.Time       `json:"at"`
	Body json.RawMessage `json:"body"`
}

// Store is the cache in one folder. It is safe for concurrent use.
type Store struct {
	dir string
	mu  sync.Mutex
}

// Open returns the cache in dir, which is created when it is first written.
func Open(dir string) *Store { return &Store{dir: dir} }

// Dir is the folder of the cache.
func (s *Store) Dir() string { return s.dir }

func name(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:10])
}

func (s *Store) path(kind, key, ext string) string {
	return filepath.Join(s.dir, kind, name(key)+ext)
}

// write puts data at path through a temporary file.
func write(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		os.Remove(tmp.Name())
		return errors.Join(werr, cerr)
	}
	return os.Rename(tmp.Name(), path)
}

// Get is the latest response cached for url.
func (s *Store) Get(url string) (Entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(s.path("src", url, ".json"))
	if err != nil {
		return Entry{}, false
	}
	var rec struct {
		URL string `json:"url"`
		Entry
	}
	if json.Unmarshal(b, &rec) != nil || rec.URL != url {
		return Entry{}, false
	}
	return rec.Entry, true
}

// Put caches the response of url, and adds it to the url's history when it
// differs from the last entry or enough time has passed. A body that is not
// JSON, or is too big, is not kept.
func (s *Store) Put(url string, body []byte, at time.Time) error {
	if len(body) > MaxBodyBytes || !json.Valid(body) {
		return errors.New("cache: the response is not kept")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := json.Marshal(struct {
		URL string `json:"url"`
		Entry
	}{url, Entry{At: at, Body: body}})
	if err != nil {
		return err
	}
	if err := write(s.path("src", url, ".json"), rec); err != nil {
		return err
	}
	return s.appendHistory(url, Entry{At: at, Body: body})
}

func (s *Store) appendHistory(url string, e Entry) error {
	path := s.path("hist", url, ".jsonl")
	entries := readHistory(path)
	if n := len(entries); n > 0 {
		last := entries[n-1]
		if e.At.Sub(last.At) < HistoryEvery && bytes.Equal(last.Body, e.Body) {
			return nil
		}
	}
	entries = append(entries, e)
	// The oldest go first, by count and by size.
	if len(entries) > MaxHistoryEntries {
		entries = entries[len(entries)-MaxHistoryEntries:]
	}
	size := 0
	for _, x := range entries {
		size += len(x.Body) + 32
	}
	for len(entries) > 1 && size > MaxHistoryBytes {
		size -= len(entries[0].Body) + 32
		entries = entries[1:]
	}
	var buf bytes.Buffer
	for _, x := range entries {
		line, err := json.Marshal(x)
		if err != nil {
			continue
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	return write(path, buf.Bytes())
}

func readHistory(path string) []Entry {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64<<10), MaxBodyBytes+64<<10)
	for sc.Scan() {
		var e Entry
		if json.Unmarshal(sc.Bytes(), &e) == nil && len(e.Body) > 0 {
			out = append(out, e)
		}
	}
	return out
}

// History is the responses kept for url, the oldest first, at most limit of
// the newest (all of them when limit is 0).
func (s *Store) History(url string, limit int) []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries := readHistory(s.path("hist", url, ".jsonl"))
	if limit > 0 && len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}
	return entries
}

// GetResult is what was cached under key, and when.
func (s *Store) GetResult(key string) ([]byte, time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(s.path("res", key, ".json"))
	if err != nil {
		return nil, time.Time{}, false
	}
	var rec struct {
		Key  string          `json:"key"`
		At   time.Time       `json:"at"`
		Data json.RawMessage `json:"data"`
	}
	if json.Unmarshal(b, &rec) != nil || rec.Key != key {
		return nil, time.Time{}, false
	}
	return rec.Data, rec.At, true
}

// PutResult caches data under key.
func (s *Store) PutResult(key string, data []byte, at time.Time) error {
	if !json.Valid(data) {
		return errors.New("cache: the result is not JSON")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, err := json.Marshal(map[string]any{"key": key, "at": at, "data": json.RawMessage(data)})
	if err != nil {
		return err
	}
	if err := write(s.path("res", key, ".json"), rec); err != nil {
		return err
	}
	return s.pruneLocked(MaxTotalBytes)
}

// Size is the bytes the cache takes.
func (s *Store) Size() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int64
	filepath.WalkDir(s.dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				n += info.Size()
			}
		}
		return nil
	})
	return n
}

// Clear deletes the cache.
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.RemoveAll(s.dir); err != nil {
		return err
	}
	return nil
}

// pruneLocked deletes the oldest files until the cache is at most max bytes.
func (s *Store) pruneLocked(max int64) error {
	type file struct {
		path string
		size int64
		mod  time.Time
	}
	var files []file
	var total int64
	filepath.WalkDir(s.dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			files = append(files, file{p, info.Size(), info.ModTime()})
			total += info.Size()
		}
		return nil
	})
	if total <= max {
		return nil
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod.Before(files[j].mod) })
	for _, f := range files {
		if total <= max {
			break
		}
		if os.Remove(f.path) == nil {
			total -= f.size
		}
	}
	return nil
}
