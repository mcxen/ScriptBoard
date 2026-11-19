package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

// redPicture writes a solid red picture, and returns its path.
func redPicture(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}
	path := filepath.Join(t.TempDir(), "red.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return path
}

// rgbAt is the 8-bit color of the tester's last frame at (x, y).
func rgbAt(tt *ui.Tester, x, y float32) (r, g, b uint32) {
	r, g, b, _ = tt.Image().At(int(x), int(y)).RGBA()
	return r >> 8, g >> 8, b >> 8
}

// showOnly keeps one script on the home page.
func showOnly(a *app, id string) {
	for _, sc := range a.store.Scripts() {
		sc.Home = sc.ID == id
		a.store.SaveScript(sc)
	}
}

// A card with a picture is the picture, edge to edge: at rest nothing but the
// picture shows, and while the pointer is on the card its text and buttons show
// in a panel over it.
func TestImageCardIsThePictureAndShowsItsTextOnHover(t *testing.T) {
	a := newTestApp(t)
	sc := store.Script{ID: "pic", Name: "图", Home: true, Origin: store.OriginUser,
		Source: `export function meta() { return { name: "图", sources: { api: "https://example.com/x" } }; }
export function render(ctx) { return { title: "图", big: "8", sub: "", lines: [], chart: null }; }`}
	a.store.SaveScript(sc)
	showOnly(a, sc.ID)
	meta, _ := a.metaOf(sc)
	_, vals := a.viewOf(sc.ID, meta.Dims)
	key := a.runKeyOf(sc.ID, sc, meta, vals)
	a.runs[key] = &runState{res: engine.Result{Title: "封面", Big: "8", Sub: "今日",
		Images: []engine.Image{{URL: "http://x/1.png", Caption: "cap-xyz", Local: redPicture(t)}}}}

	tt := ui.NewTester(a.view, 1200, 900)
	settle(tt)
	settle(tt)
	info, ok := tt.Access("cap-xyz")
	if !ok {
		t.Fatal("the caption is not in the card (it is built, though hidden)")
	}
	// In the panel's padding, left of the caption; and in the picture, above it.
	panelX, panelY := info.Bounds.X-4, info.Bounds.Y+info.Bounds.H/2
	pictureX, pictureY := info.Bounds.X, info.Bounds.Y-90

	r, g, _ := rgbAt(tt, panelX, panelY)
	if r < 200 || g > 60 {
		t.Errorf("at rest the card shows (%d, %d), want the picture (red)", r, g)
	}
	savePNG(t, tt, "cover-rest")

	hover(t, tt, "cap-xyz")
	_, g, _ = rgbAt(tt, panelX, panelY)
	if g < 150 {
		t.Errorf("hovered, the panel's green is %d, want a light panel over the picture", g)
	}
	r, g, _ = rgbAt(tt, pictureX, pictureY)
	if r < 200 || g > 60 {
		t.Errorf("hovered, the rest of the card shows (%d, %d), want the picture still (red)", r, g)
	}
	savePNG(t, tt, "cover-hover")
}

// A script that wants its picture among the card's other contents says layout
// image: "inline"; a picture that cannot be decoded is not a cover either.
func TestCoverIsNotUsedWhenNotAllowedOrNotDecodable(t *testing.T) {
	a := newTestApp(t)
	sc := store.Script{ID: "pic2"}
	res := engine.Result{Images: []engine.Image{{URL: "http://x/1.png", Local: redPicture(t)}}}
	if a.coverOf(sc, res, true) == nil {
		t.Error("a decodable picture is not a cover")
	}
	if a.coverOf(sc, res, false) != nil {
		t.Error("a cover where it is not allowed")
	}
	bad := engine.Result{Images: []engine.Image{{URL: "http://x/2.png", Local: filepath.Join(t.TempDir(), "missing.png")}}}
	if a.coverOf(sc, bad, true) != nil {
		t.Error("a picture that cannot be decoded is a cover")
	}
}

// inkIn counts the dark pixels of the tester's last frame inside r.
func inkIn(tt *ui.Tester, x, y, w, h float32) int {
	n := 0
	for py := int(y); py < int(y+h); py++ {
		for px := int(x); px < int(x+w); px++ {
			r, g, b := rgbAt(tt, float32(px), float32(py))
			if r+g+b < 3*150 {
				n++
			}
		}
	}
	return n
}

