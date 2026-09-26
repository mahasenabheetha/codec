package argo

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Check reports problems in one file's workflow specs that the file
// alone can tell: templates that don't exist, depends expressions that
// don't parse or name unknown tasks, inputs used but not declared, and
// call sites that miss an input or pass one the template doesn't have.
// templateRef calls are checked by CheckRefs, which needs an index.
func Check(f *yamlkit.File, specs []*Spec) []yamlkit.Diagnostic {
	var out []yamlkit.Diagnostic
	report := func(sev yamlkit.Severity, code string, r yamlkit.Range, msg, hint string) {
		out = append(out, yamlkit.Diagnostic{Severity: sev, Code: code, Message: msg, Hint: hint, Source: "argo", Range: r})
	}
	for _, s := range specs {
		if s.Ref == nil && len(s.Templates) > 0 {
			for _, e := range []struct{ key, name string }{{"entrypoint", s.Entrypoint}, {"onExit", s.OnExit}} {
				if e.name != "" && s.Template(e.name) == nil {
					report(yamlkit.SeverityError, "argo-template-missing", valueRange(s.node, e.key), fmt.Sprintf("The %s %s isn't a template of %s", e.key, e.name, s.Name), templateHint(s, e.name))
				}
			}
		}
		for _, t := range s.Templates {
			for _, group := range t.Steps {
				for _, c := range group {
					checkCall(s, t, c, nil, report)
				}
			}
			names := map[string]bool{}
			for _, c := range t.Tasks {
				names[c.Name] = true
			}
			for _, c := range t.Tasks {
				checkCall(s, t, c, names, report)
			}
			checkExprs(f, s, t, report)
		}
	}
	return out
}

type reporter func(sev yamlkit.Severity, code string, r yamlkit.Range, msg, hint string)

func checkCall(s *Spec, t *Template, c *Call, tasks map[string]bool, report reporter) {
	if c.Ref == nil && c.Template != "" {
		if target := s.Template(c.Template); target == nil {
			report(yamlkit.SeverityError, "argo-template-missing", c.TemplateAt, fmt.Sprintf("No template %s in %s", c.Template, s.Name), templateHint(s, c.Template))
		} else {
			checkArgs(c, target, report)
		}
	}
	if tasks == nil {
		return
	}
	if c.Depends != "" && len(c.Dependencies) > 0 {
		report(yamlkit.SeverityError, "argo-depends", c.DependsAt, "Task "+c.Name+" sets both depends and dependencies", "Argo rejects this; keep one (depends can say everything dependencies can).")
	}
	for i, d := range c.Dependencies {
		if !tasks[d] {
			report(yamlkit.SeverityError, "argo-unknown-task", c.DepsAt[i], fmt.Sprintf("%s isn't a task of this DAG", d), taskHint(t, d))
		}
	}
	if c.Depends == "" {
		return
	}
	dep, err := ParseDepends(c.Depends)
	if err != nil {
		report(yamlkit.SeverityError, "argo-depends", c.DependsAt, "depends: "+err.Error(), `Write task names joined by && and ||, e.g. "a && (b.Failed || c.Skipped)".`)
		return
	}
	for _, term := range dep.Terms() {
		if !tasks[term.Task] {
			report(yamlkit.SeverityError, "argo-unknown-task", c.DependsAt, fmt.Sprintf("%s isn't a task of this DAG", term.Task), taskHint(t, term.Task))
		}
	}
}

// checkArgs compares a call's arguments with the target's inputs.
func checkArgs(c *Call, target *Template, report reporter) {
	for _, a := range c.Args {
		if indexOf(target.Inputs, a.Name) < 0 {
			report(yamlkit.SeverityWarning, "argo-argument-unused", a.Range, fmt.Sprintf("Template %s has no input %s", target.Name, a.Name), "Argo ignores it. Check the spelling, or declare the input.")
		}
	}
	for _, in := range target.Inputs {
		if !in.HasValue && !in.HasDef && in.From == "" && indexOf(c.Args, in.Name) < 0 {
			report(yamlkit.SeverityError, "argo-input-missing", cmpRange(c.TemplateAt, c.Range), fmt.Sprintf("%s doesn't pass input %s that template %s needs", c.Name, in.Name, target.Name), "Add it under arguments.parameters, or give the input a default.")
		}
	}
}

