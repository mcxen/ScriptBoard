package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// The Mac OS 9 controls. They are drawn here rather than by Lobe's components,
// whose looks are rounded and flat: a Platinum bevel, pinstriped title bars,
// square checkboxes, a menu bar, and a dithered desk. Each wrapper in kit.go
// asks isMac and draws its own look; the default skin never reaches this file.

var (
	macFace      = ui.Hex("#EEEEEE") // a button's face, as AvaMedia's AcPopupFace
	macPanel     = ui.Hex("#EEEEEE") // a window's background
	macBlack     = ui.Hex("#111111")
	macWhite     = ui.Hex("#FFFFFF")
	macShade     = ui.Hex("#888888") // the shadow side of a bevel, and the hairlines
	macDesk      = ui.Hex("#CCCCCC") // the desk, dithered with macDot
	macDot       = ui.Hex("#AAAAAA")
	macAccent    = ui.Hex("#3D4E80") // the slate accent of the skin
	macSelection = ui.Hex("#CCCCFF") // a chosen row: lavender, as AvaMedia's AcSelection
	macPressed   = ui.Hex("#BBBBBB")
)

// isMac reports whether the Mac OS 9 skin is in use.
func isMac() bool { return curSkin() == skinMac9 }

// macBevel gives el the Platinum edge: a black outline, a white light on the
// top and left and a gray shade on the bottom and right. Pressed swaps the
// light and the shade.
func macBevel(el ui.Element, pressed bool) ui.Element {
	light, shade := macWhite, macShade
	if pressed {
		light, shade = macShade, macWhite
	}
	return el.BorderWidth(1).BorderColor(macBlack).
		InsetShadow(1, 1, 0, 0, light).InsetShadow(-1, -1, 0, 0, shade)
}

// macButton is a push button: a bevelled face with its label. primary is the
// blue default button.
func macButton(c *ui.Context, text string, primary bool) ui.Element {
	face, fg := macFace, macBlack
	if primary {
		face, fg = macAccent, macWhite
	}
	return macBevel(ui.ButtonBase(c).Padding(3, 12).Background(face).Radius(3), false).Children(func() {
		ui.Text(c, text).FontSize(fsCaption).TextColor(fg).NoWrap()
	})
}

// macTab is one segment of a segmented control: the chosen one is inverted.
func macTab(c *ui.Context, text string, chosen bool) ui.Element {
	face, fg := macFace, macBlack
	if chosen {
		face, fg = macBlack, macWhite
	}
	return macBevel(ui.ButtonBase(c).Padding(3, 10).Background(face), chosen).Children(func() {
		ui.Text(c, text).FontSize(fsCaption).TextColor(fg).NoWrap()
	})
}

// macSegmented is a row of tabs; a click on one sets *sel.
func macSegmented(c *ui.Context, sel *int, labels ...string) ui.Element {
	return ui.Row(c).Gap(0).Children(func() {
		for i, l := range labels {
			i := i
			if macTab(c, l, *sel == i).Clicked() {
				*sel = i
			}
		}
	})
}

// choose is a segmented control that reports whether the user changed it.
func choose(c *ui.Context, sel *int, labels ...string) bool {
	if isMac() {
		before := *sel
		macSegmented(c, sel, labels...)
		return *sel != before
	}
	return segmented(c, sel, labels...).Changed()
}

// macCheck is a square checkbox with a check mark when on.
func macCheck(c *ui.Context, on *bool) ui.Element {
	return macBevel(ui.ButtonBase(c).Size(15, 15).Background(macWhite), false).Children(func() {
		if *on {
			ui.Text(c, "✓").FontSize(fsLabel).Bold().TextColor(macBlack)
		}
	})
}

// toggle is a checkbox that reports whether the user changed it.
func toggle(c *ui.Context, on *bool) bool {
	if isMac() {
		if macCheck(c, on).Clicked() {
			*on = !*on
			return true
		}
		return false
	}
	return sw(c, on).Changed()
}

// macTextInput is a field sunk into the window: white, with the bevel turned
// in. The value is edited in place.
func macTextInput(c *ui.Context, v *string, placeholder string) ui.Element {
	return macBevel(ui.Row(c).Background(macWhite).Padding(0, 4), true).Children(func() {
		ui.TextInputBase(c.Key(fmt.Sprintf("mac-in-%p", v)), v).Placeholder(placeholder).
			FontSize(fsBody).Grow(1).MinWidth(0)
	})
}

