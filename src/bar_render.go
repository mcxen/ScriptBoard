package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"scriptboard/src/internal/bar"
)

// barTheme carries resolved colors for rendering a .bar virtual DOM tree in
// Auto, Light, or Dark preview/display mode.
type barTheme struct {
	dark           bool
	surface        ui.Color
	surfaceHover   ui.Color
	surfacePressed ui.Color
	text           ui.Color
	textMuted      ui.Color
	accent         ui.Color
	warning        ui.Color
	danger         ui.Color
	success        ui.Color
	border         ui.Color
}

const (
	barThemeAuto  = 0
	barThemeLight = 1
	barThemeDark  = 2
)

func barThemeFor(c *ui.Context, mode int) barTheme {
	dark := pal(c).dark
	switch mode {
	case barThemeLight:
		dark = false
	case barThemeDark:
		dark = true
	}
	if dark {
		return barTheme{
			dark:           true,
			surface:        ui.Hex("#1C1F26"),
			surfaceHover:   ui.Hex("#2A2F3A"),
			surfacePressed: ui.Hex("#343A47"),
			text:           ui.Hex("#F1F4F9"),
			textMuted:      ui.Hex("#9AA3B5"),
			accent:         ui.Hex("#5B8CFF"),
			warning:        ui.Hex("#FBBF24"),
			danger:         ui.Hex("#F87171"),
			success:        ui.Hex("#34D399"),
			border:         ui.Hex("#2D323E"),
		}
	}
	return barTheme{
		dark:           false,
		surface:        ui.Hex("#FFFFFF"),
		surfaceHover:   ui.Hex("#ECEEF3"),
		surfacePressed: ui.Hex("#DFE3EC"),
		text:           ui.Hex("#151922"),
		textMuted:      ui.Hex("#6B7280"),
		accent:         ui.Hex("#2563EB"),
		warning:        ui.Hex("#F59E0B"),
		danger:         ui.Hex("#EF4444"),
		success:        ui.Hex("#10B981"),
		border:         ui.Hex("#E6E8EF"),
	}
}

func (th barTheme) color(name string, fallback ui.Color) ui.Color {
	switch strings.TrimSpace(name) {
	case "":
		return fallback
	case "surface":
		return th.surface
	case "surfaceHover":
		return th.surfaceHover
	case "surfacePressed":
		return th.surfacePressed
	case "text", "foreground":
		return th.text
	case "textMuted", "muted", "secondary":
		return th.textMuted
	case "accent", "primary":
		return th.accent
	case "warning":
		return th.warning
	case "danger", "error":
		return th.danger
	case "success":
		return th.success
	case "border", "divider":
		return th.border
	case "transparent":
		return ui.Color{}
	}
	if strings.HasPrefix(name, "#") {
		return ui.Hex(name)
	}
	return fallback
}

var (
	barSVGMu    sync.Mutex
	barSVGCache = map[string]*ui.SVG{}
)

func parseBarSVG(raw string) *ui.SVG {
	if raw == "" {
		return nil
	}
	barSVGMu.Lock()
	if s, ok := barSVGCache[raw]; ok {
		barSVGMu.Unlock()
		return s
	}
	barSVGMu.Unlock()

	s, err := ui.ParseSVG([]byte(raw))
	if err != nil {
		return nil
	}
	barSVGMu.Lock()
	if len(barSVGCache) > 256 {
		barSVGCache = map[string]*ui.SVG{}
	}
	barSVGCache[raw] = s
	barSVGMu.Unlock()
	return s
}

// renderBarPanel paints a .bar Snapshot panel inside a rounded card matching
// the Metrix.Bar floating panel / preview appearance.
func renderBarPanel(c *ui.Context, snap bar.Snapshot, th barTheme, width float32, onAction func(string)) ui.Element {
	if width <= 0 {
		width = 300
	}
	var prefH float32
	if snap.Panel != nil {
		if pw, ok := propFloat(snap.Panel.Props, "preferredWidth"); ok && pw > 0 && width == 300 {
			width = pw
		}
		if ph, ok := propFloat(snap.Panel.Props, "preferredHeight"); ok && ph > 0 {
			prefH = ph
		}
	}
	cardEl := ui.Column(c).
		Width(width).
		Background(th.surface).
		Radius(14).
		BorderWidth(1).
		BorderColor(th.border).
		Clip()
	if prefH > 0 {
		cardEl = cardEl.Height(prefH)
	}
	return cardEl.Children(func() {
		if snap.Panel != nil {
			renderBarNode(c, snap.Panel, th, onAction)
		} else if snap.Error != "" {
			ui.Column(c).Padding(16).Gap(8).Children(func() {
				ui.Text(c, snap.Error).FontSize(12).TextColor(th.danger)
			})
		} else {
			ui.Column(c).Padding(16).Gap(8).Children(func() {
				ui.Text(c, tr("加载中…")).FontSize(12).TextColor(th.textMuted)
			})
		}
	})
}

