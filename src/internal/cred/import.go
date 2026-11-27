package cred

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	ImportAuto     = "auto"
	ImportCookie   = "cookie"
	ImportNetscape = "netscape"
	ImportCurl     = "curl"
	ImportBearer   = "bearer"
	ImportToken    = "token"
	ImportBasic    = "basic"
	ImportHeader   = "header"
)

// ParseManualImport parses user-provided credentials in any supported format:
// auto, cookie, netscape (cookies.txt), curl, bearer, token, basic, or header.
// Cookie results are strictly filtered to spec.Origin (and spec.AllowedDomains).
func ParseManualImport(spec Spec, format, input, username, password, headerName, headerPrefix string) (SecretPayload, error) {
	spec = spec.Normalized()
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = ImportAuto
	}
	now := time.Now().UnixMilli()
	trimmed := strings.TrimSpace(input)

	if format == ImportAuto {
		switch {
		case spec.Type == TypeBasic && (username != "" || password != ""):
			format = ImportBasic
		case isCurlInput(trimmed):
			format = ImportCurl
		case looksLikeNetscape(trimmed):
			format = ImportNetscape
		case spec.Type == TypeBearer:
			format = ImportBearer
		case spec.Type == TypeToken:
			format = ImportToken
		case spec.Type == TypeHeader:
			format = ImportHeader
		default:
			format = ImportCookie
		}
	}

	switch format {
	case ImportBasic:
		user := strings.TrimSpace(username)
		pass := password
		if user == "" && trimmed != "" {
			if u, p, ok := strings.Cut(trimmed, ":"); ok {
				user, pass = strings.TrimSpace(u), p
			}
		}
		if user == "" {
			return SecretPayload{}, errors.New("cred: basic auth requires username")
		}
		return SecretPayload{
			Type:      TypeBasic,
			Username:  user,
			Password:  pass,
			Provider:  ProviderManual,
			UpdatedAt: now,
		}, nil

	case ImportBearer:
		tok := stripBearerPrefix(trimmed)
		if tok == "" {
			return SecretPayload{}, errors.New("cred: bearer token cannot be empty")
		}
		return SecretPayload{
			Type:         TypeBearer,
			Token:        tok,
			HeaderName:   "Authorization",
			HeaderPrefix: "Bearer ",
			Provider:     ProviderManual,
			UpdatedAt:    now,
		}, nil

	case ImportToken, ImportHeader:
		tok := trimmed
		hName := strings.TrimSpace(headerName)
		hPref := headerPrefix
		if hName == "" {
			hName = spec.HeaderName
		}
		if hPref == "" {
			hPref = spec.HeaderPrefix
		}
		if k, v, ok := strings.Cut(tok, ":"); ok && !strings.ContainsAny(k, " \t\n\r") {
			if hName == "" {
				hName = strings.TrimSpace(k)
			}
			tok = strings.TrimSpace(v)
		}
		if tok == "" {
			return SecretPayload{}, errors.New("cred: token/header value cannot be empty")
		}
		if hName == "" {
			hName = "Authorization"
		}
		typ := TypeToken
		if format == ImportHeader || spec.Type == TypeHeader {
			typ = TypeHeader
		}
		return SecretPayload{
			Type:         typ,
			Token:        tok,
			HeaderName:   hName,
			HeaderPrefix: hPref,
			Provider:     ProviderManual,
			UpdatedAt:    now,
		}, nil

	case ImportNetscape:
		parsed, err := ParseNetscapeCookies(trimmed)
		if err != nil {
			return SecretPayload{}, err
		}
		filtered := spec.FilterAuthorizedCookies(parsed)
		if len(filtered) == 0 {
			return SecretPayload{}, fmt.Errorf("cred: no cookies in Netscape file match authorized origin %s", spec.Origin)
		}
		return SecretPayload{
			Type:      TypeCookie,
			Cookies:   filtered,
			Provider:  ProviderManual,
			UpdatedAt: now,
		}, nil

	case ImportCurl:
		p, err := ParseCurlAuth(trimmed, spec)
		if err != nil {
			return SecretPayload{}, err
		}
		p.Provider = ProviderManual
		p.UpdatedAt = now
		return p, nil

	case ImportCookie:
		cookies, err := ParseCookieHeader(trimmed, spec.Origin)
		if err != nil {
			return SecretPayload{}, err
		}
		filtered := spec.FilterAuthorizedCookies(cookies)
		if len(filtered) == 0 {
			return SecretPayload{}, fmt.Errorf("cred: no valid cookies for authorized origin %s", spec.Origin)
		}
		return SecretPayload{
			Type:      TypeCookie,
			Cookies:   filtered,
			Provider:  ProviderManual,
			UpdatedAt: now,
		}, nil

	default:
		return SecretPayload{}, fmt.Errorf("cred: unsupported import format %q", format)
	}
}

func isCurlInput(s string) bool {
	if s == "" {
		return false
	}
	first := s
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		first = s[:i]
	}
	word, _, _ := strings.Cut(strings.TrimSpace(first), " ")
	return strings.EqualFold(word, "curl")
}

