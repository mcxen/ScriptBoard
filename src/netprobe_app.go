package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/netprobe"
)

type netProbeCache struct {
	mu      sync.Mutex
	entries map[string]netProbeEntry
}

type netProbeEntry struct {
	at   time.Time
	data map[string]any
	err  string
}

const (
	netProbeTTL      = 60 * time.Second
	netProbeErrorTTL = 15 * time.Second
)

func netProbeKey(cfg engine.NetProbeConfig) string {
	rounds := cfg.Rounds
	if rounds <= 0 {
		rounds = 12
	}
	return fmt.Sprintf("r=%d|risk=%v|split=%v", rounds, cfg.WantRisk(), cfg.WantSplit())
}

func (a *app) probeClient() *http.Client {
	a.netProbe.mu.Lock()
	defer a.netProbe.mu.Unlock()
	if a.probeHTTP == nil {
		a.probeHTTP = netprobe.NewClient(10 * time.Second)
	}
	return a.probeHTTP
}

func (a *app) netProbeInput(cfg engine.NetProbeConfig, force bool, now time.Time) (map[string]any, int64, bool) {
	if a.netProbeFn != nil {
		data := a.netProbeFn(context.Background(), cfg, force)
		if data == nil {
			return map[string]any{"error": tr("离线模式")}, 0, false
		}
		at := now.UnixMilli()
		if v, ok := data["fetchedAt"].(int64); ok && v > 0 {
			at = v
		} else if f, ok := data["fetchedAt"].(float64); ok && f > 0 {
			at = int64(f)
		}
		stale, _ := data["stale"].(bool)
		return data, at, stale
	}

	set := a.store.Settings()
	key := netProbeKey(cfg)

	a.netProbe.mu.Lock()
	cached, hasCached := a.netProbe.entries[key]
	a.netProbe.mu.Unlock()

	if set.Offline {
		if hasCached && cached.err == "" && len(cached.data) > 0 {
			return cached.data, cached.at.UnixMilli(), true
		}
		return map[string]any{"error": tr("离线模式")}, 0, false
	}

	if !force && hasCached {
		ttl := netProbeTTL
		if cached.err != "" {
			ttl = netProbeErrorTTL
		}
		if now.Sub(cached.at) < ttl {
			if cached.err != "" {
				return map[string]any{"error": cached.err}, cached.at.UnixMilli(), false
			}
			return cached.data, cached.at.UnixMilli(), false
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	rep := netprobe.Run(ctx, netprobe.Config{
		Rounds:       cfg.Rounds,
		IncludeRisk:  cfg.WantRisk(),
		IncludeSplit: cfg.WantSplit(),
	}, netprobe.Options{
		Client: a.probeClient(),
		Now:    func() time.Time { return now },
	})

	if rep.Error != "" && hasCached && cached.err == "" && len(cached.data) > 0 {
		return cached.data, cached.at.UnixMilli(), true
	}

	var data map[string]any
	if b, err := json.Marshal(rep); err == nil {
		_ = json.Unmarshal(b, &data)
	}
	if data == nil {
		data = map[string]any{"error": rep.Error}
	}

	a.netProbe.mu.Lock()
	if a.netProbe.entries == nil {
		a.netProbe.entries = map[string]netProbeEntry{}
	}
	a.netProbe.entries[key] = netProbeEntry{at: now, data: data, err: rep.Error}
	a.netProbe.mu.Unlock()

	if rep.Error != "" {
		return map[string]any{"error": rep.Error}, rep.FetchedAt, false
	}
	return data, rep.FetchedAt, false
}

func (a *app) forgetNetProbe() {
	a.netProbe.mu.Lock()
	a.netProbe.entries = nil
	a.netProbe.mu.Unlock()
}
