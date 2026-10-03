// Package timeutil reads timestamps — epoch numbers in seconds,
// milliseconds, microseconds or nanoseconds, and dates as logs and APIs
// write them — and shows them in the forms people compare.
package timeutil

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // zones work on machines without a zone database (Windows)
)

// Stamp is one moment in every form the Timestamp tool shows.
type Stamp struct {
	Kind     string `json:"kind"`     // seconds, milliseconds, microseconds, nanoseconds, date
	Unix     int64  `json:"unix"`     // seconds
	UnixMs   int64  `json:"unixMs"`   // milliseconds
	UTC      string `json:"utc"`      // RFC 3339 in UTC
	Local    string `json:"local"`    // RFC 3339 in the chosen zone
	Readable string `json:"readable"` // "Tuesday, 3 February 2026, 14:05:09 CET"
	HTTP     string `json:"http"`     // RFC 1123 in GMT, as HTTP headers write it
	Zone     string `json:"zone"`     // the chosen zone's name
	Offset   string `json:"offset"`   // its offset at that moment, "+01:00"
	Relative string `json:"relative"` // "in 3 days 4 hours", "2 minutes ago"
	ISOWeek  string `json:"isoWeek"`  // "2026-W06"
	DayOfYr  int    `json:"dayOfYear"`
}

// Zone resolves a zone name: "" or "UTC" is UTC, "Local" this machine's.
func Zone(name string) (*time.Location, error) {
	switch name {
	case "", "UTC":
		return time.UTC, nil
	case "Local":
		return time.Local, nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("unknown time zone %q (use a name such as Europe/Stockholm)", name)
	}
	return loc, nil
}

// Parse reads an epoch number or a date. A date without a zone is in loc.
func Parse(input string, loc *time.Location, now time.Time) (*Stamp, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return nil, fmt.Errorf("empty input")
	}
	if t, kind, ok := epoch(s); ok {
		return Describe(t, kind, loc, now), nil
	}
	if t, ok := date(s, loc); ok {
		return Describe(t, "date", loc, now), nil
	}
	return nil, fmt.Errorf("%q is neither an epoch number nor a date this tool reads (try 2026-02-03T14:05:09Z)", s)
}

// epoch reads an integer by its size: up to 11 digits are seconds (until
// the year 5138), then milliseconds, microseconds and nanoseconds. A
// decimal is seconds with a fraction.
func epoch(s string) (time.Time, string, bool) {
	if f, err := strconv.ParseFloat(s, 64); err == nil && strings.Contains(s, ".") && !strings.ContainsAny(s, "eE") {
		sec, frac := math.Modf(f)
		return time.Unix(int64(sec), int64(math.Round(frac*1e9))).UTC(), "seconds", true
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, "", false
	}
	a := n
	if a < 0 {
		a = -a
	}
	switch {
	case a < 1e11:
		return time.Unix(n, 0).UTC(), "seconds", true
	case a < 1e14:
		return time.UnixMilli(n).UTC(), "milliseconds", true
	case a < 1e17:
		return time.UnixMicro(n).UTC(), "microseconds", true
	}
	return time.Unix(0, n).UTC(), "nanoseconds", true
}

var zoned = []string{
	time.RFC3339Nano,
	"2006-01-02T15:04:05.999999999Z0700",
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05.999999999 -0700 MST", // Go's time.String
	"2006-01-02 15:04:05.999999999 -0700",
	time.RFC1123, time.RFC1123Z, time.RFC850, time.RubyDate, time.UnixDate,
	"Mon, 2 Jan 2006 15:04:05 -0700", // RFC 2822 (email)
	"02/Jan/2006:15:04:05 -0700",     // access logs
}

var local = []string{
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02T15:04",
	"2006-01-02 15:04",
	"2006-01-02",
	"2006/01/02 15:04:05",
	"2006/01/02",
	time.ANSIC,
	"Jan 2 2006 15:04:05",
	"2 Jan 2006 15:04",
	"2 Jan 2006",
	"Jan 2, 2006",
}

func date(s string, loc *time.Location) (time.Time, bool) {
	tries := []string{s}
	if i := strings.LastIndexByte(s, ','); i > 0 && i+1 < len(s) && s[i-1] >= '0' && s[i-1] <= '9' && s[i+1] >= '0' && s[i+1] <= '9' {
		tries = append(tries, s[:i]+"."+s[i+1:]) // "15:04:05,123": ISO allows a comma
	}
	for _, try := range tries {
		for _, l := range zoned {
			if t, err := time.Parse(l, try); err == nil {
				return t, true
			}
		}
		for _, l := range local {
			if t, err := time.ParseInLocation(l, try, loc); err == nil {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

// Describe shows t in every form, in loc, relative to now.
func Describe(t time.Time, kind string, loc *time.Location, now time.Time) *Stamp {
	lt := t.In(loc)
	y, w := lt.ISOWeek()
	return &Stamp{
		Kind:     kind,
		Unix:     t.Unix(),
		UnixMs:   t.UnixMilli(),
		UTC:      t.UTC().Format(time.RFC3339Nano),
		Local:    lt.Format(time.RFC3339Nano),
		Readable: lt.Format("Monday, 2 January 2006, 15:04:05 MST"),
		HTTP:     t.UTC().Format(http1123),
		Zone:     loc.String(),
		Offset:   lt.Format("-07:00"),
		Relative: Relative(t, now),
		ISOWeek:  fmt.Sprintf("%d-W%02d", y, w),
		DayOfYr:  lt.YearDay(),
	}
}

const http1123 = "Mon, 02 Jan 2006 15:04:05 GMT"

// Relative says how far t is from now in at most two units: "in 3 days
// 4 hours", "2 minutes ago", "just now".
func Relative(t, now time.Time) string {
	d := t.Sub(now)
	future := d > 0
	if d < 0 {
		d = -d
	}
	if d < 5*time.Second {
		return "just now"
	}
	const day, year = 24 * time.Hour, 36525 * 24 * time.Hour / 100
	month := year / 12
	units := []struct {
		size time.Duration
		name string
	}{{year, "year"}, {month, "month"}, {day, "day"}, {time.Hour, "hour"}, {time.Minute, "minute"}, {time.Second, "second"}}
	var parts []string
	for _, u := range units {
		if d >= u.size {
			n := int64(d / u.size)
			d -= time.Duration(n) * u.size
			parts = append(parts, plural(n, u.name))
			if len(parts) == 2 {
				break
			}
		} else if len(parts) == 1 {
			break // "3 days", not "3 days 0 hours 12 minutes"
		}
	}
	s := strings.Join(parts, " ")
	if future {
		return "in " + s
	}
	return s + " ago"
}

func plural(n int64, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return strconv.FormatInt(n, 10) + " " + unit + "s"
}
