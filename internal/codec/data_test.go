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

// 3.8.0: UUIDs and ULIDs go to Encode & hash → UUID, hex colours to Colour.
func TestDetectIDsAndColours(t *testing.T) {
	now := time.Date(2026, 2, 3, 12, 0, 0, 0, time.UTC)
	for _, c := range []struct{ in, kind, tool, tab, has string }{
		{"017f22e2-79b0-7cc3-98c4-dc0c0c07398f", "uuid", "encode", "uuid", "Made     2022-02-22T19:22:22"},
		{"{C232AB00-9414-11EC-B3C8-9F6BDECED846}", "uuid", "encode", "uuid", "Version  1"},
		{"urn:uuid:2ed6657d-e927-568b-95e1-2665a8aea6a2", "uuid", "encode", "uuid", "name-based, SHA-1"},
		{"01ARZ3NDEKTSV4RRFFQ69G5FAV", "ulid", "encode", "uuid", "2016-07-30T23:54:10.259Z"},
		{"#4f46e5", "colour", "colour", "", "rgb(79, 70, 229)"},
		{"#FFF", "colour", "colour", "", "hsl(0, 0%, 100%)"},
		{"#4f46e580", "colour", "colour", "", "Alpha 0.50"},
	} {
		u, ok := DetectUtility(c.in, now)
		if !ok || u.Kind != c.kind || u.Tool != c.tool || u.Tab != c.tab || !strings.Contains(u.Summary, c.has) {
			t.Errorf("%q: %+v %v", c.in, u, ok)
		}
	}
	for _, in := range []string{
		"900150983cd24fb0d6963f7d28e17f72", // an MD5, not a UUID
		"ABCDEFGHJKMNPQRSTVWXYZABCD",       // 26 letters: a word, not a ULID
		"01234567890123456789012345",       // 26 digits
		"#12345",                           // five hex digits
		"4f46e5",                           // hex without #
		"#hashtag",
	} {
		if u, ok := DetectUtility(in, now); ok && (u.Kind == "uuid" || u.Kind == "ulid" || u.Kind == "colour") {
			t.Errorf("%q detected as %s", in, u.Kind)
		}
	}
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
