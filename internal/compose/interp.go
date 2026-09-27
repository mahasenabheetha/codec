package compose

import (
	"strings"
)

// Compose replaces ${VAR} in values before it merges files. The syntax
// (a subset of the shell's):
//
//	$$                 a literal $
//	$VAR  ${VAR}       the value, "" when unset (Compose warns)
//	${VAR:-default}    default when unset or empty
//	${VAR-default}     default when unset
//	${VAR:?message}    error when unset or empty
//	${VAR?message}     error when unset
//	${VAR:+other}      other when set and not empty, else ""
//	${VAR+other}       other when set, else ""
//
// Defaults may hold further ${…}.

// ref is one reference found in a string.
type ref struct {
	name       string
	op         string // "", ":-", "-", ":?", "?", ":+", "+"
	arg        string // default, message or replacement (raw)
	start, end int    // byte offsets of the whole $… text
}

// lookup returns a variable's value and whether it is set.
type lookup func(name string) (string, bool)

// expansion is what interpolating one string produced.
type expansion struct {
	text string
	refs []ref
	errs []string // ${VAR:?message} with VAR unset
}

// interpolate expands s. Every reference is reported, including those
// inside defaults that were used.
func interpolate(s string, get lookup) expansion {
	var ex expansion
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '$' || i+1 >= len(s) {
			b.WriteByte(s[i])
			i++
			continue
		}
		r, ok := parseRef(s, i)
		switch {
		case s[i+1] == '$':
			b.WriteByte('$')
			i += 2
			continue
		case !ok:
			b.WriteByte('$')
			i++
			continue
		}
		ex.refs = append(ex.refs, r)
		val, set := get(r.name)
		empty := !set || val == ""
		switch r.op {
		case ":-", "-":
			if (r.op == ":-" && empty) || (r.op == "-" && !set) {
				inner := interpolate(r.arg, get)
				ex.refs = append(ex.refs, shift(inner.refs, r.start+len(r.name)+2+len(r.op))...)
				ex.errs = append(ex.errs, inner.errs...)
				val = inner.text
			}
		case ":?", "?":
			if (r.op == ":?" && empty) || (r.op == "?" && !set) {
				msg := "required variable " + r.name + " is missing a value"
				if r.arg != "" {
					msg += ": " + r.arg
				}
				ex.errs = append(ex.errs, msg)
			}
		case ":+", "+":
			if (r.op == ":+" && !empty) || (r.op == "+" && set) {
				inner := interpolate(r.arg, get)
				ex.refs = append(ex.refs, shift(inner.refs, r.start+len(r.name)+2+len(r.op))...)
				val = inner.text
			} else {
				val = ""
			}
		}
		b.WriteString(val)
		i = r.end
	}
	ex.text = b.String()
	return ex
}

func shift(refs []ref, by int) []ref {
	for i := range refs {
		refs[i].start += by
		refs[i].end += by
	}
	return refs
}

// parseRef reads the reference starting at s[i] == '$'.
func parseRef(s string, i int) (ref, bool) {
	if i+1 >= len(s) {
		return ref{}, false
	}
	if s[i+1] != '{' {
		n := nameLen(s[i+1:])
		if n == 0 {
			return ref{}, false
		}
		return ref{name: s[i+1 : i+1+n], start: i, end: i + 1 + n}, true
	}
	// ${…}: find the matching brace; defaults may nest ${…}.
	depth, j := 1, i+2
	for ; j < len(s) && depth > 0; j++ {
		switch {
		case s[j] == '{' && s[j-1] == '$':
			depth++
		case s[j] == '}':
			depth--
		}
	}
	if depth > 0 {
		return ref{}, false
	}
	body := s[i+2 : j-1]
	n := nameLen(body)
	if n == 0 {
		return ref{}, false
	}
	r := ref{name: body[:n], start: i, end: j}
	rest := body[n:]
	for _, op := range []string{":-", ":?", ":+", "-", "?", "+"} {
		if strings.HasPrefix(rest, op) {
			r.op, r.arg = op, rest[len(op):]
			return r, true
		}
	}
	return r, rest == ""
}

// nameLen is the length of the variable name at the start of s.
func nameLen(s string) int {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && c >= '0' && c <= '9' {
			continue
		}
		return i
	}
	return len(s)
}

// scanRefs finds the references in a raw source line, ignoring a
// trailing comment, for tinting and hover in the editor. Offsets are
// bytes into line.
func scanRefs(line string) []ref {
	var out []ref
	var quote byte
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '#' && (i == 0 || line[i-1] == ' ' || line[i-1] == '\t'):
			return out
		}
		if c != '$' {
			continue
		}
		if i+1 < len(line) && line[i+1] == '$' {
			i++
			continue
		}
		if r, ok := parseRef(line, i); ok {
			out = append(out, r)
			i = r.end - 1
		}
	}
	return out
}

// --- .env files ---

// envEntry is one variable set in an env file.
type envEntry struct {
	name, value string
	line        int
}

// parseEnvFile reads a .env file the way Compose does: KEY=VALUE lines,
// optional "export ", # comments, single quotes literal, double quotes
// with \n escapes, and ${VAR} in unquoted and double-quoted values
// expanded from earlier entries (and get, for what-if values).
func parseEnvFile(content string, get lookup) []envEntry {
	var out []envEntry
	seen := map[string]string{}
	for i, raw := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || line[0] == '#' {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		k, v, ok := strings.Cut(line, "=")
		k = strings.TrimSpace(k)
		if !ok || k == "" || nameLen(k) != len(k) {
			continue // a bare KEY takes the shell's value, which codec doesn't read
		}
		v = strings.TrimSpace(v)
		expand := func(s string) string {
			return interpolate(s, func(n string) (string, bool) {
				if x, ok := seen[n]; ok {
					return x, true
				}
				return get(n)
			}).text
		}
		switch {
		case len(v) >= 2 && v[0] == '\'' && strings.IndexByte(v[1:], '\'') >= 0:
			v = v[1 : 1+strings.IndexByte(v[1:], '\'')]
		case len(v) >= 2 && v[0] == '"':
			end := closingQuote(v)
			v = strings.NewReplacer(`\n`, "\n", `\t`, "\t", `\"`, `"`, `\\`, `\`).Replace(v[1:end])
			v = expand(v)
		default:
			if c := strings.Index(v, " #"); c >= 0 {
				v = strings.TrimSpace(v[:c])
			}
			v = expand(v)
		}
		seen[k] = v
		out = append(out, envEntry{name: k, value: v, line: i + 1})
	}
	return out
}

// closingQuote is the index of the " closing v's opening one (or the
// end of v when unclosed).
func closingQuote(v string) int {
	for j := 1; j < len(v); j++ {
		if v[j] == '\\' {
			j++
			continue
		}
		if v[j] == '"' {
			return j
		}
	}
	return len(v)
}
