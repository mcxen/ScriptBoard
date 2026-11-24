package mcpsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"scriptboard/src/internal/bar"
	"scriptboard/src/internal/feed"
	"scriptboard/src/internal/store"
)

func feedTools() []Tool {
	str := func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	boolProp := func(desc string) map[string]any { return map[string]any{"type": "boolean", "description": desc} }
	intProp := func(desc string) map[string]any { return map[string]any{"type": "integer", "description": desc} }
	obj := func(props map[string]any, required ...string) map[string]any {
		return map[string]any{"type": "object", "properties": props, "required": required}
	}
	return []Tool{
		{
			Name:        "feed_list",
			Description: "List all configured host-side data feeds (curl/http/tcp/file/exec) with cookie names, header keys, extract rules, and intervals.",
			InputSchema: obj(map[string]any{
				"includeSecrets": boolProp("include raw cookies and sensitive headers (default false)"),
			}),
		},
		{
			Name:        "feed_get",
			Description: "Read one data feed's request, fallbacks, extract rule, and interval.",
			InputSchema: obj(map[string]any{
				"id":             str("feed id"),
				"includeSecrets": boolProp("include raw cookies and sensitive headers (default false)"),
			}, "id"),
		},
		{
			Name:        "feed_save",
			Description: "Create or update a host-side data feed from a curl command (with -b cookies and -H headers) or URL, plus JSON/regex extraction.",
			InputSchema: obj(map[string]any{
				"id":          str("feed id (lowercase letters, digits, hyphens)"),
				"name":        str("display name"),
				"request":     str("curl command (with optional -b cookies / -H headers) or http/https/tcp/file/exec address"),
				"fallbacks":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "optional fallback curl commands or URLs"},
				"extractKind": str("json or regex"),
				"extractRule": str("JSON dotted path (e.g. data.price or items.#.v) or regular expression"),
				"every":       intProp("poll interval in seconds (min 60, default 60)"),
				"agg":         str("bucket aggregation: last (default), avg, max, or min"),
				"names":       map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "optional display names for extracted numbers"},
				"timeout":     intProp("optional request timeout in seconds (1..120)"),
				"retries":     intProp("optional retries count (-1 for none)"),
			}, "id", "name", "request", "extractKind", "extractRule"),
		},
		{
			Name:        "feed_delete",
			Description: "Delete a host-side data feed by id.",
			InputSchema: obj(map[string]any{
				"id": str("feed id"),
			}, "id"),
		},
		{
			Name:        "feed_test",
			Description: "Execute a one-off test fetch and number extraction for a saved feed id or inline request + extract rule without saving.",
			InputSchema: obj(map[string]any{
				"id":          str("existing feed id (optional if request + extractKind + extractRule are provided)"),
				"request":     str("curl command or address"),
				"fallbacks":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "optional fallback requests"},
				"extractKind": str("json or regex"),
				"extractRule": str("JSON path or regex"),
			}),
		},
	}
}

