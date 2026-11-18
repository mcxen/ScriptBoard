package feed

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// ParseCurl reads a curl command, as copied from a browser or the docs, into a
// Request. It understands the options that describe a request: the method
// (-X), headers (-H), cookies (-b, or a Cookie header), data (-d and its
// variants), basic auth (-u), user agent (-A) and referer (-e). Options that
// only change how the transfer is shown or retried are ignored. Anything that
// would read a local file, or change the request in a way this package cannot
// reproduce, is refused rather than guessed.
func ParseCurl(cmd string) (Request, error) {
	toks, err := splitShell(cmd)
	if err != nil {
		return Request{}, err
	}
	if len(toks) > 0 && toks[0] == "curl" {
		toks = toks[1:]
	}
	if len(toks) == 0 {
		return Request{}, errors.New("没有 curl 命令")
	}

	var (
		method  string
		url     string
		data    []string
		headers = map[string]string{}
		cookies = map[string]string{}
	)
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		value := func() (string, error) {
			if i+1 >= len(toks) {
				return "", fmt.Errorf("参数 %s 后面缺少内容", t)
			}
			i++
			return toks[i], nil
		}
		var v string
		var err error
		switch {
		case t == "-X" || t == "--request":
			if v, err = value(); err != nil {
				return Request{}, err
			}
			method = v
		case strings.HasPrefix(t, "-X") && len(t) > 2:
			method = t[2:]
		case t == "-H" || t == "--header":
			if v, err = value(); err != nil {
				return Request{}, err
			}
			name, val, ok := strings.Cut(v, ":")
			if !ok {
				return Request{}, fmt.Errorf("请求头格式不对：%s", v)
			}
			headers[strings.TrimSpace(name)] = strings.TrimSpace(val)
		case t == "-b" || t == "--cookie":
			if v, err = value(); err != nil {
				return Request{}, err
			}
			if !strings.Contains(v, "=") {
				// curl would read this as a file of cookies.
				return Request{}, errors.New("不支持从文件读取 cookie，请把 cookie 直接写在命令里，例如 -b 'name=value'")
			}
			addCookies(cookies, v)
		case t == "-d" || t == "--data" || t == "--data-raw" || t == "--data-binary" ||
			t == "--data-ascii" || t == "--data-urlencode":
			if v, err = value(); err != nil {
				return Request{}, err
			}
			if strings.HasPrefix(v, "@") {
				return Request{}, errors.New("不支持从文件读取请求体，请把内容直接写在命令里")
			}
			data = append(data, v)
		case t == "-u" || t == "--user":
			if v, err = value(); err != nil {
				return Request{}, err
			}
			headers["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(v))
		case t == "-A" || t == "--user-agent":
			if v, err = value(); err != nil {
				return Request{}, err
			}
			headers["User-Agent"] = v
		case t == "-e" || t == "--referer":
			if v, err = value(); err != nil {
				return Request{}, err
			}
			headers["Referer"] = v
		case t == "--url":
			if v, err = value(); err != nil {
				return Request{}, err
			}
			url = v
		case t == "--compressed" || t == "-s" || t == "--silent" || t == "-S" || t == "--show-error" ||
			t == "-k" || t == "--insecure" || t == "-L" || t == "--location" || t == "-i" || t == "-v" ||
			t == "--verbose" || t == "--http1.1" || t == "--http2":
			// Ignored: they do not change what is asked for.
		case strings.HasPrefix(t, "-") && onlyIgnoredFlags(t):
			// Combined short flags such as -sS.
		case strings.HasPrefix(t, "-"):
			return Request{}, fmt.Errorf("不认识的参数 %s，这个 curl 命令暂时不能用", t)
		default:
			if url != "" {
				return Request{}, fmt.Errorf("多余的内容：%s", t)
			}
			url = t
		}
	}

	if cookie, ok := headers["Cookie"]; ok {
		addCookies(cookies, cookie)
		delete(headers, "Cookie")
	}
	for k := range headers {
		if strings.EqualFold(k, "Cookie") {
			addCookies(cookies, headers[k])
			delete(headers, k)
		}
	}
	r := Request{URL: url, Headers: headers, Cookies: cookies}
	if len(data) > 0 {
		r.Body = strings.Join(data, "&")
		if method == "" {
			method = "POST"
		}
	}
	if method == "" {
		method = "GET"
	}
	r.Method = strings.ToUpper(method)
	if len(r.Headers) == 0 {
		r.Headers = nil
	}
	if len(r.Cookies) == 0 {
		r.Cookies = nil
	}
	if url == "" {
		return Request{}, errors.New("命令里没有地址")
	}
	return r, nil
}

