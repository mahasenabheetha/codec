package ansiblelog

import (
	"encoding/json"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	yaml "go.yaml.in/yaml/v3"
)

// Status is a host's outcome for a task, as Ansible prints it.
type Status string

const (
	OK          Status = "ok"
	Changed     Status = "changed"
	Skipped     Status = "skipping"
	Failed      Status = "failed"
	Unreachable Status = "unreachable"
	Included    Status = "included"
	// Rescued is a task's status when its failures were caught by a
	// rescue block; results keep Failed with Rescued set.
	Rescued Status = "rescued"
	// Unfinished is the status of the task a cut-short run stopped in.
	Unfinished Status = "unfinished"
)

// Result is one host's result line in a task, with its payload.
type Result struct {
	Line     int // result line index; -1 in a json callback run
	End      int // last line of the payload (Line when it has none)
	Status   Status
	Host     string
	Delegate string // "[web-1 -> db-1]"
	Item     string // loops: the "(item=…)" label
	File     string // included: the task file
	Retries  int    // FAILED - RETRYING lines before it for this host
	Ignored  bool   // ignore_errors: "...ignoring" followed
	Censored bool   // no_log hid the result
	// Payload is the result as data when it parsed (JSON, or YAML from
	// the yaml callback); Raw is its text either way.
	Payload map[string]any
	Raw     string
	Format  string // "json", "yaml", "raw" or "" (no payload)
}

// HostRecap is one PLAY RECAP row.
type HostRecap struct {
	Host        string `json:"host"`
	OK          int    `json:"ok"`
	Changed     int    `json:"changed"`
	Unreachable int    `json:"unreachable"`
	Failed      int    `json:"failed"`
	Skipped     int    `json:"skipped"`
	Rescued     int    `json:"rescued"`
	Ignored     int    `json:"ignored"`
}

var (
	resultLine = regexp.MustCompile(`^(ok|changed|skipping|failed|fatal): \[([^\]]*)\](?::\s*(FAILED|UNREACHABLE)!)?`)
	included   = regexp.MustCompile(`^included: (.+?) for (.+?)(?:\s+=>\s+\(item=(.*)\))?\s*$`)
	retrying   = regexp.MustCompile(`^FAILED - RETRYING: (?:\[([^\]]*)\]: )?.*\(\d+ retries left\)`)
	ignoring   = regexp.MustCompile(`^\.\.\.ignoring\s*$`)
	recapCount = regexp.MustCompile(`(\w+)=(\d+)`)
	taskPath   = regexp.MustCompile(`^task path: (.+):(\d+)\s*$`)
	// profile_tasks and timer: "Thursday 01 October 2026  20:51:23 +0000
	// (0:00:01.631)       0:00:01.647 ****"; the bracketed time is how
	// long the previous task took.
	timerLine = regexp.MustCompile(`^\w+ \d{1,2} \w+ \d{4}\s+\d\d:\d\d:\d\d(?: [+-]\d{4})?\s+\((\d+):(\d\d):(\d\d(?:\.\d+)?)\)\s+[\d:.]+\s*\**\s*$`)
	// The yaml callback puts the first key on the result line as
	// "changed=false"; the rest follows indented.
	inlineKV = regexp.MustCompile(`^(\w+)=(\S*)\s*$`)
)

// Read fills in each task's results and stray lines, and each run's
// recap. Payloads that don't parse are kept as raw text.
func Read(lg *Log) {
	for i := range lg.Blocks {
		r := lg.Blocks[i].Run
		if r == nil {
			continue
		}
		if r.JSON {
			readJSONRun(lg, lg.Blocks[i], r)
			continue
		}
		var prev *Task
		for p := range r.Plays {
			for t := range r.Plays[p].Tasks {
				task := &r.Plays[p].Tasks[t]
				// A task already timed by a hidden task's line keeps it.
				if d, ok := readTask(lg.Lines, task); ok && prev != nil && prev.Duration == 0 {
					prev.Duration = d
				}
				prev = task
			}
		}
		for _, n := range r.Recap {
			r.Hosts = append(r.Hosts, readRecapRow(lg.Lines[n].Text))
		}
	}
}

