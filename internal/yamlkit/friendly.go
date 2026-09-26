package yamlkit

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// friendly turns a parser error (message + 1-based line/col) into a
// Diagnostic that says what is wrong in plain English and how to fix
// it. Classification looks at the offending line itself, because
// parser messages ("mapping value is not allowed in this context")
// describe the parser's confusion, not the author's mistake.
func (p *parseState) friendly(line, col int, msg string) Diagnostic {
	text := p.lineIn(p.masked, line)
	lower := strings.ToLower(msg)
	d := Diagnostic{Severity: SeverityError, Code: "syntax", Message: capitalize(msg)}

	switch {
	case strings.Contains(msg, "'\t'") || strings.ContainsRune(leading(text), '\t'):
		tabCol := col
		if i := strings.IndexByte(leading(text), '\t'); i >= 0 {
			tabCol = i + 1
		}
		d.Code = "tab-indent"
		d.Message = "Tab character used for indentation"
		d.Hint = "YAML only allows spaces for indentation. Replace the tab with spaces."
		d.Range = p.cols(line, tabCol, tabCol+1)
		return d

	case strings.Contains(lower, "could not find end character"):
		quote := "double"
		if r := runeAt(text, col); r == '\'' {
			quote = "single"
		}
		d.Code = "unclosed-quote"
		d.Message = fmt.Sprintf("This %s-quoted string is never closed", quote)
		d.Hint = "Add the missing closing quote, or remove the opening one."
		d.Range = p.cols(line, col, utf8.RuneCountInString(text)+1)
		return d

	case strings.Contains(lower, "mapping value"):
		if c := secondColon(text); c > 0 {
			d.Code = "unquoted-colon"
			d.Message = "A value contains ': ' but isn't quoted"
			d.Hint = `YAML reads ": " as the start of a new key. Wrap the whole value in quotes, e.g. msg: "error: bad thing".`
			d.Range = p.cols(line, c, c+1)
			return d
		}
		// "key: value" followed by a deeper line: the parser blames the
		// value, but the mistake is the next line's indentation.
		if key, value, ok := keyValue(text); ok && value != "" {
			if next, ok := p.nextContentLine(line); ok {
				nt := p.lineIn(p.masked, next)
				if got := len(leading(nt)); got > len(leading(text)) {
					d.Code = "nested-under-value"
					d.Message = "This line is indented under a key that already has a value"
					d.Hint = fmt.Sprintf("%q already has a value on line %d, so nothing can be nested under it. Line this up with %q, or move the value onto its own lines.", key, line, key)
					d.Range = p.cols(next, 1, got+2)
					return d
				}
			}
		}
	}

	// Indentation that matches no enclosing or sibling level is the
	// most common cause of the remaining "not allowed / expected" errors.
	if want, ok := p.expectedIndent(line); ok {
		got := len(leading(text))
		d.Code = "bad-indent"
		d.Message = "Indentation doesn't line up with the lines around it"
		d.Hint = fmt.Sprintf("This line is indented %d space(s); the nearest level above uses %d. Align it with its siblings.", got, want)
		d.Range = p.cols(line, 1, got+2)
		return d
	}

	d.Range = p.cols(line, col, utf8.RuneCountInString(text)+1)
	return d
}

// cols builds a range on one line from 1-based rune columns.
func (p *parseState) cols(line, from, to int) Range {
	to = max(to, from+1)
	return Range{Start: p.li.at(line, from), End: p.li.at(line, to)}
}

// leading returns a line's indentation (spaces and tabs).
func leading(s string) string {
	return s[:len(s)-len(strings.TrimLeft(s, " \t"))]
}

func runeAt(s string, col int) rune {
	for i, r := range []rune(s) {
		if i == col-1 {
			return r
		}
	}
	return 0
}

// secondColon finds a ": " (or trailing ":") inside the value part of a
// "key: value" line and returns its 1-based column, or 0.
func secondColon(line string) int {
	body := strings.TrimLeft(line, " -")
	offset := len(line) - len(body)
	first := strings.Index(body, ": ")
	if first < 0 {
		return 0
	}
	value := body[first+2:]
	if strings.HasPrefix(strings.TrimSpace(value), `"`) || strings.HasPrefix(strings.TrimSpace(value), "'") {
		return 0 // already quoted; something else is wrong
	}
	if i := strings.Index(value, ": "); i >= 0 {
		return utf8.RuneCountInString(line[:offset+first+2+i]) + 1
	}
	if strings.HasSuffix(value, ":") {
		return utf8.RuneCountInString(line[:offset+first+2+len(value)-1]) + 1
	}
	return 0
}

// expectedIndent reports whether a line's indentation matches none of
// the levels used by earlier lines in the document, returning the
// nearest level below it (its likely parent or sibling).
func (p *parseState) expectedIndent(line int) (int, bool) {
	got := len(leading(p.lineIn(p.masked, line)))
	levels := map[int]bool{}
	for n := line - 1; n >= 1 && n > line-200; n-- {
		t := p.lineIn(p.masked, n)
		trimmed := strings.TrimSpace(t)
		if isMarker(t, "---") {
			break
		}
		if trimmed == "" || trimmed[0] == '#' {
			continue
		}
		ind := len(leading(t))
		levels[ind] = true
		// Items after "- " open a level at the item's content column.
		if strings.HasPrefix(trimmed, "- ") {
			levels[ind+2] = true
		}
	}
	if len(levels) == 0 || levels[got] {
		return 0, false
	}
	best := -1
	for l := range levels {
		if l < got && l > best {
			best = l
		}
	}
	if best < 0 {
		best = 0
	}
	return best, true
}

func capitalize(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}

// keyValue splits a block mapping line "  key: value  # c" into key and
// value (comment dropped). ok is false for lines that aren't entries.
func keyValue(line string) (key, value string, ok bool) {
	body := strings.TrimLeft(line, " -")
	i := strings.Index(body, ": ")
	if i <= 0 {
		return "", "", false
	}
	value = body[i+2:]
	if c := strings.Index(value, " #"); c >= 0 {
		value = value[:c]
	}
	return body[:i], strings.TrimSpace(value), true
}

// nextContentLine returns the next line after n that isn't blank or a
// comment, within the same document.
func (p *parseState) nextContentLine(n int) (int, bool) {
	for next := n + 1; next <= p.li.lines(); next++ {
		t := p.lineIn(p.masked, next)
		if isMarker(t, "---") {
			return 0, false
		}
		if tr := strings.TrimSpace(t); tr != "" && tr[0] != '#' {
			return next, true
		}
	}
	return 0, false
}
