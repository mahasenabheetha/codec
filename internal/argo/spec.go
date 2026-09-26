// Package argo is the Argo lens: it reads Argo Workflows (Workflow,
// WorkflowTemplate, ClusterWorkflowTemplate, CronWorkflow, and
// workflows that Argo Events sensors submit) and Argo CD applications,
// and explains what they do — templates stitched across files, the
// parameters each step really gets, DAG and steps graphs.
//
// Values only known while a workflow runs (step outputs, workflow.uid,
// event payloads) stay placeholders; nothing is guessed. Like yamlkit
// the package is pure: parsed documents in, facts out.
package argo

import (
	"cmp"
	"strconv"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// WorkflowKinds are the kinds whose spec is a workflow spec.
var WorkflowKinds = map[string]bool{"Workflow": true, "WorkflowTemplate": true, "ClusterWorkflowTemplate": true, "CronWorkflow": true}

// Source says where something is written.
type Source struct {
	File  string        `json:"file,omitempty"` // workspace path; "" for a render
	Doc   int           `json:"doc"`
	Line  int           `json:"line"`
	Range yamlkit.Range `json:"range"`
}

// Spec is one workflow spec: a Workflow, a (Cluster)WorkflowTemplate,
// a CronWorkflow's workflowSpec, or a Workflow a Sensor submits.
type Spec struct {
	Kind       string       `json:"kind"`
	Name       string       `json:"name"`
	Namespace  string       `json:"namespace,omitempty"`
	Generated  bool         `json:"generated,omitempty"` // generateName: the real name is known at submit
	Source     Source       `json:"source"`
	Entrypoint string       `json:"entrypoint,omitempty"`
	OnExit     string       `json:"onExit,omitempty"`
	Account    string       `json:"serviceAccount,omitempty"`
	Arguments  []Param      `json:"arguments"`
	Templates  []*Template  `json:"templates"`
	Ref        *TemplateRef `json:"workflowTemplateRef,omitempty"` // spec.workflowTemplateRef
	Schedule   string       `json:"schedule,omitempty"`            // CronWorkflow
	// Via names the Sensor trigger that submits this workflow.
	Via *Trigger `json:"via,omitempty"`

	content []byte
	node    *yamlkit.Node // the workflow spec map
}

// Trigger is where a Sensor submits a workflow, and which event data
// fills which argument.
type Trigger struct {
	Sensor  string            `json:"sensor"`
	Name    string            `json:"name"`
	Events  map[int]EventData `json:"events,omitempty"` // argument index -> event data
	Operate string            `json:"operation,omitempty"`
}

// EventData is an event field mapped onto a workflow argument.
type EventData struct {
	Dependency string   `json:"dependency"`
	Key        string   `json:"key"`               // dataKey, e.g. body.resource
	Allowed    []string `json:"allowed,omitempty"` // values the dependency's filter accepts
}

// Param is a parameter: a workflow argument, a template input, or an
// argument passed at a call site.
type Param struct {
	Name     string        `json:"name"`
	Value    string        `json:"value,omitempty"`
	HasValue bool          `json:"hasValue,omitempty"`
	Default  string        `json:"default,omitempty"`
	HasDef   bool          `json:"hasDefault,omitempty"`
	Enum     []string      `json:"enum,omitempty"`
	From     string        `json:"valueFrom,omitempty"` // valueFrom, described
	Range    yamlkit.Range `json:"range"`

	cm *cmRef // valueFrom.configMapKeyRef
}

type cmRef struct{ name, key string }

// Template is one entry of spec.templates.
type Template struct {
	Name     string        `json:"name"`
	Type     string        `json:"type"` // container, script, dag, steps, resource, suspend, http, containerSet, data, plugin
	Inputs   []Param       `json:"inputs,omitempty"`
	Outputs  []string      `json:"outputs,omitempty"`
	Steps    [][]*Call     `json:"steps,omitempty"`
	Tasks    []*Call       `json:"tasks,omitempty"`
	Image    string        `json:"image,omitempty"`
	Action   string        `json:"action,omitempty"` // resource
	Daemon   bool          `json:"daemon,omitempty"`
	Range    yamlkit.Range `json:"range"`
	NameAt   yamlkit.Range `json:"-"`
	node     *yamlkit.Node
	inputsAt yamlkit.Range
}

// Call is a step or DAG task: what it runs, with which arguments.
type Call struct {
	Name         string          `json:"name"`
	Template     string          `json:"template,omitempty"`
	Ref          *TemplateRef    `json:"templateRef,omitempty"`
	Args         []Param         `json:"arguments,omitempty"`
	Dependencies []string        `json:"dependencies,omitempty"`
	Depends      string          `json:"depends,omitempty"`
	When         string          `json:"when,omitempty"`
	Items        []*yamlkit.Node `json:"-"`
	WithParam    string          `json:"withParam,omitempty"`
	Sequence     *Sequence       `json:"withSequence,omitempty"`
	ContinueOn   string          `json:"continueOn,omitempty"`
	OnExit       string          `json:"onExit,omitempty"`
	Range        yamlkit.Range   `json:"range"`
	TemplateAt   yamlkit.Range   `json:"-"` // the template: value
	DependsAt    yamlkit.Range   `json:"-"`
	DepsAt       []yamlkit.Range `json:"-"` // each dependencies item
	node         *yamlkit.Node
}

// Sequence is withSequence.
type Sequence struct{ Count, Start, End, Format string }

// TemplateRef points at a template of another WorkflowTemplate.
type TemplateRef struct {
	Name         string        `json:"name"`
	Template     string        `json:"template,omitempty"`
	ClusterScope bool          `json:"clusterScope,omitempty"`
	Range        yamlkit.Range `json:"range"`
	NameAt       yamlkit.Range `json:"-"`
	TemplateAt   yamlkit.Range `json:"-"`
}

// Kind is the kind of template the ref points at.
func (r *TemplateRef) Kind() string {
	if r.ClusterScope {
		return "ClusterWorkflowTemplate"
	}
	return "WorkflowTemplate"
}

func (r *TemplateRef) String() string {
	s := r.Name + "/" + r.Template
	if r.ClusterScope {
		s = "cluster:" + s
	}
	return s
}

// Template returns the template called name, or nil.
func (s *Spec) Template(name string) *Template {
	for _, t := range s.Templates {
		if t.Name == name {
			return t
		}
	}
	return nil
}

// ID names a spec: Kind/name.
func (s *Spec) ID() string { return s.Kind + "/" + s.Name }

// Key identifies a spec in a scope: its ID, or Sensor/sensor/trigger
// for a workflow a sensor submits (those share generateName prefixes).
func (s *Spec) Key() string {
	if s.Via != nil {
		return "Sensor/" + s.Via.Sensor + "/" + s.Via.Name
	}
	return s.ID()
}

// Read collects the workflow specs of a parsed file: workflow kinds,
// and the workflows Argo Events sensors submit. content is the text f
// was parsed from (for showing template bodies).
func Read(file string, content []byte, f *yamlkit.File) []*Spec {
	var out []*Spec
	for i, d := range f.Docs {
		r := d.Root
		if r == nil || !strings.HasPrefix(r.Get("apiVersion").Str(), "argoproj.io/") {
			continue
		}
		kind := r.Get("kind").Str()
		src := Source{File: file, Doc: i, Line: r.Range.Start.Line, Range: r.Range}
		switch {
		case WorkflowKinds[kind]:
			spec := r.Get("spec")
			if kind == "CronWorkflow" {
				spec = spec.Get("workflowSpec")
			}
			s := readSpec(kind, r.Get("metadata"), spec, src, content)
			if kind == "CronWorkflow" {
				s.Schedule = r.Get("spec").Get("schedule").Str()
				if s.Schedule == "" {
					s.Schedule = strings.Join(scalars(r.Get("spec").Get("schedules")), ", ")
				}
			}
			out = append(out, s)
		case kind == "Sensor":
			out = append(out, sensorWorkflows(r, src, content)...)
		}
	}
	return out
}

func readSpec(kind string, meta, spec *yamlkit.Node, src Source, content []byte) *Spec {
	s := &Spec{
		Kind:       kind,
		Name:       meta.Get("name").Str(),
		Namespace:  meta.Get("namespace").Str(),
		Source:     src,
		Entrypoint: spec.Get("entrypoint").Str(),
		OnExit:     spec.Get("onExit").Str(),
		Account:    spec.Get("serviceAccountName").Str(),
		Arguments:  params(spec.Get("arguments").Get("parameters")),
		content:    content,
		node:       spec,
	}
	if s.Name == "" {
		if g := meta.Get("generateName").Str(); g != "" {
			s.Name, s.Generated = g, true
		}
	}
	if ref := spec.Get("workflowTemplateRef"); ref != nil {
		s.Ref = &TemplateRef{Name: ref.Get("name").Str(), ClusterScope: ref.Get("clusterScope").Str() == "true", Range: ref.Range}
	}
	for _, t := range items(spec.Get("templates")) {
		s.Templates = append(s.Templates, readTemplate(t))
	}
	return s
}

// sensorWorkflows reads the workflows a Sensor's triggers submit
// (argoWorkflow and k8s triggers whose resource is a Workflow).
func sensorWorkflows(r *yamlkit.Node, src Source, content []byte) []*Spec {
	sensor := r.Get("metadata").Get("name").Str()
	filters := map[string]map[string][]string{} // dependency -> data path -> allowed values
	for _, d := range items(r.Get("spec").Get("dependencies")) {
		m := map[string][]string{}
		for _, f := range items(d.Get("filters").Get("data")) {
			m[f.Get("path").Str()] = scalars(f.Get("value"))
		}
		filters[d.Get("name").Str()] = m
	}
	var out []*Spec
	for _, t := range items(r.Get("spec").Get("triggers")) {
		tpl := t.Get("template")
		body := tpl.Get("argoWorkflow")
		if body == nil {
			body = tpl.Get("k8s")
		}
		res := body.Get("source").Get("resource")
		if res.Get("kind").Str() != "Workflow" {
			continue
		}
		s := readSpec("Workflow", res.Get("metadata"), res.Get("spec"), Source{File: src.File, Doc: src.Doc, Line: res.Range.Start.Line, Range: res.Range}, content)
		tr := &Trigger{Sensor: sensor, Name: tpl.Get("name").Str(), Operate: body.Get("operation").Str(), Events: map[int]EventData{}}
		for _, p := range items(body.Get("parameters")) {
			rest, ok := strings.CutPrefix(p.Get("dest").Str(), "spec.arguments.parameters.")
			idx, field, _ := strings.Cut(rest, ".")
			n, err := strconv.Atoi(idx)
			if !ok || err != nil || field != "value" {
				continue
			}
			sv := p.Get("src")
			dep := sv.Get("dependencyName").Str()
			key := cmp.Or(sv.Get("dataKey").Str(), sv.Get("dataTemplate").Str(), sv.Get("contextKey").Str())
			tr.Events[n] = EventData{Dependency: dep, Key: key, Allowed: filters[dep][key]}
		}
		s.Via = tr
		out = append(out, s)
	}
	return out
}

func readTemplate(t *yamlkit.Node) *Template {
	tp := &Template{Name: t.Get("name").Str(), Range: t.Range, node: t}
	if pr := t.Pair("name"); pr != nil && pr.Value != nil {
		tp.NameAt = pr.Value.Range
	}
	for _, typ := range []string{"container", "script", "dag", "steps", "resource", "suspend", "http", "containerSet", "data", "plugin"} {
		if t.Get(typ) != nil {
			tp.Type = typ
			break
		}
	}
	in := t.Get("inputs")
	tp.Inputs = params(in.Get("parameters"))
	if in != nil {
		tp.inputsAt = in.Range
	}
	for _, o := range items(t.Get("outputs").Get("parameters")) {
		tp.Outputs = append(tp.Outputs, "parameters."+o.Get("name").Str())
	}
	for _, o := range items(t.Get("outputs").Get("artifacts")) {
		tp.Outputs = append(tp.Outputs, "artifacts."+o.Get("name").Str())
	}
	switch tp.Type {
	case "container", "script":
		tp.Image = t.Get(tp.Type).Get("image").Str()
	case "resource":
		tp.Action = t.Get("resource").Get("action").Str()
	case "steps":
		for _, g := range items(t.Get("steps")) {
			var group []*Call
			if g.Kind == yamlkit.KindSeq {
				for _, st := range g.Items {
					group = append(group, readCall(st))
				}
			} else if g.Kind == yamlkit.KindMap {
				group = []*Call{readCall(g)} // a step outside a group: tolerated
			}
			tp.Steps = append(tp.Steps, group)
		}
	case "dag":
		for _, task := range items(t.Get("dag").Get("tasks")) {
			tp.Tasks = append(tp.Tasks, readCall(task))
		}
	}
	tp.Daemon = t.Get("daemon").Str() == "true"
	return tp
}

func readCall(n *yamlkit.Node) *Call {
	c := &Call{
		Name:     n.Get("name").Str(),
		Template: n.Get("template").Str(),
		Args:     params(n.Get("arguments").Get("parameters")),
		Depends:  n.Get("depends").Str(),
		When:     n.Get("when").Str(),
		Range:    n.Range,
		node:     n,
		OnExit:   n.Get("onExit").Str(),
	}
	if pr := n.Pair("template"); pr != nil && pr.Value != nil {
		c.TemplateAt = pr.Value.Range
	}
	if pr := n.Pair("depends"); pr != nil && pr.Value != nil {
		c.DependsAt = pr.Value.Range
	}
	for _, d := range items(n.Get("dependencies")) {
		c.Dependencies = append(c.Dependencies, d.Str())
		c.DepsAt = append(c.DepsAt, d.Range)
	}
	if ref := n.Get("templateRef"); ref != nil {
		c.Ref = &TemplateRef{
			Name:         ref.Get("name").Str(),
			Template:     ref.Get("template").Str(),
			ClusterScope: ref.Get("clusterScope").Str() == "true",
			Range:        ref.Range,
		}
		if pr := ref.Pair("name"); pr != nil && pr.Value != nil {
			c.Ref.NameAt = pr.Value.Range
		}
		if pr := ref.Pair("template"); pr != nil && pr.Value != nil {
			c.Ref.TemplateAt = pr.Value.Range
		}
	}
	if wi := n.Get("withItems"); wi != nil {
		c.Items = items(wi)
		if c.Items == nil {
			c.Items = []*yamlkit.Node{} // withItems: [] runs nothing
		}
	}
	c.WithParam = n.Get("withParam").Str()
	if seq := n.Get("withSequence"); seq != nil {
		c.Sequence = &Sequence{seq.Get("count").Str(), seq.Get("start").Str(), seq.Get("end").Str(), seq.Get("format").Str()}
	}
	if co := n.Get("continueOn"); co != nil {
		var on []string
		for _, k := range []string{"failed", "error"} {
			if co.Get(k).Str() == "true" {
				on = append(on, k)
			}
		}
		c.ContinueOn = strings.Join(on, ", ")
	}
	return c
}

func params(n *yamlkit.Node) []Param {
	var out []Param
	for _, p := range items(n) {
		pm := Param{Name: p.Get("name").Str(), Range: p.Range}
		if v := p.Get("value"); v != nil {
			pm.Value, pm.HasValue = scalarOrText(v), true
		}
		if v := p.Get("default"); v != nil {
			pm.Default, pm.HasDef = scalarOrText(v), true
		}
		pm.Enum = scalars(p.Get("enum"))
		if vf := p.Get("valueFrom"); vf != nil {
			switch {
			case vf.Get("configMapKeyRef") != nil:
				r := vf.Get("configMapKeyRef")
				pm.cm = &cmRef{r.Get("name").Str(), r.Get("key").Str()}
				pm.From = "ConfigMap " + pm.cm.name + "/" + pm.cm.key
			case vf.Get("supplied") != nil:
				pm.From = "supplied when resumed"
			case vf.Get("parameter") != nil:
				pm.From = vf.Get("parameter").Str()
			case vf.Get("path") != nil:
				pm.From = "file " + vf.Get("path").Str()
			case vf.Get("expression") != nil:
				pm.From = "expression " + vf.Get("expression").Str()
			case vf.Get("jsonPath") != nil:
				pm.From = "jsonPath " + vf.Get("jsonPath").Str()
			default:
				pm.From = "valueFrom"
			}
		}
		out = append(out, pm)
	}
	return out
}

// scalarOrText is a scalar's value, or a collection written as YAML
// (Argo accepts JSON-ish values for parameters).
func scalarOrText(n *yamlkit.Node) string {
	if n.Kind == yamlkit.KindScalar {
		if n.Tag == yamlkit.TagNull {
			return ""
		}
		return n.Value
	}
	return yamlkit.ValueText(n)
}

func items(n *yamlkit.Node) []*yamlkit.Node {
	if n == nil || n.Kind != yamlkit.KindSeq {
		return nil
	}
	return n.Items
}

func scalars(n *yamlkit.Node) []string {
	var out []string
	if n != nil && n.Kind == yamlkit.KindScalar && n.Value != "" {
		return []string{n.Value}
	}
	for _, it := range items(n) {
		if it.Kind == yamlkit.KindScalar {
			out = append(out, it.Value)
		}
	}
	return out
}
