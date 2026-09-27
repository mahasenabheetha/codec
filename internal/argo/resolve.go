package argo

import (
	"cmp"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Options tune Resolve.
type Options struct {
	// Params sets workflow parameters, like `argo submit -p name=value`.
	Params map[string]string
	// MaxItems caps the loop iterations expanded per step (default 20).
	MaxItems int
	// Bodies adds each template's text with the known values filled in.
	Bodies bool
}

// Result is a workflow as it would run: every step with the template it
// runs and the inputs it gets.
type Result struct {
	Workflow *Spec     `json:"workflow"`
	Base     *Spec     `json:"base,omitempty"` // what workflowTemplateRef points at
	Params   []Value   `json:"params"`         // workflow.parameters
	Nodes    []*Node   `json:"nodes"`
	Roots    []string  `json:"roots"` // the entrypoint's node, then onExit's
	Problems []Problem `json:"problems"`
}

// Problem is something that would make the workflow fail or misbehave.
type Problem struct {
	Severity yamlkit.Severity `json:"severity"`
	Code     string           `json:"code"`
	Message  string           `json:"message"`
	Hint     string           `json:"hint,omitempty"`
	Node     string           `json:"node,omitempty"` // node id
	Source   *Source          `json:"source,omitempty"`
}

// Node is one step, task or loop iteration, with the template it runs.
type Node struct {
	ID         string   `json:"id"` // path of names from the root: "main/build(0:linux)"
	Parent     string   `json:"parent,omitempty"`
	Name       string   `json:"name"`
	Template   string   `json:"template,omitempty"`
	Ref        string   `json:"ref,omitempty"` // templateRef as name/template
	Type       string   `json:"type,omitempty"`
	Group      int      `json:"group,omitempty"` // steps: 1-based parallel group
	Inputs     []Value  `json:"inputs"`
	Children   []string `json:"children,omitempty"`
	Edges      []Edge   `json:"edges,omitempty"` // between children
	When       *Cond    `json:"when,omitempty"`
	Depends    string   `json:"depends,omitempty"` // in words
	Loop       string   `json:"loop,omitempty"`
	Item       *Value   `json:"item,omitempty"`
	ContinueOn string   `json:"continueOn,omitempty"`
	Image      string   `json:"image,omitempty"`
	Action     string   `json:"action,omitempty"`
	Daemon     bool     `json:"daemon,omitempty"`
	Outputs    []string `json:"outputs,omitempty"`
	Def        *Source  `json:"def,omitempty"`  // where the template is written
	Call       *Source  `json:"call,omitempty"` // where the step or task is written
	Error      string   `json:"error,omitempty"`
	Note       string   `json:"note,omitempty"`
	Body       []Part   `json:"body,omitempty"`
}

// Edge orders two children: To runs after From.
type Edge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label,omitempty"`
}

// Value is a parameter value with what could be resolved of it.
type Value struct {
	Name  string   `json:"name,omitempty"`
	Text  string   `json:"text"`            // expressions not resolvable stay as written
	State string   `json:"state"`           // known, runtime, missing, template
	Parts []Part   `json:"parts,omitempty"` // when not one literal
	From  string   `json:"from,omitempty"`
	Enum  []string `json:"enum,omitempty"`
	Hint  string   `json:"hint,omitempty"`
}

// Part is a piece of a value: literal text, a substituted expression,
// or an expression only known at run time.
type Part struct {
	Text string `json:"text"`
	Kind string `json:"kind,omitempty"` // "" literal, resolved, runtime, expr, missing, template
	Expr string `json:"expr,omitempty"` // the expression a resolved part replaced
	Note string `json:"note,omitempty"`
}

// Cond is a `when` condition.
type Cond struct {
	Value  Value  `json:"value"`
	Result string `json:"result,omitempty"` // "true", "false"; "" = known at run time
	Error  string `json:"error,omitempty"`
}

// Value states, worst last.
const (
	StateKnown    = "known"
	StateRuntime  = "runtime"
	StateTemplate = "template" // a Helm/Go template expression: render the chart first
	StateMissing  = "missing"
)

var stateRank = map[string]int{StateKnown: 0, StateRuntime: 1, StateTemplate: 2, StateMissing: 3}

