package codec

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/mahasenabheetha/codec/v2/internal/cron"
	"github.com/mahasenabheetha/codec/v2/internal/encode"
	"github.com/mahasenabheetha/codec/v2/internal/httpkit"
	"github.com/mahasenabheetha/codec/v2/internal/jsonkit"
	"github.com/mahasenabheetha/codec/v2/internal/logview"
	"github.com/mahasenabheetha/codec/v2/internal/netutil"
	"github.com/mahasenabheetha/codec/v2/internal/table"
	"github.com/mahasenabheetha/codec/v2/internal/timeutil"
)

// Utility is input that belongs to one of the Utilities tools: an epoch
// number, URL-encoded text, a cron expression, a CIDR range, an IP
// address or a curl command. Smart paste shows the
// summary and offers to open the tool on the right tab.
type Utility struct {
	Kind    string `json:"kind"` // epoch, url, cron, log, cidr, ip, curl
	Tool    string `json:"tool"` // time, encode, logs, network
	Tab     string `json:"tab"`  // timestamp, url, cron, viewer, cidr, ip, http
	Summary string `json:"summary"`
}

var (
	// No leading zero: 0701234567 is a phone number, not 1992.
	epochText = regexp.MustCompile(`^-?[1-9]\d{8,18}(\.\d+)?$`)
	percentXX = regexp.MustCompile(`%[0-9A-Fa-f]{2}`)
	// A range as address/length, IPv4 or IPv6; never a bare number.
	cidrText = regexp.MustCompile(`^(?:\d{1,3}(?:\.\d{1,3}){3}|[0-9A-Fa-f:]*:[0-9A-Fa-f:.]*)/\d{1,3}$`)
	// An address written as one: dotted IPv4 or with colons (IPv6).
	ipText = regexp.MustCompile(`^(?:\d{1,3}(?:\.\d{1,3}){3}|\[?[0-9A-Fa-f:]*:[0-9A-Fa-f:.]*\]?)$`)
	// One cron field: digits, * ? and , / - , month and day names, and
	// the letters other dialects use (L W # H) — not words.
	cronField = regexp.MustCompile(`(?i)^(?:[0-9*?#,/()-]|L|W|H|JAN|FEB|MAR|APR|MAY|JUN|JUL|AUG|SEP|OCT|NOV|DEC|SUN|MON|TUE|WED|THU|FRI|SAT)+$`)
)

// DetectUtility recognises Utilities input. It is checked before the
// JSON, base64 and JWT detection, so it is strict:
//   - a bare number of 9 to 19 digits, no leading zero (as JSON it would
//     only be base64-encoded, which nobody wants);
//   - one line with %XX escapes, no braces, that decodes to readable text
//     (not JSON or a log that mentions a URL, not a printf format);
//   - 5 to 7 cron fields, at least two of them numbers or *, or a
//     shortcut;
//   - a log of at least two entries, most with a time or a level (not an
//     Ansible run, which has its own tool), for the Log viewer.
//
// The clipboard watcher doesn't use it.
func DetectUtility(s string, now time.Time) (*Utility, bool) {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return nil, false
	case epochText.MatchString(s):
		st, err := timeutil.Parse(s, time.Local, now)
		if err != nil {
			return nil, false
		}
		sum := fmt.Sprintf("Epoch %s\n\nUTC       %s\nLocal     %s\n          %s\nRelative  %s", st.Kind, st.UTC, st.Local, st.Readable, st.Relative)
		return &Utility{Kind: "epoch", Tool: "time", Tab: "timestamp", Summary: sum}, true
	case percentXX.MatchString(s) && !strings.ContainsAny(s, "{}\n"):
		out, err := encode.URLDecode(s, !strings.Contains(s, "%20") && strings.Contains(s, "+"))
		if err != nil || out == s || !readable(out) {
			return nil, false
		}
		return &Utility{Kind: "url", Tool: "encode", Tab: "url", Summary: out}, true
	case looksLikeCron(s):
		u := &Utility{Kind: "cron", Tool: "time", Tab: "cron"}
		sched, err := cron.Parse(s)
		if err != nil {
			u.Summary = err.Error()
			return u, true
		}
		loc, err := timeutil.Zone(sched.Zone) // a CRON_TZ= prefix, else UTC
		if err != nil {
			loc = time.UTC
		}
		runs, _ := sched.Next(now, 5, loc)
		var b strings.Builder
		b.WriteString(sched.Describe())
		fmt.Fprintf(&b, "\n\nNext runs (%s):", loc)
		for _, r := range runs {
			fmt.Fprintf(&b, "\n  %s  %s", r.Format("Mon 2006-01-02 15:04"), timeutil.Relative(r, now))
		}
		if len(runs) == 0 {
			b.WriteString("\n  never: no date matches")
		}
		u.Summary = b.String()
		return u, true
	}
	if u, ok := networkUtility(s); ok {
		return u, true
	}
	if u, ok := lenientJSON(s); ok {
		return u, true
	}
	sum, isLog := logSummary(s)
	if u, ok := tableUtility(s, isLog); ok {
		return u, true
	}
	if isLog {
		return &Utility{Kind: "log", Tool: "logs", Tab: "viewer", Summary: sum}, true
	}
	return nil, false
}

