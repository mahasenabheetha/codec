package timeutil

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ZoneTime is one instant as a zone sees it.
type ZoneTime struct {
	Zone   string `json:"zone"`
	Date   string `json:"date"`   // "Sat 31 Oct 2026"
	Time   string `json:"time"`   // "14:05"
	Abbr   string `json:"abbr"`   // "CET", or the offset where the zone has no name
	Offset string `json:"offset"` // "+01:00"
	DST    bool   `json:"dst"`
	// DayShift is the calendar day against the first zone's: -1, 0, +1.
	DayShift int `json:"dayShift"`
	// Minutes is the local time, in minutes after midnight, at each whole
	// hour of the first zone's day: 24 entries (23 or 25 hours on a DST
	// day are still 24 steps of an hour from its midnight).
	Minutes []int       `json:"minutes"`
	Next    *Transition `json:"next,omitempty"` // the next DST change, within a year
}

// Transition is a change of a zone's offset.
type Transition struct {
	At     string `json:"at"`     // RFC 3339, in the zone
	From   string `json:"from"`   // offset before, "+01:00"
	To     string `json:"to"`     // offset after
	Change string `json:"change"` // "clocks go forward 1 hour"
}

// MaxZones bounds how many zones Zones shows.
const MaxZones = 12

var clock = regexp.MustCompile(`^(\d{1,2}):(\d{2})(?::(\d{2}))?$`)

// Instant reads input as Parse does, and also a bare time of day
// ("14:00"), which is today in loc; empty is now.
func Instant(input string, loc *time.Location, now time.Time) (time.Time, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return now, nil
	}
	if m := clock.FindStringSubmatch(s); m != nil {
		var h, min, sec int
		fmt.Sscan(m[1], &h)
		fmt.Sscan(m[2], &min)
		if m[3] != "" {
			fmt.Sscan(m[3], &sec)
		}
		if h > 23 || min > 59 || sec > 59 {
			return time.Time{}, fmt.Errorf("%q isn't a time of day", s)
		}
		n := now.In(loc)
		return time.Date(n.Year(), n.Month(), n.Day(), h, min, sec, 0, loc), nil
	}
	st, err := Parse(s, loc, now)
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339Nano, st.UTC)
}

// Zones shows t in each zone; the first is the one the strip's hours
// follow. Unknown zone names are an error naming them.
func Zones(t time.Time, names []string) ([]ZoneTime, error) {
	if len(names) > MaxZones {
		return nil, fmt.Errorf("show at most %d zones", MaxZones)
	}
	locs := make([]*time.Location, len(names))
	for i, n := range names {
		loc, err := Zone(n)
		if err != nil {
			return nil, err
		}
		locs[i] = loc
	}
	out := make([]ZoneTime, len(locs))
	if len(locs) == 0 {
		return out, nil
	}
	base := t.In(locs[0])
	midnight := time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, locs[0])
	for i, loc := range locs {
		lt := t.In(loc)
		abbr, _ := lt.Zone()
		if abbr == "" || abbr[0] == '+' || abbr[0] == '-' {
			abbr = lt.Format("-07:00")
		}
		z := ZoneTime{
			Zone:     loc.String(),
			Date:     lt.Format("Mon 2 Jan 2006"),
			Time:     lt.Format("15:04"),
			Abbr:     abbr,
			Offset:   lt.Format("-07:00"),
			DST:      lt.IsDST(),
			DayShift: dayDiff(lt, base),
			Minutes:  make([]int, 24),
			Next:     nextChange(lt),
		}
		for h := range 24 {
			at := midnight.Add(time.Duration(h) * time.Hour).In(loc)
			z.Minutes[h] = at.Hour()*60 + at.Minute()
		}
		out[i] = z
	}
	return out, nil
}

// dayDiff is a's calendar day against b's, whatever their zones.
func dayDiff(a, b time.Time) int {
	da := time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, time.UTC)
	db := time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, time.UTC)
	return int(da.Sub(db).Hours() / 24)
}

// nextChange finds the zone's next offset change after t, within a
// year: a day-by-day search, then to the minute.
func nextChange(t time.Time) *Transition {
	_, off := t.Zone()
	lo := t
	for d := 1; d <= 366; d++ {
		hi := t.AddDate(0, 0, d)
		if _, o := hi.Zone(); o == off {
			lo = hi
			continue
		}
		for hi.Sub(lo) > time.Minute {
			mid := lo.Add(hi.Sub(lo) / 2)
			if _, o := mid.Zone(); o == off {
				lo = mid
			} else {
				hi = mid
			}
		}
		at := hi.Truncate(time.Minute)
		if _, o := at.Zone(); o == off {
			at = at.Add(time.Minute)
		}
		_, to := at.Zone()
		diff := time.Duration(to-off) * time.Second
		dir := "forward"
		if diff < 0 {
			dir, diff = "back", -diff
		}
		return &Transition{
			At:     at.Format(time.RFC3339),
			From:   t.Format("-07:00"),
			To:     at.Format("-07:00"),
			Change: fmt.Sprintf("clocks go %s %s", dir, span(diff)),
		}
	}
	return nil
}

func span(d time.Duration) string {
	h, m := int(d.Hours()), int(d.Minutes())%60
	switch {
	case m == 0 && h == 1:
		return "1 hour"
	case m == 0:
		return fmt.Sprintf("%d hours", h)
	case h == 0:
		return fmt.Sprintf("%d minutes", m)
	}
	return fmt.Sprintf("%dh %02dm", h, m)
}
