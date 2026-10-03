package textdiff

import (
	"strings"
	"unicode"
	"unicode/utf16"
)

// Row is one line of a side-by-side diff. A changed line is paired
// with the line that replaced it, and both carry the changed parts.
type Row struct {
	Kind  string `json:"kind"` // "equal", "change", "delete", "insert" or "gap"
	Left  *Cell  `json:"left,omitempty"`
	Right *Cell  `json:"right,omitempty"`
	Count int    `json:"count,omitempty"` // gap: unchanged lines left out
}

// Cell is one side of a row. In a gap, Line is the first line left out.
type Cell struct {
	Line  int      `json:"line"` // 1-based
	Text  string   `json:"text"` // without the line ending
	Spans [][2]int `json:"spans,omitempty"`
}

// maxInline bounds the character diff of a line pair: past this many
// runes between the common prefix and suffix, that middle is marked
// as one change. It keeps a minified line from costing megabytes.
const maxInline = 300

// inlineBudget bounds the character diffs of one call, counted as the
// product of each pair's differing middles (a rough Myers cost). Once
// spent, changed middles are marked whole, so two large, wholly
// different pastes stay fast.
const inlineBudget = 4 << 20

// SideBySide pairs the lines of old and new, keeping context unchanged
// lines around each change and folding the rest into gaps. Changed
// pairs carry the changed characters as UTF-16 spans, ready for JS
// string slicing. Line endings never count. Nil when the texts match.
func SideBySide(old, new string, context int, o Options) []Row {
	a, b := SplitLines(old), SplitLines(new)
	// Compare normalized keys, computed once per line.
	key := func(lines []string) []string {
		out := make([]string, len(lines))
		for k, l := range lines {
			out[k] = o.normalize(trimEOL(l))
		}
		return out
	}
	edits := Lines(key(a), key(b))
	budget := inlineBudget

	var rows []Row
	var dels, ins []*Cell
	flush := func() {
		for k := 0; k < max(len(dels), len(ins)); k++ {
			r := Row{Kind: "change"}
			if k < len(dels) {
				r.Left = dels[k]
			} else {
				r.Kind = "insert"
			}
			if k < len(ins) {
				r.Right = ins[k]
			} else {
				r.Kind = "delete"
			}
			if r.Kind == "change" {
				r.Left.Spans, r.Right.Spans = inline(r.Left.Text, r.Right.Text, o, &budget)
			}
			rows = append(rows, r)
		}
		dels, ins = dels[:0], ins[:0]
	}
	i, j, changed := 0, 0, false
	for _, e := range edits {
		switch e.Op {
		case Equal:
			flush()
			rows = append(rows, Row{Kind: "equal", Left: &Cell{Line: i + 1, Text: trimEOL(a[i])}, Right: &Cell{Line: j + 1, Text: trimEOL(b[j])}})
			i, j = i+1, j+1
		case Delete:
			dels = append(dels, &Cell{Line: i + 1, Text: trimEOL(a[i])})
			i, changed = i+1, true
		case Insert:
			ins = append(ins, &Cell{Line: j + 1, Text: trimEOL(b[j])})
			j, changed = j+1, true
		}
	}
	flush()
	if !changed {
		return nil
	}
	return fold(rows, context)
}

// fold replaces runs of equal rows further than context from any
// change with one gap row. A gap of a single line isn't worth a fold.
func fold(rows []Row, context int) []Row {
	keep := make([]bool, len(rows))
	for k, r := range rows {
		if r.Kind != "equal" {
			for m := max(0, k-context); m <= min(len(rows)-1, k+context); m++ {
				keep[m] = true
			}
		}
	}
	out := make([]Row, 0, len(rows))
	for k := 0; k < len(rows); {
		if keep[k] {
			out = append(out, rows[k])
			k++
			continue
		}
		end := k
		for end < len(rows) && !keep[end] {
			end++
		}
		if end-k == 1 {
			out = append(out, rows[k])
		} else {
			out = append(out, Row{Kind: "gap", Count: end - k, Left: &Cell{Line: rows[k].Left.Line}, Right: &Cell{Line: rows[k].Right.Line}})
		}
		k = end
	}
	return out
}

// normalize is the line form that Options compare.
func (o Options) normalize(s string) string {
	if o.IgnoreSpace {
		s = strings.Join(strings.Fields(s), " ")
	}
	if o.IgnoreCase {
		s = strings.ToLower(s)
	}
	return s
}

