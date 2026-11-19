package main

import (
	"errors"
	"strings"

	"scriptboard/src/internal/caps"
	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/store"
)

// Permissions. A script reaches the machine only through the interfaces the
// host gives it (ctx.usage, ctx.feeds, ctx.sources, ctx.metrics), and the host
// is where the system's differences and the user's permission are handled: a
// script never opens a file or starts a program itself. A script that is not
// built in may use a sensitive feature (the computer's files, programs, a
// service key) only after the user allows it, one script and one feature at a
// time.

// trusted reports whether a script is the app's own, unchanged: built in, and
// still the source that shipped.
func (a *app) trusted(sc store.Script) bool {
	if a.originOf(sc) != store.OriginBuiltin {
		return false
	}
	for _, b := range engine.Builtins() {
		if b.ID == sc.ID {
			return b.Source == sc.Source
		}
	}
	return false
}

// feedNeeds are the features the data sources a script reads depend on: a
// source that reads a file or runs a program takes that feature along.
func (a *app) feedNeeds(names []string) []string {
	var out []string
	add := func(n string) {
		for _, x := range out {
			if x == n {
				return
			}
		}
		out = append(out, n)
	}
	for _, id := range names {
		src, ok := a.store.Feed(id)
		if !ok {
			continue
		}
		for _, r := range src.Requests() {
			switch strings.ToLower(strings.SplitN(r.URL, ":", 2)[0]) {
			case "file":
				add(caps.Files)
			case "exec":
				add(caps.Exec)
			}
		}
	}
	return out
}

// needsOf are all the features a script depends on: what it declares and the
// host sees it use, and what the data sources it reads take along.
func (a *app) needsOf(sc store.Script) []string {
	needs := append([]string(nil), a.specOf(sc).Needs...)
	if m, err := a.metaOf(sc); err == nil {
		for _, n := range a.feedNeeds(m.Feeds) {
			have := false
			for _, x := range needs {
				have = have || x == n
			}
			if !have {
				needs = append(needs, n)
			}
		}
	}
	return needs
}

// granted reports whether a script may use a feature: any that is not
// sensitive, any for an unchanged built-in script, and the ones the user
// allowed for the others.
func (a *app) granted(sc store.Script, need string) bool {
	if !caps.Sensitive[need] || a.trusted(sc) {
		return true
	}
	for _, n := range a.set.Grants[sc.ID] {
		if n == need {
			return true
		}
	}
	return false
}

// missingGrants are the sensitive features a script uses that the user has not
// allowed.
func (a *app) missingGrants(sc store.Script) []string {
	var out []string
	for _, n := range a.needsOf(sc) {
		if !a.granted(sc, n) {
			out = append(out, n)
		}
	}
	return out
}

// setGrant allows or takes back one feature for one script, and refreshes only
// that script's card.
func (a *app) setGrant(id, need string, on bool) {
	var keep []string
	for _, n := range a.set.Grants[id] {
		if n != need {
			keep = append(keep, n)
		}
	}
	if on {
		keep = append(keep, need)
	}
	if a.set.Grants == nil {
		a.set.Grants = map[string][]string{}
	}
	if len(keep) == 0 {
		delete(a.set.Grants, id)
	} else {
		a.set.Grants[id] = keep
	}
	if err := a.store.SetSettings(a.set); err != nil {
		a.set = a.store.Settings()
	}
	a.invalidate(id)
	a.invalidate(draftKey) // the editor's preview, if it shows this script
}

// errNeedsGrant is why a script does not run: it uses features the user has
// not allowed.
func errNeedsGrant(missing []string) error {
	names := make([]string, len(missing))
	for i, n := range missing {
		names[i] = needLabel(n)
	}
	return errors.New(tr("需要授权：") + strings.Join(names, "、") + tr("。到「编辑」里这个脚本的说明处授权后才能运行。"))
}

// grantedList are the sensitive features a script uses and may use: what it
// is handed as ctx.granted.
func (a *app) grantedList(sc store.Script) []string {
	var out []string
	for _, n := range a.needsOf(sc) {
		if caps.Sensitive[n] && a.granted(sc, n) {
			out = append(out, n)
		}
	}
	return out
}