// readTask walks a task body: result lines with their payloads, retry
// and ignore markers, the task path; blank lines are skipped and
// anything else is a stray line. It returns the previous task's
// duration when a timer callback printed it here.
func readTask(lines []Line, t *Task) (prevDuration time.Duration, timed bool) {
	retries := map[string]int{}
	for i := t.From; i < t.To; i++ {
		s := lines[i].Text
		if strings.TrimSpace(s) == "" {
			continue
		}
		if m := taskPath.FindStringSubmatch(s); m != nil && t.Path == "" {
			t.Path = m[1]
			t.PathLine, _ = strconv.Atoi(m[2])
			continue
		}
		// The timer prints a line as each task starts, also for tasks
		// whose header is hidden (display_skipped_hosts: false). The
		// first in a body times the task before; a second times this
		// one, as a hidden task started after it; the rest time hidden
		// tasks.
		if m := timerLine.FindStringSubmatch(s); m != nil {
			h, _ := strconv.Atoi(m[1])
			mi, _ := strconv.Atoi(m[2])
			sec, _ := strconv.ParseFloat(m[3], 64)
			d := time.Duration((float64(h*3600+mi*60) + sec) * float64(time.Second))
			switch {
			case !timed:
				prevDuration, timed = d, true
			case t.Duration == 0:
				t.Duration = d
			}
			continue
		}
		if m := retrying.FindStringSubmatch(s); m != nil {
			retries[m[1]]++
			continue
		}
		if ignoring.MatchString(s) {
			for k := len(t.Results) - 1; k >= 0; k-- {
				if t.Results[k].Status == Failed {
					t.Results[k].Ignored = true
					break
				}
			}
			continue
		}
		if m := included.FindStringSubmatch(s); m != nil {
			for _, h := range strings.Split(m[2], ", ") {
				t.Results = append(t.Results, Result{Line: i, End: i, Status: Included, Host: h, File: m[1], Item: m[3]})
			}
			continue
		}
		m := resultLine.FindStringSubmatchIndex(s)
		if m == nil {
			t.Other = append(t.Other, i)
			continue
		}
		r := Result{Line: i, End: i, Status: Status(s[m[2]:m[3]])}
		r.Host, r.Delegate, _ = strings.Cut(s[m[4]:m[5]], " -> ")
		if r.Status == "fatal" {
			r.Status = Failed
		}
		if m[6] >= 0 && s[m[6]:m[7]] == "UNREACHABLE" {
			r.Status = Unreachable
		}
		rest := s[m[1]:]
		r.Item, rest = item(rest)
		i = payload(lines, i, len(s)-len(rest), t.To, &r)
		r.Retries = retries[r.Host] + retries[""]
		delete(retries, r.Host)
		t.Results = append(t.Results, r)
	}
	return prevDuration, timed
}

// item takes a loop label off the rest of a result line: " (item=x)"
// or " => (item=x)", balancing brackets and quotes inside the label.
func item(rest string) (string, string) {
	s := strings.TrimLeft(rest, " ")
	s = strings.TrimPrefix(s, "=> ")
	if !strings.HasPrefix(s, "(item=") {
		return "", rest
	}
	depth, quote := 0, byte(0)
	for j := 0; j < len(s); j++ {
		c := s[j]
		switch {
		case quote != 0:
			if c == '\\' {
				j++
			} else if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			depth--
			if depth == 0 {
				return s[len("(item="):j], s[j+1:]
			}
		}
	}
	return s[len("(item="):], ""
}

// payload reads what follows "=>" on lines[i] from byte col: JSON
// (possibly over several lines), YAML (the yaml callback's indented
// block), or raw text. It returns the payload's last line.
func payload(lines []Line, i, col, limit int, r *Result) int {
	s := lines[i].Text
	at := strings.Index(s[col:], "=>")
	if at < 0 {
		return i
	}
	rest := strings.TrimSpace(s[col+at+2:])
	switch {
	case strings.HasPrefix(rest, "{") || strings.HasPrefix(rest, "["):
		start := col + at + 2 + strings.IndexAny(s[col+at+2:], "{[")
		end, endCol, ok := BraceEnd(lines[:limit], i, start)
		if ok {
			text := joinText(lines, i, start, end, endCol)
			var v any
			if json.Unmarshal([]byte(text), &v) == nil {
				r.Payload, r.Raw, r.Format, r.End = asMap(v), text, "json", end
				r.Censored = isCensored(r.Payload)
				return end
			}
			r.Raw, r.Format, r.End = text, "raw", end
			return end
		}
		r.Raw, r.Format = rest, "raw"
		return i
	default:
		// The yaml callback: optional "key=value" here, then lines
		// indented under the result.
		var doc []string
		if kv := inlineKV.FindStringSubmatch(rest); kv != nil {
			doc = append(doc, kv[1]+": "+kv[2])
		} else if rest != "" {
			r.Raw, r.Format = rest, "raw"
			return i
		}
		end := i
		for j := i + 1; j < limit; j++ {
			l := lines[j].Text
			if l == "" || (l[0] != ' ' && l[0] != '\t') {
				break
			}
			doc = append(doc, l)
			end = j
		}
		if len(doc) == 0 {
			return i
		}
		text := strings.Join(doc, "\n")
		var v map[string]any
		if yaml.Unmarshal([]byte(dedent(text)), &v) == nil && v != nil {
			r.Payload, r.Format = v, "yaml"
			r.Censored = isCensored(v)
		} else {
			r.Format = "raw"
		}
		r.Raw, r.End = text, end
		return end
	}
}