func looksLikeNetscape(s string) bool {
	if strings.Contains(s, "# Netscape HTTP Cookie File") || strings.Contains(s, "# HTTP Cookie File") {
		return true
	}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.TrimPrefix(line, "#HttpOnly_")
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Count(line, "\t") >= 6 {
			return true
		}
	}
	return false
}

func stripBearerPrefix(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 7 && strings.EqualFold(s[:7], "Bearer ") {
		return strings.TrimSpace(s[7:])
	}
	return s
}

// ParseNetscapeCookies parses standard 7-column Netscape cookies.txt text,
// including "#HttpOnly_" lines.
func ParseNetscapeCookies(text string) ([]StoredCookie, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var out []StoredCookie
	for _, rawLine := range strings.Split(text, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		httpOnly := false
		if strings.HasPrefix(line, "#HttpOnly_") {
			httpOnly = true
			line = strings.TrimPrefix(line, "#HttpOnly_")
		} else if strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 7 {
			// Fall back to whitespace splitting when tabs were converted by copy-paste.
			cols = strings.Fields(line)
		}
		if len(cols) < 7 {
			continue
		}
		dom := strings.ToLower(strings.TrimSpace(cols[0]))
		path := strings.TrimSpace(cols[2])
		if path == "" || !strings.HasPrefix(path, "/") {
			path = "/"
		}
		secure := strings.EqualFold(strings.TrimSpace(cols[3]), "TRUE")
		exp, _ := strconv.ParseInt(strings.TrimSpace(cols[4]), 10, 64)
		name := strings.TrimSpace(cols[5])
		val := strings.TrimSpace(strings.Join(cols[6:], "\t"))
		if name == "" || dom == "" {
			continue
		}
		out = append(out, StoredCookie{
			Name:     name,
			Value:    val,
			Domain:   dom,
			Path:     path,
			Expires:  exp,
			Secure:   secure,
			HTTPOnly: httpOnly,
		})
	}
	if len(out) == 0 {
		return nil, errors.New("cred: no valid entries found in Netscape cookies.txt")
	}
	return out, nil
}

// ParseCookieHeader parses a Cookie header string ("Cookie: a=1; b=2" or "a=1; b=2")
// bound to the host of origin.
func ParseCookieHeader(text, origin string) ([]StoredCookie, error) {
	s := strings.TrimSpace(text)
	if len(s) >= 7 && strings.EqualFold(s[:7], "cookie:") {
		s = strings.TrimSpace(s[7:])
	}
	ou, err := url.Parse(strings.TrimSpace(origin))
	if err != nil || ou.Hostname() == "" {
		return nil, fmt.Errorf("cred: invalid origin %q for cookie import", origin)
	}
	host := strings.ToLower(ou.Hostname())
	secure := strings.EqualFold(ou.Scheme, "https")

	var out []StoredCookie
	for _, part := range strings.Split(s, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, val, ok := strings.Cut(part, "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			continue
		}
		out = append(out, StoredCookie{
			Name:   name,
			Value:  strings.TrimSpace(val),
			Domain: host,
			Path:   "/",
			Secure: secure,
		})
	}
	if len(out) == 0 {
		return nil, errors.New("cred: no name=value cookies found")
	}
	return out, nil
}

