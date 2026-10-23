package logview

import (
	"math"
	"strconv"
	"strings"
	"time"
)

// Layouts tried for a written time, most common first. A comma before
// the fraction (Java, Python) is read as a dot first. Times without a
// zone are read as UTC.
var timeLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02T15:04:05.999999999Z0700",
	"2006-01-02 15:04:05.999999999Z0700",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05.999999999 -0700 MST", // Go's time.String
	"2006-01-02 15:04:05.999999999 MST",
	"2006/01/02 15:04:05.999999999", // Go's log package
	"02/Jan/2006:15:04:05 -0700",    // access logs
	time.RFC1123Z,
	time.RFC1123,
}

// parseTime reads a timestamp as loggers write it.
func parseTime(s string) (time.Time, bool) {
	if len(s) < 8 || len(s) > 64 {
		return time.Time{}, false
	}
	if i := strings.IndexByte(s, ','); i > 0 && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '9' {
		s = s[:i] + "." + s[i+1:]
	}
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, plausible(t)
		}
	}
	return time.Time{}, false
}

// epochTime reads a number of seconds, milliseconds, microseconds or
// nanoseconds since 1970, telling them apart by size (zap writes
// seconds as a float, pino milliseconds).
func epochTime(n float64) (time.Time, bool) {
	if math.IsNaN(n) || n <= 0 {
		return time.Time{}, false
	}
	var t time.Time
	switch {
	case n < 1e11:
		sec, frac := math.Modf(n)
		t = time.Unix(int64(sec), int64(frac*1e9))
	case n < 1e14:
		t = time.UnixMilli(int64(n))
	case n < 1e17:
		t = time.UnixMicro(int64(n))
	case n < 1e19:
		t = time.Unix(0, int64(n))
	default:
		return time.Time{}, false
	}
	t = t.UTC()
	return t, plausible(t)
}

// epochText is epochTime read from the number's digits, so
// 1792656000.123456 keeps exactly its microseconds (a float64 can't).
func epochText(s string) (time.Time, bool) {
	ip, fp, _ := strings.Cut(s, ".")
	n, err := strconv.ParseInt(ip, 10, 64)
	if err != nil || n <= 0 || strings.ContainsAny(fp, "eE") {
		f, _ := strconv.ParseFloat(s, 64)
		return epochTime(f)
	}
	var frac int64 // nanoseconds, as if the unit were seconds
	if fp != "" {
		if frac, err = strconv.ParseInt((fp + "000000000")[:9], 10, 64); err != nil {
			return time.Time{}, false
		}
	}
	var t time.Time
	switch {
	case n < 1e11:
		t = time.Unix(n, frac)
	case n < 1e14:
		t = time.Unix(n/1e3, n%1e3*1e6+frac/1e3)
	case n < 1e17:
		t = time.Unix(n/1e6, n%1e6*1e3+frac/1e6)
	default:
		t = time.Unix(0, n)
	}
	t = t.UTC()
	return t, plausible(t)
}

// plausible keeps "t": 5 or a version number from passing as a time.
func plausible(t time.Time) bool {
	return t.Year() > 1990 && t.Year() < 2200
}

// Year-less times (klog, syslog) take the year of the log's other times,
// else the current year; withYear moves a time read with year 0 there.
func withYear(t time.Time, year int) time.Time {
	return time.Date(year, t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
}
