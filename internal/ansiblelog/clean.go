// Package ansiblelog reads whole Ansible logs as pipelines print them:
// CI timestamps, wrapper prefixes, colour codes, other tools' output
// and several playbook runs mixed together. It is a pure engine package.
//
// Reading happens in stages, each degrading instead of failing:
// Clean (this file) turns raw text into plain lines; later stages
// segment the lines into runs and read the task results.
package ansiblelog

import (
	"regexp"
	"slices"
	"strings"
	"time"
)

// Kind tells a text line from a CI group marker.
type Kind uint8

const (
	Text       Kind = iota
	GroupStart      // GitLab section_start, ##[group], ::group:: (Text is the title)
	GroupEnd
)

// Line is one cleaned line.
type Line struct {
	N    int       // input line it starts on, 1-based
	Text string    // without CI prefix, wrapper prefix, colour codes and redraws
	Time time.Time // from the CI prefix; zero when the log has none
	Err  bool      // the CI marked it as stderr (GitLab "00E", ##[error])
	Flag string    // the CI flagged it: "error" or "warning" (##[error], ##[warning])
	Wrap string    // the wrapper prefix removed from it ("azure-arm"), if any
	Kind Kind
}

var (
	// GitLab's raw job log: "2026-10-01T20:50:33.921608Z 00O text"; a
	// "+" after the stream ("00O+text") continues the previous line,
	// which GitLab splits every few KB.
	gitlabPrefix = regexp.MustCompile(`^(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d+)?Z) \d\d[OE]([ +])`)
	// Azure DevOps and GitHub Actions: "2026-10-01T20:50:33.1234567Z ";
	// Jenkins' timestamper and others: "[2026-10-01T20:50:33.123Z] ".
	isoPrefix = regexp.MustCompile(`^\[?(\d{4}-\d\d-\d\d[T ]\d\d:\d\d:\d\d(?:[.,]\d+)?(?:Z|[+-]\d\d:?\d\d)?)\]?(?: |$)`)
	// CSI and OSC escape sequences, then any other lone escape.
	ansi          = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b[@-Z\\-_]`)
	gitlabSection = regexp.MustCompile(`^section_(start|end):\d+:([^\[\s]+)`)
	// What a wrapper prefix may look like: a short label ending in ":",
	// "|" or ">" ("    azure-arm: ", "web-1  | ", "host> "), or indentation.
	wrapperShape = regexp.MustCompile(`^\s*(?:([\w.@/-]{1,40}(?:\s+\w[\w.-]*)?)\s*[:|>])?\s*$`)
)

// Clean strips what pipelines add around Ansible's own output. Every
// input line ends up in exactly one Line (continuations are joined to
// the line they continue), so nothing is dropped.
func Clean(input string) []Line {
	raw := strings.Split(strings.TrimSuffix(input, "\n"), "\n")
	// First the records: a continuation's raw text joins the line it
	// continues, so a colour code or JSON value split between pieces is
	// whole again before anything is stripped.
	type record struct {
		n     int
		parts []string
		time  time.Time
		err   bool
	}
	isMarker := func(s string) bool { return strings.HasPrefix(stripANSI(s), "section_") }
	recs := make([]record, 0, len(raw))
	for i, s := range raw {
		r := record{n: i + 1}
		cont := false
		if m := gitlabPrefix.FindStringSubmatchIndex(s); m != nil {
			r.time = parseTime(s[m[2]:m[3]])
			r.err = s[m[1]-2] == 'E'
			cont = s[m[4]] == '+'
			s = s[m[1]:]
		} else if m := isoPrefix.FindStringSubmatchIndex(s); m != nil {
			if t := parseTime(s[m[2]:m[3]]); !t.IsZero() {
				r.time = t
				s = s[m[1]:]
			}
		}
		// Section markers stand alone, even when GitLab writes them as
		// a continuation.
		if cont && len(recs) > 0 && !isMarker(s) && !isMarker(recs[len(recs)-1].parts[0]) {
			prev := &recs[len(recs)-1]
			// A piece's trailing \r ended its record, it isn't a redraw.
			last := len(prev.parts) - 1
			prev.parts[last] = strings.TrimSuffix(prev.parts[last], "\r")
			prev.parts = append(prev.parts, s)
			if prev.time.IsZero() {
				prev.time = r.time
			}
			continue
		}
		r.parts = []string{s}
		recs = append(recs, r)
	}
	out := make([]Line, 0, len(recs))
	for _, r := range recs {
		s := r.parts[0]
		if len(r.parts) > 1 {
			s = strings.Join(r.parts, "")
		}
		l := Line{N: r.n, Time: r.time, Err: r.err}
		s = redraw(stripANSI(s))
		l.Kind, l.Text = marker(s, &l)
		out = append(out, l)
	}
	unwrap(out)
	return out
}

// stripANSI removes colour and cursor codes.
func stripANSI(s string) string {
	if !strings.Contains(s, "\x1b") {
		return s
	}
	return ansi.ReplaceAllString(s, "")
}

// redraw keeps what a terminal would show after carriage returns: each
// "\r" starts the line over, so the last non-empty part wins.
func redraw(s string) string {
	s = strings.TrimRight(s, "\r")
	if i := strings.LastIndexByte(s, '\r'); i >= 0 {
		s = s[i+1:]
	}
	return s
}

// marker recognises CI group and log-command markers. Group markers
// become GroupStart/GroupEnd lines; other commands lose their tag.
func marker(s string, l *Line) (Kind, string) {
	if m := gitlabSection.FindStringSubmatch(s); m != nil {
		if m[1] == "start" {
			return GroupStart, m[2]
		}
		return GroupEnd, m[2]
	}
	for _, p := range []struct {
		tag  string
		kind Kind
	}{{"##[group]", GroupStart}, {"##[endgroup]", GroupEnd}, {"::group::", GroupStart}, {"::endgroup::", GroupEnd}} {
		if rest, ok := strings.CutPrefix(s, p.tag); ok {
			return p.kind, rest
		}
	}
	if strings.HasPrefix(s, "##[") {
		if end := strings.IndexByte(s, ']'); end > 0 {
			switch tag := s[3:end]; tag {
			case "error":
				l.Err, l.Flag = true, tag
			case "warning":
				l.Flag = tag
			}
			return Text, s[end+1:]
		}
	}
	return Text, s
}

// unwrap learns wrapper prefixes from the text seen before Ansible
// headers and results (Packer's "    azure-arm: ", Compose's "web-1  | ") and
// removes them from every line that carries one, so later stages see
// Ansible's output as Ansible printed it.
func unwrap(lines []Line) {
	counts := map[string]int{}
	for _, l := range lines {
		if l.Kind != Text {
			continue
		}
		if at := headerIndex(l.Text); at > 0 {
			if p := l.Text[:at]; strings.TrimSpace(p) != "" && wrapperShape.MatchString(p) {
				counts[p]++
			}
		}
	}
	// A run prints at least a header and a result; one sighting is
	// more likely a quoted header than a wrapper.
	var prefixes []string
	for p, n := range counts {
		if n >= 2 {
			prefixes = append(prefixes, p)
		}
	}
	if len(prefixes) == 0 {
		return
	}
	// Longest first, so "web-1  | " wins over a shorter overlap.
	slices.SortFunc(prefixes, func(a, b string) int { return len(b) - len(a) })
	for i := range lines {
		l := &lines[i]
		if l.Kind != Text {
			continue
		}
		for _, p := range prefixes {
			// Wrappers print blank lines as the bare label ("    azure-arm:").
			rest, ok := strings.CutPrefix(l.Text, p)
			if !ok && l.Text == strings.TrimRight(p, " \t") {
				rest, ok = "", true
			}
			if ok {
				l.Text, l.Wrap = rest, label(p)
				break
			}
		}
	}
}

// headerIndex is where the first Ansible header, result or task path
// starts in s, or -1. Results count too: a single task copied from a
// wrapped log may have lost the prefix of its only header. Plain
// substring searches: this runs on every line of a big log.
func headerIndex(s string) int {
	first := -1
	for _, h := range [...]string{"PLAY [", "TASK [", "RUNNING HANDLER [", "PLAY RECAP *",
		"ok: [", "changed: [", "skipping: [", "failed: [", "fatal: [", "included: ", "task path: "} {
		if i := strings.Index(s, h); i >= 0 && (first < 0 || i < first) {
			first = i
		}
	}
	return first
}

// label is the name inside a wrapper prefix: "azure-arm" for "    azure-arm: ".
func label(p string) string {
	if m := wrapperShape.FindStringSubmatch(p); m != nil && m[1] != "" {
		return m[1]
	}
	return strings.TrimSpace(p)
}

func parseTime(s string) time.Time {
	s = strings.Replace(s, ",", ".", 1)
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999Z07:00", "2006-01-02T15:04:05.999999999", "2006-01-02 15:04:05.999999999", "2006-01-02T15:04:05.999999999Z0700", "2006-01-02 15:04:05.999999999Z0700"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
