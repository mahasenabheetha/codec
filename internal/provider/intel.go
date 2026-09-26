package provider

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Editor features, shaped like their LSP counterparts so a future
// `codec lsp` can reuse them. A provider adds domain knowledge by
// implementing any of the optional interfaces below (Go checks with a
// type assertion); the generic implementations answer otherwise.

// Hover is what hovering a position shows.
type Hover struct {
	Range yamlkit.Range `json:"range"` // the hovered node or entry
	Title string        `json:"title"` // e.g. "spec.replicas"
	Rows  []HoverRow    `json:"rows,omitempty"`
	Code  string        `json:"code,omitempty"` // snippet, e.g. an alias's target
}

// HoverRow is one labelled fact in a hover.
type HoverRow struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Location is a definition target.
type Location struct {
	Path  string        `json:"path,omitempty"` // "" = the same file
	Range yamlkit.Range `json:"range"`
}

// Completion is one suggestion; it replaces the text in Range.
type Completion struct {
	Label  string        `json:"label"`
	Insert string        `json:"insert,omitempty"` // text to insert; "" = Label
	Detail string        `json:"detail,omitempty"`
	Doc    string        `json:"doc,omitempty"` // longer description
	Kind   string        `json:"kind"`          // anchor, key, value
	Range  yamlkit.Range `json:"range"`
}

// Optional provider capabilities.
type (
	Hoverer interface {
		Hover(f *File, pos yamlkit.Pos) *Hover
	}
	Definer interface {
		Definition(f *File, pos yamlkit.Pos) []Location
	}
	Completer interface {
		Complete(f *File, pos yamlkit.Pos) []Completion
	}
	Diagnoser interface {
		Diagnostics(f *File) []yamlkit.Diagnostic
	}
)

// HoverAt asks p first, then falls back to the generic hover.
func HoverAt(p Provider, f *File, pos yamlkit.Pos) *Hover {
	if h, ok := p.(Hoverer); ok {
		if r := h.Hover(f, pos); r != nil {
			return r
		}
	}
	return genericHover(f, pos)
}

// DefinitionAt asks p first, then falls back to in-file anchors.
func DefinitionAt(p Provider, f *File, pos yamlkit.Pos) []Location {
	if d, ok := p.(Definer); ok {
		if r := d.Definition(f, pos); len(r) > 0 {
			return r
		}
	}
	return genericDefinition(f, pos)
}

// CompleteAt returns p's suggestions followed by the generic ones.
func CompleteAt(p Provider, f *File, pos yamlkit.Pos) []Completion {
	var out []Completion
	if c, ok := p.(Completer); ok {
		out = c.Complete(f, pos)
	}
	return append(out, genericComplete(f, pos)...)
}

// --- generic hover ---

func genericHover(f *File, pos yamlkit.Pos) *Hover {
	if f.YAML == nil {
		return nil
	}
	// Expressions first: they sit inside values but mean something else.
	for _, e := range f.YAML.Expressions {
		if e.Range.Contains(pos) {
			return exprHover(e)
		}
	}
	doc := f.YAML.DocAt(pos)
	if doc == nil {
		return nil
	}
	path, n := doc.PathAt(pos)
	if n == nil || len(path) == 0 {
		return nil
	}
	h := &Hover{Title: path.String(), Range: n.Range}
	// On a key, the whole entry is what the hover is about.
	if last := path[len(path)-1]; !last.IsIndex {
		if pr := doc.NodeAt(path[:len(path)-1]).Pair(last.Key); pr != nil && pr.Key.Range.Contains(pos) {
			h.Range.Start = pr.Key.Range.Start
		}
	}
	h.Rows, h.Code = describe(n)

	if n.Anchor != "" {
		h.Rows = append(h.Rows, HoverRow{"Anchor", fmt.Sprintf("&%s · %s", n.Anchor, plural(countAliases(doc, n.Anchor), "reference"))})
	}
	if n.Kind == yamlkit.KindAlias {
		if target := anchorFor(doc, n.Value, n.Range.Start); target != nil {
			h.Rows = append(h.Rows, HoverRow{"Alias of", fmt.Sprintf("&%s, line %d", n.Value, target.Range.Start.Line)})
			h.Code = clip(yamlkit.Text(f.Content, target.Range))
		} else {
			h.Rows = append(h.Rows, HoverRow{"Alias of", "&" + n.Value + " (not defined above)"})
		}
	}
	return h
}

