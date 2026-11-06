package logsrc

import (
	"cmp"
	"slices"
	"strings"
)

// Quality says how sure a match is: shown as a badge in the Source
// drawer.
type Quality string

const (
	Exact    Quality = "exact"    // the one file the reference names
	Strong   Quality = "strong"   // its folders agree, but more than one file fits, or not all of them
	Possible Quality = "possible" // only the file name agrees
)

// Match is a file of the folder a reference may point to.
type Match struct {
	File    string  `json:"file"` // slash-separated, relative to the folder
	Quality Quality `json:"quality"`
}

// MaxMatches bounds the files listed for one reference.
const MaxMatches = 20

// Index finds the folder's files by name, built once for the folder's
// file list and used for every reference in an entry.
type Index struct {
	byName map[string][]string // lower-cased file name → paths
}

// NewIndex indexes files (slash-separated, relative to the folder).
func NewIndex(files []string) *Index {
	ix := &Index{byName: make(map[string][]string, len(files))}
	for _, f := range files {
		name := strings.ToLower(f[strings.LastIndexByte(f, '/')+1:])
		ix.byName[name] = append(ix.byName[name], f)
	}
	return ix
}

// Resolve lists the files a reference's path may mean. Logs carry the
// path where the code was built or run (/build/src/internal/x.go,
// C:\agent\_work\1\s\src\X.cs), so the path is matched by its trailing
// folders: the files sharing the most of them win, all of them listed.
// Case is ignored, as Windows and macOS do.
func (ix *Index) Resolve(path string) []Match {
	segs := split(path)
	if len(segs) == 0 {
		return nil
	}
	best, bestN := []string(nil), 0
	for _, f := range ix.byName[strings.ToLower(segs[len(segs)-1])] {
		switch n := sharedTail(segs, split(f)); {
		case n > bestN:
			best, bestN = append(best[:0], f), n
		case n == bestN:
			best = append(best, f)
		}
	}
	if len(best) == 0 {
		return nil
	}
	// Shorter paths first: a copy deep in a fixtures folder is the
	// less likely one.
	slices.SortFunc(best, func(a, b string) int {
		return cmp.Or(cmp.Compare(strings.Count(a, "/"), strings.Count(b, "/")), strings.Compare(a, b))
	})
	q := Possible
	switch {
	case bestN == len(segs) && len(best) == 1:
		q = Exact // every folder the log names agrees, and only one file does
	case bestN == len(segs) || bestN >= 2:
		// The log names a bare file several files share, or the folders
		// agree only in part (a build path above the repository).
		q = Strong
		if bestN >= 2 && len(best) == 1 && bestN == len(split(best[0])) {
			q = Exact // the file's whole path is the tail of the log's
		}
	}
	out := make([]Match, 0, min(len(best), MaxMatches))
	for _, f := range best[:min(len(best), MaxMatches)] {
		out = append(out, Match{File: f, Quality: q})
	}
	return out
}

// split cuts a path into its folders and file name, leaving out a drive,
// "." and empty parts; ".." stays (it can't be matched, so it ends the
// shared tail).
func split(p string) []string {
	p = strings.ReplaceAll(p, `\`, "/")
	if len(p) >= 2 && p[1] == ':' {
		p = p[2:]
	}
	var out []string
	for _, s := range strings.Split(p, "/") {
		if s != "" && s != "." {
			out = append(out, s)
		}
	}
	return out
}

// sharedTail counts the trailing parts two paths have in common.
func sharedTail(a, b []string) int {
	n := 0
	for n < len(a) && n < len(b) && strings.EqualFold(a[len(a)-1-n], b[len(b)-1-n]) {
		n++
	}
	return n
}
