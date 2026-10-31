package web

import (
	"net/http"

	"github.com/mahasenabheetha/codec/v2/internal/httpkit"
)

// POST /api/v2/http/{kind}: curl (a curl command to fetch), fetch (a
// fetch call to curl), status (codes by number or words). Only text is
// rewritten: no request is sent, nothing is kept (decision 19).
func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Input string `json:"input"`
	}
	if !decode(w, r, &req) {
		return
	}
	// Lists go out as [] rather than null, so the UI can count them.
	reply := func(q *httpkit.Request, out string) {
		if q.Headers == nil {
			q.Headers = []httpkit.Header{}
		}
		if q.Notes == nil {
			q.Notes = []string{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"request": q, "output": out})
	}
	switch r.PathValue("kind") {
	case "curl":
		q, err := httpkit.ParseCurl(req.Input)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		reply(q, q.Fetch())
	case "fetch":
		q, err := httpkit.ParseFetch(req.Input)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		reply(q, q.Curl())
	case "status":
		writeJSON(w, http.StatusOK, map[string]any{"statuses": httpkit.Statuses(req.Input)})
	default:
		writeError(w, http.StatusNotFound, "unknown HTTP job "+r.PathValue("kind"))
	}
}
