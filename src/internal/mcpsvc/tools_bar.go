package mcpsvc

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"scriptboard/src/internal/bar"
	"scriptboard/src/internal/store"
)

func barTools() []Tool {
	str := func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	boolProp := func(desc string) map[string]any { return map[string]any{"type": "boolean", "description": desc} }
	intProp := func(desc string) map[string]any { return map[string]any{"type": "integer", "description": desc} }
	obj := func(props map[string]any, required ...string) map[string]any {
		return map[string]any{"type": "object", "properties": props, "required": required}
	}
	return []Tool{
		{
			Name: "bar_guide",
			Description: "Read this before developing a .bar menubar component. Documents the XML schema, " +
				"permissions (including wildcard domains), inputs, ES module lifecycle, persistent CookieJar, " +
				"$metrix.storage / $metrix.cookies / localStorage APIs, VDOM components, and MCP test workflow.",
			InputSchema: obj(map[string]any{}),
		},
		{
			Name:        "bar_list",
			Description: "List all .bar menubar components with metadata, permissions, input schemas, placements, cookie counts, and storage key counts.",
			InputSchema: obj(map[string]any{}),
		},
		{
			Name:        "bar_get",
			Description: "Read one .bar component's XML source, parsed spec, placement, input values, cookie count, and storage keys.",
			InputSchema: obj(map[string]any{
				"id":             str(".bar component id"),
				"includeSecrets": boolProp("include password input values unmasked (default false)"),
			}, "id"),
		},
		{
			Name:        "bar_delete",
			Description: "Delete a user-created .bar component along with its persisted cookies, storage, and placement.",
			InputSchema: obj(map[string]any{
				"id": str(".bar component id"),
			}, "id"),
		},
		{
			Name:        "bar_check",
			Description: "Validate .bar XML, permissions, inputs, compile its ES module, and verify initial render(env) without saving.",
			InputSchema: obj(map[string]any{
				"source": str("the .bar XML document"),
				"lang":   str("zh or en (default zh)"),
			}, "source"),
		},
		{
			Name: "bar_run",
			Description: "Execute a .bar component (by source or saved id) in an isolated sandbox with optional " +
				"mockFetch (supporting Set-Cookie), seed cookies/curl, storage, inputs, allowNetwork, and actionID click testing. " +
				"Never mutates on-disk cookies or storage.",
			InputSchema: obj(map[string]any{
				"source":        str(".bar XML source (optional if id is provided)"),
				"id":            str("saved .bar component id (optional if source is provided)"),
				"lang":          str("zh or en (default zh)"),
				"inputs":        map[string]any{"type": "object", "description": "optional input values map"},
				"cookies":       map[string]any{"type": "array", "items": map[string]any{"type": "object"}, "description": "optional seed cookies [{name, value, domain, path, secure, httpOnly, expires}]"},
				"cookieHeader":  str("optional Cookie header string (requires cookieURL)"),
				"cookieURL":     str("target URL for cookieHeader"),
				"curl":          str("optional curl command to extract seed cookies from"),
				"useSavedState": boolProp("clone saved cookies and storage of id into sandbox memory without mutating disk"),
				"storage":       map[string]any{"type": "object", "description": "optional initial KV storage values"},
				"mockFetch":     map[string]any{"type": "object", "description": "optional map of URL/prefix/'*' -> {status, headers, body, setCookie}"},
				"allowNetwork":  boolProp("allow real network calls for unmocked URLs (default false)"),
				"waitScheduled": boolProp("wait for async scheduled(env) to settle (default true)"),
				"timeoutMs":     intProp("execution timeout in ms (500..15000, default 5000)"),
				"actionID":      str("optional panel/menu action ID to click after initial render"),
			}),
		},
		{
			Name:        "bar_cookies_get",
			Description: "Inspect persisted cookies for a .bar component (values masked by default).",
			InputSchema: obj(map[string]any{
				"id":            str(".bar component id"),
				"domain":        str("optional domain filter"),
				"includeValues": boolProp("return raw cookie values instead of masked values (default false)"),
			}, "id"),
		},
		{
			Name:        "bar_cookies_set",
			Description: "Import or set persisted cookies for a .bar component from a curl command, a Cookie header + url, or structured cookie objects.",
			InputSchema: obj(map[string]any{
				"id":           str(".bar component id"),
				"curl":         str("curl command containing -b or -H 'Cookie: ...'"),
				"cookieHeader": str("Cookie header string (e.g. 'a=1; b=2')"),
				"url":          str("target http/https URL when using cookieHeader"),
				"cookies":      map[string]any{"type": "array", "items": map[string]any{"type": "object"}, "description": "structured cookies [{name, value, domain, path, secure, httpOnly, expires}]"},
			}, "id"),
		},
		{
			Name:        "bar_cookies_clear",
			Description: "Clear persisted cookies for a .bar component (all cookies, or only those matching domain).",
			InputSchema: obj(map[string]any{
				"id":     str(".bar component id"),
				"domain": str("optional domain filter; omit to clear all"),
			}, "id"),
		},
		{
			Name:        "bar_storage_get",
			Description: "Read persisted KV storage for a .bar component (one key or all keys).",
			InputSchema: obj(map[string]any{
				"id":  str(".bar component id"),
				"key": str("optional key; omit to return all entries"),
			}, "id"),
		},
		{
			Name:        "bar_storage_set",
			Description: "Write one key or a map of values into a .bar component's persisted KV storage.",
			InputSchema: obj(map[string]any{
				"id":     str(".bar component id"),
				"key":    str("key name (with value)"),
				"value":  str("string value (with key)"),
				"values": map[string]any{"type": "object", "description": "map of key -> value pairs"},
			}, "id"),
		},
		{
			Name:        "bar_storage_clear",
			Description: "Delete one key or clear all persisted KV storage for a .bar component.",
			InputSchema: obj(map[string]any{
				"id":  str(".bar component id"),
				"key": str("optional key to delete; omit to clear all"),
			}, "id"),
		},
	}
}

