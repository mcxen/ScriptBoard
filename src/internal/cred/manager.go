package cred

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	defaultAttemptTimeout     = 15 * time.Second
	defaultInteractiveTimeout = 5 * time.Minute
	defaultFailCooldown       = 3 * time.Second
	maxReplayRequestBody      = 2 << 20
	maxBufferedErrorBody      = 64 << 10
)

// LoginHandlers provides host-implemented interactive credential providers.
type LoginHandlers struct {
	WebView  func(ctx context.Context, spec Spec) (SecretPayload, error)
	Browser  func(ctx context.Context, spec Spec) (SecretPayload, error)
	Ego      func(ctx context.Context, spec Spec) (SecretPayload, error)
	CDP      func(ctx context.Context, spec Spec) (SecretPayload, error)
	OnChange func(id string)
}

type inflightAuth struct {
	done            chan struct{}
	cancel          context.CancelFunc
	provider        string
	startedAt       time.Time
	cancelledByUser bool
	payload         SecretPayload
	err             error
}

// Manager coordinates declared credentials, Keychain vault persistence,
// single-flight interactive login, cooldown tracking, and HTTP auto-refresh.
type Manager struct {
	mu                 sync.RWMutex
	vault              *Vault
	records            map[string]Record
	secrets            map[string]SecretPayload
	handlers           LoginHandlers
	inFlight           map[string]*inflightAuth
	cooldownUntil      map[string]time.Time
	revision           uint64
	InteractiveTimeout time.Duration
	Cooldown           time.Duration
}

// NewManager opens the credential Manager backed by Vault in dataDir.
func NewManager(dataDir string, backend SecretBackend, handlers LoginHandlers) (*Manager, error) {
	v, err := NewVault(dataDir, backend)
	if err != nil {
		return nil, err
	}
	m := &Manager{
		vault:              v,
		records:            map[string]Record{},
		secrets:            map[string]SecretPayload{},
		handlers:           handlers,
		inFlight:           map[string]*inflightAuth{},
		cooldownUntil:      map[string]time.Time{},
		InteractiveTimeout: defaultInteractiveTimeout,
		Cooldown:           defaultFailCooldown,
	}
	if err := m.Reload(); err != nil {
		return nil, err
	}
	return m, nil
}

// Reload reads metadata records and encrypted secrets from disk.
func (m *Manager) Reload() error {
	if m == nil {
		return nil
	}
	recs, err := m.vault.LoadRecords()
	if err != nil {
		return err
	}
	secs, err := m.vault.LoadAllSecrets()
	if err != nil {
		return err
	}
	backendName := m.vault.BackendName()

	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = make(map[string]Record, len(recs))
	m.secrets = make(map[string]SecretPayload, len(secs))
	for k, p := range secs {
		m.secrets[k] = p
	}
	for _, r := range recs {
		r.Spec = r.Spec.Normalized()
		r.StorageBackend = backendName
		p, ok := m.secrets[r.Spec.ID]
		r.HasSecret = ok
		syncRecordSummary(&r, p, ok)
		if r.Status == StatusAuthenticating && m.inFlight[r.Spec.ID] == nil {
			if ok {
				r.Status = StatusReady
			} else {
				r.Status = StatusMissing
			}
		}
		m.records[r.Spec.ID] = r
	}
	m.revision++
	return nil
}

// SetHandlers replaces the interactive login callbacks and notifies listeners cleanly.
func (m *Manager) SetHandlers(h LoginHandlers) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.handlers = h
	m.mu.Unlock()
}

// BackendName reports the active master-key backend ("keychain", "file-encrypted", or "memory").
func (m *Manager) BackendName() string {
	if m == nil || m.vault == nil {
		return ""
	}
	return m.vault.BackendName()
}

// Revision returns a monotonically increasing counter bumped on any credential mutation.
func (m *Manager) Revision(_ []Spec) uint64 {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.revision
}

// RegisterSpecs registers plugin- or component-declared credential specs into
// the metadata index without overwriting existing user secrets or overrides.
func (m *Manager) RegisterSpecs(pluginID string, specs []Spec) {
	if m == nil || len(specs) == 0 {
		return
	}
	m.mu.Lock()
	changed := false
	backend := m.vault.BackendName()
	for _, raw := range specs {
		sp := raw.Normalized()
		if sp.Validate() != nil {
			continue
		}
		if pluginID != "" && sp.SourcePlugin == "" {
			sp.SourcePlugin = pluginID
		}
		existing, ok := m.records[sp.ID]
		if !ok {
			p, hasSec := m.secrets[sp.ID]
			rec := Record{
				Spec:           sp,
				Status:         StatusMissing,
				StorageBackend: backend,
			}
			if hasSec {
				rec.Status = StatusReady
			}
			syncRecordSummary(&rec, p, hasSec)
			m.records[sp.ID] = rec
			changed = true
			continue
		}
		// Refresh plugin metadata fields if they changed while preserving stored status/secrets.
		merged := sp
		if existing.Spec.Name != "" && merged.Name == "" {
			merged.Name = existing.Spec.Name
		}
		if pluginID != "" {
			merged.SourcePlugin = pluginID
		} else if merged.SourcePlugin == "" {
			merged.SourcePlugin = existing.Spec.SourcePlugin
		}
		if !specsEqual(existing.Spec, merged) {
			existing.Spec = merged
			existing.StorageBackend = backend
			m.records[sp.ID] = existing
			changed = true
		}
	}
	var snapshot []Record
	if changed {
		snapshot = m.sortedRecordsLocked()
	}
	m.mu.Unlock()

	if changed {
		_ = m.vault.SaveRecords(snapshot)
	}
}

