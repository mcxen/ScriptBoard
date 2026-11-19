package spec

import (
	"regexp"
	"testing"

	"scriptboard/src/internal/caps"
	"scriptboard/src/internal/engine"
)

var (
	semver = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	cjk    = regexp.MustCompile(`[\x{4e00}-\x{9fff}]`)
)

// Every built-in script follows the contract: it declares a version, the
// platforms it runs on and the features it needs, describes itself, and what it
// declares agrees with what it uses. A new built-in that does not is caught
// here.
func TestBuiltinsFollowTheContract(t *testing.T) {
	for _, b := range engine.Builtins() {
		for _, lang := range []string{"zh", "en"} {
			s, err := engine.CompileLang(b.ID+".js", b.Source, lang)
			if err != nil {
				t.Fatalf("%s: %v", b.ID, err)
			}
			m := s.Meta
			if !semver.MatchString(m.Version) {
				t.Errorf("%s: version %q is not x.y.z", b.ID, m.Version)
			}
			if m.Platforms == nil || len(m.Platforms) == 0 {
				t.Errorf("%s: platforms are not declared", b.ID)
			}
			if m.Needs == nil {
				t.Errorf("%s: needs are not declared (use [] for none)", b.ID)
			}
			if m.Description == "" {
				t.Errorf("%s: no description", b.ID)
			}
			if lang == "en" && cjk.MatchString(m.Description) {
				t.Errorf("%s: Chinese in the English description %q", b.ID, m.Description)
			}
			r := Resolve(m)
			if len(r.Problems) != 0 {
				t.Errorf("%s: %v", b.ID, r.Problems)
			}
			if len(r.Platforms) == 0 {
				t.Errorf("%s: runs nowhere", b.ID)
			}
		}
	}
}

// Claude Code's usage is the example of a script that depends on the platform:
// it reads the computer's files, so it is for the computer, and says so.
func TestClaudeCodeScriptIsDesktopAndFiles(t *testing.T) {
	for _, b := range engine.Builtins() {
		if b.ID != "ai-claude" {
			continue
		}
		s, err := engine.Compile("ai-claude.js", b.Source)
		if err != nil {
			t.Fatal(err)
		}
		r := Resolve(s.Meta)
		if !r.DesktopOnly() || !r.PlatformDependent() || !r.Needing(caps.Files) {
			t.Errorf("ai-claude: %+v", r)
		}
		if r.Supports(caps.Mobile) {
			t.Errorf("ai-claude claims the phone")
		}
		return
	}
	t.Fatal("no ai-claude builtin")
}

// The scripts that make requests say so, and the one that only reads the user's
// own records does not.
func TestBuiltinNeedsMatchTheirUse(t *testing.T) {
	want := map[string]bool{ // needs the network
		"weight": false, "goldk": false,
		"fx": true, "weather": true, "aqi": true, "gold": true, "goldacc": true, "hs300": true, "btc": true,
		"ai-deepseek": true, "ai-openrouter": true, "ai-claude": false,
	}
	for _, b := range engine.Builtins() {
		s, err := engine.Compile(b.ID+".js", b.Source)
		if err != nil {
			t.Fatal(err)
		}
		net, known := want[b.ID]
		if !known {
			t.Errorf("%s: add it to this table", b.ID)
			continue
		}
		if got := Resolve(s.Meta).Needing(caps.Network); got != net {
			t.Errorf("%s: needs network %v, want %v", b.ID, got, net)
		}
	}
}
