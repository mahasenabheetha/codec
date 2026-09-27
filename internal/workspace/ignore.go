package workspace

import (
	"bytes"
	"regexp"
	"strings"
)

// ignoreRule is one compiled .gitignore line.
type ignoreRule struct {
	re      *regexp.Regexp
	negate  bool // "!pattern" re-includes
	dirOnly bool // "pattern/" matches directories only
}

// ignoreFile holds the rules of one .gitignore. Its patterns are
// relative to the directory that contains it.
type ignoreFile struct {
	dir   string // slash path of that directory; "" = workspace root
	rules []ignoreRule
}

// parseIgnore compiles a .gitignore found in dir. foldCase matches
// case-insensitively, like git with core.ignorecase (the default on
// Windows and macOS). Lines that don't compile are skipped: a bad
// pattern must not hide the whole repository.
//
// Supported: comments, blank lines, "!" negation, trailing "/" for
// directories, leading or middle "/" for anchoring, *, ?, [...] and the
// three ** forms. See https://git-scm.com/docs/gitignore.
func parseIgnore(dir string, data []byte, foldCase bool) *ignoreFile {
	f := &ignoreFile{dir: dir}
	for line := range bytes.Lines(data) {
		pat := strings.TrimRight(string(line), "\r\n")
		if pat == "" || pat[0] == '#' {
			continue
		}
		// Trailing spaces are ignored unless escaped with a backslash.
		for strings.HasSuffix(pat, " ") && !strings.HasSuffix(pat, `\ `) {
			pat = pat[:len(pat)-1]
		}
		var r ignoreRule
		if pat[0] == '!' {
			r.negate, pat = true, pat[1:]
		}
		if strings.HasSuffix(pat, "/") {
			r.dirOnly, pat = true, strings.TrimSuffix(pat, "/")
		}
		if pat == "" {
			continue
		}
		// A slash at the start or in the middle anchors the pattern to
		// this directory; otherwise it matches a name at any depth.
		anchored := strings.Contains(pat, "/")
		pat = strings.TrimPrefix(pat, "/")

		expr := globToRegexp(pat)
		if !anchored {
			expr = "(?:.*/)?" + expr
		}
		expr = "^" + expr + "$"
		if foldCase {
			expr = "(?i)" + expr
		}
		re, err := regexp.Compile(expr)
		if err != nil {
			continue
		}
		r.re = re
		f.rules = append(f.rules, r)
	}
	return f
}

// globToRegexp translates one gitignore glob into a regexp body.
func globToRegexp(pat string) string {
	var b strings.Builder
	for i := 0; i < len(pat); i++ {
		c := pat[i]
		switch c {
		case '*':
			if i+1 < len(pat) && pat[i+1] == '*' {
				atStart := i == 0
				afterSlash := i > 0 && pat[i-1] == '/'
				atEnd := i+2 == len(pat)
				beforeSlash := i+2 < len(pat) && pat[i+2] == '/'
				switch {
				case atStart && beforeSlash: // "**/x": x at any depth
					b.WriteString("(?:.*/)?")
					i += 2
					continue
				case afterSlash && atEnd: // "x/**": everything inside x
					b.WriteString(".*")
					i++
					continue
				case afterSlash && beforeSlash: // "a/**/b": zero or more dirs
					b.WriteString("(?:.*/)?")
					i += 2
					continue
				}
				// Any other "**" is just two stars.
				b.WriteString("[^/]*")
				i++
				continue
			}
			b.WriteString("[^/]*")
		case '?':
			b.WriteString("[^/]")
		case '[':
			end := strings.IndexByte(pat[i+1:], ']')
			if end < 0 {
				b.WriteString(`\[`)
				continue
			}
			class := pat[i+1 : i+1+end]
			if strings.HasPrefix(class, "!") {
				class = "^" + class[1:]
			}
			b.WriteString("[" + strings.ReplaceAll(class, `\`, `\\`) + "]")
			i += end + 1
		case '\\':
			if i+1 < len(pat) {
				i++
				b.WriteString(regexp.QuoteMeta(pat[i : i+1]))
			}
		default:
			b.WriteString(regexp.QuoteMeta(pat[i : i+1]))
		}
	}
	return b.String()
}

// match reports whether this file decides p (a slash path relative to
// the workspace root, inside f.dir) and if so whether it is ignored.
// The last matching rule wins, as in git.
func (f *ignoreFile) match(p string, isDir bool) (ignored, decided bool) {
	rel := p
	if f.dir != "" {
		rel = strings.TrimPrefix(p, f.dir+"/")
	}
	for i := len(f.rules) - 1; i >= 0; i-- {
		r := f.rules[i]
		if r.dirOnly && !isDir {
			continue
		}
		if r.re.MatchString(rel) {
			return !r.negate, true
		}
	}
	return false, false
}
