package ansiblelog

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
)

// outline describes a segmented log, one block per line, with line
// numbers 1-based as an editor shows them.
func outline(lg *Log) string {
	var out []string
	for _, b := range lg.Blocks {
		where := fmt.Sprintf("%d-%d", b.From+1, b.To)
		if b.Title != "" {
			where += " [" + b.Title + "]"
		}
		if b.Run == nil {
			out = append(out, "other "+where)
			continue
		}
		r := b.Run
		s := "run " + where
		if r.Playbook != "" {
			s += " " + r.Playbook
		}
		if r.JSON {
			s += " json"
		}
		for _, p := range r.Plays {
			s += fmt.Sprintf(" | play %q:", p.Name)
			for _, t := range p.Tasks {
				kind := "task"
				if t.Handler {
					kind = "handler"
				}
				name := t.Name
				if t.Role != "" {
					name = t.Role + "/" + name
				}
				s += fmt.Sprintf(" %s %s %d-%d", kind, name, t.From+1, t.To)
			}
		}
		s += fmt.Sprintf(" | recap %d notes %d", len(r.Recap), len(r.Notes))
		if !r.Complete {
			s += " cut"
		}
		out = append(out, s)
	}
	return strings.Join(out, "\n")
}

func segment(s string) *Log { return Segment(Clean(s)) }

func TestSegmentTwoRuns(t *testing.T) {
	log := strings.Join([]string{
		"section_start:1:prepare",                       // 1
		"Preparing environment",                         // 2
		"section_end:1:prepare",                         // 3
		"section_start:2:deploy",                        // 4
		"$ ansible-playbook site.yml",                   // 5
		"Using /etc/ansible/ansible.cfg as config file", // 6
		"PLAYBOOK: site.yml ****",                       // 7
		"[WARNING]: no inventory",                       // 8
		"",                                              // 9
		"PLAY [web] ****",                               // 10
		"",                                              // 11
		"TASK [Gathering Facts] ****",                   // 12
		"ok: [web-1]",                                   // 13
		"",                                              // 14
		"TASK [nginx : Install nginx] ****",             // 15
		"changed: [web-1]",                              // 16
		"<web-1> SSH: EXEC ssh -C",                      // 17 stray -vvv line
		"",                                              // 18
		"RUNNING HANDLER [nginx : restart nginx] ****", // 19
		"changed: [web-1]",           // 20
		"",                           // 21
		"PLAY [db] ****",             // 22
		"TASK [ping] ****",           // 23
		"[DEPRECATION WARNING]: old", // 24
		"ok: [db-1]",                 // 25
		"",                           // 26
		"PLAY RECAP ****",            // 27
		"db-1 : ok=1 changed=0 unreachable=0 failed=0 skipped=0 rescued=0 ignored=0", // 28
		"web-1  : ok=3 changed=2 unreachable=0 failed=0",                             // 29
		"",                              // 30
		"$ ansible-playbook verify.yml", // 31
		"PLAY [verify] ****",            // 32
		"TASK [check] ****",             // 33
		"ok: [web-1]",                   // 34
		"PLAY RECAP ****",               // 35
		"web-1 : ok=1 changed=0 unreachable=0 failed=0", // 36
		"Job succeeded",        // 37
		"section_end:2:deploy", // 38
	}, "\n")
	want := strings.Join([]string{
		"other 1-3 [prepare]",
		"other 4-5 [deploy]",
		`run 6-30 [deploy] site.yml | play "web": task Gathering Facts 13-14 task nginx/Install nginx 16-18 handler nginx/restart nginx 20-21 | play "db": task ping 24-26 | recap 2 notes 2`,
		"other 31-31 [deploy]",
		`run 32-36 [deploy] | play "verify": task check 34-34 | recap 1 notes 0`,
		"other 37-38 [deploy]",
	}, "\n")
	if got := outline(segment(log)); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestSegmentCutShort(t *testing.T) {
	// No recap: the run ends with its CI step, and in a wrapped run the
	// wrapper tool's own lines after the last task go back to other output.
	log := strings.Join([]string{
		"    azure-arm: PLAY [all] ****",                 // 1
		"    azure-arm: TASK [ping] ****",                // 2
		"    azure-arm: ok: [default]",                   // 3
		"==> azure-arm: interleaved",                     // 4 stray, stays in the task
		"    azure-arm: TASK [build] ****",               // 5
		"    azure-arm: fatal: [default]: FAILED! => {}", // 6
		"==> azure-arm: Provisioning step had errors",    // 7
		"==> azure-arm: Deleting resources",              // 8
		"section_end:9:build",                            // 9
		"ERROR: Job failed",                              // 10
	}, "\n")
	want := strings.Join([]string{
		`run 1-6 | play "all": task ping 3-4 task build 6-6 | recap 0 notes 0 cut`,
		"other 7-9",
		"other 10-10",
	}, "\n")
	if got := outline(segment(log)); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestSegmentEdgeCases(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"error before any run", "ERROR! the playbook: x.yml could not be found\nexit 1",
			"other 1-2"},
		{"tasks without a play header", "TASK [a] ***\nok: [h]\nTASK [b] ***\nok: [h]",
			`run 1-4 | play "": task a 2-2 task b 4-4 | recap 0 notes 0 cut`},
		{"recap at the end of the log", "PLAY [p] ***\nTASK [a] ***\nok: [h]\nPLAY RECAP ***\nh : ok=1 changed=0 unreachable=0 failed=0\n",
			`run 1-5 | play "p": task a 3-3 | recap 1 notes 0`},
		{"names with brackets", "PLAY [p] ***\nTASK [set [x] fact] ***\nok: [h]",
			`run 1-3 | play "p": task set [x] fact 3-3 | recap 0 notes 0 cut`},
		{"json callback", "$ ansible-playbook site.yml\n{\n    \"custom_stats\": {},\n    \"plays\": [{\"play\": {\"name\": \"}\"}}],\n    \"stats\": {}\n}\ndone",
			"other 1-1\nrun 2-6 json | recap 0 notes 0\nother 7-7"},
		{"a lone brace is not a run", "{\n  \"a\": 1\n}", "other 1-3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := outline(segment(tt.in)); got != tt.want {
				t.Errorf("got\n%s\nwant\n%s", got, tt.want)
			}
		})
	}
}