// seg is a run of a character diff: equal (a == b runes on each side)
// or a change replacing a runes of the old line with b of the new.
type seg struct {
	eq   bool
	a, b int
}

// inline returns the changed parts of a line pair as UTF-16 spans.
// Tiny equal runs between changes are merged into them, so a reworded
// part reads as one block rather than scattered letters; a line that
// is mostly different is marked whole.
func inline(x, y string, o Options, budget *int) (left, right [][2]int) {
	ra, rb := []rune(x), []rune(y)
	same := func(p, q rune) bool { return p == q || o.IgnoreCase && unicode.ToLower(p) == unicode.ToLower(q) }
	pre := 0
	for pre < len(ra) && pre < len(rb) && same(ra[pre], rb[pre]) {
		pre++
	}
	suf := 0
	for suf < len(ra)-pre && suf < len(rb)-pre && same(ra[len(ra)-1-suf], rb[len(rb)-1-suf]) {
		suf++
	}
	ma, mb := ra[pre:len(ra)-suf], rb[pre:len(rb)-suf]

	var segs []seg
	if cost := (len(ma) + 1) * (len(mb) + 1); len(ma) > maxInline || len(mb) > maxInline || cost > *budget {
		segs = []seg{{a: len(ma), b: len(mb)}}
	} else {
		*budget -= cost
		segs = runeDiff(ma, mb, same)
	}
	segs = merge(segs)

	kept := pre + suf
	for _, s := range segs {
		if s.eq {
			kept += s.a
		}
	}
	if kept*3 < max(len(ra), len(rb)) { // mostly different: mark it all
		segs, pre = []seg{{a: len(ra), b: len(rb)}}, 0
	}

	ua, ub := utf16Offsets(ra), utf16Offsets(rb)
	pa, pb := pre, pre
	for _, s := range segs {
		if !s.eq {
			if s.a > 0 && !(o.IgnoreSpace && blank(ra[pa:pa+s.a])) {
				left = append(left, [2]int{ua[pa], ua[pa+s.a]})
			}
			if s.b > 0 && !(o.IgnoreSpace && blank(rb[pb:pb+s.b])) {
				right = append(right, [2]int{ub[pb], ub[pb+s.b]})
			}
		}
		pa, pb = pa+s.a, pb+s.b
	}
	return left, right
}

// runeDiff runs the line differ over single runes.
func runeDiff(a, b []rune, same func(p, q rune) bool) []seg {
	ta, tb := make([]string, len(a)), make([]string, len(b))
	for k, r := range a {
		ta[k] = string(r)
	}
	for k, r := range b {
		tb[k] = string(r)
	}
	var segs []seg
	for _, e := range LinesFunc(ta, tb, func(p, q string) bool { return same([]rune(p)[0], []rune(q)[0]) }) {
		eq := e.Op == Equal
		if len(segs) == 0 || segs[len(segs)-1].eq != eq {
			segs = append(segs, seg{eq: eq})
		}
		s := &segs[len(segs)-1]
		if e.Op != Insert {
			s.a++
		}
		if e.Op != Delete {
			s.b++
		}
	}
	return segs
}

// merge folds an equal run into the changes around it when it is too
// short to read as unchanged: under 3 runes, or under 8 and well
// under the size of both neighbours.
func merge(segs []seg) []seg {
	for k := 1; k+1 < len(segs); {
		p, e, n := segs[k-1], segs[k], segs[k+1]
		if !e.eq || p.eq || n.eq || !(e.a < 3 || e.a < 8 && e.a*2 < min(max(p.a, p.b), max(n.a, n.b))) {
			k++
			continue
		}
		segs[k-1] = seg{a: p.a + e.a + n.a, b: p.b + e.b + n.b}
		segs = append(segs[:k], segs[k+2:]...)
		k = max(1, k-1)
	}
	return segs
}

// utf16Offsets maps each rune index (and the end) to its UTF-16 offset.
func utf16Offsets(rs []rune) []int {
	out := make([]int, len(rs)+1)
	for k, r := range rs {
		out[k+1] = out[k] + utf16.RuneLen(r)
	}
	return out
}

func blank(rs []rune) bool {
	for _, r := range rs {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}
