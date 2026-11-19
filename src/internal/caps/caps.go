// Package caps names where a script can run and what it depends on, in words
// that scripts, providers and the host all use. It has no dependencies, so any
// package may import it.
//
// A script says in its meta() which platforms it supports and which platform
// features it needs; the host checks that against what the script really uses
// (package spec) and against the device it runs on.
package caps

import (
	"os"
	"runtime"
	"strings"
	"sync/atomic"
)

// Platforms: a computer, or a phone or tablet.
const (
	Desktop = "desktop"
	Mobile  = "mobile"
)

// Needs: the platform features a script depends on.
const (
	// Network: it makes HTTP requests (sources, feeds). All platforms have it.
	Network = "network"
	// Files: it reads files that only the computer has, such as the logs of a
	// tool installed there. A phone does not have them.
	Files = "files"
	// Exec: it runs a program on the computer.
	Exec = "exec"
	// Key: it needs a key or token of a service that the user enters.
	Key = "key"
)

// Platforms and Needs list every valid word, in the order the interface shows
// them.
var (
	Platforms = []string{Desktop, Mobile}
	Needs     = []string{Network, Files, Exec, Key}
)

// DesktopOnly are the needs a phone cannot meet.
var DesktopOnly = map[string]bool{Files: true, Exec: true}

var current atomic.Value

// Platform is the kind of device the app runs on: Mobile on Android and iOS,
// Desktop anywhere else. SCRIPTBOARD_PLATFORM=mobile makes a computer behave as
// a phone, to try the phone's rules.
func Platform() string {
	if v, ok := current.Load().(string); ok && v != "" {
		return v
	}
	if p := strings.ToLower(os.Getenv("SCRIPTBOARD_PLATFORM")); p == Desktop || p == Mobile {
		return p
	}
	if runtime.GOOS == "android" || runtime.GOOS == "ios" {
		return Mobile
	}
	return Desktop
}

// SetPlatform fixes the platform, for tests; "" goes back to the device's own.
func SetPlatform(p string) { current.Store(p) }

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
