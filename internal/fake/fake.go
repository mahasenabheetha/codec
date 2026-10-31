// Package fake makes placeholder text and rows of made-up data for tests
// and demos (3.8.0): lorem ipsum, and rows from columns you pick, as
// JSON, NDJSON, CSV, YAML or SQL INSERT statements. Everything comes
// from a seeded generator, so a seed gives the same output again.
// Nothing points at real people or places one could reach: emails use
// the reserved example domains, IPs the documentation ranges, phone
// numbers the fictional 555-01xx range.
package fake

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"
)

// Limits on one run.
const (
	MaxRows    = 10000
	MaxColumns = 50
	MaxLorem   = 1000
)

// rng is the seeded source every value comes from.
type rng struct{ *rand.Rand }

func newRNG(seed uint64) rng {
	return rng{rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))}
}

func (r rng) pick(list []string) string { return list[r.IntN(len(list))] }

// NewSeed is a fresh seed for a run that didn't ask for one; it comes
// back with the output so the run can be repeated.
func NewSeed() uint64 { return rand.Uint64() % 1_000_000_000 }

// Lorem writes n words, sentences or paragraphs of lorem ipsum; classic
// starts with "Lorem ipsum dolor sit amet".
func Lorem(unit string, n int, seed uint64, classic bool) (string, error) {
	if n < 1 || n > MaxLorem {
		return "", fmt.Errorf("count is 1 to %d", MaxLorem)
	}
	r := newRNG(seed)
	start := classic
	words := func(k int) []string {
		out := make([]string, 0, k)
		if start {
			out = append(out, loremStart[:min(k, len(loremStart))]...)
			start = false
		}
		for len(out) < k {
			out = append(out, r.pick(loremWords))
		}
		return out
	}
	sentence := func() string {
		ws := words(6 + r.IntN(10))
		if len(ws) > 7 && r.IntN(3) == 0 { // a comma now and then
			i := 2 + r.IntN(len(ws)-4)
			ws[i] += ","
		}
		s := strings.Join(ws, " ")
		return strings.ToUpper(s[:1]) + s[1:] + "."
	}
	paragraph := func() string {
		ss := make([]string, 4+r.IntN(4))
		for i := range ss {
			ss[i] = sentence()
		}
		return strings.Join(ss, " ")
	}
	switch unit {
	case "words":
		return strings.Join(words(n), " "), nil
	case "sentences":
		ss := make([]string, n)
		for i := range ss {
			ss[i] = sentence()
		}
		return strings.Join(ss, " "), nil
	case "", "paragraphs":
		ps := make([]string, n)
		for i := range ps {
			ps[i] = paragraph()
		}
		return strings.Join(ps, "\n\n"), nil
	}
	return "", fmt.Errorf("unit is words, sentences or paragraphs, not %q", unit)
}

// Column is one column of made-up rows: its name, its kind, and the
// kind's arguments (a range, decimals, choices).
type Column struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
	// Min and Max bound integer, number, date and datetime (dates as
	// YYYY-MM-DD); empty means the kind's default.
	Min      string   `json:"min,omitempty"`
	Max      string   `json:"max,omitempty"`
	Decimals int      `json:"decimals,omitempty"`
	Choices  []string `json:"choices,omitempty"`
	// Blank is the share of rows (0 to 100) left empty: null in JSON
	// and YAML, NULL in SQL, an empty cell in CSV.
	Blank int `json:"blank,omitempty"`
}

// Kinds are the column kinds, with what each makes.
var Kinds = []struct{ Kind, About string }{
	{"id", "1, 2, 3…"},
	{"uuid", "a random version 4 UUID"},
	{"firstName", "a first name"},
	{"lastName", "a family name"},
	{"name", "first and family name"},
	{"email", "the row's person at example.com, .org or .net"},
	{"username", "from the row's person: ann.lee42"},
	{"phone", "+1 555-01xx, a range kept for fiction"},
	{"company", "a made-up company"},
	{"city", "a city"},
	{"country", "a country code (ISO 3166)"},
	{"ipv4", "from the documentation ranges (192.0.2.0/24…)"},
	{"ipv6", "from 2001:db8::/32, the documentation range"},
	{"url", "https://example.com/…"},
	{"integer", "between min and max (0 to 1000)"},
	{"number", "between min and max with decimals (2)"},
	{"boolean", "true or false"},
	{"date", "between min and max (the last two years)"},
	{"datetime", "RFC 3339, UTC, between min and max"},
	{"choice", "one of the choices"},
	{"word", "a lorem word"},
	{"sentence", "a lorem sentence"},
}

func knownKind(k string) bool {
	for _, x := range Kinds {
		if x.Kind == k {
			return true
		}
	}
	return false
}

