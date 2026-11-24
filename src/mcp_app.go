package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/feedback"

	"scriptboard/src/internal/bar"
	"scriptboard/src/internal/mcpsvc"
	"scriptboard/src/internal/store"
)

// The MCP server runs inside the app, so that an assistant's changes reach the
// plugin folder the app is watching. It follows the settings: off, or
// listening on the port, on this computer or on the network, behind a key.

type mcpRunner struct {
	mu   sync.Mutex
	srv  *http.Server
	port string
	err  string
}

// copyText puts text on the clipboard. Tests replace it.
var copyText = func(text string) { mygo.Clipboard.WriteText(text) }

// status is what the settings page says about the server.
func (r *mcpRunner) status(m store.MCPSettings) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch {
	case r.srv != nil:
		return tr("正在监听端口 ") + r.port
	case r.err != "":
		return tr("没有启动：") + r.err
	case !m.Enabled:
		return tr("已关闭")
	}
	return tr("正在启动…")
}

// apply makes the server match the settings: stopped when off, else
// listening.
func (a *app) applyMCP() {
	r := &a.mcp
	r.stop()
	m := a.store.Settings().MCP
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = ""
	if !m.Enabled {
		return
	}
	if m.Key == "" {
		r.err = tr("还没有密钥")
		return
	}
	port := strconv.Itoa(m.PortOrDefault())
	host := "127.0.0.1"
	if m.LAN {
		host = ""
	}
	ln, err := net.Listen("tcp", net.JoinHostPort(host, port))
	if err != nil {
		r.err = err.Error()
		return
	}
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpsvc.Server{
		DataDir:         a.dataDir,
		ReadOnly:        func() bool { return a.store.Settings().MCP.ReadOnly },
		Displays:        a.mcpDisplays,
		MenuBarSnapshot: a.mcpMenuBarSnapshot,
		OnChange:        func() { a.post(a.reloadFromMCP) },
	}.HTTPHandler(m.Key))
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("scriptboard: mcp: %v", err)
		}
	}()
	r.srv, r.port = srv, port
}

func (r *mcpRunner) stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.srv != nil {
		_ = r.srv.Close()
		r.srv = nil
	}
}

// saveMCP stores the MCP settings and makes the server match them.
func (a *app) saveMCP(m store.MCPSettings) {
	st := a.store.Settings()
	st.MCP = m
	if err := a.store.SetSettings(st); err != nil {
		a.mcpNote = err.Error()
		return
	}
	a.set.MCP = m
	a.applyMCP()
}

// connectURL is the address a client should use: the public address when one
// is set, else this computer's address on the network when the server listens
// there, else this computer.
func connectURL(m store.MCPSettings) string {
	if u := strings.TrimSpace(m.PublicURL); u != "" {
		if !strings.HasSuffix(u, "/mcp") {
			u = strings.TrimRight(u, "/") + "/mcp"
		}
		return u
	}
	host := "127.0.0.1"
	if m.LAN {
		if ip := lanAddress(); ip != "" {
			host = ip
		}
	}
	return fmt.Sprintf("http://%s:%d/mcp", host, m.PortOrDefault())
}

func lanAddress() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && ipnet.IP.To4() != nil && !ipnet.IP.IsLoopback() && !ipnet.IP.IsLinkLocalUnicast() {
			return ipnet.IP.String()
		}
	}
	return ""
}

