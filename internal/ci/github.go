package ci

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// GitHub Actions: jobs run in parallel unless needs: orders them. A job
// that calls a reusable workflow (uses: ./.github/workflows/x.yml) runs
// that workflow's jobs; they are shown in its place, as caller/job, so
// the graph is what GitHub runs. Steps that use a local composite
// action show the action's steps.

type github struct {
	p     *Pipeline
	opts  Options
	t     tags
	calls map[string][]string // caller job id -> the called workflow's last jobs
	known map[string]bool     // job ids and callers, for needs checks
}

// maxCallDepth is how deep reusable workflows may call each other.
const maxCallDepth = 4

func readGitHub(p *Pipeline, content []byte, f *yamlkit.File, opts Options) {
	r := root(f)
	if r == nil {
		return
	}
	r = resolved(r)
	gh := &github{p: p, opts: opts, t: tags{}, calls: map[string][]string{}, known: map[string]bool{}}
	if r.Get("runs") != nil {
		gh.action(r)
		return
	}
	p.Name = r.Get("name").Str()
	on := r.Get("on")
	if on == nil {
		on = r.Get("true") // YAML 1.1 readers turn "on" into true
	}
	p.Triggers = triggers(on)
	if on.Get("workflow_call") != nil && len(p.Triggers) == 1 {
		p.Kind = "reusable"
	}
	p.Inputs = ghInputs(p.File, on)
	p.Variables = envVars(p.File, r.Get("env"), "workflow")
	p.Jobs = gh.jobs(p.File, r.Get("jobs"), "", "", 0)
	gh.edges()
	gh.checkExpressions(f)
}

// triggers names the events that start a workflow.
func triggers(on *yamlkit.Node) []string {
	switch {
	case on == nil:
		return nil
	case on.Kind == yamlkit.KindScalar:
		return []string{on.Value}
	case on.Kind == yamlkit.KindSeq:
		return strs(on)
	}
	var out []string
	for _, pr := range on.Pairs {
		ev, v := keyName(pr.Key), pr.Value
		var detail []string
		switch ev {
		case "schedule":
			for _, s := range items(v) {
				detail = append(detail, s.Get("cron").Str())
			}
		case "workflow_dispatch":
			detail = append(detail, "manual")
		case "workflow_call":
			detail = append(detail, "reusable")
		case "workflow_run":
			detail = append(detail, "after "+strings.Join(strs(v.Get("workflows")), ", "))
		}
		for _, k := range []string{"types", "branches", "branches-ignore", "tags", "paths", "paths-ignore"} {
			if vals := strs(v.Get(k)); len(vals) > 0 {
				detail = append(detail, k+" "+strings.Join(vals, ", "))
			}
		}
		if len(detail) > 0 {
			ev += " · " + strings.Join(detail, " · ")
		}
		out = append(out, ev)
	}
	return out
}

func ghInputs(file string, on *yamlkit.Node) []Input {
	var out []Input
	for _, ev := range []string{"workflow_dispatch", "workflow_call"} {
		in := on.Get(ev).Get("inputs")
		if in == nil || in.Kind != yamlkit.KindMap {
			continue
		}
		for _, pr := range in.Pairs {
			v := pr.Value
			i := Input{Name: keyName(pr.Key), From: ev, Source: src(file, pr.Key), Type: v.Get("type").Str(),
				Description: v.Get("description").Str(), Required: v.Get("required").Str() == "true", Options: strs(v.Get("options"))}
			if d := v.Get("default"); d != nil {
				i.Default, i.HasDefault = text(d), true
			}
			if i.Type == "" {
				i.Type = "string"
			}
			// Inputs declared for both events are one input.
			if k := slices.IndexFunc(out, func(o Input) bool { return o.Name == i.Name }); k >= 0 {
				out[k].From += ", " + ev
				continue
			}
			out = append(out, i)
		}
	}
	if sec := on.Get("workflow_call").Get("secrets"); sec != nil && sec.Kind == yamlkit.KindMap {
		for _, pr := range sec.Pairs {
			out = append(out, Input{Name: keyName(pr.Key), Type: "secret", From: "secret", Required: pr.Value.Get("required").Str() == "true",
				Description: pr.Value.Get("description").Str(), Source: src(file, pr.Key)})
		}
	}
	return out
}

