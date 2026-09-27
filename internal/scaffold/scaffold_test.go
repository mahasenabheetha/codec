package scaffold_test

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	_ "github.com/mahasenabheetha/codec/v2/internal/ansible"
	_ "github.com/mahasenabheetha/codec/v2/internal/argo"
	_ "github.com/mahasenabheetha/codec/v2/internal/ci"
	_ "github.com/mahasenabheetha/codec/v2/internal/compose"
	"github.com/mahasenabheetha/codec/v2/internal/helm"
	_ "github.com/mahasenabheetha/codec/v2/internal/kube"
	"github.com/mahasenabheetha/codec/v2/internal/lint"
	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/scaffold"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Every starter, with its defaults and with every switch on, must come
// out clean: no syntax, file-type or lint findings, and detected as the
// file type it is meant to be (so schema validation applies too).
func TestStartersAreClean(t *testing.T) {
	wantType := map[string]string{
		"k8s-app":               "kubernetes",
		"k8s-configmap":         "kubernetes",
		"k8s-secret":            "kubernetes",
		"helm-chart":            "helm-chart",
		"argo-workflowtemplate": "argo-workflows",
		"argo-cronworkflow":     "argo-workflows",
		"argocd-application":    "argocd",
		"github-workflow":       "github-actions",
		"gitlab-pipeline":       "gitlab-ci",
		"azure-pipeline":        "azure-pipelines",
		"compose-service":       "compose",
		"ansible-playbook":      "ansible-playbook",
		"ansible-role":          "ansible-playbook",
	}
	starters := scaffold.Builtin()
	if len(starters) != len(wantType) {
		t.Errorf("%d starters, want %d", len(starters), len(wantType))
	}
	for _, s := range starters {
		for _, variant := range []string{"defaults", "all on"} {
			values := s.Defaults()
			if variant == "all on" {
				for _, f := range s.Fields {
					switch {
					case f.Type == "bool":
						values[f.Name] = "true"
					case f.Name == "namespace":
						values[f.Name] = "team-a"
					case f.Name == "timezone":
						values[f.Name] = "Europe/Stockholm"
					}
				}
			}
			files, problems, err := s.Render(values)
			if err != nil || len(problems) > 0 {
				t.Fatalf("%s (%s): %v %v", s.ID, variant, err, problems)
			}
			if len(files) == 0 {
				t.Fatalf("%s (%s): no files", s.ID, variant)
			}
			for i, f := range files {
				if strings.Contains(f.Content, "<%") || strings.Contains(f.Path, "<%") {
					t.Errorf("%s: %s: placeholder left", s.ID, f.Path)
				}
				if !strings.HasSuffix(f.Path, ".yaml") && !strings.HasSuffix(f.Path, ".yml") {
					continue
				}
				pf := &provider.File{Path: f.Path, Content: []byte(f.Content), YAML: yamlkit.Parse([]byte(f.Content))}
				a := provider.Default.Analyze(pf)
				if i == 0 && a.Type != wantType[s.ID] {
					t.Errorf("%s: %s detected as %s, want %s", s.ID, f.Path, a.Type, wantType[s.ID])
				}
				ds := append(a.Diagnostics, lint.Run(lint.Input{YAML: pf.YAML, Content: pf.Content, Type: a.Type}, lint.Config{})...)
				for _, d := range lint.Apply(ds, lint.Config{}) {
					t.Errorf("%s (%s) %s:%d: %s %s: %s", s.ID, variant, f.Path, d.Range.Start.Line, d.Severity, d.Code, d.Message)
				}
			}
			if s.ID == "helm-chart" {
				renderChart(t, files)
			}
		}
	}
}

// renderChart renders the generated chart with the Helm SDK and lints
// the output as Kubernetes objects.
func renderChart(t *testing.T, files []scaffold.File) {
	t.Helper()
	var chart []helm.File
	for _, f := range files {
		_, rel, _ := strings.Cut(f.Path, "/")
		chart = append(chart, helm.File{Name: rel, Data: []byte(f.Content)})
	}
	res := helm.Render(context.Background(), chart, helm.Options{})
	for _, d := range res.Diagnostics {
		t.Errorf("helm: %s %s: %s", d.File, d.Code, d.Message)
	}
	if len(res.Docs) != 2 {
		t.Errorf("rendered %d objects, want 2", len(res.Docs))
	}
	y := yamlkit.Parse([]byte(res.Manifest))
	for _, d := range lint.Run(lint.Input{YAML: y, Content: []byte(res.Manifest), Type: "kubernetes"}, lint.Config{}) {
		t.Errorf("rendered chart line %d: %s %s: %s", d.Range.Start.Line, d.Severity, d.Code, d.Message)
	}
	if res.Notes == "" {
		t.Error("no NOTES.txt output")
	}
}

