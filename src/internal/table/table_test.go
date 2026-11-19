package table

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func TestParseTextDelimiters(t *testing.T) {
	for name, text := range map[string]string{
		"comma":     "日期,体重\n2026-11-01,70.5\n2026-11-02,70.1\n",
		"semicolon": "日期;体重\n2026-11-01;70.5\n2026-11-02;70.1\n",
		"tab":       "日期\t体重\n2026-11-01\t70.5\n2026-11-02\t70.1\n", // pasted from Excel
		"bom+crlf":  bom + "日期,体重\r\n2026-11-01,70.5\r\n2026-11-02,70.1\r\n",
	} {
		rows, err := ParseText(text)
		if err != nil || len(rows) != 3 || rows[0][1] != "体重" || rows[2][1] != "70.1" {
			t.Errorf("%s: %v %v", name, rows, err)
		}
	}
	rows, _ := ParseText("a,b\n\"x, y\",\"he said \"\"hi\"\"\"\n,\n\n")
	if len(rows) != 2 || rows[1][0] != "x, y" || rows[1][1] != `he said "hi"` {
		t.Errorf("quotes and blank rows: %v", rows)
	}
	if _, err := ParseText("  \n "); err == nil {
		t.Error("empty text accepted")
	}
}

// Excel on a Chinese system saves CSV as GBK.
func TestParseGBKFile(t *testing.T) {
	gbk, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("日期,体重\n2026-11-01,70.5\n"))
	if err != nil {
		t.Fatal(err)
	}
	rows, err := Parse("体重.csv", gbk)
	if err != nil || rows[0][0] != "日期" || rows[1][1] != "70.5" {
		t.Fatalf("%v %v", rows, err)
	}
	if _, err := Parse("old.xls", []byte("x")); err == nil || !strings.Contains(err.Error(), "xlsx") {
		t.Errorf(".xls: %v", err)
	}
}

func TestHeader(t *testing.T) {
	cols, body := Header([][]string{{"日期", "体重"}, {"2026-11-01", "70"}})
	if cols[0] != "日期" || len(body) != 1 {
		t.Errorf("with a header: %v %v", cols, body)
	}
	cols, body = Header([][]string{{"2026-11-01", "70"}, {"2026-11-02", "71"}})
	if cols[0] != "列 1" || len(body) != 2 {
		t.Errorf("without: %v %v", cols, body)
	}
}

func TestNumberAndTime(t *testing.T) {
	for in, want := range map[string]float64{"12": 12, "-3.5": -3.5, "1,234.5": 1234.5, "¥12": 12, "45%": 45, " 7 ": 7} {
		if v, ok := Number(in); !ok || v != want {
			t.Errorf("Number(%q) = %v %v", in, v, ok)
		}
	}
	for _, in := range []string{"", "abc", "12a", "-"} {
		if _, ok := Number(in); ok {
			t.Errorf("Number(%q) accepted", in)
		}
	}
	for _, in := range []string{"2026-11-01", "2026/11/1", "2026-11-01 08:30", "2026.11.1", "20261101"} {
		if _, ok := Time(in); !ok {
			t.Errorf("Time(%q) not read", in)
		}
	}
	if _, ok := Time("70.5"); ok {
		t.Error("a number is not a date")
	}
}

func TestWriteCSVRoundTrips(t *testing.T) {
	cols := []string{"名称", "备注"}
	rows := [][]string{{"a,b", `say "hi"`}, {"c", "line\nbreak"}}
	back, err := ParseText(WriteCSV(cols, rows))
	if err != nil || len(back) != 3 || back[1][0] != "a,b" || back[1][1] != `say "hi"` || back[2][1] != "line\nbreak" {
		t.Fatalf("%v %v", back, err)
	}
}

// xlsx builds a minimal workbook: a shared string header, a date column with a
// date style, a number with float noise, an inline string, a gap and a formula.
func xlsx(t *testing.T, sheetFile string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name, body string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(body))
	}
	add("xl/workbook.xml", `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="S" sheetId="1" r:id="rId9"/></sheets></workbook>`)
	add("xl/_rels/workbook.xml.rels", `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId9" Target="worksheets/`+sheetFile+`"/></Relationships>`)
	add("xl/sharedStrings.xml", `<sst><si><t>日期</t></si><si><t>金额</t></si><si><r><t>备</t></r><r><t>注</t></r></si></sst>`)
	add("xl/styles.xml", `<styleSheet><numFmts><numFmt numFmtId="164" formatCode="yyyy&quot;年&quot;m&quot;月&quot;d&quot;日&quot;"/><numFmt numFmtId="165" formatCode="0.00&quot;d&quot;"/></numFmts><cellXfs><xf numFmtId="0"/><xf numFmtId="14"/><xf numFmtId="164"/><xf numFmtId="165"/></cellXfs></styleSheet>`)
	add("xl/worksheets/"+sheetFile, `<worksheet><sheetData>
<row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c><c r="D1" t="s"><v>2</v></c></row>
<row r="2"><c r="A2" s="1"><v>46327</v></c><c r="B2"><v>0.30000000000000004</v></c><c r="D2" t="inlineStr"><is><t>x</t></is></c></row>
<row r="3"><c r="A3" s="2"><v>46328.5</v></c><c r="B3" s="3"><v>12.5</v></c><c r="C3"><f>B3*2</f><v>25</v></c></row>
<row r="5"><c r="A5" t="b"><v>1</v></c></row>
</sheetData></worksheet>`)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestParseXLSX(t *testing.T) {
	for _, sheet := range []string{"sheet1.xml", "data.xml"} {
		rows, err := Parse("x.xlsx", xlsx(t, sheet))
		if err != nil {
			t.Fatalf("%s: %v", sheet, err)
		}
		if len(rows) < 4 || rows[0][0] != "日期" || rows[0][3] != "备注" {
			t.Fatalf("%s header: %v", sheet, rows)
		}
		if rows[1][0] != "2026-11-01" || rows[1][1] != "0.3" || rows[1][3] != "x" {
			t.Errorf("%s row 2: %v (a date as a date, no float noise, an inline string)", sheet, rows[1])
		}
		if rows[2][0] != "2026-11-02 12:00" || rows[2][1] != "12.5" || rows[2][2] != "25" {
			t.Errorf("%s row 3: %v (a custom date format, a formula's saved value)", sheet, rows[2])
		}
		if rows[len(rows)-1][0] != "TRUE" {
			t.Errorf("%s last row: %v", sheet, rows[len(rows)-1])
		}
	}
	if _, err := ParseXLSX([]byte("not a zip")); err == nil {
		t.Error("garbage accepted")
	}
	if _, err := Parse("renamed.dat", xlsx(t, "sheet1.xml")); err != nil {
		t.Errorf("an xlsx with another name: %v", err)
	}
}

func TestIsDateFormat(t *testing.T) {
	for code, want := range map[string]bool{
		"yyyy-mm-dd": true, "h:mm": true, `0.00"d"`: false, "0.00": false, `[Red]0`: false, `"d"0`: false, `yyyy"年"m"月"`: true, "General": false,
	} {
		if got := isDateFormat(code); got != want {
			t.Errorf("isDateFormat(%q) = %v", code, got)
		}
	}
}
