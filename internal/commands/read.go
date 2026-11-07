// Package commands explains and builds command lines (decision 104): it
// reads a line as a POSIX shell would split it, matches each command to
// a reference of tools, subcommands, flags and arguments, and says what
// every part does. The reference is codec's own words, embedded in the
// binary; this is a pure engine package that runs nothing.
package commands

import (
	"strconv"
	"strings"
)

// TokenKind is what a token of a command line is.
type TokenKind uint8

const (
	Word     TokenKind = iota // a word, after quotes and escapes: a command, flag, value or argument
	Op                        // |, |&, ||, &&, ;, & or a line break between commands
	Redirect                  // >, >>, <, <<, <<<, 2>, 2>&1, &>, …
	Comment                   // # and the rest of the line
)

// Token is one part of a command line.
type Token struct {
	Kind TokenKind
	// Text is the word as the command sees it: quotes removed, escapes
	// undone. For an operator or redirection it is the operator itself.
	Text string
	// Start and End are the token's byte offsets in the line, quotes
	// included.
	Start, End int
	Quoted     bool // part of it was quoted
	Unclosed   bool // a quote or $( … ) was never closed
}

// Raw is the token as written in line.
func (t Token) Raw(line string) string { return line[t.Start:t.End] }

// Read splits a line into tokens the way bash does for a simple command
// line: words, quotes ('…', "…", $'…'), backslash escapes, command and
// parameter substitutions kept inside their word ($( … ), ` … `, ${ … }),
// operators, redirections and comments. It never fails: an unclosed
// quote runs to the end of the line.
func Read(line string) []Token {
	var out []Token
	i := 0
	for i < len(line) {
		c := line[i]
		switch {
		case c == '\n':
			out = append(out, Token{Kind: Op, Text: "\n", Start: i, End: i + 1})
			i++
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '\\' && i+1 < len(line) && line[i+1] == '\n':
			i += 2 // a line continued
		case c == '#':
			end := strings.IndexByte(line[i:], '\n')
			if end < 0 {
				end = len(line) - i
			}
			out = append(out, Token{Kind: Comment, Text: line[i : i+end], Start: i, End: i + end})
			i += end
		default:
			if t, n := operator(line, i); n > 0 {
				out = append(out, t)
				i += n
				continue
			}
			t := word(line, i)
			out = append(out, t)
			i = t.End
		}
	}
	return out
}

// operator reads an operator or a redirection at i; n is 0 when there is
// none. A redirection may start with a file descriptor (2>, 2>&1).
func operator(line string, i int) (Token, int) {
	rest := line[i:]
	j := 0
	for j < len(rest) && rest[j] >= '0' && rest[j] <= '9' {
		j++
	}
	if j > 0 && (j == len(rest) || rest[j] != '>' && rest[j] != '<') {
		return Token{}, 0 // a number, not a descriptor
	}
	for _, op := range []string{"&>>", "&>", ">>", ">&", ">|", "<<<", "<<-", "<<", "<&", "<>", ">", "<"} {
		if strings.HasPrefix(rest[j:], op) {
			n := j + len(op)
			// "2>&1": the descriptor it is sent to belongs to the operator.
			if (op == ">&" || op == "<&") && n < len(rest) && (rest[n] >= '0' && rest[n] <= '9' || rest[n] == '-') {
				n++
			}
			return Token{Kind: Redirect, Text: rest[:n], Start: i, End: i + n}, n
		}
	}
	if j > 0 {
		return Token{}, 0
	}
	for _, op := range []string{"||", "&&", "|&", ";;", "|", ";", "&"} {
		if strings.HasPrefix(rest, op) {
			return Token{Kind: Op, Text: op, Start: i, End: i + len(op)}, len(op)
		}
	}
	return Token{}, 0
}

