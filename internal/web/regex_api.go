package web

import (
	"errors"
	"net/http"

	"github.com/mahasenabheetha/codec/v2/internal/regex"
)

// maxRegexText bounds the test text: positions take 8 bytes per byte of it.
const maxRegexText = 4 << 20

type regexRequest struct {
	regex.Options
	Text    string  `json:"text"`
	Replace *string `json:"replace"` // a replace preview when set
}

// POST /api/v2/regex: test a pattern on text, preview a replace, and
// explain the pattern. A pattern that doesn't compile is an answer, not
// a failed request: "error" says why (and "hint" what would take it),
// and the explanation is still there. Nothing is stored.
func (s *Server) handleRegex(w http.ResponseWriter, r *http.Request) {
	var req regexRequest
	if !decode(w, r, &req) {
		return
	}
	resp := map[string]any{"explain": regex.Explain(req.Pattern, req.Style, req.Flags)}
	fail := func(err error) {
		var e *regex.Error
		if errors.As(err, &e) {
			resp["error"] = e
		} else {
			resp["error"] = &regex.Error{Message: err.Error()}
		}
		writeJSON(w, http.StatusOK, resp)
	}
	if len(req.Text) > maxRegexText {
		fail(&regex.Error{Message: "the test text is over 4 MB; test on a part of it"})
		return
	}
	res, err := regex.Run(req.Options, req.Text)
	if err != nil {
		fail(err)
		return
	}
	resp["result"] = res
	// A pattern that ran out of time would only run out again.
	if req.Replace != nil && !res.TimedOut {
		out, err := regex.Replace(req.Options, req.Text, *req.Replace)
		if err != nil {
			fail(err)
			return
		}
		resp["replaced"] = out
	}
	writeJSON(w, http.StatusOK, resp)
}
