package httpkit

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// lineEnd matches a cmd (^) or PowerShell (`) line continuation; a
// backslash one is handled while splitting, as the shell does.
var lineEnd = regexp.MustCompile("[ \t][\\^`][ \t]*\r?\n")

// splitShell splits a command line into words as a POSIX shell would:
// quotes, $'…', backslashes and line continuations, with nothing
// expanded ($VAR stays as written). It stops at a pipe, ; or &, and
// says so in a note.
func splitShell(s string) (words, notes []string, err error) {
	// A pasted carriage return is a Windows line ending ($'\r' still
	// makes one on purpose).
	s = strings.ReplaceAll(s, "\r", "")
	s = lineEnd.ReplaceAllString(s+"\n", " ")
	var cur strings.Builder
	inWord := false
	end := func() {
		if inWord {
			words = append(words, cur.String())
		}
		cur.Reset()
		inWord = false
	}
	variable := func(i int) {
		if i+1 < len(s) && (s[i+1] == '{' || s[i+1] == '_' || isLetter(s[i+1])) {
			notes = appendOnce(notes, "Shell variables ($NAME) are kept as written, not expanded.")
		}
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n':
			end()
		case c == '#' && !inWord:
			for i < len(s) && s[i] != '\n' {
				i++
			}
		case c == '|' || c == ';' || c == '&':
			end()
			if rest := strings.TrimSpace(s[i:]); rest != "" {
				notes = appendOnce(notes, "Only the curl command is read; "+strconv.Quote(firstLine(rest))+" and after is left out.")
			}
			return words, notes, nil
		case c == '\\':
			inWord = true
			if i+1 < len(s) {
				i++
				if s[i] != '\n' {
					cur.WriteByte(s[i])
				} else if cur.Len() == 0 {
					inWord = false // a continuation between words
				}
			}
		case c == '\'':
			inWord = true
			j := strings.IndexByte(s[i+1:], '\'')
			if j < 0 {
				return nil, nil, errors.New("a ' quote is never closed")
			}
			cur.WriteString(s[i+1 : i+1+j])
			i += j + 1
		case c == '$' && i+1 < len(s) && s[i+1] == '\'':
			inWord = true
			n, err := ansiC(s[i+2:], &cur)
			if err != nil {
				return nil, nil, err
			}
			i += 1 + n
		case c == '"':
			inWord = true
			i++
			for ; i < len(s) && s[i] != '"'; i++ {
				if s[i] == '\\' && i+1 < len(s) && strings.IndexByte("$`\"\\\n", s[i+1]) >= 0 {
					i++
					if s[i] != '\n' {
						cur.WriteByte(s[i])
					}
					continue
				}
				if s[i] == '$' {
					variable(i)
				}
				cur.WriteByte(s[i])
			}
			if i >= len(s) {
				return nil, nil, errors.New(`a " quote is never closed`)
			}
		default:
			if c == '$' {
				variable(i)
			}
			inWord = true
			cur.WriteByte(c)
		}
	}
	end()
	return words, notes, nil
}

// ansiC reads the body of a $'…' string up to its closing quote into
// out and returns how many bytes it took, the quote included.
func ansiC(s string, out *strings.Builder) (int, error) {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\'' {
			return i + 1, nil
		}
		if c != '\\' || i+1 >= len(s) {
			out.WriteByte(c)
			continue
		}
		i++
		switch e := s[i]; e {
		case 'n':
			out.WriteByte('\n')
		case 't':
			out.WriteByte('\t')
		case 'r':
			out.WriteByte('\r')
		case 'a':
			out.WriteByte('\a')
		case 'b':
			out.WriteByte('\b')
		case 'e', 'E':
			out.WriteByte(0x1b)
		case 'f':
			out.WriteByte('\f')
		case 'v':
			out.WriteByte('\v')
		case 'x', 'u', 'U':
			width := map[byte]int{'x': 2, 'u': 4, 'U': 8}[e]
			j := i + 1
			for j < len(s) && j < i+1+width && isHex(s[j]) {
				j++
			}
			if j == i+1 {
				out.WriteByte('\\')
				out.WriteByte(e)
				continue
			}
			n, _ := strconv.ParseUint(s[i+1:j], 16, 32)
			if e == 'x' {
				out.WriteByte(byte(n))
			} else if utf8.ValidRune(rune(n)) {
				out.WriteRune(rune(n))
			}
			i = j - 1
		default: // \\ \' \" \? and anything else: the character itself
			out.WriteByte(e)
		}
	}
	return 0, errors.New("a $' quote is never closed")
}

func isLetter(c byte) bool { return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' }

func isHex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(s, "\n")
	if len(s) > 40 {
		s = s[:40] + "…"
	}
	return s
}

func appendOnce(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}
