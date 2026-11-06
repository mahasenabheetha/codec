package web

import (
	"net/http"
	"path"
	"slices"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/commands"
)

// maxCommandLine bounds a line to explain: a command, or a few, not a
// script.
const maxCommandLine = 64 << 10

type commandPart struct {
	Start   int              `json:"start"` // UTF-16 offsets, as in the UI
	End     int              `json:"end"`
	Kind    string           `json:"kind"`
	Text    string           `json:"text"`
	Summary string           `json:"summary,omitempty"`
	Danger  *commands.Danger `json:"danger,omitempty"`
	Ref     string           `json:"ref,omitempty"`
}

type commandSegment struct {
	From    int              `json:"from"`
	To      int              `json:"to"`
	Tool    string           `json:"tool,omitempty"`
	Path    []string         `json:"path"`
	Summary string           `json:"summary,omitempty"`
	Danger  *commands.Danger `json:"danger,omitempty"`
	Use     *commandUse      `json:"use,omitempty"`
}

// commandUse is commands.Use with UTF-16 offsets.
type commandUse struct {
	Start     int                `json:"start"`
	End       int                `json:"end"`
	ToolStart int                `json:"toolStart"`
	Flags     []commands.FlagUse `json:"flags"`
	Args      []string           `json:"args"`
	Keep      []commands.Span    `json:"keep"`
}

// GET /api/v2/commands/tools: the tools codec can explain and build.
func (s *Server) handleCommandTools(w http.ResponseWriter, r *http.Request) {
	tools, err := commands.Tools()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tools": tools})
}

// GET /api/v2/commands/tools/{name}: a tool's whole reference, for the
// builder's form.
func (s *Server) handleCommandTool(w http.ResponseWriter, r *http.Request) {
	tool, err := commands.Lookup(r.PathValue("name"))
	switch {
	case err != nil:
		writeError(w, http.StatusInternalServerError, err.Error())
	case tool == nil:
		writeError(w, http.StatusNotFound, "codec doesn't know this tool")
	default:
		writeJSON(w, http.StatusOK, tool)
	}
}

// POST /api/v2/commands/explain {line}: what every part of a command
// line does (decision 104).
func (s *Server) handleCommandExplain(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Line string `json:"line"`
	}
	if !decode(w, r, &req) {
		return
	}
	if len(req.Line) > maxCommandLine {
		writeError(w, http.StatusUnprocessableEntity, "that is more than a command line; paste one command, or a few")
		return
	}
	ex := commands.Explain(req.Line)
	parts := make([]commandPart, 0, len(ex.Parts))
	u := utf16Offsets{text: req.Line}
	for _, p := range ex.Parts {
		parts = append(parts, commandPart{u.at(p.Start), u.at(p.End), p.Kind, p.Text, p.Summary, p.Danger, p.Ref})
	}
	segs := make([]commandSegment, 0, len(ex.Segments))
	for _, sg := range ex.Segments {
		v := commandSegment{sg.From, sg.To, sg.Tool, append([]string{}, sg.Path...), sg.Summary, sg.Danger, nil}
		if u := sg.Use; u != nil {
			c := utf16Offsets{text: req.Line} // its own walk: these offsets go back before the parts'
			v.Use = &commandUse{Start: c.at(u.Start), ToolStart: c.at(u.ToolStart), Flags: u.Flags, Args: u.Args, Keep: []commands.Span{}}
			for _, k := range u.Keep {
				v.Use.Keep = append(v.Use.Keep, commands.Span{Start: c.at(k.Start), End: c.at(k.End)})
			}
			v.Use.End = c.at(u.End)
		}
		segs = append(segs, v)
	}
	writeJSON(w, http.StatusOK, map[string]any{"parts": parts, "segments": segs})
}

// maxSearchQuery bounds a cheat-sheet query: a question, not a page.
const maxSearchQuery = 500

// GET /api/v2/commands/search?q=&tool=: the cheat sheet. Recipes,
// commands and flags that do what q asks, best first, each with why it
// matched; with no q, the recipes to browse.
func (s *Server) handleCommandSearch(w http.ResponseWriter, r *http.Request) {
	q, tool := r.URL.Query().Get("q"), r.URL.Query().Get("tool")
	if len(q) > maxSearchQuery {
		writeError(w, http.StatusUnprocessableEntity, "ask in a few words; the cheat sheet searches by purpose, not by pasted text")
		return
	}
	results, err := commands.Search(q, tool, 30)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if results == nil {
		results = []commands.Result{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

// commandSuggestions are values the open folder offers the builder: the
// namespaces, resources and labels of its Kubernetes YAML, and its Helm
// charts.
type commandSuggestions struct {
	Namespaces []string          `json:"namespaces"`
	Resources  []commandResource `json:"resources"`
	Labels     []string          `json:"labels"`
	Charts     []string          `json:"charts"`
}

type commandResource struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
}

// maxSuggestions bounds each list; a folder of thousands of resources
// needs a filter, not a longer menu.
const maxSuggestions = 500

// GET /api/v2/commands/suggest: values for the builder from the open
// folder; empty lists when none is open.
func (s *Server) handleCommandSuggest(w http.ResponseWriter, r *http.Request) {
	out := commandSuggestions{Namespaces: []string{}, Resources: []commandResource{}, Labels: []string{}, Charts: []string{}}
	ws := s.current()
	if ws == nil {
		writeJSON(w, http.StatusOK, out)
		return
	}
	objs, err := folderObjects(r.Context(), ws, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	ns, labels := map[string]bool{}, map[string]bool{}
	for _, o := range objs {
		if o.Kind == "Namespace" && o.Name != "" {
			ns[o.Name] = true
		}
		if o.Namespace != "" {
			ns[o.Namespace] = true
		}
		if o.Name != "" && len(out.Resources) < maxSuggestions {
			out.Resources = append(out.Resources, commandResource{o.Kind, o.Name, o.Namespace})
		}
		for k, v := range o.Labels {
			labels[k+"="+v] = true
		}
	}
	if files, _, err := ws.Files(r.Context()); err == nil {
		for _, f := range files {
			if f.Type == "helm-chart" && len(out.Charts) < maxSuggestions {
				out.Charts = append(out.Charts, "./"+path.Dir(f.Path))
			}
		}
	}
	out.Namespaces = sortedKeys(ns)
	out.Labels = sortedKeys(labels)
	slices.SortFunc(out.Resources, func(a, b commandResource) int {
		return strings.Compare(a.Kind+"/"+a.Name, b.Kind+"/"+b.Name)
	})
	writeJSON(w, http.StatusOK, out)
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out[:min(len(out), maxSuggestions)]
}
