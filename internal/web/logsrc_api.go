package web

import (
	"context"
	"unicode/utf8"

	"github.com/mahasenabheetha/codec/v2/internal/logsrc"
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
