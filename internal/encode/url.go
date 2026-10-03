// Package encode holds the small encode, hash and generate jobs of the
// Encode & hash tool: URL and hex encoding, digests and HMAC, random
// secrets and UUIDs, and htpasswd lines. Like internal/codec it only
// transforms data; nothing is stored.
package encode

import (
	"fmt"
	"net/url"
	"strings"
)

const upperHex = "0123456789ABCDEF"

// unreserved reports the characters RFC 3986 never escapes.
func unreserved(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9' || c == '-' || c == '_' || c == '.' || c == '~'
}

// URLEncode percent-encodes s. As a value (whole false) everything but
// the unreserved characters is escaped, like encodeURIComponent; with
// plus, spaces become "+" as in HTML forms. As a whole URL the
// characters that give a URL its structure (":/?#[]@!$&'()*+,;=") and
// existing %XX escapes are kept, like encodeURI.
func URLEncode(s string, whole, plus bool) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case unreserved(c):
			b.WriteByte(c)
		case whole && strings.IndexByte(":/?#[]@!$&'()*+,;=", c) >= 0:
			b.WriteByte(c)
		case whole && c == '%' && i+2 < len(s) && ishex(s[i+1]) && ishex(s[i+2]):
			b.WriteByte(c)
		case plus && c == ' ':
			b.WriteByte('+')
		default:
			b.WriteByte('%')
			b.WriteByte(upperHex[c>>4])
			b.WriteByte(upperHex[c&15])
		}
	}
	return b.String()
}

// URLDecode reverses percent-encoding; with plus, "+" is a space. A
// "%" not followed by two hex digits is an error naming its position.
func URLDecode(s string, plus bool) (string, error) {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '%':
			if i+2 >= len(s) || !ishex(s[i+1]) || !ishex(s[i+2]) {
				return "", fmt.Errorf("invalid escape %q at character %d", s[i:min(i+3, len(s))], i+1)
			}
			b.WriteByte(unhex(s[i+1])<<4 | unhex(s[i+2]))
			i += 2
		case c == '+' && plus:
			b.WriteByte(' ')
		default:
			b.WriteByte(c)
		}
	}
	return b.String(), nil
}

// URLParts is a URL taken apart, its query as decoded pairs in order.
// A bare query string ("a=1&b=2", "?a=1") fills only Query.
type URLParts struct {
	Scheme   string  `json:"scheme,omitempty"`
	User     string  `json:"user,omitempty"`
	Host     string  `json:"host,omitempty"`
	Port     string  `json:"port,omitempty"`
	Path     string  `json:"path,omitempty"`
	Query    []Param `json:"query"`
	Fragment string  `json:"fragment,omitempty"`
}

// Param is one key=value pair of a query string. Keys may repeat.
type Param struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ParseURL takes s apart. The password in a URL's user info is never
// returned; only the user name is.
func ParseURL(s string) (*URLParts, error) {
	s = strings.TrimSpace(s)
	if !strings.Contains(s, "://") && !strings.HasPrefix(s, "/") && (strings.Contains(s, "=") || strings.HasPrefix(s, "?")) {
		q, err := ParseQuery(strings.TrimPrefix(s, "?"))
		if err != nil {
			return nil, err
		}
		return &URLParts{Query: q}, nil
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("not a URL: %w", err)
	}
	p := &URLParts{Scheme: u.Scheme, Host: u.Hostname(), Port: u.Port(), Path: u.Path, Fragment: u.Fragment}
	if u.User != nil {
		p.User = u.User.Username()
	}
	if p.Query, err = ParseQuery(u.RawQuery); err != nil {
		return nil, err
	}
	return p, nil
}

// ParseQuery splits a query string into decoded pairs, keeping their
// order and repeats ("+" is a space, as browsers send it).
func ParseQuery(q string) ([]Param, error) {
	out := []Param{}
	for part := range strings.SplitSeq(q, "&") {
		if part == "" {
			continue
		}
		k, v, _ := strings.Cut(part, "=")
		dk, err := URLDecode(k, true)
		if err != nil {
			return nil, fmt.Errorf("query key %q: %w", k, err)
		}
		dv, err := URLDecode(v, true)
		if err != nil {
			return nil, fmt.Errorf("query value of %q: %w", dk, err)
		}
		out = append(out, Param{Key: dk, Value: dv})
	}
	return out, nil
}

func ishex(c byte) bool {
	return '0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F'
}

func unhex(c byte) byte {
	switch {
	case c <= '9':
		return c - '0'
	case c <= 'F':
		return c - 'A' + 10
	}
	return c - 'a' + 10
}
