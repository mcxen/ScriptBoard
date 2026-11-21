package main

import (
	"math"
	"slices"
	"sort"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/store"
)

// The home page lays its cards on a grid: 12 columns on a wide window, 6 on a
// middling one, and one on a phone, where every card is a row of its own. A
// card's box is kept on the 12-column grid, so that a layout keeps its shape
// as the window is resized. A row takes the cards that fit it in order, and
// its last card takes whatever columns are left, so no row ends in a gap.

const gridCols = 12

// gridSpans are the widths of the size menu's four sizes on the 12-column
// grid: small, medium, large and wide.
var gridSpans = [5]int{3, 4, 8, 12, 2}

// heightStep is the step a card's height snaps to when it is resized.
const heightStep = 16

// minCardHeight is the least a card can be resized to, in DIPs at scale 1.
const minCardHeight = 96

// sizeNames are the names of the size menu's sizes.
func sizeNames() []string { return []string{tr("小"), tr("中"), tr("大"), tr("宽"), tr("迷你")} }

// colsFor is how many columns the home page has at a width.
func colsFor(width float32, compact bool) int {
	switch {
	case compact:
		// A phone shows two columns: small, medium and mini cards take one each,
		// large and wide cards the whole row.
		return 2
	case width < 520:
		return 1
	case width < 900:
		return 6
	}
	return gridCols
}

// toCols is a span on the 12-column grid, as the columns of a page with cols.
// On a narrower page a card keeps at least half the row, so that its title
// still fits.
func toCols(span, cols int) int {
	n := int(math.Ceil(float64(span*cols) / gridCols))
	if cols < gridCols {
		n = max(n, cols/2)
	}
	return min(max(n, 1), cols)
}

// fromCols is a span of a page's columns, on the 12-column grid.
func fromCols(span, cols int) int {
	n := int(math.Round(float64(span*gridCols) / float64(cols)))
	return min(max(n, 1), gridCols)
}

// boxOf is the box of a home card: its own, else the box of its size.
func (a *app) boxOf(id string) store.CardBox {
	if b, ok := a.set.CardBoxes[id]; ok && b.Span >= 1 && b.Height > 0 {
		return b
	}
	// A plugin's default size, when the user has not chosen one.
	if sc, ok := a.store.Script(id); ok && sc.Size != "" {
		if i := sizeIndex(sc.Size); i >= 0 {
			return store.CardBox{Span: gridSpans[i], Height: cardSizes[i][1]}
		}
	}
	size := a.sizeOf(id)
	return store.CardBox{Span: gridSpans[size], Height: cardSizes[size][1]}
}

// sizeMini is the smallest card: only its number, no title and no curve.
const sizeMini = 4

// classOf is what a card shows, by its box: the headline alone, a curve under
// it, a chart, or a strip of chart across a wide card.
func classOf(b store.CardBox) int {
	switch {
	case b.Height < 120:
		return sizeMini
	case b.Height < 170:
		return 0
	case b.Span >= gridCols && b.Height < 300:
		return 3
	case b.Span >= 8 && b.Height >= 300:
		return 2
	}
	return 1
}

// sizeIndex is the size menu's index of a size name, or -1 for an unknown name.
func sizeIndex(name string) int {
	for i, n := range []string{"small", "medium", "large", "wide", "mini"} {
		if n == name {
			return i
		}
	}
	return -1
}

// setSize gives a card one of the size menu's sizes.
func (a *app) setSize(id string, size int) {
	if a.set.CardBoxes == nil {
		a.set.CardBoxes = map[string]store.CardBox{}
	}
	a.set.CardBoxes[id] = store.CardBox{Span: gridSpans[size], Height: cardSizes[size][1]}
	if err := a.store.SetSettings(a.set); err != nil {
		a.set = a.store.Settings()
	}
}

// setAllSizes gives every card the same size again, and forgets where the user
// put them.
func (a *app) setAllSizes(size int) {
	a.cardSize = size
	a.set.CardSizes = nil
	a.set.CardBoxes = nil
	if err := a.store.SetSettings(a.set); err != nil {
		a.set = a.store.Settings()
	}
}

// orderedHome is the scripts in the order the user arranged the home cards;
// cards not arranged yet follow, in the order of the scripts.
func (a *app) orderedHome(scripts []store.Script) []store.Script {
	pos := map[string]int{}
	for i, id := range a.set.CardOrder {
		pos[id] = i
	}
	type keyed struct {
		sc store.Script
		k  int
	}
	ks := make([]keyed, len(scripts))
	for i, sc := range scripts {
		k := len(a.set.CardOrder) + i
		if p, ok := pos[sc.ID]; ok {
			k = p
		}
		ks[i] = keyed{sc, k}
	}
	sort.SliceStable(ks, func(i, j int) bool { return ks[i].k < ks[j].k })
	out := make([]store.Script, len(ks))
	for i := range ks {
		out[i] = ks[i].sc
	}
	// A plugin that names a home position, and that the user has not arranged,
	// goes to that place: the first of them first.
	var placed []store.Script
	for _, sc := range out {
		if _, arranged := pos[sc.ID]; !arranged && sc.Position > 0 {
			placed = append(placed, sc)
		}
	}
	sort.SliceStable(placed, func(i, j int) bool { return placed[i].Position < placed[j].Position })
	for _, sc := range placed {
		rest := out[:0:0]
		for _, o := range out {
			if o.ID != sc.ID {
				rest = append(rest, o)
			}
		}
		at := min(sc.Position-1, len(rest))
		out = append(rest[:at:at], append([]store.Script{sc}, rest[at:]...)...)
	}
	return out
}

