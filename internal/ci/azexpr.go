package ci

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Azure Pipelines template expressions (${{ … }}) are evaluated when
// the pipeline is compiled, from parameters and statically defined
// variables. This is a small evaluator for them: literals, property
// access (parameters.x, variables['x'], loop variables) and the
// built-in functions templates use. Anything it can't know (runtime
// variables, unknown functions) evaluates to "unknown", and the text
// stays as written.

// azValue is an evaluated value: a YAML node, or unknown.
type azValue struct {
	n     *yamlkit.Node
	known bool
}

var unknown = azValue{}

func known(n *yamlkit.Node) azValue { return azValue{n: n, known: true} }

func strVal(s string) azValue {
	return known(&yamlkit.Node{Kind: yamlkit.KindScalar, Tag: yamlkit.TagStr, Value: s, Style: yamlkit.StylePlain})
}

func boolVal(b bool) azValue {
	return known(&yamlkit.Node{Kind: yamlkit.KindScalar, Tag: yamlkit.TagBool, Value: strconv.FormatBool(b), Style: yamlkit.StylePlain})
}

// str is a value as text.
func (v azValue) str() string {
	if v.n == nil {
		return ""
	}
	if v.n.Kind == yamlkit.KindScalar {
		if v.n.Tag == yamlkit.TagNull {
			return ""
		}
		return v.n.Value
	}
	return flow(v.n)
}

// truthy follows Azure: false, null, 0 and "" are false.
func (v azValue) truthy() bool {
	n := v.n
	switch {
	case n == nil:
		return false
	case n.Kind == yamlkit.KindSeq:
		return true
	case n.Kind == yamlkit.KindMap:
		return true
	case n.Tag == yamlkit.TagNull:
		return false
	case n.Tag == yamlkit.TagBool:
		return strings.EqualFold(n.Value, "true")
	case n.Tag == yamlkit.TagInt || n.Tag == yamlkit.TagFloat:
		f, _ := strconv.ParseFloat(n.Value, 64)
		return f != 0
	}
	return n.Value != ""
}

// azScope is what an expression can see.
type azScope struct {
	params map[string]*yamlkit.Node
	vars   map[string]string
	locals map[string]*yamlkit.Node // each loop variables
}

func (sc *azScope) with(name string, v *yamlkit.Node) *azScope {
	locals := map[string]*yamlkit.Node{}
	for k, x := range sc.locals {
		locals[k] = x
	}
	locals[name] = v
	c := *sc
	c.locals = locals
	return &c
}

// --- parsing ---

type azParser struct {
	s   string
	i   int
	sc  *azScope
	err error
}

// azEval evaluates the text inside ${{ }}.
func azEval(expr string, sc *azScope) azValue {
	p := &azParser{s: strings.TrimSpace(expr), sc: sc}
	v := p.expr()
	p.space()
	if p.err != nil || p.i < len(p.s) {
		return unknown
	}
	return v
}

func (p *azParser) space() {
	for p.i < len(p.s) && unicode.IsSpace(rune(p.s[p.i])) {
		p.i++
	}
}

func (p *azParser) fail(msg string) azValue {
	if p.err == nil {
		p.err = fmt.Errorf("%s at %d", msg, p.i)
	}
	return unknown
}

func (p *azParser) expr() azValue {
	p.space()
	if p.i >= len(p.s) {
		return p.fail("expression ends early")
	}
	c := p.s[p.i]
	switch {
	case c == '\'':
		return p.str()
	case c == '-' || (c >= '0' && c <= '9'):
		return p.num()
	case c == '[':
		return p.array()
	case isIdent(c):
		return p.ident()
	}
	return p.fail("unexpected " + string(c))
}

