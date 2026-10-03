package ansiblelog

import (
	"regexp"
	"strings"
	"time"
)

// Analysis is a whole log as the UI and CLI show it: its blocks in
// order (runs and other output) and a summary. Line numbers are the
// input's, 1-based.
type Analysis struct {
	Lines   int         `json:"lines"`
	Blocks  []BlockView `json:"blocks"`
	Summary Summary     `json:"summary"`
}

// Summary is the headline of a log.
type Summary struct {
	Runs        int      `json:"runs"`
	Hosts       []string `json:"hosts"`
	Tasks       int      `json:"tasks"`
	OK          int      `json:"ok"`
	Changed     int      `json:"changed"`
	Failed      int      `json:"failed"`
	Unreachable int      `json:"unreachable"`
	Skipped     int      `json:"skipped"`
	Rescued     int      `json:"rescued"`
	Ignored     int      `json:"ignored"`
	DurationMs  int64    `json:"durationMs,omitempty"` // first to last timestamp
	Other       int      `json:"other"`                // other-output blocks
	OtherErrors int      `json:"otherErrors"`          // error lines in them
	// First failed result, as indexes: Blocks[Block].Run.Plays[Play].
	// Tasks[Task].Results[Result]; nil when nothing failed.
	FirstFailure *Ref `json:"firstFailure,omitempty"`
	// Verdict is one sentence for when the log's outcome isn't Ansible's
	// alone ("Ansible succeeded; the step failed after it").
	Verdict string `json:"verdict,omitempty"`
}

// Ref points at a result.
type Ref struct {
	Block, Play, Task, Result int
}

// BlockView is a run or a stretch of other output.
type BlockView struct {
	Kind  string   `json:"kind"` // "run" or "other"
	From  int      `json:"from"` // first and last input line
	To    int      `json:"to"`
	Title string   `json:"title,omitempty"` // CI group ("step_script")
	Run   *RunView `json:"run,omitempty"`
	// Other output: its cleaned lines (capped) and what they flag.
	Text      []string `json:"text,omitempty"`
	Truncated bool     `json:"truncated,omitempty"`
	Errors    int      `json:"errors,omitempty"`
	Warnings  int      `json:"warnings,omitempty"`
}

// RunView is one ansible-playbook run.
type RunView struct {
	Playbook   string      `json:"playbook,omitempty"`
	JSON       bool        `json:"json,omitempty"` // json stdout callback
	Complete   bool        `json:"complete"`       // ended with its recap
	Status     Status      `json:"status"`         // failed, unreachable, changed or ok
	DurationMs int64       `json:"durationMs,omitempty"`
	Hosts      []string    `json:"hosts"`
	Plays      []PlayView  `json:"plays"`
	Recap      []HostRecap `json:"recap"`
	Notes      []Note      `json:"notes"`
}

// Note is a warning, deprecation or ERROR! line Ansible printed.
type Note struct {
	Line  int    `json:"line"`
	Level string `json:"level"` // "error", "warning" or "deprecation"
	Text  string `json:"text"`
}

// PlayView is a play and its tasks.
type PlayView struct {
	Name  string     `json:"name"`
	Line  int        `json:"line,omitempty"` // 0: no header
	Tasks []TaskView `json:"tasks"`
}

// TaskView is a task or handler with its results.
type TaskView struct {
	Name       string       `json:"name"`
	Role       string       `json:"role,omitempty"`
	Handler    bool         `json:"handler,omitempty"`
	Line       int          `json:"line,omitempty"`
	Path       string       `json:"path,omitempty"` // defining file (-vv)
	PathLine   int          `json:"pathLine,omitempty"`
	DurationMs int64        `json:"durationMs,omitempty"`
	Status     Status       `json:"status"` // the worst of its results
	Counts     Counts       `json:"counts"`
	Results    []ResultView `json:"results"`
	Other      []string     `json:"other,omitempty"` // stray lines (capped)
}

// Counts are results per status.
type Counts struct {
	OK          int `json:"ok,omitempty"`
	Changed     int `json:"changed,omitempty"`
	Failed      int `json:"failed,omitempty"`
	Unreachable int `json:"unreachable,omitempty"`
	Skipped     int `json:"skipped,omitempty"`
	Ignored     int `json:"ignored,omitempty"`
	Rescued     int `json:"rescued,omitempty"`
	Included    int `json:"included,omitempty"`
}

