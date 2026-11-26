package cmdrun

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ExtractAll parses stdout as JSON if applicable and evaluates each ExtractRule.
// Rules that do not match an output are omitted rather than failing the whole run,
// so error runs still surface partial metrics (e.g. packet loss percentage).
func ExtractAll(stdout []byte, rules []ExtractRule) (parsedJSON any, extracted map[string]any, err error) {
	trimmed := bytes.TrimSpace(stdout)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		var v any
		if uerr := json.Unmarshal(trimmed, &v); uerr == nil {
			parsedJSON = v
		}
	}
	extracted = make(map[string]any, len(rules))
	text := string(stdout)

	for _, rule := range rules {
		switch rule.Kind {
		case KindJSON:
			if parsedJSON == nil {
				continue
			}
			val, ok := walkJSON(parsedJSON, strings.Split(rule.Path, "."))
			if !ok {
				continue
			}
			if coerced, ok := coerceValue(val, rule.Type); ok {
				extracted[rule.Name] = coerced
			}

		case KindRegex:
			re, rerr := regexp.Compile(rule.Pattern)
			if rerr != nil {
				return parsedJSON, extracted, fmt.Errorf("提取规则 %q 正则错误：%w", rule.Name, rerr)
			}
			m := re.FindStringSubmatch(text)
			if len(m) == 0 {
				continue
			}
			raw := m[0]
			if len(m) > 1 {
				raw = m[1]
			}
			if coerced, ok := coerceValue(raw, rule.Type); ok {
				extracted[rule.Name] = coerced
			}

		case KindLine:
			norm := strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
			var selected string
			var found bool
			if rule.Line > 0 {
				lines := strings.Split(norm, "\n")
				idx := rule.Line - 1
				if idx >= 0 && idx < len(lines) {
					selected = lines[idx]
					found = true
				}
			} else if rule.Line < 0 {
				var nonEmpty []string
				for _, ln := range strings.Split(norm, "\n") {
					if strings.TrimSpace(ln) != "" {
						nonEmpty = append(nonEmpty, ln)
					}
				}
				idx := len(nonEmpty) + rule.Line
				if idx >= 0 && idx < len(nonEmpty) {
					selected = nonEmpty[idx]
					found = true
				}
			}
			if !found {
				continue
			}
			if coerced, ok := coerceValue(strings.TrimSpace(selected), rule.Type); ok {
				extracted[rule.Name] = coerced
			}

		case KindRaw:
			if coerced, ok := coerceValue(strings.TrimSpace(text), rule.Type); ok {
				extracted[rule.Name] = coerced
			}
		}
	}
	return parsedJSON, extracted, nil
}

func walkJSON(v any, path []string) (any, bool) {
	if len(path) == 0 {
		if v == nil {
			return nil, false
		}
		return v, true
	}
	key := strings.TrimSpace(path[0])
	if key == "" {
		return walkJSON(v, path[1:])
	}
	if arr, ok := v.([]any); ok {
		if key == "#" {
			out := make([]any, 0, len(arr))
			for _, item := range arr {
				if sub, ok := walkJSON(item, path[1:]); ok {
					out = append(out, sub)
				}
			}
			if len(out) == 0 {
				return nil, false
			}
			return out, true
		}
		if idx, err := strconv.Atoi(key); err == nil && idx >= 0 && idx < len(arr) {
			return walkJSON(arr[idx], path[1:])
		}
		return nil, false
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, false
	}
	child, ok := m[key]
	if !ok {
		return nil, false
	}
	return walkJSON(child, path[1:])
}

func coerceValue(v any, targetType string) (any, bool) {
	if v == nil {
		return nil, false
	}
	switch strings.ToLower(strings.TrimSpace(targetType)) {
	case "":
		return v, true
	case "string":
		if s, ok := v.(string); ok {
			return s, true
		}
		if f, ok := v.(float64); ok {
			return strconv.FormatFloat(f, 'f', -1, 64), true
		}
		return fmt.Sprint(v), true
	case "number":
		if arr, ok := v.([]any); ok {
			nums := make([]float64, 0, len(arr))
			for _, item := range arr {
				if n, ok := toFloat(item); ok {
					nums = append(nums, n)
				}
			}
			if len(nums) == 0 {
				return nil, false
			}
			return nums, true
		}
		if n, ok := toFloat(v); ok {
			return n, true
		}
		return nil, false
	case "bool":
		switch x := v.(type) {
		case bool:
			return x, true
		case float64:
			return x != 0, true
		case string:
			switch strings.ToLower(strings.TrimSpace(x)) {
			case "true", "1", "yes", "on", "ok":
				return true, true
			case "false", "0", "no", "off":
				return false, true
			}
		}
		return nil, false
	default:
		return v, true
	}
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	case string:
		s := strings.ReplaceAll(strings.TrimSpace(x), ",", "")
		if s == "" {
			return 0, false
		}
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	}
	return 0, false
}
