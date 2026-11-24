package bar

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"scriptboard/src/internal/feed"
)

const maxStoredCookies = 512

// StoredCookie is a serializable HTTP cookie persisted per .bar component.
type StoredCookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	Domain   string    `json:"domain"`
	Path     string    `json:"path,omitempty"`
	Expires  time.Time `json:"expires,omitempty"`
	Secure   bool      `json:"secure,omitempty"`
	HttpOnly bool      `json:"httpOnly,omitempty"`
	SameSite string    `json:"sameSite,omitempty"`
	Updated  time.Time `json:"updated,omitempty"`
}

// CookieJar is a thread-safe, optionally disk-persisted http.CookieJar scoped
// to a single .bar component.
type CookieJar struct {
	mu      sync.RWMutex
	path    string
	cookies []StoredCookie
	now     func() time.Time
}

type cookieFile struct {
	Cookies []StoredCookie `json:"cookies"`
}

// NewCookieJar opens or creates a CookieJar backed by path (or in-memory when
// path is empty).
func NewCookieJar(path string) (*CookieJar, error) {
	j := &CookieJar{
		path: path,
		now:  time.Now,
	}
	if path != "" {
		if err := j.Reload(); err != nil {
			return nil, err
		}
	}
	return j, nil
}

// Reload reloads persisted cookies from disk. It is a no-op for in-memory jars.
func (j *CookieJar) Reload() error {
	if j == nil || j.path == "" {
		return nil
	}
	b, err := os.ReadFile(j.path)
	if errors.Is(err, fs.ErrNotExist) {
		j.mu.Lock()
		j.cookies = nil
		j.mu.Unlock()
		return nil
	}
	if err != nil {
		return err
	}
	trimmed := strings.TrimSpace(string(b))
	if trimmed == "" {
		j.mu.Lock()
		j.cookies = nil
		j.mu.Unlock()
		return nil
	}
	var list []StoredCookie
	if strings.HasPrefix(trimmed, "[") {
		if err := json.Unmarshal(b, &list); err != nil {
			return err
		}
	} else {
		var f cookieFile
		if err := json.Unmarshal(b, &f); err != nil {
			return err
		}
		list = f.Cookies
	}
	j.mu.Lock()
	j.cookies = pruneExpiredCookies(list, j.now())
	j.mu.Unlock()
	return nil
}

// SetCookies implements http.CookieJar.
func (j *CookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	if j == nil || u == nil || len(cookies) == 0 {
		return
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return
	}
	now := j.now()
	defPath := defaultCookiePath(u.Path)

	j.mu.Lock()
	defer j.mu.Unlock()

	changed := false
	for _, c := range cookies {
		if c == nil || strings.TrimSpace(c.Name) == "" {
			continue
		}
		dom := host
		if c.Domain != "" {
			d := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(c.Domain), "."))
			if d == "" {
				continue
			}
			if host != d && (!strings.Contains(d, ".") || !strings.HasSuffix(host, "."+d)) {
				continue
			}
			dom = "." + d
		}
		path := c.Path
		if path == "" || !strings.HasPrefix(path, "/") {
			path = defPath
		}
		var exp time.Time
		if c.MaxAge < 0 {
			j.removeLocked(dom, c.Name, path)
			changed = true
			continue
		}
		if c.MaxAge > 0 {
			exp = now.Add(time.Duration(c.MaxAge) * time.Second)
		} else if !c.Expires.IsZero() {
			exp = c.Expires
			if !exp.After(now) {
				j.removeLocked(dom, c.Name, path)
				changed = true
				continue
			}
		}
		sc := StoredCookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   dom,
			Path:     path,
			Expires:  exp,
			Secure:   c.Secure,
			HttpOnly: c.HttpOnly,
			SameSite: sameSiteString(c.SameSite),
			Updated:  now,
		}
		j.upsertLocked(sc)
		changed = true
	}
	if changed {
		j.cookies = pruneExpiredCookies(j.cookies, now)
		if len(j.cookies) > maxStoredCookies {
			j.cookies = j.cookies[len(j.cookies)-maxStoredCookies:]
		}
		_ = j.saveLocked()
	}
}

