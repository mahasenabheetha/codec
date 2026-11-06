package logsrc

import (
	"bytes"
	"cmp"
	"context"
	"regexp"
	"slices"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/search"
)

// Finding the code that wrote a log line (decision 99, T3). The log
// holds the message as rendered ("order 7 failed for bob"); the code
// holds its template ("order %d failed for %s", f"order {id} failed").
// So the message is cut at its variable parts into the fixed parts the
// code must contain, those are searched, and each hit is scored by what
// is around it.

// Layer is one message searched: the whole message, or one layer of a
// wrapped error ("checkout: add item: cart is full").
type Layer struct {
	Text    string   `json:"text"`
	Phrases []string `json:"phrases"` // what was searched for, longest first
	Hits    []Hit    `json:"hits"`    // best first
	// res finds each phrase in a line as it is, ignoring case: a
	// lower-cased copy can be longer than the line (invalid UTF-8, some
	// runes), so its offsets don't hold in the line.
	res []*regexp.Regexp
}

// Hit is a line the layer may come from.
type Hit struct {
	search.Match
	Quality Quality  `json:"quality"`
	Score   int      `json:"score"`
	Why     []string `json:"why"` // what counted, for the badge's tip
}

// MaxHits bounds the hits listed per layer.
const MaxHits = 30

// LineQuery is what an entry gives to search for.
type LineQuery struct {
	Message string   // the message
	Error   string   // the entry's error, often a wrapped chain
	Fields  []string // the entry's field names (a structured log's keys)
}

// Variable parts: quoted strings, anything with a digit (counts, ids,
// times, versions, pod-7f9c), paths and URLs, e-mail addresses, the
// value after "key=". What is left between them is fixed text.
var slotRe = regexp.MustCompile(`"[^"]*"|'[^']*'|` + "`[^`]*`" + `|[a-zA-Z][\w+.-]*://\S+|\S+@\S+\.\w+|[\w.~-]*[/\\][\w./\\~-]+|=\S+|[\w.:+-]*\d[\w.:+-]*`)

// edges are stripped from the ends of a fixed part.
const edges = " \t:;,.=()[]{}<>|-–—!?#*\"'`"

// Phrases returns the fixed parts of a message worth searching for,
// longest first: each part between variable ones, and for a part of
// three words or more also the part without its first or its last
// word, in case that word was a variable too ("user bob logged in"),
// when three words are left.
func Phrases(message string) []string {
	var parts []string
	at := 0
	for _, m := range slotRe.FindAllStringIndex(message, -1) {
		parts = append(parts, message[at:m[0]])
		at = m[1]
	}
	parts = append(parts, message[at:])

	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		p = strings.Trim(p, edges)
		k := strings.ToLower(p)
		if distinctive(p) && !seen[k] {
			seen[k] = true
			out = append(out, p)
		}
	}
	for _, p := range parts {
		p = strings.Trim(p, edges)
		add(p)
		// A part less one word is searched only when three words are
		// left: two common ones ("failed for") would match everywhere and
		// crowd out the part itself.
		if w := strings.Fields(p); len(w) >= 4 {
			add(strings.Join(w[1:], " "))
			add(strings.Join(w[:len(w)-1], " "))
		}
	}
	slices.SortStableFunc(out, func(a, b string) int { return len(b) - len(a) })
	return out
}

// distinctive: two words, or one long one ("Unauthorized"), with
// letters; "failed" or "error" alone would match everywhere.
func distinctive(p string) bool {
	letters := 0
	for _, c := range p {
		if isLetterRune(c) {
			letters++
		}
	}
	w := strings.Fields(p)
	return letters >= 4 && (len(w) >= 2 || len(p) >= 10)
}

func isLetterRune(c rune) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c > 0x7f }

