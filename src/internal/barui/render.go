package barui

import (
	"encoding/base64"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"scriptboard/src/internal/bar"
	"scriptboard/src/internal/colortheme"
)

// Theme carries resolved semantic palette tokens for rendering a .bar virtual
// DOM tree in Dark or Light mode, with or without desktop window vibrancy.
type Theme struct {
	Dark            bool
	Vibrancy        bool
	Surface         ui.Color
	SurfaceCard     ui.Color
	SurfaceElevated ui.Color
	SurfaceSubtle   ui.Color
	SurfaceHover    ui.Color
	SurfacePressed  ui.Color
	Text            ui.Color
	TextMuted       ui.Color
	TextFaint       ui.Color
	Accent          ui.Color
	AccentSurface   ui.Color
	Success         ui.Color
	SuccessSurface  ui.Color
	Warning         ui.Color
	WarningSurface  ui.Color
	Danger          ui.Color
	DangerSurface   ui.Color
	Purple          ui.Color
	PurpleSurface   ui.Color
	Cyan            ui.Color
	CyanSurface     ui.Color
	Border          ui.Color
	BorderStrong    ui.Color
}

// ThemeFor constructs the default light or dark Paper & Ink Theme.
// Surface values match ScriptBoard's card palette (#1C1C1F dark, #FFFFFF light).
func ThemeFor(dark, vibrancy bool) Theme {
	if dark {
		th := Theme{
			Dark:            true,
			Vibrancy:        vibrancy,
			Surface:         ui.Hex("#1C1C1F"),
			SurfaceCard:     ui.RGBA(255, 255, 255, 0.055),
			SurfaceElevated: ui.RGBA(255, 255, 255, 0.10),
			SurfaceSubtle:   ui.RGBA(255, 255, 255, 0.042),
			SurfaceHover:    ui.Hex("#27272B"),
			SurfacePressed:  ui.Hex("#323238"),
			Text:            ui.Hex("#F0EFEA"),
			TextMuted:       ui.Hex("#9D9A93"),
			TextFaint:       ui.Hex("#6E6B65"),
			Accent:          ui.Hex("#3F76AD"),
			AccentSurface:   ui.RGBA(63, 118, 173, 0.17),
			Success:         ui.Hex("#4AA884"),
			SuccessSurface:  ui.RGBA(74, 168, 132, 0.17),
			Warning:         ui.Hex("#D99B43"),
			WarningSurface:  ui.RGBA(217, 155, 67, 0.17),
			Danger:          ui.Hex("#DC6B6B"),
			DangerSurface:   ui.RGBA(220, 107, 107, 0.17),
			Purple:          ui.Hex("#8B8EC4"),
			PurpleSurface:   ui.RGBA(139, 142, 196, 0.17),
			Cyan:            ui.Hex("#5EA3AA"),
			CyanSurface:     ui.RGBA(94, 163, 170, 0.17),
			Border:          ui.RGBA(255, 255, 255, 0.085),
			BorderStrong:    ui.RGBA(255, 255, 255, 0.16),
		}
		if vibrancy {
			th.SurfaceCard = ui.RGBA(255, 255, 255, 0.05)
			th.SurfaceSubtle = ui.RGBA(255, 255, 255, 0.045)
			th.SurfaceHover = ui.RGBA(255, 255, 255, 0.09)
			th.SurfacePressed = ui.RGBA(255, 255, 255, 0.14)
			th.Border = ui.RGBA(255, 255, 255, 0.10)
		}
		return th
	}
	th := Theme{
		Dark:            false,
		Vibrancy:        vibrancy,
		Surface:         ui.Hex("#FFFFFF"),
		SurfaceCard:     ui.RGBA(28, 25, 23, 0.032),
		SurfaceElevated: ui.Hex("#FFFFFF"),
		SurfaceSubtle:   ui.RGBA(28, 25, 23, 0.048),
		SurfaceHover:    ui.Hex("#ECEAE4"),
		SurfacePressed:  ui.Hex("#E0DDD5"),
		Text:            ui.Hex("#181715"),
		TextMuted:       ui.Hex("#625F59"),
		TextFaint:       ui.Hex("#8F8B82"),
		Accent:          ui.Hex("#23537A"),
		AccentSurface:   ui.RGBA(35, 83, 122, 0.11),
		Success:         ui.Hex("#1F7A5A"),
		SuccessSurface:  ui.RGBA(31, 122, 90, 0.12),
		Warning:         ui.Hex("#B46914"),
		WarningSurface:  ui.RGBA(180, 105, 20, 0.12),
		Danger:          ui.Hex("#C53D3D"),
		DangerSurface:   ui.RGBA(197, 61, 61, 0.11),
		Purple:          ui.Hex("#5B5F97"),
		PurpleSurface:   ui.RGBA(91, 95, 151, 0.11),
		Cyan:            ui.Hex("#3D7A80"),
		CyanSurface:     ui.RGBA(61, 122, 128, 0.11),
		Border:          ui.RGBA(28, 25, 23, 0.085),
		BorderStrong:    ui.RGBA(28, 25, 23, 0.16),
	}
	if vibrancy {
		th.SurfaceCard = ui.RGBA(255, 255, 255, 0.56)
		th.SurfaceSubtle = ui.RGBA(28, 25, 23, 0.04)
		th.SurfaceHover = ui.RGBA(28, 25, 23, 0.07)
		th.SurfacePressed = ui.RGBA(28, 25, 23, 0.12)
		th.Border = ui.RGBA(28, 25, 23, 0.085)
	}
	return th
}

// Color resolves a semantic color token, #RGB/#RGBA/#RRGGBB/#RRGGBBAA hex,
// rgb()/rgba(), or CSS named color literal into a raw ui.Color.
func (th Theme) Color(name string, fallback ui.Color) ui.Color {
	s := strings.TrimSpace(name)
	if s == "" {
		return fallback
	}
	if col, ok := th.semanticColor(s); ok {
		return col
	}
	if col, ok := colortheme.Parse(s); ok {
		return col
	}
	return fallback
}

func (th Theme) semanticColor(s string) (ui.Color, bool) {
	switch s {
	case "surface":
		return th.Surface, true
	case "surfaceCard", "card":
		return th.SurfaceCard, true
	case "surfaceElevated", "elevated":
		return th.SurfaceElevated, true
	case "surfaceSubtle", "subtle":
		return th.SurfaceSubtle, true
	case "surfaceHover":
		return th.SurfaceHover, true
	case "surfacePressed":
		return th.SurfacePressed, true
	case "text", "foreground":
		return th.Text, true
	case "textMuted", "muted", "secondary":
		return th.TextMuted, true
	case "textFaint", "faint":
		return th.TextFaint, true
	case "accent", "primary":
		return th.Accent, true
	case "accentSurface":
		return th.AccentSurface, true
	case "success":
		return th.Success, true
	case "successSurface":
		return th.SuccessSurface, true
	case "warning":
		return th.Warning, true
	case "warningSurface":
		return th.WarningSurface, true
	case "danger", "error":
		return th.Danger, true
	case "dangerSurface":
		return th.DangerSurface, true
	case "purple":
		return th.Purple, true
	case "purpleSurface":
		return th.PurpleSurface, true
	case "cyan":
		return th.Cyan, true
	case "cyanSurface":
		return th.CyanSurface, true
	case "border", "divider":
		return th.Border, true
	case "borderStrong":
		return th.BorderStrong, true
	case "neutral":
		return th.TextMuted, true
	case "neutralSurface":
		return th.SurfaceSubtle, true
	case "transparent":
		return ui.Color{}, true
	}
	return ui.Color{}, false
}

// Palette returns the base colortheme.Palette corresponding to th.
func (th Theme) Palette() colortheme.Palette {
	surf := th.Surface
	if surf.A < 255 {
		surf = colortheme.CompositeOver(surf, colortheme.DefaultPalette(th.Dark).Surface)
	}
	return colortheme.Palette{
		Dark:       th.Dark,
		Surface:    surf,
		SurfaceAlt: th.SurfaceElevated,
		Text:       th.Text,
		TextMuted:  th.TextMuted,
		Border:     th.Border,
	}
}

// Resolve maps semantic tokens directly (respecting surface polarity overrides)
// and adapts literal CSS colors according to role and surface context unless
// rawColor is true.
func (th Theme) Resolve(name string, role colortheme.Role, pal colortheme.Palette, rawColor bool, fallback ui.Color) ui.Color {
	s := strings.TrimSpace(name)
	if s == "" {
		return fallback
	}
	if col, ok := th.semanticColor(s); ok {
		if pal.Dark != th.Dark {
			switch s {
			case "text", "foreground":
				return pal.Text
			case "textMuted", "muted", "secondary":
				return pal.TextMuted
			case "textFaint", "faint":
				return pal.TextMuted.Alpha(0.72)
			case "border", "divider":
				return pal.Border
			}
		}
		return col
	}
	col, ok := colortheme.Parse(s)
	if !ok {
		return fallback
	}
	if rawColor || col.A == 0 {
		return col
	}
	return colortheme.Adapt(col, role, pal)
}

