// Package lint checks YAML for mistakes that parse fine but bite later:
// style traps (YAML 1.1 booleans and octals, inconsistent indentation),
// risky Kubernetes settings, and removed or deprecated Kubernetes APIs.
//
// Every finding says what is wrong, why it matters and how to fix it.
// Rules are data (see Rules) so the UI can list them and users can set
// each one's severity, or turn it off, in their settings.
//
// Like yamlkit it is pure: parsed content in, diagnostics out.
package lint

import (
	"cmp"
	"slices"
	"unicode/utf8"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Off is the level that disables a rule.
const Off = "off"

// Rule describes one check.
type Rule struct {
	ID      string `json:"id"`
	Group   string `json:"group"` // style, kubernetes, deprecation, schema
	Title   string `json:"title"`
	Default string `json:"default"` // error, warning, info or off
	Why     string `json:"why"`
}

// Config is the user's choice of levels and options.
type Config struct {
	Levels     map[string]string // rule id → error|warning|info|off; missing = default
	LineLength int               // max characters per line; 0 = 120
	K8sVersion string            // target Kubernetes version, e.g. "1.34"; "" = DefaultK8sVersion
}

// DefaultK8sVersion is the target when none is set: a release most
// managed clusters run in 2026, not the newest.
const DefaultK8sVersion = "1.34"

// K8sVersions are the versions a user can target; each has published
// JSON schemas.
var K8sVersions = []string{"1.30", "1.31", "1.32", "1.33", "1.34", "1.35", "1.36", "1.37"}

// Version returns the target Kubernetes version.
func (c Config) Version() string {
	if c.K8sVersion == "" {
		return DefaultK8sVersion
	}
	return c.K8sVersion
}

// Level returns the configured level of a rule ("" for unknown rules).
func (c Config) Level(id string) string {
	if l, ok := c.Levels[id]; ok && validLevel(l) {
		return l
	}
	if r, ok := byID[id]; ok {
		return r.Default
	}
	return ""
}

func validLevel(l string) bool {
	switch l {
	case string(yamlkit.SeverityError), string(yamlkit.SeverityWarning), string(yamlkit.SeverityInfo), Off:
		return true
	}
	return false
}

// Rules lists every rule, in display order.
func Rules() []Rule { return slices.Clone(rules) }

var byID = func() map[string]Rule {
	m := map[string]Rule{}
	for _, r := range rules {
		m[r.ID] = r
	}
	return m
}()

// Input is one parsed file.
type Input struct {
	YAML    *yamlkit.File
	Content []byte
	Type    string // provider id, e.g. "kubernetes", "helm-template"
}

// Run applies every enabled rule to in.
func Run(in Input, cfg Config) []yamlkit.Diagnostic {
	if in.YAML == nil {
		return nil
	}
	c := &checker{in: in, cfg: cfg, text: newLines(in.Content)}
	c.style()
	// Raw Helm templates aren't Kubernetes objects yet: most of their
	// pod spec is filled in by expressions. Their rendered output is
	// checked instead (see the Helm view).
	if in.Type != "helm-template" && in.Type != "helm-values" {
		c.kubernetes()
	}
	return c.out
}

// Apply re-grades diagnostics that come from elsewhere (the parser's
// duplicate keys, schema validation) by the configured level of their
// code, dropping the ones turned off, and adds the rule's explanation.
// Unknown codes pass unchanged.
func Apply(ds []yamlkit.Diagnostic, cfg Config) []yamlkit.Diagnostic {
	out := ds[:0:0]
	for _, d := range ds {
		if rule, known := byID[d.Code]; known {
			l := cfg.Level(d.Code)
			if l == Off {
				continue
			}
			d.Severity = yamlkit.Severity(l)
			d.Why = cmp.Or(d.Why, rule.Why)
			d.Source = cmp.Or(d.Source, rule.Group)
		}
		out = append(out, d)
	}
	return out
}

// checker collects findings for one file.
type checker struct {
	in   Input
	cfg  Config
	text *lines
	out  []yamlkit.Diagnostic
}

// report adds a finding for rule id unless the rule is off. why
// overrides the rule's general explanation when it isn't empty.
func (c *checker) report(id string, r yamlkit.Range, msg, hint string) {
	rule := byID[id]
	l := c.cfg.Level(id)
	if l == Off || l == "" {
		return
	}
	c.out = append(c.out, yamlkit.Diagnostic{
		Severity: yamlkit.Severity(l),
		Code:     id,
		Message:  msg,
		Hint:     hint,
		Why:      rule.Why,
		Source:   rule.Group,
		Range:    r,
	})
}

func (c *checker) enabled(id string) bool {
	l := c.cfg.Level(id)
	return l != Off && l != ""
}

// lines indexes the raw text: yamlkit positions are rune columns and
// byte offsets without a BOM, and text rules need the same.
type lines struct {
	src    []byte
	starts []int
}

func newLines(content []byte) *lines {
	src := content
	if len(src) >= 3 && src[0] == 0xEF && src[1] == 0xBB && src[2] == 0xBF {
		src = src[3:]
	}
	l := &lines{src: src, starts: []int{0}}
	for i, b := range src {
		if b == '\n' {
			l.starts = append(l.starts, i+1)
		}
	}
	return l
}

func (l *lines) count() int { return len(l.starts) }

// line returns line n (1-based) without its line ending.
func (l *lines) line(n int) string {
	start := l.starts[n-1]
	end := len(l.src)
	if n < len(l.starts) {
		end = l.starts[n] - 1
	}
	if end > start && l.src[end-1] == '\r' {
		end--
	}
	return string(l.src[start:end])
}

// pos is the position of rune column col (1-based) on line n.
func (l *lines) pos(n, col int) yamlkit.Pos {
	s := l.line(n)
	off := 0
	for i := 1; i < col && off < len(s); i++ {
		_, size := utf8.DecodeRuneInString(s[off:])
		off += size
	}
	return yamlkit.Pos{Offset: l.starts[n-1] + off, Line: n, Col: col}
}

// span is the range of columns [from, to) on line n.
func (l *lines) span(n, from, to int) yamlkit.Range {
	return yamlkit.Range{Start: l.pos(n, from), End: l.pos(n, to)}
}
