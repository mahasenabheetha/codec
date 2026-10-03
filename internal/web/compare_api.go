package web

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mahasenabheetha/codec/v2/internal/config"
	"github.com/mahasenabheetha/codec/v2/internal/query"
	"github.com/mahasenabheetha/codec/v2/internal/textdiff"
	"github.com/mahasenabheetha/codec/v2/internal/workspace"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// side is one thing to compare or query: a workspace file (its what-if
// buffer when content is set), a chart rendered with a profile, or
// pasted text (never stored, decision 19).
type side struct {
	Kind    string  `json:"kind"` // "file" (default), "helm" or "paste"
	Path    string  `json:"path"`
	Content *string `json:"content"` // file: what-if buffer, nil = disk; paste: the text
	Doc     *int    `json:"doc"`     // file, paste: only this document (0-based)
	Chart   string  `json:"chart"`   // helm
	Profile string  `json:"profile"` // helm: saved profile, "" = defaults
}

type sideText struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

// load returns a side's text and a title for it.
func (s *Server) load(ctx context.Context, ws *workspace.Workspace, sd side) (sideText, *httpError) {
	switch sd.Kind {
	case "helm":
		res, he := s.renderProfile(ctx, ws, sd.Chart, sd.Profile)
		if he != nil {
			return sideText{}, he
		}
		for _, d := range res.Diagnostics {
			if d.Severity == "error" {
				return sideText{}, &httpError{http.StatusUnprocessableEntity, "render failed: " + d.Message}
			}
		}
		title := "Helm " + res.Chart.Name + " · " + orText(sd.Profile, "defaults")
		return sideText{Title: title, Text: res.Manifest}, nil
	case "", "file", "paste":
		text := ""
		title := sd.Path
		switch {
		case sd.Kind == "paste":
			title = "Pasted"
			if sd.Content != nil {
				text = *sd.Content
			}
		case sd.Content != nil:
			text, title = *sd.Content, sd.Path+" (what-if)"
		default:
			c, err := ws.Read(sd.Path)
			if err != nil {
				return sideText{}, readError(err)
			}
			text = c.Text
		}
		if sd.Doc != nil {
			f := yamlkit.Parse([]byte(text))
			if *sd.Doc < 0 || *sd.Doc >= len(f.Docs) {
				return sideText{}, &httpError{http.StatusBadRequest, "no such document"}
			}
			d := f.Docs[*sd.Doc]
			text = strings.TrimPrefix(yamlkit.Text([]byte(text), d.Range), "---\n")
			title += " · " + orText(d.Name, "document "+strconv.Itoa(*sd.Doc+1))
		}
		return sideText{Title: title, Text: text}, nil
	}
	return sideText{}, &httpError{http.StatusBadRequest, "unknown kind " + sd.Kind}
}

// compareRequest is the body of POST /api/v2/compare.
type compareRequest struct {
	Left, Right side
	Ignore      []string `json:"ignore"`
	Mode        string   `json:"mode"` // "auto" (default), "structure" or "text"
	Text        bool     `json:"text"` // older clients: same as mode "text"
	IgnoreSpace bool     `json:"ignoreSpace"`
	IgnoreCase  bool     `json:"ignoreCase"`
}

// POST /api/v2/compare: a semantic diff, or a text diff when a side
// doesn't parse or (in auto mode) isn't a mapping or list. Two pasted
// sides need no open folder.
func (s *Server) handleCompare(w http.ResponseWriter, r *http.Request) {
	var req compareRequest
	if !decode(w, r, &req) {
		return
	}
	ws := s.current()
	if ws == nil && (req.Left.Kind != "paste" || req.Right.Kind != "paste") {
		writeError(w, http.StatusConflict, "no folder is open")
		return
	}
	left, he := s.load(r.Context(), ws, req.Left)
	if he == nil {
		var right sideText
		if right, he = s.load(r.Context(), ws, req.Right); he == nil {
			writeCompare(w, left, right, req)
			return
		}
	}
	writeError(w, he.status, he.msg)
}

// structured reports whether every document in f is a mapping or a
// list: text, logs and lone scalars compare better line by line.
func structured(f *yamlkit.File) bool {
	n := 0
	for _, d := range f.Docs {
		if d.Root == nil {
			continue
		}
		if d.Root.Kind != yamlkit.KindMap && d.Root.Kind != yamlkit.KindSeq {
			return false
		}
		n++
	}
	return n > 0
}

