// Package regex tests regular expressions in two styles — Go's RE2
// (the standard library) and the Python / .NET / JavaScript style
// (dlclark/regexp2, with lookarounds and backreferences) — replaces with
// them, and explains a pattern part by part.
//
// Positions are UTF-16 code units, as editors in the browser count.
package regex

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
)

// Styles.
const (
	Go   = "go"   // RE2: linear time, no lookarounds or backreferences
	PCRE = "pcre" // Python, .NET, JavaScript: regexp2, backtracking
)

// Limits: a test stops at MaxMatches or after Timeout, and says so.
const (
	MaxMatches = 1000
	Timeout    = 2 * time.Second
)

// Options are a pattern and how to run it.
type Options struct {
	Pattern string `json:"pattern"`
	Style   string `json:"style"` // go (default) or pcre
	// Flags: i ignore case, m ^ and $ at line breaks, s . matches line
	// breaks, x ignore whitespace and # comments (pcre only).
	Flags string `json:"flags"`
	All   bool   `json:"all"` // every match, not only the first
}

// Group is one capture group of a match. Start is -1 when the group
// took no part in the match.
type Group struct {
	Number int    `json:"number"`
	Name   string `json:"name,omitempty"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
	Text   string `json:"text"`
}

// Match is one match with its groups.
type Match struct {
	Start  int     `json:"start"`
	End    int     `json:"end"`
	Line   int     `json:"line"` // 1-based, of Start
	Text   string  `json:"text"`
	Groups []Group `json:"groups"`
}

// Result is a test's outcome.
type Result struct {
	Matches   []Match  `json:"matches"`
	Groups    []string `json:"groups"` // names by number; "" for unnamed, [0] is the whole match
	Truncated bool     `json:"truncated"`
	TimedOut  bool     `json:"timedOut"`
}

// Error is a pattern that doesn't compile, with a hint when the other
// style would take it.
type Error struct {
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

func (e *Error) Error() string { return e.Message }

// compiled is a pattern ready in either engine.
type compiled struct {
	re2  *regexp.Regexp
	re   *regexp2.Regexp
	opts Options
}

func compile(o Options) (*compiled, error) {
	if o.Pattern == "" {
		return nil, &Error{Message: "empty pattern"}
	}
	for _, f := range o.Flags {
		if !strings.ContainsRune("imsx", f) {
			return nil, &Error{Message: fmt.Sprintf("unknown flag %q (i, m, s, x)", f)}
		}
	}
	switch o.Style {
	case "", Go:
		if strings.ContainsRune(o.Flags, 'x') {
			return nil, &Error{Message: "Go's RE2 has no x (extended) flag", Hint: "Switch to the Python / .NET / JavaScript style, or remove the whitespace."}
		}
		p := o.Pattern
		if o.Flags != "" {
			p = "(?" + o.Flags + ")" + p
		}
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, re2Error(err)
		}
		return &compiled{re2: re, opts: o}, nil
	case PCRE:
		var opt regexp2.RegexOptions
		for _, f := range o.Flags {
			opt |= map[rune]regexp2.RegexOptions{'i': regexp2.IgnoreCase, 'm': regexp2.Multiline, 's': regexp2.Singleline, 'x': regexp2.IgnorePatternWhitespace}[f]
		}
		re, err := regexp2.Compile(pythonNames(o.Pattern), opt)
		if err != nil {
			return nil, &Error{Message: strings.TrimPrefix(err.Error(), "error parsing regexp: ")}
		}
		re.MatchTimeout = Timeout
		return &compiled{re: re, opts: o}, nil
	}
	return nil, &Error{Message: fmt.Sprintf("unknown style %q (go, pcre)", o.Style)}
}

// re2Error explains what RE2 refuses that the other style takes.
func re2Error(err error) error {
	msg := strings.TrimPrefix(err.Error(), "error parsing regexp: ")
	e := &Error{Message: msg}
	switch {
	case strings.Contains(msg, "`(?=") || strings.Contains(msg, "`(?!") || strings.Contains(msg, "`(?<=") || strings.Contains(msg, "`(?<!"):
		e.Hint = "Go's RE2 has no lookahead or lookbehind (it guarantees linear time). Switch to the Python / .NET / JavaScript style."
	case strings.Contains(msg, "invalid escape sequence: `\\") && len(msg) > 0 && strings.ContainsAny(msg[len(msg)-2:], "123456789"):
		e.Hint = "Go's RE2 has no backreferences. Switch to the Python / .NET / JavaScript style."
	case strings.Contains(msg, "`(?>") || strings.Contains(msg, "`(?P="):
		e.Hint = "Go's RE2 has no atomic groups or backreferences. Switch to the Python / .NET / JavaScript style."
	}
	return e
}

// pythonNames rewrites Python's (?P<name>…) and (?P=name) for regexp2.
func pythonNames(p string) string {
	if !strings.Contains(p, "(?P") {
		return p
	}
	p = strings.ReplaceAll(p, "(?P<", "(?<")
	var b strings.Builder
	for {
		i := strings.Index(p, "(?P=")
		if i < 0 {
			b.WriteString(p)
			return b.String()
		}
		j := strings.IndexByte(p[i:], ')')
		if j < 0 {
			b.WriteString(p)
			return b.String()
		}
		b.WriteString(p[:i] + `\k<` + p[i+4:i+j] + ">")
		p = p[i+j+1:]
	}
}

// Run tests the pattern on text.
func Run(o Options, text string) (*Result, error) {
	c, err := compile(o)
	if err != nil {
		return nil, err
	}
	pos := newPositions(text)
	res := &Result{Matches: []Match{}}
	deadline := time.Now().Add(Timeout)
	limit := MaxMatches
	if !o.All {
		limit = 1
	}
	if c.re2 != nil {
		res.Groups = c.re2.SubexpNames()
		for _, m := range c.re2.FindAllStringSubmatchIndex(text, limit+1) {
			if len(res.Matches) == limit {
				res.Truncated = o.All
				break
			}
			mt := Match{Start: pos.byteU16(m[0]), End: pos.byteU16(m[1]), Line: pos.line(m[0]), Text: text[m[0]:m[1]]}
			for g := 1; g < len(m)/2; g++ {
				gr := Group{Number: g, Name: res.Groups[g], Start: -1, End: -1}
				if m[2*g] >= 0 {
					gr.Start, gr.End, gr.Text = pos.byteU16(m[2*g]), pos.byteU16(m[2*g+1]), text[m[2*g]:m[2*g+1]]
				}
				mt.Groups = append(mt.Groups, gr)
			}
			res.Matches = append(res.Matches, mt)
		}
		return res, nil
	}
	nums := c.re.GetGroupNumbers()
	res.Groups = make([]string, len(nums))
	for i, n := range nums {
		if name := c.re.GroupNameFromNumber(n); name != fmt.Sprint(n) {
			res.Groups[i] = name
		}
	}
	m, err := c.re.FindStringMatch(text)
	for ; m != nil && err == nil; m, err = c.re.FindNextMatch(m) {
		if len(res.Matches) == limit {
			res.Truncated = o.All
			break
		}
		if time.Now().After(deadline) {
			res.TimedOut = true
			break
		}
		mt := Match{Start: pos.runeU16(m.Index), End: pos.runeU16(m.Index + m.Length), Line: pos.lineOfRune(m.Index), Text: m.String()}
		for i, g := range m.Groups()[1:] {
			gr := Group{Number: nums[i+1], Name: res.Groups[i+1], Start: -1, End: -1}
			if len(g.Captures) > 0 {
				gr.Start, gr.End, gr.Text = pos.runeU16(g.Index), pos.runeU16(g.Index+g.Length), g.String()
			}
			mt.Groups = append(mt.Groups, gr)
		}
		res.Matches = append(res.Matches, mt)
	}
	if err != nil {
		if isTimeout(err) {
			res.TimedOut = true
			return res, nil
		}
		return nil, &Error{Message: err.Error()}
	}
	return res, nil
}

func isTimeout(err error) bool {
	return err != nil && strings.Contains(err.Error(), "timeout")
}

// ErrTimeout is returned by Replace when the pattern backtracks too long.
var ErrTimeout = errors.New("the pattern took longer than 2 seconds; it probably backtracks badly (nested quantifiers such as (a+)+)")

// Replace replaces matches (only the first unless o.All) with template:
// $1, ${1}, ${name} and $0 everywhere; Python's \1 and \g<name> and
// JavaScript's $<name> are accepted too. $$ is a dollar sign.
func Replace(o Options, text, template string) (string, error) {
	c, err := compile(o)
	if err != nil {
		return "", err
	}
	t := normalizeTemplate(template)
	if c.re2 != nil {
		if o.All {
			return c.re2.ReplaceAllString(text, t), nil
		}
		loc := c.re2.FindStringSubmatchIndex(text)
		if loc == nil {
			return text, nil
		}
		return text[:loc[0]] + string(c.re2.ExpandString(nil, t, text, loc)) + text[loc[1]:], nil
	}
	count := -1
	if !o.All {
		count = 1
	}
	out, err := c.re.Replace(text, t, -1, count)
	if isTimeout(err) {
		return "", ErrTimeout
	}
	return out, err
}

var (
	pyGroup   = regexp.MustCompile(`\\(\d{1,2})`)
	pyNamed   = regexp.MustCompile(`\\g<(\w+)>`)
	jsNamed   = regexp.MustCompile(`\$<(\w+)>`)
	bareDigit = regexp.MustCompile(`\$(\d+)`)
)

func normalizeTemplate(t string) string {
	t = pyNamed.ReplaceAllString(t, `$${$1}`)
	t = jsNamed.ReplaceAllString(t, `$${$1}`)
	t = pyGroup.ReplaceAllString(t, `$${$1}`)
	// "$1x" means group 1 then "x" to people; Go would read a group "1x".
	return bareDigit.ReplaceAllString(t, `$${$1}`)
}

// positions converts byte and rune offsets of a text to UTF-16 offsets
// and lines.
type positions struct {
	text  string
	bytes []int // byte offset → UTF-16 offset (len+1)
	runes []int // rune index → byte offset (len+1)
	lines []int // byte offsets where lines start
}

func newPositions(text string) *positions {
	p := &positions{text: text, bytes: make([]int, len(text)+1), lines: []int{0}}
	u := 0
	for i, r := range text {
		p.runes = append(p.runes, i)
		n := utf8.RuneLen(r)
		for k := range n {
			p.bytes[i+k] = u
		}
		u += len(utf16.Encode([]rune{r}))
		if r == '\n' {
			p.lines = append(p.lines, i+1)
		}
	}
	p.bytes[len(text)] = u
	p.runes = append(p.runes, len(text))
	return p
}

func (p *positions) byteU16(b int) int { return p.bytes[b] }
func (p *positions) runeU16(r int) int { return p.bytes[p.runes[min(r, len(p.runes)-1)]] }
func (p *positions) lineOfRune(r int) int {
	return p.line(p.runes[min(r, len(p.runes)-1)])
}
func (p *positions) line(b int) int {
	lo, hi := 0, len(p.lines)
	for lo < hi {
		mid := (lo + hi) / 2
		if p.lines[mid] <= b {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}
