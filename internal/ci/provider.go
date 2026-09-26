package ci

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Provider is the CI lens for one tool in the provider registry: the
// built-in detection, plus an outline that names jobs and stages,
// diagnostics, hover and go to definition for the names pipelines refer
// to (needs, extends, !reference, uses:, template:, dependsOn,
// expression contexts), and the tool's expression syntax.
type Provider struct {
	provider.Provider
	tool string
}

// Register installs the lenses in reg, replacing the built-in
// detectors of the three tools.
func Register(reg *provider.Registry) {
	for _, id := range []string{GitHub, GitLab, Azure} {
		if base := reg.Lookup(id); base != nil {
			if _, done := base.(Provider); !done {
				reg.Register(Provider{base, id})
			}
		}
	}
}

func init() { Register(provider.Default) }

// Symbols names jobs, templates, stages and steps.
func (p Provider) Symbols(d *yamlkit.Document) []provider.Symbol {
	syms := provider.Outline(d)
	if d == nil || d.Root == nil {
		return syms
	}
	pl := Analyze(p.tool, "", nil, &yamlkit.File{Docs: []*yamlkit.Document{d}}, Options{})
	for _, j := range pl.Jobs {
		if j.Group != "" || j.Source.File != "" {
			continue
		}
		s := symbolAt(syms, j.Source.Range)
		if s == nil {
			continue
		}
		s.Name = "Job " + j.Name
		var detail []string
		if pl.Tool == GitLab && j.Stage != "" {
			detail = append(detail, j.Stage)
		}
		if c := j.Caption(); c != "" {
			detail = append(detail, c)
		}
		if len(j.Needs) > 0 {
			var names []string
			for _, n := range j.Needs {
				names = append(names, strings.TrimPrefix(n.Job, j.Stage+"."))
			}
			detail = append(detail, "needs "+strings.Join(names, ", "))
		}
		if len(j.Extends) > 0 {
			detail = append(detail, "extends "+strings.Join(j.Extends, ", "))
		}
		s.Detail = strings.Join(detail, " · ")
		labelSteps(s, j.Steps)
	}
	for _, t := range pl.Templates {
		if s := symbolAt(syms, t.Source.Range); s != nil && t.Source.File == "" {
			s.Name = "Template " + t.Name
		}
	}
	for _, st := range pl.Stages {
		if st.Source == nil || st.Source.File != "" {
			continue
		}
		if s := symbolAt(syms, st.Source.Range); s != nil {
			s.Name = "Stage " + st.Name
			s.Detail = plural(len(st.Jobs), "job")
		}
	}
	return syms
}

func labelSteps(parent *provider.Symbol, steps []Step) {
	for _, st := range steps {
		if st.Source.File != "" {
			continue
		}
		if s := symbolAt(parent.Children, st.Source.Range); s != nil && st.Kind != "section" && st.Kind != "script" {
			s.Name = "Step " + st.Name
			s.Detail = st.Kind
			if st.Detail != "" && st.Detail != st.Name {
				s.Detail += " · " + st.Detail
			}
		}
	}
}

// symbolAt finds the symbol starting where r starts, depth first.
func symbolAt(syms []provider.Symbol, r yamlkit.Range) *provider.Symbol {
	for i := range syms {
		s := &syms[i]
		if s.Range.Start == r.Start {
			return s
		}
		if !s.Range.Contains(r.Start) {
			continue
		}
		if found := symbolAt(s.Children, r); found != nil {
			return found
		}
	}
	return nil
}

// Diagnostics reports what the file alone can tell; the web server
// replaces them with Diagnose over the whole repository.
func (p Provider) Diagnostics(f *provider.File) []yamlkit.Diagnostic {
	return Diagnose(Analyze(p.tool, f.Path, f.Content, f.YAML, Options{}), f.Path)
}

// Expressions tints the tool's own expression syntax.
func (p Provider) Expressions(f *provider.File) []yamlkit.Expression {
	return Expressions(p.tool, f.Content, f.YAML)
}

// Diagnose turns a pipeline's problems located in file into editor
// diagnostics.
func Diagnose(pl *Pipeline, file string) []yamlkit.Diagnostic {
	var out []yamlkit.Diagnostic
	for _, pr := range pl.Problems {
		if pr.Source == nil || pr.Source.Line == 0 || (pr.Source.File != "" && pr.Source.File != file) {
			continue
		}
		out = append(out, yamlkit.Diagnostic{Severity: pr.Severity, Code: pr.Code, Message: pr.Message, Hint: pr.Hint, Source: "ci", Range: pr.Source.Range})
	}
	return out
}