// joinText is the text from (i, col) to (end, endCol), lines joined by
// newlines (JSON strings never hold a raw newline, so this is exact).
func joinText(lines []Line, i, col, end, endCol int) string {
	if i == end {
		return lines[i].Text[col:endCol]
	}
	var b strings.Builder
	b.WriteString(lines[i].Text[col:])
	for j := i + 1; j < end; j++ {
		b.WriteByte('\n')
		b.WriteString(lines[j].Text)
	}
	b.WriteByte('\n')
	b.WriteString(lines[end].Text[:endCol])
	return b.String()
}

// dedent removes the indentation shared by the indented lines; a first
// line taken from the result line ("changed: false") has none.
func dedent(s string) string {
	lines := strings.Split(s, "\n")
	cut := -1
	for k, l := range lines {
		if strings.TrimSpace(l) == "" || k == 0 && !strings.HasPrefix(l, " ") {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, " \t"))
		if cut < 0 || n < cut {
			cut = n
		}
	}
	if cut <= 0 {
		return s
	}
	for k := range lines {
		if k == 0 && !strings.HasPrefix(lines[0], " ") {
			continue
		}
		if len(lines[k]) >= cut {
			lines[k] = lines[k][cut:]
		}
	}
	return strings.Join(lines, "\n")
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{"result": v}
}

func isCensored(m map[string]any) bool {
	s, ok := m["censored"].(string)
	return ok && strings.Contains(s, "no_log")
}

// readRecapRow reads "web-1 : ok=3 changed=1 unreachable=0 failed=0 …".
func readRecapRow(s string) HostRecap {
	host, counts, _ := strings.Cut(s, ":")
	h := HostRecap{Host: strings.TrimSpace(host)}
	for _, m := range recapCount.FindAllStringSubmatch(counts, -1) {
		n, _ := strconv.Atoi(m[2])
		switch m[1] {
		case "ok":
			h.OK = n
		case "changed":
			h.Changed = n
		case "unreachable":
			h.Unreachable = n
		case "failed":
			h.Failed = n
		case "skipped":
			h.Skipped = n
		case "rescued":
			h.Rescued = n
		case "ignored":
			h.Ignored = n
		}
	}
	return h
}

// readJSONRun reads the json stdout callback's document: plays, tasks
// with a result per host, and stats as the recap.
func readJSONRun(lg *Log, b Block, r *Run) {
	var doc struct {
		Plays []struct {
			Play struct {
				Name string `json:"name"`
			} `json:"play"`
			Tasks []struct {
				Task struct {
					Name string `json:"name"`
				} `json:"task"`
				Hosts map[string]map[string]any `json:"hosts"`
			} `json:"tasks"`
		} `json:"plays"`
		Stats map[string]map[string]int `json:"stats"`
	}
	text := joinText(lg.Lines, b.From, 0, b.To-1, len(lg.Lines[b.To-1].Text))
	if json.Unmarshal([]byte(text), &doc) != nil {
		return
	}
	for _, p := range doc.Plays {
		play := Play{Line: -1, Name: p.Play.Name}
		for _, t := range p.Tasks {
			task := Task{Line: -1, Name: t.Task.Name, From: b.From, To: b.From}
			if role, rest, ok := strings.Cut(task.Name, " : "); ok {
				task.Role, task.Name = role, rest
			}
			for _, host := range sortedKeys(t.Hosts) {
				res := t.Hosts[host]
				raw, _ := json.Marshal(res)
				task.Results = append(task.Results, Result{Line: -1, End: -1, Status: statusOf(res), Host: host,
					Payload: res, Raw: string(raw), Format: "json", Censored: isCensored(res)})
			}
			play.Tasks = append(play.Tasks, task)
		}
		r.Plays = append(r.Plays, play)
	}
	for _, host := range sortedKeys(doc.Stats) {
		s := doc.Stats[host]
		r.Hosts = append(r.Hosts, HostRecap{Host: host, OK: s["ok"], Changed: s["changed"], Unreachable: s["unreachable"],
			Failed: s["failures"], Skipped: s["skipped"], Rescued: s["rescued"], Ignored: s["ignored"]})
	}
}

// statusOf derives a status from a result, as the default callback does.
func statusOf(res map[string]any) Status {
	switch {
	case res["unreachable"] == true:
		return Unreachable
	case res["failed"] == true:
		return Failed
	case res["skipped"] == true:
		return Skipped
	case res["changed"] == true:
		return Changed
	}
	return OK
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