// ResultView is one host's result.
type ResultView struct {
	Host     string         `json:"host"`
	Status   Status         `json:"status"`
	Line     int            `json:"line,omitempty"`
	EndLine  int            `json:"endLine,omitempty"`
	Delegate string         `json:"delegate,omitempty"`
	Item     string         `json:"item,omitempty"`
	File     string         `json:"file,omitempty"`
	Retries  int            `json:"retries,omitempty"`
	Ignored  bool           `json:"ignored,omitempty"`
	Rescued  bool           `json:"rescued,omitempty"` // the host carried on: a rescue block ran
	Censored bool           `json:"censored,omitempty"`
	Msg      string         `json:"msg,omitempty"` // the short reason, for lists
	Format   string         `json:"format,omitempty"`
	Payload  map[string]any `json:"payload,omitempty"`
	Raw      string         `json:"raw,omitempty"` // only when it isn't JSON
}

// Caps keep the response proportionate: other output and stray lines
// are for orientation; the full text is the input itself.
const (
	maxOtherLines = 2000
	maxTaskOther  = 200
)

var (
	// Whole words only, and not inside a hyphenated one ("on-error").
	errorWord = regexp.MustCompile(`(?i)(?:^|[^\w-])(?:error|fatal|failed|failure|exception|traceback|panic)(?:$|[^\w-])`)
	warnWord  = regexp.MustCompile(`(?i)(?:^|[^\w-])warn(?:ing)?(?:$|[^\w-])`)
	// Lines that only report a failure counted elsewhere, or none.
	notError = regexp.MustCompile(`(?i)\b(?:failed=0|errors?: 0|0 errors?|no errors?)\b`)
)

// Analyze reads a whole log: Clean, Segment, Read, then the model.
func Analyze(input string) *Analysis {
	lg := Segment(Clean(input))
	lg.Total = strings.Count(strings.TrimSuffix(input, "\n"), "\n") + 1
	Read(lg)
	return Model(lg)
}

// Model turns a read log into its Analysis.
func Model(lg *Log) *Analysis {
	a := &Analysis{Blocks: []BlockView{}}
	a.Lines = lg.Total
	// A block ends on the input line before the next block starts, so
	// continuation lines joined into its last line count as its own.
	lastLine := func(b Block) int {
		if b.To < len(lg.Lines) {
			return lg.Lines[b.To].N - 1
		}
		return lg.Total
	}
	sum := &a.Summary
	sum.Hosts = []string{}
	hosts := map[string]bool{}
	failedRun, lastRun := false, -1
	for bi, b := range lg.Blocks {
		bv := BlockView{Kind: "other", From: lg.Lines[b.From].N, To: lastLine(b), Title: b.Title}
		if b.Run == nil {
			for k, l := range lg.Lines[b.From:b.To] {
				if l.Kind != Text {
					continue
				}
				if k < maxOtherLines {
					bv.Text = append(bv.Text, l.Text)
				} else {
					bv.Truncated = true
				}
				// The CI's own flags count; plain stderr doesn't (tools
				// write progress and events there).
				switch {
				case l.Flag == "error" || errorWord.MatchString(l.Text) && !notError.MatchString(l.Text):
					bv.Errors++
				case l.Flag == "warning" || warnWord.MatchString(l.Text):
					bv.Warnings++
				}
			}
			sum.Other++
			sum.OtherErrors += bv.Errors
			a.Blocks = append(a.Blocks, bv)
			continue
		}
		bv.Kind = "run"
		bv.Run = runView(lg, b)
		r := bv.Run
		sum.Runs++
		lastRun = bi
		for _, h := range r.Hosts {
			if !hosts[h] {
				hosts[h] = true
				sum.Hosts = append(sum.Hosts, h)
			}
		}
		for pi, p := range r.Plays {
			sum.Tasks += len(p.Tasks)
			for ti, t := range p.Tasks {
				for ri, res := range t.Results {
					if (res.Status == Failed || res.Status == Unreachable) && !res.Ignored && !res.Rescued && sum.FirstFailure == nil {
						sum.FirstFailure = &Ref{bi, pi, ti, ri}
					}
				}
			}
		}
		// The recap is Ansible's own count; without one, count results.
		if len(r.Recap) > 0 {
			for _, h := range r.Recap {
				sum.OK += h.OK
				sum.Changed += h.Changed
				sum.Failed += h.Failed
				sum.Unreachable += h.Unreachable
				sum.Skipped += h.Skipped
				sum.Rescued += h.Rescued
				sum.Ignored += h.Ignored
			}
		} else {
			for _, p := range r.Plays {
				for _, t := range p.Tasks {
					sum.OK += t.Counts.OK
					sum.Changed += t.Counts.Changed
					sum.Failed += t.Counts.Failed
					sum.Unreachable += t.Counts.Unreachable
					sum.Skipped += t.Counts.Skipped
					sum.Ignored += t.Counts.Ignored
				}
			}
		}
		failedRun = failedRun || r.Status == Failed || r.Status == Unreachable || r.Status == Unfinished
		a.Blocks = append(a.Blocks, bv)
	}
	if first, last := span(lg.Lines); !first.IsZero() {
		sum.DurationMs = last.Sub(first).Milliseconds()
	}
	sum.Verdict = verdict(a, failedRun, lastRun)
	return a
}

