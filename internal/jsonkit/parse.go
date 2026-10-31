package jsonkit

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Dialect is how relaxed the text read was.
type Dialect string

const (
	// JSON is strict JSON (RFC 8259).
	JSON Dialect = "json"
	// JSONC is JSON with comments and trailing commas (VS Code settings,
	// tsconfig.json).
	JSONC Dialect = "jsonc"
	// JSON5 adds unquoted keys, single quotes, hex numbers, leading and
	// trailing decimal points, a leading +, and more (json5.org).
	JSON5 Dialect = "json5"
)

// SyntaxError is text that isn't JSON, JSONC or JSON5, with where.
type SyntaxError struct {
	Line   int // 1-based
	Column int // 1-based, in characters
	Msg    string
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("line %d, column %d: %s", e.Line, e.Column, e.Msg)
}

// maxDepth bounds nesting, so hostile input can't exhaust the stack.
const maxDepth = 1000

// Parse reads one JSON, JSONC or JSON5 value (only whitespace and
// comments may follow) and says which dialect the text needed.
func Parse(src string) (*Value, Dialect, error) {
	p := &parser{src: src, dialect: JSON}
	p.skipBOM()
	v, err := p.value(0)
	if err != nil {
		return nil, "", err
	}
	if err := p.space(); err != nil {
		return nil, "", err
	}
	if p.i < len(p.src) {
		return nil, "", p.fail(p.i, "unexpected text after the value")
	}
	return v, p.dialect, nil
}

