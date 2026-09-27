// Package names suggests the name someone probably meant ("did you
// mean"), for every lens that checks references by name: CI jobs,
// Compose services, Ansible roles and handlers.
package names

import (
	"fmt"
	"strings"
)

// Closest suggests the candidate nearest to s, or "": at most 3 edits
// away, and fewer for short names, so long names that merely share
// words aren't offered. s itself is never suggested.
func Closest(s string, candidates []string) string {
	best, bestD := "", min(3, len(s)/3)+1
	for _, c := range candidates {
		if c == s {
			continue
		}
		if d := Distance(strings.ToLower(s), strings.ToLower(c)); d < bestD {
			best, bestD = c, d
		}
	}
	return best
}

// DidYouMean is Closest as a hint sentence, or "".
func DidYouMean(s string, candidates []string) string {
	if c := Closest(s, candidates); c != "" {
		return fmt.Sprintf("Did you mean %q?", c)
	}
	return ""
}

// Distance counts the edits between a and b: inserts, deletes, changes
// and swaps of neighbours (a common typo), each one edit (the
// "optimal string alignment" Damerau–Levenshtein distance).
func Distance(a, b string) int {
	d := make([][]int, len(a)+1)
	for i := range d {
		d[i] = make([]int, len(b)+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			c := 1
			if a[i-1] == b[j-1] {
				c = 0
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+c)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+1)
			}
		}
	}
	return d[len(a)][len(b)]
}
