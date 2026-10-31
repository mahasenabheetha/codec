package httpkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Fetch writes r as JavaScript that sends it with fetch (browsers, Node
// 18 and later, Deno, Bun).
func (r *Request) Fetch() string {
	var b strings.Builder
	if len(r.Form) > 0 {
		b.WriteString("const form = new FormData();\n")
		for _, f := range r.Form {
			if f.File {
				// The third argument is the file name FormData sends.
				fmt.Fprintf(&b, "form.append(%s, fileInput.files[0], %s); // pick the file %s\n", jsString(f.Name), jsString(f.Value), f.Value)
				continue
			}
			fmt.Fprintf(&b, "form.append(%s, %s);\n", jsString(f.Name), jsString(f.Value))
		}
		b.WriteString("\n")
	}
	var opts []string
	if r.Method != "GET" {
		opts = append(opts, "method: "+jsString(r.Method))
	}
	var hs []string
	for _, h := range r.Headers {
		if len(r.Form) > 0 && strings.EqualFold(h.Name, "Content-Type") && strings.HasPrefix(strings.ToLower(h.Value), "multipart/") {
			continue // FormData sets it, with the boundary
		}
		hs = append(hs, "    "+jsString(h.Name)+": "+jsString(h.Value))
	}
	if len(hs) > 0 {
		opts = append(opts, "headers: {\n"+strings.Join(hs, ",\n")+",\n  }")
	}
	switch {
	case len(r.Form) > 0:
		opts = append(opts, "body: form")
	case r.jsonBody():
		var pretty bytes.Buffer
		json.Indent(&pretty, []byte(r.Body), "  ", "  ")
		opts = append(opts, "body: JSON.stringify("+pretty.String()+")")
	case r.Body != "":
		opts = append(opts, "body: "+jsString(r.Body))
	}
	if r.TimeoutMS > 0 {
		opts = append(opts, fmt.Sprintf("signal: AbortSignal.timeout(%d)", r.TimeoutMS))
	}
	b.WriteString("const response = await fetch(" + jsString(r.URL))
	if len(opts) > 0 {
		b.WriteString(", {\n  " + strings.Join(opts, ",\n  ") + ",\n}")
	}
	b.WriteString(");\n")
	return b.String()
}

// ParseFetch reads a fetch call (and the FormData built for it) back
// into a request. Only literal values can be read: a variable is named
// in a note.
func ParseFetch(code string) (*Request, error) {
	code = strings.ToValidUTF8(code, "\uFFFD")
	at := strings.Index(code, "fetch(")
	if at < 0 {
		return nil, errors.New("paste a fetch(…) call")
	}
	p := &jsParser{s: code, i: at + len("fetch(")}
	r := &Request{Method: "GET"}
	first, err := p.value()
	if err != nil {
		return nil, err
	}
	switch u := first.(type) {
	case string:
		r.URL = u
	case jsIdent:
		if !strings.Contains(string(u), "${") {
			return nil, errors.New("fetch's first argument must be the URL as a string")
		}
		r.URL = string(u)
		r.note("The URL has ${…} in it; fill those in.")
	default:
		return nil, errors.New("fetch's first argument must be the URL as a string")
	}
	p.space()
	if p.peek() == ',' {
		p.i++
		p.space()
		if p.peek() != ')' {
			o, err := p.value()
			if err != nil {
				return nil, err
			}
			obj, ok := o.(jsObject)
			if !ok {
				return nil, errors.New("fetch's second argument must be an object literal")
			}
			if err := r.options(obj, code); err != nil {
				return nil, err
			}
		}
	}
	return r, nil
}

func (r *Request) options(obj jsObject, code string) error {
	for _, kv := range obj {
		switch kv.key {
		case "method":
			m, ok := kv.val.(string)
			if !ok {
				r.note("method isn't a string; GET is used.")
				continue
			}
			r.Method = strings.ToUpper(m)
		case "headers":
			h, ok := kv.val.(jsObject)
			if !ok {
				r.note("headers isn't an object literal; left out.")
				continue
			}
			for _, x := range h {
				v, ok := x.val.(string)
				if !ok {
					v = jsText(x.val)
					if _, isVar := x.val.(jsIdent); isVar {
						r.note(fmt.Sprintf("The header %s is a variable; put its value in.", x.key))
					}
				}
				r.Headers = append(r.Headers, Header{x.key, v})
			}
		case "body":
			switch v := kv.val.(type) {
			case string:
				r.Body = v
			case jsIdent:
				if string(v) == "form" || strings.Contains(code, string(v)+" = new FormData") {
					r.Form = formFields(code, string(v))
					continue
				}
				r.note(fmt.Sprintf("The body is the variable %s; put its content in.", v))
			default:
				r.note("The body isn't a string or JSON.stringify(…) of a literal; left out.")
			}
		case "signal":
			if n, ok := kv.val.(jsTimeout); ok {
				r.TimeoutMS = int(n)
			} else {
				r.note("signal is left out (only AbortSignal.timeout(ms) becomes --max-time).")
			}
		case "credentials":
			if kv.val == "include" {
				r.note(`credentials: "include" sends the page's cookies; add them with -b.`)
			}
		case "redirect":
			if kv.val == "follow" {
				continue
			}
			r.note(fmt.Sprintf("redirect: %s has no curl flag; curl doesn't follow redirects unless -L is given.", jsText(kv.val)))
		case "referrer":
			if s, ok := kv.val.(string); ok && s != "" {
				r.Headers = append(r.Headers, Header{"Referer", s})
			}
		case "mode", "cache", "keepalive", "priority", "integrity", "referrerPolicy", "window", "duplex":
			// browser behaviour curl has no part in
		default:
			r.note(fmt.Sprintf("The option %s isn't converted.", kv.key))
		}
	}
	return nil
}

// formFields reads form.append("name", "value") calls for a FormData
// named v.
func formFields(code, v string) []Field {
	var out []Field
	for rest := code; ; {
		at := strings.Index(rest, v+".append(")
		if at < 0 {
			return out
		}
		p := &jsParser{s: rest, i: at + len(v+".append(")}
		rest = rest[at+1:]
		name, err := p.value()
		if err != nil {
			continue
		}
		p.space()
		if p.peek() != ',' {
			continue
		}
		p.i++
		val, err := p.value()
		n, _ := name.(string)
		if s, ok := val.(string); ok && err == nil {
			out = append(out, Field{Name: n, Value: s})
			continue
		}
		// A file: its name is the third argument when there is one.
		f := Field{Name: n, Value: jsText(val), File: true}
		if p.space(); p.peek() == ',' {
			p.i++
			if fn, err := p.value(); err == nil {
				if s, ok := fn.(string); ok {
					f.Value = s
				}
			}
		}
		out = append(out, f)
	}
}

// Values a fetch call can hold, as parsed.
type (
	jsObject []jsKV
	jsKV     struct {
		key string
		val any
	}
	jsIdent   string // a variable or expression codec can't read
	jsNumber  string // the number as written
	jsTimeout int    // AbortSignal.timeout(ms)
)

// jsText is a value as text for a note or a header.
func jsText(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case jsIdent:
		return string(v)
	case jsNumber:
		return string(v)
	case bool:
		return strconv.FormatBool(v)
	case nil:
		return "null"
	}
	b, _ := marshalJS(v)
	return string(b)
}

