package yamlkit

import (
	"bytes"
	"sort"
	"strings"
)

// exprSpan is an expression's byte range in the source, before it is
// turned into a public Expression.
type exprSpan struct {
	start, end int
	syntax     string
	phase      Phase
	standalone bool
}

// Argo Workflows resolves {{…}} at runtime with these variable roots;
// anything else in double braces is treated as a template (Helm/Jinja).
var argoPrefixes = []string{
	"workflow.", "inputs.", "outputs.", "steps.", "tasks.", "item",
	"pod.", "node.", "retries", "lastRetry", "cronworkflow.", "=",
}

// findExpressions locates template/runtime expressions: {{ … }},
// ${{ … }}, {% … %} and {# … #}. Quotes inside an expression are
// respected, so {{ "}}" }} is one expression. Unclosed openers are
// ignored — the YAML parser will report whatever they break.
func findExpressions(src []byte) []exprSpan {
	jinja := bytes.Contains(src, []byte("{%"))
	var spans []exprSpan
	for i := 0; i+1 < len(src); i++ {
		if src[i] != '{' {
			continue
		}
		var closer string
		switch src[i+1] {
		case '{':
			closer = "}}"
		case '%':
			closer = "%}"
		case '#':
			closer = "#}"
		default:
			continue
		}
		from := i + 2
		// Go template comments ({{/* … */}}) may contain stray quotes
		// ('don't'), so jump past the comment body first.
		if closer == "}}" {
			if body := bytes.TrimLeft(src[from:], "- "); bytes.HasPrefix(body, []byte("/*")) {
				if c := bytes.Index(body, []byte("*/")); c >= 0 {
					from = len(src) - len(body) + c + 2
				}
			}
		}
		end := findCloser(src, from, closer)
		if end < 0 {
			continue
		}
		s := exprSpan{start: i, end: end, phase: PhaseTemplate}
		switch {
		case closer != "}}":
			s.syntax = "jinja"
		case i > 0 && src[i-1] == '$':
			s.start, s.syntax, s.phase = i-1, "github", PhaseRuntime
		case isArgo(src[i+2 : end-2]):
			s.syntax, s.phase = "argo", PhaseRuntime
		case jinja:
			s.syntax = "jinja"
		default:
			s.syntax = "go-template"
		}
		spans = append(spans, s)
		i = end - 1
	}
	markStandalone(src, spans)
	return spans
}

// findCloser returns the offset just past closer, skipping quoted
// strings, or -1 if there is none.
func findCloser(src []byte, from int, closer string) int {
	var quote byte
	for j := from; j+1 < len(src); j++ {
		c := src[j]
		if quote != 0 {
			if c == '\\' && quote != '`' {
				j++
			} else if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'', '`':
			quote = c
		case closer[0]:
			if src[j+1] == closer[1] {
				return j + 2
			}
		}
	}
	return -1
}

func isArgo(inner []byte) bool {
	s := strings.TrimSpace(strings.Trim(strings.TrimSpace(string(inner)), "-"))
	for _, p := range argoPrefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// markStandalone flags expressions that are the only thing on their
// line(s), like {{- if .Values.x }} — template control flow rather
// than part of a YAML value.
func markStandalone(src []byte, spans []exprSpan) {
	for i := range spans {
		ls := bytes.LastIndexByte(src[:spans[i].start], '\n') + 1
		le := bytes.IndexByte(src[spans[i].end:], '\n')
		if le < 0 {
			le = len(src)
		} else {
			le += spans[i].end
		}
		// Everything else on the line must be whitespace or other
		// expressions for this one to count as standalone.
		ok := true
		for j := ls; j < le && ok; {
			if k := spanAt(spans, j); k >= 0 {
				j = spans[k].end
				continue
			}
			if c := src[j]; c != ' ' && c != '\t' && c != '\r' {
				ok = false
			}
			j++
		}
		spans[i].standalone = ok
	}
}

// spanAt returns the index of the span containing off, or -1. Spans are
// sorted and non-overlapping, so a binary search is enough.
func spanAt(spans []exprSpan, off int) int {
	i := sort.Search(len(spans), func(i int) bool { return spans[i].end > off })
	if i < len(spans) && spans[i].start <= off {
		return i
	}
	return -1
}

// mask returns a copy of src with every expression replaced by text of
// the same byte length, so the result parses as YAML and all offsets
// stay valid. Newlines are kept.
//
// Inline expressions become a run of 'x', so the value they sit in stays
// a plain scalar. Standalone ones (control flow like {{- if … }}) become
// "#" followed by spaces on each of their lines: outside a block scalar
// that is a YAML comment, a no-op; inside one (script: |) it is harmless
// text. Plain spaces would not do: whitespace-only lines deeper than a
// block's content are invalid YAML, which broke Jinja-heavy playbooks.
func mask(src []byte, spans []exprSpan) []byte {
	if len(spans) == 0 {
		return src
	}
	out := bytes.Clone(src)
	for _, s := range spans {
		lineStart := true // at the first non-blank char of a line in this span
		for j := s.start; j < s.end; j++ {
			switch c := out[j]; {
			case c == '\n' || c == '\r':
				lineStart = true
			case !s.standalone:
				out[j] = 'x'
			case lineStart && c != ' ' && c != '\t':
				out[j] = '#'
				lineStart = false
			case !lineStart:
				out[j] = ' '
			}
		}
	}
	return out
}
