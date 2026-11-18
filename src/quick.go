package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/engine"
)

// quickEntry is the row on a home card for a value the user types in by hand,
// a weight or a price: a number field that starts at the latest value, and a
// button that records it. A script declares such values in meta().inputs.
func (a *app) quickEntry(c *ui.Context, scriptID string, in engine.ManualInput) {
	p := pal(c)
	key := scriptID + "|" + in.Metric
	v, ok := a.quick[key]
	if !ok {
		v = new(float64)
		if pts := a.store.Series(in.Metric); len(pts) > 0 {
			*v = a.shownValue(in.Metric, pts[len(pts)-1].V)
		}
		a.quick[key] = v
	}
	unit := in.Unit
	if in.Metric == "weight" {
		unit = a.store.Settings().WeightUnit
	}
	step := in.Step
	if step <= 0 {
		step = 0.1
	}
	// The label gives way first, so the field and the button stay on the card.
	ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
		// A text keeps its width, so the label sits in a box that can shrink.
		ui.Row(c).Grow(1).Shrink(1).MinWidth(0).ClipX().Children(func() {
			ui.Text(c, in.Label+"（"+unit+"）").FontSize(fsSmall).TextColor(p.muted).Ellipsis("…").Shrink(1).MinWidth(0)
		})
		ui.Row(c).Shrink(0).Width(112).Children(func() {
			numberInput(c, v, 0, 1e6, step).Width(112)
		})
		if primaryBtn(c, tr("记录")).Shrink(0).Clicked() {
			if err := a.recordMetric(in.Metric, *v); err != nil {
				a.quickNote = tr("记录失败：") + err.Error()
			} else {
				a.quickNote = fmt.Sprintf(tr("已记录 %s %.2f"), in.Label, *v)
			}
		}
	})
}
