package web

import (
	"encoding/json"
	"testing"
)

func TestTableAPI(t *testing.T) {
	s := New(testOptions(false))
	defer s.Close()
	post := func(kind string, body map[string]any) (int, map[string]any) {
		t.Helper()
		b, _ := json.Marshal(body)
		rec := call(s, "POST", "/api/v2/table/"+kind, "127.0.0.1:8765", s.Token(), "", string(b))
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}

	tests := []struct {
		name, kind string
		body       map[string]any
		status     int
		check      func(map[string]any) bool
	}{
		{"parse guesses", "parse", map[string]any{"input": "a\tb\n1\t2\n"}, 200,
			func(o map[string]any) bool {
				rows, _ := o["rows"].([]any)
				cols, _ := o["columns"].([]any)
				return o["delimiter"] == "\t" && o["header"] == true && o["delimGuessed"] == true && len(rows) == 1 && len(cols) == 2
			}},
		{"parse with no header", "parse", map[string]any{"input": "a,b\n", "header": "no", "delimiter": ","}, 200,
			func(o map[string]any) bool {
				return o["header"] == false && o["total"] == float64(1) && o["headerGuessed"] == false
			}},
		{"empty input", "parse", map[string]any{"input": ""}, 200,
			func(o map[string]any) bool { r, ok := o["rows"].([]any); return ok && len(r) == 0 }},
		{"convert to typed JSON", "convert", map[string]any{"input": "n,s\n1,x\n", "to": "json", "typed": true, "indent": "min"}, 200,
			func(o map[string]any) bool { return o["output"] == `[{"n":1,"s":"x"}]` }},
		{"convert to Markdown", "convert", map[string]any{"input": "n\n1\n", "to": "markdown"}, 200,
			func(o map[string]any) bool { return o["output"] == "| n |\n| --: |\n| 1 |\n" }},
		{"from JSON to TSV", "from-json", map[string]any{"input": `[{"a":1,"b":"x"}]`, "to": "tsv"}, 200,
			func(o map[string]any) bool { return o["output"] == "a\tb\n1\tx\n" }},
		{"a syntax error has its position", "parse", map[string]any{"input": "a,b\n1,x\"y\n", "delimiter": ","}, 422,
			func(o map[string]any) bool { return o["line"] == float64(2) && o["column"] == float64(4) }},
		{"a JSON error has its position", "from-json", map[string]any{"input": "[\n  {a: }\n]"}, 422,
			func(o map[string]any) bool { return o["line"] == float64(2) }},
		{"an unknown format", "convert", map[string]any{"input": "a", "to": "xml"}, 422, nil},
		{"a bad delimiter", "parse", map[string]any{"input": "a", "delimiter": "x"}, 400, nil},
		{"an unknown job", "nope", map[string]any{"input": "a"}, 404, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, out := post(tt.kind, tt.body)
			if code != tt.status {
				t.Fatalf("status %d, want %d: %v", code, tt.status, out)
			}
			if tt.check != nil && !tt.check(out) {
				t.Errorf("unexpected answer: %v", out)
			}
		})
	}
}