func (th Theme) lookupTone(tone string) (fg, surface ui.Color, ok bool) {
	switch strings.ToLower(strings.TrimSpace(tone)) {
	case "success":
		return th.Success, th.SuccessSurface, true
	case "warning":
		return th.Warning, th.WarningSurface, true
	case "danger", "error":
		return th.Danger, th.DangerSurface, true
	case "purple":
		return th.Purple, th.PurpleSurface, true
	case "cyan":
		return th.Cyan, th.CyanSurface, true
	case "neutral", "muted":
		return th.TextMuted, th.SurfaceSubtle, true
	case "accent", "primary":
		return th.Accent, th.AccentSurface, true
	default:
		return ui.Color{}, ui.Color{}, false
	}
}

func (th Theme) toneColors(tone string) (fg, surface ui.Color) {
	if fg, surface, ok := th.lookupTone(tone); ok {
		return fg, surface
	}
	return th.Accent, th.AccentSurface
}

type renderState struct {
	pal      colortheme.Palette
	rawColor bool
}

func (st renderState) withProps(props map[string]any) renderState {
	if propBool(props, "rawColor", false) {
		st.rawColor = true
	}
	if props != nil {
		if v, ok := props["adaptColors"].(bool); ok && !v {
			st.rawColor = true
		}
	}
	return st
}

func (st renderState) themeFor(th Theme) Theme {
	if st.pal.Dark != th.Dark {
		return ThemeFor(st.pal.Dark, th.Vibrancy)
	}
	return th
}

func barUITheme(th Theme) *ui.Theme {
	var out ui.Theme
	if th.Dark {
		out = *ui.DarkTheme()
	} else {
		out = *ui.LightTheme()
	}
	out.Dark = th.Dark
	out.Background = th.Surface
	out.Surface = th.SurfaceSubtle
	out.SurfaceHover = th.SurfaceHover
	out.SurfacePressed = th.SurfacePressed
	out.Border = th.Border
	out.Text = th.Text
	out.TextMuted = th.TextMuted
	out.Accent = th.Accent
	out.AccentHover = th.Accent.Mix(out.Text, 0.12)
	out.AccentPressed = th.Accent.Mix(out.Background, 0.14)
	out.AccentText = colortheme.OnColor(th.Accent)
	out.Danger = th.Danger
	out.Warning = th.Warning
	out.Success = th.Success
	return &out
}

// RenderOptions configures RenderPanel and RenderNode.
type RenderOptions struct {
	Width       float32
	Material    string
	LoadingText string
	IconLookup  func(name string) *ui.SVG
	OnAction    func(actionID string)
	OnActionArg func(actionID string, arg any)
}

func (o RenderOptions) lookupIcon(name string) *ui.SVG {
	clean := strings.TrimSpace(strings.TrimPrefix(name, "lucide:"))
	clean = strings.ReplaceAll(clean, ".", "-")
	if clean == "" {
		return nil
	}
	if s := DefaultIcon(clean); s != nil {
		return s
	}
	if o.IconLookup != nil {
		return o.IconLookup(clean)
	}
	return nil
}

func (o RenderOptions) fireAction(actionID string) {
	if actionID == "" {
		return
	}
	if o.OnAction != nil {
		o.OnAction(actionID)
	} else if o.OnActionArg != nil {
		o.OnActionArg(actionID, nil)
	}
}

func (o RenderOptions) fireActionArg(actionID string, arg any) {
	if actionID == "" {
		return
	}
	if o.OnActionArg != nil {
		o.OnActionArg(actionID, arg)
	} else if o.OnAction != nil {
		o.OnAction(actionID)
	}
}

// RenderPanel renders snap.Panel (or snap.Error / loading placeholder) inside
// a rounded menubar panel card.
func RenderPanel(c *ui.Context, snap bar.Snapshot, th Theme, opts RenderOptions) ui.Element {
	width := opts.Width
	if width <= 0 {
		width = 300
	}
	var prefH float32
	if snap.Panel != nil {
		if pw, ok := PropFloat(snap.Panel.Props, "preferredWidth"); ok && pw > 0 && opts.Width <= 0 {
			width = pw
		}
		if ph, ok := PropFloat(snap.Panel.Props, "preferredHeight"); ok && ph > 0 {
			prefH = ph
		}
	}
	surface := th.Surface
	if th.Vibrancy {
		switch strings.ToLower(strings.TrimSpace(opts.Material)) {
		case "solid":
			surface = th.Surface
		default:
			if th.Dark {
				surface = ui.RGBA(24, 27, 34, 0.72)
			} else {
				surface = ui.RGBA(255, 255, 255, 0.78)
			}
		}
	}
	cardEl := ui.Themed(c, barUITheme(th)).
		Width(width).
		Background(surface).
		Radius(14).
		BorderWidth(1).
		BorderColor(th.Border).
		Clip()
	if prefH > 0 {
		cardEl = cardEl.Height(prefH)
	}
	st := renderState{pal: th.Palette()}
	return cardEl.Children(func() {
		if snap.Panel != nil {
			renderNodeWith(c, snap.Panel, th, opts, st)
		} else if snap.Error != "" {
			ui.Column(c).Padding(16).Gap(8).Children(func() {
				ui.Text(c, snap.Error).FontSize(12).TextColor(th.Danger)
			})
		} else {
			loading := opts.LoadingText
			if loading == "" {
				loading = "Loading..."
			}
			ui.Column(c).Padding(16).Gap(8).Children(func() {
				ui.Text(c, loading).FontSize(12).TextColor(th.TextMuted)
			})
		}
	})
}

// RenderNode renders one bar.Node subtree.
func RenderNode(c *ui.Context, n *bar.Node, th Theme, opts RenderOptions) {
	if n == nil {
		return
	}
	st := renderState{pal: th.Palette()}
	if c != nil && c.Theme().Dark != th.Dark {
		ui.Themed(c, barUITheme(th)).Children(func() {
			renderNodeWith(c, n, th, opts, st)
		})
		return
	}
	renderNodeWith(c, n, th, opts, st)
}

func renderNodeWith(c *ui.Context, n *bar.Node, th Theme, opts RenderOptions, st renderState) {
	if n == nil {
		return
	}
	props := n.Props
	st = st.withProps(props)

	switch n.Type {
	case "Spacer":
		ui.Spacer(c)
		return
	case "Divider":
		col := st.pal.Border
		if cs, ok := PropString(props, "color"); ok {
			col = th.Resolve(cs, colortheme.RoleBorder, st.pal, st.rawColor, col)
		}
		ui.Box(c).FillWidth().Height(1).Background(col)
		return
	case "Text":
		renderText(c, n, th, st)
		return
	case "Icon":
		renderIcon(c, n, th, opts, st)
		return
	case "Button":
		renderButton(c, n, th, opts, st)
		return
	case "Badge":
		renderBadge(c, n, th, st)
		return
	case "Progress":
		renderProgress(c, n, th, st)
		return
	case "Ring":
		renderRing(c, n, th, st)
		return
	case "StatusDot":
		renderStatusDot(c, n, th, st)
		return
	case "Segmented":
		renderSegmented(c, n, th, opts, st)
		return
	case "Switch":
		renderSwitch(c, n, th, opts, st)
		return
	case "ListRow":
		renderListRow(c, n, th, opts, st)
		return
	case "Image":
		renderImage(c, n, th)
		return
	case "Input":
		renderInput(c, n, th, opts, st)
		return
	case "Chart":
		renderChart(c, n, th, st)
		return
	}

	var el ui.Element
	switch n.Type {
	case "Row":
		el = ui.Row(c)
	case "Scroll":
		el = ui.Scroll(c)
	case "Box":
		el = ui.Box(c)
	default:
		el = ui.Column(c)
	}

	el, childPal := applyBoxProps(c, el, props, th, st, n.OnClickID != "")
	childState := renderState{pal: childPal, rawColor: st.rawColor}

	for keySpec, aid := range n.Shortcuts {
		if mod, key, ok := parseShortcutSpec(keySpec); ok {
			if el.OverlayShortcut(mod, key) {
				opts.fireAction(aid)
			}
		}
	}

	if n.OnClickID != "" {
		el = el.Cursor(ui.CursorPointer)
	}
	if el.Children(func() {
		for _, ch := range n.Children {
			renderNodeWith(c, ch, th, opts, childState)
		}
	}).Clicked() && n.OnClickID != "" {
		opts.fireAction(n.OnClickID)
	}
}

