package helm

import (
	"bytes"
	"slices"

	"go.yaml.in/yaml/v3"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// ValuesYAML renders merged values as YAML (keys sorted, like `helm get
// values`) and maps each line that holds a key to that key's path, so a
// viewer can show provenance per line.
func ValuesYAML(values map[string]any) (string, map[int]string) {
	var b bytes.Buffer
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if len(values) > 0 {
		_ = enc.Encode(values) // maps of plain values always encode
	}
	enc.Close()
	text := b.String()

	lines := map[int]string{}
	f := yamlkit.Parse([]byte(text))
	if len(f.Docs) == 0 || f.Docs[0].Root == nil {
		return text, lines
	}
	var walk func(n *yamlkit.Node, path yamlkit.Path)
	walk = func(n *yamlkit.Node, path yamlkit.Path) {
		for _, pr := range n.Pairs {
			if pr.Key == nil || pr.Key.Kind != yamlkit.KindScalar {
				continue
			}
			p := append(slices.Clone(path), yamlkit.Segment{Key: pr.Key.Value})
			lines[pr.Key.Range.Start.Line] = p.String()
			if pr.Value != nil && pr.Value.Kind == yamlkit.KindMap {
				walk(pr.Value, p)
			}
		}
	}
	walk(f.Docs[0].Root, nil)
	return text, lines
}
