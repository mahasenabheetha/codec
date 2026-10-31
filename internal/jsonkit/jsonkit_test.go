package jsonkit

import (
	"context"
	"strings"
	"testing"
)

func mustParse(t *testing.T, src string) *Value {
	t.Helper()
	v, _, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return v
}

func TestParseDialects(t *testing.T) {
	tests := []struct {
		name, src, want string
		dialect         Dialect
	}{
		{"strict JSON", `{"a": [1, 2.5, -3e2], "b": null, "c": true}`, `{"a":[1,2.5,-3e2],"b":null,"c":true}`, JSON},
		{"key order kept", `{"z": 1, "a": 2, "m": 3}`, `{"z":1,"a":2,"m":3}`, JSON},
		{"big integer exact", `12345678901234567890123`, `12345678901234567890123`, JSON},
		{"line comment", "{\"a\": 1 // one\n}", `{"a":1}`, JSONC},
		{"block comment", `/* settings */ {"a": 1}`, `{"a":1}`, JSONC},
		{"trailing commas", `{"a": [1, 2,],}`, `{"a":[1,2]}`, JSONC},
		{"unquoted keys", `{name: 'api', $ref: 1, _x: 2}`, `{"name":"api","$ref":1,"_x":2}`, JSON5},
		{"single quotes and escapes", `'it\'s \x41 B'`, `"it's A B"`, JSON5},
		{"hex, plus and points", `[0x1F, +5, .5, 5., -0xA]`, `[31,5,0.5,5,-10]`, JSON5},
		{"line continuation", "'one \\\ntwo'", `"one two"`, JSON5},
		{"surrogate pair", `"\ud83d\ude00"`, `"😀"`, JSON},
		{"repeated key: last wins, first place", `{"a": 1, "b": 2, "a": 3}`, `{"a":3,"b":2}`, JSON},
		{"BOM", "\ufeff[1]", `[1]`, JSON},
		{"identity escape", `"\q"`, `"q"`, JSON5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, d, err := Parse(tt.src)
			if err != nil {
				t.Fatal(err)
			}
			if got := Encode(v, ""); got != tt.want {
				t.Errorf("Encode = %s, want %s", got, tt.want)
			}
			if d != tt.dialect {
				t.Errorf("dialect = %s, want %s", d, tt.dialect)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name, src  string
		line, col  int
		msgContain string
	}{
		{"missing comma", "{\n  \"a\": 1\n  \"b\": 2\n}", 3, 3, "expected , or }"},
		{"unclosed object", `{"a": 1`, 1, 1, "not closed"},
		{"unclosed string", `["abc`, 1, 2, "not closed"},
		{"leading zero", `[01]`, 1, 2, "start with 0"},
		{"NaN", `[NaN]`, 1, 2, "no JSON form"},
		{"Infinity", `-Infinity`, 1, 1, "no JSON form"},
		{"text after", `{} x`, 1, 4, "after the value"},
		{"raw line break in string", "\"a\nb\"", 1, 3, "line break"},
		{"digit escape", `"\1"`, 1, 2, "unknown escape"},
		{"unclosed comment", `/* x`, 1, 1, "comment"},
		{"empty", ``, 1, 1, "ends"},
		{"columns count characters", `{"é": 1 2}`, 1, 9, "expected , or }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Parse(tt.src)
			se, ok := err.(*SyntaxError)
			if !ok {
				t.Fatalf("err = %v, want a SyntaxError", err)
			}
			if se.Line != tt.line || se.Column != tt.col || !strings.Contains(se.Msg, tt.msgContain) {
				t.Errorf("got %d:%d %q, want %d:%d containing %q", se.Line, se.Column, se.Msg, tt.line, tt.col, tt.msgContain)
			}
		})
	}
}

func TestParseDepthLimit(t *testing.T) {
	deep := strings.Repeat("[", maxDepth+5) + strings.Repeat("]", maxDepth+5)
	if _, _, err := Parse(deep); err == nil || !strings.Contains(err.Error(), "nested deeper") {
		t.Errorf("deep nesting: %v", err)
	}
}

func TestParseAll(t *testing.T) {
	vs, _, err := ParseAll("{\"a\":1}\n{\"a\":2}\n\n[3]\n")
	if err != nil || len(vs) != 3 {
		t.Fatalf("ParseAll: %d values, %v", len(vs), err)
	}
}

func TestEncodeIndentAndEscapes(t *testing.T) {
	v := mustParse(t, `{"a": [1, {"b": "<x> & \"q\"\n\u0001\u2028"}], "e": {}, "f": []}`)
	want := "{\n  \"a\": [\n    1,\n    {\n      \"b\": \"<x> & \\\"q\\\"\\n\\u0001\\u2028\"\n    }\n  ],\n  \"e\": {},\n  \"f\": []\n}"
	if got := Encode(v, "  "); got != want {
		t.Errorf("Encode =\n%s\nwant\n%s", got, want)
	}
}

func TestSortKeys(t *testing.T) {
	v := mustParse(t, `{"b": {"y": 1, "x": 2}, "a": [{"d": 1, "c": 2}]}`)
	if got := Encode(SortKeys(v), ""); got != `{"a":[{"c":2,"d":1}],"b":{"x":2,"y":1}}` {
		t.Errorf("SortKeys = %s", got)
	}
	if Encode(v, "") != `{"b":{"y":1,"x":2},"a":[{"d":1,"c":2}]}` {
		t.Error("SortKeys changed its input")
	}
}

func TestFlatten(t *testing.T) {
	v := mustParse(t, `{"spec": {"containers": [{"image": "app:1"}], "labels": {"app.kubernetes.io/name": "api", "a b": true}}, "e": [], "o": {}}`)
	want := `.spec.containers[0].image = "app:1"
.spec.labels["app.kubernetes.io/name"] = "api"
.spec.labels["a b"] = true
.e = []
.o = {}
`
	if got := FlattenText(v); got != want {
		t.Errorf("FlattenText =\n%s\nwant\n%s", got, want)
	}
	back, err := Unflatten(want)
	if err != nil {
		t.Fatal(err)
	}
	if Encode(back, "") != Encode(v, "") {
		t.Errorf("round trip = %s", Encode(back, ""))
	}
	if got := FlattenText(mustParse(t, `42`)); got != ". = 42\n" {
		t.Errorf("scalar = %q", got)
	}
}

func TestUnflatten(t *testing.T) {
	v, err := Unflatten("# a comment\n.items[2] = 3\n.items[0] = 1\n\n.name = \"x\"")
	if err != nil {
		t.Fatal(err)
	}
	if got := Encode(v, ""); got != `{"items":[1,null,3],"name":"x"}` {
		t.Errorf("Unflatten = %s", got)
	}
	for _, bad := range []string{".a = 1\n.a.b = 2", ".a = ", "a = 1", ".a[x] = 1", ".a[999999999] = 1", ".a = 1\n.a[0] = 2"} {
		if _, err := Unflatten(bad); err == nil {
			t.Errorf("Unflatten(%q) accepted", bad)
		}
	}
}

func TestDiff(t *testing.T) {
	a := mustParse(t, `{"name": "api", "replicas": 2, "ports": [80, 443], "env": {"A": "1"}, "n": 1.0, "t": "3"}`)
	b := mustParse(t, `{"name": "api", "replicas": 3, "ports": [80], "env": {"A": "1", "B": "2"}, "n": 1, "t": 3, "new": true}`)
	got := Diff(a, b)
	want := []Change{
		{Path: ".replicas", Kind: "changed", Old: "2", New: "3"},
		{Path: ".ports[1]", Kind: "removed", Old: "443"},
		{Path: ".env.B", Kind: "added", New: `"2"`},
		{Path: ".t", Kind: "type", Old: `"3"`, New: "3"},
		{Path: ".new", Kind: "added", New: "true"},
	}
	if len(got) != len(want) {
		t.Fatalf("Diff = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("change %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if d := Diff(a, a); len(d) != 0 {
		t.Errorf("Diff with itself = %+v", d)
	}
}

func TestInfer(t *testing.T) {
	samples, _, err := ParseAll(`{"id": 1, "email": "a@example.com", "tags": ["x"], "when": "2026-10-31T12:00:00Z", "score": 1}
{"id": 2, "email": "b@example.com", "tags": [], "score": 2.5, "note": null}`)
	if err != nil {
		t.Fatal(err)
	}
	got := Encode(Infer(samples), "")
	want := `{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","properties":{"id":{"type":"integer"},"email":{"type":"string","format":"email"},"tags":{"type":"array","items":{"type":"string"}},"when":{"type":"string","format":"date-time"},"score":{"type":"number"},"note":{"type":"null"}},"required":["id","email","tags","score"]}`
	if got != want {
		t.Errorf("Infer =\n%s\nwant\n%s", got, want)
	}
	mixed := Encode(Infer([]*Value{mustParse(t, `"a"`), mustParse(t, `1`)}), "")
	if !strings.Contains(mixed, `"type":["string","integer"]`) {
		t.Errorf("mixed types = %s", mixed)
	}
}

func TestQuery(t *testing.T) {
	v := mustParse(t, `{"items": [{"name": "a", "n": 12345678901234567890}, {"name": "b", "n": 2}]}`)
	r, err := Query(context.Background(), v, `.items[] | {name, n}`, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(r.Results, "|") != `{"n":12345678901234567890,"name":"a"}|{"n":2,"name":"b"}` {
		t.Errorf("Query = %v", r.Results)
	}
	if _, err := Query(context.Background(), v, `.items[`, ""); err == nil || !strings.Contains(err.Error(), "character") {
		t.Errorf("bad filter: %v", err)
	}
	if _, err := Query(context.Background(), v, `env`, ""); err != nil {
		t.Errorf("env: %v", err) // allowed, but empty: no environment leaks in
	}
	many, _ := Query(context.Background(), v, `range(5000)`, "")
	if len(many.Results) != QueryLimit || !many.Truncated {
		t.Errorf("limit: %d results, truncated %v", len(many.Results), many.Truncated)
	}
}

func TestYAML(t *testing.T) {
	v := mustParse(t, `{"b": 1, "a": {"list": [1, "two"]}}`)
	y, err := ToYAML(v)
	if err != nil {
		t.Fatal(err)
	}
	if y != "b: 1\na:\n  list:\n    - 1\n    - two\n" {
		t.Errorf("ToYAML =\n%s", y)
	}
	back, err := FromYAML(y)
	if err != nil || Encode(back, "") != Encode(v, "") {
		t.Errorf("FromYAML = %v, %v", back, err)
	}
	if _, err := FromYAML("a: [1, 2\n"); err == nil {
		t.Error("broken YAML accepted")
	}
}

func TestScan(t *testing.T) {
	st, err := Scan(strings.NewReader(`{"a": [1, "x", {"b": null}], "c": true}` + "\n" + `[]`))
	if err != nil {
		t.Fatal(err)
	}
	want := Stats{Bytes: st.Bytes, Values: 2, Objects: 2, Arrays: 2, Keys: 3, Strings: 1, Numbers: 1, Bools: 1, Nulls: 1, MaxDepth: 3}
	if st != want {
		t.Errorf("Scan = %+v, want %+v", st, want)
	}
	if _, err := Scan(strings.NewReader(`{"a": }`)); err == nil {
		t.Error("broken JSON scanned")
	}
}
