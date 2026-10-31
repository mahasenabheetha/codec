package jsonkit

import (
	"net/mail"
	"net/netip"
	"regexp"
	"strings"
	"time"
)

// Infer returns a JSON Schema (draft 2020-12) that every sample fits:
// the types seen at each place, object properties in the order first
// seen, a property required only when every object there has it, array
// items merged, and a string format (date-time, date, email, uri, uuid,
// ipv4, ipv6) when every string there has it.
func Infer(samples []*Value) *Value {
	n := &shape{}
	for _, s := range samples {
		n.add(s)
	}
	out := n.schema()
	out.Members = append([]Member{{Key: "$schema", Value: str("https://json-schema.org/draft/2020-12/schema")}}, out.Members...)
	return out
}

// shape is what has been seen at one place in the samples.
type shape struct {
	types    map[string]bool
	objects  int // objects seen here
	keys     []string
	props    map[string]*shape
	keyCount map[string]int
	items    *shape
	strings  int
	formats  map[string]int
}

func (s *shape) add(v *Value) {
	if s.types == nil {
		s.types = map[string]bool{}
	}
	switch v.Kind {
	case Number:
		if isInteger(v.Num) {
			s.types["integer"] = true
		} else {
			s.types["number"] = true
		}
	case Object:
		s.types["object"] = true
		s.objects++
		if s.props == nil {
			s.props, s.keyCount = map[string]*shape{}, map[string]int{}
		}
		for _, m := range v.Members {
			p, ok := s.props[m.Key]
			if !ok {
				p = &shape{}
				s.props[m.Key] = p
				s.keys = append(s.keys, m.Key)
			}
			s.keyCount[m.Key]++
			p.add(m.Value)
		}
	case Array:
		s.types["array"] = true
		if s.items == nil {
			s.items = &shape{}
		}
		for _, it := range v.Items {
			s.items.add(it)
		}
	case String:
		s.types["string"] = true
		s.strings++
		if f := format(v.Str); f != "" {
			if s.formats == nil {
				s.formats = map[string]int{}
			}
			s.formats[f]++
		}
	default:
		s.types[v.Kind.String()] = true
	}
}

func (s *shape) schema() *Value {
	out := &Value{Kind: Object, Members: []Member{}}
	add := func(k string, v *Value) { out.Members = append(out.Members, Member{Key: k, Value: v}) }
	types := s.typeList()
	switch len(types) {
	case 0: // nothing seen here (an empty array's items): anything goes
	case 1:
		add("type", str(types[0]))
	default:
		arr := &Value{Kind: Array}
		for _, t := range types {
			arr.Items = append(arr.Items, str(t))
		}
		add("type", arr)
	}
	if s.strings > 0 {
		for f, n := range s.formats {
			if n == s.strings {
				add("format", str(f))
			}
		}
	}
	if s.types["object"] {
		props := &Value{Kind: Object, Members: []Member{}}
		var required []*Value
		for _, k := range s.keys {
			props.Members = append(props.Members, Member{Key: k, Value: s.props[k].schema()})
			if s.keyCount[k] == s.objects {
				required = append(required, str(k))
			}
		}
		add("properties", props)
		if len(required) > 0 {
			add("required", &Value{Kind: Array, Items: required})
		}
	}
	if s.types["array"] && s.items != nil {
		add("items", s.items.schema())
	}
	return out
}

// typeList is the types seen, in a fixed order; integer folds into
// number when both were seen.
func (s *shape) typeList() []string {
	order := []string{"object", "array", "string", "integer", "number", "boolean", "null"}
	var out []string
	for _, t := range order {
		if !s.types[t] || (t == "integer" && s.types["number"]) {
			continue
		}
		out = append(out, t)
	}
	return out
}

var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var uriRE = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*://\S+$`)

// format names the JSON Schema format a string has, or "".
func format(s string) string {
	switch {
	case s == "":
		return ""
	case uuidRE.MatchString(s):
		return "uuid"
	case isTime(s):
		return "date-time"
	case len(s) == 10 && isDate(s):
		return "date"
	case uriRE.MatchString(s):
		return "uri"
	case strings.Count(s, "@") == 1 && !strings.ContainsAny(s, " <>") && isEmail(s):
		return "email"
	}
	if a, err := netip.ParseAddr(s); err == nil && a.String() == s {
		if a.Is4() {
			return "ipv4"
		}
		return "ipv6"
	}
	return ""
}

func isTime(s string) bool {
	_, err := time.Parse(time.RFC3339Nano, s)
	return err == nil
}

func isDate(s string) bool {
	_, err := time.Parse(time.DateOnly, s)
	return err == nil
}

func isEmail(s string) bool {
	a, err := mail.ParseAddress(s)
	return err == nil && a.Address == s
}

// isInteger reports a JSON number with no fraction and no exponent.
func isInteger(num string) bool {
	return !strings.ContainsAny(num, ".eE")
}

func str(s string) *Value { return &Value{Kind: String, Str: s} }