// macMenuBar is the menu bar along the top of the window: the app's name and
// the menus that the classic Mac had. Without the system frame it is the title
// bar too: it drags the window, and its close box closes it.
func macMenuBar(c *ui.Context, a *app) {
	ui.Row(c).Height(22).Background(macWhite).AlignItems(ui.Center).Padding(0, 8).Gap(16).
		BorderWidth(0, 0, 2, 0).BorderColor(macBlack).DragWindow().Children(func() {
		if a.frameless {
			if macCloseBox(c).Clicked() && a.win != nil {
				a.win.Close()
			}
		}
		ui.Text(c, appName()).FontSize(fsCaption).Bold().TextColor(macBlack)
		for _, m := range []string{tr("文件"), tr("编辑"), tr("查看"), tr("特殊")} {
			ui.Text(c, m).FontSize(fsCaption).TextColor(macBlack)
		}
	})
}

// macCloseBox is the close box of a Mac OS 9 window: a bevelled square.
func macCloseBox(c *ui.Context) ui.Element {
	return macBevel(ui.ButtonBase(c).Size(13, 13).Background(macFace), false)
}

// macCategory is a settings group's heading: a gray gradient strip with the
// title on it, as AvaMedia's category bars.
func macCategory(c *ui.Context, title string) {
	ui.Row(c).Height(18).AlignItems(ui.Center).Padding(0, 8).
		BorderWidth(1).BorderColor(macShade).
		Draw(func(p *ui.Painter, r ui.Rect) {
			p.FillGradient(r, ui.LinearGradient{From: ui.Hex("#FFFFFF"), To: ui.Hex("#AAAAAA"), Angle: 90}, 0)
		}).Children(func() {
		ui.Text(c, title).FontSize(fsSmall).Bold().TextColor(macBlack)
	})
}

// macWindowBar is the title bar of a window: pinstripes, a close box and the
// title in a white plate.
func macWindowBar(c *ui.Context, title string) {
	ui.Row(c).Height(22).AlignItems(ui.Center).Gap(8).Padding(0, 6).
		Draw(func(p *ui.Painter, r ui.Rect) {
			for y := r.Y + 3; y < r.Y+r.H-3; y += 2 {
				p.Fill(ui.Rect{X: r.X + 2, Y: y, W: r.W - 4, H: 1}, macBlack, 0)
			}
		}).
		BorderWidth(1).BorderColor(macBlack).Background(macPanel).Children(func() {
		macBevel(ui.Row(c).Size(13, 13).Background(macPanel), false)
		ui.Row(c).Background(macWhite).Padding(0, 8).BorderWidth(1).BorderColor(macBlack).Children(func() {
			ui.Text(c, title).FontSize(fsCaption).Bold().TextColor(macBlack).NoWrap()
		})
	})
}

// macHeader is a page's header in the Mac OS 9 look: a title bar, and the
// subtitle and actions under it.
func macHeader(c *ui.Context, title, subtitle string, actions func()) {
	ui.Column(c).Gap(8).Children(func() {
		macWindowBar(c, title)
		if subtitle != "" || actions != nil {
			ui.Row(c).AlignItems(ui.Center).Gap(gapItem).Wrap().Children(func() {
				if subtitle != "" {
					ui.Text(c, subtitle).FontSize(fsCaption).TextColor(pal(c).muted)
				}
				ui.Spacer(c)
				if actions != nil {
					ui.Row(c).AlignItems(ui.Center).Gap(8).Wrap().Children(actions)
				}
			})
		}
	})
}

// macSidebarItem is a row of the sidebar. The chosen one is inverted, as a
// selected item of the Finder's list was. It reports a click.
func macSidebarItem(c *ui.Context, label string, iconName string, chosen bool) bool {
	fg := macBlack
	row := ui.ButtonBase(c).Width(sidebarWidth-24).Padding(4, 8).Gap(8).AlignItems(ui.Center)
	if chosen {
		row = row.Background(macSelection)
	}
	return row.Children(func() {
		ui.Icon(c, icon(iconName)).FontSize(fsBody).TextColor(fg)
		ui.Text(c, label).FontSize(fsBody).TextColor(fg)
	}).Clicked()
}

// macDesktop paints the desk: a flat gray with a fine dot pattern.
func macDesktop(c *ui.Context, el ui.Element) ui.Element {
	return el.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Fill(r, macDesk, 0)
		for y := r.Y; y < r.Y+r.H; y += 4 {
			for x := r.X + float32(int(y/4)%2)*2; x < r.X+r.W; x += 4 {
				p.Fill(ui.Rect{X: x, Y: y, W: 1, H: 1}, macDot, 0)
			}
		}
	})
}

// skinSurface is a page's own background: the desk shows through in Mac OS 9.
func skinSurface(c *ui.Context, el ui.Element) ui.Element {
	if isMac() {
		return el
	}
	return el.Background(pal(c).canvas)
}

// skinPlot gives a chart's plot the white of a Mac OS 9 window's content.
func skinPlot(c *ui.Context, el ui.Element) ui.Element {
	if isMac() {
		return el.Background(macWhite)
	}
	return el
}
