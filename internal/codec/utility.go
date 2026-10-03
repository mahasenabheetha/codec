package codec

import (
	"fmt"
	"regexp"
	"strings"
	"time"

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
	epochText = regexp.MustCompile(`^-?\d{9,19}(\.\d+)?$`)
	percentXX = regexp.MustCompile(`%[0-9A-Fa-f]{2}`)
)

// DetectUtility recognises Utilities input. It is checked before the
// JSON, base64 and JWT detection, so it is strict: a bare number of 9 to
// 19 digits (as JSON it would only be base64-encoded, which nobody
// wants), text with %XX escapes that isn't JSON, or five cron fields
// with a * or a shortcut. The clipboard watcher doesn't use it.
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
	case percentXX.MatchString(s) && ValidateJSON(s) != nil:
		out, err := encode.URLDecode(s, !strings.Contains(s, "%20") && strings.Contains(s, "+"))
		if err != nil || out == s {
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
		runs, _ := sched.Next(now, 5, time.UTC)
		var b strings.Builder
		b.WriteString(sched.Describe())
		b.WriteString("\n\nNext runs (UTC):")
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

// looksLikeCron: a shortcut, or 5 to 7 fields of cron characters with at
// least one * or ? (plain numbers alone are too likely something else).
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
	for _, f := range fields {
		for _, r := range strings.ToUpper(f) {
			if !strings.ContainsRune("0123456789*/,-?LW#HABCDEFGIJKMNOPRSTUVY()", r) {
				return false
			}
		}
	}
	return true
}
