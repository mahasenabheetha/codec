package yamlkit

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Segment is one step of a path: a map key or a sequence index.
type Segment struct {
	Key     string `json:"key,omitempty"`
	Index   int    `json:"index,omitempty"`
	IsIndex bool   `json:"isIndex,omitempty"`
}

// Path addresses a node from a document's root, e.g. spec.containers[0].
type Path []Segment

// PathAt returns the path of the deepest node at pos and that node.
// Block collections are matched by line, so a cursor anywhere on a
// line — including its indentation or the "- " marker — belongs to
// the entry that starts there.
func (d *Document) PathAt(pos Pos) (Path, *Node) {
	if d == nil || d.Root == nil {
		return nil, nil
	}
	var path Path
	n := d.Root
	for {
		switch n.Kind {
		case KindMap:
			var hit *Pair
			for _, pr := range n.Pairs {
				if startsAtOrBefore(pr.Key.Range.Start, pos, n.Flow) {
					hit = pr
				}
			}
			if hit == nil || !withinNode(n, pos) {
				return path, n
			}
			path = append(path, Segment{Key: keyText(hit.Key)})
			// On the key itself, the entry (not its value) is the target.
			if hit.Key.Range.Contains(pos) || hit.Value == nil || hit.Value.Kind == KindScalar || hit.Value.Kind == KindAlias {
				return path, hit.Value
			}
			n = hit.Value
		case KindSeq:
			hit := -1
			for i, it := range n.Items {
				if startsAtOrBefore(it.Range.Start, pos, n.Flow) {
					hit = i
				}
			}
			if hit < 0 || !withinNode(n, pos) {
				return path, n
			}
			path = append(path, Segment{Index: hit, IsIndex: true})
			it := n.Items[hit]
			if it.Kind != KindMap && it.Kind != KindSeq {
				return path, it
			}
			n = it
		default:
			return path, n
		}
	}
}

// startsAtOrBefore compares positions; block entries match on the line
// alone so indentation and "- " count as part of the entry.
func startsAtOrBefore(start, pos Pos, flow bool) bool {
	if flow {
		return !pos.Before(start)
	}
	return start.Line <= pos.Line
}

// withinNode reports whether pos is inside a collection, treating any
// line up to its last line as inside (blank lines between entries).
func withinNode(n *Node, pos Pos) bool {
	if n.Flow {
		return n.Range.Contains(pos)
	}
	return pos.Line <= n.Range.End.Line
}

func keyText(k *Node) string {
	if k == nil {
		return ""
	}
	if k.Kind == KindScalar {
		return k.Value
	}
	return "?"
}

// NodeAt returns the node at path, or nil if it doesn't exist.
func (d *Document) NodeAt(path Path) *Node {
	if d == nil {
		return nil
	}
	n := d.Root
	for _, s := range path {
		switch {
		case n == nil:
			return nil
		case s.IsIndex:
			if n.Kind != KindSeq || s.Index < 0 || s.Index >= len(n.Items) {
				return nil
			}
			n = n.Items[s.Index]
		default:
			n = n.Get(s.Key)
		}
	}
	return n
}

// PathStyle selects a path notation.
type PathStyle string

const (
	PathDot      PathStyle = "dot"      // spec.containers[0].image
	PathYQ       PathStyle = "yq"       // .spec.containers[0].image
	PathJSONPath PathStyle = "jsonpath" // $.spec.containers[0].image (also kubectl -o jsonpath)
	PathHelm     PathStyle = "helm"     // .Values.image.tag, for values files
	PathSet      PathStyle = "set"      // image.tag= for helm --set
)

// PathStyles lists every notation, in display order.
var PathStyles = []PathStyle{PathDot, PathYQ, PathJSONPath, PathHelm, PathSet}

var (
	identRe    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	dotSafeRe  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)
	pathTokRe  = regexp.MustCompile(`\.?([^.\[\]]+)|\[(\d+)\]|\["((?:[^"\\]|\\.)*)"\]|\['((?:[^'\\]|\\.)*)'\]`)
	helmIdxRep = strings.NewReplacer(`\`, `\\`, `"`, `\"`)
)

// Format renders the path in the given notation.
func (p Path) Format(style PathStyle) string {
	switch style {
	case PathYQ:
		return "." + strings.TrimPrefix(p.dotted(identRe, '"'), ".")
	case PathJSONPath:
		return "$" + p.dotted(identRe, '\'')
	case PathHelm:
		return p.helm()
	case PathSet:
		return p.set() + "="
	default:
		return strings.TrimPrefix(p.dotted(dotSafeRe, '"'), ".")
	}
}

// String renders the path in dot notation.
func (p Path) String() string { return p.Format(PathDot) }

// dotted joins segments as .key and [i], bracket-quoting keys that don't
// match safe with the given quote character.
func (p Path) dotted(safe *regexp.Regexp, q byte) string {
	var b strings.Builder
	for _, s := range p {
		switch {
		case s.IsIndex:
			fmt.Fprintf(&b, "[%d]", s.Index)
		case safe.MatchString(s.Key):
			b.WriteString("." + s.Key)
		default:
			quote := string(q)
			fmt.Fprintf(&b, "[%s%s%s]", quote, strings.ReplaceAll(s.Key, quote, "\\"+quote), quote)
		}
	}
	return b.String()
}

// helm builds a Go-template expression: .Values.a.b, falling back to
// index for keys that aren't identifiers and for list positions.
func (p Path) helm() string {
	expr := ".Values"
	for _, s := range p {
		switch {
		case s.IsIndex:
			expr = fmt.Sprintf("(index %s %d)", expr, s.Index)
		case identRe.MatchString(s.Key):
			expr += "." + s.Key
		default:
			expr = fmt.Sprintf(`(index %s "%s")`, expr, helmIdxRep.Replace(s.Key))
		}
	}
	return expr
}

// set builds helm's --set syntax: dots in keys are escaped, indexes
// use brackets.
func (p Path) set() string {
	var b strings.Builder
	for i, s := range p {
		if s.IsIndex {
			fmt.Fprintf(&b, "[%d]", s.Index)
			continue
		}
		if i > 0 {
			b.WriteByte('.')
		}
		b.WriteString(strings.NewReplacer(".", `\.`, ",", `\,`).Replace(s.Key))
	}
	return b.String()
}

// ParsePath reads a dot/yq/JSONPath-style path such as
// spec.containers[0].image, .a["b.c"] or $.a['b'].
func ParsePath(s string) (Path, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "$")
	var p Path
	rest := s
	for rest != "" {
		m := pathTokRe.FindStringSubmatchIndex(rest)
		if m == nil || m[0] != 0 {
			return nil, fmt.Errorf("invalid path near %q", rest)
		}
		switch {
		case m[2] >= 0:
			p = append(p, Segment{Key: rest[m[2]:m[3]]})
		case m[4] >= 0:
			i, _ := strconv.Atoi(rest[m[4]:m[5]])
			p = append(p, Segment{Index: i, IsIndex: true})
		case m[6] >= 0:
			p = append(p, Segment{Key: strings.ReplaceAll(rest[m[6]:m[7]], `\"`, `"`)})
		default:
			p = append(p, Segment{Key: strings.ReplaceAll(rest[m[8]:m[9]], `\'`, `'`)})
		}
		rest = rest[m[1]:]
	}
	return p, nil
}
