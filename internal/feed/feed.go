// Package feed turns HTTP endpoints into time series of numbers.
//
// A Source is three things: a Request (a curl command, with cookies when the
// endpoint needs them), an Extract rule that reads one or more numbers out of
// the response, and a poll interval. The Runner polls due sources and appends
// each reading to a Store as a Sample. Aggregate groups the samples by hour or
// by day, which is what a chart is drawn from.
//
// The host makes every request. A script only sees the aggregated numbers it
// asks for, never the cookies or the network.
//
//	Source ──Poll──▶ Sample{T, Values} ──Store──▶ Aggregate(hour|day) ──▶ script ctx.feeds
//	  │
//	  ├─ Request  {method, url, headers, cookies, body}   ← ParseCurl
//	  ├─ Extract  {kind: json|regex, path|pattern}        → Numbers(body)
//	  └─ Every    seconds (≥ 60)     Agg  last|avg|max|min per bucket
package feed

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Request is one HTTP request.
type Request struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
	Cookies map[string]string `json:"cookies,omitempty"`
	Body    string            `json:"body,omitempty"`
}

// Extract kinds.
const (
	KindJSON  = "json"
	KindRegex = "regex"
)

// Extract says where the numbers are in a response.
type Extract struct {
	Kind string `json:"kind"`
	// Path, for KindJSON, is a dotted path such as "data.price". A "#" takes
	// every item of an array, so "data.items.#.price" gives one number per item.
	Path string `json:"path,omitempty"`
	// Pattern, for KindRegex, is a regular expression. Each match contributes
	// one number: its first group, or the whole match when there is none.
	Pattern string `json:"pattern,omitempty"`
}

// Source is an endpoint that is polled for numbers.
type Source struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Request Request  `json:"request"`
	Extract Extract  `json:"extract"`
	Names   []string `json:"names,omitempty"` // one name per number; default 值1, 值2, …
	Every   int      `json:"every"`           // seconds between polls, at least 60
	Agg     string   `json:"agg,omitempty"`   // how a bucket is summarised: last | avg | max | min

	// Fallbacks are tried in order when Request fails, or when Extract finds
	// no number in its response. They share the Extract rule.
	Fallbacks []Request `json:"fallbacks,omitempty"`
	// Timeout is the longest one try may take, in seconds; 0 is 10.
	Timeout int `json:"timeout,omitempty"`
	// Retries is the number of tries of one request after the first; 0 is
	// the default (2), and -1 is none.
	Retries int `json:"retries,omitempty"`
}

// Requests lists the main request and then the fallbacks.
func (s Source) Requests() []Request { return append([]Request{s.Request}, s.Fallbacks...) }

// Policy is how hard each request of the source is tried.
func (s Source) Policy() Policy {
	p := DefaultPolicy
	if s.Timeout > 0 {
		p.Timeout = time.Duration(s.Timeout) * time.Second
	}
	switch {
	case s.Retries < 0:
		p.Attempts = 1
	case s.Retries > 0:
		p.Attempts = min(s.Retries, 6) + 1
	}
	return p
}

// Sample is one reading: when it was taken and the numbers it held.
type Sample struct {
	T      int64     `json:"t"` // Unix milliseconds
	Values []float64 `json:"v"`
}

// MinEvery is the shortest poll interval, in seconds.
const MinEvery = 60

var (
	idPattern     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,39}$`)
	validMethods  = map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}
	validAggs     = map[string]bool{"": true, "last": true, "avg": true, "max": true, "min": true}
	validIDErrMsg = "数据源 id 只能用小写字母、数字和连字符，2 到 40 个字符"
)

// ValidID reports whether id can name a source (and a file in the store).
func ValidID(id string) bool { return idPattern.MatchString(id) }

// Validate checks a request by the protocol of its address (http, https,
// file, tcp, exec, or one added with Register).
func (r Request) Validate() error {
	scheme := schemeOf(r.URL)
	p, ok := protocolFor(scheme)
	if !ok {
		return fmt.Errorf("不支持的协议 %q，可用：%s", scheme, strings.Join(Schemes(), "、"))
	}
	return p.Validate(r)
}

func (r Request) method() string {
	if r.Method == "" {
		return "GET"
	}
	return strings.ToUpper(r.Method)
}

// Validate checks the extract rule.
func (e Extract) Validate() error {
	switch e.Kind {
	case KindJSON:
		if strings.TrimSpace(e.Path) == "" {
			return errors.New("JSON 路径不能为空，例如 data.price")
		}
	case KindRegex:
		if _, err := regexp.Compile(e.Pattern); err != nil {
			return fmt.Errorf("正则表达式不对：%w", err)
		}
	default:
		return errors.New("取数方式只能是 json 或 regex")
	}
	return nil
}

// Validate checks a source before it is stored or polled.
func (s Source) Validate() error {
	if !ValidID(s.ID) {
		return errors.New(validIDErrMsg)
	}
	if strings.TrimSpace(s.Name) == "" {
		return errors.New("数据源需要一个名称")
	}
	for i, r := range s.Requests() {
		if err := r.Validate(); err != nil {
			if i > 0 {
				return fmt.Errorf("备用请求 %d：%w", i, err)
			}
			return err
		}
	}
	if err := s.Extract.Validate(); err != nil {
		return err
	}
	if s.Timeout < 0 || s.Timeout > 120 {
		return errors.New("超时只能在 1 到 120 秒之间")
	}
	if s.Every < MinEvery {
		return errors.New("轮询间隔至少 1 分钟")
	}
	if !validAggs[s.Agg] {
		return errors.New("汇总方式只能是 last、avg、max 或 min")
	}
	return nil
}

// Interval is the poll interval in seconds, as the runner uses it.
func (s Source) Interval() int64 { return int64(max(s.Every, MinEvery)) }

// NumberName names the i-th number of a source.
func (s Source) NumberName(i int) string {
	if i < len(s.Names) && strings.TrimSpace(s.Names[i]) != "" {
		return strings.TrimSpace(s.Names[i])
	}
	return fmt.Sprintf("值%d", i+1)
}

// HTTPRequest builds the request to send, with its cookies in the Cookie
// header.
func (r Request) HTTPRequest(ctx context.Context) (*http.Request, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, r.method(), r.URL, strings.NewReader(r.Body))
	if err != nil {
		return nil, err
	}
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}
	if len(r.Cookies) > 0 {
		req.Header.Set("Cookie", cookieHeader(r.Cookies))
	}
	return req, nil
}

func cookieHeader(c map[string]string) string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+c[k])
	}
	return strings.Join(parts, "; ")
}

// Fetcher sends a request and returns the body of a successful response.
type Fetcher interface {
	Fetch(ctx context.Context, r Request) ([]byte, error)
}

// HTTPFetcher is the Fetcher that uses the network.
type HTTPFetcher struct {
	Client *http.Client
}

// maxBody limits how much of a response is read.
const maxBody = 1 << 20

// Fetch sends r. Anything but a 2xx status is an error.
func (f HTTPFetcher) Fetch(ctx context.Context, r Request) ([]byte, error) {
	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := r.HTTPRequest(ctx)
	if err != nil {
		return nil, permanent(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败：%w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		return nil, HTTPError{Status: resp.StatusCode}
	}
	return body, nil
}

// BasicAuth is the Authorization header value for user:password.
func BasicAuth(userPass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(userPass))
}
