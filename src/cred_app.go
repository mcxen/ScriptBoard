package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/bar"
	"scriptboard/src/internal/cred"
)

type credForm struct {
	open    bool
	editing string
	confirm string
	note    string

	id           string
	name         string
	typeIdx      int
	origin       string
	providerIdx  int
	loginURL     string
	readyURL     string
	readyCookies string
	refreshOn    string
	headerName   string
	headerPrefix string
	cdpURL       string

	importID     string
	importFmtIdx int
	importText   string
	importUser   string
	importPass   string
	importHName  string
	importHPref  string
}

var (
	credTypeKeys    = []string{cred.TypeCookie, cred.TypeBearer, cred.TypeToken, cred.TypeBasic, cred.TypeHeader}
	credTypeLabels  = []string{"Cookie", "Bearer", "Token", "Basic Auth", "Header"}
	credProvKeys    = []string{cred.ProviderWebView, cred.ProviderBrowser, cred.ProviderManual, cred.ProviderEgo, cred.ProviderCDP}
	credProvLabels  = []string{"内置登录窗", "系统浏览器", "手动导入", "Ego 会话", "CDP 调试口"}
	credImportKeys  = []string{cred.ImportAuto, cred.ImportCookie, cred.ImportNetscape, cred.ImportCurl, cred.ImportBearer, cred.ImportToken, cred.ImportBasic}
	credImportNames = []string{"自动识别", "Cookie 字符串", "Netscape cookies.txt", "curl 命令", "Bearer Token", "Token / Header", "Basic Auth"}
)

type mygoKeychainBackend struct {
	mod mygo.SecretsModule
}

func (b mygoKeychainBackend) Name() string { return "keychain" }

func (b mygoKeychainBackend) Get(key string) ([]byte, error) {
	v, err := b.mod.Get("scriptboard.cred." + key)
	if errors.Is(err, mygo.ErrSecretNotFound) {
		return nil, cred.ErrSecretNotFound
	}
	return v, err
}

func (b mygoKeychainBackend) Set(key string, val []byte) error {
	return b.mod.Set("scriptboard.cred."+key, val)
}

func (b mygoKeychainBackend) Delete(key string) error {
	return b.mod.Delete("scriptboard.cred." + key)
}

var credSecretBackendForApp = func() cred.SecretBackend {
	if testing.Testing() {
		return cred.NewMemoryBackend()
	}
	return mygoKeychainBackend{mod: mygo.Secrets}
}

func (a *app) initCredentials() {
	if a.credBrowserURL == nil {
		a.credBrowserURL = map[string]string{}
	}
	handlers := a.credLoginHandlers()
	mgr, err := cred.NewManager(a.dataDir, credSecretBackendForApp(), handlers)
	if err != nil {
		// Fall back cleanly to encrypted local file vault if backend failed.
		mgr, _ = cred.NewManager(a.dataDir, cred.NewFileKeyBackend(a.dataDir), handlers)
	}
	a.credMgr = mgr
}

func (a *app) credLoginHandlers() cred.LoginHandlers {
	return cred.LoginHandlers{
		WebView: a.openWebViewLogin,
		Browser: a.openBrowserLogin,
		OnChange: func(id string) {
			a.onCredentialUpdated(id)
		},
	}
}

func (a *app) syncDeclaredCredentials() {
	if a.credMgr == nil {
		return
	}
	for _, sc := range a.store.Scripts() {
		if sc.IsTable() {
			continue
		}
		if sc.IsBar() {
			if sp, err := bar.Parse(sc.Source); err == nil && len(sp.Credentials) > 0 {
				a.credMgr.RegisterSpecs(sc.ID, sp.Credentials)
			}
			continue
		}
		if meta, err := a.metaOf(sc); err == nil && len(meta.Credentials) > 0 {
			a.credMgr.RegisterSpecs(sc.ID, meta.Credentials)
		}
	}
}

