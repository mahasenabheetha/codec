package timeutil

import (
	"testing"
	"time"
)

func TestEpochUnits(t *testing.T) {
	now := time.Date(2026, 2, 3, 12, 0, 0, 0, time.UTC)
	for _, c := range []struct{ in, kind, utc string }{
		{"1770120309", "seconds", "2026-02-03T12:05:09Z"},
		{"1770120309123", "milliseconds", "2026-02-03T12:05:09.123Z"},
		{"1770120309123456", "microseconds", "2026-02-03T12:05:09.123456Z"},
		{"1770120309123456789", "nanoseconds", "2026-02-03T12:05:09.123456789Z"},
		{"1770120309.5", "seconds", "2026-02-03T12:05:09.5Z"},
		{"0", "seconds", "1970-01-01T00:00:00Z"},
		{"-86400", "seconds", "1969-12-31T00:00:00Z"},
		{" 99999999999 ", "seconds", "5138-11-16T09:46:39Z"}, // the largest in seconds
	} {
		s, err := Parse(c.in, time.UTC, now)
		if err != nil || s.Kind != c.kind || s.UTC != c.utc {
			t.Errorf("%q: %+v %v", c.in, s, err)
		}
	}
}

func TestDatesAndZones(t *testing.T) {
	sthlm, _ := Zone("Europe/Stockholm")
	ny, _ := Zone("America/New_York")
	now := time.Date(2026, 2, 3, 12, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		in   string
		loc  *time.Location
		unix int64
	}{
		{"2026-02-03T12:05:09Z", sthlm, 1770120309},
		{"2026-02-03T13:05:09+01:00", ny, 1770120309},
		{"2026-02-03 13:05:09", sthlm, 1770120309}, // no zone: in the chosen one
		{"2026-02-03 07:05:09", ny, 1770120309},
		{"2026-02-03T12:05:09,000Z", time.UTC, 1770120309},
		{"Tue, 03 Feb 2026 12:05:09 GMT", sthlm, 1770120309},
		{"03/Feb/2026:13:05:09 +0100", time.UTC, 1770120309},
		{"2026-02-03 12:05:09.123 +0000 UTC", sthlm, 1770120309},
	} {
		s, err := Parse(c.in, c.loc, now)
		if err != nil || s.Unix != c.unix {
			t.Errorf("%q in %s: %+v %v", c.in, c.loc, s, err)
		}
	}
	s, _ := Parse("1770120309", sthlm, now)
	if s.Local != "2026-02-03T13:05:09+01:00" || s.Offset != "+01:00" || s.Zone != "Europe/Stockholm" || s.ISOWeek != "2026-W06" || s.HTTP != "Tue, 03 Feb 2026 12:05:09 GMT" {
		t.Errorf("forms: %+v", s)
	}
	// Summer time: the same zone, another offset.
	if s, _ = Parse("2026-07-01T12:00:00Z", sthlm, now); s.Offset != "+02:00" {
		t.Errorf("summer offset %s", s.Offset)
	}
	if _, err := Parse("yesterday-ish", time.UTC, now); err == nil {
		t.Error("nonsense parsed")
	}
	if _, err := Zone("Mars/Base"); err == nil {
		t.Error("unknown zone accepted")
	}
	if l, _ := Zone(""); l != time.UTC {
		t.Error("empty zone is not UTC")
	}
}

func TestRelative(t *testing.T) {
	now := time.Date(2026, 2, 3, 12, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		d    time.Duration
		want string
	}{
		{2 * time.Second, "just now"},
		{-90 * time.Second, "1 minute 30 seconds ago"},
		{3*24*time.Hour + 4*time.Hour + 5*time.Minute, "in 3 days 4 hours"},
		{-3 * 24 * time.Hour, "3 days ago"},
		{-400 * 24 * time.Hour, "1 year 1 month ago"},
	} {
		if got := Relative(now.Add(c.d), now); got != c.want {
			t.Errorf("%v: %q, want %q", c.d, got, c.want)
		}
	}
}