// Cookies implements http.CookieJar.
func (j *CookieJar) Cookies(u *url.URL) []*http.Cookie {
	if j == nil || u == nil {
		return nil
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return nil
	}
	reqPath := u.Path
	if reqPath == "" {
		reqPath = "/"
	}
	isHTTPS := strings.EqualFold(u.Scheme, "https")
	now := j.now()

	j.mu.RLock()
	var matched []StoredCookie
	for _, c := range j.cookies {
		if !c.Expires.IsZero() && !c.Expires.After(now) {
			continue
		}
		if c.Secure && !isHTTPS {
			continue
		}
		if !cookieMatchesHost(c.Domain, host) {
			continue
		}
		if !cookieMatchesPath(c.Path, reqPath) {
			continue
		}
		matched = append(matched, c)
	}
	j.mu.RUnlock()

	sort.SliceStable(matched, func(i, k int) bool {
		if len(matched[i].Path) != len(matched[k].Path) {
			return len(matched[i].Path) > len(matched[k].Path)
		}
		return matched[i].Name < matched[k].Name
	})

	out := make([]*http.Cookie, 0, len(matched))
	for _, c := range matched {
		out = append(out, &http.Cookie{Name: c.Name, Value: c.Value})
	}
	return out
}

// HeaderFor returns the "name=value; ..." Cookie header value for rawURL.
func (j *CookieJar) HeaderFor(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	cs := j.Cookies(u)
	if len(cs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(cs))
	for _, c := range cs {
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}

// List returns a snapshot copy of all non-expired cookies, sorted stably.
func (j *CookieJar) List() []StoredCookie {
	if j == nil {
		return nil
	}
	now := j.now()
	j.mu.RLock()
	out := pruneExpiredCookies(j.cookies, now)
	j.mu.RUnlock()
	sort.Slice(out, func(i, k int) bool {
		if out[i].Domain != out[k].Domain {
			return out[i].Domain < out[k].Domain
		}
		if out[i].Path != out[k].Path {
			return out[i].Path < out[k].Path
		}
		return out[i].Name < out[k].Name
	})
	return out
}

// Count returns the number of non-expired cookies.
func (j *CookieJar) Count() int {
	return len(j.List())
}

// Set inserts or replaces a cookie and persists to disk.
func (j *CookieJar) Set(c StoredCookie) error {
	if j == nil {
		return errors.New("cookie jar is nil")
	}
	c.Name = strings.TrimSpace(c.Name)
	c.Domain = strings.ToLower(strings.TrimSpace(c.Domain))
	if c.Name == "" {
		return errors.New("cookie name cannot be empty")
	}
	if c.Domain == "" {
		return errors.New("cookie domain cannot be empty")
	}
	if c.Path == "" || !strings.HasPrefix(c.Path, "/") {
		c.Path = "/"
	}
	now := j.now()
	if !c.Expires.IsZero() && !c.Expires.After(now) {
		_, err := j.Delete(c.Domain, c.Name, c.Path)
		return err
	}
	c.Updated = now

	j.mu.Lock()
	defer j.mu.Unlock()
	j.upsertLocked(c)
	j.cookies = pruneExpiredCookies(j.cookies, now)
	if len(j.cookies) > maxStoredCookies {
		j.cookies = j.cookies[len(j.cookies)-maxStoredCookies:]
	}
	return j.saveLocked()
}

// SetHeader parses a Cookie header string ("a=1; b=2" or "Cookie: a=1; b=2")
// and stores each pair as a host cookie for rawURL.
func (j *CookieJar) SetHeader(rawURL, cookieHeader string) (int, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return 0, errors.New("需要有效的 http/https 目标地址")
	}
	raw := strings.TrimSpace(cookieHeader)
	if len(raw) >= len("cookie:") && strings.EqualFold(raw[:len("cookie:")], "cookie:") {
		raw = strings.TrimSpace(raw[len("cookie:"):])
	}
	pairs := parseCookiePairs(raw)
	if len(pairs) == 0 {
		return 0, errors.New("没有解析到有效的 Cookie（格式：name=value; name2=value2）")
	}
	host := strings.ToLower(u.Hostname())
	secure := strings.EqualFold(u.Scheme, "https")
	for _, p := range pairs {
		if err := j.Set(StoredCookie{
			Name:   p[0],
			Value:  p[1],
			Domain: host,
			Path:   "/",
			Secure: secure,
		}); err != nil {
			return 0, err
		}
	}
	return len(pairs), nil
}

