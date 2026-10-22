package logview

import "strconv"

// parseLogfmt reads key=value pairs as Go's slog text handler, logrus
// and Argo write them: values bare or double-quoted with escapes; a key
// without "=" is a bare word. It is lenient where go-logfmt is strict
// (an "=" inside a bare value is kept), since a log line can't be sent
// back. pairs and words count what it saw, for telling logfmt from text.
func parseLogfmt(s string, fields []Field) (out []Field, pairs, words int, ok bool) {
	out = fields
	i := 0
	for {
		for i < len(s) && s[i] <= ' ' {
			i++
		}
		if i >= len(s) {
			return out, pairs, words, true
		}
		start := i
		for i < len(s) && s[i] > ' ' && s[i] != '=' && s[i] != '"' {
			i++
		}
		key := s[start:i]
		if key == "" {
			// A quote or "=" where a key should start: not logfmt.
			return out, pairs, words, false
		}
		if i >= len(s) || s[i] != '=' {
			words++
			continue
		}
		i++ // '='
		pairs++
		if i < len(s) && s[i] == '"' {
			v, n, ok := unquoteLogfmt(s[i:])
			if !ok {
				return out, pairs, words, false
			}
			i += n
			out = append(out, Field{Key: key, Kind: String, Text: v})
			continue
		}
		start = i
		for i < len(s) && s[i] > ' ' {
			i++
		}
		out = append(out, typedField(key, s[start:i]))
	}
}

// typedField types a bare value: numbers and true/false keep their kind,
// so field statistics and comparisons work on them.
func typedField(key, v string) Field {
	switch v {
	case "true", "false":
		return Field{Key: key, Kind: Bool, Text: v}
	}
	if isNumber(v) {
		n, _ := strconv.ParseFloat(v, 64)
		return Field{Key: key, Kind: Number, Text: v, Num: n}
	}
	return Field{Key: key, Kind: String, Text: v}
}

// isNumber reports whether s is a number as JSON writes one.
func isNumber(s string) bool {
	if s == "" {
		return false
	}
	p := jscan{s: s}
	return p.number() && p.i == len(s)
}

// unquoteLogfmt reads the quoted value s starts with, returning it and
// the bytes it used. Escapes are Go's (slog and logrus quote with
// strconv.Quote) or JSON's; a value neither can read is kept as written.
func unquoteLogfmt(s string) (string, int, bool) {
	esc := false
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			esc = true
			i++
		case '"':
			body := s[1:i]
			if !esc {
				return body, i + 1, true
			}
			if q, err := strconv.Unquote(s[:i+1]); err == nil {
				return q, i + 1, true
			}
			p := jscan{s: s[:i+1]}
			if v, ok := p.str(); ok {
				return v, i + 1, true
			}
			return body, i + 1, true
		}
	}
	return "", 0, false
}
