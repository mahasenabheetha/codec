package cron

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Form is a schedule as simple choices, for building an expression
// without writing one.
type Form struct {
	// Kind: "minutes" (every N minutes), "hourly" (every N hours at a
	// minute), "daily", "weekly" (on chosen days) or "monthly" (on a day).
	Kind   string `json:"kind"`
	Every  int    `json:"every,omitempty"` // minutes, hourly; 1 when 0
	Minute int    `json:"minute"`          // hourly
	Time   string `json:"time,omitempty"`  // "HH:MM": daily, weekly, monthly
	Days   []int  `json:"days,omitempty"`  // weekly: 0 (Sunday) to 6
	Day    int    `json:"day,omitempty"`   // monthly: 1 to 31
}

// Build turns a form into an expression.
func Build(f Form) (string, error) {
	every := max(f.Every, 1)
	clock := func() (int, int, error) {
		h, m, ok := strings.Cut(f.Time, ":")
		hh, err1 := strconv.Atoi(h)
		mm, err2 := strconv.Atoi(m)
		if !ok || err1 != nil || err2 != nil || hh < 0 || hh > 23 || mm < 0 || mm > 59 {
			return 0, 0, fmt.Errorf("time %q must be HH:MM, 00:00 to 23:59", f.Time)
		}
		return hh, mm, nil
	}
	switch f.Kind {
	case "minutes":
		if every > 59 {
			return "", errors.New("every 1 to 59 minutes; use hourly for longer")
		}
		if every == 1 {
			return "* * * * *", nil
		}
		return fmt.Sprintf("*/%d * * * *", every), nil
	case "hourly":
		if f.Minute < 0 || f.Minute > 59 || every > 23 {
			return "", errors.New("minute 0 to 59, every 1 to 23 hours")
		}
		if every == 1 {
			return fmt.Sprintf("%d * * * *", f.Minute), nil
		}
		return fmt.Sprintf("%d */%d * * *", f.Minute, every), nil
	case "daily", "weekly", "monthly":
		h, m, err := clock()
		if err != nil {
			return "", err
		}
		switch f.Kind {
		case "daily":
			return fmt.Sprintf("%d %d * * *", m, h), nil
		case "weekly":
			if len(f.Days) == 0 {
				return "", errors.New("choose at least one day")
			}
			return fmt.Sprintf("%d %d * * %s", m, h, days(f.Days)), nil
		default:
			if f.Day < 1 || f.Day > 31 {
				return "", errors.New("day of month 1 to 31")
			}
			return fmt.Sprintf("%d %d %d * *", m, h, f.Day), nil
		}
	}
	return "", fmt.Errorf("unknown kind %q (minutes, hourly, daily, weekly, monthly)", f.Kind)
}

// days writes weekdays compactly: "1-5", "1,3,5", "0,6".
func days(ds []int) string {
	ds = slices.Clone(ds)
	slices.Sort(ds)
	ds = slices.Compact(ds)
	if len(ds) > 2 && ds[len(ds)-1]-ds[0] == len(ds)-1 {
		return fmt.Sprintf("%d-%d", ds[0], ds[len(ds)-1])
	}
	return strings.Join(ints(ds), ",")
}

// FormOf fills a form from a schedule when one of the form's kinds
// says the same thing; ok is false otherwise.
func FormOf(s *Schedule) (Form, bool) {
	f := s.Fields
	ms, hs := shapeOf(f[0].Values, fieldBounds[0]), shapeOf(f[1].Values, fieldBounds[1])
	ds, mos, ws := shapeOf(f[2].Values, fieldBounds[2]), shapeOf(f[3].Values, fieldBounds[3]), shapeOf(f[4].Values, fieldBounds[4])
	if !mos.all {
		return Form{}, false
	}
	clock := fmt.Sprintf("%02d:%02d", hs.from, ms.from)
	fixed := ms.single && hs.single
	switch {
	case ds.all && ws.all && hs.all && ms.all:
		return Form{Kind: "minutes", Every: 1}, true
	case ds.all && ws.all && hs.all && ms.step > 1 && ms.from == 0 && ms.toEnd:
		return Form{Kind: "minutes", Every: ms.step}, true
	case ds.all && ws.all && ms.single && hs.all:
		return Form{Kind: "hourly", Every: 1, Minute: ms.from}, true
	case ds.all && ws.all && ms.single && hs.step > 1 && hs.from == 0 && hs.toEnd:
		return Form{Kind: "hourly", Every: hs.step, Minute: ms.from}, true
	case ds.all && ws.all && fixed:
		return Form{Kind: "daily", Time: clock}, true
	case ds.all && fixed:
		return Form{Kind: "weekly", Time: clock, Days: f[4].Values}, true
	case ws.all && ds.single && fixed:
		return Form{Kind: "monthly", Time: clock, Day: ds.from}, true
	}
	return Form{}, false
}