// ImportCurl parses a curl command (with -b / -H Cookie:) and saves its cookies
// under the curl request's target URL.
func (j *CookieJar) ImportCurl(curlCmd string) (string, int, error) {
	req, err := feed.ParseCurl(strings.TrimSpace(curlCmd))
	if err != nil {
		return "", 0, err
	}
	if len(req.Cookies) == 0 {
		return req.URL, 0, errors.New("curl 命令中没有找到 Cookie（请确认包含 -b 或 -H 'Cookie: ...'）")
	}
	u, err := url.Parse(req.URL)
	if err != nil || u.Hostname() == "" {
		return req.URL, 0, errors.New("curl 命令中的 URL 无效")
	}
	host := strings.ToLower(u.Hostname())
	secure := strings.EqualFold(u.Scheme, "https")
	keys := make([]string, 0, len(req.Cookies))
	for k := range req.Cookies {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := j.Set(StoredCookie{
			Name:   k,
			Value:  req.Cookies[k],
			Domain: host,
			Path:   "/",
			Secure: secure,
		}); err != nil {
			return req.URL, 0, err
		}
	}
	return req.URL, len(keys), nil
}

// Delete removes cookies matching any non-empty filter (domain, name, path).
func (j *CookieJar) Delete(domain, name, path string) (int, error) {
	if j == nil {
		return 0, nil
	}
	domNorm := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(domain), "."))
	j.mu.Lock()
	defer j.mu.Unlock()

	kept := make([]StoredCookie, 0, len(j.cookies))
	removed := 0
	for _, c := range j.cookies {
		match := true
		if domNorm != "" {
			cd := strings.ToLower(strings.TrimPrefix(c.Domain, "."))
			if !strings.EqualFold(c.Domain, domain) && cd != domNorm {
				match = false
			}
		}
		if name != "" && c.Name != name {
			match = false
		}
		if path != "" && c.Path != path {
			match = false
		}
		if match {
			removed++
		} else {
			kept = append(kept, c)
		}
	}
	if removed > 0 {
		j.cookies = kept
		if err := j.saveLocked(); err != nil {
			return removed, err
		}
	}
	return removed, nil
}

// DeleteForHost removes cookies matching host (both host-only and matching
// domain cookies) and optional name.
func (j *CookieJar) DeleteForHost(host, name string) (int, error) {
	if j == nil {
		return 0, nil
	}
	host = strings.ToLower(strings.TrimSpace(host))
	j.mu.Lock()
	defer j.mu.Unlock()
	kept := make([]StoredCookie, 0, len(j.cookies))
	removed := 0
	for _, c := range j.cookies {
		if cookieMatchesHost(c.Domain, host) && (name == "" || c.Name == name) {
			removed++
		} else {
			kept = append(kept, c)
		}
	}
	if removed > 0 {
		j.cookies = kept
		if err := j.saveLocked(); err != nil {
			return removed, err
		}
	}
	return removed, nil
}

