package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
)

func TestComponentRegistryCoversAllEngineSpecs(t *testing.T) {
	for _, spec := range engine.WidgetSpecs {
		v, ok := componentOf(spec.Type)
		if !ok {
			t.Errorf("no UI ComponentView registered for %q", spec.Type)
			continue
		}
		if v.Type() != spec.Type {
			t.Errorf("component type mismatch: %q vs %q", v.Type(), spec.Type)
		}
	}
}

func TestComponentRegistryCanSwapAndReplaceRenderer(t *testing.T) {
	orig, ok := componentOf("kv")
	if !ok {
		t.Fatal("kv component missing")
	}
	defer registerComponent(orig)

	called := false
	registerComponent(componentAdapter{
		typ: "kv",
		fullFn: func(c *ui.Context, req ComponentRenderReq) ui.Element {
			called = true
			return ui.Text(c, "custom-kv-renderer")
		},
	})
	a := newTestApp(t)
	res := engine.Result{Chart: &engine.Chart{Type: "kv", KV: []engine.KVItem{{Label: "k", Value: "v"}}}}
	tt := ui.NewTester(func(c *ui.Context) {
		v, adapted, sub := resolveComponent(res, "")
		v.RenderFull(c, ComponentRenderReq{App: a, Result: adapted, Style: sub, Full: true})
	}, 400, 200)
	settle(tt)
	if !called || !tt.HasText("custom-kv-renderer") {
		t.Fatalf("custom renderer not invoked: called=%v texts=%v", called, tt.Texts())
	}
}

func TestAdaptResultForSwappableTypes(t *testing.T) {
	pbRes := engine.Result{
		Title: "Goals",
		ProgressBar: &engine.ProgressBar{
			Items: []engine.Progress{
				{Name: "OKR", Value: 75, Max: 100, Unit: "%"},
				{Name: "Read", Value: 12, Max: 24, Unit: " books"},
			},
		},
	}
	if adapted, ok := adaptResultForType(pbRes, "progressRing"); !ok || adapted.Chart == nil || adapted.Chart.Type != "progressRing" {
		t.Fatalf("progressBar -> progressRing failed: %+v", adapted)
	}
	if adapted, ok := adaptResultForType(pbRes, "barh"); !ok || adapted.Chart == nil || adapted.Chart.Type != "barh" {
		t.Fatalf("progressBar -> barh failed: %+v", adapted)
	}
	if adapted, ok := adaptResultForType(pbRes, "table"); !ok || adapted.Chart == nil || adapted.Chart.Table == nil {
		t.Fatalf("progressBar -> table failed: %+v", adapted)
	}

	donutRes := engine.Result{
		Title: "Spend",
		Chart: &engine.Chart{
			Type:   "donut",
			Slices: []engine.Slice{{Name: "Food", Value: 300}, {Name: "Rent", Value: 700}},
		},
	}
	if adapted, ok := adaptResultForType(donutRes, "kv"); !ok || adapted.Chart == nil || len(adapted.Chart.KV) != 2 {
		t.Fatalf("donut -> kv failed: %+v", adapted)
	}
	if adapted, ok := adaptResultForType(donutRes, "barh"); !ok || adapted.Chart == nil || adapted.Chart.Type != "barh" {
		t.Fatalf("donut -> barh failed: %+v", adapted)
	}
}

func TestTableWidgetSortFilterAndCopy(t *testing.T) {
	a := newTestApp(t)
	var copied string
	oldCopy := copyText
	copyText = func(s string) { copied = s }
	defer func() { copyText = oldCopy }()

	spec := &engine.TableSpec{
		Searchable: true,
		Striped:    true,
		Columns: []engine.TableColumn{
			{Title: "Asset"},
			{Title: "Price", Align: "right", Format: "number"},
			{Title: "Change", Align: "right", Format: "delta"},
			{Title: "Status", Format: "status"},
		},
		Rows: [][]string{
			{"BTC", "92000", "+2.50%", "ok"},
			{"ETH", "3400", "-1.20%", "warn"},
			{"SOL", "210", "+5.10%", "ok"},
		},
	}
	tt := ui.NewTester(func(c *ui.Context) {
		applySkin(c)
		ui.Column(c).Fill().Padding(12).Children(func() {
			a.tableWidgetView(c, "test-tb", spec, true, 0)
		})
	}, 700, 420)
	settle(tt)
	requireText(t, tt, "BTC")
	requireText(t, tt, "ETH")
	requireText(t, tt, "SOL")

	// Sort ascending by Price (210 < 3400 < 92000).
	click(t, tt, "Price")
	if a.tableViews["test-tb"].sortCol != 1 || a.tableViews["test-tb"].desc {
		t.Fatalf("sort state after 1st click: %+v", a.tableViews["test-tb"])
	}
	// Copy CSV.
	click(t, tt, "复制 CSV")
	if copied == "" || copied[:5] != "Asset" {
		t.Fatalf("unexpected CSV copy: %q", copied)
	}
	savePNG(t, tt, "26-table-widget")
}

// Card styles and runtime component swaps persist in store.Settings.CardStyles
// across app restarts.
func TestCardStylesPersistAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	a1, err := newApp(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	a1.setCardStyle("weight", "grid")
	a1.setCardStyle("ai-claude-neon", "view:progressRing")

	a2, err := newApp(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if a2.styles["weight"] != "grid" {
		t.Fatalf("weight style after restart = %q, want grid", a2.styles["weight"])
	}
	if a2.styles["ai-claude-neon"] != "view:progressRing" {
		t.Fatalf("ai-claude-neon style after restart = %q, want view:progressRing", a2.styles["ai-claude-neon"])
	}
	a2.setCardStyle("weight", "")
	a3, err := newApp(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if a3.styles["weight"] != "" {
		t.Fatalf("cleared style persisted as %q", a3.styles["weight"])
	}
}
