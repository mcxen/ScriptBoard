package main

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/chart"

	"scriptboard/src/internal/engine"
)

// A chart has a kind, set by its data: "line", "dot" or "candle". Its style,
// chosen by the user, is how it is drawn: a line, an area, dots, bars, steps,
// or for candles a K-line, a close line or close bars.

type styleOpt struct{ key, label string }

var seriesStyles = []styleOpt{
	{"line", "折线"}, {"area", "面积"}, {"dot", "点状"}, {"bar", "柱状"}, {"step", "阶梯"},
	// The retro styles: they work with any skin, on any series chart.
	{"stripe", "条纹柱"}, {"pixel", "像素点"}, {"grid", "网格折线"},
}

var candleStyles = []styleOpt{
	{"candle", "K 线"}, {"hollow", "空心K线"}, {"ohlc", "美国线"}, {"close", "收盘线"}, {"bar", "收盘柱"},
}

// stylesFor lists the styles a chart can be drawn in.
func stylesFor(ch *engine.Chart) []styleOpt {
	if ch != nil && ch.Type == "candle" {
		return candleStyles
	}
	return seriesStyles
}

// resolveStyle returns want when the chart can be drawn that way, and the
// chart's own default style otherwise.
func resolveStyle(ch *engine.Chart, want string) string {
	for _, o := range stylesFor(ch) {
		if o.key == want {
			return want
		}
	}
	switch {
	case ch != nil && ch.Type == "candle":
		return "candle"
	case ch != nil && ch.Type == "dot":
		return "dot"
	}
	return "line"
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Candles that closed higher are red and those that closed lower green, as in
// the markets in China.
var (
	upColor   = ui.Hex("#E5484D")
	downColor = ui.Hex("#18A870")
)

var seriesPalette = []ui.Color{
	ui.Hex("#4FC3F7"), ui.Hex("#FF8A65"), ui.Hex("#81C784"), ui.Hex("#FFD54F"),
}

// parseColor reads a "#RRGGBB" color from a script, or returns fallback.
func parseColor(s string, fallback ui.Color) ui.Color {
	if hexColor.MatchString(s) {
		return ui.Hex(s)
	}
	return fallback
}

// count is the number of points a chart has.
func count(ch *engine.Chart) int {
	if ch == nil {
		return 0
	}
	if ch.Type == "candle" {
		return len(ch.Candles)
	}
	n := 0
	for _, s := range ch.Series {
		n = max(n, len(s.Points))
	}
	return n
}

// seriesLine is one series with its points as plot fractions.
type seriesLine struct {
	name   string
	color  ui.Color
	vals   []float64
	xs, ys []float32
	dashed bool
}

// scaled is a chart mapped onto the plot: the value scale, the y ticks and
// the position of every point.
type scaled struct {
	n      int
	empty  bool
	y      chart.Linear
	ticks  []chart.Tick
	xs     []float32    // x of each point; candles use the middle of their band
	bands  []chart.Span // x span of each candle
	series []seriesLine // line, dot and bar charts
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// scale maps ch onto the plot. Every series shares one value range, padded a
// little so the lines do not touch the edges.
func scale(ch *engine.Chart, fallback ui.Color) scaled {
	var s scaled
	s.n = count(ch)
	if isCombo(ch) {
		s.n = comboCount(ch)
	}
	if s.n == 0 {
		s.empty = true
		return s
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	widen := func(v float64) {
		if finite(v) {
			lo = math.Min(lo, v)
			hi = math.Max(hi, v)
		}
	}
	if ch.Type == "candle" {
		for _, cd := range ch.Candles {
			widen(cd.L)
			widen(cd.H)
		}
	} else {
		for _, sr := range ch.Series {
			for _, v := range sr.Points {
				widen(v)
			}
		}
	}
	if math.IsInf(lo, 1) {
		s.empty = true
		return s
	}
	pad := (hi - lo) * 0.08
	if pad == 0 {
		pad = math.Max(math.Abs(hi)*0.01, 0.5)
	}
	s.y = chart.NewLinear(lo-pad, hi+pad).Nice(4)
	s.ticks = s.y.Ticks(4)

	if ch.Type == "candle" {
		band := candleBand(s.n, ch.Labels)
		s.xs = make([]float32, s.n)
		s.bands = make([]chart.Span, s.n)
		for i := range s.n {
			s.xs[i] = band.At(i)
			s.bands[i] = band.Band(i)
		}
		return s
	}

	s.xs = fractions(s.n)
	for i, sr := range ch.Series {
		n := len(sr.Points)
		if n == 0 {
			continue
		}
		color := parseColor(sr.Color, seriesPalette[i%len(seriesPalette)])
		ln := seriesLine{name: sr.Name, color: color, vals: sr.Points, dashed: sr.Dashed}
		ln.xs = fractions(n)
		if ch.Type == "combo" && sr.Offset >= 0 && sr.Offset+n <= len(s.xs) {
			ln.xs = s.xs[sr.Offset : sr.Offset+n]
		}
		ln.ys = make([]float32, n)
		for j, v := range sr.Points {
			ln.ys[j] = s.y.At(v)
		}
		s.series = append(s.series, ln)
	}
	return s
}

// candleBand spaces n bars evenly, with the labels of the points.
func candleBand(n int, labels []string) chart.Band {
	l := make([]string, n)
	copy(l, labels)
	return chart.NewBand(l...).Padding(0.3, 0.15)
}

// fractions spreads n points evenly from the left edge to the right edge.
func fractions(n int) []float32 {
	out := make([]float32, n)
	for i := range out {
		if n > 1 {
			out[i] = float32(i) / float32(n-1)
		} else {
			out[i] = 0.5
		}
	}
	return out
}

// xsFor returns where each point sits in a style: bars sit in the middle of
// their band, lines at their place on the edges.
func xsFor(ch *engine.Chart, s scaled, style string) []float32 {
	if ch.Type != "candle" && (style == "bar" || style == "stripe") {
		band := candleBand(s.n, ch.Labels)
		xs := make([]float32, s.n)
		for i := range xs {
			xs[i] = band.At(i)
		}
		return xs
	}
	return s.xs
}

// ChartProps are the inputs of Chart. Like Lobe's components it takes one
// struct, and a zero value of each field means a sensible default.
type ChartProps struct {
	// Data is what to draw: its kind (line, dot, candle), points, labels.
	Data *engine.Chart
	// Style is how to draw it: line, area, dot, bar, step for series, candle,
	// close or bar for candles. Empty, or a style the data cannot be drawn in,
	// is the data's own default.
	Style string
	// Height is the plot's height in DIPs; 0 is 180.
	Height float32
	// Selected, when set, is the chosen point: the pointer picks the nearest
	// one and the index is kept here (-1 for none). Nil makes the chart inert.
	Selected *int
	// Full adds a label at every few points of the x axis, and a crosshair
	// that reads out the value under the pointer. It is for the full-screen
	// view.
	Full bool
}

// Chart draws a chart from its props.
func Chart(c *ui.Context, p ChartProps) ui.Element {
	h := p.Height
	if h <= 0 {
		h = 180
	}
	return chartWith(c, p.Data, p.Style, h, p.Selected, p.Full)
}

func chartWith(c *ui.Context, ch *engine.Chart, style string, h float32, sel *int, full bool) ui.Element {
	if isCombo(ch) {
		return comboView(c, ch, style, h, sel, full)
	}
	if isHeat(ch) {
		return heatView(c, ch, sel)
	}
	if isPanels(ch) {
		return panelsView(c, ch, style, h, sel, full)
	}
	if isGaugeOrBars(ch) {
		if ch.Type == "gauge" {
			return gaugeView(c, ch, h)
		}
		return barhView(c, ch, h)
	}
	t := c.Theme()
	s := scale(ch, t.Accent)
	if s.empty {
		return ui.Text(c, tr("暂无曲线数据")).TextColor(t.TextMuted).Height(h)
	}
	style = resolveStyle(ch, style)
	xs := xsFor(ch, s, style)

	var bottom []chart.Tick
	switch {
	case full:
		bottom = axisTicks(xs, ch.Labels, 6)
	case len(ch.XLabels) == 2:
		bottom = []chart.Tick{{At: 0, Label: ch.XLabels[0]}, {At: 1, Label: ch.XLabels[1]}}
	}
	if sel == nil && full {
		sel = new(int)
		*sel = -1
	}

	pl := pal(c)
	axisLabel := func(_ chart.Side, tk chart.Tick) {
		ui.Text(c, tk.Label).NoWrap().FontSize(fsTiny).TextColor(pl.muted)
	}
	return skinPlot(c, chart.Plot(c, chart.Axes{Left: s.ticks, Bottom: bottom, Label: axisLabel}, func(area ui.Element) {
		area.Draw(func(p *ui.Painter, r ui.Rect) {
			for _, tk := range s.ticks {
				y := chart.Y(r, tk.At)
				p.Line(r.X, y, r.X+r.W, y, 1, pl.hairline)
			}
			paint(p, r, ch, s, style, xs, t.Accent)
			if full {
				lastPriceLine(p, r, ch, s)
			}
		})

		if sel != nil {
			chart.Select(c, area, sel, xs, func(i int) string { return readout(ch, i) })
			highlight(c, ch, s, xs, *sel, style)
		}
		if full {
			crosshair(c, area, s, t)
		}
	}).Height(h).Gap(6))
}

// Spark draws ch as a bare curve or dots with no axes, for cards. It fills
// the width and is h DIPs tall.
func Spark(c *ui.Context, ch *engine.Chart, h float32) ui.Element {
	return SparkStyled(c, ch, "", h)
}

// SparkStyled is Spark in the style the user chose for the card; "" is the
// chart's own default.
func SparkStyled(c *ui.Context, ch *engine.Chart, want string, h float32) ui.Element {
	if isCombo(ch) {
		return comboView(c, ch, "", h, nil, false)
	}
	if isHeat(ch) {
		// A card shows the average of the table; the table is for the full view.
		g := ch.Grid
		return heatView(c, &engine.Chart{Type: "heat", Grid: &engine.Grid{Columns: g.Columns, Average: g.Average, Footer: g.Footer}}, nil)
	}
	if isPanels(ch) {
		// A card shows the first panel: the whole stack is for the full view.
		return Spark(c, &engine.Chart{Type: "line", Series: ch.Panels[0].Series}, h)
	}
	if isGaugeOrBars(ch) {
		if ch.Type == "gauge" {
			return gaugeView(c, ch, h)
		}
		return barhView(c, ch, h)
	}
	t := c.Theme()
	s := scale(ch, t.Accent)
	return ui.Box(c).Height(h).Draw(func(p *ui.Painter, r ui.Rect) {
		if s.empty {
			return
		}
		style := resolveStyle(ch, want)
		if style == "line" {
			style = "area" // a card's curve is shaded
		}
		r.X, r.W = r.X+5, r.W-10 // keep dots and line ends inside the card
		paint(p, r, ch, s, style, s.xs, t.Accent)
	})
}

// paint draws the chart in style.
func paint(p *ui.Painter, r ui.Rect, ch *engine.Chart, s scaled, style string, xs []float32, accent ui.Color) {
	switch style {
	case "candle":
		drawCandles(p, r, ch.Candles, s)
	case "hollow":
		drawHollowCandles(p, r, ch.Candles, s)
	case "ohlc":
		drawOHLC(p, r, ch.Candles, s)
	case "close":
		drawLines(p, r, []seriesLine{closeLine(ch, s, accent)}, false, chart.CurveLinear)
	case "bar":
		if ch.Type == "candle" {
			drawCloseBars(p, r, ch.Candles, s)
		} else {
			drawBars(p, r, s, xs, false)
		}
	case "stripe":
		drawBars(p, r, s, xs, true)
	case "pixel":
		drawPixels(p, r, s.series)
	case "grid":
		drawGridLine(p, r, s, xs)
	case "area":
		drawLines(p, r, s.series, true, chart.CurveMonotone)
	case "step":
		drawLines(p, r, s.series, false, chart.CurveStepAfter)
	case "dot":
		drawDots(p, r, s.series)
	default: // line
		drawLines(p, r, s.series, false, chart.CurveMonotone)
	}
}

// highlight marks the chosen point, with a rule down from it.
func highlight(c *ui.Context, ch *engine.Chart, s scaled, xs []float32, i int, style string) {
	t := c.Theme()
	if i < 0 || i >= s.n {
		return
	}
	chart.RuleX(c, xs[i], 1).Background(pal(c).muted.Alpha(0.6))
	if ch.Type == "candle" {
		chart.Point(c, xs[i], s.y.At(ch.Candles[i].C), 9, 9).Radius(5).Background(t.Text)
		return
	}
	for _, ln := range s.series {
		if i < len(ln.ys) {
			chart.Point(c, xs[i], ln.ys[i], 10, 10).Radius(5).Background(ln.color)
		}
	}
}

// crosshair shows the value under the pointer: a rule across the plot and the
// value at its right edge. The horizontal position is shown by the readout.
func crosshair(c *ui.Context, area ui.Element, s scaled, t *ui.Theme) {
	_, py, over := area.PointerPosition()
	if !over {
		return
	}
	h := area.Bounds().H
	if h <= 0 {
		return
	}
	fy := 1 - py/h
	if fy < 0 || fy > 1 {
		return
	}
	value := s.y.Invert(fy)
	chart.RuleY(c, fy, 1).Background(pal(c).muted.Alpha(0.6))
	chart.Anchor(c, 1, fy, ui.End, ui.Center, func() {
		ui.Text(c, formatValue(value, s.y)).NoWrap().FontSize(fsTiny).Bold().Padding(3, 8).Radius(6).
			Background(t.Text).TextColor(t.Background)
	})
}

// formatValue prints a value with as many decimals as the scale needs.
func formatValue(v float64, y chart.Linear) string {
	span := math.Abs(y.Hi - y.Lo)
	switch {
	case span >= 100:
		return fmt.Sprintf("%.0f", v)
	case math.Abs(v) >= 10:
		return fmt.Sprintf("%.2f", v)
	}
	return fmt.Sprintf("%.4f", v)
}

// axisTicks picks up to n evenly spaced ticks along the x axis, labelled with
// the points' labels.
func axisTicks(xs []float32, labels []string, n int) []chart.Tick {
	if len(xs) == 0 {
		return nil
	}
	if len(labels) != len(xs) {
		return nil
	}
	step := max(1, (len(xs)-1)/(n-1))
	var idx []int
	for i := 0; i < len(xs); i += step {
		idx = append(idx, i)
	}
	// The last point always gets a label. When it is close to the one before,
	// that one gives way, so two labels never overlap.
	last := len(xs) - 1
	if len(idx) > 0 && idx[len(idx)-1] != last {
		if last-idx[len(idx)-1] < step/2+1 {
			idx[len(idx)-1] = last
		} else {
			idx = append(idx, last)
		}
	}
	out := make([]chart.Tick, 0, len(idx))
	for _, i := range idx {
		out = append(out, chart.Tick{At: xs[i], Label: labels[i]})
	}
	return out
}

// drawLines strokes each line in curve; with fill it also shades the area
// under the first line. Shading every line would lay one tint over another,
// and two tints on top of each other make a third color that belongs to no
// line (blue over orange reads as lavender).
func drawLines(p *ui.Painter, r ui.Rect, lines []seriesLine, fill bool, curve chart.Curve) {
	for i, ln := range lines {
		if len(ln.xs) == 0 {
			continue
		}
		if ln.dashed {
			drawDashed(p, r, ln)
			continue
		}
		if fill && i == 0 {
			// The fill fades from the line down to the axis, as a quote chart's does.
			p.FillPathGradient(chart.Area(r, ln.xs, nil, ln.ys, curve), ui.LinearGradient{
				From: ln.color.Alpha(0.32), To: ln.color.Alpha(0.02), Angle: 180,
			})
		}
		p.StrokePath(chart.Line(r, ln.xs, ln.ys, curve), 2, ln.color)
	}
}

// drawDots paints one dot per point.
func drawDots(p *ui.Painter, r ui.Rect, lines []seriesLine) {
	for _, ln := range lines {
		for j := range ln.xs {
			x, y := chart.X(r, ln.xs[j]), chart.Y(r, ln.ys[j])
			p.Fill(ui.Rect{X: x - 3.5, Y: y - 3.5, W: 7, H: 7}, ln.color, 3.5)
		}
	}
}

// drawBars paints a bar for each point of each series, the series side by
// side within each band.
//
// With stripes, each bar is banded with light rows, as the bars of Mac OS 9's
// charts were.
func drawBars(p *ui.Painter, r ui.Rect, s scaled, xs []float32, stripes bool) {
	k := float32(len(s.series))
	if k == 0 {
		return
	}
	band := candleBand(s.n, nil)
	base := chart.Y(r, s.y.At(s.y.Lo))
	for si, ln := range s.series {
		for j := range ln.ys {
			sp := band.Band(j)
			w := (sp.Hi - sp.Lo) / k
			x0 := sp.Lo + w*float32(si)
			xl, xr := chart.X(r, x0)+1, chart.X(r, x0+w)-1
			top := chart.Y(r, ln.ys[j])
			lo, hi := math.Min(float64(top), float64(base)), math.Max(float64(top), float64(base))
			bar := ui.Rect{X: xl, Y: float32(lo), W: xr - xl, H: float32(hi - lo)}
			if !stripes {
				p.Fill(bar, ln.color.Alpha(0.85), 2)
				continue
			}
			p.Fill(bar, ln.color, 0)
			for y := bar.Y + 2; y < bar.Y+bar.H; y += 6 {
				p.Fill(ui.Rect{X: bar.X, Y: y, W: bar.W, H: 3}, ui.Hex("#FFFFFF").Alpha(0.45), 0)
			}
		}
	}
	_ = xs
}

// drawPixels paints each point as a small square, as the pixels of an old
// screen.
func drawPixels(p *ui.Painter, r ui.Rect, lines []seriesLine) {
	for _, ln := range lines {
		for j := range ln.xs {
			// Kept inside the plot, so the last point at the edge is whole.
			x := min(max(chart.X(r, ln.xs[j]), r.X+2.5), r.X+r.W-2.5)
			y := min(max(chart.Y(r, ln.ys[j]), r.Y+2.5), r.Y+r.H-2.5)
			p.Fill(ui.Rect{X: x - 2.5, Y: y - 2.5, W: 5, H: 5}, ln.color, 0)
		}
	}
}

// drawGridLine draws the lines with a grid behind them: a vertical rule at
// each point, the same blue-gray as the horizontal rules of the chart.
func drawGridLine(p *ui.Painter, r ui.Rect, s scaled, xs []float32) {
	if len(s.series) > 0 {
		for _, x := range xs {
			xp := chart.X(r, x)
			p.Line(xp, r.Y, xp, r.Y+r.H, 1, ui.Hex("#8C8C8C").Alpha(0.35))
		}
	}
	drawLines(p, r, s.series, false, chart.CurveLinear)
}

// closeLine is the closing prices of a candle chart as a line.
func closeLine(ch *engine.Chart, s scaled, accent ui.Color) seriesLine {
	ys := make([]float32, len(ch.Candles))
	for i, cd := range ch.Candles {
		ys[i] = s.y.At(cd.C)
	}
	return seriesLine{name: tr("收盘"), color: accent, xs: s.xs, ys: ys}
}

// drawCloseBars paints each candle's close as a bar from the bottom of the
// axis: red when it closed above its open, green when below.
func drawCloseBars(p *ui.Painter, r ui.Rect, cs []engine.Candle, s scaled) {
	base := chart.Y(r, s.y.At(s.y.Lo))
	for i, cd := range cs {
		col := downColor
		if cd.C >= cd.O {
			col = upColor
		}
		sp := s.bands[i]
		xl, xr := chart.X(r, sp.Lo), chart.X(r, sp.Hi)
		top := chart.Y(r, s.y.At(cd.C))
		lo, hi := math.Min(float64(top), float64(base)), math.Max(float64(top), float64(base))
		p.Fill(ui.Rect{X: xl, Y: float32(lo), W: xr - xl, H: float32(hi - lo)}, col.Alpha(0.85), 2)
	}
}

// drawCandles paints each candle: a body from open to close and a wick from
// low to high.
func drawCandles(p *ui.Painter, r ui.Rect, cs []engine.Candle, s scaled) {
	for i, cd := range cs {
		col := downColor
		if cd.C >= cd.O {
			col = upColor
		}
		span := s.bands[i]
		xc := chart.X(r, span.Mid())
		x0, x1 := chart.X(r, span.Lo), chart.X(r, span.Hi)
		p.Line(xc, chart.Y(r, s.y.At(cd.H)), xc, chart.Y(r, s.y.At(cd.L)), 1.2, col)

		yo, yc := chart.Y(r, s.y.At(cd.O)), chart.Y(r, s.y.At(cd.C))
		top, bot := min(yo, yc), max(yo, yc)
		if bot-top < 1.5 {
			mid := (top + bot) / 2
			top, bot = mid-0.75, mid+0.75
		}
		p.Fill(ui.Rect{X: x0, Y: top, W: x1 - x0, H: bot - top}, col, 1)
	}
}

// readout describes point i of ch: its label, its values and, for lines and
// dots, the change from the point before.
func readout(ch *engine.Chart, i int) string {
	if ch == nil {
		return ""
	}
	n := count(ch)
	if i < 0 || i >= n {
		return ""
	}
	name := fmt.Sprintf(tr("第 %d 个"), i+1)
	if i < len(ch.Labels) && ch.Labels[i] != "" {
		name = ch.Labels[i]
	}
	if ch.Type == "candle" {
		cd := ch.Candles[i]
		change := cd.C - cd.O
		return fmt.Sprintf(tr("%s    开 %.2f   高 %.2f   低 %.2f   收 %.2f   涨跌 %+.2f"), name, cd.O, cd.H, cd.L, cd.C, change)
	}
	parts := []string{name}
	for _, sr := range ch.Series {
		if i >= len(sr.Points) {
			continue
		}
		v := sr.Points[i]
		text := fmt.Sprintf("%.2f", v)
		if sr.Name != "" {
			text = sr.Name + " " + text
		}
		if i > 0 && i-1 < len(sr.Points) {
			text += fmt.Sprintf("（%+.2f）", v-sr.Points[i-1])
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "    ")
}

// Detail is the readout line under a chart: the chosen point, or the latest
// one while nothing is chosen.
func Detail(c *ui.Context, ch *engine.Chart, sel *int) ui.Element {
	i := count(ch) - 1
	hint := tr("悬浮图表查看详情")
	if sel != nil && *sel >= 0 {
		i = *sel
		hint = ""
	}
	text := readout(ch, i)
	if text == "" {
		return ui.Text(c, "").FontSize(12)
	}
	if hint != "" {
		text = text + "    · " + hint
	}
	return ui.Text(c, text).FontSize(fsSmall).TextColor(pal(c).muted)
}

// drawDashed strokes a line as dashes: a projection that is not known yet.
func drawDashed(p *ui.Painter, r ui.Rect, ln seriesLine) {
	const dash, gap = 6.0, 4.0
	for j := 0; j+1 < len(ln.xs); j++ {
		x0, y0 := chart.X(r, ln.xs[j]), chart.Y(r, ln.ys[j])
		x1, y1 := chart.X(r, ln.xs[j+1]), chart.Y(r, ln.ys[j+1])
		length := math.Hypot(float64(x1-x0), float64(y1-y0))
		if length == 0 {
			continue
		}
		for t := 0.0; t < length; t += dash + gap {
			end := math.Min(t+dash, length)
			f0, f1 := float32(t/length), float32(end/length)
			p.Line(x0+(x1-x0)*f0, y0+(y1-y0)*f0, x0+(x1-x0)*f1, y0+(y1-y0)*f1, 1.6, ln.color)
		}
	}
}

// comboCount is the length of a combo chart's axis: the furthest point of
// its series.
func comboCount(ch *engine.Chart) int {
	n := 0
	for _, sr := range ch.Series {
		if end := sr.Offset + len(sr.Points); end > n {
			n = end
		}
	}
	return n
}

// drawHollowCandles paints the Chinese-market style of hollow candles: a rising
// period is an outline, a falling one is filled; the wick is as in drawCandles.
func drawHollowCandles(p *ui.Painter, r ui.Rect, cs []engine.Candle, s scaled) {
	for i, cd := range cs {
		up := cd.C >= cd.O
		col := downColor
		if up {
			col = upColor
		}
		span := s.bands[i]
		xc := chart.X(r, span.Mid())
		x0, x1 := chart.X(r, span.Lo), chart.X(r, span.Hi)
		p.Line(xc, chart.Y(r, s.y.At(cd.H)), xc, chart.Y(r, s.y.At(cd.L)), 1.2, col)

		yo, yc := chart.Y(r, s.y.At(cd.O)), chart.Y(r, s.y.At(cd.C))
		top, bot := min(yo, yc), max(yo, yc)
		if bot-top < 1.5 {
			mid := (top + bot) / 2
			top, bot = mid-0.75, mid+0.75
		}
		if !up {
			p.Fill(ui.Rect{X: x0, Y: top, W: x1 - x0, H: bot - top}, col, 1)
			continue
		}
		const w = 1.5
		p.Line(x0, top, x1, top, w, col)
		p.Line(x0, bot, x1, bot, w, col)
		p.Line(x0, top, x0, bot, w, col)
		p.Line(x1, top, x1, bot, w, col)
	}
}

// drawOHLC paints bars: a vertical line from the high to the low, with a tick
// at the open on the left and one at the close on the right.
func drawOHLC(p *ui.Painter, r ui.Rect, cs []engine.Candle, s scaled) {
	for i, cd := range cs {
		col := downColor
		if cd.C >= cd.O {
			col = upColor
		}
		span := s.bands[i]
		xc := chart.X(r, span.Mid())
		tick := (chart.X(r, span.Hi) - chart.X(r, span.Lo)) * 0.3
		yo, yc := chart.Y(r, s.y.At(cd.O)), chart.Y(r, s.y.At(cd.C))
		p.Line(xc, chart.Y(r, s.y.At(cd.H)), xc, chart.Y(r, s.y.At(cd.L)), 1.5, col)
		p.Line(xc-tick, yo, xc, yo, 1.5, col)
		p.Line(xc, yc, xc+tick, yc, 1.5, col)
	}
}

// lastPriceLine draws a dashed rule at the newest value of a line chart: the
// price a quote chart marks on its right edge.
func lastPriceLine(p *ui.Painter, r ui.Rect, ch *engine.Chart, s scaled) {
	if ch.Type == "candle" || len(s.series) == 0 {
		return
	}
	ln := s.series[0]
	if len(ln.ys) == 0 {
		return
	}
	rule := seriesLine{color: ln.color.Alpha(0.7), xs: []float32{0, 1}, ys: []float32{ln.ys[len(ln.ys)-1], ln.ys[len(ln.ys)-1]}}
	drawDashed(p, r, rule)
}
