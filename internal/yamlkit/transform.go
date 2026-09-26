package yamlkit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// Everything here produces text to show or copy. codec never writes it
// back over the user's files.

// ErrTemplated is returned when an operation would destroy template
// expressions, e.g. re-formatting a Helm template.
var ErrTemplated = errors.New("the file contains template expressions ({{ … }}); re-emitting it would break them")

// Error is a diagnostic returned as an error, so callers can still get
// at its position.
type Error struct{ Diagnostic }

func (e *Error) Error() string {
	return fmt.Sprintf("line %d, column %d: %s", e.Range.Start.Line, e.Range.Start.Col, e.Message)
}

// firstError returns the first syntax-level error in f. Duplicate keys
// don't block conversions: the file is still readable.
func firstError(f *File) error {
	for _, d := range f.Diagnostics {
		if d.Severity == SeverityError && d.Code != "duplicate-key" {
			return &Error{d}
		}
	}
	return nil
}

// FormatOptions controls Format.
type FormatOptions struct {
	Indent          int  // spaces per level; 0 means 2
	SortKeys        bool // sort every map's keys alphabetically
	KubernetesOrder bool // top level: apiVersion, kind, metadata, spec, … first
}

// k8sOrder is the conventional top-level key order of a manifest.
var k8sOrder = []string{"apiVersion", "kind", "metadata", "spec", "data", "stringData", "type", "status"}