// Resolve walks wf from its entrypoint, resolving templates (local and
// templateRef through ix) and parameters. It never fails: problems are
// reported in the result.
func Resolve(ix *Index, wf *Spec, opts Options) *Result {
	if opts.MaxItems <= 0 {
		opts.MaxItems = 20
	}
	r := &resolver{ix: ix, opts: opts, res: &Result{Workflow: wf, Params: []Value{}, Nodes: []*Node{}, Roots: []string{}, Problems: []Problem{}}, byID: map[string]*Node{}}
	owner, entry, onExit, args := wf, wf.Entrypoint, wf.OnExit, wf.Arguments
	if wf.Ref != nil {
		kind := "WorkflowTemplate"
		if wf.Ref.ClusterScope {
			kind = "ClusterWorkflowTemplate"
		}
		if base := ix.Lookup(kind, wf.Ref.Name); base != nil {
			r.res.Base, owner = base, base
			entry, onExit = cmp.Or(entry, base.Entrypoint), cmp.Or(onExit, base.OnExit)
			args = mergeParams(base.Arguments, wf.Arguments)
		} else {
			r.problem(Problem{Severity: yamlkit.SeverityWarning, Code: "missing-template", Message: kind + " " + wf.Ref.Name + " isn't in this folder", Hint: "Open the folder that has it to see the templates it runs.", Source: at(wf, wf.Ref.Range)})
			return r.res
		}
	}
	r.globals(wf, owner, entry)
	r.workflowParams(wf, args)

	if entry == "" {
		r.problem(Problem{Severity: yamlkit.SeverityError, Code: "no-entrypoint", Message: wf.Kind + " " + wf.Name + " has no entrypoint", Hint: "Set spec.entrypoint to the template that starts the workflow.", Source: &wf.Source})
	} else {
		sc := &scope{wf: r.wfParams, globals: r.global}
		// The workflow's arguments feed the entrypoint's inputs by name.
		byName := map[string]Value{}
		for _, v := range r.res.Params {
			v.From = "workflow argument " + v.Name
			byName[v.Name] = v
		}
		r.root(owner, entry, "entrypoint", byName, sc)
	}
	if onExit != "" {
		r.root(owner, onExit, "exit handler", nil, &scope{wf: r.wfParams, globals: r.global})
	}
	return r.res
}

type resolver struct {
	ix       *Index
	opts     Options
	res      *Result
	byID     map[string]*Node
	wfParams map[string]Value
	global   map[string]Value
}

type scope struct {
	wf, globals map[string]Value
	inputs      map[string]Value
	item        *Value
	itemNode    *yamlkit.Node
}

func (s *scope) with(inputs map[string]Value, item *Value, itemNode *yamlkit.Node) *scope {
	return &scope{wf: s.wf, globals: s.globals, inputs: inputs, item: item, itemNode: itemNode}
}

func (r *resolver) problem(p Problem) { r.res.Problems = append(r.res.Problems, p) }

func at(s *Spec, rg yamlkit.Range) *Source {
	return &Source{File: s.Source.File, Doc: s.Source.Doc, Line: rg.Start.Line, Range: rg}
}

func mergeParams(base, over []Param) []Param {
	out := append([]Param(nil), base...)
	for _, p := range over {
		i := indexOf(out, p.Name)
		if i < 0 {
			out = append(out, p)
		} else {
			out[i] = p
		}
	}
	return out
}

func indexOf(ps []Param, name string) int {
	for i, p := range ps {
		if p.Name == name {
			return i
		}
	}
	return -1
}

// globals are the workflow.* variables known before the run.
func (r *resolver) globals(wf, owner *Spec, entry string) {
	r.global = map[string]Value{}
	lit := func(name, v, from string) {
		if v != "" && !strings.Contains(v, "{{") {
			r.global[name] = Value{Text: v, State: StateKnown, From: from}
		}
	}
	if wf.Kind == "Workflow" && !wf.Generated {
		lit("workflow.name", wf.Name, "metadata.name")
	}
	lit("workflow.namespace", wf.Namespace, "metadata.namespace")
	lit("workflow.serviceAccountName", cmp.Or(wf.Account, owner.Account), "spec.serviceAccountName")
	lit("workflow.mainEntrypoint", entry, "spec.entrypoint")
}

