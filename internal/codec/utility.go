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
	"github.com/mahasenabheetha/codec/v2/internal/timeutil"
)

// Utility is input that belongs to one of the Utilities tools: an epoch
// number, URL-encoded text or a cron expression. Smart paste shows the
// summary and offers to open the tool on the right tab.
type Utility struct {
	Kind    string `json:"kind"` // epoch, url, cron
	Tool    string `json:"tool"` // time, encode
	Tab     string `json:"tab"`  // timestamp, url, cron
	Summary string `json:"summary"`
}

var (
	// No leading zero: 0701234567 is a phone number, not 1992.
	epochText = regexp.MustCompile(`^-?[1-9]\d{8,18}(\.\d+)?$`)
	percentXX = regexp.MustCompile(`%[0-9A-Fa-f]{2}`)
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
//     shortcut.
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
	return nil, false
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
