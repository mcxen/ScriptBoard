package mcpsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"scriptboard/src/internal/bar"
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
	base := []Tool{
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
			Name: "tray_list",
			Description: "List all menubar-capable widgets (.bar components, scripts, and local plugins) with their " +
				"menu bar pinned state, click mode (panel/menu/open), target display (cursor/internal/external), " +
				"docked placements, inputs, and connected displays.",
			InputSchema: obj(map[string]any{}),
		},
		{
			Name: "tray_set",
			Description: "Configure a widget's Menu Bar / Tray and display placement: pin or unpin, " +
				"set click mode (panel/menu/open), target display (cursor/internal/external), dock locations " +
				"(leftSidebar/rightSidebar/bottomBar/shortcut), and .bar inputs.",
			InputSchema: obj(map[string]any{
				"id":           str("widget, script, or plugin id (use plugin-<id> for local plugins)"),
				"enabled":      boolProp("true to show in menu bar, false to remove"),
				"mode":         str("menu bar click mode: panel (floating popover), menu (dropdown menu), or open (open detail)"),
				"display":      str("target monitor for dropdown panel: cursor (follow click), internal (built-in), or external (external monitor)"),
				"leftSidebar":  boolProp("dock in window left sidebar"),
				"rightSidebar": boolProp("dock in window right sidebar"),
				"bottomBar":    boolProp("dock in window bottom bar"),
				"shortcut":     boolProp("enable Alt+W quick overlay shortcut"),
				"inputs":       map[string]any{"type": "object", "description": "optional .bar input values map"},
			}, "id"),
		},
		{
			Name: "menubar_show",
			Description: "Render and return the current Menu Bar display payload (title, tooltip, icon, " +
				"mode, target display, menu entries, panel VDOM/texts, logs, cookieCount, storageKeys) for one widget or all active menubar items.",
			InputSchema: obj(map[string]any{
				"id":   str("optional widget/script/plugin id; omit to inspect all active menubar items"),
				"lang": str("zh or en (default zh)"),
			}),
		},
		{
			Name:        "bar_save",
			Description: "Validate, compile-check, and save a .bar XML component bundle, and optionally configure its menu bar display and dock locations.",
			InputSchema: obj(map[string]any{
				"id":           str("optional widget id (defaults to <bar id=\"...\">)"),
				"source":       str("the .bar XML document"),
				"home":         boolProp("show as a card on the home page"),
				"enabled":      boolProp("pin in menu bar (default true)"),
				"mode":         str("panel (default) or menu"),
				"display":      str("cursor (default), internal, or external"),
				"leftSidebar":  boolProp("dock in window left sidebar"),
				"rightSidebar": boolProp("dock in window right sidebar"),
				"bottomBar":    boolProp("dock in window bottom bar"),
				"shortcut":     boolProp("enable Alt+W quick overlay shortcut"),
				"inputs":       map[string]any{"type": "object", "description": "optional input values map"},
			}, "source"),
		},
		{
			Name:        "app_info",
			Description: "Return a machine-readable summary of ScriptBoard host capabilities, supported widgets, .bar runtime APIs, cookie/storage support, tray actions, displays, icons, and store counts.",
			InputSchema: obj(map[string]any{}),
		},
	}
	base = append(base, barTools()...)
	base = append(base, feedTools()...)
	return base
}

