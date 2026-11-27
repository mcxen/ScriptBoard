// Package cred provides a unified, host-managed credential lifecycle for
// ScriptBoard plugins and .bar components: OS Keychain-backed AES-256-GCM
// vault storage, restricted WebView SSO login, system browser loopback
// handoff, manual multi-format import (Cookie/curl/Netscape/Bearer/Token/Basic),
// optional Ego/CDP cookie extraction, and single-flight 401/403 auto-refresh.
package cred

import (
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	TypeCookie = "cookie"
	TypeBearer = "bearer"
	TypeToken  = "token"
	TypeBasic  = "basic"
	TypeHeader = "header"

	ProviderWebView = "webview"
	ProviderBrowser = "browser"
	ProviderManual  = "manual"
	ProviderEgo     = "ego"
	ProviderCDP     = "cdp"

	StatusMissing        = "missing"
	StatusReady          = "ready"
	StatusExpired        = "expired"
	StatusAuthenticating = "authenticating"
	StatusError          = "error"
)

// ReadyCondition defines when an interactive login (WebView or CDP) has
// succeeded. URLIncludes and TitleIncludes match substrings; HasCookies
// requires every listed cookie name to exist in the session.
type ReadyCondition struct {
	URLIncludes   string   `json:"urlIncludes,omitempty"`
	HasCookies    []string `json:"hasCookies,omitempty"`
	TitleIncludes string   `json:"titleIncludes,omitempty"`
}

// Satisfied reports whether the current navigation state and captured cookies
// satisfy the readiness rule.
func (rc *ReadyCondition) Satisfied(currentURL, title string, cookies []StoredCookie) bool {
	if rc == nil || (strings.TrimSpace(rc.URLIncludes) == "" &&
		strings.TrimSpace(rc.TitleIncludes) == "" &&
		len(rc.HasCookies) == 0) {
		return len(cookies) > 0
	}
	if sub := strings.TrimSpace(rc.URLIncludes); sub != "" {
		if !strings.Contains(currentURL, sub) {
			return false
		}
	}
	if sub := strings.TrimSpace(rc.TitleIncludes); sub != "" {
		if !strings.Contains(strings.ToLower(title), strings.ToLower(sub)) {
			return false
		}
	}
	if len(rc.HasCookies) > 0 {
		have := make(map[string]string, len(cookies))
		for _, c := range cookies {
			if c.Name != "" && c.Value != "" {
				have[c.Name] = c.Value
			}
		}
		for _, reqName := range rc.HasCookies {
			reqName = strings.TrimSpace(reqName)
			if reqName == "" {
				continue
			}
			if have[reqName] == "" {
				return false
			}
		}
	}
	return true
}

// Spec declares a credential requirement attached to a JS script, .bar component,
// or user-defined credential entry.
type Spec struct {
	ID             string          `json:"id"`
	Name           string          `json:"name,omitempty"`
	Type           string          `json:"type"`                     // cookie | bearer | token | basic | header
	Origin         string          `json:"origin,omitempty"`         // https://host[:port] (optional for manual/template usage)
	Provider       string          `json:"provider,omitempty"`       // webview | browser | manual | ego | cdp
	LoginURL       string          `json:"loginUrl,omitempty"`       // interactive sign-in URL
	Ready          *ReadyCondition `json:"ready,omitempty"`          // completion criteria
	RefreshOn      []int           `json:"refreshOn,omitempty"`      // HTTP status codes triggering refresh (default [401, 403])
	Cookies        []string        `json:"cookies,omitempty"`        // optional cookie names developer picks (empty keeps all)
	CookieKey      string          `json:"cookieKey,omitempty"`      // optional cookie name to map into HeaderName
	HeaderName     string          `json:"headerName,omitempty"`     // header name for type=token|header or CookieKey
	HeaderPrefix   string          `json:"headerPrefix,omitempty"`   // prefix before token/cookie value (e.g. "Bearer ")
	CDPURL         string          `json:"cdpUrl,omitempty"`         // optional CDP HTTP endpoint (default http://127.0.0.1:9222)
	Domains        []string        `json:"domains,omitempty"`        // extra domains for URL/CDP matching
	AllowedDomains []string        `json:"allowedDomains,omitempty"` // legacy alias of Domains
	SourcePlugin   string          `json:"sourcePlugin,omitempty"`   // plugin or .bar ID that declared it
}

// StoredCookie holds one authorized HTTP cookie extracted from login or import.
type StoredCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path,omitempty"`
	Expires  int64  `json:"expires,omitempty"` // Unix seconds; 0 for session
	Secure   bool   `json:"secure,omitempty"`
	HTTPOnly bool   `json:"httpOnly,omitempty"`
	SameSite string `json:"sameSite,omitempty"`
}

