package engine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

const aqiFeed = `{
  "current": {"time": "2026-11-18T15:00", "interval": 3600, "pm2_5": 41.3, "pm10": 43.3, "us_aqi": 131},
  "hourly": {
    "time": ["2026-11-18T13:00", "2026-11-18T14:00", "2026-11-18T15:00", "2026-11-18T16:00"],
    "pm2_5": [40.1, 40.8, 41.3, 42.0]
  }
}`

func TestAqiUsesCurrentAndHourly(t *testing.T) {
	res, err := builtin(t, "aqi").Run(context.Background(), Input{
		Now:     time.Date(2026, 11, 18, 15, 0, 0, 0, time.UTC),
		Sources: map[string]json.RawMessage{"air": json.RawMessage(aqiFeed)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Big != "41.3" {
		t.Errorf("big %q, want PM2.5 41.3", res.Big)
	}
	if res.Sub != "微克/立方米" {
		t.Errorf("sub %q", res.Sub)
	}
	joined := strings.Join(res.Lines, " ")
	if !strings.Contains(joined, "AQI 131") || !strings.Contains(joined, "轻度污染") {
		t.Errorf("lines %q, want AQI category", res.Lines)
	}
	if !strings.Contains(joined, "PM10 43.3") {
		t.Errorf("lines %q, want PM10", res.Lines)
	}
	if res.Chart == nil || res.Chart.Type != "line" {
		t.Fatalf("chart %+v, want line", res.Chart)
	}
	if n := len(res.Chart.Series[0].Points); n != 3 {
		t.Errorf("%d hourly points, want 3 (drop the forecast hour)", n)
	}
	if got := res.Chart.Labels[2]; got != "11-18 15:00" {
		t.Errorf("last label %q", got)
	}
}

func TestAqiEnglishCategory(t *testing.T) {
	res, err := builtin(t, "aqi").Run(context.Background(), Input{
		Now:     time.Now(),
		Lang:    "en",
		Sources: map[string]json.RawMessage{"air": json.RawMessage(aqiFeed)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Title != "Shanghai air · PM2.5" {
		t.Errorf("title %q", res.Title)
	}
	if res.Sub != "µg/m³" {
		t.Errorf("sub %q", res.Sub)
	}
	if !strings.Contains(strings.Join(res.Lines, " "), "Unhealthy for sensitive") {
		t.Errorf("lines %q", res.Lines)
	}
}

func TestAqiOfflineHasNoSample(t *testing.T) {
	s := builtin(t, "aqi")
	if !strings.Contains(s.Meta.Sources["air"], "air-quality-api.open-meteo.com") {
		t.Errorf("source %q", s.Meta.Sources["air"])
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
		t.Errorf("offline aqi still drew a chart")
	}
	if strings.Contains(strings.Join(res.Lines, " "), "示例") {
		t.Errorf("offline aqi used sample copy: %q", res.Lines)
	}
}