func normalizePluginID(id string) string {
	if strings.HasPrefix(id, "plugin:") {
		return plugins.ScriptPrefix + strings.TrimPrefix(id, "plugin:")
	}
	return id
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
		s.notifyChange()
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
		s.notifyChange()
		return fmt.Sprintf("已向数据表 %s 追加 %d 行（共 %d 行）", id, len(addRows), len(sc.Table.Rows)), true, nil

	case "tray_list":
		st, err := store.Open(s.DataDir)
		if err != nil {
			return "", true, err
		}
		set := st.Settings()
		pinned := set.MenuBar
		type candidate struct {
			ID      string            `json:"id"`
			Name    string            `json:"name"`
			Kind    string            `json:"kind"`
			Enabled bool              `json:"enabled"`
			Pinned  bool              `json:"pinned"`
			Mode    string            `json:"mode"`
			Display string            `json:"display"`
			Docks   map[string]bool   `json:"docks"`
			Inputs  map[string]string `json:"inputs,omitempty"`
		}
		var eligible []candidate
		var allPinned []string
		allPinned = append(allPinned, pinned...)
		seen := map[string]bool{}
		for _, sc := range st.Scripts() {
			if sc.IsTable() {
				continue
			}
			seen[sc.ID] = true
			if sc.IsBar() {
				pl := mcpPlacementFor(sc.ID, true, set)
				sp, _ := bar.Parse(sc.Source)
				name := sp.Title
				if name == "" {
					name = sc.Name
				}
				disp := pl.Display
				if disp == "" {
					disp = "cursor"
				}
				pinBar := pl.Enabled && pl.MenuBar
				if pinBar && !slices.Contains(allPinned, sc.ID) {
					allPinned = append(allPinned, sc.ID)
				}
				var inputs map[string]string
				if len(sp.Inputs) > 0 {
					var saved map[string]string
					if set.ScriptVars != nil {
						saved = set.ScriptVars[sc.ID]
					}
					inputs = bar.MaskInputs(sp, bar.MergeInputs(sp, saved))
				}
				eligible = append(eligible, candidate{
					ID:      sc.ID,
					Name:    name,
					Kind:    "bar",
					Enabled: pl.Enabled,
					Pinned:  pinBar,
					Mode:    pl.MenuBarMode,
					Display: disp,
					Docks: map[string]bool{
						"leftSidebar":  pl.LeftSidebar,
						"rightSidebar": pl.RightSidebar,
						"bottomBar":    pl.BottomBar,
						"shortcut":     pl.Shortcut,
					},
					Inputs: inputs,
				})
				continue
			}
			if meta, err := plugins.CheckSource(sc.ID+".js", sc.Source); err == nil && meta.Menubar {
				pl := mcpPlacementFor(sc.ID, false, set)
				disp := pl.Display
				if disp == "" {
					disp = "cursor"
				}
				kind := "script"
				if strings.HasPrefix(sc.ID, plugins.ScriptPrefix) {
					kind = "plugin"
				}
				eligible = append(eligible, candidate{
					ID:      sc.ID,
					Name:    sc.Name,
					Kind:    kind,
					Enabled: pl.Enabled,
					Pinned:  slices.Contains(pinned, sc.ID),
					Mode:    pl.MenuBarMode,
					Display: disp,
					Docks: map[string]bool{
						"leftSidebar":  pl.LeftSidebar,
						"rightSidebar": pl.RightSidebar,
						"bottomBar":    pl.BottomBar,
						"shortcut":     pl.Shortcut,
					},
				})
			}
		}
		if plist, err := plugins.List(s.DataDir); err == nil {
			for _, p := range plist {
				pid := plugins.ScriptPrefix + p.Manifest.ID
				if seen[pid] {
					continue
				}
				if meta, err := plugins.CheckSource(p.Manifest.ID+".js", p.Source); err == nil && meta.Menubar {
					pl := mcpPlacementFor(pid, false, set)
					disp := pl.Display
					if disp == "" {
						disp = "cursor"
					}
					eligible = append(eligible, candidate{
						ID:      pid,
						Name:    p.Manifest.Name,
						Kind:    "plugin",
						Enabled: pl.Enabled,
						Pinned:  slices.Contains(pinned, pid),
						Mode:    pl.MenuBarMode,
						Display: disp,
						Docks: map[string]bool{
							"leftSidebar":  pl.LeftSidebar,
							"rightSidebar": pl.RightSidebar,
							"bottomBar":    pl.BottomBar,
							"shortcut":     pl.Shortcut,
						},
					})
				}
			}
		}
		var displays []DisplayInfo
		if s.Displays != nil {
			displays = s.Displays()
		}
		payload := map[string]any{
			"pinned":       allPinned,
			"eligible":     eligible,
			"displays":     displays,
			"displayModes": []string{"cursor", "internal", "external"},
			"menuBarModes": []string{"panel", "menu", "open"},
			"actions":      engine.MenuActions,
			"paramPrefix":  []string{"set_var:<key>=<value>", "set_dim:<key>=<value>"},
			"icons":        engine.MenuBarIcons,
		}
		b, _ := json.MarshalIndent(payload, "", "  ")
		return string(b), true, nil

	case "tray_set":
		id := normalizePluginID(strings.TrimSpace(str("id")))
		if id == "" {
			return "", true, fmt.Errorf("id 为必填项")
		}
		st, err := store.Open(s.DataDir)
		if err != nil {
			return "", true, err
		}
		sc, foundScript := st.Script(id)
		isBar := foundScript && sc.IsBar()
		set := st.Settings()
		if set.BarPlacements == nil {
			set.BarPlacements = map[string]store.BarPlacement{}
		}
		pl := mcpPlacementFor(id, isBar, set)

		enabled, hasEnabled := a["enabled"].(bool)
		if err := applyPlacementArgs(&pl, a); err != nil {
			return "", true, err
		}
		if hasEnabled {
			if isBar {
				pl.MenuBar = enabled
				if enabled {
					pl.Enabled = true
				}
			} else {
				pl.MenuBar = enabled
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
			}
		}
		set.BarPlacements[id] = pl

		if rawIn, ok := a["inputs"].(map[string]any); ok && len(rawIn) > 0 {
			if set.ScriptVars == nil {
				set.ScriptVars = map[string]map[string]string{}
			}
			if set.ScriptVars[id] == nil {
				set.ScriptVars[id] = map[string]string{}
			}
			for k, v := range rawIn {
				set.ScriptVars[id][k] = fmt.Sprint(v)
			}
		}
		if err := st.SetSettings(set); err != nil {
			return "", true, err
		}
		s.notifyChange()
		disp := pl.Display
		if disp == "" {
			disp = "cursor"
		}
		if hasEnabled {
			if enabled {
				return fmt.Sprintf("已将 %s 加入菜单栏（mode=%s, display=%s，当前共 %d 项）", id, pl.MenuBarMode, disp, len(set.MenuBar)), true, nil
			}
			return fmt.Sprintf("已将 %s 移出菜单栏（当前共 %d 项）", id, len(set.MenuBar)), true, nil
		}
		return fmt.Sprintf("已更新 %s 的菜单栏配置（pinned=%v, mode=%s, display=%s）", id, pl.MenuBar, pl.MenuBarMode, disp), true, nil

	case "menubar_show":
		idFilter := normalizePluginID(strings.TrimSpace(str("id")))
		lang := strings.TrimSpace(str("lang"))
		if lang == "" {
			lang = "zh"
		}
		st, err := store.Open(s.DataDir)
		if err != nil {
			return "", true, err
		}
		set := st.Settings()
		var items []map[string]any
		seen := map[string]bool{}
		for _, sc := range st.Scripts() {
			if sc.IsTable() {
				continue
			}
			seen[sc.ID] = true
			if idFilter != "" && sc.ID != idFilter {
				continue
			}
			if sc.IsBar() {
				pl := mcpPlacementFor(sc.ID, true, set)
				if idFilter == "" && (!pl.Enabled || !pl.MenuBar) {
					continue
				}
				items = append(items, s.renderBarMCPShow(sc, pl, set, lang))
				continue
			}
			pinned := slices.Contains(set.MenuBar, sc.ID)
			if idFilter == "" && !pinned {
				continue
			}
			if meta, err := plugins.CheckSource(sc.ID+".js", sc.Source); err == nil && (meta.Menubar || idFilter != "") {
				items = append(items, s.renderScriptMCPShow(sc.ID, sc.Name, sc.Source, set, lang))
			}
		}
		if plist, err := plugins.List(s.DataDir); err == nil {
			for _, p := range plist {
				pid := plugins.ScriptPrefix + p.Manifest.ID
				if seen[pid] {
					continue
				}
				if idFilter != "" && pid != idFilter && p.Manifest.ID != idFilter {
					continue
				}
				pinned := slices.Contains(set.MenuBar, pid)
				if idFilter == "" && !pinned {
					continue
				}
				if meta, err := plugins.CheckSource(p.Manifest.ID+".js", p.Source); err == nil && (meta.Menubar || idFilter != "") {
					items = append(items, s.renderScriptMCPShow(pid, p.Manifest.Name, p.Source, set, lang))
				}
			}
		}
		var displays []DisplayInfo
		if s.Displays != nil {
			displays = s.Displays()
		}
		payload := map[string]any{
			"count":    len(items),
			"items":    items,
			"displays": displays,
		}
		b, _ := json.MarshalIndent(payload, "", "  ")
		return string(b), true, nil

	case "bar_save":
		source := str("source")
		sp, warnings, err := bar.CompileCheck(source)
		if err != nil {
			return "", true, err
		}
		id := strings.TrimSpace(str("id"))
		if id == "" {
			id = sp.ID
		}
		if strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
			return "", true, fmt.Errorf("非法的 .bar id %q", id)
		}
		st, err := store.Open(s.DataDir)
		if err != nil {
			return "", true, err
		}
		home, _ := a["home"].(bool)
		sc := store.Script{
			ID:     id,
			Name:   sp.Title,
			Source: source,
			Home:   home,
			Origin: store.OriginUser,
			Kind:   store.KindBar,
		}
		if err := st.SaveScript(sc); err != nil {
			return "", true, err
		}
		set := st.Settings()
		if set.BarPlacements == nil {
			set.BarPlacements = map[string]store.BarPlacement{}
		}
		pl := mcpPlacementFor(id, true, set)
		if en, ok := a["enabled"].(bool); ok {
			pl.Enabled = en
			pl.MenuBar = en
		}
		if err := applyPlacementArgs(&pl, a); err != nil {
			return "", true, err
		}
		set.BarPlacements[id] = pl
		if rawIn, ok := a["inputs"].(map[string]any); ok && len(rawIn) > 0 {
			if set.ScriptVars == nil {
				set.ScriptVars = map[string]map[string]string{}
			}
			if set.ScriptVars[id] == nil {
				set.ScriptVars[id] = map[string]string{}
			}
			for k, v := range rawIn {
				set.ScriptVars[id][k] = fmt.Sprint(v)
			}
		}
		if err := st.SetSettings(set); err != nil {
			return "", true, err
		}
		s.notifyChange()
		out := fmt.Sprintf("已保存 .bar 组件 %s（%s，mode=%s, display=%s）", id, sp.Title, pl.MenuBarMode, orStr(pl.Display, "cursor"))
		for _, w := range warnings {
			out += "\n警告：" + w
		}
		return out, true, nil

	case "app_info":
		st, _ := store.Open(s.DataDir)
		plist, _ := plugins.List(s.DataDir)
		tables, scripts, bars, feeds, pinned := 0, 0, 0, 0, 0
		if st != nil {
			set := st.Settings()
			for _, sc := range st.Scripts() {
				switch {
				case sc.IsTable():
					tables++
				case sc.IsBar():
					bars++
					if pl := mcpPlacementFor(sc.ID, true, set); pl.Enabled && pl.MenuBar {
						pinned++
					}
				default:
					scripts++
				}
			}
			feeds = len(st.Feeds())
			pinned += len(set.MenuBar)
		}
		var displays []DisplayInfo
		if s.Displays != nil {
			displays = s.Displays()
		}
		payload := map[string]any{
			"name":            "ScriptBoard",
			"protocolVersion": protocolVersion,
			"readOnly":        s.readOnly(),
			"platform":        map[string]string{"os": caps.OS(), "kind": caps.Platform()},
			"widgetTypes":     engine.ChartTypes,
			"barVDOMNodes":    []string{"Column", "Row", "Box", "Scroll", "Text", "Icon", "Button", "Badge", "Progress", "Ring", "Chart", "StatusDot", "Segmented", "Switch", "ListRow", "Image", "Input", "Spacer", "Divider"},
			"barRuntimeAPIs":  []string{"fetch", "Headers", "Request", "Response", "URL", "URLSearchParams", "ReadableStream", "AbortController", "setTimeout", "setInterval", "console", "localStorage", "h", "$ui", "$metrix.invalidate", "$metrix.openURL", "$metrix.copy", "$metrix.log", "$metrix.storage", "$metrix.cookies"},
			"cookieSupport":   true,
			"storageSupport":  true,
			"toolsCount":      len(AllTools()),
			"trayActions":     append(append([]string(nil), engine.MenuActions...), "set_var:<k>=<v>", "set_dim:<k>=<v>"),
			"trayIcons":       engine.MenuBarIcons,
			"displayModes":    []string{"cursor", "internal", "external"},
			"menuBarModes":    []string{"panel", "menu", "open"},
			"displays":        displays,
			"shippedIcons":    len(iconcat.Names()),
			"counts": map[string]int{
				"scripts":      scripts,
				"tables":       tables,
				"bars":         bars,
				"barTemplates": len(bar.Templates()),
				"plugins":      len(plist),
				"feeds":        feeds,
				"trayPinned":   pinned,
				"widgetTypes":  len(engine.WidgetSpecs),
			},
		}
		b, _ := json.MarshalIndent(payload, "", "  ")
		return string(b), true, nil
	}
	if out, handled, err := s.callBar(name, a); handled {
		return out, true, err
	}
	if out, handled, err := s.callFeed(name, a); handled {
		return out, true, err
	}
	return "", false, nil
}

