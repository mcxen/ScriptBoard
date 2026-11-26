package mcpsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"scriptboard/src/internal/cmdrun"
	"scriptboard/src/internal/store"
)

func commandTools() []Tool {
	str := func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	boolProp := func(desc string) map[string]any { return map[string]any{"type": "boolean", "description": desc} }
	intProp := func(desc string) map[string]any { return map[string]any{"type": "integer", "description": desc} }
	obj := func(props map[string]any, required ...string) map[string]any {
		return map[string]any{"type": "object", "properties": props, "required": required}
	}
	return []Tool{
		{
			Name:        "cmd_list",
			Description: "List all configured host command tasks (macOS/Linux sh/bash/zsh/pwsh/psh/exec) with schedule interval, latest run status, and history count.",
			InputSchema: obj(map[string]any{
				"includeSecrets": boolProp("include password param defaults and sensitive env vars (default false)"),
			}),
		},
		{
			Name:        "cmd_get",
			Description: "Inspect one saved command task's script, parameters, extract rules, and latest execution summary.",
			InputSchema: obj(map[string]any{
				"id":             str("command id"),
				"includeSecrets": boolProp("include sensitive env/password defaults (default false)"),
			}, "id"),
		},
		{
			Name:        "cmd_save",
			Description: "Create or update a host command task with safe env-passed params, multi-type stdout extraction rules, and optional schedule.",
			InputSchema: obj(map[string]any{
				"id":          str("command id (lowercase letters, digits, hyphens, 2..40 chars)"),
				"name":        str("display name"),
				"description": str("optional one-line description"),
				"shell":       str("interpreter: sh (default), bash, zsh, pwsh, psh, exec, or custom executable path"),
				"command":     str("command script (reads parameters safely via SB_PARAM_<KEY> env vars or positional args)"),
				"workDir":     str("optional working directory"),
				"env":         map[string]any{"type": "object", "description": "optional static environment variables map"},
				"params":      map[string]any{"type": "array", "items": map[string]any{"type": "object"}, "description": "optional parameter definitions [{key, label, type, default, placeholder, options, required}]"},
				"extract":     map[string]any{"type": "array", "items": map[string]any{"type": "object"}, "description": "optional extraction rules [{name, kind (json|regex|line|raw), path, pattern, line, type (string|number|bool)}]"},
				"every":       intProp("optional schedule interval in seconds (0 = manual only, >= 10 = scheduled)"),
				"timeout":     intProp("optional execution timeout in seconds (1..120, default 15)"),
				"maxHistory":  intProp("optional max persisted history records (default 200)"),
				"display":     map[string]any{"type": "object", "description": "optional DisplayStyle for return values (primaryField, variant, animation, color, unit, prefix, suffix, format, valueMap, fallback, decimals, dotCount, dotSize, speed, thresholds, fields)"},
			}, "id", "name", "command"),
		},
		{
			Name:        "cmd_style_set",
			Description: "Customize or reset visual presentation and dot animations (pulse, ripple, dots, wave, orbit, blink, matrix) for a command's extracted return values.",
			InputSchema: obj(map[string]any{
				"id":           str("command id"),
				"preset":       str("optional style preset: default, dot-pulse, dot-ripple, dot-dots, dot-wave, dot-orbit, dot-blink, dot-matrix, badge, bar"),
				"display":      map[string]any{"type": "object", "description": "full or partial DisplayStyle object ({primaryField, variant, animation, color, unit, prefix, suffix, format, valueMap, fallback, decimals, dotCount, dotSize, speed, min, max, thresholds, fields})"},
				"primaryField": str("shorthand: primary extracted field key to highlight"),
				"variant":      str("shorthand variant: default, dot, dots, badge, matrix, bar"),
				"animation":    str("shorthand dot animation: none, pulse, ripple, dots, wave, orbit, blink, matrix"),
				"color":        str("shorthand color token (success, accent, warning, danger, cyan, purple) or hex/rgb"),
				"unit":         str("shorthand display unit (e.g. ms, %, MB)"),
				"prefix":       str("shorthand value prefix"),
				"suffix":       str("shorthand value suffix"),
				"format":       str("shorthand custom format template (supports {{value}}, {{raw}}, {{unit}}, {{label}}, {{status}}, {{field}}, {{<siblingField>}}, {{param.<key>}})"),
				"valueMap":     map[string]any{"type": "object", "description": "shorthand raw value -> custom display text mapping (e.g. {\"0\": \"Healthy\"})"},
				"fallback":     str("shorthand fallback text when value is missing (default \"--\")"),
				"decimals":     intProp("shorthand decimal places (0..6)"),
				"dotCount":     intProp("shorthand number of dots for dots/wave/orbit/matrix animations (1..12)"),
				"dotSize":      map[string]any{"type": "number", "description": "shorthand dot diameter in DIP (4..16)"},
				"speed":        map[string]any{"type": "number", "description": "shorthand animation speed multiplier (0.25..4.0)"},
				"min":          map[string]any{"type": "number", "description": "shorthand bar/matrix lower bound"},
				"max":          map[string]any{"type": "number", "description": "shorthand bar/matrix upper bound"},
				"thresholds":   map[string]any{"type": "array", "items": map[string]any{"type": "object"}, "description": "shorthand threshold rules [{min, max, equals, color, label, animation}]"},
				"fields":       map[string]any{"type": "object", "description": "shorthand map of extracted field name -> FieldStyle overrides"},
				"merge":        boolProp("merge into existing display style instead of replacing it (default true)"),
				"reset":        boolProp("reset style back to default (or shipped style for built-in net-ping)"),
			}, "id"),
		},
		{
			Name:        "cmd_delete",
			Description: "Delete a saved command task by id (and optionally delete its history file).",
			InputSchema: obj(map[string]any{
				"id":            str("command id"),
				"deleteHistory": boolProp("also remove persisted JSONL execution history (default false)"),
			}, "id"),
		},
		{
			Name:        "cmd_test",
			Description: "Dry-run a saved command id or inline shell/command/extract specification without saving or recording history.",
			InputSchema: obj(map[string]any{
				"id":      str("saved command id (optional if command is provided)"),
				"shell":   str("interpreter: sh (default), bash, zsh, pwsh, psh, exec, or custom path"),
				"command": str("inline command script"),
				"workDir": str("optional working directory"),
				"env":     map[string]any{"type": "object", "description": "optional environment variables map"},
				"params":  map[string]any{"type": "object", "description": "optional runtime parameters map"},
				"extract": map[string]any{"type": "array", "items": map[string]any{"type": "object"}, "description": "optional extraction rules"},
				"timeout": intProp("optional timeout seconds (1..120)"),
			}),
		},
		{
			Name:        "cmd_run",
			Description: "Execute a saved command task on demand with optional runtime parameters and persist the record to execution history.",
			InputSchema: obj(map[string]any{
				"id":     str("saved command id"),
				"params": map[string]any{"type": "object", "description": "optional runtime parameter overrides"},
			}, "id"),
		},
		{
			Name:        "cmd_history",
			Description: "Read persisted execution history records (newest-first) for a command task.",
			InputSchema: obj(map[string]any{
				"id":    str("command id"),
				"limit": intProp("max records to return (default 20, max 200)"),
			}, "id"),
		},
		{
			Name:        "cmd_history_clear",
			Description: "Clear all persisted execution history records for a command task.",
			InputSchema: obj(map[string]any{
				"id": str("command id"),
			}, "id"),
		},
	}
}