// Layers splits what an entry says into the messages to search: the
// message, and each layer of a wrapped error ("a: b: c", as Go's
// fmt.Errorf("a: %w", err) writes them). A layer with nothing
// distinctive is left out.
func Layers(q LineQuery) []string {
	var out []string
	seen := map[string]bool{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" && !seen[s] && len(Phrases(s)) > 0 {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, text := range []string{q.Message, q.Error} {
		chain := strings.Split(text, ": ")
		// One word before the colon is a label ("error: …", "ValueError:
		// …"), not a layer of its own.
		if len(chain) > 1 && len(strings.Fields(chain[0])) < 2 {
			chain = append([]string{chain[0] + ": " + chain[1]}, chain[2:]...)
		}
		if len(chain) == 1 {
			add(text)
			continue
		}
		for _, c := range chain {
			add(c)
		}
	}
	return out
}

// maxPhrases bounds the phrases searched per layer.
const maxPhrases = 6

// FindLine searches files for where each layer of an entry was written.
// All phrases go into one search, so the folder is read once; then the
// files with hits are read again to score each hit by what is around
// it.
func FindLine(ctx context.Context, q LineQuery, files []string, read func(string) ([]byte, error)) ([]Layer, bool, error) {
	var layers []Layer
	var alts []string
	for _, text := range Layers(q) {
		ps := Phrases(text)
		ps = ps[:min(len(ps), maxPhrases)]
		l := Layer{Text: text, Phrases: ps, Hits: []Hit{}}
		for _, p := range ps {
			alts = append(alts, regexp.QuoteMeta(p))
			l.res = append(l.res, regexp.MustCompile(`(?i)`+regexp.QuoteMeta(p)))
		}
		layers = append(layers, l)
	}
	if len(alts) == 0 {
		return layers, false, nil
	}
	sr, err := search.Compile(search.Options{Query: strings.Join(alts, "|"), Regex: true, Globs: Excludes})
	if err != nil {
		return nil, false, err
	}
	// A search stopped by its deadline or the client still has what it
	// found so far; that is scored, and the error returned with it.
	res, err := sr.Run(ctx, files, read)
	if err != nil && ctx.Err() == nil {
		return nil, false, err
	}

	fields := map[string]bool{}
	for _, f := range q.Fields {
		// The last part of a dotted key is what the code writes.
		f = f[strings.LastIndexByte(f, '.')+1:]
		if len(f) >= 3 && !coreFields[f] {
			fields[strings.ToLower(f)] = true
		}
	}
	cache := map[string][]string{}
	linesOf := func(p string) []string {
		if l, ok := cache[p]; ok {
			return l
		}
		data, err := read(p)
		var l []string
		if err == nil {
			l = strings.Split(string(bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))), "\n")
		}
		cache[p] = l
		return l
	}
	for i := range layers {
		l := &layers[i]
		for _, m := range res.Matches {
			if h, ok := score(l, m, linesOf(m.Path), fields); ok {
				l.Hits = append(l.Hits, h)
			}
		}
		slices.SortStableFunc(l.Hits, func(a, b Hit) int {
			return cmp.Or(b.Score-a.Score, cmp.Compare(a.Path, b.Path), a.Line-b.Line)
		})
		l.Hits = l.Hits[:min(len(l.Hits), MaxHits)]
	}
	return layers, res.Truncated, err
}

// coreFields are written by the logger, not by the call: they say
// nothing about where it is.
var coreFields = map[string]bool{
	"time": true, "timestamp": true, "ts": true, "level": true, "lvl": true, "severity": true, "msg": true,
	"message": true, "caller": true, "logger": true, "error": true, "err": true, "stack": true, "trace": true,
	"span": true, "service": true, "host": true, "pid": true, "name": true, "version": true,
}