// addCookies reads "a=1; b=2" into cookies.
func addCookies(cookies map[string]string, s string) {
	for _, part := range strings.Split(s, ";") {
		name, val, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok && strings.TrimSpace(name) != "" {
			cookies[strings.TrimSpace(name)] = strings.TrimSpace(val)
		}
	}
}

// onlyIgnoredFlags reports whether a combined short flag such as -sSL is made
// of flags that are safe to ignore.
func onlyIgnoredFlags(t string) bool {
	if len(t) < 2 || t[0] != '-' || t[1] == '-' {
		return false
	}
	for _, r := range t[1:] {
		if !strings.ContainsRune("sSkLiv", r) {
			return false
		}
	}
	return true
}

// splitShell splits a command the way a POSIX shell would for the cases curl
// commands use: single quotes, double quotes, backslash escapes and line
// continuations.
func splitShell(s string) ([]string, error) {
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
			i++ // continuation
		case r == '\\' && i+2 < len(rs) && rs[i+1] == '\r' && rs[i+2] == '\n':
			i += 2 // continuation with CRLF
		case r == '\\' && i+1 < len(rs):
			cur.WriteRune(rs[i+1])
			inTok = true
			i++
		case r == '\'':
			j := i + 1
			for j < len(rs) && rs[j] != '\'' {
				cur.WriteRune(rs[j])
				j++
			}
			if j >= len(rs) {
				return nil, errors.New("单引号没有配对")
			}
			inTok = true
			i = j
		case r == '"':
			j := i + 1
			for j < len(rs) && rs[j] != '"' {
				if rs[j] == '\\' && j+1 < len(rs) && strings.ContainsRune(`"\$`+"`", rs[j+1]) {
					j++
				}
				cur.WriteRune(rs[j])
				j++
			}
			if j >= len(rs) {
				return nil, errors.New("双引号没有配对")
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

// ParseRequest reads what a user types for a request: a curl command, or an
// address in any protocol. For an address, the first line is the address and
// the lines after it are the body that is sent (a tcp command, a POST body).
//
//	curl 'https://…' -b 'session=…'
//	https://api.example.com/price
//	tcp://10.0.0.5:9000        (then, on the next lines, what to send; a newline is added)
//	file:///var/log/price.txt
//	exec:cat /proc/loadavg
func ParseRequest(text string) (Request, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return Request{}, errors.New("没有请求")
	}
	first := text
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		first = text[:i]
	}
	word, _, _ := strings.Cut(strings.TrimSpace(first), " ")
	if word == "curl" {
		return ParseCurl(text)
	}
	scheme := schemeOf(strings.TrimSpace(first))
	if _, ok := protocolFor(scheme); !ok {
		return Request{}, fmt.Errorf("需要 curl 命令，或以 %s 开头的地址", strings.Join(Schemes(), "://、")+"://")
	}
	r := Request{URL: strings.TrimSpace(first)}
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		r.Body = strings.TrimLeft(text[i+1:], "\n")
		switch {
		case r.Body == "":
		case scheme == "http" || scheme == "https":
			r.Method = "POST"
		case scheme == "tcp":
			r.Body += "\n" // a line protocol: the command ends with a newline
		}
	}
	if err := r.Validate(); err != nil {
		return Request{}, err
	}
	return r, nil
}
