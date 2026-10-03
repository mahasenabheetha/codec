package ansiblelog

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the .golden files from the fixtures")

// fixture reads a synthetic log; "␛" and "␍" stand for ESC and CR so
// the files stay readable.
func fixture(t testing.TB, name string) string {
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return strings.NewReplacer("␛", "\x1b", "␍", "\r").Replace(string(b))
}

// describe renders an Analysis compactly, for golden files.
func describe(a *Analysis) string {
	var b strings.Builder
	s := a.Summary
	fmt.Fprintf(&b, "lines=%d runs=%d hosts=%v tasks=%d ok=%d changed=%d failed=%d unreachable=%d skipped=%d rescued=%d ignored=%d durationMs=%d other=%d otherErrors=%d\n",
		a.Lines, s.Runs, s.Hosts, s.Tasks, s.OK, s.Changed, s.Failed, s.Unreachable, s.Skipped, s.Rescued, s.Ignored, s.DurationMs, s.Other, s.OtherErrors)
	if s.FirstFailure != nil {
		f := s.FirstFailure
		fmt.Fprintf(&b, "first failure: block %d play %d task %d result %d\n", f.Block, f.Play, f.Task, f.Result)
	}
	if s.Verdict != "" {
		fmt.Fprintf(&b, "verdict: %s\n", s.Verdict)
	}
	for _, bl := range a.Blocks {
		fmt.Fprintf(&b, "\n%s %d-%d", bl.Kind, bl.From, bl.To)
		if bl.Title != "" {
			fmt.Fprintf(&b, " [%s]", bl.Title)
		}
		if bl.Run == nil {
			fmt.Fprintf(&b, " errors=%d warnings=%d\n", bl.Errors, bl.Warnings)
			for _, l := range bl.Text {
				fmt.Fprintf(&b, "  | %s\n", l)
			}
			continue
		}
		r := bl.Run
		fmt.Fprintf(&b, " playbook=%q json=%v complete=%v status=%s durationMs=%d hosts=%v\n", r.Playbook, r.JSON, r.Complete, r.Status, r.DurationMs, r.Hosts)
		for _, n := range r.Notes {
			fmt.Fprintf(&b, "  note %d %s: %s\n", n.Line, n.Level, n.Text)
		}
		for _, p := range r.Plays {
			fmt.Fprintf(&b, "  play %q line=%d\n", p.Name, p.Line)
			for _, t := range p.Tasks {
				kind := "task"
				if t.Handler {
					kind = "handler"
				}
				fmt.Fprintf(&b, "    %s %q role=%q line=%d path=%s:%d durationMs=%d status=%s counts=%+v\n", kind, t.Name, t.Role, t.Line, t.Path, t.PathLine, t.DurationMs, t.Status, t.Counts)
				for _, res := range t.Results {
					fmt.Fprintf(&b, "      %s %s lines=%d-%d", res.Status, res.Host, res.Line, res.EndLine)
					for _, kv := range [][2]string{{"delegate", res.Delegate}, {"item", res.Item}, {"file", res.File}, {"format", res.Format}, {"msg", res.Msg}} {
						if kv[1] != "" {
							fmt.Fprintf(&b, " %s=%q", kv[0], kv[1])
						}
					}
					if res.Retries > 0 {
						fmt.Fprintf(&b, " retries=%d", res.Retries)
					}
					if res.Ignored {
						b.WriteString(" ignored")
					}
					if res.Rescued {
						b.WriteString(" rescued")
					}
					if res.Censored {
						b.WriteString(" censored")
					}
					b.WriteByte('\n')
				}
				for _, o := range t.Other {
					fmt.Fprintf(&b, "      other: %s\n", o)
				}
			}
		}
		for _, h := range r.Recap {
			fmt.Fprintf(&b, "  recap %+v\n", h)
		}
	}
	return b.String()
}

