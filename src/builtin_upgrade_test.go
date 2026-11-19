package main

import (
	"os"
	"strings"
	"testing"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

// variableBuiltins are the built-ins that switch a variable (city, currency...).
var variableBuiltins = map[string]bool{"fx": true, "weather": true, "aqi": true, "btc": true, "hs300": true}

// The built-in scripts that took switchable variables: an unedited copy of the
// version before takes the new script on start, and an edited copy is kept.
func TestUnchangedBuiltinsUpgradeToVariables(t *testing.T) {
	for _, id := range []string{"fx", "weather", "aqi", "btc", "hs300", "ai-claude", "ai-deepseek", "ai-openrouter", "gold", "goldacc", "goldk", "weight", "fxcny"} {
		old, err := os.ReadFile("testdata/legacy/" + id + ".js")
		if err != nil {
			t.Fatal(err)
		}
		a := newTestApp(t)
		if err := a.store.SaveScript(store.Script{ID: id, Name: "old", Source: string(old), Home: true, Origin: store.OriginBuiltin}); err != nil {
			t.Fatal(err)
		}
		if err := a.ensureBuiltins(); err != nil {
			t.Fatal(err)
		}
		got, _ := a.store.Script(id)
		want := ""
		for _, b := range engine.Builtins() {
			if b.ID == id {
				want = b.Source
			}
		}
		if got.Source != want {
			t.Errorf("%s: unedited copy not upgraded", id)
		}
		m, err := a.metaOf(got)
		if err != nil {
			t.Errorf("%s: upgraded script does not compile: %v", id, err)
		}
		if variableBuiltins[id] && len(m.Vars) == 0 {
			t.Errorf("%s: upgraded script has no vars", id)
		}

		// An edited copy is kept.
		edited := string(old) + "\n// my change\n"
		if err := a.store.SaveScript(store.Script{ID: id, Name: "mine", Source: edited, Home: true, Origin: store.OriginBuiltin}); err != nil {
			t.Fatal(err)
		}
		if err := a.ensureBuiltins(); err != nil {
			t.Fatal(err)
		}
		if kept, _ := a.store.Script(id); kept.Source != edited {
			t.Errorf("%s: an edited copy was overwritten", id)
		}
	}
}

// A copy from before the chart colors changed (0.1.2) is recognised too.
func TestPastelColorCopiesAreRecognised(t *testing.T) {
	for _, id := range []string{"weather", "fx"} {
		old, err := os.ReadFile("testdata/legacy/" + id + ".js")
		if err != nil {
			t.Fatal(err)
		}
		if !refreshBuiltin(id, legacyColors(string(old))) {
			t.Errorf("%s: the pastel copy of its previous version is not recognised", id)
		}
	}
}

// The fingerprints are of the files the previous versions shipped as.
func TestPreviousBuiltinFingerprintsMatchTheirFiles(t *testing.T) {
	for id, fps := range previousBuiltins {
		old, err := os.ReadFile("testdata/legacy/" + id + ".js")
		if err != nil {
			t.Fatal(err)
		}
		if !refreshBuiltin(id, string(old)) {
			t.Errorf("%s: its previous version is not recognised", id)
		}
		if len(fps) == 0 {
			t.Errorf("%s: no fingerprints", id)
		}
	}
	if strings.TrimSpace(string(mustRead(t, "testdata/legacy/weather.js"))) == "" {
		t.Error("the fixture is empty")
	}
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