// word reads one word starting at i.
func word(line string, i int) Token {
	t := Token{Kind: Word, Start: i}
	var b strings.Builder
	for i < len(line) {
		c := line[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			t.End, t.Text = i, b.String()
			return t
		case (c == '|' || c == '&' || c == ';' || c == '<' || c == '>') && i > t.Start:
			// An operator ends the word ("a|b"); at a word's start Read has
			// taken it as an operator already.
			t.End, t.Text = i, b.String()
			return t
		case c == '\\':
			if i+1 < len(line) {
				if line[i+1] != '\n' {
					b.WriteByte(line[i+1])
				}
				i += 2
			} else {
				i++
			}
		case c == '\'':
			t.Quoted = true
			end := strings.IndexByte(line[i+1:], '\'')
			if end < 0 {
				b.WriteString(line[i+1:])
				t.Unclosed, i = true, len(line)
				continue
			}
			b.WriteString(line[i+1 : i+1+end])
			i += end + 2
		case c == '$' && i+1 < len(line) && line[i+1] == '\'':
			t.Quoted = true
			i = ansiC(line, i+2, &b, &t)
		case c == '"':
			t.Quoted = true
			i = doubleQuoted(line, i+1, &b, &t)
		case c == '$' && i+1 < len(line) && (line[i+1] == '(' || line[i+1] == '{'):
			i = nested(line, i, &b, &t)
		case c == '`':
			end := strings.IndexByte(line[i+1:], '`')
			if end < 0 {
				b.WriteString(line[i:])
				t.Unclosed, i = true, len(line)
				continue
			}
			b.WriteString(line[i : i+end+2])
			i += end + 2
		default:
			b.WriteByte(c)
			i++
		}
	}
	t.End, t.Text = i, b.String()
	return t
}

// doubleQuoted reads "…" from just after its quote: only \", \\, \$, \`
// and a line break are escapes; substitutions stay as written.
func doubleQuoted(line string, i int, b *strings.Builder, t *Token) int {
	for i < len(line) {
		c := line[i]
		switch {
		case c == '"':
			return i + 1
		case c == '\\' && i+1 < len(line) && strings.IndexByte("\"\\$`\n", line[i+1]) >= 0:
			if line[i+1] != '\n' {
				b.WriteByte(line[i+1])
			}
			i += 2
		case c == '$' && i+1 < len(line) && (line[i+1] == '(' || line[i+1] == '{'):
			i = nested(line, i, b, t)
		default:
			b.WriteByte(c)
			i++
		}
	}
	t.Unclosed = true
	return i
}

// ansiC reads $'…' from just after its quote, with C escapes.
func ansiC(line string, i int, b *strings.Builder, t *Token) int {
	esc := map[byte]byte{'n': '\n', 't': '\t', 'r': '\r', '\\': '\\', '\'': '\'', '"': '"', 'a': '\a', 'b': '\b', 'e': 0x1b, 'f': '\f', 'v': '\v', '0': 0}
	for i < len(line) {
		c := line[i]
		switch {
		case c == '\'':
			return i + 1
		case c == '\\' && i+1 < len(line):
			// \xHH, \uHHHH, \UHHHHHHHH and octal \nnn, as bash reads them.
			if base, most, skip := numericEscape(line[i+1]); most > 0 {
				j := i + 1 + skip
				for j < len(line) && j-(i+1+skip) < most && isDigit(line[j], base) {
					j++
				}
				if j > i+1+skip {
					n, _ := strconv.ParseUint(line[i+1+skip:j], base, 32)
					if line[i+1] == 'u' || line[i+1] == 'U' {
						b.WriteRune(rune(n))
					} else {
						b.WriteByte(byte(n))
					}
					i = j
					continue
				}
			}
			if r, ok := esc[line[i+1]]; ok {
				b.WriteByte(r)
			} else {
				b.WriteByte('\\')
				b.WriteByte(line[i+1])
			}
			i += 2
		default:
			b.WriteByte(c)
			i++
		}
	}
	t.Unclosed = true
	return i
}

// nested keeps $( … ) or ${ … } as written, up to its closing bracket,
// quotes inside counted out.
func nested(line string, i int, b *strings.Builder, t *Token) int {
	open, close := line[i+1], byte(')')
	if open == '{' {
		close = '}'
	}
	depth, j := 0, i+1
	for j < len(line) {
		switch c := line[j]; {
		case c == '\\':
			j++
		case c == '\'':
			if end := strings.IndexByte(line[j+1:], '\''); end >= 0 {
				j += end + 1
			}
		case c == open:
			depth++
		case c == close:
			if depth--; depth == 0 {
				b.WriteString(line[i : j+1])
				return j + 1
			}
		}
		j++
	}
	b.WriteString(line[i:])
	t.Unclosed = true
	return len(line)
}

// numericEscape says how a $'…' escape that starts with c is read: in
// what base, at most how many digits, and how many letters come before
// them (x, u, U; none for octal).
func numericEscape(c byte) (base, most, skip int) {
	switch {
	case c == 'x':
		return 16, 2, 1
	case c == 'u':
		return 16, 4, 1
	case c == 'U':
		return 16, 8, 1
	case c >= '0' && c <= '7':
		return 8, 3, 0
	}
	return 0, 0, 0
}

func isDigit(c byte, base int) bool {
	if base == 8 {
		return c >= '0' && c <= '7'
	}
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}