// describe says what kind of value n is, plus a snippet for values too
// long for one row.
func describe(n *yamlkit.Node) ([]HoverRow, string) {
	switch n.Kind {
	case yamlkit.KindMap:
		return []HoverRow{{"Type", "map · " + plural(len(n.Pairs), "key")}}, ""
	case yamlkit.KindSeq:
		return []HoverRow{{"Type", "list · " + plural(len(n.Items), "item")}}, ""
	case yamlkit.KindAlias:
		return []HoverRow{{"Type", "alias"}}, ""
	}

	rows := []HoverRow{{"Type", typeName(n.Tag)}}
	if s := styleName(n.Style); s != "" {
		rows = append(rows, HoverRow{"Style", s})
	}
	code := ""
	switch {
	case n.Templated:
		rows = append(rows, HoverRow{"Value", "set by a template; known after rendering"})
	case strings.Contains(strings.TrimRight(n.Value, "\n"), "\n"):
		code = clip(n.Value)
	case n.Value != "":
		rows = append(rows, HoverRow{"Value", preview(n.Value)})
	}
	if note := yaml11Note(n); note != "" {
		rows = append(rows, HoverRow{"Note", note})
	}
	return rows, code
}

func typeName(tag string) string {
	switch tag {
	case yamlkit.TagStr:
		return "string"
	case yamlkit.TagInt:
		return "integer"
	case yamlkit.TagFloat:
		return "number"
	case yamlkit.TagBool:
		return "boolean"
	case yamlkit.TagNull:
		return "null"
	case "":
		return "scalar"
	}
	return tag // explicit tag such as !Ref
}

func styleName(s yamlkit.Style) string {
	switch s {
	case yamlkit.StyleSingle:
		return "single-quoted"
	case yamlkit.StyleDouble:
		return "double-quoted"
	case yamlkit.StyleLiteral:
		return "literal block (|), keeps newlines"
	case yamlkit.StyleFolded:
		return "folded block (>), joins lines"
	}
	return ""
}

var yaml11Bools = map[string]bool{"y": true, "yes": true, "n": true, "no": true, "on": true, "off": true}

// yaml11Note warns about plain scalars that YAML 1.1 tools (PyYAML,
// Ansible, older Kubernetes tooling) read differently from YAML 1.2.
func yaml11Note(n *yamlkit.Node) string {
	if n.Style != yamlkit.StylePlain {
		return ""
	}
	v := n.Value
	switch {
	case n.Tag == yamlkit.TagStr && yaml11Bools[strings.ToLower(v)]:
		return "YAML 1.1 tools read this as a boolean; quote it to keep a string"
	case n.Tag == yamlkit.TagInt && len(v) > 1 && v[0] == '0' && !strings.ContainsAny(v, "xXoO"):
		return "YAML 1.1 tools read a leading 0 as octal"
	}
	return ""
}

func exprHover(e yamlkit.Expression) *Hover {
	title := map[string]string{
		"go-template": "Go template expression",
		"jinja":       "Jinja expression",
		"github":      "GitHub Actions expression",
		"argo":        "Argo expression",
	}[e.Syntax]
	when := "at render time, before the YAML is parsed"
	switch {
	case e.Phase == yamlkit.PhaseRuntime:
		when = "at run time, by the workflow engine"
	case e.Syntax == "go-template":
		when = "at render time (helm template)"
	case e.Syntax == "jinja":
		when = "at render time (Ansible/Jinja)"
	}
	h := &Hover{Range: e.Range, Title: title, Rows: []HoverRow{{"Evaluated", when}}, Code: clip(e.Text)}
	if e.Standalone {
		h.Rows = append(h.Rows, HoverRow{"Role", "control flow on its own line"})
	}
	return h
}

// --- anchors and aliases ---

// anchorFor finds the node that defines &name for an alias at before:
// the last definition above it in the same document, as YAML resolves it.
func anchorFor(doc *yamlkit.Document, name string, before yamlkit.Pos) *yamlkit.Node {
	var found *yamlkit.Node
	yamlkit.Walk(doc.Root, func(n *yamlkit.Node) bool {
		if n.Anchor == name && n.Range.Start.Before(before) {
			found = n
		}
		return true
	})
	return found
}

func countAliases(doc *yamlkit.Document, name string) int {
	count := 0
	yamlkit.Walk(doc.Root, func(n *yamlkit.Node) bool {
		if n.Kind == yamlkit.KindAlias && n.Value == name {
			count++
		}
		return true
	})
	return count
}

func genericDefinition(f *File, pos yamlkit.Pos) []Location {
	if f.YAML == nil {
		return nil
	}
	doc := f.YAML.DocAt(pos)
	if doc == nil {
		return nil
	}
	_, n := doc.PathAt(pos)
	if n == nil || n.Kind != yamlkit.KindAlias {
		return nil
	}
	if target := anchorFor(doc, n.Value, n.Range.Start); target != nil {
		return []Location{{Range: target.Range}}
	}
	return nil
}

// --- completion ---