// mcpPrompt is the whole prompt for an assistant: where the server is, the
// key and the tools. It is copied from the settings page.
func mcpPrompt(m store.MCPSettings) string {
	var b strings.Builder
	b.WriteString(tr("你可以通过「盯盯板」（ScriptBoard）的 MCP 服务器，帮我开发本地插件、菜单栏 .bar 组件、数据源和图标。\n\n"))
	fmt.Fprintf(&b, tr("服务器地址：%s\n"), connectURL(m))
	b.WriteString(tr("传输方式：HTTP，每次请求 POST 一条 JSON-RPC 消息\n"))
	fmt.Fprintf(&b, tr("认证：每次请求都带上这个请求头：\n  Authorization: Bearer %s\n\n"), m.Key)
	b.WriteString(tr("工具：\n"))
	for _, t := range mcpsvc.AllTools() {
		fmt.Fprintf(&b, "- %s：%s\n", t.Name, t.Description)
	}
	b.WriteString(tr("\n开发普通脚本卡片请先调 plugin_guide；开发菜单栏悬浮面板 / 带 Cookie 会话与持久化存储的 .bar 组件请先调 bar_guide 与 bar_templates。写好后用 bar_check、bar_run、bar_preview（查看明暗双主题效果图与视觉体检）验证，再保存并配置显示位置。\n"))
	if m.ReadOnly {
		b.WriteString(tr("\n这个服务器是只读的：只有读取类的工具可用。\n"))
	}
	return b.String()
}

// claudeCodeCommand adds the server to Claude Code.
func claudeCodeCommand(m store.MCPSettings) string {
	return fmt.Sprintf("claude mcp add --transport http scriptboard %s --header \"Authorization: Bearer %s\"", connectURL(m), m.Key)
}

// httpConfig is the entry a client that connects to an address takes.
func httpConfig(m store.MCPSettings) string {
	return fmt.Sprintf("{\n  \"mcpServers\": {\n    \"scriptboard\": {\n      \"type\": \"http\",\n      \"url\": %q,\n      \"headers\": { \"Authorization\": %q }\n    }\n  }\n}", connectURL(m), "Bearer "+m.Key)
}

// stdioConfig is the entry a client that starts a program takes: this app,
// run with --mcp-stdio.
func stdioConfig() string {
	exe, err := os.Executable()
	if err != nil {
		exe = "scriptboard"
	}
	return fmt.Sprintf("{\n  \"mcpServers\": {\n    \"scriptboard\": {\n      \"command\": %q,\n      \"args\": [\"--mcp-stdio\"]\n    }\n  }\n}", exe)
}

// maskedKey shows the start and the end of a key, so it can be told apart.
func maskedKey(key string) string {
	if key == "" {
		return tr("还没有密钥")
	}
	if len(key) <= 10 {
		return key
	}
	return key[:4] + "…" + key[len(key)-6:]
}