// Blocks cover every line once, in order; task bodies stay inside
// their run and don't overlap.
func TestSegmentCoversEveryLine(t *testing.T) {
	pieces := []string{"PLAY [p] **", "TASK [t] **", "RUNNING HANDLER [h] **", "PLAY RECAP **", "h : ok=1 changed=0 unreachable=0 failed=0",
		"ok: [h]", "", "section_start:1:s", "section_end:1:s", "    azure-arm: TASK [w] **", "==> azure-arm: x", "{", `"plays": [`, "}", "[WARNING]: w", "PLAYBOOK: a.yml **", "text"}
	r := rand.New(rand.NewPCG(3, 4))
	for range 3000 {
		var b strings.Builder
		for range 1 + r.IntN(25) {
			b.WriteString(pieces[r.IntN(len(pieces))] + "\n")
		}
		lg := segment(b.String())
		at := 0
		for _, bl := range lg.Blocks {
			if bl.From != at || bl.To <= bl.From {
				t.Fatalf("gap or empty block at %d in %q:\n%s", at, b.String(), outline(lg))
			}
			at = bl.To
			if bl.Run == nil {
				continue
			}
			prev := bl.From
			for _, p := range bl.Run.Plays {
				for _, tk := range p.Tasks {
					if tk.Line < prev || tk.From != tk.Line+1 || tk.To < tk.From || tk.To > bl.To {
						t.Fatalf("task out of place in %q:\n%s", b.String(), outline(lg))
					}
					prev = tk.To
				}
			}
		}
		if at != len(lg.Lines) {
			t.Fatalf("blocks end at %d of %d in %q:\n%s", at, len(lg.Lines), b.String(), outline(lg))
		}
	}
}
