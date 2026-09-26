package ci

import (
	"fmt"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// expandMatrix lists the jobs a GitHub strategy.matrix runs, following
// GitHub's rules:
//
//   - the base combinations are the product of the variables, the first
//     varying slowest;
//   - exclude removes every combination that matches all of an entry's
//     values (an entry may name only some variables);
//   - each include entry is added to every combination whose original
//     variables it doesn't contradict (values it adds may overwrite
//     values an earlier include added); an entry that fits no
//     combination becomes a job of its own.
//
// When a matrix (or a variable) is an expression, its values are only
// known at run time; Runtime says so.
func expandMatrix(m *yamlkit.Node, job string) (*Matrix, []string) {
	mx := &Matrix{Axes: []Axis{}, Combos: []Combo{}}
	var errs []string
	if m == nil {
		return nil, nil
	}
	if m.Kind == yamlkit.KindScalar {
		mx.Runtime = m.Value
		return mx, nil
	}
	if m.Kind != yamlkit.KindMap {
		return nil, nil
	}
	original := map[string]bool{}
	for _, pr := range m.Pairs {
		name := keyName(pr.Key)
		switch name {
		case "include":
			mx.Include = combos(pr.Value)
			if pr.Value.Kind == yamlkit.KindScalar {
				mx.Runtime = "include: " + pr.Value.Value
			}
			continue
		case "exclude":
			mx.Exclude = combos(pr.Value)
			continue
		}
		ax := Axis{Name: name, Values: []string{}}
		if pr.Value.Kind == yamlkit.KindSeq {
			for _, it := range pr.Value.Items {
				ax.Values = append(ax.Values, plain(it))
			}
		} else {
			ax.Values = append(ax.Values, text(pr.Value))
			if strings.Contains(pr.Value.Value, "${{") {
				mx.Runtime = name + ": " + pr.Value.Value
			}
		}
		mx.Axes = append(mx.Axes, ax)
		original[name] = true
	}
	for _, ex := range mx.Exclude {
		for _, kv := range ex.Values {
			if !original[kv.Key] {
				errs = append(errs, fmt.Sprintf("exclude names %q, which isn't a variable of the matrix", kv.Key))
			}
		}
	}

	var base []Combo
	if len(mx.Axes) > 0 {
		base = []Combo{{Values: []KV{}}}
		for _, ax := range mx.Axes {
			var next []Combo
			for _, c := range base {
				for _, v := range ax.Values {
					vals := append(append([]KV{}, c.Values...), KV{ax.Name, v})
					next = append(next, Combo{Values: vals})
				}
			}
			base = next
		}
	}
	kept := base[:0]
	for _, c := range base {
		if !excluded(c, mx.Exclude) {
			kept = append(kept, c)
		}
	}
	base = kept

	var added []Combo
	for _, inc := range mx.Include {
		matched := false
		for i := range base {
			if !fits(base[i], inc, original) {
				continue
			}
			matched = true
			for _, kv := range inc.Values {
				if !original[kv.Key] {
					base[i].Values = put(base[i].Values, kv)
				}
			}
			base[i].Origin = "include"
		}
		if !matched || len(mx.Axes) == 0 {
			c := Combo{Values: append([]KV{}, inc.Values...), Origin: "added"}
			added = append(added, c)
		}
	}
	mx.Combos = append(base, added...)
	for i := range mx.Combos {
		vals := make([]string, len(mx.Combos[i].Values))
		for k, kv := range mx.Combos[i].Values {
			vals[k] = kv.Value
		}
		mx.Combos[i].Name = job + " (" + strings.Join(vals, ", ") + ")"
	}
	if len(mx.Combos) > 256 {
		errs = append(errs, fmt.Sprintf("the matrix makes %d jobs; GitHub allows at most 256", len(mx.Combos)))
	}
	return mx, errs
}

// combos reads a list of maps as combinations.
func combos(n *yamlkit.Node) []Combo {
	var out []Combo
	for _, it := range items(n) {
		if !isMap(it) {
			continue
		}
		c := Combo{Values: []KV{}}
		for _, pr := range it.Pairs {
			c.Values = append(c.Values, KV{keyName(pr.Key), plain(pr.Value)})
		}
		out = append(out, c)
	}
	return out
}

func value(c Combo, key string) (string, bool) {
	for _, kv := range c.Values {
		if kv.Key == key {
			return kv.Value, true
		}
	}
	return "", false
}

// excluded reports whether some exclude entry matches all its values.
func excluded(c Combo, exclude []Combo) bool {
	for _, ex := range exclude {
		all := len(ex.Values) > 0
		for _, kv := range ex.Values {
			if v, ok := value(c, kv.Key); !ok || v != kv.Value {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

// fits reports whether include entry inc doesn't change any original
// variable of combination c.
func fits(c, inc Combo, original map[string]bool) bool {
	for _, kv := range inc.Values {
		if !original[kv.Key] {
			continue
		}
		if v, _ := value(c, kv.Key); v != kv.Value {
			return false
		}
	}
	return true
}

// put sets key to kv's value, keeping the position of an existing key.
func put(vals []KV, kv KV) []KV {
	for i := range vals {
		if vals[i].Key == kv.Key {
			vals[i].Value = kv.Value
			return vals
		}
	}
	return append(vals, kv)
}

// plain is a matrix value as GitHub shows it: scalars unquoted.
func plain(n *yamlkit.Node) string {
	if n != nil && n.Kind == yamlkit.KindScalar {
		return n.Value
	}
	return flow(n)
}