func writeCompare(w http.ResponseWriter, left, right sideText, req compareRequest) {
	out := map[string]any{"left": left, "right": right}
	mode := req.Mode
	if req.Text {
		mode = "text"
	}
	if mode != "text" {
		fl, fr := yamlkit.Parse([]byte(left.Text)), yamlkit.Parse([]byte(right.Text))
		if mode != "structure" && !fl.HasErrors() && !fr.HasErrors() && (!structured(fl) || !structured(fr)) {
			out["mode"], out["changes"] = "text", []yamlkit.Change{}
			out["note"] = "A side isn't a YAML or JSON mapping or list, so it is compared as text."
			textDiff(out, left, right, req)
			writeJSON(w, http.StatusOK, out)
			return
		}
		changes, err := yamlkit.Diff(fl, fr, yamlkit.DiffOptions{Ignore: req.Ignore})
		if err == nil {
			if changes == nil {
				changes = []yamlkit.Change{}
			}
			out["mode"], out["changes"] = "semantic", changes
			writeJSON(w, http.StatusOK, out)
			return
		}
		if !errors.Is(err, yamlkit.ErrUnparsed) {
			out["note"] = err.Error()
		} else {
			out["note"] = "A side has syntax errors, so it is compared as text."
		}
	}
	out["mode"], out["changes"] = "text", []yamlkit.Change{}
	textDiff(out, left, right, req)
	writeJSON(w, http.StatusOK, out)
}

// maxRows caps the side-by-side rows; the unified diff is always whole.
const maxRows = 5000

// textDiff adds the unified diff and the side-by-side rows to out.
func textDiff(out map[string]any, left, right sideText, req compareRequest) {
	o := textdiff.Options{IgnoreSpace: req.IgnoreSpace, IgnoreCase: req.IgnoreCase}
	out["diff"] = textdiff.UnifiedWith("file", left.Text, right.Text, 3, o)
	rows := textdiff.SideBySide(left.Text, right.Text, 3, o)
	if len(rows) > maxRows {
		rows, out["rowsTruncated"] = rows[:maxRows], true
	}
	if rows == nil {
		rows = []textdiff.Row{}
	}
	out["rows"] = rows
}

// GET /api/v2/compare/ignore and POST {patterns}: the saved noise filter.
func (s *Server) handleDiffIgnore(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req struct {
			Patterns []string `json:"patterns"`
		}
		if !decode(w, r, &req) {
			return
		}
		var clean []string
		for _, p := range req.Patterns {
			if p = strings.TrimSpace(p); p != "" {
				clean = append(clean, p)
			}
		}
		if err := s.opts.Config.Update(func(st *config.Settings) { st.DiffIgnore = clean }); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	patterns := s.opts.Config.Get().DiffIgnore
	if patterns == nil {
		patterns = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"patterns": patterns})
}

type queryHit struct {
	File string `json:"file,omitempty"` // workspace path; "" for a Helm render
	query.Result
}

const (
	queryLimit   = 500
	queryTimeout = 20 * time.Second
)

// POST /api/v2/query {expr, scope: file|workspace|helm, path, content, chart, profile}
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Expr  string `json:"expr"`
		Scope string `json:"scope"`
		side
	}
	if !decode(w, r, &req) {
		return
	}
	ws := s.current()
	if ws == nil {
		writeError(w, http.StatusConflict, "no folder is open")
		return
	}
	q, err := query.Compile(req.Expr)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()

	var hits []queryHit
	truncated := false
	run := func(file string, f *yamlkit.File) error {
		rs, err := q.Run(ctx, f, queryLimit-len(hits))
		for _, x := range rs {
			hits = append(hits, queryHit{File: file, Result: x})
		}
		if errors.Is(err, query.ErrLimit) {
			truncated = true
		}
		return err
	}

	switch req.Scope {
	case "workspace":
		files, _, err := ws.Files(ctx)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		var mu sync.Mutex
		type parsed struct {
			path string
			f    *yamlkit.File
		}
		var docs []parsed
		var wg sync.WaitGroup
		sem := make(chan struct{}, 8)
		for _, f := range files {
			if f.Lang != "yaml" {
				continue
			}
			wg.Go(func() {
				sem <- struct{}{}
				defer func() { <-sem }()
				c, err := ws.Read(f.Path)
				if err != nil || c.YAML == nil {
					return
				}
				mu.Lock()
				docs = append(docs, parsed{f.Path, c.YAML})
				mu.Unlock()
			})
		}
		wg.Wait()
		sort.Slice(docs, func(i, j int) bool { return docs[i].path < docs[j].path })
		for _, d := range docs {
			// Errors in one file (e.g. iterating a missing list) don't
			// stop the search; the expression simply has no result there.
			if run(d.path, d.f); truncated || ctx.Err() != nil {
				break
			}
		}
	default:
		text, he := s.load(ctx, ws, req.side)
		if he != nil {
			writeError(w, he.status, he.msg)
			return
		}
		file := req.Path
		if req.Kind == "helm" {
			file = ""
		}
		if err := run(file, yamlkit.Parse([]byte(text.Text))); err != nil && !errors.Is(err, query.ErrLimit) && len(hits) == 0 {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
	}
	if hits == nil {
		hits = []queryHit{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": hits, "truncated": truncated, "timedOut": errors.Is(ctx.Err(), context.DeadlineExceeded)})
}

func orText(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