func mcpPlacementFor(id string, isBar bool, set store.Settings) store.BarPlacement {
	if set.BarPlacements != nil {
		if pl, ok := set.BarPlacements[id]; ok {
			if pl.MenuBarMode == "" {
				if isBar {
					pl.MenuBarMode = "panel"
				} else {
					pl.MenuBarMode = "menu"
				}
			}
			return pl
		}
	}
	if isBar {
		return store.BarPlacement{Enabled: true, MenuBar: true, MenuBarMode: "panel", ShortcutKey: "Alt+W"}
	}
	return store.BarPlacement{Enabled: true, MenuBar: slices.Contains(set.MenuBar, id), MenuBarMode: "menu", ShortcutKey: "Alt+W"}
}

func applyPlacementArgs(pl *store.BarPlacement, a map[string]any) error {
	if mode, ok := a["mode"].(string); ok && strings.TrimSpace(mode) != "" {
		m := strings.ToLower(strings.TrimSpace(mode))
		if m != "panel" && m != "menu" && m != "open" {
			return fmt.Errorf("无效的 mode %q，可选值：panel / menu / open", mode)
		}
		pl.MenuBarMode = m
	}
	if disp, ok := a["display"].(string); ok {
		d := strings.ToLower(strings.TrimSpace(disp))
		switch d {
		case "", "cursor":
			pl.Display = ""
		case "internal", "external":
			pl.Display = d
		default:
			return fmt.Errorf("无效的 display %q，可选值：cursor / internal / external", disp)
		}
	}
	if v, ok := a["leftSidebar"].(bool); ok {
		pl.LeftSidebar = v
	}
	if v, ok := a["rightSidebar"].(bool); ok {
		pl.RightSidebar = v
	}
	if v, ok := a["bottomBar"].(bool); ok {
		pl.BottomBar = v
	}
	if v, ok := a["shortcut"].(bool); ok {
		pl.Shortcut = v
	}
	return nil
}

