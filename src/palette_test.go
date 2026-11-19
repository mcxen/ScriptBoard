package main

import (
	"strings"
	"testing"

	"scriptboard/src/internal/engine"
)

// A built-in script saved with the old pastel colors, and never edited, takes
// the shipped colors on start; one the user changed is left alone.
func TestBuiltinColorsAreRefreshedOnlyWhenUnedited(t *testing.T) {
	a := newTestApp(t)
	var shipped string
	for _, b := range engine.Builtins() {
		if b.ID == "weight" {
			shipped = b.Source
		}
	}
	old := legacyColors(shipped)
	if old == shipped || !strings.Contains(old, "#4FC3F7") {
		t.Fatalf("the weight script should have a color to migrate")
	}
	sc, _ := a.store.Script("weight")
	sc.Source = old
	a.store.SaveScript(sc)
	if err := a.ensureBuiltins(); err != nil {
		t.Fatal(err)
	}
	if got, _ := a.store.Script("weight"); got.Source != shipped {
		t.Error("an unedited old copy was not refreshed")
	}

	edited := old + "\n// mine"
	sc.Source = edited
	a.store.SaveScript(sc)
	a.ensureBuiltins()
	if got, _ := a.store.Script("weight"); got.Source != edited {
		t.Error("an edited copy was overwritten")
	}
}
