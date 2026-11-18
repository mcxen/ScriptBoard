// Package usage reads how much of an AI service has been used and how much is
// left. A Provider is one source: the logs Claude Code keeps on this
// computer, or a service's balance endpoint. Providers are registered by ID,
// so another can be added without touching the rest of the app: Register it
// and name it in a script's meta().usage.
//
// Nothing here reads a credential file or calls a private endpoint. Local
// providers read logs that the tool itself writes; remote ones call a
// documented endpoint with a key the user pastes in.
package usage

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"sync"
	"time"
)

// Item is a named amount: a model's tokens, a day's total.
type Item struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

// Report is what a provider says about the service.
type Report struct {
	Provider string `json:"provider"`
	Name     string `json:"name"`
	Unit     string `json:"unit"` // "tokens", "CNY", "USD"
	// Used and Limit are for the quantity the provider reports a limit of.
	// Limit is 0 when the provider has none and the user has set no budget.
	Used  float64 `json:"used"`
	Limit float64 `json:"limit"`
	// Balance is what is left, when the provider reports a balance itself.
	BlockLimit float64 `json:"blockLimit"` // the budget of a five-hour block, when set
	Balance    float64 `json:"balance"`
	HasBalance bool    `json:"hasBalance"`
	// Windows are the usage of recent periods: today, week, month, block (the
	// last five hours of activity) for a local provider.
	Windows map[string]float64 `json:"windows"`
	// Models and Daily are breakdowns, largest model first, oldest day first.
	Models    []Item `json:"models"`
	Daily     []Item `json:"daily"`
	FetchedAt int64  `json:"fetchedAt"` // Unix milliseconds
	Note      string `json:"note,omitempty"`
}

// Options are what a provider needs for one fetch.
type Options struct {
	Key string // the service key, for a remote provider
	// Budget is the amount the user allows themselves in a month, in the
	// provider's unit; 0 is none. BlockBudget is the same for a five-hour
	// block.
	Budget, BlockBudget float64
	Now                 time.Time
	HTTP                *http.Client
	// BaseURL replaces a remote provider's address, for tests and proxies.
	BaseURL string
	// Dir replaces a local provider's log folder.
	Dir string
}

// Provider is one source of usage.
type Provider interface {
	ID() string
	Name() string
	// NeedsKey reports whether Fetch needs Options.Key.
	NeedsKey() bool
	Fetch(ctx context.Context, o Options) (Report, error)
}

var (
	mu        sync.RWMutex
	providers = map[string]Provider{}
)

// Register adds a provider, replacing one with the same ID.
func Register(p Provider) {
	mu.Lock()
	defer mu.Unlock()
	providers[p.ID()] = p
}

// Get returns the provider with id.
func Get(id string) (Provider, bool) {
	mu.RLock()
	defer mu.RUnlock()
	p, ok := providers[id]
	return p, ok
}

// List returns every provider, by ID.
func List() []Provider {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]Provider, 0, len(providers))
	for _, p := range providers {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

// Fetch asks the provider id for its report.
func Fetch(ctx context.Context, id string, o Options) (Report, error) {
	p, ok := Get(id)
	if !ok {
		return Report{}, errors.New("没有这个用量来源：" + id)
	}
	if p.NeedsKey() && o.Key == "" {
		return Report{}, errors.New("还没有填写密钥")
	}
	if o.Now.IsZero() {
		o.Now = time.Now()
	}
	r, err := p.Fetch(ctx, o)
	if err != nil {
		return Report{}, err
	}
	r.Provider, r.Name = p.ID(), p.Name()
	r.FetchedAt = o.Now.UnixMilli()
	return r, nil
}

func init() {
	Register(claudeCode{})
	Register(deepseek{})
	Register(openrouter{})
	Register(moonshot{})
	Register(siliconflow{})
}
