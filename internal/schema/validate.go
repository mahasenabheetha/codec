package schema

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

var printer = message.NewPrinter(language.English)

// Validate checks one document against sch and returns its violations,
// placed on the offending key or value. Values that contain runtime
// expressions (Argo, GitHub) are skipped: their type is only known
// when the workflow runs.
func Validate(sch *jsonschema.Schema, doc *yamlkit.Document, looseScalars bool) []yamlkit.Diagnostic {
	if sch == nil || doc == nil || doc.Root == nil {
		return nil
	}
	resolved, err := yamlkit.Resolve(doc.Root) // expand anchors and << merges
	if err != nil {
		return nil // the parser already reported the broken alias
	}
	err = sch.Validate(Value(resolved))
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return nil
	}
	m := &mapper{doc: doc, root: sch, loose: looseScalars}
	for _, leaf := range leaves(ve) {
		m.add(leaf)
	}
	return m.out
}

// Value converts a node to the JSON-like value a validator expects.
func Value(n *yamlkit.Node) any {
	if n == nil {
		return nil
	}
	switch n.Kind {
	case yamlkit.KindMap:
		m := make(map[string]any, len(n.Pairs))
		for _, p := range n.Pairs {
			if p.Key != nil {
				m[p.Key.Value] = Value(p.Value)
			}
		}
		return m
	case yamlkit.KindSeq:
		s := make([]any, len(n.Items))
		for i, it := range n.Items {
			s[i] = Value(it)
		}
		return s
	case yamlkit.KindScalar:
		switch n.Tag {
		case yamlkit.TagNull:
			return nil
		case yamlkit.TagBool:
			return strings.EqualFold(n.Value, "true")
		case yamlkit.TagInt:
			if i, err := strconv.ParseInt(strings.ReplaceAll(n.Value, "_", ""), 0, 64); err == nil {
				return json.Number(strconv.FormatInt(i, 10))
			}
		case yamlkit.TagFloat:
			if f, err := strconv.ParseFloat(n.Value, 64); err == nil {
				return json.Number(strconv.FormatFloat(f, 'g', -1, 64))
			}
		}
		return n.Value
	}
	return nil
}

// leaves flattens the error tree into the errors worth showing. For
// oneOf/anyOf it keeps only the branch that got furthest into the
// document (the one the author most likely meant); if every branch
// just wanted a different type at the same spot, they merge into one
// "expected a or b".
func leaves(e *jsonschema.ValidationError) []*jsonschema.ValidationError {
	if len(e.Causes) == 0 {
		return []*jsonschema.ValidationError{e}
	}
	switch e.ErrorKind.(type) {
	case *kind.OneOf, *kind.AnyOf:
		var best []*jsonschema.ValidationError
		bestDepth := -1
		var types []*jsonschema.ValidationError // single type-mismatch branches
		for _, c := range e.Causes {
			ls := leaves(c)
			d := depth(ls)
			if len(ls) == 1 {
				if _, ok := ls[0].ErrorKind.(*kind.Type); ok {
					types = append(types, ls[0])
				}
			}
			if d > bestDepth || (d == bestDepth && len(ls) < len(best)) {
				best, bestDepth = ls, d
			}
		}
		if len(types) == len(e.Causes) {
			return []*jsonschema.ValidationError{mergeTypes(types)}
		}
		return best
	}
	var out []*jsonschema.ValidationError
	for _, c := range e.Causes {
		out = append(out, leaves(c)...)
	}
	return out
}

func depth(ls []*jsonschema.ValidationError) int {
	d := 0
	for _, l := range ls {
		d = max(d, len(l.InstanceLocation))
	}
	return d
}

func mergeTypes(ts []*jsonschema.ValidationError) *jsonschema.ValidationError {
	merged := &kind.Type{Got: ts[0].ErrorKind.(*kind.Type).Got}
	for _, t := range ts {
		for _, w := range t.ErrorKind.(*kind.Type).Want {
			if !slices.Contains(merged.Want, w) {
				merged.Want = append(merged.Want, w)
			}
		}
	}
	return &jsonschema.ValidationError{InstanceLocation: ts[0].InstanceLocation, ErrorKind: merged}
}

// mapper turns validation errors into diagnostics on the document.
type mapper struct {
	doc   *yamlkit.Document
	root  *jsonschema.Schema
	loose bool // see Ref.LooseScalars
	out   []yamlkit.Diagnostic
	seen  map[string]bool
}

// locate walks the instance location through the original document.
// It returns the node (nil if the path goes through merged keys that
// aren't written there) and the pair holding it, if any.
func (m *mapper) locate(loc []string) (*yamlkit.Node, *yamlkit.Pair, []Step) {
	n := m.doc.Root
	var pair *yamlkit.Pair
	var steps []Step
	for _, tok := range loc {
		switch {
		case n == nil:
			return nil, nil, steps
		case n.Kind == yamlkit.KindSeq:
			i, err := strconv.Atoi(tok)
			if err != nil || i < 0 || i >= len(n.Items) {
				return nil, nil, steps
			}
			n, pair = n.Items[i], nil
			steps = append(steps, Step{Item: true})
		case n.Kind == yamlkit.KindMap:
			pair = n.Pair(tok)
			steps = append(steps, Step{Key: tok})
			if pair == nil {
				return nil, nil, steps
			}
			n = pair.Value
		default:
			return nil, nil, steps
		}
	}
	return n, pair, steps
}

