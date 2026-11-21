package table

import (
	"strings"
	"testing"
)

func TestParseAndWriteJSON(t *testing.T) {
	raw := `[{"date":"2026-11-01","amount":120.5},{"date":"2026-11-02","amount":80}]`
	rows, err := Parse("sample.json", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	cols, body := Header(rows)
	if len(cols) != 2 || len(body) != 2 {
		t.Fatalf("cols=%v body=%v", cols, body)
	}
	out := WriteJSON(cols, body)
	if !strings.Contains(out, `"2026-11-01"`) || !strings.Contains(out, `"120.5"`) {
		t.Fatalf("unexpected WriteJSON: %s", out)
	}

	objRaw := `{"columns":["Name","Score"],"rows":[["Alice","95"],["Bob","88"]]}`
	rows2, err := ParseJSON([]byte(objRaw))
	if err != nil || len(rows2) != 3 {
		t.Fatalf("object JSON: err=%v rows=%v", err, rows2)
	}
}

func TestSummarizeColumn(t *testing.T) {
	rows := [][]string{
		{"2026-11-01", "10", "ok"},
		{"2026-11-02", "20", "warn"},
		{"2026-11-03", "30", ""},
	}
	num := SummarizeColumn(rows, 1)
	if !num.Numeric || num.Sum != 60 || num.Avg != 20 || num.Min != 10 || num.Max != 30 {
		t.Fatalf("unexpected numeric stats: %+v", num)
	}
	txt := SummarizeColumn(rows, 2)
	if txt.Numeric || txt.NonEmpty != 2 {
		t.Fatalf("unexpected text stats: %+v", txt)
	}
}
