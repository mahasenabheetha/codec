package regex

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Node is one part of a pattern, explained. Start and End are UTF-16
// offsets in the pattern, so hovering a part can highlight it.
type Node struct {
	Kind     string  `json:"kind"` // literal, class, escape, anchor, group, alternation, quantifier, backref, flags, dot
	Start    int     `json:"start"`
	End      int     `json:"end"`
	Text     string  `json:"text"`
	Desc     string  `json:"desc"`
	Group    int     `json:"group,omitempty"` // a capture group's number
	Children []*Node `json:"children,omitempty"`
}

// Explain describes a pattern part by part. It reads both styles; parts
// one style doesn't support are still explained, and Run says so. A
// pattern that can't be read explains as far as it goes.
func Explain(pattern, style, flags string) []*Node {
	p := &parser{src: []rune(pattern), style: style, flags: flags}
	p.u16 = make([]int, len(p.src)+1)
	for i, r := range p.src {
		p.u16[i+1] = p.u16[i] + len(utf16.Encode([]rune{r}))
	}
	// Number capture groups as the engine will: in order in RE2, and in
	// .NET style unnamed ones first, then named ones.
	p.numbers = groupNumbers(p.src, style)
	nodes := p.alternation(0)
	return nodes
}

type parser struct {
	src     []rune
	u16     []int
	i       int
	style   string
	flags   string
	numbers map[int]int // '(' position → group number
}

func (p *parser) node(kind string, from int, desc string, children ...*Node) *Node {
	return &Node{Kind: kind, Start: p.u16[from], End: p.u16[p.i], Text: string(p.src[from:p.i]), Desc: desc, Children: children}
}

func (p *parser) more() bool { return p.i < len(p.src) }

// skip steps over a closing character when there is one: an unfinished
// pattern ("(?<", "k<x") ends without it.
func (p *parser) skip() {
	if p.more() {
		p.i++
	}
}
func (p *parser) peek(k int) rune {
	if p.i+k < len(p.src) {
		return p.src[p.i+k]
	}
	return 0
}

// alternation reads a|b|c up to a closing parenthesis.
func (p *parser) alternation(depth int) []*Node {
	from := p.i
	var branches [][]*Node
	var bounds [][2]int
	for {
		bs := p.i
		seq := p.sequence(depth)
		branches = append(branches, seq)
		bounds = append(bounds, [2]int{bs, p.i})
		if p.more() && p.peek(0) == '|' {
			p.i++
			continue
		}
		break
	}
	if len(branches) == 1 {
		return branches[0]
	}
	alt := &Node{Kind: "alternation", Start: p.u16[from], End: p.u16[p.i], Text: string(p.src[from:p.i]), Desc: fmt.Sprintf("Either of %d alternatives", len(branches))}
	for k, b := range branches {
		desc := "or"
		if k == 0 {
			desc = "either"
		}
		alt.Children = append(alt.Children, &Node{Kind: "branch", Start: p.u16[bounds[k][0]], End: p.u16[bounds[k][1]], Text: string(p.src[bounds[k][0]:bounds[k][1]]), Desc: desc, Children: b})
	}
	return []*Node{alt}
}

// sequence reads atoms with their quantifiers; runs of plain characters
// become one literal.
func (p *parser) sequence(depth int) []*Node {
	var out []*Node
	// A run of literals is joined once, when it ends: joining on every
	// character would be quadratic on a long literal.
	runFrom, runTo := -1, -1
	endRun := func() {
		if runFrom < 0 {
			return
		}
		last := out[len(out)-1]
		last.Text = string(p.src[runFrom:runTo])
		last.End = p.u16[runTo]
		last.Desc = fmt.Sprintf("The text %q", literalValue(last.Text))
		runFrom = -1
	}
	defer endRun()
	for p.more() {
		c := p.peek(0)
		if c == '|' || (c == ')' && depth > 0) {
			break
		}
		if c == ')' { // unbalanced
			endRun()
			from := p.i
			p.i++
			out = append(out, p.node("error", from, "A ) without a matching ("))
			continue
		}
		atom := p.atom(depth)
		if atom == nil {
			continue
		}
		atom = p.quantified(atom)
		// Join literal characters into one literal ("abc"), unless the
		// next one is quantified (ab+ is a, then b repeated).
		if atom.Kind == "literal" && len(out) > 0 && out[len(out)-1].Kind == "literal" {
			if runFrom < 0 {
				runFrom = p.runeAt(out[len(out)-1].Start)
			}
			runTo = p.i
			continue
		}
		endRun()
		out = append(out, atom)
	}
	return out
}

