// Package bar parses and runs Metrix.Bar-compatible .bar component bundles
// on moejs + eventloop + webapi.
package bar

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Spec is the parsed manifest and script of a .bar component.
type Spec struct {
	ProtocolVersion string      `json:"protocolVersion,omitempty"`
	Version         string      `json:"version,omitempty"`
	ID              string      `json:"id"`
	Title           string      `json:"title"`
	Category        string      `json:"category,omitempty"`
	Icon            string      `json:"icon,omitempty"`
	Description     string      `json:"description,omitempty"`
	License         string      `json:"license,omitempty"`
	Homepage        string      `json:"homepage,omitempty"`
	Refresh         string      `json:"refresh,omitempty"`
	Material        string      `json:"material,omitempty"`
	Permissions     []string    `json:"permissions,omitempty"`
	CookieDomains   []string    `json:"cookieDomains,omitempty"`
	StoragePerm     bool        `json:"storage,omitempty"`
	ClipboardPerm   bool        `json:"clipboard,omitempty"`
	Inputs          []InputSpec `json:"inputs,omitempty"`
	Script          string      `json:"-"`
	ScriptAPIs      []string    `json:"scriptAPIs,omitempty"`
}

// InputSpec describes one configurable input parameter declared in <inputs>.
type InputSpec struct {
	Name     string        `json:"name"`
	Type     string        `json:"type"` // string, select, number, boolean, password
	Label    string        `json:"label"`
	Default  string        `json:"default,omitempty"`
	Help     string        `json:"help,omitempty"`
	Required bool          `json:"required,omitempty"`
	Options  []InputOption `json:"options,omitempty"`
}

// InputOption is one option inside a select <input>.
type InputOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type xmlBar struct {
	XMLName         xml.Name        `xml:"bar"`
	ProtocolVersion string          `xml:"protocol-version,attr"`
	Version         string          `xml:"version,attr"`
	ID              string          `xml:"id,attr"`
	Title           string          `xml:"title,attr"`
	Category        string          `xml:"category,attr"`
	Icon            string          `json:"icon,attr" xml:"icon,attr"`
	Description     string          `xml:"description,attr"`
	License         string          `xml:"license,attr"`
	Homepage        string          `xml:"homepage,attr"`
	Refresh         string          `xml:"refresh,attr"`
	Material        string          `xml:"material,attr"`
	Permissions     *xmlPermissions `xml:"permissions"`
	Inputs          *xmlInputs      `xml:"inputs"`
	Script          xmlScript       `xml:"script"`
}

type xmlPermissions struct {
	Networks  []xmlNetwork    `xml:"network"`
	Cookies   []xmlCookiePerm `xml:"cookies"`
	Storage   *struct{}       `xml:"storage"`
	Clipboard *struct{}       `xml:"clipboard"`
}

type xmlNetwork struct {
	Origin string `xml:"origin,attr"`
}

type xmlCookiePerm struct {
	Domain string `xml:"domain,attr"`
}

type xmlInputs struct {
	Inputs []xmlInput `xml:"input"`
}

type xmlInput struct {
	Name     string      `xml:"name,attr"`
	Type     string      `xml:"type,attr"`
	Label    string      `xml:"label,attr"`
	Default  string      `xml:"default,attr"`
	Help     string      `xml:"help,attr"`
	Required string      `xml:"required,attr"`
	Options  []xmlOption `xml:"option"`
}

type xmlOption struct {
	Value string `xml:"value,attr"`
	Label string `xml:"label,attr"`
}

type xmlScript struct {
	APIs string `xml:"apis,attr"`
	Body string `xml:",chardata"`
}

// IsBarSource reports whether source looks like a .bar XML component document.
func IsBarSource(source string) bool {
	s := strings.TrimSpace(source)
	return strings.HasPrefix(s, "<bar") && strings.Contains(s, "</bar>")
}

