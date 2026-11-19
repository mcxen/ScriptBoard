package table

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"math"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

// maxXMLBytes bounds each part of the file that is read, against a zip bomb.
const maxXMLBytes = 64 << 20

// ParseXLSX reads the first sheet of an Excel workbook as rows of text. Dates
// come out as dates and numbers without float noise; formulas give the value
// Excel saved.
func ParseXLSX(data []byte) ([][]string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, errors.New("不是有效的 .xlsx 文件")
	}
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[f.Name] = f
	}
	sheet := firstSheet(zr, files)
	if sheet == nil {
		return nil, errors.New("文件里没有工作表")
	}
	shared := sharedStrings(files["xl/sharedStrings.xml"])
	dateStyle := dateStyles(files["xl/styles.xml"])
	rc, err := sheet.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	type cell struct {
		R string `xml:"r,attr"`
		T string `xml:"t,attr"`
		S int    `xml:"s,attr"`
		V string `xml:"v"`
		// Inline strings: <is><t>…</t></is>, possibly in runs.
		IS struct {
			T []string `xml:"t"`
			R []struct {
				T string `xml:"t"`
			} `xml:"r"`
		} `xml:"is"`
	}
	dec := xml.NewDecoder(io.LimitReader(rc, maxXMLBytes))
	var rows [][]string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.New("工作表读取失败")
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "row" {
			continue
		}
		var rowIdx int
		for _, a := range se.Attr {
			if a.Name.Local == "r" {
				rowIdx, _ = strconv.Atoi(a.Value)
			}
		}
		var row []string
		col := 0
		for {
			t2, err := dec.Token()
			if err != nil {
				return nil, errors.New("工作表读取失败")
			}
			if end, ok := t2.(xml.EndElement); ok && end.Name.Local == "row" {
				break
			}
			cs, ok := t2.(xml.StartElement)
			if !ok || cs.Name.Local != "c" {
				continue
			}
			var c cell
			if err := dec.DecodeElement(&c, &cs); err != nil {
				return nil, errors.New("工作表读取失败")
			}
			if c.R != "" {
				col = colIndex(c.R)
			}
			if col >= MaxCols {
				col++
				continue
			}
			for len(row) <= col {
				row = append(row, "")
			}
			row[col] = cellText(c.T, c.V, c.IS.T, c.IS.R, c.S, shared, dateStyle)
			col++
		}
		if rowIdx > 0 {
			for len(rows) < rowIdx-1 {
				rows = append(rows, nil)
			}
		}
		rows = append(rows, row)
		if len(rows) > MaxRows+1 {
			break
		}
	}
	rows = Rect(rows)
	if len(rows) == 0 {
		return nil, errors.New("工作表是空的")
	}
	return rows, nil
}

func cellText(t, v string, inline []string, runs []struct {
	T string `xml:"t"`
}, style int, shared []string, dateStyle map[int]bool) string {
	switch t {
	case "s":
		if i, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && i >= 0 && i < len(shared) {
			return shared[i]
		}
		return ""
	case "inlineStr":
		var b strings.Builder
		for _, s := range inline {
			b.WriteString(s)
		}
		for _, r := range runs {
			b.WriteString(r.T)
		}
		return b.String()
	case "b":
		if strings.TrimSpace(v) == "1" {
			return "TRUE"
		}
		return "FALSE"
	case "str", "e":
		return v
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil {
		return v
	}
	if dateStyle[style] {
		return FormatTime(serialTime(f))
	}
	// 0.1+0.2 is saved as 0.30000000000000004; Excel shows 15 digits.
	return FormatNumber(roundSig(f, 15))
}

func roundSig(f float64, n int) float64 {
	if f == 0 || math.IsInf(f, 0) || math.IsNaN(f) {
		return f
	}
	s := strconv.FormatFloat(f, 'g', n, 64)
	r, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return f
	}
	return r
}

// serialTime is an Excel date: days since 1899-12-30, the time of day a fraction.
func serialTime(f float64) time.Time {
	days := math.Floor(f)
	secs := math.Round((f - days) * 86400)
	base := time.Date(1899, 12, 30, 0, 0, 0, 0, time.Local)
	return base.AddDate(0, 0, int(days)).Add(time.Duration(secs) * time.Second)
}

