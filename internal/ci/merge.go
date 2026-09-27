package ci

import (
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Effective configurations are built with yamlkit's layered merge:
// origins say, per merged tree, where each entry was written.

// origin is where a merged entry was written; Via is "" (the job
// itself), "extends .base", "default", "!reference .x", "template t.yml".
type origin = yamlkit.Origin

// tags records origins by node.
type tags = yamlkit.Origins

// emit writes a merged tree as effective-configuration lines.
func emit(t tags, n *yamlkit.Node, base origin) []Line {
	var out []Line
	for _, l := range t.Emit(n, base) {
		line := Line{Text: l.Text, From: l.Origin.Via}
		if l.At != nil {
			line.Source = &Source{File: l.Origin.File, Line: l.At.Range.Start.Line, Range: l.At.Range}
		}
		out = append(out, line)
	}
	return out
}

func isMap(n *yamlkit.Node) bool { return yamlkit.IsPlainMap(n) }

// isRef reports whether n is an unresolved GitLab !reference.
func isRef(n *yamlkit.Node) bool { return n != nil && n.Tag == "!reference" }

func keyName(k *yamlkit.Node) string { return yamlkit.KeyName(k) }

func indexOf(m *yamlkit.Node, key string) int { return yamlkit.IndexOf(m, key) }

// scalar and flow show a node as YAML would on one line.
func scalar(n *yamlkit.Node) string { return yamlkit.InlineText(n) }
func flow(n *yamlkit.Node) string   { return yamlkit.InlineText(n) }
