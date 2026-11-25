package main

import (
	"math"
	"strconv"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/choice"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	lfield "github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/layout"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"scriptboard/src/internal/barui"
	"scriptboard/src/internal/engine"
)

// The look of the app. One palette per appearance, one theme per palette, and
// a few building blocks that every page uses, so that spacing, radius, type
// sizes and colors are decided in one place.

// Spacing and type scale, in DIPs.
const (
	padPage    = 24 // padding around a page on a wide window
	padPhone   = 16 // and on a phone
	gapSection = 18 // between the blocks of a page
	gapItem    = 12 // between items inside a block
	radCard    = 16
	formMaxW   = 440 // the widest a form of fields grows
	radControl = 8
)

// Type sizes, in DIPs. The default skin has a scale; Mac OS 9 has its pixel
// grid of 12 DIPs, and 24 for counts (setTypeScale).
var (
	fsTitle, fsHeading, fsBody, fsSmall, fsTiny float32
	fsCaption, fsLabel, fsSubtitle, fsEmphasis  float32
	fsBodyLarge, fsCount                        float32
)

// The look of the app is Lobe UI's (github.com/lobehub/lobe-mygo): its tokens
// for color, radius and type, and its components, which take their inputs as
// a Props struct: component(c, value, Props{Variant, Size, …}). brand is Lobe's
// token set with a blue primary; there is no purple anywhere.

// palette is the handful of colors the building blocks use, read from the
// tokens of the theme being built.
type palette struct {
	dark                         bool
	canvas, sidebar, card, hover ui.Color
	border, hairline             ui.Color
	text, muted, faint           ui.Color
	chip                         ui.Color // soft panels and tags
	rowSel, rowSelBorder         ui.Color // the chosen row of a list
	accent, accentSoft           ui.Color
	up, down, warn               ui.Color
	shadow, shadow2              ui.Color
}

func pal(c *ui.Context) palette {
	tk := theme.TokensOf(c)
	sidebar := ui.Hex("#EEF0F5")
	if tk.Dark {
		sidebar = ui.Hex("#15181F")
	}
	if c.Vibrancy() && !isMac() {
		sidebar = ui.Transparent
	}
	rowSel := ui.RGBA(255, 255, 255, 0.85)
	if tk.Dark {
		rowSel = ui.RGBA(255, 255, 255, 0.095)
	}
	return palette{
		dark:         tk.Dark,
		canvas:       tk.BgLayout,
		sidebar:      sidebar,
		card:         tk.BgContainer,
		hover:        tk.FillQuaternary,
		border:       tk.BorderSecondary,
		hairline:     tk.BorderSecondary,
		text:         tk.Text,
		muted:        tk.TextSecondary,
		faint:        tk.TextTertiary,
		chip:         tk.FillTertiary,
		rowSel:       rowSel,
		rowSelBorder: tk.BorderSecondary,
		accent:       tk.Primary,
		accentSoft:   tk.Primary.Alpha(0.12),
		up:           tk.Error,
		down:         tk.Success,
		warn:         tk.Warning,
	}
}

// Wrappers that give Lobe's components this app's usual props, so that a page
// reads as before: btn(c, "保存").Clicked().
func btn(c *ui.Context, text string) ui.Element {
	if isMac() {
		return macButton(c, text, false)
	}
	return action.Button(c, text, action.ButtonProps{Size: theme.Small})
}

func primaryBtn(c *ui.Context, text string) ui.Element {
	if isMac() {
		return macButton(c, text, true)
	}
	return action.Button(c, text, action.ButtonProps{Variant: action.Primary, Size: theme.Small})
}

func segmented(c *ui.Context, sel *int, labels ...string) ui.Element {
	if isMac() {
		return macSegmented(c, sel, labels...)
	}
	return action.Segmented(c, sel, labels, action.SegmentedProps{Size: theme.Small})
}

func sw(c *ui.Context, on *bool) ui.Element {
	if isMac() {
		return macCheck(c, on)
	}
	return choice.Switch(c, on, choice.SwitchProps{})
}

// dropdown is a select that shows the chosen label and opens the others below
// it: for a long list of choices, which a row of segments cannot hold on a
// phone. It reports whether the choice changed.
func dropdown(c *ui.Context, sel *int, labels ...string) bool {
	if isMac() {
		return choose(c, sel, labels...)
	}
	opts := make([]lfield.SelectOption, len(labels))
	for i, l := range labels {
		opts[i] = lfield.SelectOption{Value: strconv.Itoa(i), Label: l}
	}
	v := strconv.Itoa(*sel)
	lfield.Select(c, &v, lfield.SelectProps{Options: opts, Size: theme.Small})
	n, err := strconv.Atoi(v)
	if err != nil || n == *sel {
		return false
	}
	*sel = n
	return true
}

