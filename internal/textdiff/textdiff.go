// Package textdiff computes line diffs and writes them as unified
// diffs that `git apply` accepts. It is a pure engine package.
//
// The algorithm is Myers' O(ND) diff ("An O(ND) Difference Algorithm
// and Its Variations", 1986): it finds a shortest edit script by
// exploring diagonals of the edit graph, one edit distance D at a time,
// and remembers each step so the path can be traced back.
package textdiff

import (
	"fmt"
	"strings"
)

// Op is the kind of an edit.
type Op int

const (
	Equal Op = iota
	Delete
	Insert
)

// Edit is one line of the edit script. Lines keep their line ending,
// so "a" (last line, no newline) and "a\n" differ, as they do for git.
type Edit struct {
	Op   Op
	Text string
}

// maxEdits bounds the work: past this many differing lines the middle
// is reported as one block replacement. The patch is still correct,
// just less minimal. It keeps memory at a few MB for any input.
const maxEdits = 1000

// SplitLines splits text after each "\n", keeping the endings.
func SplitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.SplitAfter(s, "\n")
	if lines[len(lines)-1] == "" { // text ended with a newline
		lines = lines[:len(lines)-1]
	}
	return lines
}

// Lines returns an edit script turning a into b.
func Lines(a, b []string) []Edit {
	return LinesFunc(a, b, func(x, y string) bool { return x == y })
}

// LinesFunc is Lines with a custom line equality. Equal edits carry the
// line from a.
func LinesFunc(a, b []string, eq func(x, y string) bool) []Edit {
	// Common prefix and suffix are cheap to peel off, and what-if edits
	// are usually small, so this leaves Myers a tiny middle.
	pre := 0
	for pre < len(a) && pre < len(b) && eq(a[pre], b[pre]) {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && eq(a[len(a)-1-suf], b[len(b)-1-suf]) {
		suf++
	}
	out := make([]Edit, 0, len(a)+len(b))
	for _, l := range a[:pre] {
		out = append(out, Edit{Equal, l})
	}
	out = append(out, myers(a[pre:len(a)-suf], b[pre:len(b)-suf], eq)...)
	for _, l := range a[len(a)-suf:] {
		out = append(out, Edit{Equal, l})
	}
	return out
}

func myers(a, b []string, eq func(x, y string) bool) []Edit {
	n, m := len(a), len(b)
	limit := min(n+m, maxEdits)
	off := limit + 1
	// v[off+k] is the furthest x reached on diagonal k (k = x - y).
	v := make([]int, 2*limit+3)
	// trace[d] holds v for k in [-d, d] after step d, for the traceback.
	var trace [][]int

	for d := 0; d <= limit; d++ {
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[off+k-1] < v[off+k+1]) {
				x = v[off+k+1] // step down: insert b[y]
			} else {
				x = v[off+k-1] + 1 // step right: delete a[x]
			}
			y := x - k
			for x < n && y < m && eq(a[x], b[y]) { // follow the snake
				x++
				y++
			}
			v[off+k] = x
			if x >= n && y >= m {
				trace = append(trace, append([]int(nil), v[off-d:off+d+1]...))
				return backtrack(a, b, trace)
			}
		}
		trace = append(trace, append([]int(nil), v[off-d:off+d+1]...))
	}

	// Too different to be worth it: replace the whole middle.
	out := make([]Edit, 0, n+m)
	for _, l := range a {
		out = append(out, Edit{Delete, l})
	}
	for _, l := range b {
		out = append(out, Edit{Insert, l})
	}
	return out
}

// backtrack walks the trace from (n, m) back to (0, 0), collecting the
// edits in reverse.
func backtrack(a, b []string, trace [][]int) []Edit {
	x, y := len(a), len(b)
	var rev []Edit
	for d := len(trace) - 1; d > 0; d-- {
		prev := trace[d-1] // index k+(d-1) holds diagonal k
		at := func(k int) int { return prev[k+d-1] }
		k := x - y
		var prevK int
		if k == -d || (k != d && at(k-1) < at(k+1)) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}
		prevX := at(prevK)
		prevY := prevX - prevK
		for x > prevX && y > prevY {
			x, y = x-1, y-1
			rev = append(rev, Edit{Equal, a[x]})
		}
		if x == prevX {
			y--
			rev = append(rev, Edit{Insert, b[y]})
		} else {
			x--
			rev = append(rev, Edit{Delete, a[x]})
		}
	}
	for x > 0 && y > 0 {
		x, y = x-1, y-1
		rev = append(rev, Edit{Equal, a[x]})
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}

