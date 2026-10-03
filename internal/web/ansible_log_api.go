package web

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net/http"
	"runtime/debug"
	"unsafe"

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
	// The slot is taken before the body is read and held until the
	// answer is written, or until the analysis ends if the client gave
	// up: at most two logs are in memory at once.
	select {
	case logSlots <- struct{}{}:
	case <-r.Context().Done():
		return
	}
	held := true
	defer func() {
		if held {
			<-logSlots
		}
	}()
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxLog))
	if err != nil {
		if errors.As(err, new(*http.MaxBytesError)) {
			writeError(w, http.StatusRequestEntityTooLarge, "the log is over 64 MB; cut it to the part that matters")
			return
		}
		writeError(w, http.StatusBadRequest, "reading the log: "+err.Error())
		return
	}
	if len(body) == 0 || bytes.IndexByte(body[:min(len(body), 8000)], 0) >= 0 {
		writeError(w, http.StatusUnprocessableEntity, "this doesn't look like a text log")
		return
	}
	done := make(chan *ansiblelog.Analysis, 1)
	go func() {
		// A panic would end the server: report it as an error instead.
		defer func() {
			if p := recover(); p != nil {
				log.Printf("panic: %v\n%s", p, debug.Stack())
				done <- nil
			}
		}()
		// body is never written again, so the text can share its bytes.
		done <- ansiblelog.Analyze(unsafe.String(&body[0], len(body)))
	}()
	select {
	case a := <-done:
		if a == nil {
			writeError(w, http.StatusInternalServerError, "the log could not be read (internal error)")
			return
		}
		writeJSON(w, http.StatusOK, a)
	case <-r.Context().Done():
		held = false
		go func() {
			<-done
			<-logSlots
		}()
	}
}