func (a *app) onCredentialUpdated(credID string) {
	if credID == "" {
		return
	}
	a.post(func() {
		for _, sc := range a.store.Scripts() {
			if sc.IsTable() {
				continue
			}
			if sc.IsBar() {
				sp, err := a.barSpecOf(sc)
				if err != nil {
					continue
				}
				matched := false
				for _, cs := range sp.Credentials {
					if cs.ID == credID {
						matched = true
						break
					}
				}
				if matched {
					if inst, ok := a.barInsts[sc.ID]; ok && inst != nil {
						inst.SyncCredentials()
						inst.Refresh()
					}
				}
				continue
			}
			meta, err := a.metaOf(sc)
			if err != nil {
				continue
			}
			for _, cs := range meta.Credentials {
				if cs.ID == credID {
					a.forced[sc.ID] = true
					a.invalidate(sc.ID)
					break
				}
			}
		}
		a.syncMenuBar()
		a.syncBarMenuBar()
	})
}

func credDisplayName(sp cred.Spec) string {
	if strings.TrimSpace(sp.Name) != "" {
		return sp.Name
	}
	return sp.ID
}

// openWebViewLogin opens a restricted ephemeral WebView window so the user can
// complete SSO/site authentication. It strictly observes ONLY URL, title, and
// load state, extracts only cookies matching sp.Origin/AllowedDomains, and never
// logs or injects input listeners into page forms.
func (a *app) openWebViewLogin(ctx context.Context, sp cred.Spec) (cred.SecretPayload, error) {
	if a.credLoginHook != nil {
		return a.credLoginHook(ctx, sp)
	}
	if a.win == nil {
		return cred.SecretPayload{}, errors.New(tr("内置 WebView 登录窗口仅在桌面交互界面可用，请改用手动导入或 CDP 同步"))
	}
	sp = sp.Normalized()
	loginURL := sp.EffectiveLoginURL()
	lu, err := url.Parse(loginURL)
	if err != nil || (strings.ToLower(lu.Scheme) != "http" && strings.ToLower(lu.Scheme) != "https") {
		return cred.SecretPayload{}, errors.New(tr("无效的登录地址（仅支持 http/https）"))
	}

	sess := mygo.NewEphemeralSession()
	defer func() {
		_ = sess.ClearData(mygo.SiteDataAll)
	}()

	type outcome struct {
		payload cred.SecretPayload
		err     error
	}
	resCh := make(chan outcome, 1)
	var once sync.Once
	finish := func(p cred.SecretPayload, err error) {
		once.Do(func() {
			resCh <- outcome{payload: p, err: err}
		})
	}

	var mu sync.Mutex
	curURL := loginURL
	curTitle := ""
	var w *mygo.Window

	collectOriginCookies := func() []cred.StoredCookie {
		var all []cred.StoredCookie
		seen := map[string]bool{}
		addURL := func(u string) {
			list, err := sess.Cookies.List(mygo.CookieFilter{URL: u})
			if err != nil {
				return
			}
			for _, c := range list {
				var exp int64
				if !c.Expires.IsZero() {
					exp = c.Expires.Unix()
				}
				sc := cred.StoredCookie{
					Name:     c.Name,
					Value:    c.Value,
					Domain:   c.Domain,
					Path:     c.Path,
					Expires:  exp,
					Secure:   c.Secure,
					HTTPOnly: c.HTTPOnly,
					SameSite: fmt.Sprint(c.SameSite),
				}
				filtered := sp.FilterAuthorizedCookies([]cred.StoredCookie{sc})
				if len(filtered) == 0 {
					continue
				}
				fc := filtered[0]
				key := fc.Domain + "\x00" + fc.Path + "\x00" + fc.Name
				if !seen[key] {
					seen[key] = true
					all = append(all, fc)
				}
			}
		}
		addURL(sp.Origin)
		if sp.LoginURL != "" && sp.MatchesURL(sp.LoginURL) {
			addURL(sp.LoginURL)
		}
		for _, dom := range sp.AllowedDomains {
			addURL("https://" + strings.TrimPrefix(dom, "."))
		}
		return all
	}

	checkReady := func() {
		mu.Lock()
		u, ttl := curURL, curTitle
		win := w
		mu.Unlock()

		cookies := collectOriginCookies()
		ready := false
		if sp.Ready != nil {
			ready = sp.Ready.Satisfied(u, ttl, cookies)
		} else {
			ready = sp.MatchesURL(u) && len(cookies) > 0
		}
		if !ready {
			return
		}
		finish(cred.SecretPayload{
			Type:      cred.TypeCookie,
			Cookies:   cookies,
			Provider:  cred.ProviderWebView,
			UpdatedAt: time.Now().UnixMilli(),
		}, nil)
		if win != nil {
			a.post(func() { win.Close() })
		}
	}

	created := make(chan struct{})
	a.post(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:  tr("登录凭据 · ") + credDisplayName(sp),
			URL:    loginURL,
			Width:  980,
			Height: 720,
			Hidden: true,
			Page: mygo.PageOptions{
				Session:         sess,
				Untrusted:       true,
				UserAgentSuffix: "Version/17.0 Safari/605.1.15",
			},
		})
		mu.Lock()
		w = win
		mu.Unlock()

		if pg := win.Page(); pg != nil {
			pg.OnWillNavigate(func(e *mygo.NavigateEvent) {
				nu, nerr := url.Parse(strings.TrimSpace(e.URL))
				if nerr != nil || (strings.ToLower(nu.Scheme) != "http" && strings.ToLower(nu.Scheme) != "https") {
					e.PreventDefault()
					return
				}
				mu.Lock()
				curURL = e.URL
				mu.Unlock()
			})
			pg.SetWindowOpenHandler(func(req mygo.WindowOpenRequest) *mygo.WindowOptions {
				ru, rerr := url.Parse(strings.TrimSpace(req.URL))
				if rerr == nil && (strings.ToLower(ru.Scheme) == "http" || strings.ToLower(ru.Scheme) == "https") {
					_ = pg.LoadURL(req.URL)
				}
				return nil
			})
			pg.OnDidNavigate(func(u string) {
				mu.Lock()
				curURL = u
				mu.Unlock()
				checkReady()
			})
			pg.OnPageTitleUpdated(func(e *mygo.TitleEvent) {
				mu.Lock()
				curTitle = e.Title
				mu.Unlock()
				checkReady()
			})
			pg.OnDidFinishLoad(func() {
				checkReady()
			})
		}
		win.OnClose(func(_ *mygo.CloseEvent) {
			finish(cred.SecretPayload{}, errors.New(tr("用户取消了登录窗口")))
		})
		win.Show()
		win.Focus()
		close(created)
	})
	<-created

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case out := <-resCh:
			return out.payload, out.err
		case <-ticker.C:
			checkReady()
		case <-ctx.Done():
			mu.Lock()
			win := w
			mu.Unlock()
			if win != nil {
				a.post(func() { win.Close() })
			}
			return cred.SecretPayload{}, ctx.Err()
		}
	}
}

