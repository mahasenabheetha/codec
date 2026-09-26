package argo

import (
	"fmt"
	"strings"
)

// Node returns the node with id, or nil.
func (r *Result) Node(id string) *Node {
	for _, n := range r.Nodes {
		if n.ID == id {
			return n
		}
	}
	return nil
}

// Container finds the node whose children a graph shows: by id, by
// template name (the first node running it), or the entrypoint.
func (r *Result) Container(name string) *Node {
	if name == "" {
		if len(r.Roots) == 0 {
			return nil
		}
		return r.Node(r.Roots[0])
	}
	if n := r.Node(name); n != nil {
		return n
	}
	for _, n := range r.Nodes {
		if n.Template == name && len(n.Children) > 0 {
			return n
		}
	}
	return nil
}

// Caption is a node's second line: what it runs.
func (n *Node) Caption() string {
	var parts []string
	if n.Ref != "" {
		parts = append(parts, n.Ref)
	} else if n.Template != "" && n.Template != n.Name {
		parts = append(parts, n.Template)
	}
	if n.Type != "" {
		parts = append(parts, n.Type)
	}
	if n.Error != "" {
		parts = append(parts, "missing")
	}
	return strings.Join(parts, " · ")
}

// Mermaid draws a container's children as a Mermaid flowchart.
func Mermaid(c *Node, r *Result) string {
	var b strings.Builder
	b.WriteString("flowchart LR\n")
	ids := map[string]string{}
	for i, id := range c.Children {
		ids[id] = fmt.Sprintf("n%d", i)
		n := r.Node(id)
		label := mermaidText(n.Name)
		if cap := n.Caption(); cap != "" {
			label += "<br/><small>" + mermaidText(cap) + "</small>"
		}
		open, close := "[", "]"
		switch {
		case n.Type == "dag" || n.Type == "steps":
			open, close = "[[", "]]"
		case n.Type == "suspend":
			open, close = "([", "])"
		case n.Error != "":
			open, close = "[/", "/]"
		}
		fmt.Fprintf(&b, "  %s%s\"%s\"%s\n", ids[id], open, label, close)
	}
	for _, e := range c.Edges {
		if e.Label != "" {
			fmt.Fprintf(&b, "  %s -->|%s| %s\n", ids[e.From], mermaidText(e.Label), ids[e.To])
		} else {
			fmt.Fprintf(&b, "  %s --> %s\n", ids[e.From], ids[e.To])
		}
	}
	return b.String()
}

func mermaidText(s string) string {
	return strings.NewReplacer(`"`, "#quot;", "<", "#lt;", ">", "#gt;", "|", "#124;").Replace(s)
}

// Dot draws a container's children in Graphviz DOT.
func Dot(c *Node, r *Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "digraph %q {\n  rankdir=LR;\n  node [shape=box, style=rounded, fontname=\"sans-serif\"];\n", c.Template)
	for _, id := range c.Children {
		n := r.Node(id)
		label := n.Name
		if cap := n.Caption(); cap != "" {
			label += "\n" + cap
		}
		attrs := ""
		switch {
		case n.Type == "dag" || n.Type == "steps":
			attrs = ", peripheries=2"
		case n.Error != "":
			attrs = ", style=\"rounded,dashed\""
		}
		fmt.Fprintf(&b, "  %q [label=%q%s];\n", id, label, attrs)
	}
	for _, e := range c.Edges {
		if e.Label != "" {
			fmt.Fprintf(&b, "  %q -> %q [label=%q];\n", e.From, e.To, e.Label)
		} else {
			fmt.Fprintf(&b, "  %q -> %q;\n", e.From, e.To)
		}
	}
	b.WriteString("}\n")
	return b.String()
}
