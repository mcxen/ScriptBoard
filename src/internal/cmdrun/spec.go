// Package cmdrun executes host shell/CLI commands on desktop platforms, extracts
// structured fields (JSON, regex, lines, raw) from stdout, resolves/masks
// parameters safely via environment variables, and persists JSONL run history.
package cmdrun

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
)

const (
	MinEvery          = 10
	DefaultTimeout    = 15
	MaxTimeout        = 120
	DefaultMaxHistory = 200
	MaxStdoutBytes    = 64 << 10
	MaxStderrBytes    = 16 << 10
)

// Extract rule kinds.
const (
	KindJSON  = "json"
	KindRegex = "regex"
	KindLine  = "line"
	KindRaw   = "raw"
)

// Param defines an input parameter passed safely via environment variables and argv.
type Param struct {
	Key         string   `json:"key"`
	Label       string   `json:"label,omitempty"`
	Type        string   `json:"type,omitempty"` // "", "string", "select", "number", "password"
	Default     string   `json:"default,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Options     []string `json:"options,omitempty"`
	Required    bool     `json:"required,omitempty"`
}

// ExtractRule defines one named field to extract from command stdout.
type ExtractRule struct {
	Name    string `json:"name"`
	Kind    string `json:"kind"`              // "json", "regex", "line", "raw"
	Path    string `json:"path,omitempty"`    // dotted JSON path (supports array indices and # fan-out)
	Pattern string `json:"pattern,omitempty"` // Go regexp; capture group 1 if present else whole match
	Line    int    `json:"line,omitempty"`    // 1-based line from start, or negative from end (-1 = last non-empty)
	Type    string `json:"type,omitempty"`    // "", "string", "number", "bool"
}

// Spec configures a runnable host command with optional schedule, params, and extract rules.
type Spec struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Shell       string            `json:"shell,omitempty"` // "", "sh", "bash", "zsh", "pwsh", "psh", "exec", or custom path
	Command     string            `json:"command"`
	WorkDir     string            `json:"workDir,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Params      []Param           `json:"params,omitempty"`
	Extract     []ExtractRule     `json:"extract,omitempty"`
	Every       int               `json:"every,omitempty"`      // 0 = manual only, >= MinEvery = scheduled interval seconds
	Timeout     int               `json:"timeout,omitempty"`    // seconds; 0 uses DefaultTimeout
	MaxHistory  int               `json:"maxHistory,omitempty"` // max persisted records; 0 uses DefaultMaxHistory
	Display     *DisplayStyle     `json:"display,omitempty"`
	Builtin     bool              `json:"builtin,omitempty"`
}

