package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"scriptboard/src/internal/caps"
	"scriptboard/src/internal/engine"
	"scriptboard/src/internal/table"
)

// tableWidgetState stores per-instance sort and search state for interactive
// table widgets and the detail page's tabular view mode.
type tableWidgetState struct {
	query   string
	sortCol int // -1 none, 0..N-1 column index
	desc    bool
	initSet bool
	note    string
}

func (a *app) tableWidgetStateOf(key string, spec *engine.TableSpec) *tableWidgetState {
	if a.tableViews == nil {
		a.tableViews = map[string]*tableWidgetState{}
	}
	st, ok := a.tableViews[key]
	if !ok {
		st = &tableWidgetState{sortCol: -1}
		a.tableViews[key] = st
	}
	if !st.initSet && spec != nil {
		st.initSet = true
		if spec.SortBy > 0 && spec.SortBy <= len(spec.Columns) {
			st.sortCol = spec.SortBy - 1
			st.desc = spec.Desc
		}
	}
	return st
}

// isTableWidget reports whether ch is a native "table" widget.
func isTableWidget(ch *engine.Chart) bool { return ch != nil && ch.Type == "table" }

// filterAndSortTableRows applies search filtering and smart column sorting
// (numeric -> time -> case-insensitive text) without mutating spec.Rows.
func filterAndSortTableRows(rows [][]string, query string, sortCol int, desc bool) [][]string {
	q := strings.ToLower(strings.TrimSpace(query))
	var out [][]string
	if q == "" {
		out = append([][]string(nil), rows...)
	} else {
		for _, r := range rows {
			matched := false
			for _, cell := range r {
				if strings.Contains(strings.ToLower(cell), q) {
					matched = true
					break
				}
			}
			if matched {
				out = append(out, r)
			}
		}
	}
	if sortCol >= 0 && len(out) > 1 {
		sort.SliceStable(out, func(i, j int) bool {
			cmp := compareTableCells(cell(out[i], sortCol), cell(out[j], sortCol))
			if desc {
				return cmp > 0
			}
			return cmp < 0
		})
	}
	return out
}

// compareTableCells orders two cell strings naturally by number, date/time, or
// case-insensitive text.
func compareTableCells(a, b string) int {
	sa, sb := strings.TrimSpace(a), strings.TrimSpace(b)
	if sa == sb {
		return 0
	}
	// Strip common percent/currency/unit suffixes for numeric comparison.
	na, oka := parseSortableNumber(sa)
	nb, okb := parseSortableNumber(sb)
	if oka && okb {
		switch {
		case na < nb:
			return -1
		case na > nb:
			return 1
		default:
			return 0
		}
	}
	ta, oka := table.Time(sa)
	tb, okb := table.Time(sb)
	if oka && okb {
		switch {
		case ta.Before(tb):
			return -1
		case ta.After(tb):
			return 1
		default:
			return 0
		}
	}
	la, lb := strings.ToLower(sa), strings.ToLower(sb)
	switch {
	case la < lb:
		return -1
	case la > lb:
		return 1
	default:
		return 0
	}
}

func parseSortableNumber(s string) (float64, bool) {
	cleaned := strings.TrimSpace(s)
	cleaned = strings.TrimPrefix(cleaned, "+")
	cleaned = strings.TrimPrefix(cleaned, "$")
	cleaned = strings.TrimPrefix(cleaned, "¥")
	cleaned = strings.TrimPrefix(cleaned, "€")
	cleaned = strings.TrimSuffix(cleaned, "%")
	cleaned = strings.ReplaceAll(cleaned, ",", "")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return 0, false
	}
	return table.Number(cleaned)
}

func tableSpecToCSV(spec *engine.TableSpec, rows [][]string) string {
	cols := make([]string, len(spec.Columns))
	for i, col := range spec.Columns {
		cols[i] = col.Title
	}
	return table.WriteCSV(cols, rows)
}