// verdict says when the log's outcome isn't Ansible's alone.
func verdict(a *Analysis, failedRun bool, lastRun int) string {
	if lastRun < 0 {
		if a.Summary.OtherErrors > 0 {
			return "No Ansible run started; the other output has errors."
		}
		return ""
	}
	for _, b := range a.Blocks {
		if b.Run != nil && !b.Run.Complete && b.Run.Status != Failed && b.Run.Status != Unreachable {
			return "An Ansible run stopped before its recap."
		}
	}
	if failedRun {
		return ""
	}
	for _, b := range a.Blocks[lastRun+1:] {
		if b.Errors > 0 {
			return "Ansible succeeded; the output after it mentions errors."
		}
	}
	return ""
}

func runView(lg *Log, b Block) *RunView {
	r := b.Run
	rv := &RunView{Playbook: r.Playbook, JSON: r.JSON, Complete: r.Complete, Status: OK,
		Hosts: []string{}, Plays: []PlayView{}, Recap: r.Hosts, Notes: []Note{}}
	if rv.Recap == nil {
		rv.Recap = []HostRecap{}
	}
	if first, last := span(lg.Lines[b.From:b.To]); !first.IsZero() {
		rv.DurationMs = last.Sub(first).Milliseconds()
	}
	for _, n := range r.Notes {
		t := lg.Lines[n].Text
		level := "warning"
		switch {
		case strings.HasPrefix(t, "ERROR!"):
			level = "error"
		case strings.HasPrefix(t, "[DEPRECATION"):
			level = "deprecation"
		}
		rv.Notes = append(rv.Notes, Note{Line: lg.Lines[n].N, Level: level, Text: t})
	}
	seen := map[string]bool{}
	addHost := func(h string) {
		if h != "" && !seen[h] {
			seen[h] = true
			rv.Hosts = append(rv.Hosts, h)
		}
	}
	for _, h := range r.Hosts {
		addHost(h.Host)
	}
	// Tasks in order, to time each by the next one's start when no
	// timer callback did.
	var all []*TaskView
	var starts []time.Time
	// A failed host runs no more tasks unless a rescue block caught the
	// failure, so a failure followed by more results for the same host
	// in the same play was rescued.
	budget := map[string]int{} // rescues the recap counts per host
	for _, h := range r.Hosts {
		budget[h.Host] = h.Rescued
	}
	last := make([]map[string]int, len(r.Plays)) // per play: host → its last task
	for pi, p := range r.Plays {
		last[pi] = map[string]int{}
		for ti, t := range p.Tasks {
			for _, res := range t.Results {
				last[pi][res.Host] = ti
			}
		}
	}
	for pi, p := range r.Plays {
		pv := PlayView{Name: p.Name, Tasks: []TaskView{}}
		if p.Line >= 0 {
			pv.Line = lg.Lines[p.Line].N
		}
		for ti, t := range p.Tasks {
			rescued := func(host string) bool {
				if last[pi][host] <= ti {
					return false
				}
				// always: blocks and forced handlers also run after a
				// failure; the recap says how many were rescued.
				if len(r.Hosts) > 0 {
					if budget[host] == 0 {
						return false
					}
					budget[host]--
				}
				return true
			}
			tv := taskView(lg, t, rescued)
			for _, res := range tv.Results {
				addHost(res.Host)
			}
			st := tv.Status
			if st == Rescued {
				st = Changed
			}
			rv.Status = worse(rv.Status, st)
			pv.Tasks = append(pv.Tasks, tv)
		}
		rv.Plays = append(rv.Plays, pv)
	}
	// A run cut short stopped in its last task; with no results yet, that
	// task never finished.
	if n := len(rv.Plays); !r.Complete && !r.JSON && n > 0 {
		if tasks := rv.Plays[n-1].Tasks; len(tasks) > 0 && len(tasks[len(tasks)-1].Results) == 0 {
			tasks[len(tasks)-1].Status = Unfinished
			rv.Status = worse(rv.Status, Unfinished)
		}
	}
	for pi := range rv.Plays {
		for ti := range rv.Plays[pi].Tasks {
			t := r.Plays[pi].Tasks[ti]
			var at time.Time
			if t.Line >= 0 {
				at = lg.Lines[t.Line].Time
			}
			all = append(all, &rv.Plays[pi].Tasks[ti])
			starts = append(starts, at)
		}
	}
	end := lg.Lines[b.To-1].Time
	for k, tv := range all {
		if tv.DurationMs > 0 || starts[k].IsZero() {
			continue
		}
		next := end
		if k+1 < len(starts) {
			next = starts[k+1]
		}
		if !next.IsZero() && next.After(starts[k]) {
			tv.DurationMs = next.Sub(starts[k]).Milliseconds()
		}
	}
	return rv
}