// Unified renders the difference between old and new as a unified
// diff with the given lines of context, headed "--- a/<path>" and
// "+++ b/<path>" so `git apply` (default -p1) finds the file. Empty
// when the texts are equal.
func Unified(path, old, new string, context int) string {
	return format(path, Lines(SplitLines(old), SplitLines(new)), context)
}

// Options loosen which lines count as the same.
type Options struct {
	IgnoreSpace bool // runs of spaces and tabs, indentation, line endings
	IgnoreCase  bool
}

// UnifiedWith is Unified with looser line equality. Lines that match
// only loosely show as context, taken from old. With no options set it
// is Unified.
func UnifiedWith(path, old, new string, context int, o Options) string {
	if !o.IgnoreSpace && !o.IgnoreCase {
		return Unified(path, old, new, context)
	}
	norm := func(s string) string {
		if o.IgnoreSpace {
			s = strings.Join(strings.Fields(s), " ")
		}
		if o.IgnoreCase {
			s = strings.ToLower(s)
		}
		return s
	}
	return format(path, LinesFunc(SplitLines(old), SplitLines(new), func(x, y string) bool { return norm(x) == norm(y) }), context)
}

// format writes edits as a unified diff.
func format(path string, edits []Edit, context int) string {

	var changes []int
	for i, e := range edits {
		if e.Op != Equal {
			changes = append(changes, i)
		}
	}
	if len(changes) == 0 {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "--- a/%s\n+++ b/%s\n", path, path)

	for c := 0; c < len(changes); {
		// Grow a hunk while the next change is within 2*context lines.
		first, last := changes[c], changes[c]
		for c++; c < len(changes) && changes[c]-last <= 2*context; c++ {
			last = changes[c]
		}
		start := max(0, first-context)
		end := min(len(edits), last+context+1)

		// Line numbers: count the old/new lines before and inside the hunk.
		oldBefore, newBefore := 0, 0
		for _, e := range edits[:start] {
			if e.Op != Insert {
				oldBefore++
			}
			if e.Op != Delete {
				newBefore++
			}
		}
		oldCount, newCount := 0, 0
		for _, e := range edits[start:end] {
			if e.Op != Insert {
				oldCount++
			}
			if e.Op != Delete {
				newCount++
			}
		}
		fmt.Fprintf(&b, "@@ -%s +%s @@\n", hunkRange(oldBefore, oldCount), hunkRange(newBefore, newCount))
		for _, e := range edits[start:end] {
			b.WriteByte(" -+"[e.Op])
			b.WriteString(e.Text)
			if !strings.HasSuffix(e.Text, "\n") {
				b.WriteString("\n\\ No newline at end of file\n")
			}
		}
	}
	return b.String()
}

// hunkRange formats "start,count". An empty range names the line
// before it, as diff and git expect.
func hunkRange(before, count int) string {
	if count == 0 {
		return fmt.Sprintf("%d,0", before)
	}
	return fmt.Sprintf("%d,%d", before+1, count)
}

const bom = "\xEF\xBB\xBF"

// Patch returns a git-applicable diff that turns the file on disk into
// an edited buffer. Editors hold LF text without a BOM; the patch keeps
// the file's BOM and line endings: unchanged lines keep their own, new
// lines get the file's usual one. Empty when only endings differ.
func Patch(path string, disk []byte, edited string) string {
	old := string(disk)
	a := SplitLines(old)

	crlf := 0
	for _, l := range a {
		if strings.HasSuffix(l, "\r\n") {
			crlf++
		}
	}
	edited = strings.ReplaceAll(edited, "\r\n", "\n")
	if strings.HasPrefix(old, bom) && !strings.HasPrefix(edited, bom) {
		edited = bom + edited
	}
	b := SplitLines(edited)
	if crlf*2 > len(a) { // mostly CRLF
		for i, l := range b {
			if strings.HasSuffix(l, "\n") {
				b[i] = l[:len(l)-1] + "\r\n"
			}
		}
	}
	// Equal if the text matches and both do (or don't) end the line.
	eq := func(x, y string) bool {
		return trimEOL(x) == trimEOL(y) && strings.HasSuffix(x, "\n") == strings.HasSuffix(y, "\n")
	}
	return format(path, LinesFunc(a, b, eq), 3)
}

func trimEOL(s string) string {
	return strings.TrimSuffix(strings.TrimSuffix(s, "\n"), "\r")
}