type jsParser struct {
	s string
	i int
}

func (p *jsParser) peek() byte {
	if p.i < len(p.s) {
		return p.s[p.i]
	}
	return 0
}

// space skips white space and comments.
func (p *jsParser) space() {
	for p.i < len(p.s) {
		switch {
		case strings.IndexByte(" \t\r\n", p.s[p.i]) >= 0:
			p.i++
		case strings.HasPrefix(p.s[p.i:], "//"):
			for p.i < len(p.s) && p.s[p.i] != '\n' {
				p.i++
			}
		case strings.HasPrefix(p.s[p.i:], "/*"):
			end := strings.Index(p.s[p.i+2:], "*/")
			if end < 0 {
				p.i = len(p.s)
				return
			}
			p.i += end + 4
		default:
			return
		}
	}
}

func (p *jsParser) value() (any, error) {
	p.space()
	c := p.peek()
	switch {
	case c == '"' || c == '\'' || c == '`':
		return p.str()
	case c == '{':
		return p.object()
	case c == '[':
		return p.array()
	case c == '-' || c >= '0' && c <= '9':
		start := p.i
		p.i++
		for p.i < len(p.s) && strings.IndexByte("0123456789.eE+-_xXabcdefABCDEFn", p.s[p.i]) >= 0 {
			p.i++
		}
		return jsNumber(strings.ReplaceAll(p.s[start:p.i], "_", "")), nil
	case c == 0:
		return nil, errors.New("the fetch call ends too soon")
	}
	// A name: a keyword, a call codec knows, or a variable.
	start := p.i
	for p.i < len(p.s) {
		c := p.s[p.i]
		if c == '[' && p.i > start { // index access: input.files[0], rows[i].id
			end := strings.IndexByte(p.s[p.i:], ']')
			if end < 0 {
				break
			}
			p.i += end + 1
			continue
		}
		if !isLetter(c) && c != '_' && c != '$' && c != '.' && (c < '0' || c > '9') {
			break
		}
		p.i++
	}
	name := p.s[start:p.i]
	if name == "" {
		return nil, fmt.Errorf("can't read %q in the fetch call", firstLine(p.s[p.i:]))
	}
	if name == "new" {
		p.space()
		return p.value()
	}
	p.space()
	if p.peek() == '(' {
		p.i++
		var args []any
		for {
			p.space()
			if p.peek() == ')' {
				p.i++
				break
			}
			a, err := p.value()
			if err != nil {
				return nil, err
			}
			args = append(args, a)
			p.space()
			if p.peek() == ',' {
				p.i++
			}
		}
		switch {
		case name == "JSON.stringify" && len(args) > 0:
			if _, isVar := args[0].(jsIdent); !isVar {
				b, err := marshalJS(args[0])
				if err == nil {
					return string(b), nil
				}
			}
		case name == "Headers" && len(args) == 1:
			return args[0], nil
		case name == "AbortSignal.timeout" && len(args) == 1:
			if n, ok := args[0].(jsNumber); ok {
				if ms, err := strconv.Atoi(string(n)); err == nil {
					return jsTimeout(ms), nil
				}
			}
		case name == "String" && len(args) == 1:
			return jsText(args[0]), nil
		}
		return jsIdent(name + "(…)"), nil
	}
	switch name {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "null", "undefined":
		return nil, nil
	}
	return jsIdent(name), nil
}