func envVars(file string, env *yamlkit.Node, scope string) []Var {
	if env == nil || env.Kind != yamlkit.KindMap {
		return nil
	}
	var out []Var
	for _, pr := range env.Pairs {
		out = append(out, Var{Name: keyName(pr.Key), Value: text(pr.Value), Scope: scope, Source: src(file, pr.Key)})
	}
	return out
}

// jobs reads a jobs: map. prefix and group are set for the jobs of a
// called workflow ("deploy/", caller "deploy").
func (gh *github) jobs(file string, jobs *yamlkit.Node, prefix, group string, depth int) []*Job {
	if jobs == nil || jobs.Kind != yamlkit.KindMap {
		return nil
	}
	var out []*Job
	for _, pr := range jobs.Pairs {
		out = append(out, gh.job(file, prefix, group, pr, depth)...)
	}
	return out
}

func (gh *github) job(file, prefix, group string, pr *yamlkit.Pair, depth int) []*Job {
	id, n := keyName(pr.Key), pr.Value
	if !isMap(n) {
		return nil
	}
	j := &Job{ID: prefix + id, Name: id, Kind: "job", Group: group, Source: src(file, pr.Key), node: n}
	if name := n.Get("name").Str(); name != "" {
		j.Name = name
	}
	j.Needs = []Need{}
	needs := n.Get("needs")
	for _, x := range strs(needs) {
		j.Needs = append(j.Needs, Need{Job: prefix + x, Source: srcp(file, needs)})
	}
	for i, it := range items(needs) {
		j.Needs[i].Source = srcp(file, it)
	}
	j.If = text(n.Get("if"))
	j.Runner = runsOn(n.Get("runs-on"))
	if c := n.Get("container"); c != nil {
		img := c.Str()
		if isMap(c) {
			img = c.Get("image").Str()
		}
		j.Runner = strings.TrimPrefix(j.Runner+" · container "+img, " · ")
	}
	if env := n.Get("environment"); env != nil {
		j.Environment = env.Str()
		if isMap(env) {
			j.Environment = env.Get("name").Str()
		}
	}
	j.AllowFailure = n.Get("continue-on-error").Str() == "true"
	if s := n.Get("strategy"); s != nil {
		mx, errs := expandMatrix(s.Get("matrix"), j.Name)
		if mx != nil {
			mx.MaxParallel = s.Get("max-parallel").Str()
			mx.FailFast = s.Get("fail-fast").Str()
			j.Matrix = mx
		}
		for _, e := range errs {
			gh.p.problem(yamlkit.SeverityError, "gh-matrix", j.ID, keySrc(file, s, "matrix"), "Matrix of "+id+": "+e, "")
		}
	}
	if out := n.Get("outputs"); out != nil && out.Kind == yamlkit.KindMap {
		for _, o := range out.Pairs {
			j.Outputs = append(j.Outputs, KV{keyName(o.Key), text(o.Value)})
		}
	}
	j.Effective = gh.t.emit(gh.t.own(n, file), origin{file: file})
	gh.known[j.ID] = true

	if uses := n.Get("uses").Str(); uses != "" {
		j.Kind, j.Uses = "reusable", uses
		return gh.call(j, file, n, depth)
	}
	for _, st := range items(n.Get("steps")) {
		j.Steps = append(j.Steps, gh.step(file, st, 0))
	}
	return []*Job{j}
}

func runsOn(v *yamlkit.Node) string {
	switch {
	case v == nil:
		return ""
	case isMap(v):
		parts := []string{}
		if g := v.Get("group").Str(); g != "" {
			parts = append(parts, "group "+g)
		}
		parts = append(parts, strs(v.Get("labels"))...)
		return strings.Join(parts, ", ")
	}
	return strings.Join(strs(v), ", ")
}

// step reads one step; a local composite action brings its own steps.
func (gh *github) step(file string, st *yamlkit.Node, depth int) Step {
	s := Step{ID: st.Get("id").Str(), Name: st.Get("name").Str(), If: text(st.Get("if")), Source: src(file, st)}
	switch {
	case st.Get("uses") != nil:
		uses := st.Get("uses").Str()
		s.Kind, s.Detail = "uses", uses
		if s.Name == "" {
			s.Name = uses
		}
		if strings.HasPrefix(uses, "./") && depth < maxCallDepth {
			gh.localAction(&s, file, st, uses, depth)
		}
	case st.Get("run") != nil:
		s.Kind = "run"
		s.Detail = firstLine(st.Get("run").Str())
		if s.Name == "" {
			s.Name = s.Detail
		}
		if sh := st.Get("shell").Str(); sh != "" {
			s.Note = sh
		}
	default:
		s.Kind = "step"
		if s.Name == "" {
			s.Name = "step"
		}
	}
	return s
}