var (
	logCall     = regexp.MustCompile(`(?i)\b(?:log|logger|logging|slog|klog|zap|logrus|zerolog|log4j|logback|console|fmt\.errorf|errors\.new|errors\.wrap\w*|write-(?:host|error|warning|output)|echo|printf?|println|sprintf|panic|raise|throw|new \w*(?:error|exception))\b|\.(?:debug|info|warn|warning|error|fatal|critical|trace|exception|errorf|infof|warnf|debugf)\s*\(`)
	placeholder = regexp.MustCompile(`%[-+ #0-9.*]*[sdvqfxXegtTpwbcoiu]|\{\d*\}|\{[A-Za-z_][\w.]*(?::[^}]*)?\}|\$\{[^}]+\}|\$[A-Za-z_]\w*|["'` + "`" + `]\s*\+|\+\s*["'` + "`" + `]|\{\{[^}]*\}\}`)
	testPath    = regexp.MustCompile(`(?i)(?:^|/)(?:tests?|__tests__|spec|testdata|fixtures?|e2e)/|_test\.\w+$|\.(?:test|spec)\.\w+$|(?:^|/)test_[^/]*$|Tests?\.\w+$`)
	docPath     = regexp.MustCompile(`(?i)(?:^|/)(?:docs?|examples?)/|\.(?:md|mdx|rst|txt|adoc|html?)$`)
)

// score rates one search hit for a layer; ok is false when none of the
// layer's phrases is on the line.
func score(l *Layer, m search.Match, lines []string, fields map[string]bool) (Hit, bool) {
	if m.Line-1 >= len(lines) {
		return Hit{}, false
	}
	line := lines[m.Line-1]
	best, at := "", -1
	for i, p := range l.Phrases {
		if loc := l.res[i].FindStringIndex(line); loc != nil && len(p) > len(best) {
			best, at = p, loc[0]
		}
	}
	if best == "" {
		return Hit{}, false
	}
	h := Hit{Match: m}
	// How much of the message the phrase is.
	fixed := 0
	for _, part := range slotRe.Split(l.Text, -1) {
		fixed += len(strings.Trim(part, edges))
	}
	h.Score = 40 * len(best) / max(fixed, len(best))

	near := strings.ToLower(strings.Join(lines[max(0, m.Line-3):min(len(lines), m.Line+2)], "\n"))
	others := 0
	for _, p := range l.Phrases {
		pl := strings.ToLower(p)
		if p != best && !strings.Contains(strings.ToLower(best), pl) && strings.Contains(near, pl) {
			others++
		}
	}
	if others > 0 {
		h.Score += min(others, 2) * 15
		h.Why = append(h.Why, "other parts of the message nearby")
	}
	if inString(line, at) {
		h.Score += 15
		h.Why = append(h.Why, "in a string")
	}
	prev := ""
	if m.Line >= 2 {
		prev = lines[m.Line-2]
	}
	if logCall.MatchString(line) || logCall.MatchString(prev) {
		h.Score += 15
		h.Why = append(h.Why, "next to a logging call")
	}
	if placeholder.MatchString(line) {
		h.Score += 5
		h.Why = append(h.Why, "a placeholder on the line")
	}
	keys := 0
	for f := range fields {
		if strings.Contains(near, f) {
			keys++
		}
	}
	if keys > 0 {
		h.Score += min(keys, 3) * 5
		h.Why = append(h.Why, "the entry's field names nearby")
	}
	switch {
	case testPath.MatchString(m.Path):
		h.Score -= 25
		h.Why = append(h.Why, "a test")
	case docPath.MatchString(m.Path):
		h.Score -= 20
		h.Why = append(h.Why, "documentation")
	}
	switch {
	case h.Score >= 70:
		h.Quality = Exact
	case h.Score >= 45:
		h.Quality = Strong
	default:
		h.Quality = Possible
	}
	if h.Why == nil {
		h.Why = []string{}
	}
	return h, true
}

// inString reports whether byte offset at of line is inside a quoted
// string: an odd number of unescaped quotes of one kind before it.
func inString(line string, at int) bool {
	for _, q := range []byte{'"', '\'', '`'} {
		n := 0
		for i := 0; i < at; i++ {
			if line[i] == q && (i == 0 || line[i-1] != '\\') {
				n++
			}
		}
		if n%2 == 1 {
			return true
		}
	}
	return false
}