func (a *app) openBrowserLogin(ctx context.Context, sp cred.Spec) (cred.SecretPayload, error) {
	if a.credLoginHook != nil {
		return a.credLoginHook(ctx, sp)
	}
	cb, err := cred.StartCallbackServer(ctx, sp)
	if err != nil {
		return cred.SecretPayload{}, err
	}
	defer func() {
		a.post(func() {
			delete(a.credBrowserURL, sp.ID)
		})
		_ = cb.Close()
	}()

	a.post(func() {
		if a.credBrowserURL == nil {
			a.credBrowserURL = map[string]string{}
		}
		a.credBrowserURL[sp.ID] = cb.HandoffURL
	})

	if a.win != nil {
		_ = mygo.Shell.OpenExternal(cb.LaunchURL)
	}
	return cb.Wait(ctx)
}

func (a *app) acquireCredentialAsync(sp cred.Spec, providerOverride string) {
	if a.credMgr == nil {
		return
	}
	refKey := "cred:" + sp.ID
	if a.isRefreshing(refKey) {
		return
	}
	if providerOverride != "" {
		sp.Provider = providerOverride
	}
	a.beginRefresh(refKey, 1)
	run := func() {
		_, err := a.credMgr.Acquire(context.Background(), sp, 0, true)
		a.post(func() {
			if err != nil {
				a.credForm.note = credDisplayName(sp) + "：" + err.Error()
				a.finishRefresh(refKey, false, err.Error())
				return
			}
			a.credForm.note = fmt.Sprintf(tr("凭据 %s 已更新并加密保存"), credDisplayName(sp))
			a.finishRefresh(refKey, true, "")
		})
	}
	if a.win == nil {
		run()
		return
	}
	go run()
}

