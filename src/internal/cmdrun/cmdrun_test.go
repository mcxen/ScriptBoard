package cmdrun

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSpecValidateAndResolveParams(t *testing.T) {
	if !ValidID("net-ping") || !ValidID("db-active-1") || ValidID("Bad") || ValidID("a") {
		t.Fatal("unexpected ValidID behavior")
	}

	sp := Spec{
		ID:      "db-status",
		Name:    "DB Status",
		Shell:   "bash",
		Command: `echo "{\"ok\":true}"`,
		Every:   30,
		Params: []Param{
			{Key: "env", Type: "select", Default: "prod", Options: []string{"prod", "staging"}},
			{Key: "api-token", Type: "password", Default: "secret123", Required: true},
		},
		Extract: []ExtractRule{
			{Name: "ok", Kind: KindJSON, Path: "ok", Type: "bool"},
		},
	}
	if err := sp.Validate(); err != nil {
		t.Fatalf("valid spec failed: %v", err)
	}

	resolved, masked, ordered, err := ResolveParams(sp.Params, map[string]string{"env": "staging"})
	if err != nil {
		t.Fatalf("ResolveParams: %v", err)
	}
	if resolved["env"] != "staging" || resolved["api-token"] != "secret123" {
		t.Fatalf("unexpected resolved: %#v", resolved)
	}
	if masked["api-token"] != "***" || masked["env"] != "staging" {
		t.Fatalf("expected password masked, got %#v", masked)
	}
	if len(ordered) != 2 || ordered[0] != "staging" || ordered[1] != "secret123" {
		t.Fatalf("unexpected ordered args: %#v", ordered)
	}
	if ParamEnvKey("api-token") != "SB_PARAM_API_TOKEN" {
		t.Fatalf("unexpected env key %q", ParamEnvKey("api-token"))
	}

	if _, _, _, err := ResolveParams(sp.Params, map[string]string{"env": "invalid"}); err == nil {
		t.Fatal("expected invalid select option error")
	}
	badEvery := sp.Clone()
	badEvery.Every = 5
	if err := badEvery.Validate(); err == nil {
		t.Fatal("expected error for every=5 < MinEvery")
	}
}

func TestExtractAllJSONRegexLineRaw(t *testing.T) {
	jsonOut := []byte(`{"status":"running","progress":72.5,"workers":[{"name":"w0","ms":12},{"name":"w1","ms":19}]}`)
	rules := []ExtractRule{
		{Name: "status", Kind: KindJSON, Path: "status", Type: "string"},
		{Name: "progress", Kind: KindJSON, Path: "progress", Type: "number"},
		{Name: "firstWorker", Kind: KindJSON, Path: "workers.0.name", Type: "string"},
		{Name: "latencies", Kind: KindJSON, Path: "workers.#.ms", Type: "number"},
		{Name: "rawLine", Kind: KindLine, Line: 1, Type: "string"},
		{Name: "raw", Kind: KindRaw},
	}
	parsed, extracted, err := ExtractAll(jsonOut, rules)
	if err != nil {
		t.Fatalf("ExtractAll json: %v", err)
	}
	if parsed == nil || extracted["status"] != "running" || extracted["progress"] != 72.5 || extracted["firstWorker"] != "w0" {
		t.Fatalf("unexpected extracted json fields: %#v", extracted)
	}
	lats, ok := extracted["latencies"].([]float64)
	if !ok || len(lats) != 2 || lats[0] != 12 || lats[1] != 19 {
		t.Fatalf("unexpected fanout numbers: %#v", extracted["latencies"])
	}

	macPing := []byte(`PING 223.5.5.5 (223.5.5.5): 56 data bytes
64 bytes from 223.5.5.5: icmp_seq=0 ttl=115 time=11.421 ms

--- 223.5.5.5 ping statistics ---
3 packets transmitted, 3 packets received, 0.0% packet loss
round-trip min/avg/max/stddev = 10.100/11.421/13.200/1.280 ms
`)
	_, extMac, err := ExtractAll(macPing, BuiltinPingSpec().Extract)
	if err != nil {
		t.Fatalf("ExtractAll macPing: %v", err)
	}
	if extMac["resolvedIp"] != "223.5.5.5" || extMac["lossPct"] != 0.0 || extMac["avgMs"] != 11.421 || extMac["jitterMs"] != 1.28 {
		t.Fatalf("unexpected macPing extract: %#v", extMac)
	}

	linuxPing := []byte(`PING github.com (20.205.243.166) 56(84) bytes of data.
64 bytes from 20.205.243.166: icmp_seq=1 ttl=108 time=42.8 ms

--- github.com ping statistics ---
3 packets transmitted, 2 received, 33.3333% packet loss, time 2003ms
rtt min/avg/max/mdev = 40.1/42.8/45.5/2.7 ms
`)
	_, extLinux, err := ExtractAll(linuxPing, BuiltinPingSpec().Extract)
	if err != nil {
		t.Fatalf("ExtractAll linuxPing: %v", err)
	}
	if extLinux["resolvedIp"] != "20.205.243.166" || extLinux["lossPct"] != 33.3333 || extLinux["avgMs"] != 42.8 || extLinux["jitterMs"] != 2.7 {
		t.Fatalf("unexpected linuxPing extract: %#v", extLinux)
	}

	lineTest := []byte("alpha\n\nbeta\n\ngamma\n")
	_, lineExt, err := ExtractAll(lineTest, []ExtractRule{
		{Name: "last", Kind: KindLine, Line: -1, Type: "string"},
		{Name: "first", Kind: KindLine, Line: 1, Type: "string"},
	})
	if err != nil || lineExt["last"] != "gamma" || lineExt["first"] != "alpha" {
		t.Fatalf("unexpected line extract: %#v %v", lineExt, err)
	}
}

