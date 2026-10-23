package logview

import (
	"cmp"
	"slices"
	"strings"
	"unicode/utf8"
)

// Comparing two runs of one job, say today's that failed with
// yesterday's that passed: entries are matched by their messages'
// fingerprints, so a step logs the same message in both runs whatever
// its numbers, IDs, times and generated pod names (decision 88).

// msgIndex numbers the distinct fingerprints of a log's messages.
type msgIndex struct {
	ids  map[string]uint32
	keys []string
	of   []uint32 // per entry: its fingerprint's id + 1; 0 when it has no message
}

// messageKey is what entries are matched by: the message's fingerprint,
// and the error's when the message doesn't hold it.
func messageKey(msg, err string) string {
	k := fingerprint(msg)
	if err != "" && !strings.Contains(msg, err) {
		if k != "" {
			k += " · "
		}
		k += fingerprint(err)
	}
	return k
}

// messages fingerprints every entry once, the first time a comparison
// asks.
func (l *Log) messages() *msgIndex {
	l.msgOnce.Do(func() {
		x := &msgIndex{ids: map[string]uint32{}, of: make([]uint32, len(l.rows))}
		for i := range l.rows {
			r := &l.rows[i]
			err := ""
			if r.extra != 0 {
				err = l.extras[r.extra-1].err
			}
			k := messageKey(r.message, err)
			if k == "" {
				continue
			}
			// A hex ID that happens to be all digits masks as a number:
			// any masked value matches any other.
			k = strings.ReplaceAll(k, "<id>", "<n>")
			id, ok := x.ids[k]
			if !ok {
				id = uint32(len(x.keys))
				x.keys = append(x.keys, k)
				x.ids[k] = id
			}
			x.of[i] = id + 1
		}
		l.msg = x
	})
	return l.msg
}

// Diff is how one run differs from another. Pairs hold this run's
// value first, the other's second.
type Diff struct {
	Runs [2]RunStats `json:"runs"`
	// The first entry in each run whose message the other never has:
	// where the runs part. -1 when every message is in the other too.
	First   [2]int       `json:"first"`
	New     []DiffItem   `json:"new"`     // messages only this run has
	Missing []DiffItem   `json:"missing"` // messages only the other run has
	Changed []DiffItem   `json:"changed"` // in both, logged much more or less often
	More    [3]int       `json:"more"`    // items of New, Missing and Changed left out
	Sources []SourceDiff `json:"sources,omitempty"`
}

// RunStats is a run in numbers.
type RunStats struct {
	Entries  int     `json:"entries"`
	Errors   int     `json:"errors"` // error or worse
	Warnings int     `json:"warnings"`
	Seconds  float64 `json:"seconds"`  // from the first time to the last
	Messages int     `json:"messages"` // distinct fingerprints
}

// DiffItem is one message of a comparison.
type DiffItem struct {
	Text   string `json:"text"`   // its fingerprint, as matched
	Sample string `json:"sample"` // its first line as first logged
	Level  string `json:"level"`  // the worst it was logged at
	Count  [2]int `json:"count"`
	First  [2]int `json:"first"` // its first entry in each run; -1 where it isn't
}

// SourceDiff is one source (a pod, a container, a file) in both runs,
// matched by its name's fingerprint.
type SourceDiff struct {
	Name    [2]string  `json:"name"` // "" where it didn't log
	Entries [2]int     `json:"entries"`
	Errors  [2]int     `json:"errors"`
	Seconds [2]float64 `json:"seconds"` // from its first time to its last
	First   [2]int     `json:"first"`   // -1 where it didn't log
}

// maxDiffItems caps each list of a Diff; errors and warnings come first.
const maxDiffItems = 500

type msgStat struct {
	count int
	first int
	level Level
}

func (x *msgIndex) stats(l *Log) []msgStat {
	st := make([]msgStat, len(x.keys))
	for i, id := range x.of {
		if id == 0 {
			continue
		}
		s := &st[id-1]
		if s.count == 0 {
			s.first = i
		}
		s.count++
		s.level = max(s.level, l.rows[i].level)
	}
	return st
}

// Compare tells how run a differs from run b.
func Compare(a, b *Log) Diff {
	ma, mb := a.messages(), b.messages()
	sa, sb := ma.stats(a), mb.stats(b)
	d := Diff{Runs: [2]RunStats{a.runStats(len(ma.keys)), b.runStats(len(mb.keys))}, First: [2]int{-1, -1}}

	inB := make([]uint32, len(ma.keys)) // a's key → b's id + 1, or 0
	inA := make([]bool, len(mb.keys))
	for k, s := range ma.keys {
		if id, ok := mb.ids[s]; ok {
			inB[k] = id + 1
			inA[id] = true
		}
	}
	for i, id := range ma.of {
		if id != 0 && inB[id-1] == 0 {
			d.First[0] = i
			break
		}
	}
	for i, id := range mb.of {
		if id != 0 && !inA[id-1] {
			d.First[1] = i
			break
		}
	}

	none := msgStat{first: -1}
	for k, s := range sa {
		if inB[k] == 0 {
			d.New = append(d.New, diffItem(a, ma.keys[k], s, none, false))
		} else if o := sb[inB[k]-1]; changed(s.count, o.count) {
			d.Changed = append(d.Changed, diffItem(a, ma.keys[k], s, o, false))
		}
	}
	for k, s := range sb {
		if !inA[k] {
			d.Missing = append(d.Missing, diffItem(b, mb.keys[k], none, s, true))
		}
	}
	// Errors, then warnings, then the rest, each in log order; changes
	// by how much they changed.
	byLevel := func(side int) func(x, y DiffItem) int {
		return func(x, y DiffItem) int {
			return cmp.Or(levelClass(y.Level)-levelClass(x.Level), x.First[side]-y.First[side])
		}
	}
	slices.SortFunc(d.New, byLevel(0))
	slices.SortFunc(d.Missing, byLevel(1))
	slices.SortFunc(d.Changed, func(x, y DiffItem) int {
		return cmp.Or(spread(y)-spread(x), x.First[0]-y.First[0])
	})
	d.New, d.More[0] = capItems(d.New)
	d.Missing, d.More[1] = capItems(d.Missing)
	d.Changed, d.More[2] = capItems(d.Changed)
	d.Sources = compareSources(a, b)
	return d
}

