package schema

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// fixtureLoader serves the real Kubernetes 1.34 Deployment schema.
type fixtureLoader struct{}

func (fixtureLoader) Load(url string) (any, error) {
	raw, err := os.ReadFile("testdata/deployment-apps-v1.json.gz")
	if err != nil {
		return nil, err
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(zr)
}

func deployment(t *testing.T) *jsonschema.Schema {
	t.Helper()
	ref, ok := KubernetesRef("apps/v1", "Deployment", "1.34")
	if !ok {
		t.Fatal("no ref")
	}
	sch, err := Compile(fixtureLoader{}, ref.URL)
	if err != nil {
		t.Fatal(err)
	}
	return sch
}

func TestFor(t *testing.T) {
	tests := []struct {
		typ, path, src string
		want           string // URL suffix, "" = none
	}{
		{"kubernetes", "d.yaml", "apiVersion: apps/v1\nkind: Deployment\n", "v1.34.0-standalone-strict/deployment-apps-v1.json"},
		{"kubernetes", "s.yaml", "apiVersion: v1\nkind: Service\n", "v1.34.0-standalone-strict/service-v1.json"},
		{"kubernetes", "i.yaml", "apiVersion: networking.k8s.io/v1\nkind: Ingress\n", "v1.34.0-standalone-strict/ingress-networking-v1.json"},
		{"argo-workflows", "w.yaml", "apiVersion: argoproj.io/v1alpha1\nkind: Workflow\n", "CRDs-catalog/main/argoproj.io/workflow_v1alpha1.json"},
		{"kustomize", "kustomization.yaml", "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\n", "kustomization.json"},
		{"github-actions", ".github/workflows/ci.yml", "on: push\njobs: {}\n", "github-workflow.json"},
		{"github-actions", "action.yml", "name: x\nruns: {}\n", "github-action.json"},
		{"compose", "compose.yaml", "services: {}\n", "compose-spec.json"},
		{"helm-template", "t.yaml", "apiVersion: apps/v1\nkind: Deployment\n", ""},
		{"kubernetes", "t.yaml", "apiVersion: apps/v1\nkind: '{{ .Values.kind }}'\n", ""},
		{"yaml", "x.yaml", "a: 1\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.typ+" "+tt.path, func(t *testing.T) {
			f := yamlkit.Parse([]byte(tt.src))
			ref, ok := For(tt.typ, tt.path, f.Docs[0].Root, "1.34")
			if tt.want == "" {
				if ok {
					t.Errorf("got %s, want none", ref.URL)
				}
				return
			}
			if !ok || !strings.HasSuffix(ref.URL, tt.want) {
				t.Errorf("got %q (%v), want …%s", ref.URL, ok, tt.want)
			}
		})
	}
}

const validDeployment = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
spec:
  replicas: 2
  selector:
    matchLabels: {app: api}
  template:
    metadata:
      labels: {app: api}
    spec:
      containers:
        - name: api
          image: nginx:1.27
          ports:
            - containerPort: 80
