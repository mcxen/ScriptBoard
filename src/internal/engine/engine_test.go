package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func sampleMetrics(now time.Time) map[string][]Point {
	var w, g []Point
	for i := 0; i < 10; i++ {
		t := now.AddDate(0, 0, -(9 - i)).UnixMilli()
		w = append(w, Point{T: t, V: 66.2 - 0.1*float64(i)})
		g = append(g, Point{T: t, V: 600 + float64(i)*2.5})
	}
	return map[string][]Point{"weight": w, "gold": g}
}

func TestBuiltinsCompileAndRunOffline(t *testing.T) {
	now := time.Date(2026, 11, 18, 9, 0, 0, 0, time.Local)
	for _, b := range Builtins() {
		s, err := Compile(b.ID+".js", b.Source)
		if err != nil {
			t.Fatalf("%s: compile: %v", b.ID, err)
		}
		if s.Meta.Name == "" {
			t.Errorf("%s: empty meta name", b.ID)
		}
		res, err := s.Run(context.Background(), Input{
			Now:      now,
			Metrics:  sampleMetrics(now),
			Settings: map[string]any{"weightUnit": "kg"},
			Sources:  Offline(s.Meta.Sources),
		})
		if err != nil {
			t.Fatalf("%s: run: %v", b.ID, err)
		}
		if res.Title == "" || res.Big == "" {
			t.Errorf("%s: empty title or big: %+v", b.ID, res)
		}
	}
}

func TestWeightUsesUnitSetting(t *testing.T) {
	now := time.Now()
	s, err := Compile("weight.js", Builtins()[0].Source)
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Run(context.Background(), Input{
		Now:      now,
		Metrics:  sampleMetrics(now),
		Settings: map[string]any{"weightUnit": "斤"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(res.Big, " 斤") {
		t.Errorf("big %q, want the 斤 unit", res.Big)
	}
	if res.Chart == nil || len(res.Chart.Series) != 1 || len(res.Chart.Series[0].Points) != 10 {
		t.Fatalf("chart %+v", res.Chart)
	}
	// The last sample is 66.2 - 0.1*9 = 65.3 kg, shown in 斤 (×2).
	if got := res.Chart.Series[0].Points[9]; got < 130.5 || got > 130.7 {
		t.Errorf("last point %v, want 2× 65.3", got)
	}
}

func TestLogsComeBack(t *testing.T) {
	s, err := Compile("log.js", `export function meta(){return {name:"x",sources:{}}}
export function render(ctx){ log("hello", 1); return {title:"t", big:"b", sub:"s", lines:[], chart:null}; }`)
	if err != nil {
		t.Fatal(err)
	}
	res, err := s.Run(context.Background(), Input{Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Logs) != 1 || res.Logs[0] != "hello 1" {
		t.Errorf("logs %q", res.Logs)
	}
}

func TestRunawayScriptIsStopped(t *testing.T) {
	s, err := Compile("spin.js", `export function meta(){return {name:"spin",sources:{}}}
export function render(ctx){ for(;;){} }`)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err = s.Run(context.Background(), Input{Now: time.Now()})
	if err == nil {
		t.Fatal("expected an error from a script that never returns")
	}
	if d := time.Since(start); d > RunTimeout+2*time.Second {
		t.Errorf("stopped after %v, want about %v", d, RunTimeout)
	}
}

func TestFetchFailureBecomesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	out := Fetch(context.Background(), srv.Client(), map[string]string{"x": srv.URL + "/{start}"}, time.Now())
	if !strings.Contains(string(out["x"]), `"error"`) {
		t.Errorf("got %s, want an error object", out["x"])
	}
}

func TestFetchRunsSourcesTogether(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(250 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	start := time.Now()
	out := Fetch(context.Background(), srv.Client(), map[string]string{
		"a": srv.URL + "/a",
		"b": srv.URL + "/b",
		"c": srv.URL + "/c",
	}, time.Now())
	if d := time.Since(start); d >= 600*time.Millisecond {
		t.Errorf("took %v, want the three sources fetched together", d)
	}
	for _, k := range []string{"a", "b", "c"} {
		if !strings.Contains(string(out[k]), `"ok"`) {
			t.Errorf("%s: %s", k, out[k])
		}
	}
}

func TestExpandDates(t *testing.T) {
	now := time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)
	got := Expand("a/{start}..{end}", now)
	if got != "a/2026-10-19..2026-11-18" {
		t.Errorf("got %q", got)
	}
}
