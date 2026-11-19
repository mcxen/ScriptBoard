package usage

import "scriptboard/src/internal/caps"

// Capable is implemented by a provider that depends on the platform. One that
// does not is taken to need the network, and a key when it says so.
type Capable interface {
	// Platforms are where the provider can read from: nil is everywhere.
	Platforms() []string
	// Needs are the platform features it uses.
	Needs() []string
}

// CapsOf is where the provider id can run and what it needs. An unknown id has
// no limits and no needs.
func CapsOf(id string) (platforms, needs []string) {
	p, ok := Get(id)
	if !ok {
		return nil, nil
	}
	if c, ok := p.(Capable); ok {
		return c.Platforms(), c.Needs()
	}
	needs = []string{caps.Network}
	if p.NeedsKey() {
		needs = append(needs, caps.Key)
	}
	return nil, needs
}

// Claude Code's logs are on the computer where it ran: this provider reads
// them, so it works on a computer only.
func (claudeCode) Platforms() []string { return []string{caps.Desktop} }
func (claudeCode) Needs() []string     { return []string{caps.Files} }
