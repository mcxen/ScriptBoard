// Package mcpsvc is ScriptBoard's Model Context Protocol server. It lets an
// assistant create and change local plugins and icons, which the app loads
// from the data folder. It speaks JSON-RPC two ways: over stdio (one message
// per line), and over HTTP at /mcp for the server the app runs, guarded by a
// bearer key. Local use needs no login and no review.
package mcpsvc

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"scriptboard/src/internal/iconcat"
	"strings"
	"time"

	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/plugins"
	"scriptboard/src/internal/spec"
)

const protocolVersion = "2024-11-05"

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Tool describes one tool.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// DisplayInfo describes one connected monitor reported by the host app.
type DisplayInfo struct {
	ID       int64  `json:"id"`
	Label    string `json:"label"`
	Kind     string `json:"kind"` // "internal" or "external"
	Internal bool   `json:"internal"`
	Primary  bool   `json:"primary"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
}

// Server is the MCP server over a data folder.
type Server struct {
	DataDir string
	// ReadOnly, when set and true, refuses the tools that change anything:
	// the read tools still work.
	ReadOnly func() bool
	// Displays returns connected monitors (built-in / external) when running
	// inside the desktop app.
	Displays func() []DisplayInfo
	// MenuBarSnapshot optionally provides the live rendered state of a menubar
	// item when running inside the desktop app.
	MenuBarSnapshot func(id string) (map[string]any, bool)
	// OnChange is invoked after write tools update store.json so the live app
	// can reload settings and sync trays/views immediately.
	OnChange func()
}

func (s Server) readOnly() bool { return s.ReadOnly != nil && s.ReadOnly() }

func (s Server) notifyChange() {
	if s.OnChange != nil {
		s.OnChange()
	}
}

// Serve reads JSON-RPC messages from in, one per line, and writes the replies
// to out. The protocol uses stdout only; diagnostics go to stderr.
func (s Server) Serve(in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if reply := s.Handle([]byte(line)); reply != nil {
			if _, err := out.Write(append(reply, '\n')); err != nil {
				return err
			}
		}
	}
	return sc.Err()
}

// Handle answers one JSON-RPC message. It returns nil for a notification,
// which gets no reply.
func (s Server) Handle(msg []byte) []byte {
	var req request
	if err := json.Unmarshal(msg, &req); err != nil {
		return mustJSON(response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{-32700, "bad JSON"}})
	}
	result, rerr := s.handle(req)
	if len(req.ID) == 0 {
		return nil
	}
	resp := response{JSONRPC: "2.0", ID: req.ID}
	if rerr != nil {
		resp.Error = rerr
	} else {
		resp.Result = result
	}
	return mustJSON(resp)
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// HTTPHandler serves the same protocol at one endpoint over HTTP: a POST of a
// JSON-RPC message is answered with one JSON object. Every request needs
// "Authorization: Bearer <key>"; the key is the guard, so the origin is not
// checked and a proxy in front may use any name.
func (s Server) HTTPHandler(key string) http.Handler {
	want := []byte("Bearer " + key)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if key == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="scriptboard"`)
			http.Error(w, "unauthorized: send Authorization: Bearer <key>", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			http.Error(w, "POST a JSON-RPC message", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		reply := s.Handle(body)
		if reply == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(reply)
	})
}

// NewKey makes a key for the HTTP server: "sbk-" and 40 hex digits.
func NewKey() string {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("sbk-%040x", time.Now().UnixNano())
	}
	return "sbk-" + hex.EncodeToString(raw)
}

func (s Server) handle(req request) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		return map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "scriptboard", "version": "0.1.10"},
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": append(Tools(), extraTools()...)}, nil
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, &rpcError{-32602, "bad params"}
		}
		text, err := s.call(p.Name, p.Arguments)
		if err != nil {
			return map[string]any{
				"content": []map[string]any{{"type": "text", "text": err.Error()}},
				"isError": true,
			}, nil
		}
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": text}},
		}, nil
	}
	if strings.HasPrefix(req.Method, "notifications/") {
		return nil, nil
	}
	return nil, &rpcError{-32601, fmt.Sprintf("method %q is not supported", req.Method)}
}

