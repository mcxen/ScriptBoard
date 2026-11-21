package mcpsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"scriptboard/src/internal/caps"
	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/iconcat"
	"scriptboard/src/internal/plugins"
	"scriptboard/src/internal/store"
	"scriptboard/src/internal/table"
)

func extraTools() []Tool {
	str := func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	boolProp := func(desc string) map[string]any { return map[string]any{"type": "boolean", "description": desc} }
	intProp := func(desc string) map[string]any { return map[string]any{"type": "integer", "description": desc} }
	obj := func(props map[string]any, required ...string) map[string]any {
		return map[string]any{"type": "object", "properties": props, "required": required}
	}
	return []Tool{
		{
			Name: "widget_catalog",
			Description: "List all built-in ScriptBoard widget/component types with capabilities, " +
				"swappable view targets, icons, and schema hints, generated from the host registry.",
			InputSchema: obj(map[string]any{
				"category": str("optional category filter: plot, progress, data, time, layout"),
				"type":     str("optional single widget type key, e.g. progressBar or table"),
			}),
		},
		{
			Name: "script_run",
			Description: "Compile and execute a script in a sandbox with optional mockSources, vars, dims and lang, " +
				"returning its rendered Result, extracted tabular view, swappable component types, and validation warnings.",
			InputSchema: obj(map[string]any{
				"source":      str("the ES module script source"),
				"lang":        str("zh or en (default zh)"),
				"vars":        map[string]any{"type": "object", "description": "optional map of variable key -> string value"},
				"dims":        map[string]any{"type": "object", "description": "optional map of dim key -> number value"},
				"mockSources": map[string]any{"type": "object", "description": "optional map of source name -> JSON value for ctx.sources"},
			}, "source"),
		},
		{
			Name:        "table_list",
			Description: "List all data-table plugins stored in ScriptBoard, with column counts, row counts, and home visibility.",
			InputSchema: obj(map[string]any{}),
		},
		{
			Name:        "table_get",
			Description: "Read one data-table plugin's columns, rows, and numeric column summaries (in JSON or CSV).",
			InputSchema: obj(map[string]any{
				"id":     str("table plugin id"),
				"format": str("json (default) or csv"),
				"limit":  intProp("optional maximum rows to return"),
			}, "id"),
		},
		{
			Name:        "table_save",
			Description: "Create or replace a data-table plugin (columns + rows) in ScriptBoard's data store.",
			InputSchema: obj(map[string]any{
				"id":      str("table id (lowercase letters, digits, hyphens)"),
				"name":    str("display name"),
				"home":    boolProp("show as a card on the home page"),
				"columns": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "column titles"},
				"rows":    map[string]any{"type": "array", "items": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "description": "table rows"},
			}, "id", "name", "columns", "rows"),
		},
		{
			Name:        "table_append",
			Description: "Append one or more rows to an existing data-table plugin.",
			InputSchema: obj(map[string]any{
				"id":   str("existing table plugin id"),
				"rows": map[string]any{"type": "array", "items": map[string]any{"type": "array", "items": map[string]any{"type": "string"}}, "description": "rows to append"},
			}, "id", "rows"),
		},
		{
			Name:        "tray_list",
			Description: "List cards currently pinned to the system Tray / Menu Bar and all scripts that support menubar: true.",
			InputSchema: obj(map[string]any{}),
		},
		{
			Name:        "tray_set",
			Description: "Pin or unpin a menubar-capable card in the system Tray / Menu Bar.",
			InputSchema: obj(map[string]any{
				"id":      str("script or plugin id (use plugin:<id> for local plugins)"),
				"enabled": boolProp("true to pin in menu bar, false to remove"),
			}, "id", "enabled"),
		},
		{
			Name:        "app_info",
			Description: "Return a machine-readable summary of ScriptBoard host capabilities, supported widgets, tray actions, icons, and store counts.",
			InputSchema: obj(map[string]any{}),
		},
	}
}

