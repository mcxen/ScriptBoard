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

// The Claude Code script shows what is left of a budget as a ring, the models
// as horizontal bars, and the days as a curve; cards of different sizes share
// the home page.
func TestAIUsageCards(t *testing.T) {
	a := newTestApp(t)
	now := time.Now()
	a.usageDir = fixtureLogs(t, now)

	sc, _ := a.store.Script("ai-claude")
	// No budget: bars by model, with a hint.
	res, err := a.execute(sc, map[string]float64{"view": 0, "window": 0})
	if err != nil || res.Chart == nil || res.Chart.Type != "barh" {
		t.Fatalf("without a budget: %+v %v", res.Chart, err)
	}
	if len(res.Chart.Labels) != 3 || res.Chart.Labels[0] != "sonnet-5-5" {
		t.Errorf("models %q, want the busiest first", res.Chart.Labels)
	}

	// A budget turns it into a gauge.
	st := a.store.Settings()
	st.Usage = map[string]store.UsageConfig{"claude-code": {Budget: 100e6, BlockBudget: 5e6}}
	if err := a.store.SetSettings(st); err != nil {
		t.Fatal(err)
	}
	a.set = st
	a.forgetUsage()
	res, err = a.execute(sc, map[string]float64{"view": 0, "window": 0})
	if err != nil || res.Chart.Type != "gauge" || res.Chart.Gauge.Limit != 100e6 || res.Chart.Gauge.Used <= 0 {
		t.Fatalf("with a budget: %+v %v", res.Chart, err)
	}
	if res, _ = a.execute(sc, map[string]float64{"view": 2, "window": 0}); res.Chart.Type != "line" || len(res.Chart.Series[0].Points) != 14 {
		t.Errorf("daily view: %+v", res.Chart)
	}

	// Mixed sizes on the home page.
	a.set.CardSizes = map[string]int{"ai-claude": 2, "weight": 0, "fx": 0}
	if err := a.store.SetSettings(a.set); err != nil {
		t.Fatal(err)
	}
	a.invalidateAll()
	tt := ui.NewTester(a.view, 1280, 1000)
	settle(tt)
	settle(tt)
	requireText(t, tt, "Claude Code · 本月")
	savePNG(t, tt, "13-home-mixed-sizes")

	a.router.Push("/full/ai-claude")
	settle(tt)
	requireText(t, tt, "剩余")
	savePNG(t, tt, "13-full-gauge")
	click(t, tt, "按模型")
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