func (s Server) callBar(name string, a map[string]any) (string, bool, error) {
	str := func(key string) string {
		v, _ := a[key].(string)
		return v
	}
	switch name {
	case "bar_guide":
		return BarGuide(), true, nil

	case "bar_list":
		st, err := store.Open(s.DataDir)
		if err != nil {
			return "", true, err
		}
		set := st.Settings()
		type item struct {
			ID              string             `json:"id"`
			Title           string             `json:"title"`
			Version         string             `json:"version,omitempty"`
			Category        string             `json:"category,omitempty"`
			Icon            string             `json:"icon,omitempty"`
			Description     string             `json:"description,omitempty"`
			Origin          string             `json:"origin,omitempty"`
			Home            bool               `json:"home"`
			Refresh         string             `json:"refresh,omitempty"`
			Permissions     []string           `json:"permissions,omitempty"`
			Inputs          []bar.InputSpec    `json:"inputs,omitempty"`
			Placement       store.BarPlacement `json:"placement"`
			CookieCount     int                `json:"cookieCount"`
			StorageKeyCount int                `json:"storageKeyCount"`
		}
		var out []item
		for _, sc := range st.Scripts() {
			if !sc.IsBar() {
				continue
			}
			sp, _ := bar.Parse(sc.Source)
			pl := mcpPlacementFor(sc.ID, true, set)
			jar, _ := bar.NewCookieJar(bar.CookiesPath(s.DataDir, sc.ID))
			kv, _ := bar.NewKVStore(bar.StoragePath(s.DataDir, sc.ID))
			cc, sk := 0, 0
			if jar != nil {
				cc = jar.Count()
			}
			if kv != nil {
				sk = kv.Count()
			}
			out = append(out, item{
				ID:              sc.ID,
				Title:           orStr(sp.Title, sc.Name),
				Version:         sp.Version,
				Category:        sp.Category,
				Icon:            sp.Icon,
				Description:     sp.Description,
				Origin:          sc.Origin,
				Home:            sc.Home,
				Refresh:         sp.Refresh,
				Permissions:     sp.Permissions,
				Inputs:          sp.Inputs,
				Placement:       pl,
				CookieCount:     cc,
				StorageKeyCount: sk,
			})
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		return string(b), true, nil

	case "bar_get":
		id := strings.TrimSpace(str("id"))
		if id == "" {
			return "", true, fmt.Errorf("id 不能为空")
		}
		st, err := store.Open(s.DataDir)
		if err != nil {
			return "", true, err
		}
		sc, ok := st.Script(id)
		if !ok || !sc.IsBar() {
			return "", true, fmt.Errorf("没有 .bar 组件 %q", id)
		}
		sp, _ := bar.Parse(sc.Source)
		set := st.Settings()
		pl := mcpPlacementFor(id, true, set)
		var saved map[string]string
		if set.ScriptVars != nil {
			saved = set.ScriptVars[id]
		}
		inputs := bar.MergeInputs(sp, saved)
		if inc, _ := a["includeSecrets"].(bool); !inc {
			inputs = bar.MaskInputs(sp, inputs)
		}
		jar, _ := bar.NewCookieJar(bar.CookiesPath(s.DataDir, id))
		kv, _ := bar.NewKVStore(bar.StoragePath(s.DataDir, id))
		cc := 0
		if jar != nil {
			cc = jar.Count()
		}
		var keys []string
		if kv != nil {
			keys = kv.Keys()
		}
		payload := map[string]any{
			"id":          sc.ID,
			"name":        sc.Name,
			"origin":      sc.Origin,
			"home":        sc.Home,
			"spec":        sp,
			"source":      sc.Source,
			"placement":   pl,
			"inputs":      inputs,
			"cookieCount": cc,
			"storageKeys": keys,
		}
		b, _ := json.MarshalIndent(payload, "", "  ")
		return string(b), true, nil

	case "bar_delete":
		id := strings.TrimSpace(str("id"))
		if id == "" {
			return "", true, fmt.Errorf("id 不能为空")
		}
		if err := validateBarID(id); err != nil {
			return "", true, err
		}
		st, err := store.Open(s.DataDir)
		if err != nil {
			return "", true, err
		}
		sc, ok := st.Script(id)
		if !ok || !sc.IsBar() {
			return "", true, fmt.Errorf("没有 .bar 组件 %q", id)
		}
		if sc.Origin == store.OriginBuiltin {
			return "", true, fmt.Errorf("内置 .bar 组件 %q 不能删除", id)
		}
		if err := st.DeleteScript(id); err != nil {
			return "", true, err
		}
		set := st.Settings()
		changed := false
		if set.BarPlacements != nil {
			if _, ok := set.BarPlacements[id]; ok {
				delete(set.BarPlacements, id)
				changed = true
			}
		}
		if set.ScriptVars != nil {
			if _, ok := set.ScriptVars[id]; ok {
				delete(set.ScriptVars, id)
				changed = true
			}
		}
		if changed {
			if err := st.SetSettings(set); err != nil {
				return "", true, err
			}
		}
		if dir := bar.ComponentDataDir(s.DataDir, id); dir != "" {
			_ = os.RemoveAll(dir)
		}
		s.notifyChange()
		return "已删除 .bar 组件 " + id, true, nil

	case "bar_check":
		source := str("source")
		lang := str("lang")
		if lang != "en" {
			lang = "zh"
		}
		sp, snap, warnings, err := bar.CheckRender(source, lang)
		if err != nil {
			return "", true, err
		}
		payload := map[string]any{
			"ok":       true,
			"spec":     sp,
			"warnings": warnings,
			"preview": map[string]any{
				"tray":       snap.Tray,
				"panel":      snap.Panel,
				"panelTexts": extractBarTexts(snap.Panel),
				"logs":       snap.Logs,
			},
		}
		b, _ := json.MarshalIndent(payload, "", "  ")
		return string(b), true, nil

	case "bar_run":
		return s.callBarRun(a)

	case "bar_cookies_get":
		id := strings.TrimSpace(str("id"))
		if err := validateBarID(id); err != nil {
			return "", true, err
		}
		jar, err := bar.NewCookieJar(bar.CookiesPath(s.DataDir, id))
		if err != nil {
			return "", true, err
		}
		domFilter := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(str("domain")), "."))
		incVal, _ := a["includeValues"].(bool)
		type cookieItem struct {
			Name     string `json:"name"`
			Value    string `json:"value"`
			Domain   string `json:"domain"`
			Path     string `json:"path,omitempty"`
			Expires  string `json:"expires,omitempty"`
			Secure   bool   `json:"secure,omitempty"`
			HttpOnly bool   `json:"httpOnly,omitempty"`
		}
		var out []cookieItem
		for _, c := range jar.List() {
			if domFilter != "" {
				cd := strings.ToLower(strings.TrimPrefix(c.Domain, "."))
				if cd != domFilter && !strings.HasSuffix(cd, "."+domFilter) {
					continue
				}
			}
			val := bar.MaskValue(c.Value)
			if incVal {
				val = c.Value
			}
			exp := ""
			if !c.Expires.IsZero() {
				exp = c.Expires.UTC().Format(time.RFC3339)
			}
			out = append(out, cookieItem{
				Name: c.Name, Value: val, Domain: c.Domain, Path: c.Path,
				Expires: exp, Secure: c.Secure, HttpOnly: c.HttpOnly,
			})
		}
		payload := map[string]any{
			"id":      id,
			"count":   len(out),
			"cookies": out,
		}
		b, _ := json.MarshalIndent(payload, "", "  ")
		return string(b), true, nil

	case "bar_cookies_set":
		id := strings.TrimSpace(str("id"))
		if err := validateBarID(id); err != nil {
			return "", true, err
		}
		jar, err := bar.NewCookieJar(bar.CookiesPath(s.DataDir, id))
		if err != nil {
			return "", true, err
		}
		added := 0
		if curlCmd := strings.TrimSpace(str("curl")); curlCmd != "" {
			_, n, err := jar.ImportCurl(curlCmd)
			if err != nil {
				return "", true, err
			}
			added += n
		}
		if hdr := strings.TrimSpace(str("cookieHeader")); hdr != "" {
			u := strings.TrimSpace(str("url"))
			n, err := jar.SetHeader(u, hdr)
			if err != nil {
				return "", true, err
			}
			added += n
		}
		if rawList, ok := a["cookies"].([]any); ok && len(rawList) > 0 {
			n, err := applyCookieObjects(jar, rawList)
			if err != nil {
				return "", true, err
			}
			added += n
		}
		if added == 0 {
			return "", true, fmt.Errorf("请提供 curl、cookieHeader(+url) 或 cookies 数组")
		}
		s.notifyChange()
		return fmt.Sprintf("已为 %s 写入 %d 条 Cookie（当前共 %d 条）", id, added, jar.Count()), true, nil

	case "bar_cookies_clear":
		id := strings.TrimSpace(str("id"))
		if err := validateBarID(id); err != nil {
			return "", true, err
		}
		jar, err := bar.NewCookieJar(bar.CookiesPath(s.DataDir, id))
		if err != nil {
			return "", true, err
		}
		dom := strings.TrimSpace(str("domain"))
		if err := jar.Clear(dom); err != nil {
			return "", true, err
		}
		s.notifyChange()
		if dom != "" {
			return fmt.Sprintf("已清空 %s 在域名 %s 下的 Cookie（剩余 %d 条）", id, dom, jar.Count()), true, nil
		}
		return fmt.Sprintf("已清空 %s 的全部 Cookie", id), true, nil

	case "bar_storage_get":
		id := strings.TrimSpace(str("id"))
		if err := validateBarID(id); err != nil {
			return "", true, err
		}
		kv, err := bar.NewKVStore(bar.StoragePath(s.DataDir, id))
		if err != nil {
			return "", true, err
		}
		key := str("key")
		if key != "" {
			val, ok := kv.Get(key)
			payload := map[string]any{"id": id, "key": key, "found": ok, "value": val}
			b, _ := json.MarshalIndent(payload, "", "  ")
			return string(b), true, nil
		}
		payload := map[string]any{"id": id, "count": kv.Count(), "keys": kv.Keys(), "items": kv.All()}
		b, _ := json.MarshalIndent(payload, "", "  ")
		return string(b), true, nil

	case "bar_storage_set":
		id := strings.TrimSpace(str("id"))
		if err := validateBarID(id); err != nil {
			return "", true, err
		}
		kv, err := bar.NewKVStore(bar.StoragePath(s.DataDir, id))
		if err != nil {
			return "", true, err
		}
		wrote := 0
		if k := str("key"); k != "" {
			v := fmt.Sprint(a["value"])
			if err := kv.Set(k, v); err != nil {
				return "", true, err
			}
			wrote++
		}
		if vals, ok := a["values"].(map[string]any); ok {
			for k, v := range vals {
				if err := kv.Set(k, fmt.Sprint(v)); err != nil {
					return "", true, err
				}
				wrote++
			}
		}
		if wrote == 0 {
			return "", true, fmt.Errorf("请提供 key+value 或 values 映射")
		}
		s.notifyChange()
		return fmt.Sprintf("已为 %s 写入 %d 条存储记录（共 %d 条）", id, wrote, kv.Count()), true, nil

	case "bar_storage_clear":
		id := strings.TrimSpace(str("id"))
		if err := validateBarID(id); err != nil {
			return "", true, err
		}
		kv, err := bar.NewKVStore(bar.StoragePath(s.DataDir, id))
		if err != nil {
			return "", true, err
		}
		key := str("key")
		if key != "" {
			if err := kv.Delete(key); err != nil {
				return "", true, err
			}
			s.notifyChange()
			return fmt.Sprintf("已从 %s 删除存储键 %s", id, key), true, nil
		}
		if err := kv.Clear(); err != nil {
			return "", true, err
		}
		s.notifyChange()
		return fmt.Sprintf("已清空 %s 的全部存储", id), true, nil
	}
	return "", false, nil
}