func taskView(lg *Log, t Task, rescued func(host string) bool) TaskView {
	tv := TaskView{Name: t.Name, Role: t.Role, Handler: t.Handler, Path: t.Path, PathLine: t.PathLine,
		DurationMs: t.Duration.Milliseconds(), Status: Skipped, Results: []ResultView{}}
	if t.Line >= 0 {
		tv.Line = lg.Lines[t.Line].N
	}
	if len(t.Results) == 0 {
		tv.Status = OK
	}
	for _, r := range t.Results {
		rv := ResultView{Host: r.Host, Status: r.Status, Delegate: r.Delegate, Item: r.Item, File: r.File,
			Retries: r.Retries, Ignored: r.Ignored, Censored: r.Censored, Format: r.Format, Payload: r.Payload, Msg: msg(r)}
		if r.Line >= 0 {
			rv.Line, rv.EndLine = lg.Lines[r.Line].N, lg.Lines[r.End].N
		}
		if r.Format != "json" {
			rv.Raw = r.Raw
		}
		rv.Rescued = r.Status == Failed && !r.Ignored && rescued(r.Host)
		switch {
		case r.Ignored:
			tv.Counts.Ignored++
		case rv.Rescued:
			tv.Counts.Rescued++
		case r.Status == OK:
			tv.Counts.OK++
		case r.Status == Changed:
			tv.Counts.Changed++
		case r.Status == Failed:
			tv.Counts.Failed++
		case r.Status == Unreachable:
			tv.Counts.Unreachable++
		case r.Status == Skipped:
			tv.Counts.Skipped++
		case r.Status == Included:
			tv.Counts.Included++
		}
		st := r.Status
		switch {
		case r.Ignored || st == Included:
			st = OK
		case rv.Rescued:
			st = Rescued
		}
		tv.Status = worse(tv.Status, st)
		tv.Results = append(tv.Results, rv)
	}
	for k, n := range t.Other {
		if k == maxTaskOther {
			break
		}
		tv.Other = append(tv.Other, lg.Lines[n].Text)
	}
	return tv
}

// worse is the status that matters more in a summary.
var statusRank = map[Status]int{Skipped: 0, OK: 1, Changed: 2, Rescued: 3, Unfinished: 4, Unreachable: 5, Failed: 6}

func worse(a, b Status) Status {
	if statusRank[b] > statusRank[a] {
		return b
	}
	return a
}

// msg is a result's short reason: msg, else the first stderr line, else
// the module's reason for skipping.
func msg(r Result) string {
	if r.Censored {
		return "output hidden by no_log"
	}
	first := func(k string) string {
		s, _ := r.Payload[k].(string)
		s = strings.TrimSpace(s)
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			s = s[:i]
		}
		if rs := []rune(s); len(rs) > 300 {
			s = string(rs[:300]) + "…"
		}
		return s
	}
	for _, k := range []string{"msg", "stderr", "skip_reason", "reason"} {
		if s := first(k); s != "" {
			// "non-zero return code" says little; stderr says why.
			if e := first("stderr"); k == "msg" && e != "" && !strings.Contains(s, e) {
				s += ": " + e
			}
			return s
		}
	}
	return ""
}

// span is the first and last timestamp among lines.
func span(lines []Line) (first, last time.Time) {
	for _, l := range lines {
		if !l.Time.IsZero() {
			if first.IsZero() {
				first = l.Time
			}
			last = l.Time
		}
	}
	return first, last
}