func TestRenderValidates(t *testing.T) {
	s := scaffold.Find(scaffold.Builtin(), "k8s-app")
	_, problems, err := s.Render(map[string]string{"name": "My_App", "port": "70000", "image": "", "host": "not valid!"})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, p := range problems {
		got[p.Field] = p.Message
	}
	for _, f := range []string{"name", "port", "image"} {
		if got[f] == "" {
			t.Errorf("no problem for %s: %v", f, problems)
		}
	}
	if got["host"] != "" {
		t.Errorf("host is hidden (no ingress) but was checked: %s", got["host"])
	}

	// With the ingress on, the host is checked and the Ingress appears.
	files, problems, _ := s.Render(map[string]string{"ingress": "true", "host": "shop.example.com", "namespace": "shop"})
	if len(problems) > 0 {
		t.Fatal(problems)
	}
	if !strings.Contains(files[0].Content, "kind: Ingress") || !strings.Contains(files[0].Content, "host: shop.example.com") ||
		strings.Count(files[0].Content, "namespace: shop") != 3 {
		t.Errorf("ingress variant:\n%s", files[0].Content)
	}
	if files, _, _ := s.Render(nil); strings.Contains(files[0].Content, "Ingress") || strings.Contains(files[0].Content, "\n\n") {
		t.Errorf("default variant:\n%s", files[0].Content)
	}
}

// Personal templates: folders with or without a manifest, and loose
// files whose fields are the placeholders they use.
func TestPersonal(t *testing.T) {
	fsys := fstest.MapFS{
		"job.yaml":                  {Data: []byte("apiVersion: batch/v1\nkind: Job\nmetadata:\n  name: <% .name %>\n<% if .debug %>  labels:\n    debug: \"true\"\n<% end %>spec:\n  backoffLimit: <% .retries | printf \"%s\" %>\n")},
		"svc/starter.yaml":          {Data: []byte("title: Team service\ncategory: Team\nfields:\n  - name: name\n    default: svc\n")},
		"svc/<% .name %>.yaml.tmpl": {Data: []byte("name: <% .name %>\nowner: <% .owner %>\n")},
		"broken/x.tmpl":             {Data: []byte("<% .name ")},
		".git/config":               {Data: []byte("ignored")},
	}
	ss, errs := scaffold.Load(fsys, true)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "broken") {
		t.Errorf("errors: %v", errs)
	}
	job := scaffold.Find(ss, "personal/job.yaml")
	if job == nil || job.Category != "Personal" {
		t.Fatalf("starters: %+v", ss)
	}
	var fields []string
	for _, f := range job.Fields {
		fields = append(fields, f.Name+":"+f.Type)
	}
	if strings.Join(fields, " ") != "name:text debug:bool retries:text" || job.Fields[2].Label != "Retries" {
		t.Errorf("inferred fields: %v", fields)
	}
	files, problems, err := job.Render(map[string]string{"name": "backup", "debug": "true", "retries": "3"})
	if err != nil || len(problems) > 0 {
		t.Fatal(err, problems)
	}
	if want := "apiVersion: batch/v1\nkind: Job\nmetadata:\n  name: backup\n  labels:\n    debug: \"true\"\nspec:\n  backoffLimit: 3\n"; files[0].Content != want || files[0].Path != "job.yaml" {
		t.Errorf("got %s:\n%s", files[0].Path, files[0].Content)
	}
	if _, problems, _ := job.Render(map[string]string{"name": "x"}); len(problems) != 1 || problems[0].Field != "retries" {
		t.Errorf("empty inferred field: %v", problems)
	}

	svc := scaffold.Find(ss, "personal/svc")
	if svc == nil || svc.Title != "Team service" || svc.Category != "Team" || len(svc.Fields) != 2 {
		t.Fatalf("svc: %+v", svc)
	}
	files, _, _ = svc.Render(map[string]string{"owner": "team-a"})
	if files[0].Path != "svc.yaml" || files[0].Content != "name: svc\nowner: team-a\n" {
		t.Errorf("svc: %+v", files)
	}
}

func TestYAMLQuoting(t *testing.T) {
	s, _ := scaffold.Load(fstest.MapFS{"q.yaml": {Data: []byte("v: <% yaml .v %>\n")}}, true)
	for in, want := range map[string]string{
		"hello":         "hello",
		"yes":           `"yes"`,
		"1.27":          `"1.27"`,
		"a: b":          `"a: b"`,
		"# not comment": `"# not comment"`,
		"nginx:1.27":    "nginx:1.27",
		"*":             `"*"`,
	} {
		files, _, _ := s[0].Render(map[string]string{"v": in})
		if got := strings.TrimSuffix(strings.TrimPrefix(files[0].Content, "v: "), "\n"); got != want {
			t.Errorf("%q → %s, want %s", in, got, want)
		}
	}
}
