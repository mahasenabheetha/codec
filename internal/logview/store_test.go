package logview

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func buildFixtures(t testing.TB, names ...string) *Log {
	var files []File
	for _, n := range names {
		files = append(files, File{Name: n, Data: []byte(fixture(t, n))})
	}
	l, err := Build(files, BuildOptions{Options: testOpts})
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// where names matches as "file:line".
func where(l *Log, ids []uint32) string {
	var out []string
	for _, i := range ids {
		out = append(out, fmt.Sprintf("%s:%d", strings.TrimSuffix(l.Files[l.File(int(i))].Name, ".log"), l.Entry(int(i)).Line))
	}
	return strings.Join(out, " ")
}

func TestQueries(t *testing.T) {
	l := buildFixtures(t, "ecs.log", "zap.log", "logback.log", "argo.log")
	for _, c := range []struct{ q, want string }{
		{"level:error", "zap:2 ecs:3 logback:2 argo:5"},
		{"level>=error", "zap:2 zap:3 ecs:3 logback:2 argo:5 argo:6"},
		{"level<info", ""},
		{"level:(warn OR fatal)", "zap:3 ecs:2 logback:9 argo:6"},
		{`"PAYMENT failed"`, "ecs:3"},
		{"payment", "ecs:3 logback:2"},
		{"payment -source:ecs.log", "logback:2"},
		{"NOT level:* AND source:*checkout*", "argo:2 argo:4"},
		{"status>=500", "zap:2"},
		{"status:200 OR latency<0.01", "zap:1"},
		{"http.port:8080", "ecs:1"},
		{"HTTP.PORT:8080", "ecs:1"},
		{"tags:retry", "ecs:3"},
		{"trace:4bf92f3577b34da6a3ce929d0e0e4736", "ecs:2"},
		{"trace.id:0af7*", "ecs:3"},
		{"error:timed", "ecs:3 argo:6"},
		{"stack:*", "zap:2 ecs:3 logback:2 argo:6"},
		{"stack:IOException", "logback:2"},
		{"logger:cache", "zap:3"},
		{"caller:server.go*", "ecs:1"},
		{"message:retry*", ""},
		{"format:logfmt", "argo:1 argo:7"},
		{"file:zap.log level:info", "zap:1"},
		{`/time[sd] ?out/`, "ecs:3 argo:6"},
		{"(payment OR panic) level>=error", "zap:3 ecs:3 logback:2 argo:6"}, // dpanic,
		{"http://example.com/x", ""},
		{"event.duration>1e9", "ecs:2"}, // ECS durations are nanoseconds,
		{"message.order.status:paid", "ecs:4"},
	} {
		m, err := l.Match(c.q, time.Time{}, time.Time{})
		if err != nil {
			t.Errorf("%s: %v", c.q, err)
			continue
		}
		if got := where(l, m.IDs); got != c.want {
			t.Errorf("%s:\ngot  %s\nwant %s", c.q, got, c.want)
		}
	}
}

func TestQueryErrors(t *testing.T) {
	l := buildFixtures(t, "zap.log")
	for q, want := range map[string]string{
		`"open`:      "this quote is not closed (at 1)",
		`(a OR b`:    "this ( is not closed (at 1)",
		`a )`:        `unexpected ")" (at 3)`,
		`/[/`:        "regex: missing closing ]: `[` (at 1)",
		`status:`:    "a value is missing (at 8)",
		`level:loud`: `"loud" is not a level: use trace, debug, info, warn, error or fatal (at 1)`,
		`NOT`:        "a term is missing (at 4)",
		`a OR`:       "a term is missing (at 5)",
		`level:(`:    "this ( is not closed (at 7)",
	} {
		_, err := l.Match(q, time.Time{}, time.Time{})
		var qe *QueryError
		if !errors.As(err, &qe) || err.Error() != want {
			t.Errorf("%s: got %v, want %s", q, err, want)
		}
	}
	m, err := l.Match("nosuch>3 status:200", time.Time{}, time.Time{})
	if err != nil || m.Total != 0 || len(m.Warnings) != 1 || m.Warnings[0] != `no field is named "nosuch"` {
		t.Errorf("unknown field: %+v %v", m, err)
	}
	if m, _ := l.Match("", time.Time{}, time.Time{}); m.Total != 3 {
		t.Errorf("empty query: %d", m.Total)
	}
}

func TestTimeRange(t *testing.T) {
	l := buildFixtures(t, "ecs.log")
	from, _ := time.Parse(time.RFC3339, "2026-10-22T08:15:02Z")
	to, _ := time.Parse(time.RFC3339, "2026-10-22T08:15:04Z")
	m, _ := l.Match("", from, to)
	if got := where(l, m.IDs); got != "ecs:2 ecs:3" {
		t.Errorf("got %s", got)
	}
}

// TestMerge: entries of two files come in time order; a line without a
// time stays with the entry before it in its file.
func TestMerge(t *testing.T) {
	a := File{Name: "a", Data: []byte("2026-10-22T10:00:00Z one\n2026-10-22T10:00:02Z three\nno time, after three\n")}
	b := File{Name: "b", Data: []byte("2026-10-22T10:00:01Z two\n2026-10-22T10:00:03Z four\n")}
	l, err := Build([]File{a, b}, BuildOptions{Options: testOpts})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for i := range l.Len() {
		e := l.Entry(i)
		got = append(got, e.Source+":"+e.Message)
	}
	if want := "a:one b:two a:three a:no time, after three b:four"; strings.Join(got, " ") != want {
		t.Errorf("got %s", strings.Join(got, " "))
	}
	if _, err := Build([]File{a, b}, BuildOptions{Limit: 100}); !errors.Is(err, ErrTooLarge) {
		t.Errorf("limit: %v", err)
	}
}

func TestEntryRoundTrip(t *testing.T) {
	text := fixture(t, "ecs.log") + fixture(t, "logback.log")
	l, _ := Build([]File{{Name: "x", Data: []byte(text)}}, BuildOptions{Options: testOpts})
	want := ReadAll(text, testOpts)
	if l.Len() != len(want) {
		t.Fatalf("%d entries, want %d", l.Len(), len(want))
	}
	for i := range want {
		if got := describe([]Entry{l.Entry(i)}); got != describe(want[i:i+1]) {
			t.Errorf("entry %d:\n%s\nwant\n%s", i, got, describe(want[i:i+1]))
		}
	}
}

func TestFingerprint(t *testing.T) {
	for in, want := range map[string]string{
		"dial tcp 10.0.0.5:5432: connect: connection refused":             "dial tcp <n>.<n>.<n>.<n>:<n>: connect: connection refused",
		`relation "carts" does not exist`:                                 "relation <s> does not exist",
		"order 1042 for user 550e8400-e29b-41d4-a716-446655440000 failed": "order <n> for user <id> failed",
		"pod web-7d9c8b6f5-x2x9z restarted 3 times":                       "pod web-<id>-<id> restarted <n> times",
		"don't retry v1 utf8 at 0xc000a1":                                 "don't retry v1 utf8 at <id>",
		"took   12ms\nsecond line":                                        "took <n>ms",
	} {
		if got := fingerprint(in); got != want {
			t.Errorf("%q:\ngot  %q\nwant %q", in, got, want)
		}
	}
}

func TestGroups(t *testing.T) {
	text := ""
	for i := range 5 {
		text += fmt.Sprintf(`{"level":"error","msg":"order %d failed","error":"timeout after %dms","time":"2026-10-22T10:00:0%dZ"}`+"\n", i, i*10, i)
	}
	text += `{"level":"warn","msg":"slow"}` + "\n" + `{"level":"info","msg":"ok"}` + "\n"
	l, _ := Build([]File{{Name: "x", Data: []byte(text)}}, BuildOptions{Options: testOpts})
	g := l.Groups(nil)
	if len(g) != 2 || g[0].Text != "order <n> failed · timeout after <n>ms" || g[0].Count != 5 || g[0].First != 0 || g[0].Last != 4 ||
		g[0].Level != "error" || g[0].To.Sub(g[0].From) != 4*time.Second || g[1].Text != "slow" {
		t.Errorf("%+v", g)
	}
	m, _ := l.Match("order", time.Time{}, time.Time{})
	if g := l.Groups(m.IDs); len(g) != 1 {
		t.Errorf("groups of a query: %+v", g)
	}
	if m, _ := l.Match(fmt.Sprintf("group:%d", g[0].ID), time.Time{}, time.Time{}); m.Total != 5 {
		t.Errorf("group:%d matched %d", g[0].ID, m.Total)
	}
	if m, _ := l.Match("group:*", time.Time{}, time.Time{}); m.Total != 6 {
		t.Errorf("group:* matched %d", m.Total)
	}
	if l.Summary().Groups != 2 {
		t.Errorf("summary groups %d", l.Summary().Groups)
	}
}

func TestSummary(t *testing.T) {
	l := buildFixtures(t, "argo.log")
	s := l.Summary()
	if s.Entries != 7 || s.Lines != 11 || s.Levels["error"] != 1 || s.Levels["fatal"] != 1 || s.Levels[""] != 2 ||
		s.Formats["ecs"] != 2 || s.FirstError != 4 || s.Stacks != 1 || len(s.Sources) != 2 || s.Sources[0].Count != 4 ||
		s.From.Format(time.RFC3339) != "2026-10-22T10:00:00Z" || s.To.Format(time.RFC3339) != "2026-10-22T10:00:10Z" {
		t.Errorf("%+v", s)
	}
	// One silence of 10 minutes among entries a second apart.
	var b strings.Builder
	for i := range 20 {
		fmt.Fprintf(&b, "2026-10-22T10:00:%02dZ tick\n", i)
	}
	b.WriteString("2026-10-22T10:10:19Z back\n")
	l, _ = Build([]File{{Name: "x", Data: []byte(b.String())}}, BuildOptions{})
	gaps := l.Summary().Gaps
	if len(gaps) != 1 || gaps[0].After != 19 || gaps[0].Seconds != 600 {
		t.Errorf("gaps %+v", gaps)
	}
}

func TestHistogram(t *testing.T) {
	l := buildFixtures(t, "ecs.log")
	h := l.Histogram(nil, time.Time{}, time.Time{}, 60)
	total := 0
	for _, b := range h.Buckets {
		for _, n := range b {
			total += n
		}
	}
	if h.StepMs != 50 || total != 4 || h.Buckets[0][Info] != 1 || h.Buckets[len(h.Buckets)-1][Info] != 1 {
		t.Errorf("%+v", h)
	}
	m, _ := l.Match("level:error", time.Time{}, time.Time{})
	h2 := l.Histogram(m.IDs, time.Time{}, time.Time{}, 60)
	if len(h2.Buckets) != len(h.Buckets) || !h2.From.Equal(h.From) {
		t.Error("a query's histogram should keep the log's axis")
	}
}

func TestFieldStats(t *testing.T) {
	var b strings.Builder
	for i := range 100 {
		fmt.Fprintf(&b, `{"msg":"x","status":%d,"path":"/p%d","method":"GET"}`+"\n", 200+i%2*300, i%3)
	}
	l, _ := Build([]File{{Name: "a", Data: []byte(b.String())}, {Name: "b", Data: []byte(`{"msg":"y","method":"POST"}`)}}, BuildOptions{})
	stats := l.FieldStats(nil, 3)
	by := map[string]FieldStat{}
	for _, s := range stats {
		by[s.Key] = s
	}
	if stats[0].Key != "source" || by["source"].Distinct != 2 {
		t.Errorf("source: %+v", stats[0])
	}
	st := by["status"]
	if st.Count != 100 || st.Distinct != 2 || st.Num == nil || st.Num.Min != 200 || st.Num.Max != 500 || st.Num.Mean != 350 || st.Num.Median != 200 || st.Num.P95 != 500 {
		t.Errorf("status: %+v %+v", st, st.Num)
	}
	if m := by["method"]; m.Count != 101 || m.Top[0] != (Count{"GET", 100}) || m.Top[1] != (Count{"POST", 1}) {
		t.Errorf("method: %+v", m)
	}
	if p := by["path"]; len(p.Top) != 3 || p.Num != nil {
		t.Errorf("path: %+v", p)
	}
}

func TestSize(t *testing.T) {
	data := []byte(benchLog(1000))
	l, _ := Build([]File{{Name: "x", Data: data}}, BuildOptions{})
	if s := l.Size(); s < int64(len(data))*3/2 || s > int64(len(data))*3 {
		t.Errorf("size %d for %d bytes", s, len(data))
	}
}

func TestAround(t *testing.T) {
	l := buildFixtures(t, "zap.log")
	if a, b := l.Around(1, 5, 5); a != 0 || b != 3 {
		t.Errorf("%d %d", a, b)
	}
}

func TestContainsFold(t *testing.T) {
	for _, c := range []struct {
		s, needle string
		want      bool
	}{
		{"Connection RESET by peer", "reset", true},
		{"abc", "abcd", false},
		{"xxAbc", "abc", true},
		{"aaaaab", "ab", true},
		{"Ünïcode", "ünï", true},
		{"", "a", false},
		{"a", "", true},
	} {
		if got := containsFold(c.s, c.needle); got != c.want {
			t.Errorf("%q in %q: %v", c.needle, c.s, got)
		}
	}
}

func FuzzQuery(f *testing.F) {
	l := buildFixtures(f, "ecs.log", "argo.log")
	for _, q := range []string{"level>=warn -source:x (a OR b)", `"x\"y" /a\/b/`, "f:(a OR b) NOT c", "status:>=5", "x:*y*"} {
		f.Add(q)
	}
	f.Fuzz(func(t *testing.T, q string) {
		m, err := l.Match(q, time.Time{}, time.Time{})
		var qe *QueryError
		if err != nil && (!errors.As(err, &qe) || qe.Pos < 0 || qe.Pos > len(q)) {
			t.Fatalf("%q: %v", q, err)
		}
		if err == nil && m.Total > l.Len() {
			t.Fatal(m.Total)
		}
	})
}

func BenchmarkBuild100k(b *testing.B) {
	data := []byte(benchLog(100_000))
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		if _, err := Build([]File{{Name: "x", Data: data}}, BuildOptions{}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMatch100k(b *testing.B) {
	l, _ := Build([]File{{Name: "x", Data: []byte(benchLog(100_000))}}, BuildOptions{})
	for _, q := range []string{"level>=error", "request 4242", "http.response.status_code>=500 -url.path:/api/orders/1*", "/orders/9\\d{3}$/"} {
		b.Run(q, func(b *testing.B) {
			for b.Loop() {
				l.Match(q, time.Time{}, time.Time{})
			}
		})
	}
	b.Run("field stats", func(b *testing.B) {
		for b.Loop() {
			l.FieldStats(nil, 10)
		}
	})
	b.Run("groups", func(b *testing.B) {
		for b.Loop() {
			l.Groups(nil)
		}
	})
}
