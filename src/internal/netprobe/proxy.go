package netprobe

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type scutilProxyConfig struct {
	httpEnable  bool
	httpHost    string
	httpPort    int
	httpsEnable bool
	httpsHost   string
	httpsPort   int
	socksEnable bool
	socksHost   string
	socksPort   int
	exceptions  []string
	fetchedAt   time.Time
}

var (
	scutilMu    sync.Mutex
	scutilCache scutilProxyConfig
)

const scutilTTL = 15 * time.Second

// NewClient constructs an HTTP client tuned for concurrent keep-alive probing
// that honors environment proxy settings and macOS system proxy configuration
// (while bypassing loopback hosts so tests and local services stay direct).
func NewClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	tr := &http.Transport{
		Proxy:                 ProxyFunc,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   3 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}
}

// ProxyFunc resolves the proxy URL for req:
//  1. Loopback hosts always bypass proxies.
//  2. Standard environment proxy variables (HTTP_PROXY/HTTPS_PROXY/ALL_PROXY).
//  3. On macOS, the system scutil --proxy configuration (Clash/Surge/Mihomo).
func ProxyFunc(req *http.Request) (*url.URL, error) {
	if req == nil || req.URL == nil {
		return nil, nil
	}
	host := strings.TrimSpace(req.URL.Hostname())
	if isLoopbackHost(host) {
		return nil, nil
	}
	if u, err := http.ProxyFromEnvironment(req); err != nil || u != nil {
		return u, err
	}
	if runtime.GOOS != "darwin" {
		return nil, nil
	}
	cfg := loadScutilProxy()
	return cfg.proxyFor(req.URL)
}

func isLoopbackHost(host string) bool {
	if host == "" {
		return false
	}
	lh := strings.ToLower(host)
	if lh == "localhost" || strings.HasSuffix(lh, ".localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	return false
}

func loadScutilProxy() scutilProxyConfig {
	scutilMu.Lock()
	defer scutilMu.Unlock()
	if !scutilCache.fetchedAt.IsZero() && time.Since(scutilCache.fetchedAt) < scutilTTL {
		return scutilCache
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "scutil", "--proxy").Output()
	cfg := scutilProxyConfig{fetchedAt: time.Now()}
	if err == nil {
		cfg = parseScutilProxy(string(out))
		cfg.fetchedAt = time.Now()
	}
	scutilCache = cfg
	return cfg
}

func parseScutilProxy(raw string) scutilProxyConfig {
	var cfg scutilProxyConfig
	inExceptions := false
	for _, line := range strings.Split(raw, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		if strings.HasPrefix(trim, "ExceptionsList") {
			inExceptions = true
			continue
		}
		if inExceptions {
			if trim == "}" {
				inExceptions = false
				continue
			}
			if _, val, ok := strings.Cut(trim, ":"); ok {
				v := strings.TrimSpace(val)
				if v != "" {
					cfg.exceptions = append(cfg.exceptions, v)
				}
			}
			continue
		}
		key, val, ok := strings.Cut(trim, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch key {
		case "HTTPEnable":
			cfg.httpEnable = val == "1"
		case "HTTPProxy":
			cfg.httpHost = val
		case "HTTPPort":
			cfg.httpPort, _ = strconv.Atoi(val)
		case "HTTPSEnable":
			cfg.httpsEnable = val == "1"
		case "HTTPSProxy":
			cfg.httpsHost = val
		case "HTTPSPort":
			cfg.httpsPort, _ = strconv.Atoi(val)
		case "SOCKSEnable":
			cfg.socksEnable = val == "1"
		case "SOCKSProxy":
			cfg.socksHost = val
		case "SOCKSPort":
			cfg.socksPort, _ = strconv.Atoi(val)
		}
	}
	return cfg
}

func (c scutilProxyConfig) proxyFor(u *url.URL) (*url.URL, error) {
	if u == nil {
		return nil, nil
	}
	host := strings.TrimSpace(u.Hostname())
	for _, pat := range c.exceptions {
		if matchProxyException(pat, host) {
			return nil, nil
		}
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme == "https" && c.httpsEnable && c.httpsHost != "" && c.httpsPort > 0 {
		return &url.URL{Scheme: "http", Host: net.JoinHostPort(c.httpsHost, strconv.Itoa(c.httpsPort))}, nil
	}
	if c.httpEnable && c.httpHost != "" && c.httpPort > 0 {
		return &url.URL{Scheme: "http", Host: net.JoinHostPort(c.httpHost, strconv.Itoa(c.httpPort))}, nil
	}
	if c.socksEnable && c.socksHost != "" && c.socksPort > 0 {
		return &url.URL{Scheme: "socks5", Host: net.JoinHostPort(c.socksHost, strconv.Itoa(c.socksPort))}, nil
	}
	return nil, nil
}

func matchProxyException(pattern, host string) bool {
	p := strings.ToLower(strings.TrimSpace(pattern))
	h := strings.ToLower(strings.TrimSpace(host))
	if p == "" || h == "" {
		return false
	}
	if _, ipNet, err := net.ParseCIDR(p); err == nil {
		if ip := net.ParseIP(h); ip != nil && ipNet.Contains(ip) {
			return true
		}
	}
	return matchWildcard(p, h)
}

func matchWildcard(pattern, s string) bool {
	if !strings.Contains(pattern, "*") {
		return pattern == s
	}
	parts := strings.Split(pattern, "*")
	if len(parts) == 0 {
		return true
	}
	if parts[0] != "" && !strings.HasPrefix(s, parts[0]) {
		return false
	}
	idx := len(parts[0])
	for i := 1; i < len(parts)-1; i++ {
		seg := parts[i]
		if seg == "" {
			continue
		}
		pos := strings.Index(s[idx:], seg)
		if pos < 0 {
			return false
		}
		idx += pos + len(seg)
	}
	last := parts[len(parts)-1]
	if last == "" {
		return true
	}
	return len(s)-idx >= len(last) && strings.HasSuffix(s, last)
}
