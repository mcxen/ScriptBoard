// Package spec is the plugin contract: what a script says about itself in
// meta() (the platforms it supports, the features it needs), what the host
// finds by looking at what the script uses, and what the two add up to.
//
//	declared (meta)  +  inferred (sources, feeds, usage providers)
//	        └────────────────────┬────────────────────┘
//	                        Resolved: where it runs, what it needs, what is wrong
//
// Built-in scripts, community scripts and local plugins all go through it, so
// the editor, the home page and the MCP tools say the same thing about each.
package spec

import (
	"fmt"
	"sort"

	"scriptboard/src/internal/caps"
	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/usage"
)

// Resolved is a script's platforms and needs after the host has added what it
// sees the script use to what the script declares.
type Resolved struct {
	// OSes are the systems the script can run on, in caps.Systems' order.
	OSes []string
	// Platforms say the same in the words a script uses: "desktop" or "mobile"
	// when every system of the group is there, the system's own name when only
	// some are.
	Platforms []string
	// Needs are the features it depends on, declared and inferred, in
	// caps.Needs' order.
	Needs []string
	// Declared is true when the script says anything about its platforms or
	// needs; false for a script written before the contract.
	Declared bool
	// Problems are contradictions the author should fix: a word that is not in
	// the contract, a need the script uses without declaring it, a platform it
	// claims while using what cannot run there.
	Problems []string
}

// Resolve works out what m means.
func Resolve(m engine.Meta) Resolved {
	r := Resolved{Declared: m.Platforms != nil || m.Needs != nil}

	// What the host sees the script use.
	inferred := map[string]bool{}
	if len(m.Sources) > 0 || len(m.Feeds) > 0 || m.NetProbe != nil {
		inferred[caps.Network] = true
	}
	allowed := map[string]bool{}
	for _, sys := range caps.Systems {
		allowed[sys] = true
	}
	for _, id := range m.Usage {
		plats, needs := usage.CapsOf(id)
		for _, n := range needs {
			inferred[n] = true
		}
		if plats != nil {
			here := map[string]bool{}
			for _, w := range plats {
				for _, sys := range caps.Expand(w) {
					here[sys] = true
				}
			}
			for sys := range allowed {
				if !here[sys] {
					delete(allowed, sys)
				}
			}
		}
	}
	declared := map[string]bool{}
	for _, n := range m.Needs {
		if !caps.ValidNeed(n) {
			r.Problems = append(r.Problems, fmt.Sprintf("needs: %q is not one of %v", n, caps.Needs))
			continue
		}
		declared[n] = true
	}

	// A need only a computer can meet takes the phones away.
	needs := map[string]bool{}
	for n := range declared {
		needs[n] = true
	}
	for n := range inferred {
		needs[n] = true
		if r.Declared && m.Needs != nil && !declared[n] {
			r.Problems = append(r.Problems, fmt.Sprintf("needs: %q is used but not declared", n))
		}
	}
	for n := range needs {
		if caps.DesktopOnly[n] {
			for _, sys := range caps.Expand(caps.Mobile) {
				delete(allowed, sys)
			}
		}
	}

	// The systems the script claims, cut down to the ones it can really run on.
	claimed := map[string]bool{}
	if len(m.Platforms) == 0 {
		for _, sys := range caps.Systems {
			claimed[sys] = true
		}
	}
	for _, w := range m.Platforms {
		members := caps.Expand(w)
		if len(members) == 0 {
			r.Problems = append(r.Problems, fmt.Sprintf("platforms: %q is not one of %v", w, caps.Platforms))
			continue
		}
		cut := false
		for _, sys := range members {
			claimed[sys] = true
			if !allowed[sys] {
				cut = true
			}
		}
		if cut {
			r.Problems = append(r.Problems, fmt.Sprintf("platforms: claims %q but what it uses cannot run there", w))
		}
	}
	for _, sys := range caps.Systems {
		if claimed[sys] && allowed[sys] {
			r.OSes = append(r.OSes, sys)
		}
	}
	r.Platforms = words(r.OSes)
	for _, n := range caps.Needs {
		if needs[n] {
			r.Needs = append(r.Needs, n)
		}
	}
	sort.Strings(r.Problems)
	return r
}

// words says a set of systems in platform words: a whole group by its name,
// the rest by system.
func words(oses []string) []string {
	var out []string
	have := map[string]bool{}
	for _, s := range oses {
		have[s] = true
	}
	for _, g := range []string{caps.Desktop, caps.Mobile} {
		members := caps.Expand(g)
		all := true
		for _, s := range members {
			all = all && have[s]
		}
		if all {
			out = append(out, g)
			for _, s := range members {
				delete(have, s)
			}
		}
	}
	for _, s := range caps.Systems {
		if have[s] {
			out = append(out, s)
		}
	}
	return out
}

// SupportsOS reports whether the script can run on the system.
func (r Resolved) SupportsOS(system string) bool { return contains(r.OSes, system) }

// Supports reports whether the script can run on every system a platform word
// stands for.
func (r Resolved) Supports(word string) bool {
	members := caps.Expand(word)
	if len(members) == 0 {
		return false
	}
	for _, s := range members {
		if !contains(r.OSes, s) {
			return false
		}
	}
	return true
}

// DesktopOnly reports whether the script runs on computers and nowhere else.
func (r Resolved) DesktopOnly() bool { return r.only(caps.Desktop) }

// MobileOnly reports whether the script runs on phones and nowhere else.
func (r Resolved) MobileOnly() bool { return r.only(caps.Mobile) }

func (r Resolved) only(group string) bool {
	if len(r.OSes) == 0 {
		return false
	}
	for _, s := range r.OSes {
		if caps.GroupOf(s) != group {
			return false
		}
	}
	return true
}

// PlatformDependent reports whether the script depends on something that only
// some platforms have: the computer's files, or programs to run.
func (r Resolved) PlatformDependent() bool {
	for _, n := range r.Needs {
		if caps.DesktopOnly[n] {
			return true
		}
	}
	return false
}

// Needing reports whether the script depends on the feature.
func (r Resolved) Needing(need string) bool { return contains(r.Needs, need) }

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