func validateBarID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
		return fmt.Errorf("非法的 .bar id %q", id)
	}
	return nil
}

func applyCookieObjects(jar *bar.CookieJar, rawList []any) (int, error) {
	added := 0
	for _, item := range rawList {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		c := bar.StoredCookie{
			Name:   fmt.Sprint(orVal(m["name"], "")),
			Value:  fmt.Sprint(orVal(m["value"], "")),
			Domain: fmt.Sprint(orVal(m["domain"], "")),
			Path:   fmt.Sprint(orVal(m["path"], "/")),
		}
		if b, ok := m["secure"].(bool); ok {
			c.Secure = b
		}
		if b, ok := m["httpOnly"].(bool); ok {
			c.HttpOnly = b
		}
		switch exp := m["expires"].(type) {
		case string:
			if exp != "" {
				if t, err := time.Parse(time.RFC3339, exp); err == nil {
					c.Expires = t
				}
			}
		case float64:
			if exp > 1e11 {
				c.Expires = time.UnixMilli(int64(exp))
			} else if exp > 0 {
				c.Expires = time.Unix(int64(exp), 0)
			}
		}
		if err := jar.Set(c); err != nil {
			return added, err
		}
		added++
	}
	return added, nil
}

func orVal(v, fallback any) any {
	if v == nil {
		return fallback
	}
	return v
}

