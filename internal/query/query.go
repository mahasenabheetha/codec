// Package query runs jq expressions (via gojq) over YAML documents and
// ties every result back to where it is in the file, so a UI can jump
// to it.
//
// The trick: an expression is first run as path(expr), which yields
// the location of each result instead of its value. Path expressions
// (.spec.containers[].image, .. | select(.kind == "Service")) thus get
// exact positions. Expressions that compute new values (length, map,
// object construction) can't be paths; they are run as-is and their
// results point at their document.
//
// It is pure: parsed YAML in, results out.
package query

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/itchyny/gojq"
	yaml "go.yaml.in/yaml/v3"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Result is one value an expression produced.
type Result struct {
	Doc   int            `json:"doc"`             // 0-based document index
	Path  string         `json:"path,omitempty"`  // jq path, e.g. .spec.replicas; "" for computed values
	Value string         `json:"value"`           // the value as YAML
	Range *yamlkit.Range `json:"range,omitempty"` // where it is (the document, for computed values)
}

// Query is a compiled expression.
type Query struct {
	plain, paths *gojq.Code // paths is nil when the expression can't be a path
}

// ErrLimit is returned (with the results so far) when there are more
// results than asked for.
var ErrLimit = errors.New("too many results")

// Compile parses expr. The environment is hidden from it ($ENV, env):
// a query is for the files, not for the machine's secrets.
func Compile(expr string) (*Query, error) {
	if strings.TrimSpace(expr) == "" {
		expr = "."
	}
	noEnv := gojq.WithEnvironLoader(func() []string { return nil })
	parsed, err := gojq.Parse(expr)
	if err != nil {
		return nil, parseError(expr, err)
	}
	q := &Query{}
	if q.plain, err = gojq.Compile(parsed, noEnv); err != nil {
		return nil, err
	}
	if pq, err := gojq.Parse("path(" + expr + ")"); err == nil {
		q.paths, _ = gojq.Compile(pq, noEnv)
	}
	return q, nil
}

// parseError says where the expression is broken.
func parseError(expr string, err error) error {
	var pe *gojq.ParseError
	if errors.As(err, &pe) {
		return fmt.Errorf("invalid expression at character %d (%q): %s", pe.Offset+1, pe.Token, pe.Error())
	}
	return err
}

// Run evaluates q on every document of f, stopping after limit results
// (ErrLimit) or when ctx ends. A document the expression fails on (say,
// iterating containers of a Service) is skipped; the error is returned
// only when no document produced anything.
func (q *Query) Run(ctx context.Context, f *yamlkit.File, limit int) ([]Result, error) {
	var out []Result
	var firstErr error
	for i, d := range f.Docs {
		if d.Root == nil {
			continue
		}
		resolved, err := yamlkit.Resolve(d.Root)
		if err != nil {
			return out, err
		}
		v := Value(resolved)
		rs, err := q.runDoc(ctx, i, d, v, limit-len(out))
		out = append(out, rs...)
		switch {
		case errors.Is(err, ErrLimit), ctx.Err() != nil:
			return out, cmp.Or(ctx.Err(), err)
		case err != nil && firstErr == nil:
			firstErr = err
		}
	}
	if len(out) == 0 {
		return out, firstErr
	}
	return out, nil
}

func (q *Query) runDoc(ctx context.Context, idx int, d *yamlkit.Document, v any, limit int) ([]Result, error) {
	if q.paths != nil {
		if rs, err := q.byPath(ctx, idx, d, v, limit); err == nil || errors.Is(err, ErrLimit) {
			return rs, err
		}
		// Not a path expression: fall through and compute values.
	}
	var out []Result
	iter := q.plain.RunWithContext(ctx, v)
	for {
		x, ok := iter.Next()
		if !ok {
			return out, nil
		}
		if err, isErr := x.(error); isErr {
			return out, jqError(err)
		}
		if len(out) >= limit {
			return out, ErrLimit
		}
		r := d.Root.Range // the document's first line, not its ---
		out = append(out, Result{Doc: idx, Value: render(x), Range: &r})
	}
}

// byPath runs path(expr) and looks every path up in the document.
func (q *Query) byPath(ctx context.Context, idx int, d *yamlkit.Document, v any, limit int) ([]Result, error) {
	var out []Result
	iter := q.paths.RunWithContext(ctx, v)
	for {
		x, ok := iter.Next()
		if !ok {
			return out, nil
		}
		if err, isErr := x.(error); isErr {
			return nil, err
		}
		path, ok := x.([]any)
		if !ok {
			return nil, errors.New("not a path")
		}
		if len(out) >= limit {
			return out, ErrLimit
		}
		res := Result{Doc: idx, Path: pathText(path)}
		node, exact := locate(d.Root, path)
		if node != nil {
			r := node.Range
			res.Range = &r
		}
		if exact {
			res.Value = yamlkit.ValueText(node)
		} else {
			res.Value = render(getPath(v, path)) // e.g. a key missing here, or behind a merge
		}
		out = append(out, res)
	}
}

// locate follows a jq path through the original document. exact is
// false when the path leaves what is written (it then returns the
// deepest node reached).
func locate(n *yamlkit.Node, path []any) (*yamlkit.Node, bool) {
	for _, step := range path {
		var next *yamlkit.Node
		switch s := step.(type) {
		case string:
			next = n.Get(s)
		case int:
			if n.Kind == yamlkit.KindSeq {
				if s < 0 {
					s += len(n.Items)
				}
				if s >= 0 && s < len(n.Items) {
					next = n.Items[s]
				}
			}
		}
		if next == nil {
			return n, false
		}
		n = next
	}
	return n, true
}

func getPath(v any, path []any) any {
	for _, step := range path {
		switch s := step.(type) {
		case string:
			m, _ := v.(map[string]any)
			v = m[s]
		case int:
			a, _ := v.([]any)
			if s < 0 {
				s += len(a)
			}
			if s < 0 || s >= len(a) {
				return nil
			}
			v = a[s]
		default:
			return nil
		}
	}
	return v
}

// pathText shows a path the way jq users write it: .a.b[0]["x.y"].
func pathText(path []any) string {
	var b strings.Builder
	for _, step := range path {
		switch s := step.(type) {
		case string:
			if isIdent(s) {
				b.WriteString("." + s)
			} else {
				b.WriteString("[" + strconv.Quote(s) + "]")
			}
		case int:
			b.WriteString("[" + strconv.Itoa(s) + "]")
		}
	}
	if b.Len() == 0 {
		return "."
	}
	return b.String()
}

func isIdent(s string) bool {
	for i, r := range s {
		if !(r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || i > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return s != ""
}

// jqError trims gojq's wording to what a user needs.
func jqError(err error) error {
	return fmt.Errorf("jq: %s", err.Error())
}

// Value converts a node to the types gojq works with.
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
	}
	switch n.Tag {
	case yamlkit.TagNull:
		return nil
	case yamlkit.TagBool:
		return strings.EqualFold(n.Value, "true")
	case yamlkit.TagInt:
		if i, err := strconv.ParseInt(strings.ReplaceAll(n.Value, "_", ""), 0, 64); err == nil && i >= math.MinInt && i <= math.MaxInt {
			return int(i)
		}
	case yamlkit.TagFloat:
		if f, err := strconv.ParseFloat(n.Value, 64); err == nil {
			return f
		}
	}
	return n.Value
}

// render shows a computed value as YAML (scalars bare, like yq).
func render(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return x
	}
	out, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return strings.TrimRight(string(out), "\n")
}
