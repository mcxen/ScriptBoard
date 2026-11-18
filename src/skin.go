package main

import (
	"sync/atomic"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/layout"
	"github.com/lobehub/lobe-mygo/pkg/theme"
)

// Skins change the whole look of the app: colors, radius, fonts and the
// shape of cards and controls. The default skin is the app's own (Lobe's
// tokens with a blue primary); Mac OS 9 is the gray-white, square, pixel-edged
// look of the classic Mac. The skin is global: every page follows it at once.

const (
	skinDefault = ""     // the app's own look
	skinMac9    = "mac9" // the classic Mac OS 9 look
)

var skinNames = []string{skinDefault, skinMac9}

// skin is the skin in use, set from the settings and changed on the settings
// page.
var skin atomic.Value

func init() { skin.Store(skinDefault) }

// curSkin is the skin in use.
func curSkin() string { return skin.Load().(string) }

// setSkin picks a skin. An unknown name is the default.
func setSkin(name string) {
	for _, s := range skinNames {
		if s == name {
			skin.Store(name)
			return
		}
	}
	skin.Store(skinDefault)
}

// brandDefault is the app's own look: Lobe's tokens with a blue primary.
var brandDefault = func() *theme.Config {
	light, dark := *theme.Light(), *theme.Dark()
	// The whole primary family is set together, as Lobe's docs advise.
	light.Primary, light.PrimaryHover, light.PrimaryActive = ui.Hex("#0559FF"), ui.Hex("#044EE0"), ui.Hex("#0343C2")
	dark.Primary, dark.PrimaryHover, dark.PrimaryActive = ui.Hex("#3D7BFF"), ui.Hex("#5C8FFF"), ui.Hex("#2F66E0")
	light.PrimaryForeground, dark.PrimaryForeground = light.TextLightSolid, dark.TextLightSolid
	return theme.NewConfig(light, dark)
}()

// brandMac9 is the Mac OS 9 look. It is a light look: the dark appearance uses
// the same colors, so a dark system does not turn the classic Mac dark.
var brandMac9 = func() *theme.Config {
	t := *theme.Light()
	t.BgLayout = ui.Hex("#E2E2E2")    // the desk behind the windows
	t.BgContainer = ui.Hex("#F4F4F4") // windows, cards and panels
	t.BgElevated = ui.Hex("#FFFFFF")  // menus and the field under a cursor
	t.BorderSecondary = ui.Hex("#8C8C8C")
	t.Border = ui.Hex("#6E6E6E")
	t.Primary = ui.Hex("#2B5FCC") // the highlight blue of the Platinum look
	t.PrimaryHover = ui.Hex("#2250B0")
	t.PrimaryActive = ui.Hex("#1B4290")
	t.PrimaryForeground = ui.Hex("#FFFFFF")
	t.Text = ui.Hex("#000000")
	t.TextSecondary = ui.Hex("#3A3A3A")
	t.TextTertiary = ui.Hex("#5C5C5C")
	t.FillTertiary = ui.Hex("#D6D6D6")
	t.FillQuaternary = ui.Hex("#CCCCCC")
	// Square corners everywhere: controls, cards and tags.
	t.BorderRadius, t.BorderRadiusLG, t.BorderRadiusSM, t.BorderRadiusXS = 0, 0, 0, 0
	dark := t
	return theme.NewConfig(t, dark)
}()

// brandFor is the Lobe config of a skin.
func brandFor(name string) *theme.Config {
	if name == skinMac9 {
		return brandMac9
	}
	return brandDefault
}

// applySkin makes the frame use the skin's theme, and sets the page's font
// and background when the skin has them.
func applySkin(c *ui.Context) {
	brandFor(curSkin()).Apply(c)
}

// skinFont is the font family of the skin's text, as Element.Font takes it;
// "" for the default skin, which keeps the system font.
func skinFont() string {
	if curSkin() == skinMac9 {
		// Geneva is the classic Mac's text face; a system without it (Android)
		// falls back to its own.
		return "Geneva, system-ui"
	}
	return ""
}

// skinBackground paints the page's background: a gray-white gradient for the
// Mac OS 9 skin, the layout color otherwise.
func skinBackground(c *ui.Context, el ui.Element) ui.Element {
	// The font is inherited by the text under it.
	if f := skinFont(); f != "" {
		el = el.Font(f)
	}
	if isMac() {
		return macDesktop(c, el)
	}
	return el.Background(pal(c).canvas)
}

// skinCard is a card of the skin: a soft block by default, a square window
// panel with a dark 1 px edge in Mac OS 9.
func skinCard(c *ui.Context) ui.Element {
	if isMac() {
		return macBevel(layout.Block(c, layout.BlockProps{Variant: layout.BlockFilled}).Radius(0).Background(macPanel), false)
	}
	return layout.Block(c, layout.BlockProps{Variant: layout.BlockOutlined, Shadow: true})
}
