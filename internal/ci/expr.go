package ci

import (
	"bytes"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// ghRef is a context reference inside a GitHub expression, such as
// needs.build.outputs.version or env['HOME'].
type ghRef struct {
	ctx        string
	path       []string
	start, end int // byte offsets in the expression text
}

func (r ghRef) part(i int) string {
	if i < len(r.path) {
		return r.path[i]
	}
	return ""
}

var (
	ghRefRe  = regexp.MustCompile(`(^|[^\w.'"-])(inputs|env|secrets|matrix|needs|steps|vars|github|runner|job|jobs|strategy)((?:\.[A-Za-z_*][\w-]*|\[\s*'[^']*'\s*\])*)`)
	ghPartRe = regexp.MustCompile(`\.([A-Za-z_*][\w-]*)|\[\s*'([^']*)'\s*\]`)
)

// ghRefs finds the context references of an expression, skipping text
// inside string literals.
func ghRefs(expr string) []ghRef {
	quoted := quotedSpans(expr)
	var out []ghRef
	for _, m := range ghRefRe.FindAllStringSubmatchIndex(expr, -1) {
		start := m[4]
		if inSpans(quoted, start) {
			continue
		}
		r := ghRef{ctx: expr[m[4]:m[5]], start: start, end: m[7]}
		for _, pm := range ghPartRe.FindAllStringSubmatch(expr[m[6]:m[7]], -1) {
			r.path = append(r.path, pm[1]+pm[2])
		}
		out = append(out, r)
	}
	return out
}

// quotedSpans are the '…' literals of an expression, except indexes
// like env['X'].
func quotedSpans(s string) [][2]int {
	var out [][2]int
	for i := 0; i < len(s); i++ {
		if s[i] != '\'' {
			continue
		}
		j := i + 1
		for j < len(s) && !(s[j] == '\'' && (j+1 >= len(s) || s[j+1] != '\'')) {
			if s[j] == '\'' {
				j++
			}
			j++
		}
		if k := strings.TrimRight(s[:i], " "); !strings.HasSuffix(k, "[") {
			out = append(out, [2]int{i, j})
		}
		i = j
	}
	return out
}

func inSpans(spans [][2]int, off int) bool {
	for _, s := range spans {
		if off >= s[0] && off <= s[1] {
			return true
		}
	}
	return false
}

// --- extra expressions for the editor ---

var (
	azMacro   = regexp.MustCompile(`\$\([A-Za-z_][\w.\-]*\)`)
	glInputRe = regexp.MustCompile(`\$\[\[[^\]]*\]\]`)
)

// Expressions returns the expressions of a CI file as the editor should
// tint them. yamlkit reads ${{ }} as GitHub's run-time syntax; Azure's
// is compiled before the run, and Azure adds $(macro) and $[ runtime ]
// syntax; GitLab adds $[[ inputs ]] (filled in when the file is
// included or the pipeline is created).
func Expressions(tool string, content []byte, f *yamlkit.File) []yamlkit.Expression {
	out := append([]yamlkit.Expression(nil), f.Expressions...)
	src := bytes.TrimPrefix(content, []byte{0xEF, 0xBB, 0xBF})
	switch tool {
	case Azure:
		for i := range out {
			if out[i].Syntax == "github" {
				out[i].Syntax, out[i].Phase = "azure", yamlkit.PhaseTemplate
			}
		}
		li := newLines(src)
		for _, m := range azMacro.FindAllIndex(src, -1) {
			out = append(out, li.expr(src, m[0], m[1], "azure-macro", yamlkit.PhaseRuntime))
		}
		for _, sp := range runtimeSpans(src) {
			out = append(out, li.expr(src, sp[0], sp[1], "azure-runtime", yamlkit.PhaseRuntime))
		}
	case GitLab:
		li := newLines(src)
		for _, m := range glInputRe.FindAllIndex(src, -1) {
			out = append(out, li.expr(src, m[0], m[1], "gitlab-input", yamlkit.PhaseTemplate))
		}
	}
	return out
}

// runtimeSpans finds Azure $[ … ] expressions (brackets balanced,
// quotes respected).
func runtimeSpans(src []byte) [][2]int {
	var out [][2]int
	for i := 0; i+1 < len(src); i++ {
		if src[i] != '$' || src[i+1] != '[' || (i+2 < len(src) && src[i+2] == '[') {
			continue
		}
		depth, quote := 0, false
		for j := i + 1; j < len(src) && src[j] != '\n'; j++ {
			switch c := src[j]; {
			case c == '\'':
				quote = !quote
			case quote:
			case c == '[':
				depth++
			case c == ']':
				depth--
				if depth == 0 {
					out = append(out, [2]int{i, j + 1})
					i = j
					j = len(src)
				}
			}
		}
	}
	return out
}

// lines maps byte offsets to positions.
type lines struct{ starts []int }

func newLines(src []byte) lines {
	l := lines{starts: []int{0}}
	for i, c := range src {
		if c == '\n' {
			l.starts = append(l.starts, i+1)
		}
	}
	return l
}

func (l lines) pos(src []byte, off int) yamlkit.Pos {
	lo, hi := 0, len(l.starts)-1
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if l.starts[mid] <= off {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return yamlkit.Pos{Offset: off, Line: lo + 1, Col: utf8.RuneCount(src[l.starts[lo]:off]) + 1}
}

func (l lines) expr(src []byte, start, end int, syntax string, phase yamlkit.Phase) yamlkit.Expression {
	return yamlkit.Expression{Range: yamlkit.Range{Start: l.pos(src, start), End: l.pos(src, end)}, Text: string(src[start:end]), Syntax: syntax, Phase: phase}
}
