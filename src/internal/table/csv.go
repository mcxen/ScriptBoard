package table

import (
	"encoding/csv"
	"errors"
	"strings"
)

// ParseText reads pasted or saved text as rows: comma, semicolon or tab
// separated (the one the first line has most of), with quotes.
func ParseText(text string) ([][]string, error) {
	text = strings.TrimPrefix(text, bom)
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("没有内容")
	}
	r := csv.NewReader(strings.NewReader(text))
	r.Comma = delimiter(text)
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	r.TrimLeadingSpace = true
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	rows = Rect(rows)
	if len(rows) == 0 {
		return nil, errors.New("没有内容")
	}
	return rows, nil
}

// delimiter is the separator the first line uses most.
func delimiter(text string) rune {
	line := text
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		line = text[:i]
	}
	best, n := ',', 0
	for _, d := range []rune{'\t', ';', ','} {
		if c := strings.Count(line, string(d)); c > n {
			best, n = d, c
		}
	}
	return best
}

// WriteCSV writes the table as CSV text with a header row, quoting what needs it.
func WriteCSV(cols []string, rows [][]string) string {
	var b strings.Builder
	w := csv.NewWriter(&b)
	_ = w.Write(cols)
	for _, r := range rows {
		_ = w.Write(r)
	}
	w.Flush()
	return b.String()
}
