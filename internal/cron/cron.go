// Package cron reads standard cron expressions — five fields plus the
// @daily-style shortcuts — and tells when they run, in plain words, and
// builds them from simple choices. Other dialects (Quartz, Jenkins) are
// recognised and named as unsupported rather than misread.
package cron

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // zones work on machines without a zone database (Windows)
)

// Field is one of the five fields, parsed.
type Field struct {
	Name   string `json:"name"`
	Text   string `json:"text"`   // as written
	Values []int  `json:"values"` // every value it allows, ascending
	// Star: the field starts with "*" ("*", "*/5"). For the two day
	// fields this decides how they combine (see Schedule.Matches).
	Star bool `json:"star"`
}

type bounds struct {
	name     string
	min, max int
	names    []string // index = value - min, for months and weekdays
}

var fieldBounds = [5]bounds{
	{"minute", 0, 59, nil},
	{"hour", 0, 23, nil},
	{"day of month", 1, 31, nil},
	{"month", 1, 12, []string{"JAN", "FEB", "MAR", "APR", "MAY", "JUN", "JUL", "AUG", "SEP", "OCT", "NOV", "DEC"}},
	{"day of week", 0, 6, []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}},
}

var shortcuts = map[string]string{
	"@yearly":   "0 0 1 1 *",
	"@annually": "0 0 1 1 *",
	"@monthly":  "0 0 1 * *",
	"@weekly":   "0 0 * * 0",
	"@daily":    "0 0 * * *",
	"@midnight": "0 0 * * *",
	"@hourly":   "0 * * * *",
}

// Schedule is a parsed expression.
type Schedule struct {
	Expr     string    `json:"expr"`               // as given, trimmed
	Standard string    `json:"standard,omitempty"` // a shortcut's five fields
	Zone     string    `json:"zone,omitempty"`     // from a CRON_TZ= or TZ= prefix
	Fields   [5]Field  `json:"fields"`
	every    [5][]bool // value → allowed, for quick matching
}

// Error says what is wrong, and for another dialect, which one.
type Error struct {
	Msg     string `json:"message"`
	Dialect string `json:"dialect,omitempty"` // "Quartz", "Jenkins": recognised, not supported
	Field   string `json:"field,omitempty"`
}

func (e *Error) Error() string { return e.Msg }

const standardHint = "standard cron has 5 fields: minute hour day-of-month month day-of-week"

// Parse reads an expression: five fields, or a shortcut, optionally after
// CRON_TZ=Zone (or TZ=Zone).
func Parse(expr string) (*Schedule, error) {
	s := &Schedule{Expr: strings.TrimSpace(expr)}
	rest := s.Expr
	for _, p := range []string{"CRON_TZ=", "TZ="} {
		if r, ok := strings.CutPrefix(rest, p); ok {
			zone, after, _ := strings.Cut(r, " ")
			if _, err := time.LoadLocation(zone); err != nil {
				return nil, &Error{Msg: fmt.Sprintf("unknown time zone %q in %s", zone, p[:len(p)-1])}
			}
			s.Zone, rest = zone, strings.TrimSpace(after)
			break
		}
	}
	if rest == "" {
		return nil, &Error{Msg: "empty expression; " + standardHint}
	}
	if strings.HasPrefix(rest, "@") {
		if rest == "@reboot" {
			return nil, &Error{Msg: "@reboot runs once when the cron daemon starts; it has no schedule"}
		}
		std, ok := shortcuts[strings.ToLower(rest)]
		if !ok {
			return nil, &Error{Msg: fmt.Sprintf("unknown shortcut %s (have @yearly, @monthly, @weekly, @daily, @hourly)", rest)}
		}
		s.Standard, rest = std, std
	}
	parts := strings.Fields(rest)
	if err := dialect(parts); err != nil {
		return nil, err
	}
	for i, text := range parts {
		f, err := parseField(text, fieldBounds[i])
		if err != nil {
			return nil, err
		}
		s.Fields[i] = f
		s.every[i] = make([]bool, fieldBounds[i].max+1)
		for _, v := range f.Values {
			s.every[i][v] = true
		}
	}
	return s, nil
}

// dialect names expressions written for other cron flavours.
func dialect(parts []string) error {
	quartz := &Error{Dialect: "Quartz"}
	switch {
	case len(parts) == 6 || len(parts) == 7:
		quartz.Msg = fmt.Sprintf("%d fields: this looks like Quartz or Spring cron, which starts with seconds%s; %s", len(parts), map[bool]string{true: " and ends with a year", false: ""}[len(parts) == 7], standardHint)
		return quartz
	case len(parts) != 5:
		return &Error{Msg: fmt.Sprintf("%d field%s; %s", len(parts), map[bool]string{true: "", false: "s"}[len(parts) == 1], standardHint)}
	}
	for i, p := range parts {
		bare := strings.ToUpper(p)
		for _, n := range fieldBounds[i].names {
			bare = strings.ReplaceAll(bare, n, "")
		}
		switch {
		case strings.Contains(bare, "H"):
			return &Error{Dialect: "Jenkins", Field: fieldBounds[i].name, Msg: fmt.Sprintf("%q uses H, Jenkins' hashed value, which spreads jobs out per job name; standard cron has no H — pick a fixed value", p)}
		case p == "?":
			quartz.Msg, quartz.Field = fmt.Sprintf("? (no specific value) is Quartz cron; standard cron uses * in the %s field", fieldBounds[i].name), fieldBounds[i].name
			return quartz
		case (i == 2 || i == 4) && strings.ContainsAny(bare, "LW#"):
			quartz.Msg, quartz.Field = fmt.Sprintf("%q (L, W or #: last, weekday, nth) is Quartz cron, not standard cron", p), fieldBounds[i].name
			return quartz
		}
	}
	return nil
}

