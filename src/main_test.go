package main

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestMain fixes the language to Chinese, which the page tests read; the
// English tests switch it themselves.
func TestMain(m *testing.M) {
	os.Setenv("SCRIPTBOARD_LANG", "zh")
	// The pixel font of the Mac OS 9 skin, as the app registers it.
	_ = registerSkinFonts()
	os.Exit(m.Run())
}

// shotDir is where the page screenshots are written: $SCRIPTBOARD_SHOTS, or
// ./screenshots.
func shotDir() string {
	if d := os.Getenv("SCRIPTBOARD_SHOTS"); d != "" {
		return d
	}
	return "../screenshots"
}

// newTestApp opens a fresh store in a temporary folder, offline, so the pages
// show their sample data without any network request.
func newTestApp(t *testing.T) *app {
	t.Helper()
	a, err := newApp(t.TempDir(), true)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// settle draws frames until page transitions and animations are done, as a
// user would see the page once it is still.
func settle(tt *ui.Tester) {
	tt.Frame()
	for i := 0; i < 60 && tt.FramePending(); i++ {
		tt.Advance(50 * time.Millisecond)
		tt.Frame()
	}
}

// click settles the page, clicks label, and settles again.
func click(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	settle(tt)
	if err := tt.Click(label); err != nil {
		t.Fatalf("click %q: %v", label, err)
	}
	settle(tt)
}

// savePNG writes the tester's last frame to the screenshot folder.
func savePNG(t *testing.T, tt *ui.Tester, name string) {
	t.Helper()
	settle(tt)
	dir := shotDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

func requireText(t *testing.T, tt *ui.Tester, want string) {
	t.Helper()
	if !tt.HasText(want) {
		t.Errorf("no %q on the page; texts: %s", want, strings.Join(tt.Texts(), " | "))
	}
}

// requireTextStart is requireText for a title that may be cut short with an
// ellipsis on a narrow card.
func requireTextStart(t *testing.T, tt *ui.Tester, prefix string) {
	t.Helper()
	for _, s := range tt.Texts() {
		if strings.HasPrefix(s, prefix) {
			return
		}
	}
	t.Errorf("no text starting %q on the page; texts: %s", prefix, strings.Join(tt.Texts(), " | "))
}

func TestHomePage(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 960, 640)
	settle(tt)
	requireText(t, tt, "首页")
	requireTextStart(t, tt, "USD / CNY") // fx script, sample data offline; the title gives way to the buttons on a narrow card
	requireText(t, tt, "金价 · 元/克")
	savePNG(t, tt, "1-home-medium")

	click(t, tt, "大")
	savePNG(t, tt, "1-home-large")

	click(t, tt, "小")
	savePNG(t, tt, "1-home-small")
}

func TestEditorPage(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 960, 900)
	click(t, tt, "编辑")
	requireText(t, tt, "体重 · kg") // the first script opens in the editor
	savePNG(t, tt, "2-editor")

	before := len(a.store.Series("weight"))
	click(t, tt, "记录")
	if got := len(a.store.Series("weight")); got != before+1 {
		t.Errorf("weight records %d after 记录, want %d", got, before+1)
	}
	requireText(t, tt, "已记录 体重")

	click(t, tt, "保存")
	requireText(t, tt, "已保存")
	savePNG(t, tt, "2-editor-saved")
}

func TestSettingsPage(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 960, 640)
	click(t, tt, "设置")
	requireText(t, tt, "不请求网络，用缓存的数据，没有缓存就显示示例")
	savePNG(t, tt, "3-settings")

	click(t, tt, "斤")
	if a.set.WeightUnit != "斤" {
		t.Errorf("weight unit %q after choosing 斤", a.set.WeightUnit)
	}
	if got := a.store.Settings().WeightUnit; got != "斤" {
		t.Errorf("stored weight unit %q, want 斤", got)
	}

	// The unit shows on the home card.
	click(t, tt, "首页")
	requireText(t, tt, "体重 · 斤")
}

func TestRefreshBuiltin(t *testing.T) {
	if !refreshBuiltin("gold", "export function meta(){return {name:'x',sources:{}}}") {
		t.Error("old gold should refresh")
	}
	if refreshBuiltin("gold", `https://tmini.net/api/gold-price?type=json`) {
		t.Error("live gold should stay")
	}
	if !refreshBuiltin("goldk", "示例数据：还没有金价录入") {
		t.Error("old goldk should refresh")
	}
	if refreshBuiltin("goldk", "没有录入时不造数") {
		t.Error("new goldk should stay")
	}
	if refreshBuiltin("fx", "anything") {
		t.Error("other builtins stay")
	}
}
