package plugins

import (
	"strings"
	"testing"
)

const src = `export function meta() { return { name: "天气", icon: "cloud-sun", sources: {} }; }
export function render(ctx) { return { title: "t", big: "1", sub: "", lines: [], chart: null }; }`

func TestSaveListDeleteAndFingerprint(t *testing.T) {
	dir := t.TempDir()
	before := Fingerprint(dir)
	if err := Save(dir, Manifest{ID: "weather-sh", Name: "上海天气", Icon: "cloud-sun", Home: true}, src); err != nil {
		t.Fatal(err)
	}
	if Fingerprint(dir) == before {
		t.Error("the fingerprint did not change after a save")
	}
	list, err := List(dir)
	if err != nil || len(list) != 1 || list[0].Manifest.ID != "weather-sh" || !strings.Contains(list[0].Source, "render") {
		t.Fatalf("list %+v, %v", list, err)
	}
	if err := Delete(dir, "weather-sh"); err != nil {
		t.Fatal(err)
	}
	if list, _ := List(dir); len(list) != 0 {
		t.Errorf("%d plugins after delete", len(list))
	}
}

func TestSaveRefusesBadInput(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]struct {
		m   Manifest
		src string
	}{
		"bad id":        {Manifest{ID: "Bad_ID", Name: "x"}, src},
		"empty name":    {Manifest{ID: "ok-one", Name: " "}, src},
		"bad icon":      {Manifest{ID: "ok-one", Name: "x", Icon: "../etc"}, src},
		"no render":     {Manifest{ID: "ok-one", Name: "x"}, "export function meta(){ return {name:'x'} }"},
		"not compiling": {Manifest{ID: "ok-one", Name: "x"}, "export const ("},
	}
	for name, c := range cases {
		if err := Save(dir, c.m, c.src); err == nil {
			t.Errorf("%s: saved", name)
		}
	}
}

func TestValidateSVG(t *testing.T) {
	good := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="10"/></svg>`
	if err := ValidateSVG([]byte(good)); err != nil {
		t.Errorf("good SVG refused: %v", err)
	}
	for _, bad := range []string{
		`<html></html>`,
		`<svg><script>x</script></svg>`,
		`<svg onload="x()"><circle r="1"/></svg>`,
		`<svg><a href="javascript:alert(1)"><circle r="1"/></a></svg>`,
		`<svg><foreignObject></foreignObject></svg>`,
		``,
	} {
		if err := ValidateSVG([]byte(bad)); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestIconLookupFromPlugins(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, Manifest{ID: "weather-sh", Name: "x"}, src); err != nil {
		t.Fatal(err)
	}
	svg := []byte(`<svg viewBox="0 0 24 24"><circle r="3"/></svg>`)
	if _, err := AddIcon(dir, "weather-sh", "sun-dot", svg); err != nil {
		t.Fatal(err)
	}
	if _, err := AddIcon(dir, "", "shared-dot", svg); err != nil {
		t.Fatal(err)
	}
	names := strings.Join(ListIcons(dir), ",")
	if !strings.Contains(names, "sun-dot (weather-sh)") || !strings.Contains(names, "shared-dot") {
		t.Errorf("ListIcons = %s", names)
	}
}

// A plugin names its home position (1 to 200) and its default size, and both
// come back from the list; a position or size outside those is refused.
func TestSavePositionAndSize(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, Manifest{ID: "ok-one", Name: "x", Position: 2, Size: "small"}, src); err != nil {
		t.Fatal(err)
	}
	list, err := List(dir)
	if err != nil || len(list) != 1 || list[0].Manifest.Position != 2 || list[0].Manifest.Size != "small" {
		t.Fatalf("list %+v, %v", list, err)
	}
	bad := map[string]Manifest{
		"negative position": {ID: "ok-two", Name: "x", Position: -1},
		"position too big":  {ID: "ok-two", Name: "x", Position: 201},
		"unknown size":      {ID: "ok-two", Name: "x", Size: "huge"},
	}
	for name, m := range bad {
		if err := Save(dir, m, src); err == nil {
			t.Errorf("%s: saved", name)
		}
	}
}
