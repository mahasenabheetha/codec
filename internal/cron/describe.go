package cron

import (
	"fmt"
	"strconv"
	"strings"
)

var monthNames = []string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
var dayNames = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

// shape is what a field's values look like: all of them, one, a run
// (from–to), or every step from one value, else a plain list.
type shape struct {
	all, single bool
	from, to    int // first and last value
	step        int // 1 for a run; 0 for a list
	toEnd       bool
}

func shapeOf(vals []int, b bounds) shape {
	sh := shape{from: vals[0], to: vals[len(vals)-1]}
	switch {
	case len(vals) == b.max-b.min+1:
		sh.all, sh.step = true, 1
	case len(vals) == 1:
		sh.single = true
	default:
		step := vals[1] - vals[0]
		for i := 2; i < len(vals); i++ {
			if vals[i]-vals[i-1] != step {
				step = 0
				break
			}
		}
		sh.step = step
		sh.toEnd = step > 0 && sh.to+step > b.max
	}
	return sh
}

// Describe says when the schedule runs, in plain words: "Every 15
// minutes, from 02:00 to 06:59, Monday to Friday".
func (s *Schedule) Describe() string {
	f := s.Fields
	min, hour := f[0].Values, f[1].Values
	ms, hs := shapeOf(min, fieldBounds[0]), shapeOf(hour, fieldBounds[1])
	at := false // the phrase names clock times ("At 02:30")
	var when string
	hourSpan := func() string {
		return fmt.Sprintf("from %02d:00 to %02d:59", hs.from, hs.to)
	}
	switch {
	case ms.all && hs.all:
		when = "Every minute"
	case ms.all && hs.step == 1:
		when = "Every minute, " + hourSpan()
	case ms.step > 1 && ms.from == 0 && ms.toEnd && hs.all:
		when = fmt.Sprintf("Every %d minutes", ms.step)
	case ms.step > 1 && ms.from == 0 && ms.toEnd && (hs.step == 1 || hs.single):
		when = fmt.Sprintf("Every %d minutes, %s", ms.step, hourSpan())
	case ms.single && hs.all:
		if ms.from == 0 {
			when = "Every hour, on the hour"
		} else {
			when = fmt.Sprintf("Every hour at minute %d", ms.from)
		}
	case ms.single && hs.step > 1 && hs.from == 0 && hs.toEnd:
		when = fmt.Sprintf("Every %d hours at minute %d", hs.step, ms.from)
	case ms.single && hs.step == 1 && len(hour) > 2:
		when = fmt.Sprintf("Every hour from %02d:%02d to %02d:%02d", hs.from, ms.from, hs.to, ms.from)
	case hs.single && ms.step == 1:
		when = fmt.Sprintf("Every minute from %02d:%02d to %02d:%02d", hs.from, ms.from, hs.from, ms.to)
	case len(min)*len(hour) <= 6:
		var times []string
		for _, h := range hour {
			for _, m := range min {
				times = append(times, fmt.Sprintf("%02d:%02d", h, m))
			}
		}
		when, at = "At "+join(times), true
	default:
		when = minutePhrase(min, ms) + ", " + hourPhrase(hour, hs)
	}

	parts := []string{when}
	dom, month, dow := f[2], f[3], f[4]
	ds, mos, ws := shapeOf(dom.Values, fieldBounds[2]), shapeOf(month.Values, fieldBounds[3]), shapeOf(dow.Values, fieldBounds[4])
	// "on 1 January": one day of one month reads as a date.
	if ds.single && mos.single && ws.all {
		parts = append(parts, fmt.Sprintf("on %d %s", ds.from, monthNames[mos.from-1]))
	} else {
		var days []string
		if !ds.all {
			days = append(days, domPhrase(dom.Values, ds))
		}
		if !ws.all {
			days = append(days, dowPhrase(dow.Values, ws))
		}
		switch {
		case len(days) == 2 && !dom.Star && !dow.Star:
			parts = append(parts, days[0]+" or "+days[1]) // either day runs
		case len(days) == 2:
			parts = append(parts, days[0]+", if it is "+strings.TrimPrefix(days[1], "on "))
		case len(days) == 1:
			parts = append(parts, days[0])
		}
		if !mos.all {
			parts = append(parts, monthPhrase(month.Values, mos))
		}
	}
	if len(parts) == 1 && at {
		parts = append(parts, "every day")
	}
	return strings.Join(parts, ", ")
}

