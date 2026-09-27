package scaffold

import (
	_ "embed"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/mahasenabheetha/codec/v2/internal/argo"
	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Snippet is an editor snippet for some file types.
type Snippet struct {
	Label  string   `yaml:"label"`
	Types  []string `yaml:"types"`
	Detail string   `yaml:"detail"`
	Body   string   `yaml:"body"`
}

//go:embed snippets.yaml
var snippetsYAML []byte

var snippets = func() []Snippet {
	var out []Snippet
	if err := yaml.Unmarshal(snippetsYAML, &out); err != nil {
		panic(err) // a broken built-in is a bug; tests catch it
	}
	for i := range out {
		out[i].Body = strings.TrimRight(out[i].Body, "\n")
	}
	return out
}()

// reSnippetLine is a line where a snippet can go: indentation, an
// optional list dash, and the start of a word.
var reSnippetLine = regexp.MustCompile(`^(\s*)(- )?([A-Za-z][\w-]*)?$`)

// reField is a CodeMirror placeholder: ${1:text}, ${0}.
var reField = regexp.MustCompile(`\$\{\d+(?::([^{}]*))?\}`)

// Snippets returns the snippets for a file of type typ at pos, as
// completions whose Insert is a CodeMirror snippet template. They are
// offered only where a key or list item starts.
func Snippets(f *provider.File, typ string, pos yamlkit.Pos) []provider.Completion {
	lines := strings.Split(string(f.Content), "\n")
	if pos.Line < 1 || pos.Line > len(lines) {
		return nil
	}
	line := strings.TrimRight(lines[pos.Line-1], "\r")
	before := []rune(line)
	before = before[:min(len(before), pos.Col-1)]
	m := reSnippetLine.FindStringSubmatch(string(before))
	if m == nil {
		return nil
	}
	dash, word := m[2] != "", m[3]
	start := yamlkit.PosAt(f.Content, pos.Line, pos.Col-len([]rune(word)))
	rng := yamlkit.Range{Start: start, End: pos}

	var out []provider.Completion
	add := func(label, detail, body string) {
		if dash {
			// The line already has the dash: the item's first line
			// goes after it, the rest keeps its indentation.
			body = strings.TrimPrefix(body, "- ")
		}
		out = append(out, provider.Completion{
			Label: label, Detail: "snippet", Doc: detail + "\n\n" + preview(body),
			Insert: body, Kind: "snippet", Range: rng,
		})
	}
	for _, s := range snippets {
		if slices.Contains(s.Types, typ) {
			add(s.Label, s.Detail, s.Body)
		}
	}
	if typ == "argo-workflows" && f.YAML != nil {
		if a := argo.DAGAt(f.YAML, f.Content, pos); a != nil {
			add("dag-task", "A DAG task after the last one", dagTask(a))
		}
	}
	return out
}

// dagTask is a task for the DAG around the cursor: it runs another
// template of the spec and depends on the last task.
func dagTask(a *argo.Around) string {
	tasks := a.Tasks()
	template := "template"
	for _, t := range a.Templates {
		if t != a.Template.Name {
			template = t
			break
		}
	}
	body := fmt.Sprintf("- name: ${1:task-%d}\n  template: ${2:%s}", len(tasks)+1, template)
	if len(tasks) > 0 {
		// Completion in depends: offers the other tasks by name.
		body += fmt.Sprintf("\n  depends: ${3:%s}", tasks[len(tasks)-1])
	}
	return body + "${0}"
}

// preview is the body as it will read with the defaults filled in.
func preview(body string) string {
	return reField.ReplaceAllString(body, "$1")
}
