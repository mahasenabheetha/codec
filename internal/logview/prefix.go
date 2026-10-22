package logview

import (
	"regexp"
	"strings"
	"time"
)

// What tools put in front of a log line. Time prefixes are always
// recognised; a label prefix (the line's source) only when most lines
// carry the same shape, because "error: x" also looks like "pod: x".
var (
	// Container runtimes (CRI, the files kubelet keeps):
	// "2026-10-22T10:00:00.123456789Z stdout F text"; P marks a line the
	// runtime split, continued on the next one.
	criPrefix = regexp.MustCompile(`^(\d{4}-\d\d-\d\dT[\d:.]+(?:Z|[+-]\d\d:\d\d)) (stdout|stderr) ([FP]) ?`)
	// GitLab's raw job log: "2026-10-22T10:00:00.123456Z 00O text".
	gitlabPrefix = regexp.MustCompile(`^(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d+)?Z) \d\d[OE][ +]`)
	// kubectl logs --timestamps, Azure DevOps, GitHub Actions, Jenkins'
	// timestamper: an RFC 3339 time with a zone, perhaps in brackets.
	isoPrefix = regexp.MustCompile(`^\[?(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d+)?(?:Z|[+-]\d\d:?\d\d))\]? `)
	ansi      = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b[@-Z\\-_]`)
)

type labelShape uint8

const (
	noLabel      labelShape = iota
	kubectlLabel            // kubectl logs --prefix: "[pod/web-7d9c/app] "
	composeLabel            // docker compose logs: "web-1  | "
	podLabel                // argo logs: "my-wf-step-1234567890: "
	sternLabel              // stern: "web-7d9c8b6f5-x2x9z app "
	labelShapes
)

// An empty line keeps its label, often without the space after it.
var labelRes = [labelShapes]*regexp.Regexp{
	kubectlLabel: regexp.MustCompile(`^\[((?:pod|pods|deployment|deploy|job|statefulset|daemonset|replicaset)/[^\]\s]+)\](?: |$)`),
	composeLabel: regexp.MustCompile(`^([\w.-]+)\s*\|(?: |$)`),
	podLabel:     regexp.MustCompile(`^([a-z0-9](?:[a-z0-9.-]{0,251}[a-z0-9])?):(?: |$)`),
	sternLabel:   regexp.MustCompile(`^([a-z0-9](?:[a-z0-9.-]*[a-z0-9])?-[a-z0-9]{5}) ([a-z0-9](?:[-a-z0-9]*[a-z0-9])?)(?: |$)`),
}

// prefix is what was taken off the front of a line.
type prefix struct {
	time    time.Time
	source  string
	stream  string // stdout or stderr, from a container runtime or Docker
	partial bool   // the runtime continues this line on the next one
	fields  []Field
}

// timePrefix removes one time prefix.
func timePrefix(s string, p *prefix) string {
	if s == "" || (s[0] < '0' || s[0] > '9') && s[0] != '[' {
		return s
	}
	if m := criPrefix.FindStringSubmatchIndex(s); m != nil {
		if t, ok := parseTime(s[m[2]:m[3]]); ok {
			p.time, p.stream, p.partial = t, s[m[4]:m[5]], s[m[6]] == 'P'
			return s[m[1]:]
		}
	}
	for _, re := range [...]*regexp.Regexp{gitlabPrefix, isoPrefix} {
		if m := re.FindStringSubmatchIndex(s); m != nil {
			if t, ok := parseTime(s[m[2]:m[3]]); ok {
				p.time = t
				return s[m[1]:]
			}
		}
	}
	return s
}

// label removes a label prefix of the given shape and names the source.
func label(s string, shape labelShape, p *prefix) (string, bool) {
	if shape == noLabel {
		return s, false
	}
	m := labelRes[shape].FindStringSubmatchIndex(s)
	if m == nil {
		return s, false
	}
	src := s[m[2]:m[3]]
	if shape == sternLabel {
		src += "/" + s[m[4]:m[5]]
	}
	// A level word ("error: x") or a lone word ("response: {") is text:
	// pod names have dashes.
	if ParseLevel(src) != LevelNone || shape == podLabel && !strings.Contains(src, "-") {
		return s, false
	}
	p.source = src
	return s[m[1]:], true
}

// learnLabel picks the label shape at least half the sample's lines
// carry, so it can be removed from every line. Labels that are level
// words ("error: ", "warning: ") don't count.
func learnLabel(sample []string) labelShape {
	var counts [labelShapes]int
	lines := 0
	for _, s := range sample {
		if strings.TrimSpace(s) == "" {
			continue
		}
		lines++
		var p prefix
		s = timePrefix(s, &p)
		for sh := kubectlLabel; sh < labelShapes; sh++ {
			if _, ok := label(s, sh, &prefix{}); ok {
				counts[sh]++
			}
		}
	}
	best := noLabel
	for sh := kubectlLabel; sh < labelShapes; sh++ {
		if counts[sh] > counts[best] {
			best = sh
		}
	}
	if best == noLabel || counts[best]*2 < lines {
		return noLabel
	}
	return best
}

// jsonAfterLabel finds a JSON object behind a label prefix of any shape
// when no shape is learned (a few pasted lines): "my-pod-7d9c: {...}".
func jsonAfterLabel(s string, p *prefix) (string, bool) {
	i := strings.IndexByte(s, '{')
	if i <= 0 || i > 256 {
		return s, false
	}
	pre := s[:i]
	if strings.TrimSpace(pre) == "" {
		return s[i:], true
	}
	for sh := kubectlLabel; sh < labelShapes; sh++ {
		var q prefix
		if rest, ok := label(pre, sh, &q); ok && strings.TrimSpace(timePrefix(rest, &q)) == "" {
			p.source = q.source
			if !q.time.IsZero() {
				p.time = q.time
			}
			return s[i:], true
		}
	}
	return s, false
}

// stripANSI removes colour and cursor codes.
func stripANSI(s string) string {
	if !strings.Contains(s, "\x1b") {
		return s
	}
	return ansi.ReplaceAllString(s, "")
}

// redraw keeps what a terminal would show after carriage returns: each
// "\r" starts the line over, so the last part wins.
func redraw(s string) string {
	s = strings.TrimRight(s, "\r")
	if i := strings.LastIndexByte(s, '\r'); i >= 0 {
		s = s[i+1:]
	}
	return s
}