// placed is a home card with the columns it takes on the page.
type placed struct {
	sc   store.Script
	box  store.CardBox
	span int
}

// packRows lays the cards in rows of cols columns, in order, and gives the last
// card of each row the columns the row has left.
func packRows(items []placed, cols int) [][]placed {
	var rows [][]placed
	var cur []placed
	used := 0
	for _, it := range items {
		it.span = toCols(it.box.Span, cols)
		if used+it.span > cols && len(cur) > 0 {
			rows = append(rows, cur)
			cur, used = nil, 0
		}
		cur = append(cur, it)
		used += it.span
	}
	if len(cur) > 0 {
		rows = append(rows, cur)
	}
	for _, r := range rows {
		sum := 0
		for _, it := range r {
			sum += it.span
		}
		r[len(r)-1].span += cols - sum
	}
	return rows
}

// homeGrid draws the cards in a single wrapping row whose widths are computed
// by packRows so each row fills the full content width. Keeping every card as a
// keyed direct child of one row preserves element identity (and active corner
// resize drags) even when a card wraps onto another line while being resized.
func (a *app) homeGrid(c *ui.Context, items []placed) {
	gap := float32(cardGap)
	rawUnit := (a.contentW() - gap*float32(a.cols-1)) / float32(a.cols)
	a.unit = float32(math.Floor(float64(rawUnit)*100)) / 100
	ui.Row(c).Wrap().Gap(gap).AlignItems(ui.Start).Children(func() {
		for _, row := range packRows(items, a.cols) {
			for _, it := range row {
				w := a.unit*float32(it.span) + gap*float32(it.span-1)
				a.card(c.Key("card:"+it.sc.ID), it.sc, it.box, it.span, w)
			}
		}
	})
}

// cardRef is what a card carries while it is dragged to a new place.
type cardRef struct{ id string }

// resize is a card being resized by its corner: where it was when the drag
// began, and how far the corner has moved since.
type resize struct {
	id     string
	span   int
	height float32
	dx, dy float32
}

// resizeHandle is the corner a card is resized by in edit mode. It is pinned
// to the bottom-right corner with a stable key so dragging across size classes
// (mini ↔ small ↔ medium ↔ large ↔ wide) never breaks the active pointer drag.
func (a *app) resizeHandle(c *ui.Context, id string, span int, b store.CardBox) {
	p := pal(c)
	active := a.rz != nil && a.rz.id == id
	bg := p.card.Alpha(0.94)
	fg := p.accent
	if active {
		bg = p.accent
		fg = ui.Hex("#FFFFFF")
	}
	h := ui.Row(c.Key("resize:"+id)).
		Absolute().Right(6).Bottom(6).
		Size(26, 26).Radius(7).
		Background(bg).
		BorderWidth(1).BorderColor(p.accent.Alpha(0.6)).
		AlignItems(ui.Center).Justify(ui.Center).
		Cursor(ui.CursorResizeNWSE).
		Label("resize:" + id)
	h.Children(func() {
		ui.Icon(c, icon("maximize-2")).FontSize(14).TextColor(fg).PassThrough()
	})
	dx, dy, ok := h.Dragged()
	switch {
	case ok:
		if a.rz == nil || a.rz.id != id {
			a.rz = &resize{id: id, span: span, height: b.Height}
		}
		a.rz.dx += dx
		a.rz.dy += dy
		a.applyResize()
	case a.rz != nil && a.rz.id == id:
		a.rz = nil
		if err := a.store.SetSettings(a.set); err != nil {
			a.set = a.store.Settings()
		}
	}
}

// applyResize puts the card at the width and height its corner has been
// dragged to. The width snaps to the nearest span of columns, the height to
// heightStep. The change is kept when the drag ends.
func (a *app) applyResize() {
	r := a.rz
	gap := float32(cardGap)
	startPx := a.unit*float32(r.span) + gap*float32(r.span-1)
	target := startPx + r.dx
	span := int(math.Round(float64((target + gap) / (a.unit + gap))))
	span = min(max(span, 1), a.cols)
	h := r.height + r.dy/a.scale()
	h = max(minCardHeight, float32(math.Round(float64(h)/heightStep))*heightStep)
	if a.set.CardBoxes == nil {
		a.set.CardBoxes = map[string]store.CardBox{}
	}
	a.set.CardBoxes[r.id] = store.CardBox{Span: fromCols(span, a.cols), Height: h}
}

// moveCard puts the card from where the home page shows it before the card to.
func (a *app) moveCard(from, to string) {
	order := make([]string, 0, len(a.homeIDs))
	for _, id := range a.homeIDs {
		if id != from {
			order = append(order, id)
		}
	}
	at := slices.Index(order, to)
	if at < 0 {
		return
	}
	a.set.CardOrder = slices.Insert(order, at, from)
	if err := a.store.SetSettings(a.set); err != nil {
		a.set = a.store.Settings()
	}
}
