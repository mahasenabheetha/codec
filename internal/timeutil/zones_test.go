package timeutil

import (
	"strings"
	"testing"
	"time"
)

func TestZones(t *testing.T) {
	at := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	zs, err := Zones(at, []string{"Europe/Stockholm", "America/New_York", "Asia/Kolkata", "Pacific/Auckland", "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []struct {
		time, offset, abbr string
		dst                bool
		shift              int
	}{
		{"13:00", "+01:00", "CET", false, 0},
		{"08:00", "-04:00", "EDT", true, 0},
		{"17:30", "+05:30", "IST", false, 0},
		{"01:00", "+13:00", "NZDT", true, 1},
		{"12:00", "+00:00", "UTC", false, 0},
	} {
		z := zs[i]
		if z.Time != want.time || z.Offset != want.offset || z.Abbr != want.abbr || z.DST != want.dst || z.DayShift != want.shift {
			t.Errorf("%s: %+v, want %+v", z.Zone, z, want)
		}
	}
	// Stockholm's midnight is 23:00 UTC the day before: 04:30 in Kolkata.
	if m := zs[2].Minutes[0]; m != 4*60+30 {
		t.Errorf("Kolkata at Stockholm midnight: %d minutes", m)
	}
	if len(zs[0].Minutes) != 24 || zs[0].Minutes[13] != 13*60 {
		t.Errorf("base minutes: %v", zs[0].Minutes)
	}
	// New York's clocks go back at 02:00 on 1 November.
	if n := zs[1].Next; n == nil || n.At != "2026-11-01T01:00:00-05:00" || n.Change != "clocks go back 1 hour" || n.From != "-04:00" || n.To != "-05:00" {
		t.Errorf("New York next: %+v", n)
	}
	if zs[4].Next != nil || zs[2].Next != nil {
		t.Errorf("UTC and India have no DST: %+v %+v", zs[4].Next, zs[2].Next)
	}
	if _, err := Zones(at, []string{"Mars/Olympus"}); err == nil {
		t.Error("unknown zone accepted")
	}
	if _, err := Zones(at, make([]string, MaxZones+1)); err == nil {
		t.Error("too many zones accepted")
	}
}

func TestZonesOnDSTDay(t *testing.T) {
	// 29 March 2026: Stockholm skips 02:00–03:00.
	zs, _ := Zones(time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC), []string{"Europe/Stockholm", "UTC"})
	m := zs[0].Minutes
	if m[0] != 0 || m[1] != 60 || m[2] != 180 || m[23] != 0 {
		t.Errorf("Stockholm on its DST day: %v", m)
	}
}

func TestInstant(t *testing.T) {
	now := time.Date(2026, 10, 31, 22, 30, 0, 0, time.UTC)
	ny, _ := Zone("America/New_York")
	got, err := Instant("14:00", ny, now)
	if err != nil || !got.Equal(time.Date(2026, 10, 31, 18, 0, 0, 0, time.UTC)) {
		t.Errorf("14:00 in New York: %v (%v)", got, err)
	}
	if got, _ := Instant("", ny, now); !got.Equal(now) {
		t.Errorf("empty is now: %v", got)
	}
	if got, err := Instant("2026-02-03T14:05:09Z", ny, now); err != nil || got.Unix() != 1770127509 {
		t.Errorf("RFC 3339: %v (%v)", got, err)
	}
	for _, bad := range []string{"25:00", "12:61", "tomorrow"} {
		if _, err := Instant(bad, ny, now); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestFormats(t *testing.T) {
	at := time.Date(2026, 10, 31, 14, 5, 9, 123456789, time.UTC)
	sthlm, _ := Zone("Europe/Stockholm")
	got := map[string]string{}
	for _, f := range Formats(at, sthlm) {
		got[f.Name] = f.Value
	}
	for name, want := range map[string]string{
		"RFC 3339 / ISO 8601":             "2026-10-31T15:05:09+01:00",
		"RFC 3339 with milliseconds, UTC": "2026-10-31T14:05:09.123Z",
		"ISO week date":                   "2026-W44-6",
		"ISO ordinal date":                "2026-304",
		"RFC 2822 (email)":                "Sat, 31 Oct 2026 15:05:09 +0100",
		"HTTP date (RFC 9110)":            "Sat, 31 Oct 2026 14:05:09 GMT",
		"Unix seconds":                    "1793455509",
		"Unix nanoseconds":                "1793455509123456789",
	} {
		if got[name] != want {
			t.Errorf("%s: %q, want %q", name, got[name], want)
		}
	}
	if !strings.Contains(unixNano(time.Date(2300, 1, 1, 0, 0, 0, 0, time.UTC)), "out of range") {
		t.Error("nanoseconds past 2262")
	}
	// Each Go layout writes its own example.
	for _, p := range Patterns(at, sthlm) {
		if strings.Contains(p.Go, "(") || strings.HasPrefix(p.Go, "http.") {
			continue
		}
		if got := at.In(sthlm).Format(p.Go); got != p.Example {
			t.Errorf("%s: layout %q writes %q, example %q", p.Name, p.Go, got, p.Example)
		}
	}
}
