package argo

import (
	"regexp"
	"slices"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Around is the DAG around a position: the template, the task the
// position is in (nil between tasks) and the template names of the
// spec, for snippets and completion.
type Around struct {
	Template  *Template
	Task      *Call
	Templates []string
}

// Tasks returns the names of the template's other DAG tasks.
func (a *Around) Tasks() []string {
	var out []string
	for _, c := range a.Template.Tasks {
		if c != a.Task && c.Name != "" {
			out = append(out, c.Name)
		}
	}
	return out
}

// DAGAt finds the DAG template whose lines hold pos; nil if none.
// Lines, not exact ranges: a half-typed line at the end of a task
// belongs to it.
func DAGAt(f *yamlkit.File, content []byte, pos yamlkit.Pos) *Around {
	within := func(r yamlkit.Range) bool { return r.Start.Line <= pos.Line && pos.Line <= r.End.Line }
	for _, s := range Read("", content, f) {
		for _, t := range s.Templates {
			if t.Type != "dag" || !within(t.Range) {
				continue
			}
			a := &Around{Template: t}
			for _, c := range t.Tasks {
				if within(c.Range) {
					a.Task = c
				}
			}
			for _, o := range s.Templates {
				if o.Name != "" {
					a.Templates = append(a.Templates, o.Name)
				}
			}
			return a
		}
	}
	return nil
}

var (
	reDependsLine = regexp.MustCompile(`^\s*(?:- )?(depends|dependencies|template):\s*(.*)$`)
	reDepItem     = regexp.MustCompile(`^\s*-\s*[\w.-]*$`)
	reWordBefore  = regexp.MustCompile(`[\w.-]*$`)
)

// Complete offers the other tasks' names in depends and dependencies
// (the dependency picker) and the spec's template names in template:.
func (p Provider) Complete(f *provider.File, pos yamlkit.Pos) []provider.Completion {
	if f.YAML == nil {
		return nil
	}
	lines := strings.Split(string(f.Content), "\n")
	if pos.Line < 1 || pos.Line > len(lines) {
		return nil
	}
	line := lines[pos.Line-1]
	before := line[:min(len(line), byteCol(line, pos.Col))]

	key := ""
	if m := reDependsLine.FindStringSubmatch(before); m != nil {
		key = m[1]
		if key == "dependencies" && !strings.HasPrefix(strings.TrimSpace(m[2]), "[") {
			return nil // the list items come on the next lines
		}
	} else if reDepItem.MatchString(before) && underKey(lines, pos.Line, "dependencies") {
		key = "dependencies"
	}
	if key == "" {
		return nil
	}
	a := DAGAt(f.YAML, f.Content, pos)
	if a == nil || (key == "template" && a.Task == nil) {
		return nil
	}

	word := reWordBefore.FindString(before)
	start := yamlkit.PosAt(f.Content, pos.Line, pos.Col-len([]rune(word)))
	rng := yamlkit.Range{Start: start, End: pos}
	var out []provider.Completion
	if key == "template" {
		for _, name := range a.Templates {
			if name != a.Template.Name {
				out = append(out, provider.Completion{Label: name, Detail: "template", Kind: "value", Range: rng})
			}
		}
		return out
	}
	for _, name := range a.Tasks() {
		if a.Task != nil && slices.Contains(a.Task.Dependencies, name) && key == "dependencies" {
			continue
		}
		out = append(out, provider.Completion{Label: name, Detail: "task in " + a.Template.Name, Kind: "value", Range: rng})
	}
	return out
}

// underKey reports whether the list item on line (1-based) belongs to
// a "key:" line above it with less indentation.
func underKey(lines []string, line int, key string) bool {
	indent := len(lines[line-1]) - len(strings.TrimLeft(lines[line-1], " "))
	for i := line - 2; i >= 0; i-- {
		l := lines[i]
		t := strings.TrimLeft(l, " ")
		if t == "" {
			continue
		}
		in := len(l) - len(t)
		if in < indent || (in == indent && !strings.HasPrefix(t, "-")) {
			t = strings.TrimPrefix(t, "- ")
			return strings.TrimSpace(t) == key+":"
		}
	}
	return false
}

// byteCol converts a 1-based rune column to a byte offset in line.
func byteCol(line string, col int) int {
	n := 0
	for i := range line {
		if n == col-1 {
			return i
		}
		n++
	}
	return len(line)
}
