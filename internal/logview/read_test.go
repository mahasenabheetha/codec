package logview

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite the .golden files from the fixtures")

// testNow fixes the year of klog and syslog times.
var testOpts = Options{Now: func() time.Time { return time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC) }}

// fixture reads a synthetic log; "␛" and "␉" stand for ESC and tab so
// the files stay readable.
func fixture(t testing.TB, name string) string {
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return strings.NewReplacer("␛", "\x1b", "␉", "\t").Replace(string(b))
}

// describe renders entries compactly, for golden files.
func describe(entries []Entry) string {
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "L%d+%d %s", e.Line, e.Lines, e.Format)
		if e.Level != LevelNone || e.RawLevel != "" {
			fmt.Fprintf(&b, " %s(%s)", e.Level, e.RawLevel)
		}
		if !e.Time.IsZero() {
			fmt.Fprintf(&b, " %s", e.Time.UTC().Format(time.RFC3339Nano))
		}
		if e.Source != "" {
			fmt.Fprintf(&b, " src=%s", e.Source)
		}
		fmt.Fprintf(&b, " msg=%q", e.Message)
		for _, kv := range [][2]string{{"logger", e.Logger}, {"caller", e.Caller}, {"err", e.Error}, {"trace", e.TraceID}} {
			if kv[1] != "" {
				fmt.Fprintf(&b, " %s=%q", kv[0], kv[1])
			}
		}
		b.WriteByte('\n')
		if e.Stack != "" {
			for _, l := range strings.Split(e.Stack, "\n") {
				fmt.Fprintf(&b, "    | %s\n", l)
			}
		}
		for _, f := range e.Fields {
			fmt.Fprintf(&b, "    %s:%s=%s\n", f.Key, kindNames[f.Kind], f.Text)
		}
		if strings.Contains(e.Raw, "\n") || strings.Contains(e.Raw, "\x1b") {
			fmt.Fprintf(&b, "    raw=%q\n", e.Raw)
		}
	}
	return b.String()
}

var kindNames = [...]string{"s", "n", "b", "null", "a", "o"}

// fixtures lists the sample logs: testdata's files but the goldens
// (and the fuzz corpus folder).
func fixtures() []string {
	entries, _ := os.ReadDir("testdata")
	var names []string
	for _, e := range entries {
		if !e.IsDir() && !strings.HasSuffix(e.Name(), ".golden") {
			names = append(names, e.Name())
		}
	}
	return names
}

func TestFixtures(t *testing.T) {
	for _, name := range fixtures() {
		t.Run(name, func(t *testing.T) {
			got := describe(ReadAll(fixture(t, name), testOpts))
			golden := filepath.Join("testdata", strings.TrimSuffix(name, filepath.Ext(name))+".golden")
			if *update {
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v (run with -update)", err)
			}
			if got != strings.ReplaceAll(string(want), "\r\n", "\n") {
				t.Errorf("%s changed; run go test -run TestFixtures -update and review the diff\ngot:\n%s", name, got)
			}
		})
	}
}

// TestEveryLineOnce checks that entries come in input order and that
// every non-blank line is covered once.
func TestEveryLineOnce(t *testing.T) {
	names, _ := filepath.Glob("testdata/*.log")
	for _, path := range names {
		text := fixture(t, filepath.Base(path))
		entries := ReadAll(text, testOpts)
		total := 0
		for _, l := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
			if strings.TrimSpace(l) != "" {
				total++
			}
		}
		sum, last := 0, 0
		for _, e := range entries {
			if e.Line <= last {
				t.Errorf("%s: entry at line %d after %d", path, e.Line, last)
			}
			last = e.Line
			sum += e.Lines
		}
		// Blank lines inside a Go panic belong to it.
		if sum < total || sum > total+strings.Count(text, "\n\n") {
			t.Errorf("%s: entries cover %d lines, the log has %d non-blank", path, sum, total)
		}
	}
}