// The fixture set covers the formats the analyzer promises to read;
// each has a golden description checked by eye when it changes.
func TestFixtures(t *testing.T) {
	logs, _ := filepath.Glob("testdata/*.log")
	if len(logs) < 7 {
		t.Fatalf("only %d fixtures", len(logs))
	}
	for _, path := range logs {
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			got := describe(Analyze(fixture(t, name)))
			golden := strings.TrimSuffix(path, ".log") + ".golden"
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

// FuzzAnalyze: any input is read without a panic, and the blocks cover
// every input line once, in order.
func FuzzAnalyze(f *testing.F) {
	logs, _ := filepath.Glob("testdata/*.log")
	for _, path := range logs {
		f.Add(fixture(f, filepath.Base(path)))
	}
	f.Add("PLAY [x] **\nTASK [y] **\nok: [h] => {\"a\": [\n")
	f.Add("fatal: [h]: FAILED! => changed=false \n  msg: |\n")
	f.Fuzz(func(t *testing.T, in string) {
		a := Analyze(in)
		at := 1
		for _, b := range a.Blocks {
			if b.From < at || b.To < b.From {
				t.Fatalf("block %d-%d after line %d", b.From, b.To, at)
			}
			at = b.To + 1
		}
		if len(a.Blocks) > 0 && a.Blocks[len(a.Blocks)-1].To != a.Lines {
			t.Fatalf("blocks end at %d of %d lines", a.Blocks[len(a.Blocks)-1].To, a.Lines)
		}
	})
}

// A 50 MB log, mostly results with payloads, as a pipeline prints it.
func BenchmarkAnalyze50MB(b *testing.B) {
	var sb strings.Builder
	for i := 0; sb.Len() < 50<<20; i++ {
		fmt.Fprintf(&sb, "2026-01-05T09:00:00.%06dZ 01O \x1b[0;32m    azure-arm: TASK [role : step %d] ****\x1b[0m\n", i%1000000, i)
		fmt.Fprintf(&sb, "2026-01-05T09:00:00.%06dZ 01O \x1b[0;32m    azure-arm: task path: /build/roles/role/tasks/main.yml:%d\x1b[0m\n", i%1000000, i)
		fmt.Fprintf(&sb, "2026-01-05T09:00:00.%06dZ 01O \x1b[0;32m    azure-arm: changed: [default] => {\"changed\": true, \"cmd\": \"echo %d\", \"rc\": 0, \"stdout\": \"%s\"}\x1b[0m\n", i%1000000, i, strings.Repeat("x", 300))
	}
	in := sb.String()
	b.SetBytes(int64(len(in)))
	b.ResetTimer()
	for b.Loop() {
		Analyze(in)
	}
}

// Findings from the 2.2.0 review, one log each.
func TestReviewCases(t *testing.T) {
	first := func(a *Analysis) string {
		if f := a.Summary.FirstFailure; f != nil {
			return a.Blocks[f.Block].Run.Plays[f.Play].Tasks[f.Task].Name
		}
		return ""
	}
	// An always: task runs after a failure; the recap says nothing was
	// rescued, so the failure stands.
	a := Analyze("PLAY [p] ***\nTASK [deploy] ***\nfatal: [h]: FAILED! => {\"msg\": \"x\"}\nTASK [cleanup] ***\nok: [h]\nPLAY RECAP ***\nh : ok=1 changed=0 unreachable=0 failed=1 skipped=0 rescued=0 ignored=0\n")
	if first(a) != "deploy" || a.Blocks[0].Run.Status != Failed {
		t.Errorf("always: first failure %q, status %s", first(a), a.Blocks[0].Run.Status)
	}
	// A loop with ignore_errors prints "...ignoring" once for all items.
	a = Analyze("PLAY [p] ***\nTASK [try] ***\nfailed: [h] (item=1) => {\"msg\": \"a\"}\nfailed: [h] (item=2) => {\"msg\": \"b\"}\n...ignoring\nPLAY RECAP ***\nh : ok=0 changed=0 unreachable=0 failed=0 skipped=0 rescued=0 ignored=1\n")
	if first(a) != "" || a.Blocks[0].Run.Plays[0].Tasks[0].Counts.Ignored != 2 {
		t.Errorf("ignored loop: first failure %q, counts %+v", first(a), a.Blocks[0].Run.Plays[0].Tasks[0].Counts)
	}
	// An apostrophe in a loop item isn't a quote.
	a = Analyze("PLAY [p] ***\nTASK [t] ***\nfailed: [h] (item=don't) => {\"msg\": \"boom\"}\n")
	if r := a.Blocks[0].Run.Plays[0].Tasks[0].Results[0]; r.Item != "don't" || r.Msg != "boom" {
		t.Errorf("apostrophe: item %q msg %q", r.Item, r.Msg)
	}
	// YAML with numeric keys and .nan still encodes as JSON.
	a = Analyze("PLAY [p] ***\nTASK [t] ***\nok: [h] => \n  ports:\n    80: http\n  ratio: .nan\n")
	if _, err := json.Marshal(a); err != nil {
		t.Errorf("yaml payload: %v", err)
	}
	// A delegated retry counts for its host, and only once.
	a = Analyze("PLAY [p] ***\nTASK [wait] ***\nFAILED - RETRYING: [h -> db]: wait (2 retries left).\nok: [h -> db]\nok: [g]\n")
	if rs := a.Blocks[0].Run.Plays[0].Tasks[0].Results; rs[0].Retries != 1 || rs[1].Retries != 0 {
		t.Errorf("retries %d %d", rs[0].Retries, rs[1].Retries)
	}
}
