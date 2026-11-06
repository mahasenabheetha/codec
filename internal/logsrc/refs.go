// Package logsrc goes from a log line to the source that wrote it
// (decision 99): it finds the file and line references a log carries
// (stack frames, compiler and tool errors) and matches them to the
// files of the open folder. It is a pure engine package: it is handed
// text and a list of paths, and reads nothing itself.
package logsrc

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Ref is a file and line reference found in a text.
type Ref struct {
	Start, End int    // byte offsets of the reference in the text
	Path       string // slash-separated, as written (Java: built from the package)
	Line       int    // 1-based
	Col        int    // 1-based; 0 when not given
}

// MaxRefs bounds the references taken from one text: a 1 MB line of
// paths shouldn't turn into a million links.
const MaxRefs = 200

// The patterns, most specific first: a span one of them takes is not
// read again by the ones after it (Java's "(X.java:42)" is also a
// plain "X.java:42", but only the Java pattern knows the package).
var (
	// Python: File "app/x.py", line 12, in f
	pyRef = regexp.MustCompile(`File "([^"\n]+)", line (\d+)`)
	// .NET: at X.Y() in C:\src\X.cs:line 42
	dotnetRef = regexp.MustCompile(` in ((?:[A-Za-z]:)?[^\s:]+?\.\w+):line (\d+)`)
	// Java, Kotlin, Scala: at com.acme.Shop.buy(Shop.java:42); the
	// frame's class gives the package folders.
	javaRef = regexp.MustCompile(`([\w$]+(?:\.[\w$<>]+)+)\(([\w$-]+\.(?:java|kt|kts|scala|groovy|clj)):(\d+)\)`)
	// Terraform: on main.tf line 12, in resource …
	tfRef = regexp.MustCompile(`\bon ([^\s,:]+\.(?:tf|tfvars|hcl)) line (\d+)`)
	// MSBuild, dotnet, tsc: src/x.ts(10,5): error TS2322 · X.cs(42,7)
	parenRef = regexp.MustCompile(`((?:[A-Za-z]:)?[\w.@~+\\/-]*[\w-]\.(\w{1,12}))\((\d+)(?:,(\d+))?\)`)
	// Everything else: Go, Node, Rust, Helm, Ansible, eslint's unix
	// format, gcc: path/x.go:12 · /app/x.js:10:5 · C:\x\y.go:7
	plainRef = regexp.MustCompile(`((?:[A-Za-z]:[\\/])?[\w.@~+\\/-]*[\w-]):(\d+)(?::(\d+))?`)
	// eslint's default output: the file on a line of its own, then its
	// problems as "  10:5  error  …".
	eslintRow = regexp.MustCompile(`(?m)^[ \t]+(\d+):(\d+)[ \t]+(?:error|warning)\s`)
)

// Find returns the file and line references in text, in the order they
// appear. Only files with a source or config extension count (and the
// few well-known names without one, such as Dockerfile), so a time
// (12:30:45), a port (localhost:8080) or a host (api.example.com:443)
// is not taken for a file.
func Find(text string) []Ref {
	var out []Ref
	taken := func(s, e int) bool {
		for _, r := range out {
			if s < r.End && e > r.Start {
				return true
			}
		}
		return false
	}
	add := func(s, e int, path string, line, col int) {
		if len(out) >= MaxRefs || line <= 0 || taken(s, e) {
			return
		}
		out = append(out, Ref{Start: s, End: e, Path: clean(path), Line: line, Col: col})
	}

	for _, m := range pyRef.FindAllStringSubmatchIndex(text, MaxRefs) {
		add(m[0], m[1], text[m[2]:m[3]], num(text, m[4], m[5]), 0)
	}
	for _, m := range dotnetRef.FindAllStringSubmatchIndex(text, MaxRefs) {
		// The link is the path and its line, not the leading " in ".
		add(m[2], m[1], text[m[2]:m[3]], num(text, m[4], m[5]), 0)
	}
	for _, m := range javaRef.FindAllStringSubmatchIndex(text, MaxRefs) {
		file := text[m[4]:m[5]]
		add(m[4], m[7], javaPath(text[m[2]:m[3]], file), num(text, m[6], m[7]), 0)
	}
	for _, m := range tfRef.FindAllStringSubmatchIndex(text, MaxRefs) {
		add(m[2], m[1], text[m[2]:m[3]], num(text, m[4], m[5]), 0)
	}
	for _, m := range parenRef.FindAllStringSubmatchIndex(text, MaxRefs) {
		path := text[m[2]:m[3]]
		if !isSource(path, text[m[4]:m[5]]) {
			continue
		}
		add(m[2], m[1], path, num(text, m[6], m[7]), num(text, m[8], m[9]))
	}
	// A rejected match (a port, a time, a host) is consumed whole, so
	// "localhost:3000/static/x.js:10" leaves "/static/x.js:10" to match.
	for _, m := range plainRef.FindAllStringSubmatchIndex(text, -1) {
		s, path := m[2], text[m[2]:m[3]]
		// A "drive" inside a word is a scheme's last letter: file:///x.
		if len(path) > 1 && path[1] == ':' && s > 0 && isWord(text[s-1]) {
			s, path = s+2, path[2:]
		}
		if isSource(path, ext(path)) {
			add(s, m[1], path, num(text, m[4], m[5]), num(text, m[6], m[7]))
		}
	}
	eslint(text, add)

	slices.SortFunc(out, func(a, b Ref) int { return a.Start - b.Start })
	return out
}