func renderText(c *ui.Context, n *bar.Node, th Theme, st renderState) {
	props := n.Props
	txt := n.Text
	if txt == "" && len(n.Children) > 0 {
		var parts []string
		for _, ch := range n.Children {
			if ch.Text != "" {
				parts = append(parts, ch.Text)
			}
		}
		txt = strings.Join(parts, "")
	}
	el := ui.Text(c, txt)
	if fs, ok := PropFloat(props, "fontSize"); ok && fs > 0 {
		el = el.FontSize(fs)
	} else {
		el = el.FontSize(13)
	}
	if fw, ok := PropFloat(props, "fontWeight"); ok && fw > 0 {
		el = el.FontWeight(int(fw))
	}
	if ls, ok := PropFloat(props, "letterSpacing"); ok {
		el = el.LetterSpacing(ls)
	}
	if ff, ok := PropString(props, "font"); ok && ff != "" {
		switch strings.ToLower(strings.TrimSpace(ff)) {
		case "mono", "monospace":
			el = el.Font("monospace")
		case "sans", "system":
		default:
			el = el.Font(ff)
		}
	}
	if feats := propStringSlice(props, "fontFeatures"); len(feats) > 0 {
		el = el.FontFeatures(feats...)
	}
	col := st.pal.Text
	if tc, ok := PropString(props, "textColor"); ok {
		col = th.Resolve(tc, colortheme.RoleText, st.pal, st.rawColor, st.pal.Text)
	}
	el = el.TextColor(col)

	if op, ok := PropFloat(props, "opacity"); ok {
		el = el.Opacity(op)
	}
	if ts, ok := props["textShadow"].(map[string]any); ok {
		x, _ := PropFloat(ts, "x")
		y, _ := PropFloat(ts, "y")
		blur, okB := PropFloat(ts, "blur")
		if !okB {
			blur = 3
		}
		sc := ui.RGBA(0, 0, 0, 0.45)
		if cs, ok := PropString(ts, "color"); ok {
			sc = th.Color(cs, sc)
		}
		el = el.TextShadow(x, y, blur, sc)
	}
	if ta, ok := PropString(props, "textAlign"); ok {
		switch ta {
		case "center":
			el = el.TextAlign(ui.Center)
		case "end", "right":
			el = el.TextAlign(ui.End)
		case "start", "left":
			el = el.TextAlign(ui.Start)
		}
	}
	if propBool(props, "singleLine", false) {
		el = el.SingleLine().Ellipsis("…")
	} else if w, ok := props["wrap"].(bool); ok && !w {
		el = el.NoWrap()
	}
	if ct := parseContentTransition(props["contentTransition"]); ct != nil {
		el = el.ContentTransition(*ct)
	}
	if pad, ok := PropFloatSlice(props, "padding"); ok && len(pad) > 0 {
		el = el.Padding(pad...)
	}
	applySizeProps(el, props)
}

func renderIcon(c *ui.Context, n *bar.Node, th Theme, opts RenderOptions, st renderState) {
	props := n.Props
	svgStr, _ := PropString(props, "svg")
	svg := ParseSVG(svgStr)
	if svg == nil {
		if name, ok := PropString(props, "name"); ok && name != "" {
			svg = opts.lookupIcon(name)
		}
	}
	if svg == nil {
		return
	}
	size := float32(16)
	if fs, ok := PropFloat(props, "fontSize"); ok && fs > 0 {
		size = fs
	} else if w, ok := PropFloat(props, "width"); ok && w > 0 {
		size = w
	}
	col := st.pal.Text
	if tc, ok := PropString(props, "textColor"); ok {
		col = th.Resolve(tc, colortheme.RoleText, st.pal, st.rawColor, st.pal.Text)
	}
	el := ui.Icon(c, svg).FontSize(size).TextColor(col)
	if ct := parseContentTransition(props["contentTransition"]); ct != nil {
		el = el.ContentTransition(*ct)
	}
	applySizeProps(el, props)

	if kf, ok := props["keyframes"].(map[string]any); ok {
		durMs := float32(900)
		if d, ok := PropFloat(kf, "duration"); ok && d > 0 {
			durMs = d
		}
		rep := 1
		if r, ok := PropFloat(kf, "repeat"); ok {
			if r < 0 {
				rep = ui.RepeatForever
			} else {
				rep = int(r)
			}
		}
		deg := el.Keyframes("spin", []ui.Keyframe{
			{At: 0, Value: 0},
			{At: time.Duration(durMs) * time.Millisecond, Value: 360},
		}, ui.KeyframeOptions{Repeat: rep})
		el.Rotate(deg)
	}
}

func renderButton(c *ui.Context, n *bar.Node, th Theme, opts RenderOptions, st renderState) {
	props := n.Props
	lbl := n.Text
	if lbl == "" && len(n.Children) > 0 {
		lbl = n.Children[0].Text
	}
	variant, _ := PropString(props, "variant")
	disabled := propBool(props, "disabled", false)

	baseTh := st.themeFor(th)
	fg := baseTh.Accent
	baseBG := baseTh.AccentSurface
	switch variant {
	case "primary":
		baseBG = th.Accent
		fg = colortheme.OnColor(baseBG)
	case "danger":
		baseBG = baseTh.DangerSurface
		fg = baseTh.Danger
	case "ghost":
		baseBG = ui.Color{}
		fg = st.pal.TextMuted
	case "soft":
		baseBG = baseTh.SurfaceSubtle
		fg = st.pal.Text
	}
	bs, hasBG := PropString(props, "background")
	if hasBG {
		baseBG = th.Resolve(bs, colortheme.RoleSurface, st.pal, st.rawColor, baseBG)
	}
	btnPal := st.pal.WithSurface(baseBG)
	tc, hasTC := PropString(props, "textColor")
	if variant == "primary" {
		fg = colortheme.OnColor(btnPal.Surface)
	} else if hasBG && !hasTC && baseBG.A >= 180 {
		fg = colortheme.OnColor(btnPal.Surface)
	}
	if hasTC {
		fg = th.Resolve(tc, colortheme.RoleText, btnPal, st.rawColor, fg)
	}

	b := ui.Row(c).
		Padding(6, 12).
		Radius(8).
		Gap(5).
		AlignItems(ui.Center).
		Justify(ui.Center)
	if !disabled {
		b = b.Cursor(ui.CursorPointer)
	}
	bg := baseBG
	solidFill := variant == "primary" || (hasBG && !colortheme.IsNeutral(baseBG) && baseBG.A >= 180)
	if !disabled {
		if b.Pressed() {
			if solidFill {
				bg = baseBG.Alpha(0.82)
			} else {
				bg = baseTh.SurfacePressed
			}
		} else if b.Hovered() {
			if solidFill {
				bg = baseBG.Alpha(0.90)
			} else {
				bg = baseTh.SurfaceHover
			}
		}
	} else {
		b = b.Opacity(0.5)
	}
	b = b.Background(bg)
	applySizeProps(b, props)

	var iconSVG *ui.SVG
	if ic, ok := PropString(props, "icon"); ok && ic != "" {
		iconSVG = opts.lookupIcon(ic)
	}
	if b.Children(func() {
		if iconSVG != nil {
			ui.Icon(c, iconSVG).FontSize(13).TextColor(fg)
		}
		if lbl != "" {
			ui.Text(c, lbl).FontSize(12).FontWeight(600).TextColor(fg).NoWrap()
		}
	}).Clicked() && !disabled && n.OnClickID != "" {
		opts.fireAction(n.OnClickID)
	}
}

func renderBadge(c *ui.Context, n *bar.Node, th Theme, st renderState) {
	props := n.Props
	txt := n.Text
	if txt == "" && len(n.Children) > 0 {
		txt = n.Children[0].Text
	}
	baseTh := st.themeFor(th)
	tone, hasTone := PropString(props, "tone")
	fg, bg := baseTh.Accent, baseTh.AccentSurface
	if hasTone {
		fg, bg = baseTh.toneColors(tone)
	}
	customLiteralBG := false
	if bs, ok := PropString(props, "background"); ok {
		handledSemanticBG := false
		if !hasTone {
			switch bs {
			case "warning":
				bg, fg, handledSemanticBG = baseTh.WarningSurface, baseTh.Warning, true
			case "danger", "error":
				bg, fg, handledSemanticBG = baseTh.DangerSurface, baseTh.Danger, true
			case "success":
				bg, fg, handledSemanticBG = baseTh.SuccessSurface, baseTh.Success, true
			case "accent", "primary":
				bg, fg, handledSemanticBG = baseTh.AccentSurface, baseTh.Accent, true
			}
		}
		if !handledSemanticBG {
			bg = th.Resolve(bs, colortheme.RoleSurface, st.pal, st.rawColor, bg)
			customLiteralBG = true
		}
	}
	badgePal := st.pal.WithSurface(bg)
	if tc, ok := PropString(props, "textColor"); ok {
		fg = th.Resolve(tc, colortheme.RoleText, badgePal, st.rawColor, fg)
	} else if customLiteralBG {
		if bg.A >= 180 && colortheme.RelativeLuminance(badgePal.Surface) >= 0.35 {
			fg = colortheme.OnColor(badgePal.Surface)
		} else {
			fg = colortheme.EnsureContrast(fg, badgePal.Surface, 4.5)
		}
	}
	fs := float32(11)
	if v, ok := PropFloat(props, "fontSize"); ok && v > 0 {
		fs = v
	}
	showDot := propBool(props, "dot", false)
	dotAnim, _ := PropString(props, "dotAnimation")
	dotPulse := propBool(props, "pulse", false)
	effDotAnim := NormalizeDotAnimation(dotAnim, dotPulse)
	dotSpeed := 1.0
	if sv, ok := PropFloat(props, "dotSpeed"); ok && sv > 0 {
		dotSpeed = float64(sv)
	}
	dotCnt := 0
	if cv, ok := PropFloat(props, "dotCount"); ok && cv > 0 {
		dotCnt = int(cv)
	}
	b := ui.Row(c).Padding(2, 8).Radius(999).Gap(5).Background(bg).AlignItems(ui.Center).Justify(ui.Center)
	b = applySizeProps(b, props)
	b.Children(func() {
		if showDot {
			if effDotAnim != "none" {
				dw, dh := DotAnimationBoxSize(effDotAnim, 5, dotCnt)
				ui.Box(c).Size(dw, dh).Draw(func(pt *ui.Painter, r ui.Rect) {
					PaintDotAnimation(pt, r, effDotAnim, fg, 5, dotCnt, dotSpeed, true)
				})
			} else {
				ui.Box(c).Size(5, 5).Radius(2.5).Background(fg)
			}
		}
		ui.Text(c, txt).FontSize(fs).FontWeight(600).TextColor(fg).NoWrap()
	})
}

