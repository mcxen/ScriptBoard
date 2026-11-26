package main

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/bar"
	"scriptboard/src/internal/caps"
	"scriptboard/src/internal/cmdrun"
	"scriptboard/src/internal/store"
)

func TestCommandsPageLifecycleAndEnglishI18N(t *testing.T) {
	t.Cleanup(func() { setLang(langZH) })
	a := newTestApp(t)
	a.set.Offline = false
	a.set.AllowExec = true
	_ = a.store.SetSettings(a.set)

	// Custom parameterized shell command (executes quickly on local macOS/Linux without network).
	customCmd := cmdrun.Spec{
		ID:      "db-job",
		Name:    "数据库任务",
		Shell:   "sh",
		Command: `printf '{"stage":"%s","pct":%s}' "$SB_PARAM_STAGE" "$SB_PARAM_PCT"`,
		Every:   60,
		Params: []cmdrun.Param{
			{Key: "stage", Label: "阶段", Type: "string", Default: "indexing"},
			{Key: "pct", Label: "进度", Type: "select", Default: "75", Options: []string{"25", "75", "100"}},
		},
		Extract: []cmdrun.ExtractRule{
			{Name: "stage", Kind: cmdrun.KindJSON, Path: "stage", Type: "string"},
			{Name: "pct", Kind: cmdrun.KindJSON, Path: "pct", Type: "number"},
		},
		Display: &cmdrun.DisplayStyle{
			PrimaryField: "pct",
			Variant:      cmdrun.StyleVariantDot,
			Animation:    cmdrun.AnimRipple,
			DotCount:     5,
			DotSize:      8,
			Fields: map[string]cmdrun.FieldStyle{
				"pct": {
					Label:     "进度",
					Unit:      "%",
					Animation: cmdrun.AnimDots,
					Thresholds: []cmdrun.StyleThreshold{
						{Color: "success", Label: "极速", Animation: cmdrun.AnimPulse},
					},
				},
			},
		},
	}
	if err := a.store.SaveCommand(customCmd); err != nil {
		t.Fatal(err)
	}

	a.router.Push("/commands")
	tt := ui.NewTester(a.view, 1100, 900)
	settle(tt)

	requireText(t, tt, "命令任务")
	requireText(t, tt, "网络 Ping 检测")
	requireText(t, tt, "数据库任务")
	requireText(t, tt, "样式：雷达波纹")

	// Run the custom command with its parameterized defaults from the UI card button.
	// Builtin net-ping is card 1, custom db-job has 2 cards; trigger runCommandInBackground
	// and verify styled return value, threshold status label, and history expand.
	a.runCommandInBackground(customCmd, map[string]string{"stage": "compact", "pct": "100"}, "manual")
	settle(tt)
	requireText(t, tt, "compact")
	requireText(t, tt, "100%")
	requireText(t, tt, "极速")
	requireText(t, tt, "历史记录 (1)")

	// Expand history and raw output.
	click(t, tt, "历史记录 (1)")
	requireText(t, tt, "收起历史")
	click(t, tt, "原始输出")
	requireText(t, tt, "收起输出")

	// Seed one built-in net-ping run record so English audit also covers styled extracted metrics & dot animations.
	_ = a.cmdHistory.Append(cmdrun.BuiltinPingID, cmdrun.Record{
		ID:         "ping-sample-1",
		CommandID:  cmdrun.BuiltinPingID,
		T:          time.Now().UnixMilli(),
		Trigger:    "manual",
		OK:         true,
		DurationMs: 22,
		Extracted: map[string]any{
			"avgMs":      22.4,
			"minMs":      19.1,
			"maxMs":      25.8,
			"jitterMs":   1.4,
			"lossPct":    0.0,
			"resolvedIp": "223.5.5.5",
		},
	}, 10)

	// Switch to English and verify zero Chinese leakage.
	setLang(langEN)
	settle(tt)
	requireText(t, tt, "Commands")
	requireText(t, tt, "Network Ping Monitor")
	requireText(t, tt, "Style: Dot ripple")
	requireText(t, tt, "Allow custom local commands")
	// Note custom user command name "数据库任务" is user-authored data, so remove custom card before strict onlyEnglish check.
	a.deleteCommand(customCmd.ID)
	settle(tt)
	requireText(t, tt, "Avg RTT")
	requireText(t, tt, "Fast")
	onlyEnglish(t, tt, "commands")
}

