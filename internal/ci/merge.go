package ci

import (
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Effective configurations are built from pieces written in different
// places (a job, the templates it extends, default:, an included file).
// Every map entry in a merged tree gets its own copy of its key node,
// so a map from key node to origin says, per tree, where each entry was
// written; entries below an unrecorded key inherit their parent's.

// origin is where a merged entry was written.
type origin struct {
	file string
	via  string // "" (the job itself), "extends .base", "default", "!reference .x", "template t.yml"
}

// tags records origins by node: copied key nodes, and copies of values
// substituted from elsewhere (a !reference, an inserted template).
type tags map[*yamlkit.Node]origin

// own copies a map tree, tagging each key as written in file. Nested
// maps are copied too; scalars and lists are shared (they are never
// changed, only replaced).
func (t tags) own(n *yamlkit.Node, file string) *yamlkit.Node {
	return t.copyTree(n, origin{file: file}, true)
}

// copyTree copies n's maps with fresh key nodes. Keys already tagged
// keep their origin, relabelled with at.via when their own via is ""
// (the parent's own entries become "extends parent"); untagged keys get
// at when force is set and otherwise inherit.
func (t tags) copyTree(n *yamlkit.Node, at origin, force bool) *yamlkit.Node {
	if n == nil || n.Kind != yamlkit.KindMap || isRef(n) {
		return n
	}
	out := &yamlkit.Node{Kind: yamlkit.KindMap, Range: n.Range, Flow: n.Flow, Pairs: make([]*yamlkit.Pair, 0, len(n.Pairs))}
	for _, p := range n.Pairs {
		k := *p.Key
		o, ok := t[p.Key]
		switch {
		case ok && o.via == "":
			o.via = at.via
			t[&k] = o
		case ok:
			t[&k] = o
		case force:
			t[&k] = at
		}
		out.Pairs = append(out.Pairs, &yamlkit.Pair{Key: &k, Value: t.copyTree(p.Value, at, force)})
	}
	return out
}

// merge returns base with over applied the way GitLab merges extends
// and includes: maps merge key by key, recursively; anything else in
// over (scalars, lists) replaces base's value. over's own entries are
// relabelled with via. Neither input changes.
func (t tags) merge(base, over *yamlkit.Node, via string) *yamlkit.Node {
	at := origin{via: via}
	if base == nil || base.Kind != yamlkit.KindMap {
		return t.copyTree(over, at, false)
	}
	if over == nil || over.Kind != yamlkit.KindMap {
		return t.copyTree(base, origin{}, false)
	}
	out := t.copyTree(base, origin{}, false)
	for _, p := range over.Pairs {
		name := keyName(p.Key)
		if i := indexOf(out, name); i >= 0 {
			cur := out.Pairs[i]
			if isMap(cur.Value) && isMap(p.Value) {
				// The entry now shows where the overriding side wrote it.
				if o, ok := t[p.Key]; ok {
					if o.via == "" {
						o.via = via
					}
					t[cur.Key] = o
				}
				cur.Value = t.merge(cur.Value, p.Value, via)
				continue
			}
			out.Pairs = append(out.Pairs[:i], out.Pairs[i+1:]...)
			out.Pairs = append(out.Pairs[:i], append([]*yamlkit.Pair{t.copyPair(p, at)}, out.Pairs[i:]...)...)
			continue
		}
		out.Pairs = append(out.Pairs, t.copyPair(p, at))
	}
	return out
}

func (t tags) copyPair(p *yamlkit.Pair, at origin) *yamlkit.Pair {
	one := &yamlkit.Node{Kind: yamlkit.KindMap, Pairs: []*yamlkit.Pair{p}}
	return t.copyTree(one, at, false).Pairs[0]
}

func isMap(n *yamlkit.Node) bool { return n != nil && n.Kind == yamlkit.KindMap && !isRef(n) }

// isRef reports whether n is an unresolved GitLab !reference.
func isRef(n *yamlkit.Node) bool { return n != nil && n.Tag == "!reference" }

func keyName(k *yamlkit.Node) string {
	if k == nil {
		return ""
	}
	return k.Value
}

func indexOf(m *yamlkit.Node, key string) int {
	for i, p := range m.Pairs {
		if keyName(p.Key) == key {
			return i
		}
	}
	return -1
}

// set puts key: v into map m (replacing an existing entry), tagging the
// new key with o.
func (t tags) set(m *yamlkit.Node, key string, v *yamlkit.Node, o origin) {
	k := &yamlkit.Node{Kind: yamlkit.KindScalar, Tag: yamlkit.TagStr, Value: key, Style: yamlkit.StylePlain}
	t[k] = o
	if i := indexOf(m, key); i >= 0 {
		m.Pairs[i] = &yamlkit.Pair{Key: k, Value: v}
		return
	}
	m.Pairs = append(m.Pairs, &yamlkit.Pair{Key: k, Value: v})
}

// --- emitting ---

// emit writes a merged map as YAML lines, each with where it was
// written. base is the origin of untagged top-level entries.
func (t tags) emit(n *yamlkit.Node, base origin) []Line {
	e := &emitter{t: t}
	if n == nil {
		return nil
	}
	if n.Kind != yamlkit.KindMap {
		e.entry(0, 1, "-", nil, n, base)
		return e.out
	}
	for _, p := range n.Pairs {
		e.entry(0, 1, scalar(p.Key)+":", p.Key, p.Value, base)
	}
	return e.out
}

type emitter struct {
	t   tags
	out []Line
}

func (e *emitter) line(indent int, s string, o origin, at *yamlkit.Node) {
	l := Line{Text: strings.Repeat("  ", indent) + s, From: o.via}
	if at != nil && at.Range.Start.Line > 0 {
		l.Source = &Source{File: o.file, Line: at.Range.Start.Line, Range: at.Range}
	}
	e.out = append(e.out, l)
}

func (e *emitter) origin(n *yamlkit.Node, o origin) origin {
	if x, ok := e.t[n]; ok {
		return x
	}
	return o
}

// entry writes "head value": head is "key:" or "-" (or "- key:" for
// the first entry of a map in a list). Children go at child indent.
func (e *emitter) entry(indent, child int, head string, key, v *yamlkit.Node, o origin) {
	if key != nil {
		o = e.origin(key, o)
	}
	o = e.origin(v, o)
	at := key
	if at == nil || at.Range.Start.Line == 0 {
		at = v
	}
	switch {
	case v == nil:
		e.line(indent, head+" null", o, at)
	case isRef(v) || (v.Kind != yamlkit.KindScalar && strings.HasPrefix(v.Tag, "!") && !strings.HasPrefix(v.Tag, "!!")):
		e.line(indent, head+" "+v.Tag+" "+flow(v), o, at)
	case v.Kind == yamlkit.KindScalar && strings.Contains(strings.TrimRight(v.Value, "\n"), "\n"):
		ind := "|"
		if !strings.HasSuffix(v.Value, "\n") {
			ind = "|-"
		}
		e.line(indent, head+" "+ind, o, at)
		for _, l := range strings.Split(strings.TrimSuffix(v.Value, "\n"), "\n") {
			e.line(child, l, o, nil)
		}
	case v.Kind == yamlkit.KindScalar || v.Kind == yamlkit.KindAlias:
		e.line(indent, head+" "+scalar(v), o, at)
	case v.Kind == yamlkit.KindMap && len(v.Pairs) == 0:
		e.line(indent, head+" {}", o, at)
	case v.Kind == yamlkit.KindSeq && len(v.Items) == 0:
		e.line(indent, head+" []", o, at)
	case v.Kind == yamlkit.KindMap && head == "-":
		for i, p := range v.Pairs {
			if i == 0 {
				e.entry(indent, child+1, "- "+scalar(p.Key)+":", p.Key, p.Value, o)
				continue
			}
			e.entry(child, child+1, scalar(p.Key)+":", p.Key, p.Value, o)
		}
	case v.Kind == yamlkit.KindMap:
		e.line(indent, head, o, at)
		for _, p := range v.Pairs {
			e.entry(child, child+1, scalar(p.Key)+":", p.Key, p.Value, o)
		}
	default: // a list
		e.line(indent, head, o, at)
		for _, it := range v.Items {
			e.entry(child, child+1, "-", nil, it, o)
		}
	}
}

// scalar shows a scalar as YAML would (quoted where needed).
func scalar(n *yamlkit.Node) string {
	switch {
	case n == nil:
		return "null"
	case n.Kind == yamlkit.KindAlias:
		return "*" + n.Value
	case n.Kind != yamlkit.KindScalar:
		return flow(n)
	case n.Tag == yamlkit.TagNull && n.Value == "":
		return "null"
	}
	return yamlkit.ValueText(n)
}

// flow shows a list or map on one line: [a, b] or {k: v}.
func flow(n *yamlkit.Node) string {
	switch {
	case n == nil:
		return "null"
	case n.Kind == yamlkit.KindSeq:
		parts := make([]string, len(n.Items))
		for i, it := range n.Items {
			parts[i] = flow(it)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case n.Kind == yamlkit.KindMap:
		parts := make([]string, len(n.Pairs))
		for i, p := range n.Pairs {
			parts[i] = scalar(p.Key) + ": " + flow(p.Value)
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return scalar(n)
}
