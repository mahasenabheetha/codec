package names

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Words splits an identifier or phrase into its words: at separators
// (space, _ - . / : and the like) and at case changes, keeping acronyms
// whole and digits with the letters before them: "getHTTPResponse" is
// get, HTTP, Response; "ipv6Address" is ipv6, Address.
func Words(s string) []string {
	var words []string
	for _, tok := range strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		rs := []rune(tok)
		start := 0
		for i := 1; i < len(rs); i++ {
			prev, cur := rs[i-1], rs[i]
			split := unicode.IsUpper(cur) && (unicode.IsLower(prev) || unicode.IsDigit(prev)) || // aB, 2B
				unicode.IsUpper(prev) && unicode.IsUpper(cur) && i+1 < len(rs) && unicode.IsLower(rs[i+1]) // HTTPServer
			if split {
				words = append(words, string(rs[start:i]))
				start = i
			}
		}
		words = append(words, string(rs[start:]))
	}
	return words
}

// Case is one way of writing the words.
type Case struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Cases writes s in every common case. Acronyms (HTTP, ID) stay upper
// case in camel, Pascal and Title case, unless the whole input was
// upper case (CONSTANT_CASE has no acronyms to keep).
func Cases(s string) []Case {
	ws := Words(s)
	keep := hasLower(s)
	acronym := func(w string) bool {
		return keep && utf8.RuneCountInString(w) > 1 && w == strings.ToUpper(w) && hasLetter(w)
	}
	lower := make([]string, len(ws))
	upper := make([]string, len(ws))
	cap := make([]string, len(ws))
	for i, w := range ws {
		lower[i], upper[i] = strings.ToLower(w), strings.ToUpper(w)
		cap[i] = title(w)
		if acronym(w) {
			cap[i] = w
		}
	}
	camel := append([]string{}, cap...)
	if len(camel) > 0 {
		camel[0] = lower[0]
	}
	sentence := append([]string{}, lower...)
	for i, w := range ws {
		if acronym(w) {
			sentence[i] = w
		}
	}
	if len(sentence) > 0 {
		sentence[0] = cap[0]
	}
	return []Case{
		{"camelCase", strings.Join(camel, "")},
		{"PascalCase", strings.Join(cap, "")},
		{"snake_case", strings.Join(lower, "_")},
		{"kebab-case", strings.Join(lower, "-")},
		{"CONSTANT_CASE", strings.Join(upper, "_")},
		{"dot.case", strings.Join(lower, ".")},
		{"Title Case", strings.Join(cap, " ")},
		{"Sentence case", strings.Join(sentence, " ")},
		{"lower case", strings.Join(lower, " ")},
		{"path/case", strings.Join(lower, "/")},
	}
}

// title is w with its first letter upper case and the rest lower.
func title(w string) string {
	r, n := utf8.DecodeRuneInString(w)
	return string(unicode.ToUpper(r)) + strings.ToLower(w[n:])
}

func hasLower(s string) bool {
	return strings.IndexFunc(s, unicode.IsLower) >= 0
}

func hasLetter(s string) bool {
	return strings.IndexFunc(s, unicode.IsLetter) >= 0
}

// Check is a name tested against one naming rule.
type Check struct {
	Rule     string   `json:"rule"`
	For      string   `json:"for"` // where the rule applies
	Valid    bool     `json:"valid"`
	Problems []string `json:"problems"`
	// Fixed is the nearest name that passes, made from the words; ""
	// when the name already passes or nothing usable is left.
	Fixed string `json:"fixed,omitempty"`
}

