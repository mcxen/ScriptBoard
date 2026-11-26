package mcpsvc

import (
	"encoding/json"
	"strings"
	"testing"

	"scriptboard/src/internal/bar"
	"scriptboard/src/internal/cmdrun"
	"scriptboard/src/internal/store"
)

func TestMCPCommandLifecycleAndPermissions(t *testing.T) {
	dir := t.TempDir()
	ro := false
	srv := Server{DataDir: dir, ReadOnly: func() bool { return ro }}

	// 1. Save a parameterized shell command with JSON + regex extract rules and a password param.
	out, isErr := callMCPTool(t, srv, "cmd_save", map[string]any{
		"id":          "db-activity",
		"name":        "DB Activity",
		"description": "Inspect local activity sample",
		"shell":       "sh",
		"command":     `printf '{"progress":%s,"state":"running","tokenLen":%d}' "$SB_PARAM_PCT" "${#SB_PARAM_TOKEN}"`,
		"env":         map[string]any{"API_TOKEN": "top-secret-env", "MODE": "fast"},
		"params": []any{
			map[string]any{"key": "pct", "label": "Pct", "type": "number", "default": "64"},
			map[string]any{"key": "token", "label": "Token", "type": "password", "default": "pw-hidden"},
		},
		"extract": []any{
			map[string]any{"name": "progress", "kind": "json", "path": "progress", "type": "number"},
			map[string]any{"name": "state", "kind": "json", "path": "state", "type": "string"},
		},
		"every": 30,
	})
	if isErr || !strings.Contains(out, "已保存命令 db-activity") {
		t.Fatalf("cmd_save failed: err=%v out=%s", isErr, out)
	}

	// 2. cmd_list and cmd_get redact secrets by default; includeSecrets=true reveals them.
	out, isErr = callMCPTool(t, srv, "cmd_get", map[string]any{"id": "db-activity"})
	if isErr || !strings.Contains(out, `"API_TOKEN": "***"`) || strings.Contains(out, "pw-hidden") {
		t.Fatalf("expected redacted secrets in cmd_get: err=%v out=%s", isErr, out)
	}
	out, isErr = callMCPTool(t, srv, "cmd_get", map[string]any{"id": "db-activity", "includeSecrets": true})
	if isErr || !strings.Contains(out, "top-secret-env") || !strings.Contains(out, "pw-hidden") {
		t.Fatalf("expected raw secrets with includeSecrets=true: err=%v out=%s", isErr, out)
	}

	// 3. Running custom command fails while AllowExec is false.
	out, isErr = callMCPTool(t, srv, "cmd_run", map[string]any{"id": "db-activity"})
	if !isErr || !strings.Contains(out, "未开启") {
		t.Fatalf("expected AllowExec denial, got err=%v out=%s", isErr, out)
	}

	// 4. Enable AllowExec and verify cmd_test leaves no history while cmd_run persists masked history.
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	set := st.Settings()
	set.AllowExec = true
	if err := st.SetSettings(set); err != nil {
		t.Fatal(err)
	}

	out, isErr = callMCPTool(t, srv, "cmd_test", map[string]any{
		"id":     "db-activity",
		"params": map[string]any{"pct": "91"},
	})
	if isErr || !strings.Contains(out, `"progress": 91`) {
		t.Fatalf("cmd_test failed: err=%v out=%s", isErr, out)
	}
	histOut, isErr := callMCPTool(t, srv, "cmd_history", map[string]any{"id": "db-activity"})
	if isErr || !strings.Contains(histOut, `"count": 0`) {
		t.Fatalf("cmd_test should not record history: err=%v out=%s", isErr, histOut)
	}

	// Two cmd_run executions with params -> persisted newest-first with masked password param.
	out, isErr = callMCPTool(t, srv, "cmd_run", map[string]any{
		"id":     "db-activity",
		"params": map[string]any{"pct": "55", "token": "live-secret"},
	})
	if isErr || !strings.Contains(out, `"progress": 55`) || !strings.Contains(out, `"tokenLen": 11`) {
		t.Fatalf("cmd_run #1 failed: err=%v out=%s", isErr, out)
	}
	out, isErr = callMCPTool(t, srv, "cmd_run", map[string]any{
		"id":     "db-activity",
		"params": map[string]any{"pct": "100"},
	})
	if isErr || !strings.Contains(out, `"progress": 100`) {
		t.Fatalf("cmd_run #2 failed: err=%v out=%s", isErr, out)
	}

	histOut, isErr = callMCPTool(t, srv, "cmd_history", map[string]any{"id": "db-activity", "limit": 10})
	if isErr || !strings.Contains(histOut, `"count": 2`) || strings.Contains(histOut, "live-secret") {
		t.Fatalf("cmd_history unexpected: err=%v out=%s", isErr, histOut)
	}
	var histPayload struct {
		Count   int             `json:"count"`
		Records []cmdrun.Record `json:"records"`
	}
	if err := json.Unmarshal([]byte(histOut), &histPayload); err != nil {
		t.Fatalf("decode cmd_history: %v", err)
	}
	if histPayload.Records[0].Params["pct"] != "100" || histPayload.Records[1].Params["token"] != "***" {
		t.Fatalf("unexpected history order or masking: %+v", histPayload.Records)
	}

	// 5. Read-only mode blocks mutations and executions, allows inspection.
	ro = true
	for _, tc := range []struct {
		tool string
		args map[string]any
	}{
		{"cmd_save", map[string]any{"id": "x", "name": "x", "command": "echo 1"}},
		{"cmd_style_set", map[string]any{"id": "db-activity", "preset": "dot-pulse"}},
		{"cmd_run", map[string]any{"id": "db-activity"}},
		{"cmd_history_clear", map[string]any{"id": "db-activity"}},
		{"cmd_delete", map[string]any{"id": "db-activity"}},
	} {
		out, isErr = callMCPTool(t, srv, tc.tool, tc.args)
		if !isErr || !strings.Contains(out, "只读") {
			t.Fatalf("expected ReadOnly rejection for %s, got err=%v out=%s", tc.tool, isErr, out)
		}
	}
	ro = false

	// 6. Clear history and delete command.
	out, isErr = callMCPTool(t, srv, "cmd_history_clear", map[string]any{"id": "db-activity"})
	if isErr {
		t.Fatalf("cmd_history_clear failed: %s", out)
	}
	out, isErr = callMCPTool(t, srv, "cmd_delete", map[string]any{"id": "db-activity", "deleteHistory": true})
	if isErr || !strings.Contains(out, "已删除命令 db-activity") {
		t.Fatalf("cmd_delete failed: err=%v out=%s", isErr, out)
	}
}

