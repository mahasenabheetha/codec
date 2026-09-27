package ci

import (
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/mahasenabheetha/codec/v2/internal/provider"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// analyze reads testdata/<dir>/<file> with the folder as the workspace.
func analyze(t *testing.T, tool, dir, file string, params map[string]string) *Pipeline {
	t.Helper()
	fsys := os.DirFS("testdata/" + dir)
	content, err := fs.ReadFile(fsys, file)
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	opts := Options{Load: func(p string) ([]byte, error) { return fs.ReadFile(fsys, p) }, Files: files, Params: params}
	return Analyze(tool, file, content, yamlkit.Parse(content), opts)
}

// effective renders a job's effective config as "text  ← from" lines.
func effective(j *Job) string {
	var b strings.Builder
	for _, l := range j.Effective {
		b.WriteString(l.Text)
		if l.From != "" {
			b.WriteString("  ← " + l.From)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func edges(p *Pipeline) []string {
	var out []string
	for _, e := range p.Edges {
		out = append(out, e.From+" → "+e.To+" ("+e.Label+")")
	}
	return out
}

func problems(p *Pipeline) []string {
	var out []string
	for _, pr := range p.Problems {
		out = append(out, string(pr.Severity)+" "+pr.Code+": "+pr.Message)
	}
	return out
}

func TestGitLabEffective(t *testing.T) {
	p := analyze(t, GitLab, "gitlab", ".gitlab-ci.yml", nil)
	build := p.Job("build")
	if build == nil {
		t.Fatalf("no build job; jobs: %v", p.Jobs)
	}
	// extends .base, then the job's own keys (with the << anchor merged
	// in, shallowly: the job's own variables replace the anchor's),
	// then default: and global variables.
	want := `retry: 2
stage: build
script:
  - echo setup  ← !reference .setup
  - git fetch  ← !reference .setup
  - make build
variables:
  CGO_ENABLED: "0"  ← extends .base
  REGION: us
  GLOBAL: "1"  ← global variables
image: golang:1.26  ← extends .base
before_script:  ← extends .base
  - go version  ← extends .base
tags:  ← default
  - docker  ← default
`
	if got := sortedEffective(build); got != sortLines(want) {
		t.Errorf("build effective:\n%s\nwant (any order):\n%s", effective(build), want)
	}
	if !slices.Equal(build.Extends, []string{".base"}) {
		t.Errorf("build extends = %v", build.Extends)
	}
	// The script line from .setup points into the included file.
	for _, l := range build.Effective {
		if strings.Contains(l.Text, "git fetch") && (l.Source == nil || l.Source.File != "ci/templates.yml" || l.Source.Line != 18) {
			t.Errorf("git fetch source = %+v, want ci/templates.yml:18", l.Source)
		}
	}

	test := p.Job("test")
	if got := effective(test); !strings.Contains(got, "TEST: \"1\"  ← extends .tester") || !strings.Contains(got, "REGION: base  ← extends .base") {
		t.Errorf("test effective:\n%s", got)
	}
	if !slices.Equal(test.Instances, []string{"test: [1.25, linux]", "test: [1.26, linux]"}) {
		t.Errorf("test instances = %v", test.Instances)
	}
	deploy := p.Job("deploy")
	if got := effective(deploy); strings.Contains(got, "alpine") || strings.Contains(got, "docker") {
		t.Errorf("deploy has inherit: default: false, got defaults:\n%s", got)
	}
	if deploy.When != "manual" || deploy.Environment != "production" {
		t.Errorf("deploy when/env = %q/%q", deploy.When, deploy.Environment)
	}
	lint := p.Job("lint")
	if !strings.Contains(effective(lint), "image: golangci/golangci-lint\n") {
		t.Errorf("lint keeps its own image:\n%s", effective(lint))
	}

	// Execution order: stages unless needs: says otherwise; lint
	// (needs: []) starts at once; notify (.post) waits for what isn't
	// already waited for.
	wantEdges := []string{
		"build → test (stage)",
		"build → deploy (needs)",
		"lint → deploy (optional)",
		"test → notify (stage)",
		"deploy → notify (stage)",
	}
	if got := edges(p); !slices.Equal(got, wantEdges) {
		t.Errorf("edges:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(wantEdges, "\n"))
	}
	var stages []string
	for _, s := range p.Stages {
		stages = append(stages, s.Name+":"+strings.Join(s.Jobs, ","))
	}
	if want := []string{"build:build", "test:test,lint", "deploy:deploy", ".post:notify"}; !slices.Equal(stages, want) {
		t.Errorf("stages = %v, want %v", stages, want)
	}
	// .from_project may come from the project include: worth knowing only.
	if got := problems(p); len(got) != 1 || !strings.HasPrefix(got[0], "info gitlab-extends-unknown") {
		t.Errorf("problems = %v", got)
	}
	var kinds []string
	for _, i := range p.Includes {
		kinds = append(kinds, i.Kind+":"+i.State)
	}
	if want := []string{"local:resolved", "project:unresolved"}; !slices.Equal(kinds, want) {
		t.Errorf("includes = %v", kinds)
	}
}

func sortedEffective(j *Job) string { return sortLines(effective(j)) }

func sortLines(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	slices.Sort(lines)
	return strings.Join(lines, "\n")
}

func TestGitLabProblems(t *testing.T) {
	p := analyze(t, GitLab, "gitlab", "broken.gitlab-ci.yml", nil)
	got := strings.Join(problems(p), "\n")
	for _, want := range []string{
		`error gitlab-stage-undefined: Job build uses stage "biuld", which isn't in stages`,
		"error gitlab-extends-unknown: test extends .nothere, which isn't defined",
		"error gitlab-needs-unknown: Job test needs bild, which isn't defined",
		"error gitlab-reference: !reference points to .missing, which isn't defined",
		"error gitlab-extends-cycle:",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	for _, pr := range p.Problems {
		if pr.Code == "gitlab-stage-undefined" && pr.Message != "" && !strings.Contains(pr.Hint, `"build"`) {
			t.Errorf("stage hint = %q", pr.Hint)
		}
	}
}

func TestGitHubMatrix(t *testing.T) {
	// The example from GitHub's documentation on include.
	p := analyze(t, GitHub, "github", ".github/workflows/ci.yml", nil)
	test := p.Job("test")
	if test == nil || test.Matrix == nil {
		t.Fatalf("no test matrix")
	}
	var got []string
	for _, c := range test.Matrix.Combos {
		var kv []string
		for _, v := range c.Values {
			kv = append(kv, v.Key+"="+v.Value)
		}
		got = append(got, strings.Join(kv, " "))
	}
	want := []string{
		"fruit=apple animal=cat color=pink shape=circle",
		"fruit=apple animal=dog color=green shape=circle",
		"fruit=pear animal=cat color=pink",
		"fruit=pear animal=dog color=green",
		"fruit=banana",
		"fruit=banana animal=cat",
	}
	if !slices.Equal(got, want) {
		t.Errorf("combos:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if n := test.Matrix.Combos[0].Name; n != "test (apple, cat, pink, circle)" {
		t.Errorf("name = %q", n)
	}
}

func TestMatrixExclude(t *testing.T) {
	m := yamlkit.Parse([]byte("os: [linux, windows]\ngo: ['1.25', '1.26']\nexclude:\n  - os: windows\n    go: '1.25'\n  - arch: arm\n")).Docs[0].Root
	mx, errs := expandMatrix(m, "build")
	var names []string
	for _, c := range mx.Combos {
		names = append(names, c.Name)
	}
	if want := []string{"build (linux, 1.25)", "build (linux, 1.26)", "build (windows, 1.26)"}; !slices.Equal(names, want) {
		t.Errorf("combos = %v", names)
	}
	if len(errs) != 1 || !strings.Contains(errs[0], `"arch"`) {
		t.Errorf("errs = %v", errs)
	}
	// Only include: each entry is a job.
	m = yamlkit.Parse([]byte("include:\n  - os: linux\n  - os: mac\n")).Docs[0].Root
	mx, _ = expandMatrix(m, "b")
	if len(mx.Combos) != 2 {
		t.Errorf("include-only combos = %v", mx.Combos)
	}
	// An expression matrix is known at run time.
	m = yamlkit.Parse([]byte("x: ${{ fromJSON(needs.a.outputs.m) }}")).Docs[0].Root
	mx, _ = expandMatrix(m.Get("x"), "b")
	if mx.Runtime == "" {
		t.Errorf("runtime not set")
	}
}

func TestGitHubWorkflow(t *testing.T) {
	p := analyze(t, GitHub, "github", ".github/workflows/ci.yml", nil)
	var ids []string
	for _, j := range p.Jobs {
		ids = append(ids, j.ID)
	}
	// The reusable workflow's jobs take the caller's place.
	if want := []string{"test", "release/build", "release/publish", "notify"}; !slices.Equal(ids, want) {
		t.Errorf("jobs = %v", ids)
	}
	wantEdges := []string{
		"test → release/build (needs)",
		"release/build → release/publish (needs)",
		"release/publish → notify (needs)",
	}
	if got := edges(p); !slices.Equal(got, wantEdges) {
		t.Errorf("edges = %v", got)
	}
	got := strings.Join(problems(p), "\n")
	for _, want := range []string{
		"error gh-input-missing: Job release doesn't pass required input dry",
		"error gh-input-unknown: ./.github/workflows/release.yml has no input extra",
		"warning gh-step-unknown: steps.biuld: no step in job test has id biuld",
		"warning gh-matrix-unknown: matrix.colour",
		"error gh-needs-context: needs.test: job notify doesn't need test",
		"warning gh-input-undeclared: inputs.envv isn't declared",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "needs.x") || strings.Contains(got, "matrix.fruit") || strings.Contains(got, "inputs.env ") {
		t.Errorf("false positive in:\n%s", got)
	}
	// The composite action's steps are inside its step.
	setup := p.Job("test").Steps[2]
	if setup.Note != "composite action" || len(setup.Children) != 1 || setup.Children[0].Detail != "go version" {
		t.Errorf("setup step = %+v", setup)
	}
	rel := p.Job("release/build")
	if rel.Group != "release" {
		t.Errorf("group = %q", rel.Group)
	}
}

func TestAzureTemplates(t *testing.T) {
	p := analyze(t, Azure, "azure", "azure-pipelines.yml", nil)
	var ids []string
	for _, j := range p.Jobs {
		ids = append(ids, j.ID)
	}
	if want := []string{"Build.compile", "Build.package", "Deploy.deploy_eu", "Deploy.deploy_us"}; !slices.Equal(ids, want) {
		t.Fatalf("jobs = %v", ids)
	}
	var steps []string
	for _, s := range p.Job("Build.compile").Steps {
		steps = append(steps, s.Name)
	}
	// The template's steps with its parameters; the stepList parameter
	// inserted; eq('$(config)', 'Debug') is false at compile time.
	if want := []string{"Build $(config)", "echo extra", "Test"}; !slices.Equal(steps, want) {
		t.Errorf("compile steps = %v", steps)
	}
	if s := p.Job("Build.compile").Steps[0]; s.From != "templates/build-steps.yml" {
		t.Errorf("step from = %q", s.From)
	}
	if s := p.Job("Build.compile").Steps[1]; s.From != "" {
		t.Errorf("parameter step came from the caller, got from = %q", s.From)
	}
	dep := p.Job("Deploy.deploy_eu")
	if dep.Kind != "deployment" || dep.Environment != "dev" || len(dep.Steps) != 1 || dep.Steps[0].Children[0].Detail != "./deploy.sh eu" {
		t.Errorf("deploy_eu = %+v", dep)
	}
	wantEdges := []string{
		"Build.compile → Build.package (dependsOn)",
		"Build.package → Deploy.deploy_eu (stage)",
		"Build.package → Deploy.deploy_us (stage)",
	}
	if got := edges(p); !slices.Equal(got, wantEdges) {
		t.Errorf("edges = %v", got)
	}
	if got := problems(p); len(got) != 1 || !strings.Contains(got[0], "azure-template-missing") {
		t.Errorf("problems = %v", got)
	}
	// What-if parameters: runTests false drops the Test step.
	p = analyze(t, Azure, "azure", "azure-pipelines.yml", map[string]string{"runTests": "false", "env": "prod"})
	if n := len(p.Job("Build.compile").Steps); n != 2 {
		t.Errorf("with runTests=false: %d steps", n)
	}
	if env := p.Job("Deploy.deploy_us").Environment; env != "prod" {
		t.Errorf("env = %q", env)
	}
	var vars []string
	for _, v := range p.Variables {
		vars = append(vars, v.Name+"="+v.Value)
	}
	if want := []string{"config=Release", "shared-secrets=(variable group)"}; !slices.Equal(vars, want) {
		t.Errorf("vars = %v", vars)
	}
}

func TestAzureEval(t *testing.T) {
	sc := &azScope{
		params: map[string]*yamlkit.Node{"env": scalarNode("Prod"), "list": yamlkit.Parse([]byte("[a, b]")).Docs[0].Root},
		vars:   map[string]string{"config": "Release"},
	}
	for expr, want := range map[string]string{
		"eq(parameters.env, 'prod')":                   "true",
		"ne(parameters.env, 'prod')":                   "false",
		"and(true, eq(variables.config, 'Release'))":   "true",
		"or(eq(variables['Build.Reason'], 'x'), true)": "true",
		"in(parameters.env, 'dev', 'prod')":            "true",
		"format('{0}-{1}', parameters.env, 'x')":       "Prod-x",
		"length(parameters.list)":                      "2",
		"join(',', parameters.list)":                   "a,b",
		"parameters.list[1]":                           "b",
		"startsWith('refs/heads/main', 'refs/heads')":  "true",
		"not(contains('abc', 'd'))":                    "true",
		"'it''s'":                                      "it's",
	} {
		v := azEval(expr, sc)
		if !v.known || v.str() != want {
			t.Errorf("%s = %q (known %v), want %q", expr, v.str(), v.known, want)
		}
	}
	for _, expr := range []string{"eq(variables['Build.Reason'], 'x')", "counter('a', 0)", "eq(parameters.env"} {
		if v := azEval(expr, sc); v.known {
			t.Errorf("%s should be unknown, got %q", expr, v.str())
		}
	}
}

func TestGhRefs(t *testing.T) {
	refs := ghRefs(" needs.build.outputs.v && env['HOME'] || 'steps.x' || contains(github.ref, 'x') ")
	var got []string
	for _, r := range refs {
		got = append(got, r.ctx+":"+strings.Join(r.path, "."))
	}
	if want := []string{"needs:build.outputs.v", "env:HOME", "github:ref"}; !slices.Equal(got, want) {
		t.Errorf("refs = %v", got)
	}
}

func TestExpressions(t *testing.T) {
	src := []byte("steps:\n  - script: echo $(config) $[ variables.x ]\n    condition: ${{ parameters.y }}\n")
	f := yamlkit.Parse(src)
	var got []string
	for _, e := range Expressions(Azure, src, f) {
		got = append(got, e.Syntax+":"+string(e.Phase)+":"+e.Text)
	}
	slices.Sort(got)
	want := []string{"azure-macro:runtime:$(config)", "azure-runtime:runtime:$[ variables.x ]", "azure:template:${{ parameters.y }}"}
	if !slices.Equal(got, want) {
		t.Errorf("expressions = %v", got)
	}
}

// posOf is the position of the n-th occurrence of needle (plus off).
func posOf(t *testing.T, content []byte, needle string, off int) yamlkit.Pos {
	t.Helper()
	i := strings.Index(string(content), needle)
	if i < 0 {
		t.Fatalf("%q not found", needle)
	}
	i += off
	line := strings.Count(string(content[:i]), "\n") + 1
	col := i - strings.LastIndex(string(content[:i]), "\n")
	return yamlkit.PosAt(content, line, col)
}

func TestEditor(t *testing.T) {
	type tc struct {
		tool, dir, file, needle string
		off                     int
		title, def              string // hover title; definition "file:line"
	}
	for _, c := range []tc{
		{GitLab, "gitlab", ".gitlab-ci.yml", "extends: .base", 10, "Template .base", "ci/templates.yml:1"},
		{GitLab, "gitlab", ".gitlab-ci.yml", "!reference [.setup", 12, "Template .setup", "ci/templates.yml:15"},
		{GitLab, "gitlab", ".gitlab-ci.yml", "needs: [build", 9, "Job build", ":21"},
		{GitLab, "gitlab", ".gitlab-ci.yml", "local: ci/templates.yml", 9, "File ci/templates.yml", "ci/templates.yml:1"},
		{GitHub, "github", ".github/workflows/ci.yml", "steps.build.outputs.version", 7, "Step build", ":34"},
		{GitHub, "github", ".github/workflows/ci.yml", "matrix.fruit", 8, "Matrix value fruit", ":19"},
		{GitHub, "github", ".github/workflows/ci.yml", "needs.test.outputs.version", 20, "Output version of job test", ":31"},
		{GitHub, "github", ".github/workflows/ci.yml", "inputs.env }}", 8, "Input env", ":7"},
		{GitHub, "github", ".github/workflows/ci.yml", "uses: ./.github/workflows/release.yml", 8, "File .github/workflows/release.yml", ".github/workflows/release.yml:1"},
		{GitHub, "github", ".github/workflows/ci.yml", "uses: ./.github/actions/setup", 8, "Action .github/actions/setup", ".github/actions/setup/action.yml:1"},
		{Azure, "azure", "azure-pipelines.yml", "$(config)", 3, "Variable config", ":11"},
		{Azure, "azure", "azure-pipelines.yml", "parameters.runTests", 12, "Parameter runTests", ":7"},
		{Azure, "azure", "azure-pipelines.yml", "templates/build-steps.yml", 3, "File templates/build-steps.yml", "templates/build-steps.yml:1"},
		{Azure, "azure", "azure-pipelines.yml", "dependsOn: compile", 12, "Job compile", ":19"},
	} {
		t.Run(c.needle, func(t *testing.T) {
			fsys := os.DirFS("testdata/" + c.dir)
			content, _ := fs.ReadFile(fsys, c.file)
			f := &provider.File{Path: c.file, YAML: yamlkit.Parse(content), Content: content}
			opts := Options{Load: func(p string) ([]byte, error) { return fs.ReadFile(fsys, p) }}
			pos := posOf(t, content, c.needle, c.off)
			h := HoverIn(c.tool, f, pos, opts)
			if h == nil || h.Title != c.title {
				t.Errorf("hover = %+v, want title %q", h, c.title)
			}
			locs := DefinitionIn(c.tool, f, pos, opts)
			if len(locs) != 1 {
				t.Fatalf("definition = %v", locs)
			}
			if got := fmt.Sprintf("%s:%d", locs[0].Path, locs[0].Range.Start.Line); got != c.def {
				t.Errorf("definition = %s, want %s", got, c.def)
			}
		})
	}
}

func TestOutline(t *testing.T) {
	content, _ := os.ReadFile("testdata/gitlab/.gitlab-ci.yml")
	f := yamlkit.Parse(content)
	syms := Provider{tool: GitLab}.Symbols(f.Docs[0])
	var names []string
	for _, s := range syms {
		names = append(names, s.Name)
	}
	got := strings.Join(names, ",")
	for _, want := range []string{"Job build", "Job deploy", "Template .defaults"} {
		if !strings.Contains(got, want) {
			t.Errorf("outline %s lacks %q", got, want)
		}
	}
}