// Hover explains what the cursor is on, within the file.
func (p Provider) Hover(f *provider.File, pos yamlkit.Pos) *provider.Hover {
	return HoverIn(p.tool, f, pos, Options{})
}

// Definition jumps to what the cursor refers to, within the file.
func (p Provider) Definition(f *provider.File, pos yamlkit.Pos) []provider.Location {
	return DefinitionIn(p.tool, f, pos, Options{})
}

// --- what the cursor is on ---

// target is a name the cursor is on, and what kind of thing it names.
type target struct {
	kind  string // job, template, file, input, env, secret, matrix, output, step, context, param, var, stage
	name  string
	sub   string // an output name, a context property
	job   *Job   // the job the cursor is in
	rng   yamlkit.Range
	where *Source // where it is defined
	pl    *Pipeline
}

var (
	azParamRe = regexp.MustCompile(`parameters\.([\w-]+)|parameters\['([^']+)'\]`)
	azVarRe   = regexp.MustCompile(`variables\.([\w.-]+)|variables\['([^']+)'\]`)
)

// at works out what the cursor is on and where that is defined.
func at(tool string, f *provider.File, pos yamlkit.Pos, opts Options) *target {
	if f.YAML == nil {
		return nil
	}
	pl := Analyze(tool, f.Path, f.Content, f.YAML, opts)
	t := &target{pl: pl}
	for _, j := range pl.Jobs {
		if j.Group == "" && (j.Source.File == f.Path || j.Source.File == "") && j.node != nil && (j.Source.Range.Contains(pos) || j.node.Range.Contains(pos)) {
			t.job = j
		}
	}
	for _, e := range Expressions(tool, f.Content, f.YAML) {
		if !e.Range.Contains(pos) {
			continue
		}
		off := pos.Offset - e.Range.Start.Offset
		switch e.Syntax {
		case "github":
			for _, r := range ghRefs(e.Text) {
				if off >= r.start && off <= r.end {
					t.rng = e.Range
					return t.ghContext(f, pos, r)
				}
			}
		case "azure":
			for _, m := range azParamRe.FindAllStringSubmatchIndex(e.Text, -1) {
				if off >= m[0] && off <= m[1] {
					t.kind, t.name, t.rng = "param", group(e.Text, m), e.Range
					return t.find()
				}
			}
			for _, m := range azVarRe.FindAllStringSubmatchIndex(e.Text, -1) {
				if off >= m[0] && off <= m[1] {
					t.kind, t.name, t.rng = "var", group(e.Text, m), e.Range
					return t.find()
				}
			}
		case "azure-macro":
			t.kind, t.name, t.rng = "var", strings.TrimSuffix(strings.TrimPrefix(e.Text, "$("), ")"), e.Range
			return t.find()
		}
	}
	doc := f.YAML.DocAt(pos)
	if doc == nil {
		return nil
	}
	path, n := doc.PathAt(pos)
	if len(path) == 0 {
		return nil
	}
	keys := make([]string, len(path))
	for i, s := range path {
		keys[i] = s.Key
		if s.IsIndex {
			keys[i] = "#"
		}
	}
	last := keys[len(keys)-1]
	str := n.Str()
	onKey := func() bool {
		parent := doc.NodeAt(path[:len(path)-1])
		pr := parent.Pair(last)
		if pr != nil && pr.Key.Range.Contains(pos) {
			t.rng = pr.Key.Range
			return true
		}
		return false
	}
	has := func(i int, k string) bool { return i < len(keys) && keys[i] == k }
	if n != nil && n.Kind == yamlkit.KindScalar {
		t.rng = n.Range
	}
	switch tool {
	case GitHub:
		switch {
		case len(keys) == 2 && has(0, "jobs") && onKey():
			t.kind, t.name = "job", keys[1]
		case len(keys) >= 3 && has(0, "jobs") && has(2, "needs") && str != "":
			t.kind, t.name = "job", str
		case len(keys) == 3 && has(0, "jobs") && last == "uses" && strings.HasPrefix(str, "./"):
			t.kind, t.name = "file", opts.repoPath(str)
		case has(0, "jobs") && last == "uses" && strings.HasPrefix(str, "./"):
			t.kind, t.name = "action", opts.repoPath(str)
		}
	case GitLab:
		switch {
		case len(keys) == 1 && onKey() && !glKeywords[last]:
			t.kind, t.name = "job", last
		case len(keys) >= 2 && has(1, "extends") && str != "":
			t.kind, t.name = "job", str
		case len(keys) >= 3 && has(1, "needs") && str != "" && (last == "#" || last == "job"):
			t.kind, t.name = "job", str
		case last == "#" && doc.NodeAt(path[:len(path)-1]).Tag == "!reference" && path[len(path)-1].Index == 0:
			t.kind, t.name = "job", str
		case has(0, "include") && (last == "local" || last == "#") && str != "" && !isURL(str):
			t.kind, t.name = "file", opts.repoPath(str)
		}
	case Azure:
		switch {
		case last == "template" && str != "":
			file, repo, _ := strings.Cut(str, "@")
			if repo == "" || repo == "self" {
				from := f.Path
				t.kind, t.name = "file", (&azure{opts: opts}).templatePath(from, file)
			}
		case (last == "dependsOn" || (last == "#" && len(keys) > 1 && keys[len(keys)-2] == "dependsOn")) && str != "":
			t.kind, t.name = "stage", str
			if t.job != nil {
				t.kind = "job"
				if t.job.Stage != "" {
					t.name = t.job.Stage + "." + str
				}
			}
		}
	}
	if t.kind == "" {
		return nil
	}
	return t.find()
}