func (s Server) commandHistoryStore() *cmdrun.HistoryStore {
	return cmdrun.NewHistoryStore(filepath.Join(s.DataDir, "commands"))
}

func (s Server) callCommand(name string, a map[string]any) (string, bool, error) {
	str := func(key string) string {
		v, _ := a[key].(string)
		return v
	}
	allowFn := func(st *store.Store) func(cmdrun.Spec) bool {
		return func(sp cmdrun.Spec) bool {
			return cmdrun.IsTrustedBuiltin(sp) || (st != nil && st.Settings().AllowExec)
		}
	}

	switch name {
	case "cmd_list":
		st, err := store.Open(s.DataDir)
		if err != nil {
			return "", true, err
		}
		inc, _ := a["includeSecrets"].(bool)
		hist := s.commandHistoryStore()
		type cmdSummary struct {
			Spec         cmdrun.Spec    `json:"spec"`
			HistoryCount int            `json:"historyCount"`
			Latest       *cmdrun.Record `json:"latest,omitempty"`
		}
		cmds := st.Commands()
		out := make([]cmdSummary, 0, len(cmds))
		for _, sp := range cmds {
			viewSp := sp
			if !inc {
				viewSp = cmdrun.MaskSpecSecrets(sp)
			}
			item := cmdSummary{
				Spec:         viewSp,
				HistoryCount: hist.Count(sp.ID),
			}
			if latest, ok, _ := hist.Latest(sp.ID); ok {
				item.Latest = &latest
			}
			out = append(out, item)
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		return string(b), true, nil

	case "cmd_get":
		id := strings.TrimSpace(str("id"))
		if id == "" {
			return "", true, fmt.Errorf("id 不能为空")
		}
		st, err := store.Open(s.DataDir)
		if err != nil {
			return "", true, err
		}
		sp, ok := st.Command(id)
		if !ok {
			return "", true, fmt.Errorf("没有命令 %q", id)
		}
		if inc, _ := a["includeSecrets"].(bool); !inc {
			sp = cmdrun.MaskSpecSecrets(sp)
		}
		hist := s.commandHistoryStore()
		var latestPtr *cmdrun.Record
		if latest, ok, _ := hist.Latest(id); ok {
			latestPtr = &latest
		}
		payload := map[string]any{
			"spec":         sp,
			"historyCount": hist.Count(id),
			"latest":       latestPtr,
		}
		b, _ := json.MarshalIndent(payload, "", "  ")
		return string(b), true, nil

	case "cmd_save":
		sp, err := parseCommandSpecArgs(a)
		if err != nil {
			return "", true, err
		}
		st, err := store.Open(s.DataDir)
		if err != nil {
			return "", true, err
		}
		if old, exists := st.Command(sp.ID); exists {
			if _, hasDisplay := a["display"]; !hasDisplay && old.Display != nil {
				sp.Display = old.Display.Clone()
			}
			if old.Builtin && sp.Shell == old.Shell && sp.Command == old.Command && sp.WorkDir == old.WorkDir && len(sp.Env) == 0 {
				sp.Builtin = true
			}
		}
		if err := sp.Validate(); err != nil {
			return "", true, err
		}
		if err := st.SaveCommand(sp); err != nil {
			return "", true, err
		}
		s.notifyChange()
		return fmt.Sprintf("已保存命令 %s（%s，shell=%s，every=%ds，params=%d，extract=%d）",
			sp.ID, sp.Name, orStr(sp.Shell, "sh"), sp.Every, len(sp.Params), len(sp.Extract)), true, nil

	case "cmd_style_set":
		id := strings.TrimSpace(str("id"))
		if id == "" {
			return "", true, fmt.Errorf("id 不能为空")
		}
		st, err := store.Open(s.DataDir)
		if err != nil {
			return "", true, err
		}
		sp, ok := st.Command(id)
		if !ok {
			return "", true, fmt.Errorf("没有命令 %q", id)
		}
		reset, _ := a["reset"].(bool)
		if reset {
			if cmdrun.IsTrustedBuiltin(sp) && sp.ID == cmdrun.BuiltinPingID {
				sp.Display = cmdrun.BuiltinPingSpec().Display.Clone()
			} else {
				sp.Display = nil
			}
		} else {
			var patch *cmdrun.DisplayStyle
			if preset := strings.TrimSpace(str("preset")); preset != "" {
				pStyle, exists := cmdrun.PresetDisplayStyle(preset)
				if !exists {
					return "", true, fmt.Errorf("未知样式预设 %q（可选 default, dot-pulse, dot-ripple, dot-dots, dot-wave, dot-orbit, dot-blink, dot-matrix, badge, bar）", preset)
				}
				patch = pStyle
			}
			explicitPatch, err := parseDisplayStyleArgs(a)
			if err != nil {
				return "", true, err
			}
			if patch != nil || explicitPatch != nil {
				patch = cmdrun.MergeDisplayStyle(patch, explicitPatch)
			}
			if patch == nil {
				return "", true, fmt.Errorf("请提供 preset、display 或样式参数（variant / animation / color / unit / fields / thresholds）")
			}
			merge := true
			if m, has := a["merge"].(bool); has {
				merge = m
			}
			if merge {
				base := sp.Display.Clone()
				if base != nil && base.Fields != nil {
					targetField := patch.PrimaryField
					if targetField == "" {
						targetField = base.PrimaryField
					}
					_, hasExplicitField := patch.Fields[targetField]
					if targetField != "" && !hasExplicitField {
						if fs, exists := base.Fields[targetField]; exists {
							if patch.Animation != "" || patch.Variant != "" {
								fs.Animation = ""
								fs.Variant = ""
								for i := range fs.Thresholds {
									fs.Thresholds[i].Animation = ""
								}
							}
							if patch.Color != "" && len(patch.Thresholds) == 0 {
								fs.Color = ""
								for i := range fs.Thresholds {
									fs.Thresholds[i].Color = ""
								}
							}
							if patch.Format != "" {
								fs.Format = ""
							}
							if patch.ValueMap != nil {
								fs.ValueMap = nil
							}
							if patch.Fallback != "" {
								fs.Fallback = ""
							}
							base.Fields[targetField] = fs
						}
					}
				}
				sp.Display = cmdrun.MergeDisplayStyle(base, patch)
			} else {
				sp.Display = patch.Clone()
			}
		}
		if err := sp.Validate(); err != nil {
			return "", true, err
		}
		if err := st.SaveCommand(sp); err != nil {
			return "", true, err
		}
		s.notifyChange()
		outPayload := map[string]any{
			"id":      sp.ID,
			"name":    sp.Name,
			"builtin": cmdrun.IsTrustedBuiltin(sp),
			"display": sp.Display,
		}
		b, _ := json.MarshalIndent(outPayload, "", "  ")
		return string(b), true, nil

	case "cmd_delete":
		id := strings.TrimSpace(str("id"))
		if id == "" {
			return "", true, fmt.Errorf("id 不能为空")
		}
		st, err := store.Open(s.DataDir)
		if err != nil {
			return "", true, err
		}
		if _, ok := st.Command(id); !ok {
			return "", true, fmt.Errorf("没有命令 %q", id)
		}
		if err := st.DeleteCommand(id); err != nil {
			return "", true, err
		}
		if delHist, _ := a["deleteHistory"].(bool); delHist {
			if err := s.commandHistoryStore().Delete(id); err != nil {
				return "", true, err
			}
		}
		s.notifyChange()
		return "已删除命令 " + id, true, nil

	case "cmd_test":
		st, _ := store.Open(s.DataDir)
		var sp cmdrun.Spec
		if id := strings.TrimSpace(str("id")); id != "" {
			if st == nil {
				return "", true, fmt.Errorf("无法读取存储")
			}
			saved, ok := st.Command(id)
			if !ok {
				return "", true, fmt.Errorf("没有命令 %q", id)
			}
			sp = saved
		} else {
			inline, err := parseInlineCommandArgs(a)
			if err != nil {
				return "", true, err
			}
			sp = inline
		}
		params := toStringMap(a["params"])
		runner := cmdrun.NewRunner(nil, allowFn(st))
		rec, err := runner.Test(context.Background(), sp, params)
		if err != nil {
			return "", true, err
		}
		b, _ := json.MarshalIndent(rec, "", "  ")
		return string(b), true, nil

	case "cmd_run":
		id := strings.TrimSpace(str("id"))
		if id == "" {
			return "", true, fmt.Errorf("id 不能为空")
		}
		st, err := store.Open(s.DataDir)
		if err != nil {
			return "", true, err
		}
		sp, ok := st.Command(id)
		if !ok {
			return "", true, fmt.Errorf("没有命令 %q", id)
		}
		params := toStringMap(a["params"])
		hist := s.commandHistoryStore()
		runner := cmdrun.NewRunner(hist, allowFn(st))
		rec, err := runner.Run(context.Background(), sp, params, "mcp")
		if err != nil {
			return "", true, err
		}
		s.notifyChange()
		b, _ := json.MarshalIndent(rec, "", "  ")
		return string(b), true, nil

	case "cmd_history":
		id := strings.TrimSpace(str("id"))
		if id == "" {
			return "", true, fmt.Errorf("id 不能为空")
		}
		limit := 20
		if lv, ok := a["limit"].(float64); ok && int(lv) > 0 {
			limit = int(lv)
		}
		if limit > 200 {
			limit = 200
		}
		hist := s.commandHistoryStore()
		recs, err := hist.Recent(id, limit)
		if err != nil {
			return "", true, err
		}
		if recs == nil {
			recs = []cmdrun.Record{}
		}
		payload := map[string]any{
			"id":      id,
			"count":   len(recs),
			"records": recs,
		}
		b, _ := json.MarshalIndent(payload, "", "  ")
		return string(b), true, nil

	case "cmd_history_clear":
		id := strings.TrimSpace(str("id"))
		if id == "" {
			return "", true, fmt.Errorf("id 不能为空")
		}
		hist := s.commandHistoryStore()
		if err := hist.Clear(id); err != nil {
			return "", true, err
		}
		s.notifyChange()
		return "已清空命令历史 " + id, true, nil
	}
	return "", false, nil
}

func parseCommandSpecArgs(a map[string]any) (cmdrun.Spec, error) {
	str := func(key string) string {
		v, _ := a[key].(string)
		return v
	}
	shell := strings.TrimSpace(str("shell"))
	if shell == "" {
		shell = "sh"
	}
	sp := cmdrun.Spec{
		ID:          strings.TrimSpace(str("id")),
		Name:        strings.TrimSpace(str("name")),
		Description: strings.TrimSpace(str("description")),
		Shell:       shell,
		Command:     str("command"),
		WorkDir:     strings.TrimSpace(str("workDir")),
		Env:         toStringMap(a["env"]),
	}
	if rawParams, exists := a["params"]; exists && rawParams != nil {
		b, err := json.Marshal(rawParams)
		if err != nil {
			return cmdrun.Spec{}, fmt.Errorf("params 格式错误：%w", err)
		}
		if err := json.Unmarshal(b, &sp.Params); err != nil {
			return cmdrun.Spec{}, fmt.Errorf("params 解析失败：%w", err)
		}
	}
	if rawExtract, exists := a["extract"]; exists && rawExtract != nil {
		b, err := json.Marshal(rawExtract)
		if err != nil {
			return cmdrun.Spec{}, fmt.Errorf("extract 格式错误：%w", err)
		}
		if err := json.Unmarshal(b, &sp.Extract); err != nil {
			return cmdrun.Spec{}, fmt.Errorf("extract 解析失败：%w", err)
		}
	}
	if ev, ok := a["every"].(float64); ok {
		sp.Every = int(ev)
	}
	if to, ok := a["timeout"].(float64); ok {
		sp.Timeout = int(to)
	}
	if mh, ok := a["maxHistory"].(float64); ok {
		sp.MaxHistory = int(mh)
	}
	if _, hasDisplay := a["display"]; hasDisplay {
		ds, err := parseDisplayStyleMap(a["display"])
		if err != nil {
			return cmdrun.Spec{}, err
		}
		sp.Display = ds
	}
	return sp, nil
}

func parseDisplayStyleMap(raw any) (*cmdrun.DisplayStyle, error) {
	if raw == nil {
		return nil, nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("display 格式错误：%w", err)
	}
	var ds cmdrun.DisplayStyle
	if err := json.Unmarshal(b, &ds); err != nil {
		return nil, fmt.Errorf("display 解析失败：%w", err)
	}
	return &ds, nil
}

func parseDisplayStyleArgs(a map[string]any) (*cmdrun.DisplayStyle, error) {
	var base *cmdrun.DisplayStyle
	if raw, exists := a["display"]; exists && raw != nil {
		ds, err := parseDisplayStyleMap(raw)
		if err != nil {
			return nil, err
		}
		base = ds
	}
	shorthandKeys := []string{
		"primaryField", "variant", "animation", "color", "unit", "prefix", "suffix",
		"format", "valueMap", "fallback",
		"decimals", "dotCount", "dotSize", "speed", "min", "max", "thresholds", "fields",
	}
	sh := map[string]any{}
	for _, k := range shorthandKeys {
		if v, exists := a[k]; exists && v != nil {
			sh[k] = v
		}
	}
	if len(sh) > 0 {
		sp, err := parseDisplayStyleMap(sh)
		if err != nil {
			return nil, err
		}
		base = cmdrun.MergeDisplayStyle(base, sp)
	}
	return base, nil
}

func parseInlineCommandArgs(a map[string]any) (cmdrun.Spec, error) {
	str := func(key string) string {
		v, _ := a[key].(string)
		return v
	}
	shell := strings.TrimSpace(str("shell"))
	if shell == "" {
		shell = "sh"
	}
	sp := cmdrun.Spec{
		ID:      "inline-test",
		Name:    "inline-test",
		Shell:   shell,
		Command: str("command"),
		WorkDir: strings.TrimSpace(str("workDir")),
		Env:     toStringMap(a["env"]),
	}
	if rawExtract, exists := a["extract"]; exists && rawExtract != nil {
		b, err := json.Marshal(rawExtract)
		if err != nil {
			return cmdrun.Spec{}, fmt.Errorf("extract 格式错误：%w", err)
		}
		if err := json.Unmarshal(b, &sp.Extract); err != nil {
			return cmdrun.Spec{}, fmt.Errorf("extract 解析失败：%w", err)
		}
	}
	if to, ok := a["timeout"].(float64); ok {
		sp.Timeout = int(to)
	}
	return sp, nil
}

func toStringMap(v any) map[string]string {
	m, ok := v.(map[string]any)
	if !ok || len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, val := range m {
		if val != nil {
			out[k] = fmt.Sprint(val)
		}
	}
	return out
}
