package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

// varsOf is the value of each of the script's variables: the user's choice, or
// the script's default.
func (a *app) varsOf(sc store.Script, m engine.Meta) map[string]string {
	return engine.ResolveVars(m.Vars, a.set.ScriptVars[sc.ID])
}

// varSuffix tells the runs of a script apart by the values of its variables, so
// that a new choice never shows an old result. Password variables are hashed so
// raw secrets never appear in run keys.
func (a *app) varSuffix(sc store.Script, m engine.Meta) string {
	if len(m.Vars) == 0 {
		return ""
	}
	vals := a.varsOf(sc, m)
	var b strings.Builder
	for _, v := range m.Vars {
		val := vals[v.Key]
		if v.Type == "password" && val != "" {
			sum := sha256.Sum256([]byte(val))
			val = hex.EncodeToString(sum[:8])
		}
		fmt.Fprintf(&b, "|%s=%s", v.Key, val)
	}
	return b.String()
}

// runKeyOf is the key of the run of sc with the values of its dimensions and
// variables.
func (a *app) runKeyOf(id string, sc store.Script, m engine.Meta, vals map[string]float64) string {
	return runKey(id, m.Dims, vals) + a.varSuffix(sc, m)
}

// setVar keeps the user's choice of a variable, and runs the script again with
// it.
func (a *app) setVar(sc store.Script, key, value string) {
	if a.set.ScriptVars == nil {
		a.set.ScriptVars = map[string]map[string]string{}
	}
	if a.set.ScriptVars[sc.ID] == nil {
		a.set.ScriptVars[sc.ID] = map[string]string{}
	}
	a.set.ScriptVars[sc.ID][key] = value
	if err := a.store.SetSettings(a.set); err != nil {
		a.set = a.store.Settings()
	}
	a.invalidate(sc.ID)
	a.invalidate(draftKey)
}

// varRow is the switch or text/password editor for each of the script's
// variables: a city, a currency pair, or a cookie/token. A script with none
// draws nothing.
func (a *app) varRow(c *ui.Context, sc store.Script, m engine.Meta) {
	if len(m.Vars) == 0 {
		return
	}
	p := pal(c)
	vals := a.varsOf(sc, m)
	if a.barInputDrafts == nil {
		a.barInputDrafts = map[string]map[string]string{}
	}
	if a.barInputDrafts[sc.ID] == nil {
		a.barInputDrafts[sc.ID] = map[string]string{}
	}
	drafts := a.barInputDrafts[sc.ID]

	for _, v := range m.Vars {
		v := v
		name := v.Label
		if name == "" {
			name = v.Key
		}
		if len(v.Options) == 0 {
			curVal := vals[v.Key]
			if _, ok := drafts[v.Key]; !ok {
				drafts[v.Key] = curVal
			}
			dVal := drafts[v.Key]
			dirty := dVal != curVal
			ph := v.Placeholder
			if ph == "" {
				ph = v.Default
			}
			ui.Row(c.Key("var:"+sc.ID+":"+v.Key)).AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Text(c, name).FontSize(fsSmall).TextColor(p.muted)
				var ti ui.Element
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					if v.Type == "password" {
						ti = passwordInput(c.Key("var-in:"+sc.ID+":"+v.Key), &dVal, ph).FillWidth()
					} else {
						ti = textInput(c.Key("var-in:"+sc.ID+":"+v.Key), &dVal, ph).FillWidth()
					}
				})
				drafts[v.Key] = dVal
				if ti.Submitted() {
					a.setVar(sc, v.Key, dVal)
				}
				if dirty {
					if primaryBtn(c, tr("应用")).Clicked() {
						a.setVar(sc, v.Key, dVal)
					}
				}
			})
			continue
		}

		labels := make([]string, len(v.Options))
		idx := 0
		for i, o := range v.Options {
			labels[i] = o.Label
			if o.Value == vals[v.Key] {
				idx = i
			}
		}
		ui.Row(c.Key("var:"+sc.ID+":"+v.Key)).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, name).FontSize(fsSmall).TextColor(p.muted)
			pick := choose
			if len(labels) > 4 {
				pick = dropdown
			}
			if pick(c, &idx, labels...) {
				a.setVar(sc, v.Key, v.Options[idx].Value)
			}
		})
	}
}

// resetData forgets what the script has read and kept: the responses and the
// history of each of its sources (for every value of its variables), its saved
// results, and the samples of its data sources. Its next run asks the network
// afresh, so an old reading never sits beside a new one.
func (a *app) resetData(sc store.Script) error {
	m, err := a.metaOf(sc)
	if err != nil {
		// A script that does not compile has no sources to forget, but its
		// saved results still go.
		m = engine.Meta{}
	}
	now := a.now()
	if a.cache != nil {
		combos := append(engine.Combinations(m.Vars), a.varsOf(sc, m))
		for _, vals := range combos {
			for name, src := range m.Sources {
				r := src.Request(now, vals)
				for k, v := range m.SourceHeaders[name] {
					r.Headers[k] = engine.ExpandVars(v, now, vals)
				}
				if err := a.cache.Forget(r.CacheKey()); err != nil {
					return err
				}
				if r.URL != r.CacheKey() {
					_ = a.cache.Forget(r.URL)
				}
			}
		}
		if err := a.cache.ForgetOwner(sc.ID); err != nil {
			return err
		}
	}
	for _, id := range m.Feeds {
		if err := a.samples.Reset(id); err != nil {
			return err
		}
	}
	if m.NetProbe != nil {
		a.forgetNetProbe()
	}
	a.invalidate(sc.ID)
	a.invalidate(draftKey)
	a.forced[sc.ID] = true
	return nil
}

// resetButton forgets a script's data, as a button. The first click asks for a
// second one, as deleting does, since what is forgotten is read again.
func (a *app) resetButton(c *ui.Context, sc store.Script) {
	label := tr("重置数据")
	if a.confirmReset == sc.ID {
		label = tr("确认重置")
	}
	if btn(c, label).Clicked() {
		a.clickReset(sc)
	}
}

// resetIcon is resetButton as an icon, for a phone.
func (a *app) resetIcon(c *ui.Context, sc store.Script) {
	if action.ActionIcon(c, icon("rotate-ccw"), action.ActionIconProps{
		Size: theme.Small, Title: tr("重置数据"), Active: a.confirmReset == sc.ID,
	}).Clicked() {
		a.clickReset(sc)
	}
}

// clickReset resets on the second click, and says so on the first.
func (a *app) clickReset(sc store.Script) {
	if a.confirmReset != sc.ID {
		a.confirmReset = sc.ID
		a.status = tr("再点一次「确认重置」才会清空这个脚本的数据")
		return
	}
	a.confirmReset = ""
	if err := a.resetData(sc); err != nil {
		a.status = tr("重置失败：") + err.Error()
		return
	}
	a.status = tr("已重置，下次打开会重新取数")
}