// SecretPayload holds sensitive credential values stored exclusively inside the
// AES-256-GCM Keychain-encrypted vault (never written to plaintext JSON).
type SecretPayload struct {
	Type         string         `json:"type"`
	Cookies      []StoredCookie `json:"cookies,omitempty"`
	Token        string         `json:"token,omitempty"`
	Username     string         `json:"username,omitempty"`
	Password     string         `json:"password,omitempty"`
	HeaderName   string         `json:"headerName,omitempty"`
	HeaderPrefix string         `json:"headerPrefix,omitempty"`
	Provider     string         `json:"provider,omitempty"`
	UpdatedAt    int64          `json:"updatedAt,omitempty"`
}

// Record holds non-secret metadata persisted in credentials.json and returned
// to UI views and read-only MCP endpoints.
type Record struct {
	Spec           Spec     `json:"spec"`
	Status         string   `json:"status"`
	HasSecret      bool     `json:"hasSecret"`
	StorageBackend string   `json:"storageBackend"`
	LastProvider   string   `json:"lastProvider,omitempty"`
	UpdatedAt      int64    `json:"updatedAt,omitempty"`
	CookieCount    int      `json:"cookieCount,omitempty"`
	CookieNames    []string `json:"cookieNames,omitempty"`
	TokenPreview   string   `json:"tokenPreview,omitempty"`
	LastError      string   `json:"lastError,omitempty"`
	LastStatusCode int      `json:"lastStatusCode,omitempty"`
}

// Status is the redacted runtime status summary.
type Status struct {
	ID          string   `json:"id"`
	Name        string   `json:"name,omitempty"`
	Type        string   `json:"type"`
	Origin      string   `json:"origin"`
	Provider    string   `json:"provider"`
	Status      string   `json:"status"`
	HasSecret   bool     `json:"hasSecret"`
	CookieCount int      `json:"cookieCount,omitempty"`
	CookieNames []string `json:"cookieNames,omitempty"`
	UpdatedAt   int64    `json:"updatedAt,omitempty"`
	LastError   string   `json:"lastError,omitempty"`
}

// RuntimeData is the complete developer-facing runtime object passed into
// script ctx.credentials[id] and .bar $metrix.credentials.get(id), letting
// plugin authors read whichever cookie or token fields they need directly.
type RuntimeData struct {
	ID           string            `json:"id"`
	Name         string            `json:"name,omitempty"`
	Type         string            `json:"type"`
	Origin       string            `json:"origin,omitempty"`
	Provider     string            `json:"provider"`
	Status       string            `json:"status"`
	HasSecret    bool              `json:"hasSecret"`
	CookieCount  int               `json:"cookieCount,omitempty"`
	CookieNames  []string          `json:"cookieNames,omitempty"`
	Cookies      map[string]string `json:"cookies,omitempty"`
	CookieHeader string            `json:"cookieHeader,omitempty"`
	Token        string            `json:"token,omitempty"`
	Username     string            `json:"username,omitempty"`
	HeaderName   string            `json:"headerName,omitempty"`
	HeaderValue  string            `json:"headerValue,omitempty"`
	UpdatedAt    int64             `json:"updatedAt,omitempty"`
	LastError    string            `json:"lastError,omitempty"`
}

// Normalized returns a copy of s with canonical defaults and trimmed fields.
func (s Spec) Normalized() Spec {
	out := s
	out.ID = strings.TrimSpace(out.ID)
	out.Name = strings.TrimSpace(out.Name)
	out.Type = strings.ToLower(strings.TrimSpace(out.Type))
	if out.Type == "" {
		out.Type = TypeCookie
	}
	out.Origin = strings.TrimRight(strings.TrimSpace(out.Origin), "/")
	if u, err := url.Parse(out.Origin); err == nil && u.Scheme != "" && u.Host != "" {
		out.Origin = strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host)
	}
	out.Provider = strings.ToLower(strings.TrimSpace(out.Provider))
	if out.Provider == "" {
		out.Provider = ProviderWebView
	}
	out.LoginURL = strings.TrimSpace(out.LoginURL)
	out.CookieKey = strings.TrimSpace(out.CookieKey)
	out.HeaderName = strings.TrimSpace(out.HeaderName)
	out.CDPURL = strings.TrimRight(strings.TrimSpace(out.CDPURL), "/")
	out.SourcePlugin = strings.TrimSpace(out.SourcePlugin)

	if len(out.RefreshOn) == 0 {
		out.RefreshOn = []int{401, 403}
	} else {
		cp := make([]int, 0, len(out.RefreshOn))
		for _, code := range out.RefreshOn {
			if code >= 100 && code <= 599 && !slices.Contains(cp, code) {
				cp = append(cp, code)
			}
		}
		if len(cp) == 0 {
			cp = []int{401, 403}
		}
		out.RefreshOn = cp
	}

	if out.Ready != nil {
		rc := *out.Ready
		rc.URLIncludes = strings.TrimSpace(rc.URLIncludes)
		rc.TitleIncludes = strings.TrimSpace(rc.TitleIncludes)
		var ck []string
		for _, n := range rc.HasCookies {
			if n = strings.TrimSpace(n); n != "" && !slices.Contains(ck, n) {
				ck = append(ck, n)
			}
		}
		rc.HasCookies = ck
		out.Ready = &rc
	}

	if len(out.Cookies) > 0 {
		var pick []string
		for _, n := range out.Cookies {
			if n = strings.TrimSpace(n); n != "" && !slices.Contains(pick, n) {
				pick = append(pick, n)
			}
		}
		out.Cookies = pick
	}

	var doms []string
	for _, rawList := range [][]string{out.Domains, out.AllowedDomains} {
		for _, d := range rawList {
			d = strings.ToLower(strings.TrimSpace(d))
			if d != "" && !slices.Contains(doms, d) {
				doms = append(doms, d)
			}
		}
	}
	if len(out.Domains) > 0 || len(doms) > 0 {
		out.Domains = doms
	}
	if len(out.AllowedDomains) > 0 {
		out.AllowedDomains = append([]string(nil), doms...)
	}

	return out
}

