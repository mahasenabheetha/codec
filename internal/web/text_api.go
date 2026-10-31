package web

import (
	"net/http"

	"github.com/mahasenabheetha/codec/v2/internal/names"
)

// POST /api/v2/text/{kind}: case (a name in every case, its words, and
// the naming rules it passes or breaks). Nothing is kept (decision 19).
func (s *Server) handleText(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Input string `json:"input"`
	}
	if !decode(w, r, &req) {
		return
	}
	switch r.PathValue("kind") {
	case "case":
		words := names.Words(req.Input)
		if words == nil {
			words = []string{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"words": words, "cases": names.Cases(req.Input), "checks": names.Checks(req.Input)})
	default:
		writeError(w, http.StatusNotFound, "unknown text job "+r.PathValue("kind"))
	}
}