// workflowParams resolves spec.arguments.parameters: your values
// first, then event data (for Sensor-submitted workflows), ConfigMaps
// and the written values.
func (r *resolver) workflowParams(wf *Spec, args []Param) {
	r.wfParams = map[string]Value{}
	for name := range r.opts.Params {
		if indexOf(args, name) < 0 {
			r.problem(Problem{Severity: yamlkit.SeverityWarning, Code: "unknown-parameter", Message: fmt.Sprintf("%q isn't a parameter of %s", name, wf.Name), Hint: "Argo ignores parameters the workflow doesn't declare."})
		}
	}
	sc := &scope{wf: map[string]Value{}, globals: r.global}
	for _, p := range args {
		v := Value{Name: p.Name, Enum: p.Enum}
		ev, fromEvent := EventData{}, false
		if wf.Via != nil {
			ev, fromEvent = wf.Via.Events[indexOf(wf.Arguments, p.Name)]
		}
		if set, ok := r.opts.Params[p.Name]; ok {
			v.Text, v.State, v.From = set, StateKnown, "set by you"
		} else if fromEvent {
			expr := "event " + ev.Dependency + " · " + ev.Key
			v.Text, v.State, v.From = "{{"+expr+"}}", StateRuntime, "Sensor "+wf.Via.Sensor+" fills it from "+expr
			v.Parts = []Part{{Text: v.Text, Kind: "runtime", Note: v.From}}
			if len(ev.Allowed) > 0 {
				v.Hint = "the sensor's filter accepts " + strings.Join(ev.Allowed, " | ")
			}
		} else if p.cm != nil {
			if cv, ok := r.ix.configMap(p.cm.name, p.cm.key); ok {
				v.Text, v.State, v.From = cv, StateKnown, "ConfigMap "+p.cm.name+"/"+p.cm.key
			} else {
				v.Text, v.State, v.From = "{{configmap "+p.cm.name+"/"+p.cm.key+"}}", StateRuntime, "ConfigMap "+p.cm.name+"/"+p.cm.key+", read when the workflow starts (not in this folder)"
			}
		} else if p.HasValue {
			v = r.subst(p.Value, sc)
			v.Name, v.Enum, v.From = p.Name, p.Enum, "spec.arguments"
		} else {
			v.State, v.From = StateMissing, "no value: pass it when submitting (-p "+p.Name+"=…)"
			r.problem(Problem{Severity: yamlkit.SeverityInfo, Code: "parameter-unset", Message: fmt.Sprintf("parameter %s has no value", p.Name), Hint: "Set it in the parameters panel, or with -p " + p.Name + "=… when submitting.", Source: at(wf, p.Range)})
		}
		if len(v.Enum) > 0 && v.State == StateKnown && !contains(v.Enum, v.Text) {
			r.problem(Problem{Severity: yamlkit.SeverityError, Code: "parameter-enum", Message: fmt.Sprintf("parameter %s is %q, not one of %s", p.Name, v.Text, strings.Join(v.Enum, ", ")), Source: at(wf, p.Range)})
		}
		r.res.Params = append(r.res.Params, v)
		r.wfParams[p.Name] = v
		sc.wf[p.Name] = v
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

// root adds a top-level node: the entrypoint or the exit handler.
func (r *resolver) root(owner *Spec, name, role string, args map[string]Value, sc *scope) {
	t := owner.Template(name)
	n := &Node{ID: name, Name: name, Template: name, Inputs: []Value{}}
	if role == "exit handler" {
		n.ID, n.Note = "onExit:"+name, "runs when the workflow ends (exit handler)"
	}
	r.add(n)
	r.res.Roots = append(r.res.Roots, n.ID)
	if t == nil {
		n.Error = "no template " + name + " in " + owner.Kind + " " + owner.Name
		r.problem(Problem{Severity: yamlkit.SeverityError, Code: "missing-template", Message: fmt.Sprintf("the %s %s isn't a template of %s", role, name, owner.Name), Node: n.ID, Source: &owner.Source})
		return
	}
	inputs := r.bind(owner, t, args, sc, n, "the workflow")
	r.expand(n, owner, t, sc.with(inputs, nil, nil), []string{owner.ID() + "#" + t.Name})
}

func (r *resolver) add(n *Node) {
	r.res.Nodes = append(r.res.Nodes, n)
	r.byID[n.ID] = n
}

// bind works out a template's inputs from what the caller passed.
func (r *resolver) bind(owner *Spec, t *Template, args map[string]Value, sc *scope, n *Node, caller string) map[string]Value {
	inputs := map[string]Value{}
	defScope := sc.with(map[string]Value{}, sc.item, sc.itemNode)
	for _, in := range t.Inputs {
		var v Value
		switch a, ok := args[in.Name]; {
		case ok:
			v = a
		case in.HasValue || in.HasDef:
			v = r.subst(in.Default, defScope)
			if in.HasValue {
				v = r.subst(in.Value, defScope)
			}
			v.From = "default in template " + t.Name
		case in.From != "":
			v = Value{Text: "{{" + in.From + "}}", State: StateRuntime, From: in.From}
			if in.cm != nil {
				if cv, ok := r.ix.configMap(in.cm.name, in.cm.key); ok {
					v = Value{Text: cv, State: StateKnown, From: in.From}
				}
			}
		case n.Call == nil:
			// The entrypoint of a template library: callers using
			// templateRef pass it; submitting it directly would fail.
			v = Value{State: StateMissing, From: "not in spec.arguments, and no default"}
			r.problem(Problem{Severity: yamlkit.SeverityWarning, Code: "input-missing", Message: fmt.Sprintf("submitted on its own, %s needs parameter %s (input of %s, no default)", owner.Name, in.Name, t.Name), Hint: "Fine if it only runs through templateRef (callers pass it); add it to spec.arguments to submit it directly.", Node: n.ID, Source: at(owner, t.Range)})
		default:
			v = Value{State: StateMissing, From: "not passed, and no default"}
			r.problem(Problem{Severity: yamlkit.SeverityError, Code: "input-missing", Message: fmt.Sprintf("%s doesn't pass input %s that template %s needs", caller, in.Name, t.Name), Hint: "Pass it under arguments.parameters, or give the input a default value.", Node: n.ID, Source: n.Call})
		}
		v.Name, v.Enum = in.Name, in.Enum
		if len(in.Enum) > 0 && v.State == StateKnown && !contains(in.Enum, v.Text) {
			r.problem(Problem{Severity: yamlkit.SeverityError, Code: "input-enum", Message: fmt.Sprintf("input %s of %s is %q, not one of %s", in.Name, t.Name, v.Text, strings.Join(in.Enum, ", ")), Node: n.ID, Source: n.Call})
		}
		inputs[in.Name] = v
		n.Inputs = append(n.Inputs, v)
	}
	return inputs
}

// expand fills a node from its template: type facts, children and the
// edges between them.
func (r *resolver) expand(n *Node, owner *Spec, t *Template, sc *scope, stack []string) {
	n.Type, n.Image, n.Action, n.Daemon, n.Outputs = t.Type, t.Image, t.Action, t.Daemon, t.Outputs
	n.Def = at(owner, t.Range)
	if n.Image != "" {
		n.Image = r.subst(n.Image, sc).Text
	}
	if r.opts.Bodies {
		n.Body = r.body(owner, t, sc)
	}
	if len(stack) > 40 || len(r.res.Nodes) > 3000 {
		n.Note = "not expanded further (too deep)"
		return
	}
	switch t.Type {
	case "steps":
		var prev []string
		for gi, group := range t.Steps {
			var cur []string
			for _, c := range group {
				for _, id := range r.call(n, owner, c, sc, stack, gi+1) {
					for _, p := range prev {
						n.Edges = append(n.Edges, Edge{From: p, To: id})
					}
					cur = append(cur, id)
				}
			}
			prev = cur
		}
	case "dag":
		ids := map[string][]string{}
		for _, c := range t.Tasks {
			ids[c.Name] = r.call(n, owner, c, sc, stack, 0)
		}
		for _, c := range t.Tasks {
			dep, text := r.dagDeps(n, owner, c, ids)
			if dep == nil {
				continue
			}
			for _, id := range ids[c.Name] {
				if child := r.byID[id]; child != nil {
					child.Depends = text
				}
			}
			for _, term := range dep.Terms() {
				for _, from := range ids[term.Task] {
					for _, to := range ids[c.Name] {
						n.Edges = append(n.Edges, Edge{From: from, To: to, Label: term.Label()})
					}
				}
			}
		}
	}
}

// dagDeps is a task's dependencies as an expression, with problems for
// tasks that don't exist.
func (r *resolver) dagDeps(n *Node, owner *Spec, c *Call, ids map[string][]string) (*Dep, string) {
	if c.Depends != "" && len(c.Dependencies) > 0 {
		r.problem(Problem{Severity: yamlkit.SeverityError, Code: "depends-and-dependencies", Message: "task " + c.Name + " sets both depends and dependencies", Hint: "Argo rejects this; keep one (depends can say everything dependencies can).", Node: n.ID, Source: at(owner, c.Range)})
	}
	var dep *Dep
	switch {
	case c.Depends != "":
		d, err := ParseDepends(c.Depends)
		if err != nil {
			r.problem(Problem{Severity: yamlkit.SeverityError, Code: "depends-syntax", Message: fmt.Sprintf("task %s: depends %q: %v", c.Name, c.Depends, err), Node: n.ID, Source: at(owner, c.DependsAt)})
			return nil, ""
		}
		dep = d
	case len(c.Dependencies) > 0:
		dep = DependenciesDep(c.Dependencies)
	default:
		return nil, ""
	}
	for _, term := range dep.Terms() {
		if _, ok := ids[term.Task]; !ok {
			r.problem(Problem{Severity: yamlkit.SeverityError, Code: "unknown-task", Message: fmt.Sprintf("task %s depends on %s, which isn't a task of this DAG", c.Name, term.Task), Node: n.ID, Source: at(owner, cmp.Or(c.DependsAt, c.Range))})
		}
	}
	return dep, "runs when " + dep.Explain()
}

// call adds the nodes for one step or task: one per loop iteration.
func (r *resolver) call(parent *Node, owner *Spec, c *Call, sc *scope, stack []string, group int) []string {
	target, t, reason := owner, owner.Template(c.Template), ""
	ref := ""
	switch {
	case c.Ref != nil:
		ref = c.Ref.Name + "/" + c.Ref.Template
		target, t, reason = r.ix.RefTarget(c.Ref)
	case c.Template == "":
		reason = "step " + c.Name + " names no template"
	case t == nil:
		reason = "no template " + c.Template + " in " + owner.Kind + " " + owner.Name
	}

	iters := []iteration{{}}
	loop := ""
	switch {
	case c.Items != nil:
		iters = nil
		for i, it := range c.Items {
			if i >= r.opts.MaxItems {
				loop = fmt.Sprintf("withItems: %d items, showing %d", len(c.Items), r.opts.MaxItems)
				break
			}
			v := r.subst(itemText(it), sc)
			v.From = fmt.Sprintf("withItems[%d]", i)
			iters = append(iters, iteration{fmt.Sprintf("(%d:%s)", i, v.Text), &v, it})
		}
		if loop == "" {
			loop = fmt.Sprintf("withItems: %d items", len(c.Items))
		}
	case c.WithParam != "":
		src := r.subst(c.WithParam, sc)
		loop = "withParam: one iteration per element of " + src.Text
		v := Value{Text: "{{item}}", State: StateRuntime, From: "an element of " + src.Text, Parts: []Part{{Text: "{{item}}", Kind: "runtime", Note: "an element of " + src.Text}}}
		if src.State == StateKnown {
			if vals, ok := jsonList(src.Text); ok {
				iters = nil
				for i, x := range vals {
					if i >= r.opts.MaxItems {
						break
					}
					xv := Value{Text: x, State: StateKnown, From: fmt.Sprintf("withParam[%d]", i)}
					iters = append(iters, iteration{fmt.Sprintf("(%d:%s)", i, x), &xv, nil})
				}
				loop = fmt.Sprintf("withParam: %d items", len(vals))
				break
			}
		}
		iters = []iteration{{"(…)", &v, nil}}
	case c.Sequence != nil:
		iters, loop = r.sequence(c.Sequence, sc)
	}

	var ids []string
	for _, it := range iters {
		n := &Node{
			ID:       parent.ID + "/" + c.Name + it.suffix,
			Parent:   parent.ID,
			Name:     c.Name + it.suffix,
			Template: c.Template,
			Ref:      ref,
			Group:    group,
			Inputs:   []Value{},
			Loop:     loop,
			Item:     it.item,
			Call:     at(owner, c.Range),
		}
		if c.ContinueOn != "" {
			n.ContinueOn = "continues on " + c.ContinueOn
		}
		if c.Ref != nil {
			n.Template = c.Ref.Template
		}
		r.add(n)
		parent.Children = append(parent.Children, n.ID)
		ids = append(ids, n.ID)

		isc := sc.with(sc.inputs, it.item, it.node)
		args := map[string]Value{}
		for _, a := range c.Args {
			v := r.subst(a.Value, isc)
			if !a.HasValue && a.From != "" {
				v = Value{Text: "{{" + a.From + "}}", State: StateRuntime, From: a.From}
			}
			v.Name = a.Name
			if v.From == "" {
				v.From = "passed by " + c.Name
			}
			args[a.Name] = v
		}
		if c.When != "" {
			n.When = r.cond(c.When, isc)
			if n.When.Result == "false" {
				n.Note = "skipped with these values (when is false)"
			}
		}
		if reason != "" {
			n.Error = reason
			sev := yamlkit.SeverityError
			if c.Ref != nil && target == nil {
				sev = yamlkit.SeverityWarning // it may be installed in the cluster
			}
			r.problem(Problem{Severity: sev, Code: "missing-template", Message: "step " + c.Name + ": " + reason, Node: n.ID, Source: at(owner, cmp.Or(c.TemplateAt, c.Range))})
			continue
		}
		for _, a := range c.Args {
			if indexOf(t.Inputs, a.Name) < 0 {
				r.problem(Problem{Severity: yamlkit.SeverityWarning, Code: "argument-unused", Message: fmt.Sprintf("step %s passes %s, but template %s has no input %s", c.Name, a.Name, t.Name, a.Name), Hint: "Argo ignores it. Check the spelling, or declare the input.", Node: n.ID, Source: n.Call})
			}
		}
		inputs := r.bind(target, t, args, isc, n, "step "+c.Name)
		key := target.ID() + "#" + t.Name
		if contains(stack, key) {
			n.Type, n.Def = t.Type, at(target, t.Range)
			n.Note = "calls " + t.Name + " again (recursion): not expanded"
			continue
		}
		// A templateRef'd template runs with its own spec's templates,
		// but workflow.* still means the submitted workflow.
		r.expand(n, target, t, sc.with(inputs, nil, nil), append(stack[:len(stack):len(stack)], key))
	}
	return ids
}

// iteration is one run of a looped step: its name suffix and item.
type iteration struct {
	suffix string
	item   *Value
	node   *yamlkit.Node
}

func (r *resolver) sequence(seq *Sequence, sc *scope) ([]iteration, string) {
	num := func(s string) (int, bool) {
		v := r.subst(s, sc)
		n, err := strconv.Atoi(strings.TrimSpace(v.Text))
		return n, v.State == StateKnown && err == nil
	}
	start, end := 0, -1
	switch {
	case seq.Count != "":
		c, ok := num(seq.Count)
		if !ok {
			v := Value{Text: "{{item}}", State: StateRuntime, From: "withSequence"}
			return []iteration{{"(…)", &v, nil}}, "withSequence: count " + r.subst(seq.Count, sc).Text
		}
		if seq.Start != "" {
			if s, ok := num(seq.Start); ok {
				start = s
			}
		}
		end = start + c - 1
	default:
		s, ok1 := num(cmp.Or(seq.Start, "0"))
		e, ok2 := num(seq.End)
		if !ok1 || !ok2 {
			v := Value{Text: "{{item}}", State: StateRuntime, From: "withSequence"}
			return []iteration{{"(…)", &v, nil}}, "withSequence: known at run time"
		}
		start, end = s, e
	}
	var out []iteration
	step := 1
	if end < start {
		step = -1
	}
	for i, x := 0, start; len(out) < r.opts.MaxItems; i, x = i+1, x+step {
		if step > 0 && x > end || step < 0 && x < end {
			break
		}
		s := strconv.Itoa(x)
		if seq.Format != "" {
			s = fmt.Sprintf(seq.Format, x)
		}
		v := Value{Text: s, State: StateKnown, From: "withSequence"}
		out = append(out, iteration{fmt.Sprintf("(%d:%s)", i, s), &v, nil})
	}
	return out, fmt.Sprintf("withSequence: %d to %d", start, end)
}

// jsonList reads a JSON list literal like ["a","b"] or [1,2].
func jsonList(s string) ([]string, bool) {
	f := yamlkit.Parse([]byte(s))
	if len(f.Docs) != 1 || f.Docs[0].Root == nil || f.Docs[0].Root.Kind != yamlkit.KindSeq || f.HasErrors() {
		return nil, false
	}
	var out []string
	for _, it := range f.Docs[0].Root.Items {
		out = append(out, itemText(it))
	}
	return out, true
}

// itemText is how Argo prints a loop item in node names: a scalar as
// is, a map as key:value pairs.
func itemText(n *yamlkit.Node) string {
	if n.Kind == yamlkit.KindScalar {
		return n.Value
	}
	if n.Kind == yamlkit.KindMap {
		var parts []string
		for _, p := range n.Pairs {
			parts = append(parts, p.Key.Value+":"+itemText(p.Value))
		}
		return strings.Join(parts, ",")
	}
	return strings.ReplaceAll(yamlkit.ValueText(n), "\n", " ")
}

// cond resolves and, when possible, evaluates a `when` condition.
func (r *resolver) cond(when string, sc *scope) *Cond {
	c := &Cond{Value: r.subst(when, sc)}
	if c.Value.State != StateKnown {
		return c
	}
	ok, err := EvalWhen(c.Value.Text)
	switch {
	case err != nil:
		c.Error = "can't evaluate: " + err.Error()
	case ok:
		c.Result = "true"
	default:
		c.Result = "false"
	}
	return c
}

// body is a template's text with the known values filled in.
func (r *resolver) body(owner *Spec, t *Template, sc *scope) []Part {
	text := yamlkit.Text(owner.content, t.Range)
	if text == "" {
		return nil
	}
	// The first line starts after "- "; indent the rest back by as much.
	pad := strings.Repeat(" ", max(0, t.Range.Start.Col-1))
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i := 1; i < len(lines); i++ {
		lines[i] = strings.TrimPrefix(lines[i], pad)
	}
	v := r.subst(strings.Join(lines, "\n"), sc)
	if v.Parts == nil {
		return []Part{{Text: v.Text}}
	}
	return v.Parts
}

// --- substitution ---

var (
	// Helm's ways of writing a literal Argo expression in a template.
	reHelmRaw   = regexp.MustCompile("\\{\\{-?\\s*`([^`]*)`\\s*-?\\}\\}")
	reHelmBrace = regexp.MustCompile(`\{\{-?\s*"(\{\{|\}\})"\s*-?\}\}`)
)

// unhelm undoes Helm escaping, for Argo files that are still raw Helm
// templates: {{ `{{ inputs.parameters.x }}` }} → {{ inputs.parameters.x }}.
func unhelm(s string) string {
	if !strings.Contains(s, "{{") {
		return s
	}
	s = reHelmRaw.ReplaceAllString(s, "$1")
	return reHelmBrace.ReplaceAllString(s, "$1")
}

// subst resolves the Argo expressions in s.
func (r *resolver) subst(s string, sc *scope) Value {
	return r.substDepth(s, sc, 0)
}

func (r *resolver) substDepth(s string, sc *scope, depth int) Value {
	raw := strings.Contains(s, "{{`") || strings.Contains(s, "{{ `")
	s = unhelm(s)
	if raw && len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1] // {{ `"{{ x }}"` }}: the quotes were YAML's
	}
	var parts []Part
	lit := func(t string) {
		if t == "" {
			return
		}
		if n := len(parts); n > 0 && parts[n-1].Kind == "" {
			parts[n-1].Text += t
			return
		}
		parts = append(parts, Part{Text: t})
	}
	for {
		i := strings.Index(s, "{{")
		if i < 0 {
			lit(s)
			break
		}
		j := strings.Index(s[i+2:], "}}")
		if j < 0 {
			lit(s)
			break
		}
		lit(s[:i])
		expr := s[i : i+2+j+2]
		inner := strings.TrimSpace(s[i+2 : i+2+j])
		s = s[i+2+j+2:]
		switch {
		case strings.HasPrefix(inner, "="):
			parts = append(parts, Part{Text: expr, Kind: "expr", Note: "an expression, evaluated at run time"})
		case !isArgoVar(inner):
			parts = append(parts, Part{Text: expr, Kind: "template", Note: "a template expression: render the chart to see its value"})
		default:
			parts = append(parts, r.lookup(inner, expr, sc, depth)...)
		}
	}
	v := Value{State: StateKnown}
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(p.Text)
		st := StateKnown
		switch p.Kind {
		case "runtime", "expr":
			st = StateRuntime
		case "missing":
			st = StateMissing
		case "template":
			st = StateTemplate
		}
		if stateRank[st] > stateRank[v.State] {
			v.State = st
		}
	}
	v.Text = b.String()
	if len(parts) > 1 || len(parts) == 1 && parts[0].Kind != "" {
		v.Parts = parts
	}
	return v
}

