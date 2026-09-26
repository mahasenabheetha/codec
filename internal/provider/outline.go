package provider

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Outline builds a generic outline: every key and list item, with a
// short preview of scalar values. Lens providers build richer outlines
// ("Container nginx · nginx:1.27") in later phases.
func Outline(d *yamlkit.Document) []Symbol {
	if d == nil || d.Root == nil {
		return nil
	}
	return children(d.Root)
}

func children(n *yamlkit.Node) []Symbol {
	var out []Symbol
	switch n.Kind {
	case yamlkit.KindMap:
		for _, p := range n.Pairs {
			s := symbolFor(p.Value)
			s.Name = keyName(p.Key)
			s.Range = yamlkit.Range{Start: p.Key.Range.Start, End: p.Value.Range.End}
			if s.Range.End.Before(p.Key.Range.End) {
				s.Range.End = p.Key.Range.End
			}
			out = append(out, s)
		}
	case yamlkit.KindSeq:
		for i, it := range n.Items {
			s := symbolFor(it)
			s.Name = fmt.Sprintf("[%d]", i)
			if label := itemLabel(it); label != "" {
				s.Name += " " + label
			}
			s.Range = it.Range
			out = append(out, s)
		}
	}
	return out
}

func symbolFor(v *yamlkit.Node) Symbol {
	s := Symbol{Kind: string(v.Kind)}
	switch v.Kind {
	case yamlkit.KindMap:
		s.Detail = fmt.Sprintf("{%d}", len(v.Pairs))
		s.Children = children(v)
	case yamlkit.KindSeq:
		s.Detail = fmt.Sprintf("[%d]", len(v.Items))
		s.Children = children(v)
	case yamlkit.KindAlias:
		s.Detail = "*" + v.Value
	default:
		s.Detail = preview(v.Value)
	}
	return s
}

func keyName(k *yamlkit.Node) string {
	if k.Kind == yamlkit.KindScalar {
		return k.Value
	}
	return "?"
}

// labelKeys are keys whose value names a list item, checked in order.
var labelKeys = []string{"name", "id", "key", "stage", "job", "task", "title", "hosts"}

// itemLabel names a list item that is a map, e.g. "- name: api".
func itemLabel(it *yamlkit.Node) string {
	if it.Kind != yamlkit.KindMap {
		return ""
	}
	for _, k := range labelKeys {
		if v := it.Get(k).Str(); v != "" {
			return preview(v)
		}
	}
	return ""
}

// preview shortens a value to one line of at most 60 characters.
func preview(s string) string {
	s = strings.ReplaceAll(strings.TrimRight(s, "\n"), "\n", " ⏎ ")
	if utf8.RuneCountInString(s) > 60 {
		r := []rune(s)
		s = string(r[:59]) + "…"
	}
	return s
}