func TestExecuteShellExecInjectionSafetyAndTimeout(t *testing.T) {
	sp := Spec{
		ID:      "safe-echo",
		Name:    "Safe Echo",
		Shell:   "sh",
		Command: `printf '{"q":"%s","arg1":"%s","count":%d}' "$SB_PARAM_QUERY" "$1" 2`,
		Params: []Param{
			{Key: "query", Type: "string", Required: true},
		},
		Extract: []ExtractRule{
			{Name: "q", Kind: KindJSON, Path: "q", Type: "string"},
			{Name: "count", Kind: KindJSON, Path: "count", Type: "number"},
		},
	}
	inj := "$(touch /tmp/should-not-run);`id`"
	rec := Execute(context.Background(), sp, map[string]string{"query": inj}, "manual", time.Now())
	if !rec.OK || rec.ExitCode != 0 {
		t.Fatalf("unexpected failed execution: %+v", rec)
	}
	if rec.Extracted["q"] != inj || rec.Extracted["count"] != float64(2) {
		t.Fatalf("unexpected extracted: %#v (stdout=%q)", rec.Extracted, rec.Stdout)
	}

	execSpec := Spec{
		ID:      "direct-exec",
		Name:    "Direct Exec",
		Shell:   "exec",
		Command: `printf "ok;123"`,
		Extract: []ExtractRule{
			{Name: "raw", Kind: KindRaw},
		},
	}
	rec2 := Execute(context.Background(), execSpec, nil, "manual", time.Now())
	if !rec2.OK || rec2.Extracted["raw"] != "ok;123" {
		t.Fatalf("direct exec failed: %+v", rec2)
	}

	timeoutSpec := Spec{
		ID:      "slow-cmd",
		Name:    "Slow",
		Shell:   "sh",
		Command: "sleep 3",
		Timeout: 1,
	}
	rec3 := Execute(context.Background(), timeoutSpec, nil, "manual", time.Now())
	if rec3.OK || !strings.Contains(rec3.Error, "超时") {
		t.Fatalf("expected timeout error, got %+v", rec3)
	}
}

