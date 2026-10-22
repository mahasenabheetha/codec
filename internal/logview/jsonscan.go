package logview

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// A JSON reader of our own, because a log entry's fields should keep the
// order they were written in (encoding/json's maps lose it) and most
// strings in a log need no unescaping, so they can share the input's
// memory. It accepts exactly what encoding/json accepts (a fuzz test
// checks this).

const (
	maxFlatten = 16    // object levels flattened into dotted keys; deeper ones stay JSON text
	maxDepth   = 10000 // nesting accepted at all, as encoding/json does
)

type jscan struct {
	s   string
	i   int
	out []Field
}

// parseObject reads s as one JSON object, surrounded by nothing but
// whitespace, into fields flattened in the order written.
func parseObject(s string, fields []Field) ([]Field, bool) {
	p := jscan{s: s, out: fields}
	p.ws()
	if p.i >= len(s) || s[p.i] != '{' || !p.object("", 0) {
		return fields, false
	}
	p.ws()
	return p.out, p.i == len(s)
}

// arrayItems returns where each item of a JSON array starts and ends,
// or false when s isn't one.
func arrayItems(s string) ([][2]int, bool) {
	p := jscan{s: s}
	p.ws()
	if p.i >= len(s) || s[p.i] != '[' {
		return nil, false
	}
	p.i++
	var items [][2]int
	p.ws()
	if p.i < len(s) && s[p.i] == ']' {
		p.i++
	} else {
		for {
			p.ws()
			start := p.i
			if !p.skip() {
				return nil, false
			}
			items = append(items, [2]int{start, p.i})
			p.ws()
			if p.i >= len(s) {
				return nil, false
			}
			if s[p.i] == ']' {
				p.i++
				break
			}
			if s[p.i] != ',' {
				return nil, false
			}
			p.i++
		}
	}
	p.ws()
	return items, p.i == len(s)
}

func (p *jscan) ws() {
	for p.i < len(p.s) {
		switch p.s[p.i] {
		case ' ', '\t', '\n', '\r':
			p.i++
		default:
			return
		}
	}
}

// object reads the object at p.i, naming its fields prefix.key.
func (p *jscan) object(prefix string, depth int) bool {
	p.i++ // '{'
	p.ws()
	if p.i < len(p.s) && p.s[p.i] == '}' {
		p.i++
		return true
	}
	for {
		p.ws()
		key, ok := p.str()
		if !ok {
			return false
		}
		p.ws()
		if p.i >= len(p.s) || p.s[p.i] != ':' {
			return false
		}
		p.i++
		p.ws()
		if prefix != "" {
			key = prefix + "." + key
		}
		if !p.value(key, depth) {
			return false
		}
		p.ws()
		if p.i >= len(p.s) {
			return false
		}
		switch p.s[p.i] {
		case ',':
			p.i++
		case '}':
			p.i++
			return true
		default:
			return false
		}
	}
}

func (p *jscan) value(key string, depth int) bool {
	if p.i >= len(p.s) {
		return false
	}
	start := p.i
	switch c := p.s[p.i]; c {
	case '{':
		n := len(p.out)
		if depth+1 < maxFlatten {
			if !p.object(key, depth+1) {
				return false
			}
			if len(p.out) > n {
				return true
			}
		} else if !p.skip() {
			return false
		}
		// Too deep to flatten, or empty: keep it as one field.
		p.out = append(p.out, Field{Key: key, Kind: Object, Text: compact(p.s[start:p.i])})
	case '[':
		if !p.skip() {
			return false
		}
		p.out = append(p.out, Field{Key: key, Kind: Array, Text: compact(p.s[start:p.i])})
	case '"':
		v, ok := p.str()
		if !ok {
			return false
		}
		p.out = append(p.out, Field{Key: key, Kind: String, Text: v})
	default:
		k, ok := p.scalar()
		if !ok {
			return false
		}
		f := Field{Key: key, Kind: k, Text: p.s[start:p.i]}
		if k == Number {
			// Out of range is still valid JSON: Num is then ±Inf.
			f.Num, _ = strconv.ParseFloat(f.Text, 64)
		}
		p.out = append(p.out, f)
	}
	return true
}

// compact drops the whitespace of pretty-printed JSON.
func compact(s string) string {
	if !strings.ContainsAny(s, "\n\r\t ") {
		return s
	}
	var b bytes.Buffer
	if json.Compact(&b, []byte(s)) != nil {
		return s
	}
	return b.String()
}