// eslint links each "  10:5  error" row to the file named above it:
// the nearest line that is neither blank nor another row, when that
// line is a path and nothing else.
func eslint(text string, add func(s, e int, path string, line, col int)) {
	for _, m := range eslintRow.FindAllStringSubmatchIndex(text, MaxRefs) {
		// m[0] starts a line, so text[:end] ends with the line above's
		// newline; walk up one line at a time.
		for end := m[0]; end > 0; {
			start := strings.LastIndexByte(text[:end-1], '\n') + 1
			l := strings.TrimSpace(text[start : end-1])
			end = start
			if l == "" || rowStart.MatchString(l+" ") {
				continue
			}
			if !strings.ContainsAny(l, " \t") && isSource(l, ext(l)) {
				add(m[2], m[5], l, num(text, m[2], m[3]), num(text, m[4], m[5]))
			}
			break
		}
	}
}

var rowStart = regexp.MustCompile(`^\d+:\d+\s+(?:error|warning)\s`)

// javaPath turns a frame's class and file into a path: the class
// "com.acme.shop.Cart$Item.add" in "Cart.java" lives in
// com/acme/shop/Cart.java. A class without a package gives the file
// alone.
func javaPath(frame, file string) string {
	parts := strings.Split(frame, ".")
	// The method is last, then the class (with any $Inner); the rest
	// is the package, written in lower case by convention.
	var pkg []string
	for _, p := range parts[:max(len(parts)-2, 0)] {
		if p == "" || p[0] < 'a' || p[0] > 'z' {
			break
		}
		pkg = append(pkg, p)
	}
	if len(pkg) == 0 {
		return file
	}
	return strings.Join(pkg, "/") + "/" + file
}

// clean writes a path with forward slashes, without a leading "./" or
// a "file://" left by the pattern.
func clean(p string) string {
	p = strings.ReplaceAll(p, `\`, "/")
	for strings.HasPrefix(p, "./") {
		p = p[2:]
	}
	return p
}

func num(text string, s, e int) int {
	if s < 0 {
		return 0
	}
	n, err := strconv.Atoi(text[s:e])
	if err != nil {
		return 0
	}
	return n
}

func ext(p string) string {
	if i := strings.LastIndexByte(p, '.'); i >= 0 && !strings.ContainsAny(p[i:], `/\`) {
		return p[i+1:]
	}
	return ""
}

// isSource reports whether a path names a file a log may point into: a
// source, template or config extension, or a well-known name without
// one.
func isSource(path, ext string) bool {
	if ext == "" {
		base := path[strings.LastIndexAny(path, `/\`)+1:]
		return bareNames[base]
	}
	return sourceExts[strings.ToLower(ext)]
}

var bareNames = map[string]bool{
	"Dockerfile": true, "Containerfile": true, "Makefile": true, "makefile": true, "GNUmakefile": true,
	"Jenkinsfile": true, "Vagrantfile": true, "Gemfile": true, "Rakefile": true, "Tiltfile": true, "Justfile": true,
}

var sourceExts = func() map[string]bool {
	m := map[string]bool{}
	for _, e := range strings.Fields(`
		go mod tmpl gotmpl tpl
		java kt kts scala groovy gradle clj cljs
		py pyi pyx rb erb rake
		js mjs cjs jsx ts tsx mts cts vue svelte astro
		cs csx fs fsx vb razor cshtml xaml csproj fsproj props targets
		c h cc cpp cxx hpp hh hxx m mm swift rs zig
		php pl pm lua r dart ex exs erl hrl elm hs ml
		sh bash zsh ps1 psm1 psd1 bat cmd
		yaml yml json jsonc json5 toml ini cfg conf properties env xml
		tf tfvars hcl nomad cue jsonnet libsonnet rego j2 jinja jinja2
		sql graphql gql proto thrift avsc
		html htm css scss sass less md mdx dockerfile containerfile`) {
		m[e] = true
	}
	return m
}()

func isWord(c byte) bool {
	return c == '_' || (c|0x20) >= 'a' && (c|0x20) <= 'z' || c >= '0' && c <= '9'
}
