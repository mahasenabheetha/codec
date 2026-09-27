package kube

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

func appObjects(t *testing.T) []Object {
	t.Helper()
	var objs []Object
	for _, name := range []string{"web.yaml", "config.yaml", "edge.yaml"} {
		data, err := os.ReadFile(filepath.Join("testdata/app", name))
		if err != nil {
			t.Fatal(err)
		}
		objs = append(objs, Collect(name, yamlkit.Parse(data))...)
	}
	return objs
}

func TestRelate(t *testing.T) {
	g := Relate(appObjects(t))
	var findings []string
	for _, f := range g.Findings {
		findings = append(findings, f.Code+" "+f.Object+" "+f.Severity)
	}
	slices.Sort(findings)
	want := []string{
		"ingress-port Ingress//web warning", // /api → port 9090 isn't exposed
		"missing-ref Ingress//web info",     // web-tls, made by cert-manager usually
		"missing-ref Ingress//web warning",  // Service legacy
		"selector-no-match Service//web-typo warning",
		"target-port Service//web-badport warning", // "metrics" isn't a container port name
	}
	if !slices.Equal(findings, want) {
		t.Errorf("findings\n  %q\nwant\n  %q", findings, want)
	}

	edges := map[string]bool{}
	for _, e := range g.Edges {
		edges[e.From+" "+e.Kind+" "+e.To] = true
	}
	for _, e := range []string{
		"Service//web selects Deployment//web",
		"Service//web-badport selects Deployment//web",
		"Ingress//web routes Service//web",
		"Deployment//web config ConfigMap//web-config",
		"Deployment//web secret Secret//db",
		"Deployment//web pulls Secret//registry",
		"Deployment//web volume PersistentVolumeClaim//web-data",
		"Deployment//web account ServiceAccount//web",
		"HorizontalPodAutoscaler//web scales Deployment//web",
		"RoleBinding//web-read binds ServiceAccount//web",
		"RoleBinding//web-read grants ClusterRole//view",
	} {
		if !edges[e] {
			t.Errorf("missing edge %s", e)
		}
	}
	if edges["Service//web-typo selects Deployment//web"] {
		t.Error("a non-matching selector was linked")
	}
}

func TestSelectors(t *testing.T) {
	sel := yamlkit.Parse([]byte("matchLabels: {app: web}\nmatchExpressions:\n  - {key: tier, operator: In, values: [frontend, api]}\n  - {key: canary, operator: DoesNotExist}\n")).Docs[0].Root
	tests := []struct {
		labels map[string]string
		want   bool
	}{
		{map[string]string{"app": "web", "tier": "frontend"}, true},
		{map[string]string{"app": "web", "tier": "db"}, false},
		{map[string]string{"app": "web", "tier": "api", "canary": "1"}, false},
		{map[string]string{"tier": "api"}, false},
	}
	for _, tt := range tests {
		if got := matchSelector(sel, tt.labels); got != tt.want {
			t.Errorf("%v: got %v", tt.labels, got)
		}
	}
	if matchLabels(nil, map[string]string{"a": "b"}) {
		t.Error("an empty selector must match nothing")
	}
}

func TestInventory(t *testing.T) {
	objs := appObjects(t)
	inv := Summarize(objs, Relate(objs))
	if len(inv.Images) != 2 || !inv.Images[0].Pinned || inv.Images[0].Tag != "sha256:abc" {
		t.Errorf("images: %+v", inv.Images)
	}
	tot := inv.Resources.Total
	// (250m + 100m) × 3 replicas; (256Mi + 64Mi) × 3.
	if tot.RequestsCPU != "1050m" || tot.RequestsMem != "960Mi" || !tot.Missing {
		t.Errorf("total: %+v", tot)
	}
	var cfg []string
	for _, c := range inv.Config {
		cfg = append(cfg, c.Kind+" "+c.Name)
	}
	if !slices.Contains(cfg, "Secret maybe") || !slices.Contains(cfg, "ConfigMap web-config") {
		t.Errorf("config: %v", cfg)
	}
}

func TestSecretValues(t *testing.T) {
	var db Object
	for _, o := range appObjects(t) {
		if o.Kind == "Secret" && o.Name == "db" {
			db = o
		}
	}
	vs := SecretValues(db.Root)
	if len(vs) != 2 || vs[0].Value != "s3cr3t" || !vs[1].Binary || vs[1].Size != 5 {
		t.Errorf("got %+v", vs)
	}
}

