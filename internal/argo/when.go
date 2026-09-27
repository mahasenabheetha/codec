package argo

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// EvalWhen evaluates a `when` condition whose variables are already
// substituted, e.g. "false == true" or "'prod' =~ '^pr'". It covers
// what conditions are written with in practice (comparisons, regex
// match, && || !, parentheses; strings, numbers, booleans) and reports
// an error for anything else, so the caller can say "can't tell"
// instead of guessing.
func EvalWhen(s string) (bool, error) {
	p := &whenParser{src: s}
	p.next()
	v, err := p.or()
	if err != nil {
		return false, err
	}
	if p.tok.kind != tEnd {
		return false, fmt.Errorf("unexpected %q", p.tok.text)
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("%v is not a condition", v)
	}
	return b, nil
}

type tokKind int

const (
	tEnd tokKind = iota
	tOp
	tStr // quoted
	tWord
)

type whenTok struct {
	kind tokKind
	text string
}

type whenParser struct {
	src string
	pos int
	tok whenTok
}

var whenOps = []string{"&&", "||", "==", "!=", "<=", ">=", "=~", "!~", "<", ">", "!", "(", ")"}

func (p *whenParser) next() {
	for p.pos < len(p.src) && strings.ContainsRune(" \t\r\n", rune(p.src[p.pos])) {
		p.pos++
	}
	if p.pos >= len(p.src) {
		p.tok = whenTok{kind: tEnd}
		return
	}
	rest := p.src[p.pos:]
	for _, op := range whenOps {
		if strings.HasPrefix(rest, op) {
			p.tok = whenTok{tOp, op}
			p.pos += len(op)
			return
		}
	}
	if q := rest[0]; q == '\'' || q == '"' || q == '`' {
		end := strings.IndexByte(rest[1:], q)
		if end < 0 {
			p.tok = whenTok{tWord, rest} // unclosed: fails as a comparison operand
			p.pos = len(p.src)
			return
		}
		p.tok = whenTok{tStr, rest[1 : end+1]}
		p.pos += end + 2
		return
	}
	end := 0
	for end < len(rest) && !strings.ContainsRune(" \t\r\n()!=<>&|~'\"`", rune(rest[end])) {
		end++
	}
	if end == 0 {
		end = 1
	}
	p.tok = whenTok{tWord, rest[:end]}
	p.pos += end
}

func (p *whenParser) or() (any, error) {
	l, err := p.and()
	for err == nil && p.tok.text == "||" && p.tok.kind == tOp {
		p.next()
		var r any
		if r, err = p.and(); err == nil {
			l, err = logic(l, r, "||")
		}
	}
	return l, err
}

func (p *whenParser) and() (any, error) {
	l, err := p.cmp()
	for err == nil && p.tok.text == "&&" && p.tok.kind == tOp {
		p.next()
		var r any
		if r, err = p.cmp(); err == nil {
			l, err = logic(l, r, "&&")
		}
	}
	return l, err
}

func logic(l, r any, op string) (any, error) {
	lb, ok1 := l.(bool)
	rb, ok2 := r.(bool)
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("%s needs conditions on both sides", op)
	}
	if op == "&&" {
		return lb && rb, nil
	}
	return lb || rb, nil
}

func (p *whenParser) cmp() (any, error) {
	l, err := p.unary()
	if err != nil {
		return nil, err
	}
	switch op := p.tok.text; {
	case p.tok.kind != tOp:
		return l, nil
	case op == "==" || op == "!=" || op == "<" || op == ">" || op == "<=" || op == ">=" || op == "=~" || op == "!~":
		p.next()
		r, err := p.unary()
		if err != nil {
			return nil, err
		}
		return compare(l, r, op)
	}
	return l, nil
}

func (p *whenParser) unary() (any, error) {
	t := p.tok
	switch {
	case t.kind == tOp && t.text == "!":
		p.next()
		v, err := p.unary()
		if err != nil {
			return nil, err
		}
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("! needs a condition")
		}
		return !b, nil
	case t.kind == tOp && t.text == "(":
		p.next()
		v, err := p.or()
		if err != nil {
			return nil, err
		}
		if p.tok.kind != tOp || p.tok.text != ")" {
			return nil, fmt.Errorf("unclosed parenthesis")
		}
		p.next()
		return v, nil
	case t.kind == tStr:
		p.next()
		return t.text, nil
	case t.kind == tWord:
		p.next()
		switch t.text {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
		if f, err := strconv.ParseFloat(t.text, 64); err == nil {
			return f, nil
		}
		return t.text, nil // a bare word: a substituted string value
	}
	if t.kind == tEnd {
		return nil, fmt.Errorf("the condition ends early")
	}
	return nil, fmt.Errorf("unexpected %q", t.text)
}

func compare(l, r any, op string) (any, error) {
	if op == "=~" || op == "!~" {
		re, err := regexp.Compile(fmt.Sprint(r))
		if err != nil {
			return nil, err
		}
		m := re.MatchString(fmt.Sprint(l))
		return m == (op == "=~"), nil
	}
	lf, lnum := l.(float64)
	rf, rnum := r.(float64)
	if lnum && rnum {
		switch op {
		case "==":
			return lf == rf, nil
		case "!=":
			return lf != rf, nil
		case "<":
			return lf < rf, nil
		case ">":
			return lf > rf, nil
		case "<=":
			return lf <= rf, nil
		case ">=":
			return lf >= rf, nil
		}
	}
	ls, rs := fmt.Sprint(l), fmt.Sprint(r)
	switch op {
	case "==":
		return ls == rs, nil
	case "!=":
		return ls != rs, nil
	}
	return nil, fmt.Errorf("%s compares numbers only", op)
}
