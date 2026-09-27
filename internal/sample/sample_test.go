package sample

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mahasenabheetha/codec/v2/internal/check"
	"github.com/mahasenabheetha/codec/v2/internal/config"
	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// TestExtract writes the sample twice: the second run changes nothing
// but repairs a file edited in between.
func TestExtract(t *testing.T) {
	dir := t.TempDir()
	if err := Extract(dir); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"README.md", ".github/workflows/ci.yml", ".env", "charts/shop/.helmignore", "charts/shop/templates/_helpers.tpl"} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(p))); err != nil {
			t.Errorf("%s not extracted: %v", p, err)
		}
	}
	chart := filepath.Join(dir, "charts", "shop", "Chart.yaml")
	os.WriteFile(chart, []byte("changed"), 0o644)
	if err := Extract(dir); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(chart); !strings.Contains(string(b), "name: shop") {
		t.Errorf("Chart.yaml not restored: %q", b)
	}
}

// TestShowcase keeps the sample a showcase: every file type codec knows
// is in it, and the only problems are the ones k8s/legacy has on
// purpose (the README points at them).
func TestShowcase(t *testing.T) {
	chk := check.New(provider.Default, config.Lint{NoSchemas: true}, "test")
	src := Files()
	patches := check.NewPatches(func(p string) ([]byte, error) { return fs.ReadFile(src, p) })
	types := map[string]bool{}
	fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !(strings.HasSuffix(p, ".yaml") || strings.HasSuffix(p, ".yml")) {
			return err
		}
		data, _ := fs.ReadFile(src, p)
		f := &provider.File{Path: p, Content: data, YAML: yamlkit.Parse(data), Patch: patches.Is(p)}
		a := chk.Analyze(context.Background(), f, time.Second)
		types[a.Type] = true
		if strings.HasPrefix(p, "k8s/legacy/") {
			if len(a.Diagnostics) == 0 {
				t.Errorf("%s: expected the planted problems", p)
			}
			return nil
		}
		for _, d := range a.Diagnostics {
			t.Errorf("%s:%d: %s [%s]", p, d.Range.Start.Line, d.Message, d.Code)
		}
		return nil
	})
	for _, id := range []string{"helm-chart", "helm-template", "helm-values", "kustomize", "kubernetes", "argo-workflows", "argocd",
		"github-actions", "gitlab-ci", "azure-pipelines", "compose", "ansible-playbook", "ansible-inventory"} {
		if !types[id] {
			got := make([]string, 0, len(types))
			for k := range types {
				got = append(got, k)
			}
			slices.Sort(got)
			t.Errorf("no %s file in the sample (have %v)", id, got)
		}
	}
}
