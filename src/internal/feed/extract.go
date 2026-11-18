package feed

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Numbers reads the numbers a rule points at in a response body. It fails when
// the rule finds none, so that a page that changed shows as an error rather
// than as a flat line of nothing.
func (e Extract) Numbers(body []byte) ([]float64, error) {
	switch e.Kind {
	case KindJSON:
		var v any
		if err := json.Unmarshal(body, &v); err != nil {
			return nil, fmt.Errorf("响应不是 JSON：%w", err)
		}
		out := walk(v, strings.Split(e.Path, "."))
		if len(out) == 0 {
			return nil, fmt.Errorf("路径 %s 里没有数字", e.Path)
		}
		return out, nil
	case KindRegex:
		re, err := regexp.Compile(e.Pattern)
		if err != nil {
			return nil, err
		}
		var out []float64
		for _, m := range re.FindAllStringSubmatch(string(body), -1) {
			text := m[0]
			if len(m) > 1 {
				text = m[1]
			}
			if n, ok := parseNumber(text); ok {
				out = append(out, n)
			}
		}
		if len(out) == 0 {
			return nil, errors.New("正则没有匹配到数字")
		}
		return out, nil
	}
	return nil, errors.New("未知的取数方式")
}

// walk follows path through v and collects the numbers at its end. A "#"
// fans out over an array.
func walk(v any, path []string) []float64 {
	if len(path) == 0 {
		if n, ok := toNumber(v); ok {
			return []float64{n}
		}
		return nil
	}
	key := strings.TrimSpace(path[0])
	if key == "#" {
		arr, ok := v.([]any)
		if !ok {
			return nil
		}
		var out []float64
		for _, item := range arr {
			out = append(out, walk(item, path[1:])...)
		}
		return out
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	child, ok := m[key]
	if !ok {
		return nil
	}
	return walk(child, path[1:])
}

func toNumber(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case string:
		return parseNumber(x)
	}
	return 0, false
}

// parseNumber reads 1,234.5 and " 7.02 " as numbers.
func parseNumber(s string) (float64, bool) {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	if s == "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}