// actionFile finds a local action's metadata file.
func (gh *github) actionFile(dir string) (string, *yamlkit.File, error) {
	var last error
	for _, name := range []string{"action.yml", "action.yaml"} {
		p := path.Join(dir, name)
		_, f, err := gh.opts.load(p)
		if err == nil {
			return p, f, nil
		}
		last = err
	}
	return "", nil, last
}

func (gh *github) localAction(s *Step, file string, st *yamlkit.Node, uses string, depth int) {
	dir := gh.opts.repoPath(uses)
	inc := Include{Kind: "action", Target: uses, Source: src(file, st.Get("uses"))}
	if gh.opts.Load == nil {
		inc.State, inc.Note = "unresolved", "open the folder to follow local actions"
		gh.p.Includes = append(gh.p.Includes, inc)
		return
	}
	p, f, err := gh.actionFile(dir)
	if err != nil {
		inc.State = "missing"
		gh.p.Includes = append(gh.p.Includes, inc)
		gh.p.problem(yamlkit.SeverityError, "gh-action-missing", "", &inc.Source, fmt.Sprintf("No action.yml in %s", dir),
			"Local actions are relative to the repository root, not to the workflow.")
		return
	}
	inc.State, inc.File = "resolved", p
	if !slices.ContainsFunc(gh.p.Includes, func(i Include) bool { return i.File == p }) {
		gh.p.Includes = append(gh.p.Includes, inc)
	}
	a := resolved(root(f))
	runs := a.Get("runs")
	s.From = p
	switch using := runs.Get("using").Str(); using {
	case "composite":
		s.Note = "composite action"
		for _, c := range items(runs.Get("steps")) {
			s.Children = append(s.Children, gh.step(p, c, depth+1))
		}
	case "docker":
		s.Note = "Docker action · " + runs.Get("image").Str()
	default:
		s.Note = "JavaScript action · " + using
	}
}

// call reads the reusable workflow a job calls. A local one's jobs take
// the caller's place; a remote one stays a single job.
func (gh *github) call(j *Job, file string, n *yamlkit.Node, depth int) []*Job {
	inc := Include{Kind: "workflow", Target: j.Uses, Source: src(file, n.Get("uses")), State: "unresolved"}
	if !strings.HasPrefix(j.Uses, "./") || gh.opts.Load == nil {
		inc.Note = "in another repository"
		if gh.opts.Load == nil && strings.HasPrefix(j.Uses, "./") {
			inc.Note = "open the folder to follow reusable workflows"
		}
		gh.p.Includes = append(gh.p.Includes, inc)
		return []*Job{j}
	}
	p := gh.opts.repoPath(j.Uses)
	_, f, err := gh.opts.load(p)
	if err != nil {
		inc.State = "missing"
		gh.p.Includes = append(gh.p.Includes, inc)
		gh.p.problem(yamlkit.SeverityError, "gh-workflow-missing", j.ID, &inc.Source, fmt.Sprintf("Reusable workflow %s doesn't exist", p),
			"Paths are relative to the repository root: ./.github/workflows/<file>.yml")
		return []*Job{j}
	}
	inc.State, inc.File = "resolved", p
	gh.p.Includes = append(gh.p.Includes, inc)
	r := resolved(root(f))
	call := r.Get("on").Get("workflow_call")
	if call == nil {
		gh.p.problem(yamlkit.SeverityError, "gh-workflow-not-callable", j.ID, &inc.Source, fmt.Sprintf("%s can't be called: it has no on: workflow_call", p), "")
	}
	gh.args(j, file, n, ghInputs(p, r.Get("on")))
	if depth >= maxCallDepth {
		gh.p.problem(yamlkit.SeverityError, "gh-workflow-depth", j.ID, &inc.Source, "Reusable workflows nest more than 4 levels deep", "")
		return []*Job{j}
	}
	called := gh.jobs(p, r.Get("jobs"), j.ID+"/", j.ID, depth+1)
	if len(called) == 0 {
		return []*Job{j}
	}
	// The called workflow's first jobs wait for what the caller needs;
	// jobs needing the caller wait for its last jobs.
	needed := map[string]bool{}
	for _, c := range called {
		for _, nd := range c.Needs {
			needed[nd.Job] = true
		}
	}
	for _, c := range called {
		if len(c.Needs) == 0 {
			c.Needs = append(c.Needs, j.Needs...)
		}
		if !needed[c.ID] {
			gh.calls[j.ID] = append(gh.calls[j.ID], c.ID)
		}
		if c.If == "" {
			c.If = j.If
		}
	}
	return called
}

