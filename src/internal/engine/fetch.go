package engine

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	fetchTimeout  = 8 * time.Second
	maxFetchBytes = 2 << 20
)

// Expand replaces {start} and {end} in a source URL with the dates of the
// last 30 days, as yyyy-mm-dd.
func Expand(tmpl string, now time.Time) string {
	r := strings.NewReplacer(
		"{start}", now.AddDate(0, 0, -30).Format("2006-01-02"),
		"{end}", now.Format("2006-01-02"),
	)
	return r.Replace(tmpl)
}

// Fetch loads every source at the same time. A source that fails becomes
// {"error": "..."}, so the script can show its fallback instead of the whole
// run failing.
func Fetch(ctx context.Context, client *http.Client, sources map[string]string, now time.Time) map[string]json.RawMessage {
	names := make([]string, 0, len(sources))
	urls := make([]string, 0, len(sources))
	for name, tmpl := range sources {
		names = append(names, name)
		urls = append(urls, Expand(tmpl, now))
	}
	raw := make([]json.RawMessage, len(names))
	var wg sync.WaitGroup
	for i := range names {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body, err := get(ctx, client, urls[i], nil)
			switch {
			case err != nil:
				raw[i] = errorJSON(err)
			case !json.Valid(body):
				raw[i] = errorJSON(errors.New("response is not JSON"))
			default:
				raw[i] = body
			}
		}(i)
	}
	wg.Wait()
	out := make(map[string]json.RawMessage, len(names))
	for i, name := range names {
		out[name] = raw[i]
	}
	return out
}

// Offline gives every source the same error, for runs without the network.
func Offline[V any](sources map[string]V) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(sources))
	for name := range sources {
		out[name] = errorJSON(errors.New(offlineWord("")))
	}
	return out
}

func get(ctx context.Context, client *http.Client, url string, header map[string]string) ([]byte, error) {
	if client == nil {
		client = http.DefaultClient
	}
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("HTTP " + resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxFetchBytes))
}

func errorJSON(err error) json.RawMessage {
	b, _ := json.Marshal(map[string]string{"error": err.Error()})
	return b
}

// SourceCache is the cache the host keeps of sources' responses: the latest of
// each URL, and a history of them.
type SourceCache interface {
	Get(url string) (CachedBody, bool)
	Put(url string, body []byte, at time.Time)
	History(url string, limit int) []CachedBody
}

// CachedBody is a response, and when it was fetched.
type CachedBody struct {
	At   time.Time
	Body []byte
}

// SourceInfo says where a source's data came from, so that a script can show
// that it is old: ctx.sourceInfo[name].
type SourceInfo struct {
	// At is when the data was fetched, in Unix milliseconds.
	At int64 `json:"t"`
	// Cached: the data is from the cache, not from a request made now.
	Cached bool `json:"cached"`
	// Stale: the cache stood in for a request that failed or was not allowed
	// (offline mode), so the data may be old.
	Stale bool `json:"stale"`
	// Error is why the request failed or was not made.
	Error string `json:"error,omitempty"`
}

// HistoryPoint is one response a source gave, and when: ctx.history[name].
type HistoryPoint struct {
	T int64           `json:"t"`
	V json.RawMessage `json:"v"`
}

// FetchOptions say how sources are loaded.
type FetchOptions struct {
	// Cache keeps the responses; nil is no cache.
	Cache SourceCache
	// TTL: a cached response younger than this is used without a request, so
	// that opening a card again does not ask the network again. 0 always asks.
	TTL time.Duration
	// Offline: no request is made; the cache answers, or the source has an error.
	Offline bool
	// Force asks the network whatever the TTL: the user pressed refresh.
	Force bool
	// Headers are the request headers of each source, by name (a script's
	// sourceHeaders).
	Headers map[string]map[string]string
	// Lang is the language of the messages a source's error carries: "zh" or
	// "en"; anything else is English.
	Lang string
}

// offlineWord is why a source gave no request: the app is offline.
func offlineWord(lang string) string {
	if lang == "zh" {
		return "离线模式"
	}
	return "offline mode"
}

// offlineNothing is the error of a source that is offline with nothing saved.
func offlineNothing(lang string) string {
	if lang == "zh" {
		return "离线模式，没有缓存的数据"
	}
	return "offline mode: nothing cached"
}

