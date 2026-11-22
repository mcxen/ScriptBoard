package main

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type displayInfo struct {
	ID       int64
	Label    string
	Internal bool
	Primary  bool
	Bounds   mygo.Rectangle
	WorkArea mygo.Rectangle
}

func connectedDisplays() []displayInfo {
	ds := mygo.Screen.Displays()
	out := make([]displayInfo, 0, len(ds))
	for _, d := range ds {
		out = append(out, displayInfo{
			ID:       d.ID,
			Label:    d.Label,
			Internal: d.Internal,
			Primary:  d.Primary,
			Bounds:   d.Bounds,
			WorkArea: d.WorkArea,
		})
	}
	return out
}

func displayKindLabel(internal bool) string {
	if internal {
		return tr("内接")
	}
	return tr("外接")
}

func displaysSummary(displays []displayInfo) string {
	if len(displays) == 0 {
		return ""
	}
	var parts []string
	for _, d := range displays {
		name := strings.TrimSpace(d.Label)
		if name == "" {
			name = fmt.Sprintf("#%d", d.ID)
		}
		parts = append(parts, fmt.Sprintf("%s: %s", displayKindLabel(d.Internal), name))
	}
	return strings.Join(parts, " · ")
}

func nearestDisplay(displays []displayInfo, p mygo.Point) (displayInfo, bool) {
	if len(displays) == 0 {
		return displayInfo{}, false
	}
	best := displays[0]
	bestDist := math.Inf(1)
	for _, d := range displays {
		b := d.Bounds
		dx := math.Max(math.Max(float64(b.X-p.X), 0), float64(p.X-(b.X+b.Width)))
		dy := math.Max(math.Max(float64(b.Y-p.Y), 0), float64(p.Y-(b.Y+b.Height)))
		if dist := dx*dx + dy*dy; dist < bestDist {
			best = d
			bestDist = dist
		}
	}
	return best, true
}

func matchingDisplay(displays []displayInfo, r mygo.Rectangle) (displayInfo, bool) {
	if len(displays) == 0 || r.Width <= 0 || r.Height <= 0 {
		return displayInfo{}, false
	}
	best := displays[0]
	bestArea := -1
	for _, d := range displays {
		b := d.Bounds
		w := min(b.X+b.Width, r.X+r.Width) - max(b.X, r.X)
		h := min(b.Y+b.Height, r.Y+r.Height) - max(b.Y, r.Y)
		area := max(w, 0) * max(h, 0)
		if area > bestArea {
			best = d
			bestArea = area
		}
	}
	if bestArea <= 0 {
		return displayInfo{}, false
	}
	return best, true
}

func primaryDisplay(displays []displayInfo) (displayInfo, bool) {
	if len(displays) == 0 {
		return displayInfo{}, false
	}
	for _, d := range displays {
		if d.Primary {
			return d, true
		}
	}
	return displays[0], true
}