func (s Server) callExtra(name string, a map[string]any) (string, bool, error) {
	str := func(key string) string {
		v, _ := a[key].(string)
		return v
	}
	switch name {
	case "widget_catalog":
		cat := strings.TrimSpace(str("category"))
		typ := strings.TrimSpace(str("type"))
		var out []engine.WidgetSpec
		for _, sp := range engine.WidgetSpecs {
			if typ != "" && sp.Type != typ {
				continue
			}
			if cat != "" && string(sp.Category) != cat {
				continue
			}
			out = append(out, sp)
		}
		if len(out) == 0 {
			return "", true, fmt.Errorf("未找到匹配的组件类型（type=%q category=%q）", typ, cat)
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		return string(b), true, nil

	case "script_run":
		source := str("source")
		if strings.TrimSpace(source) == "" {
			return "", true, fmt.Errorf("source 不能为空")
		}
		lang := str("lang")
		if lang != "en" {
			lang = "zh"
		}
		sc, err := engine.CompileLang("run.js", source, lang)
		if err != nil {
			return "", true, err
		}
		vars := map[string]string{}
		if rawVars, ok := a["vars"].(map[string]any); ok {
			for k, v := range rawVars {
				vars[k] = fmt.Sprint(v)
			}
		}
		dims := map[string]float64{}
		if rawDims, ok := a["dims"].(map[string]any); ok {
			for k, v := range rawDims {
				if f, ok := v.(float64); ok {
					dims[k] = f
				}
			}
		}
		mockSrc := map[string]json.RawMessage{}
		if rawMock, ok := a["mockSources"].(map[string]any); ok {
			for k, v := range rawMock {
				mb, err := json.Marshal(v)
				if err == nil {
					mockSrc[k] = mb
				}
			}
		}
		res, err := sc.Run(context.Background(), engine.Input{
			Now:     time.Now(),
			Lang:    lang,
			OS:      caps.OS(),
			Kind:    caps.Platform(),
			Vars:    engine.ResolveVars(sc.Meta.Vars, vars),
			View:    dims,
			Sources: mockSrc,
		})
		if err != nil {
			return "", true, err
		}
		var warnings []string
		if res.Chart != nil && engine.UnknownChartType(res.Chart.Type) {
			warnings = append(warnings, "未知组件类型: "+res.Chart.Type)
		}
		warnings = append(warnings, engine.MenuBarProblems(sc.Meta, res)...)
		payload := map[string]any{
			"meta":       sc.Meta,
			"widgetType": engine.ResultWidgetType(res),
			"swappable":  engine.SwappableTypes(res),
			"result":     res,
			"table":      engine.ExtractTable(res, lang),
			"warnings":   warnings,
		}
		b, _ := json.MarshalIndent(payload, "", "  ")
		return string(b), true, nil

	case "table_list":
		st, err := store.Open(s.DataDir)
		if err != nil {
			return "", true, err
		}
		type item struct {
			ID      string   `json:"id"`
			Name    string   `json:"name"`
			Home    bool     `json:"home"`
			Origin  string   `json:"origin"`
			Metric  string   `json:"metric,omitempty"`
			Columns []string `json:"columns"`
			Rows    int      `json:"rows"`
		}
		var out []item
		for _, sc := range st.Scripts() {
			if !sc.IsTable() || sc.Table == nil {
				continue
			}
			rowCount := len(sc.Table.Rows)
			if sc.Table.Metric != "" {
				rowCount = len(st.Series(sc.Table.Metric))
			}
			out = append(out, item{
				ID: sc.ID, Name: sc.Name, Home: sc.Home, Origin: sc.Origin,
				Metric: sc.Table.Metric, Columns: sc.Table.Columns, Rows: rowCount,
			})
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		return string(b), true, nil

	case "table_get":
		st, err := store.Open(s.DataDir)
		if err != nil {
			return "", true, err
		}
		sc, ok := st.Script(str("id"))
		if !ok || !sc.IsTable() || sc.Table == nil {
			return "", true, fmt.Errorf("没有数据表 %q", str("id"))
		}
		cols := sc.Table.Columns
		rows := sc.Table.Rows
		if sc.Table.Metric != "" {
			pts := st.Series(sc.Table.Metric)
			if len(cols) == 0 {
				cols = []string{"时间", sc.Table.Metric}
			}
			rows = make([][]string, len(pts))
			for i, pt := range pts {
				rows[i] = []string{time.UnixMilli(pt.T).Format("2006-01-02 15:04"), table.FormatNumber(pt.V)}
			}
		}
		if lim, ok := a["limit"].(float64); ok && int(lim) > 0 && int(lim) < len(rows) {
			rows = rows[:int(lim)]
		}
		if strings.EqualFold(str("format"), "csv") {
			return table.WriteCSV(cols, rows), true, nil
		}
		summaries := make(map[string]table.ColumnStats, len(cols))
		for j, col := range cols {
			summaries[col] = table.SummarizeColumn(rows, j)
		}
		payload := map[string]any{
			"id":        sc.ID,
			"name":      sc.Name,
			"home":      sc.Home,
			"metric":    sc.Table.Metric,
			"columns":   cols,
			"rows":      rows,
			"rowCount":  len(rows),
			"summaries": summaries,
		}
		b, _ := json.MarshalIndent(payload, "", "  ")
		return string(b), true, nil

	case "table_save":
		id := strings.TrimSpace(str("id"))
		title := strings.TrimSpace(str("name"))
		if id == "" || title == "" {
			return "", true, fmt.Errorf("id 和 name 不能为空")
		}
		cols := toStringSlice(a["columns"])
		rows := toStringMatrix(a["rows"])
		if len(cols) == 0 {
			return "", true, fmt.Errorf("columns 不能为空")
		}
		if len(cols) > table.MaxCols {
			return "", true, fmt.Errorf("列数超过上限 %d", table.MaxCols)
		}
		if len(rows) > table.MaxRows {
			rows = rows[:table.MaxRows]
		}
		st, err := store.Open(s.DataDir)
		if err != nil {
			return "", true, err
		}
		if existing, ok := st.Script(id); ok && existing.IsTable() && existing.Table != nil && existing.Table.Metric != "" {
			return "", true, fmt.Errorf("内置绑定数据表 %q 不能直接覆盖结构", id)
		}
		home, _ := a["home"].(bool)
		sc := store.Script{
			ID:     id,
			Name:   title,
			Kind:   store.KindTable,
			Origin: store.OriginUser,
			Home:   home,
			Table:  &store.Table{Columns: cols, Rows: rows},
		}
		if err := st.SaveScript(sc); err != nil {
			return "", true, err
		}
		return fmt.Sprintf("已保存数据表 %s（%d 列，%d 行）", id, len(cols), len(rows)), true, nil

	case "table_append":
		id := strings.TrimSpace(str("id"))
		addRows := toStringMatrix(a["rows"])
		if id == "" || len(addRows) == 0 {
			return "", true, fmt.Errorf("id 和 rows 不能为空")
		}
		st, err := store.Open(s.DataDir)
		if err != nil {
			return "", true, err
		}
		sc, ok := st.Script(id)
		if !ok || !sc.IsTable() || sc.Table == nil {
			return "", true, fmt.Errorf("没有数据表 %q", id)
		}
		if sc.Table.Metric != "" {
			return "", true, fmt.Errorf("绑定指标表 %q 请通过记录接口写入", id)
		}
		colCount := len(sc.Table.Columns)
		for _, r := range addRows {
			row := make([]string, colCount)
			copy(row, r)
			sc.Table.Rows = append(sc.Table.Rows, row)
		}
		if len(sc.Table.Rows) > table.MaxRows {
			sc.Table.Rows = sc.Table.Rows[len(sc.Table.Rows)-table.MaxRows:]
		}
		if err := st.SaveScript(sc); err != nil {
			return "", true, err
		}
		return fmt.Sprintf("已向数据表 %s 追加 %d 行（共 %d 行）", id, len(addRows), len(sc.Table.Rows)), true, nil

	case "tray_list":
		st, err := store.Open(s.DataDir)
		if err != nil {
			return "", true, err
		}
		pinned := st.Settings().MenuBar
		type candidate struct {
			ID     string `json:"id"`
			Name   string `json:"name"`
			Pinned bool   `json:"pinned"`
		}
		var eligible []candidate
		for _, sc := range st.Scripts() {
			if sc.IsTable() {
				continue
			}
			if meta, err := plugins.CheckSource(sc.ID+".js", sc.Source); err == nil && meta.Menubar {
				eligible = append(eligible, candidate{ID: sc.ID, Name: sc.Name, Pinned: slices.Contains(pinned, sc.ID)})
			}
		}
		if plist, err := plugins.List(s.DataDir); err == nil {
			for _, p := range plist {
				pid := "plugin:" + p.Manifest.ID
				if meta, err := plugins.CheckSource(p.Manifest.ID+".js", p.Source); err == nil && meta.Menubar {
					eligible = append(eligible, candidate{ID: pid, Name: p.Manifest.Name, Pinned: slices.Contains(pinned, pid)})
				}
			}
		}
		payload := map[string]any{
			"pinned":      pinned,
			"eligible":    eligible,
			"actions":     engine.MenuActions,
			"paramPrefix": []string{"set_var:<key>=<value>", "set_dim:<key>=<value>"},
			"icons":       engine.MenuBarIcons,
		}
		b, _ := json.MarshalIndent(payload, "", "  ")
		return string(b), true, nil

	case "tray_set":
		id := strings.TrimSpace(str("id"))
		enabled, hasEnabled := a["enabled"].(bool)
		if id == "" || !hasEnabled {
			return "", true, fmt.Errorf("id 和 enabled 为必填项")
		}
		st, err := store.Open(s.DataDir)
		if err != nil {
			return "", true, err
		}
		set := st.Settings()
		if enabled {
			if !slices.Contains(set.MenuBar, id) {
				set.MenuBar = append(set.MenuBar, id)
			}
		} else {
			kept := set.MenuBar[:0:0]
			for _, x := range set.MenuBar {
				if x != id {
					kept = append(kept, x)
				}
			}
			set.MenuBar = kept
		}
		if err := st.SetSettings(set); err != nil {
			return "", true, err
		}
		if enabled {
			return fmt.Sprintf("已将 %s 加入菜单栏（当前共 %d 项）", id, len(set.MenuBar)), true, nil
		}
		return fmt.Sprintf("已将 %s 移出菜单栏（当前共 %d 项）", id, len(set.MenuBar)), true, nil

	case "app_info":
		st, _ := store.Open(s.DataDir)
		plist, _ := plugins.List(s.DataDir)
		tables, scripts, feeds, pinned := 0, 0, 0, 0
		if st != nil {
			for _, sc := range st.Scripts() {
				if sc.IsTable() {
					tables++
				} else {
					scripts++
				}
			}
			feeds = len(st.Feeds())
			pinned = len(st.Settings().MenuBar)
		}
		payload := map[string]any{
			"name":            "ScriptBoard",
			"protocolVersion": protocolVersion,
			"readOnly":        s.readOnly(),
			"platform":        map[string]string{"os": caps.OS(), "kind": caps.Platform()},
			"widgetTypes":     engine.ChartTypes,
			"trayActions":     append(append([]string(nil), engine.MenuActions...), "set_var:<k>=<v>", "set_dim:<k>=<v>"),
			"trayIcons":       engine.MenuBarIcons,
			"shippedIcons":    len(iconcat.Names()),
			"counts": map[string]int{
				"scripts":     scripts,
				"tables":      tables,
				"plugins":     len(plist),
				"feeds":       feeds,
				"trayPinned":  pinned,
				"widgetTypes": len(engine.WidgetSpecs),
			},
		}
		b, _ := json.MarshalIndent(payload, "", "  ")
		return string(b), true, nil
	}
	return "", false, nil
}

func toStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, x := range arr {
		out = append(out, fmt.Sprint(x))
	}
	return out
}

func toStringMatrix(v any) [][]string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([][]string, 0, len(arr))
	for _, row := range arr {
		out = append(out, toStringSlice(row))
	}
	return out
}