type mockFetchSpec struct {
	Status    int
	Headers   map[string]string
	SetCookie []string
	Body      string
}

type recordedFetch struct {
	Method      string   `json:"method"`
	URL         string   `json:"url"`
	HasCookie   bool     `json:"hasCookie"`
	CookieNames []string `json:"cookieNames,omitempty"`
	Status      int      `json:"status,omitempty"`
	Mocked      bool     `json:"mocked"`
	Error       string   `json:"error,omitempty"`
}

type barSandboxTransport struct {
	mocks        map[string]mockFetchSpec
	allowNetwork bool
	base         http.RoundTripper

	mu       sync.Mutex
	requests []recordedFetch
}

func (t *barSandboxTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	u := req.URL.String()
	var cookieNames []string
	for _, c := range req.Cookies() {
		cookieNames = append(cookieNames, c.Name)
	}
	rec := recordedFetch{
		Method:      req.Method,
		URL:         u,
		HasCookie:   req.Header.Get("Cookie") != "",
		CookieNames: cookieNames,
	}

	if spec, ok := t.matchMock(u); ok {
		rec.Mocked = true
		rec.Status = spec.Status
		if rec.Status == 0 {
			rec.Status = http.StatusOK
		}
		hdr := http.Header{}
		for k, v := range spec.Headers {
			if strings.EqualFold(k, "Set-Cookie") {
				hdr.Add("Set-Cookie", v)
			} else {
				hdr.Set(k, v)
			}
		}
		for _, sc := range spec.SetCookie {
			hdr.Add("Set-Cookie", sc)
		}
		if hdr.Get("Content-Type") == "" {
			hdr.Set("Content-Type", "application/json")
		}
		t.appendRec(rec)
		return &http.Response{
			StatusCode: rec.Status,
			Status:     fmt.Sprintf("%d %s", rec.Status, http.StatusText(rec.Status)),
			Header:     hdr,
			Body:       io.NopCloser(strings.NewReader(spec.Body)),
			Request:    req,
		}, nil
	}

	if !t.allowNetwork {
		err := errors.New("network disabled in bar_run sandbox (provide mockFetch or set allowNetwork: true)")
		rec.Error = err.Error()
		t.appendRec(rec)
		return nil, err
	}
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)
	if err != nil {
		rec.Error = err.Error()
	} else {
		rec.Status = resp.StatusCode
	}
	t.appendRec(rec)
	return resp, err
}

