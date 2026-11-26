package cmdrun

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// HistoryStore persists per-command execution records as JSONL files in dir.
type HistoryStore struct {
	dir string
	mu  sync.Mutex
}

// NewHistoryStore stores command execution histories under dir.
func NewHistoryStore(dir string) *HistoryStore { return &HistoryStore{dir: dir} }

func (h *HistoryStore) file(id string) (string, error) {
	if !ValidID(id) {
		return "", fmt.Errorf("命令 id 不合法：%q", id)
	}
	return filepath.Join(h.dir, id+".jsonl"), nil
}

// Append writes rec to id's JSONL log and atomically trims old entries beyond maxKeep.
func (h *HistoryStore) Append(id string, rec Record, maxKeep int) error {
	path, err := h.file(id)
	if err != nil {
		return err
	}
	if maxKeep <= 0 {
		maxKeep = DefaultMaxHistory
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := os.MkdirAll(h.dir, 0o755); err != nil {
		return err
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, werr := f.Write(append(line, '\n'))
	cerr := f.Close()
	if werr != nil {
		return werr
	}
	if cerr != nil {
		return cerr
	}
	all, err := readRecords(path)
	if err != nil || len(all) <= maxKeep {
		return nil
	}
	kept := all[len(all)-maxKeep:]
	return writeRecordsAtomic(path, kept)
}

// Recent returns up to limit execution records for id, newest-first.
func (h *HistoryStore) Recent(id string, limit int) ([]Record, error) {
	path, err := h.file(id)
	if err != nil {
		return nil, err
	}
	h.mu.Lock()
	all, err := readRecords(path)
	h.mu.Unlock()
	if err != nil {
		return nil, err
	}
	n := len(all)
	if n == 0 {
		return nil, nil
	}
	if limit <= 0 || limit > n {
		limit = n
	}
	out := make([]Record, 0, limit)
	for i := n - 1; i >= n-limit; i-- {
		out = append(out, all[i])
	}
	return out, nil
}

// Latest returns the most recent record for id.
func (h *HistoryStore) Latest(id string) (Record, bool, error) {
	recs, err := h.Recent(id, 1)
	if err != nil || len(recs) == 0 {
		return Record{}, false, err
	}
	return recs[0], true, nil
}

// Count returns the number of stored records for id.
func (h *HistoryStore) Count(id string) int {
	path, err := h.file(id)
	if err != nil {
		return 0
	}
	h.mu.Lock()
	all, err := readRecords(path)
	h.mu.Unlock()
	if err != nil {
		return 0
	}
	return len(all)
}

// Clear removes all saved history for id.
func (h *HistoryStore) Clear(id string) error {
	path, err := h.file(id)
	if err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Delete removes the history file for id.
func (h *HistoryStore) Delete(id string) error {
	return h.Clear(id)
}

func readRecords(path string) ([]Record, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Record
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 4<<20)
	for sc.Scan() {
		var r Record
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			continue
		}
		out = append(out, r)
	}
	return out, sc.Err()
}

func writeRecordsAtomic(path string, recs []Record) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	for _, r := range recs {
		b, _ := json.Marshal(r)
		_, _ = w.Write(append(b, '\n'))
	}
	if err := w.Flush(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