// changed: logged at least half as often again (or a third less), and
// at least two times more or fewer.
func changed(a, b int) bool {
	lo, hi := min(a, b), max(a, b)
	return hi-lo >= 2 && 2*hi >= 3*lo
}

func spread(x DiffItem) int {
	d := x.Count[0] - x.Count[1]
	if d < 0 {
		return -d
	}
	return d
}

// levelClass sorts errors (and worse) before warnings before the rest.
func levelClass(level string) int {
	switch l := ParseLevel(level); {
	case l >= Error:
		return 2
	case l == Warn:
		return 1
	}
	return 0
}

func capItems(xs []DiffItem) ([]DiffItem, int) {
	if len(xs) <= maxDiffItems {
		return xs, 0
	}
	return xs[:maxDiffItems], len(xs) - maxDiffItems
}

// diffItem describes key with its stats in each run; the sample comes
// from the run that has it (the other when other is set).
func diffItem(l *Log, key string, here, there msgStat, other bool) DiffItem {
	s := here
	if other {
		s = there
	}
	return DiffItem{
		Text: key, Sample: sample(l, s.first), Level: max(here.level, there.level).String(),
		Count: [2]int{here.count, there.count}, First: [2]int{here.first, there.first},
	}
}

// sample is entry i's message (or error) as a list shows it: its first
// line, at most 300 bytes.
func sample(l *Log, i int) string {
	r := &l.rows[i]
	s := r.message
	if strings.TrimSpace(s) == "" && r.extra != 0 {
		s = l.extras[r.extra-1].err
	}
	if j := strings.IndexByte(s, '\n'); j >= 0 {
		s = s[:j]
	}
	if len(s) > 300 {
		n := 300
		for n > 0 && !utf8.RuneStart(s[n]) {
			n--
		}
		s = s[:n] + "…"
	}
	return strings.TrimRight(s, "\r")
}

func (l *Log) runStats(messages int) RunStats {
	s := &l.summary
	st := RunStats{Entries: s.Entries, Errors: s.Levels["error"] + s.Levels["fatal"], Warnings: s.Levels["warn"], Messages: messages}
	if !s.From.IsZero() {
		st.Seconds = s.To.Sub(s.From).Seconds()
	}
	return st
}

type sourceStat struct {
	name          string
	entries, errs int
	first         int
	from, to      int64
}

// sourceStats sums a log's entries by their source's fingerprint, in the
// order the sources first log; entries without a source are left out.
func (l *Log) sourceStats() (map[string]*sourceStat, []string) {
	by := map[string]*sourceStat{}
	var order []string
	keys := make([]string, len(l.strs)) // a source's fingerprint, by its id
	for i := range l.rows {
		r := &l.rows[i]
		if r.source == 0 {
			continue
		}
		k := keys[r.source]
		if k == "" {
			k = fingerprint(l.strs[r.source])
			keys[r.source] = k
		}
		s := by[k]
		if s == nil {
			s = &sourceStat{name: l.strs[r.source], first: i}
			by[k] = s
			order = append(order, k)
		}
		s.entries++
		if r.level >= Error {
			s.errs++
		}
		if r.time != 0 {
			if s.from == 0 || r.time < s.from {
				s.from = r.time
			}
			s.to = max(s.to, r.time)
		}
	}
	return by, order
}

// compareSources lines up the sources of two runs: this run's in the
// order they first logged, then those only the other run has.
func compareSources(a, b *Log) []SourceDiff {
	ba, oa := a.sourceStats()
	bb, ob := b.sourceStats()
	if len(oa)+len(ob) == 0 {
		return nil
	}
	none := &sourceStat{first: -1}
	pair := func(x, y *sourceStat) SourceDiff {
		return SourceDiff{
			Name:    [2]string{x.name, y.name},
			Entries: [2]int{x.entries, y.entries},
			Errors:  [2]int{x.errs, y.errs},
			Seconds: [2]float64{x.seconds(), y.seconds()},
			First:   [2]int{x.first, y.first},
		}
	}
	var out []SourceDiff
	for _, k := range oa {
		y := bb[k]
		if y == nil {
			y = none
		}
		out = append(out, pair(ba[k], y))
	}
	for _, k := range ob {
		if ba[k] == nil {
			out = append(out, pair(none, bb[k]))
		}
	}
	return out
}

func (s *sourceStat) seconds() float64 {
	if s.from == 0 {
		return 0
	}
	return float64(s.to-s.from) / 1e9
}
