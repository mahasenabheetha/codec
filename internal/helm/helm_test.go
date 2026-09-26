package helm

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// loadChart reads a fixture chart the way the workspace adapter does:
// every file under dir, minus .helmignore matches.
func loadChart(t *testing.T, dir string) []File {
	t.Helper()
	ig, _ := os.ReadFile(filepath.Join(dir, ".helmignore"))
	skip, err := Ignorer(ig)
	if err != nil {
		t.Fatal(err)
	}
	var files []File
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == dir {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		rel = filepath.ToSlash(rel)
		if skip(rel, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(p)
		files = append(files, File{Name: rel, Data: data})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func prodLayer(t *testing.T) Layer {
	data, err := os.ReadFile("testdata/values-prod.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return Layer{Name: "values-prod.yaml", Data: data}
}

func render(t *testing.T, opts Options) *Result {
	t.Helper()
	return Render(context.Background(), loadChart(t, "testdata/app"), opts)
}

func errorsOf(r *Result) []Diagnostic {
	var out []Diagnostic
	for _, d := range r.Diagnostics {
		if d.Severity == "error" {
			out = append(out, d)
		}
	}
	return out
}

// TestMatchesHelmCLI compares against the real `helm template` of the
// same version. Opt-in: set HELM_BIN to a helm v4.3 binary (build it
// with `go build helm.sh/helm/v4/cmd/helm`, or use alpine/helm:4.3.0).
func TestMatchesHelmCLI(t *testing.T) {
	bin := os.Getenv("HELM_BIN")
	if bin == "" {
		t.Skip("set HELM_BIN to compare with the helm CLI")
	}
	tests := []struct {
		name string
		args []string
		opts Options
	}{
		{"defaults", nil, Options{}},
		{"values file", []string{"-f", "testdata/values-prod.yaml"}, Options{Values: []Layer{prodLayer(t)}}},
		{"set", []string{"--set", "replicaCount=7,image.tag=x", "--set", "db.enabled=false"}, Options{Set: []string{"replicaCount=7,image.tag=x", "db.enabled=false"}}},
		{"release and namespace", []string{"--namespace", "prod", "--kube-version", "1.30.0"}, Options{Release: "web", Namespace: "prod", KubeVersion: "1.30.0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := tt.opts.Release
			if name == "" {
				name = "release-name"
			}
			args := append([]string{"template", name, "testdata/app"}, tt.args...)
			want, err := exec.Command(bin, args...).Output()
			if err != nil {
				t.Fatalf("helm %v: %v", args, err)
			}
			got := render(t, tt.opts)
			if errs := errorsOf(got); len(errs) > 0 {
				t.Fatalf("render errors: %+v", errs)
			}
			if got.Manifest != string(want) {
				t.Errorf("output differs from helm template\n--- codec ---\n%s\n--- helm ---\n%s", got.Manifest, want)
			}
		})
	}
}

func TestRender(t *testing.T) {
	r := render(t, Options{Values: []Layer{prodLayer(t)}})
	if errs := errorsOf(r); len(errs) > 0 {
		t.Fatalf("errors: %+v", errs)
	}
	if r.Chart.Name != "app" || !strings.Contains(r.Manifest, "replicas: 3") {
		t.Errorf("chart %+v, manifest missing replicas: 3", r.Chart)
	}
	var sawHook, sawDB bool
	for _, d := range r.Docs {
		sawHook = sawHook || (d.Hook && d.Kind == "Job")
		sawDB = sawDB || strings.HasPrefix(d.Source, "app/charts/db/")
	}
	if !sawHook || !sawDB {
		t.Errorf("docs = %+v; want the pre-install hook and the db subchart", r.Docs)
	}
	if !strings.Contains(r.Notes, "Get the application URL") {
		t.Errorf("notes = %q", r.Notes)
	}
}

func TestProvenance(t *testing.T) {
	r := render(t, Options{Values: []Layer{prodLayer(t)}, Set: []string{"image.tag=hotfix"}})
	tests := []struct {
		path    string
		layers  []string // winner first
		deleted bool
	}{
		{"replicaCount", []string{"values-prod.yaml", "values.yaml"}, false},
		{"image.tag", []string{"--set image.tag=hotfix", "values-prod.yaml", "values.yaml"}, false},
		{"image.repository", []string{"values.yaml"}, false},
		{"extraArgs", []string{"values-prod.yaml", "values.yaml"}, false}, // lists replace
		{"resources", []string{"values-prod.yaml", "values.yaml"}, true},  // null deletes
		{"db.replicaCount", []string{"values-prod.yaml", "values.yaml", "db chart values.yaml"}, false},
		{"db.global.env", []string{"values-prod.yaml", "values.yaml", "db chart values.yaml"}, false}, // parent globals win
		{"db.global.team", []string{"db chart values.yaml"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			chain := r.Provenance[tt.path]
			var got []string
			for _, o := range chain {
				got = append(got, o.Layer)
			}
			if strings.Join(got, " | ") != strings.Join(tt.layers, " | ") {
				t.Fatalf("chain = %v, want %v", got, tt.layers)
			}
			if chain[0].Deleted != tt.deleted {
				t.Errorf("deleted = %v, want %v", chain[0].Deleted, tt.deleted)
			}
			if !tt.deleted && chain[0].Kind != "set" && chain[0].Line == 0 && chain[0].Kind != "global" {
				t.Errorf("winner has no line: %+v", chain[0])
			}
		})
	}
	// The values themselves are Helm's own.
	if r.Values["replicaCount"] != float64(3) {
		t.Errorf("replicaCount = %#v", r.Values["replicaCount"])
	}
	if _, ok := r.Values["resources"]; ok {
		t.Error("resources should be deleted by null")
	}
	if db := r.Values["db"].(map[string]any)["global"].(map[string]any); db["env"] != "prod" || db["team"] != "data" {
		t.Errorf("db.global = %v", db)
	}
}

func TestReferences(t *testing.T) {
	files := loadChart(t, "testdata/app")
	for i, f := range files {
		if f.Name == "templates/service.yaml" {
			files[i].Data = append(f.Data, []byte("# {{ .Values.service.nodePortt }}\n")...)
		}
	}
	r := Render(context.Background(), files, Options{})
	var undefined, unusedExtra bool
	for _, d := range r.Diagnostics {
		if d.Code == "undefined-value" && strings.Contains(d.Message, "service.nodePortt") && d.File == "templates/service.yaml" && d.Line > 0 {
			undefined = true
		}
		if d.Code == "unused-value" && strings.HasPrefix(d.Message, "extraArgs") {
			unusedExtra = true
		}
	}
	if !undefined {
		t.Errorf("want an undefined-value warning, got %+v", r.Diagnostics)
	}
	if unusedExtra {
		t.Error("extraArgs is used by the deployment")
	}
}

func TestRenderErrors(t *testing.T) {
	files := loadChart(t, "testdata/app")
	for i, f := range files {
		if f.Name == "templates/service.yaml" {
			files[i].Data = []byte("apiVersion: v1\nkind: Service\nmetadata:\n  name: {{ required \"name is required\" .Values.nope }}\n")
		}
	}
	r := Render(context.Background(), files, Options{})
	errs := errorsOf(r)
	if len(errs) != 1 || errs[0].File != "templates/service.yaml" || errs[0].Line != 4 || errs[0].Message != "name is required" {
		t.Fatalf("errors = %+v", errs)
	}

	missing := loadChart(t, "testdata/app")
	var kept []File
	for _, f := range missing {
		if !strings.HasPrefix(f.Name, "charts/db/") {
			kept = append(kept, f)
		}
	}
	r = Render(context.Background(), kept, Options{})
	if errs := errorsOf(r); len(errs) != 1 || errs[0].Code != "missing-dependency" {
		t.Fatalf("want missing-dependency, got %+v", errs)
	}
}

func TestFindCharts(t *testing.T) {
	got := FindCharts([]string{"Chart.yaml", "charts/sub/Chart.yaml", "deploy/api/Chart.yaml", "deploy/api/charts/x/Chart.yaml", "values.yaml"})
	if strings.Join(got, ",") != ".,deploy/api" {
		t.Errorf("FindCharts = %v", got)
	}
	if ChartOf("deploy/api/templates/a.yaml", got) != "deploy/api" || ChartOf("docs/x.yaml", []string{"deploy/api"}) != "" {
		t.Error("ChartOf")
	}
}