// lenientJSON recognises an object or array that is JSONC or JSON5 but
// not strict JSON (comments, trailing commas, unquoted keys…), for the
// JSON & YAML tool; the summary is it written as JSON. Strict JSON is
// left to the JSON detector.
func lenientJSON(s string) (*Utility, bool) {
	if s[0] != '{' && s[0] != '[' && !strings.HasPrefix(s, "//") && !strings.HasPrefix(s, "/*") {
		return nil, false
	}
	v, d, err := jsonkit.Parse(s)
	if err != nil || d == jsonkit.JSON || v.Kind != jsonkit.Object && v.Kind != jsonkit.Array {
		return nil, false
	}
	return &Utility{Kind: string(d), Tool: "json", Tab: "format", Summary: jsonkit.Encode(v, "  ")}, true
}

// tableUtility recognises CSV or TSV, for Text & data: at least three
// lines, the delimiter guessed, two or more columns, every row as wide
// as the first, and short cells, most of them a few words (prose with a
// comma per line isn't a table). Text whose first row isn't a header and that reads as a log
// (a Python log's "12:00:00,123") stays a log. JSON is never a table.
func tableUtility(s string, isLog bool) (*Utility, bool) {
	if s[0] == '{' || s[0] == '[' || strings.Count(s, "\n") < 2 {
		return nil, false
	}
	t, err := table.Parse(s, table.Options{})
	if err != nil || len(t.Columns) < 2 || t.Ragged > 0 || len(t.Rows) < 2 || isLog && !t.Header {
		return nil, false
	}
	cells, chars, wordy := 0, 0, 0
	for _, r := range t.Rows[:min(len(t.Rows), 200)] {
		for _, c := range r {
			cells++
			chars += utf8.RuneCountInString(c)
			if len(strings.Fields(c)) > 4 {
				wordy++
			}
		}
	}
	if chars > cells*40 || wordy*2 >= cells {
		return nil, false
	}
	kind, delim := "csv", map[rune]string{',': "comma", '\t': "tab", ';': "semicolon", '|': "pipe"}[t.Delim]
	if t.Delim == '\t' {
		kind = "tsv"
	}
	var b strings.Builder
	head := "no header row"
	if t.Header {
		head = "a header row"
	}
	fmt.Fprintf(&b, "%s · %s · %d rows · %d columns · %s\n", strings.ToUpper(kind), delim, len(t.Rows), len(t.Columns), head)
	width := 0
	for _, c := range t.Columns[:min(len(t.Columns), 20)] {
		width = max(width, utf8.RuneCountInString(c.Name))
	}
	for _, c := range t.Columns[:min(len(t.Columns), 20)] {
		if c.Empty > 0 {
			fmt.Fprintf(&b, "\n%-*s  %-8s  %d empty", width, c.Name, c.Type, c.Empty)
		} else {
			fmt.Fprintf(&b, "\n%-*s  %s", width, c.Name, c.Type)
		}
	}
	if n := len(t.Columns) - 20; n > 0 {
		fmt.Fprintf(&b, "\n… and %d more columns", n)
	}
	return &Utility{Kind: kind, Tool: "text", Tab: "table", Summary: b.String()}, true
}

// curlLike tells a curl command from a sentence that starts with
// "curl": one URL, with a real host (a dot, localhost or an IPv6
// address), and no stray words read as more URLs.
func curlLike(r *httpkit.Request) bool {
	for _, n := range r.Notes {
		if strings.HasPrefix(n, "curl requests") {
			return false
		}
	}
	host := r.URL
	if _, after, ok := strings.Cut(host, "://"); ok {
		host = after
	}
	host, _, _ = strings.Cut(host, "/")
	return strings.Contains(host, ".") || strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "[")
}