// args lines up with: values with the called workflow's inputs.
func (gh *github) args(j *Job, file string, n *yamlkit.Node, inputs []Input) {
	with := n.Get("with")
	passed := map[string]bool{}
	for _, in := range inputs {
		if in.Type == "secret" {
			continue
		}
		a := Arg{Name: in.Name, Type: in.Type, Required: in.Required}
		switch v := with.Get(in.Name); {
		case v != nil:
			a.Value, a.State = text(v), "passed"
		case in.HasDefault:
			a.Value, a.State = in.Default, "default"
		case in.Required:
			a.State = "missing"
			gh.p.problem(yamlkit.SeverityError, "gh-input-missing", j.ID, keySrc(file, n, "uses"),
				fmt.Sprintf("Job %s doesn't pass required input %s to %s", j.ID, in.Name, j.Uses), "")
		default:
			a.State = "default"
		}
		passed[in.Name] = true
		j.With = append(j.With, a)
	}
	if isMap(with) {
		var names []string
		for _, in := range inputs {
			names = append(names, in.Name)
		}
		for _, pr := range with.Pairs {
			if name := keyName(pr.Key); !passed[name] {
				j.With = append(j.With, Arg{Name: name, Value: text(pr.Value), State: "unknown"})
				gh.p.problem(yamlkit.SeverityError, "gh-input-unknown", j.ID, srcp(file, pr.Key),
					fmt.Sprintf("%s has no input %s", j.Uses, name), didYouMean(name, names))
			}
		}
	}
}

// ends lists what a need on id waits for: id, or the last jobs of the
// workflow it calls.
func (gh *github) ends(id string, depth int) []string {
	last, ok := gh.calls[id]
	if !ok || depth > maxCallDepth {
		return []string{id}
	}
	var out []string
	for _, l := range last {
		out = append(out, gh.ends(l, depth+1)...)
	}
	return out
}

