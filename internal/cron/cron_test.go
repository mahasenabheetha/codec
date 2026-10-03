package cron

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

func values(t *testing.T, expr string, field int) []int {
	t.Helper()
	s, err := Parse(expr)
	if err != nil {
		t.Fatalf("%q: %v", expr, err)
	}
	return s.Fields[field].Values
}

func TestFields(t *testing.T) {
	for _, c := range []struct {
		expr  string
		field int
		want  []int
	}{
		{"*/15 * * * *", 0, []int{0, 15, 30, 45}},
		{"5/20 * * * *", 0, []int{5, 25, 45}},
		{"0-10/5,30 * * * *", 0, []int{0, 5, 10, 30}},
		{"0 9-17/4 * * *", 1, []int{9, 13, 17}},
		{"0 0 1,15 * *", 2, []int{1, 15}},
		{"0 0 * JAN-MAR,dec *", 3, []int{1, 2, 3, 12}},
		{"0 0 * * MON-FRI", 4, []int{1, 2, 3, 4, 5}},
		{"0 0 * * 7", 4, []int{0}},         // 7 is Sunday
		{"0 0 * * 5-7", 4, []int{0, 5, 6}}, // Friday to Sunday
		{"0 0 * * sun,Thu", 4, []int{0, 4}},
	} {
		if got := values(t, c.expr, c.field); !slices.Equal(got, c.want) {
			t.Errorf("%q field %d = %v, want %v", c.expr, c.field, got, c.want)
		}
	}
}

func TestShortcutsAndZone(t *testing.T) {
	s, err := Parse("@weekly")
	if err != nil || s.Standard != "0 0 * * 0" || !slices.Equal(s.Fields[4].Values, []int{0}) {
		t.Errorf("@weekly: %+v %v", s, err)
	}
	s, err = Parse("CRON_TZ=Europe/Stockholm 0 2 * * *")
	if err != nil || s.Zone != "Europe/Stockholm" || s.Fields[1].Values[0] != 2 {
		t.Errorf("CRON_TZ: %+v %v", s, err)
	}
}

func TestErrors(t *testing.T) {
	for _, c := range []struct{ expr, dialect, want string }{
		{"0 0 12 * * ?", "Quartz", "seconds"},
		{"0 0 12 ? * MON", "Quartz", "starts with seconds"}, // 6 fields wins over ?
		{"0 12 ? * MON", "Quartz", "? (no specific value)"},
		{"0 12 L * *", "Quartz", "L, W or #"},
		{"0 12 * * 5#2", "Quartz", "L, W or #"},
		{"0 0 15W * *", "Quartz", "L, W or #"},
		{"H/15 * * * *", "Jenkins", "Jenkins"},
		{"0 H(0-7) * * *", "Jenkins", "Jenkins"},
		{"61 * * * *", "", "minute: 61 is out of range 0–59"},
		{"0 0 * * 8", "", "day of week: 8 is out of range 0–7"},
		{"0 0 * FOO *", "", `"FOO" is not a number or a name (JAN–DEC)`},
		{"0 17-9 * * *", "", "goes backwards"},
		{"*/0 * * * *", "", "step"},
		{"* * * *", "", "4 fields"},
		{"@reboot", "", "no schedule"},
		{"CRON_TZ=Mars/Base * * * * *", "", "unknown time zone"},
	} {
		_, err := Parse(c.expr)
		var e *Error
		if !errors.As(err, &e) || e.Dialect != c.dialect || !strings.Contains(e.Msg, c.want) {
			t.Errorf("%q: %v (dialect %q)", c.expr, err, e.Dialect)
		}
	}
	// THU and WED contain H and W, which are not Jenkins or Quartz here.
	if _, err := Parse("0 9 * * WED,THU"); err != nil {
		t.Errorf("names: %v", err)
	}
}

