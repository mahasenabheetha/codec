package jsonkit

import (
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// SortKeys returns a copy of v with every object's keys in order
// (byte order, as jq -S does); arrays keep theirs.
func SortKeys(v *Value) *Value {
	switch v.Kind {
	case Array:
		out := &Value{Kind: Array, Items: make([]*Value, len(v.Items))}
		for i, it := range v.Items {
			out.Items[i] = SortKeys(it)
		}
		return out
	case Object:
		out := &Value{Kind: Object, Members: make([]Member, len(v.Members))}
		for i, m := range v.Members {
			out.Members[i] = Member{Key: m.Key, Value: SortKeys(m.Value)}
		}
		sort.SliceStable(out.Members, func(i, j int) bool { return out.Members[i].Key < out.Members[j].Key })
		return out
	}
	c := *v
	return &c
}

// A path step: an object key, or an array index when Key is "" and
// Index >= 0.
type step struct {
	Key   string
	Index int
}

// PathString writes a path the way jq does: .spec.containers[0].image,
// with keys that aren't simple names in brackets: .labels["app.kubernetes.io/name"].
// The whole value is ".".
func pathString(path []step) string {
	if len(path) == 0 {
		return "."
	}
	var b strings.Builder
	for _, s := range path {
		switch {
		case s.Index >= 0:
			b.WriteString("[" + strconv.Itoa(s.Index) + "]")
		case simpleKey(s.Key):
			b.WriteString("." + s.Key)
		default:
			b.WriteString("[")
			quote(&b, s.Key)
			b.WriteString("]")
		}
	}
	return b.String()
}

func simpleKey(k string) bool {
	if k == "" {
		return false
	}
	for i, r := range k {
		if !(r == '_' || unicode.IsLetter(r) || (i > 0 && unicode.IsDigit(r))) {
			return false
		}
	}
	return true
}

// Leaf is one value of a flattened document: a scalar, or an empty
// array or object (so flattening loses nothing).
type Leaf struct {
	Path  string `json:"path"`
	Value string `json:"value"` // compact JSON
}

// Flatten lists every leaf of v with its path.
func Flatten(v *Value) []Leaf {
	var out []Leaf
	var walk func(v *Value, path []step)
	walk = func(v *Value, path []step) {
		switch {
		case v.Kind == Array && len(v.Items) > 0:
			for i, it := range v.Items {
				walk(it, append(path, step{Index: i}))
			}
		case v.Kind == Object && len(v.Members) > 0:
			for _, m := range v.Members {
				walk(m.Value, append(path, step{Key: m.Key, Index: -1}))
			}
		default:
			out = append(out, Leaf{Path: pathString(path), Value: Encode(v, "")})
		}
	}
	walk(v, nil)
	return out
}

// FlattenText writes the leaves as "path = value" lines.
func FlattenText(v *Value) string {
	var b strings.Builder
	for _, l := range Flatten(v) {
		b.WriteString(l.Path + " = " + l.Value + "\n")
	}
	return b.String()
}

// Unflatten rebuilds a value from "path = value" lines (as FlattenText
// writes them; blank lines and lines starting with # are skipped).
// Array indexes may come in any order; gaps are filled with null.
func Unflatten(text string) (*Value, error) {
	var root *Value
	for n, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		path, rest, err := parsePath(line)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", n+1, err)
		}
		rest = strings.TrimSpace(rest)
		if !strings.HasPrefix(rest, "=") {
			return nil, fmt.Errorf("line %d: expected = after the path", n+1)
		}
		val, _, err := Parse(strings.TrimSpace(rest[1:]))
		if err != nil {
			return nil, fmt.Errorf("line %d: the value: %w", n+1, err)
		}
		if root, err = set(root, path, val); err != nil {
			return nil, fmt.Errorf("line %d: %w", n+1, err)
		}
	}
	if root == nil {
		return nil, fmt.Errorf("no path = value lines")
	}
	fillGaps(root)
	return root, nil
}

// fillGaps puts null in array places no line set.
func fillGaps(v *Value) {
	for i, it := range v.Items {
		if it == nil {
			v.Items[i] = &Value{Kind: Null}
		} else {
			fillGaps(it)
		}
	}
	for _, m := range v.Members {
		fillGaps(m.Value)
	}
}

// maxIndex bounds an index in a flattened path, so a typo can't make an
// array of a billion nulls.
const maxIndex = 100_000