func group(s string, m []int) string {
	for i := 2; i+1 < len(m); i += 2 {
		if m[i] >= 0 {
			return s[m[i]:m[i+1]]
		}
	}
	return ""
}

// ghContext resolves a GitHub context reference.
func (t *target) ghContext(f *provider.File, pos yamlkit.Pos, r ghRef) *target {
	t.name, t.sub = r.part(0), strings.Join(r.path[min(1, len(r.path)):], ".")
	switch r.ctx {
	case "inputs":
		t.kind = "input"
	case "env":
		t.kind = "env"
		t.where = envAt(f, pos, t.name)
	case "secrets":
		t.kind = "secret"
	case "matrix":
		t.kind = "matrix"
	case "needs":
		t.kind = "job"
		if r.part(1) == "outputs" {
			t.kind, t.sub = "output", r.part(2)
		}
	case "steps":
		t.kind = "step"
	default:
		t.kind, t.name, t.sub = "context", r.ctx, strings.Join(r.path, ".")
	}
	return t.find()
}

// envAt finds the nearest env definition of name around pos: the
// step's, the job's, then the workflow's.
func envAt(f *provider.File, pos yamlkit.Pos, name string) *Source {
	r := root(f.YAML)
	if r == nil {
		return nil
	}
	var best *yamlkit.Node
	consider := func(env *yamlkit.Node) {
		if pr := env.Pair(name); pr != nil {
			best = pr.Key
		}
	}
	consider(r.Get("env"))
	for _, jp := range pairs(r.Get("jobs")) {
		if !jp.Value.Range.Contains(pos) && !jp.Key.Range.Contains(pos) {
			continue
		}
		consider(jp.Value.Get("env"))
		for _, st := range items(jp.Value.Get("steps")) {
			if st.Range.Contains(pos) {
				consider(st.Get("env"))
			}
		}
	}
	if best == nil {
		return nil
	}
	return srcp(f.Path, best)
}

// find fills in where the target is defined.
func (t *target) find() *target {
	pl := t.pl
	switch t.kind {
	case "job":
		if j := pl.Job(t.name); j != nil {
			t.where = &j.Source
			return t
		}
		for _, tp := range pl.Templates {
			if tp.Name == t.name {
				t.kind, t.where = "template", &tp.Source
				return t
			}
		}
	case "output":
		if j := pl.Job(t.name); j != nil {
			t.where = &j.Source
			if pr := j.node.Get("outputs").Pair(t.sub); pr != nil {
				t.where = srcp(j.Source.File, pr.Key)
			}
		}
	case "step":
		if t.job != nil {
			for _, s := range t.job.Steps {
				if s.ID == t.name {
					t.where = &s.Source
				}
			}
		}
	case "input", "param", "secret":
		for _, in := range pl.Inputs {
			if in.Name == t.name {
				t.where = &in.Source
			}
		}
	case "matrix":
		if t.job != nil {
			if pr := t.job.node.Get("strategy").Get("matrix").Pair(t.name); pr != nil {
				t.where = srcp(t.job.Source.File, pr.Key)
			}
		}
	case "var":
		for _, v := range pl.Variables {
			if v.Name == t.name {
				t.where = &v.Source
			}
		}
	case "stage":
		for _, s := range pl.Stages {
			if s.Name == t.name {
				t.where = s.Source
			}
		}
	case "file", "action":
		t.where = &Source{File: t.name, Line: 1}
	}
	return t
}