func (a *app) saveCredForm() {
	if a.credMgr == nil {
		return
	}
	f := &a.credForm
	name := strings.TrimSpace(f.name)
	id := strings.TrimSpace(f.id)
	if f.editing != "" {
		id = f.editing
	} else if id == "" {
		id = slugOf(name)
	}
	sp := cred.Spec{
		ID:           id,
		Name:         name,
		Type:         credTypeKeys[clampIndex(f.typeIdx, len(credTypeKeys))],
		Origin:       strings.TrimSpace(f.origin),
		Provider:     credProvKeys[clampIndex(f.providerIdx, len(credProvKeys))],
		LoginURL:     strings.TrimSpace(f.loginURL),
		HeaderName:   strings.TrimSpace(f.headerName),
		HeaderPrefix: f.headerPrefix,
		CDPURL:       strings.TrimSpace(f.cdpURL),
	}
	if strings.TrimSpace(f.readyURL) != "" || strings.TrimSpace(f.readyCookies) != "" {
		rc := &cred.ReadyCondition{URLIncludes: strings.TrimSpace(f.readyURL)}
		for _, ck := range strings.Split(f.readyCookies, ",") {
			if ck = strings.TrimSpace(ck); ck != "" {
				rc.HasCookies = append(rc.HasCookies, ck)
			}
		}
		sp.Ready = rc
	}
	if strings.TrimSpace(f.refreshOn) != "" {
		for _, part := range strings.Split(f.refreshOn, ",") {
			if n, err := strconv.Atoi(strings.TrimSpace(part)); err == nil && n >= 400 && n <= 599 {
				sp.RefreshOn = append(sp.RefreshOn, n)
			}
		}
	}
	rec, err := a.credMgr.SaveSpec(sp)
	if err != nil {
		f.note = err.Error()
		return
	}
	f.open = false
	f.editing = ""
	f.note = tr("已保存凭据 ") + rec.Spec.ID
}

func (a *app) editCredRecord(rec cred.Record) {
	f := &a.credForm
	f.open = true
	f.editing = rec.Spec.ID
	f.confirm = ""
	f.id = rec.Spec.ID
	f.name = rec.Spec.Name
	f.origin = rec.Spec.Origin
	f.loginURL = rec.Spec.LoginURL
	f.headerName = rec.Spec.HeaderName
	f.headerPrefix = rec.Spec.HeaderPrefix
	f.cdpURL = rec.Spec.CDPURL
	f.typeIdx = slices.Index(credTypeKeys, rec.Spec.Type)
	if f.typeIdx < 0 {
		f.typeIdx = 0
	}
	f.providerIdx = slices.Index(credProvKeys, rec.Spec.Provider)
	if f.providerIdx < 0 {
		f.providerIdx = 0
	}
	if rec.Spec.Ready != nil {
		f.readyURL = rec.Spec.Ready.URLIncludes
		f.readyCookies = strings.Join(rec.Spec.Ready.HasCookies, ", ")
	} else {
		f.readyURL = ""
		f.readyCookies = ""
	}
	var ro []string
	for _, code := range rec.Spec.RefreshOn {
		ro = append(ro, strconv.Itoa(code))
	}
	f.refreshOn = strings.Join(ro, ",")
}

func clampIndex(idx, n int) int {
	if idx < 0 || idx >= n {
		return 0
	}
	return idx
}

func (a *app) credStatusBadge(p palette, st string) (string, ui.Color) {
	switch st {
	case cred.StatusReady:
		return tr("已就绪"), p.down
	case cred.StatusAuthenticating:
		return tr("登录中…"), p.accent
	case cred.StatusExpired:
		return tr("已过期"), p.warn
	case cred.StatusError:
		return tr("需重新登录"), p.up
	default:
		return tr("未配置"), p.muted
	}
}

func (a *app) credProvLabel(prov string) string {
	if i := slices.Index(credProvKeys, prov); i >= 0 {
		return tr(credProvLabels[i])
	}
	return prov
}