// CheckRefs checks the templateRef calls of one file's specs against
// an index of the folder.
func CheckRefs(specs []*Spec, ix *Index) []yamlkit.Diagnostic {
	var out []yamlkit.Diagnostic
	report := func(sev yamlkit.Severity, code string, r yamlkit.Range, msg, hint string) {
		out = append(out, yamlkit.Diagnostic{Severity: sev, Code: code, Message: msg, Hint: hint, Source: "argo", Range: r})
	}
	for _, s := range specs {
		if s.Ref != nil && ix.Lookup(s.Ref.Kind(), s.Ref.Name) == nil {
			report(yamlkit.SeverityWarning, "argo-ref-missing", s.Ref.Range, s.Ref.Kind()+" "+s.Ref.Name+" isn't in this folder", "It may be installed in the cluster; open the folder that has it to check.")
		}
		for _, t := range s.Templates {
			for _, c := range t.calls() {
				if c.Ref == nil {
					continue
				}
				spec, target, reason := ix.RefTarget(c.Ref)
				switch {
				case spec == nil:
					report(yamlkit.SeverityWarning, "argo-ref-missing", cmpRange(c.Ref.NameAt, c.Ref.Range), reason, "It may be installed in the cluster; open the folder that has it to check.")
				case target == nil:
					report(yamlkit.SeverityError, "argo-ref-missing", cmpRange(c.Ref.TemplateAt, c.Ref.Range), reason, templateHint(spec, c.Ref.Template))
				default:
					checkArgs(c, target, report)
				}
			}
		}
	}
	return out
}

func (t *Template) calls() []*Call {
	out := append([]*Call(nil), t.Tasks...)
	for _, g := range t.Steps {
		out = append(out, g...)
	}
	return out
}

var reArgoVar = regexp.MustCompile(`\{\{\s*(inputs\.parameters|workflow\.parameters|steps|tasks)\.([A-Za-z0-9_-]+)`)

// checkExprs finds {{inputs.parameters.x}}, {{steps.x…}} and
// {{tasks.x…}} that name nothing in their template, and undeclared
// workflow parameters in a Workflow.
func checkExprs(f *yamlkit.File, s *Spec, t *Template, report reporter) {
	steps := map[string]bool{}
	for _, c := range t.calls() {
		steps[c.Name] = true
	}
	for _, e := range f.Expressions {
		if e.Syntax != "argo" || !t.Range.Contains(e.Range.Start) {
			continue
		}
		for _, m := range reArgoVar.FindAllStringSubmatch(e.Text, -1) {
			root, name := m[1], m[2]
			switch root {
			case "inputs.parameters":
				if indexOf(t.Inputs, name) < 0 {
					report(yamlkit.SeverityError, "argo-input-undeclared", e.Range, fmt.Sprintf("Template %s has no input %s", t.Name, name), "Declare it under inputs.parameters, or fix the name.")
				}
			case "workflow.parameters":
				// A WorkflowTemplate may be called from other workflows,
				// whose parameters these are; only a Workflow is sure.
				if (s.Kind == "Workflow" || s.Kind == "CronWorkflow") && s.Ref == nil && indexOf(s.Arguments, name) < 0 {
					report(yamlkit.SeverityWarning, "argo-parameter-undeclared", e.Range, fmt.Sprintf("%s has no workflow parameter %s", s.Name, name), "Declare it under spec.arguments.parameters.")
				}
			case "steps", "tasks":
				if (root == "steps") != (t.Type == "steps") || len(t.calls()) == 0 {
					continue // e.g. {{tasks.x}} in a leaf template: not ours to judge
				}
				if !steps[name] {
					report(yamlkit.SeverityError, "argo-unknown-task", e.Range, fmt.Sprintf("%s isn't a %s of template %s", name, strings.TrimSuffix(root, "s"), t.Name), "")
				}
			}
		}
	}
}

func templateHint(s *Spec, name string) string {
	var names []string
	for _, t := range s.Templates {
		names = append(names, t.Name)
	}
	if best := closest(name, names); best != "" {
		return "Did you mean " + best + "?"
	}
	if len(names) > 0 && len(names) <= 8 {
		return "Templates: " + strings.Join(names, ", ")
	}
	return ""
}

func taskHint(t *Template, name string) string {
	var names []string
	for _, c := range t.calls() {
		names = append(names, c.Name)
	}
	if best := closest(name, names); best != "" {
		return "Did you mean " + best + "?"
	}
	return ""
}

// closest returns the candidate within two edits of s, if any.
func closest(s string, cands []string) string {
	best, bestD := "", 3
	for _, c := range cands {
		if d := distance(s, c); d < bestD && d < len(s) {
			best, bestD = c, d
		}
	}
	return best
}

func distance(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

func valueRange(n *yamlkit.Node, key string) yamlkit.Range {
	if pr := n.Pair(key); pr != nil && pr.Value != nil {
		return pr.Value.Range
	}
	return n.Range
}

func cmpRange(a, b yamlkit.Range) yamlkit.Range {
	if a == (yamlkit.Range{}) {
		return b
	}
	return a
}