func renderBarNode(c *ui.Context, n *bar.Node, th barTheme, onAction func(string)) {
	if n == nil {
		return
	}
	props := n.Props

	switch n.Type {
	case "Spacer":
		ui.Spacer(c)
		return
	case "Divider":
		ui.Box(c).FillWidth().Height(1).Background(th.border)
		return
	case "Text":
		renderBarText(c, n, th)
		return
	case "Icon":
		renderBarIcon(c, n, th)
		return
	case "Button":
		renderBarButton(c, n, th, onAction)
		return
	case "Chart":
		renderBarChart(c, n, th)
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
	default: // Column and unknown containers
		el = ui.Column(c)
	}

	el = applyBarBoxProps(el, props, th, n.OnClickID != "")

	// Register keyboard shortcuts declared on the node (e.g. "CmdOrCtrl+R").
	for keySpec, aid := range n.Shortcuts {
		if mod, key, ok := parseShortcutSpec(keySpec); ok && onAction != nil {
			if el.OverlayShortcut(mod, key) {
				onAction(aid)
			}
		}
	}

	if n.OnClickID != "" {
		el = el.Cursor(ui.CursorPointer)
	}
	if el.Children(func() {
		for _, ch := range n.Children {
			renderBarNode(c, ch, th, onAction)
		}
	}).Clicked() && n.OnClickID != "" && onAction != nil {
		onAction(n.OnClickID)
	}
}

