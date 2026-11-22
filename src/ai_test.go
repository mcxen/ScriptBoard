package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/store"
)

// fixtureLogs writes made-up Claude Code logs, so that no test or screenshot
// reads the logs of whoever runs it.
func fixtureLogs(t *testing.T, now time.Time) string {
	t.Helper()
	dir := t.TempDir()
	var lines []string
	models := []string{"claude-opus-5-5", "claude-sonnet-5-5", "claude-haiku-5-5"}
	weights := []int{60, 130, 25}
	id := 0
	for d := 12; d >= 0; d-- {
		for k, m := range models {
			id++
			at := now.AddDate(0, 0, -d).Add(-time.Duration(k) * time.Hour)
			lines = append(lines, fmt.Sprintf(`{"timestamp":%q,"requestId":"r%d","message":{"id":"m%d","model":%q,"usage":{"input_tokens":100,"output_tokens":%d,"cache_creation_input_tokens":5000,"cache_read_input_tokens":%d}}}`,
				at.UTC().Format(time.RFC3339Nano), id, id, m, 3000*weights[k]/10, 40000*weights[k]/10))
		}
	}
	p := filepath.Join(dir, "proj", "s.jsonl")
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// The Claude Code board shows quotas as neon progress bars, models as a donut,
// and daily activity as a contrib heatmap; cards of different sizes share the home page.
func TestAIUsageCards(t *testing.T) {
	a := newTestApp(t)
	now := time.Now()
	a.usageDir = fixtureLogs(t, now)

	sc, ok := a.store.Script("ai-claude-neon")
	if !ok {
		t.Fatal("missing ai-claude-neon builtin")
	}
	sc.Home = true
	if err := a.store.SaveScript(sc); err != nil {
		t.Fatal(err)
	}

	// View 1: donut by model.
	res, err := a.execute(sc, map[string]float64{"view": 1})
	if err != nil || res.Chart == nil || res.Chart.Type != "donut" || len(res.Chart.Slices) != 3 {
		t.Fatalf("model donut: %+v %v", res.Chart, err)
	}
	if res.Chart.Slices[0].Name != "sonnet-5-5" {
		t.Errorf("models %+v, want the busiest first", res.Chart.Slices)
	}

	// A budget turns view 0 into neon progress bars.
	st := a.store.Settings()
	st.Usage = map[string]store.UsageConfig{"claude-code": {Budget: 100e6, BlockBudget: 5e6}}
	if err := a.store.SetSettings(st); err != nil {
		t.Fatal(err)
	}
	a.set = st
	a.forgetUsage()
	res, err = a.execute(sc, map[string]float64{"view": 0})
	if err != nil || res.ProgressBar == nil || len(res.ProgressBar.Items) == 0 {
		t.Fatalf("with a budget: %+v %v", res.ProgressBar, err)
	}
	if res, _ = a.execute(sc, map[string]float64{"view": 2}); res.Chart == nil || res.Chart.Type != "contrib" {
		t.Errorf("activity view: %+v", res.Chart)
	}

	// Mixed sizes on the home page.
	a.set.CardSizes = map[string]int{"ai-claude-neon": 2, "weight": 0, "fx": 0}
	if err := a.store.SetSettings(a.set); err != nil {
		t.Fatal(err)
	}
	a.invalidateAll()
	tt := ui.NewTester(a.view, 1280, 1000)
	settle(tt)
	settle(tt)
	requireText(t, tt, "Claude Code 看板")
	savePNG(t, tt, "13-home-mixed-sizes")

	a.router.Push("/full/ai-claude-neon")
	settle(tt)
	requireText(t, tt, "霓虹额度")
	savePNG(t, tt, "13-full-gauge")
	click(t, tt, "模型占比")
	savePNG(t, tt, "13-full-barh")
}

// A card's size can be changed with the button in its header, and the choice
// is kept.
func TestCardSizeButton(t *testing.T) {
	a := newTestApp(t)
	if got := a.sizeOf("weight"); got != 1 {
		t.Fatalf("default size %d", got)
	}
	a.setSize("weight", 2)
	if got := a.boxOf("weight").Span; got != gridSpans[2] {
		t.Errorf("span %d after setSize(2)", got)
	}
	if got := a.store.Settings().CardBoxes["weight"].Span; got != gridSpans[2] {
		t.Errorf("the size was not saved: span %d", got)
	}
	a.setAllSizes(0)
	if got := a.boxOf("weight").Span; got != gridSpans[0] {
		t.Errorf("size %d after setAllSizes(0), want the override gone", got)
	}
}
