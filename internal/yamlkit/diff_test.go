package yamlkit

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

const deployA = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
  labels: {app: api, tier: web}
spec:
  replicas: 2
  template:
    spec:
      containers:
        - name: api
          image: api:1.0
          env:
            - name: A
              value: "1"
            - name: B
              value: "2"
          args: [--port, "8080"]
        - name: sidecar
          image: proxy:2
---
apiVersion: v1
kind: Service
metadata:
  name: api
spec:
  ports: [{port: 80}]
`

func diffOf(t *testing.T, a, b string, opts DiffOptions) []string {
	t.Helper()
	cs, err := Diff(Parse([]byte(a)), Parse([]byte(b)), opts)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, c := range cs {
		s := fmt.Sprintf("%s %s %s", c.Kind, c.Doc, c.Path)
		if c.Kind == Changed {
			s += fmt.Sprintf(": %s -> %s", c.Old, c.New)
		}
		out = append(out, s)
	}
	return out
}

func TestDiff(t *testing.T) {
	tests := []struct {
		name string
		b    string
		opts DiffOptions
		want []string
	}{
		{"identical", deployA, DiffOptions{}, nil},
		{"reordered keys, docs, containers and env", `apiVersion: v1
kind: Service
spec:
  ports: [{port: 80}]
metadata:
  name: api
---
kind: Deployment
apiVersion: apps/v1
spec:
  template:
    spec:
      containers:
        - image: proxy:2
          name: sidecar
        - name: api
          args: [--port, "8080"]
          env:
            - {name: B, value: "2"}
            - {value: "1", name: A}
          image: api:1.0
  replicas: 2
metadata:
  labels: {tier: web, app: api}
  name: api
`, DiffOptions{}, nil},
		{"value changes", replaceAll(deployA, "replicas: 2", "replicas: 3", "api:1.0", "api:1.1"), DiffOptions{},
			[]string{"changed Deployment api spec.replicas: 2 -> 3", "changed Deployment api spec.template.spec.containers[name=api].image: api:1.0 -> api:1.1"}},
		{"type change is a change", replaceAll(deployA, `value: "1"`, "value: 1"), DiffOptions{},
			[]string{`changed Deployment api spec.template.spec.containers[name=api].env[name=A].value: "1" -> 1`}},
		{"renamed container", replaceAll(deployA, "- name: sidecar", "- name: envoy"), DiffOptions{},
			[]string{"changed Deployment api spec.template.spec.containers[name=sidecar].name: sidecar -> envoy"}},
		{"added and removed", replaceAll(deployA, "  labels: {app: api, tier: web}\n", "  labels: {app: api}\n  annotations: {a: b}\n"), DiffOptions{},
			[]string{"removed Deployment api metadata.labels.tier", "added Deployment api metadata.annotations"}},
		{"args order matters", replaceAll(deployA, `args: [--port, "8080"]`, `args: ["8080", --port]`), DiffOptions{},
			[]string{"reordered Deployment api spec.template.spec.containers[name=api].args"}},
		{"new document", deployA + "---\napiVersion: v1\nkind: ConfigMap\nmetadata: {name: c}\n", DiffOptions{},
			[]string{"added ConfigMap c "}},
		{"ignore paths", replaceAll(deployA, "replicas: 2", "replicas: 3", "api:1.0", "api:1.1"),
			DiffOptions{Ignore: []string{"spec.replicas", "spec.template.spec.containers[*].image"}}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := diffOf(t, deployA, tt.b, tt.opts)
			if !slices.Equal(got, tt.want) {
				t.Errorf("got\n  %q\nwant\n  %q", got, tt.want)
			}
		})
	}
}

func TestDiffRanges(t *testing.T) {
	cs, _ := Diff(Parse([]byte("a: 1\nb: 2\n")), Parse([]byte("b: 2\n\na: 5\n")), DiffOptions{})
	if len(cs) != 1 || cs[0].OldRange.Start.Line != 1 || cs[0].NewRange.Start.Line != 3 {
		t.Fatalf("ranges: %+v", cs)
	}
}

func TestDiffUnparsed(t *testing.T) {
	if _, err := Diff(Parse([]byte("a: [")), Parse([]byte("a: 1")), DiffOptions{}); !errors.Is(err, ErrUnparsed) {
		t.Errorf("got %v", err)
	}
}

// replaceAll applies old/new pairs once each; a missing old is a
// broken fixture.
func replaceAll(s string, pairs ...string) string {
	for i := 0; i+1 < len(pairs); i += 2 {
		if !strings.Contains(s, pairs[i]) {
			panic("fixture: " + pairs[i] + " not found")
		}
		s = strings.Replace(s, pairs[i], pairs[i+1], 1)
	}
	return s
}
