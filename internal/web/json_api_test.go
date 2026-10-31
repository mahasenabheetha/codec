package web

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONAPI(t *testing.T) {
	s := New(testOptions(false))
	defer s.Close()
	post := func(kind string, body map[string]any) (int, map[string]any) {
		t.Helper()
		b, _ := json.Marshal(body)
		rec := call(s, "POST", "/api/v2/json/"+kind, "127.0.0.1:8765", s.Token(), "", string(b))
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
		{"format JSON5, sorted, one line", "format", map[string]any{"input": "{b: 1, a: [1,],}", "sort": true, "indent": "min"}, 200,
			func(o map[string]any) bool { return o["output"] == `{"a":[1],"b":1}` && o["dialect"] == "json5" }},
		{"format indents by default", "format", map[string]any{"input": `{"a":1}`}, 200,
			func(o map[string]any) bool { return o["output"] == "{\n  \"a\": 1\n}" }},
		{"to YAML", "to-yaml", map[string]any{"input": `{"a": [1]}`}, 200,
			func(o map[string]any) bool { return o["output"] == "a:\n  - 1\n" }},
		{"from YAML", "from-yaml", map[string]any{"input": "a: 1\nb: [x]\n", "indent": "min"}, 200,
			func(o map[string]any) bool { return o["output"] == `{"a":1,"b":["x"]}` }},
		{"flatten", "flatten", map[string]any{"input": `{"a": {"b": 1}}`}, 200,
			func(o map[string]any) bool { return o["output"] == ".a.b = 1\n" }},
		{"unflatten", "unflatten", map[string]any{"input": ".a.b = 1", "indent": "min"}, 200,
			func(o map[string]any) bool { return o["output"] == `{"a":{"b":1}}` }},
		{"diff", "diff", map[string]any{"input": `{"a": 1}`, "other": `{"a": 2}`}, 200,
			func(o map[string]any) bool {
				c, _ := o["changes"].([]any)
				return len(c) == 1
			}},
		{"diff, no changes, an empty list", "diff", map[string]any{"input": `[1]`, "other": `[1]`}, 200,
			func(o map[string]any) bool { c, ok := o["changes"].([]any); return ok && len(c) == 0 }},
		{"schema from JSON Lines", "schema", map[string]any{"input": "{\"a\":1}\n{\"a\":2}", "indent": "min"}, 200,
			func(o map[string]any) bool {
				return o["samples"] == float64(2) && strings.Contains(o["output"].(string), `"required":["a"]`)
			}},
		{"query", "query", map[string]any{"input": `{"a": [1, 2]}`, "expr": ".a[] * 10", "indent": "min"}, 200,
			func(o map[string]any) bool {
				r, _ := o["results"].([]any)
				return len(r) == 2 && r[1] == "20"
			}},
		{"stats", "stats", map[string]any{"input": `{"a": [1, null]}`}, 200,
			func(o map[string]any) bool { return o["keys"] == float64(1) && o["nulls"] == float64(1) }},
		{"a syntax error has its position", "format", map[string]any{"input": "{\n  \"a\" 1\n}"}, 422,
			func(o map[string]any) bool { return o["line"] == float64(2) && o["column"] == float64(7) }},
		{"a diff error says which side", "diff", map[string]any{"input": `{}`, "other": `{`}, 422,
			func(o map[string]any) bool { return strings.HasPrefix(o["error"].(string), "the right side") }},
		{"a bad filter", "query", map[string]any{"input": `{}`, "expr": ".a["}, 422, nil},
		{"a bad indent", "format", map[string]any{"input": `{}`, "indent": "xx"}, 400, nil},
		{"an unknown job", "nope", map[string]any{"input": `{}`}, 404, nil},
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
