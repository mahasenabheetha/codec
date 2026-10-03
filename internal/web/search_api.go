package web

import (
	"context"
	"errors"
	"net/http"

	"github.com/mahasenabheetha/codec/v2/internal/search"
)

// POST /api/v2/search {query, regex, matchCase, wholeWord, globs, limit}:
// text across the files the Explorer lists. A new search from the UI
// cancels the request of the previous one, which stops the scan.
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	var req search.Options
	if !decode(w, r, &req) {
		return
	}
	// The response is held and sent at once, so it stays small.
	req.Limit = min(max(req.Limit, 0), search.DefaultLimit)
	ws := s.current()
	if ws == nil {
		writeError(w, http.StatusConflict, "no folder is open")
		return
	}
	sr, err := search.Compile(req)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()
	files, _, err := ws.Files(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}
	res, err := sr.Run(ctx, paths, ws.ReadText)
	if errors.Is(err, context.Canceled) {
		return // the client gave up on this search
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"matches":   res.Matches,
		"files":     res.Files,
		"searched":  res.Searched,
		"truncated": res.Truncated,
		"timedOut":  errors.Is(err, context.DeadlineExceeded),
	})
}
