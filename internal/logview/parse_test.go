package logview

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

func TestParseObject(t *testing.T) {
	for _, c := range []struct {
		in   string
		want string // key=kind:text, one per field; "!" for invalid
	}{
		{`{}`, ""},
		{` {"a":1} `, "a=n:1"},
		{`{"a":{"b":{"c":"x"}},"d":[1,{"e":2}]}`, "a.b.c=s:x d=a:[1,{\"e\":2}]"},
		{`{"s":"tab\there \"q\" \u00e9 \ud83d\ude00 \ud800"}`, "s=s:tab\there \"q\" é 😀 \ufffd"},
		{`{"t":true,"f":false,"n":null,"x":-1.5e3}`, "t=b:true f=b:false n=null:null x=n:-1.5e3"},
		{`{"e":{}}`, "e=o:{}"},
		{"{\"p\": [ 1,\n 2 ]}", "p=a:[1,2]"},
		{`{"dup":1,"dup":2}`, "dup=n:1 dup=n:2"},
		{`{"a":1,}`, "!"},
		{`{"a":01}`, "!"},
		{`{"a":"ctl` + "\x01" + `"}`, "!"},
		{`{"a":"\x"}`, "!"},
		{`{"a":1} x`, "!"},
		{`{"a":tru}`, "!"},
		{`[1]`, "!"},
	} {
		fields, ok := parseObject(c.in, nil)
		var got []string
		for _, f := range fields {
			got = append(got, fmt.Sprintf("%s=%s:%s", f.Key, kindNames[f.Kind], f.Text))
		}
		g := strings.Join(got, " ")
		if !ok {
			g = "!"
		}
		if g != c.want {
			t.Errorf("%s:\ngot  %s\nwant %s", c.in, g, c.want)
		}
	}
	deep := strings.Repeat(`{"a":`, 20) + "1" + strings.Repeat("}", 20)
	fields, ok := parseObject(deep, nil)
	if !ok || len(fields) != 1 || fields[0].Kind != Object || strings.Count(fields[0].Key, ".") != maxFlatten-1 {
		t.Errorf("deep object: %v %+v", ok, fields)
	}
	if _, ok := parseObject(`{"a":`+strings.Repeat("[", maxDepth+1)+strings.Repeat("]", maxDepth+1)+`}`, nil); ok {
		t.Error("nesting past encoding/json's limit was accepted")
	}
}

// FuzzParseObject: the scanner accepts exactly the objects encoding/json
// accepts, and reads the same values.
func FuzzParseObject(f *testing.F) {
	for _, s := range []string{`{}`, `{"a":"b\u00e9","c":[1,2,{"d":null}],"e":{"f":-0.5e+2}}`, `{"a":"\ud83d\ude00\ud800x"}`, `{"a":{}}`, `{"a" : 1 , "b":true}`} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		fields, ok := parseObject(s, nil)
		var v map[string]any
		err := json.Unmarshal([]byte(s), &v)
		valid := json.Valid([]byte(s)) && strings.HasPrefix(strings.TrimLeft(s, " \t\r\n"), "{")
		if ok != valid {
			t.Fatalf("parseObject=%v, encoding/json valid=%v for %q", ok, valid, s)
		}
		if !ok || err != nil || !utf8.ValidString(s) {
			return
		}
		// Each flattened top-level string field matches encoding/json's
		// value when its key is unique and has no dots.
		seen := map[string]int{}
		for _, f := range fields {
			seen[strings.SplitN(f.Key, ".", 2)[0]]++
		}
		for _, f := range fields {
			if strings.Contains(f.Key, ".") || seen[f.Key] > 1 || f.Kind != String {
				continue
			}
			if want, ok := v[f.Key].(string); ok && want != f.Text {
				t.Fatalf("%q: got %q, encoding/json %q", f.Key, f.Text, want)
			}
		}
	})
}

func TestArrayItems(t *testing.T) {
	s := ` [ {"a":1}, "x" ,[2]] `
	spans, ok := arrayItems(s)
	var got []string
	for _, sp := range spans {
		got = append(got, s[sp[0]:sp[1]])
	}
	if !ok || !slices.Equal(got, []string{`{"a":1}`, `"x"`, `[2]`}) {
		t.Errorf("%v %q", ok, got)
	}
	for _, bad := range []string{`[1,]`, `[1 2]`, `{"a":1}`, `[`} {
		if _, ok := arrayItems(bad); ok {
			t.Errorf("%s accepted", bad)
		}
	}
}

