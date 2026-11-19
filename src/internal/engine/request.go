package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// A source is where a response comes from. In meta() it is either the address
// as a string, or an object for a request of its own:
//
//	sources: {
//	  list: "https://example.com/api/items",                     // GET
//	  pay:  { url: "https://example.com/api/query", method: "POST",
//	          headers: { Cookie: "session=…" }, body: "{\"day\":\"{start}\"}" },
//	  page: { url: "https://example.com/api/list",
//	          paging: { param: "page", start: 1, pages: 5, items: "data.list" } },
//	}
//
// {start}, {end} and the variables fill the address, the query, the headers
// and the body alike.
type Source struct {
	URL     string            `json:"url"`
	Method  string            `json:"method,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Query   map[string]string `json:"query,omitempty"`
	Body    string            `json:"body,omitempty"`
	Paging  *Paging           `json:"paging,omitempty"`
}

// Paging reads a list that runs over several pages: one request a page, with
// the page number in the query parameter Param, from Start in steps of Step, at
// most Pages pages. Items is the path of the list inside each response ("data.list");
// empty means the response is the list itself. The reading stops at the first
// page that has no items, and the lists of the pages are joined into one.
type Paging struct {
	Param string `json:"param"`
	Start int    `json:"start,omitempty"`
	Step  int    `json:"step,omitempty"`
	Pages int    `json:"pages,omitempty"`
	Items string `json:"items,omitempty"`
}

// maxPages bounds how many pages one source reads.
const maxPages = 50

// UnmarshalJSON takes a source as its address, or as an object.
func (s *Source) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		return json.Unmarshal(b, &s.URL)
	}
	type plain Source
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	*s = Source(p)
	return nil
}

// Request is a source made ready to send: its variables and dates filled in.
type Request struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    string
	Paging  *Paging
}

// Request makes the request of the source at now, with the variables vals.
func (s Source) Request(now time.Time, vals map[string]string) Request {
	r := Request{
		Method:  strings.ToUpper(s.Method),
		URL:     ExpandVars(s.URL, now, vals),
		Body:    ExpandVars(s.Body, now, vals),
		Headers: map[string]string{},
		Paging:  s.Paging,
	}
	if r.Method == "" {
		r.Method = http.MethodGet
	}
	for k, v := range s.Headers {
		r.Headers[k] = ExpandVars(v, now, vals)
	}
	if len(s.Query) > 0 {
		keys := make([]string, 0, len(s.Query))
		for k := range s.Query {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		q := url.Values{}
		for _, k := range keys {
			q.Set(k, ExpandVars(s.Query[k], now, vals))
		}
		sep := "?"
		if strings.Contains(r.URL, "?") {
			sep = "&"
		}
		r.URL += sep + q.Encode()
	}
	return r
}

// CacheKey names the response of the request in the cache. A plain GET is
// its address, as it always was; any other request has its method, body and
// paging in its key, so that two requests to one address are two responses.
func (r Request) CacheKey() string {
	if r.Method == http.MethodGet && r.Body == "" && r.Paging == nil {
		return r.URL
	}
	key := r.Method + " " + r.URL + "\n" + r.Body
	if p := r.Paging; p != nil {
		key += fmt.Sprintf("\n#paged %s %d %d %d %s", p.Param, p.Start, p.Step, p.Pages, p.Items)
	}
	return key
}

// send makes one request and returns the response body.
func send(ctx context.Context, client *http.Client, r Request) ([]byte, error) {
	if client == nil {
		client = http.DefaultClient
	}
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	var body io.Reader
	if r.Body != "" {
		body = strings.NewReader(r.Body)
	}
	method := r.Method
	if method == "" {
		method = http.MethodGet
	}
	req, err := http.NewRequestWithContext(ctx, method, r.URL, body)
	if err != nil {
		return nil, err
	}
	for k, v := range r.Headers {
		req.Header.Set(k, v)
	}
	if r.Body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.New("HTTP " + resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxFetchBytes))
}

// fetchPaged reads the pages of a list and joins them. A page that fails after
// the first keeps what the earlier pages gave.
func fetchPaged(ctx context.Context, client *http.Client, r Request) ([]byte, error) {
	p := r.Paging
	start, step, pages := p.Start, p.Step, p.Pages
	if start == 0 {
		start = 1
	}
	if step == 0 {
		step = 1
	}
	if pages <= 0 {
		pages = 1
	}
	pages = min(pages, maxPages)
	var items []json.RawMessage
	for i := 0; i < pages; i++ {
		page := start + i*step
		pr := r
		pr.Paging = nil
		pr.URL = withQuery(r.URL, p.Param, strconv.Itoa(page))
		body, err := send(ctx, client, pr)
		if err == nil && !json.Valid(body) {
			err = errors.New("response is not JSON")
		}
		if err != nil {
			if i == 0 {
				return nil, err
			}
			break
		}
		list, ok := listAt(body, p.Items)
		if !ok {
			if i == 0 {
				return nil, fmt.Errorf("no list at %q", p.Items)
			}
			break
		}
		if len(list) == 0 {
			break
		}
		items = append(items, list...)
	}
	if items == nil {
		items = []json.RawMessage{}
	}
	return json.Marshal(items)
}

// withQuery adds key=value to the query of rawURL.
func withQuery(rawURL, key, value string) string {
	sep := "?"
	if strings.Contains(rawURL, "?") {
		sep = "&"
	}
	return rawURL + sep + url.QueryEscape(key) + "=" + url.QueryEscape(value)
}

// listAt is the array at the dotted path in body, in the order of the
// document; an empty path is the body itself.
func listAt(body []byte, path string) ([]json.RawMessage, bool) {
	var v any
	if json.Unmarshal(body, &v) != nil {
		return nil, false
	}
	if path != "" {
		for _, seg := range strings.Split(path, ".") {
			m, ok := v.(map[string]any)
			if !ok {
				return nil, false
			}
			if v, ok = m[seg]; !ok {
				return nil, false
			}
		}
	}
	if _, ok := v.([]any); !ok {
		return nil, false
	}
	var raw []json.RawMessage
	if path == "" {
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, false
		}
		return raw, true
	}
	// Re-read the array from the original bytes, to keep each item as it was.
	var root map[string]json.RawMessage
	cur := body
	for _, seg := range strings.Split(path, ".") {
		if err := json.Unmarshal(cur, &root); err != nil {
			return nil, false
		}
		next, ok := root[seg]
		if !ok {
			return nil, false
		}
		cur = next
	}
	if err := json.Unmarshal(cur, &raw); err != nil {
		return nil, false
	}
	return raw, true
}
