package main

import (
	"embed"
	"os"
	"path/filepath"
	"regexp"

	"github.com/egoist/mygo/ui"
)

// The icons are Lucide (ISC license, see icons/LICENSE-lucide.txt). Scripts
// name one in the "icon" field of their result; an unknown name falls back to
// a default.

//go:embed icons/*.svg
var iconFS embed.FS

const defaultIcon = "chart-line"

var iconCache = map[string]*ui.SVG{}

// userIconDirs are the folders of icons that come with plugins or were added
// by the user. They are read at run time, so a new icon needs no rebuild.
var userIconDirs []string

// setIconDirs sets the folders icons are looked up in and forgets the icons
// already read.
func setIconDirs(dirs []string) {
	userIconDirs = dirs
	iconCache = map[string]*ui.SVG{}
}

// validIconName keeps an icon name to letters, digits and hyphens, so that a
// name can never reach outside its folder.
var validIconName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)

// icon returns the SVG named name: a built-in one, then one from the plugin
// or user folders, else the default. It is only called on the main thread.
func icon(name string) *ui.SVG {
	if s, ok := iconCache[name]; ok {
		return s
	}
	if validIconName.MatchString(name) {
		if s := loadIcon(name); s != nil {
			iconCache[name] = s
			return s
		}
	}
	if name == defaultIcon {
		panic("missing default icon")
	}
	return icon(defaultIcon)
}

func loadIcon(name string) *ui.SVG {
	if data, err := iconFS.ReadFile("icons/" + name + ".svg"); err == nil {
		return ui.MustParseSVG(data)
	}
	for _, dir := range userIconDirs {
		data, err := os.ReadFile(filepath.Join(dir, name+".svg"))
		if err != nil {
			continue
		}
		if s, err := ui.ParseSVG(data); err == nil {
			return s
		}
	}
	return nil
}
