package sanitize

import (
	"regexp"
	"strings"
	"unicode"
)

// Secrets: values of fields named for a secret (password=, "token": …,
// password: in YAML, Password= in a connection string), credentials in
// headers and URLs, JWTs, private keys, and tokens that say what they
// are by their shape (AWS, GitHub, GitLab, Slack, Google, Stripe, npm).
// The Log viewer's copy masking (frontend/src/features/logs/mask.ts) is
// the quick, local subset of these.

var (
	// "password": "…" in JSON, also escaped inside a JSON string.
	jsonField    = regexp.MustCompile(`"([\w.@$-]+)"\s*:\s*"((?:[^"\\]|\\.)*)"`)
	escJSONField = regexp.MustCompile(`\\"([\w.@$-]+)\\"\s*:\s*\\"((?:[^"\\]|\\[^"])*)\\"`)
	// password: … in YAML and headers (X-Api-Key: …), to the end of the line.
	colonField = regexp.MustCompile(`(?m)^[ \t]*-?[ \t]*([\w.@$-]+):[ \t]+(\S[^\n#]*?)[ \t]*$`)
	// Authorization: Bearer …, Basic …, Token …
	bearer      = regexp.MustCompile(`\b(?:Bearer|Basic|Token)\s+([A-Za-z0-9._~+/=-]{8,})`)
	schemeValue = regexp.MustCompile(`^(?:Bearer|Basic|Token)\s`)
	jwt         = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{6,}`)
	// scheme://user:password@host
	urlPassword = regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.-]*://[^\s:/@]+:([^\s@/]+)@`)
	privateKey  = regexp.MustCompile(`-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z0-9 ]*PRIVATE KEY-----`)
	// Tokens known by their shape.
	shaped = regexp.MustCompile(`\b(?:` +
		`(?:AKIA|ASIA)[0-9A-Z]{16}` + // AWS access key id
		`|gh[pousr]_[A-Za-z0-9]{36,}|github_pat_[A-Za-z0-9_]{22,}` + // GitHub
		`|glpat-[A-Za-z0-9_-]{20,}` + // GitLab
		`|xox[abposr]-[A-Za-z0-9-]{10,}` + // Slack
		`|AIza[0-9A-Za-z_-]{35}` + // Google API key
		`|[sr]k_live_[0-9A-Za-z]{20,}` + // Stripe
		`|npm_[A-Za-z0-9]{36}` + // npm
		`)\b`)
)

// secrets calls found with the span of each secret value in text.
func secrets(text string, found func(start, end int)) {
	keyValues(text, found)
	for _, re := range []*regexp.Regexp{jsonField, escJSONField, colonField} {
		for _, m := range re.FindAllStringSubmatchIndex(text, -1) {
			if !secretName(text[m[2]:m[3]]) {
				continue
			}
			s, e := m[4], m[5]
			if e-s >= 2 && (text[s] == '"' || text[s] == '\'') && text[e-1] == text[s] {
				s, e = s+1, e-1 // the value inside its quotes
			}
			// Authorization: Bearer … is the bearer rule's, scheme kept.
			if !placeholder(text[s:e]) && !schemeValue.MatchString(text[s:e]) {
				found(s, e)
			}
		}
	}
	for _, r := range []struct {
		re   *regexp.Regexp
		mark []string // it can't match without one of these
	}{
		{bearer, []string{"Bearer", "Basic", "Token"}},
		{urlPassword, []string{"://"}},
	} {
		if !anyIn(text, r.mark) {
			continue
		}
		for _, m := range r.re.FindAllStringSubmatchIndex(text, -1) {
			if !placeholder(text[m[2]:m[3]]) {
				found(m[2], m[3])
			}
		}
	}
	for _, r := range []struct {
		re   *regexp.Regexp
		mark []string
	}{
		{jwt, []string{"eyJ"}},
		{privateKey, []string{"PRIVATE KEY"}},
		{shaped, []string{"AKIA", "ASIA", "ghp_", "gho_", "ghu_", "ghs_", "ghr_", "github_pat_", "glpat-", "xox", "AIza", "k_live_", "npm_"}},
	} {
		if !anyIn(text, r.mark) {
			continue
		}
		for _, m := range r.re.FindAllStringIndex(text, -1) {
			found(m[0], m[1])
		}
	}
}

// secretWords and secretPairs name a secret field: password, apiKey,
// x-auth-token, AccountKey, client_secret. A word must be whole, so
// max_tokens and passed are not.
var (
	secretWords = map[string]bool{
		"password": true, "passwd": true, "passphrase": true, "pwd": true, "pw": true, "pass": true, "secret": true, "token": true,
		"apikey": true, "authorization": true, "cookie": true, "credential": true, "credentials": true, "sessionid": true,
		"sig": true, "signature": true, "privatekey": true, "accesskey": true, "secretkey": true, "accountkey": true,
	}
	secretPairs = map[[2]string]bool{
		{"api", "key"}: true, {"access", "key"}: true, {"secret", "key"}: true, {"private", "key"}: true, {"account", "key"}: true,
		{"client", "secret"}: true, {"session", "id"}: true, {"auth", "key"}: true, {"shared", "key"}: true, {"set", "cookie"}: true,
	}
)

func secretName(name string) bool {
	w := nameWords(name)
	for i, x := range w {
		if secretWords[x] || i+1 < len(w) && secretPairs[[2]string{x, w[i+1]}] {
			return true
		}
	}
	return false
}

// nameWords splits a field name into lower-case words: at symbols and
// where the case turns (accessToken, X-Auth-Token, DB_PASSWORD).
func nameWords(s string) []string {
	var out []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.ToLower(string(cur)))
			cur = cur[:0]
		}
	}
	rs := []rune(s)
	for i, r := range rs {
		switch {
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			flush()
		case unicode.IsUpper(r) && len(cur) > 0 && (unicode.IsLower(cur[len(cur)-1]) || i+1 < len(rs) && unicode.IsLower(rs[i+1])):
			flush()
			cur = append(cur, r)
		default:
			cur = append(cur, r)
		}
	}
	flush()
	return out
}

// placeholderShape is a whole value that is a variable or a template:
// ${NAME}, $(command), $NAME, %NAME%, {{ .Values.x }}, <your-password>.
// Only the whole value counts: a password that starts with $, % or <
// ($uper5ecret!, %40bc as a URL writes @bc) is still a password.
var placeholderShape = regexp.MustCompile(`^(?:\$\{[^}]*\}|\$\([^)]*\)|\$[A-Za-z_]\w*|%[A-Za-z_]\w*%|\{\{.*\}\}|<[A-Za-z][A-Za-z _.-]*>)$`)

// placeholder: a value that is already no secret: empty, masked, a
// variable or a template to fill in.
func placeholder(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" || strings.Trim(v, "*•x.") == "" || strings.HasPrefix(v, "[secret-") || placeholderShape.MatchString(v) {
		return true
	}
	switch strings.ToLower(v) {
	case "null", "none", "true", "false", "changeme", "redacted", "<redacted>", "hidden", "***":
		return true
	}
	return false
}

func anyIn(text string, marks []string) bool {
	for _, m := range marks {
		if strings.Contains(text, m) {
			return true
		}
	}
	return false
}
