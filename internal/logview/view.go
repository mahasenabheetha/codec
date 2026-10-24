package logview

// The view: how the entries a query matched are shown, built here so a
// 100k-entry log pages as smoothly as a short one. Over the matches, in
// log order:
//
//   - context: the entries around each match, whatever the query, with
//     a gap row where entries are skipped ("⋯ 240 entries");
//   - output folding: a run of plain output lines (text without a level,
//     an error or a stack) from one source becomes one row, ended by any
//     structured entry or gap, so a database's chatter inside an Argo
//     run doesn't bury the entries around it;
//   - repeats: the same line again and again from one source becomes
//     one row "×120".
//
// A fold the user opened (by its first entry) is shown whole. Counts,
// the chart and the error groups stay those of the matches.

// RowKind is what a row of the view is.
type RowKind uint8

const (
	RowMatch   RowKind = iota // an entry the query matched
	RowContext                // an entry around a match
	RowGap                    // entries skipped between context windows
	RowOutput                 // a run of plain output, folded
	RowRepeat                 // a line repeated, folded
)

var rowKindNames = [...]string{"", "context", "gap", "output", "repeat"}

func (k RowKind) String() string { return rowKindNames[k] }

// Repeats says which consecutive lines fold together.
type Repeats uint8

const (
	RepeatsOff     Repeats = iota
	RepeatsExact           // the same message
	RepeatsSimilar         // the same apart from numbers, IDs and quoted values
)

// ViewOptions shape a view; the zero value is the matches, one row each.
type ViewOptions struct {
	Context  int // entries shown around each match; 0 for none
	Output   bool
	Repeats  Repeats
	Expanded map[uint32]bool // folds shown whole, by their first entry
}

// ViewRow is one row of a view. For an entry, I = Last and N = 1; for a
// fold, I is its first entry, Last its last, N how many entries it holds
// and Matches how many of them the query matched; for a gap, I…Last are
// the entries skipped and N how many.
type ViewRow struct {
	I, Last    uint32
	N, Matches uint32
	Kind       RowKind
}

// MaxContext bounds the entries shown around each match.
const MaxContext = 50

// outputRun is the shortest run of plain output that is folded.
const outputRun = 3

// View lays out the entries m matched with o.
func (l *Log) View(m *Matches, o ViewOptions) []ViewRow {
	rows := l.withContext(m.IDs, min(o.Context, MaxContext))
	if o.Output {
		rows = l.foldOutput(rows, o.Expanded)
	}
	if o.Repeats != RepeatsOff {
		rows = l.foldRepeats(rows, o.Repeats, o.Expanded)
	}
	return rows
}

func one(i uint32, k RowKind) ViewRow {
	n := uint32(0)
	if k == RowMatch {
		n = 1
	}
	return ViewRow{I: i, Last: i, N: 1, Matches: n, Kind: k}
}

// withContext is the matches, one row each, with c entries around each
// and gap rows between windows that don't touch.
func (l *Log) withContext(ids []uint32, c int) []ViewRow {
	if c <= 0 {
		rows := make([]ViewRow, len(ids))
		for k, id := range ids {
			rows[k] = one(id, RowMatch)
		}
		return rows
	}
	if len(ids) == 0 {
		return nil
	}
	n := len(l.rows)
	rows := make([]ViewRow, 0, min(n+2, len(ids)*(2*c+1)+2))
	gap := func(from, to int) {
		if from <= to {
			rows = append(rows, ViewRow{I: uint32(from), Last: uint32(to), N: uint32(to - from + 1), Kind: RowGap})
		}
	}
	last := -1 // the last entry added
	p := 0     // the next match not yet passed
	for _, id := range ids {
		lo, hi := max(0, int(id)-c), min(n-1, int(id)+c)
		if lo <= last {
			lo = last + 1
		} else {
			gap(last+1, lo-1)
		}
		for j := lo; j <= hi; j++ {
			for p < len(ids) && int(ids[p]) < j {
				p++
			}
			if p < len(ids) && int(ids[p]) == j {
				rows = append(rows, one(uint32(j), RowMatch))
			} else {
				rows = append(rows, one(uint32(j), RowContext))
			}
		}
		last = max(last, hi)
	}
	gap(last+1, n-1)
	return rows
}

// plain reports an entry that is only output: text with no level, error
// or stack trace.
func (l *Log) plain(i uint32) bool {
	r := &l.rows[i]
	if r.format != Text || r.level != LevelNone {
		return false
	}
	if r.extra != 0 {
		x := &l.extras[r.extra-1]
		return x.err == "" && x.stack == ""
	}
	return true
}

