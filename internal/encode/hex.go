package encode

import (
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"
)

// HexEncode writes data as hex digits, lower case unless upper, with
// sep between bytes ("", " " or ":" are the useful ones).
func HexEncode(data []byte, upper bool, sep string) string {
	digits := "0123456789abcdef"
	if upper {
		digits = upperHex
	}
	var b strings.Builder
	b.Grow(len(data) * (2 + len(sep)))
	for i, c := range data {
		if i > 0 {
			b.WriteString(sep)
		}
		b.WriteByte(digits[c>>4])
		b.WriteByte(digits[c&15])
	}
	return b.String()
}

// HexDecode reads hex as tools print it: in either case, with or
// without whitespace, ":" or "-" between bytes, and "0x" or "\x" before
// them. An odd digit count or another character is an error.
func HexDecode(s string) ([]byte, error) {
	var digits strings.Builder
	digits.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case ishex(c):
			// "0x" and "\x" prefixes: drop the marker, keep the digits.
			if c == '0' && i+1 < len(s) && (s[i+1] == 'x' || s[i+1] == 'X') && (i == 0 || !ishex(s[i-1])) {
				i++
				continue
			}
			digits.WriteByte(c)
		case c == '\\' && i+1 < len(s) && s[i+1] == 'x':
			i++
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == ':' || c == '-' || c == ',':
		default:
			r, _ := utf8.DecodeRuneInString(s[i:])
			return nil, fmt.Errorf("%q at character %d is not a hex digit", r, utf8.RuneCountInString(s[:i])+1)
		}
	}
	if digits.Len()%2 == 1 {
		return nil, fmt.Errorf("odd number of hex digits (%d): a byte is two digits", digits.Len())
	}
	return hex.DecodeString(digits.String())
}

// Printable renders bytes as text: valid UTF-8 as is, and anything else
// as \xNN escapes so binary data stays visible. ok says the bytes were
// text.
func Printable(data []byte) (s string, ok bool) {
	if utf8.Valid(data) {
		return string(data), true
	}
	var b strings.Builder
	for len(data) > 0 {
		r, n := utf8.DecodeRune(data)
		if r == utf8.RuneError && n == 1 {
			fmt.Fprintf(&b, `\x%02x`, data[0])
		} else {
			b.Write(data[:n])
		}
		data = data[n:]
	}
	return b.String(), false
}
