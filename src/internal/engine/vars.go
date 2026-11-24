package engine

import (
	"strings"
	"time"
)

// A variable is a key a script's sources depend on, which the user switches
// without touching the script: a city, a currency pair. A source URL writes
// it as {key}, and the value the user chose fills it in. Each combination of
// values is its own data: its own cache, history and result.
//
//	meta(): vars: [{ key: "city", label: "城市", default: "shanghai",
//	                 options: [{ value: "shanghai", label: "上海" }, …] }],
//	        sources: { w: "https://wttr.in/{city}?format=j1" }
//
// render reads the chosen values as ctx.vars.city.

// Var is a variable the user switches or fills in.
type Var struct {
	Key         string      `json:"key"`
	Label       string      `json:"label"`
	Type        string      `json:"type,omitempty"`        // "", "select", "string", "password"
	Default     string      `json:"default"`
	Placeholder string      `json:"placeholder,omitempty"`
	Help        string      `json:"help,omitempty"`
	Options     []VarOption `json:"options"`
}

// VarOption is one value of a variable: what fills the URL, and what the user
// reads.
type VarOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// maxCombos bounds how many combinations of values a script can have, so a
// long list of options cannot make a reset walk thousands of URLs.
const maxCombos = 256

// ResolveVars is the value of each variable: for option variables, the chosen
// value when it is one of the options, else default, else first option; for
// free-text/password variables (no options), the chosen value or default.
func ResolveVars(vars []Var, chosen map[string]string) map[string]string {
	out := make(map[string]string, len(vars))
	for _, v := range vars {
		val := ""
		if len(v.Options) == 0 {
			if c, ok := chosen[v.Key]; ok {
				val = c
			} else {
				val = v.Default
			}
			out[v.Key] = val
			continue
		}
		if c, ok := chosen[v.Key]; ok && hasOption(v, c) {
			val = c
		} else if hasOption(v, v.Default) {
			val = v.Default
		} else if len(v.Options) > 0 {
			val = v.Options[0].Value
		}
		out[v.Key] = val
	}
	return out
}

func hasOption(v Var, value string) bool {
	for _, o := range v.Options {
		if o.Value == value {
			return true
		}
	}
	return false
}

// ExpandVars fills the variables and the dates of a source URL.
func ExpandVars(tmpl string, now time.Time, vals map[string]string) string {
	if len(vals) > 0 {
		args := make([]string, 0, 2*len(vals))
		for k, v := range vals {
			args = append(args, "{"+k+"}", v)
		}
		tmpl = strings.NewReplacer(args...).Replace(tmpl)
	}
	return Expand(tmpl, now)
}

// Combinations lists every combination of the variables' options, as the values
// a run can have. It stops at maxCombos.
func Combinations(vars []Var) []map[string]string {
	out := []map[string]string{{}}
	for _, v := range vars {
		if len(v.Options) == 0 {
			continue
		}
		var next []map[string]string
		for _, base := range out {
			for _, o := range v.Options {
				m := make(map[string]string, len(base)+1)
				for k, val := range base {
					m[k] = val
				}
				m[v.Key] = o.Value
				next = append(next, m)
				if len(next) >= maxCombos {
					return next
				}
			}
		}
		out = next
	}
	return out
}