func renderProgress(c *ui.Context, n *bar.Node, th Theme, st renderState) {
	props := n.Props
	val, _ := PropFloat(props, "value")
	maxVal, hasMax := PropFloat(props, "max")
	if !hasMax || maxVal <= 0 {
		maxVal = 100
	}
	frac := val / maxVal
	if frac < 0 {
		frac = 0
	} else if frac > 1 {
		frac = 1
	}
	pct := frac * 100

	style, _ := PropString(props, "style")
	style = strings.ToLower(strings.TrimSpace(style))

	h := float32(6)
	if style == "matrix" || style == "neon" {
		h = 16
	}
	if hv, ok := PropFloat(props, "height"); ok && hv > 0 {
		h = hv
	}
	rad := h / 2
	if rv, ok := PropFloat(props, "radius"); ok && rv >= 0 {
		rad = rv
	}

	baseTh := st.themeFor(th)
	effPal := st.pal
	if style == "matrix" || style == "neon" {
		effPal = colortheme.DefaultPalette(true)
	}
	fillCol := baseTh.Accent
	if fc, ok := PropString(props, "color"); ok {
		fillCol = th.Resolve(fc, colortheme.RoleAccent, effPal, st.rawColor, fillCol)
	}
	toCol := fillCol.Mix(ui.Hex("#FFFFFF"), 0.25)
	if tc, ok := PropString(props, "toColor"); ok {
		toCol = th.Resolve(tc, colortheme.RoleAccent, effPal, st.rawColor, toCol)
	} else if style == "gradient" {
		toCol = fillCol.Mix(baseTh.Cyan, 0.55)
	}
	trackCol := baseTh.SurfaceHover
	if tc, ok := PropString(props, "trackColor"); ok {
		trackCol = th.Resolve(tc, colortheme.RoleSurface, st.pal, st.rawColor, trackCol)
	} else if tc, ok := PropString(props, "background"); ok {
		trackCol = th.Resolve(tc, colortheme.RoleSurface, st.pal, st.rawColor, trackCol)
	}

	if style == "segmented" {
		segs := 10
		if sv, ok := PropFloat(props, "segments"); ok && sv >= 2 {
			segs = int(sv)
			if segs > 24 {
				segs = 24
			}
		}
		lit := int(math.Round(float64(frac) * float64(segs)))
		row := ui.Row(c).Height(h).Gap(3).AlignItems(ui.Stretch)
		if propBool(props, "fillWidth", true) {
			row = row.FillWidth()
		}
		row = applySizeProps(row, props)
		row.Children(func() {
			for i := 0; i < segs; i++ {
				col := trackCol
				if i < lit {
					col = fillCol
				}
				ui.Box(c.Key(fmt.Sprintf("pseg-%d", i))).Grow(1).Shrink(1).Basis(0).MinWidth(0).FillHeight().Radius(rad).Background(col)
			}
		})
		return
	}

	if style == "matrix" || style == "neon" {
		animate := propBool(props, "animate", false)
		box := ui.Box(c).Height(h).Radius(6).Background(ui.Hex("#070A10")).BorderWidth(1).BorderColor(ui.RGBA(255, 255, 255, 0.08)).Clip()
		if propBool(props, "fillWidth", true) {
			box = box.FillWidth()
		}
		box = applySizeProps(box, props)
		box.Draw(func(pt *ui.Painter, r ui.Rect) {
			if r.W <= 12 || r.H <= 6 {
				return
			}
			var sec float64
			if animate {
				pt.AnimationFrameRate(24)
				sec = float64(pt.Now().UnixNano()) / 1e9
			}
			padX, padY := float32(6), float32(4)
			innerW := r.W - padX*2
			innerH := r.H - padY*2
			if innerW <= 4 || innerH <= 2 {
				return
			}
			rows := 2
			if innerH < 8 {
				rows = 1
			} else if innerH >= 18 {
				rows = 3
			}
			step := float32(6.5)
			cols := int(innerW / step)
			if cols < 8 {
				cols = 8
			}
			colStep := innerW / float32(cols)
			rowStep := innerH / float32(rows)
			dotR := clampF(min(colStep, rowStep)*0.30, 1.3, 2.8)
			litCols := int(math.Round(float64(frac) * float64(cols)))
			for cxIdx := 0; cxIdx < cols; cxIdx++ {
				cx := r.X + padX + (float32(cxIdx)+0.5)*colStep
				for ryIdx := 0; ryIdx < rows; ryIdx++ {
					cy := r.Y + padY + (float32(ryIdx)+0.5)*rowStep
					if cxIdx < litCols {
						intensity := float32(0.88)
						whiteMix := float32(0.32)
						if animate {
							wave := float32(0.5 + 0.5*math.Sin(sec*3.0-float64(cxIdx)*0.35))
							intensity = 0.72 + 0.28*wave
							whiteMix = 0.20 + 0.35*wave
						}
						paintLED(pt, cx, cy, dotR, fillCol, intensity, whiteMix)
					} else {
						pt.Fill(ui.Rect{X: cx - dotR*0.8, Y: cy - dotR*0.8, W: dotR * 1.6, H: dotR * 1.6}, ui.RGBA(255, 255, 255, 0.08), dotR*0.8)
					}
				}
			}
		})
		return
	}

	track := ui.Box(c).Height(h).Radius(rad).Background(trackCol).Clip()
	if propBool(props, "fillWidth", true) {
		track = track.FillWidth()
	}
	track = applySizeProps(track, props)
	track.Children(func() {
		if pct > 0 {
			barEl := ui.Box(c).Absolute().Top(0).Bottom(0).Left(0).WidthPercent(pct).Radius(rad)
			if style == "gradient" {
				barEl = barEl.LinearGradient(ui.LinearGradient{
					From:  fillCol,
					To:    toCol,
					Angle: 90,
					Oklab: true,
				})
			} else {
				barEl = barEl.Background(fillCol)
			}
		}
	})
}