// --- hover and definition ---

// DefinitionIn jumps to what the cursor refers to, following includes,
// templates and reusable workflows through opts.
func DefinitionIn(tool string, f *provider.File, pos yamlkit.Pos, opts Options) []provider.Location {
	t := at(tool, f, pos, opts)
	if t == nil || t.where == nil {
		return nil
	}
	w := t.where
	if t.kind == "action" {
		if p, _, err := (&github{opts: opts}).actionFile(t.name); err == nil {
			w = &Source{File: p, Line: 1}
		} else {
			w = &Source{File: t.name + "/action.yml", Line: 1}
		}
	}
	loc := provider.Location{Range: w.Range}
	if w.File != "" && w.File != f.Path {
		loc.Path = w.File
	}
	if w.Line > 0 && w.Range.Start.Line == 0 {
		loc.Range = yamlkit.Range{Start: yamlkit.Pos{Line: w.Line, Col: 1}, End: yamlkit.Pos{Line: w.Line, Col: 1}}
	}
	return []provider.Location{loc}
}

// ghContexts explains the GitHub contexts that aren't declared in the
// file.
var ghContexts = map[string]string{
	"github":   "Information about the run and the event that started it",
	"runner":   "The machine running the job",
	"job":      "The current job (status, container, services)",
	"jobs":     "Outputs of this reusable workflow's jobs",
	"strategy": "The matrix strategy of the current job",
	"vars":     "A configuration variable set in the repository, organization or environment settings",
}

var ghProps = map[string]string{
	"github.event_name": "The event that started the run (push, pull_request, …)",
	"github.ref":        "The branch or tag ref, e.g. refs/heads/main",
	"github.ref_name":   "The short branch or tag name, e.g. main",
	"github.sha":        "The commit SHA that started the run",
	"github.actor":      "Who started the run",
	"github.repository": "owner/name of the repository",
	"github.run_id":     "A unique number for the run",
	"github.run_number": "The run's number in this workflow",
	"github.workspace":  "The checkout folder on the runner",
	"github.token":      "The job's automatic GITHUB_TOKEN (never shown)",
	"github.event":      "The full webhook payload of the event",
	"github.head_ref":   "The pull request's source branch",
	"github.base_ref":   "The pull request's target branch",
	"runner.os":         "Linux, Windows or macOS",
	"runner.temp":       "A temporary folder, emptied after each job",
	"job.status":        "success, failure or cancelled",
}

