// Package caps names where a script can run and what it depends on, in words
// that scripts, providers and the host all use. It has no dependencies, so any
// package may import it.
//
// A script says in its meta() which platforms it supports and which platform
// features it needs; the host checks that against what the script really uses
// (package spec) and against the system it runs on.
//
// A platform is a system ("macos", "windows", "linux", "android", "ios") or a
// group of them ("desktop" for the first three, "mobile" for the last two). A
// script that is only for Windows says "windows" and is not offered, and does
// not run, anywhere else.
package caps

import (
	"os"
	"runtime"
	"strings"
	"sync/atomic"
)

// Systems.
const (
	MacOS   = "macos"
	Windows = "windows"
	Linux   = "linux"
	Android = "android"
	IOS     = "ios"
)

// Groups of systems.
const (
	Desktop = "desktop"
	Mobile  = "mobile"
)

// Systems lists every system, in the order the interface shows them.
var Systems = []string{MacOS, Windows, Linux, Android, IOS}

// Platforms lists every word a script may use for where it runs: the two
// groups, then the systems.
var Platforms = []string{Desktop, Mobile, MacOS, Windows, Linux, Android, IOS}

var groups = map[string][]string{
	Desktop: {MacOS, Windows, Linux},
	Mobile:  {Android, IOS},
}

// Expand is the systems a platform word stands for: a group's members, or the
// system itself. An unknown word is none.
func Expand(word string) []string {
	if g, ok := groups[word]; ok {
		return append([]string(nil), g...)
	}
	if contains(Systems, word) {
		return []string{word}
	}
	return nil
}

// GroupOf is the group a system belongs to.
func GroupOf(system string) string {
	for g, members := range groups {
		if contains(members, system) {
			return g
		}
	}
	return ""
}

// Needs: the platform features a script depends on.
const (
	// Network: it makes HTTP requests (sources, feeds). All systems have it.
	Network = "network"
	// Files: it reads files that only a computer has, such as the logs of a
	// tool installed there. A phone does not have them.
	Files = "files"
	// Exec: it runs a program on the computer.
	Exec = "exec"
	// Key: it needs a key or token of a service that the user enters.
	Key = "key"
)

// Needs lists every valid need, in the order the interface shows them.
var Needs = []string{Network, Files, Exec, Key}

// DesktopOnly are the needs a phone cannot meet.
var DesktopOnly = map[string]bool{Files: true, Exec: true}

// Sensitive are the needs that reach beyond the app: a script that is not built
// in may use them only after the user allows it.
var Sensitive = map[string]bool{Files: true, Exec: true, Key: true}

var current atomic.Value

// OS is the system the app runs on. SCRIPTBOARD_OS (a system) makes a computer
// behave as another system, and SCRIPTBOARD_PLATFORM (a group) as a system of
// that group, to try their rules.
func OS() string {
	if v, ok := current.Load().(string); ok && v != "" {
		return v
	}
	if s := strings.ToLower(os.Getenv("SCRIPTBOARD_OS")); contains(Systems, s) {
		return s
	}
	own := native()
	switch strings.ToLower(os.Getenv("SCRIPTBOARD_PLATFORM")) {
	case Desktop:
		if GroupOf(own) == Desktop {
			return own
		}
		return MacOS
	case Mobile:
		if GroupOf(own) == Mobile {
			return own
		}
		return Android
	}
	return own
}

func native() string {
	switch runtime.GOOS {
	case "darwin":
		return MacOS
	case "windows":
		return Windows
	case "android":
		return Android
	case "ios":
		return IOS
	}
	return Linux
}

// Platform is the group of the system the app runs on.
func Platform() string { return GroupOf(OS()) }

// SetOS fixes the system, for tests; "" goes back to the device's own. A group
// word fixes the first system of the group.
func SetOS(system string) {
	if g := Expand(system); len(g) > 0 {
		system = g[0]
	}
	current.Store(system)
}

// SetPlatform is SetOS, kept for the groups: SetPlatform(Mobile) is Android.
func SetPlatform(p string) { SetOS(p) }

// ValidPlatform and ValidNeed report whether a word is one of the lists above.
func ValidPlatform(p string) bool { return contains(Platforms, p) }
func ValidNeed(n string) bool     { return contains(Needs, n) }

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
