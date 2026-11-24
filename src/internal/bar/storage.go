package bar

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
	"sync"
)

const (
	MaxStorageKeys  = 256
	MaxStorageBytes = 1 << 20
)

// KVStore is a thread-safe, optionally disk-persisted key-value store scoped
// to a single .bar component.
type KVStore struct {
	mu   sync.RWMutex
	path string
	data map[string]string
}

type storageFile struct {
	Items map[string]string `json:"items"`
}

// NewKVStore opens or creates a KVStore backed by path (or in-memory when path
// is empty).
func NewKVStore(path string) (*KVStore, error) {
	s := &KVStore{
		path: path,
		data: map[string]string{},
	}
	if path != "" {
		if err := s.Reload(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// Reload reloads persisted key-value pairs from disk.
func (s *KVStore) Reload() error {
	if s == nil || s.path == "" {
		return nil
	}
	b, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		s.mu.Lock()
		s.data = map[string]string{}
		s.mu.Unlock()
		return nil
	}
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(b)) == "" {
		s.mu.Lock()
		s.data = map[string]string{}
		s.mu.Unlock()
		return nil
	}
	var f storageFile
	if err := json.Unmarshal(b, &f); err != nil {
		return err
	}
	if f.Items == nil {
		f.Items = map[string]string{}
	}
	s.mu.Lock()
	s.data = f.Items
	s.mu.Unlock()
	return nil
}

// Get returns the value stored under key.
func (s *KVStore) Get(key string) (string, bool) {
	if s == nil {
		return "", false
	}
	s.mu.RLock()
	v, ok := s.data[key]
	s.mu.RUnlock()
	return v, ok
}

// Set stores key -> val, enforcing key count and total byte budgets.
func (s *KVStore) Set(key, val string) error {
	if s == nil {
		return errors.New("storage is nil")
	}
	if key == "" {
		return errors.New("storage key cannot be empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.data[key]; !exists && len(s.data) >= MaxStorageKeys {
		return fmt.Errorf("storage key limit (%d) reached", MaxStorageKeys)
	}
	total := len(key) + len(val)
	for k, v := range s.data {
		if k != key {
			total += len(k) + len(v)
		}
	}
	if total > MaxStorageBytes {
		return fmt.Errorf("storage byte limit (%d) exceeded", MaxStorageBytes)
	}
	if s.data == nil {
		s.data = map[string]string{}
	}
	s.data[key] = val
	return s.saveLocked()
}

// Delete removes key from the store.
func (s *KVStore) Delete(key string) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[key]; !ok {
		return nil
	}
	delete(s.data, key)
	return s.saveLocked()
}

// Clear removes all entries from the store.
func (s *KVStore) Clear() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = map[string]string{}
	return s.saveLocked()
}

// Keys returns all stored keys in sorted order.
func (s *KVStore) Keys() []string {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	s.mu.RUnlock()
	sort.Strings(keys)
	return keys
}

// All returns a copy of all stored key-value pairs.
func (s *KVStore) All() map[string]string {
	if s == nil {
		return map[string]string{}
	}
	s.mu.RLock()
	out := make(map[string]string, len(s.data))
	for k, v := range s.data {
		out[k] = v
	}
	s.mu.RUnlock()
	return out
}

// Count returns the number of stored keys.
func (s *KVStore) Count() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	n := len(s.data)
	s.mu.RUnlock()
	return n
}

// CloneInMemory copies all entries into a fresh in-memory KVStore.
func (s *KVStore) CloneInMemory() *KVStore {
	if s == nil {
		return &KVStore{data: map[string]string{}}
	}
	return &KVStore{data: s.All()}
}

func (s *KVStore) saveLocked() error {
	if s.path == "" {
		return nil
	}
	b, err := json.MarshalIndent(storageFile{Items: s.data}, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(s.path, append(b, '\n'), 0o600)
}
