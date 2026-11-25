package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"

	"scriptboard/src/internal/caps"
	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

func TestMobileTabBarHidesForKeyboard(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 390, 844)
	settle(tt)
	settle(tt)
	requireText(t, tt, "组件管理")
	requireText(t, tt, "菜单管理")

	// Opening the on-screen keyboard raises the bottom safe-area inset above 120 DIPs.
	tt.SetInsets(0, 0, 260, 0)
	settle(tt)
	if tt.HasText("组件管理") || tt.HasText("菜单管理") {
		t.Errorf("bottom tabBar still visible while software keyboard inset is active: %v", tt.Texts())
	}

	// Dismissing the keyboard back to the home-indicator inset restores the tab bar.
	tt.SetInsets(0, 0, 34, 0)
	settle(tt)
	requireText(t, tt, "组件管理")
	requireText(t, tt, "菜单管理")
}

func TestMobileCardLongPressActionSheet(t *testing.T) {
	t.Cleanup(func() { caps.SetPlatform("") })
	caps.SetPlatform(caps.Mobile)

	a := newTestApp(t)
	var shared []mygo.ShareOptions
	a.shareFn = func(opts mygo.ShareOptions) error {
		shared = append(shared, opts)
		return nil
	}

	tt := ui.NewTester(a.view, 390, 844)
	settle(tt)
	settle(tt)

	// Locate a visible text node inside the fx card body on phone home.
	var fxPos ui.Rect
	found := false
	for _, s := range tt.Texts() {
		if len(s) >= 5 && s[:5] == "USD /" {
			if info, ok := tt.Access(s); ok {
				fxPos = info.Bounds
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatalf("missing fx card text on phone home: %v", tt.Texts())
	}
	cx := fxPos.X + fxPos.W/2
	cy := fxPos.Y + fxPos.H/2

	tt.FingerDown(cx, cy)
	tt.Advance(600 * time.Millisecond)
	tt.Frame()
	tt.FingerUp(cx, cy)
	settle(tt)

	if !a.sheetOpen {
		t.Fatalf("expected long-press action sheet to open; path=%s", a.router.Path())
	}
	if got := a.router.Path(); got != "/" {
		t.Fatalf("long press navigated to %q instead of staying on home", got)
	}
	requireText(t, tt, "刷新数据")
	requireText(t, tt, "分享脚本")
	requireText(t, tt, "从首页移除")

	click(t, tt, "分享脚本")
	if a.sheetOpen {
		t.Errorf("expected action sheet to close after tapping action")
	}
	if len(shared) != 1 || shared[0].Text == "" {
		t.Fatalf("expected Share callback with script export, got %+v", shared)
	}
}

func TestMobileZoomableGalleryAndReset(t *testing.T) {
	t.Cleanup(func() {
		caps.SetPlatform("")
		setLang(langZH)
	})
	caps.SetPlatform(caps.Mobile)

	a := newTestApp(t)
	sc := store.Script{
		ID:     "gallery-mob",
		Name:   "必应壁纸",
		Home:   false,
		Origin: store.OriginUser,
		Source: `export function meta() { return { name: "必应壁纸", sources: {} }; }
export function render(ctx) { return { title: "必应壁纸", big: "2", sub: "", lines: [], chart: null, images: [] }; }`,
	}
	if err := a.store.SaveScript(sc); err != nil {
		t.Fatal(err)
	}
	for _, u := range []string{"http://x/1.png", "http://x/2.png"} {
		if _, err := a.cache.PutBlob(sc.ID, u, pngBytes(t)); err != nil {
			t.Fatal(err)
		}
	}
	a.keepImages(sc, []engine.Image{
		{URL: "http://x/1.png", Caption: "one"},
		{URL: "http://x/2.png", Caption: "two"},
	}, time.Now())

	tt := ui.NewTester(a.view, 390, 844)
	a.router.Push("/full/gallery-mob")
	settle(tt)
	settle(tt)

	if a.galleryZoom.Scale != 1 {
		t.Fatalf("initial zoom scale = %v, want 1", a.galleryZoom.Scale)
	}
	requireText(t, tt, "分享")

	// Zoom in via double-click / state and reset via the floating scale pill.
	a.galleryZoom.Scale = 2
	settle(tt)
	requireText(t, tt, "2.0×")
	click(t, tt, "2.0×")
	if a.galleryZoom.Scale != 1 {
		t.Errorf("scale after tapping reset pill = %v, want 1", a.galleryZoom.Scale)
	}

	// Switching images also resets active zoom back to 1x.
	a.galleryZoom.Scale = 2.5
	settle(tt)
	click(t, tt, "›")
	requireText(t, tt, "2 / 2")
	if a.galleryZoom.Scale != 1 {
		t.Errorf("scale after switching photo = %v, want 1", a.galleryZoom.Scale)
	}

	// Verify English localization on mobile gallery & ActionSheet labels.
	a.set.Lang = langEN
	setLang(langEN)
	settle(tt)
	requireText(t, tt, "Share")
	onlyEnglish(t, tt, "mobile:gallery")

	aEn := newTestApp(t)
	aEn.set.Lang = langEN
	setLang(langEN)
	ttEn := ui.NewTester(aEn.view, 390, 844)
	settle(ttEn)
	settle(ttEn)
	aEn.sheetScriptID = "fx"
	aEn.sheetOpen = true
	settle(ttEn)
	requireText(t, ttEn, "Refresh data")
	requireText(t, ttEn, "Share script")
	requireText(t, ttEn, "Remove from home")
	if mobile.Current() == mobile.Cupertino {
		requireText(t, ttEn, "Cancel")
	}
	onlyEnglish(t, ttEn, "mobile:actionsheet")
}

func TestMobilePlatformAndSystemBarsAndState(t *testing.T) {
	t.Cleanup(func() { caps.SetPlatform("") })
	caps.SetPlatform(caps.Mobile)

	a := newTestApp(t)
	if !isMobile() || !a.touchUI() {
		t.Errorf("expected isMobile() and touchUI() to be true when caps platform is Mobile")
	}

	hooked := false
	a.openSettingsHook = func() { hooked = true }
	a.goTo("/settings")
	if hooked {
		t.Errorf("mobile goTo(/settings) invoked desktop openSettingsHook")
	}
	if got := a.router.Path(); got != "/settings" {
		t.Errorf("router path = %q, want /settings", got)
	}

	tt := ui.NewTester(a.view, 390, 844)
	a.router.Push("/bar")
	settle(tt)
	requireText(t, tt, "移动端在应用内「菜单管理」与首页卡片中显示")

	// SystemBars style mapping across modes & skins.
	cases := []struct {
		mode string
		mac  bool
		dark bool
		want mygo.BarStyle
	}{
		{themeSystem, false, false, mygo.BarStyleAuto},
		{themeSystem, false, true, mygo.BarStyleAuto},
		{themeSystem, true, false, mygo.BarStyleDark},
		{themeLight, false, false, mygo.BarStyleDark},
		{themeDark, false, true, mygo.BarStyleLight},
	}
	for _, tc := range cases {
		if got := systemBarStyle(tc.mode, tc.mac, tc.dark); got != tc.want {
			t.Errorf("systemBarStyle(%q, %v, %v) = %q, want %q", tc.mode, tc.mac, tc.dark, got, tc.want)
		}
	}

	// Mobile background-kill state encoding & restoration.
	a.router.Push("/full/weight")
	a.settingsTab = "about"
	a.editTab = 2
	raw := a.encodeAppState()

	a2 := newTestApp(t)
	a2.restoreAppState(raw)
	if a2.router.Path() != "/full/weight" || a2.settingsTab != "about" || a2.editTab != 2 {
		t.Errorf("restored state mismatch: path=%q tab=%q editTab=%d", a2.router.Path(), a2.settingsTab, a2.editTab)
	}

	a3 := newTestApp(t)
	a3.restoreAppState([]byte(`{"path":"/unknown-route","settingsTab":"nope","editTab":99}`))
	if a3.router.Path() != "/" || a3.editTab != 0 {
		t.Errorf("invalid restored state should be ignored: path=%q editTab=%d", a3.router.Path(), a3.editTab)
	}
}

func TestPullToRefreshStateLifecycle(t *testing.T) {
	a := newTestApp(t)
	scripts := a.homeScripts()
	if len(scripts) == 0 {
		t.Fatal("expected built-in home scripts")
	}
	if a.anyHomeRefreshing(scripts) {
		t.Fatal("unexpected refreshing state initially")
	}

	// Render Home, FullView, and Community with stale Refreshing=true and verify
	// headless frames clear them cleanly instead of looping AnimationFrame.
	a.homeRefresh.Refreshing = true
	a.fullRefresh.Refreshing = true
	a.commRefresh.Refreshing = true

	tt := ui.NewTester(a.view, 390, 844)
	settle(tt)
	if a.homeRefresh.Refreshing {
		t.Errorf("homeRefresh.Refreshing did not clear after synchronous settle")
	}

	a.router.Push("/full/fx")
	settle(tt)
	if a.fullRefresh.Refreshing {
		t.Errorf("fullRefresh.Refreshing did not clear after synchronous settle")
	}

	a.router.Push("/community")
	settle(tt)
	if a.commRefresh.Refreshing {
		t.Errorf("commRefresh.Refreshing did not clear after synchronous settle")
	}

	// Synchronous refreshAllHome triggers all home cards safely without panic.
	a.refreshAllHome(scripts)
	if a.anyHomeRefreshing(scripts) {
		t.Errorf("expected offline synchronous refreshAllHome to settle immediately")
	}
}