// skip passes over one valid value without recording it. It keeps its
// own stack, so deep nesting can't exhaust the goroutine's.
func (p *jscan) skip() bool {
	var stack []byte
	for {
		// A value.
		p.ws()
		if p.i >= len(p.s) {
			return false
		}
		switch c := p.s[p.i]; c {
		case '{', '[':
			if len(stack) >= maxDepth {
				return false
			}
			p.i++
			p.ws()
			if p.i < len(p.s) && (c == '{' && p.s[p.i] == '}' || c == '[' && p.s[p.i] == ']') {
				p.i++
				break
			}
			stack = append(stack, c)
			if c == '{' && !p.key() {
				return false
			}
			continue
		case '"':
			if !p.skipStr() {
				return false
			}
		default:
			if _, ok := p.scalar(); !ok {
				return false
			}
		}
		// What follows a value: closing brackets, then a comma or the end.
		for {
			if len(stack) == 0 {
				return true
			}
			p.ws()
			if p.i >= len(p.s) {
				return false
			}
			top, c := stack[len(stack)-1], p.s[p.i]
			if top == '{' && c == '}' || top == '[' && c == ']' {
				p.i++
				stack = stack[:len(stack)-1]
				continue
			}
			if c != ',' {
				return false
			}
			p.i++
			if top == '{' && !p.key() {
				return false
			}
			break
		}
	}
}

// key passes over `"key" :`.
func (p *jscan) key() bool {
	p.ws()
	if !p.skipStr() {
		return false
	}
	p.ws()
	if p.i >= len(p.s) || p.s[p.i] != ':' {
		return false
	}
	p.i++
	return true
}

// skipStr passes over a string, checking its escapes.
func (p *jscan) skipStr() bool {
	if p.i >= len(p.s) || p.s[p.i] != '"' {
		return false
	}
	for i := p.i + 1; i < len(p.s); i++ {
		switch c := p.s[i]; {
		case c == '"':
			p.i = i + 1
			return true
		case c < 0x20:
			return false
		case c == '\\':
			n, ok := escapeLen(p.s[i:])
			if !ok {
				return false
			}
			i += n - 1
		}
	}
	return false
}

// escapeLen is the length of the escape sequence s starts with.
func escapeLen(s string) (int, bool) {
	if len(s) < 2 {
		return 0, false
	}
	switch s[1] {
	case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
		return 2, true
	case 'u':
		if len(s) < 6 {
			return 0, false
		}
		for _, c := range []byte(s[2:6]) {
			if !isHex(c) {
				return 0, false
			}
		}
		return 6, true
	}
	return 0, false
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// str reads a string. Without escapes it shares the input's memory.
func (p *jscan) str() (string, bool) {
	if p.i >= len(p.s) || p.s[p.i] != '"' {
		return "", false
	}
	start := p.i + 1
	for i := start; i < len(p.s); i++ {
		switch c := p.s[i]; {
		case c == '"':
			p.i = i + 1
			return p.s[start:i], true
		case c < 0x20:
			return "", false
		case c == '\\':
			if !p.skipStr() {
				return "", false
			}
			return unescape(p.s[start : p.i-1]), true
		}
	}
	return "", false
}

// unescape decodes a string body whose escapes skipStr has checked. A
// lone surrogate becomes U+FFFD, as encoding/json does.
func unescape(s string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); {
		c := s[i]
		if c != '\\' {
			b = append(b, c)
			i++
			continue
		}
		switch s[i+1] {
		case 'b':
			b = append(b, '\b')
		case 'f':
			b = append(b, '\f')
		case 'n':
			b = append(b, '\n')
		case 'r':
			b = append(b, '\r')
		case 't':
			b = append(b, '\t')
		case 'u':
			r := rune(hex4(s[i+2:]))
			i += 6
			if utf16.IsSurrogate(r) {
				if i+6 <= len(s) && s[i] == '\\' && s[i+1] == 'u' {
					if r2 := utf16.DecodeRune(r, rune(hex4(s[i+2:]))); r2 != utf8.RuneError {
						b = utf8.AppendRune(b, r2)
						i += 6
						continue
					}
				}
				r = utf8.RuneError
			}
			b = utf8.AppendRune(b, r)
			continue
		default: // '"', '\\', '/'
			b = append(b, s[i+1])
		}
		i += 2
	}
	return string(b)
}

func hex4(s string) uint16 {
	var v uint16
	for _, c := range []byte(s[:4]) {
		switch {
		case c >= '0' && c <= '9':
			c -= '0'
		case c >= 'a' && c <= 'f':
			c -= 'a' - 10
		default:
			c -= 'A' - 10
		}
		v = v<<4 | uint16(c)
	}
	return v
}

// scalar passes over true, false, null or a number.
func (p *jscan) scalar() (Kind, bool) {
	for _, lit := range [...]struct {
		s string
		k Kind
	}{{"true", Bool}, {"false", Bool}, {"null", Null}} {
		if strings.HasPrefix(p.s[p.i:], lit.s) {
			p.i += len(lit.s)
			return lit.k, true
		}
	}
	return Number, p.number()
}

// number passes over -?(0|[1-9]\d*)(\.\d+)?([eE][+-]?\d+)?.
func (p *jscan) number() bool {
	s, i := p.s, p.i
	digits := func() bool {
		start := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		return i > start
	}
	if i < len(s) && s[i] == '-' {
		i++
	}
	switch {
	case i < len(s) && s[i] == '0':
		i++
	case !digits():
		return false
	}
	if i < len(s) && s[i] == '.' {
		i++
		if !digits() {
			return false
		}
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		if !digits() {
			return false
		}
	}
	p.i = i
	return true
}
