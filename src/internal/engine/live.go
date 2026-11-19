package engine

import "time"

// LiveMinEvery is the shortest interval, in seconds, a live source is asked for
// again. A faster poll loads the source more than it improves the line.
const LiveMinEvery = 5

// Live says which of a script's sources are asked for again every few seconds
// while its card is on screen, in the menu bar or in its full view, so that
// their history grows into a line that moves. The other sources keep the
// cache's minute.
type Live struct {
	// Sources are the names of the sources to ask for again (names in Sources).
	Sources []string `json:"sources"`
	// Every is the interval in seconds; 0 and anything under LiveMinEvery is
	// LiveMinEvery.
	Every int `json:"every"`
}

// LiveEvery is the interval the live sources are asked for again, or 0 when the
// script has none.
func (m Meta) LiveEvery() time.Duration {
	if m.Live == nil {
		return 0
	}
	return time.Duration(max(m.Live.Every, LiveMinEvery)) * time.Second
}

// LiveTTL gives each live source the time its cached response counts as fresh:
// its interval less a second, so that each run asks the source again. Names that
// are not sources of the script are left out; nil when there are none.
func (m Meta) LiveTTL() map[string]time.Duration {
	if m.Live == nil {
		return nil
	}
	var out map[string]time.Duration
	for _, name := range m.Live.Sources {
		if _, ok := m.Sources[name]; !ok {
			continue
		}
		if out == nil {
			out = map[string]time.Duration{}
		}
		out[name] = m.LiveEvery() - time.Second
	}
	return out
}
