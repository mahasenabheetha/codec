package web

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/mahasenabheetha/codec/v2/internal/logsrc"
	"github.com/mahasenabheetha/codec/v2/internal/search"
)

// refView is a file and line reference in one of an entry's texts. The
// offsets count UTF-16 code units, as JavaScript strings do, so the UI
// can cut the text it shows without converting.
type refView struct {
	Start int    `json:"start"`
	End   int    `json:"end"`
	Path  string `json:"path"`
	Line  int    `json:"line"`
	Col   int    `json:"col,omitempty"`
}

// entryRefs finds the references in an entry's texts and, when a
// folder is open, the files each may mean (decision 99, T1). Sources
// is keyed by the reference's path; a path with no file in the folder
// gets an empty list, so the UI can say so.
func (s *Server) entryRefs(ctx context.Context, v *entryView) {
	texts := map[string]string{"message": v.Message, "error": v.Error, "caller": v.Caller, "stack": v.Stack, "raw": v.Raw}
	v.Refs = map[string][]refView{}
	paths := map[string]bool{}
	for key, text := range texts {
		refs := logsrc.Find(text)
		if len(refs) == 0 {
			continue
		}
		out := make([]refView, 0, len(refs))
		u := utf16Offsets{text: text}
		for _, r := range refs {
			out = append(out, refView{Start: u.at(r.Start), End: u.at(r.End), Path: r.Path, Line: r.Line, Col: r.Col})
			paths[r.Path] = true
		}
		v.Refs[key] = out
	}
	ws := s.current()
	if ws == nil || len(paths) == 0 {
		return
	}
	ix := logsrc.NewIndex(wsFiles(ctx, ws))
	v.Sources = map[string][]logsrc.Match{}
	for p := range paths {
		v.Sources[p] = append([]logsrc.Match{}, ix.Resolve(p)...)
	}
}

// utf16Offsets turns byte offsets into UTF-16 ones. Offsets are asked
// for in increasing order (Find returns refs in order, Start before
// End), so it walks the text once.
type utf16Offsets struct {
	text      string
	byteAt, n int
}

func (u *utf16Offsets) at(b int) int {
	if b < u.byteAt { // out of order: start again
		u.byteAt, u.n = 0, 0
	}
	for u.byteAt < b {
		r, size := utf8.DecodeRuneInString(u.text[u.byteAt:])
		u.byteAt += size
		u.n += utf16Len(r)
	}
	return u.n
}

func utf16Len(r rune) int {
	if r >= 0x10000 {
		return 2
	}
	return 1
}

// findSourceLimit bounds the matches of a search from a log: the
// drawer lists them by eye.
const findSourceLimit = 500

// POST /api/v2/logs/find-source {text}: where a piece of a log line
// appears in the open folder (decision 99, T2), searched literally with
// logsrc.Excludes left out.
func (s *Server) handleFindSource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
	}
	if !decode(w, r, &req) {
		return
	}
	text := strings.TrimSpace(req.Text)
	if text == "" {
		writeError(w, http.StatusUnprocessableEntity, search.ErrEmpty.Error())
		return
	}
	if len(text) > logsrc.MaxFragment {
		writeError(w, http.StatusUnprocessableEntity, "select a shorter piece of the line to search for")
		return
	}
	ws := s.current()
	if ws == nil {
		writeError(w, http.StatusConflict, "no folder is open")
		return
	}
	sr, err := search.Compile(search.Options{Query: text, Globs: logsrc.Excludes, Limit: findSourceLimit})
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()
	res, err := sr.Run(ctx, wsFiles(ctx, ws), ws.ReadText)
	if errors.Is(err, context.Canceled) {
		return // the client gave up on this search
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"matches":   res.Matches,
		"truncated": res.Truncated,
		"timedOut":  errors.Is(err, context.DeadlineExceeded),
	})
}