// value is one cell: a string, an integer, a number (kept as text so its
// decimals print as asked), a boolean, or blank.
type value struct {
	kind  byte // 's' string, 'i' integer, 'n' number, 'b' boolean, 0 blank
	s     string
	b     bool
	blank bool
}

// person is the made-up person a row's name columns share.
type person struct{ first, last string }

// Rows makes count rows for cols.
func Rows(cols []Column, count int, seed uint64) ([][]value, error) {
	if len(cols) == 0 || len(cols) > MaxColumns {
		return nil, fmt.Errorf("give 1 to %d columns", MaxColumns)
	}
	if count < 1 || count > MaxRows {
		return nil, fmt.Errorf("rows are 1 to %d", MaxRows)
	}
	seen := map[string]bool{}
	gens := make([]func(r rng, p person, row int) value, len(cols))
	for i, c := range cols {
		if strings.TrimSpace(c.Name) == "" {
			return nil, fmt.Errorf("column %d has no name", i+1)
		}
		if seen[c.Name] {
			return nil, fmt.Errorf("two columns are named %q", c.Name)
		}
		seen[c.Name] = true
		if !knownKind(c.Kind) {
			return nil, fmt.Errorf("column %q: unknown kind %q", c.Name, c.Kind)
		}
		g, err := generator(c)
		if err != nil {
			return nil, fmt.Errorf("column %q: %w", c.Name, err)
		}
		gens[i] = g
	}
	r := newRNG(seed)
	out := make([][]value, count)
	for row := range out {
		p := person{r.pick(firstNames), r.pick(lastNames)}
		cells := make([]value, len(cols))
		for i, g := range gens {
			if cols[i].Blank > 0 && r.IntN(100) < cols[i].Blank {
				cells[i] = value{blank: true}
				continue
			}
			cells[i] = g(r, p, row)
		}
		out[row] = cells
	}
	return out, nil
}

func str(s string) value { return value{kind: 's', s: s} }

// generator turns a column into a function making its values.
func generator(c Column) (func(r rng, p person, row int) value, error) {
	switch c.Kind {
	case "id":
		return func(_ rng, _ person, row int) value { return value{kind: 'i', s: strconv.Itoa(row + 1)} }, nil
	case "uuid":
		return func(r rng, _ person, _ int) value {
			var b [16]byte
			for i := range b {
				b[i] = byte(r.Uint32())
			}
			b[6] = b[6]&0x0f | 0x40
			b[8] = b[8]&0x3f | 0x80
			return str(fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:]))
		}, nil
	case "firstName":
		return func(_ rng, p person, _ int) value { return str(p.first) }, nil
	case "lastName":
		return func(_ rng, p person, _ int) value { return str(p.last) }, nil
	case "name":
		return func(_ rng, p person, _ int) value { return str(p.first + " " + p.last) }, nil
	case "email":
		return func(r rng, p person, _ int) value {
			return str(fmt.Sprintf("%s.%s@%s", ascii(p.first), ascii(p.last), r.pick(exampleDomains)))
		}, nil
	case "username":
		return func(r rng, p person, _ int) value {
			return str(fmt.Sprintf("%s.%s%d", ascii(p.first), ascii(p.last)[:min(3, len(ascii(p.last)))], r.IntN(100)))
		}, nil
	case "phone":
		return func(r rng, _ person, _ int) value {
			return str(fmt.Sprintf("+1 %03d-555-01%02d", 200+r.IntN(800), r.IntN(100)))
		}, nil
	case "company":
		return func(r rng, _ person, _ int) value { return str(r.pick(lastNames) + " " + r.pick(companySuffixes)) }, nil
	case "city":
		return func(r rng, _ person, _ int) value { return str(r.pick(cities)) }, nil
	case "country":
		return func(r rng, _ person, _ int) value { return str(r.pick(countries)) }, nil
	case "ipv4":
		return func(r rng, _ person, _ int) value {
			return str(fmt.Sprintf("%s.%d", r.pick(docNets4), 1+r.IntN(254)))
		}, nil
	case "ipv6":
		return func(r rng, _ person, _ int) value {
			return str(fmt.Sprintf("2001:db8:%x:%x::%x", r.IntN(0x10000), r.IntN(0x10000), 1+r.IntN(0xffff)))
		}, nil
	case "url":
		return func(r rng, _ person, _ int) value {
			return str(fmt.Sprintf("https://%s/%s/%s", r.pick(exampleDomains), r.pick(loremWords), r.pick(loremWords)))
		}, nil
	case "integer":
		lo, hi, err := intRange(c.Min, c.Max, 0, 1000)
		if err != nil {
			return nil, err
		}
		return func(r rng, _ person, _ int) value {
			return value{kind: 'i', s: strconv.FormatInt(lo+r.Int64N(hi-lo+1), 10)}
		}, nil
	case "number":
		lo, hi, err := floatRange(c.Min, c.Max)
		if err != nil {
			return nil, err
		}
		dec := c.Decimals
		if dec == 0 {
			dec = 2
		}
		if dec < 0 || dec > 10 {
			return nil, fmt.Errorf("decimals are 0 to 10")
		}
		return func(r rng, _ person, _ int) value {
			return value{kind: 'n', s: strconv.FormatFloat(lo+r.Float64()*(hi-lo), 'f', dec, 64)}
		}, nil
	case "boolean":
		return func(r rng, _ person, _ int) value { return value{kind: 'b', b: r.IntN(2) == 1} }, nil
	case "date", "datetime":
		lo, hi, err := timeRange(c.Min, c.Max)
		if err != nil {
			return nil, err
		}
		layout := time.DateOnly
		if c.Kind == "datetime" {
			layout = time.RFC3339
		}
		// In whole seconds: a Duration (nanoseconds) can't span more than
		// 292 years, and dates may run from year 0 to 9999.
		from, span := lo.Unix(), hi.Unix()-lo.Unix()
		return func(r rng, _ person, _ int) value {
			return str(time.Unix(from+r.Int64N(span+1), 0).UTC().Format(layout))
		}, nil
	case "choice":
		var opts []string
		for _, s := range c.Choices {
			if s = strings.TrimSpace(s); s != "" {
				opts = append(opts, s)
			}
		}
		if len(opts) == 0 {
			return nil, fmt.Errorf("give the choices")
		}
		return func(r rng, _ person, _ int) value { return str(r.pick(opts)) }, nil
	case "word":
		return func(r rng, _ person, _ int) value { return str(r.pick(loremWords)) }, nil
	case "sentence":
		return func(r rng, _ person, _ int) value {
			s, _ := Lorem("sentences", 1, r.Uint64(), false)
			return str(s)
		}, nil
	}
	return nil, fmt.Errorf("unknown kind %q", c.Kind)
}