func TestRunnerPermissionsHistoryAndSchedule(t *testing.T) {
	dir := t.TempDir()
	hist := NewHistoryStore(dir)
	allowCustom := false
	runner := NewRunner(hist, func(sp Spec) bool {
		return IsTrustedBuiltin(sp) || allowCustom
	})
	now := time.Date(2026, 11, 26, 19, 0, 0, 0, time.UTC)
	runner.Now = func() time.Time { return now }

	custom := Spec{
		ID:      "custom-echo",
		Name:    "Custom",
		Shell:   "sh",
		Command: `echo "v=$SB_PARAM_TAG"`,
		Every:   30,
		Params:  []Param{{Key: "tag", Type: "string", Default: "ok"}},
		Extract: []ExtractRule{{Name: "out", Kind: KindRaw}},
	}
	if _, err := runner.Run(context.Background(), custom, nil, "manual"); !errors.Is(err, ErrExecDisabled) {
		t.Fatalf("expected ErrExecDisabled when AllowExec=false, got %v", err)
	}

	// Modified builtin command is no longer trusted.
	tampered := BuiltinPingSpec()
	tampered.Command = "echo pwned"
	if IsTrustedBuiltin(tampered) {
		t.Fatal("tampered builtin must not be trusted")
	}
	if _, err := runner.Run(context.Background(), tampered, map[string]string{"host": "127.0.0.1"}, "manual"); !errors.Is(err, ErrExecDisabled) {
		t.Fatalf("tampered builtin should require AllowExec, got %v", err)
	}
	// Builtin ping rejects dangerous host parameters even when trusted.
	if err := ValidateBuiltinPingParams(map[string]string{"host": "1.1.1.1;rm -rf /", "count": "3"}); err == nil {
		t.Fatal("expected invalid builtin ping host error")
	}

	// Test run does not write history.
	allowCustom = true
	testRec, err := runner.Test(context.Background(), custom, map[string]string{"tag": "dry"})
	if err != nil || !testRec.OK || testRec.Extracted["out"] != "v=dry" {
		t.Fatalf("runner.Test unexpected: %+v %v", testRec, err)
	}
	if hist.Count(custom.ID) != 0 {
		t.Fatalf("runner.Test wrote history count=%d", hist.Count(custom.ID))
	}

	// Manual + scheduled runs write history and advance NextRun.
	due := runner.Due([]Spec{custom})
	if len(due) != 1 {
		t.Fatalf("expected 1 due spec, got %d", len(due))
	}
	var doneCount int
	runner.PollDue(context.Background(), []Spec{custom}, 2, func(_ Spec, _ Record) { doneCount++ })
	if doneCount != 1 {
		t.Fatalf("expected PollDue callback once, got %d", doneCount)
	}
	if len(runner.Due([]Spec{custom})) != 0 {
		t.Fatal("expected 0 due specs immediately after schedule run")
	}
	now = now.Add(31 * time.Second)
	if len(runner.Due([]Spec{custom})) != 1 {
		t.Fatal("expected spec due again after 31s")
	}

	custom.MaxHistory = 2
	for _, tag := range []string{"r1", "r2", "r3"} {
		if _, err := runner.Run(context.Background(), custom, map[string]string{"tag": tag}, "manual"); err != nil {
			t.Fatalf("Run(%s): %v", tag, err)
		}
	}
	recs, err := hist.Recent(custom.ID, 10)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(recs) != 2 || recs[0].Extracted["out"] != "v=r3" || recs[1].Extracted["out"] != "v=r2" {
		t.Fatalf("expected newest-first pruned records, got %+v", recs)
	}
	if err := hist.Clear(custom.ID); err != nil || hist.Count(custom.ID) != 0 {
		t.Fatalf("Clear failed: %v count=%d", err, hist.Count(custom.ID))
	}
}

