package mcpsvc

import (
	"encoding/json"
	"strings"
	"testing"

	"scriptboard/src/internal/engine"
)

func callMCPTool(t *testing.T, srv Server, name string, args map[string]any) (string, bool) {
	t.Helper()
	raw := srv.Handle([]byte(reqCall(1, name, args)))
	var resp struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error *rpcError `json:"error"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("bad JSON reply %s: %v", string(raw), err)
	}
	if resp.Error != nil {
		t.Fatalf("rpc error: %+v", resp.Error)
	}
	if len(resp.Result.Content) == 0 {
		return "", resp.Result.IsError
	}
	return resp.Result.Content[0].Text, resp.Result.IsError
}

func TestMCPWidgetCatalogScriptRunTablesTrayAndAppInfo(t *testing.T) {
	dir := t.TempDir()
	readOnly := false
	srv := Server{DataDir: dir, ReadOnly: func() bool { return readOnly }}

	// 1. widget_catalog lists all engine specs including table & progressBar.
	out, isErr := callMCPTool(t, srv, "widget_catalog", map[string]any{})
	if isErr || !strings.Contains(out, `"progressBar"`) || !strings.Contains(out, `"table"`) {
		t.Fatalf("widget_catalog failed: err=%v out=%s", isErr, out)
	}
	out, isErr = callMCPTool(t, srv, "widget_catalog", map[string]any{"type": "table"})
	if isErr || !strings.Contains(out, `"table"`) {
		t.Fatalf("widget_catalog(table) failed: %v %s", isErr, out)
	}

	// 2. script_run executes a script with mockSources & vars and returns extracted table.
	src := `export function meta() {
  return {
    name: "测试行情", icon: "chart-line", menubar: true,
    vars: [{ key: "coin", label: "币种", default: "btc", options: [{ value: "btc", label: "BTC" }] }],
    sources: { ticker: "https://example.com/{coin}" }
  };
}
export function render(ctx) {
  const p = (ctx.sources.ticker && ctx.sources.ticker.price) || 0;
  return {
    title: "测试行情", big: "$" + p, sub: ctx.vars.coin, lines: [],
    menubar: { title: "$" + p, icon: "pulse", color: "#10B981", refreshSec: 30 },
    menu: [{ label: "复制价格", action: "copy_headline" }],
    chart: { type: "line", labels: ["T1", "T2"], series: [{ name: "Price", points: [p - 10, p] }] }
  };
}`
	out, isErr = callMCPTool(t, srv, "script_run", map[string]any{
		"source":      src,
		"lang":        "en",
		"vars":        map[string]any{"coin": "btc"},
		"mockSources": map[string]any{"ticker": map[string]any{"price": 95000}},
	})
	if isErr || !strings.Contains(out, `"$95000"`) || !strings.Contains(out, `"widgetType": "line"`) || !strings.Contains(out, `"table"`) {
		t.Fatalf("script_run failed: err=%v out=%s", isErr, out)
	}

	// 3. table_save, table_append, table_list, table_get (JSON & CSV) + ReadOnly guard.
	out, isErr = callMCPTool(t, srv, "table_save", map[string]any{
		"id":      "mcp-budget",
		"name":    "预算表",
		"home":    true,
		"columns": []string{"日期", "金额", "分类"},
		"rows":    [][]string{{"2026-11-01", "120", "餐饮"}},
	})
	if isErr || !strings.Contains(out, "mcp-budget") {
		t.Fatalf("table_save failed: %v %s", isErr, out)
	}
	out, isErr = callMCPTool(t, srv, "table_append", map[string]any{
		"id":   "mcp-budget",
		"rows": [][]string{{"2026-11-02", "80", "交通"}},
	})
	if isErr || !strings.Contains(out, "共 2 行") {
		t.Fatalf("table_append failed: %v %s", isErr, out)
	}
	out, isErr = callMCPTool(t, srv, "table_list", map[string]any{})
	if isErr || !strings.Contains(out, `"mcp-budget"`) {
		t.Fatalf("table_list failed: %v %s", isErr, out)
	}
	out, isErr = callMCPTool(t, srv, "table_get", map[string]any{"id": "mcp-budget"})
	if isErr || !strings.Contains(out, `"sum": 200`) {
		t.Fatalf("table_get JSON summary failed: %v %s", isErr, out)
	}
	out, isErr = callMCPTool(t, srv, "table_get", map[string]any{"id": "mcp-budget", "format": "csv"})
	if isErr || !strings.Contains(out, "2026-11-02,80,交通") {
		t.Fatalf("table_get CSV failed: %v %s", isErr, out)
	}

	// 4. tray_set & tray_list.
	out, isErr = callMCPTool(t, srv, "tray_set", map[string]any{"id": "world-clock", "enabled": true})
	if isErr || !strings.Contains(out, "加入菜单栏") {
		t.Fatalf("tray_set enable failed: %v %s", isErr, out)
	}
	out, isErr = callMCPTool(t, srv, "tray_list", map[string]any{})
	if isErr || !strings.Contains(out, `"world-clock"`) || !strings.Contains(out, `"copy_headline"`) {
		t.Fatalf("tray_list failed: %v %s", isErr, out)
	}
	out, isErr = callMCPTool(t, srv, "tray_set", map[string]any{"id": "world-clock", "enabled": false})
	if isErr || !strings.Contains(out, "移出菜单栏") {
		t.Fatalf("tray_set disable failed: %v %s", isErr, out)
	}

	// 5. ReadOnly enforces refusal on all new write tools while keeping reads open.
	readOnly = true
	for _, writeTool := range []struct {
		name string
		args map[string]any
	}{
		{"table_save", map[string]any{"id": "x", "name": "x", "columns": []string{"a"}, "rows": [][]string{{"1"}}}},
		{"table_append", map[string]any{"id": "mcp-budget", "rows": [][]string{{"a", "1", "b"}}}},
		{"tray_set", map[string]any{"id": "world-clock", "enabled": true}},
	} {
		if _, isErr := callMCPTool(t, srv, writeTool.name, writeTool.args); !isErr {
			t.Errorf("read-only mode allowed %s", writeTool.name)
		}
	}

	// 6. app_info reports full host capabilities in read-only mode.
	out, isErr = callMCPTool(t, srv, "app_info", map[string]any{})
	if isErr || !strings.Contains(out, `"readOnly": true`) || !strings.Contains(out, `"widgetTypes"`) {
		t.Fatalf("app_info failed: %v %s", isErr, out)
	}
	for _, typ := range engine.ChartTypes {
		if !strings.Contains(out, `"`+typ+`"`) {
			t.Errorf("app_info missing widget type %q", typ)
		}
	}
}
