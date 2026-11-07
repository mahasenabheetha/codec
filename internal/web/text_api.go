package web

import (
	"net/http"

	"github.com/mahasenabheetha/codec/v2/internal/names"
	"github.com/mahasenabheetha/codec/v2/internal/quote"
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

// maxQuoteValue bounds a value to quote: a value, not a file.
const maxQuoteValue = 64 << 10

// GET /api/v2/text/quote/targets: where a value can be quoted for.
func (s *Server) handleQuoteTargets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"targets": quote.Targets})
}

// POST /api/v2/text/quote {value, target, unquote, layers}: the value
// written for the target, every way it can be, with why (decision 104);
// with unquote, a written value read back; with layers, the value
// quoted through each in turn.
func (s *Server) handleQuote(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Value   string        `json:"value"`
		Target  string        `json:"target"`
		Unquote bool          `json:"unquote"`
		Layers  []quote.Layer `json:"layers"`
	}
	if !decode(w, r, &req) {
		return
	}
	if len(req.Value) > maxQuoteValue {
		writeError(w, http.StatusUnprocessableEntity, "that is more than a value; quote a value, not a file")
		return
	}
	if req.Unquote {
		v, err := quote.Unquote(req.Target, req.Value)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"value": v})
		return
	}
	res, err := quote.Quote(req.Target, req.Value)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	steps, err := quote.Layers(req.Value, req.Layers)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"forms": res.Forms, "notes": res.Notes, "steps": steps})
}
