package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/cache"
	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

// sourceTTL is how long a response counts as fresh: opening a card again
// within it does not ask the network again. The refresh button always asks.
const sourceTTL = 60 * time.Second

// defaultHistory is how many past responses a script gets when it asks for the
// history of a source without saying how many.
const defaultHistory = 120

// cacheAdapter lets the engine use the on-disk cache.
type cacheAdapter struct{ s *cache.Store }

func (c cacheAdapter) Get(url string) (engine.CachedBody, bool) {
	e, ok := c.s.Get(url)
	return engine.CachedBody{At: e.At, Body: e.Body}, ok
}

func (c cacheAdapter) Put(url string, body []byte, at time.Time) { _ = c.s.Put(url, body, at) }

func (c cacheAdapter) History(url string, limit int) []engine.CachedBody {
	es := c.s.History(url, limit)
	out := make([]engine.CachedBody, len(es))
	for i, e := range es {
		out[i] = engine.CachedBody{At: e.At, Body: e.Body}
	}
	return out
}

// historyWanted is the sources a script wants the history of, and how many
// responses of each.
func historyWanted(m engine.Meta) ([]string, int) {
	if len(m.History) == 0 {
		return nil, 0
	}
	n := m.HistoryLimit
	if n <= 0 {
		n = defaultHistory
	}
	return m.History, min(n, cache.MaxHistoryEntries)
}

// dataTimes says how old the data of a run is: when the oldest source was
// fetched, and whether any of it stands in for a request that did not work.
func dataTimes(info map[string]engine.SourceInfo) (at int64, stale bool) {
	for _, in := range info {
		if in.At > 0 && (at == 0 || in.At < at) {
			at = in.At
		}
		stale = stale || in.Stale
	}
	return at, stale
}

// resultKey names the saved result of a run: the card and its dimensions, the
// language and the script, so that a changed script never shows an old result.
func resultKey(sc store.Script, run, lang string) string {
	h := sha1.Sum([]byte(run + "\x00" + lang + "\x00" + sc.Source))
	return hex.EncodeToString(h[:])
}

// savedResult is the result a run last gave, kept on disk so that a card shows
// something at once when the app opens, before the network answers. Only
// scripts that read the network keep one: the others are quick, and the values
// they show can change under the user's hand.
func (a *app) savedResult(sc store.Script, run string, m engine.Meta) (engine.Result, bool) {
	if a.cache == nil || len(m.Sources) == 0 || strings.HasPrefix(run, draftKey) {
		return engine.Result{}, false
	}
	data, at, ok := a.cache.GetResultOf(sc.ID, resultKey(sc, run, curLang()))
	if !ok {
		return engine.Result{}, false
	}
	var res engine.Result
	if json.Unmarshal(data, &res) != nil || res.Chart == nil && res.Big == "" {
		return engine.Result{}, false
	}
	if res.DataAt == 0 {
		res.DataAt = at.UnixMilli()
	}
	return res, true
}

// saveResult keeps a result that worked.
func (a *app) saveResult(sc store.Script, run string, m engine.Meta, res engine.Result) {
	if a.cache == nil || len(m.Sources) == 0 || strings.HasPrefix(run, draftKey) {
		return
	}
	if data, err := json.Marshal(res); err == nil {
		_ = a.cache.PutResultOf(sc.ID, resultKey(sc, run, curLang()), data, a.now())
	}
}

// cacheSize says how much the cache takes, for the settings.
func (a *app) cacheSize() string {
	if a.cache == nil {
		return "0 B"
	}
	n := a.cache.Size()
	switch {
	case n < 1<<10:
		return fmt.Sprintf("%d B", n)
	case n < 1<<20:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
}

// clearCache forgets every saved response, history and result.
func (a *app) clearCache() error {
	if a.cache == nil {
		return nil
	}
	err := a.cache.Clear()
	a.invalidateAll()
	return err
}

// ageText says how long ago ms was, for the chip of a card that shows old data.
func ageText(ms int64, now time.Time) string {
	d := now.Sub(time.UnixMilli(ms))
	switch {
	case ms == 0:
		return ""
	case d < time.Minute:
		return tr("刚刚")
	case d < time.Hour:
		return fmt.Sprintf(tr("%d 分钟前"), int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf(tr("%d 小时前"), int(d.Hours()))
	}
	return fmt.Sprintf(tr("%d 天前"), int(d.Hours()/24))
}

// freshness says under a card's headline that what it shows is not from a
// request made just now: saved from an earlier run, or standing in for a
// request that failed or was not made offline.
func (a *app) freshness(c *ui.Context, st *runState, size int) {
	p := pal(c)
	switch {
	case st.updating:
		ui.Text(c, tr("更新中…")).FontSize(fsTiny).TextColor(p.muted).NoWrap()
	case st.old || st.res.Stale:
		if age := ageText(st.res.DataAt, a.now()); age != "" {
			ui.Text(c, fmt.Sprintf(tr("缓存 · %s"), age)).FontSize(fsTiny).TextColor(p.muted).NoWrap()
		}
	}
}
