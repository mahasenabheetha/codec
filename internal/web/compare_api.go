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
// buffer when content is set), or a chart rendered with a profile.
type side struct {
	Kind    string  `json:"kind"` // "file" (default) or "helm"
	Path    string  `json:"path"`
	Content *string `json:"content"` // file: what-if buffer; nil = disk
	Doc     *int    `json:"doc"`     // file: only this document (0-based)
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
	case "", "file":
		text := ""
		title := sd.Path
		if sd.Content != nil {
			text, title = *sd.Content, sd.Path+" (what-if)"
		} else {
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

// POST /api/v2/compare {left, right, ignore[]}: a semantic diff, or a
// text diff when either side doesn't parse.
func (s *Server) handleCompare(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Left, Right side
		Ignore      []string `json:"ignore"`
		Text        bool     `json:"text"` // force a text diff
	}
	if !decode(w, r, &req) {
		return
	}
	ws := s.current()
	if ws == nil {
		writeError(w, http.StatusConflict, "no folder is open")
		return
	}
	left, he := s.load(r.Context(), ws, req.Left)
	if he == nil {
		var right sideText
		if right, he = s.load(r.Context(), ws, req.Right); he == nil {
			s.writeCompare(w, left, right, req.Ignore, req.Text)
			return
		}
	}
	writeError(w, he.status, he.msg)
}

func (s *Server) writeCompare(w http.ResponseWriter, left, right sideText, ignore []string, text bool) {
	out := map[string]any{"left": left, "right": right}
	if !text {
		changes, err := yamlkit.Diff(yamlkit.Parse([]byte(left.Text)), yamlkit.Parse([]byte(right.Text)), yamlkit.DiffOptions{Ignore: ignore})
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
	out["diff"] = textdiff.Unified("file", left.Text, right.Text, 3)
	writeJSON(w, http.StatusOK, out)
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
