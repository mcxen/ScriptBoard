package feed

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Store keeps the samples of every source: one file per source, one JSON line
// per sample, appended as they are taken. A file of samples only grows, so
// Prune drops the old ones.
type Store struct {
	dir string
	mu  sync.Mutex
}

// NewStore keeps samples in dir. The folder is created on first write.
func NewStore(dir string) *Store { return &Store{dir: dir} }

func (s *Store) file(id string) (string, error) {
	if !ValidID(id) {
		return "", fmt.Errorf("数据源 id 不对：%q", id)
	}
	return filepath.Join(s.dir, id+".jsonl"), nil
}

// Append adds a sample to the source's file.
func (s *Store) Append(id string, smp Sample) error {
	path, err := s.file(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	line, err := json.Marshal(smp)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}

// Read returns the samples taken at or after since, oldest first. A line that
// does not parse is skipped: one bad write must not hide the rest.
func (s *Store) Read(id string, since time.Time) ([]Sample, error) {
	all, err := s.all(id)
	if err != nil {
		return nil, err
	}
	cut := since.UnixMilli()
	out := all[:0]
	for _, smp := range all {
		if smp.T >= cut {
			out = append(out, smp)
		}
	}
	return out, nil
}

// Latest returns the most recent sample of a source.
func (s *Store) Latest(id string) (Sample, bool, error) {
	all, err := s.all(id)
	if err != nil || len(all) == 0 {
		return Sample{}, false, err
	}
	return all[len(all)-1], true, nil
}

// Prune drops the samples taken before cut. It rewrites the file through a
// temporary file, so a crash keeps the old one.
func (s *Store) Prune(id string, cut time.Time) error {
	path, err := s.file(id)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := readSamples(path)
	if err != nil {
		return err
	}
	var kept []Sample
	for _, smp := range all {
		if smp.T >= cut.UnixMilli() {
			kept = append(kept, smp)
		}
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	for _, smp := range kept {
		line, _ := json.Marshal(smp)
		w.Write(append(line, '\n'))
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *Store) all(id string) ([]Sample, error) {
	path, err := s.file(id)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return readSamples(path)
}

func readSamples(path string) ([]Sample, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Sample
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		var smp Sample
		if err := json.Unmarshal(sc.Bytes(), &smp); err != nil {
			continue
		}
		out = append(out, smp)
	}
	return out, sc.Err()
}
