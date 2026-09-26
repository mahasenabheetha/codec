package provider

import (
	"strings"
	"testing"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

func fileOf(path, src string) *File {
	return &File{Path: path, YAML: yamlkit.Parse([]byte(src)), Content: []byte(src)}
}

func row(h *Hover, label string) string {
	for _, r := range h.Rows {
		if r.Label == label {
			return r.Value
		}
	}
	return ""
}

const anchors = `base: &base
  image: nginx
  replicas: 2
web:
  <<: *base
  enabled: yes
  mode: 0755
  script: |
    echo one
    echo two
`

func TestHover(t *testing.T) {
	f := fileOf("x.yaml", anchors)
	p := Default.Best(f)
	tests := []struct {
		name      string
		line, col int
		title     string
		label     string
		want      string
	}{
		{"map key", 1, 2, "base", "Type", "map · 2 keys"},
		{"anchor count", 1, 2, "base", "Anchor", "&base · 1 reference"},
		{"scalar on value", 2, 12, "base.image", "Value", "nginx"},
		{"scalar on key", 3, 4, "base.replicas", "Type", "integer"},
		{"alias", 5, 8, `web["<<"]`, "Alias of", "&base, line 1"},
		{"norway", 6, 13, "web.enabled", "Note", "YAML 1.1 tools read this as a boolean; quote it to keep a string"},
		{"octal", 7, 10, "web.mode", "Note", "YAML 1.1 tools read a leading 0 as octal"},
		{"block scalar", 8, 4, "web.script", "Style", "literal block (|), keeps newlines"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := HoverAt(p, f, yamlkit.PosAt(f.Content, tt.line, tt.col))
			if h == nil {
				t.Fatal("no hover")
			}
			if h.Title != tt.title || row(h, tt.label) != tt.want {
				t.Errorf("title %q, %s = %q; want %q, %q (rows %+v)", h.Title, tt.label, row(h, tt.label), tt.title, tt.want, h.Rows)
			}
		})
	}

	h := HoverAt(p, f, yamlkit.PosAt(f.Content, 5, 8))
	if !strings.HasPrefix(h.Code, "&base") {
		t.Errorf("alias hover should show the anchored value, got %q", h.Code)
	}
}

func TestHoverExpression(t *testing.T) {
	f := fileOf("charts/app/templates/cm.yaml", "metadata:\n  name: {{ .Release.Name }}-cm\n")
	h := HoverAt(Default.Best(f), f, yamlkit.PosAt(f.Content, 2, 14))
	if h == nil || h.Title != "Go template expression" || h.Code != "{{ .Release.Name }}" {
		t.Fatalf("hover = %+v", h)
	}
}

func TestDefinition(t *testing.T) {
	f := fileOf("x.yaml", anchors)
	locs := DefinitionAt(Default.Best(f), f, yamlkit.PosAt(f.Content, 5, 9))
	if len(locs) != 1 || locs[0].Range.Start.Line != 1 {
		t.Fatalf("definition = %+v, want the anchor on line 1", locs)
	}
	if locs := DefinitionAt(Default.Best(f), f, yamlkit.PosAt(f.Content, 2, 12)); len(locs) != 0 {
		t.Errorf("a plain value has no definition, got %+v", locs)
	}
}

func TestCompleteAnchors(t *testing.T) {
	// Half-typed: the last line doesn't parse, completion must still work.
	src := "a: &alpha 1\nb: &beta 2\n---\nc: &gamma 3\nd: &delta 4\ne: *d"
	f := fileOf("x.yaml", src)
	got := CompleteAt(Default.Best(f), f, yamlkit.PosAt(f.Content, 6, 6))
	if len(got) != 1 || got[0].Label != "delta" || got[0].Range.Start.Col != 5 {
		t.Fatalf("completions = %+v, want only delta (this document) replacing from col 5", got)
	}
	if got := CompleteAt(Default.Best(f), f, yamlkit.PosAt(f.Content, 5, 3)); len(got) != 0 {
		t.Errorf("no completion outside an alias, got %+v", got)
	}
}

func TestAnalyze(t *testing.T) {
	f := fileOf("k8s/svc.yaml", "apiVersion: v1\nkind: Service\nmetadata:\n  name: web\n---\nbad: [\n")
	a := Default.Analyze(f)
	if a.Type != "kubernetes" || len(a.Docs) != 2 {
		t.Fatalf("analysis = %+v", a)
	}
	if len(a.Docs[0].Symbols) != 3 || a.Docs[0].Name != "Service/web" {
		t.Errorf("doc 0 = %+v", a.Docs[0])
	}
	if len(a.Diagnostics) == 0 || a.Docs[1].Symbols == nil {
		t.Errorf("want a diagnostic for doc 2 and non-nil symbols, got %+v", a)
	}
}