func isIdent(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func (p *azParser) str() azValue {
	var b strings.Builder
	p.i++
	for p.i < len(p.s) {
		if p.s[p.i] == '\'' {
			if p.i+1 < len(p.s) && p.s[p.i+1] == '\'' { // '' is a quote
				b.WriteByte('\'')
				p.i += 2
				continue
			}
			p.i++
			return p.path(strVal(b.String()))
		}
		b.WriteByte(p.s[p.i])
		p.i++
	}
	return p.fail("unclosed string")
}

func (p *azParser) num() azValue {
	start := p.i
	p.i++
	for p.i < len(p.s) && (p.s[p.i] == '.' || (p.s[p.i] >= '0' && p.s[p.i] <= '9')) {
		p.i++
	}
	t := p.s[start:p.i]
	tag := yamlkit.TagInt
	if strings.Contains(t, ".") {
		tag = yamlkit.TagFloat
	}
	return known(&yamlkit.Node{Kind: yamlkit.KindScalar, Tag: tag, Value: t, Style: yamlkit.StylePlain})
}

func (p *azParser) array() azValue {
	p.i++
	out := &yamlkit.Node{Kind: yamlkit.KindSeq}
	ok := true
	for {
		p.space()
		if p.i < len(p.s) && p.s[p.i] == ']' {
			p.i++
			break
		}
		v := p.expr()
		ok = ok && v.known
		out.Items = append(out.Items, v.n)
		p.space()
		if p.i < len(p.s) && p.s[p.i] == ',' {
			p.i++
		}
		if p.err != nil || p.i >= len(p.s) {
			return p.fail("unclosed [")
		}
	}
	if !ok {
		return unknown
	}
	return known(out)
}

func (p *azParser) name() string {
	start := p.i
	for p.i < len(p.s) && (isIdent(p.s[p.i]) || (p.s[p.i] >= '0' && p.s[p.i] <= '9') || p.s[p.i] == '-') {
		p.i++
	}
	return p.s[start:p.i]
}

func (p *azParser) ident() azValue {
	name := p.name()
	p.space()
	if p.i < len(p.s) && p.s[p.i] == '(' {
		p.i++
		var args []azValue
		for {
			p.space()
			if p.i < len(p.s) && p.s[p.i] == ')' {
				p.i++
				break
			}
			args = append(args, p.expr())
			p.space()
			if p.i < len(p.s) && p.s[p.i] == ',' {
				p.i++
			}
			if p.err != nil || p.i >= len(p.s) {
				return p.fail("unclosed (")
			}
		}
		return p.path(call(name, args))
	}
	switch strings.ToLower(name) {
	case "true":
		return boolVal(true)
	case "false":
		return boolVal(false)
	case "null":
		return known(&yamlkit.Node{Kind: yamlkit.KindScalar, Tag: yamlkit.TagNull})
	case "parameters":
		m := &yamlkit.Node{Kind: yamlkit.KindMap}
		for k, v := range p.sc.params {
			m.Pairs = append(m.Pairs, &yamlkit.Pair{Key: &yamlkit.Node{Kind: yamlkit.KindScalar, Value: k}, Value: v})
		}
		return p.path(known(m))
	case "variables":
		return p.variables()
	}
	if v, ok := p.sc.locals[name]; ok {
		return p.path(known(v))
	}
	p.path(unknown) // consume the rest of the reference
	return unknown
}

// variables reads variables.x / variables['x']: known only when set
// statically in the pipeline.
func (p *azParser) variables() azValue {
	var name string
	switch {
	case p.i < len(p.s) && p.s[p.i] == '.':
		p.i++
		start := p.i
		for p.i < len(p.s) && (isIdent(p.s[p.i]) || p.s[p.i] == '.' || (p.s[p.i] >= '0' && p.s[p.i] <= '9') || p.s[p.i] == '-') {
			p.i++
		}
		name = p.s[start:p.i]
	case p.i < len(p.s) && p.s[p.i] == '[':
		p.i++
		p.space()
		k := p.expr()
		p.space()
		if p.i >= len(p.s) || p.s[p.i] != ']' {
			return p.fail("unclosed [")
		}
		p.i++
		name = k.str()
	}
	if v, ok := p.sc.vars[name]; ok {
		return strVal(v)
	}
	return unknown
}

// path applies .key and ['key'] accessors to v.
func (p *azParser) path(v azValue) azValue {
	for p.i < len(p.s) {
		var key string
		switch p.s[p.i] {
		case '.':
			p.i++
			key = p.name()
		case '[':
			p.i++
			k := p.expr()
			p.space()
			if p.i >= len(p.s) || p.s[p.i] != ']' {
				return p.fail("unclosed [")
			}
			p.i++
			if !k.known {
				v = unknown
				continue
			}
			key = k.str()
		default:
			return v
		}
		v = member(v, key)
	}
	return v
}

// member is v.key: a map entry, a list index, or key/value of an each
// pair.
func member(v azValue, key string) azValue {
	if !v.known || v.n == nil {
		return unknown
	}
	switch v.n.Kind {
	case yamlkit.KindMap:
		for _, pr := range v.n.Pairs {
			if strings.EqualFold(pr.Key.Value, key) {
				return known(pr.Value)
			}
		}
		return known(&yamlkit.Node{Kind: yamlkit.KindScalar, Tag: yamlkit.TagNull})
	case yamlkit.KindSeq:
		if i, err := strconv.Atoi(key); err == nil && i >= 0 && i < len(v.n.Items) {
			return known(v.n.Items[i])
		}
	}
	return unknown
}

// --- functions ---

func call(name string, args []azValue) azValue {
	allKnown := true
	for _, a := range args {
		allKnown = allKnown && a.known
	}
	fn := strings.ToLower(name)
	// and/or can be decided by one known argument.
	switch fn {
	case "and":
		for _, a := range args {
			if a.known && !a.truthy() {
				return boolVal(false)
			}
		}
		if allKnown {
			return boolVal(true)
		}
		return unknown
	case "or":
		for _, a := range args {
			if a.known && a.truthy() {
				return boolVal(true)
			}
		}
		if allKnown {
			return boolVal(false)
		}
		return unknown
	case "coalesce":
		for _, a := range args {
			if !a.known {
				return unknown
			}
			if a.str() != "" {
				return a
			}
		}
		return strVal("")
	}
	if !allKnown {
		return unknown
	}
	arg := func(i int) azValue {
		if i < len(args) {
			return args[i]
		}
		return known(&yamlkit.Node{Kind: yamlkit.KindScalar, Tag: yamlkit.TagNull})
	}
	switch fn {
	case "eq":
		return boolVal(azEqual(arg(0), arg(1)))
	case "ne":
		return boolVal(!azEqual(arg(0), arg(1)))
	case "not":
		return boolVal(!arg(0).truthy())
	case "xor":
		return boolVal(arg(0).truthy() != arg(1).truthy())
	case "gt", "lt", "ge", "le":
		c, ok := azCompare(arg(0), arg(1))
		if !ok {
			return unknown
		}
		return boolVal(map[string]bool{"gt": c > 0, "lt": c < 0, "ge": c >= 0, "le": c <= 0}[fn])
	case "in", "notin":
		found := false
		for _, a := range args[1:] {
			found = found || azEqual(arg(0), a)
		}
		return boolVal(found == (fn == "in"))
	case "contains":
		return boolVal(strings.Contains(strings.ToLower(arg(0).str()), strings.ToLower(arg(1).str())))
	case "containsvalue":
		c := arg(0).n
		for _, it := range items(c) {
			if azEqual(known(it), arg(1)) {
				return boolVal(true)
			}
		}
		if isMap(c) {
			for _, pr := range c.Pairs {
				if azEqual(known(pr.Value), arg(1)) {
					return boolVal(true)
				}
			}
		}
		return boolVal(false)
	case "startswith":
		return boolVal(strings.HasPrefix(strings.ToLower(arg(0).str()), strings.ToLower(arg(1).str())))
	case "endswith":
		return boolVal(strings.HasSuffix(strings.ToLower(arg(0).str()), strings.ToLower(arg(1).str())))
	case "lower":
		return strVal(strings.ToLower(arg(0).str()))
	case "upper":
		return strVal(strings.ToUpper(arg(0).str()))
	case "trim":
		return strVal(strings.TrimSpace(arg(0).str()))
	case "replace":
		return strVal(strings.ReplaceAll(arg(0).str(), arg(1).str(), arg(2).str()))
	case "length":
		n := arg(0).n
		switch {
		case n.Kind == yamlkit.KindSeq:
			return strVal(strconv.Itoa(len(n.Items)))
		case n.Kind == yamlkit.KindMap:
			return strVal(strconv.Itoa(len(n.Pairs)))
		}
		return strVal(strconv.Itoa(len([]rune(arg(0).str()))))
	case "join":
		var parts []string
		for _, it := range items(arg(1).n) {
			parts = append(parts, known(it).str())
		}
		return strVal(strings.Join(parts, arg(0).str()))
	case "split":
		out := &yamlkit.Node{Kind: yamlkit.KindSeq}
		for _, s := range strings.Split(arg(0).str(), arg(1).str()) {
			out.Items = append(out.Items, strVal(s).n)
		}
		return known(out)
	case "format":
		s := arg(0).str()
		for i, a := range args[1:] {
			s = strings.ReplaceAll(s, "{"+strconv.Itoa(i)+"}", a.str())
		}
		return strVal(s)
	case "iif":
		if arg(0).truthy() {
			return arg(1)
		}
		return arg(2)
	case "converttojson":
		return strVal(flow(arg(0).n))
	}
	return unknown
}

// azEqual compares like Azure: numbers as numbers, text ignoring case.
func azEqual(a, b azValue) bool {
	if c, ok := azCompare(a, b); ok {
		return c == 0
	}
	return strings.EqualFold(a.str(), b.str())
}

func azCompare(a, b azValue) (int, bool) {
	x, e1 := strconv.ParseFloat(a.str(), 64)
	y, e2 := strconv.ParseFloat(b.str(), 64)
	if e1 != nil || e2 != nil {
		s, t := strings.ToLower(a.str()), strings.ToLower(b.str())
		return strings.Compare(s, t), a.str() != "" && b.str() != "" && e1 != nil && e2 != nil
	}
	switch {
	case x < y:
		return -1, true
	case x > y:
		return 1, true
	}
	return 0, true
}