func (t *barSandboxTransport) matchMock(u string) (mockFetchSpec, bool) {
	if len(t.mocks) == 0 {
		return mockFetchSpec{}, false
	}
	if spec, ok := t.mocks[u]; ok {
		return spec, true
	}
	bestKey := ""
	for k := range t.mocks {
		if k == "*" {
			continue
		}
		if strings.HasPrefix(u, k) && len(k) > len(bestKey) {
			bestKey = k
		}
	}
	if bestKey != "" {
		return t.mocks[bestKey], true
	}
	if spec, ok := t.mocks["*"]; ok {
		return spec, true
	}
	return mockFetchSpec{}, false
}

func (t *barSandboxTransport) appendRec(r recordedFetch) {
	t.mu.Lock()
	t.requests = append(t.requests, r)
	t.mu.Unlock()
}

func (t *barSandboxTransport) snapshot() []recordedFetch {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]recordedFetch(nil), t.requests...)
}

func parseMockFetch(raw map[string]any) map[string]mockFetchSpec {
	if len(raw) == 0 {
		return nil
	}
	out := make(map[string]mockFetchSpec, len(raw))
	for pattern, v := range raw {
		var sp mockFetchSpec
		switch x := v.(type) {
		case string:
			sp.Status = http.StatusOK
			sp.Body = x
		case map[string]any:
			if st, ok := x["status"].(float64); ok {
				sp.Status = int(st)
			}
			if sp.Status == 0 {
				sp.Status = http.StatusOK
			}
			if hm, ok := x["headers"].(map[string]any); ok {
				sp.Headers = map[string]string{}
				for hk, hv := range hm {
					sp.Headers[hk] = fmt.Sprint(hv)
				}
			}
			switch sc := x["setCookie"].(type) {
			case string:
				if sc != "" {
					sp.SetCookie = append(sp.SetCookie, sc)
				}
			case []any:
				for _, item := range sc {
					if s := fmt.Sprint(item); s != "" {
						sp.SetCookie = append(sp.SetCookie, s)
					}
				}
			}
			if b, has := x["body"]; has && b != nil {
				switch bv := b.(type) {
				case string:
					sp.Body = bv
				default:
					mb, _ := json.Marshal(bv)
					sp.Body = string(mb)
				}
			}
		}
		out[pattern] = sp
	}
	return out
}

