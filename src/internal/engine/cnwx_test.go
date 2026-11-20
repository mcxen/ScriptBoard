package engine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

const cnwxNow = `{
  "code": 200,
  "message": "ok",
  "data": {
    "location": {"name": "上海上海", "city": "上海市"},
    "weather": {
      "condition": "晴",
      "condition_code": "00",
      "temperature": 10,
      "humidity": 29,
      "pressure": 1027,
      "precipitation": 0,
      "wind_direction": "南风",
      "wind_power": "3-4",
      "updated": "2026-11-19 10:40:00"
    },
    "air_quality": {"aqi": 22, "level": 1, "quality": "优", "pm25": 12, "pm10": 22},
    "sunrise": {"sunrise_desc": "06:24", "sunset_desc": "16:54"}
  }
}`

const cnwxForecast = `{
  "code": 200,
  "data": {
    "hourly_forecast": [
      {"datetime": "2026-11-19 10:00", "temperature": 10, "condition": "多云", "condition_code": "01"},
      {"datetime": "2026-11-19 11:00", "temperature": 12, "condition": "晴", "condition_code": "00"}
    ]
  }
}`

func cnwxSources() map[string]json.RawMessage {
	return map[string]json.RawMessage{
		"now": json.RawMessage(cnwxNow),
		"fc":  json.RawMessage(cnwxForecast),
	}
}

func TestCnwxShowsNowcast(t *testing.T) {
	res, err := builtin(t, "cnwx").Run(context.Background(), Input{
		Now:     time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC),
		Sources: cnwxSources(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Big != "10°" {
		t.Errorf("big %q, want 10°", res.Big)
	}
	if res.Title != "国内实况 · 上海" {
		t.Errorf("title %q", res.Title)
	}
	if res.Icon != "sun" {
		t.Errorf("icon %q, want sun", res.Icon)
	}
	if !strings.Contains(res.Sub, "晴") || !strings.Contains(res.Sub, "南风") || !strings.Contains(res.Sub, "3-4") {
		t.Errorf("sub %q, want condition and wind", res.Sub)
	}
	if len(res.Stats) != 4 {
		t.Fatalf("stats %+v, want 4", res.Stats)
	}
	if res.Stats[0].Value != "29%" || res.Stats[1].Value != "0" || res.Stats[1].Sub != "mm" {
		t.Errorf("humidity/precip %+v", res.Stats[:2])
	}
	if res.Stats[2].Value != "22" || res.Stats[2].Sub != "优" {
		t.Errorf("aqi %+v", res.Stats[2])
	}
	if res.Stats[3].Value != "1027" {
		t.Errorf("pressure %+v", res.Stats[3])
	}
	joined := strings.Join(res.Lines, " ")
	if !strings.Contains(joined, "06:24") || !strings.Contains(joined, "16:54") {
		t.Errorf("lines %q, want sunrise/sunset", res.Lines)
	}
	if !strings.Contains(joined, "2026-11-19 10:40:00") {
		t.Errorf("lines %q, want updated time", res.Lines)
	}
	if res.Chart == nil || res.Chart.Type != "line" {
		t.Fatalf("chart %+v, want line", res.Chart)
	}
	if n := len(res.Chart.Series[0].Points); n != 2 {
		t.Errorf("%d hourly points, want 2", n)
	}
	if res.Chart.Labels[0] != "11-19 10:00" {
		t.Errorf("first label %q", res.Chart.Labels[0])
	}
}

func TestCnwxAllowsZeroTemp(t *testing.T) {
	src := `{
	  "code": 200,
	  "data": {"weather": {"condition_code": "00", "temperature": 0, "wind_direction": "北风", "wind_power": "1"}}
	}`
	res, err := builtin(t, "cnwx").Run(context.Background(), Input{
		Now:     time.Now(),
		Sources: map[string]json.RawMessage{"now": json.RawMessage(src)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Big != "0°" {
		t.Errorf("big %q, want 0°", res.Big)
	}
	if res.Chart != nil {
		t.Errorf("drew a chart without a forecast")
	}
}

func TestCnwxSkipsBadCode(t *testing.T) {
	res, err := builtin(t, "cnwx").Run(context.Background(), Input{
		Now:     time.Now(),
		Sources: map[string]json.RawMessage{"now": json.RawMessage(`{"code":500,"message":"busy"}`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Big != "--" || res.Chart != nil || len(res.Stats) != 0 {
		t.Errorf("bad code still drew: %+v", res)
	}
}

func TestCnwxForecastOptional(t *testing.T) {
	res, err := builtin(t, "cnwx").Run(context.Background(), Input{
		Now: time.Now(),
		Sources: map[string]json.RawMessage{
			"now": json.RawMessage(cnwxNow),
			"fc":  json.RawMessage(`{"error":"timeout"}`),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Big != "10°" {
		t.Errorf("big %q, want the nowcast even without a forecast", res.Big)
	}
	if res.Chart != nil {
		t.Errorf("drew a chart from a failed forecast")
	}
}

func TestCnwxOfflineHasNoSample(t *testing.T) {
	s := builtin(t, "cnwx")
	if !strings.Contains(s.Meta.Sources["now"].URL, "60s.viki.moe/v2/weather") {
		t.Errorf("now source %q", s.Meta.Sources["now"].URL)
	}
	if !strings.Contains(s.Meta.Sources["fc"].URL, "60s.viki.moe/v2/weather/forecast") {
		t.Errorf("fc source %q", s.Meta.Sources["fc"].URL)
	}
	if !strings.Contains(s.Meta.Sources["now"].URL, "{city}") {
		t.Errorf("now source has no {city}: %q", s.Meta.Sources["now"].URL)
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
		t.Errorf("offline cnwx still drew a chart")
	}
	if len(res.Stats) != 0 {
		t.Errorf("offline cnwx still drew stats: %+v", res.Stats)
	}
	if strings.Contains(strings.Join(res.Lines, " "), "示例") {
		t.Errorf("offline cnwx used sample copy: %q", res.Lines)
	}
}

func TestCnwxEnglishMeta(t *testing.T) {
	var src string
	for _, b := range Builtins() {
		if b.ID == "cnwx" {
			src = b.Source
		}
	}
	s, err := CompileLang("cnwx.js", src, "en")
	if err != nil {
		t.Fatal(err)
	}
	if s.Meta.Name != "China now" {
		t.Errorf("name %q, want China now", s.Meta.Name)
	}
	res, err := s.Run(context.Background(), Input{
		Now:     time.Now(),
		Lang:    "en",
		Sources: cnwxSources(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Title != "China now · Shanghai" {
		t.Errorf("title %q", res.Title)
	}
	if res.Sub != "Clear · S 3-4" {
		t.Errorf("sub %q", res.Sub)
	}
	if len(res.Stats) < 3 || res.Stats[2].Sub != "Good" {
		t.Errorf("aqi stat %+v", res.Stats)
	}
	if res.Chart == nil || res.Chart.Series[0].Name != "Temperature" {
		t.Errorf("chart %+v", res.Chart)
	}
}
