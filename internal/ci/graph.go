package ci

import (
	"fmt"
	"strings"
)

// Caption is a job's second line: what and where it runs.
func (j *Job) Caption() string {
	var parts []string
	switch {
	case j.Uses != "":
		parts = append(parts, j.Uses)
	case j.Runner != "":
		parts = append(parts, j.Runner)
	}
	switch {
	case j.Matrix != nil && j.Matrix.Runtime != "":
		parts = append(parts, "matrix at run time")
	case j.Matrix != nil:
		parts = append(parts, fmt.Sprintf("matrix × %d", len(j.Matrix.Combos)))
	case len(j.Instances) > 1:
		parts = append(parts, fmt.Sprintf("× %d", len(j.Instances)))
	}
	if j.When != "" && j.When != "on_success" {
		parts = append(parts, j.When)
	}
	return strings.Join(parts, " · ")
}

// Mermaid draws the pipeline's jobs as a Mermaid flowchart, one
// subgraph per stage.
func Mermaid(p *Pipeline) string {
	var b strings.Builder
	b.WriteString("flowchart LR\n")
	ids := map[string]string{}
	for i, j := range p.Jobs {
		ids[j.ID] = fmt.Sprintf("j%d", i)
	}
	node := func(j *Job, indent string) {
		label := mermaidText(j.Name)
		if c := j.Caption(); c != "" {
			label += "<br/><small>" + mermaidText(c) + "</small>"
		}
		open, close := "[", "]"
		switch {
		case j.Kind == "trigger" || j.Kind == "reusable":
			open, close = "[[", "]]"
		case j.When == "manual":
			open, close = "([", "])"
		}
		fmt.Fprintf(&b, "%s%s%s\"%s\"%s\n", indent, ids[j.ID], open, label, close)
	}
	staged := map[string]bool{}
	for i, s := range p.Stages {
		if s.Name == "" || len(s.Jobs) == 0 {
			continue
		}
		fmt.Fprintf(&b, "  subgraph s%d[\"%s\"]\n", i, mermaidText(s.Name))
		for _, id := range s.Jobs {
			if j := p.Job(id); j != nil {
				node(j, "    ")
				staged[id] = true
			}
		}
		b.WriteString("  end\n")
	}
	for _, j := range p.Jobs {
		if !staged[j.ID] {
			node(j, "  ")
		}
	}
	for _, e := range p.Edges {
		if e.Label != "" && e.Label != "stage" {
			fmt.Fprintf(&b, "  %s -->|%s| %s\n", ids[e.From], e.Label, ids[e.To])
		} else {
			fmt.Fprintf(&b, "  %s --> %s\n", ids[e.From], ids[e.To])
		}
	}
	return b.String()
}

func mermaidText(s string) string {
	return strings.NewReplacer(`"`, "#quot;", "<", "#lt;", ">", "#gt;", "|", "#124;").Replace(s)
}

// Dot draws the pipeline's jobs in Graphviz DOT, stages as clusters.
func Dot(p *Pipeline) string {
	var b strings.Builder
	name := p.Name
	if name == "" {
		name = p.File
	}
	fmt.Fprintf(&b, "digraph %q {\n  rankdir=LR;\n  node [shape=box, style=rounded, fontname=\"sans-serif\"];\n", name)
	node := func(j *Job, indent string) {
		label := j.Name
		if c := j.Caption(); c != "" {
			label += "\n" + c
		}
		attrs := ""
		if j.When == "manual" {
			attrs = ", style=\"rounded,dashed\""
		}
		fmt.Fprintf(&b, "%s%q [label=%q%s];\n", indent, j.ID, label, attrs)
	}
	staged := map[string]bool{}
	for i, s := range p.Stages {
		if s.Name == "" || len(s.Jobs) == 0 {
			continue
		}
		fmt.Fprintf(&b, "  subgraph cluster_%d {\n    label=%q;\n", i, s.Name)
		for _, id := range s.Jobs {
			if j := p.Job(id); j != nil {
				node(j, "    ")
				staged[id] = true
			}
		}
		b.WriteString("  }\n")
	}
	for _, j := range p.Jobs {
		if !staged[j.ID] {
			node(j, "  ")
		}
	}
	for _, e := range p.Edges {
		if e.Label != "" && e.Label != "stage" {
			fmt.Fprintf(&b, "  %q -> %q [label=%q];\n", e.From, e.To, e.Label)
		} else {
			fmt.Fprintf(&b, "  %q -> %q;\n", e.From, e.To)
		}
	}
	b.WriteString("}\n")
	return b.String()
}
