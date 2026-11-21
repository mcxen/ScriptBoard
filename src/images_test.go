package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

// pngBytes is a small picture to serve and keep.
func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	img.Set(1, 1, color.RGBA{R: 200, G: 90, B: 40, A: 255})
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// A script's pictures are downloaded once, kept, and listed newest first.
func TestPicturesAreKeptAndHistoried(t *testing.T) {
	data := pngBytes(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(data)
	}))
	defer srv.Close()
	a := newTestApp(t)
	sc := store.Script{ID: "pics", Name: "图片", Home: true, Origin: store.OriginUser}
	run := func(url string) engine.Result {
		t.Helper()
		src := fmt.Sprintf(`export function meta() { return { name: "图片", sources: {} }; }
export function render(ctx) { return { title: "图片", big: "1", sub: "", lines: [], chart: null, images: [{ url: %q, caption: %q }] }; }`, url, url)
		sc.Source = src
		a.store.SaveScript(sc)
		res, err := a.execute(sc, nil)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	first := run(srv.URL + "/a.png")
	if len(first.Images) != 1 || first.Images[0].Local == "" {
		t.Fatalf("the picture was not kept: %+v", first.Images)
	}
	if _, err := os.Stat(first.Images[0].Local); err != nil {
		t.Fatalf("the copy is missing: %v", err)
	}
	second := run(srv.URL + "/b.png")
	if len(second.Images) != 2 || second.Images[0].URL != srv.URL+"/b.png" || second.Images[1].URL != srv.URL+"/a.png" {
		t.Errorf("history %+v, want the newest first", second.Images)
	}
}

// The history keeps at most imageHistoryMax pictures; the ones cut off lose
// their copies.
func TestHistoryIsCapped(t *testing.T) {
	a := newTestApp(t)
	sc := store.Script{ID: "capped", Name: "x", Origin: store.OriginUser}
	var fresh []engine.Image
	for i := 0; i < imageHistoryMax+5; i++ {
		fresh = append(fresh, engine.Image{URL: fmt.Sprintf("http://127.0.0.1:1/%d.png", i)})
	}
	list := a.keepImages(sc, fresh, time.Now())
	if len(list) != imageHistoryMax {
		t.Errorf("history has %d pictures, want %d", len(list), imageHistoryMax)
	}
}

// Clearing the pictures forgets the history and the copies.
func TestClearImagesForgetsThem(t *testing.T) {
	a := newTestApp(t)
	sc := store.Script{ID: "clear", Name: "x", Origin: store.OriginUser}
	if _, err := a.cache.PutBlob(sc.ID, "http://x/y.png", pngBytes(t)); err != nil {
		t.Fatal(err)
	}
	a.keepImages(sc, []engine.Image{{URL: "http://x/y.png", Caption: "y"}}, time.Now())
	a.clearImages(sc)
	if got := a.imageHistory(sc.ID); len(got) != 0 {
		t.Errorf("history kept after clearing: %+v", got)
	}
	if _, ok := a.cache.BlobPath(sc.ID, "http://x/y.png"); ok {
		t.Error("the copy was kept after clearing")
	}
}

// The full view shows the position of the carousel, and its arrows move it.
func TestCarouselCountsItsPictures(t *testing.T) {
	a := newTestApp(t)
	sc := store.Script{ID: "carousel", Name: "轮播", Home: true, Origin: store.OriginUser,
		Source: `export function meta() { return { name: "轮播", sources: {} }; }
export function render(ctx) { return { title: "轮播", big: "2", sub: "", lines: [], chart: null, images: [] }; }`}
	a.store.SaveScript(sc)
	for _, u := range []string{"http://x/1.png", "http://x/2.png"} {
		if _, err := a.cache.PutBlob(sc.ID, u, pngBytes(t)); err != nil {
			t.Fatal(err)
		}
	}
	a.keepImages(sc, []engine.Image{{URL: "http://x/1.png", Caption: "one"}, {URL: "http://x/2.png", Caption: "two"}}, time.Now())
	tt := ui.NewTester(a.view, 1200, 1400)
	a.router.Push("/full/carousel")
	settle(tt)
	settle(tt)
	requireText(t, tt, "1 / 2")
	requireText(t, tt, "保存图片")
	click(t, tt, "›")
	requireText(t, tt, "2 / 2")
	savePNG(t, tt, "28-bing-gallery-full")
}

// Saving an image chooses a safe file name with the detected format and copies
// the original full-resolution bytes intact.
func TestSaveImageFilenameAndCopy(t *testing.T) {
	data := pngBytes(t)
	gotName := imageFilename(engine.Image{URL: "https://bing.com/x.jpg", Caption: "2026-11-21 · 阿尔卑斯/薄雾"}, data)
	if gotName != "2026-11-21-阿尔卑斯-薄雾.png" {
		t.Errorf("filename %q, want 2026-11-21-阿尔卑斯-薄雾.png", gotName)
	}
	dir := t.TempDir()
	src := dir + "/cached.bin"
	dst := dir + "/saved.png"
	if err := os.WriteFile(src, data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeImageCopy(src, dst); err != nil {
		t.Fatal(err)
	}
	copied, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(copied, data) {
		t.Errorf("copied image bytes differ (%d vs %d)", len(copied), len(data))
	}
}
