package web

import (
	"bytes"
	"errors"
	"io"
	"net/http"

	"github.com/mahasenabheetha/codec/v2/internal/ansiblelog"
)

// maxLog bounds a log sent to the analyzer: whole pipeline logs run to
// tens of MB, so this endpoint takes the text as the raw body (no JSON
// escaping) with its own limit (decision 76).
const maxLog = 64 << 20

// logSlots lets two logs be analyzed at once; a big log costs a few
// hundred MB while it is read, so a third waits.
var logSlots = make(chan struct{}, 2)

// POST /api/v2/ansible/log, body: the log as text. A whole Ansible log
// read into runs, plays, tasks and results, with the output around
// them. Nothing is stored (decision 19); no folder is needed.
func (s *Server) handleAnsibleLog(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxLog))
	if err != nil {
		if errors.As(err, new(*http.MaxBytesError)) {
			writeError(w, http.StatusRequestEntityTooLarge, "the log is over 64 MB; cut it to the part that matters")
			return
		}
		writeError(w, http.StatusBadRequest, "reading the log: "+err.Error())
		return
	}
	if bytes.IndexByte(body[:min(len(body), 8000)], 0) >= 0 {
		writeError(w, http.StatusUnprocessableEntity, "this doesn't look like a text log")
		return
	}
	select {
	case logSlots <- struct{}{}:
	case <-r.Context().Done():
		return
	}
	// The slot is held until the analysis ends, even if the client
	// stopped waiting for it.
	a, err := cancellable(r.Context(), func() (*ansiblelog.Analysis, error) {
		defer func() { <-logSlots }()
		return ansiblelog.Analyze(string(body)), nil
	})
	if err != nil {
		return // the client gave up
	}
	writeJSON(w, http.StatusOK, a)
}
