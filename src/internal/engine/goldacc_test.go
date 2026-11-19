package engine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

const jinICBC = `{"code":200,"msg":"success","data":{"source":"JD API","name":"工商银行","symbol":"ICBC","currency":"¥","price":812.54,"change":-16.87,"change_pct":-2.03,"prev_close":829.41,"update_time":"23:04:44"}}`

const jinMS = `{"code":200,"msg":"success","data":{"name":"民生银行","price":808.78,"change":-24.29,"change_pct":-2.92,"update_time":"23:04:52"}}`

const jinZS = `{"code":200,"msg":"success","data":{"name":"浙商银行","price":809.02,"change":-24,"change_pct":-2.88,"update_time":"23:04:54"}}`

const jinCIB = `{"code":200,"msg":"success","data":{"name":"兴业银行","price":808.79,"change":-24.45,"change_pct":-2.93,"update_time":"23:04:55"}}`

const jinCGB = `{"code":200,"msg":"success","data":{"name":"广发银行","price":808.77,"change":-24.23,"change_pct":-2.91,"update_time":"23:04:56"}}`

const jdGold = `{"resultData":{"datas":{"upAndDownRate":"-2.99%","price":"808.16","yesterdayPrice":"833.07","upAndDownAmt":"-24.91","time":"1795014458000"},"status":"SUCCESS"},"success":true}`

const icbcTicks = `{"code":200,"data":[{"t":1794931200,"p":829.41},{"t":1794974002,"p":815.03},{"t":1795014395,"p":812.54}]}`

func goldaccSources() map[string]json.RawMessage {
	return map[string]json.RawMessage{
		"icbc":  json.RawMessage(jinICBC),
		"ms":    json.RawMessage(jinMS),
		"zs":    json.RawMessage(jinZS),
		"cib":   json.RawMessage(jinCIB),
		"cgb":   json.RawMessage(jinCGB),
		"jd":    json.RawMessage(jdGold),
		"ticks": json.RawMessage(icbcTicks),
	}
}

func TestGoldAccComparesBanks(t *testing.T) {
	res, err := builtin(t, "goldacc").Run(context.Background(), Input{
		Now:     time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC),
		Sources: goldaccSources(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Big != "812.54" {
		t.Errorf("big %q, want ICBC 812.54", res.Big)
	}
	if res.Sub != "工商银行" {
		t.Errorf("sub %q, want 工商银行", res.Sub)
	}
	if res.Chart == nil || res.Chart.Type != "barh" {
		t.Fatalf("chart %+v, want barh", res.Chart)
	}
	if n := len(res.Chart.Series[0].Points); n != 6 {
		t.Errorf("%d bank bars, want 6", n)
	}
	got := strings.Join(res.Chart.Labels, ",")
	if got != "工行,民生,浙商,兴业,广发,京东" {
		t.Errorf("labels %q", got)
	}
	if res.Chart.Series[0].Points[5] != 808.16 {
		t.Errorf("JD bar %v, want 808.16", res.Chart.Series[0].Points[5])
	}
	joined := strings.Join(res.Lines, " ")
	if !strings.Contains(joined, "23:04:44") || !strings.Contains(joined, "-16.87") {
		t.Errorf("lines %q, want the ICBC time and change", res.Lines)
	}
}

func TestGoldAccICBCToday(t *testing.T) {
	res, err := builtin(t, "goldacc").Run(context.Background(), Input{
		Now:      time.Date(2026, 11, 18, 15, 0, 0, 0, time.UTC),
		TZOffset: 8 * 3600,
		View:     map[string]float64{"kind": 1},
		Sources:  goldaccSources(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Big != "812.54" {
		t.Errorf("big %q, want last tick 812.54", res.Big)
	}
	if res.Chart == nil || res.Chart.Type != "line" {
		t.Fatalf("chart %+v, want line", res.Chart)
	}
	if n := len(res.Chart.Series[0].Points); n != 3 {
		t.Errorf("%d points, want 3", n)
	}
	got := strings.Join(res.Chart.Labels, ",")
	if got != "00:00,11:53,23:06" {
		t.Errorf("labels %q, want 00:00,11:53,23:06", got)
	}
}

func TestGoldAccSkipsFailedBanks(t *testing.T) {
	res, err := builtin(t, "goldacc").Run(context.Background(), Input{
		Now: time.Now(),
		Sources: map[string]json.RawMessage{
			"icbc": json.RawMessage(`{"error":"timeout"}`),
			"ms":   json.RawMessage(jinMS),
			"jd":   json.RawMessage(jdGold),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Sub != "民生银行" {
		t.Errorf("sub %q, want 民生银行 when ICBC failed", res.Sub)
	}
	if res.Big != "808.78" {
		t.Errorf("big %q", res.Big)
	}
	if res.Chart == nil || len(res.Chart.Series[0].Points) != 2 {
		t.Fatalf("chart %+v, want 2 bars", res.Chart)
	}
}

func TestGoldAccOfflineHasNoSample(t *testing.T) {
	s := builtin(t, "goldacc")
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
		t.Errorf("offline goldacc still drew a chart")
	}
	if strings.Contains(strings.Join(res.Lines, " "), "示例") {
		t.Errorf("offline goldacc used sample copy: %q", res.Lines)
	}
	if res.Title == "" {
		t.Error("empty title")
	}
}
