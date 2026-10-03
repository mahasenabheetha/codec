package ansiblelog

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// results describes every result of every run, one per line.
func results(lg *Log) string {
	var out []string
	for _, b := range lg.Blocks {
		if b.Run == nil {
			continue
		}
		for _, p := range b.Run.Plays {
			for _, t := range p.Tasks {
				for _, r := range t.Results {
					s := fmt.Sprintf("%s: %s %s", t.Name, r.Status, r.Host)
					if r.Delegate != "" {
						s += "->" + r.Delegate
					}
					if r.Item != "" {
						s += " item=" + r.Item
					}
					if r.File != "" {
						s += " file=" + r.File
					}
					if r.Retries > 0 {
						s += fmt.Sprintf(" retries=%d", r.Retries)
					}
					if r.Ignored {
						s += " ignored"
					}
					if r.Censored {
						s += " censored"
					}
					if r.Format != "" {
						s += " " + r.Format
						if msg, ok := r.Payload["msg"]; ok {
							s += fmt.Sprintf(" msg=%v", msg)
						}
						if r.End > r.Line {
							s += fmt.Sprintf(" lines=%d", r.End-r.Line+1)
						}
					}
					out = append(out, s)
				}
				if len(t.Other) > 0 {
					out = append(out, fmt.Sprintf("%s: %d other", t.Name, len(t.Other)))
				}
			}
		}
		for _, h := range b.Run.Hosts {
			out = append(out, fmt.Sprintf("recap %s ok=%d changed=%d failed=%d skipped=%d rescued=%d ignored=%d unreachable=%d",
				h.Host, h.OK, h.Changed, h.Failed, h.Skipped, h.Rescued, h.Ignored, h.Unreachable))
		}
	}
	return strings.Join(out, "\n")
}

func read(s string) *Log {
	lg := Segment(Clean(s))
	Read(lg)
	return lg
}