// Tools lists the tools the server offers.
func Tools() []Tool {
	str := func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	obj := func(props map[string]any, required ...string) map[string]any {
		return map[string]any{"type": "object", "properties": props, "required": required}
	}
	return []Tool{
		{
			Name:        "plugin_list",
			Description: "List the local plugins in the ScriptBoard data folder, and any that are broken.",
			InputSchema: obj(map[string]any{}),
		},
		{
			Name:        "plugin_get",
			Description: "Read one plugin: its manifest and script source.",
			InputSchema: obj(map[string]any{"id": str("plugin id")}, "id"),
		},
		{
			Name: "plugin_save",
			Description: "Create or update a local plugin. The script must be an ES module that exports " +
				"meta() and render(ctx); it is compiled before anything is written. The app picks the " +
				"change up within a second. No login or review is needed for local plugins.",
			InputSchema: obj(map[string]any{
				"id":          str("plugin id: lowercase letters, digits and hyphens"),
				"name":        str("name shown in the app"),
				"icon":        str("icon name, such as coins (optional)"),
				"description": str("one line about the plugin (optional)"),
				"home":        map[string]any{"type": "boolean", "description": "show as a home card"},
				"position":    map[string]any{"type": "integer", "description": "where the home card sits: 1 is first; leave it out to put it last"},
				"size":        str("default size of the home card: mini, small, medium, large or wide; leave it out for medium"),
				"menubar":     map[string]any{"type": "boolean", "description": "show the card in the menu bar (notification area) when the plugin first appears; the script must say menubar: true in meta()"},
				"source":      str("the script source"),
			}, "id", "name", "source"),
		},
		{
			Name:        "plugin_delete",
			Description: "Delete a local plugin and its icons.",
			InputSchema: obj(map[string]any{"id": str("plugin id")}, "id"),
		},
		{
			Name: "plugin_guide",
			Description: "Read this first. The plugin reference: the script shape, meta() fields, the platforms " +
				"(desktop, mobile, or one system such as windows) and needs (network, files, exec, key), which " +
				"features the user must allow, what the host provides as ctx, and how system differences are " +
				"handled by the host instead of the script.",
			InputSchema: obj(map[string]any{}),
		},
		{
			Name: "script_check",
			Description: "Compile a script without saving it and report its meta (name, icon, dims) and its " +
				"contract: the platforms it runs on and the features it needs (declared plus what the host sees " +
				"it use), with warnings when what it declares does not match what it uses.",
			InputSchema: obj(map[string]any{"source": str("the script source")}, "source"),
		},
		{
			Name: "icon_add",
			Description: "Save an SVG icon. Give plugin to put it in that plugin's icons; leave it out " +
				"for a shared icon. Scripts use it by name. Scripts, event attributes and external links " +
				"in the SVG are refused; the SVG must be at most 32 KB.",
			InputSchema: obj(map[string]any{
				"name":   str("icon name: lowercase letters, digits and hyphens"),
				"svg":    str("the SVG markup"),
				"plugin": str("plugin id, optional"),
			}, "name", "svg"),
		},
		{
			Name:        "icon_list",
			Description: "List the icons scripts can use: the shared ones and each plugin's.",
			InputSchema: obj(map[string]any{}),
		},
		{
			Name: "icon_catalog",
			Description: "The shipped icons, by category, with what each shows in Chinese and English. " +
				"Pick a script's meta().icon from this list: it is the one shown on its card. " +
				"Give category to list one category only.",
			InputSchema: obj(map[string]any{
				"category": str("category key, such as weather or finance; optional"),
			}),
		},
	}
}