func parseField(text string, b bounds) (Field, error) {
	f := Field{Name: b.name, Text: text, Star: strings.HasPrefix(text, "*")}
	fail := func(format string, a ...any) (Field, error) {
		return Field{}, &Error{Field: b.name, Msg: b.name + ": " + fmt.Sprintf(format, a...)}
	}
	seen := make([]bool, b.max+2)
	for item := range strings.SplitSeq(text, ",") {
		if item == "" {
			return fail("empty item in %q", text)
		}
		rng, stepText, hasStep := strings.Cut(item, "/")
		step := 1
		if hasStep {
			n, err := strconv.Atoi(stepText)
			if err != nil || n < 1 {
				return fail("step %q must be a whole number of 1 or more", stepText)
			}
			step = n
		}
		lo, hi := b.min, b.max
		switch {
		case rng == "*":
		case strings.Contains(rng, "-"):
			a, z, _ := strings.Cut(rng, "-")
			var err error
			if lo, err = value(a, b); err != nil {
				return fail("%v", err)
			}
			if hi, err = value(z, b); err != nil {
				return fail("%v", err)
			}
			if b.max == 6 && lo == 7 && hi == 7 {
				seen[0] = true
				continue
			}
			if b.max == 6 && hi == 7 {
				hi = 6 // "5-7": Friday to Sunday
				if lo <= 6 {
					seen[0] = true
				}
			}
			if lo > hi {
				return fail("range %s goes backwards; cron ranges don't wrap around", rng)
			}
		default:
			v, err := value(rng, b)
			if err != nil {
				return fail("%v", err)
			}
			lo = v
			if !hasStep {
				hi = v // "5" is just 5; "5/15" is 5, 20, 35, 50
			}
		}
		for v := lo; v <= hi; v += step {
			seen[v] = true
		}
	}
	if b.max == 6 && seen[7] {
		seen[0] = true // 7 is Sunday too
	}
	for v := b.min; v <= b.max; v++ {
		if seen[v] {
			f.Values = append(f.Values, v)
		}
	}
	return f, nil
}

// value reads a number or a name (JAN, mon) within the field's bounds;
// 7 is accepted for Sunday.
func value(s string, b bounds) (int, error) {
	if i := slices.Index(b.names, strings.ToUpper(s)); i >= 0 && len(s) == 3 {
		return b.min + i, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		if b.names != nil {
			return 0, fmt.Errorf("%q is not a number or a name (%s–%s)", s, b.names[0], b.names[len(b.names)-1])
		}
		return 0, fmt.Errorf("%q is not a number", s)
	}
	top := b.max
	if b.max == 6 {
		top = 7
	}
	if n < b.min || n > top {
		return 0, fmt.Errorf("%d is out of range %d–%d", n, b.min, top)
	}
	return n, nil
}

// Matches reports whether the schedule runs on day d (its date in the
// schedule's zone). As in Vixie cron and Kubernetes: when both day fields
// are restricted (neither starts with "*"), a day matching either runs;
// otherwise both must match.
func (s *Schedule) Matches(d time.Time) bool {
	if !s.every[3][int(d.Month())] {
		return false
	}
	dom, dow := s.every[2][d.Day()], s.every[4][int(d.Weekday())]
	if !s.Fields[2].Star && !s.Fields[4].Star {
		return dom || dow
	}
	return dom && dow
}

// horizon bounds the search for runs: 29 February with a weekday can be
// years apart, and "30 2 *" never comes.
const horizon = 12 * 366

// Skipped is a run that falls in a daylight-saving gap (the clock jumps
// over it), so it doesn't happen that day.
type Skipped struct {
	Wall string    `json:"wall"` // "2026-03-29 02:30"
	At   time.Time `json:"at"`   // the moment after the gap
}

// Next lists the next n runs after from, in loc, with the runs a
// daylight-saving gap skipped among them. A wall time that happens twice
// (clocks going back) runs once. An empty list means it never runs.
func (s *Schedule) Next(from time.Time, n int, loc *time.Location) ([]time.Time, []Skipped) {
	var runs []time.Time
	var skipped []Skipped
	from = from.In(loc)
	day := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC) // a calendar date, no zone
	for range horizon {
		if s.Matches(day) {
			for _, h := range s.Fields[1].Values {
				for _, m := range s.Fields[0].Values {
					t := time.Date(day.Year(), day.Month(), day.Day(), h, m, 0, 0, loc)
					if t.Hour() != h || t.Minute() != m {
						if t.After(from) && len(runs) < n {
							skipped = append(skipped, Skipped{Wall: fmt.Sprintf("%s %02d:%02d", day.Format("2006-01-02"), h, m), At: t})
						}
						continue
					}
					// A wall time that happens twice runs at its first occurrence.
					if e := t.Add(-time.Hour); e.Hour() == h && e.Minute() == m && e.Day() == t.Day() {
						t = e
					}
					if !t.After(from) || (len(runs) > 0 && !t.After(runs[len(runs)-1])) {
						continue
					}
					runs = append(runs, t)
					if len(runs) == n {
						return runs, skipped
					}
				}
			}
		}
		day = day.AddDate(0, 0, 1)
	}
	return runs, skipped
}