func TestReadCases(t *testing.T) {
	for _, c := range []struct {
		name, in string
		want     string
	}{
		{"empty", "", ""},
		{"blank lines", "\n\n  \n", ""},
		{"crlf", "{\"msg\":\"a\",\"level\":\"info\"}\r\nplain\r\n", "L1+1 json info(info) msg=\"a\"\n    msg:s=a\n    level:s=info\nL2+1 text msg=\"plain\"\n"},
		{"truncated json", `{"msg":"cut off`, "L1+1 text msg=\"{\\\"msg\\\":\\\"cut off\"\n"},
		{"redraw", "10%\r50%\r100% done", "L1+1 text msg=\"100% done\"\n"},
		{"label words are not pods", "error: one\nerror: two\nwarning: three\n",
			"L1+1 text error(error) msg=\"one\"\nL2+1 text error(error) msg=\"two\"\nL3+1 text warn(warning) msg=\"three\"\n"},
		{"pasted pod line", `web-7d9c8b6f5-x2x9z: {"msg":"hi"}`, "L1+1 json src=web-7d9c8b6f5-x2x9z msg=\"hi\"\n    msg:s=hi\n"},
		{"word before json is text", `response: {"msg":"hi"}`, "L1+1 text msg=\"response: {\\\"msg\\\":\\\"hi\\\"}\"\n"},
		{"logfmt needs pairs", "user=bob logged in\n", "L1+1 text msg=\"user=bob logged in\"\n"},
		{"pretty json entry", "{\n  \"msg\": \"pretty\",\n  \"level\": \"warn\"\n}\n", "L1+4 json warn(warn) msg=\"pretty\"\n    msg:s=pretty\n    level:s=warn\n    raw=\"{\\n  \\\"msg\\\": \\\"pretty\\\",\\n  \\\"level\\\": \\\"warn\\\"\\n}\"\n"},
		{"json array", `[{"msg":"a"},{"msg":"b"}]`, "L1+1 json msg=\"a\"\n    msg:s=a\nL1+1 json msg=\"b\"\n    msg:s=b\n"},
		{"empty hits", `{"hits":{"hits":[]}}` + "\n" + `{"x":1}`, "L1+1 json msg=\"\"\n    hits.hits:a=[]\nL2+1 json msg=\"\"\n    x:n=1\n"},
		{"traceback alone", "Traceback (most recent call last):\n  File \"a.py\", line 1, in <module>\nKeyError: 'x'\n",
			"L1+3 text error() msg=\"KeyError: 'x'\" err=\"KeyError: 'x'\"\n    | Traceback (most recent call last):\n    |   File \"a.py\", line 1, in <module>\n    | KeyError: 'x'\n    raw=\"Traceback (most recent call last):\\n  File \\\"a.py\\\", line 1, in <module>\\nKeyError: 'x'\"\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := describe(ReadAll(c.in, testOpts)); got != c.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, c.want)
			}
		})
	}
}

// TestInterleavedStacks: a stack trace joins the entry of its own pod,
// even when another pod's lines come between.
func TestInterleavedStacks(t *testing.T) {
	in := "a-1234567890: 2026-10-22 10:00:00 ERROR failed\n" +
		"b-2345678901: 2026-10-22 10:00:00 INFO working\n" +
		"a-1234567890: java.lang.RuntimeException: boom\n" +
		"a-1234567890: \tat X.y(X.java:1)\n" +
		"b-2345678901: 2026-10-22 10:00:01 INFO still working\n" +
		"a-1234567890: \tat X.z(X.java:2)\n"
	entries := ReadAll(in, testOpts)
	if len(entries) != 3 {
		t.Fatalf("got %d entries:\n%s", len(entries), describe(entries))
	}
	a := entries[0]
	if a.Source != "a-1234567890" || a.Lines != 4 || a.Error != "java.lang.RuntimeException: boom" || strings.Count(a.Stack, "\n") != 2 {
		t.Errorf("pod a's entry:\n%s", describe(entries[:1]))
	}
	if entries[1].Line != 2 || entries[2].Line != 5 {
		t.Errorf("order:\n%s", describe(entries))
	}
}

func TestEarlyStop(t *testing.T) {
	n := 0
	for range Read("a\nb\nc\n", testOpts) {
		n++
		break
	}
	if n != 1 {
		t.Fatal(n)
	}
}

func FuzzRead(f *testing.F) {
	for _, name := range fixtures() {
		f.Add(fixture(f, name))
	}
	f.Fuzz(func(t *testing.T, s string) {
		lines := strings.Count(s, "\n") + 1
		sum, last := 0, 0
		// Items of a JSON document may share lines; read lines may not.
		_, doc := documentItems(s)
		for e := range Read(s, testOpts) {
			if e.Line < last {
				t.Fatalf("entry at line %d after %d", e.Line, last)
			}
			if e.Line < 1 || e.Line > lines || e.Lines < 1 {
				t.Fatalf("entry at line %d+%d of %d", e.Line, e.Lines, lines)
			}
			last = e.Line
			sum += e.Lines
		}
		if sum > lines && !doc {
			t.Fatalf("entries cover %d lines of %d", sum, lines)
		}
	})
}

// benchLog is n ECS lines, as an instrumented service writes them.
func benchLog(n int) string {
	var b strings.Builder
	for i := range n {
		level := "info"
		if i%50 == 0 {
			level = "error"
		}
		fmt.Fprintf(&b, `{"@timestamp":"2026-10-22T10:%02d:%02d.%03dZ","log.level":"%s","message":"handled request %d","ecs.version":"1.6.0","service.name":"orders","http":{"request":{"method":"GET"},"response":{"status_code":%d}},"url.path":"/api/orders/%d","event.duration":%d,"trace.id":"4bf92f3577b34da6a3ce929d0e0e%04d"}`+"\n",
			i/60%60, i%60, i%1000, level, i, 200+i%3*100, i, i*1000, i%10000)
	}
	return b.String()
}

func BenchmarkRead100k(b *testing.B) {
	text := benchLog(100_000)
	b.SetBytes(int64(len(text)))
	for b.Loop() {
		for range Read(text, testOpts) {
		}
	}
}