func (m *mapper) add(e *jsonschema.ValidationError) {
	n, pair, steps := m.locate(e.InstanceLocation)
	if n == nil {
		n, pair = m.doc.Root, nil // somewhere under a merge: blame the document
	}
	if templated(n) {
		return
	}
	// Default spot: the key when there is one (a short squiggle), else the node.
	at := n.Range
	if pair != nil && pair.Key != nil && n.Kind != yamlkit.KindScalar {
		at = pair.Key.Range
	}
	where := pathText(steps)

	switch k := e.ErrorKind.(type) {
	case *kind.AdditionalProperties:
		allowed := m.properties(steps)
		for _, p := range k.Properties {
			r := at
			if pr := n.Pair(p); pr != nil && pr.Key != nil {
				r = pr.Key.Range
			}
			hint := "Remove it, or check its spelling and indentation."
			if s := closest(p, allowed); s != "" {
				hint = fmt.Sprintf("Did you mean %q?", s)
			} else if len(allowed) > 0 {
				hint = "Allowed here: " + list(allowed, 10) + "."
			}
			m.emit(r, fmt.Sprintf("Unknown field %q%s", p, in(where)), hint)
		}
	case *kind.Required:
		for _, p := range k.Missing {
			m.emit(at, fmt.Sprintf("Missing required field %q%s", p, in(where)), fmt.Sprintf("Add %s: to %s.", p, orRoot(where)))
		}
	case *kind.Type:
		if m.loose && scalarType(k.Got) && slices.ContainsFunc(k.Want, scalarType) {
			return
		}
		hint := ""
		// Kubernetes schemas allow null almost everywhere; saying so
		// in "expected null or integer" only adds noise.
		wants := slices.DeleteFunc(slices.Clone(k.Want), func(w string) bool { return w == "null" && k.Got != "null" })
		if len(wants) == 0 {
			wants = k.Want
		}
		want := strings.Join(wants, " or ")
		switch {
		case k.Got == "null":
			hint = "The value is empty: add it, or remove the key."
		case slices.Contains(k.Want, "string") && n.Kind == yamlkit.KindScalar:
			hint = fmt.Sprintf("Quote it (%q) to make it text.", n.Value)
		}
		m.emit(at, fmt.Sprintf("Expected %s, got %s%s", want, k.Got, in(where)), hint)
	case *kind.Enum:
		var want []string
		for _, w := range k.Want {
			want = append(want, fmt.Sprint(w))
		}
		m.emit(at, fmt.Sprintf("Must be one of %s%s", list(want, 12), in(where)), "")
	case *kind.FalseSchema:
		m.emit(at, fmt.Sprintf("%s is not allowed here", orRoot(where)), "Remove it.")
	default:
		m.emit(at, capitalize(e.ErrorKind.LocalizedString(printer))+in(where), "")
	}
}

func scalarType(t string) bool {
	switch t {
	case "string", "number", "integer", "boolean":
		return true
	}
	return false
}

func (m *mapper) emit(r yamlkit.Range, msg, hint string) {
	key := fmt.Sprintf("%d:%d:%s", r.Start.Line, r.Start.Col, msg)
	if m.seen == nil {
		m.seen = map[string]bool{}
	}
	if m.seen[key] {
		return
	}
	m.seen[key] = true
	m.out = append(m.out, yamlkit.Diagnostic{
		Severity: yamlkit.SeverityError,
		Code:     "schema",
		Source:   "schema",
		Message:  msg,
		Hint:     hint,
		Range:    r,
	})
}

// properties lists the field names the schema allows at a path.
func (m *mapper) properties(steps []Step) []string {
	var names []string
	for _, p := range Properties(m.root, steps) {
		names = append(names, p.Name)
	}
	return names
}

// templated reports whether a value holds an expression (it may stand
// for anything until it is evaluated).
func templated(n *yamlkit.Node) bool {
	return n != nil && n.Templated
}

func pathText(steps []Step) string {
	var b strings.Builder
	for _, s := range steps {
		if s.Item {
			b.WriteString("[]")
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('.')
		}
		b.WriteString(s.Key)
	}
	return b.String()
}

func in(where string) string {
	if where == "" {
		return ""
	}
	return " in " + where
}

func orRoot(where string) string {
	if where == "" {
		return "the document"
	}
	return where
}

func list(items []string, limit int) string {
	if len(items) > limit {
		return strings.Join(items[:limit], ", ") + fmt.Sprintf(", … (%d more)", len(items)-limit)
	}
	return strings.Join(items, ", ")
}

func capitalize(s string) string {
	for i, r := range s {
		return string(unicode.ToUpper(r)) + s[i+len(string(r)):]
	}
	return s
}

// closest suggests an allowed name for a misspelled one: same letters
// ignoring case, a longer name it starts ("timeout" → "timeout-minutes"),
// or at most two edits away.
func closest(name string, allowed []string) string {
	best, bestD := "", 3
	lower := strings.ToLower(name)
	for _, a := range allowed {
		if strings.EqualFold(a, name) {
			return a
		}
		if len(name) >= 4 && strings.HasPrefix(strings.ToLower(a), lower) {
			return a
		}
		if d := distance(lower, strings.ToLower(a)); d < bestD {
			best, bestD = a, d
		}
	}
	return best
}

// distance is the Levenshtein edit distance.
func distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}