func TestNeat(t *testing.T) {
	in := `apiVersion: v1
kind: ConfigMap
metadata:
  name: c # keep me
  uid: 1234
  resourceVersion: "99"
  creationTimestamp: "2026-01-01T00:00:00Z"
  managedFields: [{manager: kubectl}]
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: '{}'
data:
  a: "1"
status: {}
`
	out, err := Neat([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	want := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: c # keep me\ndata:\n  a: \"1\"\n"
	if string(out) != want {
		t.Errorf("got\n%s", out)
	}
	if _, err := Neat([]byte("a: {{ .Values.x }}")); err == nil {
		t.Error("templates must be refused")
	}
}

func TestOutline(t *testing.T) {
	data, _ := os.ReadFile("testdata/app/web.yaml")
	f := yamlkit.Parse(data)
	p := provider.Default.Best(&provider.File{Path: "web.yaml", YAML: f})
	if _, ok := p.(Provider); !ok {
		t.Fatalf("the kube lens isn't registered: %T", p)
	}
	var found []string
	var walk func([]provider.Symbol)
	walk = func(ss []provider.Symbol) {
		for _, s := range ss {
			if strings.HasPrefix(s.Name, "Container") || strings.HasPrefix(s.Name, "Env ") || strings.HasPrefix(s.Name, "Volume") {
				found = append(found, s.Name+" = "+s.Detail)
			}
			walk(s.Children)
		}
	}
	walk(p.Symbols(f.Docs[0]))
	for _, w := range []string{
		"Container app = nginx:1.27 · 1 port",
		"Env DB_PASSWORD = from Secret db/password",
		"Volume data = PVC web-data",
	} {
		if !slices.Contains(found, w) {
			t.Errorf("missing %q in %q", w, found)
		}
	}
}

// diskReader reads fixtures by slash path under a root.
type diskReader string

func (d diskReader) ReadFile(p string) ([]byte, error) {
	return os.ReadFile(filepath.Join(string(d), filepath.FromSlash(p)))
}

func (d diskReader) IsDir(p string) bool {
	fi, err := os.Stat(filepath.Join(string(d), filepath.FromSlash(p)))
	return err == nil && fi.IsDir()
}

// TestKustomize compares with `kubectl kustomize` output (the goldens
// were produced by kubectl 1.36 / kustomize v5.8.1).
func TestKustomize(t *testing.T) {
	for _, tt := range []struct{ dir, golden string }{
		{"overlays/prod", "prod.golden.yaml"},
		{"base", "base.golden.yaml"},
	} {
		t.Run(tt.dir, func(t *testing.T) {
			files, err := KustomizeFiles(diskReader("testdata/kustomize"), tt.dir)
			if err != nil {
				t.Fatal(err)
			}
			got, err := Kustomize(files, tt.dir)
			if err != nil {
				t.Fatal(err)
			}
			want, _ := os.ReadFile(filepath.Join("testdata/kustomize", tt.golden))
			if got != strings.ReplaceAll(string(want), "\r\n", "\n") {
				t.Errorf("differs from kubectl kustomize:\n%s", got)
			}
		})
	}
	if _, err := KustomizeFiles(diskReader("testdata"), "app"); err == nil {
		t.Error("a folder without kustomization.yaml must fail")
	}
}

// TestSameObjectTwice: a base and an overlay's patch define the same
// Deployment. The graph shows it once; each card keeps a unique id (the
// UI keys lists by it).
func TestSameObjectTwice(t *testing.T) {
	deploy := "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: shop\nspec:\n  replicas: 1\n"
	var objs []Object
	for _, file := range []string{"base/deployment.yaml", "overlays/prod/replicas.yaml"} {
		objs = append(objs, Collect(file, yamlkit.Parse([]byte(deploy)))...)
	}
	g := Relate(objs)
	if len(g.Nodes) != 1 {
		t.Errorf("%d nodes, want 1", len(g.Nodes))
	}
	cards := Cards(objs, g)
	if len(cards) != 2 || cards[0].ID == cards[1].ID {
		t.Errorf("cards %+v", cards)
	}
}
