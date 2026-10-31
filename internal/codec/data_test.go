package codec

import (
	"strings"
	"testing"
	"time"
)

// FuzzDetectUtility: smart paste's first detector never panics.
func FuzzDetectUtility(f *testing.F) {
	for _, s := range []string{"a,b\n1,2\n3,4", "{a: 1,}", "// x\n[1]", "*/5 * * * *", "10.0.0.0/8", "a\tb\n\"x\n"} {
		f.Add(s)
	}
	now := time.Date(2026, 2, 3, 12, 0, 0, 0, time.UTC)
	f.Fuzz(func(t *testing.T, s string) {
		DetectUtility(s, now)
	})
}

// 3.7.0: JSONC and JSON5 go to JSON & YAML, CSV and TSV to Text & data.
func TestDetectData(t *testing.T) {
	now := time.Date(2026, 2, 3, 12, 0, 0, 0, time.UTC)
	pythonLog := "2026-10-22 10:00:00,123 INFO started\n2026-10-22 10:00:01,456 ERROR failed\n2026-10-22 10:00:02,789 INFO retried"
	for _, c := range []struct {
		in, kind, tab, has string
	}{
		{"{\n  // port\n  \"port\": 8080,\n}", "jsonc", "format", `"port": 8080`},
		{"{name: 'api', replicas: 3}", "json5", "format", `"name": "api"`},
		{"// a list\n[1, 2,]", "jsonc", "format", "1,"},
		{`{"a":"x%20y",}`, "jsonc", "format", "x%20y"},
		{"id,name,score\n1,ann,3.5\n2,bob,\n", "csv", "table", "CSV · comma · 2 rows · 3 columns · a header row"},
		{"a\tb\n1\t2\n3\t4\n", "tsv", "table", "b  integer"},
		{"name;price\nmilk;1,5\negg;2,25\n", "csv", "table", "semicolon"},
		{"when,level,msg\n2026-10-22T10:00:00Z,info,up\n2026-10-22T10:00:01Z,error,down\n", "csv", "table", "when   date"},
	} {
		u, ok := DetectUtility(c.in, now)
		if !ok || u.Kind != c.kind || u.Tab != c.tab || !strings.Contains(u.Summary, c.has) {
			t.Errorf("%q: %+v %v", c.in, u, ok)
		}
	}
	for _, in := range []string{
		`{"strict": "json"}`, // the JSON detector's
		"[1, 2]",
		"'just a string'",
		"a,b\n1,2",                         // two lines only
		"one, two\nthree\nfour, five, six", // uneven
		"Dear team, thanks for the review today and for all the comments.\nWe fixed the build, and the tests pass on every platform now.\nNext week, we release it to everyone who asked for it.",
		pythonLog,
		"{\"level\":\"info\",\"msg\":\"a,b\"}\n{\"level\":\"info\",\"msg\":\"c,d\"}\n{\"level\":\"info\",\"msg\":\"e,f\"}",
	} {
		if u, ok := DetectUtility(in, now); ok && (u.Tool == "json" || u.Tool == "text") {
			t.Errorf("%q detected as %s", in, u.Kind)
		}
	}
	if u, _ := DetectUtility(pythonLog, now); u == nil || u.Kind != "log" {
		t.Errorf("a Python log: %+v", u)
	}
}