`

func TestValidate(t *testing.T) {
	sch := deployment(t)
	tests := []struct {
		name string
		src  string
		want []string // "line: message", hint after " | " when checked
	}{
		{"valid", validDeployment, nil},
		{"typo with suggestion", strings.Replace(validDeployment, "  replicas: 2", "  replica: 2", 1),
			[]string{`6: Unknown field "replica" in spec | Did you mean "replicas"?`}},
		{"prefix suggestion", strings.Replace(validDeployment, "  replicas: 2", "  revisionHistory: 2", 1),
			[]string{`6: Unknown field "revisionHistory" in spec | Did you mean "revisionHistoryLimit"?`}},
		{"wrong type", strings.Replace(validDeployment, "replicas: 2", "replicas: two", 1),
			[]string{`6: Expected integer, got string in spec.replicas`}},
		{"number where string wanted", strings.Replace(validDeployment, "name: api\n          image", "name: 42\n          image", 1),
			[]string{`14: Expected string, got number in spec.template.spec.containers[].name | Quote it ("42") to make it text.`}},
		{"int-or-string accepts both", strings.Replace(validDeployment, "containerPort: 80", "containerPort: 80\n              name: http", 1), nil},
		{"enum", strings.Replace(validDeployment, "apps/v1", "apps/v2", 1),
			[]string{`1: Must be one of apps/v1 in apiVersion`}},
		{"runtime expression skipped", strings.Replace(validDeployment, "replicas: 2", `replicas: "{{workflow.parameters.n}}"`, 1), nil},
		{"required container name", strings.Replace(validDeployment, "        - name: api\n          image", "        - image", 1),
			[]string{`14: Missing required field "name" in spec.template.spec.containers[] | Add name: to spec.template.spec.containers[].`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := yamlkit.Parse([]byte(tt.src))
			if f.HasErrors() {
				t.Fatalf("fixture doesn't parse: %v", f.Diagnostics)
			}
			var got []string
			for _, d := range Validate(sch, f.Docs[0], false) {
				s := fmt.Sprintf("%d: %s", d.Range.Start.Line, d.Message)
				for _, w := range tt.want {
					if strings.Contains(w, " | ") && strings.HasPrefix(w, s+" | ") {
						s += " | " + d.Hint
					}
				}
				got = append(got, s)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("got\n  %q\nwant\n  %q", got, tt.want)
			}
		})
	}
}

// at returns the 1-based line/col of the "|" marker and the text without it.
func cursor(src string) (string, int, int) {
	i := strings.Index(src, "|")
	before := src[:i]
	line := strings.Count(before, "\n") + 1
	col := len(before) - strings.LastIndex(before, "\n")
	return before + src[i+1:], line, col
}

func TestComplete(t *testing.T) {
	sch := deployment(t)
	tests := []struct {
		name     string
		src      string
		want     []string // must be offered
		not      []string // must not be offered
		wantFrom int
	}{
		{"deployment spec keys", "apiVersion: apps/v1\nkind: Deployment\nspec:\n  re|\n", []string{"replicas", "revisionHistoryLimit"}, []string{"selector"}, 3},
		{"skips present keys", "apiVersion: apps/v1\nkind: Deployment\nspec:\n  replicas: 1\n  |\n  selector: {}\n", []string{"template", "strategy"}, []string{"replicas", "selector"}, 3},
		{"container keys in a list item", "spec:\n  template:\n    spec:\n      containers:\n        - name: a\n          im|\n", []string{"image", "imagePullPolicy"}, []string{"name"}, 11},
		{"new list item", "spec:\n  template:\n    spec:\n      containers:\n        - name: a\n        - |\n", []string{"name", "image"}, nil, 11},
		{"non-indented list", "spec:\n  template:\n    spec:\n      containers:\n      - name: a\n        res|\n", []string{"resources"}, nil, 9},
		{"top-level keys", "apiVersion: apps/v1\n|\n", []string{"kind", "metadata", "spec"}, []string{"apiVersion"}, 1},
		{"enum values", "spec:\n  template:\n    spec:\n      containers:\n        - name: a\n          imagePullPolicy: If|\n", []string{"IfNotPresent"}, []string{"Always"}, 28},
		{"boolean values", "spec:\n  paused: |\n", []string{"true", "false"}, nil, 11},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, line, col := cursor(tt.src)
			items, from := Complete(sch, []byte(src), line, col)
			var labels []string
			for _, it := range items {
				labels = append(labels, it.Label)
			}
			for _, w := range tt.want {
				if !slices.Contains(labels, w) {
					t.Errorf("missing %q in %v", w, labels)
				}
			}
			for _, n := range tt.not {
				if slices.Contains(labels, n) {
					t.Errorf("unexpected %q", n)
				}
			}
			if from != tt.wantFrom {
				t.Errorf("from = %d, want %d", from, tt.wantFrom)
			}
		})
	}
}

func TestInfoAt(t *testing.T) {
	sch := deployment(t)
	i, ok := InfoAt(sch, []Step{{Key: "spec"}, {Key: "replicas"}})
	if !ok || !strings.Contains(i.Description, "desired pods") || !slices.Contains(i.Types, "integer") {
		t.Errorf("replicas info = %+v", i)
	}
	i, _ = InfoAt(sch, []Step{{Key: "spec"}, {Key: "template"}, {Key: "spec"}, {Key: "containers"}, {Item: true}, {Key: "imagePullPolicy"}})
	if !slices.Contains(i.Enum, "IfNotPresent") {
		t.Errorf("imagePullPolicy enum = %v", i.Enum)
	}
}
