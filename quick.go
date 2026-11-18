package main

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"scriptboard/internal/engine"
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
	ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
		ui.Text(c, in.Label+"（"+unit+"）").FontSize(fsSmall).TextColor(p.muted).NoWrap()
		ui.Spacer(c)
		numberInput(c, v, 0, 1e6, step).Width(112)
		if primaryBtn(c, tr("记录")).Clicked() {
			if err := a.recordMetric(in.Metric, *v); err != nil {
				a.quickNote = tr("记录失败：") + err.Error()
			} else {
				a.quickNote = fmt.Sprintf(tr("已记录 %s %.2f"), in.Label, *v)
			}
		}
	})
}
