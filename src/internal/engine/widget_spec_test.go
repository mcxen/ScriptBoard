package engine

import (
	"testing"

	"scriptboard/src/internal/iconcat"
)

func TestWidgetSpecsCompleteAndValid(t *testing.T) {
	if len(WidgetSpecs) < 19 {
		t.Fatalf("expected at least 19 widget specs, got %d", len(WidgetSpecs))
	}
	seen := map[string]bool{}
	for _, s := range WidgetSpecs {
		if s.Type == "" || s.ZhName == "" || s.EnName == "" {
			t.Errorf("incomplete widget spec: %+v", s)
		}
		if seen[s.Type] {
			t.Errorf("duplicate widget spec type %q", s.Type)
		}
		seen[s.Type] = true
		if !iconcat.Has(s.Icon) {
			t.Errorf("widget spec %q references unknown icon %q", s.Type, s.Icon)
		}
		if UnknownChartType(s.Type) {
			t.Errorf("ChartTypes missing %q", s.Type)
		}
		for _, sw := range s.Swappable {
			if _, ok := WidgetSpecOf(sw); !ok {
				t.Errorf("widget spec %q references unknown swappable type %q", s.Type, sw)
			}
		}
	}
	if !UnknownChartType("nonexistent-widget") {
		t.Error("UnknownChartType should return true for unknown types")
	}
	if UnknownChartType("") {
		t.Error("UnknownChartType should allow empty type (default line)")
	}
}

func TestExtractTableAcrossWidgetTypes(t *testing.T) {
	cases := []struct {
		name     string
		res      Result
		wantCols int
		wantRows int
	}{
		{
			name: "line series",
			res: Result{Chart: &Chart{
				Type:   "line",
				Labels: []string{"Mon", "Tue", "Wed"},
				Series: []Series{{Name: "Price", Points: []float64{10, 11.5, 12}}},
			}},
			wantCols: 2, wantRows: 3,
		},
		{
			name: "candle",
			res: Result{Chart: &Chart{
				Type:    "candle",
				Labels:  []string{"10:00", "11:00"},
				Candles: []Candle{{O: 100, H: 105, L: 99, C: 103}, {O: 103, H: 104, L: 100, C: 101}},
			}},
			wantCols: 6, wantRows: 2,
		},
		{
			name: "standalone progressBar",
			res: Result{ProgressBar: &ProgressBar{
				Items: []Progress{{Name: "OKR", Value: 75, Max: 100, Unit: "%"}},
			}},
			wantCols: 4, wantRows: 1,
		},
		{
			name: "donut",
			res: Result{Chart: &Chart{
				Type:   "donut",
				Slices: []Slice{{Name: "Food", Value: 200}, {Name: "Rent", Value: 800}},
			}},
			wantCols: 4, wantRows: 2,
		},
		{
			name: "kv",
			res: Result{Chart: &Chart{
				Type: "kv",
				KV:   []KVItem{{Label: "API", Value: "24ms", Status: "ok", Badge: "OK"}},
			}},
			wantCols: 5, wantRows: 1,
		},
		{
			name: "checklist",
			res: Result{Chart: &Chart{
				Type:      "checklist",
				Checklist: []CheckItem{{Label: "Ship", Done: true, Badge: "P0"}},
			}},
			wantCols: 4, wantRows: 1,
		},
		{
			name: "contrib",
			res: Result{Chart: &Chart{
				Type:    "contrib",
				Contrib: &ContribSpec{Unit: "kg", Cells: []ContribCell{{Date: "2026-11-20", Value: 65.2}}},
			}},
			wantCols: 3, wantRows: 1,
		},
		{
			name: "native table",
			res: Result{Chart: &Chart{
				Type: "table",
				Table: &TableSpec{
					Columns: []TableColumn{{Title: "Coin"}, {Title: "Price", Align: "right"}},
					Rows:    [][]string{{"BTC", "92000"}, {"ETH", "3400"}},
				},
			}},
			wantCols: 2, wantRows: 2,
		},
		{
			name: "gauge",
			res: Result{Chart: &Chart{
				Type:  "gauge",
				Gauge: &Gauge{Label: "Monthly", Used: 40, Limit: 100, Unit: "USD"},
			}},
			wantCols: 6, wantRows: 1,
		},
		{
			name:     "stats fallback",
			res:      Result{Stats: []Stat{{Label: "Wind", Value: "3", Sub: "NE"}}},
			wantCols: 3, wantRows: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tb := ExtractTable(tc.res, "en")
			if tb == nil {
				t.Fatalf("ExtractTable returned nil")
			}
			if len(tb.Columns) != tc.wantCols || len(tb.Rows) != tc.wantRows {
				t.Errorf("got %d cols x %d rows, want %d x %d", len(tb.Columns), len(tb.Rows), tc.wantCols, tc.wantRows)
			}
		})
	}
}

func TestSwappableTypes(t *testing.T) {
	pb := Result{ProgressBar: &ProgressBar{Items: []Progress{{Name: "A", Value: 50, Max: 100}}}}
	sw := SwappableTypes(pb)
	if len(sw) < 3 {
		t.Fatalf("expected progressBar to swap with progressRing/barh/table, got %v", sw)
	}
	donut := Result{Chart: &Chart{Type: "donut", Slices: []Slice{{Name: "A", Value: 10}, {Name: "B", Value: 20}}}}
	sw = SwappableTypes(donut)
	if len(sw) < 3 {
		t.Fatalf("expected donut to swap with barh/kv/table, got %v", sw)
	}
}