func renderRing(c *ui.Context, n *bar.Node, th Theme, st renderState) {
	props := n.Props
	val, _ := PropFloat(props, "value")
	maxVal, hasMax := PropFloat(props, "max")
	if !hasMax || maxVal <= 0 {
		maxVal = 100
	}
	frac := float64(val / maxVal)
	if frac < 0 {
		frac = 0
	} else if frac > 1 {
		frac = 1
	}

	size := float32(74)
	if sv, ok := PropFloat(props, "size"); ok && sv >= 28 {
		size = sv
	}
	thick := float32(7)
	if tv, ok := PropFloat(props, "thickness"); ok && tv >= 2 {
		thick = tv
	}
	style, _ := PropString(props, "style")
	style = strings.ToLower(strings.TrimSpace(style))

	baseTh := st.themeFor(th)
	effPal := st.pal
	if style == "matrix" {
		effPal = colortheme.DefaultPalette(true)
	}
	col := baseTh.Accent
	if cs, ok := PropString(props, "color"); ok {
		col = th.Resolve(cs, colortheme.RoleAccent, effPal, st.rawColor, col)
	}
	toCol := col
	if ts, ok := PropString(props, "toColor"); ok {
		toCol = th.Resolve(ts, colortheme.RoleAccent, effPal, st.rawColor, col)
	} else if style == "gradient" {
		toCol = baseTh.Cyan
	}
	trackCol := baseTh.SurfaceHover
	if ts, ok := PropString(props, "trackColor"); ok {
		trackCol = th.Resolve(ts, colortheme.RoleSurface, st.pal, st.rawColor, trackCol)
	}

	label, _ := PropString(props, "label")
	sublabel, _ := PropString(props, "sublabel")

	wrap := ui.Box(c).Size(size, size)
	wrap = applySizeProps(wrap, props)
	wrap.Draw(func(pt *ui.Painter, r ui.Rect) {
		cx := r.X + r.W/2
		cy := r.Y + r.H/2
		rad := min(r.W, r.H)/2 - thick/2 - 1
		if rad <= 4 {
			return
		}
		if style == "matrix" {
			discR := min(r.W, r.H) / 2
			pt.Fill(ui.Rect{X: cx - discR, Y: cy - discR, W: discR * 2, H: discR * 2}, ui.Hex("#070A10"), discR)
			pt.StrokePath((&ui.Path{}).Circle(cx, cy, discR-0.5), 1, ui.RGBA(255, 255, 255, 0.08))
			rings := 2
			dotsPerRing := 22
			band := clampF(discR*0.24, 6, 12)
			outerR := discR - 4.5
			dotR := clampF(band/float32(rings)*0.30, 1.3, 2.4)
			for i := 0; i < dotsPerRing; i++ {
				ang := -math.Pi/2 + (float64(i)/float64(dotsPerRing))*2*math.Pi
				lit := float64(i)/float64(dotsPerRing) < frac
				for rr := 0; rr < rings; rr++ {
					ringR := outerR - float32(rr)*(band/float32(rings)) - dotR
					px := cx + float32(math.Cos(ang))*ringR
					py := cy + float32(math.Sin(ang))*ringR
					if lit {
						paintLED(pt, px, py, dotR, col, 0.92, 0.35)
					} else {
						pt.Fill(ui.Rect{X: px - dotR*0.8, Y: py - dotR*0.8, W: dotR * 1.6, H: dotR * 1.6}, ui.RGBA(255, 255, 255, 0.09), dotR*0.8)
					}
				}
			}
			return
		}

		pt.StrokePath((&ui.Path{}).Circle(cx, cy, rad), thick, trackCol)
		if frac > 0.005 {
			steps := 48
			end := -math.Pi/2 + frac*2*math.Pi
			p := (&ui.Path{}).Cap(ui.RoundCap)
			for i := 0; i <= steps; i++ {
				t := float64(i) / float64(steps)
				ang := -math.Pi/2 + t*(end+math.Pi/2)
				px := cx + float32(math.Cos(ang))*rad
				py := cy + float32(math.Sin(ang))*rad
				if i == 0 {
					p.MoveTo(px, py)
				} else {
					p.LineTo(px, py)
				}
			}
			if style == "gradient" || toCol != col {
				pt.StrokePathGradient(p, thick, ui.LinearGradient{
					From:  col,
					To:    toCol,
					Angle: 120,
					Oklab: true,
				})
			} else {
				pt.StrokePath(p, thick, col)
			}
		}
	})
	wrap.Children(func() {
		if label != "" || sublabel != "" {
			txtCol := st.pal.Text
			if style == "matrix" {
				txtCol = ui.Hex("#F1F4F9")
			}
			mutedCol := st.pal.TextMuted
			if style == "matrix" {
				mutedCol = ui.Hex("#9AA3B5")
			}
			ui.Column(c).Fill().Center().Gap(1).Children(func() {
				if label != "" {
					fs := clampF(size*0.22, 12, 22)
					ui.Text(c, label).FontSize(fs).FontWeight(700).FontFeatures("tnum").TextColor(txtCol).NoWrap()
				}
				if sublabel != "" {
					ui.Text(c, sublabel).FontSize(9).FontWeight(600).LetterSpacing(0.4).TextColor(mutedCol).NoWrap()
				}
			})
		}
	})
}

func renderStatusDot(c *ui.Context, n *bar.Node, th Theme, st renderState) {
	props := n.Props
	colName, _ := PropString(props, "color")
	col := th.Resolve(colName, colortheme.RoleAccent, st.pal, st.rawColor, st.themeFor(th).Success)
	size := float32(7)
	if sv, ok := PropFloat(props, "size"); ok && sv >= 4 {
		size = sv
	}
	animStr, _ := PropString(props, "animation")
	pulse := propBool(props, "pulse", false)
	effAnim := NormalizeDotAnimation(animStr, pulse)
	count := 0
	if cv, ok := PropFloat(props, "count"); ok && cv > 0 {
		count = int(cv)
	}
	speed := 1.0
	if spv, ok := PropFloat(props, "speed"); ok && spv > 0 {
		speed = float64(spv)
	}
	animate := propBool(props, "animate", true)

	bw, bh := DotAnimationBoxSize(effAnim, size, count)
	box := applySizeProps(ui.Box(c).Size(bw, bh), props)
	box.Draw(func(pt *ui.Painter, r ui.Rect) {
		PaintDotAnimation(pt, r, effAnim, col, size, count, speed, animate)
	})
}

func renderSegmented(c *ui.Context, n *bar.Node, th Theme, opts RenderOptions, st renderState) {
	props := n.Props
	curVal, _ := PropString(props, "value")
	rawOpts, _ := props["options"].([]any)
	type segItem struct {
		val, lbl string
	}
	var items []segItem
	for _, raw := range rawOpts {
		switch x := raw.(type) {
		case string:
			items = append(items, segItem{val: x, lbl: x})
		case map[string]any:
			v, _ := PropString(x, "value")
			l, okL := PropString(x, "label")
			if !okL || l == "" {
				l = v
			}
			if v != "" || l != "" {
				items = append(items, segItem{val: v, lbl: l})
			}
		}
	}
	if len(items) == 0 {
		return
	}

	baseTh := st.themeFor(th)
	tray := ui.Row(c).Padding(2).Radius(9).Gap(2).Background(baseTh.SurfaceSubtle)
	if !baseTh.Vibrancy {
		tray = tray.BorderWidth(1).BorderColor(st.pal.Border)
	}
	if propBool(props, "fillWidth", true) {
		tray = tray.FillWidth()
	}
	tray = applySizeProps(tray, props)
	tray.Children(func() {
		for idx, it := range items {
			it := it
			active := it.val == curVal
			btn := ui.Row(c.Key(fmt.Sprintf("seg-%d-%s", idx, it.val))).
				Grow(1).Shrink(1).Basis(0).MinWidth(0).
				Padding(5, 8).Radius(7).Center().Cursor(ui.CursorPointer)
			fg := st.pal.TextMuted
			if active {
				btn = btn.Background(baseTh.SurfaceElevated).Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.14))
				fg = st.pal.Text
			} else if btn.Hovered() {
				btn = btn.Background(baseTh.SurfaceHover)
				fg = st.pal.Text
			}
			if btn.Children(func() {
				ui.Text(c, it.lbl).FontSize(11).FontWeight(600).TextColor(fg).SingleLine().Ellipsis("…")
			}).Clicked() && !active {
				if n.OnChangeID != "" {
					opts.fireActionArg(n.OnChangeID, it.val)
				} else if n.OnClickID != "" {
					opts.fireActionArg(n.OnClickID, it.val)
				}
			}
		}
	})
}

func renderSwitch(c *ui.Context, n *bar.Node, th Theme, opts RenderOptions, st renderState) {
	props := n.Props
	checked := propBool(props, "checked", false)
	disabled := propBool(props, "disabled", false)
	baseTh := st.themeFor(th)
	onCol := baseTh.Accent
	if cs, ok := PropString(props, "color"); ok {
		onCol = th.Resolve(cs, colortheme.RoleAccent, st.pal, st.rawColor, onCol)
	}
	trackCol := baseTh.SurfaceHover
	if checked {
		trackCol = onCol
	}

	el := ui.Box(c).Size(36, 20).Radius(10).Background(trackCol).BorderWidth(1).BorderColor(st.pal.Border)
	if checked {
		el = el.BorderColor(trackCol)
	}
	if !disabled {
		el = el.Cursor(ui.CursorPointer)
	} else {
		el = el.Opacity(0.5)
	}
	el = applySizeProps(el, props)
	el.Children(func() {
		thumb := ui.Box(c).Absolute().Top(2).Size(14, 14).Radius(7).
			Background(ui.Hex("#FFFFFF")).
			Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.28))
		if checked {
			thumb.Right(2)
		} else {
			thumb.Left(2)
		}
	})
	if el.Clicked() && !disabled {
		next := !checked
		if n.OnChangeID != "" {
			opts.fireActionArg(n.OnChangeID, next)
		} else if n.OnClickID != "" {
			opts.fireActionArg(n.OnClickID, next)
		}
	}
}