var argoRoots = []string{"workflow.", "inputs.", "outputs.", "steps.", "tasks.", "item", "pod.", "node.", "retries", "lastRetry", "cronworkflow.", "event "}

func isArgoVar(s string) bool {
	for _, p := range argoRoots {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

// lookup resolves one variable to parts.
func (r *resolver) lookup(name, expr string, sc *scope, depth int) []Part {
	resolved := func(v Value, ok bool, what string) []Part {
		if !ok {
			return []Part{{Text: expr, Kind: "missing", Note: what + " isn't defined"}}
		}
		if v.State == StateMissing {
			return []Part{{Text: expr, Kind: "missing", Note: what + " has no value"}}
		}
		if v.Parts == nil {
			return []Part{{Text: v.Text, Kind: "resolved", Expr: expr, Note: cmp.Or(v.From, what)}}
		}
		out := make([]Part, len(v.Parts))
		for i, p := range v.Parts {
			if p.Kind == "" {
				p.Kind, p.Expr, p.Note = "resolved", expr, cmp.Or(v.From, what)
			}
			out[i] = p
		}
		return out
	}
	runtime := func(note string) []Part { return []Part{{Text: expr, Kind: "runtime", Note: note}} }

	switch {
	case strings.HasPrefix(name, "inputs.parameters."):
		p := strings.TrimPrefix(name, "inputs.parameters.")
		v, ok := sc.inputs[p]
		return resolved(v, ok, "input "+p)
	case strings.HasPrefix(name, "workflow.parameters."):
		p := strings.TrimPrefix(name, "workflow.parameters.")
		v, ok := sc.wf[p]
		return resolved(v, ok, "workflow parameter "+p)
	case name == "item":
		if sc.item == nil {
			return []Part{{Text: expr, Kind: "missing", Note: "{{item}} outside a loop"}}
		}
		return resolved(*sc.item, true, "the loop item")
	case strings.HasPrefix(name, "item."):
		if sc.item != nil && sc.item.State != StateKnown {
			return runtime("a field of the loop item")
		}
		key := strings.TrimPrefix(name, "item.")
		if f := sc.itemNode.Get(key); f != nil {
			return resolved(r.substDepth(itemText(f), sc, depth+1), true, "item."+key)
		}
		return []Part{{Text: expr, Kind: "missing", Note: "the loop item has no field " + key}}
	case strings.HasPrefix(name, "workflow."):
		if v, ok := sc.globals[name]; ok {
			return resolved(v, true, name)
		}
		return runtime("set when the workflow runs")
	case strings.HasPrefix(name, "steps."):
		step, _, _ := strings.Cut(strings.TrimPrefix(name, "steps."), ".")
		return runtime("from step " + step + ", when it has run")
	case strings.HasPrefix(name, "tasks."):
		task, _, _ := strings.Cut(strings.TrimPrefix(name, "tasks."), ".")
		return runtime("from task " + task + ", when it has run")
	case strings.HasPrefix(name, "inputs.artifacts."):
		return runtime("an input artifact's path")
	case strings.HasPrefix(name, "outputs."):
		return runtime("this template's outputs")
	case strings.HasPrefix(name, "event "):
		return runtime("event data the Sensor passes")
	}
	return runtime("set at run time")
}
