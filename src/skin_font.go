package main

import (
	_ "embed"

	"github.com/egoist/mygo/ui"
)

// The pixel face of the Mac OS 9 skin: Fusion Pixel 12px, by TakWolf, under
// the SIL Open Font License 1.1 (src/assets/fonts). It has a 12 DIP design
// grid, so the Platinum text is drawn at its own size and stays sharp.
//
//go:embed assets/fonts/fusion-pixel-12px-proportional-zh_hans.ttf
var platinumFont []byte

// registerSkinFonts makes the bundled faces available to Font(family). It is
// called once, before the window opens.
func registerSkinFonts() error {
	return ui.RegisterFont(platinumFont, "")
}