// call runs one tool and returns its text reply.
func (s Server) call(name string, raw json.RawMessage) (string, error) {
	dataDir := s.DataDir
	switch name {
	case "plugin_save", "plugin_delete", "icon_add", "table_save", "table_append", "tray_set", "bar_save":
		if s.readOnly() {
			return "", fmt.Errorf("MCP 服务器现在是只读的，不能执行 %s", name)
		}
	}
	var a map[string]any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &a); err != nil {
			return "", fmt.Errorf("arguments are not an object: %w", err)
		}
	}
	str := func(key string) string {
		v, _ := a[key].(string)
		return v
	}
	switch name {
	case "plugin_list":
		list, err := plugins.List(dataDir)
		var b strings.Builder
		for _, p := range list {
			fmt.Fprintf(&b, "%s  %s  icon=%s  home=%v", p.Manifest.ID, p.Manifest.Name, p.Manifest.Icon, p.Manifest.Home)
			if p.Manifest.Position > 0 {
				fmt.Fprintf(&b, "  position=%d", p.Manifest.Position)
			}
			if p.Manifest.Size != "" {
				fmt.Fprintf(&b, "  size=%s", p.Manifest.Size)
			}
			if p.Manifest.Menubar {
				b.WriteString("  menubar=true")
			}
			if meta, err := plugins.CheckSource(p.Manifest.ID+".js", p.Source); err == nil {
				r := spec.Resolve(meta)
				fmt.Fprintf(&b, "  platforms=%s  needs=%s", strings.Join(r.Platforms, ","), strings.Join(r.Needs, ","))
				if len(r.Problems) > 0 {
					fmt.Fprintf(&b, "  warnings=%d", len(r.Problems))
				}
			}
			b.WriteString("\n")
		}
		if b.Len() == 0 {
			b.WriteString("没有插件\n")
		}
		if err != nil {
			fmt.Fprintf(&b, "有问题的插件：%v\n", err)
		}
		return b.String(), nil
	case "plugin_get":
		list, err := plugins.List(dataDir)
		if err != nil {
			return "", err
		}
		for _, p := range list {
			if p.Manifest.ID == str("id") {
				manifest, _ := json.MarshalIndent(p.Manifest, "", "  ")
				return string(manifest) + "\n\n" + p.Source, nil
			}
		}
		return "", fmt.Errorf("没有插件 %q", str("id"))
	case "plugin_save":
		home, _ := a["home"].(bool)
		position, _ := a["position"].(float64)
		menubar, _ := a["menubar"].(bool)
		m := plugins.Manifest{
			ID:          str("id"),
			Name:        str("name"),
			Icon:        str("icon"),
			Description: str("description"),
			Home:        home,
			Position:    int(position),
			Size:        str("size"),
			Menubar:     menubar,
		}
		if err := plugins.Save(dataDir, m, str("source")); err != nil {
			return "", err
		}
		s.notifyChange()
		out := fmt.Sprintf("已保存插件 %s，在 %s", m.ID, plugins.PluginDir(dataDir))
		if meta, err := plugins.CheckSource(m.ID+".js", str("source")); err == nil {
			out += "\n" + contractReport(meta)
		}
		return out, nil
	case "plugin_delete":
		if err := plugins.Delete(dataDir, str("id")); err != nil {
			return "", err
		}
		s.notifyChange()
		return "已删除插件 " + str("id"), nil
	case "plugin_guide":
		return Guide(), nil
	case "script_check":
		meta, err := plugins.CheckSource("check.js", str("source"))
		if err != nil {
			return "", err
		}
		out, _ := json.MarshalIndent(meta, "", "  ")
		return "脚本可以运行\n" + string(out) + "\n" + contractReport(meta) + sourceReport(meta) + iconReport(meta, dataDir) + chartReport(str("source")) + menubarReport(meta, str("source")), nil
	case "icon_catalog":
		return catalogText(str("category")), nil
	case "icon_add":
		path, err := plugins.AddIcon(dataDir, str("plugin"), str("name"), []byte(str("svg")))
		if err != nil {
			return "", err
		}
		s.notifyChange()
		return "已保存图标 " + str("name") + "，在 " + path, nil
	case "icon_list":
		names := plugins.ListIcons(dataDir)
		if len(names) == 0 {
			return "没有自定义图标（内置图标见应用的 icons 目录）\n", nil
		}
		return strings.Join(names, "\n") + "\n", nil
	}
	if out, handled, err := s.callExtra(name, a); handled {
		return out, err
	}
	return "", fmt.Errorf("没有这个工具：%s", name)
}

