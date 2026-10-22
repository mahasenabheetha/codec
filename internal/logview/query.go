package logview

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// The query language, close to Kibana's:
//
//	level>=warn service.name:api -logger:health status>=500
//	"connection reset" /time.?out/ (error OR fatal) message:retry*
//
// Terms are ANDed; OR, NOT (or a leading "-") and parentheses combine
// them. A bare word or "quoted phrase" is searched in the whole entry,
// ignoring case; /regex/ is Go's RE2. field:value matches a field's
// value ignoring case (* is a wildcard; field:* means the field is
// there); >, >=, < and <= compare numbers, durations and levels. The
// core fields have short names: level, message, source, logger,
// caller, error, stack, trace, format, file, and group (an error
// group's id, from Groups); any other name is a field's key ("http.response.status_code", or "log.level" for the
// level as written).

// QueryError is a query that can't be read, with where the problem is.
type QueryError struct {
	Pos int // byte offset in the query
	Msg string
}

func (e *QueryError) Error() string { return fmt.Sprintf("%s (at %d)", e.Msg, e.Pos+1) }

type node interface {
	match(l *Log, i int) bool
}

type andNode []node
type orNode []node
type notNode struct{ n node }

func (n andNode) match(l *Log, i int) bool {
	for _, c := range n {
		if !c.match(l, i) {
			return false
		}
	}
	return true
}

func (n orNode) match(l *Log, i int) bool {
	for _, c := range n {
		if c.match(l, i) {
			return true
		}
	}
	return false
}

func (n notNode) match(l *Log, i int) bool { return !n.n.match(l, i) }

type matchAll struct{}

func (matchAll) match(*Log, int) bool { return true }

type matchNone struct{}

func (matchNone) match(*Log, int) bool { return false }

// textNode searches the entry's whole text.
type textNode struct {
	needle string // lower case when fold
	re     *regexp.Regexp
}

func (n textNode) match(l *Log, i int) bool {
	raw := l.rows[i].raw
	if n.re != nil {
		return n.re.MatchString(raw)
	}
	return containsFold(raw, n.needle)
}

type op uint8

const (
	opIs op = iota
	opGT
	opGE
	opLT
	opLE
)

// value is a term's right-hand side.
type value struct {
	text  string
	any   bool           // "*": the field is there
	re    *regexp.Regexp // /regex/, or a wildcard pattern
	num   float64
	isNum bool
	dur   time.Duration
	isDur bool
}

func (v *value) is(s string, contains bool) bool {
	switch {
	case v.any:
		return true
	case v.re != nil:
		return v.re.MatchString(s)
	case v.isNum && isNumber(s):
		n, _ := strconv.ParseFloat(s, 64)
		return n == v.num
	case contains:
		return containsFold(s, strings.ToLower(v.text))
	}
	return strings.EqualFold(s, v.text)
}

func (v *value) compare(s string, o op) bool {
	var c int
	if v.isNum && isNumber(s) {
		n, _ := strconv.ParseFloat(s, 64)
		c = cmpFloat(n, v.num)
	} else if d, err := time.ParseDuration(s); err == nil && v.isDur {
		c = cmpFloat(float64(d), float64(v.dur))
	} else {
		return false
	}
	switch o {
	case opGT:
		return c > 0
	case opGE:
		return c >= 0
	case opLT:
		return c < 0
	}
	return c <= 0
}