func TestParseLogfmt(t *testing.T) {
	for _, c := range []struct {
		in           string
		want         string
		pairs, words int
		ok           bool
	}{
		{`time="2026-10-22T10:00:00Z" level=info msg="a \"quoted\" word" n=3 ok=true`, `time=2026-10-22T10:00:00Z level=info msg=a "quoted" word n=3 ok=true`, 5, 0, true},
		{`level=warn msg=bare k=a=b flag`, `level=warn msg=bare k=a=b`, 3, 1, true},
		{`msg="tab\there\u00e9"`, "msg=tab\thereé", 1, 0, true},
		{`msg="unterminated`, ``, 1, 0, false},
		{`="no key"`, ``, 0, 0, false},
		{`a= b=`, `a= b=`, 2, 0, true},
	} {
		fields, pairs, words, ok := parseLogfmt(c.in, nil)
		var got []string
		for _, f := range fields {
			got = append(got, f.Key+"="+f.Text)
		}
		if g := strings.Join(got, " "); ok != c.ok || ok && (g != c.want || pairs != c.pairs || words != c.words) {
			t.Errorf("%s:\ngot  %v %d %d %s\nwant %v %d %d %s", c.in, ok, pairs, words, g, c.ok, c.pairs, c.words, c.want)
		}
	}
	fields, _, _, _ := parseLogfmt(`n=42 f=1.5 b=false s=12ms`, nil)
	kinds := []Kind{Number, Number, Bool, String}
	for i, f := range fields {
		if f.Kind != kinds[i] {
			t.Errorf("%s: kind %d, want %d", f.Key, f.Kind, kinds[i])
		}
	}
}