// Meaning describes one field on its own, for a field-by-field table.
func (f Field) Meaning() string {
	for i, b := range fieldBounds {
		if b.name != f.Name {
			continue
		}
		sh := shapeOf(f.Values, b)
		switch i {
		case 2:
			if sh.all {
				return "every day of the month"
			}
			return strings.TrimSuffix(strings.TrimPrefix(domPhrase(f.Values, sh), "on "), " of the month")
		case 3:
			if sh.all {
				return "every month"
			}
			return strings.TrimPrefix(monthPhrase(f.Values, sh), "in ")
		case 4:
			if sh.all {
				return "every day of the week"
			}
			return strings.TrimPrefix(dowPhrase(f.Values, sh), "on ")
		}
		return setPhrase(f.Values, sh, b, b.name)
	}
	return ""
}

// setPhrase describes minutes or hours: "every 15 minutes", "minutes 0
// to 29", "hours 1, 3 and 5".
func setPhrase(vals []int, sh shape, b bounds, unit string) string {
	switch {
	case sh.all:
		return "every " + unit
	case sh.single:
		return unit + " " + strconv.Itoa(sh.from)
	case sh.step > 1 && sh.from == b.min && sh.toEnd:
		return fmt.Sprintf("every %d %ss", sh.step, unit)
	case sh.step > 1 && sh.toEnd:
		return fmt.Sprintf("every %d %ss from %s %d", sh.step, unit, unit, sh.from)
	case sh.step > 1:
		return fmt.Sprintf("every %d %ss from %d to %d", sh.step, unit, sh.from, sh.to)
	case sh.step == 1:
		return fmt.Sprintf("%ss %d to %d", unit, sh.from, sh.to)
	}
	return unit + "s " + join(ints(vals))
}

func domPhrase(vals []int, sh shape) string {
	switch {
	case sh.single:
		return fmt.Sprintf("on day %d of the month", sh.from)
	case sh.step > 1 && sh.from == 1 && sh.toEnd:
		return fmt.Sprintf("on every %s day of the month", ordinal(sh.step))
	case sh.step == 1:
		return fmt.Sprintf("on days %d to %d of the month", sh.from, sh.to)
	}
	return "on days " + join(ints(vals)) + " of the month"
}

func dowPhrase(vals []int, sh shape) string {
	if sh.step == 1 && !sh.single {
		return dayNames[sh.from] + " to " + dayNames[sh.to]
	}
	// Monday first, as people list a week; Sunday at the end.
	order := vals
	if vals[0] == 0 && len(vals) > 1 {
		order = append(append([]int{}, vals[1:]...), 0)
	}
	names := make([]string, len(order))
	for i, v := range order {
		names[i] = dayNames[v]
	}
	return "on " + join(names)
}

func monthPhrase(vals []int, sh shape) string {
	names := make([]string, len(vals))
	for i, v := range vals {
		names[i] = monthNames[v-1]
	}
	switch {
	case sh.step > 1 && sh.from == 1 && sh.toEnd:
		return fmt.Sprintf("every %d months (%s)", sh.step, join(names))
	case sh.step == 1 && !sh.single:
		return names[0] + " to " + names[len(names)-1]
	}
	return "in " + join(names)
}

func ints(vals []int) []string {
	out := make([]string, len(vals))
	for i, v := range vals {
		out[i] = strconv.Itoa(v)
	}
	return out
}

// join lists words as people do: "a", "a and b", "a, b and c".
func join(words []string) string {
	switch len(words) {
	case 0:
		return ""
	case 1:
		return words[0]
	}
	return strings.Join(words[:len(words)-1], ", ") + " and " + words[len(words)-1]
}

func ordinal(n int) string {
	suffix := "th"
	switch {
	case n%100 >= 11 && n%100 <= 13:
	case n%10 == 1:
		suffix = "st"
	case n%10 == 2:
		suffix = "nd"
	case n%10 == 3:
		suffix = "rd"
	}
	return strconv.Itoa(n) + suffix
}

// minutePhrase and hourPhrase describe the two time fields when no
// simpler sentence fits: "Every 20 minutes, during hours 1, 3 and 5".
func minutePhrase(vals []int, sh shape) string {
	switch {
	case sh.all:
		return "Every minute"
	case sh.single:
		return fmt.Sprintf("At minute %d", sh.from)
	case sh.step > 1 && sh.from == 0 && sh.toEnd:
		return fmt.Sprintf("Every %d minutes", sh.step)
	case sh.step == 1:
		return fmt.Sprintf("At minutes %d to %d", sh.from, sh.to)
	}
	return "At minutes " + join(ints(vals))
}

func hourPhrase(vals []int, sh shape) string {
	switch {
	case sh.all:
		return "every hour"
	case sh.single:
		return fmt.Sprintf("during hour %d (%02d:00 to %02d:59)", sh.from, sh.from, sh.from)
	case sh.step == 1:
		return fmt.Sprintf("from %02d:00 to %02d:59", sh.from, sh.to)
	case sh.step > 1 && sh.from == 0 && sh.toEnd:
		return fmt.Sprintf("every %d hours", sh.step)
	}
	return "during hours " + join(ints(vals))
}
