package spec

import (
	"strings"
	"testing"

	"scriptboard/src/internal/caps"
	"scriptboard/src/internal/engine"
)

func has(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// A script that says nothing runs on both platforms, and needs the network
// only if it has sources.
func TestUndeclaredScriptRunsEverywhere(t *testing.T) {
	r := Resolve(engine.Meta{Name: "plain"})
	if r.Declared || !r.Supports(caps.Desktop) || !r.Supports(caps.Mobile) || len(r.Needs) != 0 || len(r.Problems) != 0 {
		t.Errorf("plain script: %+v", r)
	}
	r = Resolve(engine.Meta{Sources: map[string]string{"x": "https://example.com"}})
	if !r.Needing(caps.Network) || r.Declared || len(r.Problems) != 0 {
		t.Errorf("script with a source: %+v", r)
	}
}

// Claude Code's logs are on the computer: a script that reads them is a desktop
// script, and says so to the host without declaring it.
func TestClaudeCodeUsageIsDesktopOnly(t *testing.T) {
	r := Resolve(engine.Meta{Usage: []string{"claude-code"}})
	if !r.DesktopOnly() || r.Supports(caps.Mobile) || !r.PlatformDependent() || !r.Needing(caps.Files) {
		t.Errorf("claude-code script: %+v", r)
	}
}

// A balance provider works anywhere, with a key and the network.
func TestBalanceUsageNeedsKeyAndNetwork(t *testing.T) {
	r := Resolve(engine.Meta{Usage: []string{"deepseek-balance"}})
	if !r.Supports(caps.Mobile) || !r.Needing(caps.Network) || !r.Needing(caps.Key) || r.PlatformDependent() {
		t.Errorf("balance script: %+v", r)
	}
}

// A script that claims the phone while it reads the computer's files is told
// so, and is not offered on the phone.
func TestClaimingAPhoneItCannotRunOn(t *testing.T) {
	r := Resolve(engine.Meta{
		Platforms: []string{caps.Desktop, caps.Mobile},
		Needs:     []string{caps.Files},
	})
	if r.Supports(caps.Mobile) || !r.Supports(caps.Desktop) {
		t.Errorf("platforms %v", r.Platforms)
	}
	if len(r.Problems) == 0 || !strings.Contains(r.Problems[0], "mobile") {
		t.Errorf("no problem reported: %v", r.Problems)
	}
}

// A declared script is held to what it uses: a need it does not list is a
// problem, and a word outside the contract is one too.
func TestDeclaredScriptsAreChecked(t *testing.T) {
	r := Resolve(engine.Meta{
		Needs:   []string{},
		Sources: map[string]string{"x": "https://example.com"},
	})
	if len(r.Problems) != 1 || !strings.Contains(r.Problems[0], "network") {
		t.Errorf("undeclared network: %v", r.Problems)
	}
	r = Resolve(engine.Meta{Platforms: []string{"tablet"}, Needs: []string{"magic"}})
	if len(r.Problems) != 2 {
		t.Errorf("bad words: %v", r.Problems)
	}
}

// A script can limit itself to a phone, and the other way round.
func TestPlatformsCanBeLimited(t *testing.T) {
	r := Resolve(engine.Meta{Platforms: []string{caps.Mobile}})
	if !r.Supports(caps.Mobile) || r.Supports(caps.Desktop) {
		t.Errorf("mobile only: %v", r.Platforms)
	}
}

// The platform follows the device, or what a test sets.
func TestPlatformCanBeSet(t *testing.T) {
	t.Cleanup(func() { caps.SetPlatform("") })
	caps.SetPlatform(caps.Mobile)
	if caps.Platform() != caps.Mobile {
		t.Errorf("platform %q", caps.Platform())
	}
	caps.SetPlatform("")
	if p := caps.Platform(); p != caps.Desktop && p != caps.Mobile {
		t.Errorf("platform %q", p)
	}
}

// A script for one system runs there and nowhere else: Windows only is not
// offered on macOS.
func TestOneSystemOnly(t *testing.T) {
	r := Resolve(engine.Meta{Platforms: []string{caps.Windows}})
	if !r.SupportsOS(caps.Windows) || r.SupportsOS(caps.MacOS) || r.SupportsOS(caps.Linux) || r.SupportsOS(caps.Android) {
		t.Errorf("windows only: %v", r.OSes)
	}
	if len(r.Platforms) != 1 || r.Platforms[0] != caps.Windows {
		t.Errorf("platform words %v", r.Platforms)
	}
	if r.DesktopOnly() != true || r.MobileOnly() {
		t.Errorf("windows only is a desktop script: %+v", r)
	}
}

// Systems and groups mix, and the words say them back the short way.
func TestSystemsAndGroups(t *testing.T) {
	r := Resolve(engine.Meta{Platforms: []string{caps.Windows, caps.Linux}})
	if got := strings.Join(r.Platforms, ","); got != "windows,linux" {
		t.Errorf("windows and linux: %q", got)
	}
	r = Resolve(engine.Meta{Platforms: []string{caps.Desktop, caps.IOS}})
	if got := strings.Join(r.Platforms, ","); got != "desktop,ios" {
		t.Errorf("desktop and ios: %q", got)
	}
	if r.Supports(caps.Mobile) || !r.SupportsOS(caps.IOS) || r.SupportsOS(caps.Android) {
		t.Errorf("desktop and ios: %v", r.OSes)
	}
	if got := strings.Join(Resolve(engine.Meta{}).Platforms, ","); got != "desktop,mobile" {
		t.Errorf("undeclared: %q", got)
	}
}

// Claiming a system that what the script uses cannot run on is a problem, and
// the system is left out: Claude Code's logs are on computers.
func TestSystemClaimsAreChecked(t *testing.T) {
	r := Resolve(engine.Meta{Platforms: []string{caps.Windows, caps.Android}, Usage: []string{"claude-code"}})
	if !r.SupportsOS(caps.Windows) || r.SupportsOS(caps.Android) {
		t.Errorf("systems %v", r.OSes)
	}
	if len(r.Problems) != 1 || !strings.Contains(r.Problems[0], "android") {
		t.Errorf("problems %v", r.Problems)
	}
	r = Resolve(engine.Meta{Platforms: []string{caps.Android}, Needs: []string{caps.Exec}})
	if len(r.OSes) != 0 || len(r.Problems) == 0 {
		t.Errorf("a phone script that runs programs: %+v", r)
	}
}

// The system follows the device, or what a test sets, and a group word picks a
// system of it.
func TestSystemCanBeSet(t *testing.T) {
	t.Cleanup(func() { caps.SetOS("") })
	caps.SetOS(caps.Windows)
	if caps.OS() != caps.Windows || caps.Platform() != caps.Desktop {
		t.Errorf("os %q platform %q", caps.OS(), caps.Platform())
	}
	caps.SetPlatform(caps.Mobile)
	if caps.OS() != caps.Android || caps.Platform() != caps.Mobile {
		t.Errorf("os %q platform %q", caps.OS(), caps.Platform())
	}
}
