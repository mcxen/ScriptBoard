package bar

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/MoeForge/moejs"
	"github.com/MoeForge/moejs/engine"
)

// Snapshot is the immutable UI + tray tree produced by a render(env) call.
type Snapshot struct {
	Tray      *TraySpec `json:"tray,omitempty"`
	Panel     *Node     `json:"panel,omitempty"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
	Error     string    `json:"error,omitempty"`
	Logs      []string  `json:"logs,omitempty"`
}

// TraySpec describes the menu bar / tray icon, title, tooltip and menu.
type TraySpec struct {
	Title        string         `json:"title,omitempty"`
	Icon         string         `json:"icon,omitempty"`
	IconTemplate bool           `json:"iconTemplate,omitempty"`
	Tooltip      string         `json:"tooltip,omitempty"`
	Error        string         `json:"error,omitempty"`
	Menu         []MenuItemSpec `json:"menu,omitempty"`
}

// MenuItemSpec describes one tray menu item or separator.
type MenuItemSpec struct {
	Label     string `json:"label,omitempty"`
	ActionID  string `json:"actionID,omitempty"`
	Separator bool   `json:"separator,omitempty"`
	Disabled  bool   `json:"disabled,omitempty"`
}

// Node is one virtual DOM node produced by h(type, props, ...children).
type Node struct {
	Type      string            `json:"type"`
	Props     map[string]any    `json:"props,omitempty"`
	Text      string            `json:"text,omitempty"`
	Children  []*Node           `json:"children,omitempty"`
	OnClickID string            `json:"onClickID,omitempty"`
	Shortcuts map[string]string `json:"shortcuts,omitempty"`
}

type actionCollector struct {
	seq     int
	actions map[string]*moejs.Object
}

func (ac *actionCollector) register(v any) (string, bool) {
	obj, ok := v.(*moejs.Object)
	if !ok || obj == nil {
		return "", false
	}
	if !engine.IsCallable(engine.ObjectValue(obj)) {
		return "", false
	}
	ac.seq++
	id := fmt.Sprintf("act-%d", ac.seq)
	ac.actions[id] = obj
	return id, true
}

func extractSnapshot(rt *moejs.Runtime, val moejs.Value) (Snapshot, map[string]*moejs.Object, error) {
	if val.IsUndefined() || val.IsNull() {
		return Snapshot{}, map[string]*moejs.Object{}, nil
	}
	raw, err := rt.ToGo(val)
	if err != nil {
		return Snapshot{}, nil, err
	}
	rootMap, ok := raw.(map[string]any)
	if !ok {
		return Snapshot{}, map[string]*moejs.Object{}, nil
	}

	ac := &actionCollector{actions: map[string]*moejs.Object{}}
	var snap Snapshot

	if trayRaw, ok := rootMap["tray"]; ok && trayRaw != nil {
		if tm, ok := trayRaw.(map[string]any); ok {
			snap.Tray = parseTray(tm, ac)
		}
	}
	if panelRaw, ok := rootMap["panel"]; ok && panelRaw != nil {
		snap.Panel = parseNode(panelRaw, ac)
	} else if _, hasType := rootMap["type"]; hasType {
		// Allow components that directly return a root Node without {tray, panel}.
		snap.Panel = parseNode(rootMap, ac)
	}
	return snap, ac.actions, nil
}

func parseTray(m map[string]any, ac *actionCollector) *TraySpec {
	t := &TraySpec{
		Title:        asString(m["title"]),
		Icon:         asString(m["icon"]),
		IconTemplate: asBool(m["iconTemplate"], true),
		Tooltip:      asString(m["tooltip"]),
		Error:        asString(m["error"]),
	}
	if menuRaw, ok := m["menu"].([]any); ok {
		for _, item := range menuRaw {
			switch v := item.(type) {
			case string:
				if strings.TrimSpace(v) == "-" || strings.EqualFold(v, "separator") {
					t.Menu = append(t.Menu, MenuItemSpec{Separator: true})
				} else if v != "" {
					t.Menu = append(t.Menu, MenuItemSpec{Label: v})
				}
			case map[string]any:
				if asBool(v["separator"], false) || strings.EqualFold(asString(v["type"]), "separator") {
					t.Menu = append(t.Menu, MenuItemSpec{Separator: true})
					continue
				}
				mi := MenuItemSpec{
					Label:    asString(v["label"]),
					Disabled: asBool(v["disabled"], false),
				}
				if aid, ok := ac.register(v["onClick"]); ok {
					mi.ActionID = aid
				}
				if mi.Label != "" || mi.ActionID != "" {
					t.Menu = append(t.Menu, mi)
				}
			}
		}
	}
	return t
}

func parseNode(raw any, ac *actionCollector) *Node {
	switch v := raw.(type) {
	case nil, bool:
		return nil
	case string:
		return &Node{Type: "Text", Text: v}
	case int64:
		return &Node{Type: "Text", Text: strconv.FormatInt(v, 10)}
	case float64:
		return &Node{Type: "Text", Text: formatNum(v)}
	case map[string]any:
		typ := asString(v["type"])
		if typ == "" {
			return nil
		}
		n := &Node{Type: typ}
		if pm, ok := v["props"].(map[string]any); ok && len(pm) > 0 {
			props := make(map[string]any, len(pm))
			for k, pv := range pm {
				switch k {
				case "onClick":
					if aid, ok := ac.register(pv); ok {
						n.OnClickID = aid
					}
				case "shortcuts":
					if sm, ok := pv.(map[string]any); ok {
						for sk, sv := range sm {
							if aid, ok := ac.register(sv); ok {
								if n.Shortcuts == nil {
									n.Shortcuts = map[string]string{}
								}
								n.Shortcuts[sk] = aid
							}
						}
					}
				default:
					if clean := sanitizeProp(pv); clean != nil {
						props[k] = clean
					}
				}
			}
			if len(props) > 0 {
				n.Props = props
			}
		}
		var textParts []string
		collectChildren(v["children"], ac, n, &textParts)
		if len(textParts) > 0 && (n.Type == "Text" || n.Type == "Button" || n.Type == "Badge") {
			n.Text = strings.Join(textParts, "")
		} else if len(textParts) > 0 {
			for _, tp := range textParts {
				n.Children = append(n.Children, &Node{Type: "Text", Text: tp})
			}
		}
		return n
	default:
		return nil
	}
}

func collectChildren(raw any, ac *actionCollector, parent *Node, textParts *[]string) {
	switch c := raw.(type) {
	case nil, bool:
		return
	case []any:
		for _, item := range c {
			collectChildren(item, ac, parent, textParts)
		}
	case string:
		if parent.Type == "Text" || parent.Type == "Button" || parent.Type == "Badge" {
			*textParts = append(*textParts, c)
		} else {
			parent.Children = append(parent.Children, &Node{Type: "Text", Text: c})
		}
	case int64:
		s := strconv.FormatInt(c, 10)
		if parent.Type == "Text" || parent.Type == "Button" || parent.Type == "Badge" {
			*textParts = append(*textParts, s)
		} else {
			parent.Children = append(parent.Children, &Node{Type: "Text", Text: s})
		}
	case float64:
		s := formatNum(c)
		if parent.Type == "Text" || parent.Type == "Button" || parent.Type == "Badge" {
			*textParts = append(*textParts, s)
		} else {
			parent.Children = append(parent.Children, &Node{Type: "Text", Text: s})
		}
	case map[string]any:
		if child := parseNode(c, ac); child != nil {
			parent.Children = append(parent.Children, child)
		}
	}
}

func sanitizeProp(v any) any {
	switch x := v.(type) {
	case nil, bool, string, int64, float64:
		return x
	case int:
		return int64(x)
	case *moejs.Object:
		return nil
	case []any:
		out := make([]any, 0, len(x))
		for _, item := range x {
			if c := sanitizeProp(item); c != nil {
				out = append(out, c)
			}
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, mv := range x {
			if c := sanitizeProp(mv); c != nil {
				out[k] = c
			}
		}
		return out
	default:
		return nil
	}
}

func asString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case int64:
		return strconv.FormatInt(x, 10)
	case int:
		return strconv.Itoa(x)
	case float64:
		return formatNum(x)
	default:
		return ""
	}
}

func asBool(v any, fallback bool) bool {
	b, ok := v.(bool)
	if !ok {
		return fallback
	}
	return b
}

func formatNum(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// ActionInfo describes a clickable panel node or tray menu item collected from
// a rendered Snapshot.
type ActionInfo struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Text    string `json:"text,omitempty"`
	Tooltip string `json:"tooltip,omitempty"`
}

// CollectActions returns all clickable actions present in snap's tray menu and
// panel VDOM tree.
func CollectActions(snap Snapshot) []ActionInfo {
	var out []ActionInfo
	if snap.Tray != nil {
		for _, m := range snap.Tray.Menu {
			if m.ActionID != "" {
				out = append(out, ActionInfo{
					ID:   m.ActionID,
					Type: "MenuItem",
					Text: m.Label,
				})
			}
		}
	}
	var walk func(n *Node)
	walk = func(n *Node) {
		if n == nil {
			return
		}
		if n.OnClickID != "" {
			tip, _ := n.Props["tooltip"].(string)
			out = append(out, ActionInfo{
				ID:      n.OnClickID,
				Type:    n.Type,
				Text:    n.Text,
				Tooltip: tip,
			})
		}
		for _, ch := range n.Children {
			walk(ch)
		}
	}
	walk(snap.Panel)
	return out
}
