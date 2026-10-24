package logview

import (
	"cmp"
	"math"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Error groups: warnings and worse whose messages differ only in
// numbers, IDs, UUIDs, hex and quoted values are one group.

type groupKey struct {
	text string // the masked message, as shown
}

type grouper struct {
	ids  map[string]uint32
	keys []groupKey
}

func newGrouper() *grouper { return &grouper{ids: map[string]uint32{}} }

func (g *grouper) id(msg, err string) uint32 {
	k := messageKey(msg, err)
	if id, ok := g.ids[k]; ok {
		return id
	}
	g.keys = append(g.keys, groupKey{k})
	id := uint32(len(g.keys))
	g.ids[k] = id
	return id
}

// fingerprint masks what varies between repeats of one message: its
// first line, with numbers as <n>, IDs (words with digits, hex runs,
// the random part of a generated Kubernetes name) as <id> and quoted
// values as <s>.
func fingerprint(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 300 {
		s = s[:300]
	}
	var b strings.Builder
	space := false
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ' || c == '\t':
			space = true
			i++
			continue
		case (c == '"' || c == '\'' && (i == 0 || !isAlnum(s[i-1]))) && i+1 < len(s):
			if end := strings.IndexByte(s[i+1:], c); end >= 0 && end < 200 {
				flush(&b, &space)
				b.WriteString("<s>")
				i += end + 2
				continue
			}
		case isUUID(s[i:]) && (i == 0 || !isAlnum(s[i-1])):
			flush(&b, &space)
			b.WriteString("<id>")
			i += 36
			continue
		case isAlnum(c):
			j := i
			for j < len(s) && isAlnum(s[j]) {
				j++
			}
			flush(&b, &space)
			if i > 0 && s[i-1] == '-' && kubeRandom(s[i:j]) {
				b.WriteString("<id>")
			} else {
				b.WriteString(maskWord(s[i:j]))
			}
			i = j
			continue
		}
		flush(&b, &space)
		b.WriteByte(c)
		i++
	}
	return b.String()
}

func flush(b *strings.Builder, space *bool) {
	if *space && b.Len() > 0 {
		b.WriteByte(' ')
	}
	*space = false
}

func isAlnum(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '_'
}

// maskWord: all digits → <n>; a number with a short unit ("12ms",
// "500MB") → <n> and the unit; with a digit and long, or hex → <id>;
// short words like "v1", "s3" or "utf8" stay.
func maskWord(w string) string {
	digits, lead, hex := 0, 0, true
	for i := 0; i < len(w); i++ {
		c := w[i]
		if c >= '0' && c <= '9' {
			digits++
			if lead == i {
				lead++
			}
		} else if !(c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			hex = false
		}
	}
	switch {
	case digits == len(w):
		return "<n>"
	case lead == digits && lead > 0 && len(w)-lead <= 3 && !hex:
		return "<n>" + w[lead:]
	case digits > 0 && (len(w) >= 5 || hex && len(w) >= 4):
		return "<id>"
	}
	return w
}

// kubeRandom reports whether w, after a dash, looks like what Kubernetes
// adds to a generated name: five characters (or nine or ten, a pod
// template hash) from its alphabet, which has no vowels so it spells no
// words. "build-xkqpz" names the same pod template as "build-7f2kd". A
// number stays a number (<n>).
func kubeRandom(w string) bool {
	if n := len(w); n != 5 && n != 9 && n != 10 {
		return false
	}
	letters := false
	for i := 0; i < len(w); i++ {
		if !strings.ContainsRune("bcdfghjklmnpqrstvwxz2456789", rune(w[i])) {
			return false
		}
		letters = letters || w[i] > '9'
	}
	return letters
}