func TestMCPBarRunMockCommandsScriptRunAndAppInfo(t *testing.T) {
	dir := t.TempDir()
	srv := Server{DataDir: dir}

	// Save a command so app_info reflects counts.commands >= 1 and allowExec.
	_, _ = callMCPTool(t, srv, "cmd_save", map[string]any{
		"id":      "net-ping",
		"name":    "Ping",
		"shell":   "sh",
		"command": "echo ok",
	})

	// Test bar_run with mockCommands against BuiltinPingBar.
	out, isErr := callMCPTool(t, srv, "bar_run", map[string]any{
		"source": bar.BuiltinPingBar,
		"lang":   "en",
		"inputs": map[string]any{"host": "1.1.1.1"},
		"mockCommands": map[string]any{
			"net-ping": map[string]any{
				"ok":         true,
				"exitCode":   0,
				"durationMs": 14,
				"extracted": map[string]any{
					"avgMs":      11.2,
					"minMs":      9.8,
					"maxMs":      13.4,
					"jitterMs":   1.1,
					"lossPct":    0.0,
					"resolvedIp": "1.1.1.1",
				},
			},
		},
	})
	if isErr {
		t.Fatalf("bar_run with mockCommands failed: %s", out)
	}
	for _, want := range []string{`"title": "11ms"`, `"Healthy"`, `"1.1.1.1"`, `"mocked": true`} {
		if !strings.Contains(out, want) {
			t.Fatalf("bar_run output missing %q:\n%s", want, out)
		}
	}

	// Test script_run with mockCommands.
	scriptOut, isErr := callMCPTool(t, srv, "script_run", map[string]any{
		"source": `export function meta() { return { name: "Job", platforms: ["desktop"], needs: ["exec"], commands: ["net-ping"] }; }
export function render(ctx) { return { title: "Job", big: String(ctx.commands["net-ping"].latest.extracted.avgMs) + "ms" }; }`,
		"mockCommands": map[string]any{
			"net-ping": map[string]any{
				"id": "net-ping", "name": "Ping", "ok": true,
				"latest": map[string]any{"ok": true, "extracted": map[string]any{"avgMs": 23.5}},
			},
		},
	})
	if isErr || !strings.Contains(scriptOut, `"big": "23.5ms"`) {
		t.Fatalf("script_run mockCommands failed: err=%v out=%s", isErr, scriptOut)
	}

	// Test app_info advertises $metrix.command and command counts.
	infoOut, isErr := callMCPTool(t, srv, "app_info", map[string]any{})
	if isErr {
		t.Fatalf("app_info failed: %s", infoOut)
	}
	for _, want := range []string{`"$metrix.command"`, `"commandSupport": true`, `"commands": 1`, `"commandDisplayAnimations"`, `"orbit"`, `"ripple"`} {
		if !strings.Contains(infoOut, want) {
			t.Fatalf("app_info missing %q:\n%s", want, infoOut)
		}
	}
}