// resolveTargetDisplay picks the target display (respecting "cursor",
// "internal", or "external"), decides whether native NSPopover anchored to
// trayBounds can be used on that display, and computes a work-area-clamped
// screen position when opening a floating panel window on another display.
func resolveTargetDisplay(displays []displayInfo, pref string, cursor mygo.Point, trayBounds mygo.Rectangle, panelW, panelH int) (target displayInfo, useNativePopover bool, pos mygo.Point) {
	if len(displays) == 0 {
		return displayInfo{}, trayBounds.Width > 0 && trayBounds.Height > 0, mygo.Point{}
	}
	clicked, hasClicked := nearestDisplay(displays, cursor)
	trayDisp, hasTray := matchingDisplay(displays, trayBounds)
	prim, _ := primaryDisplay(displays)

	switch strings.ToLower(strings.TrimSpace(pref)) {
	case "internal":
		found := false
		for _, d := range displays {
			if d.Internal {
				if !found || d.Primary {
					target = d
					found = true
				}
			}
		}
		if !found {
			if hasClicked {
				target = clicked
			} else {
				target = prim
			}
		}
	case "external":
		found := false
		if hasClicked && !clicked.Internal {
			target = clicked
			found = true
		} else {
			for _, d := range displays {
				if !d.Internal {
					target = d
					found = true
					break
				}
			}
		}
		if !found {
			if hasClicked {
				target = clicked
			} else {
				target = prim
			}
		}
	default: // "" or "cursor"
		switch {
		case hasClicked:
			target = clicked
		case hasTray:
			target = trayDisp
		default:
			target = prim
		}
	}

	useNativePopover = hasTray && trayDisp.ID == target.ID

	wa := target.WorkArea
	if wa.Width <= 0 || wa.Height <= 0 {
		wa = target.Bounds
	}
	if panelW <= 0 {
		panelW = 300
	}
	if panelH <= 0 {
		panelH = 320
	}

	// Anchor horizontally near cursor if cursor is inside target display (or tray
	// bounds if on target display); otherwise place at top-right of target work area.
	var x, y int
	tb := target.Bounds
	cursorInTarget := cursor.X >= tb.X && cursor.X < tb.X+tb.Width && cursor.Y >= tb.Y && cursor.Y < tb.Y+tb.Height
	switch {
	case useNativePopover:
		x = trayBounds.X + (trayBounds.Width-panelW)/2
	case cursorInTarget:
		x = cursor.X - panelW/2
	default:
		x = wa.X + wa.Width - panelW - 12
	}

	minX := wa.X + 8
	maxX := wa.X + max(8, wa.Width-panelW-8)
	if minX > maxX {
		minX, maxX = wa.X, wa.X
	}
	x = max(minX, min(x, maxX))

	// If cursor is near bottom taskbar of target display, place panel above;
	// otherwise place near top menu bar edge of target display.
	if cursorInTarget && cursor.Y > tb.Y+tb.Height/2 {
		y = wa.Y + wa.Height - panelH - 8
	} else {
		y = wa.Y + 6
	}
	minY := wa.Y + 6
	maxY := wa.Y + max(6, wa.Height-panelH-6)
	if minY > maxY {
		minY, maxY = wa.Y, wa.Y
	}
	y = max(minY, min(y, maxY))

	return target, useNativePopover, mygo.Point{X: x, Y: y}
}

func (a *app) recentlyClosedFlyout(id string) bool {
	if a.barFlyoutClosedAt == nil {
		return false
	}
	t, ok := a.barFlyoutClosedAt[id]
	return ok && time.Since(t) < 250*time.Millisecond
}

func (a *app) markFlyoutClosed(id string) {
	delete(a.barFlyouts, id)
	if a.barFlyoutClosedAt == nil {
		a.barFlyoutClosedAt = map[string]time.Time{}
	}
	a.barFlyoutClosedAt[id] = time.Now()
}

func (a *app) openTrayPanelWindow(id string, t *mygo.Tray, pref string, panelW, panelH int, build func(c *ui.Context)) *mygo.Window {
	displays := connectedDisplays()
	cursor := mygo.Screen.CursorScreenPoint()
	trayBounds := t.Bounds()
	_, useNativePopover, pos := resolveTargetDisplay(displays, pref, cursor, trayBounds, panelW, panelH)

	if useNativePopover {
		f := mygo.NewFlyout(mygo.FlyoutOptions{
			Tray:         t,
			Placement:    mygo.PlacementBottom,
			Gap:          4,
			Width:        panelW,
			Height:       panelH,
			Focusable:    true,
			Popover:      true,
			Shadow:       true,
			CornerRadius: 14,
			Content:      ui.View(build),
		})
		f.OnClosed(func() {
			a.markFlyoutClosed(id)
		})
		return f
	}

	var win *mygo.Window
	win = mygo.NewWindow(mygo.WindowOptions{
		Panel:             true,
		Frameless:         true,
		Transparent:       true,
		Vibrancy:          mygo.VibrancyPopover,
		CornerRadius:      14,
		Width:             panelW,
		Height:            panelH,
		X:                 pos.X,
		Y:                 pos.Y,
		UseContentSize:    true,
		DisableResize:     true,
		DisableMinimize:   true,
		DisableMaximize:   true,
		DisableFullScreen: true,
		AlwaysOnTop:       true,
		SkipTaskbar:       true,
		Content: ui.View(func(c *ui.Context) {
			if c.Shortcut(0, ui.KeyEscape) && win != nil {
				win.Close()
			}
			build(c)
		}),
	})
	win.SetPosition(pos.X, pos.Y)
	win.OnBlur(func() {
		win.Close()
	})
	win.OnClosed(func() {
		a.markFlyoutClosed(id)
	})
	return win
}