// List returns non-secret metadata for all known credentials, sorted by ID.
func (m *Manager) List() []Record {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sortedRecordsLocked()
}

// Get returns non-secret metadata for one credential ID.
func (m *Manager) Get(id string) (Record, bool) {
	if m == nil {
		return Record{}, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.records[strings.TrimSpace(id)]
	return r, ok
}

// GetSecret returns a copy of the stored secret payload for internal runtime use.
func (m *Manager) GetSecret(id string) (SecretPayload, bool) {
	if m == nil {
		return SecretPayload{}, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.secrets[strings.TrimSpace(id)]
	if ok && len(p.Cookies) > 0 {
		cp := make([]StoredCookie, len(p.Cookies))
		copy(cp, p.Cookies)
		p.Cookies = cp
	}
	return p, ok
}

// StatusMap builds a redacted status map keyed by credential ID for scripts/.bar.
func (m *Manager) StatusMap(specs []Spec) map[string]Status {
	out := make(map[string]Status, len(specs))
	if m == nil {
		return out
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, raw := range specs {
		sp := raw.Normalized()
		if sp.ID == "" {
			continue
		}
		if rec, ok := m.records[sp.ID]; ok {
			out[sp.ID] = rec.ToStatus()
		} else {
			out[sp.ID] = Status{
				ID:       sp.ID,
				Name:     sp.Name,
				Type:     sp.Type,
				Origin:   sp.Origin,
				Provider: sp.Provider,
				Status:   StatusMissing,
			}
		}
	}
	return out
}

// RuntimeDataOf returns the developer-facing runtime object (status + cookies/token)
// for a single stored credential ID.
func (m *Manager) RuntimeDataOf(id string) (RuntimeData, bool) {
	if m == nil {
		return RuntimeData{}, false
	}
	id = strings.TrimSpace(id)
	m.mu.RLock()
	defer m.mu.RUnlock()
	rec, ok := m.records[id]
	if !ok {
		return RuntimeData{}, false
	}
	p, hasSec := m.secrets[id]
	return BuildRuntimeData(rec, p, hasSec), true
}

// RuntimeDataMap returns a map of id -> RuntimeData for all declared specs so
// JS scripts (ctx.credentials) and .bar components ($metrix.credentials) can
// freely inspect status and pick any extracted cookie or token value.
func (m *Manager) RuntimeDataMap(specs []Spec) map[string]RuntimeData {
	out := make(map[string]RuntimeData, len(specs))
	if m == nil {
		return out
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, raw := range specs {
		sp := raw.Normalized()
		if sp.ID == "" {
			continue
		}
		if rec, ok := m.records[sp.ID]; ok {
			p, hasSec := m.secrets[sp.ID]
			out[sp.ID] = BuildRuntimeData(rec, p, hasSec)
		} else {
			out[sp.ID] = RuntimeData{
				ID:       sp.ID,
				Name:     sp.Name,
				Type:     sp.Type,
				Origin:   sp.Origin,
				Provider: sp.Provider,
				Status:   StatusMissing,
			}
		}
	}
	return out
}

// BuildRuntimeData constructs a RuntimeData view combining non-secret metadata
// with live decrypted credential values for plugin runtime execution.
func BuildRuntimeData(rec Record, p SecretPayload, hasSec bool) RuntimeData {
	rd := RuntimeData{
		ID:          rec.Spec.ID,
		Name:        rec.Spec.Name,
		Type:        rec.Spec.Type,
		Origin:      rec.Spec.Origin,
		Provider:    rec.Spec.Provider,
		Status:      rec.Status,
		HasSecret:   hasSec,
		CookieCount: rec.CookieCount,
		CookieNames: append([]string(nil), rec.CookieNames...),
		UpdatedAt:   rec.UpdatedAt,
		LastError:   rec.LastError,
	}
	if !hasSec {
		return rd
	}
	if len(p.Cookies) > 0 {
		rd.Cookies, rd.CookieHeader = CookieMap(p.Cookies)
	}
	rd.Token = p.Token
	rd.Username = p.Username
	hName, hVal := resolvedHeaderPair(rec.Spec, p)
	rd.HeaderName = hName
	rd.HeaderValue = hVal
	return rd
}

// ExpandTemplate replaces {cred.<id>.cookie.<name>}, {cred.<id>.cookieHeader},
// {cred.<id>.token}, and {cred.<id>.header} placeholders inside URLs, headers,
// and request bodies for specs and known stored credentials.
func (m *Manager) ExpandTemplate(s string, specs []Spec) string {
	if m == nil || !strings.Contains(s, "{cred.") {
		return s
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	seen := map[string]bool{}
	var ids []string
	for _, sp := range specs {
		if sp.ID != "" && !seen[sp.ID] {
			seen[sp.ID] = true
			ids = append(ids, sp.ID)
		}
	}
	for id := range m.secrets {
		if id != "" && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}

	out := s
	for _, id := range ids {
		p, hasSec := m.secrets[id]
		if !hasSec {
			continue
		}
		rec := m.records[id]
		if rec.Spec.ID == "" {
			rec.Spec.ID = id
		}
		rd := BuildRuntimeData(rec, p, true)
		prefix := "{cred." + id + "."
		if !strings.Contains(out, prefix) {
			continue
		}
		out = strings.ReplaceAll(out, prefix+"cookieHeader}", rd.CookieHeader)
		out = strings.ReplaceAll(out, prefix+"token}", rd.Token)
		if rd.HeaderValue != "" {
			out = strings.ReplaceAll(out, prefix+"header}", rd.HeaderValue)
		} else {
			out = strings.ReplaceAll(out, prefix+"header}", rd.CookieHeader)
		}
		for k, v := range rd.Cookies {
			out = strings.ReplaceAll(out, prefix+"cookie."+k+"}", v)
		}
	}
	return out
}

func resolvedHeaderPair(sp Spec, p SecretPayload) (string, string) {
	typ := p.Type
	if typ == "" {
		typ = sp.Type
	}
	switch typ {
	case TypeCookie:
		if sp.CookieKey != "" {
			for _, c := range p.Cookies {
				if c.Name == sp.CookieKey && c.Value != "" {
					hName := firstNonEmpty(p.HeaderName, sp.HeaderName, "Authorization")
					pref := p.HeaderPrefix
					if pref == "" {
						pref = sp.HeaderPrefix
					}
					return hName, pref + c.Value
				}
			}
		}
		_, hdr := CookieMap(p.Cookies)
		if hdr != "" {
			return "Cookie", hdr
		}
	case TypeBearer:
		if strings.TrimSpace(p.Token) != "" {
			pref := p.HeaderPrefix
			if pref == "" {
				pref = "Bearer "
			}
			return firstNonEmpty(p.HeaderName, "Authorization"), pref + strings.TrimSpace(p.Token)
		}
	case TypeToken, TypeHeader:
		if strings.TrimSpace(p.Token) != "" {
			hName := firstNonEmpty(p.HeaderName, sp.HeaderName, "Authorization")
			pref := p.HeaderPrefix
			if pref == "" {
				pref = sp.HeaderPrefix
			}
			return hName, pref + p.Token
		}
	case TypeBasic:
		if p.Username != "" {
			return "Authorization", "Basic " + base64.StdEncoding.EncodeToString([]byte(p.Username+":"+p.Password))
		}
	}
	return "", ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// SaveSpec creates or updates a credential specification cleanly.
func (m *Manager) SaveSpec(spec Spec) (Record, error) {
	if m == nil {
		return Record{}, errors.New("cred: manager is nil")
	}
	sp := spec.Normalized()
	if err := sp.Validate(); err != nil {
		return Record{}, err
	}

	m.mu.Lock()
	rec, ok := m.records[sp.ID]
	if !ok {
		rec = Record{Status: StatusMissing}
	}
	rec.Spec = sp
	rec.StorageBackend = m.vault.BackendName()
	p, hasSec := m.secrets[sp.ID]
	syncRecordSummary(&rec, p, hasSec)
	if hasSec && rec.Status == StatusMissing {
		rec.Status = StatusReady
	}
	m.records[sp.ID] = rec
	m.revision++
	recs := m.sortedRecordsLocked()
	onChange := m.handlers.OnChange
	m.mu.Unlock()

	if err := m.vault.SaveRecords(recs); err != nil {
		return Record{}, err
	}
	if onChange != nil {
		onChange(sp.ID)
	}
	return rec, nil
}

// SaveSecret validates origin scoping and stores a secret payload in the encrypted vault.
func (m *Manager) SaveSecret(id string, p SecretPayload) (Record, error) {
	if m == nil {
		return Record{}, errors.New("cred: manager is nil")
	}
	id = strings.TrimSpace(id)

	m.mu.RLock()
	rec, ok := m.records[id]
	m.mu.RUnlock()
	if !ok {
		return Record{}, fmt.Errorf("cred: unknown credential %q", id)
	}

	if p.Type == "" {
		p.Type = rec.Spec.Type
	}
	if p.UpdatedAt == 0 {
		p.UpdatedAt = time.Now().UnixMilli()
	}
	if p.Type == TypeCookie || len(p.Cookies) > 0 {
		selected := rec.Spec.SelectCookies(p.Cookies, false)
		if len(selected) == 0 {
			return Record{}, errors.New("cred: no matching cookies to save")
		}
		p.Cookies = selected
	}
	if err := m.vault.SetSecret(id, p); err != nil {
		return Record{}, err
	}

	m.mu.Lock()
	rec = m.records[id]
	m.secrets[id] = p
	delete(m.cooldownUntil, id)
	rec.Status = StatusReady
	rec.LastError = ""
	rec.LastStatusCode = 0
	if p.Provider != "" {
		rec.LastProvider = p.Provider
	}
	rec.StorageBackend = m.vault.BackendName()
	syncRecordSummary(&rec, p, true)
	m.records[id] = rec
	m.revision++
	recs := m.sortedRecordsLocked()
	onChange := m.handlers.OnChange
	m.mu.Unlock()

	if err := m.vault.SaveRecords(recs); err != nil {
		return Record{}, err
	}
	if onChange != nil {
		onChange(id)
	}
	return rec, nil
}

// ImportManual parses manual input (Cookie, Netscape, curl, Bearer, Token, Basic)
// scoped to the declared credential and stores it in the encrypted vault.
func (m *Manager) ImportManual(id, format, input, username, password, headerName, headerPrefix string) (Record, error) {
	rec, ok := m.Get(id)
	if !ok {
		return Record{}, fmt.Errorf("cred: unknown credential %q", id)
	}
	payload, err := ParseManualImport(rec.Spec, format, input, username, password, headerName, headerPrefix)
	if err != nil {
		return Record{}, err
	}
	return m.SaveSecret(id, payload)
}

// ClearSecret wipes only the stored secret payload while preserving the credential spec.
func (m *Manager) ClearSecret(id string) error {
	if m == nil {
		return nil
	}
	id = strings.TrimSpace(id)
	m.Cancel(id)
	if err := m.vault.DeleteSecret(id); err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.secrets, id)
	delete(m.cooldownUntil, id)
	if rec, ok := m.records[id]; ok {
		rec.Status = StatusMissing
		rec.LastError = ""
		rec.LastStatusCode = 0
		syncRecordSummary(&rec, SecretPayload{}, false)
		m.records[id] = rec
	}
	m.revision++
	recs := m.sortedRecordsLocked()
	onChange := m.handlers.OnChange
	m.mu.Unlock()

	if err := m.vault.SaveRecords(recs); err != nil {
		return err
	}
	if onChange != nil {
		onChange(id)
	}
	return nil
}

// Delete removes both secret payload and metadata record for id.
func (m *Manager) Delete(id string) error {
	if m == nil {
		return nil
	}
	id = strings.TrimSpace(id)
	m.Cancel(id)
	if err := m.vault.DeleteSecret(id); err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.secrets, id)
	delete(m.records, id)
	delete(m.cooldownUntil, id)
	m.revision++
	recs := m.sortedRecordsLocked()
	onChange := m.handlers.OnChange
	m.mu.Unlock()

	if err := m.vault.SaveRecords(recs); err != nil {
		return err
	}
	if onChange != nil {
		onChange(id)
	}
	return nil
}

// ApplyRequest injects stored credentials for the matching spec in specs into req.
func (m *Manager) ApplyRequest(req *http.Request, specs []Spec) (*Spec, bool) {
	if m == nil || req == nil || req.URL == nil || len(specs) == 0 {
		return nil, false
	}
	reqURL := req.URL.String()

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, raw := range specs {
		sp := raw.Normalized()
		if rec, ok := m.records[sp.ID]; ok {
			sp = rec.Spec
		}
		if !sp.MatchesURL(reqURL) {
			continue
		}
		p, ok := m.secrets[sp.ID]
		if !ok {
			spCopy := sp
			return &spCopy, false
		}
		applySecretToRequest(req, sp, p)
		spCopy := sp
		return &spCopy, true
	}
	return nil, false
}

func applySecretToRequest(req *http.Request, sp Spec, p SecretPayload) {
	typ := p.Type
	if typ == "" {
		typ = sp.Type
	}
	switch typ {
	case TypeCookie:
		matching := matchingCookiesForURL(p.Cookies, req.URL.Hostname(), req.URL.Path, strings.EqualFold(req.URL.Scheme, "https"))
		if len(matching) > 0 {
			if merged := mergeCookieHeader(req.Header.Get("Cookie"), matching); merged != "" {
				req.Header.Set("Cookie", merged)
			}
		}
		if sp.CookieKey != "" && sp.HeaderName != "" {
			for _, c := range p.Cookies {
				if c.Name == sp.CookieKey && c.Value != "" {
					req.Header.Set(sp.HeaderName, sp.HeaderPrefix+c.Value)
					break
				}
			}
		}
	case TypeBearer:
		if strings.TrimSpace(p.Token) == "" {
			return
		}
		pref := p.HeaderPrefix
		if pref == "" {
			pref = "Bearer "
		}
		hName := p.HeaderName
		if hName == "" {
			hName = "Authorization"
		}
		req.Header.Set(hName, pref+strings.TrimSpace(p.Token))
	case TypeToken, TypeHeader:
		if strings.TrimSpace(p.Token) == "" {
			return
		}
		hName := p.HeaderName
		if hName == "" {
			hName = sp.HeaderName
		}
		if hName == "" {
			hName = "Authorization"
		}
		pref := p.HeaderPrefix
		if pref == "" {
			pref = sp.HeaderPrefix
		}
		req.Header.Set(hName, pref+p.Token)
	case TypeBasic:
		if p.Username != "" {
			req.SetBasicAuth(p.Username, p.Password)
		}
	}
}

func matchingCookiesForURL(cookies []StoredCookie, host, path string, isHTTPS bool) []StoredCookie {
	host = strings.ToLower(strings.TrimSpace(host))
	if path == "" {
		path = "/"
	}
	nowSec := time.Now().Unix()
	var strict, unexpired []StoredCookie
	for _, c := range cookies {
		if c.Name == "" {
			continue
		}
		if c.Expires > 0 && c.Expires <= nowSec {
			continue
		}
		unexpired = append(unexpired, c)
		if c.Secure && !isHTTPS {
			continue
		}
		if c.Domain != "" && c.Domain != "*" && !domainAppliesToHost(host, c.Domain) {
			continue
		}
		cp := c.Path
		if cp == "" {
			cp = "/"
		}
		if !strings.HasPrefix(path, cp) {
			continue
		}
		strict = append(strict, c)
	}
	if len(strict) > 0 {
		return strict
	}
	return unexpired
}

func mergeCookieHeader(existing string, fresh []StoredCookie) string {
	type pair struct {
		name string
		val  string
	}
	var order []pair
	index := map[string]int{}
	add := func(name, val string, override bool) {
		if name == "" {
			return
		}
		if idx, exists := index[name]; exists {
			if override {
				order[idx].val = val
			}
			return
		}
		index[name] = len(order)
		order = append(order, pair{name: name, val: val})
	}
	for _, part := range strings.Split(existing, ";") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok {
			add(strings.TrimSpace(k), strings.TrimSpace(v), false)
		}
	}
	for _, c := range fresh {
		add(c.Name, c.Value, true)
	}
	if len(order) == 0 {
		return ""
	}
	parts := make([]string, 0, len(order))
	for _, kv := range order {
		parts = append(parts, kv.name+"="+kv.val)
	}
	return strings.Join(parts, "; ")
}

// IsAuthenticating reports whether a credential currently has an active
// in-flight login or Ego/CDP cookie sync.
func (m *Manager) IsAuthenticating(id string) bool {
	if m == nil {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.inFlight[strings.TrimSpace(id)] != nil
}

// Cancel terminates an active in-flight login or cookie sync for id.
func (m *Manager) Cancel(id string) bool {
	if m == nil {
		return false
	}
	id = strings.TrimSpace(id)
	m.mu.Lock()
	infl := m.inFlight[id]
	var cancel context.CancelFunc
	if infl != nil {
		infl.cancelledByUser = true
		cancel = infl.cancel
	}
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return infl != nil
}

// CancelAll terminates every active in-flight credential login or cookie sync.
func (m *Manager) CancelAll() int {
	if m == nil {
		return 0
	}
	m.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(m.inFlight))
	count := len(m.inFlight)
	for _, infl := range m.inFlight {
		infl.cancelledByUser = true
		if infl.cancel != nil {
			cancels = append(cancels, infl.cancel)
		}
	}
	m.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
	return count
}

// SyncCookies executes a tracked, cancellable Ego/CDP cookie sync for an
// existing credential without overwriting its saved provider or CDPURL.
func (m *Manager) SyncCookies(ctx context.Context, id, provider, cdpURL string) (Record, error) {
	if m == nil {
		return Record{}, errors.New("cred: manager is nil")
	}
	id = strings.TrimSpace(id)
	rec, ok := m.Get(id)
	if !ok {
		return Record{}, fmt.Errorf("cred: unknown credential %q", id)
	}
	prov := strings.ToLower(strings.TrimSpace(provider))
	if prov == "" {
		prov = ProviderEgo
	}
	if prov != ProviderEgo && prov != ProviderCDP {
		return Record{}, fmt.Errorf("cred: SyncCookies supports only ego or cdp, got %q", prov)
	}
	sp := rec.Spec
	sp.Provider = prov
	if cu := strings.TrimSpace(cdpURL); cu != "" {
		sp.CDPURL = cu
	}
	if _, err := m.Acquire(ctx, sp, 0, true); err != nil {
		return Record{}, err
	}
	updated, _ := m.Get(id)
	return updated, nil
}

// Acquire executes the configured credential provider with single-flight
// deduplication and failure/cancel cooldown protection.
func (m *Manager) Acquire(ctx context.Context, rawSpec Spec, triggerStatus int, forceInteractive bool) (SecretPayload, error) {
	if m == nil {
		return SecretPayload{}, errors.New("cred: manager is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	sp := rawSpec.Normalized()
	if err := sp.Validate(); err != nil {
		return SecretPayload{}, err
	}
	if _, exists := m.Get(sp.ID); !exists {
		m.RegisterSpecs(sp.SourcePlugin, []Spec{sp})
	}

	m.mu.Lock()
	if rec, ok := m.records[sp.ID]; ok {
		runSpec := rec.Spec
		if strings.TrimSpace(rawSpec.Provider) != "" {
			runSpec.Provider = strings.ToLower(strings.TrimSpace(rawSpec.Provider))
		}
		if strings.TrimSpace(rawSpec.CDPURL) != "" {
			runSpec.CDPURL = strings.TrimRight(strings.TrimSpace(rawSpec.CDPURL), "/")
		}
		sp = runSpec
	}

	if infl, busy := m.inFlight[sp.ID]; busy {
		m.mu.Unlock()
		select {
		case <-infl.done:
			return infl.payload, infl.err
		case <-ctx.Done():
			return SecretPayload{}, ctx.Err()
		}
	}

	if !forceInteractive {
		if until, cooling := m.cooldownUntil[sp.ID]; cooling && time.Now().Before(until) {
			lastErr := ""
			if rec, ok := m.records[sp.ID]; ok {
				lastErr = rec.LastError
			}
			m.mu.Unlock()
			if lastErr == "" {
				lastErr = "credential refresh cooling down"
			}
			return SecretPayload{}, errors.New(lastErr)
		}
	}

	if sp.Provider == ProviderManual {
		if rec, ok := m.records[sp.ID]; ok {
			rec.Status = StatusExpired
			rec.LastStatusCode = triggerStatus
			rec.LastError = "credential requires manual import in Credentials settings"
			m.records[sp.ID] = rec
		}
		recs := m.sortedRecordsLocked()
		m.mu.Unlock()
		_ = m.vault.SaveRecords(recs)
		return SecretPayload{}, errors.New("credential requires manual import")
	}

	handlers := m.handlers
	timeout := m.InteractiveTimeout
	if timeout <= 0 {
		timeout = defaultInteractiveTimeout
	}
	cooldown := m.Cooldown
	acquireCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	infl := &inflightAuth{
		done:      make(chan struct{}),
		cancel:    cancel,
		provider:  sp.Provider,
		startedAt: time.Now(),
	}
	m.inFlight[sp.ID] = infl
	if rec, ok := m.records[sp.ID]; ok {
		rec.Status = StatusAuthenticating
		rec.LastStatusCode = triggerStatus
		rec.LastError = ""
		m.records[sp.ID] = rec
	}
	m.revision++
	onStartChange := m.handlers.OnChange
	m.mu.Unlock()

	if onStartChange != nil {
		onStartChange(sp.ID)
	}

	payload, err := dispatchProvider(acquireCtx, sp, handlers)
	if err == nil {
		_, saveErr := m.SaveSecret(sp.ID, payload)
		if saveErr != nil {
			err = saveErr
		} else {
			m.mu.Lock()
			delete(m.inFlight, sp.ID)
			infl.payload = payload
			infl.err = nil
			close(infl.done)
			onChange := m.handlers.OnChange
			m.mu.Unlock()
			if onChange != nil {
				onChange(sp.ID)
			}
			return payload, nil
		}
	}

	m.mu.Lock()
	cancelled := infl.cancelledByUser || errors.Is(err, context.Canceled) || errors.Is(acquireCtx.Err(), context.Canceled)
	if cancelled {
		if !errors.Is(err, context.Canceled) {
			err = fmt.Errorf("cred: cancelled: %w", context.Canceled)
		}
		if rec, ok := m.records[sp.ID]; ok {
			if rec.HasSecret {
				rec.Status = StatusReady
			} else {
				rec.Status = StatusMissing
			}
			rec.LastError = ""
			m.records[sp.ID] = rec
		}
	} else {
		if cooldown > 0 {
			m.cooldownUntil[sp.ID] = time.Now().Add(cooldown)
		}
		if rec, ok := m.records[sp.ID]; ok {
			if rec.HasSecret {
				rec.Status = StatusExpired
			} else {
				rec.Status = StatusError
			}
			rec.LastStatusCode = triggerStatus
			rec.LastError = err.Error()
			m.records[sp.ID] = rec
		}
	}
	recs := m.sortedRecordsLocked()
	m.revision++
	delete(m.inFlight, sp.ID)
	infl.payload = payload
	infl.err = err
	close(infl.done)
	onChange := m.handlers.OnChange
	m.mu.Unlock()

	_ = m.vault.SaveRecords(recs)
	if onChange != nil {
		onChange(sp.ID)
	}
	return payload, err
}

func dispatchProvider(ctx context.Context, sp Spec, h LoginHandlers) (SecretPayload, error) {
	if ctx != nil && ctx.Err() != nil {
		return SecretPayload{}, ctx.Err()
	}
	switch sp.Provider {
	case ProviderWebView:
		if h.WebView != nil {
			return h.WebView(ctx, sp)
		}
		if h.Browser != nil {
			return h.Browser(ctx, sp)
		}
		return SecretPayload{}, errors.New("webview login is only available in desktop UI mode")
	case ProviderBrowser:
		if h.Browser != nil {
			return h.Browser(ctx, sp)
		}
		return SecretPayload{}, errors.New("system browser login handler is unavailable")
	case ProviderEgo:
		if h.Ego != nil {
			return h.Ego(ctx, sp)
		}
		return FetchEgoCookies(ctx, sp)
	case ProviderCDP:
		if h.CDP != nil {
			return h.CDP(ctx, sp)
		}
		return FetchCDPCookies(ctx, sp.CDPURL, sp)
	default:
		return SecretPayload{}, fmt.Errorf("unsupported provider %q", sp.Provider)
	}
}

// AuthTransport wraps an http.RoundTripper to inject stored credentials, intercept
// configured auth failures (401/403), run single-flight host authentication, and
// retry the original HTTP request once.
type AuthTransport struct {
	Base               http.RoundTripper
	Manager            *Manager
	Specs              []Spec
	AttemptTimeout     time.Duration
	InteractiveTimeout time.Duration
	OnAcquired         func(Spec, SecretPayload)
}

// HandlesPerAttemptTimeout signals callers (such as engine.FetchRequests) that
// AuthTransport manages per-HTTP-attempt deadlines itself so interactive
// logins can run past short single-request timeouts.
func (t *AuthTransport) HandlesPerAttemptTimeout() bool { return true }

// WrapClient clones base with AuthTransport scoped to specs. It moves base.Timeout
// into per-attempt timeouts so interactive WebView/Browser logins are not
// prematurely cancelled by http.Client.Timeout.
func (m *Manager) WrapClient(base *http.Client, specs []Spec, onAcquired func(Spec, SecretPayload)) *http.Client {
	if base == nil {
		base = &http.Client{Timeout: defaultAttemptTimeout}
	}
	clone := *base
	attemptTimeout := clone.Timeout
	if attemptTimeout <= 0 {
		attemptTimeout = defaultAttemptTimeout
	}
	clone.Timeout = 0
	baseRT := clone.Transport
	if baseRT == nil {
		baseRT = http.DefaultTransport
	}
	interTimeout := defaultInteractiveTimeout
	if m != nil && m.InteractiveTimeout > 0 {
		interTimeout = m.InteractiveTimeout
	}
	specsCopy := make([]Spec, len(specs))
	for i, s := range specs {
		specsCopy[i] = s.Normalized()
	}
	clone.Transport = &AuthTransport{
		Base:               baseRT,
		Manager:            m,
		Specs:              specsCopy,
		AttemptTimeout:     attemptTimeout,
		InteractiveTimeout: interTimeout,
		OnAcquired:         onAcquired,
	}
	return &clone
}

func (t *AuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	baseRT := t.Base
	if baseRT == nil {
		baseRT = http.DefaultTransport
	}
	if req == nil || t.Manager == nil || len(t.Specs) == 0 {
		return baseRT.RoundTrip(req)
	}

	var bodyBuf []byte
	var canReplayBody bool
	if req.Body != nil && req.GetBody == nil {
		limited := io.LimitReader(req.Body, maxReplayRequestBody+1)
		b, err := io.ReadAll(limited)
		_ = req.Body.Close()
		if err != nil {
			return nil, err
		}
		if len(b) <= maxReplayRequestBody {
			bodyBuf = b
			canReplayBody = true
			req.Body = io.NopCloser(bytes.NewReader(bodyBuf))
		} else {
			req.Body = io.NopCloser(io.MultiReader(bytes.NewReader(b), req.Body))
		}
	} else {
		canReplayBody = true
	}

	req1, cancel1 := t.prepareAttempt(req, req.Context(), bodyBuf, canReplayBody)
	resp, err := baseRT.RoundTrip(req1)
	if err != nil {
		cancel1()
		return nil, err
	}

	matchedSpec, ok := t.matchAuthFailure(req.URL.String(), resp.StatusCode)
	if !ok || !canReplayBody {
		resp.Body = &cancelOnCloseReadCloser{ReadCloser: resp.Body, cancel: cancel1}
		return resp, nil
	}

	// Buffer a small portion of the auth-failure body so if interactive auth is
	// cancelled/unavailable/manual we still return the original HTTP response cleanly.
	errBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxBufferedErrorBody))
	_ = resp.Body.Close()
	cancel1()

	interTimeout := t.InteractiveTimeout
	if interTimeout <= 0 {
		interTimeout = defaultInteractiveTimeout
	}
	acquireCtx, acquireCancel := context.WithTimeout(context.WithoutCancel(req.Context()), interTimeout)
	payload, acqErr := t.Manager.Acquire(acquireCtx, matchedSpec, resp.StatusCode, false)
	acquireCancel()
	if acqErr != nil {
		resp.Body = io.NopCloser(bytes.NewReader(errBody))
		return resp, nil
	}

	if t.OnAcquired != nil {
		t.OnAcquired(matchedSpec, payload)
	}

	// Retry original request EXACTLY ONCE with newly acquired credentials.
	req2, cancel2 := t.prepareAttempt(req, context.WithoutCancel(req.Context()), bodyBuf, canReplayBody)
	resp2, err2 := baseRT.RoundTrip(req2)
	if err2 != nil {
		cancel2()
		return nil, err2
	}
	resp2.Body = &cancelOnCloseReadCloser{ReadCloser: resp2.Body, cancel: cancel2}
	return resp2, nil
}

func (t *AuthTransport) prepareAttempt(orig *http.Request, parentCtx context.Context, bodyBuf []byte, canReplay bool) (*http.Request, context.CancelFunc) {
	timeout := t.AttemptTimeout
	if timeout <= 0 {
		timeout = defaultAttemptTimeout
	}
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	r := orig.Clone(ctx)
	if orig.Body != nil {
		if orig.GetBody != nil {
			if nb, err := orig.GetBody(); err == nil {
				r.Body = nb
			}
		} else if canReplay && bodyBuf != nil {
			r.Body = io.NopCloser(bytes.NewReader(bodyBuf))
		}
	}
	t.Manager.ApplyRequest(r, t.Specs)
	return r, cancel
}

func (t *AuthTransport) matchAuthFailure(rawURL string, statusCode int) (Spec, bool) {
	t.Manager.mu.RLock()
	defer t.Manager.mu.RUnlock()
	for _, raw := range t.Specs {
		sp := raw.Normalized()
		if rec, ok := t.Manager.records[sp.ID]; ok {
			sp = rec.Spec
		}
		if sp.MatchesURL(rawURL) && sp.ShouldRefreshStatus(statusCode) {
			return sp, true
		}
	}
	return Spec{}, false
}

type cancelOnCloseReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnCloseReadCloser) Close() error {
	err := c.ReadCloser.Close()
	if c.cancel != nil {
		c.cancel()
	}
	return err
}

func (m *Manager) sortedRecordsLocked() []Record {
	out := make([]Record, 0, len(m.records))
	for _, r := range m.records {
		out = append(out, r)
	}
	sort.Slice(out, func(i, k int) bool {
		return out[i].Spec.ID < out[k].Spec.ID
	})
	return out
}

func syncRecordSummary(r *Record, p SecretPayload, hasSec bool) {
	r.HasSecret = hasSec
	if !hasSec {
		r.CookieCount = 0
		r.CookieNames = nil
		r.TokenPreview = ""
		return
	}
	r.UpdatedAt = p.UpdatedAt
	r.CookieCount = len(p.Cookies)
	if len(p.Cookies) > 0 {
		names := make([]string, 0, len(p.Cookies))
		for _, c := range p.Cookies {
			if c.Name != "" && !slices.Contains(names, c.Name) {
				names = append(names, c.Name)
			}
		}
		sort.Strings(names)
		r.CookieNames = names
	} else {
		r.CookieNames = nil
	}
	switch {
	case p.Token != "":
		r.TokenPreview = RedactToken(p.Token)
	case p.Username != "":
		r.TokenPreview = p.Username + ":•••"
	default:
		r.TokenPreview = ""
	}
}

func specsEqual(a, b Spec) bool {
	if a.ID != b.ID || a.Name != b.Name || a.Type != b.Type || a.Origin != b.Origin ||
		a.Provider != b.Provider || a.LoginURL != b.LoginURL || a.CookieKey != b.CookieKey ||
		a.HeaderName != b.HeaderName || a.HeaderPrefix != b.HeaderPrefix || a.CDPURL != b.CDPURL ||
		a.SourcePlugin != b.SourcePlugin {
		return false
	}
	if !slices.Equal(a.RefreshOn, b.RefreshOn) || !slices.Equal(a.Cookies, b.Cookies) ||
		!slices.Equal(a.effectiveDomains(), b.effectiveDomains()) {
		return false
	}
	if (a.Ready == nil) != (b.Ready == nil) {
		return false
	}
	if a.Ready != nil && b.Ready != nil {
		if a.Ready.URLIncludes != b.Ready.URLIncludes ||
			a.Ready.TitleIncludes != b.Ready.TitleIncludes ||
			!slices.Equal(a.Ready.HasCookies, b.Ready.HasCookies) {
			return false
		}
	}
	return true
}