func TestDescribe(t *testing.T) {
	for _, c := range []struct{ expr, want string }{
		{"* * * * *", "Every minute"},
		{"*/15 * * * *", "Every 15 minutes"},
		{"*/15 2-6 * * 1-5", "Every 15 minutes, from 02:00 to 06:59, Monday to Friday"},
		{"0 * * * *", "Every hour, on the hour"},
		{"5 */2 * * *", "Every 2 hours at minute 5"},
		{"30 9-17 * * *", "Every hour from 09:30 to 17:30"},
		{"30 2 * * *", "At 02:30, every day"},
		{"0 9,13,17 * * *", "At 09:00, 13:00 and 17:00, every day"},
		{"@yearly", "At 00:00, on 1 January"},
		{"0 0 1,15 * *", "At 00:00, on days 1 and 15 of the month"},
		{"0 0 1 * 1", "At 00:00, on day 1 of the month or on Monday"},
		{"0 0 * * 0,6", "At 00:00, on Saturday and Sunday"},
		{"0 0 1 */3 *", "At 00:00, on day 1 of the month, every 3 months (January, April, July and October)"},
		{"0 6 * 6-8 *", "At 06:00, June to August"},
		{"0,20,40 1,3,5 * * *", "Every 20 minutes, during hours 1, 3 and 5"},
		{"0,25 1,3,5,7 * * *", "At minutes 0 and 25, during hours 1, 3, 5 and 7"},
		{"0-4 9 * * *", "Every minute from 09:00 to 09:04"},
		{"0-4,30 9 * * *", "At 09:00, 09:01, 09:02, 09:03, 09:04 and 09:30, every day"},
		{"0-4,30 9,10 * * *", "At minutes 0, 1, 2, 3, 4 and 30, from 09:00 to 10:59"},
		{"0 0 */2 * 1", "At 00:00, on every 2nd day of the month, if it is Monday"},
	} {
		s, err := Parse(c.expr)
		if err != nil {
			t.Fatalf("%q: %v", c.expr, err)
		}
		if got := s.Describe(); got != c.want {
			t.Errorf("%q:\n got %s\nwant %s", c.expr, got, c.want)
		}
	}
	s, _ := Parse("*/10 9-17 * JAN,JUL MON-FRI")
	var meanings []string
	for _, f := range s.Fields {
		meanings = append(meanings, f.Meaning())
	}
	if got := strings.Join(meanings, " | "); got != "every 10 minutes | hours 9 to 17 | every day of the month | every 6 months (January and July) | Monday to Friday" {
		t.Errorf("meanings: %s", got)
	}
}

func TestNext(t *testing.T) {
	utc := time.UTC
	from := time.Date(2026, 1, 30, 23, 0, 0, 0, utc) // a Friday
	s, _ := Parse("0 9 * * 1-5")
	runs, _ := s.Next(from, 3, utc)
	want := []string{"2026-02-02 09:00", "2026-02-03 09:00", "2026-02-04 09:00"}
	for i, r := range runs {
		if r.Format("2006-01-02 15:04") != want[i] {
			t.Errorf("run %d = %s, want %s", i, r.Format("2006-01-02 15:04"), want[i])
		}
	}
	// Day of month or day of week: the 1st (a Sunday) and Mondays.
	s, _ = Parse("0 0 1 * MON")
	runs, _ = s.Next(time.Date(2026, 2, 27, 0, 0, 0, 0, utc), 3, utc)
	if got := fmtDays(runs); got != "2026-03-01 2026-03-02 2026-03-09" {
		t.Errorf("dom or dow: %s", got)
	}
	// 29 February comes every four years.
	s, _ = Parse("0 0 29 2 *")
	if runs, _ = s.Next(from, 2, utc); fmtDays(runs) != "2028-02-29 2032-02-29" {
		t.Errorf("leap day: %s", fmtDays(runs))
	}
	// 30 February never comes.
	s, _ = Parse("0 0 30 2 *")
	if runs, _ = s.Next(from, 1, utc); len(runs) != 0 {
		t.Errorf("30 Feb: %v", runs)
	}
}

func TestNextDST(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Stockholm")
	if err != nil {
		t.Fatal(err)
	}
	// Clocks go forward on 29 March 2026 at 02:00: 02:30 doesn't happen.
	s, _ := Parse("30 2 * * *")
	runs, skipped := s.Next(time.Date(2026, 3, 28, 0, 0, 0, 0, loc), 2, loc)
	if fmtDays(runs) != "2026-03-28 2026-03-30" || len(skipped) != 1 || skipped[0].Wall != "2026-03-29 02:30" {
		t.Errorf("spring: %s %+v", fmtDays(runs), skipped)
	}
	// Clocks go back on 25 October 2026 at 03:00: 02:00–02:59 happens
	// twice, and an hourly job runs once at 02:00.
	s, _ = Parse("0 * * * *")
	runs, _ = s.Next(time.Date(2026, 10, 25, 0, 30, 0, 0, loc), 4, loc)
	var walls []string
	for _, r := range runs {
		walls = append(walls, r.Format("15:04 MST"))
	}
	if got := strings.Join(walls, " "); got != "01:00 CEST 02:00 CEST 03:00 CET 04:00 CET" {
		t.Errorf("autumn: %s", got)
	}
}

