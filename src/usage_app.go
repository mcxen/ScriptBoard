package main

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/store"
	"scriptboard/src/internal/usage"
)

// usageCache keeps the last report of each provider for a little while, so
// that several cards and every redraw do not each read the logs or call the
// service. A failure is kept shorter, so that a fixed key shows soon.
type usageCache struct {
	mu      sync.Mutex
	entries map[string]usageEntry
}

type usageEntry struct {
	at     time.Time
	report map[string]any
}

const (
	usageTTL      = 90 * time.Second
	usageErrorTTL = 20 * time.Second
)

// usageInput gathers the report of each provider a script names, as the plain
// maps the script reads. It is safe to call from any goroutine.
func (a *app) usageInput(ids []string, now time.Time) map[string]any {
	out := make(map[string]any, len(ids))
	for _, id := range ids {
		out[id] = a.usageReport(id, now)
	}
	return out
}

// usageReport is the report of one provider for a script, with its notes and
// errors in the language in use. The cache keeps them as the packages wrote
// them, so a change of language needs no new request.
func (a *app) usageReport(id string, now time.Time) map[string]any {
	r := a.usageReportRaw(id, now)
	out := make(map[string]any, len(r))
	for k, v := range r {
		if s, ok := v.(string); ok && (k == "note" || k == "error") {
			v = trMsg(s)
		}
		out[k] = v
	}
	return out
}

func (a *app) usageReportRaw(id string, now time.Time) map[string]any {
	a.usage.mu.Lock()
	if e, ok := a.usage.entries[id]; ok {
		ttl := usageTTL
		if _, failed := e.report["error"]; failed {
			ttl = usageErrorTTL
		}
		if now.Sub(e.at) < ttl {
			a.usage.mu.Unlock()
			return e.report
		}
	}
	a.usage.mu.Unlock()

	set := a.store.Settings()
	cfg := set.Usage[id]
	var report map[string]any
	switch {
	case set.Offline && id != "claude-code":
		report = map[string]any{"error": tr("离线模式：不请求网络")}
	default:
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		r, err := usage.Fetch(ctx, id, usage.Options{
			Key: cfg.Key, Budget: cfg.Budget, BlockBudget: cfg.BlockBudget,
			Now: now, HTTP: a.client, Dir: a.usageDir,
		})
		if err != nil {
			report = map[string]any{"error": err.Error()}
		} else {
			b, _ := json.Marshal(r)
			_ = json.Unmarshal(b, &report)
		}
	}
	a.usage.mu.Lock()
	if a.usage.entries == nil {
		a.usage.entries = map[string]usageEntry{}
	}
	a.usage.entries[id] = usageEntry{at: now, report: report}
	a.usage.mu.Unlock()
	return report
}

// forgetUsage drops what is cached, after a key or a budget changed.
func (a *app) forgetUsage() {
	a.usage.mu.Lock()
	a.usage.entries = nil
	a.usage.mu.Unlock()
	a.invalidateAll()
}

// usageForm is the text and numbers being typed for each provider.
type usageForm struct {
	key         string
	budget      float64
	blockBudget float64
	loaded      bool
}

// usageGroups are the settings of the AI usage providers: a key for each
// remote one, and the budgets that turn a count into "how much is left".
func (a *app) usageGroups(c *ui.Context, compact bool) {
	rows := []row{}
	for _, p := range usage.List() {
		p := p
		f := a.usageForms[p.ID()]
		if f == nil {
			cfg := a.store.Settings().Usage[p.ID()]
			f = &usageForm{key: cfg.Key, budget: cfg.Budget, blockBudget: cfg.BlockBudget}
			a.usageForms[p.ID()] = f
		}
		save := func() {
			st := a.store.Settings()
			if st.Usage == nil {
				st.Usage = map[string]store.UsageConfig{}
			}
			st.Usage[p.ID()] = store.UsageConfig{Key: f.key, Budget: f.budget, BlockBudget: f.blockBudget}
			if err := a.store.SetSettings(st); err == nil {
				a.set = st
				a.forgetUsage()
			}
		}
		note := tr("读取本机 Claude Code 的日志，不需要密钥。填月额度可以看到剩余。")
		if p.NeedsKey() {
			note = tr("填密钥后读取余额；填「充值总额」可以看到剩余比例。密钥只保存在这台电脑上。")
		}
		rows = append(rows, row{title: tr(p.Name()), note: note, control: func() {
			if p.NeedsKey() {
				if passwordInput(c, &f.key, tr("API 密钥")).Width(180).Changed() {
					save()
				}
			}
			label := tr("月额度（tokens）")
			if p.NeedsKey() {
				label = tr("充值总额")
			}
			if numberInput(c, &f.budget, 0, 1e12, 1).Width(120).Label(label).Changed() {
				save()
			}
			if p.ID() == "claude-code" {
				if numberInput(c, &f.blockBudget, 0, 1e12, 1).Width(120).Label(tr("5 小时额度")).Changed() {
					save()
				}
			}
		}})
	}
	group(c, tr("AI 用量"), compact, rows...)
}