var (
	aliasPrefix = regexp.MustCompile(`\*([\w.-]*)$`)
	anchorDef   = regexp.MustCompile(`&([\w.-]+)`)
)

// genericComplete suggests anchor names after "*". It works on raw
// text, because the YAML is usually unparseable mid-typing.
func genericComplete(f *File, pos yamlkit.Pos) []Completion {
	if len(f.Content) == 0 || pos.Line < 1 {
		return nil
	}
	// Offsets and lines ignore a leading BOM, as yamlkit does.
	src := bytes.TrimPrefix(f.Content, []byte{0xEF, 0xBB, 0xBF})
	lines := strings.Split(string(src), "\n")
	if pos.Line > len(lines) {
		return nil
	}
	line := []rune(strings.TrimRight(lines[pos.Line-1], "\r"))
	prefix := string(line[:min(len(line), max(0, pos.Col-1))])
	m := aliasPrefix.FindStringSubmatch(prefix)
	if m == nil {
		return nil
	}
	partial := m[1]

	// Anchors defined above, in this document (after the last "---").
	docStart := 0
	for i := pos.Line - 2; i >= 0; i-- {
		if strings.HasPrefix(lines[i], "---") {
			docStart = i + 1
			break
		}
	}
	seen := map[string]int{} // name -> line of its latest definition
	var order []string
	for i := docStart; i < pos.Line; i++ {
		text := lines[i]
		if i == pos.Line-1 {
			text = prefix
		}
		for _, am := range anchorDef.FindAllStringSubmatch(text, -1) {
			if _, ok := seen[am[1]]; !ok {
				order = append(order, am[1])
			}
			seen[am[1]] = i + 1
		}
	}

	start := yamlkit.PosAt(f.Content, pos.Line, pos.Col-utf8.RuneCountInString(partial))
	var out []Completion
	for _, name := range order {
		if strings.HasPrefix(name, partial) {
			out = append(out, Completion{
				Label:  name,
				Detail: fmt.Sprintf("anchor · line %d", seen[name]),
				Kind:   "anchor",
				Range:  yamlkit.Range{Start: start, End: pos},
			})
		}
	}
	return out
}

// --- analysis ---

// Analysis is everything the editor shows for a file at once.
type Analysis struct {
	Type        string               `json:"type"`
	TypeTitle   string               `json:"typeTitle"`
	Docs        []DocSummary         `json:"docs"`
	Diagnostics []yamlkit.Diagnostic `json:"diagnostics"`
	Expressions []yamlkit.Expression `json:"expressions"`
}

// DocSummary is one document with its outline.
type DocSummary struct {
	Index   int           `json:"index"`
	Name    string        `json:"name,omitempty"`
	Range   yamlkit.Range `json:"range"`
	Symbols []Symbol      `json:"symbols"`
	Schema  *SchemaStatus `json:"schema,omitempty"` // set by the checker
}

// SchemaStatus says which schema a document was checked against.
type SchemaStatus struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	State   string `json:"state"`             // ok, pending, none, unavailable
	Message string `json:"message,omitempty"` // for none/unavailable
}

// Analyze classifies f and gathers its outline, diagnostics and
// expressions. Slices are never nil, so JSON clients get [] not null.
func (r *Registry) Analyze(f *File) *Analysis {
	p := r.Best(f)
	a := &Analysis{
		Type:        p.ID(),
		TypeTitle:   p.Title(),
		Docs:        []DocSummary{},
		Diagnostics: slices.Clone(f.YAML.Diagnostics),
		Expressions: f.YAML.Expressions,
	}
	if d, ok := p.(Diagnoser); ok {
		a.Diagnostics = append(a.Diagnostics, d.Diagnostics(f)...)
	}
	slices.SortStableFunc(a.Diagnostics, func(x, y yamlkit.Diagnostic) int {
		switch {
		case x.Range.Start.Before(y.Range.Start):
			return -1
		case y.Range.Start.Before(x.Range.Start):
			return 1
		}
		return 0
	})
	if a.Diagnostics == nil {
		a.Diagnostics = []yamlkit.Diagnostic{}
	}
	if a.Expressions == nil {
		a.Expressions = []yamlkit.Expression{}
	}
	for _, d := range f.YAML.Docs {
		syms := p.Symbols(d)
		if syms == nil {
			syms = []Symbol{}
		}
		a.Docs = append(a.Docs, DocSummary{Index: d.Index, Name: d.Name, Range: d.Range, Symbols: syms})
	}
	return a
}

// --- helpers ---

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// clip keeps a snippet to 12 lines.
func clip(s string) string {
	s = strings.TrimRight(s, "\n")
	lines := strings.Split(s, "\n")
	if len(lines) > 12 {
		return strings.Join(lines[:12], "\n") + "\n…"
	}
	return s
}