func slider(c *ui.Context, v *float64, lo, hi float64) ui.Element {
	return choice.Slider(c, v, choice.SliderProps{Min: lo, Max: hi})
}

func textInput(c *ui.Context, v *string, placeholder string) ui.Element {
	if isMac() {
		return macTextInput(c, v, placeholder)
	}
	return lfield.Input(c, v, lfield.InputProps{Placeholder: placeholder})
}

func passwordInput(c *ui.Context, v *string, placeholder string) ui.Element {
	return lfield.InputPassword(c, v, lfield.InputProps{Placeholder: placeholder})
}

func textArea(c *ui.Context, v *string, placeholder string, rows int) ui.Element {
	return lfield.TextArea(c, v, lfield.TextAreaProps{Placeholder: placeholder, MinRows: rows, MaxRows: rows})
}

func numberInput(c *ui.Context, v *float64, lo, hi, step float64) ui.Element {
	return lfield.InputNumber(c, v, lfield.InputNumberProps{Min: lo, Max: hi, Step: step, Precision: 2}).
		Keyboard(ui.KeyboardDecimal).
		ReturnKey(ui.ReturnDone)
}

// notice shows a line of feedback as a Lobe alert.
func notice(c *ui.Context, text string, kind feedback.AlertType) ui.Element {
	return feedback.Alert(c, text, feedback.AlertProps{Type: kind})
}

// card is Lobe's Block: an outlined rounded surface with a soft shadow.
func card(c *ui.Context) ui.Element { return skinCard(c) }

// pad is the padding around a page.
func pad(compact bool) float32 {
	if compact {
		return padPhone
	}
	return padPage
}

// page is the scrolling body of a page of text or forms: padded, with its
// blocks a section apart, and no wider than a line of text stays readable,
// unless the user chose otherwise (pagewidth.go).
func page(c *ui.Context, compact bool, body func()) ui.Element {
	return pageIn(c, compact, false, body)
}

// pageFull is the body of a page of data, the home page's grid and the detail
// page: it fills the window, unless the user chose a width.
func pageFull(c *ui.Context, compact bool, body func()) ui.Element {
	return pageIn(c, compact, true, body)
}

// header is the top of a page: a title with a line of context under it, and
// the page's actions at the right.
func header(c *ui.Context, title, subtitle string, actions func()) {
	pageHead(c, nil, fsTitle, title, subtitle, actions)
}

// subheader is a page header with a back button, for pages inside a page.
func subheader(c *ui.Context, a *app, title, subtitle string, actions func()) {
	pageHead(c, a, fsSubtitle, title, subtitle, actions)
}

// pageHead draws both: the back button when a is given, and the title in the
// size it is passed.
func pageHead(c *ui.Context, a *app, titleSize float32, title, subtitle string, actions func()) {
	if isMac() {
		if a == nil {
			macHeader(c, title, subtitle, actions)
			return
		}
		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.BackButton(c, a.router)
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				macHeader(c, title, subtitle, actions)
			})
		})
		return
	}
	p := pal(c)
	ui.Row(c).AlignItems(ui.Center).Gap(gapItem).Wrap().Children(func() {
		if a != nil {
			ui.BackButton(c, a.router)
		}
		ui.Column(c).Gap(2).Children(func() {
			ui.Text(c, title).FontSize(titleSize).FontWeight(650).LetterSpacing(-0.4)
			if subtitle != "" {
				ui.Text(c, subtitle).FontSize(fsCaption).FontFeatures("tnum").TextColor(p.muted)
			}
		})
		ui.Spacer(c)
		if actions != nil {
			ui.Row(c).AlignItems(ui.Center).Gap(8).Wrap().Children(actions)
		}
	})
}

// sectionTitle is the heading of a block on a page, such as "预览".
func sectionTitle(c *ui.Context, text string) ui.Element {
	return ui.Text(c, text).FontSize(fsHeading).FontWeight(650)
}

// homeSwitch is the switch that puts a script or table on the home page.
func homeSwitch(c *ui.Context, on *bool) {
	ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
		ui.Text(c, tr("显示在首页")).FontSize(fsBody).Grow(1)
		sw(c, on)
	})
}

