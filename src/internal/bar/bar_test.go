package bar

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"scriptboard/src/internal/cmdrun"
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

	// Clicking Refresh on Weather bar shows Updated check state after completion.
	refBtnID := findTooltipAction(snapImp.Panel, "Refresh")
	if refBtnID == "" {
		t.Fatal("expected Refresh action after UpdateInputs")
	}
	inst.InvokeAction(refBtnID)
	snapUpdated := waitSnapshot(t, updates, func(s Snapshot) bool {
		return findTooltipAction(s.Panel, "Updated") != ""
	})
	if snapUpdated.Panel == nil {
		t.Fatal("expected updated weather panel")
	}
}

func TestRefreshLifecycleFlagAndEnv(t *testing.T) {
	gate := make(chan struct{})
	raw := `<bar protocol-version="0" id="ref-life" title="RefLife" version="1.0.0">
  <permissions>
    <network origin="https://example.com" />
  </permissions>
  <script><![CDATA[
    export default {
      async scheduled() {
        const resp = await fetch("https://example.com/data");
        if (!resp.ok) throw new Error("HTTP " + resp.status);
      },
      render(env) {
        return { panel: h("Text", {}, env.refreshing ? "loading" : "idle") };
      }
    };
  ]]></script>
</bar>`
	spec, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	var failReq atomic.Bool
	updates := make(chan Snapshot, 16)
	inst, err := NewInstance(spec, Options{
		Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			<-gate
			if failReq.Load() {
				return jsonResponse(500, `{}`), nil
			}
			return jsonResponse(200, `{}`), nil
		})},
		OnUpdate: func(s Snapshot) {
			select {
			case updates <- s:
			default:
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer inst.Close()

	// Initial scheduled run is gated; Refreshing should be true and panel text "loading".
	snapLoading := waitSnapshot(t, updates, func(s Snapshot) bool {
		return s.Refreshing && s.Panel != nil && s.Panel.Text == "loading"
	})
	if !snapLoading.Refreshing {
		t.Fatal("expected Refreshing=true while scheduled is gated")
	}

	close(gate)
	snapDone := waitSnapshot(t, updates, func(s Snapshot) bool {
		return !s.Refreshing && s.Panel != nil && s.Panel.Text == "idle"
	})
	if snapDone.Error != "" {
		t.Fatalf("unexpected error on success refresh: %q", snapDone.Error)
	}

	// Next Refresh() fails and should report Error with Refreshing=false.
	failReq.Store(true)
	inst.Refresh()
	snapErr := waitSnapshot(t, updates, func(s Snapshot) bool {
		return !s.Refreshing && strings.Contains(s.Error, "HTTP 500")
	})
	if snapErr.Error == "" {
		t.Fatal("expected Error after failed scheduled()")
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

func findActionByType(n *Node, typ string) string {
	if n == nil {
		return ""
	}
	if n.Type == typ {
		if n.OnChangeID != "" {
			return n.OnChangeID
		}
		if n.OnClickID != "" {
			return n.OnClickID
		}
	}
	for _, c := range n.Children {
		if id := findActionByType(c, typ); id != "" {
			return id
		}
	}
	return ""
}

type mockCommandBridge struct {
	mu      sync.Mutex
	history []cmdrun.Record
	calls   []map[string]string
	style   *cmdrun.DisplayStyle
}

func (m *mockCommandBridge) Style(_ string) (*cmdrun.DisplayStyle, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.style == nil {
		return nil, false
	}
	return m.style.Clone(), true
}

func (m *mockCommandBridge) setStyle(ds *cmdrun.DisplayStyle) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.style = ds.Clone()
}

func (m *mockCommandBridge) Run(_ context.Context, id string, params map[string]string) (cmdrun.Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make(map[string]string, len(params))
	for k, v := range params {
		cp[k] = v
	}
	m.calls = append(m.calls, cp)
	host := cp["host"]
	if host == "" {
		host = "223.5.5.5"
	}
	rec := cmdrun.Record{
		ID:         "r1",
		CommandID:  id,
		T:          1700000000000 + int64(len(m.history)*1000),
		Trigger:    "bar",
		Params:     map[string]string{"host": host},
		OK:         true,
		ExitCode:   0,
		DurationMs: 18,
		Extracted: map[string]any{
			"avgMs":      12.4,
			"minMs":      10.1,
			"maxMs":      15.2,
			"jitterMs":   1.6,
			"lossPct":    0.0,
			"resolvedIp": host,
		},
	}
	m.history = append([]cmdrun.Record{rec}, m.history...)
	return rec, nil
}

func (m *mockCommandBridge) Latest(_ string) (cmdrun.Record, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.history) == 0 {
		return cmdrun.Record{}, false, nil
	}
	return m.history[0], true, nil
}

func (m *mockCommandBridge) History(_ string, limit int) ([]cmdrun.Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit <= 0 || limit >= len(m.history) {
		return append([]cmdrun.Record(nil), m.history...), nil
	}
	return append([]cmdrun.Record(nil), m.history[:limit]...), nil
}

func (m *mockCommandBridge) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func TestCommandPermissionAndBuiltinPingBar(t *testing.T) {
	// 1. Undeclared command call is rejected by CommandAllowed.
	deniedXML := `<bar id="denied-cmd" title="Denied" version="1.0.0">
  <script><![CDATA[
    let status = "";
    export default {
      async scheduled() {
        try {
          await $metrix.command.run("net-ping", {});
          status = "unexpected-ok";
        } catch (e) {
          status = String(e.message || e);
        }
      },
      render() { return { panel: h("Text", {}, status) }; }
    };
  ]]></script>
</bar>`
	spDenied, err := Parse(deniedXML)
	if err != nil {
		t.Fatal(err)
	}
	bridge := &mockCommandBridge{}
	instDenied, err := NewInstance(spDenied, Options{Commands: bridge})
	if err != nil {
		t.Fatal(err)
	}
	defer instDenied.Close()
	if !instDenied.WaitIdle(3 * time.Second) {
		t.Fatal("denied WaitIdle timed out")
	}
	textsDenied := strings.Join(CollectTexts(instDenied.Snapshot().Panel), " ")
	if !strings.Contains(textsDenied, "not permitted") {
		t.Fatalf("expected permission denial error, got %q", textsDenied)
	}

	// 2. BuiltinPingBar parses cleanly, passes CheckRender, and updates interactively via CommandBridge.
	if err := CommandAllowed("net-ping", nil); err == nil {
		t.Fatal("expected CommandAllowed to deny empty perms")
	}
	sp, _, warnings, err := CheckRender(BuiltinPingBar, "en")
	if err != nil {
		t.Fatalf("CheckRender BuiltinPingBar: %v", err)
	}
	if len(sp.CommandPerms) != 1 || sp.CommandPerms[0] != "net-ping" {
		t.Fatalf("unexpected CommandPerms: %v", sp.CommandPerms)
	}
	if len(warnings) > 0 {
		t.Fatalf("unexpected warnings on BuiltinPingBar: %v", warnings)
	}

	updates := make(chan Snapshot, 16)
	inst, err := NewInstance(sp, Options{
		DataDir:  t.TempDir(),
		Lang:     "en",
		Theme:    "dark",
		Commands: bridge,
		OnUpdate: func(s Snapshot) {
			select {
			case updates <- s:
			default:
			}
		},
	})
	if err != nil {
		t.Fatalf("NewInstance BuiltinPingBar: %v", err)
	}
	defer inst.Close()

	snap := waitSnapshot(t, updates, func(s Snapshot) bool {
		return !s.Refreshing && s.Tray != nil && s.Tray.Title == "12ms"
	})
	panelTxt := strings.Join(CollectTexts(snap.Panel), " | ")
	if !strings.Contains(panelTxt, "Healthy") || !strings.Contains(panelTxt, "223.5.5.5") {
		t.Fatalf("unexpected BuiltinPingBar texts after initial run: %q", panelTxt)
	}

	// Switch preset via Segmented onChange.
	seg := findActionByType(snap.Panel, "Segmented")
	if seg == "" {
		t.Fatal("missing Segmented action in BuiltinPingBar")
	}
	inst.InvokeActionWithArg(seg, "1.1.1.1")
	snapPreset := waitSnapshot(t, updates, func(s Snapshot) bool {
		return !s.Refreshing && strings.Contains(strings.Join(CollectTexts(s.Panel), " "), "1.1.1.1") && bridge.callCount() >= 2
	})
	if len(CollectTexts(snapPreset.Panel)) == 0 {
		t.Fatal("empty snapshot after preset switch")
	}

	// 3. Apply custom DisplayStyle with dot wave/dots animation and assert BuiltinPingBar reflects it.
	max50 := 50.0
	bridge.setStyle(&cmdrun.DisplayStyle{
		PrimaryField: "avgMs",
		Variant:      cmdrun.StyleVariantDots,
		Animation:    cmdrun.AnimDots,
		DotCount:     6,
		DotSize:      9,
		Speed:        1.5,
		Fields: map[string]cmdrun.FieldStyle{
			"avgMs": {
				Label: "Live Latency",
				Unit:  "ms",
				Thresholds: []cmdrun.StyleThreshold{
					{Max: &max50, Color: "cyan", Label: "Blazing", Animation: cmdrun.AnimDots},
				},
			},
		},
	})
	inst.RefreshRender()
	snapStyled := waitSnapshot(t, updates, func(s Snapshot) bool {
		return strings.Contains(strings.Join(CollectTexts(s.Panel), " "), "Blazing")
	})
	if !hasStatusDotWithAnim(snapStyled.Panel, "dots", 6) {
		t.Fatalf("expected StatusDot with animation=dots count=6 in styled ping snapshot")
	}

	// 4. Test custom format template & valueMap on BuiltinPingBar via $metrix.command.resolve().
	bridge.setStyle(&cmdrun.DisplayStyle{
		PrimaryField: "avgMs",
		Variant:      cmdrun.StyleVariantDot,
		Animation:    cmdrun.AnimOrbit,
		Format:       "{{value}}ms/{{status}}",
		Thresholds: []cmdrun.StyleThreshold{
			{Max: &max50, Color: "success", Label: "Sonic"},
		},
		Fields: map[string]cmdrun.FieldStyle{
			"lossPct": {ValueMap: map[string]string{"0": "Clean"}},
		},
	})
	inst.RefreshRender()
	snapFmt := waitSnapshot(t, updates, func(s Snapshot) bool {
		return strings.Contains(strings.Join(CollectTexts(s.Panel), " "), "12.4ms/Sonic")
	})
	fmtTxt := strings.Join(CollectTexts(snapFmt.Panel), " | ")
	if !strings.Contains(fmtTxt, "Clean") || !hasStatusDotWithAnim(snapFmt.Panel, "orbit", 4) {
		t.Fatalf("expected custom format/valueMap/orbit in BuiltinPingBar snapshot, got: %q", fmtTxt)
	}

	// 5. Undeclared $metrix.command.resolve() is rejected by CommandAllowed.
	deniedResolveXML := `<bar id="denied-resolve" title="DeniedResolve" version="1.0.0">
  <script><![CDATA[
    let errText = "";
    export default {
      scheduled() {
        try {
          $metrix.command.resolve("svc-health", {});
          errText = "unexpected-ok";
        } catch (e) {
          errText = String(e.message || e);
        }
      },
      render() { return { panel: h("Text", {}, errText) }; }
    };
  ]]></script>
</bar>`
	spDenRes, err := Parse(deniedResolveXML)
	if err != nil {
		t.Fatal(err)
	}
	instDenRes, err := NewInstance(spDenRes, Options{Commands: bridge})
	if err != nil {
		t.Fatal(err)
	}
	defer instDenRes.Close()
	if !instDenRes.WaitIdle(3 * time.Second) {
		t.Fatal("denied resolve WaitIdle timed out")
	}
	if got := strings.Join(CollectTexts(instDenRes.Snapshot().Panel), " "); !strings.Contains(got, "not permitted") {
		t.Fatalf("expected not permitted error from $metrix.command.resolve, got %q", got)
	}

	// 6. Generic custom .bar component renders any non-Ping command via $metrix.command.resolve + $ui.commandValue + $ui.commandFieldStrip.
	genericBarXML := `<bar id="svc-health-bar" title="Service Health" version="1.0.0">
  <permissions><command id="svc-health" /></permissions>
  <script><![CDATA[
    export default {
      render() {
        const resolved = $metrix.command.resolve("svc-health", {
          commandId: "svc-health",
          ok: true,
          params: { env: "prod" },
          extracted: { code: "0", qps: 420 }
        });
        return {
          panel: h("Column", { preferredWidth: 300, padding: 14, gap: 10 },
            $ui.commandValue(resolved.primary),
            $ui.commandFieldStrip(resolved, { excludePrimary: true })
          )
        };
      }
    };
  ]]></script>
</bar>`
	spGen, err := Parse(genericBarXML)
	if err != nil {
		t.Fatal(err)
	}
	genBridge := &mockCommandBridge{
		style: &cmdrun.DisplayStyle{
			PrimaryField: "code",
			Variant:      cmdrun.StyleVariantDot,
			Animation:    cmdrun.AnimWave,
			DotCount:     5,
			ValueMap:     map[string]string{"0": "Healthy"},
			Format:       "{{param.env}}:{{value}}",
			Fields: map[string]cmdrun.FieldStyle{
				"qps": {Label: "QPS", Unit: "req/s"},
			},
		},
	}
	instGen, err := NewInstance(spGen, Options{Commands: genBridge})
	if err != nil {
		t.Fatal(err)
	}
	defer instGen.Close()
	if !instGen.WaitIdle(3 * time.Second) {
		t.Fatal("generic bar WaitIdle timed out")
	}
	genSnap := instGen.Snapshot()
	genTexts := strings.Join(CollectTexts(genSnap.Panel), " | ")
	if !strings.Contains(genTexts, "prod:Healthy") || !strings.Contains(genTexts, "QPS") || !strings.Contains(genTexts, "420 req/s") {
		t.Fatalf("unexpected generic command UI render texts: %q", genTexts)
	}
	if !hasStatusDotWithAnim(genSnap.Panel, "wave", 5) {
		t.Fatalf("expected StatusDot animation=wave count=5 on generic command .bar")
	}
}

func hasStatusDotWithAnim(n *Node, wantAnim string, wantCount int) bool {
	if n == nil {
		return false
	}
	if n.Type == "StatusDot" {
		if a, _ := n.Props["animation"].(string); a == wantAnim {
			if cnt, ok := n.Props["count"].(float64); !ok || int(cnt) == wantCount {
				return true
			}
		}
	}
	for _, c := range n.Children {
		if hasStatusDotWithAnim(c, wantAnim, wantCount) {
			return true
		}
	}
	return false
}
