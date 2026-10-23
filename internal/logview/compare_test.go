package logview

import (
	"strings"
	"sync"
	"testing"
)

// Two runs of one made-up build: the one that passed yesterday, and
// today's, whose tests retried the database and failed.
const (
	passedRun = `build-7f2kd-checkout-1111111111: {"@timestamp":"2026-10-21T09:00:00Z","log.level":"info","message":"cloning repo at commit 3fa9c1e"}
build-7f2kd-checkout-1111111111: {"@timestamp":"2026-10-21T09:00:04Z","log.level":"info","message":"checkout done in 4s"}
build-7f2kd-test-2222222222: {"@timestamp":"2026-10-21T09:00:05Z","log.level":"info","message":"running 42 tests"}
build-7f2kd-test-2222222222: {"@timestamp":"2026-10-21T09:00:06Z","log.level":"warn","message":"retrying db connection"}
build-7f2kd-test-2222222222: {"@timestamp":"2026-10-21T09:00:07Z","log.level":"info","message":"connected to db at 10.0.0.5:5432"}
build-7f2kd-test-2222222222: {"@timestamp":"2026-10-21T09:00:20Z","log.level":"info","message":"all tests passed"}
build-7f2kd-publish-3333333333: {"@timestamp":"2026-10-21T09:00:21Z","log.level":"info","message":"published image app:41"}
`
	failedRun = `build-xkqpz-checkout-4444444444: {"@timestamp":"2026-10-22T09:00:00Z","log.level":"info","message":"cloning repo at commit 9bc2d07"}
build-xkqpz-checkout-4444444444: {"@timestamp":"2026-10-22T09:00:03Z","log.level":"info","message":"checkout done in 3s"}
build-xkqpz-test-5555555555: {"@timestamp":"2026-10-22T09:00:04Z","log.level":"info","message":"running 43 tests"}
build-xkqpz-test-5555555555: {"@timestamp":"2026-10-22T09:00:05Z","log.level":"warn","message":"retrying db connection"}
build-xkqpz-test-5555555555: {"@timestamp":"2026-10-22T09:00:15Z","log.level":"warn","message":"retrying db connection"}
build-xkqpz-test-5555555555: {"@timestamp":"2026-10-22T09:00:25Z","log.level":"warn","message":"retrying db connection"}
build-xkqpz-test-5555555555: {"@timestamp":"2026-10-22T09:00:35Z","log.level":"warn","message":"retrying db connection"}
build-xkqpz-test-5555555555: {"@timestamp":"2026-10-22T09:01:05Z","log.level":"error","message":"test failed","error.message":"timeout after 30000ms"}
`
)

func texts(xs []DiffItem) string {
	var out []string
	for _, x := range xs {
		out = append(out, x.Text)
	}
	return strings.Join(out, " | ")
}

func TestCompare(t *testing.T) {
	failed, passed := buildText(t, failedRun), buildText(t, passedRun)
	d := Compare(failed, passed)

	if d.First != [2]int{7, 4} {
		t.Errorf("first differences %v", d.First)
	}
	if got := texts(d.New); got != "test failed · timeout after <n>ms" {
		t.Errorf("new: %s", got)
	}
	if x := d.New[0]; x.Level != "error" || x.Count != [2]int{1, 0} || x.First != [2]int{7, -1} || x.Sample != "test failed" {
		t.Errorf("new item %+v", x)
	}
	if got := texts(d.Missing); got != "connected to db at <n>.<n>.<n>.<n>:<n> | all tests passed | published image app:<n>" {
		t.Errorf("missing: %s", got)
	}
	if x := d.Missing[0]; x.Count != [2]int{0, 1} || x.First != [2]int{-1, 4} || x.Sample != "connected to db at 10.0.0.5:5432" {
		t.Errorf("missing item %+v", x)
	}
	if got := texts(d.Changed); got != "retrying db connection" || d.Changed[0].Count != [2]int{4, 1} || d.Changed[0].First != [2]int{3, 3} {
		t.Errorf("changed: %s %+v", got, d.Changed)
	}
	if r := d.Runs; r[0].Entries != 8 || r[0].Errors != 1 || r[0].Warnings != 4 || r[0].Seconds != 65 || r[1].Seconds != 21 || r[0].Messages != 5 || r[1].Messages != 7 {
		t.Errorf("runs %+v", r)
	}

	// Sources match whatever their generated part, in this run's order,
	// then the step only the other run had.
	if len(d.Sources) != 3 {
		t.Fatalf("sources %+v", d.Sources)
	}
	test := d.Sources[1]
	if test.Name != [2]string{"build-xkqpz-test-5555555555", "build-7f2kd-test-2222222222"} ||
		test.Entries != [2]int{6, 4} || test.Errors != [2]int{1, 0} || test.Seconds != [2]float64{61, 15} || test.First != [2]int{2, 2} {
		t.Errorf("test step %+v", test)
	}
	if p := d.Sources[2]; p.Name != [2]string{"", "build-7f2kd-publish-3333333333"} || p.First != [2]int{-1, 6} {
		t.Errorf("publish step %+v", p)
	}

	// The same run against itself differs in nothing.
	same := Compare(passed, passed)
	if same.First != [2]int{-1, -1} || len(same.New)+len(same.Missing)+len(same.Changed) != 0 {
		t.Errorf("a run against itself: %+v", same)
	}
}

func TestCompareMasked(t *testing.T) {
	// A hex ID that happens to be all digits is still the same message.
	d := Compare(buildText(t, "fetched object 3fa9c1e2b0\n"), buildText(t, "fetched object 9879061596\n"))
	if len(d.New)+len(d.Missing) != 0 || d.First != [2]int{-1, -1} {
		t.Errorf("%+v", d)
	}
}

func TestCompareCaps(t *testing.T) {
	var a, b strings.Builder
	for i := range maxDiffItems + 20 {
		a.WriteString("step " + strings.Repeat("a", i%26+1) + strings.Repeat("b", i/26+1) + " done\n")
		b.WriteString("other\n")
	}
	a.WriteString(`{"level":"error","msg":"boom"}` + "\n")
	d := Compare(buildText(t, a.String()), buildText(t, b.String()))
	if len(d.New) != maxDiffItems || d.More[0] != 21 || d.New[0].Text != "boom" {
		t.Errorf("%d new, %v more, first %q", len(d.New), d.More, d.New[0].Text)
	}
	if d.Sources != nil {
		t.Errorf("sources of logs without any: %+v", d.Sources)
	}
}

func TestChanged(t *testing.T) {
	for _, c := range []struct {
		a, b int
		want bool
	}{{1, 1, false}, {1, 2, false}, {1, 3, true}, {3, 1, true}, {100, 120, false}, {100, 150, true}, {150, 100, true}, {2, 4, true}} {
		if got := changed(c.a, c.b); got != c.want {
			t.Errorf("changed(%d, %d) = %v", c.a, c.b, got)
		}
	}
}

func BenchmarkCompare100k(b *testing.B) {
	x := buildText(b, benchLog(100_000))
	y := buildText(b, benchLog(90_000))
	b.Run("fingerprints", func(b *testing.B) {
		for b.Loop() {
			x.msgOnce, x.msg = sync.Once{}, nil
			x.messages()
		}
	})
	b.Run("compare", func(b *testing.B) {
		y.messages()
		for b.Loop() {
			Compare(x, y)
		}
	})
}