// networkUtility recognises a curl command, a CIDR range and an IP
// address, for the Network tool.
func networkUtility(s string) (*Utility, bool) {
	cmd := strings.TrimPrefix(s, "$ ")
	if low := strings.ToLower(cmd); strings.HasPrefix(low, "curl ") || strings.HasPrefix(low, "curl.exe ") || strings.HasPrefix(low, "curl\\") {
		r, err := httpkit.ParseCurl(cmd)
		if err != nil || !curlLike(r) {
			return nil, false
		}
		sum := strings.TrimRight(r.Fetch(), "\n")
		for _, n := range r.Notes {
			sum += "\n// note: " + n
		}
		return &Utility{Kind: "curl", Tool: "network", Tab: "http", Summary: sum}, true
	}
	if cidrText.MatchString(s) {
		r, err := netutil.Details(s)
		if err != nil {
			return nil, false
		}
		broadcast := ""
		if r.Broadcast != "" {
			broadcast = "\nBroadcast  " + r.Broadcast
		}
		sum := fmt.Sprintf("%s · IPv%d · %s\n\nNetwork    %s%s\nFirst host %s\nLast host  %s\nNetmask    %s\nHosts      %s", r.CIDR, r.Version, r.Kind, r.Network, broadcast, r.First, r.Last, r.Mask, r.Hosts)
		return &Utility{Kind: "cidr", Tool: "network", Tab: "cidr", Summary: sum}, true
	}
	if ipText.MatchString(s) {
		ip, err := netutil.ParseIP(s)
		if err != nil {
			return nil, false
		}
		sum := fmt.Sprintf("%s · IPv%d · %s\n\nInteger  %s\nHex      %s\nReverse  %s", ip.Address, ip.Version, ip.Kind, ip.Integer, ip.Hex, ip.Reverse)
		return &Utility{Kind: "ip", Tool: "network", Tab: "ip", Summary: sum}, true
	}
	return nil, false
}

// logSummary reads the first lines of s as a log and describes it, when
// at least two entries and most of them have a time or a level. A single
// JSON object is JSON, not a log; an Ansible run goes to its own tool.
func logSummary(s string) (string, bool) {
	if strings.Contains(s, "TASK [") || strings.Contains(s, "PLAY [") {
		return "", false
	}
	head := s
	for i, n := 0, 0; i < len(s); i++ {
		if s[i] == '\n' {
			if n++; n == 200 {
				head = s[:i]
				break
			}
		}
	}
	entries, marked := 0, 0
	formats := map[string]int{}
	levels := map[logview.Level]int{}
	for e := range logview.Read(head, logview.Options{}) {
		entries++
		formats[e.Format.String()]++
		levels[e.Level]++
		if !e.Time.IsZero() || e.Level != logview.LevelNone {
			marked++
		}
	}
	if entries < 2 || marked*10 < entries*6 {
		return "", false
	}
	var b strings.Builder
	fmt.Fprintf(&b, "A log: %d entries", entries)
	if strings.Count(s, "\n") >= 200 {
		b.WriteString(" in its first 200 lines")
	}
	for _, f := range []string{"ecs", "json", "logfmt", "klog", "text"} {
		if formats[f] > 0 {
			fmt.Fprintf(&b, " · %s %d", f, formats[f])
		}
	}
	switch n := levels[logview.Error] + levels[logview.Fatal]; n {
	case 0:
	case 1:
		b.WriteString("\n1 error")
	default:
		fmt.Fprintf(&b, "\n%d errors", n)
	}
	return b.String(), true
}

// readable reports UTF-8 text without control characters: what a decoded
// URL value is, and what printf formats ("%02x") don't decode to.
func readable(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) && r != '\t' {
			return false
		}
	}
	return true
}

// looksLikeCron: a shortcut, or 5 to 7 cron fields with at least one * or
// ? and at least two fields that are numbers or * (so a short sentence
// with a question mark isn't cron).
func looksLikeCron(s string) bool {
	if strings.HasPrefix(s, "@") {
		_, err := cron.Parse(s)
		return err == nil || s == "@reboot"
	}
	if strings.HasPrefix(s, "CRON_TZ=") || strings.HasPrefix(s, "TZ=") {
		_, rest, _ := strings.Cut(s, " ")
		s = rest
	}
	fields := strings.Fields(s)
	if len(fields) < 5 || len(fields) > 7 || !strings.ContainsAny(s, "*?") || strings.Contains(s, "\n") {
		return false
	}
	numeric := 0
	for _, f := range fields {
		if !cronField.MatchString(f) {
			return false
		}
		if strings.ContainsAny(f, "0123456789*") {
			numeric++
		}
	}
	return numeric >= 2
}
