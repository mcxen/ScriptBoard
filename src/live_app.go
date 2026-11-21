package main

import (
	"sync/atomic"
	"time"

	"scriptboard/src/internal/engine"
)

// A script with live sources (meta live) runs again every few seconds while its
// card is on screen, in the menu bar or in its full view, so that the history of
// its sources grows into a line that moves. The loop ticks only while such a card
// is drawn: a tick redraws the window, the redraw marks the cards it draws, and
// the next tick runs the ones that are due. With no live card drawn nothing ticks.

// liveTick is how often the live cards are checked.
const liveTick = time.Second

// liveWatch is how long a card counts as on screen after it was last drawn. A
// card is drawn on each tick, so this allows one tick to be missed.
const liveWatch = 3 * time.Second

// liveState is what the live loop knows: whether a live card is drawn (so that
// the loop should tick), when each live card was last drawn, and when each one's
// run was last asked for. The maps are kept on the main thread.
type liveState struct {
	on   atomic.Bool
	seen map[string]time.Time
	at   map[string]time.Time
}

// watchLive marks a live card as drawn now, so that the loop keeps ticking.
func (a *app) watchLive(id string, m engine.Meta) {
	if m.Live == nil {
		return
	}
	if a.live.seen == nil {
		a.live.seen = map[string]time.Time{}
	}
	a.live.seen[id] = a.now()
	a.live.on.Store(true)
}

// liveLoop posts a tick each second, while a live card is drawn.
func (a *app) liveLoop() {
	for range time.Tick(liveTick) {
		if a.live.on.Load() {
			a.post(a.liveCheck)
		}
	}
}

// liveCheck runs on the main thread: each live card that is on screen and due
// asks for its data again. A card in the menu bar counts as on screen.
func (a *app) liveCheck() {
	a.live.on.Store(false)
	if a.set.Offline {
		return
	}
	now := a.now()
	for _, sc := range a.store.Scripts() {
		meta, err := a.metaOf(sc)
		if err != nil || meta.Live == nil {
			continue
		}
		if a.menuBarOn(sc.ID) {
			a.watchLive(sc.ID, meta)
		}
		if !liveDue(now, a.live.seen[sc.ID], a.live.at[sc.ID], meta.LiveEvery()) {
			continue
		}
		_, vals := a.viewOf(sc.ID, meta.Dims)
		key := a.runKeyOf(sc.ID, sc, meta, vals)
		if st, ok := a.runs[key]; ok && (st.loading || st.updating) {
			continue // the last run is still under way
		}
		if a.live.at == nil {
			a.live.at = map[string]time.Time{}
		}
		a.live.at[sc.ID] = now
		a.invalidate(sc.ID)
		a.ensure(key, sc, vals)
	}
}

// liveDue says whether a live card's run is due: the card was drawn lately, and
// its last run was asked for at least an interval ago (or never).
func liveDue(now, seen, last time.Time, every time.Duration) bool {
	if seen.IsZero() || now.Sub(seen) > liveWatch {
		return false
	}
	return last.IsZero() || now.Sub(last) >= every
}
