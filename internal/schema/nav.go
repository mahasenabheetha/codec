package schema

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Step is one step of a path into a document: a map key, or "any item"
// of a list (schemas describe items, not particular indexes).
type Step struct {
	Key  string
	Item bool
}

// at returns every subschema that applies at path, with $ref and
// allOf/anyOf/oneOf/if-then-else expanded (a value may match any
// branch, so all of them are candidates).
func at(root *jsonschema.Schema, path []Step) []*jsonschema.Schema {
	cur := expand([]*jsonschema.Schema{root})
	for _, s := range path {
		var next []*jsonschema.Schema
		for _, sch := range cur {
			if s.Item {
				next = append(next, itemSchemas(sch)...)
			} else {
				next = append(next, childSchemas(sch, s.Key)...)
			}
		}
		cur = expand(next)
		if len(cur) == 0 {
			break
		}
	}
	return cur
}

func expand(in []*jsonschema.Schema) []*jsonschema.Schema {
	var out []*jsonschema.Schema
	seen := map[*jsonschema.Schema]bool{}
	var visit func(s *jsonschema.Schema)
	visit = func(s *jsonschema.Schema) {
		if s == nil || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
		visit(s.Ref)
		for _, group := range [][]*jsonschema.Schema{s.AllOf, s.AnyOf, s.OneOf} {
			for _, g := range group {
				visit(g)
			}
		}
		visit(s.Then)
		visit(s.Else)
	}
	for _, s := range in {
		visit(s)
	}
	return out
}

func childSchemas(s *jsonschema.Schema, key string) []*jsonschema.Schema {
	if p, ok := s.Properties[key]; ok {
		return []*jsonschema.Schema{p}
	}
	var out []*jsonschema.Schema
	for re, p := range s.PatternProperties {
		if re.MatchString(key) {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		if ap, ok := s.AdditionalProperties.(*jsonschema.Schema); ok {
			out = append(out, ap)
		}
	}
	return out
}

func itemSchemas(s *jsonschema.Schema) []*jsonschema.Schema {
	var out []*jsonschema.Schema
	if s.Items2020 != nil {
		out = append(out, s.Items2020)
	}
	switch it := s.Items.(type) {
	case *jsonschema.Schema:
		out = append(out, it)
	case []*jsonschema.Schema:
		out = append(out, it...)
	}
	out = append(out, s.PrefixItems...)
	if ai, ok := s.AdditionalItems.(*jsonschema.Schema); ok {
		out = append(out, ai)
	}
	return out
}

// Property is one field a schema allows.
type Property struct {
	Name     string
	Required bool
	Info     Info
}

// Properties lists the fields allowed at path, sorted with required
// fields first.
func Properties(root *jsonschema.Schema, path []Step) []Property {
	if root == nil {
		return nil
	}
	schemas := at(root, path)
	required := map[string]bool{}
	props := map[string][]*jsonschema.Schema{}
	for _, s := range schemas {
		for _, r := range s.Required {
			required[r] = true
		}
		for name, p := range s.Properties {
			props[name] = append(props[name], p)
		}
	}
	out := make([]Property, 0, len(props))
	for name, ps := range props {
		out = append(out, Property{Name: name, Required: required[name], Info: describe(expand(ps))})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Required != out[j].Required {
			return out[i].Required
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Info is what a schema says about one value.
type Info struct {
	Description string   `json:"description,omitempty"`
	Types       []string `json:"types,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Default     string   `json:"default,omitempty"`
	Deprecated  bool     `json:"deprecated,omitempty"`
}

// InfoAt describes the value at path; ok is false when the schema says
// nothing about it.
func InfoAt(root *jsonschema.Schema, path []Step) (Info, bool) {
	if root == nil {
		return Info{}, false
	}
	schemas := at(root, path)
	if len(schemas) == 0 {
		return Info{}, false
	}
	i := describe(schemas)
	return i, i.Description != "" || len(i.Types) > 0 || len(i.Enum) > 0
}

// describe merges what candidate schemas say: the first description
// (the property's own comes first, before its $ref target's), all
// types and all enum values.
func describe(schemas []*jsonschema.Schema) Info {
	var i Info
	for _, s := range schemas {
		if i.Description == "" {
			i.Description = strings.TrimSpace(cmpOr(s.Description, s.Title))
		}
		if s.Types != nil {
			for _, t := range s.Types.ToStrings() {
				if t != "null" && !slices.Contains(i.Types, t) {
					i.Types = append(i.Types, t)
				}
			}
		}
		if s.Enum != nil {
			for _, v := range s.Enum.Values {
				if v == nil {
					continue
				}
				if t := fmt.Sprint(v); !slices.Contains(i.Enum, t) {
					i.Enum = append(i.Enum, t)
				}
			}
		}
		if s.Const != nil && *s.Const != nil {
			if t := fmt.Sprint(*s.Const); !slices.Contains(i.Enum, t) {
				i.Enum = append(i.Enum, t)
			}
		}
		if i.Default == "" && s.Default != nil && *s.Default != nil {
			i.Default = fmt.Sprint(*s.Default)
		}
		i.Deprecated = i.Deprecated || s.Deprecated
	}
	if len(i.Enum) == 0 {
		i.Enum = enumFromText(i.Description)
	}
	return i
}

var (
	rePossible = regexp.MustCompile("- `\"([^\"`]+)\"`")
	reOneOf    = regexp.MustCompile(`(?i)\bone of:?\s+([^.;:]+)`)
	reWord     = regexp.MustCompile(`^[A-Za-z][\w-]*$`)
)

// enumFromText finds allowed values that a schema only lists in prose,
// as Kubernetes' do ("One of Always, Never, IfNotPresent", or a
// "Possible enum values" list). They feed suggestions, never
// validation: prose is not a contract.
func enumFromText(desc string) []string {
	if ms := rePossible.FindAllStringSubmatch(desc, -1); len(ms) > 0 {
		var out []string
		for _, m := range ms {
			if !slices.Contains(out, m[1]) {
				out = append(out, m[1])
			}
		}
		return out
	}
	m := reOneOf.FindStringSubmatch(desc)
	if m == nil {
		return nil
	}
	var out []string
	for _, part := range regexp.MustCompile(`,\s*(?:or\s+)?|\s+or\s+`).Split(m[1], -1) {
		w := strings.Trim(strings.TrimSpace(part), `"'`+"`")
		if !reWord.MatchString(w) {
			return nil // not a plain list of words
		}
		out = append(out, w)
	}
	if len(out) < 2 || len(out) > 12 {
		return nil
	}
	return out
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