// Format re-emits YAML with consistent indentation, keeping comments,
// quoting styles and document boundaries.
func Format(content []byte, opts FormatOptions) ([]byte, error) {
	src, _ := stripBOM(content)
	f := Parse(src)
	if f.HasTemplates() {
		return nil, ErrTemplated
	}
	if err := firstError(f); err != nil {
		return nil, err
	}
	indent := opts.Indent
	if indent <= 0 {
		indent = 2
	}

	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(indent)
	dec := yaml.NewDecoder(bytes.NewReader(src))
	for {
		var n yaml.Node
		err := dec.Decode(&n)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(n.Content) == 0 {
			continue // empty document
		}
		reorder(n.Content[0], opts, true)
		if err := enc.Encode(&n); err != nil {
			return nil, err
		}
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	res := out.Bytes()
	if f.CRLF {
		res = bytes.ReplaceAll(res, []byte("\n"), []byte("\r\n"))
	}
	return res, nil
}

// reorder sorts mapping entries in place per opts, recursively. It also
// clears the tag yaml.v3 puts on merge keys, which it would otherwise
// print back as "!!merge <<:".
func reorder(n *yaml.Node, opts FormatOptions, top bool) {
	if n.Kind == yaml.ScalarNode && n.Tag == TagMerge {
		n.Tag = ""
	}
	if n.Kind == yaml.MappingNode && (opts.SortKeys || (opts.KubernetesOrder && top)) {
		type kv struct{ k, v *yaml.Node }
		pairs := make([]kv, 0, len(n.Content)/2)
		for i := 0; i+1 < len(n.Content); i += 2 {
			pairs = append(pairs, kv{n.Content[i], n.Content[i+1]})
		}
		rank := func(k string) int {
			if opts.KubernetesOrder && top {
				for i, name := range k8sOrder {
					if k == name {
						return i
					}
				}
			}
			return len(k8sOrder)
		}
		sort.SliceStable(pairs, func(i, j int) bool {
			ri, rj := rank(pairs[i].k.Value), rank(pairs[j].k.Value)
			if ri != rj {
				return ri < rj
			}
			return opts.SortKeys && pairs[i].k.Value < pairs[j].k.Value
		})
		for i, p := range pairs {
			n.Content[2*i], n.Content[2*i+1] = p.k, p.v
		}
	}
	for _, c := range n.Content {
		reorder(c, opts, false)
	}
}

// Resolve returns a copy of root with aliases replaced by what they
// point to and merge keys (<<) applied — what consumers actually see.
func Resolve(root *Node) (*Node, error) {
	r := &resolver{anchors: map[string]*Node{}}
	return r.resolve(root, 0)
}

type resolver struct {
	anchors map[string]*Node
}

const maxDepth = 256

func (r *resolver) resolve(n *Node, depth int) (*Node, error) {
	if n == nil {
		return nil, nil
	}
	if depth > maxDepth {
		return nil, errors.New("aliases nest too deeply (is an anchor referring to itself?)")
	}
	if n.Anchor != "" {
		r.anchors[n.Anchor] = n
	}
	switch n.Kind {
	case KindAlias:
		target, ok := r.anchors[n.Value]
		if !ok {
			return nil, &Error{Diagnostic{Severity: SeverityError, Code: "unknown-alias",
				Message: fmt.Sprintf("Alias *%s refers to an anchor that isn't defined before it", n.Value), Range: n.Range}}
		}
		return r.resolve(target, depth+1)
	case KindSeq:
		out := &Node{Kind: KindSeq, Range: n.Range, Flow: n.Flow, Items: []*Node{}}
		for _, it := range n.Items {
			v, err := r.resolve(it, depth+1)
			if err != nil {
				return nil, err
			}
			out.Items = append(out.Items, v)
		}
		return out, nil
	case KindMap:
		return r.resolveMap(n, depth)
	default:
		c := *n
		c.Anchor = ""
		return &c, nil
	}
}

// resolveMap applies YAML merge keys: entries from << sources are added
// where the merge key sits, unless the map sets the key explicitly
// (explicit keys always win) or an earlier source already provided it.
func (r *resolver) resolveMap(n *Node, depth int) (*Node, error) {
	out := &Node{Kind: KindMap, Range: n.Range, Flow: n.Flow, Pairs: []*Pair{}}
	explicit := map[string]bool{}
	for _, p := range n.Pairs {
		if p.Key.Tag != TagMerge {
			explicit[keyText(p.Key)] = true
		}
	}
	added := map[string]bool{}
	for _, p := range n.Pairs {
		if p.Key.Tag != TagMerge {
			v, err := r.resolve(p.Value, depth+1)
			if err != nil {
				return nil, err
			}
			k := *p.Key
			out.Pairs = append(out.Pairs, &Pair{Key: &k, Value: v})
			added[keyText(p.Key)] = true
			continue
		}
		src, err := r.resolve(p.Value, depth+1)
		if err != nil {
			return nil, err
		}
		sources := []*Node{src}
		if src != nil && src.Kind == KindSeq {
			sources = src.Items
		}
		for _, s := range sources {
			if s == nil || s.Kind != KindMap {
				return nil, &Error{Diagnostic{Severity: SeverityError, Code: "bad-merge",
					Message: "A merge key (<<) must point to a map or a list of maps", Range: p.Value.Range}}
			}
			for _, sp := range s.Pairs {
				k := keyText(sp.Key)
				if explicit[k] || added[k] {
					continue
				}
				out.Pairs = append(out.Pairs, sp)
				added[k] = true
			}
		}
	}
	return out, nil
}

// resolvedRoots resolves every parsable document of f.
func resolvedRoots(f *File) ([]*Node, error) {
	if err := firstError(f); err != nil {
		return nil, err
	}
	var roots []*Node
	for _, d := range f.Docs {
		if d.Root == nil {
			continue
		}
		r, err := Resolve(d.Root)
		if err != nil {
			return nil, err
		}
		roots = append(roots, r)
	}
	return roots, nil
}

// ResolvedYAML renders f with anchors expanded and merges applied.
// Comments are not kept: this is a view of the data, not the file.
func ResolvedYAML(f *File) ([]byte, error) {
	roots, err := resolvedRoots(f)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	for _, r := range roots {
		if err := enc.Encode(toYAMLNode(r)); err != nil {
			return nil, err
		}
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// toYAMLNode converts a (resolved) Node for emitting with yaml.v3.
func toYAMLNode(n *Node) *yaml.Node {
	if n == nil {
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: TagNull, Value: "null"}
	}
	switch n.Kind {
	case KindMap:
		out := &yaml.Node{Kind: yaml.MappingNode}
		if n.Flow {
			out.Style = yaml.FlowStyle
		}
		for _, p := range n.Pairs {
			out.Content = append(out.Content, toYAMLNode(p.Key), toYAMLNode(p.Value))
		}
		return out
	case KindSeq:
		out := &yaml.Node{Kind: yaml.SequenceNode}
		if n.Flow {
			out.Style = yaml.FlowStyle
		}
		for _, it := range n.Items {
			out.Content = append(out.Content, toYAMLNode(it))
		}
		return out
	default:
		out := &yaml.Node{Kind: yaml.ScalarNode, Tag: n.Tag, Value: n.Value}
		switch n.Style {
		case StyleLiteral:
			out.Style = yaml.LiteralStyle
		case StyleFolded:
			out.Style = yaml.FoldedStyle
		}
		if n.Tag == TagNull && n.Value == "" {
			out.Value = "null"
		}
		return out
	}
}

// ToJSON converts f to JSON, keeping key order. One document becomes a
// JSON value; several become a JSON array of them.
func ToJSON(f *File, indent string) ([]byte, error) {
	roots, err := resolvedRoots(f)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	switch len(roots) {
	case 0:
		b.WriteString("null")
	case 1:
		writeJSON(&b, roots[0], indent, 0)
	default:
		seq := &Node{Kind: KindSeq, Items: roots}
		writeJSON(&b, seq, indent, 0)
	}
	b.WriteByte('\n')
	return b.Bytes(), nil
}

func writeJSON(b *bytes.Buffer, n *Node, indent string, depth int) {
	nl := func(d int) {
		if indent != "" {
			b.WriteByte('\n')
			b.WriteString(strings.Repeat(indent, d))
		}
	}
	switch {
	case n == nil:
		b.WriteString("null")
	case n.Kind == KindMap:
		if len(n.Pairs) == 0 {
			b.WriteString("{}")
			return
		}
		b.WriteByte('{')
		for i, p := range n.Pairs {
			if i > 0 {
				b.WriteByte(',')
			}
			nl(depth + 1)
			key := keyText(p.Key)
			if p.Key.Kind != KindScalar {
				var kb bytes.Buffer
				writeJSON(&kb, p.Key, "", 0)
				key = kb.String()
			}
			writeString(b, key)
			b.WriteByte(':')
			if indent != "" {
				b.WriteByte(' ')
			}
			writeJSON(b, p.Value, indent, depth+1)
		}
		nl(depth)
		b.WriteByte('}')
	case n.Kind == KindSeq:
		if len(n.Items) == 0 {
			b.WriteString("[]")
			return
		}
		b.WriteByte('[')
		for i, it := range n.Items {
			if i > 0 {
				b.WriteByte(',')
			}
			nl(depth + 1)
			writeJSON(b, it, indent, depth+1)
		}
		nl(depth)
		b.WriteByte(']')
	default:
		b.WriteString(jsonScalar(n))
	}
}

// jsonScalar renders a scalar as its JSON value according to its tag.
func jsonScalar(n *Node) string {
	switch n.Tag {
	case TagNull:
		return "null"
	case TagBool:
		return strings.ToLower(n.Value)
	case TagInt:
		if i, err := strconv.ParseInt(n.Value, 0, 64); err == nil {
			return strconv.FormatInt(i, 10)
		}
	case TagFloat:
		if json.Valid([]byte(n.Value)) {
			return n.Value
		}
		v := strings.ToLower(strings.TrimPrefix(n.Value, "+"))
		if f, err := strconv.ParseFloat(v, 64); err == nil && !math.IsInf(f, 0) && !math.IsNaN(f) {
			return strconv.FormatFloat(f, 'g', -1, 64)
		}
	}
	var b bytes.Buffer
	writeString(&b, n.Value)
	return b.String()
}

func writeString(b *bytes.Buffer, s string) {
	enc, _ := json.Marshal(s)
	b.Write(enc)
}

// FromJSON converts JSON to block-style YAML, keeping key order.
func FromJSON(content []byte) ([]byte, error) {
	if !json.Valid(content) {
		var v any
		err := json.Unmarshal(content, &v)
		var se *json.SyntaxError
		if errors.As(err, &se) {
			p := newLineIndex(content).pos(int(se.Offset))
			return nil, fmt.Errorf("invalid JSON at line %d, column %d: %v", p.Line, p.Col, se)
		}
		return nil, fmt.Errorf("invalid JSON: %v", err)
	}
	var n yaml.Node
	if err := yaml.Unmarshal(content, &n); err != nil {
		return nil, err
	}
	blockStyle(&n)
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(&n); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// blockStyle clears JSON's flow/quoted styles; the encoder re-quotes
// only strings that need it.
func blockStyle(n *yaml.Node) {
	n.Style = 0
	for _, c := range n.Content {
		blockStyle(c)
	}
}

// FlatEntry is one leaf of a flattened document.
type FlatEntry struct {
	Doc   int
	Path  Path
	Value *Node
}

// Flatten lists every leaf of every document as a path and value.
// Empty maps and lists are leaves too, so nothing is lost.
func Flatten(f *File) ([]FlatEntry, error) {
	roots, err := resolvedRoots(f)
	if err != nil {
		return nil, err
	}
	var out []FlatEntry
	for i, r := range roots {
		var walk func(n *Node, p Path)
		walk = func(n *Node, p Path) {
			switch {
			case n.Kind == KindMap && len(n.Pairs) > 0:
				for _, pr := range n.Pairs {
					walk(pr.Value, append(append(Path{}, p...), Segment{Key: keyText(pr.Key)}))
				}
			case n.Kind == KindSeq && len(n.Items) > 0:
				for j, it := range n.Items {
					walk(it, append(append(Path{}, p...), Segment{Index: j, IsIndex: true}))
				}
			default:
				out = append(out, FlatEntry{Doc: i, Path: p, Value: n})
			}
		}
		walk(r, nil)
	}
	return out, nil
}

// FlattenText renders Flatten's output as "path: value" lines, with a
// comment line between documents.
func FlattenText(f *File) ([]byte, error) {
	entries, err := Flatten(f)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	doc := -1
	for _, e := range entries {
		if e.Doc != doc {
			if doc >= 0 {
				b.WriteString("---\n")
			}
			doc = e.Doc
		}
		path := e.Path.String()
		if path == "" {
			path = "."
		}
		fmt.Fprintf(&b, "%s: %s\n", path, flatValue(e.Value))
	}
	return b.Bytes(), nil
}

var needsQuoteRe = regexp.MustCompile(`^[\s\-?:,\[\]{}#&*!|>'"%@` + "`" + `]|:\s|\s#|\s$|[\n\r\t]`)

// flatValue renders a leaf as a one-line YAML value that reads back as
// the same value.
func flatValue(n *Node) string {
	switch {
	case n.Kind == KindMap:
		return "{}"
	case n.Kind == KindSeq:
		return "[]"
	case n.Tag == TagNull:
		return "null"
	case n.Tag == TagStr:
		if n.Value == "" || resolvePlain(n.Value) != TagStr || needsQuoteRe.MatchString(n.Value) {
			var b bytes.Buffer
			writeString(&b, n.Value)
			return b.String()
		}
	}
	return n.Value
}

// Unflatten turns "path: value" lines (as written by FlattenText) back
// into YAML. Lines starting with # are ignored; "---" starts a new
// document.
func Unflatten(text []byte) ([]byte, error) {
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	root := &yaml.Node{Kind: yaml.MappingNode}
	used := false
	flush := func() error {
		if !used {
			return nil
		}
		used = false
		r := root
		root = &yaml.Node{Kind: yaml.MappingNode}
		return enc.Encode(r)
	}
	for i, raw := range strings.Split(strings.ReplaceAll(string(text), "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
			continue
		case line == "---":
			if err := flush(); err != nil {
				return nil, err
			}
			continue
		}
		pathText, valueText, ok := splitFlatLine(line)
		if !ok {
			return nil, fmt.Errorf("line %d: expected \"path: value\"", i+1)
		}
		path, err := ParsePath(pathText)
		if err != nil {
			return nil, fmt.Errorf("line %d: %v", i+1, err)
		}
		var val yaml.Node
		if err := yaml.Unmarshal([]byte(valueText), &val); err != nil {
			return nil, fmt.Errorf("line %d: invalid value: %v", i+1, err)
		}
		v := &yaml.Node{Kind: yaml.ScalarNode, Tag: TagNull, Value: "null"}
		if len(val.Content) > 0 {
			v = val.Content[0]
			v.Style = 0
		}
		if len(path) == 0 {
			return nil, fmt.Errorf("line %d: empty path", i+1)
		}
		root = setPath(root, path, v)
		used = true
	}
	if err := flush(); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// splitFlatLine splits "path: value" at the first ": " outside brackets
// and quotes (keys like ["a: b"] may contain the separator).
func splitFlatLine(line string) (string, string, bool) {
	depth := 0
	var quote byte
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quote != 0:
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '[':
			depth++
		case c == ']':
			depth--
		case c == ':' && depth == 0 && (i+1 == len(line) || line[i+1] == ' '):
			return strings.TrimSpace(line[:i]), strings.TrimSpace(line[i+1:]), true
		}
	}
	return "", "", false
}

// setPath stores v at path under n, creating maps and lists as needed,
// and returns n (replaced when its kind has to change).
func setPath(n *yaml.Node, path Path, v *yaml.Node) *yaml.Node {
	if len(path) == 0 {
		return v
	}
	s := path[0]
	if s.IsIndex {
		if n == nil || n.Kind != yaml.SequenceNode {
			n = &yaml.Node{Kind: yaml.SequenceNode}
		}
		for len(n.Content) <= s.Index {
			n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: TagNull, Value: "null"})
		}
		n.Content[s.Index] = setPath(n.Content[s.Index], path[1:], v)
		return n
	}
	if n == nil || n.Kind != yaml.MappingNode {
		n = &yaml.Node{Kind: yaml.MappingNode}
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == s.Key {
			n.Content[i+1] = setPath(n.Content[i+1], path[1:], v)
			return n
		}
	}
	child := setPath(nil, path[1:], v)
	n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: s.Key}, child)
	return n
}