func renderListRow(c *ui.Context, n *bar.Node, th Theme, opts RenderOptions, st renderState) {
	props := n.Props
	baseTh := st.themeFor(th)
	title, _ := PropString(props, "title")
	if title == "" {
		title = n.Text
	}
	subtitle, _ := PropString(props, "subtitle")
	value, _ := PropString(props, "value")
	badge, _ := PropString(props, "badge")
	badgeTone, _ := PropString(props, "badgeTone")
	dotName, hasDot := PropString(props, "dot")
	iconName, _ := PropString(props, "icon")
	shortcut, _ := PropString(props, "shortcut")
	chevron := propBool(props, "chevron", false)
	active := propBool(props, "active", false)
	disabled := propBool(props, "disabled", false)
	pulse := propBool(props, "pulse", false)
	dotAnim, _ := PropString(props, "dotAnimation")
	effDotAnim := NormalizeDotAnimation(dotAnim, pulse)
	dotSpeed := 1.0
	if sv, ok := PropFloat(props, "dotSpeed"); ok && sv > 0 {
		dotSpeed = float64(sv)
	}
	dotCnt := 0
	if cv, ok := PropFloat(props, "dotCount"); ok && cv > 0 {
		dotCnt = int(cv)
	}

	clickable := !disabled && n.OnClickID != ""
	row := ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(9).Padding(7, 9).Radius(9)
	if clickable {
		row = row.Cursor(ui.CursorPointer)
	}
	bg := ui.Color{}
	if active {
		bg = baseTh.AccentSurface
	} else if clickable && row.Pressed() {
		bg = baseTh.SurfacePressed
	} else if clickable && row.Hovered() {
		bg = baseTh.SurfaceHover
	}
	if bg != (ui.Color{}) {
		row = row.Background(bg)
	}
	if disabled {
		row = row.Opacity(0.5)
	}
	row = applySizeProps(row, props)

	valCol := st.pal.TextMuted
	if vc, ok := PropString(props, "valueColor"); ok {
		valCol = th.Resolve(vc, colortheme.RoleText, st.pal, st.rawColor, valCol)
	}
	iconSVG := opts.lookupIcon(iconName)
	chevSVG := opts.lookupIcon("chevron-right")

	var childrenRes ui.Element
	childrenRes = row.Children(func() {
		if hasDot && dotName != "" {
			dotCol := th.Resolve(dotName, colortheme.RoleAccent, st.pal, st.rawColor, baseTh.Success)
			dw, dh := DotAnimationBoxSize(effDotAnim, 7, dotCnt)
			ui.Box(c).Size(dw, dh).Draw(func(pt *ui.Painter, r ui.Rect) {
				PaintDotAnimation(pt, r, effDotAnim, dotCol, 7, dotCnt, dotSpeed, true)
			})
		}
		if iconSVG != nil {
			icTone, hasIcTone := PropString(props, "iconColor")
			plain := propBool(props, "iconPlain", false)
			if ib, ok := PropString(props, "iconBg"); ok && strings.EqualFold(strings.TrimSpace(ib), "transparent") {
				plain = true
			}
			if plain {
				icFg := st.pal.TextMuted
				if hasIcTone && icTone != "" {
					if tfg, _, ok := baseTh.lookupTone(icTone); ok {
						icFg = tfg
					} else {
						icFg = th.Resolve(icTone, colortheme.RoleAccent, st.pal, st.rawColor, st.pal.TextMuted)
					}
				}
				ui.Box(c).Size(18, 18).Center().Children(func() {
					ui.Icon(c, iconSVG).FontSize(15).TextColor(icFg)
				})
			} else {
				icFg, icBg := baseTh.toneColors(icTone)
				if icTone != "" {
					if _, _, ok := baseTh.lookupTone(icTone); !ok {
						icFg = th.Resolve(icTone, colortheme.RoleAccent, st.pal, st.rawColor, baseTh.Accent)
						icBg = icFg.Alpha(0.15)
					}
				}
				if ib, ok := PropString(props, "iconBg"); ok {
					icBg = th.Resolve(ib, colortheme.RoleSurface, st.pal, st.rawColor, icBg)
					if icBg.A > 0 && !hasIcTone {
						icFg = colortheme.OnColor(st.pal.WithSurface(icBg).Surface)
					}
				}
				ui.Box(c).Size(24, 24).Radius(7).Background(icBg).Center().Children(func() {
					ui.Icon(c, iconSVG).FontSize(13).TextColor(icFg)
				})
			}
		}
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(1).Children(func() {
			ui.Text(c, title).FontSize(12.5).FontWeight(550).TextColor(st.pal.Text).SingleLine().Ellipsis("…")
			if subtitle != "" {
				ui.Text(c, subtitle).FontSize(10.5).TextColor(st.pal.TextMuted).SingleLine().Ellipsis("…")
			}
		})
		if badge != "" {
			bt := badgeTone
			if bt == "" {
				bt = "accent"
			}
			bfg, bbg := baseTh.toneColors(bt)
			ui.Row(c).Padding(2, 6).Radius(999).Background(bbg).Center().Children(func() {
				ui.Text(c, badge).FontSize(10).FontWeight(600).TextColor(bfg).NoWrap()
			})
		}
		if value != "" {
			ui.Text(c, value).FontSize(11.5).FontWeight(600).FontFeatures("tnum").TextColor(valCol).NoWrap()
		}
		if shortcut != "" {
			ui.Row(c).Padding(2, 5).Radius(5).Background(baseTh.SurfaceSubtle).BorderWidth(1).BorderColor(st.pal.Border).Center().Children(func() {
				ui.Text(c, shortcut).FontSize(10).Font("monospace").TextColor(st.pal.TextMuted).NoWrap()
			})
		}
		if chevron && chevSVG != nil {
			ui.Icon(c, chevSVG).FontSize(13).TextColor(baseTh.TextFaint)
		}
	})
	if clickable && childrenRes.Clicked() {
		opts.fireAction(n.OnClickID)
	}
}

var (
	imgCacheMu sync.Mutex
	imgCache   = map[string]ui.ImageSource{}
)

func decodeDataURLImage(src string) ui.ImageSource {
	if !strings.HasPrefix(src, "data:image/") {
		return nil
	}
	imgCacheMu.Lock()
	if cached, ok := imgCache[src]; ok {
		imgCacheMu.Unlock()
		return cached
	}
	imgCacheMu.Unlock()

	idx := strings.Index(src, ";base64,")
	if idx < 0 {
		return nil
	}
	raw, err := base64.StdEncoding.DecodeString(src[idx+8:])
	if err != nil || len(raw) == 0 {
		return nil
	}
	decoded, err := ui.DecodeImage(raw)
	if err != nil {
		return nil
	}
	imgCacheMu.Lock()
	if len(imgCache) > 64 {
		imgCache = map[string]ui.ImageSource{}
	}
	imgCache[src] = decoded
	imgCacheMu.Unlock()
	return decoded
}

func renderImage(c *ui.Context, n *bar.Node, th Theme) {
	props := n.Props
	srcStr, _ := PropString(props, "src")
	img := decodeDataURLImage(srcStr)
	if img == nil {
		w := float32(36)
		h := float32(36)
		if wv, ok := PropFloat(props, "width"); ok && wv > 0 {
			w = wv
		}
		if hv, ok := PropFloat(props, "height"); ok && hv > 0 {
			h = hv
		}
		rad := float32(8)
		if rv, ok := PropFloat(props, "radius"); ok && rv >= 0 {
			rad = rv
		}
		ui.Box(c).Size(w, h).Radius(rad).Background(th.SurfaceSubtle)
		return
	}
	fit := ui.Contain
	if fs, ok := PropString(props, "fit"); ok && strings.EqualFold(fs, "cover") {
		fit = ui.Cover
	}
	el := ui.Image(c, img).Fit(fit)
	if rad, ok := PropFloatSlice(props, "radius"); ok && len(rad) > 0 {
		el = el.Radius(rad...)
	}
	if ct := parseContentTransition(props["contentTransition"]); ct != nil {
		el = el.ContentTransition(*ct)
	}
	applySizeProps(el, props)
}

func renderInput(c *ui.Context, n *bar.Node, th Theme, opts RenderOptions, st renderState) {
	props := n.Props
	baseTh := st.themeFor(th)
	val, _ := PropString(props, "value")
	ph, _ := PropString(props, "placeholder")
	keyHint := n.OnChangeID + ":" + n.OnSubmitID + ":" + ph

	wrap := ui.Row(c.Key("in-wrap:"+keyHint)).
		Padding(6, 10).
		Radius(8).
		Background(baseTh.SurfaceSubtle).
		BorderWidth(1).
		BorderColor(st.pal.Border).
		AlignItems(ui.Center)
	if propBool(props, "fillWidth", true) {
		wrap = wrap.FillWidth()
	}
	wrap = applySizeProps(wrap, props)

	cur := val
	wrap.Children(func() {
		ti := ui.TextInput(c.Key("in:"+keyHint), &cur).
			FillWidth().
			FontSize(12.5).
			TextColor(st.pal.Text)
		if ph != "" {
			ti = ti.Placeholder(ph)
		}
		if ti.Changed() && n.OnChangeID != "" {
			opts.fireActionArg(n.OnChangeID, cur)
		}
		if ti.Submitted() && n.OnSubmitID != "" {
			opts.fireActionArg(n.OnSubmitID, cur)
		}
	})
}

