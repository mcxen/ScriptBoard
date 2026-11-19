package engine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

const fxcnyFeed = `{
  "amount": 1.0,
  "base": "CNY",
  "date": "2026-11-18",
  "rates": {"USD": 0.15, "EUR": 0.125, "JPY": 20, "GBP": 0.1, "HKD": 1.25}
}`

func TestFxcnyDrawsBars(t *testing.T) {
	res, err := builtin(t, "fxcny").Run(context.Background(), Input{
		Now:     time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC),
		Sources: map[string]json.RawMessage{"fx": json.RawMessage(fxcnyFeed)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Big != "6.6667" {
		t.Errorf("big %q, want 1/0.15 as 6.6667", res.Big)
	}
	if res.Sub != "美元" {
		t.Errorf("sub %q, want 美元", res.Sub)
	}
	if res.Chart == nil || res.Chart.Type != "barh" {
		t.Fatalf("chart %+v, want barh", res.Chart)
	}
	pts := res.Chart.Series[0].Points
	if len(pts) != 5 {
		t.Fatalf("%d bars, want 5", len(pts))
	}
	want := []float64{1 / 0.15, 1 / 0.125, 100 / 20.0, 1 / 0.1, 1 / 1.25}
	for i, v := range want {
		if pts[i] != v {
			t.Errorf("bar %d = %v, want %v", i, pts[i], v)
		}
	}
	got := strings.Join(res.Chart.Labels, ",")
	if got != "美元,欧元,日元×100,英镑,港币" {
		t.Errorf("labels %q", got)
	}
	joined := strings.Join(res.Lines, " ")
	if !strings.Contains(joined, "2026-11-18") {
		t.Errorf("lines %q, want the date", res.Lines)
	}
}

func TestFxcnySkipsMissing(t *testing.T) {
	res, err := builtin(t, "fxcny").Run(context.Background(), Input{
		Now:     time.Now(),
		Sources: map[string]json.RawMessage{"fx": json.RawMessage(`{"date":"2026-11-18","rates":{"EUR":0.125,"GBP":0.1}}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Sub != "欧元" {
		t.Errorf("sub %q, want 欧元 as the first remaining pair", res.Sub)
	}
	if res.Chart == nil || len(res.Chart.Series[0].Points) != 2 {
		t.Fatalf("chart %+v, want 2 bars", res.Chart)
	}
}

func TestFxcnyOfflineHasNoSample(t *testing.T) {
	s := builtin(t, "fxcny")
	if !strings.Contains(s.Meta.Sources["fx"].URL, "api.frankfurter.dev/v1/latest") {
		t.Errorf("source %q, want frankfurter latest", s.Meta.Sources["fx"].URL)
	}
	res, err := s.Run(context.Background(), Input{
		Now:     time.Now(),
		Sources: Offline(s.Meta.Sources),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Big != "--" {
		t.Errorf("big %q, want --", res.Big)
	}
	if res.Chart != nil {
		t.Errorf("offline fxcny still drew a chart")
	}
	if strings.Contains(strings.Join(res.Lines, " "), "示例") {
		t.Errorf("offline fxcny used sample copy: %q", res.Lines)
	}
}

func TestFxcnyEnglishMeta(t *testing.T) {
	var src string
	for _, b := range Builtins() {
		if b.ID == "fxcny" {
			src = b.Source
		}
	}
	s, err := CompileLang("fxcny.js", src, "en")
	if err != nil {
		t.Fatal(err)
	}
	if s.Meta.Name != "vs CNY" {
		t.Errorf("name %q, want vs CNY", s.Meta.Name)
	}
	res, err := s.Run(context.Background(), Input{
		Now:     time.Now(),
		Lang:    "en",
		Sources: map[string]json.RawMessage{"fx": json.RawMessage(fxcnyFeed)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Title != "vs CNY · today" {
		t.Errorf("title %q", res.Title)
	}
	if res.Sub != "USD" {
		t.Errorf("sub %q", res.Sub)
	}
	got := strings.Join(res.Chart.Labels, ",")
	if got != "USD,EUR,JPY×100,GBP,HKD" {
		t.Errorf("labels %q", got)
	}
}