// Clear deletes all cookies when domain is empty, or all cookies for domain.
func (j *CookieJar) Clear(domain string) error {
	if j == nil {
		return nil
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if strings.TrimSpace(domain) == "" {
		j.cookies = nil
		return j.saveLocked()
	}
	domNorm := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(domain), "."))
	kept := make([]StoredCookie, 0, len(j.cookies))
	for _, c := range j.cookies {
		cd := strings.ToLower(strings.TrimPrefix(c.Domain, "."))
		if !strings.EqualFold(c.Domain, domain) && cd != domNorm {
			kept = append(kept, c)
		}
	}
	j.cookies = kept
	return j.saveLocked()
}

// CloneInMemory copies all non-expired cookies into a fresh in-memory jar.
func (j *CookieJar) CloneInMemory() *CookieJar {
	clone := &CookieJar{now: time.Now}
	if j == nil {
		return clone
	}
	clone.cookies = j.List()
	return clone
}

// MaskValue redacts a sensitive cookie or token string for display/MCP output.
func MaskValue(v string) string {
	if v == "" {
		return ""
	}
	n := len(v)
	if n <= 6 {
		return "***"
	}
	return fmt.Sprintf("%s…%s (%d chars)", v[:2], v[n-2:], n)
}

func (j *CookieJar) upsertLocked(sc StoredCookie) {
	for i := range j.cookies {
		c := j.cookies[i]
		if strings.EqualFold(c.Domain, sc.Domain) && c.Path == sc.Path && c.Name == sc.Name {
			j.cookies[i] = sc
			return
		}
	}
	j.cookies = append(j.cookies, sc)
}

func (j *CookieJar) removeLocked(domain, name, path string) {
	kept := j.cookies[:0]
	for _, c := range j.cookies {
		if strings.EqualFold(c.Domain, domain) && c.Path == path && c.Name == name {
			continue
		}
		kept = append(kept, c)
	}
	j.cookies = kept
}

func (j *CookieJar) saveLocked() error {
	if j.path == "" {
		return nil
	}
	b, err := json.MarshalIndent(cookieFile{Cookies: j.cookies}, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(j.path, append(b, '\n'), 0o600)
}

func pruneExpiredCookies(list []StoredCookie, now time.Time) []StoredCookie {
	if len(list) == 0 {
		return nil
	}
	out := make([]StoredCookie, 0, len(list))
	for _, c := range list {
		if !c.Expires.IsZero() && !c.Expires.After(now) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func cookieMatchesHost(cookieDomain, host string) bool {
	cd := strings.ToLower(cookieDomain)
	if strings.HasPrefix(cd, ".") {
		base := cd[1:]
		return host == base || strings.HasSuffix(host, cd)
	}
	return host == cd
}

func cookieMatchesPath(cookiePath, reqPath string) bool {
	if cookiePath == "" || cookiePath == "/" || reqPath == cookiePath {
		return true
	}
	if strings.HasPrefix(reqPath, cookiePath) {
		if strings.HasSuffix(cookiePath, "/") || reqPath[len(cookiePath)] == '/' {
			return true
		}
	}
	return false
}

func defaultCookiePath(rawPath string) string {
	if rawPath == "" || !strings.HasPrefix(rawPath, "/") {
		return "/"
	}
	i := strings.LastIndexByte(rawPath, '/')
	if i <= 0 {
		return "/"
	}
	return rawPath[:i]
}

func sameSiteString(s http.SameSite) string {
	switch s {
	case http.SameSiteLaxMode:
		return "Lax"
	case http.SameSiteStrictMode:
		return "Strict"
	case http.SameSiteNoneMode:
		return "None"
	default:
		return ""
	}
}

func parseCookiePairs(raw string) [][2]string {
	var out [][2]string
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, val, ok := strings.Cut(part, "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			continue
		}
		out = append(out, [2]string{name, strings.TrimSpace(val)})
	}
	return out
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(tmpName)
		return errors.Join(werr, cerr)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