// mcpGroups are the settings groups of the MCP server.
func (a *app) mcpGroups(c *ui.Context, compact, titled bool) {
	m := a.store.Settings().MCP
	on := m.Enabled
	lan, ro := m.LAN, m.ReadOnly
	if a.mcpPort == "" {
		a.mcpPort = strconv.Itoa(m.PortOrDefault())
		a.mcpPublic = m.PublicURL
	}
	w := float32(240)
	if compact {
		w = 170
	}

	rows := []row{
		row{title: tr("开启 MCP 服务器"), note: tr("让 AI 助手创建和修改本地插件与图标；默认关闭，每次请求都要带密钥。") + a.mcp.status(m), control: func() {
			if toggle(c, &on) {
				m.Enabled = on
				if on && m.Key == "" {
					m.Key = mcpsvc.NewKey()
				}
				a.saveMCP(m)
			}
		}},
	}
	// The rest matters once the server is on.
	if m.Enabled {
		rows = append(rows,
			row{title: tr("端口"), note: tr("改完按回车保存"), control: func() {
				in := textInput(c, &a.mcpPort, "")
				in.Width(96)
				if in.Submitted() {
					port, err := strconv.Atoi(strings.TrimSpace(a.mcpPort))
					if err != nil || port < 1024 || port > 65535 {
						a.mcpNote = tr("端口要在 1024 到 65535 之间")
						return
					}
					m.Port = port
					if port == store.DefaultMCPPort {
						m.Port = 0
					}
					a.mcpNote = ""
					a.saveMCP(m)
				}
			}},
			row{title: tr("允许局域网访问"), note: tr("关闭时只有这台电脑能连；打开后其他电脑和代理也能连，仍然需要密钥"), control: func() {
				if toggle(c, &lan) {
					m.LAN = lan
					a.saveMCP(m)
				}
			}},
			row{title: tr("只读"), note: tr("助手只能读取，不能保存或删除插件和图标"), control: func() {
				if toggle(c, &ro) {
					m.ReadOnly = ro
					a.saveMCP(m)
				}
			}},
			row{title: tr("公开地址"), note: tr("可选，经代理访问时填写，如 https://example.com/mcp，会写进复制的配置；按回车保存"), control: func() {
				in := textInput(c, &a.mcpPublic, "https://…")
				in.Width(w)
				if in.Submitted() {
					m.PublicURL = strings.TrimSpace(a.mcpPublic)
					a.saveMCP(m)
				}
			}},
			row{title: tr("密钥"), note: maskedKey(m.Key), control: func() {
				if btn(c, tr("复制密钥")).Clicked() && m.Key != "" {
					copyText(m.Key)
					a.mcpNote = tr("密钥已复制")
				}
				if btn(c, tr("重新生成")).Clicked() {
					m.Key = mcpsvc.NewKey()
					a.saveMCP(m)
					a.mcpNote = tr("已生成新密钥，旧密钥不能再用")
				}
			}},
			row{stack: true, title: tr("连接助手"), note: tr("复制后粘贴给助手。stdio 配置由本应用以 --mcp-stdio 运行，不需要密钥，直接读写数据目录。"), control: func() {
				copyBtn := func(label string, text func() string) {
					if btn(c, label).Clicked() {
						if m.Key == "" && label != tr("复制 stdio 配置") {
							a.mcpNote = tr("先开启 MCP 服务器，生成密钥")
							return
						}
						copyText(text())
						a.mcpNote = tr("已复制：") + label
					}
				}
				copyBtn(tr("复制提示词"), func() string { return mcpPrompt(m) })
				copyBtn(tr("复制 Claude Code 命令"), func() string { return claudeCodeCommand(m) })
				copyBtn(tr("复制 JSON 配置"), func() string { return httpConfig(m) })
				copyBtn(tr("复制 stdio 配置"), stdioConfig)
			}},
		)
	}
	title := ""
	if titled {
		title = tr("MCP 服务器")
	}
	group(c, title, compact, rows...)
	if a.mcpNote != "" {
		notice(c, a.mcpNote, feedback.Info)
	}
}

func (a *app) mcpDisplays() []mcpsvc.DisplayInfo {
	if a.win == nil {
		return nil
	}
	ds := connectedDisplays()
	out := make([]mcpsvc.DisplayInfo, 0, len(ds))
	for _, d := range ds {
		kind := "external"
		if d.Internal {
			kind = "internal"
		}
		out = append(out, mcpsvc.DisplayInfo{
			ID:       d.ID,
			Label:    d.Label,
			Kind:     kind,
			Internal: d.Internal,
			Primary:  d.Primary,
			Width:    d.Bounds.Width,
			Height:   d.Bounds.Height,
		})
	}
	return out
}