func TestDisplayStyleValidationPresetsMergeAndResolve(t *testing.T) {
	// 1. Validation rejects invalid variants/animations/dotCount/speed.
	bad := &DisplayStyle{Variant: "sparkle"}
	if err := bad.NormalizeAndValidate(); err == nil {
		t.Fatal("expected invalid variant error")
	}
	badAnim := &DisplayStyle{Animation: "spin-wildly"}
	if err := badAnim.NormalizeAndValidate(); err == nil {
		t.Fatal("expected invalid animation error")
	}
	badCount := &DisplayStyle{DotCount: 25}
	if err := badCount.NormalizeAndValidate(); err == nil {
		t.Fatal("expected invalid dotCount error")
	}
	badSpeed := &DisplayStyle{Speed: 10}
	if err := badSpeed.NormalizeAndValidate(); err == nil {
		t.Fatal("expected invalid speed error")
	}

	// 2. Built-in net-ping stays trusted even when Display is customized.
	ping := BuiltinPingSpec()
	if !IsTrustedBuiltin(ping) || ping.Display == nil || ping.Display.Animation != AnimRipple {
		t.Fatalf("unexpected builtin ping spec/display: trusted=%v display=%+v", IsTrustedBuiltin(ping), ping.Display)
	}
	preset, ok := PresetDisplayStyle("dot-wave")
	if !ok {
		t.Fatal("expected dot-wave preset")
	}
	merged := MergeDisplayStyle(ping.Display, preset)
	ping.Display = merged
	if err := ping.Validate(); err != nil {
		t.Fatalf("customized ping display Validate failed: %v", err)
	}
	if !IsTrustedBuiltin(ping) {
		t.Fatal("customizing Display on net-ping should preserve IsTrustedBuiltin")
	}

	// 3. Clone isolates nested pointers and map entries.
	cp := ping.Clone()
	cp.Display.Animation = AnimBlink
	if ping.Display.Animation == AnimBlink {
		t.Fatal("Spec.Clone did not deep copy Display")
	}

	// 4. Threshold resolution matches numeric bounds and string Equals, while keeping secondary fields calm.
	pingOrigin := BuiltinPingSpec()
	fast := pingOrigin.Display.ResolveField("avgMs", 18.44, true)
	if fast.Color != "success" || fast.StatusLabel != "极速" || fast.Animation != AnimRipple || fast.FormattedValue != "18.4 ms" {
		t.Fatalf("unexpected fast avgMs resolve: %+v", fast)
	}
	slow := pingOrigin.Display.ResolveField("avgMs", 180.2, true)
	if slow.Color != "warning" || slow.StatusLabel != "偏高" || slow.Animation != AnimRipple {
		t.Fatalf("unexpected slow avgMs resolve: %+v", slow)
	}
	minRes := pingOrigin.Display.ResolveField("minMs", 12.0, true)
	if minRes.Animation != AnimNone || minRes.Variant != StyleVariantDefault || minRes.FormattedValue != "12.0 ms" {
		t.Fatalf("secondary field minMs should stay calm, got %+v", minRes)
	}
	loss := pingOrigin.Display.ResolveField("lossPct", 25, true)
	if loss.Color != "danger" || loss.Animation != AnimBlink || loss.FormattedValue != "25%" {
		t.Fatalf("unexpected lossPct resolve: %+v", loss)
	}

	// Updating top-level animation immediately propagates to PrimaryField ("avgMs").
	orbitPing := BuiltinPingSpec()
	orbitPing.Display.Animation = AnimOrbit
	orbitRes := orbitPing.Display.ResolveField("avgMs", 22.0, true)
	if orbitRes.Animation != AnimOrbit || orbitRes.Color != "success" {
		t.Fatalf("expected top-level AnimOrbit on avgMs, got %+v", orbitRes)
	}

	dec2 := 2
	strStyle := &DisplayStyle{
		Prefix:   "~",
		Decimals: &dec2,
		Thresholds: []StyleThreshold{
			{Equals: "up", Color: "success", Label: "OK", Animation: AnimPulse},
			{Equals: "down", Color: "danger", Label: "DOWN", Animation: AnimBlink},
		},
	}
	resStr := strStyle.ResolveField("state", "down", true)
	if resStr.Color != "danger" || resStr.StatusLabel != "DOWN" || resStr.Animation != AnimBlink {
		t.Fatalf("unexpected string threshold resolve: %+v", resStr)
	}

	// 5. ValueMap, Fallback, Format templates, and ResolveRecord ordering/resolution.
	customStyle := &DisplayStyle{
		PrimaryField: "avgMs",
		Variant:      StyleVariantDot,
		Animation:    AnimWave,
		Unit:         "ms",
		Format:       "{{value}}ms ({{status}}, loss={{lossPct}}%, target={{param.host}})",
		Fallback:     "N/A",
		Thresholds: []StyleThreshold{
			{Max: ptrFloat(40), Color: "success", Label: "Turbo"},
		},
		Fields: map[string]FieldStyle{
			"state": {
				ValueMap: map[string]string{"0": "Up", "1": "Down"},
				Fallback: "Offline",
			},
		},
	}
	rec := Record{
		CommandID: "net-ping",
		OK:        true,
		Params:    map[string]string{"host": "1.1.1.1"},
		Extracted: map[string]any{
			"avgMs":   18.4,
			"lossPct": 0.0,
			"state":   "0",
		},
	}
	rc := customStyle.ResolveRecord(rec)
	if rc.Primary == nil || rc.PrimaryField != "avgMs" {
		t.Fatalf("expected Primary avgMs, got %+v", rc)
	}
	wantFmt := "18.4ms (Turbo, loss=0%, target=1.1.1.1)"
	if rc.Primary.FormattedValue != wantFmt || rc.Primary.Value != "18.4" {
		t.Fatalf("unexpected template FormattedValue: got %q (value=%q), want %q", rc.Primary.FormattedValue, rc.Primary.Value, wantFmt)
	}
	if rc.Fields["state"].FormattedValue != "Up" {
		t.Fatalf("expected ValueMap 0->Up, got %+v", rc.Fields["state"])
	}
	if len(rc.Order) == 0 || rc.Order[0] != "avgMs" {
		t.Fatalf("expected PrimaryField first in Order, got %v", rc.Order)
	}
	rcEmpty := customStyle.ResolveRecord(Record{CommandID: "net-ping", OK: true})
	if rcEmpty.Primary == nil || rcEmpty.Primary.FormattedValue != "N/A" || rcEmpty.Fields["state"].FormattedValue != "Offline" {
		t.Fatalf("unexpected empty record fallback resolution: %+v", rcEmpty)
	}
}

func ptrFloat(v float64) *float64 { return &v }