func TestReadDefaultCallback(t *testing.T) {
	log := `PLAY [all] ****
TASK [ping] ****
ok: [web-1]
changed: [web-2 -> db-1]
TASK [debug] ****
ok: [web-1] => {
    "msg": "a } inside a string, {\"nested\": true}"
}
TASK [loop] ****
ok: [web-1] => (item=a)
changed: [web-1] => (item={'name': 'x (y)', 'v': 1}) => {"changed": true, "item": {"name": "x (y)"}}
skipping: [web-2] => (item=b)
failed: [web-2] (item=c) => {"changed": false, "msg": "bad item"}
TASK [wait] ****
FAILED - RETRYING: [web-1]: wait (3 retries left).
FAILED - RETRYING: [web-1]: wait (2 retries left).
<web-1> SSH: EXEC ssh -C web-1
ok: [web-1]
TASK [might fail] ****
fatal: [web-1]: FAILED! => {"changed": false, "msg": "nope"}
...ignoring
fatal: [web-2]: UNREACHABLE! => {"changed": false, "msg": "Failed to connect", "unreachable": true}
TASK [secret] ****
ok: [web-1] => {"censored": "the output has been hidden due to the fact that 'no_log: true' was specified for this result", "changed": false}
TASK [include] ****
included: /src/roles/app/tasks/setup.yml for web-1, web-2
TASK [broken json] ****
ok: [web-1] => {"msg": "never closed
TASK [after] ****
ok: [web-1] => some plain words
PLAY RECAP ****
web-1 : ok=6 changed=1 unreachable=0 failed=0 skipped=0 rescued=0 ignored=1
web-2 : ok=0 changed=1 unreachable=1 failed=1 skipped=1 rescued=0 ignored=0
`
	want := strings.Join([]string{
		"ping: ok web-1",
		"ping: changed web-2->db-1",
		`debug: ok web-1 json msg=a } inside a string, {"nested": true} lines=3`,
		"loop: ok web-1 item=a",
		"loop: changed web-1 item={'name': 'x (y)', 'v': 1} json",
		"loop: skipping web-2 item=b",
		"loop: failed web-2 item=c json msg=bad item",
		"wait: ok web-1 retries=2",
		"wait: 1 other",
		"might fail: failed web-1 ignored json msg=nope",
		"might fail: unreachable web-2 json msg=Failed to connect",
		"secret: ok web-1 censored json",
		"include: included web-1 file=/src/roles/app/tasks/setup.yml",
		"include: included web-2 file=/src/roles/app/tasks/setup.yml",
		`broken json: ok web-1 raw`,
		"after: ok web-1 raw",
		"recap web-1 ok=6 changed=1 failed=0 skipped=0 rescued=0 ignored=1 unreachable=0",
		"recap web-2 ok=0 changed=1 failed=1 skipped=1 rescued=0 ignored=0 unreachable=1",
	}, "\n")
	if got := results(read(log)); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestReadYAMLCallback(t *testing.T) {
	log := `PLAY [all] ****
TASK [command] ****
fatal: [web-1]: FAILED! => changed=false
  cmd:
  - /bin/false
  msg: non-zero return code
  rc: 1
  stderr: ''
TASK [debug] ****
ok: [web-1] =>
  msg: |-
    line one
    line two
`
	want := strings.Join([]string{
		"command: failed web-1 yaml msg=non-zero return code lines=6",
		"debug: ok web-1 yaml msg=line one\nline two lines=4",
	}, "\n")
	if got := results(read(log)); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestReadJSONCallback(t *testing.T) {
	log := `{
    "custom_stats": {},
    "plays": [
        {
            "play": {"id": "1", "name": "web"},
            "tasks": [
                {"task": {"name": "nginx : install"}, "hosts": {
                    "web-2": {"changed": true, "action": "apt"},
                    "web-1": {"failed": true, "msg": "no {apt}"}
                }},
                {"task": {"name": "skip me"}, "hosts": {"web-1": {"skipped": true, "changed": false}}}
            ]
        }
    ],
    "stats": {"web-1": {"ok": 1, "changed": 0, "failures": 1, "skipped": 1, "unreachable": 0, "rescued": 0, "ignored": 0}}
}`
	want := strings.Join([]string{
		"install: failed web-1 json msg=no {apt}",
		"install: changed web-2 json",
		"skip me: skipping web-1 json",
		"recap web-1 ok=1 changed=0 failed=1 skipped=1 rescued=0 ignored=0 unreachable=0",
	}, "\n")
	if got := results(read(log)); got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestReadPathsAndDurations(t *testing.T) {
	log := `PLAY [all] ****
TASK [first] ****
task path: /src/site.yml:4
Thursday 01 October 2026  20:51:22 +0000 (0:00:00.016)       0:00:00.016 ******
ok: [web-1]

TASK [second] ****
task path: /src/roles/app/tasks/main.yml:12
Thursday 01 October 2026  20:51:24 +0000 (0:00:01.631)       0:00:01.647 ******
META: role_complete for web-1
ok: [web-1]
TASK [third] ****
Thursday 01 October 2026  20:53:24 +0000 (0:02:00.500)       0:02:02.147 ******
ok: [web-1]
`
	tasks := read(log).Blocks[0].Run.Plays[0].Tasks
	got := fmt.Sprintf("%s:%d %v %d | %s:%d %v %d | %v", tasks[0].Path, tasks[0].PathLine, tasks[0].Duration, len(tasks[0].Other),
		tasks[1].Path, tasks[1].PathLine, tasks[1].Duration, len(tasks[1].Other), tasks[2].Duration)
	if want := "/src/site.yml:4 1.631s 0 | /src/roles/app/tasks/main.yml:12 2m0.5s 1 | 0s"; got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// With skipped tasks hidden, the timer still prints a line as each of
// them starts: those lines time the visible task before them.
func TestReadHiddenTaskTimers(t *testing.T) {
	log := `PLAY [all] ****
TASK [a] ****
Thursday 01 October 2026  20:51:22 +0000 (0:00:00.010)       0:00:00.010 ******
ok: [web-1]
Thursday 01 October 2026  20:51:25 +0000 (0:00:03.000)       0:00:03.010 ******
Thursday 01 October 2026  20:51:25 +0000 (0:00:00.020)       0:00:03.030 ******
TASK [b] ****
Thursday 01 October 2026  20:51:25 +0000 (0:00:00.030)       0:00:03.060 ******
ok: [web-1]
`
	tasks := read(log).Blocks[0].Run.Plays[0].Tasks
	if tasks[0].Duration != 3*time.Second || len(tasks[0].Other) != 0 {
		t.Errorf("a: %v, %d other", tasks[0].Duration, len(tasks[0].Other))
	}
}

// One task copied from a wrapped log, its header without the prefix:
// the wrapper is learned from the task path and result lines.
func TestReadWrappedFragment(t *testing.T) {
	log := "TASK [db : Deploy code] ****\n" +
		"    azure-arm: task path: /src/roles/db/tasks/main.yml:135\n" +
		"    azure-arm: ok: [default] => {\"changed\": false, \"rc\": 0, \"msg\": \"\"}\n"
	lg := read(log)
	if got := results(lg); got != "Deploy code: ok default json msg=" {
		t.Errorf("got %q", got)
	}
	if tk := lg.Blocks[0].Run.Plays[0].Tasks[0]; tk.Path != "/src/roles/db/tasks/main.yml" || tk.PathLine != 135 {
		t.Errorf("path %s:%d", tk.Path, tk.PathLine)
	}
}