func cmpFloat(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// pseudo fields: the entry's core fields, by short name.
type pseudo uint8

const (
	pNone pseudo = iota
	pLevel
	pMessage
	pSource
	pLogger
	pCaller
	pError
	pStack
	pTrace
	pFormat
	pFile
	pGroup
)

var pseudoNames = map[string]pseudo{
	"level": pLevel, "message": pMessage, "msg": pMessage, "source": pSource, "logger": pLogger,
	"caller": pCaller, "error": pError, "err": pError, "stack": pStack, "trace": pTrace,
	"format": pFormat, "file": pFile, "group": pGroup,
}

// fieldNode matches a field's value.
type fieldNode struct {
	pseudo pseudo
	keys   []uint32 // field keys with this name, for a field of the log
	op     op
	val    value
	level  Level // for level comparisons
}

func (n *fieldNode) match(l *Log, i int) bool {
	r := &l.rows[i]
	if n.pseudo != pNone {
		return n.matchPseudo(l, r)
	}
	for _, c := range l.fields(i) {
		for _, k := range n.keys {
			if c.key != k {
				continue
			}
			if n.op != opIs {
				if n.val.compare(c.text, n.op) {
					return true
				}
				continue
			}
			if c.kind == Array && !n.val.any {
				if n.matchItems(c.text) {
					return true
				}
				continue
			}
			if n.val.is(c.text, false) {
				return true
			}
		}
	}
	return false
}

// matchItems matches any item of an array field: tags:retry.
func (n *fieldNode) matchItems(s string) bool {
	spans, ok := arrayItems(s)
	if !ok {
		return false
	}
	for _, sp := range spans {
		it := s[sp[0]:sp[1]]
		if strings.HasPrefix(it, `"`) {
			p := jscan{s: it}
			it, _ = p.str()
		}
		if n.val.is(it, false) {
			return true
		}
	}
	return false
}

func (n *fieldNode) matchPseudo(l *Log, r *row) bool {
	var x extra
	if r.extra != 0 {
		x = l.extras[r.extra-1]
	}
	var s string
	contains := false
	switch n.pseudo {
	case pLevel:
		if n.val.any {
			return r.level != LevelNone
		}
		if n.val.re != nil {
			return n.val.re.MatchString(r.level.String())
		}
		switch n.op {
		case opIs:
			return r.level == n.level
		case opGT:
			return r.level > n.level
		case opGE:
			return r.level >= n.level
		case opLT:
			return r.level != LevelNone && r.level < n.level
		}
		return r.level != LevelNone && r.level <= n.level
	case pMessage:
		s, contains = r.message, true
	case pSource:
		s = l.strs[r.source]
	case pLogger:
		s = x.logger
	case pCaller:
		s = x.caller
	case pError:
		s, contains = x.err, true
	case pStack:
		s, contains = x.stack, true
	case pTrace:
		s = x.trace
	case pFormat:
		s = r.format.String()
	case pFile:
		s = l.Files[r.file].Name
	case pGroup:
		return r.group != 0 && (n.val.any || n.val.isNum && float64(r.group) == n.val.num)
	}
	if n.op != opIs {
		return n.val.compare(s, n.op)
	}
	if n.val.any {
		return s != ""
	}
	return s != "" && n.val.is(s, contains)
}

// containsFold reports whether s contains the lower-case needle,
// ignoring ASCII case, without allocating.
func containsFold(s, needle string) bool {
	if needle == "" {
		return true
	}
	c := needle[0]
	upper := c
	if c >= 'a' && c <= 'z' {
		upper = c - 32
	}
	n := len(needle)
	for i := 0; i+n <= len(s); i++ {
		// The next place the first letter appears, in either case.
		rest := s[i : len(s)-n+1]
		j := strings.IndexByte(rest, c)
		if upper != c {
			lim := len(rest)
			if j >= 0 {
				lim = j
			}
			if k := strings.IndexByte(rest[:lim], upper); k >= 0 {
				j = k
			}
		}
		if j < 0 {
			return false
		}
		i += j
		if strings.EqualFold(s[i:i+n], needle) {
			return true
		}
	}
	return false
}

// Query parsing.

type parser struct {
	s   string
	i   int
	log *Log
}

// compile reads a query against l's field keys. An empty query matches
// everything. Field names the log doesn't have are reported in warnings.
func (l *Log) compile(q string) (node, []string, error) {
	p := &parser{s: q, log: l}
	p.ws()
	if p.i == len(q) {
		return matchAll{}, nil, nil
	}
	n, err := p.or()
	if err != nil {
		return nil, nil, err
	}
	if p.i < len(q) {
		return nil, nil, &QueryError{p.i, "unexpected " + strconv.Quote(string(q[p.i]))}
	}
	return n, p.warnings(n), nil
}

func (p *parser) ws() {
	for p.i < len(p.s) && (p.s[p.i] == ' ' || p.s[p.i] == '\t' || p.s[p.i] == '\n' || p.s[p.i] == '\r') {
		p.i++
	}
}

// keyword reports whether the next word is kw (in capitals), and takes it.
func (p *parser) keyword(kw string) bool {
	if !strings.HasPrefix(p.s[p.i:], kw) {
		return false
	}
	end := p.i + len(kw)
	if end < len(p.s) && !strings.ContainsRune(" \t\r\n()", rune(p.s[end])) {
		return false
	}
	p.i = end
	return true
}

func (p *parser) or() (node, error) {
	var ns orNode
	for {
		n, err := p.and()
		if err != nil {
			return nil, err
		}
		ns = append(ns, n)
		p.ws()
		if !p.keyword("OR") {
			break
		}
	}
	if len(ns) == 1 {
		return ns[0], nil
	}
	return ns, nil
}

func (p *parser) and() (node, error) {
	var ns andNode
	for {
		p.ws()
		if p.i >= len(p.s) || p.s[p.i] == ')' {
			break
		}
		save := p.i
		if p.keyword("OR") {
			p.i = save
			break
		}
		p.keyword("AND")
		p.ws()
		n, err := p.unary()
		if err != nil {
			return nil, err
		}
		ns = append(ns, n)
	}
	switch len(ns) {
	case 0:
		return nil, &QueryError{p.i, "a term is missing"}
	case 1:
		return ns[0], nil
	}
	return ns, nil
}

func (p *parser) unary() (node, error) {
	p.ws()
	if p.i >= len(p.s) {
		return nil, &QueryError{p.i, "a term is missing"}
	}
	neg := p.s[p.i] == '-'
	if neg {
		p.i++
	} else {
		neg = p.keyword("NOT")
	}
	if neg {
		n, err := p.unary()
		if err != nil {
			return nil, err
		}
		return notNode{n}, nil
	}
	if p.s[p.i] == '(' {
		open := p.i
		p.i++
		n, err := p.or()
		if err != nil {
			return nil, err
		}
		p.ws()
		if p.i >= len(p.s) || p.s[p.i] != ')' {
			return nil, &QueryError{open, "this ( is not closed"}
		}
		p.i++
		return n, nil
	}
	return p.term()
}

func isFieldChar(c byte) bool {
	return isAlnum(c) || c == '.' || c == '@' || c == '-' || c == '$' || c == '/' // label keys: app.kubernetes.io/name
}

func (p *parser) term() (node, error) {
	start := p.i
	switch p.s[p.i] {
	case '"':
		s, err := p.quoted()
		if err != nil {
			return nil, err
		}
		return textNode{needle: strings.ToLower(s)}, nil
	case '/':
		re, err := p.regex()
		if err != nil {
			return nil, err
		}
		return textNode{re: re}, nil
	}
	// field:value, field>=value …
	j := p.i
	for j < len(p.s) && isFieldChar(p.s[j]) {
		j++
	}
	if j > p.i && j < len(p.s) && strings.ContainsRune(":<>", rune(p.s[j])) {
		name := p.s[p.i:j]
		p.i = j
		o := p.op()
		if o == opIs && p.i < len(p.s) && p.s[p.i] == '(' {
			return p.valueList(name, start)
		}
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		return p.field(name, o, v, start)
	}
	w := p.word()
	if w == "" {
		return nil, &QueryError{p.i, "a term is missing"}
	}
	return wordNode(w), nil
}

// valueList reads field:(a OR b c): any of the values.
func (p *parser) valueList(name string, start int) (node, error) {
	open := p.i
	p.i++
	var ns orNode
	for {
		p.ws()
		if p.i >= len(p.s) {
			return nil, &QueryError{open, "this ( is not closed"}
		}
		if p.s[p.i] == ')' {
			p.i++
			break
		}
		if p.keyword("OR") {
			continue
		}
		vstart := p.i
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		n, err := p.field(name, opIs, v, vstart)
		if err != nil {
			return nil, err
		}
		if u, ok := n.(*unknownField); ok {
			// Search the value as text, as for a single unknown field.
			u.text = wordNode(name + ":" + p.s[vstart:p.i])
		}
		ns = append(ns, n)
	}
	if len(ns) == 0 {
		return nil, &QueryError{start, "a value is missing"}
	}
	if len(ns) == 1 {
		return ns[0], nil
	}
	return ns, nil
}

// wordNode searches for a bare word; * matches anything.
func wordNode(w string) node {
	if strings.Contains(w, "*") {
		return textNode{re: globRe(w, false)}
	}
	return textNode{needle: strings.ToLower(w)}
}

func (p *parser) op() op {
	if p.s[p.i] == ':' {
		p.i++
	}
	for _, o := range [...]struct {
		s string
		o op
	}{{">=", opGE}, {"<=", opLE}, {">", opGT}, {"<", opLT}} {
		if strings.HasPrefix(p.s[p.i:], o.s) {
			p.i += len(o.s)
			return o.o
		}
	}
	return opIs
}

func (p *parser) value() (value, error) {
	if p.i >= len(p.s) || strings.ContainsRune(" \t\r\n)", rune(p.s[p.i])) {
		return value{}, &QueryError{p.i, "a value is missing"}
	}
	switch p.s[p.i] {
	case '"':
		s, err := p.quoted()
		return value{text: s}, err
	case '/':
		re, err := p.regex()
		return value{re: re}, err
	}
	w := p.word()
	v := value{text: w}
	switch {
	case w == "*":
		v.any = true
	case strings.Contains(w, "*"):
		v.re = globRe(w, true)
	}
	if isNumber(w) {
		v.num, _ = strconv.ParseFloat(w, 64)
		v.isNum = true
	} else if d, err := time.ParseDuration(w); err == nil {
		v.dur, v.isDur = d, true
	}
	return v, nil
}

// word reads up to a space or a parenthesis.
func (p *parser) word() string {
	start := p.i
	for p.i < len(p.s) && !strings.ContainsRune(" \t\r\n()", rune(p.s[p.i])) {
		p.i++
	}
	return p.s[start:p.i]
}

func (p *parser) quoted() (string, error) {
	start := p.i
	var b strings.Builder
	for p.i++; p.i < len(p.s); p.i++ {
		switch c := p.s[p.i]; c {
		case '\\':
			if p.i+1 < len(p.s) {
				p.i++
				b.WriteByte(p.s[p.i])
			}
		case '"':
			p.i++
			return b.String(), nil
		default:
			b.WriteByte(c)
		}
	}
	return "", &QueryError{start, "this quote is not closed"}
}

func (p *parser) regex() (*regexp.Regexp, error) {
	start := p.i
	var b strings.Builder
	for p.i++; p.i < len(p.s); p.i++ {
		c := p.s[p.i]
		if c == '\\' && p.i+1 < len(p.s) && p.s[p.i+1] == '/' {
			b.WriteByte('/')
			p.i++
			continue
		}
		if c == '/' {
			p.i++
			re, err := regexp.Compile(b.String())
			if err != nil {
				return nil, &QueryError{start, "regex: " + strings.TrimPrefix(err.Error(), "error parsing regexp: ")}
			}
			return re, nil
		}
		b.WriteByte(c)
	}
	return nil, &QueryError{start, "this regex is not closed with /"}
}

// globRe turns a wildcard pattern into a case-insensitive regex, whole
// value or anywhere in the text.
func globRe(w string, whole bool) *regexp.Regexp {
	parts := strings.Split(w, "*")
	for i, s := range parts {
		parts[i] = regexp.QuoteMeta(s)
	}
	pat := strings.Join(parts, ".*")
	if whole {
		pat = "^" + pat + "$"
	}
	return regexp.MustCompile("(?is)" + pat)
}

// field builds a field term. An unknown field with ":" is searched as
// text (a URL like http://x is not a field), with < or > it matches
// nothing and is reported.
func (p *parser) field(name string, o op, v value, start int) (node, error) {
	if ps, ok := pseudoNames[name]; ok {
		n := &fieldNode{pseudo: ps, op: o, val: v}
		if ps == pLevel && !v.any && v.re == nil {
			if n.level = ParseLevel(v.text); n.level == LevelNone {
				return nil, &QueryError{start, fmt.Sprintf("%q is not a level: use trace, debug, info, warn, error or fatal", v.text)}
			}
		}
		return n, nil
	}
	keys := p.keys(name)
	if len(keys) == 0 && o == opIs {
		return &unknownField{name: name, text: wordNode(p.s[start:p.i])}, nil
	}
	if len(keys) == 0 {
		return &unknownField{name: name, text: matchNone{}}, nil
	}
	return &fieldNode{keys: keys, op: o, val: v}, nil
}

// keys finds the log's field keys with this name: exactly, else
// ignoring case.
func (p *parser) keys(name string) []uint32 {
	if p.log == nil {
		return nil
	}
	if id, ok := p.log.keyIDs[name]; ok {
		return []uint32{id}
	}
	var ids []uint32
	for id, k := range p.log.keys {
		if len(k) == len(name) && strings.EqualFold(k, name) {
			ids = append(ids, uint32(id))
		}
	}
	return ids
}

// unknownField stands for a field the log doesn't have.
type unknownField struct {
	name string
	text node
}

func (n *unknownField) match(l *Log, i int) bool { return n.text.match(l, i) }

func (p *parser) warnings(n node) []string {
	var out []string
	var walk func(node)
	walk = func(n node) {
		switch n := n.(type) {
		case andNode:
			for _, c := range n {
				walk(c)
			}
		case orNode:
			for _, c := range n {
				walk(c)
			}
		case notNode:
			walk(n.n)
		case *unknownField:
			out = append(out, "no field is named "+strconv.Quote(n.name))
		}
	}
	walk(n)
	return out
}

// Matches is what a query found.
type Matches struct {
	IDs      []uint32 `json:"-"` // entry indexes, in log order
	Total    int      `json:"total"`
	Levels   [7]int   `json:"levels"`
	Warnings []string `json:"warnings,omitempty"`
}

// Match runs a query over the entries between from and to (zero for no
// bound; entries without a time are kept only when neither is set).
func (l *Log) Match(q string, from, to time.Time) (*Matches, error) {
	if !utf8.ValidString(q) {
		return nil, &QueryError{0, "the query is not valid text"}
	}
	n, warnings, err := l.compile(q)
	if err != nil {
		return nil, err
	}
	m := &Matches{Warnings: warnings}
	lo, hi := int64(0), int64(0)
	if !from.IsZero() {
		lo = from.UnixNano()
	}
	if !to.IsZero() {
		hi = to.UnixNano()
	}
	bounded := lo != 0 || hi != 0
	for i := range l.rows {
		r := &l.rows[i]
		if bounded && (r.time == 0 || lo != 0 && r.time < lo || hi != 0 && r.time >= hi) {
			continue
		}
		if n.match(l, i) {
			m.IDs = append(m.IDs, uint32(i))
			m.Levels[r.level]++
		}
	}
	m.Total = len(m.IDs)
	return m, nil
}
