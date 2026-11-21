package main

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

// ComponentRenderReq bundles every input a pluggable ComponentView needs to
// render either a compact home card or a full-screen detail view.
type ComponentRenderReq struct {
	App      *app
	Script   store.Script
	Result   engine.Result
	Style    string // user-chosen plot style or swapped component/substyle override
	Size     int    // -1 full view, 0 small, 1 medium, 2 large, 3 wide
	Height   float32
	Limit    int // max items for compact cards; 0 = all
	Selected *int
	Pinned   *int
	Now      time.Time
	Legend   bool
	Full     bool
}

// ComponentView is the pluggable abstraction implemented by every ScriptBoard
// component renderer. Replacing or extending any component type only requires
// calling registerComponent with a ComponentView for that type.
type ComponentView interface {
	Type() string
	Spec() engine.WidgetSpec
	RenderCard(c *ui.Context, req ComponentRenderReq) ui.Element
	RenderFull(c *ui.Context, req ComponentRenderReq) ui.Element
	// SummaryTiles renders full-view summary tiles if this component provides
	// custom tiles; returns true when handled (or intentionally suppressed).
	SummaryTiles(a *app, c *ui.Context, res engine.Result) bool
}

// componentAdapter turns plain functions into a ComponentView so built-in and
// custom renderers stay concise and easy to replace.
type componentAdapter struct {
	typ     string
	cardFn  func(*ui.Context, ComponentRenderReq) ui.Element
	fullFn  func(*ui.Context, ComponentRenderReq) ui.Element
	tilesFn func(*app, *ui.Context, engine.Result) bool
}

func (a componentAdapter) Type() string { return a.typ }
func (a componentAdapter) Spec() engine.WidgetSpec {
	s, _ := engine.WidgetSpecOf(a.typ)
	return s
}
func (a componentAdapter) RenderCard(c *ui.Context, req ComponentRenderReq) ui.Element {
	if a.cardFn != nil {
		return a.cardFn(c, req)
	}
	return a.RenderFull(c, req)
}
func (a componentAdapter) RenderFull(c *ui.Context, req ComponentRenderReq) ui.Element {
	if a.fullFn != nil {
		return a.fullFn(c, req)
	}
	return ui.Text(c, "")
}
func (a componentAdapter) SummaryTiles(ap *app, c *ui.Context, res engine.Result) bool {
	if a.tilesFn != nil {
		return a.tilesFn(ap, c, res)
	}
	return false
}

var (
	componentMu sync.RWMutex
	components  = map[string]ComponentView{}
)

// registerComponent registers or replaces the renderer for v.Type().
func registerComponent(v ComponentView) {
	componentMu.Lock()
	defer componentMu.Unlock()
	components[v.Type()] = v
}

// componentOf looks up the renderer for component type typ.
func componentOf(typ string) (ComponentView, bool) {
	componentMu.RLock()
	defer componentMu.RUnlock()
	v, ok := components[typ]
	return v, ok
}

