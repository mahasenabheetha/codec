package argo

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// Dep is a parsed DAG `depends` expression, e.g.
// "A && (B.Failed || C.Skipped)". A term is a task name with an
// optional result; operators combine terms.
type Dep struct {
	Op     string // "&&", "||", "!", or "" for a term
	Args   []*Dep // operands of an operator
	Task   string // term: the task it waits for
	Result string // term: "" = the default (Succeeded, Skipped or Daemoned)
	Offset int    // term: byte offset in the expression
}

// DepResults are the results a term can test.
var DepResults = []string{"Succeeded", "Failed", "Errored", "Skipped", "Omitted", "Daemoned", "AnySucceeded", "AllFailed"}

// DepError is a syntax error at a byte offset of the expression.
type DepError struct {
	Offset int
	Msg    string
}

func (e *DepError) Error() string { return fmt.Sprintf("column %d: %s", e.Offset+1, e.Msg) }

// ParseDepends parses a depends expression. The grammar is Argo's:
// terms joined by && and || (&& binds tighter), ! negates, and
// parentheses group.
func ParseDepends(s string) (*Dep, error) {
	p := &depParser{src: s}
	p.next()
	d, err := p.or()
	if err != nil {
		return nil, err
	}
	if p.tok != "" {
		return nil, &DepError{p.at, fmt.Sprintf("unexpected %q", p.tok)}
	}
	return d, nil
}

// DependenciesDep turns a `dependencies` list into the expression it
// means: every task with its default result.
func DependenciesDep(tasks []string) *Dep {
	d := &Dep{Op: "&&"}
	for _, t := range tasks {
		d.Args = append(d.Args, &Dep{Task: t})
	}
	return d
}

// Terms returns every term, in order. Negated says whether the term is
// under a "!" (it then means "unless").
func (d *Dep) Terms() []DepTerm {
	var out []DepTerm
	var walk func(*Dep, bool)
	walk = func(d *Dep, neg bool) {
		if d == nil {
			return
		}
		if d.Op == "" {
			out = append(out, DepTerm{Task: d.Task, Result: d.Result, Negated: neg, Offset: d.Offset})
			return
		}
		for _, a := range d.Args {
			walk(a, neg != (d.Op == "!"))
		}
	}
	walk(d, false)
	return out
}

// DepTerm is one task a depends expression mentions.
type DepTerm struct {
	Task    string `json:"task"`
	Result  string `json:"result,omitempty"`
	Negated bool   `json:"negated,omitempty"`
	Offset  int    `json:"-"`
}

// Label is how an edge for the term is captioned: nothing for the
// default result, else the result ("Failed", "not Succeeded").
func (t DepTerm) Label() string {
	l := t.Result
	if t.Negated {
		l = "not " + cmp.Or(l, "Succeeded")
	}
	return l
}

// String prints the expression back with minimal parentheses.
func (d *Dep) String() string {
	switch d.Op {
	case "":
		if d.Result != "" {
			return d.Task + "." + d.Result
		}
		return d.Task
	case "!":
		s := d.Args[0].String()
		if d.Args[0].Op != "" && d.Args[0].Op != "!" {
			s = "(" + s + ")"
		}
		return "!" + s
	}
	parts := make([]string, len(d.Args))
	for i, a := range d.Args {
		parts[i] = a.String()
		if d.Op == "&&" && a.Op == "||" {
			parts[i] = "(" + parts[i] + ")"
		}
	}
	return strings.Join(parts, " "+d.Op+" ")
}

// Explain says in words when the task runs.
func (d *Dep) Explain() string {
	switch d.Op {
	case "":
		switch d.Result {
		case "":
			return d.Task + " succeeded (or was skipped)"
		case "AnySucceeded":
			return "any iteration of " + d.Task + " succeeded"
		case "AllFailed":
			return "every iteration of " + d.Task + " failed"
		}
		return d.Task + " " + strings.ToLower(d.Result)
	case "!":
		return "not (" + d.Args[0].Explain() + ")"
	}
	parts := make([]string, len(d.Args))
	for i, a := range d.Args {
		parts[i] = a.Explain()
		if a.Op != "" && a.Op != "!" && a.Op != d.Op {
			parts[i] = "(" + parts[i] + ")"
		}
	}
	word := " and "
	if d.Op == "||" {
		word = " or "
	}
	return strings.Join(parts, word)
}

type depParser struct {
	src string
	pos int    // next unread byte
	tok string // current token; "" at the end
	at  int    // offset of tok
}

func (p *depParser) next() {
	for p.pos < len(p.src) && (p.src[p.pos] == ' ' || p.src[p.pos] == '\t' || p.src[p.pos] == '\n' || p.src[p.pos] == '\r') {
		p.pos++
	}
	p.at = p.pos
	if p.pos >= len(p.src) {
		p.tok = ""
		return
	}
	switch c := p.src[p.pos]; {
	case c == '(' || c == ')' || c == '!':
		p.tok = p.src[p.pos : p.pos+1]
		p.pos++
	case strings.HasPrefix(p.src[p.pos:], "&&") || strings.HasPrefix(p.src[p.pos:], "||"):
		p.tok = p.src[p.pos : p.pos+2]
		p.pos += 2
	default:
		end := p.pos
		for end < len(p.src) && isNameByte(p.src[end]) {
			end++
		}
		if end == p.pos {
			end++ // one stray byte, reported by the caller
		}
		p.tok = p.src[p.pos:end]
		p.pos = end
	}
}

func isNameByte(c byte) bool {
	return c == '-' || c == '_' || c == '.' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func (p *depParser) or() (*Dep, error) {
	return p.binary("||", p.and)
}

func (p *depParser) and() (*Dep, error) {
	return p.binary("&&", p.unary)
}

func (p *depParser) binary(op string, operand func() (*Dep, error)) (*Dep, error) {
	first, err := operand()
	if err != nil {
		return nil, err
	}
	d := &Dep{Op: op, Args: []*Dep{first}}
	for p.tok == op {
		p.next()
		next, err := operand()
		if err != nil {
			return nil, err
		}
		d.Args = append(d.Args, next)
	}
	if len(d.Args) == 1 {
		return first, nil
	}
	return d, nil
}

func (p *depParser) unary() (*Dep, error) {
	switch p.tok {
	case "!":
		p.next()
		d, err := p.unary()
		if err != nil {
			return nil, err
		}
		return &Dep{Op: "!", Args: []*Dep{d}}, nil
	case "(":
		open := p.at
		p.next()
		d, err := p.or()
		if err != nil {
			return nil, err
		}
		if p.tok != ")" {
			return nil, &DepError{open, "unclosed parenthesis"}
		}
		p.next()
		return d, nil
	case "":
		return nil, &DepError{p.at, "expected a task name"}
	}
	if !isNameByte(p.tok[0]) {
		return nil, &DepError{p.at, fmt.Sprintf("unexpected %q", p.tok)}
	}
	d := &Dep{Offset: p.at}
	d.Task, d.Result, _ = strings.Cut(p.tok, ".")
	if d.Task == "" {
		return nil, &DepError{p.at, "expected a task name before the dot"}
	}
	if _, has := strings.CutPrefix(p.tok, d.Task+"."); has && !slices.Contains(DepResults, d.Result) {
		return nil, &DepError{p.at + len(d.Task) + 1, fmt.Sprintf("%q isn't a task result (use one of %s)", d.Result, strings.Join(DepResults, ", "))}
	}
	p.next()
	return d, nil
}
