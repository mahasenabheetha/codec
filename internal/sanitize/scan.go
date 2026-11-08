package sanitize

import (
	"strings"
)

// Hand-written scanners for the values found on most lines of a log
// (hosts, emails, addresses, UUIDs, key=value): a pass over the bytes is
// many times faster than a regular expression that starts with \b,
// which can't skip ahead to a literal. A 5 MB log is read in well under
// a second.

func isAlnum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}
func isWord(c byte) bool { return isAlnum(c) || c == '_' }
func isHex(c byte) bool  { return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' }

// runs calls f with each longest run of bytes that in accepts.
func runs(text string, in func(byte) bool, f func(s, e int)) {
	for i := 0; i < len(text); {
		if !in(text[i]) {
			i++
			continue
		}
		j := i
		for j < len(text) && in(text[j]) {
			j++
		}
		f(i, j)
		i = j
	}
}

// apart says whether [s, e) stands on its own: no letter, digit or _
// touching it on either side.
func apart(text string, s, e int) bool {
	return (s == 0 || !isWord(text[s-1])) && (e == len(text) || !isWord(text[e]))
}

// tlds are the top-level domains (and private ones) a host name is
// taken for, so file.go and fmt.Println aren't.
var tlds = set("com net org io dev app cloud ai co me info biz eu uk de fr nl se no dk fi ch at be es it pl in lk au nz jp cn us ca br " +
	"internal local lan corp intra intranet home svc")

func set(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(s) {
		m[w] = true
	}
	return m
}

// hosts calls f with each host name: labels of letters, digits and -,
// at least two, the last a known top-level domain.
func hosts(text string, f func(s, e int)) {
	runs(text, func(c byte) bool { return isAlnum(c) || c == '.' || c == '-' }, func(s, e int) {
		for s < e && (text[s] == '.' || text[s] == '-') {
			s++
		}
		for e > s && (text[e-1] == '.' || text[e-1] == '-') {
			e--
		}
		h := text[s:e]
		dot := strings.LastIndexByte(h, '.')
		if dot <= 0 || !tlds[strings.ToLower(h[dot+1:])] || (s > 0 && (text[s-1] == '@' || isWord(text[s-1]))) {
			return
		}
		for _, label := range strings.Split(h, ".") {
			if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return
			}
		}
		f(s, e)
	})
}

// emails calls f with each address: a local part, @, a domain with a dot
// and a top-level domain of letters.
func emails(text string, f func(s, e int)) {
	local := func(c byte) bool { return isAlnum(c) || strings.IndexByte("._%+-", c) >= 0 }
	for i := strings.IndexByte(text, '@'); i >= 0; {
		s, e := i, i+1
		for s > 0 && local(text[s-1]) {
			s--
		}
		for e < len(text) && (isAlnum(text[e]) || text[e] == '.' || text[e] == '-') {
			e++
		}
		for e > i+1 && text[e-1] == '.' {
			e--
		}
		domain := text[i+1 : e]
		dot := strings.LastIndexByte(domain, '.')
		tld := dot > 0 && len(domain)-dot > 2 && letters(domain[dot+1:])
		glued := s > 0 && text[s-1] == '@' || e < len(text) && text[e] == '@' // a@b@c is no address
		if s < i && tld && !glued && (e == len(text) || !isWord(text[e])) {
			f(s, e)
		}
		next := strings.IndexByte(text[i+1:], '@')
		if next < 0 {
			break
		}
		i += 1 + next
	}
}

func letters(s string) bool {
	for i := 0; i < len(s); i++ {
		if !(s[i] >= 'a' && s[i] <= 'z' || s[i] >= 'A' && s[i] <= 'Z') {
			return false
		}
	}
	return true
}

// ipv4s calls f with each dotted address: four parts of one to three
// digits, standing on its own.
func ipv4s(text string, f func(s, e int)) {
	runs(text, func(c byte) bool { return c >= '0' && c <= '9' || c == '.' }, func(s, e int) {
		parts := strings.Split(text[s:e], ".")
		if len(parts) != 4 || !apart(text, s, e) {
			return
		}
		for _, p := range parts {
			if p == "" || len(p) > 3 {
				return
			}
		}
		f(s, e)
	})
}

// ipv6s calls f with each run of hex digits and colons, with two colons
// at least, that stands on its own and isn't glued to a dot (192.0.2.1::0).
func ipv6s(text string, f func(s, e int)) {
	runs(text, func(c byte) bool { return isHex(c) || c == ':' }, func(s, e int) {
		if strings.Count(text[s:e], ":") < 2 || !apart(text, s, e) || s > 0 && text[s-1] == '.' || e < len(text) && text[e] == '.' {
			return
		}
		f(s, e)
	})
}

// uuids calls f with each 8-4-4-4-12 run of hex digits.
func uuids(text string, f func(s, e int)) {
	runs(text, func(c byte) bool { return isHex(c) || c == '-' }, func(s, e int) {
		u := text[s:e]
		if len(u) != 36 || !apart(text, s, e) {
			return
		}
		for i := 0; i < 36; i++ {
			dash := i == 8 || i == 13 || i == 18 || i == 23
			if dash != (u[i] == '-') {
				return
			}
		}
		f(s, e)
	})
}

// keyValues calls f with the value of each name=value whose name is a
// secret's (password=, token=, Password= in a connection string), the
// value inside its quotes when it has them. Each = is looked at on its
// own, so a token=… inside url=…?token=… is found too.
func keyValues(text string, f func(s, e int)) {
	key := func(c byte) bool { return isWord(c) || strings.IndexByte(".@$-", c) >= 0 }
	for i := strings.IndexByte(text, '='); i >= 0; {
		k := i
		for k > 0 && key(text[k-1]) {
			k--
		}
		if k < i && likelySecretName(text[k:i]) && secretName(text[k:i]) && i+1 < len(text) {
			s, e := i+1, i+1
			switch q := text[s]; q {
			case '"', '\'':
				e = s + 1
				for e < len(text) && text[e] != q {
					if text[e] == '\\' && q == '"' {
						e++
					}
					e++
				}
				s, e = s+1, min(e, len(text))
			default:
				for e < len(text) && strings.IndexByte(" \t\r\n&\";,?", text[e]) < 0 {
					e++
				}
			}
			if e > s && !placeholder(text[s:e]) && !schemeValue.MatchString(text[s:e]) {
				f(s, e)
			}
		}
		next := strings.IndexByte(text[i+1:], '=')
		if next < 0 {
			break
		}
		i += 1 + next
	}
}

// likelySecretName is a quick no for most names, before secretName
// splits them into words.
func likelySecretName(name string) bool {
	n := strings.ToLower(name)
	for _, w := range []string{"pass", "pw", "secret", "token", "key", "auth", "cookie", "cred", "session", "sig"} {
		if strings.Contains(n, w) {
			return true
		}
	}
	return false
}
