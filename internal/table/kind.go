package table

import (
	"strings"
	"time"
)

// kind is what a value looks like; a column's kind is its values' merged.
type kind uint8

const (
	empty kind = iota
	integer
	number
	boolean
	date
	text
)

func (k kind) String() string {
	return [...]string{"empty", "integer", "number", "boolean", "date", "text"}[k]
}

// merge is the kind of a column holding both: integers and numbers are
// numbers, anything else mixed is text.
func merge(a, b kind) kind {
	switch {
	case a == empty || a == b:
		return b
	case b == empty:
		return a
	case a == integer && b == number || a == number && b == integer:
		return number
	}
	return text
}

var dateLayouts = []string{
	"2006-01-02",
	time.RFC3339Nano,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"2006-01-02T15:04",
}

// kindOf says what one value looks like, ignoring spaces around it.
// Numbers are JSON's (no leading zeros, so 007 and ZIP codes stay text).
func kindOf(v string) kind {
	v = strings.TrimSpace(v)
	switch {
	case v == "":
		return empty
	case strings.EqualFold(v, "true") || strings.EqualFold(v, "false"):
		return boolean
	}
	if k := numberKind(v); k != empty {
		return k
	}
	if len(v) >= 10 && v[4] == '-' {
		for _, l := range dateLayouts {
			if _, err := time.Parse(l, v); err == nil {
				return date
			}
		}
	}
	return text
}

// numberKind is integer or number when v is a JSON number, else empty.
func numberKind(v string) kind {
	i := 0
	if i < len(v) && v[i] == '-' {
		i++
	}
	digits := func() int {
		n := 0
		for i < len(v) && v[i] >= '0' && v[i] <= '9' {
			i++
			n++
		}
		return n
	}
	start := i
	n := digits()
	if n == 0 || n > 1 && v[start] == '0' {
		return empty
	}
	k := integer
	if i < len(v) && v[i] == '.' {
		i++
		if digits() == 0 {
			return empty
		}
		k = number
	}
	if i < len(v) && (v[i] == 'e' || v[i] == 'E') {
		i++
		if i < len(v) && (v[i] == '+' || v[i] == '-') {
			i++
		}
		if digits() == 0 {
			return empty
		}
		k = number
	}
	if i != len(v) {
		return empty
	}
	return k
}
