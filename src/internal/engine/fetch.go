package engine

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
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

// Fetch loads every source. A source that fails becomes {"error": "..."}, so
// the script can show its fallback instead of the whole run failing.
func Fetch(ctx context.Context, client *http.Client, sources map[string]string, now time.Time) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(sources))
	for name, tmpl := range sources {
		body, err := get(ctx, client, Expand(tmpl, now))
		switch {
		case err != nil:
			out[name] = errorJSON(err)
		case !json.Valid(body):
			out[name] = errorJSON(errors.New("response is not JSON"))
		default:
			out[name] = body
		}
	}
	return out
}

// Offline gives every source the same error, for runs without the network.
func Offline(sources map[string]string) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(sources))
	for name := range sources {
		out[name] = errorJSON(errors.New("offline mode"))
	}
	return out
}

func get(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	if client == nil {
		client = http.DefaultClient
	}
	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
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