func (a *app) credentialsPage(c *ui.Context, compact bool) {
	p := pal(c)
	page(c, compact, func() {
		subheader(c, a, tr("凭据管理"), tr("统一管理插件与菜单栏组件的登录态、Token 与 Cookie；敏感凭据使用 AES-256-GCM 加密并通过系统钥匙串保存"), func() {
			if !a.credForm.open {
				if primaryBtn(c, tr("+ 新建凭据")).Clicked() {
					a.credForm.open = true
					a.credForm.editing = ""
					a.credForm.id = ""
					a.credForm.name = ""
					a.credForm.origin = ""
					a.credForm.loginURL = ""
					a.credForm.readyURL = ""
					a.credForm.readyCookies = ""
					a.credForm.refreshOn = "401,403"
					a.credForm.typeIdx = 0
					a.credForm.providerIdx = 0
					a.credForm.headerName = ""
					a.credForm.headerPrefix = ""
					a.credForm.cdpURL = ""
				}
			}
		})

		hint(c, tr("仅提取并注入目标源站（Origin）授权凭据，内置登录窗仅检测页面 URL 与目标 Cookie 写入状态，不记录表单输入。"))

		var recs []cred.Record
		backendName := "keychain"
		if a.credMgr != nil {
			recs = a.credMgr.List()
			backendName = a.credMgr.BackendName()
		}
		backendLabel := tr("系统钥匙串（Keychain + AES-256-GCM）")
		if backendName == "file-encrypted" {
			backendLabel = tr("本机加密库（AES-256-GCM）")
		} else if backendName == "memory" {
			backendLabel = tr("内存加密会话")
		}

		ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
			chip(c, backendLabel, p.accent, p.accentSoft)
			if len(recs) > 0 {
				ready := 0
				for _, r := range recs {
					if r.Status == cred.StatusReady && r.HasSecret {
						ready++
					}
				}
				chip(c, fmt.Sprintf(tr("已就绪 %d / 共 %d"), ready, len(recs)), p.down, p.down.Alpha(0.12))
			}
		})

		if a.credForm.note != "" {
			chip(c, a.credForm.note, p.accent, p.accentSoft)
		}

		if len(recs) == 0 && !a.credForm.open {
			card(c).Children(func() {
				empty(c, "log-in", tr("还没有凭据"), tr("脚本在 meta().credentials 或 .bar 在 <credentials> 中声明后会自动出现在这里，也可点击右上角手动新建。"))
			})
		}

		for _, rec := range recs {
			a.credCard(c, p, rec, compact)
		}

		if a.credForm.open {
			a.credEditFormCard(c, compact)
		}
	})
}