// Parse decodes a .bar XML document into a Spec.
func Parse(source string) (Spec, error) {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return Spec{}, errors.New("empty .bar source")
	}
	var doc xmlBar
	if err := xml.Unmarshal([]byte(trimmed), &doc); err != nil {
		return Spec{}, fmt.Errorf("invalid .bar xml: %w", err)
	}
	if strings.TrimSpace(doc.ID) == "" {
		return Spec{}, errors.New(".bar missing id attribute")
	}
	script := strings.TrimSpace(doc.Script.Body)
	if script == "" {
		return Spec{}, errors.New(".bar missing <script> body")
	}
	title := strings.TrimSpace(doc.Title)
	if title == "" {
		title = doc.ID
	}
	spec := Spec{
		ProtocolVersion: doc.ProtocolVersion,
		Version:         doc.Version,
		ID:              strings.TrimSpace(doc.ID),
		Title:           title,
		Category:        strings.TrimSpace(doc.Category),
		Icon:            strings.TrimSpace(doc.Icon),
		Description:     strings.TrimSpace(doc.Description),
		License:         strings.TrimSpace(doc.License),
		Homepage:        strings.TrimSpace(doc.Homepage),
		Refresh:         strings.TrimSpace(doc.Refresh),
		Material:        strings.TrimSpace(doc.Material),
		Script:          script,
	}
	if doc.Script.APIs != "" {
		for _, api := range strings.Split(doc.Script.APIs, ",") {
			if a := strings.TrimSpace(api); a != "" {
				spec.ScriptAPIs = append(spec.ScriptAPIs, a)
			}
		}
	}
	if doc.Permissions != nil {
		for _, n := range doc.Permissions.Networks {
			if orig := strings.TrimSpace(n.Origin); orig != "" {
				spec.Permissions = append(spec.Permissions, orig)
			}
		}
		for _, ck := range doc.Permissions.Cookies {
			if dom := strings.TrimSpace(ck.Domain); dom != "" {
				spec.CookieDomains = append(spec.CookieDomains, dom)
			}
		}
		spec.StoragePerm = doc.Permissions.Storage != nil
		spec.ClipboardPerm = doc.Permissions.Clipboard != nil
	}
	if doc.Inputs != nil {
		for _, in := range doc.Inputs.Inputs {
			name := strings.TrimSpace(in.Name)
			if name == "" {
				continue
			}
			typ := strings.ToLower(strings.TrimSpace(in.Type))
			if typ == "" {
				typ = "string"
			}
			lbl := strings.TrimSpace(in.Label)
			if lbl == "" {
				lbl = name
			}
			req := strings.EqualFold(strings.TrimSpace(in.Required), "true") || strings.TrimSpace(in.Required) == "1"
			item := InputSpec{
				Name:     name,
				Type:     typ,
				Label:    lbl,
				Default:  in.Default,
				Help:     strings.TrimSpace(in.Help),
				Required: req,
			}
			for _, opt := range in.Options {
				optLbl := opt.Label
				if optLbl == "" {
					optLbl = opt.Value
				}
				item.Options = append(item.Options, InputOption{
					Value: opt.Value,
					Label: optLbl,
				})
			}
			spec.Inputs = append(spec.Inputs, item)
		}
	}
	return spec, nil
}

// RefreshDuration parses spec.Refresh (e.g. "15m", "30s", "1h"), defaulting to 15m
// and clamping to at least 10s.
func RefreshDuration(spec Spec) time.Duration {
	const fallback = 15 * time.Minute
	const minRefresh = 10 * time.Second
	raw := strings.TrimSpace(spec.Refresh)
	if raw == "" {
		return fallback
	}
	if d, err := time.ParseDuration(raw); err == nil {
		if d < minRefresh {
			return minRefresh
		}
		return d
	}
	if secs, err := strconv.Atoi(raw); err == nil && secs > 0 {
		d := time.Duration(secs) * time.Second
		if d < minRefresh {
			return minRefresh
		}
		return d
	}
	return fallback
}

// MergeInputs returns a fresh input map populated with spec defaults for any
// missing keys.
func MergeInputs(spec Spec, values map[string]string) map[string]string {
	out := make(map[string]string, len(spec.Inputs)+len(values))
	for _, in := range spec.Inputs {
		out[in.Name] = in.Default
	}
	for k, v := range values {
		out[k] = v
	}
	return out
}

// MaskInputs returns a copy of vals with non-empty password input values
// replaced by "[REDACTED]".
func MaskInputs(spec Spec, vals map[string]string) map[string]string {
	if len(vals) == 0 {
		return nil
	}
	isSecret := make(map[string]bool, len(spec.Inputs))
	for _, in := range spec.Inputs {
		if in.Type == "password" {
			isSecret[in.Name] = true
		}
	}
	out := make(map[string]string, len(vals))
	for k, v := range vals {
		if isSecret[k] && v != "" {
			out[k] = "[REDACTED]"
		} else {
			out[k] = v
		}
	}
	return out
}

// OriginAllowed enforces that rawURL matches one of the allowed network
// origins by normalized scheme, lowercase hostname (or *.subdomain wildcard),
// and effective port.
func OriginAllowed(rawURL string, allowed []string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("scheme not allowed: %s", u.Scheme)
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return errors.New("missing host")
	}
	port := effectivePort(scheme, u.Port())

	for _, orig := range allowed {
		au, err := url.Parse(orig)
		if err != nil {
			continue
		}
		aScheme := strings.ToLower(au.Scheme)
		if aScheme != scheme || effectivePort(aScheme, au.Port()) != port {
			continue
		}
		aHost := strings.ToLower(au.Hostname())
		if strings.HasPrefix(aHost, "*.") {
			suffix := aHost[1:]
			if len(suffix) > 1 && strings.HasSuffix(host, suffix) && len(host) > len(suffix) {
				return nil
			}
			continue
		}
		if aHost == host {
			return nil
		}
	}
	return fmt.Errorf("network origin %s://%s not permitted", scheme, u.Host)
}

func effectivePort(scheme, port string) string {
	if port != "" {
		return port
	}
	switch scheme {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func safeComponentID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
		sum := sha256.Sum256([]byte(id))
		return "bad-" + hex.EncodeToString(sum[:8])
	}
	return id
}

// ComponentDataDir returns the per-.bar persistent data folder under rootDataDir.
func ComponentDataDir(rootDataDir, id string) string {
	if rootDataDir == "" {
		return ""
	}
	return filepath.Join(rootDataDir, "bars", safeComponentID(id))
}

// CookiesPath returns the persisted CookieJar JSON file path for component id.
func CookiesPath(rootDataDir, id string) string {
	dir := ComponentDataDir(rootDataDir, id)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "cookies.json")
}

// StoragePath returns the persisted KVStore JSON file path for component id.
func StoragePath(rootDataDir, id string) string {
	dir := ComponentDataDir(rootDataDir, id)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "storage.json")
}