var (
	dnsLabel     = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	dnsSubdomain = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	labelValue   = regexp.MustCompile(`^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$`)
	envName      = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// Checks tests s against the naming rules of Kubernetes, environment
// variables and Helm, saying what breaks each and how it would read.
func Checks(s string) []Check {
	ws := Words(s)
	kebab := ascii(strings.ToLower(strings.Join(ws, "-")), "abcdefghijklmnopqrstuvwxyz0123456789-")
	constant := ascii(strings.ToUpper(strings.Join(ws, "_")), "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_")
	if constant != "" && constant[0] >= '0' && constant[0] <= '9' {
		constant = "_" + constant
	}
	return []Check{
		dns("Kubernetes name (DNS-1123 label)", "Services, namespaces, pods' container names, most names a resource's DNS name comes from", s, 63, dnsLabel, false, trimDNS(kebab, 63)),
		dns("Kubernetes name (DNS-1123 subdomain)", "ConfigMaps, Secrets, Deployments and most other resources", s, 253, dnsSubdomain, true, trimDNS(kebab, 253)),
		label(s),
		env(s, constant),
		dns("Helm release name", "helm install <name>: a DNS-1123 subdomain of at most 53 characters", s, 53, dnsSubdomain, true, trimDNS(kebab, 53)),
	}
}

func dns(rule, where, s string, max int, re *regexp.Regexp, dots bool, fixed string) Check {
	c := Check{Rule: rule, For: where, Problems: []string{}}
	switch {
	case s == "":
		c.Problems = append(c.Problems, "It is empty.")
	default:
		if n := utf8.RuneCountInString(s); n > max {
			c.Problems = append(c.Problems, fmt.Sprintf("It is %d characters long; at most %d.", n, max))
		}
		if strings.ToLower(s) != s {
			c.Problems = append(c.Problems, "It has upper-case letters; only lower case is allowed.")
		}
		allowed := "lower-case letters, digits and -"
		if dots {
			allowed = "lower-case letters, digits, - and ."
		}
		if bad := others(strings.ToLower(s), "abcdefghijklmnopqrstuvwxyz0123456789-"+map[bool]string{true: ".", false: ""}[dots]); bad != "" {
			c.Problems = append(c.Problems, fmt.Sprintf("It has %s; only %s are allowed.", bad, allowed))
		}
		if strings.Trim(s, "-.") != s {
			c.Problems = append(c.Problems, "It must start and end with a letter or digit.")
		}
		if len(c.Problems) == 0 && !re.MatchString(s) {
			c.Problems = append(c.Problems, "Each part between dots must start and end with a letter or digit.")
		}
	}
	c.Valid = len(c.Problems) == 0
	if !c.Valid && fixed != s {
		c.Fixed = fixed
	}
	return c
}

func label(s string) Check {
	c := Check{Rule: "Kubernetes label value", For: "metadata.labels values and selectors (empty is allowed)", Problems: []string{}}
	if n := utf8.RuneCountInString(s); n > 63 {
		c.Problems = append(c.Problems, fmt.Sprintf("It is %d characters long; at most 63.", n))
	}
	if bad := others(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_."); bad != "" {
		c.Problems = append(c.Problems, fmt.Sprintf("It has %s; only letters, digits, -, _ and . are allowed.", bad))
	}
	if strings.Trim(s, "-_.") != s {
		c.Problems = append(c.Problems, "It must start and end with a letter or digit.")
	}
	c.Valid = len(c.Problems) == 0 && labelValue.MatchString(s)
	if !c.Valid {
		var b strings.Builder
		for _, r := range s {
			if r < utf8.RuneSelf && (unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("-_.", r)) {
				b.WriteRune(r)
			} else {
				b.WriteByte('-')
			}
		}
		fixed := strings.Trim(squeeze(b.String(), '-'), "-_.")
		if len(fixed) > 63 {
			fixed = strings.TrimRight(fixed[:63], "-_.")
		}
		if fixed != s {
			c.Fixed = fixed
		}
	}
	return c
}

func env(s, constant string) Check {
	c := Check{Rule: "Environment variable", For: "env: in a container, Compose, CI variables, a shell export", Problems: []string{}}
	switch {
	case s == "":
		c.Problems = append(c.Problems, "It is empty.")
	case !envName.MatchString(s):
		if s[0] >= '0' && s[0] <= '9' {
			c.Problems = append(c.Problems, "It starts with a digit.")
		}
		if bad := others(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_"); bad != "" {
			c.Problems = append(c.Problems, fmt.Sprintf("It has %s; only letters, digits and _ work in every shell.", bad))
		}
	}
	c.Valid = len(c.Problems) == 0
	if c.Valid && strings.ToUpper(s) != s {
		c.Problems = append(c.Problems, "Allowed, but environment variables are usually upper case.")
	}
	if (!c.Valid || strings.ToUpper(s) != s) && constant != s {
		c.Fixed = constant
	}
	return c
}

// others describes the characters of s outside allowed, quoted, at most
// five of them.
func others(s, allowed string) string {
	var seen []string
	for _, r := range s {
		if strings.ContainsRune(allowed, r) {
			continue
		}
		q := fmt.Sprintf("%q", string(r))
		if r == ' ' {
			q = "spaces"
		}
		if !contains(seen, q) {
			seen = append(seen, q)
		}
	}
	if len(seen) > 5 {
		seen = append(seen[:5], "…")
	}
	return strings.Join(seen, ", ")
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// ascii keeps the characters of s in allowed (letters with accents and
// other scripts drop out).
func ascii(s, allowed string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(allowed, r) {
			b.WriteRune(r)
		}
	}
	return squeeze(squeeze(b.String(), '-'), '_')
}

// squeeze turns runs of c into one.
func squeeze(s string, c byte) string {
	double := string([]byte{c, c})
	for strings.Contains(s, double) {
		s = strings.ReplaceAll(s, double, string(c))
	}
	return s
}

// trimDNS cuts a kebab name to max, not ending on a dash.
func trimDNS(s string, max int) string {
	s = strings.Trim(s, "-")
	if len(s) > max {
		s = strings.TrimRight(s[:max], "-")
	}
	return s
}
