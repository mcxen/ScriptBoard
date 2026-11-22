package bar

import (
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestParseBuiltinWeather(t *testing.T) {
	spec, err := Parse(BuiltinWeatherBar)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if spec.ID != "sb-weather" || spec.Title != "Weather" || spec.Version != "1.0.0" {
		t.Fatalf("unexpected spec header: %+v", spec)
	}
	if len(spec.Permissions) != 2 {
		t.Fatalf("want 2 network permissions, got %v", spec.Permissions)
	}
	if len(spec.Inputs) != 2 || !spec.Inputs[0].Required || len(spec.Inputs[1].Options) != 2 {
		t.Fatalf("unexpected inputs: %+v", spec.Inputs)
	}
	if got := RefreshDuration(spec); got != 15*time.Minute {
		t.Fatalf("RefreshDuration = %v, want 15m", got)
	}
	if !IsBarSource(BuiltinWeatherBar) {
		t.Fatal("IsBarSource returned false")
	}
}

func TestParseUserUploadedWeatherBarIfPresent(t *testing.T) {
	const path = "/Users/mcx/.gemini/antigravity/brain/302ae9ae-3eb0-4cce-8206-c0a10eae599b/.user_uploaded/media_1795351310761_ab5d2834.bar"
	b, err := os.ReadFile(path)
	if err != nil {
		t.Skip("uploaded .bar sample not present")
	}
	spec, err := Parse(string(b))
	if err != nil {
		t.Fatalf("Parse uploaded .bar failed: %v", err)
	}
	if spec.ID != "official--weather" || spec.Title != "Weather" {
		t.Fatalf("unexpected spec: %+v", spec)
	}
	if len(spec.Permissions) != 2 || len(spec.Inputs) != 2 {
		t.Fatalf("unexpected permissions/inputs: %+v / %+v", spec.Permissions, spec.Inputs)
	}
}

func TestOriginAllowed(t *testing.T) {
	allowed := []string{"https://api.open-meteo.com", "http://localhost:8080"}
	for _, u := range []string{
		"https://api.open-meteo.com/v1/forecast?latitude=30",
		"https://API.OPEN-METEO.COM:443/v1/forecast",
		"http://localhost:8080/ping",
	} {
		if err := OriginAllowed(u, allowed); err != nil {
			t.Errorf("expected allowed for %s, got %v", u, err)
		}
	}
	for _, u := range []string{
		"http://api.open-meteo.com/v1/forecast",
		"https://evil.example.com/steal",
		"https://api.open-meteo.com:8443/v1/forecast",
		"http://localhost:8081/ping",
		"file:///etc/passwd",
	} {
		if err := OriginAllowed(u, allowed); err == nil {
			t.Errorf("expected rejected for %s", u)
		}
	}
}

func mockOpenMeteoClient(calls *atomic.Int32) *http.Client {
	return &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calls.Add(1)
			u := req.URL.String()
			switch {
			case strings.Contains(u, "geocoding-api.open-meteo.com"):
				return jsonResponse(200, `{"results":[{"name":"Hangzhou","admin1":"Zhejiang","country_code":"CN","latitude":30.29,"longitude":120.16}]}`), nil
			case strings.Contains(u, "api.open-meteo.com"):
				imperial := strings.Contains(u, "temperature_unit=fahrenheit")
				temp := "11.2"
				if imperial {
					temp = "52.1"
				}
				return jsonResponse(200, `{
					"current":{"temperature_2m":`+temp+`,"apparent_temperature":8.1,"relative_humidity_2m":65,"weather_code":2,"is_day":0,"wind_speed_10m":9.2},
					"hourly":{
						"time":["2026-11-22T20:00","2026-11-22T21:00","2026-11-22T22:00","2026-11-22T23:00","2026-11-23T00:00","2026-11-23T01:00","2026-11-23T02:00","2026-11-23T03:00","2026-11-23T04:00","2026-11-23T05:00","2026-11-23T06:00","2026-11-23T07:00","2026-11-23T08:00"],
						"temperature_2m":[11,11,11,11,11,11,11,10.5,10,10,10,11,12],
						"weather_code":[2,2,3,3,3,3,3,3,2,2,3,3,3],
						"is_day":[0,0,0,0,0,0,0,0,0,0,0,1,1],
						"precipitation_probability":[0,0,10,10,10,10,10,10,10,10,10,10,10]
					},
					"daily":{
						"time":["2026-11-22","2026-11-23","2026-11-24","2026-11-25","2026-11-26","2026-11-27"],
						"weather_code":[3,61,61,3,61,0],
						"temperature_2m_max":[18,16,13,12,19,21],
						"temperature_2m_min":[3,10,5,2,8,7],
						"precipitation_probability_max":[0,42,39,10,20,0]
					}
				}`), nil
			default:
				return jsonResponse(404, `{}`), nil
			}
		}),
	}
}

func waitSnapshot(t *testing.T, ch <-chan Snapshot, pred func(Snapshot) bool) Snapshot {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case s := <-ch:
			if pred(s) {
				return s
			}
		case <-deadline:
			t.Fatal("timed out waiting for matching snapshot")
		}
	}
}

