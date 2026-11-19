// Package store keeps ScriptBoard's data in one JSON file: the scripts, the
// recorded values and the settings. Every change is written at once, through
// a temporary file, so a crash never leaves half a file behind.
package store

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/feed"
)

// Where a script came from.
const (
	// OriginBuiltin: it ships with the app.
	OriginBuiltin = "builtin"
	// OriginCommunity: it was downloaded from the community.
	OriginCommunity = "community"
	// OriginPlugin: it is a local plugin, kept in the plugin folder.
	OriginPlugin = "plugin"
	// OriginUser: the user wrote, copied or imported it.
	OriginUser = "user"
)

// Script is a user's script as stored.
type Script struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Source string `json:"source"`
	// Home shows the script as a card on the home page.
	Home bool `json:"home"`
	// Origin is where the script came from: one of the Origin constants. It is
	// empty on a script saved before origins existed; the app works it out.
	Origin string `json:"origin,omitempty"`
	// CommunityID is the id the script has in the community, for one that was
	// downloaded from it.
	CommunityID string `json:"communityID,omitempty"`
}

// Settings are the app's settings.
type Settings struct {
	// WeightUnit is "kg" or "斤".
	WeightUnit string `json:"weightUnit"`
	// Lang is "zh", "en", or "" to follow the language of the computer.
	Lang string `json:"lang"`
	// Skin is the look of the app: "" the default, "mac9" Mac OS 9.
	Skin string `json:"skin"`
	// ThemeMode is the appearance: "" or "system" follows the system, "light"
	// and "dark" force one.
	ThemeMode string `json:"themeMode"`
	// SidebarCollapsed folds the sidebar into a narrow rail of icons.
	SidebarCollapsed bool `json:"sidebarCollapsed"`
	// UpdateSource is where updates come from: "" automatic (the tvv.tw
	// mirror for Chinese, GitHub otherwise), "github" or "tvv".
	UpdateSource string `json:"updateSource"`
	// AllowExec lets a data source run a local program (the exec protocol).
	// Off until the user turns it on.
	AllowExec bool `json:"allowExec"`
	// Offline stops network requests; scripts show their sample data.
	Offline bool `json:"offline"`
	// RemindAt is the daily reminder time, "HH:MM". Not wired to
	// notifications yet.
	RemindAt string `json:"remindAt"`
	// CommunityURL is the address of the community Worker, without a slash at
	// the end. Empty means the community is not set up.
	CommunityURL string `json:"communityURL"`
	// Username and Token are the login of the community account. The token is
	// kept in this file on this computer only.
	Username string `json:"username"`
	Token    string `json:"token"`
	// MCP is the Model Context Protocol server the app runs for assistants.
	MCP MCPSettings `json:"mcp"`
	// Usage holds the key and the budgets of each AI usage provider, by ID.
	Usage map[string]UsageConfig `json:"usage"`
	// CardSizes is the size of each home card that is not the default: 0
	// small, 1 medium, 2 large, by script ID.
	CardSizes map[string]int `json:"cardSizes"`
	// CardScale scales the home cards and their icons, from about 0.7 to 1.6.
	CardScale float64 `json:"cardScale"`
}

// UsageConfig is what the user tells the app about one AI usage provider.
type UsageConfig struct {
	// Key is the service key of a remote provider; it stays in this file.
	Key string `json:"key"`
	// Budget is what the user allows in a month, or what they put in a
	// balance, in the provider's unit; BlockBudget is the same for a
	// five-hour block. 0 is none.
	Budget      float64 `json:"budget"`
	BlockBudget float64 `json:"blockBudget"`
}

// DefaultMCPPort is the port of the MCP server unless the settings say another.
const DefaultMCPPort = 47831

// MCPSettings are the settings of the MCP server. It is off by default, and
// every request needs the key.
type MCPSettings struct {
	Enabled bool `json:"enabled"`
	// Key is the secret an assistant sends as "Authorization: Bearer <key>".
	Key string `json:"key"`
	// Port is the TCP port; 0 is DefaultMCPPort.
	Port int `json:"port"`
	// LAN listens on every network interface, so other computers and a proxy
	// can reach the server. Off, it listens on this computer only.
	LAN bool `json:"lan"`
	// ReadOnly lets assistants read but not change anything.
	ReadOnly bool `json:"readOnly"`
	// PublicURL is the address the server is reached at from outside, copied
	// into the prompt. Optional.
	PublicURL string `json:"publicURL"`
}

// PortOrDefault is the port to listen on.
func (m MCPSettings) PortOrDefault() int {
	if m.Port == 0 {
		return DefaultMCPPort
	}
	return m.Port
}