// str reads a quoted string; a template with ${…} is an expression.
func (p *jsParser) str() (any, error) {
	q := p.s[p.i]
	p.i++
	var b strings.Builder
	for p.i < len(p.s) {
		c := p.s[p.i]
		switch {
		case c == q:
			p.i++
			return b.String(), nil
		case c == '$' && q == '`' && p.i+1 < len(p.s) && p.s[p.i+1] == '{':
			// An expression: kept as written ("Bearer ${token}") so it
			// shows what to fill in.
			end := strings.IndexByte(p.s[p.i:], '`')
			if end < 0 {
				return nil, errors.New("a ` string is never closed")
			}
			text := b.String() + p.s[p.i:p.i+end]
			p.i += end + 1
			return jsIdent(text), nil
		case c == '\\' && p.i+1 < len(p.s):
			p.i++
			n, err := jsEscape(p.s[p.i:], &b)
			if err != nil {
				return nil, err
			}
			p.i += n
		case c == '\n' && q != '`':
			return nil, errors.New("a string runs past the end of its line")
		default:
			b.WriteByte(c)
			p.i++
		}
	}
	return nil, errors.New("a string is never closed")
}

// jsEscape writes the character an escape stands for and returns how
// many bytes after the backslash it took.
func jsEscape(s string, b *strings.Builder) (int, error) {
	switch c := s[0]; c {
	case 'n':
		b.WriteByte('\n')
	case 't':
		b.WriteByte('\t')
	case 'r':
		b.WriteByte('\r')
	case 'b':
		b.WriteByte('\b')
	case 'f':
		b.WriteByte('\f')
	case 'v':
		b.WriteByte('\v')
	case '0':
		b.WriteByte(0)
	case '\n':
	case 'x':
		if len(s) < 3 {
			return 0, errors.New(`a \x escape is cut short`)
		}
		n, err := strconv.ParseUint(s[1:3], 16, 8)
		if err != nil {
			return 0, errors.New(`a \x escape isn't hex`)
		}
		b.WriteRune(rune(n))
		return 3, nil
	case 'u':
		if strings.HasPrefix(s, "u{") {
			end := strings.IndexByte(s, '}')
			n, err := strconv.ParseUint(s[2:max(end, 2)], 16, 32)
			if end < 0 || err != nil {
				return 0, errors.New(`a \u{…} escape isn't hex`)
			}
			b.WriteRune(rune(n))
			return end + 1, nil
		}
		if len(s) < 5 {
			return 0, errors.New(`a \u escape is cut short`)
		}
		n, err := strconv.ParseUint(s[1:5], 16, 16)
		if err != nil {
			return 0, errors.New(`a \u escape isn't hex`)
		}
		r := rune(n)
		// A surrogate pair, as JSON writes characters past U+FFFF.
		if 0xd800 <= r && r < 0xdc00 && len(s) >= 11 && s[5] == '\\' && s[6] == 'u' {
			if lo, err := strconv.ParseUint(s[7:11], 16, 16); err == nil && 0xdc00 <= lo && lo < 0xe000 {
				b.WriteRune((r-0xd800)<<10 + (rune(lo) - 0xdc00) + 0x10000)
				return 11, nil
			}
		}
		b.WriteRune(r)
		return 5, nil
	default:
		b.WriteByte(c)
	}
	return 1, nil
}