// Record stores one command execution result and its extracted values.
type Record struct {
	ID         string            `json:"id"`
	CommandID  string            `json:"commandId"`
	T          int64             `json:"t"` // Unix milliseconds
	Trigger    string            `json:"trigger"`
	Params     map[string]string `json:"params,omitempty"` // passwords masked as "***"
	OK         bool              `json:"ok"`
	ExitCode   int               `json:"exitCode"`
	DurationMs int64             `json:"durationMs"`
	Stdout     string            `json:"stdout,omitempty"`
	Stderr     string            `json:"stderr,omitempty"`
	JSON       any               `json:"json,omitempty"`
	Extracted  map[string]any    `json:"extracted,omitempty"`
	Error      string            `json:"error,omitempty"`
}

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,39}$`)

// ValidID reports whether id is a valid lowercase command identifier.
func ValidID(id string) bool { return idPattern.MatchString(id) }

// Clone returns a deep copy of Spec.
func (s Spec) Clone() Spec {
	out := s
	if s.Env != nil {
		out.Env = make(map[string]string, len(s.Env))
		for k, v := range s.Env {
			out.Env[k] = v
		}
	}
	if s.Params != nil {
		out.Params = make([]Param, len(s.Params))
		for i, p := range s.Params {
			cp := p
			if p.Options != nil {
				cp.Options = append([]string(nil), p.Options...)
			}
			out.Params[i] = cp
		}
	}
	if s.Extract != nil {
		out.Extract = append([]ExtractRule(nil), s.Extract...)
	}
	out.Display = s.Display.Clone()
	return out
}

// TimeoutDuration returns the configured execution timeout clamped to [1, MaxTimeout] seconds.
func (s Spec) TimeoutDuration() time.Duration {
	sec := s.Timeout
	if sec <= 0 {
		sec = DefaultTimeout
	}
	if sec > MaxTimeout {
		sec = MaxTimeout
	}
	return time.Duration(sec) * time.Second
}

// MaxHistoryOrDefault returns the history retention limit.
func (s Spec) MaxHistoryOrDefault() int {
	if s.MaxHistory <= 0 {
		return DefaultMaxHistory
	}
	return s.MaxHistory
}

// Validate checks Spec fields before persistence or execution.
func (s Spec) Validate() error {
	if !ValidID(s.ID) {
		return fmt.Errorf("命令 id 只能用小写字母、数字和连字符，2 到 40 个字符（当前 %q）", s.ID)
	}
	if strings.TrimSpace(s.Name) == "" {
		return errors.New("命令需要一个名称")
	}
	if strings.TrimSpace(s.Command) == "" {
		return errors.New("执行命令不能为空")
	}
	if strings.EqualFold(strings.TrimSpace(s.Shell), "exec") {
		args, err := splitArgs(s.Command)
		if err != nil {
			return fmt.Errorf("exec 命令解析失败：%w", err)
		}
		if len(args) == 0 {
			return errors.New("exec 命令不能为空")
		}
	}
	if s.Every < 0 || (s.Every > 0 && s.Every < MinEvery) {
		return fmt.Errorf("定时间隔需为 0（仅手动）或不少于 %d 秒", MinEvery)
	}
	if s.Timeout < 0 {
		return errors.New("超时时间不能为负数")
	}
	seenParams := map[string]bool{}
	for _, p := range s.Params {
		k := strings.TrimSpace(p.Key)
		if k == "" {
			return errors.New("参数 key 不能为空")
		}
		if seenParams[k] {
			return fmt.Errorf("重复的参数 key %q", k)
		}
		seenParams[k] = true
		switch strings.ToLower(strings.TrimSpace(p.Type)) {
		case "", "string", "select", "number", "password":
		default:
			return fmt.Errorf("不支持的参数类型 %q", p.Type)
		}
		if strings.EqualFold(strings.TrimSpace(p.Type), "select") && len(p.Options) == 0 {
			return fmt.Errorf("下拉参数 %q 至少需要一个选项", k)
		}
	}
	seenExtract := map[string]bool{}
	for _, r := range s.Extract {
		name := strings.TrimSpace(r.Name)
		if name == "" {
			return errors.New("提取字段名不能为空")
		}
		if seenExtract[name] {
			return fmt.Errorf("重复的提取字段 %q", name)
		}
		seenExtract[name] = true
		switch r.Kind {
		case KindJSON:
			if strings.TrimSpace(r.Path) == "" {
				return fmt.Errorf("提取规则 %q 的 JSON 路径不能为空", name)
			}
		case KindRegex:
			if strings.TrimSpace(r.Pattern) == "" {
				return fmt.Errorf("提取规则 %q 的正则表达式不能为空", name)
			}
			if _, err := regexp.Compile(r.Pattern); err != nil {
				return fmt.Errorf("提取规则 %q 正则有误：%w", name, err)
			}
		case KindLine:
			if r.Line == 0 {
				return fmt.Errorf("提取规则 %q 的行号不能为 0", name)
			}
		case KindRaw:
		default:
			return fmt.Errorf("提取规则 %q 的方式无效：%q（可用 json / regex / line / raw）", name, r.Kind)
		}
		switch strings.ToLower(strings.TrimSpace(r.Type)) {
		case "", "string", "number", "bool":
		default:
			return fmt.Errorf("提取规则 %q 的类型无效：%q", name, r.Type)
		}
	}
	if s.Display != nil {
		if err := s.Display.NormalizeAndValidate(); err != nil {
			return err
		}
	}
	return nil
}

// ParamEnvKey normalizes a parameter key to a safe SB_PARAM_<KEY> environment variable name.
func ParamEnvKey(key string) string {
	var b strings.Builder
	prevUnderscore := false
	for _, r := range key {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToUpper(r))
			prevUnderscore = false
			continue
		}
		if b.Len() > 0 && !prevUnderscore {
			b.WriteByte('_')
			prevUnderscore = true
		}
	}
	s := strings.Trim(b.String(), "_")
	if s == "" {
		s = "ARG"
	}
	return "SB_PARAM_" + s
}

// ResolveParams merges overrides with declared Param defaults, validates required fields,
// produces a password-masked copy for history persistence, and returns ordered positional args.
func ResolveParams(params []Param, overrides map[string]string) (resolved map[string]string, masked map[string]string, ordered []string, err error) {
	resolved = make(map[string]string, len(params)+len(overrides))
	masked = make(map[string]string, len(params)+len(overrides))
	ordered = make([]string, 0, len(params))
	declared := make(map[string]bool, len(params))

	for _, p := range params {
		declared[p.Key] = true
		val := p.Default
		if overrides != nil {
			if ov, ok := overrides[p.Key]; ok {
				val = ov
			}
		}
		if p.Type == "select" && len(p.Options) > 0 && val != "" {
			matched := false
			for _, opt := range p.Options {
				if opt == val {
					matched = true
					break
				}
			}
			if !matched {
				return nil, nil, nil, fmt.Errorf("参数 %q 的值 %q 不在可选项内", p.Key, val)
			}
		}
		if p.Required && strings.TrimSpace(val) == "" {
			return nil, nil, nil, fmt.Errorf("缺少必填参数 %q", p.Key)
		}
		resolved[p.Key] = val
		if strings.EqualFold(p.Type, "password") && val != "" {
			masked[p.Key] = "***"
		} else {
			masked[p.Key] = val
		}
		ordered = append(ordered, val)
	}
	for k, v := range overrides {
		if declared[k] || strings.TrimSpace(k) == "" {
			continue
		}
		resolved[k] = v
		masked[k] = v
	}
	return resolved, masked, ordered, nil
}

// MaskSpecSecrets returns a copy of Spec with sensitive environment values redacted.
func MaskSpecSecrets(s Spec) Spec {
	out := s.Clone()
	for k := range out.Env {
		if isSensitiveName(k) {
			out.Env[k] = "***"
		}
	}
	for i := range out.Params {
		if strings.EqualFold(out.Params[i].Type, "password") && out.Params[i].Default != "" {
			out.Params[i].Default = "***"
		}
	}
	return out
}

func isSensitiveName(k string) bool {
	u := strings.ToUpper(k)
	for _, marker := range []string{"TOKEN", "SECRET", "PASS", "KEY", "AUTH", "COOKIE"} {
		if strings.Contains(u, marker) {
			return true
		}
	}
	return false
}
