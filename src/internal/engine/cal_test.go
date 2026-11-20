package engine

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"
)

var calCJK = regexp.MustCompile(`[\x{4e00}-\x{9fff}]`)

func TestCalRunsOffline(t *testing.T) {
	now := time.Date(2024, 2, 10, 9, 0, 0, 0, time.UTC)
	res, err := builtin(t, "cal").Run(context.Background(), Input{Now: now, Lang: "zh"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Title != "日历" {
		t.Errorf("title %q", res.Title)
	}
	if res.Big != "10" {
		t.Errorf("big %q, want 10", res.Big)
	}
	if !strings.Contains(res.Sub, "春节") {
		t.Errorf("sub %q, want 春节", res.Sub)
	}
	if res.Chart == nil || res.Chart.Type != "calendar" {
		t.Fatalf("chart %+v", res.Chart)
	}
	if len(res.Stats) < 3 {
		t.Fatalf("stats %+v", res.Stats)
	}
	if res.Stats[2].Value != "春节" {
		t.Errorf("holiday stat %q", res.Stats[2].Value)
	}
}

func TestCalEnglishHasNoCJK(t *testing.T) {
	now := time.Date(2024, 2, 10, 9, 0, 0, 0, time.UTC)
	var src string
	for _, b := range Builtins() {
		if b.ID == "cal" {
			src = b.Source
		}
	}
	s, err := CompileLang("cal.js", src, "en")
	if err != nil {
		t.Fatal(err)
	}
	if s.Meta.Name != "Calendar" {
		t.Errorf("name %q", s.Meta.Name)
	}
	res, err := s.Run(context.Background(), Input{Now: now, Lang: "en"})
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range append([]string{res.Title, res.Big, res.Sub}, res.Lines...) {
		if calCJK.MatchString(v) {
			t.Errorf("CJK in English result %q", v)
		}
	}
	for _, st := range res.Stats {
		for _, v := range []string{st.Label, st.Value, st.Sub} {
			if calCJK.MatchString(v) {
				t.Errorf("CJK in English stat %q", v)
			}
		}
	}
	if !strings.Contains(res.Sub, "Spring Festival") {
		t.Errorf("sub %q, want Spring Festival", res.Sub)
	}
}

func TestCalendarIsAKnownChartType(t *testing.T) {
	if UnknownChartType("calendar") {
		t.Error("calendar is reported as unknown")
	}
}