// isUUID reports whether s starts with a UUID (8-4-4-4-12 hex digits).
func isUUID(s string) bool {
	if len(s) < 36 || len(s) > 36 && isAlnum(s[36]) {
		return false
	}
	for i := 0; i < 36; i++ {
		c := s[i]
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

// Group is an error group among some entries.
type Group struct {
	ID      int       `json:"id"` // for the query group:ID
	Text    string    `json:"text"`
	Level   string    `json:"level"` // the worst level in it
	Count   int       `json:"count"`
	First   int       `json:"first"` // entry index
	Last    int       `json:"last"`
	From    time.Time `json:"from,omitzero"`
	To      time.Time `json:"to,omitzero"`
	Sources int       `json:"sources"`
}

// Groups counts the error groups of the given entries (all when ids is
// nil), most frequent first.
func (l *Log) Groups(ids []uint32) []Group {
	type acc struct {
		g       Group
		level   Level
		sources map[uint32]bool
	}
	by := map[uint32]*acc{}
	l.each(ids, func(i int) {
		r := &l.rows[i]
		if r.group == 0 {
			return
		}
		a := by[r.group]
		if a == nil {
			a = &acc{g: Group{ID: int(r.group), Text: l.groups[r.group-1].text, First: i}, sources: map[uint32]bool{}}
			by[r.group] = a
		}
		a.g.Count++
		a.g.Last = i
		a.level = max(a.level, r.level)
		a.sources[r.source] = true
		if r.time != 0 {
			t := time.Unix(0, r.time).UTC()
			if a.g.From.IsZero() || t.Before(a.g.From) {
				a.g.From = t
			}
			if t.After(a.g.To) {
				a.g.To = t
			}
		}
	})
	out := make([]Group, 0, len(by))
	for _, a := range by {
		a.g.Level = a.level.String()
		a.g.Sources = len(a.sources)
		out = append(out, a.g)
	}
	slices.SortFunc(out, func(a, b Group) int {
		return cmp.Or(b.Count-a.Count, a.First-b.First)
	})
	return out
}

// each calls fn for the given entries, or all of them when ids is nil.
func (l *Log) each(ids []uint32, fn func(int)) {
	if ids == nil {
		for i := range l.rows {
			fn(i)
		}
		return
	}
	for _, i := range ids {
		fn(int(i))
	}
}

// Summary is what the overview shows.
type Summary struct {
	Entries    int            `json:"entries"`
	Lines      int            `json:"lines"`
	Formats    map[string]int `json:"formats"`
	Levels     map[string]int `json:"levels"` // "" counts entries without a level
	Timed      int            `json:"timed"`  // entries with a time
	From       time.Time      `json:"from,omitzero"`
	To         time.Time      `json:"to,omitzero"`
	Sources    []Count        `json:"sources,omitempty"`
	FirstError int            `json:"firstError"` // entry index of the first error or worse; -1 when none
	Stacks     int            `json:"stacks"`     // entries with a stack trace
	Groups     int            `json:"groups"`     // error groups
	Gaps       []Gap          `json:"gaps,omitempty"`
	// An Argo run's workflows and steps, in the order the steps start.
	Workflows []string   `json:"workflows,omitempty"`
	Steps     []StepStat `json:"steps,omitempty"`
}

// Count is a value and how often it occurs.
type Count struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// Gap is a stretch in which nothing logged, between two entries.
type Gap struct {
	After   int       `json:"after"` // entry index before the gap
	From    time.Time `json:"from"`
	To      time.Time `json:"to"`
	Seconds float64   `json:"seconds"`
}

// Summary returns the log's overview, worked out when it was built.
func (l *Log) Summary() Summary { return l.summary }

func (l *Log) summarize() {
	s := Summary{Formats: map[string]int{}, Levels: map[string]int{}, FirstError: -1, Groups: len(l.groups)}
	sources := map[uint32]int{}
	for i := range l.rows {
		r := &l.rows[i]
		s.Lines += int(r.lines)
		s.Formats[r.format.String()]++
		s.Levels[r.level.String()]++
		sources[r.source]++
		if r.level >= Error && s.FirstError < 0 {
			s.FirstError = i
		}
		if r.extra != 0 && l.extras[r.extra-1].stack != "" {
			s.Stacks++
		}
		if r.time != 0 {
			s.Timed++
			t := time.Unix(0, r.time).UTC()
			if s.From.IsZero() || t.Before(s.From) {
				s.From = t
			}
			if t.After(s.To) {
				s.To = t
			}
		}
	}
	s.Entries = len(l.rows)
	if len(sources) > 1 || sources[0] == 0 {
		for id, n := range sources {
			s.Sources = append(s.Sources, Count{l.strs[id], n})
		}
		slices.SortFunc(s.Sources, func(a, b Count) int { return cmp.Or(b.Count-a.Count, strings.Compare(a.Value, b.Value)) })
	}
	s.Gaps = l.gaps()
	s.Workflows, s.Steps = l.workflows, l.stepStats()
	l.summary = s
}

// gaps finds the longest silences: at least 30 s and 20 times the usual
// interval between entries, the five longest, in log order.
func (l *Log) gaps() []Gap {
	type step struct {
		after int
		d     int64
	}
	var steps []step
	prev := -1
	for i := range l.rows {
		if l.rows[i].time == 0 {
			continue
		}
		if prev >= 0 {
			if d := l.rows[i].time - l.rows[prev].time; d > 0 {
				steps = append(steps, step{prev, d})
			}
		}
		prev = i
	}
	if len(steps) < 2 {
		return nil
	}
	ds := make([]int64, len(steps))
	for i, s := range steps {
		ds[i] = s.d
	}
	slices.Sort(ds)
	limit := max(int64(30*time.Second), 20*ds[len(ds)/2])
	var big []step
	for _, s := range steps {
		if s.d >= limit {
			big = append(big, s)
		}
	}
	slices.SortFunc(big, func(a, b step) int { return cmp.Compare(b.d, a.d) })
	big = big[:min(len(big), 5)]
	slices.SortFunc(big, func(a, b step) int { return a.after - b.after })
	out := make([]Gap, len(big))
	for i, s := range big {
		from := l.rows[s.after].time
		out[i] = Gap{After: s.after, From: time.Unix(0, from).UTC(), To: time.Unix(0, from+s.d).UTC(), Seconds: float64(s.d) / 1e9}
	}
	return out
}

// Histogram is entries over time, per level.
type Histogram struct {
	From    time.Time `json:"from,omitzero"`
	StepMs  int64     `json:"stepMs"`
	Buckets [][7]int  `json:"buckets"` // counts by level: none, trace … fatal
	Untimed int       `json:"untimed"`
}

var histSteps = []time.Duration{
	10 * time.Millisecond, 50 * time.Millisecond, 100 * time.Millisecond, 250 * time.Millisecond, 500 * time.Millisecond,
	time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second, 15 * time.Second, 30 * time.Second,
	time.Minute, 2 * time.Minute, 5 * time.Minute, 10 * time.Minute, 15 * time.Minute, 30 * time.Minute,
	time.Hour, 2 * time.Hour, 3 * time.Hour, 6 * time.Hour, 12 * time.Hour, 24 * time.Hour, 48 * time.Hour, 7 * 24 * time.Hour,
}

// Histogram counts the given entries (all when ids is nil) in at most
// about maxBuckets steps of a round size over from–to; zero times mean
// the log's own span, so the axis stays put while filtering.
func (l *Log) Histogram(ids []uint32, from, to time.Time, maxBuckets int) Histogram {
	if from.IsZero() {
		from = l.summary.From
	}
	if to.IsZero() {
		to = l.summary.To
	}
	h := Histogram{}
	start, step, n, ok := l.timeBuckets(from, to, maxBuckets)
	if !ok {
		l.each(ids, func(i int) { h.Untimed++ })
		return h
	}
	h.From, h.StepMs, h.Buckets = start, step.Milliseconds(), make([][7]int, n)
	s0, st := start.UnixNano(), int64(step)
	l.each(ids, func(i int) {
		r := &l.rows[i]
		if r.time == 0 {
			h.Untimed++
			return
		}
		if b := (r.time - s0) / st; r.time >= s0 && b < int64(n) {
			h.Buckets[b][r.level]++
		}
	})
	return h
}

// FieldStat describes one field over some entries.
type FieldStat struct {
	Key      string   `json:"key"`
	Count    int      `json:"count"`    // entries that have it
	Distinct int      `json:"distinct"` // different values, up to distinctCap
	More     bool     `json:"more,omitempty"`
	Long     bool     `json:"long,omitempty"` // long text (stack traces, statements): values not counted
	Top      []Count  `json:"top,omitempty"`
	Num      *NumStat `json:"num,omitempty"`
}

// NumStat summarises a numeric field. Min, max and mean are exact; the
// median and 95th percentile come from a sample of numSample values.
type NumStat struct {
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Mean   float64 `json:"mean"`
	Median float64 `json:"median"`
	P95    float64 `json:"p95"`
}

const (
	distinctCap = 10000
	numSample   = 100000
	longValue   = 200
)

// FieldStats describes every field of the given entries (all when ids
// is nil), in the order fields first appear; "source" comes first when
// the entries have several.
func (l *Log) FieldStats(ids []uint32, top int) []FieldStat {
	type acc struct {
		count, long int
		values      map[string]int
		more        bool
		nums        []float64
		numCount    int
		min, max    float64
		sum         float64
	}
	accs := make([]*acc, len(l.keys))
	rng := rand.New(rand.NewPCG(1, 2))
	sources := map[uint32]int{}
	seen := 0
	l.each(ids, func(i int) {
		seen++
		sources[l.rows[i].source]++
		for _, c := range l.fields(i) {
			a := accs[c.key]
			if a == nil {
				a = &acc{values: map[string]int{}, min: math.Inf(1), max: math.Inf(-1)}
				accs[c.key] = a
			}
			a.count++
			if len(c.text) > longValue {
				a.long++
				continue
			}
			if _, ok := a.values[c.text]; ok || len(a.values) < distinctCap {
				a.values[c.text]++
			} else {
				a.more = true
			}
			if c.kind == Number {
				n, err := strconv.ParseFloat(c.text, 64)
				if err != nil || math.IsInf(n, 0) {
					continue
				}
				a.numCount++
				a.sum += n
				a.min, a.max = min(a.min, n), max(a.max, n)
				// A reservoir sample (algorithm R), seeded so answers repeat.
				if len(a.nums) < numSample {
					a.nums = append(a.nums, n)
				} else if j := rng.IntN(a.numCount); j < numSample {
					a.nums[j] = n
				}
			}
		}
	})
	var out []FieldStat
	if len(sources) > 1 {
		fs := FieldStat{Key: "source", Count: seen - sources[0], Distinct: len(sources)}
		for id, n := range sources {
			if id != 0 {
				fs.Top = append(fs.Top, Count{l.strs[id], n})
			}
		}
		if _, ok := sources[0]; ok {
			fs.Distinct--
		}
		fs.Top = topCounts(fs.Top, top)
		out = append(out, fs)
	}
	for k, a := range accs {
		if a == nil {
			continue
		}
		fs := FieldStat{Key: l.keys[k], Count: a.count, Distinct: len(a.values), More: a.more, Long: a.long == a.count}
		for v, n := range a.values {
			fs.Top = append(fs.Top, Count{v, n})
		}
		fs.Top = topCounts(fs.Top, top)
		if a.numCount > 0 {
			slices.Sort(a.nums)
			fs.Num = &NumStat{Min: a.min, Max: a.max, Mean: a.sum / float64(a.numCount),
				Median: quantile(a.nums, 0.5), P95: quantile(a.nums, 0.95)}
		}
		out = append(out, fs)
	}
	return out
}

func topCounts(cs []Count, n int) []Count {
	slices.SortFunc(cs, func(a, b Count) int { return cmp.Or(b.Count-a.Count, strings.Compare(a.Value, b.Value)) })
	return cs[:min(len(cs), n)]
}

// quantile of sorted values, by the nearest rank.
func quantile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	i := int(math.Ceil(q*float64(len(sorted)))) - 1
	return sorted[max(0, min(i, len(sorted)-1))]
}

// Around returns the indexes of the entries before and after entry i,
// whatever a query matched: what led up to an error.
func (l *Log) Around(i, before, after int) (from, to int) {
	return max(0, i-before), min(len(l.rows), i+after+1)
}