// ParseAll reads every value in src, one after another (JSON Lines, or
// several documents pasted together).
func ParseAll(src string) ([]*Value, Dialect, error) {
	p := &parser{src: src, dialect: JSON}
	p.skipBOM()
	var out []*Value
	for {
		if err := p.space(); err != nil {
			return nil, "", err
		}
		if p.i >= len(p.src) {
			break
		}
		v, err := p.value(0)
		if err != nil {
			return nil, "", err
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, "", p.fail(p.i, "no JSON value")
	}
	return out, p.dialect, nil
}

type parser struct {
	src     string
	i       int
	dialect Dialect
}

// uses records that the text needed at least dialect d.
func (p *parser) uses(d Dialect) {
	if d == JSON5 || (d == JSONC && p.dialect == JSON) {
		p.dialect = d
	}
}

func (p *parser) skipBOM() {
	if strings.HasPrefix(p.src, "\ufeff") {
		p.i = len("\ufeff")
	}
}

// fail makes a SyntaxError at byte offset off.
func (p *parser) fail(off int, format string, args ...any) error {
	if off > len(p.src) {
		off = len(p.src)
	}
	line, col := 1, 1
	for _, r := range p.src[:off] {
		if r == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return &SyntaxError{Line: line, Column: col, Msg: fmt.Sprintf(format, args...)}
}

func (p *parser) peek() rune {
	if p.i >= len(p.src) {
		return -1
	}
	r, _ := utf8.DecodeRuneInString(p.src[p.i:])
	return r
}

// space skips whitespace and comments.
func (p *parser) space() error {
	for p.i < len(p.src) {
		r, size := utf8.DecodeRuneInString(p.src[p.i:])
		switch {
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			p.i += size
		case r == '\v' || r == '\f' || r == '\u00a0' || r == '\ufeff' || r == '\u2028' || r == '\u2029' || (r > 127 && unicode.Is(unicode.Zs, r)):
			p.uses(JSON5)
			p.i += size
		case r == '/' && strings.HasPrefix(p.src[p.i:], "//"):
			p.uses(JSONC)
			end := strings.IndexByte(p.src[p.i:], '\n')
			if end < 0 {
				p.i = len(p.src)
			} else {
				p.i += end + 1
			}
		case r == '/' && strings.HasPrefix(p.src[p.i:], "/*"):
			p.uses(JSONC)
			end := strings.Index(p.src[p.i+2:], "*/")
			if end < 0 {
				return p.fail(p.i, "comment not closed with */")
			}
			p.i += 2 + end + 2
		default:
			return nil
		}
	}
	return nil
}

func (p *parser) value(depth int) (*Value, error) {
	if depth > maxDepth {
		return nil, p.fail(p.i, "nested deeper than %d levels", maxDepth)
	}
	if err := p.space(); err != nil {
		return nil, err
	}
	start := p.i
	switch r := p.peek(); {
	case r == -1:
		return nil, p.fail(p.i, "the text ends where a value should be")
	case r == '{':
		return p.object(depth)
	case r == '[':
		return p.array(depth)
	case r == '"' || r == '\'':
		s, err := p.string()
		if err != nil {
			return nil, err
		}
		return &Value{Kind: String, Str: s}, nil
	case r == '-' || r == '+' || r == '.' || (r >= '0' && r <= '9'):
		return p.number()
	case r == 't' || r == 'f' || r == 'n' || r == 'I' || r == 'N':
		word := p.ident()
		switch word {
		case "true", "false":
			return &Value{Kind: Bool, Bool: word == "true"}, nil
		case "null":
			return &Value{Kind: Null}, nil
		case "Infinity", "NaN":
			return nil, p.fail(start, "%s is JSON5 but has no JSON form", word)
		}
		if word == "" {
			word = string(r)
		}
		return nil, p.fail(start, "unexpected %q", word)
	default:
		return nil, p.fail(start, "unexpected %q where a value should be", r)
	}
}

func (p *parser) object(depth int) (*Value, error) {
	open := p.i
	p.i++ // {
	v := &Value{Kind: Object, Members: []Member{}}
	index := map[string]int{}
	for {
		if err := p.space(); err != nil {
			return nil, err
		}
		if p.peek() == '}' {
			p.i++
			return v, nil
		}
		keyAt := p.i
		var key string
		switch r := p.peek(); {
		case r == '"' || r == '\'':
			k, err := p.string()
			if err != nil {
				return nil, err
			}
			key = k
		case r == '_' || r == '$' || unicode.IsLetter(r):
			key = p.ident()
			p.uses(JSON5)
		case r == -1:
			return nil, p.fail(open, "object not closed with }")
		default:
			return nil, p.fail(keyAt, "expected a key in quotes")
		}
		if err := p.space(); err != nil {
			return nil, err
		}
		if p.peek() != ':' {
			return nil, p.fail(p.i, "expected : after the key %q", key)
		}
		p.i++
		val, err := p.value(depth + 1)
		if err != nil {
			return nil, err
		}
		// A repeated key: the last value wins, in the first one's place
		// (as JSON.parse and encoding/json read it).
		if at, ok := index[key]; ok {
			v.Members[at].Value = val
		} else {
			index[key] = len(v.Members)
			v.Members = append(v.Members, Member{Key: key, Value: val})
		}
		if err := p.space(); err != nil {
			return nil, err
		}
		switch p.peek() {
		case ',':
			p.i++
			if err := p.space(); err != nil {
				return nil, err
			}
			if p.peek() == '}' {
				p.uses(JSONC) // a trailing comma
				p.i++
				return v, nil
			}
		case '}':
			p.i++
			return v, nil
		case -1:
			return nil, p.fail(open, "object not closed with }")
		default:
			return nil, p.fail(p.i, "expected , or } after a value")
		}
	}
}

func (p *parser) array(depth int) (*Value, error) {
	open := p.i
	p.i++ // [
	v := &Value{Kind: Array, Items: []*Value{}}
	for {
		if err := p.space(); err != nil {
			return nil, err
		}
		if p.peek() == ']' {
			p.i++
			return v, nil
		}
		if p.peek() == -1 {
			return nil, p.fail(open, "array not closed with ]")
		}
		it, err := p.value(depth + 1)
		if err != nil {
			return nil, err
		}
		v.Items = append(v.Items, it)
		if err := p.space(); err != nil {
			return nil, err
		}
		switch p.peek() {
		case ',':
			p.i++
			if err := p.space(); err != nil {
				return nil, err
			}
			if p.peek() == ']' {
				p.uses(JSONC)
				p.i++
				return v, nil
			}
		case ']':
			p.i++
			return v, nil
		case -1:
			return nil, p.fail(open, "array not closed with ]")
		default:
			return nil, p.fail(p.i, "expected , or ] after a value")
		}
	}
}

// ident reads a JSON5 identifier (or a literal such as true).
func (p *parser) ident() string {
	start := p.i
	for p.i < len(p.src) {
		r, size := utf8.DecodeRuneInString(p.src[p.i:])
		if r == '_' || r == '$' || unicode.IsLetter(r) || (p.i > start && (unicode.IsDigit(r) || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Pc, r))) {
			p.i += size
			continue
		}
		break
	}
	return p.src[start:p.i]
}

func (p *parser) string() (string, error) {
	start := p.i
	q := p.src[p.i]
	if q == '\'' {
		p.uses(JSON5)
	}
	p.i++
	var b strings.Builder
	for {
		if p.i >= len(p.src) {
			return "", p.fail(start, "string not closed with %c", q)
		}
		c := p.src[p.i]
		switch {
		case c == q:
			p.i++
			return b.String(), nil
		case c == '\\':
			if err := p.escape(&b); err != nil {
				return "", err
			}
		case c == '\n' || c == '\r':
			return "", p.fail(p.i, "line break inside a string (write \\n)")
		case c < 0x20:
			return "", p.fail(p.i, "control character inside a string (write it as \\u%04x)", c)
		default:
			r, size := utf8.DecodeRuneInString(p.src[p.i:])
			if r == utf8.RuneError && size == 1 {
				return "", p.fail(p.i, "not valid UTF-8")
			}
			b.WriteString(p.src[p.i : p.i+size])
			p.i += size
		}
	}
}

func (p *parser) escape(b *strings.Builder) error {
	at := p.i
	p.i++ // backslash
	if p.i >= len(p.src) {
		return p.fail(at, "string not closed")
	}
	c := p.src[p.i]
	p.i++
	switch c {
	case '"', '\\', '/':
		b.WriteByte(c)
	case 'b':
		b.WriteByte('\b')
	case 'f':
		b.WriteByte('\f')
	case 'n':
		b.WriteByte('\n')
	case 'r':
		b.WriteByte('\r')
	case 't':
		b.WriteByte('\t')
	case 'u':
		r, err := p.hex4(at)
		if err != nil {
			return err
		}
		if r >= 0xD800 && r <= 0xDBFF && strings.HasPrefix(p.src[p.i:], `\u`) {
			save := p.i
			p.i += 2
			lo, err := p.hex4(at)
			if err == nil && lo >= 0xDC00 && lo <= 0xDFFF {
				r = (r-0xD800)<<10 + (lo - 0xDC00) + 0x10000
			} else {
				p.i = save
			}
		}
		if r >= 0xD800 && r <= 0xDFFF {
			r = utf8.RuneError // a lone surrogate has no UTF-8 form
		}
		b.WriteRune(r)
	// JSON5 only from here on.
	case '\'':
		p.uses(JSON5)
		b.WriteByte('\'')
	case 'v':
		p.uses(JSON5)
		b.WriteByte('\v')
	case '0':
		if p.i < len(p.src) && p.src[p.i] >= '0' && p.src[p.i] <= '9' {
			return p.fail(at, "\\0 followed by a digit isn't allowed")
		}
		p.uses(JSON5)
		b.WriteByte(0)
	case 'x':
		p.uses(JSON5)
		if p.i+2 > len(p.src) {
			return p.fail(at, "\\x needs two hex digits")
		}
		n, err := strconv.ParseUint(p.src[p.i:p.i+2], 16, 8)
		if err != nil {
			return p.fail(at, "\\x needs two hex digits")
		}
		p.i += 2
		b.WriteRune(rune(n))
	case '\n':
		p.uses(JSON5) // a line continuation
	case '\r':
		p.uses(JSON5)
		if p.i < len(p.src) && p.src[p.i] == '\n' {
			p.i++
		}
	default:
		if c >= '1' && c <= '9' {
			return p.fail(at, "unknown escape \\%c", c)
		}
		// JSON5 lets any other character stand for itself.
		p.i--
		r, size := utf8.DecodeRuneInString(p.src[p.i:])
		if r == utf8.RuneError && size == 1 {
			return p.fail(p.i, "not valid UTF-8")
		}
		if r == '\u2028' || r == '\u2029' || unicode.IsPrint(r) {
			p.uses(JSON5)
			b.WriteString(p.src[p.i : p.i+size])
			p.i += size
			return nil
		}
		return p.fail(at, "unknown escape \\%c", r)
	}
	return nil
}

func (p *parser) hex4(at int) (rune, error) {
	if p.i+4 > len(p.src) {
		return 0, p.fail(at, "\\u needs four hex digits")
	}
	n, err := strconv.ParseUint(p.src[p.i:p.i+4], 16, 32)
	if err != nil {
		return 0, p.fail(at, "\\u needs four hex digits")
	}
	p.i += 4
	return rune(n), nil
}

// number reads a JSON or JSON5 number and writes it as JSON: +1 → 1,
// .5 → 0.5, 5. → 5, 0x1F → 31.
func (p *parser) number() (*Value, error) {
	start := p.i
	neg := false
	switch p.src[p.i] {
	case '-':
		neg = true
		p.i++
	case '+':
		p.uses(JSON5)
		p.i++
	}
	if strings.HasPrefix(p.src[p.i:], "Infinity") || strings.HasPrefix(p.src[p.i:], "NaN") {
		return nil, p.fail(start, "Infinity and NaN are JSON5 but have no JSON form")
	}
	// Hexadecimal.
	if strings.HasPrefix(p.src[p.i:], "0x") || strings.HasPrefix(p.src[p.i:], "0X") {
		p.uses(JSON5)
		p.i += 2
		d := p.i
		for p.i < len(p.src) && isHex(p.src[p.i]) {
			p.i++
		}
		if p.i == d {
			return nil, p.fail(start, "0x needs hex digits")
		}
		n, ok := new(big.Int).SetString(p.src[d:p.i], 16)
		if !ok {
			return nil, p.fail(start, "bad hex number")
		}
		if neg {
			n.Neg(n)
		}
		return &Value{Kind: Number, Num: n.String()}, nil
	}
	intStart := p.i
	for p.i < len(p.src) && isDigit(p.src[p.i]) {
		p.i++
	}
	intPart := p.src[intStart:p.i]
	if len(intPart) > 1 && intPart[0] == '0' {
		return nil, p.fail(start, "a number can't start with 0 (unless it is 0)")
	}
	frac := ""
	hasDot := false
	if p.i < len(p.src) && p.src[p.i] == '.' {
		hasDot = true
		p.i++
		fs := p.i
		for p.i < len(p.src) && isDigit(p.src[p.i]) {
			p.i++
		}
		frac = p.src[fs:p.i]
	}
	if intPart == "" && frac == "" {
		return nil, p.fail(start, "a number needs digits")
	}
	if intPart == "" || (hasDot && frac == "") {
		p.uses(JSON5) // .5 or 5.
	}
	exp := ""
	if p.i < len(p.src) && (p.src[p.i] == 'e' || p.src[p.i] == 'E') {
		es := p.i
		p.i++
		if p.i < len(p.src) && (p.src[p.i] == '+' || p.src[p.i] == '-') {
			p.i++
		}
		ds := p.i
		for p.i < len(p.src) && isDigit(p.src[p.i]) {
			p.i++
		}
		if p.i == ds {
			return nil, p.fail(es, "an exponent needs digits")
		}
		exp = p.src[es:p.i]
	}
	if intPart == "" {
		intPart = "0"
	}
	num := intPart
	if frac != "" {
		num += "." + frac
	}
	num += exp
	if neg {
		num = "-" + num
	}
	return &Value{Kind: Number, Num: num}, nil
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func isHex(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
