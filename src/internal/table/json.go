package table

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

// ColumnStats summarizes one column of a table across its rows.
type ColumnStats struct {
	Total    int     `json:"total"`
	NonEmpty int     `json:"nonEmpty"`
	Numeric  bool    `json:"numeric"`
	CountNum int     `json:"countNum"`
	Sum      float64 `json:"sum"`
	Avg      float64 `json:"avg"`
	Min      float64 `json:"min"`
	Max      float64 `json:"max"`
}

// SummarizeColumn computes non-empty count and numeric summary statistics
// (sum, average, min, max) for column col across rows.
func SummarizeColumn(rows [][]string, col int) ColumnStats {
	st := ColumnStats{Total: len(rows), Min: math.Inf(1), Max: math.Inf(-1)}
	for _, r := range rows {
		if col >= len(r) {
			continue
		}
		val := strings.TrimSpace(r[col])
		if val == "" {
			continue
		}
		st.NonEmpty++
		if v, ok := Number(val); ok && !math.IsNaN(v) && !math.IsInf(v, 0) {
			st.CountNum++
			st.Sum += v
			if v < st.Min {
				st.Min = v
			}
			if v > st.Max {
				st.Max = v
			}
		}
	}
	if st.NonEmpty > 0 && float64(st.CountNum) >= 0.6*float64(st.NonEmpty) {
		st.Numeric = true
		st.Avg = st.Sum / float64(st.CountNum)
	} else {
		st.Min, st.Max = 0, 0
	}
	return st
}

// ParseJSON reads either a JSON array of objects ([{"a":1,"b":"x"},...]) or an
// object with {columns:[...], rows:[[...]]} into rectangular table rows
// (header row first).
func ParseJSON(data []byte) ([][]string, error) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(string(data), bom))
	if trimmed == "" {
		return nil, errors.New("没有内容")
	}
	if strings.HasPrefix(trimmed, "{") {
		var obj struct {
			Columns []string   `json:"columns"`
			Rows    [][]string `json:"rows"`
		}
		if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
			return nil, err
		}
		if len(obj.Columns) == 0 && len(obj.Rows) == 0 {
			return nil, errors.New("JSON 里没有表格数据")
		}
		out := make([][]string, 0, len(obj.Rows)+1)
		if len(obj.Columns) > 0 {
			out = append(out, obj.Columns)
		}
		out = append(out, obj.Rows...)
		return Rect(out), nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var list []map[string]any
		if err := json.Unmarshal([]byte(trimmed), &list); err != nil {
			return nil, err
		}
		if len(list) == 0 {
			return nil, errors.New("JSON 数组为空")
		}
		keySet := map[string]bool{}
		for _, item := range list {
			for k := range item {
				keySet[k] = true
			}
		}
		cols := make([]string, 0, len(keySet))
		for k := range keySet {
			cols = append(cols, k)
		}
		sort.Strings(cols)
		out := make([][]string, 0, len(list)+1)
		out = append(out, cols)
		for _, item := range list {
			r := make([]string, len(cols))
			for i, k := range cols {
				if v, ok := item[k]; ok && v != nil {
					switch x := v.(type) {
					case string:
						r[i] = x
					case float64:
						r[i] = FormatNumber(x)
					default:
						r[i] = fmt.Sprint(x)
					}
				}
			}
			out = append(out, r)
		}
		return Rect(out), nil
	}
	return nil, errors.New("无法解析 JSON 表格")
}

// WriteJSON serializes a table as a pretty-printed JSON array of objects.
func WriteJSON(cols []string, rows [][]string) string {
	out := make([]map[string]string, 0, len(rows))
	for _, r := range rows {
		obj := make(map[string]string, len(cols))
		for j, col := range cols {
			if j < len(r) {
				obj[col] = r[j]
			} else {
				obj[col] = ""
			}
		}
		out = append(out, obj)
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	return string(b)
}