// FetchCached loads every source at the same time, through the cache. For each:
//
//   - offline, the cached response, or an error when there is none;
//   - a cached response younger than the TTL, as it is;
//   - otherwise a request, cached when it works; when it fails, the cached
//     response if there is one, marked stale, and the error if not.
//
// A source that fails with nothing cached becomes {"error": "..."}, as in Fetch.
func FetchCached(ctx context.Context, client *http.Client, sources map[string]string, now time.Time, opt FetchOptions) (map[string]json.RawMessage, map[string]SourceInfo) {
	reqs := make(map[string]Request, len(sources))
	for name, tmpl := range sources {
		reqs[name] = Request{Method: http.MethodGet, URL: Expand(tmpl, now)}
	}
	return FetchRequests(ctx, client, reqs, now, opt)
}

// FetchRequests loads every request at the same time, through the cache, as
// FetchCached does. A request's headers from opt.Headers join its own.
func FetchRequests(ctx context.Context, client *http.Client, reqs map[string]Request, now time.Time, opt FetchOptions) (map[string]json.RawMessage, map[string]SourceInfo) {
	names := make([]string, 0, len(reqs))
	for name := range reqs {
		names = append(names, name)
	}
	raw := make([]json.RawMessage, len(names))
	info := make([]SourceInfo, len(names))
	var wg sync.WaitGroup
	for i := range names {
		r := reqs[names[i]]
		if len(opt.Headers[names[i]]) > 0 {
			merged := map[string]string{}
			for k, v := range r.Headers {
				merged[k] = v
			}
			for k, v := range opt.Headers[names[i]] {
				merged[k] = v
			}
			r.Headers = merged
		}
		wg.Add(1)
		go func(i int, r Request) {
			defer wg.Done()
			raw[i], info[i] = fetchOne(ctx, client, r, now, opt)
		}(i, r)
	}
	wg.Wait()
	bodies := make(map[string]json.RawMessage, len(names))
	infos := make(map[string]SourceInfo, len(names))
	for i, name := range names {
		bodies[name], infos[name] = raw[i], info[i]
	}
	return bodies, infos
}

func fetchOne(ctx context.Context, client *http.Client, r Request, now time.Time, opt FetchOptions) (json.RawMessage, SourceInfo) {
	key := r.CacheKey()
	var cached CachedBody
	var have bool
	if opt.Cache != nil {
		cached, have = opt.Cache.Get(key)
	}
	fromCache := func(stale bool, why error) (json.RawMessage, SourceInfo) {
		in := SourceInfo{At: cached.At.UnixMilli(), Cached: true, Stale: stale}
		if why != nil {
			in.Error = why.Error()
		}
		return cached.Body, in
	}
	if opt.Offline {
		if have {
			return fromCache(true, errors.New(offlineWord(opt.Lang)))
		}
		err := errors.New(offlineNothing(opt.Lang))
		return errorJSON(err), SourceInfo{Error: err.Error()}
	}
	if have && !opt.Force && opt.TTL > 0 && now.Sub(cached.At) < opt.TTL {
		return fromCache(false, nil)
	}
	var body []byte
	var err error
	if r.Paging != nil {
		body, err = fetchPaged(ctx, client, r)
	} else {
		body, err = send(ctx, client, r)
	}
	if err == nil && !json.Valid(body) {
		err = errors.New("response is not JSON")
	}
	if err != nil {
		if have {
			return fromCache(true, err)
		}
		return errorJSON(err), SourceInfo{Error: err.Error()}
	}
	if opt.Cache != nil {
		opt.Cache.Put(key, body, now)
	}
	return body, SourceInfo{At: now.UnixMilli()}
}

// HistoryOf is the history of the sources a script asked for (the names in
// want), each as its responses with their times, the oldest first, at most
// limit of the newest. A name that is not one of the script's sources is
// ignored.
func HistoryOf(cache SourceCache, sources map[string]string, want []string, limit int, now time.Time) map[string][]HistoryPoint {
	out := map[string][]HistoryPoint{}
	if cache == nil {
		return out
	}
	for _, name := range want {
		tmpl, ok := sources[name]
		if !ok {
			continue
		}
		var pts []HistoryPoint
		for _, e := range cache.History(Expand(tmpl, now), limit) {
			pts = append(pts, HistoryPoint{T: e.At.UnixMilli(), V: e.Body})
		}
		out[name] = pts
	}
	return out
}