func fmtDays(ts []time.Time) string {
	var out []string
	for _, t := range ts {
		out = append(out, t.Format("2006-01-02"))
	}
	return strings.Join(out, " ")
}

func TestBuild(t *testing.T) {
	for _, c := range []struct {
		form Form
		want string
	}{
		{Form{Kind: "minutes", Every: 15}, "*/15 * * * *"},
		{Form{Kind: "minutes"}, "* * * * *"},
		{Form{Kind: "hourly", Minute: 30}, "30 * * * *"},
		{Form{Kind: "hourly", Minute: 0, Every: 6}, "0 */6 * * *"},
		{Form{Kind: "daily", Time: "02:30"}, "30 2 * * *"},
		{Form{Kind: "weekly", Time: "09:00", Days: []int{5, 1, 2, 3, 4}}, "0 9 * * 1-5"},
		{Form{Kind: "weekly", Time: "09:00", Days: []int{0, 6}}, "0 9 * * 0,6"},
		{Form{Kind: "monthly", Time: "00:00", Day: 1}, "0 0 1 * *"},
	} {
		got, err := Build(c.form)
		if err != nil || got != c.want {
			t.Errorf("%+v = %q %v, want %q", c.form, got, err, c.want)
			continue
		}
		// And back: the expression fills the same form.
		s, _ := Parse(got)
		f, ok := FormOf(s)
		if !ok || f.Kind != c.form.Kind {
			t.Errorf("%q back to a form: %+v %v", got, f, ok)
		}
	}
	for _, f := range []Form{{Kind: "daily", Time: "25:00"}, {Kind: "weekly", Time: "09:00"}, {Kind: "monthly", Time: "09:00", Day: 32}, {Kind: "yearly"}} {
		if _, err := Build(f); err == nil {
			t.Errorf("%+v: no error", f)
		}
	}
	if s, _ := Parse("0,30 9 * * *"); func() bool { _, ok := FormOf(s); return ok }() {
		t.Error("two minutes don't fit a form")
	}
}

// Findings from the 2.3.0 review.
func TestReviewCases(t *testing.T) {
	for _, c := range []struct {
		expr string
		want []int
	}{
		{"0 0 * * 7/2", []int{0, 2, 4, 6}}, // was a panic: no values
		{"0 0 * * 2-7/2", []int{2, 4, 6}},  // the step never reaches 7
		{"0 0 * * 6-7/2", []int{6}},
		{"0 0 * * 5-7", []int{0, 5, 6}},
		{"0 0 * * 7-7", []int{0}},
	} {
		s, err := Parse(c.expr)
		if err != nil || !slices.Equal(s.Fields[4].Values, c.want) {
			t.Errorf("%q: %v %v, want %v", c.expr, s, err, c.want)
			continue
		}
		s.Describe() // no panic
	}
	if _, err := Parse("CRON_TZ=Local 0 9 * * *"); err == nil {
		t.Error("CRON_TZ=Local accepted: it would mean whichever machine reads it")
	}
	// Uneven steps read as lists, not "every N".
	for expr, want := range map[string]string{
		"0,45 * * * *": "At minutes 0 and 45, every hour",
		"*/45 * * * *": "At minutes 0 and 45, every hour",
		"0 */5 * * *":  "At 00:00, 05:00, 10:00, 15:00 and 20:00, every day",
		"0 0 */2 * *":  "At 00:00, on every 2nd day of the month",
	} {
		s, _ := Parse(expr)
		if got := s.Describe(); got != want {
			t.Errorf("%q: %q, want %q", expr, got, want)
		}
	}
	// Lord Howe's clocks go back 30 minutes: 01:30-01:59 happens twice,
	// and runs once, the first time.
	lh, _ := time.LoadLocation("Australia/Lord_Howe")
	s, _ := Parse("45 1 * * *")
	runs, _ := s.Next(time.Date(2026, 4, 4, 12, 0, 0, 0, lh), 1, lh)
	if _, off := runs[0].Zone(); off != 11*3600 {
		t.Errorf("Lord Howe: %v (offset %d)", runs[0], off)
	}
	if _, err := Build(Form{Kind: "weekly", Time: "09:00", Days: []int{8}}); err == nil {
		t.Error("day 8 accepted")
	}
}