// runeAt is the rune index of a UTF-16 offset in the pattern.
func (p *parser) runeAt(u int) int {
	lo, hi := 0, len(p.u16)-1
	for lo < hi {
		mid := (lo + hi) / 2
		if p.u16[mid] < u {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

func literalValue(t string) string {
	var b strings.Builder
	for i := 0; i < len(t); i++ {
		if t[i] == '\\' && i+1 < len(t) {
			i++
		}
		b.WriteByte(t[i])
	}
	return b.String()
}

func (p *parser) atom(depth int) *Node {
	from := p.i
	c := p.peek(0)
	switch c {
	case '(':
		return p.groupNode(depth)
	case '[':
		return p.class()
	case '.':
		p.i++
		if strings.ContainsRune(p.flags, 's') {
			return p.node("dot", from, "Any character")
		}
		return p.node("dot", from, "Any character except a line break")
	case '^':
		p.i++
		if strings.ContainsRune(p.flags, 'm') {
			return p.node("anchor", from, "Start of a line")
		}
		return p.node("anchor", from, "Start of the text")
	case '$':
		p.i++
		if strings.ContainsRune(p.flags, 'm') {
			return p.node("anchor", from, "End of a line")
		}
		return p.node("anchor", from, "End of the text")
	case '\\':
		return p.escape(false)
	case '*', '+', '?', '{':
		if c == '{' && !p.isCount() {
			p.i++
			return p.node("literal", from, `The text "{"`)
		}
		p.i++
		return p.node("error", from, "A repeat with nothing before it to repeat")
	}
	if strings.ContainsRune(p.flags, 'x') && (c == ' ' || c == '\t' || c == '\n' || c == '#') {
		if c == '#' {
			for p.more() && p.peek(0) != '\n' {
				p.i++
			}
			return p.node("comment", from, "A comment")
		}
		p.i++
		return nil // ignored whitespace
	}
	p.i++
	return p.node("literal", from, fmt.Sprintf("The text %q", string(c)))
}

// isCount reports whether a "{" starts a repeat count: {n}, {n,}, {n,m}.
func (p *parser) isCount() bool {
	j := p.i + 1
	digits := 0
	for j < len(p.src) && p.src[j] >= '0' && p.src[j] <= '9' {
		j, digits = j+1, digits+1
	}
	if digits == 0 {
		return false
	}
	if j < len(p.src) && p.src[j] == ',' {
		j++
		for j < len(p.src) && p.src[j] >= '0' && p.src[j] <= '9' {
			j++
		}
	}
	return j < len(p.src) && p.src[j] == '}'
}

// quantified wraps atom in the repeat that follows it, if any.
func (p *parser) quantified(atom *Node) *Node {
	if !p.more() {
		return atom
	}
	from := p.i
	var desc string
	switch c := p.peek(0); {
	case c == '*':
		p.i++
		desc = "zero or more times"
	case c == '+':
		p.i++
		desc = "one or more times"
	case c == '?':
		p.i++
		desc = "optional (zero or one time)"
	case c == '{' && p.isCount():
		p.i++
		start := p.i
		for p.peek(0) != '}' {
			p.i++
		}
		spec := string(p.src[start:p.i])
		p.i++
		lo, hi, comma := strings.Cut(spec, ",")
		switch {
		case !comma:
			desc = fmt.Sprintf("exactly %s %s", lo, times(lo))
		case hi == "":
			desc = fmt.Sprintf("%s or more times", lo)
		default:
			desc = fmt.Sprintf("%s to %s times", lo, hi)
		}
	default:
		return atom
	}
	// No possessive repeats (a++): neither RE2 nor regexp2 (.NET) has them.
	mode := "as many as possible"
	if p.peek(0) == '?' {
		p.i++
		mode = "as few as possible (lazy)"
	}
	q := &Node{Kind: "quantifier", Start: atom.Start, End: p.u16[p.i], Text: atom.Text + string(p.src[from:p.i]), Desc: capitalize(desc) + ", " + mode, Children: []*Node{atom}}
	return q
}

func times(n string) string {
	if n == "1" {
		return "time"
	}
	return "times"
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func (p *parser) groupNode(depth int) *Node {
	from := p.i
	p.i++ // (
	var desc string
	capture := false
	name := ""
	if p.peek(0) == '?' {
		rest := string(p.src[p.i:min(p.i+4, len(p.src))])
		switch {
		case strings.HasPrefix(rest, "?:"):
			p.i += 2
			desc = "Group (not captured)"
		case strings.HasPrefix(rest, "?="):
			p.i += 2
			desc = "Lookahead: followed by this, which isn't part of the match"
		case strings.HasPrefix(rest, "?!"):
			p.i += 2
			desc = "Negative lookahead: not followed by this"
		case strings.HasPrefix(rest, "?<="):
			p.i += 3
			desc = "Lookbehind: preceded by this, which isn't part of the match"
		case strings.HasPrefix(rest, "?<!"):
			p.i += 3
			desc = "Negative lookbehind: not preceded by this"
		case strings.HasPrefix(rest, "?>"):
			p.i += 2
			desc = "Atomic group: once matched, never backtracked into"
		case strings.HasPrefix(rest, "?P<"), strings.HasPrefix(rest, "?<"), strings.HasPrefix(rest, "?'"):
			p.i += 2
			if p.peek(-1) == 'P' {
				p.i++
			}
			end := '>'
			if p.peek(-1) == '\'' {
				end = '\''
			}
			s := p.i
			for p.more() && p.peek(0) != end {
				p.i++
			}
			name = string(p.src[s:p.i])
			p.skip() // the closing character, if there is one
			capture = true
		case strings.HasPrefix(rest, "?P="):
			p.i += 3
			s := p.i
			for p.more() && p.peek(0) != ')' {
				p.i++
			}
			name := string(p.src[s:p.i])
			p.skip()
			return p.node("backref", from, fmt.Sprintf("The same text group %q matched", name))
		case strings.HasPrefix(rest, "?#"):
			for p.more() && p.peek(0) != ')' {
				p.i++
			}
			p.skip()
			return p.node("comment", from, "A comment")
		default:
			// Inline flags: (?i) for the rest, (?i:…) for a group.
			p.i++
			s := p.i
			for p.more() && p.peek(0) != ')' && p.peek(0) != ':' {
				p.i++
			}
			fl := string(p.src[s:p.i])
			if p.more() && p.peek(0) == ')' {
				p.i++
				return p.node("flags", from, "From here on: "+flagWords(fl))
			}
			p.skip()
			desc = "Group (not captured) with " + flagWords(fl)
		}
	} else {
		capture = true
	}
	if capture {
		n := p.numbers[from]
		if name != "" {
			desc = fmt.Sprintf("Group %d, named %q: captured", n, name)
		} else {
			desc = fmt.Sprintf("Group %d: captured", n)
		}
	}
	children := p.alternation(depth + 1)
	if !p.more() {
		return p.node("error", from, "A ( without a matching )", children...)
	}
	p.i++ // )
	g := p.node("group", from, desc, children...)
	if capture {
		g.Group = p.numbers[from]
	}
	return g
}

func flagWords(fl string) string {
	on, off, _ := strings.Cut(fl, "-")
	words := map[rune]string{'i': "ignore case", 'm': "^ and $ at line breaks", 's': ". matches line breaks", 'x': "ignore whitespace", 'U': "lazy by default", 'n': "only named groups capture"}
	var parts []string
	for _, r := range on {
		parts = append(parts, words[r])
	}
	for _, r := range off {
		parts = append(parts, "not "+words[r])
	}
	return strings.Join(parts, ", ")
}

// groupNumbers numbers capture groups by the position of their "(".
func groupNumbers(src []rune, style string) map[int]int {
	type g struct {
		pos   int
		named bool
	}
	var gs []g
	inClass := false
	for i := 0; i < len(src); i++ {
		switch c := src[i]; {
		case c == '\\' && i+1 < len(src) && src[i+1] == 'Q' && style != PCRE:
			// \Q…\E is literal text: parentheses in it open no group.
			for i += 2; i < len(src) && !(src[i] == '\\' && i+1 < len(src) && src[i+1] == 'E'); i++ {
			}
			i++
		case c == '\\':
			i++
		case inClass:
			if c == ']' {
				inClass = false
			}
		case c == '[':
			inClass = true
			// A ] first in a class ([]a], [^]a]) is a literal, not its end.
			if i+1 < len(src) && src[i+1] == '^' {
				i++
			}
			if i+1 < len(src) && src[i+1] == ']' {
				i++
			}
		case c == '(':
			rest := string(src[i+1 : min(i+5, len(src))])
			switch {
			case !strings.HasPrefix(rest, "?"):
				gs = append(gs, g{i, false})
			case strings.HasPrefix(rest, "?P<"), strings.HasPrefix(rest, "?'"), strings.HasPrefix(rest, "?<") && !strings.HasPrefix(rest, "?<=") && !strings.HasPrefix(rest, "?<!"):
				gs = append(gs, g{i, true})
			}
		}
	}
	out := map[int]int{}
	n := 0
	for _, x := range gs {
		if style != PCRE || !x.named {
			n++
			out[x.pos] = n
		}
	}
	if style == PCRE { // .NET: named groups after the unnamed ones
		for _, x := range gs {
			if x.named {
				n++
				out[x.pos] = n
			}
		}
	}
	return out
}

// class reads [...]: "one of: a to z, 0 to 9, _".
func (p *parser) class() *Node {
	from := p.i
	p.i++ // [
	negate := false
	if p.peek(0) == '^' {
		negate = true
		p.i++
	}
	var items []string
	first := true
	for p.more() && (p.peek(0) != ']' || first) {
		first = false
		var item string
		if p.peek(0) == '\\' {
			item = p.escape(true).Desc
		} else if p.peek(0) == '[' && p.peek(1) == ':' {
			s := p.i
			for p.more() && !(p.peek(0) == ']' && p.peek(-1) == ':') {
				p.i++
			}
			p.skip()
			item = "POSIX class " + string(p.src[s:p.i])
		} else {
			item = quoteChar(p.src[p.i])
			p.i++
		}
		if p.peek(0) == '-' && p.peek(1) != ']' && p.peek(1) != 0 {
			p.i++
			var hi string
			if p.peek(0) == '\\' {
				hi = p.escape(true).Desc
			} else {
				hi = quoteChar(p.src[p.i])
				p.i++
			}
			item = item + " to " + hi
		}
		items = append(items, item)
	}
	if !p.more() {
		return p.node("error", from, "A [ without a matching ]")
	}
	p.i++ // ]
	if negate {
		return p.node("class", from, "Any character except "+strings.Join(items, ", "))
	}
	if len(items) == 1 {
		return p.node("class", from, "One character: "+items[0])
	}
	return p.node("class", from, "One of: "+strings.Join(items, ", "))
}

func quoteChar(r rune) string {
	switch r {
	case ' ':
		return "space"
	case '\t':
		return "tab"
	}
	return strconv.QuoteRune(r)
}

var escapes = map[rune]string{
	'd': "a digit", 'D': "a character that isn't a digit",
	'w': "a word character (letter, digit or _)", 'W': "a character that isn't a word character",
	's': "whitespace (space, tab, line break)", 'S': "a character that isn't whitespace",
	'n': "a line break (\\n)", 'r': "a carriage return (\\r)", 't': "a tab", 'f': "a form feed", 'v': "a vertical tab", '0': "a NUL character",
}

var anchors = map[rune]string{
	'b': "A word boundary", 'B': "Not a word boundary",
	'A': "Start of the text", 'z': "End of the text", 'Z': "End of the text, or before a final line break", 'G': "Where the previous match ended",
}

// escape reads a backslash sequence.
func (p *parser) escape(inClass bool) *Node {
	from := p.i
	p.i++ // \
	if !p.more() {
		return p.node("error", from, "A \\ at the end of the pattern")
	}
	c := p.peek(0)
	p.i++
	if d, ok := escapes[c]; ok {
		if inClass {
			return p.node("escape", from, d)
		}
		return p.node("escape", from, capitalize(d))
	}
	if d, ok := anchors[c]; ok && !inClass {
		return p.node("anchor", from, d)
	}
	switch {
	case c >= '1' && c <= '9' && !inClass:
		for p.more() && p.peek(0) >= '0' && p.peek(0) <= '9' {
			p.i++
		}
		return p.node("backref", from, "The same text group "+string(p.src[from+1:p.i])+" matched")
	case c == 'k' && (p.peek(0) == '<' || p.peek(0) == '{' || p.peek(0) == '\''):
		closer := map[rune]rune{'<': '>', '{': '}', '\'': '\''}[p.peek(0)]
		p.i++ // the opener; for \k'name' it is also the closer
		s := p.i
		for p.more() && p.peek(0) != closer {
			p.i++
		}
		name := string(p.src[s:p.i])
		p.skip()
		return p.node("backref", from, fmt.Sprintf("The same text group %q matched", name))
	case c == 'x':
		if p.peek(0) == '{' {
			for p.more() && p.peek(0) != '}' {
				p.i++
			}
			p.skip()
		} else {
			p.i = min(p.i+2, len(p.src))
		}
		return p.node("escape", from, "The character with code "+string(p.src[from+2:p.i]))
	case c == 'u':
		p.i = min(p.i+4, len(p.src))
		return p.node("escape", from, "The character U+"+strings.ToUpper(string(p.src[from+2:p.i])))
	case c == 'p' || c == 'P':
		name := ""
		if p.peek(0) == '{' {
			s := p.i + 1
			for p.more() && p.peek(0) != '}' {
				p.i++
			}
			name = string(p.src[s:p.i])
			p.skip()
		} else if p.more() {
			name = string(p.peek(0))
			p.i++
		}
		if c == 'P' {
			return p.node("escape", from, "A character not in the Unicode class "+name)
		}
		return p.node("escape", from, "A character in the Unicode class "+name)
	case c == 'Q' && p.style != PCRE:
		s := p.i
		for p.more() && !(p.peek(0) == '\\' && p.peek(1) == 'E') {
			p.i++
		}
		lit := string(p.src[s:p.i])
		p.i = min(p.i+2, len(p.src))
		return p.node("quote", from, fmt.Sprintf("The text %q, taken literally", lit)) // not joined with literals around it
	}
	if inClass {
		return p.node("literal", from, quoteChar(c))
	}
	return p.node("literal", from, fmt.Sprintf("The text %q", string(c)))
}