// Validate verifies that s has a safe id, valid type/provider, and valid http/https URLs when provided.
func (s Spec) Validate() error {
	s = s.Normalized()
	if s.ID == "" || utf8.RuneCountInString(s.ID) > 64 || strings.ContainsAny(s.ID, `/\`) || strings.Contains(s.ID, "..") {
		return fmt.Errorf("cred: invalid credential id %q", s.ID)
	}
	switch s.Type {
	case TypeCookie, TypeBearer, TypeToken, TypeBasic, TypeHeader:
	default:
		return fmt.Errorf("cred: unsupported credential type %q", s.Type)
	}
	switch s.Provider {
	case ProviderWebView, ProviderBrowser, ProviderManual, ProviderEgo, ProviderCDP:
	default:
		return fmt.Errorf("cred: unsupported provider %q", s.Provider)
	}
	if s.Origin != "" {
		u, err := url.Parse(s.Origin)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
			return fmt.Errorf("cred: invalid origin %q (must be http/https)", s.Origin)
		}
	}
	if s.LoginURL != "" {
		lu, err := url.Parse(s.LoginURL)
		if err != nil || (strings.ToLower(lu.Scheme) != "http" && strings.ToLower(lu.Scheme) != "https") || lu.Hostname() == "" {
			return fmt.Errorf("cred: invalid loginUrl %q (must be http/https)", s.LoginURL)
		}
	}
	return nil
}

func (s Spec) effectiveDomains() []string {
	var doms []string
	for _, list := range [][]string{s.Domains, s.AllowedDomains} {
		for _, d := range list {
			d = strings.ToLower(strings.TrimSpace(d))
			if d != "" && !slices.Contains(doms, d) {
				doms = append(doms, d)
			}
		}
	}
	return doms
}

// ShouldRefreshStatus reports whether statusCode should trigger credential refresh.
func (s Spec) ShouldRefreshStatus(statusCode int) bool {
	codes := s.RefreshOn
	if len(codes) == 0 {
		codes = []int{401, 403}
	}
	return slices.Contains(codes, statusCode)
}

// EffectiveLoginURL returns LoginURL when set, otherwise Origin.
func (s Spec) EffectiveLoginURL() string {
	if strings.TrimSpace(s.LoginURL) != "" {
		return strings.TrimSpace(s.LoginURL)
	}
	return s.Origin
}

// MatchesURL reports whether rawURL falls within s.Origin (exact scheme, host,
// effective port, or AllowedDomains / subdomain when Origin allows).
func (s Spec) MatchesURL(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return false
	}
	reqHost := strings.ToLower(u.Hostname())
	if reqHost == "" {
		return false
	}
	ou, err := url.Parse(s.Origin)
	if err != nil || ou.Hostname() == "" {
		return false
	}
	if strings.ToLower(ou.Scheme) == scheme && effectivePort(scheme, ou.Port()) == effectivePort(scheme, u.Port()) {
		origHost := strings.ToLower(ou.Hostname())
		if reqHost == origHost || domainAppliesToHost(reqHost, origHost) {
			return true
		}
	}
	for _, dom := range s.effectiveDomains() {
		if domainAppliesToHost(reqHost, dom) {
			return true
		}
	}
	return false
}

// FilterAuthorizedCookies selects cookies matching s.Origin / s.Domains (and
// optional developer-chosen cookie names in s.Cookies).
func (s Spec) FilterAuthorizedCookies(in []StoredCookie) []StoredCookie {
	return s.SelectCookies(in, true)
}

// SelectCookies normalizes cookies, optionally filters by s.Origin / s.Domains
// when filterDomain is true (e.g. from shared CDP/Ego browser stores), and picks
// only names in s.Cookies if the developer configured an explicit list.
func (s Spec) SelectCookies(in []StoredCookie, filterDomain bool) []StoredCookie {
	if len(in) == 0 {
		return nil
	}
	var origHost string
	if ou, err := url.Parse(s.Origin); err == nil && ou.Hostname() != "" {
		origHost = strings.ToLower(ou.Hostname())
	}
	doms := s.effectiveDomains()

	var pick map[string]bool
	if len(s.Cookies) > 0 {
		pick = make(map[string]bool, len(s.Cookies))
		for _, n := range s.Cookies {
			if n = strings.TrimSpace(n); n != "" {
				pick[n] = true
			}
		}
	}

	var out []StoredCookie
	for _, c := range in {
		c.Name = strings.TrimSpace(c.Name)
		if c.Name == "" {
			continue
		}
		if len(pick) > 0 && !pick[c.Name] {
			continue
		}
		dom := strings.ToLower(strings.TrimSpace(c.Domain))
		if dom == "" {
			dom = origHost
		}
		if filterDomain && (origHost != "" || len(doms) > 0) {
			if dom == "" || !isCookieDomainAuthorized(dom, origHost, doms) {
				continue
			}
		}
		if c.Path == "" || !strings.HasPrefix(c.Path, "/") {
			c.Path = "/"
		}
		c.Domain = dom
		out = append(out, c)
	}
	return out
}

// CookieMap builds a name -> value map and canonical Cookie header string.
func CookieMap(cookies []StoredCookie) (map[string]string, string) {
	if len(cookies) == 0 {
		return nil, ""
	}
	m := make(map[string]string, len(cookies))
	names := make([]string, 0, len(cookies))
	for _, c := range cookies {
		if c.Name == "" {
			continue
		}
		if _, exists := m[c.Name]; !exists {
			names = append(names, c.Name)
		}
		m[c.Name] = c.Value
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, name+"="+m[name])
	}
	return m, strings.Join(parts, "; ")
}

func isCookieDomainAuthorized(cookieDom, originHost string, allowedDomains []string) bool {
	cleanCookie := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(cookieDom)), ".")
	cleanOrig := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(originHost)), ".")
	if cleanCookie == "" || cleanOrig == "" {
		return false
	}
	if cleanCookie == cleanOrig {
		return true
	}
	if strings.Contains(cleanCookie, ".") && strings.HasSuffix(cleanOrig, "."+cleanCookie) {
		return true
	}
	if strings.Contains(cleanOrig, ".") && strings.HasSuffix(cleanCookie, "."+cleanOrig) {
		return true
	}
	for _, a := range allowedDomains {
		cleanAllowed := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(a)), ".")
		if cleanAllowed == "" {
			continue
		}
		if cleanCookie == cleanAllowed {
			return true
		}
		if strings.Contains(cleanAllowed, ".") && strings.HasSuffix(cleanCookie, "."+cleanAllowed) {
			return true
		}
		if strings.Contains(cleanCookie, ".") && strings.HasSuffix(cleanAllowed, "."+cleanCookie) {
			return true
		}
	}
	return false
}

func domainAppliesToHost(host, domain string) bool {
	host = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(host)), ".")
	dom := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(domain)), ".")
	if host == "" || dom == "" {
		return false
	}
	if host == dom {
		return true
	}
	return strings.Contains(dom, ".") && strings.HasSuffix(host, "."+dom)
}

func effectivePort(scheme, port string) string {
	if port != "" {
		return port
	}
	switch scheme {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

// RedactToken returns a safe preview of a token without revealing secret contents.
func RedactToken(tok string) string {
	t := strings.TrimSpace(tok)
	if t == "" {
		return ""
	}
	if len(t) <= 8 {
		return "•••"
	}
	return t[:2] + "•••" + t[len(t)-4:]
}

// ToStatus builds a non-secret Status summary from r.
func (r Record) ToStatus() Status {
	return Status{
		ID:          r.Spec.ID,
		Name:        r.Spec.Name,
		Type:        r.Spec.Type,
		Origin:      r.Spec.Origin,
		Provider:    r.Spec.Provider,
		Status:      r.Status,
		HasSecret:   r.HasSecret,
		CookieCount: r.CookieCount,
		CookieNames: append([]string(nil), r.CookieNames...),
		UpdatedAt:   r.UpdatedAt,
		LastError:   r.LastError,
	}
}