// label is the small heading over a group or a field.
func label(c *ui.Context, text string) ui.Element {
	return ui.Text(c, text).FontSize(fsSmall).FontWeight(600).TextColor(pal(c).muted)
}

// hint is a line of explanation in muted type.
func hint(c *ui.Context, text string) ui.Element {
	return ui.Text(c, text).FontSize(fsSmall).TextColor(pal(c).muted)
}

// field puts a visible label over a control, and an explanation under it.
func field(c *ui.Context, name, note string, control func()) {
	ui.Column(c).Gap(6).Children(func() {
		label(c, name)
		control()
		if note != "" {
			ui.Text(c, note).FontSize(fsTiny).TextColor(pal(c).muted)
		}
	})
}

// row is one line of a settings group: a title with an optional explanation at
// the left and a control at the right.
type row struct {
	title, note string
	control     func()
	// stack puts the control under the title on a phone, for a control too wide
	// to sit beside a line of explanation (a row of segments).
	stack bool
}

// group draws a small heading and a soft panel of rows with hairlines between
// them, as Kero's settings do.
func group(c *ui.Context, title string, compact bool, rows ...row) {
	p := pal(c)
	ui.Column(c).Gap(8).Children(func() {
		if title != "" && isMac() {
			macCategory(c, title)
		} else if title != "" {
			ui.Text(c, title).FontSize(fsTiny).FontWeight(600).LetterSpacing(0.3).TextColor(p.muted).Padding(0, 4)
		}
		var panel ui.Element
		if isMac() {
			panel = macBevel(layout.Block(c, layout.BlockProps{Variant: layout.BlockFilled}).Radius(0).Background(macPanel).Padding(2, 16), false)
		} else {
			panel = ui.Column(c).Radius(12).Background(p.card).BorderWidth(1).BorderColor(p.border).Padding(2, 16)
		}
		panel.Children(func() {
			for i, r := range rows {
				r := r
				if i > 0 {
					ui.Box(c).Height(1).Shrink(0).Background(p.hairline)
				}
				body := func() {
					ui.Column(c).Gap(3).Grow(1).MinWidth(0).Children(func() {
						ui.Text(c, r.title).FontSize(fsBody).FontWeight(500)
						if r.note != "" {
							ui.Text(c, r.note).FontSize(fsSmall).TextColor(p.muted).LineHeight(1.4)
						}
					})
					if r.control != nil && !(compact && r.stack) {
						ui.Row(c).Shrink(0).Gap(8).AlignItems(ui.Center).Children(r.control)
					}
				}
				if compact && r.stack && r.control != nil {
					ui.Column(c).Gap(10).Padding(12, 0).Children(func() {
						ui.Row(c).Children(body)
						ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(r.control)
					})
					continue
				}
				ui.Row(c).AlignItems(ui.Center).Gap(16).Padding(12, 0).Children(body)
			}
		})
	})
}

// chip is a small rounded tag.
func chip(c *ui.Context, text string, fg, bg ui.Color) ui.Element {
	return ui.Row(c).Padding(3, 9).Radius(99).Background(bg).AlignItems(ui.Center).Children(func() {
		ui.Text(c, text).FontSize(fsTiny).FontWeight(600).TextColor(fg).NoWrap()
	})
}

// deltaChip shows a change as a Lobe tag with an arrow: red up, green down.
func deltaChip(c *ui.Context, pct float64) ui.Element {
	switch {
	case math.Abs(pct) < 0.005:
		return display.Tag(c, "— 0.00%", display.TagProps{Size: theme.Small})
	case pct > 0:
		return display.Tag(c, "▲ "+sign(pct), display.TagProps{Color: display.TagError, Size: theme.Small})
	}
	return display.Tag(c, "▼ "+sign(pct), display.TagProps{Color: display.TagSuccess, Size: theme.Small})
}

func sign(pct float64) string {
	return formatPct(math.Abs(pct))
}

// iconBadge is an icon on a tinted rounded square.
func iconBadge(c *ui.Context, name string, color ui.Color, size float32) ui.Element {
	if isMac() {
		return macBevel(ui.Row(c).Size(size, size).Background(macWhite), false).
			AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Icon(c, icon(name)).FontSize(size * 0.56).TextColor(macBlack)
		})
	}
	return ui.Row(c).Size(size, size).Radius(size * 0.26).Background(color.Alpha(0.11)).
		AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		ui.Icon(c, icon(name)).FontSize(size * 0.52).TextColor(color)
	})
}

