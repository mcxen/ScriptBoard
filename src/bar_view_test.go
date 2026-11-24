package main

import (
	"image"
	"image/color"
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

func countRGBInBounds(img image.Image, box image.Rectangle, want ui.Color) int {
	n := 0
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if c.R == want.R && c.G == want.G && c.B == want.B {
				n++
			}
		}
	}
	return n
}

func boundingBoxOfRGB(img image.Image, search image.Rectangle, want ui.Color) (image.Rectangle, bool) {
	var minX, minY, maxX, maxY int
	found := false
	for y := search.Min.Y; y < search.Max.Y; y++ {
		for x := search.Min.X; x < search.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if c.R == want.R && c.G == want.G && c.B == want.B {
				if !found {
					minX, maxX = x, x+1
					minY, maxY = y, y+1
					found = true
				} else {
					if x < minX {
						minX = x
					}
					if x+1 > maxX {
						maxX = x + 1
					}
					if y < minY {
						minY = y
					}
					if y+1 > maxY {
						maxY = y + 1
					}
				}
			}
		}
	}
	if !found {
		return image.Rectangle{}, false
	}
	return image.Rect(minX, minY, maxX, maxY), true
}

func TestScriptMenuCardPreviewFollowsThemeOverride(t *testing.T) {
	t.Cleanup(func() {
		setThemeMode(themeSystem)
		setSkin(skinDefault)
		setLang(langZH)
	})
	a := newTestApp(t)
	t.Cleanup(a.closeBarInstances)

	fxTableSrc := `export function meta() {
  return { name: "外汇牌价数据表", icon: "file-spreadsheet", menubar: true, sources: {} };
}
export function render() {
  return {
    title: "外汇牌价数据表",
    icon: "file-spreadsheet",
    big: "¥6.6778",
    sub: "美元兑人民币 · 2026-11-20",
    lines: [],
    menubar: { title: "¥6.6778" },
    menu: [
      { label: "复制美元汇率", action: "copy_headline" },
      { label: "打开设置", action: "settings" }
    ],
    chart: {
      type: "table",
      table: {
        striped: true,
        compact: 5,
        columns: [
          { title: "币种", width: 2 },
          { title: "代码", format: "badge", align: "center" },
          { title: "折合人民币", align: "right", format: "number" },
          { title: "100 CNY 换", align: "right", format: "number" },
          { title: "状态", format: "status" }
        ],
        rows: [
          ["美元", "USD", "6.6778", "14.97", "ok"],
          ["欧元", "EUR", "7.3380", "13.63", "ok"],
          ["英镑", "GBP", "8.7640", "11.41", "ok"],
          ["港币", "HKD", "0.8518", "117.39", "ok"],
          ["日元 (100)", "JPY", "4.2095", "2375.58", "ok"]
        ]
      }
    }
  };
}`
	if err := a.store.SaveScript(store.Script{
		ID:     "test-fx-table",
		Name:   "外汇牌价数据表",
		Origin: store.OriginUser,
		Source: fxTableSrc,
	}); err != nil {
		t.Fatal(err)
	}

	tt := ui.NewTester(a.view, 1280, 860)

	// Case A: Dark app window + Light menu card preview override.
	setThemeMode(themeDark)
	a.barPreviewTheme = barThemeLight
	a.router.Push("/bar/test-fx-table")
	settle(tt)
	settle(tt)
	savePNG(t, tt, "34-script-menu-card-light-in-dark")

	requireText(t, tt, "外汇牌价数据表")
	requireText(t, tt, "复制美元汇率")
	requireText(t, tt, "打开设置")

	imgLight := tt.Image()
	b := imgLight.Bounds()
	// Search inside preview stage area (right of left sidebar & component list).
	stageArea := image.Rect(b.Min.X+b.Dx()/3, b.Min.Y, b.Max.X, b.Max.Y)
	cardBox, ok := boundingBoxOfRGB(imgLight, stageArea, ui.Hex("#FFFFFF"))
	if !ok {
		t.Fatalf("could not locate white preview card (#FFFFFF) in dark window")
	}
	lightHdrN := countRGBInBounds(imgLight, cardBox, ui.Hex("#EDEFF4"))
	darkHdrN := countRGBInBounds(imgLight, cardBox, ui.Hex("#232731"))
	if lightHdrN < 100 || darkHdrN != 0 {
		t.Fatalf("light preview inside dark window used wrong table header theme: lightHdrPixels=%d darkHdrPixels=%d (cardBox=%v)", lightHdrN, darkHdrN, cardBox)
	}

	// Case B: Light app window + Dark menu card preview override.
	setThemeMode(themeLight)
	a.barPreviewTheme = barThemeDark
	settle(tt)
	settle(tt)

	imgDark := tt.Image()
	darkCardBox, ok := boundingBoxOfRGB(imgDark, stageArea, ui.Hex("#1C1F26"))
	if !ok {
		t.Fatalf("could not locate dark preview card (#1C1F26) in light window")
	}
	darkCardDarkHdrN := countRGBInBounds(imgDark, darkCardBox, ui.Hex("#232731"))
	darkCardLightHdrN := countRGBInBounds(imgDark, darkCardBox, ui.Hex("#EDEFF4"))
	if darkCardDarkHdrN < 100 || darkCardLightHdrN != 0 {
		t.Fatalf("dark preview inside light window used wrong table header theme: darkHdrPixels=%d lightHdrPixels=%d (darkCardBox=%v)", darkCardDarkHdrN, darkCardLightHdrN, darkCardBox)
	}
}