func TestMCPCommandStyleSetAndPingPreview(t *testing.T) {
	dir := t.TempDir()
	srv := Server{DataDir: dir}
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SaveCommand(cmdrun.BuiltinPingSpec()); err != nil {
		t.Fatal(err)
	}

	// 1. Customize net-ping style via MCP cmd_style_set (preset + overrides) without breaking IsTrustedBuiltin.
	styleOut, isErr := callMCPTool(t, srv, "cmd_style_set", map[string]any{
		"id":       "net-ping",
		"preset":   "dot-wave",
		"dotCount": 6,
		"dotSize":  9,
		"speed":    1.5,
		"fields": map[string]any{
			"avgMs": map[string]any{
				"label":     "Live Latency",
				"unit":      "ms",
				"animation": "wave",
				"thresholds": []any{
					map[string]any{"max": 40, "color": "cyan", "label": "Turbo", "animation": "wave"},
				},
			},
		},
	})
	if isErr {
		t.Fatalf("cmd_style_set failed: %s", styleOut)
	}
	for _, want := range []string{`"builtin": true`, `"animation": "wave"`, `"dotCount": 6`, `"Turbo"`} {
		if !strings.Contains(styleOut, want) {
			t.Fatalf("cmd_style_set response missing %q:\n%s", want, styleOut)
		}
	}

	// Verify store persisted Display while preserving IsTrustedBuiltin.
	stReload, _ := store.Open(dir)
	savedPing, ok := stReload.Command("net-ping")
	if !ok || !cmdrun.IsTrustedBuiltin(savedPing) || savedPing.Display == nil || savedPing.Display.Animation != cmdrun.AnimWave {
		t.Fatalf("persisted ping command unexpected: ok=%v trusted=%v display=%+v", ok, cmdrun.IsTrustedBuiltin(savedPing), savedPing.Display)
	}

	mockPingRec := map[string]any{
		"ok":         true,
		"exitCode":   0,
		"durationMs": 12,
		"extracted": map[string]any{
			"avgMs":      15.4,
			"minMs":      12.0,
			"maxMs":      18.8,
			"jitterMs":   1.9,
			"lossPct":    0.0,
			"resolvedIp": "223.5.5.5",
		},
	}

	// 2. BuiltinPingBar reads persisted style from $metrix.command.style("net-ping").
	runOut, isErr := callMCPTool(t, srv, "bar_run", map[string]any{
		"source":       bar.BuiltinPingBar,
		"lang":         "en",
		"mockCommands": map[string]any{"net-ping": mockPingRec},
	})
	if isErr {
		t.Fatalf("bar_run styled BuiltinPingBar failed: %s", runOut)
	}
	if !strings.Contains(runOut, `"Turbo"`) || !strings.Contains(runOut, `"wave"`) {
		t.Fatalf("bar_run missing custom threshold label / wave animation:\n%s", runOut)
	}

	// 2b. Top-level preset + color shorthand immediately overrides avgMs without needing fields.
	topOut, isErr := callMCPTool(t, srv, "cmd_style_set", map[string]any{
		"id":       "net-ping",
		"preset":   "dot-orbit",
		"color":    "purple",
		"format":   "{{value}}ms·{{status}}",
		"valueMap": map[string]any{"15.4": "Optimal"},
		"fallback": "PendingPing",
	})
	if isErr || !strings.Contains(topOut, `"animation": "orbit"`) || !strings.Contains(topOut, `"color": "purple"`) {
		t.Fatalf("top-level cmd_style_set override failed: err=%v out=%s", isErr, topOut)
	}
	runTopOut, isErr := callMCPTool(t, srv, "bar_run", map[string]any{
		"source":       bar.BuiltinPingBar,
		"lang":         "en",
		"mockCommands": map[string]any{"net-ping": mockPingRec},
	})
	if isErr || !strings.Contains(runTopOut, `"orbit"`) || !strings.Contains(runTopOut, `"purple"`) || !strings.Contains(runTopOut, `Optimalms·Turbo`) {
		t.Fatalf("bar_run after top-level cmd_style_set missing orbit/purple/format/valueMap: err=%v\n%s", isErr, runTopOut)
	}

	// 3. Sandbox override via mockCommandStyles + bar_preview renders Light/Dark PNGs cleanly.
	prevDir := t.TempDir()
	prevOut, isErr := callMCPTool(t, srv, "bar_preview", map[string]any{
		"source":    bar.BuiltinPingBar,
		"lang":      "en",
		"theme":     "both",
		"outputDir": prevDir,
		"mockCommands": map[string]any{
			"net-ping": mockPingRec,
		},
		"mockCommandStyles": map[string]any{
			"net-ping": map[string]any{
				"primaryField": "avgMs",
				"variant":      "dot",
				"animation":    "orbit",
				"color":        "purple",
				"dotCount":     3,
			},
		},
	})
	if isErr {
		t.Fatalf("bar_preview styled BuiltinPingBar failed: %s", prevOut)
	}
	if !strings.Contains(prevOut, `"ok": true`) {
		t.Fatalf("expected clean bar_preview audit, got:\n%s", prevOut)
	}

	// 4. Reset restores shipped BuiltinPingSpec display style (ripple).
	resetOut, isErr := callMCPTool(t, srv, "cmd_style_set", map[string]any{
		"id":    "net-ping",
		"reset": true,
	})
	if isErr || !strings.Contains(resetOut, `"animation": "ripple"`) {
		t.Fatalf("cmd_style_set reset failed: err=%v out=%s", isErr, resetOut)
	}
}