func (a *app) credCard(c *ui.Context, p palette, rec cred.Record, compact bool) {
	sp := rec.Spec
	refKey := "cred:" + sp.ID
	busy := a.isRefreshing(refKey)
	stText, stColor := a.credStatusBadge(p, rec.Status)

	card(c.Key("cred-card:" + sp.ID)).Padding(16).Gap(10).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(10).Wrap().Children(func() {
			iconBadge(c, "log-in", stColor, 34)
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(6).Wrap().Children(func() {
					ui.Text(c, credDisplayName(sp)).Bold()
					chip(c, strings.ToUpper(sp.Type), p.muted, p.chip)
					chip(c, a.credProvLabel(sp.Provider), p.accent, p.accentSoft)
					chip(c, stText, stColor, stColor.Alpha(0.13))
				})
				metaLine := "id " + sp.ID + " · " + sp.Origin
				if sp.SourcePlugin != "" {
					metaLine += " · " + sp.SourcePlugin
				}
				ui.Text(c, metaLine).Font("monospace").FontSize(fsTiny).TextColor(p.muted)
			})
			a.refreshIconButton(c, "cred-spin:"+sp.ID, refKey, tr("登录状态"), 26, 14, true, p.hover, p.muted, p.accent, p.down, p.up, nil)
		})

		if rec.HasSecret {
			summary := ""
			if rec.CookieCount > 0 {
				summary = fmt.Sprintf(tr("已保存 %d 条授权 Cookie（%s）"), rec.CookieCount, strings.Join(rec.CookieNames, ", "))
			} else if rec.TokenPreview != "" {
				summary = tr("已保存凭据摘要：") + rec.TokenPreview
			}
			if rec.UpdatedAt > 0 {
				summary += " · " + fmtTime(rec.UpdatedAt)
			}
			ui.Text(c, summary).FontSize(fsSmall).TextColor(p.text)
		} else {
			ui.Text(c, tr("尚未导入或登录获取凭据；遇到 401/403 时将自动拉起预设认证方式。")).FontSize(fsTiny).TextColor(p.muted)
		}

		if rec.LastError != "" {
			ui.Text(c, rec.LastError).FontSize(fsTiny).TextColor(p.up)
		}

		if handoffURL := a.credBrowserURL[sp.ID]; handoffURL != "" {
			ui.Row(c).AlignItems(ui.Center).Gap(8).Wrap().Children(func() {
				chip(c, tr("等待系统浏览器完成授权或粘贴凭据回传"), p.accent, p.accentSoft)
				if btn(c, tr("打开本地回传页")).Clicked() && a.win != nil {
					_ = mygo.Shell.OpenExternal(handoffURL)
				}
			})
		}

		ui.Row(c).Gap(8).Wrap().AlignItems(ui.Center).Children(func() {
			if !isMobile() {
				wvLbl := tr("内置登录")
				if busy {
					wvLbl = tr("登录中…")
				}
				if primaryBtn(c, wvLbl).Clicked() && !busy {
					a.acquireCredentialAsync(sp, cred.ProviderWebView)
				}
				if btn(c, tr("浏览器授权")).Clicked() && !busy {
					a.acquireCredentialAsync(sp, cred.ProviderBrowser)
				}
				if btn(c, tr("Ego/CDP 同步")).Clicked() && !busy {
					a.acquireCredentialAsync(sp, cred.ProviderEgo)
				}
			}

			impLbl := tr("手动导入")
			if a.credForm.importID == sp.ID {
				impLbl = tr("收起导入")
			}
			if btn(c, impLbl).Clicked() {
				if a.credForm.importID == sp.ID {
					a.credForm.importID = ""
				} else {
					a.credForm.importID = sp.ID
					a.credForm.importFmtIdx = 0
					a.credForm.importText = ""
					a.credForm.importUser = ""
					a.credForm.importPass = ""
					a.credForm.importHName = sp.HeaderName
					a.credForm.importHPref = sp.HeaderPrefix
				}
			}
			if btn(c, tr("编辑")).Clicked() {
				a.editCredRecord(rec)
			}
			if rec.HasSecret {
				if btn(c, tr("清空")).Clicked() && a.credMgr != nil {
					if err := a.credMgr.ClearSecret(sp.ID); err != nil {
						a.credForm.note = err.Error()
					} else {
						a.credForm.note = tr("已清空凭据 ") + sp.ID
					}
				}
			}
			if sp.SourcePlugin == "" {
				delLbl := tr("删除")
				if a.credForm.confirm == sp.ID {
					delLbl = tr("确认删除")
				}
				if btn(c, delLbl).Clicked() && a.credMgr != nil {
					if a.credForm.confirm == sp.ID {
						_ = a.credMgr.Delete(sp.ID)
						a.credForm.confirm = ""
						a.credForm.note = tr("已删除凭据 ") + sp.ID
					} else {
						a.credForm.confirm = sp.ID
					}
				}
			}
		})

		if a.credForm.importID == sp.ID {
			a.credImportPanel(c, p, sp, compact)
		}
	})
}

