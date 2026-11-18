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

	"scriptboard/src/internal/engine"
)

// The look of the app. One palette per appearance, one theme per palette, and
// a few building blocks that every page uses, so that spacing, radius, type
// sizes and colors are decided in one place.

// Spacing and type scale, in DIPs.
const (
	padPage    = 24 // padding around a page on a wide window
	padPhone   = 16 // and on a phone
	gapSection = 20 // between the blocks of a page
	gapItem    = 12 // between items inside a block
	radCard    = 14
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
	return palette{
		dark:         tk.Dark,
		canvas:       tk.BgLayout,
		sidebar:      tk.BgLayout,
		card:         tk.BgContainer,
		hover:        tk.FillQuaternary,
		border:       tk.BorderSecondary,
		hairline:     tk.BorderSecondary,
		text:         tk.Text,
		muted:        tk.TextSecondary,
		faint:        tk.TextTertiary,
		chip:         tk.FillTertiary,
		rowSel:       tk.BgContainer,
		rowSelBorder: tk.BorderSecondary,
		accent:       tk.Primary,
		accentSoft:   tk.Primary.Alpha(0.10),
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
	return lfield.InputNumber(c, v, lfield.InputNumberProps{Min: lo, Max: hi, Step: step, Precision: 2})
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

// page is the scrolling body of a page: padded, with its blocks a section
// apart, and no wider than a line of text stays readable.
func page(c *ui.Context, compact bool, body func()) ui.Element {
	return ui.Scroll(c).Grow(1).Padding(pad(compact)).Children(func() {
		ui.Column(c).MaxWidth(1120).Gap(gapSection).Children(body)
	})
}

// header is the top of a page: a title with a line of context under it, and
// the page's actions at the right.
func header(c *ui.Context, title, subtitle string, actions func()) {
	if isMac() {
		macHeader(c, title, subtitle, actions)
		return
	}
	p := pal(c)
	ui.Row(c).AlignItems(ui.Center).Gap(gapItem).Wrap().Children(func() {
		ui.Column(c).Gap(2).Children(func() {
			ui.Text(c, title).FontSize(fsTitle).FontWeight(650)
			if subtitle != "" {
				ui.Text(c, subtitle).FontSize(fsCaption).TextColor(p.muted)
			}
		})
		ui.Spacer(c)
		if actions != nil {
			ui.Row(c).AlignItems(ui.Center).Gap(8).Wrap().Children(actions)
		}
	})
}

// subheader is a page header with a back button, for pages inside a page.
func subheader(c *ui.Context, a *app, title, subtitle string, actions func()) {
	if isMac() {
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
		ui.BackButton(c, a.router)
		ui.Column(c).Gap(2).Children(func() {
			ui.Text(c, title).FontSize(fsSubtitle).FontWeight(650)
			if subtitle != "" {
				ui.Text(c, subtitle).FontSize(fsCaption).TextColor(p.muted)
			}
		})
		ui.Spacer(c)
		if actions != nil {
			ui.Row(c).AlignItems(ui.Center).Gap(8).Wrap().Children(actions)
		}
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
}

// group draws a small heading and a soft panel of rows with hairlines between
// them, as Kero's settings do.
func group(c *ui.Context, title string, compact bool, rows ...row) {
	p := pal(c)
	ui.Column(c).Gap(8).Children(func() {
		if title != "" && isMac() {
			macCategory(c, title)
		} else if title != "" {
			ui.Text(c, title).FontSize(fsSmall).FontWeight(600).TextColor(p.muted).Padding(0, 4)
		}
		panel := layout.Block(c, layout.BlockProps{Variant: layout.BlockFilled}).Padding(2, 16)
		if isMac() {
			panel = macBevel(layout.Block(c, layout.BlockProps{Variant: layout.BlockFilled}).Radius(0).Background(macPanel).Padding(2, 16), false)
		}
		panel.Children(func() {
			for i, r := range rows {
				r := r
				if i > 0 {
					ui.Box(c).Height(1).Shrink(0).Background(p.text.Alpha(0.07))
				}
				ui.Row(c).AlignItems(ui.Center).Gap(16).Padding(11, 0).Children(func() {
					ui.Column(c).Gap(3).Grow(1).MinWidth(0).Children(func() {
						ui.Text(c, r.title).FontSize(fsBody)
						if r.note != "" {
							ui.Text(c, r.note).FontSize(fsSmall).TextColor(p.muted).LineHeight(1.35)
						}
					})
					if r.control != nil {
						ui.Row(c).Shrink(0).Gap(8).AlignItems(ui.Center).Children(r.control)
					}
				})
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
	return ui.Row(c).Size(size, size).Radius(size * 0.28).Background(color.Alpha(0.13)).
		AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		ui.Icon(c, icon(name)).FontSize(size * 0.52).TextColor(color)
	})
}

// accentOf is the color a card takes: the first line's color when the script
// gave one, else the app's accent.
func accentOf(res engine.Result, fallback ui.Color) ui.Color {
	if res.Chart != nil && res.Chart.Type != "candle" && len(res.Chart.Series) > 0 {
		return parseColor(res.Chart.Series[0].Color, fallback)
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
