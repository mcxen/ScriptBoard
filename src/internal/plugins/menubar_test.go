package plugins

import "testing"

const menubarSrc = `export function meta() { return { name: "天气", menubar: true, sources: {} }; }
export function render(ctx) { return { title: "t", big: "1", sub: "", lines: [], chart: null }; }`

// A plugin may ask to be put in the menu bar when it first appears, if its
// script says it can be there.
func TestSaveMenubar(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, Manifest{ID: "no-bar", Name: "x", Menubar: true}, src); err == nil {
		t.Error("saved a menubar plugin whose script does not say menubar: true")
	}
	if err := Save(dir, Manifest{ID: "with-bar", Name: "x", Menubar: true}, menubarSrc); err != nil {
		t.Fatal(err)
	}
	list, err := List(dir)
	if err != nil || len(list) != 1 || !list[0].Manifest.Menubar {
		t.Fatalf("list %+v, %v", list, err)
	}
}
