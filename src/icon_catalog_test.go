package main

import (
	"testing"

	"scriptboard/src/internal/iconcat"
)

// Every icon the catalog names has its SVG, so a script that picks one from the
// catalog always gets its picture.
func TestCatalogIconsExist(t *testing.T) {
	for _, name := range iconcat.Names() {
		if _, err := iconFS.ReadFile("icons/" + name + ".svg"); err != nil {
			t.Errorf("catalog icon %q has no SVG", name)
		}
	}
}
