package web

import (
	"net/http"

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
		segs = append(segs, commandSegment{sg.From, sg.To, sg.Tool, append([]string{}, sg.Path...), sg.Summary, sg.Danger})
	}
	writeJSON(w, http.StatusOK, map[string]any{"parts": parts, "segments": segs})
}