// parsePath reads a jq-style path at the start of s and returns the rest.
func parsePath(s string) ([]step, string, error) {
	if !strings.HasPrefix(s, ".") && !strings.HasPrefix(s, "[") {
		return nil, "", fmt.Errorf("a path starts with . or [")
	}
	var path []step
	i := 0
	if strings.HasPrefix(s, ".") && (len(s) == 1 || s[1] == ' ' || s[1] == '=') {
		return nil, s[1:], nil // the whole value
	}
	for i < len(s) {
		switch s[i] {
		case '.':
			j := i + 1
			for j < len(s) && s[j] != '.' && s[j] != '[' && s[j] != ' ' && s[j] != '=' {
				j++
			}
			if j == i+1 {
				return nil, "", fmt.Errorf("an empty key after a dot")
			}
			path = append(path, step{Key: s[i+1 : j], Index: -1})
			i = j
		case '[':
			if i+1 < len(s) && s[i+1] == '"' {
				v := &parser{src: s, i: i + 1, dialect: JSON}
				key, err := v.string()
				if err != nil {
					return nil, "", fmt.Errorf("a quoted key: %w", err)
				}
				if v.i >= len(s) || s[v.i] != ']' {
					return nil, "", fmt.Errorf("expected ] after a quoted key")
				}
				path = append(path, step{Key: key, Index: -1})
				i = v.i + 1
				continue
			}
			end := strings.IndexByte(s[i:], ']')
			if end < 0 {
				return nil, "", fmt.Errorf("[ not closed with ]")
			}
			n, err := strconv.Atoi(s[i+1 : i+end])
			if err != nil || n < 0 || n > maxIndex {
				return nil, "", fmt.Errorf("not an index from 0 to %d: [%s]", maxIndex, s[i+1:i+end])
			}
			path = append(path, step{Index: n})
			i += end + 1
		default:
			return path, s[i:], nil
		}
	}
	return path, "", nil
}

// set puts val at path inside root, making objects and arrays as needed.
func set(root *Value, path []step, val *Value) (*Value, error) {
	if len(path) == 0 {
		if root != nil {
			return nil, fmt.Errorf("the whole value is set twice")
		}
		return val, nil
	}
	s := path[0]
	if s.Index >= 0 {
		if root == nil {
			root = &Value{Kind: Array, Items: []*Value{}}
		}
		if root.Kind != Array {
			return nil, fmt.Errorf("%s is %s, not an array", pathString(path[:1]), root.Kind)
		}
		for len(root.Items) <= s.Index {
			root.Items = append(root.Items, nil)
		}
		child, err := set(root.Items[s.Index], path[1:], val)
		if err != nil {
			return nil, err
		}
		root.Items[s.Index] = child // gaps stay nil until fillGaps
		return root, nil
	}
	if root == nil {
		root = &Value{Kind: Object, Members: []Member{}}
	}
	if root.Kind != Object {
		return nil, fmt.Errorf("the value at %q is %s, not an object", s.Key, root.Kind)
	}
	for i, m := range root.Members {
		if m.Key == s.Key {
			child, err := set(m.Value, path[1:], val)
			if err != nil {
				return nil, err
			}
			root.Members[i].Value = child
			return root, nil
		}
	}
	child, err := set(nil, path[1:], val)
	if err != nil {
		return nil, err
	}
	root.Members = append(root.Members, Member{Key: s.Key, Value: child})
	return root, nil
}

// Change is one difference between two values, at a path.
type Change struct {
	Path string `json:"path"`
	// Kind is "added", "removed", "changed" or "type" (the value's type
	// changed, e.g. a string became a number).
	Kind string `json:"kind"`
	Old  string `json:"old,omitempty"` // compact JSON
	New  string `json:"new,omitempty"`
}

// Diff lists what changed from a to b, by path: object members by key
// (in a's order, then b's new keys), array items by index. Numbers are
// compared by value, so 1.0 and 1 are the same.
func Diff(a, b *Value) []Change {
	out := []Change{}
	var walk func(a, b *Value, path []step)
	walk = func(a, b *Value, path []step) {
		p := pathString(path)
		switch {
		case a.Kind != b.Kind:
			out = append(out, Change{Path: p, Kind: "type", Old: Encode(a, ""), New: Encode(b, "")})
		case a.Kind == Object:
			for _, m := range a.Members {
				sub := append(append([]step{}, path...), step{Key: m.Key, Index: -1})
				if nb := b.Get(m.Key); nb != nil {
					walk(m.Value, nb, sub)
				} else {
					out = append(out, Change{Path: pathString(sub), Kind: "removed", Old: Encode(m.Value, "")})
				}
			}
			for _, m := range b.Members {
				if a.Get(m.Key) == nil {
					sub := append(append([]step{}, path...), step{Key: m.Key, Index: -1})
					out = append(out, Change{Path: pathString(sub), Kind: "added", New: Encode(m.Value, "")})
				}
			}
		case a.Kind == Array:
			for i := 0; i < len(a.Items) || i < len(b.Items); i++ {
				sub := append(append([]step{}, path...), step{Index: i})
				switch {
				case i >= len(b.Items):
					out = append(out, Change{Path: pathString(sub), Kind: "removed", Old: Encode(a.Items[i], "")})
				case i >= len(a.Items):
					out = append(out, Change{Path: pathString(sub), Kind: "added", New: Encode(b.Items[i], "")})
				default:
					walk(a.Items[i], b.Items[i], sub)
				}
			}
		case !sameScalar(a, b):
			out = append(out, Change{Path: p, Kind: "changed", Old: Encode(a, ""), New: Encode(b, "")})
		}
	}
	walk(a, b, nil)
	return out
}

func sameScalar(a, b *Value) bool {
	switch a.Kind {
	case Bool:
		return a.Bool == b.Bool
	case String:
		return a.Str == b.Str
	case Number:
		if a.Num == b.Num {
			return true
		}
		x, ok1 := new(big.Float).SetString(a.Num)
		y, ok2 := new(big.Float).SetString(b.Num)
		return ok1 && ok2 && x.Cmp(y) == 0
	}
	return true // null
}
