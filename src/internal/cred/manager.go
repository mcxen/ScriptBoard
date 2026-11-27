package cred

import (
	"bytes"
	"context"
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
	defaultFailCooldown       = 15 * time.Second
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
	done    chan struct{}
	payload SecretPayload
	err     error
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
		if r.Status == StatusAuthenticating {
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
		filtered := rec.Spec.FilterAuthorizedCookies(p.Cookies)
		if len(filtered) == 0 {
			return Record{}, fmt.Errorf("cred: no cookies match authorized origin %s", rec.Spec.Origin)
		}
		p.Cookies = filtered
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
		if len(matching) == 0 {
			return
		}
		merged := mergeCookieHeader(req.Header.Get("Cookie"), matching)
		if merged != "" {
			req.Header.Set("Cookie", merged)
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
	var out []StoredCookie
	for _, c := range cookies {
		if c.Name == "" {
			continue
		}
		if c.Expires > 0 && c.Expires <= nowSec {
			continue
		}
		if c.Secure && !isHTTPS {
			continue
		}
		if !domainAppliesToHost(host, c.Domain) {
			continue
		}
		cp := c.Path
		if cp == "" {
			cp = "/"
		}
		if !strings.HasPrefix(path, cp) {
			continue
		}
		out = append(out, c)
	}
	return out
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

// Acquire executes the configured credential provider with single-flight
// deduplication and failure/cancel cooldown protection.
func (m *Manager) Acquire(ctx context.Context, rawSpec Spec, triggerStatus int, forceInteractive bool) (SecretPayload, error) {
	if m == nil {
		return SecretPayload{}, errors.New("cred: manager is nil")
	}
	sp := rawSpec.Normalized()
	if err := sp.Validate(); err != nil {
		return SecretPayload{}, err
	}
	m.RegisterSpecs(sp.SourcePlugin, []Spec{sp})

	m.mu.Lock()
	if rec, ok := m.records[sp.ID]; ok && rawSpec.Provider == "" {
		sp = rec.Spec
	} else if rawSpec.Provider != "" {
		sp.Provider = strings.ToLower(strings.TrimSpace(rawSpec.Provider))
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

	infl := &inflightAuth{done: make(chan struct{})}
	m.inFlight[sp.ID] = infl
	if rec, ok := m.records[sp.ID]; ok {
		rec.Status = StatusAuthenticating
		rec.LastStatusCode = triggerStatus
		rec.LastError = ""
		m.records[sp.ID] = rec
	}
	handlers := m.handlers
	timeout := m.InteractiveTimeout
	if timeout <= 0 {
		timeout = defaultInteractiveTimeout
	}
	cooldown := m.Cooldown
	m.mu.Unlock()

	if ctx == nil {
		ctx = context.Background()
	}
	acquireCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	payload, err := dispatchProvider(acquireCtx, sp, handlers)
	if err == nil {
		_, saveErr := m.SaveSecret(sp.ID, payload)
		if saveErr != nil {
			err = saveErr
		}
	} else {
		m.mu.Lock()
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
		recs := m.sortedRecordsLocked()
		m.revision++
		m.mu.Unlock()
		_ = m.vault.SaveRecords(recs)
	}

	m.mu.Lock()
	delete(m.inFlight, sp.ID)
	infl.payload = payload
	infl.err = err
	close(infl.done)
	m.mu.Unlock()

	return payload, err
}

func dispatchProvider(ctx context.Context, sp Spec, h LoginHandlers) (SecretPayload, error) {
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
		var egoErr error
		if h.Ego != nil {
			p, err := h.Ego(ctx, sp)
			if err == nil {
				return p, nil
			}
			egoErr = err
		} else {
			p, err := FetchEgoCookies(ctx, sp)
			if err == nil {
				return p, nil
			}
			egoErr = err
		}
		if h.WebView != nil {
			return h.WebView(ctx, sp)
		}
		return SecretPayload{}, egoErr
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
		a.Provider != b.Provider || a.LoginURL != b.LoginURL || a.HeaderName != b.HeaderName ||
		a.HeaderPrefix != b.HeaderPrefix || a.CDPURL != b.CDPURL || a.SourcePlugin != b.SourcePlugin {
		return false
	}
	if !slices.Equal(a.RefreshOn, b.RefreshOn) || !slices.Equal(a.AllowedDomains, b.AllowedDomains) {
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