// The buttons of a card without pictures show while the pointer is on it, and
// are not there to see, or to click, at rest.
func TestCardButtonsShowOnlyOnHover(t *testing.T) {
	a := newTestApp(t)
	sc := store.Script{ID: "plain-card", Name: "普通", Home: true, Origin: store.OriginUser,
		Source: `export function meta() { return { name: "普通", sources: { api: "https://example.com/x" } }; }
export function render(ctx) { return { title: "普通", big: "5", sub: "", lines: [], chart: null }; }`}
	a.store.SaveScript(sc)
	showOnly(a, sc.ID)
	meta, _ := a.metaOf(sc)
	_, vals := a.viewOf(sc.ID, meta.Dims)
	a.runs[a.runKeyOf(sc.ID, sc, meta, vals)] = &runState{res: engine.Result{Title: "普通", Big: "5"}}

	tt := ui.NewTester(a.view, 1200, 900)
	settle(tt)
	settle(tt)
	info, ok := tt.Access("刷新数据")
	if !ok {
		t.Fatal("the refresh button is not in the card (it is built, though hidden)")
	}
	b := info.Bounds
	if n := inkIn(tt, b.X, b.Y, b.W, b.H); n != 0 {
		t.Errorf("at rest the refresh button paints %d dark pixels, want none", n)
	}
	hover(t, tt, "刷新数据")
	if n := inkIn(tt, b.X, b.Y, b.W, b.H); n == 0 {
		t.Error("hovered, the refresh button paints nothing")
	}
}

// A script that asks for its buttons to stay says layout actions: "always".
func TestCardButtonsStayWhenTheScriptAsks(t *testing.T) {
	a := newTestApp(t)
	sc := store.Script{ID: "always-card", Name: "常显", Home: true, Origin: store.OriginUser,
		Source: `export function meta() { return { name: "常显", sources: { api: "https://example.com/x" } }; }
export function render(ctx) { return { title: "常显", big: "5", sub: "", lines: [], chart: null, layout: { actions: "always" } }; }`}
	a.store.SaveScript(sc)
	showOnly(a, sc.ID)
	meta, _ := a.metaOf(sc)
	_, vals := a.viewOf(sc.ID, meta.Dims)
	a.runs[a.runKeyOf(sc.ID, sc, meta, vals)] = &runState{res: engine.Result{Title: "常显", Big: "5", Layout: &engine.Layout{Actions: "always"}}}

	tt := ui.NewTester(a.view, 1200, 900)
	settle(tt)
	settle(tt)
	info, ok := tt.Access("刷新数据")
	if !ok {
		t.Fatal("no refresh button")
	}
	b := info.Bounds
	if n := inkIn(tt, b.X, b.Y, b.W, b.H); n == 0 {
		t.Error("the script asked for its buttons to stay, and the refresh button paints nothing")
	}
}

// A card is its content: its name is not there to see at rest, and shows with
// the buttons while the pointer is on the card. A script that wants the title
// row says layout title: "show".
func TestCardTitleShowsOnlyOnHoverUnlessAsked(t *testing.T) {
	a := newTestApp(t)
	mk := func(id, title, layout string) {
		a.store.SaveScript(store.Script{ID: id, Name: title, Home: true, Origin: store.OriginUser,
			Source: `export function meta() { return { name: "` + title + `", sources: {} }; }
export function render(ctx) { return { title: "` + title + `", big: "5", sub: "", lines: [], chart: null` + layout + ` }; }`})
	}
	mk("tt-hidden", "标题隐藏", "")
	mk("tt-shown", "标题常显", `, layout: { title: "show" }`)
	for _, sc := range a.store.Scripts() {
		sc.Home = sc.ID == "tt-hidden" || sc.ID == "tt-shown"
		a.store.SaveScript(sc)
	}
	tt := ui.NewTester(a.view, 1200, 600)
	settle(tt)
	settle(tt)
	ink := func(title string) int {
		info, ok := tt.Access(title)
		if !ok {
			t.Fatalf("no %q in the card (it is built, though hidden)", title)
		}
		return inkIn(tt, info.Bounds.X, info.Bounds.Y, info.Bounds.W, info.Bounds.H)
	}
	if n := ink("标题隐藏"); n != 0 {
		t.Errorf("at rest the hidden title paints %d dark pixels, want none", n)
	}
	if n := ink("标题常显"); n == 0 {
		t.Error("the script asked for its title row, and it paints nothing")
	}
	hover(t, tt, "标题隐藏")
	if n := ink("标题隐藏"); n == 0 {
		t.Error("hovered, the name paints nothing")
	}
}