// navRow draws one Apple-native sidebar / settings navigation item.
func navRow(c *ui.Context, key, iconName, label string, active bool, width float32, trailing func()) bool {
	p := pal(c)
	icCol, txtCol, weight := p.muted, p.text.Alpha(0.84), 500
	if active {
		icCol, txtCol, weight = p.accent, p.text, 600
	}
	b := ui.ButtonBase(c.Key(key)).
		Width(width).
		Height(32).
		Padding(0, 10).
		Radius(radControl).
		Gap(9).
		AlignItems(ui.Center).
		Label(label)
	if active {
		b = b.Background(p.rowSel)
		if !p.dark {
			b = b.BorderWidth(1).BorderColor(p.hairline)
		}
	} else if b.Hovered() {
		b = b.Background(p.hover)
	}
	return b.Children(func() {
		ui.Icon(c, icon(iconName)).FontSize(15).TextColor(icCol)
		ui.Text(c, label).FontSize(fsBody).FontWeight(weight).TextColor(txtCol).SingleLine()
		ui.Spacer(c)
		if trailing != nil {
			trailing()
		}
	}).Clicked()
}

// ghostIconBtn draws a 26x26 borderless hoverable action icon matching Weather Bar.
func ghostIconBtn(c *ui.Context, svg *ui.SVG, tip string, active bool) ui.Element {
	p := pal(c)
	b := ui.ButtonBase(c).
		Size(26, 26).
		Radius(7).
		AlignItems(ui.Center).
		Justify(ui.Center).
		Cursor(ui.CursorPointer).
		Label(tip).
		Tooltip(tip)
	if active || b.Hovered() {
		b = b.Background(p.hover)
	}
	return b.Children(func() {
		ui.Icon(c, svg).FontSize(14).TextColor(p.muted)
	})
}

// popoverMenuRow draws a compact Apple-style menu item inside context/floating popovers.
func popoverMenuRow(c *ui.Context, label string, checked, danger bool) ui.Element {
	p := pal(c)
	b := ui.ButtonBase(c).
		FillWidth().
		Height(28).
		Padding(0, 10).
		Radius(7).
		Gap(8).
		AlignItems(ui.Center).
		Justify(ui.Start).
		Cursor(ui.CursorPointer).
		Label(label)
	if b.Hovered() {
		b = b.Background(p.hover)
	}
	fg, weight := p.text, 500
	if danger {
		fg = p.up
	} else if checked {
		fg, weight = p.accent, 600
	}
	checkSVG := barui.DefaultIcon("check")
	return b.Children(func() {
		if checked && checkSVG != nil {
			ui.Icon(c, checkSVG).FontSize(13).TextColor(p.accent)
		} else {
			ui.Box(c).Size(13, 13).Shrink(0)
		}
		ui.Text(c, label).FontSize(12.5).FontWeight(weight).TextColor(fg).SingleLine()
	})
}

// accentOf is the color a card takes: the first item's or line's color when
// the script gave one, else the app's accent.
func accentOf(res engine.Result, fallback ui.Color) ui.Color {
	if pb, ok := progressBarOf(res); ok && len(pb.Items) > 0 {
		if pb.Style == "neon" && !hexColor.MatchString(pb.Items[0].Color) {
			return neonPalette[0]
		}
		return parseColor(pb.Items[0].Color, fallback)
	}
	if res.Chart != nil {
		switch res.Chart.Type {
		case "donut":
			if len(res.Chart.Slices) > 0 {
				return parseColor(res.Chart.Slices[0].Color, donutPalette[0])
			}
		case "countdown":
			if len(res.Chart.Countdown) > 0 {
				return parseColor(res.Chart.Countdown[0].Color, fallback)
			}
		case "clock":
			if res.Chart.Clock != nil {
				return parseColor(res.Chart.Clock.Color, fallback)
			}
		case "contrib":
			if res.Chart.Contrib != nil {
				return parseColor(res.Chart.Contrib.Color, ui.Hex("#10B981"))
			}
		case "candle":
		default:
			if len(res.Chart.Series) > 0 {
				return parseColor(res.Chart.Series[0].Color, fallback)
			}
		}
	}
	return fallback
}

// empty is Lobe's Empty: an icon, a title and a line of explanation.
func empty(c *ui.Context, iconName, title, note string) {
	display.Empty(c, display.EmptyProps{Icon: icon(iconName), Title: title, Description: note})
}

func formatPct(v float64) string { return fmtFloat(v, 2) + "%" }

func fmtFloat(v float64, decimals int) string {
	return strconv.FormatFloat(v, 'f', decimals, 64)
}