type file struct {
	Seeded   bool                      `json:"seeded"`
	Scripts  []Script                  `json:"scripts"`
	Metrics  map[string][]engine.Point `json:"metrics"`
	Settings Settings                  `json:"settings"`
	Feeds    []feed.Source             `json:"feeds"`
}

// Store is the app's data. It is safe for concurrent use.
type Store struct {
	path string
	mu   sync.Mutex
	f    file
}

// Open loads the data in dir, creating an empty store when there is none.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{path: filepath.Join(dir, "data.json")}
	b, err := os.ReadFile(s.path)
	switch {
	case err == nil:
		if err := json.Unmarshal(b, &s.f); err != nil {
			return nil, err
		}
	case errors.Is(err, fs.ErrNotExist):
	default:
		return nil, err
	}
	if s.f.Metrics == nil {
		s.f.Metrics = map[string][]engine.Point{}
	}
	if s.f.Settings.WeightUnit == "" {
		s.f.Settings = Settings{WeightUnit: "kg", RemindAt: "20:00"}
	}
	if s.f.Settings.CardScale == 0 {
		s.f.Settings.CardScale = 1
	}
	return s, nil
}

// Path is the file the data lives in.
func (s *Store) Path() string { return s.path }

// Seed fills an empty store with the first scripts and sample values. It does
// nothing once it has run.
func (s *Store) Seed(scripts []Script, metrics map[string][]engine.Point) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f.Seeded {
		return nil
	}
	s.f.Seeded = true
	s.f.Scripts = append([]Script(nil), scripts...)
	for name, pts := range metrics {
		s.f.Metrics[name] = append([]engine.Point(nil), pts...)
	}
	return s.saveLocked()
}

// Scripts returns every script, in order.
func (s *Store) Scripts() []Script {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Script(nil), s.f.Scripts...)
}

// Script returns the script with id.
func (s *Store) Script(id string) (Script, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, sc := range s.f.Scripts {
		if sc.ID == id {
			return sc, true
		}
	}
	return Script{}, false
}

// SaveScript adds sc, or replaces the script with the same ID.
func (s *Store) SaveScript(sc Script) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.f.Scripts {
		if s.f.Scripts[i].ID == sc.ID {
			s.f.Scripts[i] = sc
			return s.saveLocked()
		}
	}
	s.f.Scripts = append(s.f.Scripts, sc)
	return s.saveLocked()
}

// DeleteScript removes the script with id, if there is one.
func (s *Store) DeleteScript(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.f.Scripts[:0]
	for _, sc := range s.f.Scripts {
		if sc.ID != id {
			kept = append(kept, sc)
		}
	}
	s.f.Scripts = kept
	return s.saveLocked()
}

// Feeds returns every data source, in the order they were added.
func (s *Store) Feeds() []feed.Source {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]feed.Source(nil), s.f.Feeds...)
}

// Feed returns the data source with id.
func (s *Store) Feed(id string) (feed.Source, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.f.Feeds {
		if f.ID == id {
			return f, true
		}
	}
	return feed.Source{}, false
}

// SaveFeed adds a data source, or replaces the one with the same ID.
func (s *Store) SaveFeed(src feed.Source) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.f.Feeds {
		if s.f.Feeds[i].ID == src.ID {
			s.f.Feeds[i] = src
			return s.saveLocked()
		}
	}
	s.f.Feeds = append(s.f.Feeds, src)
	return s.saveLocked()
}

// DeleteFeed removes a data source. Its samples are kept in their file.
func (s *Store) DeleteFeed(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.f.Feeds[:0]
	for _, f := range s.f.Feeds {
		if f.ID != id {
			kept = append(kept, f)
		}
	}
	s.f.Feeds = kept
	return s.saveLocked()
}

// Metrics returns a copy of every recorded value, by metric name.
func (s *Store) Metrics() map[string][]engine.Point {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string][]engine.Point, len(s.f.Metrics))
	for name, pts := range s.f.Metrics {
		out[name] = append([]engine.Point(nil), pts...)
	}
	return out
}

// Series returns the values of one metric, oldest first.
func (s *Store) Series(metric string) []engine.Point {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]engine.Point(nil), s.f.Metrics[metric]...)
}

// Append records a value of metric.
func (s *Store) Append(metric string, p engine.Point) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.f.Metrics[metric] = append(s.f.Metrics[metric], p)
	return s.saveLocked()
}

// Settings returns the settings.
func (s *Store) Settings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.f.Settings
}

// SetSettings replaces the settings.
func (s *Store) SetSettings(v Settings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.f.Settings = v
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	b, err := json.MarshalIndent(s.f, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
