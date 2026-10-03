// Package search finds text in files: a phrase or a regular expression,
// optionally case-sensitive and whole-word, in files picked by
// include/exclude globs. It is a pure engine package: the caller hands
// in the file list and a read function, so the web server and the CLI
// search exactly the files the Explorer lists.
//
// Matching is line by line. Columns are in UTF-16 code units, the unit
// JavaScript strings and the editor count in.
package search

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf16"
	"unicode/utf8"
)

// Options say what to look for and where.
type Options struct {
	Query     string   `json:"query"`
	Regex     bool     `json:"regex"`
	MatchCase bool     `json:"matchCase"`
	WholeWord bool     `json:"wholeWord"`
	Globs     []string `json:"globs"` // "charts/**", "*.yaml"; "!" excludes
	Limit     int      `json:"limit"` // most matches returned; 0 = DefaultLimit
}

// DefaultLimit caps the matches of one search: enough to scan by eye,
// small enough to send and render at once.
const DefaultLimit = 2000

// maxLine is the longest line text returned; longer lines (minified
// files) are cut to a window around the first match.
const maxLine = 240

// Match is one line with at least one hit.
type Match struct {
	Path  string `json:"path"`
	Line  int    `json:"line"` // 1-based
	Text  string `json:"text"` // the line, maybe cut to a window
	Spans []Span `json:"spans"`
	// Col is the first hit's column in the full line (1-based, UTF-16),
	// for opening the file at it.
	Col int `json:"col"`
}

// Span is one hit within Match.Text, in UTF-16 code units.
type Span struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Result is what a search found, in path order.
type Result struct {
	Matches   []Match `json:"matches"`
	Files     int     `json:"files"`     // files with a match
	Searched  int     `json:"searched"`  // files read
	Truncated bool    `json:"truncated"` // stopped at the limit
}

// ErrEmpty is returned for a blank query.
var ErrEmpty = errors.New("type something to search for")

// Searcher is a compiled search.
type Searcher struct {
	re      *regexp.Regexp
	include []*regexp.Regexp
	exclude []*regexp.Regexp
	limit   int
}

// Compile checks the options and builds a Searcher. A bad regular
// expression is an error the user can fix.
func Compile(o Options) (*Searcher, error) {
	if strings.TrimSpace(o.Query) == "" {
		return nil, ErrEmpty
	}
	expr := o.Query
	if !o.Regex {
		expr = regexp.QuoteMeta(expr)
	}
	if o.WholeWord {
		expr = `\b(?:` + expr + `)\b`
	}
	if !o.MatchCase {
		expr = `(?i)` + expr
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, err
	}
	s := &Searcher{re: re, limit: o.Limit}
	if s.limit <= 0 {
		s.limit = DefaultLimit
	}
	for _, g := range o.Globs {
		g = strings.TrimSpace(g)
		if neg, ok := strings.CutPrefix(g, "!"); ok {
			if neg = strings.TrimSpace(neg); neg != "" {
				s.exclude = append(s.exclude, globRegexp(neg))
			}
		} else if g != "" {
			s.include = append(s.include, globRegexp(g))
		}
	}
	return s, nil
}