func (s Server) callFeed(name string, a map[string]any) (string, bool, error) {
	str := func(key string) string {
		v, _ := a[key].(string)
		return v
	}
	switch name {
	case "feed_list":
		st, err := store.Open(s.DataDir)
		if err != nil {
			return "", true, err
		}
		inc, _ := a["includeSecrets"].(bool)
		var out []map[string]any
		for _, f := range st.Feeds() {
			out = append(out, summarizeFeed(f, inc))
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		return string(b), true, nil

	case "feed_get":
		id := strings.TrimSpace(str("id"))
		if id == "" {
			return "", true, fmt.Errorf("id 不能为空")
		}
		st, err := store.Open(s.DataDir)
		if err != nil {
			return "", true, err
		}
		f, ok := st.Feed(id)
		if !ok {
			return "", true, fmt.Errorf("没有数据源 %q", id)
		}
		inc, _ := a["includeSecrets"].(bool)
		payload := summarizeFeed(f, inc)
		if inc {
			payload["request"] = f.Request
			payload["fallbacks"] = f.Fallbacks
		} else {
			payload["request"] = redactFeedRequest(f.Request)
			if len(f.Fallbacks) > 0 {
				fbs := make([]feed.Request, len(f.Fallbacks))
				for i, fb := range f.Fallbacks {
					fbs[i] = redactFeedRequest(fb)
				}
				payload["fallbacks"] = fbs
			}
		}
		b, _ := json.MarshalIndent(payload, "", "  ")
		return string(b), true, nil

	case "feed_save":
		id := strings.TrimSpace(str("id"))
		title := strings.TrimSpace(str("name"))
		reqText := str("request")
		kind := strings.TrimSpace(str("extractKind"))
		rule := strings.TrimSpace(str("extractRule"))
		req, err := feed.ParseRequest(reqText)
		if err != nil {
			return "", true, err
		}
		var fallbacks []feed.Request
		for _, rawFB := range toStringSlice(a["fallbacks"]) {
			if strings.TrimSpace(rawFB) == "" {
				continue
			}
			fb, err := feed.ParseRequest(rawFB)
			if err != nil {
				return "", true, fmt.Errorf("备用请求格式不对：%w", err)
			}
			fallbacks = append(fallbacks, fb)
		}
		every := feed.MinEvery
		if ev, ok := a["every"].(float64); ok && int(ev) > 0 {
			every = int(ev)
		}
		agg := strings.TrimSpace(str("agg"))
		if agg == "" {
			agg = "last"
		}
		src := feed.Source{
			ID:        id,
			Name:      title,
			Request:   req,
			Fallbacks: fallbacks,
			Extract:   feed.Extract{Kind: kind, Path: rule, Pattern: rule},
			Names:     toStringSlice(a["names"]),
			Every:     every,
			Agg:       agg,
		}
		if kind == feed.KindJSON {
			src.Extract.Pattern = ""
		} else if kind == feed.KindRegex {
			src.Extract.Path = ""
		}
		if to, ok := a["timeout"].(float64); ok {
			src.Timeout = int(to)
		}
		if rt, ok := a["retries"].(float64); ok {
			src.Retries = int(rt)
		}
		if err := src.Validate(); err != nil {
			return "", true, err
		}
		st, err := store.Open(s.DataDir)
		if err != nil {
			return "", true, err
		}
		if err := st.SaveFeed(src); err != nil {
			return "", true, err
		}
		s.notifyChange()
		return fmt.Sprintf("已保存数据源 %s（%s，every=%ds，cookies=%d）", src.ID, src.Name, src.Every, len(src.Request.Cookies)), true, nil

	case "feed_delete":
		id := strings.TrimSpace(str("id"))
		if id == "" {
			return "", true, fmt.Errorf("id 不能为空")
		}
		st, err := store.Open(s.DataDir)
		if err != nil {
			return "", true, err
		}
		if _, ok := st.Feed(id); !ok {
			return "", true, fmt.Errorf("没有数据源 %q", id)
		}
		if err := st.DeleteFeed(id); err != nil {
			return "", true, err
		}
		s.notifyChange()
		return "已删除数据源 " + id, true, nil

	case "feed_test":
		st, err := store.Open(s.DataDir)
		allowExec := err == nil && st.Settings().AllowExec

		var src feed.Source
		if id := strings.TrimSpace(str("id")); id != "" {
			if st == nil {
				return "", true, err
			}
			f, ok := st.Feed(id)
			if !ok {
				return "", true, fmt.Errorf("没有数据源 %q", id)
			}
			src = f
		} else {
			req, err := feed.ParseRequest(str("request"))
			if err != nil {
				return "", true, err
			}
			var fallbacks []feed.Request
			for _, rawFB := range toStringSlice(a["fallbacks"]) {
				if strings.TrimSpace(rawFB) == "" {
					continue
				}
				fb, err := feed.ParseRequest(rawFB)
				if err != nil {
					return "", true, fmt.Errorf("备用请求格式不对：%w", err)
				}
				fallbacks = append(fallbacks, fb)
			}
			kind := strings.TrimSpace(str("extractKind"))
			rule := strings.TrimSpace(str("extractRule"))
			ex := feed.Extract{Kind: kind}
			if kind == feed.KindJSON {
				ex.Path = rule
			} else {
				ex.Pattern = rule
			}
			if err := ex.Validate(); err != nil {
				return "", true, err
			}
			src = feed.Source{
				ID:        "test-feed",
				Name:      "test",
				Request:   req,
				Fallbacks: fallbacks,
				Extract:   ex,
				Every:     feed.MinEvery,
			}
		}

		mux := feed.Mux{
			HTTP:      feed.HTTPFetcher{Client: &http.Client{Timeout: 10 * time.Second}},
			AllowExec: func() bool { return allowExec },
		}
		var lastErr error
		for idx, req := range src.Requests() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			body, err := mux.Fetch(ctx, req)
			cancel()
			if err != nil {
				lastErr = err
				continue
			}
			nums, err := src.Extract.Numbers(body)
			if err != nil {
				lastErr = err
				continue
			}
			payload := map[string]any{
				"ok":          true,
				"usedRequest": idx,
				"numbers":     nums,
			}
			b, _ := json.MarshalIndent(payload, "", "  ")
			return string(b), true, nil
		}
		if lastErr == nil {
			lastErr = fmt.Errorf("没有可用的请求")
		}
		return "", true, lastErr
	}
	return "", false, nil
}

func summarizeFeed(f feed.Source, includeSecrets bool) map[string]any {
	cookieNames := make([]string, 0, len(f.Request.Cookies))
	for k := range f.Request.Cookies {
		cookieNames = append(cookieNames, k)
	}
	sort.Strings(cookieNames)
	headerKeys := make([]string, 0, len(f.Request.Headers))
	for k := range f.Request.Headers {
		headerKeys = append(headerKeys, k)
	}
	sort.Strings(headerKeys)

	out := map[string]any{
		"id":             f.ID,
		"name":           f.Name,
		"url":            f.Request.URL,
		"method":         orStr(f.Request.Method, "GET"),
		"every":          f.Every,
		"agg":            orStr(f.Agg, "last"),
		"extract":        f.Extract,
		"names":          f.Names,
		"cookieNames":    cookieNames,
		"headerKeys":     headerKeys,
		"fallbacksCount": len(f.Fallbacks),
	}
	if includeSecrets {
		out["cookies"] = f.Request.Cookies
		out["headers"] = f.Request.Headers
	}
	return out
}

func redactFeedRequest(r feed.Request) feed.Request {
	out := r
	if len(r.Cookies) > 0 {
		out.Cookies = make(map[string]string, len(r.Cookies))
		for k, v := range r.Cookies {
			out.Cookies[k] = bar.MaskValue(v)
		}
	}
	if len(r.Headers) > 0 {
		out.Headers = make(map[string]string, len(r.Headers))
		for k, v := range r.Headers {
			lk := strings.ToLower(k)
			if lk == "authorization" || lk == "cookie" || strings.Contains(lk, "token") || strings.Contains(lk, "secret") || strings.Contains(lk, "key") {
				out.Headers[k] = bar.MaskValue(v)
			} else {
				out.Headers[k] = v
			}
		}
	}
	return out
}
