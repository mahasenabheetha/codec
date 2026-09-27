package yamlkit

import (
	"strings"
)

// Layered merges: tools build one effective configuration from pieces
// written in different places (a GitLab job and the templates it
// extends, a Compose file and its override). Every map entry in a
// merged tree gets its own copy of its key node, so a map from key node
// to origin says, per tree, where each entry was written; entries below
// an unrecorded key inherit their parent's.

// Origin is where a merged entry was written.
type Origin struct {
	File string
	Via  string // how it got there: "" (written here), "extends .base", "compose.override.yaml", …
}

// Origins records origins by node: copied key nodes, copied list items,
// and copies of values substituted from elsewhere.
type Origins map[*Node]Origin

// MergeRules adapts Merge to a tool. The zero value merges maps key by
// key and lets anything else (scalars, lists) replace.
type MergeRules struct {
	// List, when set, is asked for the lists found at path (keys from
	// the merge root): a nil result replaces base's list with over's;
	// otherwise items are keyed with the returned function — an over
	// item replaces the base item with the same key, the rest are
	// appended ("" never matches, so it always appends).
	List func(path []string) func(item *Node) string
}

// Own copies a map tree, tagging each key as written in file. Nested
// maps are copied too; scalars and lists are shared (they are never
// changed, only replaced).
func (t Origins) Own(n *Node, file string) *Node {
	return t.Copy(n, Origin{File: file}, true)
}

// Copy copies n's maps with fresh key nodes. Keys already tagged keep
// their origin, relabelled with at.Via when their own Via is "" (a
// parent's own entries become "extends parent"); untagged keys get at
// when force is set and otherwise inherit.
func (t Origins) Copy(n *Node, at Origin, force bool) *Node {
	if !IsPlainMap(n) {
		return n
	}
	out := &Node{Kind: KindMap, Range: n.Range, Flow: n.Flow, Pairs: make([]*Pair, 0, len(n.Pairs))}
	for _, p := range n.Pairs {
		k := *p.Key
		o, ok := t[p.Key]
		switch {
		case ok && o.Via == "":
			o.Via = at.Via
			t[&k] = o
		case ok:
			t[&k] = o
		case force:
			t[&k] = at
		}
		out.Pairs = append(out.Pairs, &Pair{Key: &k, Value: t.Copy(p.Value, at, force)})
	}
	return out
}

// CopyPair copies one entry the way Copy copies a map's entries.
func (t Origins) CopyPair(p *Pair, at Origin) *Pair {
	one := &Node{Kind: KindMap, Pairs: []*Pair{p}}
	return t.Copy(one, at, false).Pairs[0]
}

// Merge returns base with over applied: maps merge key by key,
// recursively; lists follow rules; anything else in over replaces
// base's value. over's own entries are relabelled with via. Neither
// input changes. rules may be nil.
func (t Origins) Merge(base, over *Node, via string, rules *MergeRules) *Node {
	return t.merge(base, over, via, rules, nil)
}

func (t Origins) merge(base, over *Node, via string, rules *MergeRules, path []string) *Node {
	at := Origin{Via: via}
	if !IsPlainMap(base) {
		return t.Copy(over, at, false)
	}
	if !IsPlainMap(over) {
		return t.Copy(base, Origin{}, false)
	}
	out := t.Copy(base, Origin{}, false)
	for _, p := range over.Pairs {
		name := KeyName(p.Key)
		i := IndexOf(out, name)
		if i < 0 {
			out.Pairs = append(out.Pairs, t.CopyPair(p, at))
			continue
		}
		cur := out.Pairs[i]
		sub := append(path[:len(path):len(path)], name)
		switch {
		case IsPlainMap(cur.Value) && IsPlainMap(p.Value):
			t.rekey(cur, p.Key, via)
			cur.Value = t.merge(cur.Value, p.Value, via, rules, sub)
			continue
		case isPlainSeq(cur.Value) && isPlainSeq(p.Value) && rules != nil && rules.List != nil:
			if key := rules.List(sub); key != nil {
				was := t[cur.Key]
				t.rekey(cur, p.Key, via)
				cur.Value = t.mergeList(cur.Value, p.Value, key, was, t.relabel(p.Key, via))
				continue
			}
		}
		out.Pairs[i] = t.CopyPair(p, at)
	}
	return out
}

// rekey makes a merged entry show where the overriding side wrote it:
// its key becomes a copy of over's key, position included.
func (t Origins) rekey(cur *Pair, over *Node, via string) {
	if _, ok := t[over]; !ok {
		return
	}
	k := *over
	t[&k] = t.relabel(over, via)
	cur.Key = &k
}

// relabel is n's origin with an empty Via replaced by via.
func (t Origins) relabel(n *Node, via string) Origin {
	o := t[n]
	if o.Via == "" {
		o.Via = via
	}
	return o
}

// mergeList appends over's items to base's, replacing items with the
// same key in place. Each item remembers the side it came from.
func (t Origins) mergeList(base, over *Node, key func(*Node) string, from, by Origin) *Node {
	out := &Node{Kind: KindSeq, Range: base.Range, Flow: base.Flow && over.Flow}
	for _, it := range base.Items {
		out.Items = append(out.Items, t.item(it, from))
	}
	for _, it := range over.Items {
		c := t.item(it, by)
		if k := key(it); k != "" {
			if j := indexFunc(out.Items, func(x *Node) bool { return key(x) == k }); j >= 0 {
				out.Items[j] = c
				continue
			}
		}
		out.Items = append(out.Items, c)
	}
	return out
}

