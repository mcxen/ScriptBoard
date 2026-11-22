package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/store"
)

type testRoundTrip func(*http.Request) (*http.Response, error)

func (f testRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func testWeatherClient() *http.Client {
	return &http.Client{
		Transport: testRoundTrip(func(req *http.Request) (*http.Response, error) {
			u := req.URL.String()
			body := `{}`
			switch {
			case strings.Contains(u, "geocoding-api.open-meteo.com"):
				body = `{"results":[{"name":"Hangzhou","admin1":"Zhejiang","country_code":"CN","latitude":30.29,"longitude":120.16}]}`
			case strings.Contains(u, "api.open-meteo.com"):
				body = `{
					"current":{"temperature_2m":11.0,"apparent_temperature":8.0,"relative_humidity_2m":65,"weather_code":2,"is_day":0,"wind_speed_10m":9.0},
					"hourly":{
						"time":["2026-11-22T20:00","2026-11-22T21:00","2026-11-22T22:00","2026-11-22T23:00","2026-11-23T00:00","2026-11-23T01:00","2026-11-23T02:00","2026-11-23T03:00","2026-11-23T04:00","2026-11-23T05:00","2026-11-23T06:00","2026-11-23T07:00","2026-11-23T08:00"],
						"temperature_2m":[11,11,11,11,11,11,11,10,10,10,10,11,12],
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
				}`
			}
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		}),
	}
}

func TestBarStudioViewAndEnglish(t *testing.T) {
	t.Cleanup(func() {
		setLang(langZH)
		setThemeMode(themeSystem)
	})
	a := newTestApp(t)
	t.Cleanup(a.closeBarInstances)
	a.client = testWeatherClient()
	a.set.Offline = false

	tt := ui.NewTester(a.view, 1280, 820)
	settle(tt)

	// Navigate to Menu Bar Management (/bar)
	click(t, tt, "菜单管理")
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if inst, ok := a.barInsts["sb-weather"]; ok && inst.Snapshot().Tray != nil && inst.Snapshot().Tray.Title == "11°" {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}
	settle(tt)

	requireText(t, tt, "Weather")
	requireText(t, tt, "预览")
	requireText(t, tt, "显示位置")
	requireText(t, tt, "输入参数")
	requireText(t, tt, "来源")
	requireText(t, tt, "菜单栏")
	requireText(t, tt, "浮动面板")
	requireText(t, tt, "11°")
	requireText(t, tt, "Partly cloudy")
	savePNG(t, tt, "32-bar-studio-light")

	// Dark mode visual check
	setThemeMode(themeDark)
	settle(tt)
	savePNG(t, tt, "32-bar-studio-dark")

	// English check with zero leaked Chinese text
	setThemeMode(themeLight)
	setLang(langEN)
	settle(tt)
	requireText(t, tt, "Display locations")
	requireText(t, tt, "Inputs")
	onlyEnglish(t, tt, "bar:studio")
}

func TestBarDockPlacementsAndShortcutOverlay(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.closeBarInstances)
	a.client = testWeatherClient()
	a.set.Offline = false

	pl := a.barPlacement("sb-weather")
	pl.LeftSidebar = true
	pl.RightSidebar = true
	pl.BottomBar = true
	pl.Shortcut = true
	a.setBarPlacement("sb-weather", pl)
	a.barOverlayID = "sb-weather"

	tt := ui.NewTester(a.view, 1400, 900)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if inst, ok := a.barInsts["sb-weather"]; ok && inst.Snapshot().Tray != nil && inst.Snapshot().Tray.Title == "11°" {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}
	settle(tt)
	requireText(t, tt, "11°")
	requireText(t, tt, "打开组件详情")
}

func TestBarImportAndInputsAndHomeCard(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.closeBarInstances)

	customBar := `<bar id="custom-ping" title="Custom Ping" version="0.2.0" category="system" icon="activity">
  <inputs>
    <input name="host" type="string" label="Host" default="localhost" />
  </inputs>
  <script><![CDATA[
    export default {
      render(env) {
        const host = (env && env.inputs && env.inputs.host) || "none";
        return {
          tray: { title: host },
          panel: { type: "Column", props: { padding: 12 }, children: [
            { type: "Text", props: { fontSize: 14 }, children: ["Host: ", host] }
          ]}
        };
      }
    };
  ]]></script>
</bar>`

	if err := a.importText(customBar); err != nil {
		t.Fatalf("importText .bar failed: %v", err)
	}
	sc, ok := a.store.Script("custom-ping")
	if !ok || !sc.IsBar() || sc.Kind != store.KindBar {
		t.Fatalf("expected stored KindBar script, got %+v ok=%v", sc, ok)
	}
	a.setBarInput("custom-ping", "host", "127.0.0.1")

	// Enable on Home to verify home card rendering
	sc.Home = true
	if err := a.store.SaveScript(sc); err != nil {
		t.Fatal(err)
	}

	tt := ui.NewTester(a.view, 1200, 800)
	a.router.Push("/bar/custom-ping")
	settle(tt)
	requireText(t, tt, "Custom Ping")
	requireText(t, tt, "Host: 127.0.0.1")

	a.router.Push("/")
	settle(tt)
	requireText(t, tt, "Host: 127.0.0.1")
}

func TestMenuManageRegularScripts(t *testing.T) {
	t.Cleanup(func() { setLang(langZH) })
	a := newTestApp(t)
	t.Cleanup(a.closeBarInstances)

	tt := ui.NewTester(a.view, 1280, 820)
	a.router.Push("/bar/world-clock")
	settle(tt)
	requireText(t, tt, "世界时钟")
	requireText(t, tt, "显示位置")
	requireText(t, tt, "在组件管理中编辑")

	setLang(langEN)
	a.invalidateAll()
	settle(tt)
	requireText(t, tt, "Display locations")
	onlyEnglish(t, tt, "bar:regular-script")
}

func TestComponentManagerFiltering(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.closeBarInstances)

	tt := ui.NewTester(a.view, 1300, 860)
	click(t, tt, "组件管理")
	requireText(t, tt, "全部")
	requireText(t, tt, "+ .bar")

	click(t, tt, ".bar")
	requireText(t, tt, "Weather")

	click(t, tt, "数据表")
	requireText(t, tt, "体重数据")

	click(t, tt, "全部")
}
