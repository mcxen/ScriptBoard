package main

import (
	"runtime"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestResolveTargetDisplayBuiltInAndExternal(t *testing.T) {
	internal := displayInfo{
		ID:       1,
		Label:    "Built-in Retina Display",
		Internal: true,
		Primary:  true,
		Bounds:   mygo.Rectangle{X: 0, Y: 0, Width: 1512, Height: 982},
		WorkArea: mygo.Rectangle{X: 0, Y: 38, Width: 1512, Height: 944},
	}
	external := displayInfo{
		ID:       2,
		Label:    "Studio Display",
		Internal: false,
		Primary:  false,
		Bounds:   mygo.Rectangle{X: 1512, Y: 0, Width: 2560, Height: 1440},
		WorkArea: mygo.Rectangle{X: 1512, Y: 25, Width: 2560, Height: 1415},
	}
	displays := []displayInfo{internal, external}
	trayOnInternal := mygo.Rectangle{X: 1200, Y: 4, Width: 60, Height: 24}
	trayOnExternal := mygo.Rectangle{X: 3600, Y: 4, Width: 60, Height: 24}

	// 1. Default ("cursor"): clicking on external opens on external with native popover when tray is on external
	target, pop, _ := resolveTargetDisplay(displays, "", mygo.Point{X: 3620, Y: 12}, trayOnExternal, 300, 512)
	if target.ID != 2 || !pop {
		t.Fatalf("cursor on external: got target=%d pop=%v, want target=2 pop=true", target.ID, pop)
	}

	// 2. Default ("cursor"): clicking on external while trayBounds still reports internal -> switches to external window
	target, pop, pos := resolveTargetDisplay(displays, "cursor", mygo.Point{X: 2800, Y: 10}, trayOnInternal, 300, 512)
	if target.ID != 2 || pop {
		t.Fatalf("cursor on external with internal tray: got target=%d pop=%v, want target=2 pop=false", target.ID, pop)
	}
	if pos.X < external.WorkArea.X || pos.X+300 > external.WorkArea.X+external.WorkArea.Width {
		t.Fatalf("pos.X=%d out of external work area %+v", pos.X, external.WorkArea)
	}

	// 3. Forced "external" while clicked on internal -> opens on external display clamped inside external work area
	target, pop, pos = resolveTargetDisplay(displays, "external", mygo.Point{X: 1220, Y: 12}, trayOnInternal, 300, 512)
	if target.ID != 2 || pop {
		t.Fatalf("forced external: got target=%d pop=%v, want target=2 pop=false", target.ID, pop)
	}
	if pos.X < external.WorkArea.X || pos.Y < external.WorkArea.Y {
		t.Fatalf("pos=%+v not inside external work area %+v", pos, external.WorkArea)
	}

	// 4. Forced "internal" while clicked on external -> opens on internal display
	target, pop, pos = resolveTargetDisplay(displays, "internal", mygo.Point{X: 3620, Y: 12}, trayOnExternal, 300, 512)
	if target.ID != 1 || pop {
		t.Fatalf("forced internal: got target=%d pop=%v, want target=1 pop=false", target.ID, pop)
	}
	if pos.X < internal.WorkArea.X || pos.X+300 > internal.WorkArea.X+internal.WorkArea.Width {
		t.Fatalf("pos.X=%d out of internal work area %+v", pos.X, internal.WorkArea)
	}

	// 5. Displays summary labels built-in vs external
	summary := displaysSummary(displays)
	if !strings.Contains(summary, "内接") || !strings.Contains(summary, "外接") {
		t.Fatalf("unexpected displaysSummary: %q", summary)
	}
}

func TestTrayFlyoutOptionsNonActivating(t *testing.T) {
	opts := trayFlyoutOptions(nil, 300, 420, mygo.VibrancyPopover, func(*ui.Context) {})
	if opts.Popover {
		t.Fatalf("opts.Popover = true, want false so clicking menubar does not call activateIgnoringOtherApps and raise main window")
	}
	if !opts.Focusable || !opts.Shadow || opts.CornerRadius != 14 {
		t.Fatalf("unexpected flyout presentation options: Focusable=%v Shadow=%v CornerRadius=%d", opts.Focusable, opts.Shadow, opts.CornerRadius)
	}
	if opts.Placement != mygo.PlacementBottom || opts.Gap != 4 || opts.Width != 300 || opts.Height != 420 {
		t.Fatalf("unexpected flyout geometry options: Placement=%q Gap=%d Width=%d Height=%d", opts.Placement, opts.Gap, opts.Width, opts.Height)
	}
	if opts.Vibrancy != mygo.VibrancyPopover {
		t.Fatalf("opts.Vibrancy = %q, want %q", opts.Vibrancy, mygo.VibrancyPopover)
	}
	if wantHidden := runtime.GOOS == "darwin"; opts.Hidden != wantHidden {
		t.Fatalf("opts.Hidden = %v, want %v", opts.Hidden, wantHidden)
	}
}

func TestTrayPanelWindowOptionsNonActivating(t *testing.T) {
	opts := trayPanelWindowOptions(mygo.Point{X: 100, Y: 40}, 300, 320, mygo.VibrancyPopover, nil)
	if !opts.Panel || !opts.Frameless || !opts.Transparent || !opts.AlwaysOnTop || !opts.SkipTaskbar {
		t.Fatalf("unexpected window panel options: %+v", opts)
	}
	if opts.Vibrancy != mygo.VibrancyPopover || opts.CornerRadius != 14 {
		t.Fatalf("unexpected vibrancy/radius: Vibrancy=%q CornerRadius=%d", opts.Vibrancy, opts.CornerRadius)
	}
	if opts.Width != 300 || opts.Height != 320 || opts.X != 100 || opts.Y != 40 {
		t.Fatalf("unexpected bounds: %dx%d at (%d,%d)", opts.Width, opts.Height, opts.X, opts.Y)
	}

	solidOpts := trayPanelWindowOptions(mygo.Point{X: 10, Y: 20}, 300, 320, mygo.VibrancyNone, nil)
	if solidOpts.Transparent {
		t.Fatalf("solid panel Transparent = true, want false when Vibrancy is VibrancyNone")
	}
}
