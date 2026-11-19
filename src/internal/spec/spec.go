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
	// Platforms are where the script can run, in caps.Platforms' order.
	Platforms []string
	// Needs are the features it depends on, declared and inferred, in
	// caps.Needs' order.
	Needs []string
	// Declared is true when the script says anything about its platforms or
	// needs; false for a script written before the contract.
	Declared bool
	// Problems are contradictions the author should fix: a word that is not in
	// the contract, a need the script uses without declaring it, a phone it
	// claims to support while reading the computer's files.
	Problems []string
}

// Resolve works out what m means.
func Resolve(m engine.Meta) Resolved {
	r := Resolved{Declared: m.Platforms != nil || m.Needs != nil}

	// What the host sees the script use.
	inferred := map[string]bool{}
	if len(m.Sources) > 0 || len(m.Feeds) > 0 {
		inferred[caps.Network] = true
	}
	allowed := map[string]bool{caps.Desktop: true, caps.Mobile: true}
	for _, id := range m.Usage {
		plats, needs := usage.CapsOf(id)
		for _, n := range needs {
			inferred[n] = true
		}
		if plats != nil {
			for p := range allowed {
				if !contains(plats, p) {
					delete(allowed, p)
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

	// A need only the computer can meet takes the phone away.
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
			delete(allowed, caps.Mobile)
		}
	}

	// The platforms the script claims, cut down to the ones it can really run on.
	claimed := map[string]bool{}
	for _, p := range m.Platforms {
		if !caps.ValidPlatform(p) {
			r.Problems = append(r.Problems, fmt.Sprintf("platforms: %q is not one of %v", p, caps.Platforms))
			continue
		}
		claimed[p] = true
	}
	for _, p := range caps.Platforms {
		if len(m.Platforms) > 0 && !claimed[p] {
			continue
		}
		if !allowed[p] {
			if claimed[p] {
				r.Problems = append(r.Problems, fmt.Sprintf("platforms: claims %q but what it uses cannot run there", p))
			}
			continue
		}
		r.Platforms = append(r.Platforms, p)
	}
	for _, n := range caps.Needs {
		if needs[n] {
			r.Needs = append(r.Needs, n)
		}
	}
	sort.Strings(r.Problems)
	return r
}

// Supports reports whether the script can run on platform.
func (r Resolved) Supports(platform string) bool { return contains(r.Platforms, platform) }

// DesktopOnly reports whether the script runs on a computer and nowhere else.
func (r Resolved) DesktopOnly() bool {
	return len(r.Platforms) == 1 && r.Platforms[0] == caps.Desktop
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