func renderChart(c *ui.Context, n *bar.Node, th Theme, st renderState) {
	props := n.Props
	baseTh := st.themeFor(th)
	h := float32(40)
	if v, ok := PropFloat(props, "height"); ok && v > 0 {
		h = v
	}
	kind := strings.ToLower(strings.TrimSpace(asPropStr(props["type"])))
	if kind == "" {
		kind = "line"
	}

	type chartPt struct{ x, y float64 }
	type chartSeries struct {
		col      ui.Color
		fillFrom float32
		fillTo   float32
		hasFill  bool
		pts      []chartPt
	}
	var allSeries []chartSeries
	if rawList, ok := props["series"].([]any); ok {
		for _, item := range rawList {
			sm, ok := item.(map[string]any)
			if !ok {
				continue
			}
			col := baseTh.Accent
			if cs, ok := PropString(sm, "color"); ok {
				col = th.Resolve(cs, colortheme.RoleAccent, st.pal, st.rawColor, col)
			}
			cs := chartSeries{col: col}
			if fm, ok := sm["fill"].(map[string]any); ok {
				cs.hasFill = true
				if f0, ok := PropFloat(fm, "from"); ok {
					cs.fillFrom = f0
				} else {
					cs.fillFrom = 0.18
				}
				if f1, ok := PropFloat(fm, "to"); ok {
					cs.fillTo = f1
				}
			} else if kind == "area" || kind == "sparkline" {
				cs.hasFill = true
				cs.fillFrom = 0.18
				cs.fillTo = 0
			}
			if rawPts, ok := sm["points"].([]any); ok {
				for _, rp := range rawPts {
					pm, ok := rp.(map[string]any)
					if !ok {
						continue
					}
					x, okX := toFloat64(pm["x"])
					y, okY := toFloat64(pm["y"])
					if okX && okY && !math.IsNaN(x) && !math.IsNaN(y) {
						cs.pts = append(cs.pts, chartPt{x, y})
					}
				}
			}
			if len(cs.pts) > 0 {
				allSeries = append(allSeries, cs)
			}
		}
	}

	strokeW := float32(2)
	if swv, ok := PropFloat(props, "strokeWidth"); ok && swv > 0 {
		strokeW = swv
	}
	curveKind := chart.CurveLinear
	if cv, ok := PropString(props, "curve"); ok && cv == "monotone" || kind == "sparkline" {
		curveKind = chart.CurveMonotone
	}

	box := ui.Box(c).Height(h)
	if propBool(props, "fillWidth", true) {
		box = box.FillWidth()
	}
	applySizeProps(box, props)

	if len(allSeries) == 0 {
		return
	}

	xMin, xMax := math.Inf(1), math.Inf(-1)
	yMin, yMax := math.Inf(1), math.Inf(-1)
	for _, s := range allSeries {
		for _, pt := range s.pts {
			xMin = math.Min(xMin, pt.x)
			xMax = math.Max(xMax, pt.x)
			yMin = math.Min(yMin, pt.y)
			yMax = math.Max(yMax, pt.y)
		}
	}
	var hasRef bool
	var refVal float64
	refCol := baseTh.Warning.Alpha(0.65)
	if rf, ok := toFloat64(props["referenceLine"]); ok {
		hasRef, refVal = true, rf
	} else if rm, ok := props["referenceLine"].(map[string]any); ok {
		if v, ok := toFloat64(rm["value"]); ok {
			hasRef, refVal = true, v
			if cs, ok := PropString(rm, "color"); ok {
				refCol = th.Resolve(cs, colortheme.RoleAccent, st.pal, st.rawColor, refCol).Alpha(0.75)
			}
		}
	}
	if hasRef {
		yMin = math.Min(yMin, refVal)
		yMax = math.Max(yMax, refVal)
	}

	if xd, ok := props["xDomain"].([]any); ok && len(xd) >= 2 {
		if a, ok1 := toFloat64(xd[0]); ok1 {
			xMin = a
		}
		if b, ok2 := toFloat64(xd[1]); ok2 {
			xMax = b
		}
	}
	if kind == "bar" && yMin > 0 {
		yMin = 0
	}
	xSpan := xMax - xMin
	if xSpan <= 0 {
		xSpan = 1
	}
	ySpan := yMax - yMin
	if ySpan <= 0 {
		yMin -= 1
		yMax += 1
		ySpan = 2
	} else {
		pad := ySpan * 0.14
		if kind != "bar" {
			yMin -= pad
		}
		yMax += pad
		ySpan = yMax - yMin
	}

	box.Draw(func(p *ui.Painter, r ui.Rect) {
		if r.W <= 1 || r.H <= 1 {
			return
		}
		if hasRef {
			ry := chart.Y(r, float32((refVal-yMin)/ySpan))
			refPath := (&ui.Path{}).MoveTo(r.X, ry).LineTo(r.X+r.W, ry).Dash(0, 4, 4)
			p.StrokePath(refPath, 1, refCol)
		}
		if kind == "bar" {
			s := allSeries[0]
			nPts := len(s.pts)
			if nPts == 0 {
				return
			}
			slotW := r.W / float32(nPts)
			barW := clampF(slotW*0.64, 3, 28)
			for i, pt := range s.pts {
				norm := clampF(float32((pt.y-yMin)/ySpan), 0.04, 1)
				bh := r.H * norm
				bx := r.X + float32(i)*slotW + (slotW-barW)/2
				by := r.Y + r.H - bh
				col := s.col
				if i == nPts-1 {
					col = col.Mix(ui.Hex("#FFFFFF"), 0.12)
				}
				p.Fill(ui.Rect{X: bx, Y: by, W: barW, H: bh}, col, clampF(barW/3, 2, 5))
			}
			return
		}

		for _, s := range allSeries {
			if len(s.pts) < 2 {
				continue
			}
			xs := make([]float32, len(s.pts))
			ys := make([]float32, len(s.pts))
			for i, pt := range s.pts {
				xs[i] = float32((pt.x - xMin) / xSpan)
				ys[i] = float32((pt.y - yMin) / ySpan)
			}
			if s.hasFill {
				p.FillPathGradient(chart.Area(r, xs, nil, ys, curveKind), ui.LinearGradient{
					From: s.col.Alpha(s.fillFrom), To: s.col.Alpha(s.fillTo), Angle: 180,
				})
			}
			p.StrokePath(chart.Line(r, xs, ys, curveKind), strokeW, s.col)
			if kind == "sparkline" {
				lx := chart.X(r, xs[len(xs)-1])
				ly := chart.Y(r, ys[len(ys)-1])
				p.Fill(ui.Rect{X: lx - 3.5, Y: ly - 3.5, W: 7, H: 7}, s.col.Alpha(0.28), 3.5)
				p.Fill(ui.Rect{X: lx - 2, Y: ly - 2, W: 4, H: 4}, s.col, 2)
			}
		}
	})
}

func paintLED(pt *ui.Painter, cx, cy, dotR float32, col ui.Color, intensity, whiteMix float32) {
	white := ui.Hex("#FFFFFF")
	rOuter := dotR * 1.65
	pt.Fill(ui.Rect{X: cx - rOuter, Y: cy - rOuter, W: rOuter * 2, H: rOuter * 2}, col.Alpha(0.22*intensity), rOuter)
	body := col.Mix(white, whiteMix*0.75).Alpha(min(intensity, 1))
	pt.Fill(ui.Rect{X: cx - dotR, Y: cy - dotR, W: dotR * 2, H: dotR * 2}, body, dotR)
	if whiteMix > 0.25 {
		coreR := dotR * 0.48
		pt.Fill(ui.Rect{X: cx - coreR, Y: cy - coreR, W: coreR * 2, H: coreR * 2}, white.Alpha(0.75), coreR)
	}
}