func (s Server) callBarRun(a map[string]any) (string, bool, error) {
	str := func(key string) string {
		v, _ := a[key].(string)
		return v
	}
	source := str("source")
	id := strings.TrimSpace(str("id"))
	var savedInputs map[string]string
	if source == "" && id != "" {
		st, err := store.Open(s.DataDir)
		if err != nil {
			return "", true, err
		}
		sc, ok := st.Script(id)
		if !ok || !sc.IsBar() {
			return "", true, fmt.Errorf("没有 .bar 组件 %q", id)
		}
		source = sc.Source
		set := st.Settings()
		if set.ScriptVars != nil {
			savedInputs = set.ScriptVars[id]
		}
	}
	if strings.TrimSpace(source) == "" {
		return "", true, fmt.Errorf("请提供 source 或 id")
	}

	lang := str("lang")
	if lang != "en" {
		lang = "zh"
	}
	sp, _, warnings, err := bar.CheckRender(source, lang)
	if err != nil {
		return "", true, err
	}
	if id == "" {
		id = sp.ID
	}

	inputs := map[string]string{}
	for k, v := range savedInputs {
		inputs[k] = v
	}
	if rawIn, ok := a["inputs"].(map[string]any); ok {
		for k, v := range rawIn {
			inputs[k] = fmt.Sprint(v)
		}
	}

	useSaved, _ := a["useSavedState"].(bool)
	var jar *bar.CookieJar
	var kv *bar.KVStore
	if useSaved && id != "" {
		if diskJar, err := bar.NewCookieJar(bar.CookiesPath(s.DataDir, id)); err == nil {
			jar = diskJar.CloneInMemory()
		}
		if diskKV, err := bar.NewKVStore(bar.StoragePath(s.DataDir, id)); err == nil {
			kv = diskKV.CloneInMemory()
		}
	}
	if jar == nil {
		jar, _ = bar.NewCookieJar("")
	}
	if kv == nil {
		kv, _ = bar.NewKVStore("")
	}

	if rawList, ok := a["cookies"].([]any); ok && len(rawList) > 0 {
		if _, err := applyCookieObjects(jar, rawList); err != nil {
			return "", true, err
		}
	}
	if hdr := strings.TrimSpace(str("cookieHeader")); hdr != "" {
		if _, err := jar.SetHeader(str("cookieURL"), hdr); err != nil {
			return "", true, err
		}
	}
	if curlCmd := strings.TrimSpace(str("curl")); curlCmd != "" {
		if _, _, err := jar.ImportCurl(curlCmd); err != nil {
			return "", true, err
		}
	}
	if rawStorage, ok := a["storage"].(map[string]any); ok {
		for k, v := range rawStorage {
			if err := kv.Set(k, fmt.Sprint(v)); err != nil {
				return "", true, err
			}
		}
	}

	var mocks map[string]mockFetchSpec
	if rawMock, ok := a["mockFetch"].(map[string]any); ok {
		mocks = parseMockFetch(rawMock)
	}
	allowNetwork, _ := a["allowNetwork"].(bool)
	offline := !allowNetwork && len(mocks) == 0

	waitScheduled := true
	if ws, ok := a["waitScheduled"].(bool); ok {
		waitScheduled = ws
	}
	timeout := 5 * time.Second
	if tm, ok := a["timeoutMs"].(float64); ok && tm > 0 {
		ms := int(tm)
		if ms < 500 {
			ms = 500
		} else if ms > 15000 {
			ms = 15000
		}
		timeout = time.Duration(ms) * time.Millisecond
	}

	tr := &barSandboxTransport{mocks: mocks, allowNetwork: allowNetwork}
	var mu sync.Mutex
	var openedURLs []string
	var copiedTexts []string

	inst, err := bar.NewInstance(sp, bar.Options{
		Client:  &http.Client{Timeout: timeout, Transport: tr},
		Inputs:  inputs,
		Lang:    lang,
		Offline: offline,
		Cookies: jar,
		Storage: kv,
		OnOpenURL: func(u string) {
			mu.Lock()
			openedURLs = append(openedURLs, u)
			mu.Unlock()
		},
		OnCopyText: func(txt string) {
			mu.Lock()
			copiedTexts = append(copiedTexts, txt)
			mu.Unlock()
		},
	})
	if err != nil {
		return "", true, err
	}
	defer inst.Close()

	settled := true
	if waitScheduled {
		settled = inst.WaitIdle(timeout)
	}

	actionID := strings.TrimSpace(str("actionID"))
	actionInvoked := false
	if actionID != "" {
		if _, ok := inst.InvokeActionSync(actionID, timeout); ok {
			actionInvoked = true
			if waitScheduled {
				inst.WaitIdle(timeout)
			}
		}
	}

	snap := inst.Snapshot()
	actions := bar.CollectActions(snap)

	cookieSummary := make([]map[string]any, 0)
	for _, c := range jar.List() {
		cookieSummary = append(cookieSummary, map[string]any{
			"name":        c.Name,
			"domain":      c.Domain,
			"path":        c.Path,
			"secure":      c.Secure,
			"httpOnly":    c.HttpOnly,
			"maskedValue": bar.MaskValue(c.Value),
		})
	}
	sort.Slice(cookieSummary, func(i, j int) bool {
		return fmt.Sprint(cookieSummary[i]["name"]) < fmt.Sprint(cookieSummary[j]["name"])
	})

	mu.Lock()
	urlsCopy := append([]string(nil), openedURLs...)
	copiedCopy := append([]string(nil), copiedTexts...)
	mu.Unlock()

	payload := map[string]any{
		"id":            sp.ID,
		"settled":       settled,
		"actionInvoked": actionInvoked,
		"tray":          snap.Tray,
		"panel":         snap.Panel,
		"panelTexts":    extractBarTexts(snap.Panel),
		"actions":       actions,
		"logs":          snap.Logs,
		"error":         snap.Error,
		"cookies":       cookieSummary,
		"storage":       kv.All(),
		"requests":      tr.snapshot(),
		"openedURLs":    urlsCopy,
		"copiedTexts":   copiedCopy,
		"warnings":      warnings,
	}
	b, _ := json.MarshalIndent(payload, "", "  ")
	return string(b), true, nil
}
