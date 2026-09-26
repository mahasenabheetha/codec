package provider

import (
	"testing"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

func TestDetect(t *testing.T) {
	tests := []struct {
		name string
		path string
		src  string
		want string
	}{
		{"helm chart", "charts/app/Chart.yaml", "apiVersion: v2\nname: app\nversion: 0.1.0\n", "helm-chart"},
		{"helm template", "charts/app/templates/svc.yaml",
			"apiVersion: v1\nkind: Service\nmetadata:\n  name: {{ include \"app.fullname\" . }}\n", "helm-template"},
		{"helm template without path", "",
			"kind: Service\nmetadata:\n  name: {{ .Release.Name }}\n", "helm-template"},
		{"helm values", "charts/app/values-prod.yaml", "replicaCount: 3\nimage:\n  tag: v1\n", "helm-values"},
		{"kustomize by name", "overlays/prod/kustomization.yaml", "resources:\n  - ../../base\n", "kustomize"},
		{"argo workflow", "", "apiVersion: argoproj.io/v1alpha1\nkind: WorkflowTemplate\nmetadata:\n  name: build\n", "argo-workflows"},
		{"argo cd app", "", "apiVersion: argoproj.io/v1alpha1\nkind: Application\nmetadata:\n  name: api\n", "argocd"},
		{"kubernetes multi-doc", "k8s/all.yaml",
			"apiVersion: v1\nkind: Service\nmetadata:\n  name: a\n---\napiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: a\n", "kubernetes"},
		{"github actions by path", ".github/workflows/ci.yml", "on: push\njobs:\n  test:\n    runs-on: ubuntu-latest\n", "github-actions"},
		{"github actions by content", "ci.yml", "on: [push]\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps: []\n", "github-actions"},
		{"gitlab ci by name", ".gitlab-ci.yml", "stages: [build]\nbuild:\n  stage: build\n  script: [make]\n", "gitlab-ci"},
		{"gitlab ci by content", "pipeline.yml", "build:\n  stage: build\n  script:\n    - make\n", "gitlab-ci"},
		{"azure pipelines", "azure-pipelines.yml", "trigger: [main]\npool:\n  vmImage: ubuntu-latest\nsteps:\n  - script: make\n", "azure-pipelines"},
		{"azure by content", "build.yml", "stages:\n  - stage: Build\n    jobs: []\n", "azure-pipelines"},
		{"compose", "docker-compose.override.yml", "services:\n  db:\n    image: postgres\n", "compose"},
		{"compose by content", "stack.yml", "services:\n  web:\n    build: .\n", "compose"},
		{"ansible playbook", "site.yml", "- hosts: web\n  tasks:\n    - name: ping\n      ping: {}\n", "ansible-playbook"},
		{"ansible role tasks", "roles/web/tasks/main.yml", "- name: install nginx\n  apt:\n    name: nginx\n", "ansible-playbook"},
		{"ansible inventory", "inventory/prod.yml", "all:\n  children:\n    web:\n      hosts:\n        web1: {}\n", "ansible-inventory"},
		{"plain yaml", "notes.yaml", "title: hello\nitems: [1, 2]\n", "yaml"},
		{"unparseable still gets generic", "x.yaml", "a: \"unclosed\n", "yaml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &File{Path: tt.path, YAML: yamlkit.Parse([]byte(tt.src))}
			matches := Default.Detect(f)
			if len(matches) == 0 {
				t.Fatal("no provider matched (generic YAML should always match)")
			}
			if got := matches[0].Provider.ID(); got != tt.want {
				t.Errorf("best = %s (%d), want %s; all: %v", got, matches[0].Confidence, tt.want, ids(matches))
			}
		})
	}
}

func ids(ms []Match) []string {
	var out []string
	for _, m := range ms {
		out = append(out, m.Provider.ID())
	}
	return out
}

func TestOutline(t *testing.T) {
	src := "metadata:\n  name: api\nspec:\n  containers:\n    - name: web\n      image: nginx\n    - plain\n"
	doc := yamlkit.Parse([]byte(src)).Docs[0]
	syms := Outline(doc)
	if len(syms) != 2 || syms[0].Name != "metadata" || syms[0].Detail != "{1}" {
		t.Fatalf("top level = %+v", syms)
	}
	containers := syms[1].Children[0]
	if containers.Name != "containers" || containers.Detail != "[2]" {
		t.Errorf("containers = %+v", containers)
	}
	if got := containers.Children[0].Name; got != "[0] web" {
		t.Errorf("list item labelled %q, want \"[0] web\"", got)
	}
	if got := containers.Children[1]; got.Name != "[1]" || got.Detail != "plain" {
		t.Errorf("scalar item = %+v", got)
	}
	if r := syms[0].Range; r.Start.Line != 1 || r.End.Line != 2 {
		t.Errorf("metadata spans %d-%d, want 1-2", r.Start.Line, r.End.Line)
	}
}

func TestRegisterReplaces(t *testing.T) {
	r := NewRegistry(det("x", "X", func(*File) Confidence { return 10 }))
	r.Register(det("x", "X2", func(*File) Confidence { return 20 }))
	if got := r.Best(&File{}); got.Title() != "X2" {
		t.Errorf("Register should replace by ID, got %s", got.Title())
	}
}
