package textdiff

import (
	"slices"
	"testing"
)

// FuzzLines: the edits rebuild both sides, and the views over them
// never fail hard.
func FuzzLines(f *testing.F) {
	f.Add("a\nb\nc\n", "a\nx\nc\nd\n")
	f.Add("", "only new\n")
	f.Add("same\r\nlines\r\n", "same\nlines\n")
	f.Fuzz(func(t *testing.T, a, b string) {
		la, lb := SplitLines(a), SplitLines(b)
		var gotA, gotB []string
		for _, e := range Lines(la, lb) {
			if e.Op != Insert {
				gotA = append(gotA, e.Text)
			}
			if e.Op != Delete {
				gotB = append(gotB, e.Text)
			}
		}
		if !slices.Equal(gotA, la) || !slices.Equal(gotB, lb) {
			t.Fatalf("edits don't rebuild the sides:\n%q\n%q", gotA, gotB)
		}
		SideBySide(a, b, 3, Options{IgnoreSpace: true})
		UnifiedWith("f", a, b, 3, Options{IgnoreCase: true})
	})
}
