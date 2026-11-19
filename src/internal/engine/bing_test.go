package engine

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// The Bing script reads two archive pages of eight through paging (idx 0, 8),
// then de-duplicates the pictures, and gives each its picture, a dated caption
// and its Bing page.
func TestBingScriptReadsPagedArchive(t *testing.T) {
	var src string
	for _, b := range Builtins() {
		if b.ID == "bing" {
			src = b.Source
		}
	}
	s, err := Compile("bing.js", src)
	if err != nil {
		t.Fatal(err)
	}
	p := s.Meta.Sources["bing"].Paging
	if p == nil || p.Param != "idx" || p.Step != 8 || p.Pages != 2 || p.Items != "images" {
		t.Fatalf("paging %+v", p)
	}
	archive := `[
	 {"urlbase":"/th?id=OHR.Hill_ZH-CN1_UHD","startdate":"20261119","copyright":"A hill (© Someone)","copyrightlink":"/search?q=hill"},
	 {"urlbase":"/th?id=OHR.Hill_ZH-CN1_UHD","startdate":"20261119","copyright":"A hill (© Someone)"},
	 {"urlbase":"/th?id=OHR.Lake_EN-US2","startdate":"20261118","copyright":"A lake"}
	]`
	var sources map[string]json.RawMessage = map[string]json.RawMessage{"bing": json.RawMessage(archive)}
	res, err := s.Run(context.Background(), Input{Sources: sources, Lang: "zh"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Images) != 2 {
		t.Fatalf("%d pictures, want 2 (the duplicate dropped): %+v", len(res.Images), res.Images)
	}
	first := res.Images[0]
	if first.URL != "https://www.bing.com/th?id=OHR.Hill_ZH-CN1_UHD_1920x1080.jpg" {
		t.Errorf("url %q", first.URL)
	}
	if !strings.Contains(first.Caption, "2026-11-19") || !strings.Contains(first.Caption, "A hill") || strings.Contains(first.Caption, "Someone") {
		t.Errorf("caption %q: the date and the title, without the credit", first.Caption)
	}
	if first.Source != "https://www.bing.com/search?q=hill" {
		t.Errorf("source %q", first.Source)
	}
	if res.Images[1].Source != "" {
		t.Errorf("a picture without a page has source %q", res.Images[1].Source)
	}
}
