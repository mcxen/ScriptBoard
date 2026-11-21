package engine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

const tminiGold = `{
  "date": "2026-11-18",
  "metals": [
    {"name": "今日金价", "sell_price": "850.5", "today_price": "848", "unit": "元/克"}
  ],
  "stores": [
    {"brand": "水贝黄金", "price": "1051"},
    {"brand": "周大福", "price": "1156"},
    {"brand": "老凤祥", "price": "1160"}
  ],
  "banks": [
    {"bank": "工商银行", "product": "如意金条", "price": "898.87"},
    {"bank": "中国银行", "product": "投资金条", "price": "906.78"}
  ],
  "recycle": [
    {"type": "黄金回收", "price": "840"},
    {"type": "18k金回收", "price": "630"}
  ]
}`

func TestGoldUsesStorePrices(t *testing.T) {
	res, err := builtin(t, "gold").Run(context.Background(), Input{
		Now:     time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC),
		Sources: map[string]json.RawMessage{"gold": json.RawMessage(tminiGold)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Big != "1156.00" {
		t.Errorf("big %q, want 周大福 1156.00", res.Big)
	}
	if res.Sub != "周大福" {
		t.Errorf("sub %q, want 周大福", res.Sub)
	}
	if res.Chart == nil || res.Chart.Type != "barh" {
		t.Fatalf("chart %+v, want barh", res.Chart)
	}
	if n := len(res.Chart.Series[0].Points); n != 3 {
		t.Errorf("%d store bars, want 3", n)
	}
	joined := strings.Join(res.Lines, " ")
	if !strings.Contains(joined, "850.50") {
		t.Errorf("lines %q, want the spot price", res.Lines)
	}
}

func TestGoldBankBars(t *testing.T) {
	res, err := builtin(t, "gold").Run(context.Background(), Input{
		Now:     time.Now(),
		View:    map[string]float64{"kind": 1},
		Sources: map[string]json.RawMessage{"gold": json.RawMessage(tminiGold)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Sub != "工商银行" {
		t.Errorf("sub %q, want 工商银行", res.Sub)
	}
	if res.Big != "898.87" {
		t.Errorf("big %q", res.Big)
	}
}

func TestGoldOfflineHasNoSample(t *testing.T) {
	s := builtin(t, "gold")
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
		t.Errorf("offline gold still drew a chart")
	}
	if strings.Contains(strings.Join(res.Lines, " "), "示例") {
		t.Errorf("offline gold used sample copy: %q", res.Lines)
	}
}

func TestGoldFallsBackToSGEQuotes(t *testing.T) {
	sge := `{"data":{"diff":[
		{"f12":"AU9999","f14":"黄金9999","f2":763.0,"f18":764.55},
		{"f12":"AUTD","f14":"黄金T+D","f2":761.41,"f18":764.0},
		{"f12":"SHAU","f14":"上海金","f2":"-","f18":765.76}
	]}}`
	res, err := builtin(t, "gold").Run(context.Background(), Input{
		Now:     time.Now(),
		Sources: map[string]json.RawMessage{"gold": json.RawMessage(sge)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Big != "763.00" || res.Sub != "黄金9999" {
		t.Errorf("got big %q sub %q, want 763.00 黄金9999", res.Big, res.Sub)
	}
	if res.Chart == nil || len(res.Chart.Series[0].Points) != 3 {
		t.Fatalf("chart %+v", res.Chart)
	}
}
