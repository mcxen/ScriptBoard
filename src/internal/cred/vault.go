package cred

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

const (
	vaultKeyName = "cred-vault-key"
	vaultMagic   = "SBVAULT1"
	vaultKeyLen  = 32
)

// ErrSecretNotFound is returned by SecretBackend.Get when the key has not been stored.
var ErrSecretNotFound = errors.New("credential secret not found")

// SecretBackend abstracts master-key storage (OS Keychain in production,
// MemoryBackend in headless tests, or FileKeyBackend when Keychain is unsupported).
type SecretBackend interface {
	Name() string
	Get(key string) ([]byte, error)
	Set(key string, val []byte) error
	Delete(key string) error
}

// MemoryBackend keeps master keys in process memory for unit and headless tests.
type MemoryBackend struct {
	mu   sync.RWMutex
	data map[string][]byte
}

// NewMemoryBackend creates an in-memory SecretBackend.
func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{data: map[string][]byte{}}
}

func (m *MemoryBackend) Name() string { return "memory" }

func (m *MemoryBackend) Get(key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.data[key]
	if !ok {
		return nil, ErrSecretNotFound
	}
	return append([]byte(nil), v...), nil
}

func (m *MemoryBackend) Set(key string, val []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = append([]byte(nil), val...)
	return nil
}

func (m *MemoryBackend) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	return nil
}

// FileKeyBackend stores the 32-byte AES master key in a mode-0600 file when
// the OS credential store reports errors.ErrUnsupported (e.g. headless Linux).
type FileKeyBackend struct {
	path string
	mu   sync.Mutex
}

// NewFileKeyBackend creates a FileKeyBackend rooted in dir.
func NewFileKeyBackend(dir string) *FileKeyBackend {
	return &FileKeyBackend{path: filepath.Join(dir, ".cred-master.key")}
}

func (f *FileKeyBackend) Name() string { return "file-encrypted" }

func (f *FileKeyBackend) Get(_ string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, err := os.ReadFile(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrSecretNotFound
	}
	if err != nil {
		return nil, err
	}
	if len(b) != vaultKeyLen {
		return nil, ErrSecretNotFound
	}
	return b, nil
}

func (f *FileKeyBackend) Set(_ string, val []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return writeAtomicFile(f.path, val, 0o600)
}