func TestBarCookiesUIImportClearAndBadgeProgressEnglish(t *testing.T) {
	a := newTestApp(t)
	src := `<bar id="cookie-ui-bar" title="Cookie UI" version="1.0.0" category="developer" icon="key" refresh="10m">
  <permissions>
    <network origin="https://api.example.com" />
  </permissions>
  <inputs>
    <input name="tok" type="password" label="API Token" default="secret-default-token" />
  </inputs>
  <script><![CDATA[
    const h = (type, props, ...children) => ({
      type, props: props || {}, children: children.flat().filter(c => c !== null && c !== undefined && c !== false)
    });
    export default {
      render(env) {
        const cnt = Object.keys($metrix.cookies.get("https://api.example.com")).length;
        return {
          tray: { title: "CK:" + cnt },
          panel: h("Column", { preferredWidth: 290, padding: 12, gap: 8 },
            h("Row", { alignItems: "center", gap: 6 },
              h("Text", { fontSize: 13, fontWeight: 600 }, "Session Panel"),
              h("Spacer"),
              h("Badge", { background: cnt > 0 ? "accent" : "warning" }, cnt > 0 ? "Live" : "Empty")
            ),
            h("Progress", { value: 60, max: 100, height: 8 })
          )
        };
      }
    };
  ]]></script>
</bar>`
	sc, err := a.importBarSource(src)
	if err != nil {
		t.Fatal(err)
	}

	tt := ui.NewTester(a.view, 1280, 1400)
	a.router.Push("/bar/" + sc.ID)
	settle(tt)
	requireText(t, tt, "Session Panel")
	requireText(t, tt, "Empty")
	requireText(t, tt, "Cookie 会话")

	// Open cookie session manager, import cookies, and verify count updates.
	click(t, tt, "管理 Cookie")
	requireText(t, tt, "收起")
	a.barCookieURL = "https://api.example.com"
	a.barCookieInput = "sid=demo123; csrf=456"
	click(t, tt, "导入并刷新")
	if got := a.barCookieCount(sc.ID); got != 2 {
		t.Fatalf("barCookieCount after UI import = %d, want 2", got)
	}
	if inst, ok := a.barInsts[sc.ID]; ok {
		inst.WaitIdle(0)
	}
	settle(tt)
	requireText(t, tt, "已保存 2 条 Cookie")
	requireText(t, tt, "Live")

	// Clear cookies via UI button.
	click(t, tt, "清空 Cookie")
	if got := a.barCookieCount(sc.ID); got != 0 {
		t.Fatalf("barCookieCount after clear = %d, want 0", got)
	}
	if inst, ok := a.barInsts[sc.ID]; ok {
		inst.WaitIdle(0)
	}
	settle(tt)
	requireText(t, tt, "未保存 Cookie")

	// Switch to English UI and test importing & clearing in English.
	setLang("en")
	defer setLang("zh")
	settle(tt)
	requireText(t, tt, "Cookie session")
	requireText(t, tt, "No cookies saved")
	requireText(t, tt, "Cookies cleared")
	requireText(t, tt, "Import & refresh")
	onlyEnglish(t, tt, "bar cookies empty")

	a.barCookieURL = "https://api.example.com"
	a.barCookieInput = "sid=en123"
	click(t, tt, "Import & refresh")
	if got := a.barCookieCount(sc.ID); got != 1 {
		t.Fatalf("barCookieCount after English import = %d, want 1", got)
	}
	if inst, ok := a.barInsts[sc.ID]; ok {
		inst.WaitIdle(0)
	}
	settle(tt)
	requireText(t, tt, "1 cookies saved")
	requireText(t, tt, "Clear cookies")
	requireText(t, tt, "Live")
	onlyEnglish(t, tt, "bar cookies loaded")

	click(t, tt, "Clear cookies")
	if inst, ok := a.barInsts[sc.ID]; ok {
		inst.WaitIdle(0)
	}
	settle(tt)
	requireText(t, tt, "No cookies saved")
	requireText(t, tt, "Empty")
	onlyEnglish(t, tt, "bar cookies cleared")
}
