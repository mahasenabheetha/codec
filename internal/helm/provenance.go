package helm

import (
	"bytes"
	"fmt"
	"slices"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	"helm.sh/helm/v4/pkg/strvals"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Origin is one layer's say on a value.
type Origin struct {
	Layer   string `json:"layer"` // "values-prod.yaml", "--set a=b", "values.yaml", "db chart"
	Kind    string `json:"kind"`  // set, file, default, global, computed
	Line    int    `json:"line,omitempty"`
	Value   any    `json:"value,omitempty"` // scalar, or a summary like "{3 keys}"
	Deleted bool   `json:"deleted,omitempty"`
}

// Provenance maps a path (yamlkit dot notation, e.g. "image.tag") to
// the layers that set it, winner first. Lists are atomic in Helm (they
// replace, never merge), so list items take their list's entry.
type Provenance map[string][]Origin

// source is one values layer as the provenance lookup sees it.
type source struct {
	name  string
	kind  string
	vals  map[string]any
	lines map[string]int // dot path -> line of its key
}

// provenance explains final by asking every layer, highest precedence
// first, whether it defines each path. The SDK computed final itself
// (decision #33), so this never disagrees with what Helm renders.
func provenance(ch *chart.Chart, files []File, opts Options, final map[string]any) Provenance {
	top := userSources(opts)
	if raw := fileData(files, "values.yaml"); raw != nil {
		top = append(top, valuesSource("values.yaml", "default", raw))
	}
	p := Provenance{}
	explain(p, nil, final, top, ch)
	deletions(p, top, final)
	return p
}

// userSources returns --set (last first) then -f files (last first).
func userSources(opts Options) []source {
	var out []source
	for _, s := range slices.Backward(opts.Set) {
		if m, err := strvals.Parse(s); err == nil {
			out = append(out, source{name: "--set " + s, kind: "set", vals: m})
		}
	}
	for _, l := range slices.Backward(opts.Values) {
		out = append(out, valuesSource(l.Name, "file", l.Data))
	}
	return out
}

func valuesSource(name, kind string, data []byte) source {
	vals, _ := loader.LoadValues(bytes.NewReader(data))
	return source{name: name, kind: kind, vals: vals, lines: keyLines(data)}
}

// keyLines maps every key's dot path in a values file to its line.
func keyLines(data []byte) map[string]int {
	lines := map[string]int{}
	f := yamlkit.Parse(data)
	if len(f.Docs) == 0 || f.Docs[0].Root == nil {
		return lines
	}
	var walk func(n *yamlkit.Node, path yamlkit.Path)
	walk = func(n *yamlkit.Node, path yamlkit.Path) {
		for _, pr := range n.Pairs {
			if pr.Key == nil || pr.Key.Kind != yamlkit.KindScalar {
				continue
			}
			p := append(slices.Clone(path), yamlkit.Segment{Key: pr.Key.Value})
			lines[p.String()] = pr.Key.Range.Start.Line
			if pr.Value != nil && pr.Value.Kind == yamlkit.KindMap {
				walk(pr.Value, p)
			}
		}
	}
	walk(f.Docs[0].Root, nil)
	return lines
}

// explain records origins for every key of vals (the final values at
// path) and recurses into subchart scopes.
func explain(p Provenance, path yamlkit.Path, vals map[string]any, srcs []source, ch *chart.Chart) {
	for key, v := range vals {
		kp := append(slices.Clone(path), yamlkit.Segment{Key: key})
		front, back := subchartOrigins(kp, ch, srcs)
		chain := front
		for _, s := range srcs {
			if val, ok := lookup(s.vals, kp); ok {
				chain = append(chain, Origin{Layer: s.name, Kind: s.kind, Line: s.lines[kp.String()], Value: summarize(val)})
			}
		}
		chain = append(chain, back...)
		if len(chain) == 0 {
			if _, isMap := v.(map[string]any); !isMap {
				chain = []Origin{{Layer: "Helm", Kind: "computed", Value: summarize(v)}}
			}
		}
		if len(chain) > 0 {
			p[kp.String()] = chain
		}
		if m, ok := v.(map[string]any); ok {
			explain(p, kp, m, srcs, ch)
		}
	}
}

// subchartOrigins returns what applies to a path under a subchart's
// key besides the parent's own layers: first (winning) the parent's
// globals, which Helm copies over <sub>.global.*, and last (losing) the
// subchart's own values.yaml.
func subchartOrigins(path yamlkit.Path, ch *chart.Chart, top []source) (front, back []Origin) {
	if ch == nil || len(path) < 2 {
		return nil, nil
	}
	for _, dep := range ch.Dependencies() {
		if dep.Metadata == nil || dep.Metadata.Name != path[0].Key {
			continue
		}
		rest := path[1:]
		if rest[0].Key == "global" {
			for _, s := range top {
				if val, ok := lookup(s.vals, rest); ok {
					front = append(front, Origin{Layer: s.name, Kind: "global", Line: s.lines[rest.String()], Value: summarize(val)})
				}
			}
		}
		src := depSource(dep)
		if val, ok := lookup(src.vals, rest); ok {
			back = append(back, Origin{Layer: src.name, Kind: "default", Line: src.lines[rest.String()], Value: summarize(val)})
		}
		f, b := subchartOrigins(rest, dep, top)
		return append(front, f...), append(back, b...)
	}
	return nil, nil
}

// depSource reads a dependency's default values, with lines from its
// raw values.yaml when the chart kept it.
func depSource(dep *chart.Chart) source {
	name := dep.Metadata.Name + " chart values.yaml"
	for _, f := range dep.Raw {
		if f.Name == "values.yaml" {
			return valuesSource(name, "default", f.Data)
		}
	}
	return source{name: name, kind: "default", vals: dep.Values}
}

// deletions records keys a user layer removed with null.
func deletions(p Provenance, srcs []source, final map[string]any) {
	var walk func(vals map[string]any, path yamlkit.Path)
	walk = func(vals map[string]any, path yamlkit.Path) {
		for key, v := range vals {
			kp := append(slices.Clone(path), yamlkit.Segment{Key: key})
			if _, ok := lookup(final, kp); ok {
				if m, ok := v.(map[string]any); ok {
					walk(m, kp)
				}
				continue
			}
			var chain []Origin
			for _, s := range srcs {
				if val, ok := lookup(s.vals, kp); ok {
					chain = append(chain, Origin{Layer: s.name, Kind: s.kind, Line: s.lines[kp.String()], Value: summarize(val), Deleted: val == nil})
				}
			}
			if len(chain) > 1 && chain[0].Deleted {
				p[kp.String()] = chain
			}
		}
	}
	for _, s := range srcs {
		walk(s.vals, nil)
	}
}

// lookup walks maps along path; ok reports that the key exists (even
// with a nil value).
func lookup(vals map[string]any, path yamlkit.Path) (any, bool) {
	var cur any = vals
	for _, seg := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		if cur, ok = m[seg.Key]; !ok {
			return nil, false
		}
	}
	return cur, true
}

// summarize keeps scalars and shortens collections for display.
func summarize(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return fmt.Sprintf("{%d keys}", len(t))
	case []any:
		return fmt.Sprintf("[%d items]", len(t))
	}
	return v
}

func fileData(files []File, name string) []byte {
	for _, f := range files {
		if f.Name == name {
			return f.Data
		}
	}
	return nil
}