func intRange(minS, maxS string, dlo, dhi int64) (int64, int64, error) {
	lo, hi := dlo, dhi
	var err error
	if minS != "" {
		if lo, err = strconv.ParseInt(strings.TrimSpace(minS), 10, 64); err != nil {
			return 0, 0, fmt.Errorf("min %q is not a whole number", minS)
		}
	}
	if maxS != "" {
		if hi, err = strconv.ParseInt(strings.TrimSpace(maxS), 10, 64); err != nil {
			return 0, 0, fmt.Errorf("max %q is not a whole number", maxS)
		}
	}
	if lo > hi {
		return 0, 0, fmt.Errorf("min %d is above max %d", lo, hi)
	}
	if hi-lo < 0 || hi-lo == 1<<63-1 { // overflow guard for Int64N
		return 0, 0, fmt.Errorf("the range is too wide")
	}
	return lo, hi, nil
}

func floatRange(minS, maxS string) (float64, float64, error) {
	lo, hi := 0.0, 1000.0
	var err error
	if minS != "" {
		if lo, err = strconv.ParseFloat(strings.TrimSpace(minS), 64); err != nil {
			return 0, 0, fmt.Errorf("min %q is not a number", minS)
		}
	}
	if maxS != "" {
		if hi, err = strconv.ParseFloat(strings.TrimSpace(maxS), 64); err != nil {
			return 0, 0, fmt.Errorf("max %q is not a number", maxS)
		}
	}
	if lo > hi {
		return 0, 0, fmt.Errorf("min is above max")
	}
	return lo, hi, nil
}

// timeRange reads YYYY-MM-DD or RFC 3339 bounds; by default the two
// years up to 2026-01-01, a fixed end so a seed always gives the same.
func timeRange(minS, maxS string) (time.Time, time.Time, error) {
	parse := func(s string) (time.Time, error) {
		s = strings.TrimSpace(s)
		if t, err := time.Parse(time.DateOnly, s); err == nil {
			return t, nil
		}
		return time.Parse(time.RFC3339, s)
	}
	hi := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	lo := hi.AddDate(-2, 0, 0)
	var err error
	if minS != "" {
		if lo, err = parse(minS); err != nil {
			return lo, hi, fmt.Errorf("min %q is not a date (YYYY-MM-DD)", minS)
		}
	}
	if maxS != "" {
		if hi, err = parse(maxS); err != nil {
			return lo, hi, fmt.Errorf("max %q is not a date (YYYY-MM-DD)", maxS)
		}
	}
	if lo.After(hi) {
		return lo, hi, fmt.Errorf("min is after max")
	}
	return lo.UTC(), hi.UTC(), nil
}

// ascii lower-cases a name for an address: letters only.
func ascii(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r >= 'a' && r <= 'z' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
