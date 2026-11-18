package main

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// Numbers that move: a figure eases from its last value to a new one, the way
// a quote board's digits roll, and the page is redrawn until it settles. It is
// kept per key (a card's number, the hero of a full view), on the UI thread.

// tickDuration is how long a number takes to reach its new value.
const tickDuration = 350 * time.Millisecond

type tickState struct {
	from, to, shown float64
	start           time.Time
}

var ticks = map[string]*tickState{}

// tickNumber is the value to show for key, moving toward v. It asks for a
// new frame while it moves.
func tickNumber(c *ui.Context, key string, v float64) float64 {
	now := c.Now()
	st, ok := ticks[key]
	if !ok {
		ticks[key] = &tickState{from: v, to: v, shown: v, start: now}
		return v
	}
	if st.to != v {
		st.from, st.to, st.start = st.shown, v, now
	}
	t := float64(now.Sub(st.start)) / float64(tickDuration)
	if t >= 1 {
		st.shown = v
		return v
	}
	// Ease out: fast at first, settling on the new value.
	e := 1 - (1-t)*(1-t)*(1-t)
	st.shown = st.from + (st.to-st.from)*e
	c.Invalidate()
	return st.shown
}

// numberText matches a figure at the start of a text, with what comes before
// it (such as "$") and after it (such as " kg") kept as they are.
var numberText = regexp.MustCompile(`^([^0-9+\-]*)([+-]?[0-9][0-9,]*(?:\.[0-9]+)?)(.*)$`)

// tickText animates the figure in text (the "65.3" of "65.3 kg") toward its
// value, with the same number of decimals. A text with no figure is shown as it
// is.
func tickText(c *ui.Context, key, text string) string {
	m := numberText.FindStringSubmatch(text)
	if m == nil {
		return text
	}
	target, err := strconv.ParseFloat(strings.ReplaceAll(m[2], ",", ""), 64)
	if err != nil {
		return text
	}
	dec := 0
	if i := strings.IndexByte(m[2], '.'); i >= 0 {
		dec = len(m[2]) - i - 1
	}
	shown := tickNumber(c, key, target)
	return m[1] + strconv.FormatFloat(shown, 'f', dec, 64) + m[3]
}