func (gh *github) edges() {
	p := gh.p
	var ids []string
	for id := range gh.known {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, j := range p.Jobs {
		for _, nd := range j.Needs {
			if !gh.known[nd.Job] {
				p.problem(yamlkit.SeverityError, "gh-needs-unknown", j.ID, nd.Source, fmt.Sprintf("Job %s needs %s, which isn't a job of this workflow", j.ID, nd.Job), didYouMean(nd.Job, ids))
				continue
			}
			for _, from := range gh.ends(nd.Job, 0) {
				p.Edges = append(p.Edges, Edge{From: from, To: j.ID, Label: "needs"})
			}
		}
	}
	// A cycle means none of its jobs can start.
	state := map[string]int{} // 0 new, 1 on the stack, 2 done
	var visit func(id string) bool
	visit = func(id string) bool {
		switch state[id] {
		case 1:
			return true
		case 2:
			return false
		}
		state[id] = 1
		for _, e := range p.Edges {
			if e.To == id && visit(e.From) {
				return true
			}
		}
		state[id] = 2
		return false
	}
	for _, j := range p.Jobs {
		if state[j.ID] == 0 && visit(j.ID) {
			p.problem(yamlkit.SeverityError, "gh-needs-cycle", j.ID, &j.Source, fmt.Sprintf("Job %s is part of a needs cycle, so it never starts", j.ID), "")
			break
		}
	}
}

// action reads a composite (or JavaScript/Docker) action's metadata.
func (gh *github) action(r *yamlkit.Node) {
	p := gh.p
	p.Kind, p.Name = "action", r.Get("name").Str()
	if in := r.Get("inputs"); in != nil && in.Kind == yamlkit.KindMap {
		for _, pr := range in.Pairs {
			v := pr.Value
			i := Input{Name: keyName(pr.Key), From: "action", Source: src(p.File, pr.Key), Description: v.Get("description").Str(), Required: v.Get("required").Str() == "true"}
			if d := v.Get("default"); d != nil {
				i.Default, i.HasDefault = text(d), true
			}
			p.Inputs = append(p.Inputs, i)
		}
	}
	runs := r.Get("runs")
	j := &Job{ID: "action", Name: p.Name, Kind: "job", Source: keyRange(p.File, r, "runs"), node: runs, Needs: []Need{}}
	if j.Name == "" {
		j.Name = "action"
	}
	j.Runner = runs.Get("using").Str()
	if out := r.Get("outputs"); out != nil && out.Kind == yamlkit.KindMap {
		for _, o := range out.Pairs {
			j.Outputs = append(j.Outputs, KV{keyName(o.Key), text(o.Value.Get("value"))})
		}
	}
	for _, st := range items(runs.Get("steps")) {
		j.Steps = append(j.Steps, gh.step(p.File, st, 0))
	}
	j.Effective = gh.t.emit(gh.t.own(runs, p.File), origin{file: p.File})
	p.Jobs = []*Job{j}
}

func keyRange(file string, m *yamlkit.Node, k string) Source {
	return *keySrc(file, m, k)
}

// --- expressions ---

// checkExpressions looks at the ${{ }} expressions of the file: needs
// and steps they read must exist where they are used.
func (gh *github) checkExpressions(f *yamlkit.File) {
	p := gh.p
	declared := map[string]bool{}
	for _, in := range p.Inputs {
		declared[in.Name] = true
	}
	hasInputs := slices.ContainsFunc(p.Triggers, func(t string) bool {
		return strings.HasPrefix(t, "workflow_dispatch") || strings.HasPrefix(t, "workflow_call")
	})
	for _, e := range f.Expressions {
		if e.Syntax != "github" {
			continue
		}
		j := gh.jobAt(e.Range.Start)
		for _, r := range ghRefs(e.Text) {
			s := &Source{File: p.File, Line: e.Range.Start.Line, Range: e.Range}
			name := r.part(0)
			switch r.ctx {
			case "needs":
				if j == nil || name == "" {
					continue
				}
				if !slices.ContainsFunc(j.Needs, func(n Need) bool { return n.Job == name }) {
					p.problem(yamlkit.SeverityError, "gh-needs-context", j.ID, s,
						fmt.Sprintf("needs.%s: job %s doesn't need %s, so it can't read its outputs", name, j.ID, name),
						fmt.Sprintf("Add %s to the needs of %s.", name, j.ID))
				}
			case "steps":
				if j == nil || name == "" || len(j.Steps) == 0 {
					continue
				}
				if !slices.ContainsFunc(j.Steps, func(s Step) bool { return s.ID == name }) {
					var ids []string
					for _, s := range j.Steps {
						if s.ID != "" {
							ids = append(ids, s.ID)
						}
					}
					p.problem(yamlkit.SeverityWarning, "gh-step-unknown", j.ID, s, fmt.Sprintf("steps.%s: no step in job %s has id %s", name, j.ID, name), didYouMean(name, ids))
				}
			case "matrix":
				if j == nil || name == "" || j.Matrix == nil || j.Matrix.Runtime != "" {
					continue
				}
				if !matrixHas(j.Matrix, name) {
					p.problem(yamlkit.SeverityWarning, "gh-matrix-unknown", j.ID, s, fmt.Sprintf("matrix.%s: the matrix of job %s has no %s", name, j.ID, name), "")
				}
			case "inputs":
				if name == "" || declared[name] {
					continue
				}
				hint := didYouMean(name, keys(declared))
				if !hasInputs {
					hint = "Only workflow_dispatch and workflow_call workflows have inputs."
				}
				p.problem(yamlkit.SeverityWarning, "gh-input-undeclared", "", s, fmt.Sprintf("inputs.%s isn't declared", name), hint)
			}
		}
	}
}

func matrixHas(m *Matrix, key string) bool {
	for _, c := range m.Combos {
		if _, ok := value(c, key); ok {
			return true
		}
	}
	return slices.ContainsFunc(m.Axes, func(a Axis) bool { return a.Name == key })
}

func keys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// jobAt is the top-level job whose definition contains pos.
func (gh *github) jobAt(pos yamlkit.Pos) *Job {
	for _, j := range gh.p.Jobs {
		if j.Group == "" && j.node != nil && (j.Source.Range.Contains(pos) || j.node.Range.Contains(pos)) {
			return j
		}
	}
	return nil
}