func (p *jsParser) object() (any, error) {
	p.i++
	var obj jsObject
	for {
		p.space()
		switch p.peek() {
		case '}':
			p.i++
			return obj, nil
		case 0:
			return nil, errors.New("an object is never closed")
		}
		var key string
		if c := p.peek(); c == '"' || c == '\'' {
			k, err := p.str()
			if err != nil {
				return nil, err
			}
			key, _ = k.(string)
		} else {
			start := p.i
			for p.i < len(p.s) && (isLetter(p.s[p.i]) || p.s[p.i] == '_' || p.s[p.i] == '$' || p.s[p.i] >= '0' && p.s[p.i] <= '9') {
				p.i++
			}
			key = p.s[start:p.i]
			if key == "" {
				return nil, fmt.Errorf("can't read the object key at %q", firstLine(p.s[p.i:]))
			}
		}
		p.space()
		var val any = jsIdent(key) // shorthand { body }
		if p.peek() == ':' {
			p.i++
			v, err := p.value()
			if err != nil {
				return nil, err
			}
			val = v
		}
		obj = append(obj, jsKV{key, val})
		p.space()
		if p.peek() == ',' {
			p.i++
		}
	}
}

func (p *jsParser) array() (any, error) {
	p.i++
	var arr []any
	for {
		p.space()
		switch p.peek() {
		case ']':
			p.i++
			return arr, nil
		case 0:
			return nil, errors.New("an array is never closed")
		}
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		arr = append(arr, v)
		p.space()
		if p.peek() == ',' {
			p.i++
		}
	}
}

// marshalJS writes a literal as compact JSON, keys in their order and
// numbers as written. A variable can't be written.
func marshalJS(v any) ([]byte, error) {
	var b bytes.Buffer
	var write func(any) error
	write = func(v any) error {
		switch v := v.(type) {
		case string:
			b.WriteString(jsString(v))
		case jsNumber:
			if !json.Valid([]byte(v)) {
				return fmt.Errorf("%s isn't a JSON number", string(v))
			}
			b.WriteString(string(v))
		case bool:
			b.WriteString(strconv.FormatBool(v))
		case nil:
			b.WriteString("null")
		case jsObject:
			b.WriteByte('{')
			for i, kv := range v {
				if i > 0 {
					b.WriteByte(',')
				}
				b.WriteString(jsString(kv.key) + ":")
				if err := write(kv.val); err != nil {
					return err
				}
			}
			b.WriteByte('}')
		case []any:
			b.WriteByte('[')
			for i, x := range v {
				if i > 0 {
					b.WriteByte(',')
				}
				if err := write(x); err != nil {
					return err
				}
			}
			b.WriteByte(']')
		default:
			return fmt.Errorf("%s can't be written as JSON", jsText(v))
		}
		return nil
	}
	err := write(v)
	return b.Bytes(), err
}
