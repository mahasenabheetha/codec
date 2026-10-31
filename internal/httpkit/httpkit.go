// Package httpkit is the engine of the Network tool's HTTP tab: a curl
// command read as a shell would and turned into a fetch call, a fetch
// call turned back into curl, and what an HTTP status code means. It
// only rewrites text; no request is ever sent and nothing is stored.
package httpkit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Request is an HTTP request as both curl and fetch can say it.
type Request struct {
	Method  string   `json:"method"`
	URL     string   `json:"url"`
	Headers []Header `json:"headers"`
	Body    string   `json:"body,omitempty"`
	// Form is a multipart body (curl -F, fetch FormData); a request has
	// a Body or a Form, never both.
	Form      []Field `json:"form,omitempty"`
	TimeoutMS int     `json:"timeoutMs,omitempty"`
	// Notes say what didn't carry over and why, in words.
	Notes []string `json:"notes"`
}

// Header is one request header; names keep the case they were given.
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Field is one multipart form field. A File field names a file to
// attach, which neither side can carry as text.
type Field struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	File  bool   `json:"file,omitempty"`
}

// header is the value of the first header called name, any case.
func (r *Request) header(name string) (string, bool) {
	for _, h := range r.Headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value, true
		}
	}
	return "", false
}

func (r *Request) setDefault(name, value string) {
	if _, ok := r.header(name); !ok {
		r.Headers = append(r.Headers, Header{name, value})
	}
}

func (r *Request) note(s string) {
	for _, n := range r.Notes {
		if n == s {
			return
		}
	}
	r.Notes = append(r.Notes, s)
}

// jsonBody reports a body that is JSON sent as JSON, so fetch can show
// it as JSON.stringify of the value.
func (r *Request) jsonBody() bool {
	ct, _ := r.header("Content-Type")
	return r.Body != "" && strings.Contains(strings.ToLower(ct), "json") && json.Valid([]byte(r.Body))
}

// jsString quotes s as a JavaScript string (JSON's rules, without the
// HTML escapes).
func jsString(s string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.Encode(s)
	return strings.TrimSuffix(b.String(), "\n")
}

// shellQuote quotes s for a POSIX shell in single quotes; a quote inside
// ends the string, adds an escaped quote and starts a new one.
func shellQuote(s string) string {
	if strings.IndexFunc(s, func(r rune) bool { return r < 0x20 && r != '\n' && r != '\t' || r == 0x7f }) < 0 {
		return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	}
	// Control characters (a carriage return) need $'…' to survive a paste.
	var b strings.Builder
	b.WriteString("$'")
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\\' || c == '\'':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c == '\n':
			b.WriteString(`\n`)
		case c == '\r':
			b.WriteString(`\r`)
		case c == '\t':
			b.WriteString(`\t`)
		case c < 0x20 || c == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, c)
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('\'')
	return b.String()
}