// ParseCurlAuth extracts Cookie, Authorization, Basic (-u), or configured custom
// headers from a copied curl command, scoped to spec.Origin.
func ParseCurlAuth(cmd string, spec Spec) (SecretPayload, error) {
	toks, err := splitShellTokens(cmd)
	if err != nil {
		return SecretPayload{}, err
	}
	if len(toks) > 0 && strings.EqualFold(toks[0], "curl") {
		toks = toks[1:]
	}
	if len(toks) == 0 {
		return SecretPayload{}, errors.New("cred: empty curl command")
	}

	headers := map[string]string{}
	var rawCookies []string
	var basicUser, basicPass string

	for i := 0; i < len(toks); i++ {
		t := toks[i]
		nextVal := func() (string, error) {
			if i+1 >= len(toks) {
				return "", fmt.Errorf("cred: missing argument after %s", t)
			}
			i++
			return toks[i], nil
		}
		switch {
		case t == "-H" || t == "--header":
			v, err := nextVal()
			if err != nil {
				return SecretPayload{}, err
			}
			k, val, ok := strings.Cut(v, ":")
			if ok {
				headers[strings.TrimSpace(k)] = strings.TrimSpace(val)
			}
		case strings.HasPrefix(t, "-H") && len(t) > 2:
			k, val, ok := strings.Cut(t[2:], ":")
			if ok {
				headers[strings.TrimSpace(k)] = strings.TrimSpace(val)
			}
		case t == "-b" || t == "--cookie":
			v, err := nextVal()
			if err != nil {
				return SecretPayload{}, err
			}
			if strings.Contains(v, "=") {
				rawCookies = append(rawCookies, v)
			}
		case strings.HasPrefix(t, "-b") && len(t) > 2:
			if strings.Contains(t[2:], "=") {
				rawCookies = append(rawCookies, t[2:])
			}
		case t == "-u" || t == "--user":
			v, err := nextVal()
			if err != nil {
				return SecretPayload{}, err
			}
			basicUser, basicPass, _ = strings.Cut(v, ":")
		case t == "-X" || t == "--request" || t == "-d" || t == "--data" || t == "--data-raw" ||
			t == "--data-binary" || t == "--data-ascii" || t == "--data-urlencode" ||
			t == "-A" || t == "--user-agent" || t == "-e" || t == "--referer" || t == "--url":
			if _, err := nextVal(); err != nil {
				return SecretPayload{}, err
			}
		}
	}

	for k, v := range headers {
		if strings.EqualFold(k, "Cookie") && strings.TrimSpace(v) != "" {
			rawCookies = append(rawCookies, v)
		}
	}

	// Priority: match spec.Type if clear, otherwise extract cookies -> Authorization -> custom header.
	if (spec.Type == TypeCookie || spec.Type == "") && len(rawCookies) > 0 {
		cookies, err := ParseCookieHeader(strings.Join(rawCookies, "; "), spec.Origin)
		if err != nil {
			return SecretPayload{}, err
		}
		filtered := spec.FilterAuthorizedCookies(cookies)
		if len(filtered) == 0 {
			return SecretPayload{}, fmt.Errorf("cred: curl cookies do not match authorized origin %s", spec.Origin)
		}
		return SecretPayload{Type: TypeCookie, Cookies: filtered}, nil
	}

	for k, v := range headers {
		if strings.EqualFold(k, "Authorization") && v != "" {
			switch {
			case len(v) >= 7 && strings.EqualFold(v[:7], "Bearer "):
				return SecretPayload{
					Type:         TypeBearer,
					Token:        strings.TrimSpace(v[7:]),
					HeaderName:   "Authorization",
					HeaderPrefix: "Bearer ",
				}, nil
			case len(v) >= 6 && strings.EqualFold(v[:6], "Basic "):
				raw, decErr := base64.StdEncoding.DecodeString(strings.TrimSpace(v[6:]))
				if decErr == nil {
					u, p, ok := strings.Cut(string(raw), ":")
					if ok {
						return SecretPayload{Type: TypeBasic, Username: u, Password: p}, nil
					}
				}
			default:
				return SecretPayload{
					Type:       TypeToken,
					Token:      v,
					HeaderName: "Authorization",
				}, nil
			}
		}
	}

	if basicUser != "" {
		return SecretPayload{Type: TypeBasic, Username: basicUser, Password: basicPass}, nil
	}

	if len(rawCookies) > 0 {
		cookies, err := ParseCookieHeader(strings.Join(rawCookies, "; "), spec.Origin)
		if err != nil {
			return SecretPayload{}, err
		}
		filtered := spec.FilterAuthorizedCookies(cookies)
		if len(filtered) > 0 {
			return SecretPayload{Type: TypeCookie, Cookies: filtered}, nil
		}
	}

	if spec.HeaderName != "" {
		for k, v := range headers {
			if strings.EqualFold(k, spec.HeaderName) && strings.TrimSpace(v) != "" {
				return SecretPayload{
					Type:         TypeHeader,
					Token:        strings.TrimSpace(v),
					HeaderName:   k,
					HeaderPrefix: spec.HeaderPrefix,
				}, nil
			}
		}
	}

	return SecretPayload{}, errors.New("cred: no Cookie, Authorization, or custom auth header found in curl command")
}

func splitShellTokens(s string) ([]string, error) {
	var (
		out   []string
		cur   strings.Builder
		inTok bool
	)
	rs := []rune(s)
	flush := func() {
		if inTok {
			out = append(out, cur.String())
			cur.Reset()
			inTok = false
		}
	}
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case r == '\\' && i+1 < len(rs) && rs[i+1] == '\n':
			i++
		case r == '\\' && i+2 < len(rs) && rs[i+1] == '\r' && rs[i+2] == '\n':
			i += 2
		case r == '\\' && i+1 < len(rs):
			cur.WriteRune(rs[i+1])
			inTok = true
			i++
		case r == '\'' || r == '"' || (r == '$' && i+1 < len(rs) && rs[i+1] == '\''):
			ansiC := false
			if r == '$' {
				ansiC = true
				i++
			}
			quote := rs[i]
			j := i + 1
			for j < len(rs) && rs[j] != quote {
				if (quote == '"' || ansiC) && rs[j] == '\\' && j+1 < len(rs) {
					j++
				}
				cur.WriteRune(rs[j])
				j++
			}
			if j >= len(rs) {
				return nil, errors.New("cred: unclosed quote in curl command")
			}
			inTok = true
			i = j
		case unicode.IsSpace(r):
			flush()
		default:
			cur.WriteRune(r)
			inTok = true
		}
	}
	flush()
	return out, nil
}