// contractReport says where a script runs and what it needs, after the host
// has added what it sees the script use to what the script declares, and lists
// what the author should fix. A meta() declares them as platforms ("desktop",
// "mobile") and needs ("network", "files", "exec", "key"), with a version.
func contractReport(meta engine.Meta) string {
	r := spec.Resolve(meta)
	var b strings.Builder
	fmt.Fprintf(&b, "平台：%s\n依赖：%s\n", strings.Join(r.Platforms, "、"), strings.Join(r.Needs, "、"))
	if !r.Declared {
		b.WriteString("提示：meta() 里没有声明 platforms 和 needs，宿主按脚本实际用到的来推断；规范的脚本应该声明它们和 version。\n")
	}
	for _, p := range r.Problems {
		fmt.Fprintf(&b, "警告：%s\n", p)
	}
	return b.String()
}

// catalogText lists the shipped icons, all categories or one.
func catalogText(category string) string {
	var b strings.Builder
	for _, g := range iconcat.Categories {
		if category != "" && g.Key != category {
			continue
		}
		fmt.Fprintf(&b, "%s · %s (%s)\n", g.Key, g.Zh, g.En)
		for _, it := range g.Items {
			fmt.Fprintf(&b, "  %s  %s / %s\n", it.Name, it.Zh, it.En)
		}
	}
	if b.Len() == 0 {
		return "没有这个分类；分类有：" + categoryKeys() + "\n"
	}
	return b.String()
}

func categoryKeys() string {
	keys := make([]string, 0, len(iconcat.Categories))
	for _, g := range iconcat.Categories {
		keys = append(keys, g.Key)
	}
	return strings.Join(keys, ", ")
}

// iconReport warns when a script's icon is neither in the catalog nor among the
// icons the user or a plugin added, so that it would show the default picture.
func iconReport(meta engine.Meta, dataDir string) string {
	if meta.Icon == "" || iconcat.Has(meta.Icon) {
		return ""
	}
	for _, n := range plugins.ListIcons(dataDir) {
		if strings.HasPrefix(n, meta.Icon+" ") || n == meta.Icon || strings.HasSuffix(n, " ("+meta.Icon+")") {
			return ""
		}
	}
	return "警告：图标 " + meta.Icon + " 不存在，会显示默认图标。用 icon_catalog 查看可选的图标。\n"
}

// chartReport runs the script once, with nothing to read, and warns when the
// chart it draws has a type the app does not know: such a chart shows nothing.
func chartReport(source string) string {
	s, err := engine.CompileLang("check.js", source, "zh")
	if err != nil {
		return ""
	}
	res, err := s.Run(context.Background(), engine.Input{Now: time.Now(), Lang: "zh"})
	if err != nil || res.Chart == nil || !engine.UnknownChartType(res.Chart.Type) {
		return ""
	}
	return "警告：图表类型 " + res.Chart.Type + " 不认识，卡片不会画出图。可用的类型：" + strings.Join(engine.ChartTypes, ", ") + "\n"
}

// menubarReport runs the script once, with nothing to read, and warns about what
// it gives the menu bar that the app cannot use: a menu on a card that did not
// ask for the bar, a link that is not a web address, an action that does not
// exist, a title that will be cut.
func menubarReport(meta engine.Meta, source string) string {
	s, err := engine.CompileLang("check.js", source, "zh")
	if err != nil {
		return ""
	}
	res, err := s.Run(context.Background(), engine.Input{Now: time.Now(), Lang: "zh"})
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, p := range engine.MenuBarProblems(meta, res) {
		b.WriteString("警告：" + p + "\n")
	}
	return b.String()
}

// sourceReport checks the script's sources and their fallbacks for addresses
// the host cannot fetch.
func sourceReport(meta engine.Meta) string {
	var b strings.Builder
	checkURL := func(label, u string) {
		u = strings.TrimSpace(u)
		if u == "" {
			fmt.Fprintf(&b, "警告：数据源 %s 的地址为空。\n", label)
			return
		}
		if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
			fmt.Fprintf(&b, "警告：数据源 %s 的地址 %q 要以 http:// 或 https:// 开头。\n", label, u)
		}
	}
	names := make([]string, 0, len(meta.Sources))
	for k := range meta.Sources {
		names = append(names, k)
	}
	for _, name := range names {
		src := meta.Sources[name]
		checkURL(name, src.URL)
		for i, fb := range src.Fallbacks {
			checkURL(fmt.Sprintf("%s 的备用请求 %d", name, i+1), fb.URL)
		}
	}
	return b.String()
}
