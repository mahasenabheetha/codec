package argo

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

func index(t *testing.T, names ...string) *Index {
	t.Helper()
	ix := NewIndex()
	for _, n := range names {
		data, err := os.ReadFile(filepath.Join("testdata", n))
		if err != nil {
			t.Fatal(err)
		}
		ix.Add(n, data, yamlkit.Parse(data))
	}
	return ix
}

func TestParseDepends(t *testing.T) {
	for _, tt := range []struct{ in, out, explain string }{
		{"a", "a", "a succeeded (or was skipped)"},
		{"a && b.Failed", "a && b.Failed", "a succeeded (or was skipped) and b failed"},
		{"(a || b) && !c.Skipped", "(a || b) && !c.Skipped", "(a succeeded (or was skipped) or b succeeded (or was skipped)) and not (c skipped)"},
		{"a&&b||c", "a && b || c", "(a succeeded (or was skipped) and b succeeded (or was skipped)) or c succeeded (or was skipped)"},
		{"build.AnySucceeded", "build.AnySucceeded", "any iteration of build succeeded"},
	} {
		d, err := ParseDepends(tt.in)
		if err != nil {
			t.Errorf("%q: %v", tt.in, err)
			continue
		}
		if d.String() != tt.out || d.Explain() != tt.explain {
			t.Errorf("%q: got %q / %q", tt.in, d.String(), d.Explain())
		}
	}
	d, _ := ParseDepends("a && !(b.Failed || c)")
	var terms []string
	for _, term := range d.Terms() {
		terms = append(terms, term.Task+":"+term.Label())
	}
	if !slices.Equal(terms, []string{"a:", "b:not Failed", "c:not Succeeded"}) {
		t.Errorf("terms %v", terms)
	}
	for _, bad := range []string{"", "a &&", "(a || b", "a.Done", "a b", "&& a"} {
		if _, err := ParseDepends(bad); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}

func TestEvalWhen(t *testing.T) {
	for in, want := range map[string]bool{
		"prod == prod":                  true,
		"dev == prod":                   false,
		"false == true":                 false,
		"'a b' != 'a c'":                true,
		"3 > 10":                        false,
		"10 >= 3 && (x == y || !false)": true,
		"release-1.2 =~ '^release-'":    true,
		"main !~ ^release":              true,
	} {
		got, err := EvalWhen(in)
		if err != nil || got != want {
			t.Errorf("%q: got %v, %v", in, got, err)
		}
	}
	for _, bad := range []string{"a ==", "(a == b", "a < b", "prod"} {
		if _, err := EvalWhen(bad); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}

func TestResolve(t *testing.T) {
	ix := index(t, "ci-pipeline.yaml", "lib.yaml")
	res := Resolve(ix, ix.Find("ci-pipeline"), Options{})
	if len(res.Problems) != 0 {
		t.Errorf("problems: %+v", res.Problems)
	}
	param := map[string]Value{}
	for _, v := range res.Params {
		param[v.Name] = v
	}
	if v := param["image"]; v.Text != "registry.example.com/app:main" || v.State != StateKnown {
		t.Errorf("image: %+v", v)
	}

	node := func(id string) *Node {
		n := res.Node(id)
		if n == nil {
			t.Fatalf("no node %s", id)
		}
		return n
	}
	input := func(n *Node, name string) Value {
		for _, v := range n.Inputs {
			if v.Name == name {
				return v
			}
		}
		t.Fatalf("%s: no input %s", n.ID, name)
		return Value{}
	}
	// withItems: one node per item, with {{item.x}} resolved.
	b := node("main/build(1:os:darwin,arch:arm64)")
	if v := input(b, "target"); v.Text != "darwin-arm64" {
		t.Errorf("build target: %+v", v)
	}
	if v := input(b, "flags"); v.Text != "-trimpath" || !strings.Contains(v.From, "default") {
		t.Errorf("default input: %+v", v)
	}
	// Runtime values stay as written, marked.
	if v := input(node("main/diagnose"), "from"); v.State != StateRuntime || v.Text != "{{tasks.test.outputs.parameters.report}}" {
		t.Errorf("runtime value: %+v", v)
	}
	// when is evaluated once its values are known.
	if w := node("main/publish").When; w == nil || w.Value.Text != "dev == prod" || w.Result != "false" {
		t.Errorf("when: %+v", w)
	}
	// templateRef: the template runs with its own spec's templates, and
	// its inputs are resolved through the chain.
	if v := input(node("main/publish/sign"), "ref"); v.Text != "registry.example.com/app:main.sig" {
		t.Errorf("stitched value: %+v", v)
	}
	if v := input(node("onExit:report/notify"), "channel"); v.Text != "#builds-dev" {
		t.Errorf("workflow.parameters inside a templateRef: %+v", v)
	}
	if n := node("main/scan"); n.Type != "container" || n.Ref != "cluster-tools/scan" {
		t.Errorf("cluster template: %+v", n)
	}

	edges := map[string]bool{}
	for _, e := range node("main").Edges {
		edges[strings.TrimPrefix(e.From, "main/")+" -"+e.Label+"-> "+strings.TrimPrefix(e.To, "main/")] = true
	}
	for _, e := range []string{
		"checkout --> build(0:os:linux,arch:amd64)",
		"checkout --> lint",
		"lint --> test",
		"test -Succeeded-> publish",
		"test -Failed-> diagnose",
		"lint -Failed-> diagnose",
		"build(1:os:darwin,arch:arm64) -AnySucceeded-> scan",
		"diagnose -not Succeeded-> scan",
	} {
		if !edges[e] {
			t.Errorf("missing edge %s (have %v)", e, edges)
		}
	}
	// steps: every step of group 1 comes before every step of group 2.
	pub := node("main/publish")
	if len(pub.Edges) != 2 || node("main/publish/announce").Group != 2 {
		t.Errorf("steps edges %+v", pub.Edges)
	}

	// Parameters set by the user win; enums are checked.
	res = Resolve(ix, ix.Find("ci-pipeline"), Options{Params: map[string]string{"env": "prod", "branch": "fix-1"}})
	if w := res.Node("main/publish").When; w.Result != "true" {
		t.Errorf("when with env=prod: %+v", w)
	}
	if v := res.Params[2]; v.Text != "registry.example.com/app:fix-1" {
		t.Errorf("image with branch set: %+v", v)
	}
	res = Resolve(ix, ix.Find("ci-pipeline"), Options{Params: map[string]string{"env": "qa"}})
	if len(res.Problems) != 1 || res.Problems[0].Code != "parameter-enum" {
		t.Errorf("enum: %+v", res.Problems)
	}
}

func TestResolveTriggers(t *testing.T) {
	ix := index(t, "triggers.yaml", "ci-pipeline.yaml", "lib.yaml")
	var sensor *Spec
	for _, s := range ix.Specs {
		if s.Via != nil {
			sensor = s
		}
	}
	res := Resolve(ix, sensor, Options{})
	if res.Base == nil || res.Base.Name != "ci-pipeline" {
		t.Fatalf("base: %+v", res.Base)
	}
	branch, env := res.Params[0], res.Params[1]
	if branch.State != StateRuntime || !strings.Contains(branch.From, "push-sensor") || branch.Hint != "the sensor's filter accepts main | release" {
		t.Errorf("event parameter: %+v", branch)
	}
	if env.Text != "staging" {
		t.Errorf("the workflow's arguments override the template's: %+v", env)
	}
	if v := res.Node("main/checkout").Inputs[0]; v.State != StateRuntime {
		t.Errorf("event data flows into inputs: %+v", v)
	}

	cron := Resolve(ix, ix.Find("CronWorkflow/nightly"), Options{})
	if cron.Workflow.Schedule != "0 2 * * *" || cron.Params[1].Text != "prod" || cron.Node("main/publish").When.Result != "true" {
		t.Errorf("cron: %+v", cron.Params)
	}
}

func TestResolveHelmTemplate(t *testing.T) {
	// A raw Helm template: Argo expressions escaped for Helm resolve,
	// Go template expressions are marked "render the chart".
	src := "apiVersion: argoproj.io/v1alpha1\nkind: WorkflowTemplate\nmetadata:\n  name: t\nspec:\n  entrypoint: main\n" +
		"  arguments:\n    parameters:\n      - name: env\n        value: {{ .Values.env | quote }}\n" +
		"  templates:\n    - name: main\n      steps:\n        - - name: s\n            template: leaf\n            arguments:\n              parameters:\n" +
		"                - name: a\n                  value: {{ `\"{{ workflow.parameters.env }}\"` }}\n" +
		"                - name: b\n                  value: \"x-{{ \"{{\" }}workflow.name{{ \"}}\" }}\"\n" +
		"    - name: leaf\n      inputs:\n        parameters: [{name: a}, {name: b}]\n      container: {image: busybox}\n"
	ix := NewIndex()
	ix.Add("t.yaml", []byte(src), yamlkit.Parse([]byte(src)))
	res := Resolve(ix, ix.Specs[0], Options{})
	in := res.Node("main/s").Inputs
	if in[0].State != StateTemplate || in[0].Text != `{{ .Values.env | quote }}` {
		t.Errorf("a: %+v", in[0])
	}
	if in[1].State != StateRuntime || in[1].Text != "x-{{workflow.name}}" {
		t.Errorf("b: %+v", in[1])
	}
}

func TestCheck(t *testing.T) {
	data, _ := os.ReadFile("testdata/broken.yaml")
	f := yamlkit.Parse(data)
	var got []string
	for _, d := range Check(f, Read("broken.yaml", data, f)) {
		got = append(got, d.Code+" "+strings.SplitN(d.Message, " ", 2)[0]+" "+d.Hint)
	}
	want := []string{
		"argo-template-missing The Did you mean main?",
		"argo-argument-unused Template Argo ignores it. Check the spelling, or declare the input.",
		"argo-template-missing No Did you mean echo?",
		`argo-depends depends: Write task names joined by && and ||, e.g. "a && (b.Failed || c.Skipped)".`,
		"argo-unknown-task e ",
		"argo-input-missing f Add it under arguments.parameters, or give the input a default.",
		"argo-unknown-task zz ",
		"argo-parameter-undeclared broken Declare it under spec.arguments.parameters.",
		"argo-input-undeclared Template Declare it under inputs.parameters, or fix the name.",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}

	// The fixtures that are fine stay quiet, templateRefs included.
	ix := index(t, "ci-pipeline.yaml", "lib.yaml", "triggers.yaml")
	for _, name := range []string{"ci-pipeline.yaml", "lib.yaml", "triggers.yaml"} {
		data, _ := os.ReadFile(filepath.Join("testdata", name))
		f := yamlkit.Parse(data)
		specs := Read(name, data, f)
		if ds := append(Check(f, specs), CheckRefs(specs, ix)...); len(ds) > 0 {
			t.Errorf("%s: %+v", name, ds)
		}
	}
	data, _ = os.ReadFile("testdata/ci-pipeline.yaml")
	f = yamlkit.Parse(data)
	ds := CheckRefs(Read("ci-pipeline.yaml", data, f), NewIndex())
	if len(ds) != 3 || ds[0].Code != "argo-ref-missing" || ds[0].Severity != yamlkit.SeverityWarning {
		t.Errorf("refs outside the folder: %+v", ds)
	}
}

func TestProvider(t *testing.T) {
	data, _ := os.ReadFile("testdata/ci-pipeline.yaml")
	f := &provider.File{Path: "ci-pipeline.yaml", YAML: yamlkit.Parse(data), Content: data}
	p := provider.Default.Best(f)
	if _, ok := p.(Provider); !ok {
		t.Fatalf("the Argo lens isn't registered: %T", p)
	}
	lineCol := func(line int, text string) yamlkit.Pos {
		l := strings.Split(string(data), "\n")[line-1]
		return yamlkit.PosAt(data, line, strings.Index(l, text)+2)
	}
	for _, tt := range []struct {
		line   int
		at     string
		target int // line of the definition
	}{
		{7, "main", 20},                        // entrypoint
		{24, "git-clone", 72},                  // template: name
		{31, "checkout", 23},                   // depends term
		{48, "lint", 39},                       // depends term after &&
		{28, "workflow.parameters.branch", 12}, // workflow parameter
		{89, "inputs.parameters.target", 82},   // input in a script
	} {
		locs := provider.DefinitionAt(p, f, lineCol(tt.line, tt.at))
		if len(locs) != 1 || locs[0].Range.Start.Line != tt.target {
			t.Errorf("line %d %q: got %+v, want line %d", tt.line, tt.at, locs, tt.target)
		}
	}
	// templateRef jumps into the other file with an index.
	ix := index(t, "ci-pipeline.yaml", "lib.yaml")
	locs := DefinitionIn(f, lineCol(51, "shared-steps"), ix)
	if len(locs) != 1 || locs[0].Path != "lib.yaml" || locs[0].Range.Start.Line != 18 {
		t.Errorf("templateRef: %+v", locs)
	}
	if h := HoverIn(f, lineCol(52, "publish"), ix); h == nil || !strings.Contains(h.Title, "shared-steps · publish") || !strings.Contains(h.Code, "registry = registry.example.com") {
		t.Errorf("templateRef hover: %+v", h)
	}
	if h := provider.HoverAt(p, f, lineCol(48, "lint")); h == nil || h.Rows[0].Value != "build succeeded (or was skipped) and lint succeeded (or was skipped)" {
		t.Errorf("depends hover: %+v", h)
	}

	var names []string
	for _, s := range p.Symbols(f.YAML.Docs[0]) {
		if s.Name == "spec" {
			for _, c := range s.Children {
				if c.Name == "templates" {
					for _, t := range c.Children {
						names = append(names, t.Name+" = "+t.Detail)
					}
				}
			}
		}
	}
	if len(names) != 6 || names[0] != "Template main = dag · entrypoint" || names[5] != "Template report = steps · exit handler" {
		t.Errorf("outline: %q", names)
	}
}

func TestApps(t *testing.T) {
	data, _ := os.ReadFile("testdata/apps.yaml")
	apps := Apps("apps.yaml", yamlkit.Parse(data))
	if len(apps) != 2 {
		t.Fatalf("apps: %+v", apps)
	}
	a := apps[0]
	s := a.Sources[0]
	if s.Tool != "helm" || s.Path != "charts/web" || s.Helm.Release != "web" || len(s.Helm.ValueFiles) != 2 ||
		s.Helm.Parameters[0] != "image.tag=1.27" || s.Helm.Values != "replicaCount: 3\n" ||
		a.Destination.Namespace != "web" || a.Sync != "automated (prune, selfHeal)" {
		t.Errorf("application: %+v %+v", a, s.Helm)
	}
	set := apps[1]
	if set.Sources[0].Tool != "kustomize" || len(set.Generators) != 1 || set.Generators[0] != "matrix: git directories in https://git.example.com/deploy.git × list (2 elements)" {
		t.Errorf("applicationset: %+v", set)
	}
}

func TestMermaid(t *testing.T) {
	ix := index(t, "ci-pipeline.yaml", "lib.yaml")
	res := Resolve(ix, ix.Find("ci-pipeline"), Options{})
	out := Mermaid(res.Container(""), res)
	for _, want := range []string{"flowchart LR", `n5[["publish<br/><small>shared-steps/publish · steps</small>"]]`, "n4 -->|Failed| n6"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	if !strings.Contains(Dot(res.Container("publish"), res), `"main/publish/push" -> "main/publish/sign"`) {
		t.Error("dot: steps edges")
	}
}
