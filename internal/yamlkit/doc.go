// Package yamlkit is codec's YAML engine: it parses YAML (including
// files full of Helm/Jinja/GitHub template expressions) into a
// position-rich tree, explains syntax errors in plain English, maps
// cursor positions to paths, and converts between representations.
//
// Like internal/codec it is pure: bytes in, structs out. No file,
// network or clock access — adapters feed it content.
package yamlkit

// Pos is a location in a file. Offset is a 0-based byte offset into the
// content with any UTF-8 byte-order mark removed; Line and Col are
// 1-based, and Col counts characters (runes), not bytes — the same
// numbers an editor shows.
type Pos struct {
	Offset int `json:"offset"`
	Line   int `json:"line"`
	Col    int `json:"col"`
}

// Before reports whether p comes strictly before q.
func (p Pos) Before(q Pos) bool {
	return p.Line < q.Line || (p.Line == q.Line && p.Col < q.Col)
}

// Range is a half-open span [Start, End).
type Range struct {
	Start Pos `json:"start"`
	End   Pos `json:"end"`
}

// Contains reports whether p lies within r (End exclusive).
func (r Range) Contains(p Pos) bool {
	return !p.Before(r.Start) && p.Before(r.End)
}

// Severity grades a diagnostic. It is a string so it serialises as-is
// to the web UI and CLI.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Diagnostic is one problem found in a file, explained for humans.
type Diagnostic struct {
	Severity Severity `json:"severity"`
	Code     string   `json:"code"`           // stable id, e.g. "tab-indent"
	Message  string   `json:"message"`        // what is wrong
	Hint     string   `json:"hint,omitempty"` // how to fix it
	Range    Range    `json:"range"`
}

// Kind is the shape of a node.
type Kind string

const (
	KindMap    Kind = "map"
	KindSeq    Kind = "seq"
	KindScalar Kind = "scalar"
	KindAlias  Kind = "alias"
)

// Style records how a scalar was written, which matters when showing
// or re-emitting it.
type Style string

const (
	StylePlain   Style = "plain"
	StyleSingle  Style = "single"
	StyleDouble  Style = "double"
	StyleLiteral Style = "literal" // |
	StyleFolded  Style = "folded"  // >
)

// Core-schema tags assigned to scalars (YAML 1.2). Explicit tags written
// in the file (e.g. !Ref) are kept verbatim instead.
const (
	TagStr   = "!!str"
	TagInt   = "!!int"
	TagFloat = "!!float"
	TagBool  = "!!bool"
	TagNull  = "!!null"
	TagMerge = "!!merge"
)

// Node is one YAML value with its exact location in the source.
type Node struct {
	Kind  Kind  `json:"kind"`
	Range Range `json:"range"`

	Tag    string `json:"tag,omitempty"`    // scalars: resolved or explicit tag
	Value  string `json:"value,omitempty"`  // scalars: text; aliases: anchor name
	Style  Style  `json:"style,omitempty"`  // scalars only
	Anchor string `json:"anchor,omitempty"` // &name on this node
	Flow   bool   `json:"flow,omitempty"`   // maps/seqs written as {…} / […]

	// Templated is true when the value contains a template expression
	// (e.g. {{ .Values.x }}), so its real value is only known after
	// rendering. Value still shows the original expression text.
	Templated bool `json:"templated,omitempty"`

	Pairs []*Pair `json:"pairs,omitempty"` // KindMap
	Items []*Node `json:"items,omitempty"` // KindSeq
}

// Pair is one key/value entry of a map.
type Pair struct {
	Key   *Node `json:"key"`
	Value *Node `json:"value"`
}

// Get returns the value for key in a map node (the last one wins, as in
// most YAML parsers), or nil.
func (n *Node) Get(key string) *Node {
	if n == nil || n.Kind != KindMap {
		return nil
	}
	var found *Node
	for _, p := range n.Pairs {
		if p.Key != nil && p.Key.Kind == KindScalar && p.Key.Value == key {
			found = p.Value
		}
	}
	return found
}

// Str returns a scalar's value, or "" for nil and non-scalars.
func (n *Node) Str() string {
	if n == nil || n.Kind != KindScalar {
		return ""
	}
	return n.Value
}

// Document is one YAML document of a (possibly multi-document) file.
type Document struct {
	Index int    `json:"index"`
	Name  string `json:"name,omitempty"` // e.g. "Deployment/api"; "" if none
	Range Range  `json:"range"`
	Root  *Node  `json:"root,omitempty"` // nil if empty or unparseable
}

// Phase says when an expression is evaluated: before the YAML exists
// (a template engine) or while the system runs (a workflow engine).
type Phase string

const (
	PhaseTemplate Phase = "template" // Helm/Go templates, Jinja
	PhaseRuntime  Phase = "runtime"  // Argo {{…}}, GitHub ${{…}}
)

// Expression is a template or runtime expression found in the source.
type Expression struct {
	Range  Range  `json:"range"`
	Text   string `json:"text"`
	Syntax string `json:"syntax"` // go-template, jinja, github, argo
	Phase  Phase  `json:"phase"`
	// Standalone is true when the expression fills its whole line
	// (control flow like {{- if … }}), rather than sitting in a value.
	Standalone bool `json:"standalone,omitempty"`
}

// File is the parsed form of one YAML file.
type File struct {
	Docs        []*Document  `json:"docs"`
	Diagnostics []Diagnostic `json:"diagnostics"`
	Expressions []Expression `json:"expressions"`
	CRLF        bool         `json:"crlf,omitempty"` // uses \r\n line endings
	BOM         bool         `json:"bom,omitempty"`  // started with a UTF-8 BOM
}

// HasErrors reports whether any diagnostic is an error.
func (f *File) HasErrors() bool {
	for _, d := range f.Diagnostics {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

// HasTemplates reports whether the file contains template-time
// expressions, i.e. it is a template rather than final YAML.
func (f *File) HasTemplates() bool {
	for _, e := range f.Expressions {
		if e.Phase == PhaseTemplate {
			return true
		}
	}
	return false
}