func applyBoxProps(c *ui.Context, el ui.Element, props map[string]any, th Theme, st renderState, clickable bool) (ui.Element, colortheme.Palette) {
	childPal := st.pal
	if props == nil {
		return el, childPal
	}
	el = applySizeProps(el, props)

	if gap, ok := PropFloat(props, "gap"); ok {
		el = el.Gap(gap)
	}
	var padTop, padRight, padBottom, padLeft float32
	hasPad := false
	if pad, ok := PropFloatSlice(props, "padding"); ok && len(pad) > 0 {
		hasPad = true
		switch len(pad) {
		case 1:
			padTop, padRight, padBottom, padLeft = pad[0], pad[0], pad[0], pad[0]
		case 2:
			padTop, padRight, padBottom, padLeft = pad[0], pad[1], pad[0], pad[1]
		case 3:
			padTop, padRight, padBottom, padLeft = pad[0], pad[1], pad[2], pad[1]
		default:
			padTop, padRight, padBottom, padLeft = pad[0], pad[1], pad[2], pad[3]
		}
	}
	if c != nil && propBool(props, "safeArea", false) {
		saT, saR, saB, saL := c.SafeArea()
		if saT != 0 || saR != 0 || saB != 0 || saL != 0 {
			padTop += saT
			padRight += saR
			padBottom += saB
			padLeft += saL
			hasPad = true
		}
	}
	if hasPad {
		el = el.Padding(padTop, padRight, padBottom, padLeft)
	}
	if rad, ok := PropFloatSlice(props, "radius"); ok && len(rad) > 0 {
		el = el.Radius(rad...)
	}
	if ai, ok := PropString(props, "alignItems"); ok {
		if a, ok := parseAlign(ai); ok {
			el = el.AlignItems(a)
		}
	}
	if j, ok := PropString(props, "justify"); ok {
		if a, ok := parseAlign(j); ok {
			el = el.Justify(a)
		}
	}
	if propBool(props, "wrap", false) {
		el = el.Wrap()
	}
	if propBool(props, "clip", false) {
		el = el.Clip()
	}
	if op, ok := PropFloat(props, "opacity"); ok {
		el = el.Opacity(op)
	}
	if propBool(props, "absolute", false) {
		el = el.Absolute()
	}
	if v, ok := PropFloat(props, "top"); ok {
		el = el.Top(v)
	}
	if v, ok := PropFloat(props, "right"); ok {
		el = el.Right(v)
	}
	if v, ok := PropFloat(props, "bottom"); ok {
		el = el.Bottom(v)
	}
	if v, ok := PropFloat(props, "left"); ok {
		el = el.Left(v)
	}
	if v, ok := PropFloat(props, "leftPercent"); ok {
		el = el.LeftPercent(v)
	}
	if v, ok := PropFloat(props, "widthPercent"); ok {
		el = el.WidthPercent(v)
	}
	if v, ok := PropFloat(props, "topPercent"); ok {
		el = el.TopPercent(v)
	}
	if v, ok := PropFloat(props, "heightPercent"); ok {
		el = el.HeightPercent(v)
	}
	if bw, ok := PropFloatSlice(props, "borderWidth"); ok && len(bw) > 0 {
		el = el.BorderWidth(bw...)
	}
	if bs, ok := PropString(props, "borderStyle"); ok && strings.EqualFold(bs, "dashed") {
		el = el.BorderStyle(ui.BorderDashed)
	}
	if bc, ok := PropString(props, "borderColor"); ok {
		el = el.BorderColor(th.Resolve(bc, colortheme.RoleBorder, st.pal, st.rawColor, st.pal.Border))
	}
	if tip, ok := PropString(props, "tooltip"); ok && tip != "" {
		el = el.Tooltip(tip)
	}

	if shStr, ok := PropString(props, "shadow"); ok {
		switch strings.ToLower(strings.TrimSpace(shStr)) {
		case "sm":
			el = el.Shadow(0, 1, 4, 0, ui.RGBA(0, 0, 0, 0.14))
		case "md":
			el = el.Shadow(0, 4, 12, 0, ui.RGBA(0, 0, 0, 0.22))
		}
	} else if sm, ok := props["shadow"].(map[string]any); ok {
		x, _ := PropFloat(sm, "x")
		y, _ := PropFloat(sm, "y")
		blur, okB := PropFloat(sm, "blur")
		if !okB {
			blur = 8
		}
		spread, _ := PropFloat(sm, "spread")
		sc := ui.RGBA(0, 0, 0, 0.20)
		if cs, ok := PropString(sm, "color"); ok {
			sc = th.Color(cs, sc)
		}
		el = el.Shadow(x, y, blur, spread, sc)
	}

	if gm, ok := props["gradient"].(map[string]any); ok {
		baseTh := st.themeFor(th)
		fromCol := baseTh.Accent
		if fs, ok := PropString(gm, "from"); ok {
			fromCol = th.Resolve(fs, colortheme.RoleSurface, st.pal, st.rawColor, fromCol)
		}
		toCol := baseTh.Cyan
		if ts, ok := PropString(gm, "to"); ok {
			toCol = th.Resolve(ts, colortheme.RoleSurface, st.pal, st.rawColor, toCol)
		}
		angle := float32(135)
		if av, ok := PropFloat(gm, "angle"); ok {
			angle = av
		}
		lg := ui.LinearGradient{
			From:  fromCol,
			To:    toCol,
			Angle: angle,
			Oklab: propBool(gm, "oklab", true),
		}
		if rawStops, ok := gm["stops"].([]any); ok && len(rawStops) > 0 {
			for _, rs := range rawStops {
				if stMap, ok := rs.(map[string]any); ok {
					sc := baseTh.Accent
					if cs, ok := PropString(stMap, "color"); ok {
						sc = th.Resolve(cs, colortheme.RoleSurface, st.pal, st.rawColor, sc)
					}
					at, _ := PropFloat(stMap, "at")
					lg.Stops = append(lg.Stops, ui.GradientStop{Color: sc, At: at})
				}
			}
		}
		childPal = st.pal.WithSurface(fromCol.Mix(toCol, 0.5))
		el = el.LinearGradient(lg)
		return el, childPal
	}

	var bg ui.Color
	hasBG := false
	if bs, ok := PropString(props, "background"); ok {
		bg = th.Resolve(bs, colortheme.RoleSurface, st.pal, st.rawColor, ui.Color{})
		hasBG = true
	}
	if clickable || props["hover"] != nil || props["pressed"] != nil {
		if el.Pressed() {
			if pm, ok := props["pressed"].(map[string]any); ok {
				if bs, ok := PropString(pm, "background"); ok {
					bg = th.Resolve(bs, colortheme.RoleSurface, st.pal, st.rawColor, bg)
					hasBG = true
				}
			}
		} else if el.Hovered() {
			if hm, ok := props["hover"].(map[string]any); ok {
				if bs, ok := PropString(hm, "background"); ok {
					bg = th.Resolve(bs, colortheme.RoleSurface, st.pal, st.rawColor, bg)
					hasBG = true
				}
			}
		}
	}
	if hasBG {
		el = el.Background(bg)
		if bg.A > 0 {
			childPal = st.pal.WithSurface(bg)
		}
	}
	return el, childPal
}

func applySizeProps(el ui.Element, props map[string]any) ui.Element {
	if props == nil {
		return el
	}
	if propBool(props, "fill", false) {
		el = el.Fill()
	}
	if propBool(props, "fillWidth", false) {
		el = el.FillWidth()
	}
	if propBool(props, "fillHeight", false) {
		el = el.FillHeight()
	}
	if w, ok := PropFloat(props, "width"); ok {
		el = el.Width(w)
	}
	if h, ok := PropFloat(props, "height"); ok {
		el = el.Height(h)
	}
	if v, ok := PropFloat(props, "minWidth"); ok {
		el = el.MinWidth(v)
	}
	if v, ok := PropFloat(props, "minHeight"); ok {
		el = el.MinHeight(v)
	}
	if v, ok := PropFloat(props, "maxWidth"); ok {
		el = el.MaxWidth(v)
	}
	if v, ok := PropFloat(props, "maxHeight"); ok {
		el = el.MaxHeight(v)
	}
	if v, ok := PropFloat(props, "grow"); ok {
		el = el.Grow(v)
	}
	if v, ok := PropFloat(props, "shrink"); ok {
		el = el.Shrink(v)
	}
	if v, ok := PropFloat(props, "basis"); ok {
		el = el.Basis(v)
	}
	return el
}

func parseAlign(s string) (ui.Align, bool) {
	switch s {
	case "start", "flex-start":
		return ui.Start, true
	case "center":
		return ui.Center, true
	case "end", "flex-end":
		return ui.End, true
	case "stretch":
		return ui.Stretch, true
	case "spaceBetween", "space-between":
		return ui.SpaceBetween, true
	default:
		return 0, false
	}
}

func parseShortcutSpec(s string) (ui.Modifiers, ui.Key, bool) {
	parts := strings.Split(s, "+")
	if len(parts) == 0 {
		return 0, 0, false
	}
	var mod ui.Modifiers
	for i := 0; i < len(parts)-1; i++ {
		switch strings.ToLower(strings.TrimSpace(parts[i])) {
		case "cmdorctrl", "cmd", "ctrl", "meta", "super":
			mod |= ui.Cmd
		case "alt", "opt", "option":
			mod |= ui.Alt
		case "shift":
			mod |= ui.Shift
		}
	}
	k := strings.ToUpper(strings.TrimSpace(parts[len(parts)-1]))
	if len(k) == 1 && k[0] >= 'A' && k[0] <= 'Z' {
		return mod, ui.KeyA + ui.Key(k[0]-'A'), true
	}
	return 0, 0, false
}

// PropString extracts a scalar string property from a VDOM prop map.
func PropString(m map[string]any, key string) (string, bool) {
	if m == nil {
		return "", false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return "", false
	}
	switch x := v.(type) {
	case string:
		return x, true
	case int64:
		return strconv.FormatInt(x, 10), true
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), true
	default:
		return fmt.Sprint(x), true
	}
}

func asPropStr(v any) string {
	s, _ := v.(string)
	return s
}

func propStringSlice(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	switch x := v.(type) {
	case string:
		if strings.TrimSpace(x) == "" {
			return nil
		}
		return []string{strings.TrimSpace(x)}
	case []any:
		var out []string
		for _, it := range x {
			if s := strings.TrimSpace(fmt.Sprint(it)); s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func propBool(m map[string]any, key string, fallback bool) bool {
	if m == nil {
		return fallback
	}
	b, ok := m[key].(bool)
	if !ok {
		return fallback
	}
	return b
}

// PropFloat reads a numeric float32 property from a VDOM prop map.
func parseContentTransition(v any) *ui.ContentTransition {
	switch x := v.(type) {
	case bool:
		if x {
			return &ui.ContentTransition{}
		}
	case map[string]any:
		return &ui.ContentTransition{
			Numeric: propBool(x, "numeric", false),
			Down:    propBool(x, "down", false),
			Morph:   propBool(x, "morph", false),
		}
	}
	return nil
}

func PropFloat(m map[string]any, key string) (float32, bool) {
	if m == nil {
		return 0, false
	}
	f, ok := toFloat64(m[key])
	if !ok {
		return 0, false
	}
	return float32(f), true
}

// PropFloatSlice reads either a single number or a numeric array from a VDOM prop map.
func PropFloatSlice(m map[string]any, key string) ([]float32, bool) {
	if m == nil {
		return nil, false
	}
	v, ok := m[key]
	if !ok || v == nil {
		return nil, false
	}
	if f, ok := toFloat64(v); ok {
		return []float32{float32(f)}, true
	}
	arr, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]float32, 0, len(arr))
	for _, it := range arr {
		if f, ok := toFloat64(it); ok {
			out = append(out, float32(f))
		}
	}
	return out, len(out) > 0
}

func toFloat64(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int64:
		return float64(x), true
	case int:
		return float64(x), true
	default:
		return 0, false
	}
}

func clampF(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