func (s Server) renderBarMCPShow(sc store.Script, pl store.BarPlacement, set store.Settings, lang string) map[string]any {
	if s.MenuBarSnapshot != nil {
		if live, ok := s.MenuBarSnapshot(sc.ID); ok {
			return live
		}
	}
	sp, err := bar.Parse(sc.Source)
	if err != nil {
		return map[string]any{"id": sc.ID, "name": sc.Name, "kind": "bar", "error": err.Error()}
	}
	var saved map[string]string
	if set.ScriptVars != nil {
		saved = set.ScriptVars[sc.ID]
	}
	inputs := bar.MergeInputs(sp, saved)
	var snap bar.Snapshot
	jar, _ := bar.NewCookieJar(bar.CookiesPath(s.DataDir, sc.ID))
	kv, _ := bar.NewKVStore(bar.StoragePath(s.DataDir, sc.ID))
	inst, err := bar.NewInstance(sp, bar.Options{
		Inputs:  inputs,
		Lang:    lang,
		Offline: true,
		Cookies: jar,
		Storage: kv,
	})
	if err == nil {
		snap = inst.Snapshot()
		inst.Close()
	} else {
		snap.Error = err.Error()
	}
	cookieCount := 0
	if jar != nil {
		cookieCount = jar.Count()
	}
	var storageKeys []string
	if kv != nil {
		storageKeys = kv.Keys()
	}
	return map[string]any{
		"id":          sc.ID,
		"name":        orStr(sp.Title, sc.Name),
		"kind":        "bar",
		"enabled":     pl.Enabled,
		"pinned":      pl.Enabled && pl.MenuBar,
		"mode":        pl.MenuBarMode,
		"display":     orStr(pl.Display, "cursor"),
		"inputs":      bar.MaskInputs(sp, inputs),
		"permissions": sp.Permissions,
		"tray":        snap.Tray,
		"panel":       snap.Panel,
		"panelTexts":  extractBarTexts(snap.Panel),
		"logs":        snap.Logs,
		"cookieCount": cookieCount,
		"storageKeys": storageKeys,
		"error":       snap.Error,
	}
}

