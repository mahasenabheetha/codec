package logview

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// viewLog is a small run: structured entries, plain output and retries.
const viewLog = `{"level":"info","msg":"start"}
plain a
plain b
plain c
{"level":"error","msg":"boom"}
{"level":"warn","msg":"retry in 5ms"}
{"level":"warn","msg":"retry in 7ms"}
{"level":"warn","msg":"retry in 7ms"}
{"level":"info","msg":"done"}
`

func buildText(t testing.TB, text string) *Log {
	t.Helper()
	l, err := Build([]File{{Name: "x", Data: []byte(text)}}, BuildOptions{Options: testOpts})
	if err != nil {
		t.Fatal(err)
	}
	return l
}

// rowsOf writes a view compactly: "4" for an entry, "~3" for context,
// "gap0-2", "out1-3×3", "rep6-7×2".
func rowsOf(rows []ViewRow) string {
	var out []string
	for _, r := range rows {
		switch r.Kind {
		case RowMatch:
			out = append(out, fmt.Sprint(r.I))
		case RowContext:
			out = append(out, fmt.Sprintf("~%d", r.I))
		default:
			out = append(out, fmt.Sprintf("%s%d-%d×%d", r.Kind.String()[:3], r.I, r.Last, r.N))
		}
	}
	return strings.Join(out, " ")
}

func TestView(t *testing.T) {
	l := buildText(t, viewLog)
	all, _ := l.Match("", time.Time{}, time.Time{})
	boom, _ := l.Match("boom", time.Time{}, time.Time{})
	for _, c := range []struct {
		name string
		m    *Matches
		o    ViewOptions
		want string
	}{
		{"plain", all, ViewOptions{}, "0 1 2 3 4 5 6 7 8"},
		{"context", boom, ViewOptions{Context: 1}, "gap0-2×3 ~3 4 ~5 gap6-8×3"},
		{"context at the ends", all, ViewOptions{Context: 3}, "0 1 2 3 4 5 6 7 8"},
		{"output", all, ViewOptions{Output: true}, "0 out1-3×3 4 5 6 7 8"},
		{"output opened", all, ViewOptions{Output: true, Expanded: map[uint32]bool{1: true}}, "0 1 2 3 4 5 6 7 8"},
		{"output in context", boom, ViewOptions{Context: 4, Output: true}, "~0 out1-3×3 4 ~5 ~6 ~7 ~8"},
		{"repeats", all, ViewOptions{Repeats: RepeatsExact}, "0 1 2 3 4 5 rep6-7×2 8"},
		{"similar repeats", all, ViewOptions{Repeats: RepeatsSimilar}, "0 1 2 3 4 rep5-7×3 8"}, // "plain a/b/c" differ by a letter, not a number,
		{"both", all, ViewOptions{Output: true, Repeats: RepeatsSimilar}, "0 out1-3×3 4 rep5-7×3 8"},
		{"repeat opened", all, ViewOptions{Repeats: RepeatsSimilar, Expanded: map[uint32]bool{5: true}}, "0 1 2 3 4 5 6 7 8"},
	} {
		if got := rowsOf(l.View(c.m, c.o)); got != c.want {
			t.Errorf("%s:\ngot  %s\nwant %s", c.name, got, c.want)
		}
	}
}

func TestViewOutputBySource(t *testing.T) {
	// Two pods' plain output interleaved folds apart; a structured entry
	// from either ends both runs; a run shorter than three stays.
	l := buildText(t, `pod-a-1111111111: {"level":"info","msg":"start"}
pod-b-2222222222: b one
pod-c-3333333333: c one
pod-b-2222222222: b two
pod-c-3333333333: c two
pod-b-2222222222: b three
pod-a-1111111111: {"level":"info","msg":"middle"}
pod-b-2222222222: b four
pod-b-2222222222: b five
`)
	all, _ := l.Match("", time.Time{}, time.Time{})
	rows := l.View(all, ViewOptions{Output: true})
	if got := rowsOf(rows); got != "0 out1-5×3 2 4 6 7 8" {
		t.Errorf("got %s", got)
	}
	// Entry 3 (b two) is in pod b's fold; entry 4 (c two) has its own row.
	if pos := l.RowAt(rows, 3); pos != 1 {
		t.Errorf("RowAt(3) = %d", pos)
	}
	if pos := l.RowAt(rows, 4); pos != 3 {
		t.Errorf("RowAt(4) = %d", pos)
	}
}

