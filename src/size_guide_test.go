package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// The guide draws every size as a valid SVG, dashed unless it is the chosen one.
func TestSizeGuideSVGs(t *testing.T) {
	for i := 0; i < 4; i++ {
		for _, chosen := range []bool{false, true} {
			data := sizeGuideSVG(i, chosen)
			svg, err := ui.ParseSVG(data)
			if err != nil || svg == nil {
				t.Fatalf("size %d chosen %v: %v\n%s", i, chosen, err, data)
			}
		}
	}
	if w, _ := sizeDims(3); w != 2*float64(cardSizes[2][0])*guideScale {
		t.Errorf("wide guide width %v", w)
	}
}

// Recording a value refreshes only the cards that read it: the weight card
// runs again, and the others keep their runs.
func TestRecordRefreshesOnlyItsCards(t *testing.T) {
	a := newTestApp(t)
	a.runs["weight"] = &runState{}
	a.runs["fx"] = &runState{}
	if err := a.recordMetric("weight", 65.2); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.runs["weight"]; ok {
		t.Errorf("the weight card kept its run after a weight was recorded")
	}
	if _, ok := a.runs["fx"]; !ok {
		t.Errorf("an unrelated card lost its run")
	}
}

// The size guide shows for a few seconds after a size is chosen.
func TestSizeGuideShowsAfterAChoice(t *testing.T) {
	a := newTestApp(t)
	a.setSize("weight", 3)
	if a.guideSize != 3 || !a.now().Before(a.guideUntil) {
		t.Errorf("guide not up after choosing wide: size %d until %v", a.guideSize, a.guideUntil)
	}
	if got := a.sizeOf("weight"); got != 3 {
		t.Errorf("wide not kept: %d", got)
	}
}

// The guide shows the sizes, the chosen one solid, right after a choice. The
// card's own right-click menu is the system's native menu, which the Tester
// cannot open, so the choice is made directly here.
func TestSizeGuideOnHome(t *testing.T) {
	a := newTestApp(t)
	a.setSize("weight", 3)
	tt := ui.NewTester(func(c *ui.Context) {
		applySkin(c)
		a.home(c, false)
	}, 1100, 760)
	// The guide lasts four seconds of the app's clock; the capture settles the
	// page again, so the test holds it up for the screenshot.
	a.guideUntil = a.now().Add(time.Hour)
	settle(tt)
	requireText(t, tt, "宽")
	savePNG(t, tt, "24-size-guide")
}

// Right-clicking a card opens its size menu in the window; choosing a size
// changes that card alone, and the others keep theirs.
func TestRightClickChangesOnlyThatCard(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(func(c *ui.Context) {
		applySkin(c)
		a.home(c, false)
	}, 1100, 760)
	settle(tt)
	if err := tt.RightClick("体重 · kg"); err != nil {
		t.Fatalf("right click: %v", err)
	}
	settle(tt)
	click(t, tt, "宽")
	if got := a.sizeOf("weight"); got != 3 {
		t.Fatalf("weight card size %d after choosing wide, want 3", got)
	}
	if got := a.sizeOf("fx"); got == 3 {
		t.Errorf("another card changed size with it")
	}
}