func renderBarText(c *ui.Context, n *bar.Node, th barTheme) {
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
	if fs, ok := propFloat(props, "fontSize"); ok && fs > 0 {
		el = el.FontSize(fs)
	} else {
		el = el.FontSize(13)
	}
	if fw, ok := propFloat(props, "fontWeight"); ok && fw > 0 {
		el = el.FontWeight(int(fw))
	}
	if ls, ok := propFloat(props, "letterSpacing"); ok {
		el = el.LetterSpacing(ls)
	}
	col := th.text
	if tc, ok := propString(props, "textColor"); ok {
		col = th.color(tc, th.text)
	}
	el = el.TextColor(col)

	if ta, ok := propString(props, "textAlign"); ok {
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
	applyBarSizeProps(el, props)
}

func renderBarIcon(c *ui.Context, n *bar.Node, th barTheme) {
	props := n.Props
	svgStr, _ := propString(props, "svg")
	svg := parseBarSVG(svgStr)
	if svg == nil {
		if name, ok := propString(props, "name"); ok && name != "" {
			svg = icon(strings.TrimPrefix(name, "lucide:"))
		}
	}
	if svg == nil {
		return
	}
	size := float32(16)
	if fs, ok := propFloat(props, "fontSize"); ok && fs > 0 {
		size = fs
	} else if w, ok := propFloat(props, "width"); ok && w > 0 {
		size = w
	}
	col := th.text
	if tc, ok := propString(props, "textColor"); ok {
		col = th.color(tc, th.text)
	}
	el := ui.Icon(c, svg).FontSize(size).TextColor(col)
	applyBarSizeProps(el, props)

	if kf, ok := props["keyframes"].(map[string]any); ok {
		durMs := float32(900)
		if d, ok := propFloat(kf, "duration"); ok && d > 0 {
			durMs = d
		}
		rep := 1
		if r, ok := propFloat(kf, "repeat"); ok {
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

func renderBarButton(c *ui.Context, n *bar.Node, th barTheme, onAction func(string)) {
	props := n.Props
	lbl := n.Text
	if lbl == "" && len(n.Children) > 0 {
		lbl = n.Children[0].Text
	}
	bg := th.surfaceHover
	b := ui.Row(c).
		Padding(6, 12).
		Radius(8).
		AlignItems(ui.Center).
		Justify(ui.Center).
		Cursor(ui.CursorPointer)
	if b.Pressed() {
		bg = th.surfacePressed
	} else if b.Hovered() {
		bg = th.surfaceHover
	} else {
		bg = th.accent.Alpha(0.14)
	}
	b = b.Background(bg)
	applyBarSizeProps(b, props)
	if b.Children(func() {
		ui.Text(c, lbl).FontSize(12).FontWeight(600).TextColor(th.accent).NoWrap()
	}).Clicked() && n.OnClickID != "" && onAction != nil {
		onAction(n.OnClickID)
	}
}

func renderBarChart(c *ui.Context, n *bar.Node, th barTheme) {
	props := n.Props
	h := float32(40)
	if v, ok := propFloat(props, "height"); ok && v > 0 {
		h = v
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
			col := th.accent
			if cs, ok := propString(sm, "color"); ok {
				col = th.color(cs, th.accent)
			}
			cs := chartSeries{col: col}
			if fm, ok := sm["fill"].(map[string]any); ok {
				cs.hasFill = true
				if f0, ok := propFloat(fm, "from"); ok {
					cs.fillFrom = f0
				} else {
					cs.fillFrom = 0.16
				}
				if f1, ok := propFloat(fm, "to"); ok {
					cs.fillTo = f1
				}
			} else if strings.EqualFold(asPropStr(props["type"]), "area") {
				cs.hasFill = true
				cs.fillFrom = 0.16
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
	if swv, ok := propFloat(props, "strokeWidth"); ok && swv > 0 {
		strokeW = swv
	}
	curveKind := chart.CurveLinear
	if cv, ok := propString(props, "curve"); ok && cv == "monotone" {
		curveKind = chart.CurveMonotone
	}

	box := ui.Box(c).Height(h)
	if propBool(props, "fillWidth", true) {
		box = box.FillWidth()
	}
	applyBarSizeProps(box, props)

	if len(allSeries) == 0 {
		return
	}

	// Compute x/y domains.
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
	if xd, ok := props["xDomain"].([]any); ok && len(xd) >= 2 {
		if a, ok1 := toFloat64(xd[0]); ok1 {
			xMin = a
		}
		if b, ok2 := toFloat64(xd[1]); ok2 {
			xMax = b
		}
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
		yMin -= pad
		yMax += pad
		ySpan = yMax - yMin
	}

	box.Draw(func(p *ui.Painter, r ui.Rect) {
		if r.W <= 1 || r.H <= 1 {
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
		}
	})
}

func applyBarBoxProps(el ui.Element, props map[string]any, th barTheme, clickable bool) ui.Element {
	if props == nil {
		return el
	}
	el = applyBarSizeProps(el, props)

	if gap, ok := propFloat(props, "gap"); ok {
		el = el.Gap(gap)
	}
	if pad, ok := propFloatSlice(props, "padding"); ok && len(pad) > 0 {
		el = el.Padding(pad...)
	}
	if rad, ok := propFloatSlice(props, "radius"); ok && len(rad) > 0 {
		el = el.Radius(rad...)
	}
	if ai, ok := propString(props, "alignItems"); ok {
		if a, ok := parseAlign(ai); ok {
			el = el.AlignItems(a)
		}
	}
	if j, ok := propString(props, "justify"); ok {
		if a, ok := parseAlign(j); ok {
			el = el.Justify(a)
		}
	}
	if propBool(props, "absolute", false) {
		el = el.Absolute()
	}
	if v, ok := propFloat(props, "top"); ok {
		el = el.Top(v)
	}
	if v, ok := propFloat(props, "right"); ok {
		el = el.Right(v)
	}
	if v, ok := propFloat(props, "bottom"); ok {
		el = el.Bottom(v)
	}
	if v, ok := propFloat(props, "left"); ok {
		el = el.Left(v)
	}
	if v, ok := propFloat(props, "leftPercent"); ok {
		el = el.LeftPercent(v)
	}
	if v, ok := propFloat(props, "widthPercent"); ok {
		el = el.WidthPercent(v)
	}
	if v, ok := propFloat(props, "topPercent"); ok {
		el = el.TopPercent(v)
	}
	if v, ok := propFloat(props, "heightPercent"); ok {
		el = el.HeightPercent(v)
	}
	if bw, ok := propFloat(props, "borderWidth"); ok && bw > 0 {
		el = el.BorderWidth(bw)
	}
	if bc, ok := propString(props, "borderColor"); ok {
		el = el.BorderColor(th.color(bc, th.border))
	}
	if tip, ok := propString(props, "tooltip"); ok && tip != "" {
		el = el.Tooltip(tip)
	}

	var bg ui.Color
	hasBG := false
	if bs, ok := propString(props, "background"); ok {
		bg = th.color(bs, ui.Color{})
		hasBG = true
	}
	if clickable || props["hover"] != nil || props["pressed"] != nil {
		if el.Pressed() {
			if pm, ok := props["pressed"].(map[string]any); ok {
				if bs, ok := propString(pm, "background"); ok {
					bg = th.color(bs, bg)
					hasBG = true
				}
			}
		} else if el.Hovered() {
			if hm, ok := props["hover"].(map[string]any); ok {
				if bs, ok := propString(hm, "background"); ok {
					bg = th.color(bs, bg)
					hasBG = true
				}
			}
		}
	}
	if hasBG {
		el = el.Background(bg)
	}
	return el
}

func applyBarSizeProps(el ui.Element, props map[string]any) ui.Element {
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
	if w, ok := propFloat(props, "width"); ok {
		el = el.Width(w)
	}
	if h, ok := propFloat(props, "height"); ok {
		el = el.Height(h)
	}
	if v, ok := propFloat(props, "minWidth"); ok {
		el = el.MinWidth(v)
	}
	if v, ok := propFloat(props, "minHeight"); ok {
		el = el.MinHeight(v)
	}
	if v, ok := propFloat(props, "maxWidth"); ok {
		el = el.MaxWidth(v)
	}
	if v, ok := propFloat(props, "maxHeight"); ok {
		el = el.MaxHeight(v)
	}
	if v, ok := propFloat(props, "grow"); ok {
		el = el.Grow(v)
	}
	if v, ok := propFloat(props, "shrink"); ok {
		el = el.Shrink(v)
	}
	if v, ok := propFloat(props, "basis"); ok {
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

func propString(m map[string]any, key string) (string, bool) {
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

func propFloat(m map[string]any, key string) (float32, bool) {
	if m == nil {
		return 0, false
	}
	f, ok := toFloat64(m[key])
	if !ok {
		return 0, false
	}
	return float32(f), true
}

func propFloatSlice(m map[string]any, key string) ([]float32, bool) {
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