// colIndex is the zero-based column of a reference such as "C12".
func colIndex(ref string) int {
	n := 0
	for _, r := range ref {
		if r < 'A' || r > 'Z' {
			if r >= 'a' && r <= 'z' {
				n = n*26 + int(r-'a') + 1
				continue
			}
			break
		}
		n = n*26 + int(r-'A') + 1
	}
	return n - 1
}

func readPart(f *zip.File) []byte {
	if f == nil {
		return nil
	}
	rc, err := f.Open()
	if err != nil {
		return nil
	}
	defer rc.Close()
	b, _ := io.ReadAll(io.LimitReader(rc, maxXMLBytes))
	return b
}

// firstSheet finds the first sheet in the workbook's order, or failing that the
// first worksheet file by name.
func firstSheet(zr *zip.Reader, files map[string]*zip.File) *zip.File {
	var wb struct {
		Sheets []struct {
			RID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		} `xml:"sheets>sheet"`
	}
	var rels struct {
		R []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if xml.Unmarshal(readPart(files["xl/workbook.xml"]), &wb) == nil && len(wb.Sheets) > 0 &&
		xml.Unmarshal(readPart(files["xl/_rels/workbook.xml.rels"]), &rels) == nil {
		for _, r := range rels.R {
			if r.ID == wb.Sheets[0].RID {
				p := strings.TrimPrefix(r.Target, "/")
				if !strings.HasPrefix(p, "xl/") {
					p = path.Join("xl", p)
				}
				if f := files[p]; f != nil {
					return f
				}
			}
		}
	}
	var names []string
	for n := range files {
		if strings.HasPrefix(n, "xl/worksheets/") && strings.HasSuffix(n, ".xml") {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil
	}
	return files[names[0]]
}

func sharedStrings(f *zip.File) []string {
	b := readPart(f)
	if b == nil {
		return nil
	}
	var sst struct {
		SI []struct {
			T string `xml:"t"`
			R []struct {
				T string `xml:"t"`
			} `xml:"r"`
		} `xml:"si"`
	}
	if xml.Unmarshal(b, &sst) != nil {
		return nil
	}
	out := make([]string, len(sst.SI))
	for i, si := range sst.SI {
		s := si.T
		for _, r := range si.R {
			s += r.T
		}
		out[i] = s
	}
	return out
}

// dateStyles are the cell styles (indexes into cellXfs) that format a date.
func dateStyles(f *zip.File) map[int]bool {
	out := map[int]bool{}
	b := readPart(f)
	if b == nil {
		return out
	}
	var st struct {
		Fmts []struct {
			ID   int    `xml:"numFmtId,attr"`
			Code string `xml:"formatCode,attr"`
		} `xml:"numFmts>numFmt"`
		Xfs []struct {
			NumFmt int `xml:"numFmtId,attr"`
		} `xml:"cellXfs>xf"`
	}
	if xml.Unmarshal(b, &st) != nil {
		return out
	}
	custom := map[int]bool{}
	for _, f := range st.Fmts {
		custom[f.ID] = isDateFormat(f.Code)
	}
	for i, xf := range st.Xfs {
		id := xf.NumFmt
		switch {
		case id >= 14 && id <= 22, id >= 45 && id <= 47:
			out[i] = true
		case custom[id]:
			out[i] = true
		}
	}
	return out
}

// isDateFormat reports whether a number format code shows a date: it has y, m,
// d, h or s outside quotes, brackets and escapes.
func isDateFormat(code string) bool {
	inQuote, inBracket := false, false
	for i := 0; i < len(code); i++ {
		ch := code[i]
		switch {
		case ch == '"':
			inQuote = !inQuote
		case ch == '[':
			inBracket = true
		case ch == ']':
			inBracket = false
		case ch == '\\' || ch == '_' || ch == '*':
			i++
		case inQuote || inBracket:
		case strings.ContainsRune("ydhsYDHS", rune(ch)):
			return true
		case ch == 'm' || ch == 'M':
			return true
		}
	}
	return false
}