func TestParseLevel(t *testing.T) {
	for in, want := range map[string]Level{
		"INFO": Info, "Information": Info, "notice": Info, "30": Info, "I": Info,
		"WARNING": Warn, "warn": Warn, "W": Warn, "40": Warn,
		"err": Error, "SEVERE": Error, "50": Error,
		"dpanic": Fatal, "CRITICAL": Fatal, "emerg": Fatal, "60": Fatal,
		"verbose": Trace, "DBG": Debug, " debug ": Debug,
		"": LevelNone, "35": LevelNone, "loud": LevelNone,
	} {
		if got := ParseLevel(in); got != want {
			t.Errorf("ParseLevel(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestTimes(t *testing.T) {
	utc := func(s string) time.Time {
		tm, err := time.Parse(time.RFC3339Nano, s)
		if err != nil {
			t.Fatal(err)
		}
		return tm
	}
	for in, want := range map[string]string{
		"2026-10-22T10:00:00.123456789Z":  "2026-10-22T10:00:00.123456789Z",
		"2026-10-22T12:00:00+02:00":       "2026-10-22T10:00:00Z",
		"2026-10-22 10:00:00,250":         "2026-10-22T10:00:00.25Z",
		"2026-10-22T10:00:00.5+0200":      "2026-10-22T08:00:00.5Z",
		"2026-10-22 10:00:00.5 +0000 UTC": "2026-10-22T10:00:00.5Z",
		"2026/10/22 10:00:00":             "2026-10-22T10:00:00Z",
		"22/Oct/2026:10:00:00 +0000":      "2026-10-22T10:00:00Z",
		"Thu, 22 Oct 2026 10:00:00 +0000": "2026-10-22T10:00:00Z",
		"1792656000":                      "2026-10-22T08:00:00Z",
		"1792656000.123456":               "2026-10-22T08:00:00.123456Z",
		"1792656000123":                   "2026-10-22T08:00:00.123Z",
		"1792656000123.5":                 "2026-10-22T08:00:00.1235Z",
		"1792656000123456":                "2026-10-22T08:00:00.123456Z",
		"1792656000123456789":             "2026-10-22T08:00:00.123456789Z",
		"1.792656e9":                      "2026-10-22T08:00:00Z",
	} {
		var got time.Time
		var ok bool
		if isNumber(in) {
			got, ok = epochText(in)
		} else {
			got, ok = parseTime(in)
		}
		if !ok || !got.Equal(utc(want)) {
			t.Errorf("%s: got %v %v, want %s", in, got, ok, want)
		}
	}
	for _, in := range []string{"5", "1.2", "2026", "not a time", "0000-01-01T00:00:00Z", "-5"} {
		if isNumber(in) {
			if _, ok := epochText(in); ok {
				t.Errorf("%s read as a time", in)
			}
		} else if _, ok := parseTime(in); ok {
			t.Errorf("%s read as a time", in)
		}
	}
}

func TestTextLevel(t *testing.T) {
	for _, c := range []struct{ in, level, msg string }{
		{"[INFO] fetching", "info", "fetching"},
		{"ERROR: bad thing", "error", "bad thing"},
		{"ERROR:root:budget gone", "error", "budget gone"},
		{"warning: lower case with colon", "warn", "lower case with colon"},
		{"Error connecting to db", "error", "connecting to db"},
		{"[main] INFO c.e.App - started", "info", "[main] INFO c.e.App - started"},
		{"error happened somewhere", "", "error happened somewhere"},
		{"I think so", "", "I think so"},
		{"40 items left", "", "40 items left"},
		{"a b c d INFO too far", "", "a b c d INFO too far"},
	} {
		l, _, msg := textLevel(c.in)
		if l.String() != c.level || msg != c.msg {
			t.Errorf("%q: got %s %q, want %s %q", c.in, l, msg, c.level, c.msg)
		}
	}
}

func TestDecode(t *testing.T) {
	plain := "line one\nline two é\n"
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	w.Write([]byte(plain))
	w.Close()
	le := []byte{0xff, 0xfe}
	be := []byte{0xfe, 0xff}
	for _, u := range utf16.Encode([]rune(plain)) {
		le = append(le, byte(u), byte(u>>8))
		be = append(be, byte(u>>8), byte(u))
	}
	for name, c := range map[string]struct {
		in  []byte
		enc []string
	}{
		"plain":     {[]byte(plain), nil},
		"utf-8 bom": {append([]byte{0xef, 0xbb, 0xbf}, plain...), nil},
		"gzip":      {gz.Bytes(), []string{"gzip"}},
		"utf-16le":  {le, []string{"utf-16le"}},
		"utf-16be":  {be, []string{"utf-16be"}},
	} {
		got, enc, err := Decode(c.in, 0)
		if err != nil || got != plain || !slices.Equal(enc, c.enc) {
			t.Errorf("%s: %q %v %v", name, got, enc, err)
		}
	}
	if _, _, err := Decode(gz.Bytes(), 5); !errors.Is(err, ErrTooLarge) {
		t.Errorf("limit: %v", err)
	}
	if _, _, err := Decode([]byte{0x1f, 0x8b, 0}, 0); err == nil {
		t.Error("broken gzip accepted")
	}
}

func TestLearnLabel(t *testing.T) {
	for want, sample := range map[labelShape][]string{
		podLabel:     {"wf-a-123: x", "wf-b-456: y", "plain", "wf-a-123:"},
		composeLabel: {"web-1  | x", "db-1   | y"},
		kubectlLabel: {"[pod/web-1/app] x", "[pod/web-2/app] y"},
		sternLabel:   {"web-7d9c8b6f5-x2x9z app {\"a\":1}", "web-7d9c8b6f5-x2x9z app hello"},
		noLabel:      {"error: x", "warning: y", "note: z", "response: {}", "plain"},
	} {
		if got := learnLabel(sample); got != want {
			t.Errorf("%q: got %d, want %d", sample, got, want)
		}
	}
}

func TestRenderTemplate(t *testing.T) {
	fields := []Field{{Key: "Count", Text: "12"}, {Key: "User", Text: "ann"}}
	for in, want := range map[string]string{
		"Processed {Count} for {@User}": "Processed 12 for ann",
		"{Count:000} {Missing} {{x}}":   "12 {Missing} {x}",
		"no holes":                      "no holes",
		"broken {":                      "broken {",
	} {
		if got := renderTemplate(in, fields); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

// Order-preserving reading should be at least as fast as encoding/json's
// map decoding, which loses the order (decision 86).
func BenchmarkParseObject(b *testing.B) {
	line := strings.SplitN(benchLog(1), "\n", 2)[0]
	b.Run("logview", func(b *testing.B) {
		b.SetBytes(int64(len(line)))
		fields := make([]Field, 0, 16)
		for b.Loop() {
			parseObject(line, fields[:0])
		}
	})
	b.Run("encoding/json", func(b *testing.B) {
		b.SetBytes(int64(len(line)))
		for b.Loop() {
			var v map[string]any
			json.Unmarshal([]byte(line), &v)
			_ = slices.Collect(maps.Keys(v))
		}
	})
}