// registeredComponentTypes returns all registered component type keys sorted.
func registeredComponentTypes() []string {
	componentMu.RLock()
	defer componentMu.RUnlock()
	out := make([]string, 0, len(components))
	for k := range components {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// parseViewOverride splits a persisted card style value. A style prefixed with
// "view:" (such as "view:progressRing" or "view:table") requests a swapped
// component type; anything else is a sub-style for the native component (such
// as "grid", "neon", or "digital").
func parseViewOverride(style string) (swapType, subStyle string) {
	if strings.HasPrefix(style, "view:") {
		rest := strings.TrimPrefix(style, "view:")
		if typ, sub, ok := strings.Cut(rest, ":"); ok {
			return typ, sub
		}
		return rest, ""
	}
	return "", style
}

// resolveComponent resolves the active ComponentView and adapted Result for a
// script result plus an optional user style/swap override.
func resolveComponent(res engine.Result, styleOverride string) (ComponentView, engine.Result, string) {
	baseType := engine.ResultWidgetType(res)
	if baseType == "" {
		return nil, res, ""
	}
	swapType, subStyle := parseViewOverride(styleOverride)
	targetType := baseType
	adapted := res
	if swapType != "" && swapType != baseType {
		if cand, ok := adaptResultForType(res, swapType); ok {
			targetType = swapType
			adapted = cand
		}
	}
	if v, ok := componentOf(targetType); ok {
		return v, adapted, subStyle
	}
	if v, ok := componentOf(baseType); ok {
		return v, res, subStyle
	}
	return nil, res, subStyle
}

// adaptResultForType transforms res's payload so a compatible target component
// can render the same underlying data without modifying the script.
func adaptResultForType(res engine.Result, target string) (engine.Result, bool) {
	base := engine.ResultWidgetType(res)
	if target == "" || target == base {
		return res, true
	}
	out := res
	switch target {
	case "table":
		tb := engine.ExtractTable(res, curLang())
		if tb == nil {
			return res, false
		}
		out.ProgressBar = nil
		out.Chart = &engine.Chart{Type: "table", Table: tb}
		return out, true

	case "progressBar":
		items := extractProgressItems(res)
		if len(items) == 0 {
			return res, false
		}
		style := engine.BarStyle{Style: "gradient", Height: 9}
		if res.ProgressBar != nil {
			style = res.ProgressBar.BarStyle
		} else if res.Chart != nil && res.Chart.Bar != nil {
			style = *res.Chart.Bar
		}
		out.Chart = nil
		out.ProgressBar = &engine.ProgressBar{Items: items, BarStyle: style}
		return out, true

	case "progressRing":
		items := extractProgressItems(res)
		if len(items) == 0 {
			return res, false
		}
		ring := &engine.RingStyle{Layout: "row"}
		if len(items) >= 2 && len(items) <= 4 {
			ring.Layout = "concentric"
		}
		if res.Chart != nil && res.Chart.Ring != nil {
			ring = res.Chart.Ring
		}
		out.ProgressBar = nil
		out.Chart = &engine.Chart{Type: "progressRing", Progress: items, Ring: ring}
		return out, true

	case "barh":
		if items := extractProgressItems(res); len(items) > 0 {
			labels := make([]string, len(items))
			pts := make([]float64, len(items))
			for i, it := range items {
				labels[i] = it.Name
				pts[i] = math.Round(it.Raw()*10) / 10
			}
			out.ProgressBar = nil
			out.Chart = &engine.Chart{
				Type:   "barh",
				Labels: labels,
				Series: []engine.Series{{Name: tr("完成度") + " (%)", Points: pts}},
			}
			return out, true
		}
		if res.Chart != nil && len(res.Chart.Slices) > 0 {
			labels := make([]string, len(res.Chart.Slices))
			pts := make([]float64, len(res.Chart.Slices))
			for i, s := range res.Chart.Slices {
				labels[i] = s.Name
				pts[i] = s.Value
			}
			out.ProgressBar = nil
			out.Chart = &engine.Chart{
				Type:   "barh",
				Labels: labels,
				Series: []engine.Series{{Name: res.Title, Points: pts}},
			}
			return out, true
		}
		if res.Chart != nil && len(res.Chart.Series) > 0 {
			cp := *res.Chart
			cp.Type = "barh"
			out.Chart = &cp
			return out, true
		}
		return res, false

	case "donut":
		if res.Chart == nil {
			return res, false
		}
		if len(res.Chart.Slices) > 0 {
			return res, true
		}
		if len(res.Chart.Series) > 0 && len(res.Chart.Series[0].Points) > 0 {
			sr := res.Chart.Series[0]
			var slices []engine.Slice
			for i, v := range sr.Points {
				if v <= 0 {
					continue
				}
				name := fmt.Sprintf("#%d", i+1)
				if i < len(res.Chart.Labels) && res.Chart.Labels[i] != "" {
					name = res.Chart.Labels[i]
				}
				slices = append(slices, engine.Slice{Name: name, Value: v})
			}
			if len(slices) > 0 {
				out.ProgressBar = nil
				out.Chart = &engine.Chart{Type: "donut", Slices: slices}
				return out, true
			}
		}
		return res, false

	case "kv":
		var items []engine.KVItem
		if res.Chart != nil {
			switch {
			case len(res.Chart.KV) > 0:
				items = res.Chart.KV
			case len(res.Chart.Slices) > 0:
				total := 0.0
				for _, s := range res.Chart.Slices {
					if s.Value > 0 {
						total += s.Value
					}
				}
				for _, s := range res.Chart.Slices {
					badge := ""
					if total > 0 && s.Value > 0 {
						badge = fmt.Sprintf("%.0f%%", math.Round(s.Value/total*100))
					}
					items = append(items, engine.KVItem{
						Label: s.Name, Value: trimNum(s.Value) + s.Unit, Badge: badge, Color: s.Color,
					})
				}
			case len(res.Chart.Checklist) > 0:
				for _, it := range res.Chart.Checklist {
					st, val := "warn", tr("未完成")
					if it.Done {
						st, val = "ok", tr("已完成")
					}
					items = append(items, engine.KVItem{
						Label: it.Label, Value: val, Sub: it.Note, Badge: it.Badge, Status: st, Color: it.Color,
					})
				}
			case len(res.Chart.Countdown) > 0:
				for _, it := range res.Chart.Countdown {
					items = append(items, engine.KVItem{
						Label: it.Label, Value: it.Target, Sub: it.Note, Color: it.Color,
					})
				}
			case res.Chart.Type == "barh" && len(res.Chart.Series) > 0:
				sr := res.Chart.Series[0]
				for i, v := range sr.Points {
					lbl := fmt.Sprintf("#%d", i+1)
					if i < len(res.Chart.Labels) && res.Chart.Labels[i] != "" {
						lbl = res.Chart.Labels[i]
					}
					items = append(items, engine.KVItem{Label: lbl, Value: trimNum(v)})
				}
			case res.Chart.Table != nil && len(res.Chart.Table.Columns) >= 2:
				for _, r := range res.Chart.Table.Rows {
					lbl, val, sub := "", "", ""
					if len(r) > 0 {
						lbl = r[0]
					}
					if len(r) > 1 {
						val = r[1]
					}
					if len(r) > 2 {
						sub = strings.Join(r[2:], " · ")
					}
					items = append(items, engine.KVItem{Label: lbl, Value: val, Sub: sub})
				}
			}
		}
		if len(items) == 0 {
			return res, false
		}
		out.ProgressBar = nil
		out.Chart = &engine.Chart{Type: "kv", KV: items}
		return out, true

	case "line", "dot":
		if res.Chart != nil && len(res.Chart.Series) > 0 {
			cp := *res.Chart
			cp.Type = target
			out.Chart = &cp
			return out, true
		}
		return res, false
	}
	return res, false
}

func extractProgressItems(res engine.Result) []engine.Progress {
	if res.ProgressBar != nil && len(res.ProgressBar.Items) > 0 {
		return res.ProgressBar.Items
	}
	if res.Chart != nil {
		if len(res.Chart.Progress) > 0 {
			return res.Chart.Progress
		}
		if g := res.Chart.Gauge; g != nil && g.Limit > 0 {
			lbl := g.Label
			if lbl == "" {
				lbl = tr("额度")
			}
			return []engine.Progress{{Name: lbl, Value: g.Used, Max: g.Limit, Unit: g.Unit}}
		}
	}
	return nil
}

func init() {
	// Register all built-in component renderers into the unified registry.
	registerPlot := func(typ string) {
		registerComponent(componentAdapter{
			typ: typ,
			cardFn: func(c *ui.Context, req ComponentRenderReq) ui.Element {
				ch := req.Result.Chart
				if req.Size == 1 {
					return SparkStyled(c, ch, req.Style, req.Height)
				}
				return Chart(c, ChartProps{Data: ch, Style: req.Style, Height: req.Height, Selected: req.Selected, Now: req.Now})
			},
			fullFn: func(c *ui.Context, req ComponentRenderReq) ui.Element {
				return Chart(c, ChartProps{Data: req.Result.Chart, Style: req.Style, Height: req.Height, Selected: req.Selected, Pinned: req.Pinned, Full: true, Now: req.Now})
			},
			tilesFn: func(a *app, c *ui.Context, res engine.Result) bool {
				if a.kindTiles(c, res.Chart) {
					return true
				}
				a.statTiles(c, summarize(res.Chart))
				return true
			},
		})
	}
	for _, typ := range []string{"line", "dot", "candle", "gauge", "barh", "combo", "heat", "panels"} {
		registerPlot(typ)
	}

	// Standalone progressBar (and legacy "progress").
	pbComp := func(typ string) componentAdapter {
		return componentAdapter{
			typ: typ,
			cardFn: func(c *ui.Context, req ComponentRenderReq) ui.Element {
				pb, ok := progressBarOf(req.Result)
				if !ok {
					return ui.Text(c, "")
				}
				styled := applyBarStyleOverride(pb, req.Style)
				if req.Size == 0 && len(styled.Items) > 0 {
					compactPB := *styled
					compactPB.HideLabel = true
					if compactPB.Height <= 0 || compactPB.Height > 7 {
						compactPB.Height = 6
					}
					compactPB.Items = []engine.Progress{{
						Value: styled.Items[0].Value, Max: styled.Items[0].Max,
						Color: styled.Items[0].Color, To: styled.Items[0].To,
					}}
					return progressBarView(c, &compactPB, 1)
				}
				return progressBarView(c, styled, req.Limit)
			},
			fullFn: func(c *ui.Context, req ComponentRenderReq) ui.Element {
				pb, ok := progressBarOf(req.Result)
				if !ok {
					return ui.Text(c, "")
				}
				return progressBarView(c, applyBarStyleOverride(pb, req.Style), 0)
			},
			tilesFn: func(a *app, c *ui.Context, res engine.Result) bool {
				return true // progressBar intentionally omits chart stat tiles
			},
		}
	}
	registerComponent(pbComp("progressBar"))
	registerComponent(pbComp("progress"))

	// progressRing component.
	registerComponent(componentAdapter{
		typ: "progressRing",
		cardFn: func(c *ui.Context, req ComponentRenderReq) ui.Element {
			d := float32(46)
			limit := req.Limit
			if req.Size >= 2 {
				d = 64
				limit = 0
			}
			return progressRingView(c, req.Result.Chart, limit, d)
		},
		fullFn: func(c *ui.Context, req ComponentRenderReq) ui.Element {
			return progressRingView(c, req.Result.Chart, 0, 84)
		},
		tilesFn: func(a *app, c *ui.Context, res engine.Result) bool {
			return a.kindTiles(c, res.Chart)
		},
	})

	// calendar component.
	registerComponent(componentAdapter{
		typ: "calendar",
		cardFn: func(c *ui.Context, req ComponentRenderReq) ui.Element {
			return Chart(c, ChartProps{Data: req.Result.Chart, Height: req.Height, Now: req.Now})
		},
		fullFn: func(c *ui.Context, req ComponentRenderReq) ui.Element {
			return Chart(c, ChartProps{Data: req.Result.Chart, Height: req.Height, Selected: req.Selected, Full: true, Now: req.Now})
		},
		tilesFn: func(a *app, c *ui.Context, res engine.Result) bool { return true },
	})

	// Non-plot rich widgets.
	registerWidget := func(typ string) {
		registerComponent(componentAdapter{
			typ: typ,
			cardFn: func(c *ui.Context, req ComponentRenderReq) ui.Element {
				return widgetViewWith(c, req.App, req.Script.ID, req.Result.Chart, req.Style, req.Height, req.Selected, false, req.Now, req.Limit)
			},
			fullFn: func(c *ui.Context, req ComponentRenderReq) ui.Element {
				return widgetViewWith(c, req.App, req.Script.ID, req.Result.Chart, req.Style, req.Height, req.Selected, true, req.Now, 0)
			},
			tilesFn: func(a *app, c *ui.Context, res engine.Result) bool {
				return a.widgetTiles(c, res.Chart)
			},
		})
	}
	for _, typ := range []string{"donut", "checklist", "kv", "countdown", "clock", "contrib"} {
		registerWidget(typ)
	}

	// Native interactive data table component.
	registerComponent(componentAdapter{
		typ: "table",
		cardFn: func(c *ui.Context, req ComponentRenderReq) ui.Element {
			if req.Result.Chart == nil || req.Result.Chart.Table == nil {
				return ui.Text(c, tr("暂无数据")).FontSize(fsSmall).TextColor(pal(c).muted)
			}
			return req.App.tableWidgetView(c, "card:"+req.Script.ID, req.Result.Chart.Table, false, req.Limit)
		},
		fullFn: func(c *ui.Context, req ComponentRenderReq) ui.Element {
			if req.Result.Chart == nil || req.Result.Chart.Table == nil {
				return ui.Text(c, tr("暂无数据")).FontSize(fsSmall).TextColor(pal(c).muted)
			}
			return req.App.tableWidgetView(c, "full:"+req.Script.ID, req.Result.Chart.Table, true, 0)
		},
		tilesFn: func(a *app, c *ui.Context, res engine.Result) bool {
			return true
		},
	})
}

func applyBarStyleOverride(pb *engine.ProgressBar, style string) *engine.ProgressBar {
	if pb == nil {
		return nil
	}
	switch style {
	case "neon", "gradient", "solid", "segmented":
		cp := *pb
		cp.Style = style
		return &cp
	}
	return pb
}
