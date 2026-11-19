// Package table reads and writes the data of a data-table plugin: rows of
// cells typed in, pasted, or imported from a CSV, TSV or Excel (.xlsx) file.
//
// A table is plain rows of text. What a cell means (a number, a date) is worked
// out when it is used, by Number and Time, so that nothing is lost on the way
// in or out.
package table

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// MaxRows and MaxCols bound what an import takes, so that a huge sheet cannot
// make the editor unusable.
const (
	MaxRows = 20000
	MaxCols = 40
)

// Parse reads the rows of a file by its name: .xlsx, or text (.csv, .tsv, .txt).
func Parse(name string, data []byte) ([][]string, error) {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".xlsx", ".xlsm":
		return ParseXLSX(data)
	case ".xls":
		return nil, errors.New("不支持旧的 .xls 格式，请在 Excel 里另存为 .xlsx 或 .csv")
	}
	if len(data) > 4 && string(data[:2]) == "PK" {
		return ParseXLSX(data) // an Excel file with another name
	}
	text, err := decode(data)
	if err != nil {
		return nil, err
	}
	return ParseText(text)
}

// decode turns the bytes of a text file into text: UTF-8 (with or without a
// byte-order mark), or GBK, which is what Excel on a Chinese system writes.
func decode(data []byte) (string, error) {
	data = []byte(strings.TrimPrefix(string(data), bom))
	if utf8.Valid(data) {
		return string(data), nil
	}
	out, err := simplifiedchinese.GBK.NewDecoder().Bytes(data)
	if err != nil {
		return "", errors.New("无法识别文件编码，请另存为 UTF-8 的 CSV")
	}
	return string(out), nil
}

// Rect makes every row as long as the longest, drops rows that are empty, and
// applies the limits.
func Rect(rows [][]string) [][]string {
	width := 0
	for _, r := range rows {
		for len(r) > 0 && strings.TrimSpace(r[len(r)-1]) == "" {
			r = r[:len(r)-1]
		}
		width = max(width, len(r))
	}
	width = min(width, MaxCols)
	var out [][]string
	for _, r := range rows {
		blank := true
		cells := make([]string, width)
		for i := 0; i < width && i < len(r); i++ {
			cells[i] = strings.TrimSpace(r[i])
			if cells[i] != "" {
				blank = false
			}
		}
		if !blank {
			out = append(out, cells)
		}
		if len(out) >= MaxRows+1 {
			break
		}
	}
	return out
}

// Header splits off the header row. The first row is a header when some cell in
// it is neither a number nor a date; otherwise the columns are called 列 1, 列 2…
func Header(rows [][]string) (cols []string, body [][]string) {
	if len(rows) == 0 {
		return nil, nil
	}
	first := rows[0]
	header := false
	for _, c := range first {
		if c == "" {
			continue
		}
		if _, ok := Number(c); ok {
			continue
		}
		if _, ok := Time(c); ok {
			continue
		}
		header = true
	}
	if header {
		cols = make([]string, len(first))
		for i, c := range first {
			cols[i] = c
			if c == "" {
				cols[i] = fmt.Sprintf("列 %d", i+1)
			}
		}
		return cols, rows[1:]
	}
	cols = make([]string, len(first))
	for i := range cols {
		cols[i] = fmt.Sprintf("列 %d", i+1)
	}
	return cols, rows
}

// Number reads a cell as a number: 12, -3.5, 1,234.5, ¥12, 45%.
func Number(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v, true
	}
	t := strings.TrimLeft(s, "¥￥$€£")
	t = strings.TrimRight(t, "%")
	t = strings.ReplaceAll(t, ",", "")
	if v, err := strconv.ParseFloat(t, 64); err == nil {
		return v, true
	}
	return 0, false
}

var timeLayouts = []string{
	"2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02T15:04:05Z07:00", "2006-01-02",
	"2006/1/2 15:04:05", "2006/1/2 15:04", "2006/1/2",
	"2006.1.2", "2006-1-2 15:04", "2006-1-2", "20060102",
}

// Time reads a cell as a date or a date and time, in the local zone.
func Time(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if len(s) < 8 {
		return time.Time{}, false
	}
	for _, l := range timeLayouts {
		if t, err := time.ParseInLocation(l, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// FormatTime writes a time as a cell: the date, with the time when it has one.
func FormatTime(t time.Time) string {
	if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 {
		return t.Format("2006-01-02")
	}
	return t.Format("2006-01-02 15:04")
}

// FormatNumber writes a number as a cell, without trailing zeros.
func FormatNumber(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// bom is the byte-order mark some programs put first in a UTF-8 file.
const bom = "\xef\xbb\xbf"