func TestScriptConsumesCommandsAndRefresh(t *testing.T) {
	a := newTestApp(t)
	a.set.Offline = false
	_ = a.store.SetSettings(a.set)

	cmd := cmdrun.Spec{
		ID:      "act-prog",
		Name:    "Activity Progress",
		Shell:   "sh",
		Command: `printf '{"active":4,"progress":88}'`,
		Extract: []cmdrun.ExtractRule{
			{Name: "active", Kind: cmdrun.KindJSON, Path: "active", Type: "number"},
			{Name: "progress", Kind: cmdrun.KindJSON, Path: "progress", Type: "number"},
		},
		Display: &cmdrun.DisplayStyle{
			PrimaryField: "progress",
			Format:       "{{value}}% ready",
		},
	}
	if err := a.store.SaveCommand(cmd); err != nil {
		t.Fatal(err)
	}

	sc := store.Script{
		ID: "cmd-card-demo", Name: "任务进度", Home: true,
		Source: `export function meta() {
  return { name: "任务进度", platforms: ["desktop"], needs: ["exec"], commands: ["act-prog"] };
}
export function render(ctx) {
  const c = ctx.commands["act-prog"] || {};
  const ext = (c.latest && c.latest.extracted) || {};
  const primary = c.resolved && c.resolved.primary;
  return { title: "任务进度", big: primary ? primary.formattedValue : (ext.progress != null ? ext.progress + "%" : "--"), sub: c.error || ("active=" + (ext.active ?? 0)) };
}`,
	}
	if err := a.store.SaveScript(sc); err != nil {
		t.Fatal(err)
	}

	// 1. Before script-level exec grant, execute must refuse.
	if _, err := a.execute(sc, nil); err == nil {
		t.Fatal("expected missing exec grant error before setGrant")
	}

	// 2. Grant exec on script, enable AllowExec, and refreshScript -> executes command and updates card.
	a.setGrant(sc.ID, caps.Exec, true)
	a.set.AllowExec = true
	_ = a.store.SetSettings(a.set)

	a.refreshScript(sc) // synchronous when a.win == nil
	if st := a.refreshStateOf(sc.ID); st.phase != refreshOK {
		t.Fatalf("expected refreshOK after script command refresh, got %+v", st)
	}
	res, err := a.execute(sc, nil)
	if err != nil {
		t.Fatalf("execute after refreshScript: %v", err)
	}
	if res.Big != "88% ready" || res.Sub != "active=4" {
		t.Fatalf("unexpected script result from ctx.commands: %+v", res)
	}
}

func TestBuiltinPingCommandAndBarSeeded(t *testing.T) {
	if os.Getenv("CI_SKIP_PING_NET") == "1" {
		return
	}
	a := newTestApp(t)
	a.client = &http.Client{Timeout: time.Second}
	if _, ok := a.store.Command(cmdrun.BuiltinPingID); !ok {
		t.Fatal("expected net-ping command seeded by newApp")
	}
	sc, ok := a.store.Script("sb-ping")
	if !ok || !sc.IsBar() {
		t.Fatalf("expected sb-ping builtin .bar seeded, got %+v ok=%v", sc, ok)
	}
	if sc.Home != !isMobile() {
		t.Fatalf("sb-ping Home = %v, want %v", sc.Home, !isMobile())
	}
	sp, err := bar.Parse(sc.Source)
	if err != nil || len(sp.CommandPerms) != 1 || sp.CommandPerms[0] != cmdrun.BuiltinPingID {
		t.Fatalf("unexpected sb-ping spec: %+v err=%v", sp, err)
	}
}
