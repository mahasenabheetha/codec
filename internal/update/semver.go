package update

import (
	"strconv"
	"strings"
)

// semver is a parsed vMAJOR.MINOR.PATCH[-PRERELEASE][+BUILD] version.
type semver struct {
	nums [3]int
	pre  []string // dot-separated identifiers; empty for a release
}

// parse reads a version with or without the leading v; ok is false for
// anything else, such as "dev".
func parse(v string) (semver, bool) {
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i] // build metadata doesn't count
	}
	var s semver
	core := v
	if i := strings.IndexByte(v, '-'); i >= 0 {
		core = v[:i]
		pre := v[i+1:]
		if pre == "" {
			return semver{}, false
		}
		s.pre = strings.Split(pre, ".")
		for _, id := range s.pre {
			if id == "" {
				return semver{}, false
			}
		}
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return semver{}, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || (len(p) > 1 && p[0] == '0') {
			return semver{}, false
		}
		s.nums[i] = n
	}
	return s, true
}

// compare orders a and b by Semantic Versioning precedence: -1, 0 or 1.
// A pre-release comes before its release; pre-release identifiers
// compare numerically when both are numbers, else as text, numbers
// first, and a shorter list first when the rest is equal.
func compare(a, b semver) int {
	for i := range a.nums {
		if c := cmpInt(a.nums[i], b.nums[i]); c != 0 {
			return c
		}
	}
	switch {
	case len(a.pre) == 0 && len(b.pre) == 0:
		return 0
	case len(a.pre) == 0:
		return 1
	case len(b.pre) == 0:
		return -1
	}
	for i := 0; i < len(a.pre) && i < len(b.pre); i++ {
		x, y := a.pre[i], b.pre[i]
		xn, xerr := strconv.Atoi(x)
		yn, yerr := strconv.Atoi(y)
		switch {
		case xerr == nil && yerr == nil:
			if c := cmpInt(xn, yn); c != 0 {
				return c
			}
		case xerr == nil:
			return -1
		case yerr == nil:
			return 1
		default:
			if c := strings.Compare(x, y); c != 0 {
				return c
			}
		}
	}
	return cmpInt(len(a.pre), len(b.pre))
}

func cmpInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
