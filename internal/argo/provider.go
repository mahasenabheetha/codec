package argo

import (
	"fmt"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Provider is the Argo Workflows lens in the provider registry: the
// built-in detection, plus an outline of templates by type, go to
// definition for template names, depends terms and parameters, hover
// that explains them, and diagnostics.
type Provider struct {
	provider.Provider
}

// Register installs the lenses in reg, replacing the built-in
// "argo-workflows" and "argocd" detectors.
func Register(reg *provider.Registry) {
	if base := reg.Lookup("argo-workflows"); base != nil {
		if _, done := base.(Provider); !done {
			reg.Register(Provider{base})
		}
	}
	if base := reg.Lookup("argocd"); base != nil {
		if _, done := base.(CDProvider); !done {
			reg.Register(CDProvider{base})
		}
	}
}

func init() { Register(provider.Default) }

// Symbols labels templates with their type (and the entrypoint), and
// steps and tasks with what they run.
func (p Provider) Symbols(d *yamlkit.Document) []provider.Symbol {
	syms := provider.Outline(d)
	if d == nil || d.Root == nil {
		return syms
	}
	f := &yamlkit.File{Docs: []*yamlkit.Document{d}}
	specs := Read("", nil, f)
	for _, s := range specs {
		labelSpec(syms, d.Root, s)
	}
	return syms
}

// labelSpec finds the outline entries of s's templates and relabels
// them. The outline mirrors the tree, so entries are found by range.
func labelSpec(syms []provider.Symbol, root *yamlkit.Node, s *Spec) {
	for _, t := range s.Templates {
		sym := symbolAt(syms, t.Range)
		if sym == nil {
			continue
		}
		sym.Name = "Template " + t.Name
		detail := []string{t.Type}
		if t.Name == s.Entrypoint {
			detail = append(detail, "entrypoint")
		}
		if t.Name == s.OnExit {
			detail = append(detail, "exit handler")
		}
		if t.Image != "" {
			detail = append(detail, t.Image)
		}
		sym.Detail = strings.Join(nonEmpty(detail), " · ")
		for _, c := range t.calls() {
			cs := symbolAt(sym.Children, c.Range)
			if cs == nil {
				continue
			}
			label := "Step "
			if t.Type == "dag" {
				label = "Task "
			}
			cs.Name = label + c.Name
			cs.Detail = "→ " + callTarget(c)
		}
	}
}

func callTarget(c *Call) string {
	if c.Ref != nil {
		return c.Ref.String()
	}
	return c.Template
}

// symbolAt finds the symbol with exactly range r, searching depth first.
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

// Diagnostics reports what the file alone can tell (see Check).
func (p Provider) Diagnostics(f *provider.File) []yamlkit.Diagnostic {
	return Check(f.YAML, Read(f.Path, f.Content, f.YAML))
}

// Place is what a position in an Argo file refers to.
type Place struct {
	Spec     *Spec
	Template *Template
	Call     *Call
	What     string // template, ref-name, ref-template, task, input, parameter, step-ref, entrypoint
	Name     string
	Expr     *yamlkit.Expression
}

// At works out what the cursor is on.
func At(f *provider.File, pos yamlkit.Pos) *Place {
	if f.YAML == nil {
		return nil
	}
	for _, s := range Read(f.Path, f.Content, f.YAML) {
		if !s.Source.Range.Contains(pos) {
			continue
		}
		pl := &Place{Spec: s}
		for _, e := range []string{"entrypoint", "onExit"} {
			if pr := s.node.Pair(e); pr != nil && pr.Value != nil && pr.Value.Range.Contains(pos) {
				pl.What, pl.Name = "template", pr.Value.Str()
				return pl
			}
		}
		for _, t := range s.Templates {
			if !t.Range.Contains(pos) {
				continue
			}
			pl.Template = t
			for i := range f.YAML.Expressions {
				e := &f.YAML.Expressions[i]
				if e.Syntax == "argo" && e.Range.Contains(pos) {
					pl.Expr = e
					if m := reArgoVar.FindStringSubmatch(e.Text); m != nil {
						pl.Name = m[2]
						switch m[1] {
						case "inputs.parameters":
							pl.What = "input"
						case "workflow.parameters":
							pl.What = "parameter"
						default:
							pl.What = "step-ref"
						}
					}
					return pl
				}
			}
			for _, c := range t.calls() {
				if !c.Range.Contains(pos) {
					continue
				}
				pl.Call = c
				switch {
				case c.TemplateAt.Contains(pos):
					pl.What, pl.Name = "template", c.Template
				case c.Ref != nil && c.Ref.NameAt.Contains(pos):
					pl.What, pl.Name = "ref-name", c.Ref.Name
				case c.Ref != nil && c.Ref.TemplateAt.Contains(pos):
					pl.What, pl.Name = "ref-template", c.Ref.Template
				case c.DependsAt.Contains(pos):
					pl.What, pl.Name = "task", wordAt(f.Content, pos)
				default:
					for i, r := range c.DepsAt {
						if r.Contains(pos) {
							pl.What, pl.Name = "task", c.Dependencies[i]
						}
					}
				}
				return pl
			}
			return pl
		}
		return pl
	}
	return nil
}

// wordAt is the task name under pos in a depends expression.
func wordAt(content []byte, pos yamlkit.Pos) string {
	src := strings.TrimPrefix(string(content), "\xEF\xBB\xBF") // offsets ignore a BOM
	i := min(pos.Offset, len(src))
	start, end := i, i
	for start > 0 && isNameByte(src[start-1]) {
		start--
	}
	for end < len(src) && isNameByte(src[end]) {
		end++
	}
	task, _, _ := strings.Cut(src[start:end], ".")
	return task
}

// Definition jumps from a template name, a depends term or a
// parameter to where it is declared, within the file. templateRef
// targets in other files are resolved by DefinitionIn.
func (p Provider) Definition(f *provider.File, pos yamlkit.Pos) []provider.Location {
	pl := At(f, pos)
	if pl == nil {
		return nil
	}
	return pl.definition(nil)
}

// DefinitionIn is Definition with an index of the folder, so
// templateRef names jump to other files.
func DefinitionIn(f *provider.File, pos yamlkit.Pos, ix *Index) []provider.Location {
	pl := At(f, pos)
	if pl == nil {
		return nil
	}
	return pl.definition(ix)
}

func (pl *Place) definition(ix *Index) []provider.Location {
	loc := func(s *Spec, r yamlkit.Range) []provider.Location {
		return []provider.Location{{Path: s.Source.File, Range: r}}
	}
	s, t := pl.Spec, pl.Template
	switch pl.What {
	case "template":
		if target := s.Template(pl.Name); target != nil {
			return loc(s, target.NameAt)
		}
	case "ref-name", "ref-template":
		if ix == nil {
			ix = NewIndex()
			ix.Specs = []*Spec{s}
		}
		spec, target, _ := ix.RefTarget(pl.Call.Ref)
		switch {
		case target != nil:
			return loc(spec, target.NameAt)
		case spec != nil:
			return loc(spec, spec.Source.Range)
		}
	case "task", "step-ref":
		for _, c := range t.calls() {
			if c.Name == pl.Name {
				return loc(s, c.Range)
			}
		}
	case "input":
		if i := indexOf(t.Inputs, pl.Name); i >= 0 {
			return loc(s, t.Inputs[i].Range)
		}
	case "parameter":
		if i := indexOf(s.Arguments, pl.Name); i >= 0 {
			return loc(s, s.Arguments[i].Range)
		}
	}
	return nil
}

// Hover explains template references, depends expressions and Argo
// variables.
func (p Provider) Hover(f *provider.File, pos yamlkit.Pos) *provider.Hover {
	pl := At(f, pos)
	if pl == nil {
		return nil
	}
	return pl.hover(nil)
}

// HoverIn is Hover with an index of the folder, for templateRef.
func HoverIn(f *provider.File, pos yamlkit.Pos, ix *Index) *provider.Hover {
	pl := At(f, pos)
	if pl == nil {
		return nil
	}
	return pl.hover(ix)
}

func (pl *Place) hover(ix *Index) *provider.Hover {
	s, c := pl.Spec, pl.Call
	switch {
	case pl.Expr != nil:
		return exprHover(pl)
	case pl.What == "template":
		target := s.Template(pl.Name)
		if target == nil {
			return nil
		}
		return templateHover(target, "Template "+target.Name, rangeOf(c, pl))
	case pl.What == "ref-name" || pl.What == "ref-template":
		if ix == nil {
			return nil
		}
		spec, target, reason := ix.RefTarget(c.Ref)
		if target == nil {
			return &provider.Hover{Range: c.Ref.Range, Title: "templateRef " + c.Ref.String(), Rows: []provider.HoverRow{{Label: "Not found", Value: reason}}}
		}
		h := templateHover(target, c.Ref.Kind()+" "+spec.Name+" · "+target.Name, c.Ref.Range)
		h.Rows = append([]provider.HoverRow{{Label: "File", Value: fmt.Sprintf("%s:%d", spec.Source.File, target.Range.Start.Line)}}, h.Rows...)
		return h
	case pl.What == "task" && c != nil && c.Depends != "":
		h := &provider.Hover{Range: c.DependsAt, Title: "depends"}
		if dep, err := ParseDepends(c.Depends); err != nil {
			h.Rows = []provider.HoverRow{{Label: "Error", Value: err.Error()}}
		} else {
			h.Rows = []provider.HoverRow{{Label: "Runs when", Value: dep.Explain()}}
			if pl.Name != "" {
				h.Rows = append(h.Rows, provider.HoverRow{Label: pl.Name, Value: "F12 jumps to the task"})
			}
		}
		return h
	}
	return nil
}

func rangeOf(c *Call, pl *Place) yamlkit.Range {
	if c != nil {
		return c.TemplateAt
	}
	if pr := pl.Spec.node.Pair("entrypoint"); pr != nil && pr.Value != nil && pr.Value.Str() == pl.Name {
		return pr.Value.Range
	}
	if pr := pl.Spec.node.Pair("onExit"); pr != nil && pr.Value != nil {
		return pr.Value.Range
	}
	return pl.Spec.node.Range
}

func templateHover(t *Template, title string, r yamlkit.Range) *provider.Hover {
	h := &provider.Hover{Range: r, Title: title, Rows: []provider.HoverRow{{Label: "Type", Value: t.Type}}}
	if t.Image != "" {
		h.Rows = append(h.Rows, provider.HoverRow{Label: "Image", Value: t.Image})
	}
	if t.Action != "" {
		h.Rows = append(h.Rows, provider.HoverRow{Label: "Action", Value: t.Action})
	}
	var in []string
	for _, p := range t.Inputs {
		switch {
		case p.HasValue:
			in = append(in, p.Name+" = "+oneLine(p.Value))
		case p.HasDef:
			in = append(in, p.Name+" = "+oneLine(p.Default))
		default:
			in = append(in, p.Name+" (required)")
		}
	}
	if len(in) > 0 {
		h.Code = strings.Join(in, "\n")
		h.Rows = append(h.Rows, provider.HoverRow{Label: "Inputs", Value: fmt.Sprint(len(in))})
	}
	if len(t.Outputs) > 0 {
		h.Rows = append(h.Rows, provider.HoverRow{Label: "Outputs", Value: strings.Join(t.Outputs, ", ")})
	}
	if n := len(t.calls()); n > 0 {
		word := "step"
		if t.Type == "dag" {
			word = "task"
		}
		h.Rows = append(h.Rows, provider.HoverRow{Label: "Runs", Value: plural(n, word)})
	}
	return h
}

// exprHover explains an Argo variable: what it names, and its value
// when the file knows it.
func exprHover(pl *Place) *provider.Hover {
	e := pl.Expr
	h := &provider.Hover{Range: e.Range, Title: "Argo expression", Rows: []provider.HoverRow{{Label: "Evaluated", Value: "at run time, by the workflow controller"}}, Code: e.Text}
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(e.Text, "{{"), "}}"))
	row := func(l, v string) { h.Rows = append(h.Rows, provider.HoverRow{Label: l, Value: v}) }
	switch pl.What {
	case "input":
		t := pl.Template
		if i := indexOf(t.Inputs, pl.Name); i >= 0 {
			in := t.Inputs[i]
			row("Input", pl.Name+" of template "+t.Name)
			switch {
			case in.HasValue:
				row("Default", oneLine(in.Value))
			case in.HasDef:
				row("Default", oneLine(in.Default))
			default:
				row("Default", "none: every caller must pass it")
			}
		} else {
			row("Input", pl.Name+" isn't declared in template "+t.Name)
		}
	case "parameter":
		row("Parameter", "workflow parameter "+pl.Name)
		if i := indexOf(pl.Spec.Arguments, pl.Name); i >= 0 && pl.Spec.Arguments[i].HasValue {
			row("Value", oneLine(pl.Spec.Arguments[i].Value)+" (unless set when submitting)")
		} else if pl.Spec.Kind == "WorkflowTemplate" || pl.Spec.Kind == "ClusterWorkflowTemplate" {
			row("Value", "from the workflow that runs this template")
		}
	case "step-ref":
		row("From", strings.SplitN(inner, ".", 2)[0]+" "+pl.Name+", once it has run")
	default:
		switch {
		case strings.HasPrefix(inner, "="):
			row("Kind", "expression (expr-lang), evaluated at run time")
		case inner == "item" || strings.HasPrefix(inner, "item."):
			row("Kind", "the current loop item (withItems / withParam)")
		case strings.HasPrefix(inner, "workflow."):
			row("Kind", "a workflow variable, set when it runs")
		}
	}
	return h
}

func oneLine(s string) string {
	s = strings.TrimSpace(s)
	if first, _, multi := strings.Cut(s, "\n"); multi {
		s = first + " …"
	}
	if r := []rune(s); len(r) > 80 {
		s = string(r[:79]) + "…"
	}
	if s == "" {
		return `""`
	}
	return s
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func nonEmpty(s []string) []string {
	var out []string
	for _, x := range s {
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}
