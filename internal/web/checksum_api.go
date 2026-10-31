package web

import (
	"errors"
	"net/http"

	"github.com/mahasenabheetha/codec/v2/internal/encode"
)

// maxChecksum bounds a file sent to be checksummed. It is read as a
// stream and never kept, so the bound only stops a runaway upload.
const maxChecksum = 64 << 30

// POST /api/v2/checksum, body: a file's bytes. Every digest of it,
// read once as a stream and never kept (decision 19); no folder needed.
func (s *Server) handleChecksum(w http.ResponseWriter, r *http.Request) {
	d, n, err := encode.Checksum(http.MaxBytesReader(w, r.Body, maxChecksum))
	if err != nil {
		if errors.As(err, new(*http.MaxBytesError)) {
			writeError(w, http.StatusRequestEntityTooLarge, "the file is over 64 GB")
			return
		}
		writeError(w, http.StatusBadRequest, "reading the file: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"digests": d, "size": n})
}

// GET /api/v2/checksum?path=: every digest of a file in the open folder,
// streamed through the root (so nothing outside it is read).
func (s *Server) handleChecksumFile(w http.ResponseWriter, r *http.Request) {
	ws := s.current()
	if ws == nil {
		writeError(w, http.StatusConflict, "no folder is open")
		return
	}
	f, _, err := ws.Open(r.URL.Query().Get("path"))
	if err != nil {
		he := readError(err)
		writeError(w, he.status, he.msg)
		return
	}
	defer f.Close()
	d, n, err := encode.Checksum(f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "reading the file: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"digests": d, "size": n})
}