func (a *app) mcpMenuBarSnapshot(id string) (map[string]any, bool) {
	type res struct {
		data map[string]any
		ok   bool
	}
	ch := make(chan res, 1)
	a.post(func() {
		sc, ok := a.store.Script(id)
		if !ok || sc.IsTable() {
			ch <- res{}
			return
		}
		if sc.IsBar() {
			pl := a.barPlacement(id)
			inst, sp, err := a.ensureBarInstance(sc)
			var snap bar.Snapshot
			cookieCount := 0
			var storageKeys []string
			if err == nil && inst != nil {
				snap = inst.Snapshot()
				if inst.Cookies() != nil {
					cookieCount = inst.Cookies().Count()
				}
				if inst.Storage() != nil {
					storageKeys = inst.Storage().Keys()
				}
			} else if err != nil {
				snap.Error = err.Error()
			}
			disp := pl.Display
			if disp == "" {
				disp = "cursor"
			}
			title := sp.Title
			if title == "" {
				title = sc.Name
			}
			ch <- res{
				ok: true,
				data: map[string]any{
					"id":          sc.ID,
					"name":        title,
					"kind":        "bar",
					"enabled":     pl.Enabled,
					"pinned":      pl.Enabled && pl.MenuBar,
					"mode":        pl.MenuBarMode,
					"display":     disp,
					"inputs":      bar.MaskInputs(sp, a.barInputs(sc.ID, sp)),
					"permissions": sp.Permissions,
					"tray":        snap.Tray,
					"panel":       snap.Panel,
					"panelTexts":  collectBarNodeTexts(snap.Panel),
					"logs":        snap.Logs,
					"cookieCount": cookieCount,
					"storageKeys": storageKeys,
					"updatedAt":   snap.UpdatedAt,
					"error":       snap.Error,
				},
			}
			return
		}
		meta, err := a.metaOf(sc)
		if err != nil {
			ch <- res{}
			return
		}
		_, vals := a.viewOf(sc.ID, meta.Dims)
		st := a.ensure(a.runKeyOf(sc.ID, sc, meta, vals), sc, vals)
		pl := a.scriptDockPlacement(sc.ID)
		mode, _ := a.scriptMenuBarKind(sc, st.res)
		disp := pl.Display
		if disp == "" {
			disp = "cursor"
		}
		ch <- res{
			ok: true,
			data: map[string]any{
				"id":      sc.ID,
				"name":    sc.Name,
				"kind":    "script",
				"enabled": pl.Enabled,
				"pinned":  a.menuBarOn(sc.ID),
				"mode":    mode,
				"display": disp,
				"title":   st.res.Title,
				"big":     st.res.Big,
				"sub":     st.res.Sub,
				"icon":    st.res.Icon,
				"tray":    st.res.MenuBar,
				"menu":    st.res.Menu,
				"lines":   st.res.Lines,
			},
		}
	})
	r := <-ch
	return r.data, r.ok
}

func (a *app) reloadFromMCP() {
	if err := a.store.Reload(); err != nil {
		return
	}
	a.set = a.store.Settings()
	a.syncPlugins()

	present := map[string]store.Script{}
	for _, sc := range a.barScripts() {
		present[sc.ID] = sc
	}
	for id, inst := range a.barInsts {
		sc, exists := present[id]
		if !exists {
			inst.Close()
			delete(a.barInsts, id)
			delete(a.barSpecs, id)
			delete(a.barSourceSig, id)
			if f, ok := a.barFlyouts[id]; ok {
				f.Close()
				delete(a.barFlyouts, id)
			}
			if t, ok := a.barTrays[id]; ok {
				t.Destroy()
				delete(a.barTrays, id)
				delete(a.barTraySig, id)
			}
			if a.barOverlayID == id {
				a.barOverlayID = ""
			}
			if a.barSelectedID == id {
				a.barSelectedID = ""
			}
			continue
		}
		if a.barSourceSig == nil || a.barSourceSig[id] != sc.Source {
			inst.Close()
			delete(a.barInsts, id)
			delete(a.barSpecs, id)
			delete(a.barSourceSig, id)
			continue
		}
		if sp, err := a.barSpecOf(sc); err == nil {
			inst.SetOffline(a.set.Offline)
			_ = inst.ReloadState()
			inst.UpdateInputs(a.barInputs(id, sp), curLang())
		}
	}
	a.invalidateAll()
	a.syncMenuBar()
}

func collectBarNodeTexts(n *bar.Node) []string {
	return bar.CollectTexts(n)
}