func TestRowAt(t *testing.T) {
	l := buildText(t, viewLog)
	boom, _ := l.Match("boom", time.Time{}, time.Time{})
	rows := l.View(boom, ViewOptions{Context: 1})
	for i, want := range map[uint32]int{0: 0, 2: 0, 3: 1, 4: 2, 7: 4} {
		if got := l.RowAt(rows, i); got != want {
			t.Errorf("RowAt(%d) = %d, want %d", i, got, want)
		}
	}
	if got := l.RowAt(nil, 3); got != -1 {
		t.Errorf("empty view: %d", got)
	}
}

func TestMarkers(t *testing.T) {
	l := buildText(t, viewLog)
	all, _ := l.Match("", time.Time{}, time.Time{})
	rows := l.View(all, ViewOptions{})
	rule, err := l.Filter("retry")
	if err != nil {
		t.Fatal(err)
	}
	m := l.Markers(rows, 9, []*Filter{nil, rule}, []uint32{8})
	want := []uint8{0, 0, 0, 0, MarkError, MarkWarn | MarkRule<<1, MarkWarn | MarkRule<<1, MarkWarn | MarkRule<<1, MarkPin}
	if fmt.Sprint(m) != fmt.Sprint(want) {
		t.Errorf("got %v\nwant %v", m, want)
	}
	// Fewer buckets than rows: each holds what its rows hold.
	if m := l.Markers(rows, 3, nil, nil); fmt.Sprint(m) != fmt.Sprint([]uint8{0, MarkError | MarkWarn, MarkWarn}) {
		t.Errorf("3 buckets: %v", m)
	}
	if _, err := l.Filter(`"open`); err == nil {
		t.Error("a bad rule compiled")
	}
}

// FuzzView: whatever the log and the options, rows come in entry order,
// every entry the view shows is found by RowAt, and the rows account for
// the matches.
func FuzzView(f *testing.F) {
	f.Add(viewLog, uint8(1), true, uint8(2))
	f.Add(fixture(f, "argo-mixed.log"), uint8(3), true, uint8(1))
	f.Fuzz(func(t *testing.T, text string, ctx uint8, output bool, rep uint8) {
		l, err := Build([]File{{Name: "x", Data: []byte(text)}}, BuildOptions{Options: testOpts})
		if err != nil {
			return
		}
		m, _ := l.Match("level>=warn OR plain", time.Time{}, time.Time{})
		rows := l.View(m, ViewOptions{Context: int(ctx % 8), Output: output, Repeats: Repeats(rep % 3)})
		matched := 0
		for k, r := range rows {
			if k > 0 && r.I < rows[k-1].I {
				t.Fatalf("row %d starts at %d, before %d", k, r.I, rows[k-1].I)
			}
			if r.Last < r.I || int(r.Last) >= l.Len() || r.N == 0 {
				t.Fatalf("row %+v", r)
			}
			if r.Kind != RowGap && l.RowAt(rows, r.I) != k {
				t.Fatalf("RowAt(%d) = %d, want %d", r.I, l.RowAt(rows, r.I), k)
			}
			matched += int(r.Matches)
		}
		if matched != m.Total {
			t.Fatalf("rows hold %d matches of %d", matched, m.Total)
		}
		l.Markers(rows, 7, nil, []uint32{0})
	})
}

func BenchmarkView100k(b *testing.B) {
	l, _ := Build([]File{{Name: "x", Data: []byte(benchLog(100_000))}}, BuildOptions{})
	errs, _ := l.Match("level>=error", time.Time{}, time.Time{})
	all, _ := l.Match("", time.Time{}, time.Time{})
	b.Run("context 10", func(b *testing.B) {
		for b.Loop() {
			l.View(errs, ViewOptions{Context: 10, Output: true})
		}
	})
	b.Run("all, folded", func(b *testing.B) {
		for b.Loop() {
			l.View(all, ViewOptions{Output: true, Repeats: RepeatsSimilar})
		}
	})
	b.Run("markers", func(b *testing.B) {
		rows := l.View(all, ViewOptions{})
		f, _ := l.Filter("request 42")
		for b.Loop() {
			l.Markers(rows, 300, []*Filter{f}, nil)
		}
	})
}
