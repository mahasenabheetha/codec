package timeutil

import (
	"fmt"
	"strings"
	"time"
)

// Format is one instant written one standard way.
type Format struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Pattern is one shape of date and how each language writes its format
// string; Example is the instant in that shape.
type Pattern struct {
	Name    string `json:"name"`
	Example string `json:"example"`
	Go      string `json:"go"`
	Java    string `json:"java"`   // java.time.format.DateTimeFormatter
	Python  string `json:"python"` // strftime
	DotNet  string `json:"dotnet"` // .NET custom format strings
	JS      string `json:"js"`     // JavaScript has no format strings: the call that writes it
	Note    string `json:"note,omitempty"`
}

// Formats writes t, in loc, in the formats APIs, logs and headers use.
func Formats(t time.Time, loc *time.Location) []Format {
	lt := t.In(loc)
	y, w := lt.ISOWeek()
	wd := int(lt.Weekday())
	if wd == 0 {
		wd = 7 // ISO: Monday 1 … Sunday 7
	}
	return []Format{
		{"RFC 3339 / ISO 8601", lt.Format(time.RFC3339)},
		{"RFC 3339 with milliseconds, UTC", t.UTC().Format("2006-01-02T15:04:05.000Z07:00")},
		{"RFC 3339 with nanoseconds", lt.Format(time.RFC3339Nano)},
		{"ISO 8601 basic", lt.Format("20060102T150405Z0700")},
		{"ISO week date", fmt.Sprintf("%04d-W%02d-%d", y, w, wd)},
		{"ISO ordinal date", fmt.Sprintf("%04d-%03d", lt.Year(), lt.YearDay())},
		{"RFC 2822 (email)", lt.Format(time.RFC1123Z)},
		{"HTTP date (RFC 9110)", t.UTC().Format(http1123)},
		{"SQL / log", lt.Format("2006-01-02 15:04:05")},
		{"Unix seconds", fmt.Sprint(t.Unix())},
		{"Unix milliseconds", fmt.Sprint(t.UnixMilli())},
		{"Unix microseconds", fmt.Sprint(t.UnixMicro())},
		{"Unix nanoseconds", unixNano(t)},
	}
}

// unixNano is t in nanoseconds, or a note where int64 can't hold it
// (before 1678 or after 2262).
func unixNano(t time.Time) string {
	if t.Year() < 1678 || t.Year() > 2261 {
		return "out of range for 64-bit nanoseconds"
	}
	return fmt.Sprint(t.UnixNano())
}

// Patterns shows common date shapes, t written in each (in loc), and
// the format string each language uses for it.
func Patterns(t time.Time, loc *time.Location) []Pattern {
	lt := t.In(loc)
	ps := []Pattern{
		{Name: "ISO 8601 with offset", Example: lt.Format(time.RFC3339),
			Go: "2006-01-02T15:04:05Z07:00", Java: "yyyy-MM-dd'T'HH:mm:ssXXX", Python: "%Y-%m-%dT%H:%M:%S%:z", DotNet: "yyyy-MM-dd'T'HH:mm:sszzz",
			JS: "date.toISOString() // always UTC, with milliseconds", Note: "Python's %:z needs 3.12; before that %z writes +0100."},
		{Name: "With milliseconds", Example: lt.Format("2006-01-02T15:04:05.000Z07:00"),
			Go: "2006-01-02T15:04:05.000Z07:00", Java: "yyyy-MM-dd'T'HH:mm:ss.SSSXXX", Python: "%Y-%m-%dT%H:%M:%S.%f%:z", DotNet: "yyyy-MM-dd'T'HH:mm:ss.fffzzz",
			JS: "date.toISOString()", Note: "Python's %f is microseconds (six digits)."},
		{Name: "Date", Example: lt.Format("2006-01-02"),
			Go: "2006-01-02", Java: "yyyy-MM-dd", Python: "%Y-%m-%d", DotNet: "yyyy-MM-dd",
			JS: "date.toISOString().slice(0, 10) // UTC"},
		{Name: "Date and time (SQL, logs)", Example: lt.Format("2006-01-02 15:04:05"),
			Go: "2006-01-02 15:04:05", Java: "yyyy-MM-dd HH:mm:ss", Python: "%Y-%m-%d %H:%M:%S", DotNet: "yyyy-MM-dd HH:mm:ss",
			JS: "date.toLocaleString('sv-SE') // local time"},
		{Name: "Time, 24-hour", Example: lt.Format("15:04:05"),
			Go: "15:04:05", Java: "HH:mm:ss", Python: "%H:%M:%S", DotNet: "HH:mm:ss",
			JS: "date.toLocaleTimeString('en-GB')"},
		{Name: "RFC 2822 (email)", Example: lt.Format(time.RFC1123Z),
			Go: "Mon, 02 Jan 2006 15:04:05 -0700", Java: "EEE, dd MMM yyyy HH:mm:ss Z", Python: "%a, %d %b %Y %H:%M:%S %z", DotNet: "ddd, dd MMM yyyy HH:mm:ss zzz",
			JS: "date.toString() // close; no exact built-in", Note: ".NET's zzz writes +01:00, not +0100."},
		{Name: "HTTP date (GMT)", Example: t.UTC().Format(http1123),
			Go: "http.TimeFormat (on a UTC time)", Java: "DateTimeFormatter.RFC_1123_DATE_TIME", Python: "email.utils.formatdate(usegmt=True)", DotNet: "R",
			JS: "date.toUTCString()"},
		{Name: "Readable", Example: lt.Format("Monday, 2 January 2006"),
			Go: "Monday, 2 January 2006", Java: "EEEE, d MMMM yyyy", Python: "%A, %-d %B %Y", DotNet: "dddd, d MMMM yyyy",
			JS: "date.toLocaleDateString('en-GB', { dateStyle: 'full' })", Note: "Python's %-d (no leading zero) doesn't work on Windows; use %#d there."},
	}
	return ps
}

// FormatsText writes the formats as aligned lines, for the CLI.
func FormatsText(fs []Format) string {
	w := 0
	for _, f := range fs {
		w = max(w, len(f.Name))
	}
	var b strings.Builder
	for _, f := range fs {
		fmt.Fprintf(&b, "%-*s  %s\n", w, f.Name, f.Value)
	}
	return strings.TrimRight(b.String(), "\n")
}