// tableWidgetView renders a structured TableSpec as either a compact home card
// preview or a full interactive sortable/searchable table with CSV copy/export.
func (a *app) tableWidgetView(c *ui.Context, key string, spec *engine.TableSpec, full bool, limit int) ui.Element {
	p := pal(c)
	if spec == nil || len(spec.Columns) == 0 {
		return ui.Text(c, tr("暂无数据")).FontSize(fsSmall).TextColor(p.muted)
	}
	st := a.tableWidgetStateOf(key, spec)
	rows := filterAndSortTableRows(spec.Rows, st.query, st.sortCol, st.desc)
	if !full {
		rows = filterAndSortTableRows(spec.Rows, "", st.sortCol, st.desc)
	}

	maxRows := len(rows)
	if !full {
		capRows := spec.Compact
		if capRows <= 0 {
			capRows = 4
		}
		if limit > 0 && limit < capRows {
			capRows = limit
		}
		if maxRows > capRows {
			maxRows = capRows
		}
	}

	return ui.Column(c).Gap(8).Children(func() {
		if full {
			ui.Row(c).AlignItems(ui.Center).Gap(8).Wrap().Children(func() {
				if spec.Searchable || len(spec.Rows) > 4 {
					textInput(c.Key("tsearch:"+key), &st.query, tr("筛选表格行…")).Width(220)
				}
				if st.query != "" {
					if btn(c.Key("tclear:"+key), tr("清空筛选")).Clicked() {
						st.query = ""
					}
				}
				ui.Text(c, fmt.Sprintf(tr("共 %d 行"), len(rows))).FontSize(fsSmall).TextColor(p.muted)
				if st.note != "" {
					ui.Text(c, st.note).FontSize(fsTiny).TextColor(p.accent)
				}
				ui.Spacer(c)
				if btn(c.Key("tcopy:"+key), tr("复制 CSV")).Clicked() {
					copyText(tableSpecToCSV(spec, rows))
					st.note = tr("已复制 CSV")
				}
				if caps.Platform() == caps.Desktop {
					if btn(c.Key("texport:"+key), tr("导出 CSV")).Clicked() {
						csvText := tableSpecToCSV(spec, rows)
						a.background(func() (string, error) {
							path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
								Title: tr("导出 CSV"), DefaultPath: "data.csv",
								Filters: []mygo.FileFilter{{Name: "CSV", Extensions: []string{"csv"}}},
							})
							if err != nil || path == "" {
								return "", err
							}
							return path, os.WriteFile(path, []byte("\xef\xbb\xbf"+csvText), 0o644)
						}, func(path string, err error) {
							if err != nil {
								st.note = err.Error()
							} else if path != "" {
								st.note = tr("已导出到 ") + path
							}
						})
					}
				}
			})
		}

		// Column header row (clickable to sort in full view).
		ui.Row(c).AlignItems(ui.Center).Gap(14).Padding(6, 10).Radius(radControl).Background(p.chip).Children(func() {
			for j, col := range spec.Columns {
				j, col := j, col
				weight := float32(max(col.Width, 1))
				align := tableCellAlign(col.Align)
				title := col.Title
				if full && st.sortCol == j {
					if st.desc {
						title += " ↓"
					} else {
						title += " ↑"
					}
				}
				if full {
					hdr := ui.ButtonBase(c.Key(fmt.Sprintf("thdr:%s:%d", key, j))).
						Grow(weight).MinWidth(0).Justify(align).Padding(2, 0).Cursor(ui.CursorPointer)
					if hdr.Children(func() {
						ui.Text(c, title).FontSize(fsTiny).FontWeight(650).TextColor(p.muted).NoWrap()
					}).Clicked() {
						if st.sortCol == j {
							if !st.desc {
								st.desc = true
							} else {
								st.sortCol = -1
								st.desc = false
							}
						} else {
							st.sortCol = j
							st.desc = false
						}
					}
				} else {
					ui.Row(c).Grow(weight).MinWidth(0).Justify(align).ClipX().Children(func() {
						ui.Text(c, title).FontSize(fsTiny).FontWeight(650).TextColor(p.muted).NoWrap()
					})
				}
			}
		})

		if len(rows) == 0 {
			ui.Text(c, tr("没有匹配的行")).FontSize(fsSmall).TextColor(p.muted).Padding(8, 8)
			return
		}

		// Data rows.
		for i := 0; i < maxRows; i++ {
			r := rows[i]
			rowEl := ui.Row(c).AlignItems(ui.Center).Gap(14).Padding(6, 10).Radius(6)
			if spec.Striped && i%2 == 1 {
				rowEl = rowEl.Background(p.chip.Alpha(0.45))
			}
			if full && rowEl.Hovered() {
				rowEl = rowEl.Background(p.hover)
			}
			rowEl.Children(func() {
				for j, col := range spec.Columns {
					val := cell(r, j)
					weight := float32(max(col.Width, 1))
					align := tableCellAlign(col.Align)
					ui.Row(c).Grow(weight).MinWidth(0).Justify(align).AlignItems(ui.Center).ClipX().Children(func() {
						renderTableCell(c, p, col, val, full, func(copied string) {
							copyText(copied)
							st.note = tr("已复制：") + copied
						})
					})
				}
			})
			if i < maxRows-1 {
				ui.Box(c).Height(1).Shrink(0).Background(p.hairline)
			}
		}

		if !full && len(rows) > maxRows {
			ui.Text(c, fmt.Sprintf(tr("另外 %d 行"), len(rows)-maxRows)).FontSize(fsTiny).TextColor(p.muted).Padding(2, 8)
		}
	})
}

func tableCellAlign(align string) ui.Align {
	switch align {
	case "right":
		return ui.End
	case "center":
		return ui.Center
	default:
		return ui.Start
	}
}

func renderTableCell(c *ui.Context, p palette, col engine.TableColumn, val string, clickable bool, onCopy func(string)) {
	if val == "" {
		ui.Text(c, "—").FontSize(fsSmall).TextColor(p.faint).NoWrap()
		return
	}
	var el ui.Element
	switch col.Format {
	case "delta":
		fg := p.muted
		if strings.HasPrefix(val, "+") && val != "+0.00%" && val != "+0%" {
			fg = p.up
		} else if strings.HasPrefix(val, "-") && val != "-0.00%" && val != "-0%" {
			fg = p.down
		}
		el = ui.Text(c, val).FontSize(fsSmall).FontWeight(600).TextColor(fg).NoWrap()
	case "badge":
		el = chip(c, val, p.accent, p.accent.Alpha(0.14))
	case "status":
		dotCol := p.muted
		switch strings.ToLower(strings.TrimSpace(val)) {
		case "ok", "online", "up", "正常", "在线":
			dotCol = p.down
		case "warn", "warning", "偏高", "警告":
			dotCol = p.warn
		case "err", "error", "down", "异常", "离线":
			dotCol = p.up
		case "info":
			dotCol = p.accent
		}
		el = ui.Row(c).AlignItems(ui.Center).Gap(5).Children(func() {
			ui.Box(c).Size(7, 7).Radius(3.5).Background(dotCol).Shrink(0)
			ui.Text(c, val).FontSize(fsSmall).TextColor(p.text).NoWrap()
		})
	case "number":
		el = ui.Text(c, val).FontSize(fsSmall).FontWeight(600).TextColor(p.text).NoWrap()
	default:
		el = ui.Text(c, val).FontSize(fsSmall).TextColor(p.text).NoWrap()
	}
	if clickable && el.Cursor(ui.CursorPointer).Clicked() {
		onCopy(val)
	}
}