// item copies a list item so it can carry its own origin (the same
// source node can reach a tree through different sides).
func (t Origins) item(it *Node, at Origin) *Node {
	if it == nil {
		return nil
	}
	if o, ok := t[it]; ok {
		if o.Via == "" {
			o.Via = at.Via
		}
		at = o
	}
	var c *Node
	if IsPlainMap(it) {
		c = t.Copy(it, at, false)
	} else {
		cp := *it
		c = &cp
	}
	t[c] = at
	return c
}

func indexFunc(items []*Node, f func(*Node) bool) int {
	for i, it := range items {
		if f(it) {
			return i
		}
	}
	return -1
}

// Set puts key: v into map m (replacing an existing entry), tagging the
// new key with o.
func (t Origins) Set(m *Node, key string, v *Node, o Origin) {
	k := &Node{Kind: KindScalar, Tag: TagStr, Value: key, Style: StylePlain}
	t[k] = o
	if i := IndexOf(m, key); i >= 0 {
		m.Pairs[i] = &Pair{Key: k, Value: v}
		return
	}
	m.Pairs = append(m.Pairs, &Pair{Key: k, Value: v})
}

// IsCustomTag reports whether n carries a tool's own tag (!reference,
// !reset), which makes it an opaque value rather than a mergeable map.
func IsCustomTag(n *Node) bool {
	return n != nil && strings.HasPrefix(n.Tag, "!") && !strings.HasPrefix(n.Tag, "!!")
}

// IsPlainMap reports whether n is a map without a custom tag.
func IsPlainMap(n *Node) bool { return n != nil && n.Kind == KindMap && !IsCustomTag(n) }

func isPlainSeq(n *Node) bool { return n != nil && n.Kind == KindSeq && !IsCustomTag(n) }

// KeyName is a key node's text ("" for nil).
func KeyName(k *Node) string {
	if k == nil {
		return ""
	}
	return k.Value
}

// IndexOf is the position of key in map m, or -1.
func IndexOf(m *Node, key string) int {
	for i, p := range m.Pairs {
		if KeyName(p.Key) == key {
			return i
		}
	}
	return -1
}

// --- emitting ---

// OriginLine is one line of an emitted merged tree.
type OriginLine struct {
	Text   string
	Origin Origin
	At     *Node // the node the line shows (its position is in Origin.File); nil for continuation lines
}

// Emit writes a merged tree as YAML lines, each with where it was
// written. base is the origin of untagged top-level entries.
func (t Origins) Emit(n *Node, base Origin) []OriginLine {
	e := &emitter{t: t}
	if n == nil {
		return nil
	}
	if n.Kind != KindMap {
		e.entry(0, 1, "-", nil, n, base)
		return e.out
	}
	for _, p := range n.Pairs {
		e.entry(0, 1, InlineText(p.Key)+":", p.Key, p.Value, base)
	}
	return e.out
}

type emitter struct {
	t   Origins
	out []OriginLine
}

func (e *emitter) line(indent int, s string, o Origin, at *Node) {
	if at != nil && at.Range.Start.Line == 0 {
		at = nil
	}
	e.out = append(e.out, OriginLine{Text: strings.Repeat("  ", indent) + s, Origin: o, At: at})
}

func (e *emitter) origin(n *Node, o Origin) Origin {
	if x, ok := e.t[n]; ok {
		return x
	}
	return o
}

// entry writes "head value": head is "key:" or "-" (or "- key:" for
// the first entry of a map in a list). Children go at child indent.
func (e *emitter) entry(indent, child int, head string, key, v *Node, o Origin) {
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
	case v.Kind != KindScalar && IsCustomTag(v):
		e.line(indent, head+" "+v.Tag+" "+InlineText(v), o, at)
	case v.Kind == KindScalar && strings.Contains(strings.TrimRight(v.Value, "\n"), "\n"):
		ind := "|"
		if !strings.HasSuffix(v.Value, "\n") {
			ind = "|-"
		}
		e.line(indent, head+" "+ind, o, at)
		for _, l := range strings.Split(strings.TrimSuffix(v.Value, "\n"), "\n") {
			e.line(child, l, o, nil)
		}
	case v.Kind == KindScalar || v.Kind == KindAlias:
		e.line(indent, head+" "+InlineText(v), o, at)
	case v.Kind == KindMap && len(v.Pairs) == 0:
		e.line(indent, head+" {}", o, at)
	case v.Kind == KindSeq && len(v.Items) == 0:
		e.line(indent, head+" []", o, at)
	case v.Kind == KindMap && head == "-":
		for i, p := range v.Pairs {
			if i == 0 {
				e.entry(indent, child+1, "- "+InlineText(p.Key)+":", p.Key, p.Value, o)
				continue
			}
			e.entry(child, child+1, InlineText(p.Key)+":", p.Key, p.Value, o)
		}
	case v.Kind == KindMap:
		e.line(indent, head, o, at)
		for _, p := range v.Pairs {
			e.entry(child, child+1, InlineText(p.Key)+":", p.Key, p.Value, o)
		}
	default: // a list
		e.line(indent, head, o, at)
		for _, it := range v.Items {
			e.entry(child, child+1, "-", nil, it, o)
		}
	}
}

// InlineText shows a node on one line as YAML would: scalars quoted
// where needed, lists as [a, b], maps as {k: v}.
func InlineText(n *Node) string {
	switch {
	case n == nil:
		return "null"
	case n.Kind == KindAlias:
		return "*" + n.Value
	case n.Kind == KindSeq:
		parts := make([]string, len(n.Items))
		for i, it := range n.Items {
			parts[i] = InlineText(it)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case n.Kind == KindMap:
		parts := make([]string, len(n.Pairs))
		for i, p := range n.Pairs {
			parts[i] = InlineText(p.Key) + ": " + InlineText(p.Value)
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case n.Tag == TagNull && n.Value == "":
		return "null"
	}
	return ValueText(n)
}
