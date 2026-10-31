// Package jsonkit is the engine behind the JSON & YAML tool (3.7.0):
// JSON read leniently (JSONC and JSON5 as well as JSON) with exact error
// positions and key order kept, written back as strict JSON; sorting,
// flattening to paths and back, a diff by path, a JSON Schema inferred
// from samples, jq, JSON ⇄ YAML, and a scan of large input that builds
// no tree. It has no input or output of its own.
package jsonkit

import (
	"strings"
	"unicode/utf8"
)

// Kind is the type of a JSON value.
type Kind uint8

const (
	Null Kind = iota
	Bool
	Number
	String
	Array
	Object
)

// String names the kind as JSON Schema does ("integer" is a Number).
func (k Kind) String() string {
	return [...]string{"null", "boolean", "number", "string", "array", "object"}[k]
}

// Member is one key and value of an object, in the order read.
type Member struct {
	Key   string
	Value *Value
}

// Value is a JSON value that keeps its objects' key order and its
// numbers' text (so 1.50 stays 1.50 and big integers stay exact).
type Value struct {
	Kind    Kind
	Bool    bool
	Num     string // a Number's JSON text
	Str     string
	Items   []*Value
	Members []Member
}

// Get returns an object's value for key, or nil.
func (v *Value) Get(key string) *Value {
	for _, m := range v.Members {
		if m.Key == key {
			return m.Value
		}
	}
	return nil
}

// Encode writes v as strict JSON: compact when indent is "", else one
// member or item per line, indented by indent.
func Encode(v *Value, indent string) string {
	var b strings.Builder
	encode(&b, v, indent, 0)
	return b.String()
}

func encode(b *strings.Builder, v *Value, indent string, depth int) {
	nl := func(d int) {
		if indent != "" {
			b.WriteByte('\n')
			b.WriteString(strings.Repeat(indent, d))
		}
	}
	switch v.Kind {
	case Null:
		b.WriteString("null")
	case Bool:
		if v.Bool {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case Number:
		b.WriteString(v.Num)
	case String:
		quote(b, v.Str)
	case Array:
		if len(v.Items) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteByte('[')
		for i, it := range v.Items {
			if i > 0 {
				b.WriteByte(',')
			}
			nl(depth + 1)
			encode(b, it, indent, depth+1)
		}
		nl(depth)
		b.WriteByte(']')
	case Object:
		if len(v.Members) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteByte('{')
		for i, m := range v.Members {
			if i > 0 {
				b.WriteByte(',')
			}
			nl(depth + 1)
			quote(b, m.Key)
			b.WriteByte(':')
			if indent != "" {
				b.WriteByte(' ')
			}
			encode(b, m.Value, indent, depth+1)
		}
		nl(depth)
		b.WriteByte('}')
	}
}

// quote writes s as a JSON string. Unlike encoding/json it leaves < > &
// alone: this text is read by people, not put into HTML.
func quote(b *strings.Builder, s string) {
	const hex = "0123456789abcdef"
	b.WriteByte('"')
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			switch {
			case c == '"' || c == '\\':
				b.WriteByte('\\')
				b.WriteByte(c)
			case c == '\n':
				b.WriteString(`\n`)
			case c == '\r':
				b.WriteString(`\r`)
			case c == '\t':
				b.WriteString(`\t`)
			case c < 0x20:
				b.WriteString(`\u00`)
				b.WriteByte(hex[c>>4])
				b.WriteByte(hex[c&0xf])
			default:
				b.WriteByte(c)
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			b.WriteString("\\ufffd")
		case r == '\u2028' || r == '\u2029': // valid JSON, but not in JavaScript strings
			b.WriteString(`\u202`)
			b.WriteByte(hex[r&0xf])
		default:
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	b.WriteByte('"')
}