func TestInstanceLiveCycleAndActions(t *testing.T) {
	spec, err := Parse(BuiltinWeatherBar)
	if err != nil {
		t.Fatal(err)
	}

	var calls atomic.Int32
	updates := make(chan Snapshot, 16)
	var openedMu sync.Mutex
	var openedURL string

	inst, err := NewInstance(spec, Options{
		Client: mockOpenMeteoClient(&calls),
		Inputs: map[string]string{"location": "Hangzhou", "units": "metric"},
		Lang:   "en",
		OnUpdate: func(s Snapshot) {
			select {
			case updates <- s:
			default:
			}
		},
		OnOpenURL: func(u string) {
			openedMu.Lock()
			openedURL = u
			openedMu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("NewInstance: %v", err)
	}
	defer inst.Close()

	// Wait for scheduled() fetch + $metrix.invalidate() to populate temperature.
	snap := waitSnapshot(t, updates, func(s Snapshot) bool {
		return s.Tray != nil && s.Tray.Title == "11°"
	})
	if snap.Tray.Icon != "lucide:cloud-moon" {
		t.Fatalf("Tray.Icon = %q, want lucide:cloud-moon", snap.Tray.Icon)
	}
	if snap.Panel == nil || !hasNodeType(snap.Panel, "Chart") {
		t.Fatalf("expected Chart node in rendered panel")
	}
	if !hasText(snap.Panel, "Hangzhou") || !hasText(snap.Panel, "Partly cloudy") {
		t.Fatalf("expected Hangzhou and Partly cloudy in panel")
	}

	// Find refresh button action and invoke it.
	refreshID := findTooltipAction(snap.Panel, "Refresh")
	if refreshID == "" {
		t.Fatal("missing Refresh action ID on panel")
	}
	beforeCalls := calls.Load()
	inst.InvokeAction(refreshID)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && calls.Load() == beforeCalls {
		time.Sleep(15 * time.Millisecond)
	}
	if calls.Load() <= beforeCalls {
		t.Fatalf("Refresh action did not trigger new fetch")
	}

	// Test openURL button.
	extID := findTooltipAction(snap.Panel, "Weather data by Open-Meteo.com")
	if extID == "" {
		t.Fatal("missing external link action ID")
	}
	inst.InvokeAction(extID)
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		openedMu.Lock()
		u := openedURL
		openedMu.Unlock()
		if u == "https://open-meteo.com/" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	openedMu.Lock()
	if openedURL != "https://open-meteo.com/" {
		t.Fatalf("openedURL = %q, want https://open-meteo.com/", openedURL)
	}
	openedMu.Unlock()

	// Test UpdateInputs switching to imperial.
	inst.UpdateInputs(map[string]string{"location": "Hangzhou", "units": "imperial"}, "en")
	snapImp := waitSnapshot(t, updates, func(s Snapshot) bool {
		return s.Tray != nil && s.Tray.Title == "52°"
	})
	if !hasText(snapImp.Panel, "52°") {
		t.Fatalf("expected 52° in imperial panel")
	}
}

func TestInstanceBlocksUnauthorizedOrigin(t *testing.T) {
	raw := `<bar id="bad-net" title="BadNet" version="1.0.0">
  <permissions>
    <network origin="https://allowed.example" />
  </permissions>
  <script><![CDATA[
    let msg = "init";
    export default {
      async scheduled() {
        try {
          await fetch("https://blocked.example/secret");
          msg = "unexpected-ok";
        } catch (e) {
          msg = String(e && e.message || e);
        }
        $metrix.invalidate();
      },
      render() {
        return { panel: { type: "Text", props: {}, children: [msg] } };
      }
    };
  ]]></script>
</bar>`
	spec, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	updates := make(chan Snapshot, 4)
	inst, err := NewInstance(spec, Options{
		Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return jsonResponse(200, `{}`), nil
		})},
		OnUpdate: func(s Snapshot) { updates <- s },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close()

	snap := waitSnapshot(t, updates, func(s Snapshot) bool {
		return s.Panel != nil && s.Panel.Text != "init"
	})
	if snap.Panel.Text == "unexpected-ok" {
		t.Fatalf("unauthorized origin was not blocked!")
	}
}

func hasNodeType(n *Node, typ string) bool {
	if n == nil {
		return false
	}
	if n.Type == typ {
		return true
	}
	for _, c := range n.Children {
		if hasNodeType(c, typ) {
			return true
		}
	}
	return false
}

func hasText(n *Node, needle string) bool {
	if n == nil {
		return false
	}
	if strings.Contains(n.Text, needle) {
		return true
	}
	for _, c := range n.Children {
		if hasText(c, needle) {
			return true
		}
	}
	return false
}

func findTooltipAction(n *Node, tip string) string {
	if n == nil {
		return ""
	}
	if n.OnClickID != "" && n.Props != nil {
		if t, ok := n.Props["tooltip"].(string); ok && t == tip {
			return n.OnClickID
		}
	}
	for _, c := range n.Children {
		if id := findTooltipAction(c, tip); id != "" {
			return id
		}
	}
	return ""
}