// Wants reports whether the globs select path.
func (s *Searcher) Wants(path string) bool {
	for _, re := range s.exclude {
		if re.MatchString(path) {
			return false
		}
	}
	if len(s.include) == 0 {
		return true
	}
	for _, re := range s.include {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}

// globRegexp turns a glob into a regexp over slash paths: "**" spans
// folders, "*" and "?" stay inside one. A glob without a slash matches
// a name at any depth ("*.yaml", "tests"); any glob also matches
// everything under a folder it names ("charts/shop").
func globRegexp(g string) *regexp.Regexp {
	g = strings.Trim(g, "/")
	var b strings.Builder
	if !strings.Contains(g, "/") {
		b.WriteString(`(?:^|.*/)`)
	} else {
		b.WriteString(`^`)
	}
	for i := 0; i < len(g); i++ {
		switch c := g[i]; {
		case c == '*' && i+1 < len(g) && g[i+1] == '*':
			i++
			if i+1 < len(g) && g[i+1] == '/' { // "**/" = zero or more folders
				i++
				b.WriteString(`(?:.*/)?`)
			} else {
				b.WriteString(`.*`)
			}
		case c == '*':
			b.WriteString(`[^/]*`)
		case c == '?':
			b.WriteString(`[^/]`)
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString(`(?:/.*)?$`)
	return regexp.MustCompile(b.String())
}

// File returns the matching lines of one file, at most limit of them.
func (s *Searcher) File(path string, data []byte, limit int) []Match {
	if limit <= 0 || !s.re.Match(data) {
		return nil
	}
	var out []Match
	for n, rest := 1, data; len(rest) > 0 && len(out) < limit; n++ {
		line := rest
		if i := bytes.IndexByte(rest, '\n'); i >= 0 {
			line, rest = rest[:i], rest[i+1:]
		} else {
			rest = nil
		}
		line = bytes.TrimSuffix(line, []byte("\r"))
		hits := s.re.FindAllIndex(line, -1)
		if hits == nil {
			continue
		}
		out = append(out, matchOf(path, n, string(line), hits))
	}
	return out
}

// matchOf builds a Match, cutting long lines to a window that starts a
// little before the first hit.
func matchOf(path string, n int, line string, hits [][]int) Match {
	from, to := 0, len(line)
	if len(line) > maxLine {
		from = max(0, hits[0][0]-40)
		for from > 0 && !utf8.RuneStart(line[from]) {
			from--
		}
		to = min(len(line), from+maxLine)
		for to < len(line) && !utf8.RuneStart(line[to]) {
			to--
		}
	}
	m := Match{Path: path, Line: n, Text: line[from:to], Col: u16(line[:hits[0][0]]) + 1}
	for _, h := range hits {
		if h[0] >= to || h[1] <= from || h[0] == h[1] {
			continue
		}
		start, end := max(h[0], from), min(h[1], to)
		m.Spans = append(m.Spans, Span{u16(line[from:start]), u16(line[from:end])})
	}
	if from > 0 {
		m.Text = "…" + m.Text
		for i := range m.Spans {
			m.Spans[i].Start++
			m.Spans[i].End++
		}
	}
	if to < len(line) {
		m.Text += "…"
	}
	return m
}

// u16 is the length of s in UTF-16 code units.
func u16(s string) int {
	n := 0
	for _, r := range s {
		n += utf16.RuneLen(r)
	}
	return n
}

// Run searches paths (in sorted order) with up to 8 files read at a
// time, stopping early at the limit or when ctx is cancelled. Files
// that can't be read (binaries, too big, gone) are skipped.
func (s *Searcher) Run(ctx context.Context, paths []string, read func(string) ([]byte, error)) (Result, error) {
	var picked []string
	for _, p := range paths {
		if s.Wants(p) {
			picked = append(picked, p)
		}
	}
	sort.Strings(picked)

	// Workers take files in order, so when the limit is reached every
	// file before the cut has been searched and the result is the same
	// on every run.
	found := make([][]Match, len(picked))
	var next, total, searched atomic.Int64
	var wg sync.WaitGroup
	for range min(8, len(picked)) {
		wg.Go(func() {
			for ctx.Err() == nil && total.Load() < int64(s.limit) {
				i := int(next.Add(1) - 1)
				if i >= len(picked) {
					return
				}
				data, err := read(picked[i])
				if err != nil || bytes.IndexByte(data[:min(len(data), 8000)], 0) >= 0 {
					continue
				}
				searched.Add(1)
				found[i] = s.File(picked[i], data, s.limit)
				total.Add(int64(len(found[i])))
			}
		})
	}
	wg.Wait()

	res := Result{Matches: []Match{}, Searched: int(searched.Load())}
	for i, ms := range found {
		if len(ms) == 0 {
			continue
		}
		if room := s.limit - len(res.Matches); len(ms) > room {
			ms, res.Truncated = ms[:room], true
		}
		res.Matches = append(res.Matches, ms...)
		res.Files++
		if len(res.Matches) >= s.limit {
			res.Truncated = res.Truncated || i < len(found)-1 || int(next.Load()) < len(picked)
			break
		}
	}
	return res, ctx.Err()
}