// foldOutput folds each source's runs of plain output. A run is broken
// by a gap or by any entry that isn't plain output, whatever its
// source; plain lines of different sources between them fold apart.
func (l *Log) foldOutput(rows []ViewRow, expanded map[uint32]bool) []ViewRow {
	type group struct {
		head, size int
		last       uint32
		matches    uint32
	}
	var groups []group
	of := make([]int32, len(rows))
	open := map[uint32]int32{}
	for k, r := range rows {
		of[k] = -1
		if r.Kind == RowGap || !l.plain(r.I) {
			clear(open)
			continue
		}
		src := l.rows[r.I].source
		g, ok := open[src]
		if !ok {
			g = int32(len(groups))
			groups = append(groups, group{head: k})
			open[src] = g
		}
		groups[g].size++
		groups[g].last = r.I
		groups[g].matches += r.Matches
		of[k] = g
	}
	out := make([]ViewRow, 0, len(rows))
	for k, r := range rows {
		g := of[k]
		if g < 0 || groups[g].size < outputRun || expanded[rows[groups[g].head].I] {
			out = append(out, r)
			continue
		}
		if k == groups[g].head {
			out = append(out, ViewRow{I: r.I, Last: groups[g].last, N: uint32(groups[g].size), Matches: groups[g].matches, Kind: RowOutput})
		}
	}
	return out
}

// foldRepeats folds rows next to each other that say the same thing:
// same kind (match or context), source, level and message.
func (l *Log) foldRepeats(rows []ViewRow, mode Repeats, expanded map[uint32]bool) []ViewRow {
	// Each row's key once, and only for rows the cheap checks don't part.
	keys := make([]string, len(rows))
	key := func(k int) string {
		if keys[k] == "" {
			msg := l.rows[rows[k].I].message
			if mode == RepeatsSimilar {
				msg = fingerprint(msg)
			}
			keys[k] = msg + "\x00" // never empty, so computed once
		}
		return keys[k]
	}
	same := func(a, b int) bool {
		ra, rb := rows[a], rows[b]
		if ra.N != 1 || rb.N != 1 || ra.Kind != rb.Kind || ra.Kind > RowContext {
			return false
		}
		ea, eb := &l.rows[ra.I], &l.rows[rb.I]
		return ea.source == eb.source && ea.level == eb.level && key(a) == key(b)
	}
	out := make([]ViewRow, 0, len(rows))
	for k := 0; k < len(rows); {
		j := k + 1
		for j < len(rows) && same(k, j) {
			j++
		}
		if j-k < 2 || expanded[rows[k].I] {
			out = append(out, rows[k:j]...)
		} else {
			f := ViewRow{I: rows[k].I, Last: rows[j-1].I, N: uint32(j - k), Kind: RowRepeat}
			for _, r := range rows[k:j] {
				f.Matches += r.Matches
			}
			out = append(out, f)
		}
		k = j
	}
	return out
}

// RowAt finds the row of a view that shows entry i: the entry's own
// row, the fold holding it, or the gap that skips it; -1 when none.
func (l *Log) RowAt(rows []ViewRow, i uint32) int {
	lo, hi := 0, len(rows)
	for lo < hi {
		mid := (lo + hi) / 2
		if rows[mid].I <= i {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	// Rows start in entry order, but folds of different sources overlap:
	// look back from the last row starting at or before i. All the way:
	// a fold's run of output may be crossed by many other sources' lines,
	// and this runs once per request.
	for k := lo - 1; k >= 0; k-- {
		if l.holds(rows[k], i) {
			return k
		}
	}
	return -1
}

// holds reports whether row r shows entry i.
func (l *Log) holds(r ViewRow, i uint32) bool {
	switch {
	case r.I == i:
		return true
	case i < r.I || i > r.Last:
		return false
	case r.Kind == RowGap:
		return true
	case r.Kind == RowOutput:
		return l.rows[i].source == l.rows[r.I].source && l.plain(i)
	case r.Kind == RowRepeat:
		return l.rows[i].source == l.rows[r.I].source && l.rows[i].level == l.rows[r.I].level
	}
	return false
}

// Filter is a compiled query, for marking entries (highlight rules).
type Filter struct {
	l *Log
	n node
}

// Filter compiles a query against the log, as Match does.
func (l *Log) Filter(q string) (*Filter, error) {
	n, _, err := l.compile(q)
	if err != nil {
		return nil, err
	}
	return &Filter{l, n}, nil
}

// Match reports whether entry i matches.
func (f *Filter) Match(i int) bool { return f.n.match(f.l, i) }

// Marker bits: what a stretch of the view holds, for the marks beside
// the scrollbar. Highlight rule k is MarkRule << k.
const (
	MarkError uint8 = 1 << iota
	MarkWarn
	MarkPin
	MarkRule // first of four
)

// Markers sums a view into buckets (a picture of the whole list): the
// worst level, the highlight rules matched and the pins in each. A fold
// is marked by its first entry.
func (l *Log) Markers(rows []ViewRow, buckets int, rules []*Filter, pins []uint32) []uint8 {
	if buckets <= 0 || len(rows) == 0 {
		return nil
	}
	buckets = min(buckets, len(rows))
	out := make([]uint8, buckets)
	at := func(pos int) int { return pos * buckets / len(rows) }
	for pos, r := range rows {
		if r.Kind == RowGap {
			continue
		}
		var m uint8
		switch lv := l.rows[r.I].level; {
		case lv >= Error:
			m |= MarkError
		case lv == Warn:
			m |= MarkWarn
		}
		for k, f := range rules[:min(len(rules), 4)] {
			if f != nil && f.Match(int(r.I)) {
				m |= MarkRule << k
			}
		}
		out[at(pos)] |= m
	}
	for _, p := range pins {
		if pos := l.RowAt(rows, p); pos >= 0 {
			out[at(pos)] |= MarkPin
		}
	}
	return out
}