func (s Server) renderScriptMCPShow(id, name, source string, set store.Settings, lang string) map[string]any {
	if s.MenuBarSnapshot != nil {
		if live, ok := s.MenuBarSnapshot(id); ok {
			return live
		}
	}
	pl := mcpPlacementFor(id, false, set)
	out := map[string]any{
		"id":      id,
		"name":    name,
		"kind":    "script",
		"enabled": pl.Enabled,
		"pinned":  slices.Contains(set.MenuBar, id),
		"mode":    pl.MenuBarMode,
		"display": orStr(pl.Display, "cursor"),
	}
	compiled, err := engine.CompileLang(name, source, lang)
	if err != nil {
		out["error"] = err.Error()
		return out
	}
	res, err := compiled.Run(context.Background(), engine.Input{
		Now:  time.Now(),
		Lang: lang,
		OS:   caps.OS(),
		Kind: caps.Platform(),
		Vars: engine.ResolveVars(compiled.Meta.Vars, set.ScriptVars[id]),
	})
	if err != nil {
		out["error"] = err.Error()
		return out
	}
	out["title"] = res.Title
	out["big"] = res.Big
	out["sub"] = res.Sub
	out["icon"] = res.Icon
	out["tray"] = res.MenuBar
	out["menu"] = res.Menu
	out["lines"] = res.Lines
	return out
}

func extractBarTexts(n *bar.Node) []string {
	return bar.CollectTexts(n)
}

func orStr(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
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