// HoverIn explains what the cursor is on, following includes,
// templates and reusable workflows through opts.
func HoverIn(tool string, f *provider.File, pos yamlkit.Pos, opts Options) *provider.Hover {
	t := at(tool, f, pos, opts)
	if t == nil {
		return nil
	}
	h := &provider.Hover{Range: t.rng}
	row := func(label, value string) {
		if value != "" {
			h.Rows = append(h.Rows, provider.HoverRow{Label: label, Value: value})
		}
	}
	where := func() {
		if t.where != nil && t.where.File != "" && t.where.File != f.Path {
			row("Defined in", fmt.Sprintf("%s:%d", t.where.File, t.where.Line))
		} else if t.where != nil && t.where.Line > 0 {
			row("Defined at", fmt.Sprintf("line %d", t.where.Line))
		}
	}
	pl := t.pl
	switch t.kind {
	case "job", "output":
		j := pl.Job(t.name)
		if j == nil {
			h.Title = "Job " + t.name
			row("Not found", "no job of this pipeline has that name")
			break
		}
		h.Title = "Job " + j.Name
		if t.kind == "output" {
			h.Title = "Output " + t.sub + " of job " + j.Name
			for _, o := range j.Outputs {
				if o.Key == t.sub {
					row("Value", o.Value)
				}
			}
		}
		row("Stage", j.Stage)
		row("Runs on", j.Runner)
		row("Runs", j.Caption())
		var needs []string
		for _, n := range j.Needs {
			needs = append(needs, n.Job)
		}
		row("Needs", strings.Join(needs, ", "))
		row("Extends", strings.Join(j.Extends, " → "))
		row("When", j.When)
		row("If", j.If)
		if len(j.Steps) > 0 {
			row("Steps", fmt.Sprint(len(j.Steps)))
		}
		where()
		h.Code = effectiveText(j, 12)
	case "template":
		h.Title = "Template " + t.name
		where()
		row("Used by", strings.Join(usedBy(pl, t.name), ", "))
	case "file", "action":
		h.Title = map[string]string{"file": "File ", "action": "Action "}[t.kind] + t.name
		for _, inc := range pl.Includes {
			if inc.File == t.name || strings.HasPrefix(inc.File, t.name+"/") {
				row("State", map[string]string{"resolved": "read", "missing": "missing", "unresolved": "not read"}[inc.State])
				row("Note", inc.Note)
				break
			}
		}
	case "input", "param":
		h.Title = map[string]string{"input": "Input ", "param": "Parameter "}[t.kind] + t.name
		found := false
		for _, in := range pl.Inputs {
			if in.Name == t.name {
				found = true
				row("Type", in.Type)
				if in.HasDefault {
					row("Default", in.Default)
				}
				if in.Required {
					row("Required", "yes")
				}
				row("One of", strings.Join(in.Options, " | "))
				row("From", in.From)
				row("About", in.Description)
			}
		}
		if !found {
			row("Not declared", "this workflow declares no such input")
		}
		where()
	case "secret":
		h.Title = "Secret " + t.name
		row("Value", "never shown; set in the repository, organization or environment settings")
		if t.name == "GITHUB_TOKEN" {
			row("About", "the job's automatic token")
		}
		where()
	case "env":
		h.Title = "Environment variable " + t.name
		if t.where == nil {
			row("Set", "not in this file: by the runner, or a step's $GITHUB_ENV")
		}
		where()
	case "matrix":
		h.Title = "Matrix value " + t.name
		if t.job != nil && t.job.Matrix != nil {
			seen := map[string]bool{}
			var vals []string
			for _, c := range t.job.Matrix.Combos {
				if v, ok := value(c, t.name); ok && !seen[v] {
					seen[v] = true
					vals = append(vals, v)
				}
			}
			row("Values", strings.Join(vals, ", "))
			row("Jobs", fmt.Sprint(len(t.job.Matrix.Combos)))
			row("Known", t.job.Matrix.Runtime)
		}
		where()
	case "step":
		h.Title = "Step " + t.name
		if t.job != nil {
			for _, s := range t.job.Steps {
				if s.ID == t.name {
					row("Name", s.Name)
					row("Runs", s.Kind+" · "+s.Detail)
				}
			}
		}
		if t.where == nil {
			row("Not found", "no step of this job has that id")
		}
		where()
	case "context":
		h.Title = "GitHub context " + t.sub
		row("About", ghProps[t.sub])
		if ghProps[t.sub] == "" {
			row("About", ghContexts[t.name])
		}
		row("Known", "at run time")
	case "var":
		h.Title = "Variable " + t.name
		for _, v := range pl.Variables {
			if v.Name == t.name {
				row("Value", v.Value)
				row("Scope", v.Scope)
			}
		}
		if t.where == nil {
			row("Set", "not in this pipeline: predefined, a variable group, or set at queue time")
		}
		where()
	case "stage":
		h.Title = "Stage " + t.name
		for _, s := range pl.Stages {
			if s.Name == t.name {
				row("Jobs", strings.Join(s.Jobs, ", "))
			}
		}
		where()
	}
	return h
}

func usedBy(pl *Pipeline, template string) []string {
	var out []string
	for _, j := range pl.Jobs {
		for _, e := range j.Extends {
			if e == template {
				out = append(out, j.ID)
				break
			}
		}
	}
	return out
}

// effectiveText is the first lines of a job's effective configuration.
func effectiveText(j *Job, max int) string {
	var lines []string
	for i, l := range j.Effective {
		if i == max {
			lines = append(lines, "…")
			break
		}
		lines = append(lines, l.Text)
	}
	return strings.Join(lines, "\n")
}
