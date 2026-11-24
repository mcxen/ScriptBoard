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

// brandDefault is the app's own look: Lobe's tokens with a calm blue primary
// and cool neutrals, so that the surfaces, lines and text step down evenly:
// canvas, card, hairline; text, secondary, tertiary.
var brandDefault = func() *theme.Config {
	light, dark := *theme.Light(), *theme.Dark()
	// Cool-neutral Apple-native palette harmonized with Weather .bar tokens.
	light.Primary, light.PrimaryHover, light.PrimaryActive = ui.Hex("#2563EB"), ui.Hex("#1D4FD8"), ui.Hex("#1E40AF")
	dark.Primary, dark.PrimaryHover, dark.PrimaryActive = ui.Hex("#5B8CFF"), ui.Hex("#7BA3FF"), ui.Hex("#4A78E8")
	light.PrimaryForeground, dark.PrimaryForeground = light.TextLightSolid, dark.TextLightSolid

	light.BgLayout, light.BgContainer, light.BgElevated = ui.Hex("#F5F6F9"), ui.Hex("#FFFFFF"), ui.Hex("#FFFFFF")
	light.BorderSecondary, light.Border = ui.RGBA(15, 23, 42, 0.075), ui.RGBA(15, 23, 42, 0.12)
	light.Text, light.TextSecondary, light.TextTertiary = ui.Hex("#141821"), ui.Hex("#646C7E"), ui.Hex("#7E869A")
	light.FillTertiary, light.FillQuaternary = ui.RGBA(15, 23, 42, 0.065), ui.RGBA(15, 23, 42, 0.045)
	light.Success, light.Warning, light.Error = ui.Hex("#10B981"), ui.Hex("#D97706"), ui.Hex("#EF4444")

	dark.BgLayout, dark.BgContainer, dark.BgElevated = ui.Hex("#111318"), ui.Hex("#1A1D24"), ui.Hex("#222631")
	dark.BorderSecondary, dark.Border = ui.RGBA(255, 255, 255, 0.08), ui.RGBA(255, 255, 255, 0.12)
	dark.Text, dark.TextSecondary, dark.TextTertiary = ui.Hex("#F2F4F8"), ui.Hex("#9BA3B4"), ui.Hex("#687082")
	dark.FillTertiary, dark.FillQuaternary = ui.RGBA(255, 255, 255, 0.085), ui.RGBA(255, 255, 255, 0.055)
	dark.Success, dark.Warning, dark.Error = ui.Hex("#34D399"), ui.Hex("#FBBF24"), ui.Hex("#F87171")
	return theme.NewConfig(light, dark)
}()

// brandMac9 is the Mac OS 9 look. It is a light look: the dark appearance uses
// the same colors, so a dark system does not turn the classic Mac dark.
var brandMac9 = func() *theme.Config {
	// The colors of AvaMedia's Platinum skin (PlatinumTokens.axaml): a gray
	// desk, light panels, a slate accent, and text that is near black.
	t := *theme.Light()
	t.BgLayout = ui.Hex("#CCCCCC")    // the desk behind the windows
	t.BgContainer = ui.Hex("#EEEEEE") // windows, cards and panels
	t.BgElevated = ui.Hex("#FFFFFF")  // fields, and the menus
	t.BorderSecondary = ui.Hex("#888888")
	t.Border = ui.Hex("#555555")
	t.Primary = ui.Hex("#3D4E80") // the slate accent
	t.PrimaryHover = ui.Hex("#34446E")
	t.PrimaryActive = ui.Hex("#2B3A5F")
	t.PrimaryForeground = ui.Hex("#FFFFFF")
	t.Text = ui.Hex("#111111")
	t.TextSecondary = ui.Hex("#444444")
	t.TextTertiary = ui.Hex("#777777")
	t.FillTertiary = ui.Hex("#DDDDDD")
	t.FillQuaternary = ui.Hex("#EEEEEE")
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

// Appearances: the system's, or one of the two forced.
const (
	themeSystem = "system"
	themeLight  = "light"
	themeDark   = "dark"
)

// themeModes are the appearance choices, in the order of the settings row.
var themeModes = []string{themeSystem, themeLight, themeDark}

var themeMode atomic.Value

func init() { themeMode.Store(themeSystem) }

// curThemeMode is the appearance in use.
func curThemeMode() string { return themeMode.Load().(string) }

// setThemeMode picks an appearance. An unknown name follows the system.
func setThemeMode(name string) {
	switch name {
	case themeLight, themeDark:
		themeMode.Store(name)
	default:
		themeMode.Store(themeSystem)
	}
}

// applySkin makes the frame use the skin's theme in the appearance chosen:
// the system's unless the user forced light or dark.
func applySkin(c *ui.Context) {
	setTypeScale(isMac())
	cfg := brandFor(curSkin())
	switch curThemeMode() {
	case themeLight:
		c.SetTheme(cfg.Theme(false))
	case themeDark:
		c.SetTheme(cfg.Theme(true))
	default:
		cfg.Apply(c)
	}
}

// skinFont is the font family of the skin's text, as Element.Font takes it;
// "" for the default skin, which keeps the system font.
func skinFont() string {
	if curSkin() == skinMac9 {
		// The bundled pixel face is Platinum's own (Fusion Pixel 12px, OFL);
		// Geneva and the system's face follow if it is not registered.
		return "Fusion Pixel 12px Prop zh-Hans, Geneva, system-ui"
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
	if c.Vibrancy() {
		return el
	}
	return el.Background(pal(c).sidebar)
}

// skinCard is a card of the skin: a quiet Apple widget surface by default, a
// square window panel with a dark 1 px edge in Mac OS 9.
func skinCard(c *ui.Context) ui.Element {
	if isMac() {
		return macBevel(layout.Block(c, layout.BlockProps{Variant: layout.BlockFilled}).Radius(0).Background(macPanel), false)
	}
	p := pal(c)
	sh := ui.RGBA(15, 23, 42, 0.05)
	if p.dark {
		sh = ui.RGBA(0, 0, 0, 0.24)
	}
	return ui.Column(c).Radius(radCard).Background(p.card).
		BorderWidth(1).BorderColor(p.border).Shadow(0, 1, 3, 0, sh)
}

// setTypeScale sets the type sizes of the skin. The default has the app's
// scale. Mac OS 9 uses its 12 DIP pixel grid for all text, and 24 for the
// counts: the pixel font has no other sizes that stay sharp.
func setTypeScale(mac bool) {
	if mac {
		fsTitle, fsHeading, fsBody, fsSmall, fsTiny = 12, 12, 12, 12, 12
		fsCaption, fsLabel, fsSubtitle, fsEmphasis, fsBodyLarge, fsCount = 12, 12, 12, 12, 12, 24
		return
	}
	fsTitle, fsHeading, fsBody, fsSmall, fsTiny = 22, 15, 13.5, 12, 11
	fsCaption, fsLabel, fsSubtitle, fsEmphasis, fsBodyLarge, fsCount = 12, 12, 18, 15, 14, 20
}

func init() { setTypeScale(false) }