func (a *app) credImportPanel(c *ui.Context, p palette, sp cred.Spec, compact bool) {
	f := &a.credForm
	w := float32(420)
	if compact {
		w = 290
	}
	fmtLabels := make([]string, len(credImportNames))
	for i, lbl := range credImportNames {
		fmtLabels[i] = tr(lbl)
	}
	formatKey := credImportKeys[clampIndex(f.importFmtIdx, len(credImportKeys))]

	ui.Column(c).FillWidth().Padding(12).Radius(10).Background(p.chip).Gap(10).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(8).Wrap().Children(func() {
			ui.Text(c, tr("导入格式")).FontSize(fsTiny).TextColor(p.muted)
			dropdown(c, &f.importFmtIdx, fmtLabels...)
		})

		if formatKey == cred.ImportBasic {
			ui.Row(c).Gap(8).Wrap().Children(func() {
				textInput(c, &f.importUser, tr("用户名")).Width(180)
				passwordInput(c, &f.importPass, tr("密码")).Width(180)
			})
		} else {
			if formatKey == cred.ImportToken {
				ui.Row(c).Gap(8).Wrap().Children(func() {
					textInput(c, &f.importHName, tr("请求头名（默认 Authorization）")).Width(200)
					textInput(c, &f.importHPref, tr("前缀（如 Bearer ）")).Width(160)
				})
			}
			ph := tr("粘贴 Cookie 字符串、curl 命令、Netscape cookies.txt 或 Token 值")
			textArea(c.Key("cred-imp-ta:"+sp.ID), &f.importText, ph, 4).Font("monospace").FontSize(fsSmall).Width(w)
		}

		ui.Row(c).Gap(8).Children(func() {
			if primaryBtn(c, tr("保存并加密")).Clicked() && a.credMgr != nil {
				_, err := a.credMgr.ImportManual(sp.ID, formatKey, f.importText, f.importUser, f.importPass, f.importHName, f.importHPref)
				if err != nil {
					f.note = err.Error()
				} else {
					f.importID = ""
					f.importText = ""
					f.importUser = ""
					f.importPass = ""
					f.note = fmt.Sprintf(tr("凭据 %s 已导入并加密保存"), credDisplayName(sp))
				}
			}
			if btn(c, tr("取消")).Clicked() {
				f.importID = ""
			}
		})
	})
}

func (a *app) credEditFormCard(c *ui.Context, compact bool) {
	f := &a.credForm
	w := float32(400)
	if compact {
		w = 290
	}
	title := tr("新建凭据")
	if f.editing != "" {
		title = tr("编辑凭据")
	}
	provLabels := make([]string, len(credProvLabels))
	for i, lbl := range credProvLabels {
		provLabels[i] = tr(lbl)
	}

	sectionTitle(c, title)
	card(c).Padding(18).Gap(12).Children(func() {
		field(c, tr("名称"), "", func() {
			textInput(c, &f.name, tr("例如 Token Hub")).Width(w)
		})
		if f.editing == "" {
			field(c, "id", tr("留空则自动生成；小写字母、数字和连字符"), func() {
				textInput(c, &f.id, "token-hub").Width(w)
			})
		} else {
			field(c, "id", "", func() {
				ui.Text(c, f.id).Font("monospace").TextColor(pal(c).muted)
			})
		}
		field(c, tr("凭据类型"), "", func() {
			segmented(c, &f.typeIdx, credTypeLabels...)
		})
		field(c, tr("授权源站 Origin"), tr("仅该源站（及授权子域）会读取或注入此凭据"), func() {
			textInput(c, &f.origin, "https://token-hub.alibaba-inc.com").Width(w).
				Keyboard(ui.KeyboardURL).Capitalize(ui.CapitalizeNone).Autocorrect(false)
		})
		field(c, tr("认证提供器"), "", func() {
			dropdown(c, &f.providerIdx, provLabels...)
		})
		field(c, tr("登录地址 Login URL（可选）"), tr("留空时使用 Origin；支持 {callbackUrl} 与 {state}"), func() {
			textInput(c, &f.loginURL, "https://token-hub.alibaba-inc.com/bridge/usage/credits").Width(w).
				Keyboard(ui.KeyboardURL).Capitalize(ui.CapitalizeNone).Autocorrect(false)
		})
		field(c, tr("完成判定 URL 包含（可选）"), "", func() {
			textInput(c, &f.readyURL, "/bridge/").Width(w)
		})
		field(c, tr("完成判定必需 Cookie 名（可选，逗号分隔）"), "", func() {
			textInput(c, &f.readyCookies, "token-portal-bridge_SSO_TOKEN_V3").Width(w)
		})
		field(c, tr("触发自动刷新的 HTTP 状态码"), "", func() {
			textInput(c, &f.refreshOn, "401,403").Width(160)
		})
		field(c, tr("CDP 调试地址（可选）"), tr("默认 http://127.0.0.1:9222"), func() {
			textInput(c, &f.cdpURL, "http://127.0.0.1:9222").Width(w)
		})
		ui.Row(c).Gap(8).Children(func() {
			if primaryBtn(c, tr("保存")).Clicked() {
				a.saveCredForm()
			}
			if btn(c, tr("取消")).Clicked() {
				f.open = false
				f.editing = ""
			}
		})
	})
}
