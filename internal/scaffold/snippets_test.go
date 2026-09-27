package scaffold

import (
	"strings"
	"testing"

	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

func file(src string) *provider.File {
	return &provider.File{Path: "x.yaml", Content: []byte(src), YAML: yamlkit.Parse([]byte(src))}
}

// cursor finds "|" in src, removes it and returns the file and position.
func cursor(src string) (*provider.File, yamlkit.Pos) {
	i := strings.Index(src, "|")
	src = src[:i] + src[i+1:]
	line := strings.Count(src[:i], "\n") + 1
	col := i - strings.LastIndex(src[:i], "\n")
	return file(src), yamlkit.PosAt([]byte(src), line, col)
}

func labels(cs []provider.Completion) string {
	var out []string
	for _, c := range cs {
		out = append(out, c.Label)
	}
	return strings.Join(out, " ")
}

func TestSnippets(t *testing.T) {
	for _, tc := range []struct {
		name, typ, src, want string
	}{
		{"kubernetes key", "kubernetes", "spec:\n  containers:\n    - name: a\n      reso|\n", "container probe resources"},
		{"after a dash", "kubernetes", "containers:\n  - con|\n", "container probe resources"},
		{"not in a value", "kubernetes", "image: con|\n", ""},
		{"github", "github-actions", "jobs:\n  a:\n    steps:\n      |\n", "step uses job"},
		{"gitlab", "gitlab-ci", "|\n", "job"},
		{"ansible", "ansible-playbook", "- hosts: all\n  tasks:\n    - |\n", "task"},
		{"compose", "compose", "services:\n  |\n", "service"},
		{"plain yaml", "yaml", "|\n", ""},
	} {
		f, pos := cursor(tc.src)
		if got := labels(Snippets(f, tc.typ, pos)); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, got, tc.want)
		}
	}

	// After "- " the item's own dash is dropped; the range is the word.
	f, pos := cursor("containers:\n  - con|\n")
	c := Snippets(f, "kubernetes", pos)[0]
	if !strings.HasPrefix(c.Insert, "name: ${1:app}\n  image:") || c.Range.Start.Col != 5 || c.Range.End.Col != 8 {
		t.Errorf("dash: %+v", c)
	}
	if !strings.Contains(c.Doc, "image: registry.example.com/app:1.0.0") {
		t.Errorf("preview: %s", c.Doc)
	}
}

func TestDAGTaskSnippet(t *testing.T) {
	src := `apiVersion: argoproj.io/v1alpha1
kind: WorkflowTemplate
metadata:
  name: w
spec:
  templates:
    - name: main
      dag:
        tasks:
          - name: fetch
            template: get
          - name: unpack
            template: get
          - |
    - name: get
      container:
        image: alpine:3.22
`
	f, pos := cursor(src)
	var task *provider.Completion
	for _, c := range Snippets(f, "argo-workflows", pos) {
		if c.Label == "dag-task" {
			task = &c
		}
	}
	if task == nil || task.Insert != "name: ${1:task-3}\n  template: ${2:get}\n  depends: ${3:unpack}${0}" {
		t.Fatalf("dag-task: %+v", task)
	}
}