func (f *FileKeyBackend) Delete(_ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	err := os.Remove(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// Vault persists non-secret Record metadata in credentials.json and seals
// sensitive SecretPayloads in credentials.vault with AES-256-GCM using a 32-byte
// key protected by SecretBackend (which safely fits within mygo.MaxSecretSize).
type Vault struct {
	mu          sync.Mutex
	metaPath    string
	vaultPath   string
	backend     SecretBackend
	fallback    *FileKeyBackend
	usingFallbk bool
	key         []byte
}

// NewVault opens a Vault inside dataDir using backend for master key storage.
func NewVault(dataDir string, backend SecretBackend) (*Vault, error) {
	if dataDir == "" {
		return nil, errors.New("cred: dataDir is required")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	fb := NewFileKeyBackend(dataDir)
	if backend == nil {
		backend = fb
	}
	return &Vault{
		metaPath:  filepath.Join(dataDir, "credentials.json"),
		vaultPath: filepath.Join(dataDir, "credentials.vault"),
		backend:   backend,
		fallback:  fb,
	}, nil
}

// BackendName returns "keychain", "file-encrypted", or "memory".
func (v *Vault) BackendName() string {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.usingFallbk && v.fallback != nil {
		return v.fallback.Name()
	}
	if v.backend == nil {
		return "file-encrypted"
	}
	return v.backend.Name()
}

// LoadRecords reads non-secret credential metadata from credentials.json.
func (v *Vault) LoadRecords() ([]Record, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	b, err := os.ReadFile(v.metaPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, nil
	}
	var out []Record
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SaveRecords atomically persists non-secret credential metadata to credentials.json.
func (v *Vault) SaveRecords(recs []Record) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(v.metaPath), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomicFile(v.metaPath, append(b, '\n'), 0o600)
}

// LoadAllSecrets decrypts the full secret map from credentials.vault.
func (v *Vault) LoadAllSecrets() (map[string]SecretPayload, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.loadAllLocked()
}

// GetSecret retrieves one secret payload by credential ID.
func (v *Vault) GetSecret(id string) (SecretPayload, bool, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	all, err := v.loadAllLocked()
	if err != nil {
		return SecretPayload{}, false, err
	}
	p, ok := all[id]
	return p, ok, nil
}

// SetSecret encrypts and stores one secret payload by credential ID.
func (v *Vault) SetSecret(id string, p SecretPayload) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	all, err := v.loadAllLocked()
	if err != nil {
		return err
	}
	all[id] = p
	return v.saveAllLocked(all)
}

// DeleteSecret removes one secret payload by credential ID.
func (v *Vault) DeleteSecret(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	all, err := v.loadAllLocked()
	if err != nil {
		return err
	}
	if _, ok := all[id]; !ok {
		return nil
	}
	delete(all, id)
	return v.saveAllLocked(all)
}

func (v *Vault) ensureKeyLocked() ([]byte, error) {
	if len(v.key) == vaultKeyLen {
		return v.key, nil
	}
	b, err := v.backend.Get(vaultKeyName)
	if errors.Is(err, errors.ErrUnsupported) {
		v.usingFallbk = true
		b, err = v.fallback.Get(vaultKeyName)
	}
	if err == nil && len(b) == vaultKeyLen {
		v.key = b
		return v.key, nil
	}
	if err != nil && !errors.Is(err, ErrSecretNotFound) {
		return nil, fmt.Errorf("cred vault key get: %w", err)
	}
	key := make([]byte, vaultKeyLen)
	if _, rerr := io.ReadFull(rand.Reader, key); rerr != nil {
		return nil, rerr
	}
	if !v.usingFallbk {
		err = v.backend.Set(vaultKeyName, key)
		if errors.Is(err, errors.ErrUnsupported) {
			v.usingFallbk = true
			err = v.fallback.Set(vaultKeyName, key)
		}
	} else {
		err = v.fallback.Set(vaultKeyName, key)
	}
	if err != nil {
		return nil, fmt.Errorf("cred vault key save: %w", err)
	}
	v.key = key
	return v.key, nil
}

func (v *Vault) loadAllLocked() (map[string]SecretPayload, error) {
	raw, err := os.ReadFile(v.vaultPath)
	if errors.Is(err, fs.ErrNotExist) || len(raw) == 0 {
		return map[string]SecretPayload{}, nil
	}
	if err != nil {
		return nil, err
	}
	key, err := v.ensureKeyLocked()
	if err != nil {
		return nil, err
	}
	if len(raw) < len(vaultMagic)+12 {
		return nil, errors.New("cred vault corrupt: file too short")
	}
	if string(raw[:len(vaultMagic)]) != vaultMagic {
		return nil, errors.New("cred vault corrupt: bad header")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	payload := raw[len(vaultMagic):]
	if len(payload) < nonceSize {
		return nil, errors.New("cred vault corrupt: short nonce")
	}
	plain, err := gcm.Open(nil, payload[:nonceSize], payload[nonceSize:], nil)
	if err != nil {
		return nil, fmt.Errorf("cred vault decrypt: %w", err)
	}
	out := map[string]SecretPayload{}
	if err := json.Unmarshal(plain, &out); err != nil {
		return nil, fmt.Errorf("cred vault decode: %w", err)
	}
	return out, nil
}

func (v *Vault) saveAllLocked(all map[string]SecretPayload) error {
	if err := os.MkdirAll(filepath.Dir(v.vaultPath), 0o700); err != nil {
		return err
	}
	key, err := v.ensureKeyLocked()
	if err != nil {
		return err
	}
	plain, err := json.Marshal(all)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}
	sealed := gcm.Seal(nil, nonce, plain, nil)
	out := make([]byte, 0, len(vaultMagic)+len(nonce)+len(sealed))
	out = append(out, []byte(vaultMagic)...)
	out = append(out, nonce...)
	out = append(out, sealed...)
	return writeAtomicFile(v.vaultPath, out, 0o600)
}

func writeAtomicFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	cleanup = false
	return nil
}
