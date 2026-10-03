package web

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/mahasenabheetha/codec/v2/internal/ansible"
	"github.com/mahasenabheetha/codec/v2/internal/argo"
	"github.com/mahasenabheetha/codec/v2/internal/ci"
	"github.com/mahasenabheetha/codec/v2/internal/compose"
	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/scaffold"
	"github.com/mahasenabheetha/codec/v2/internal/textdiff"
	"github.com/mahasenabheetha/codec/v2/internal/workspace"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Editor endpoints. Each takes the file's workspace path and, for an
// edited (what-if) buffer, its content; without content the file is
// read from disk. Positions are 1-based line and rune column.

// maxBody bounds request bodies: the largest file is 8 MB, which JSON
// escaping can grow, plus what-if copies of other files.
const maxBody = 24 << 20

type yamlRequest struct {
	Path    string  `json:"path"`
	Content *string `json:"content"` // nil = the file on disk
	Line    int     `json:"line"`
	Col     int     `json:"col"`
	// Type is the file type from the last analysis, so schema help
	// keeps working while the buffer doesn't parse mid-edit.
	Type string `json:"type,omitempty"`
}

// httpError carries a status with a message, so helpers can return one
// error and the handler maps it once.
type httpError struct {
	status int
	msg    string
}

func (e *httpError) Error() string { return e.msg }

// decode reads a JSON body into v.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return false
	}
	return true
}

// yamlFile parses the request's buffer (or the file on disk) and finds
// the provider for it.
func (s *Server) yamlFile(req *yamlRequest) (*provider.File, provider.Provider, error) {
	f := &provider.File{Path: req.Path}
	if req.Content != nil {
		f.Content = []byte(*req.Content)
		f.YAML = yamlkit.Parse(f.Content)
	} else {
		ws := s.current()
		if ws == nil {
			return nil, nil, &httpError{http.StatusConflict, "no folder is open"}
		}
		c, err := ws.Read(req.Path)
		if err != nil {
			return nil, nil, readError(err)
		}
		f.Content, f.YAML = []byte(c.Text), c.YAML
		if f.YAML == nil { // not a .yaml name, or the cached parse failed
			f.YAML = yamlkit.Parse(f.Content)
		}
	}
	// A Kustomize patch is a partial object; the editor must not demand
	// the fields the complete one has.
	if ws := s.current(); ws != nil && req.Path != "" {
		f.Patch = patchesOf(ws).Is(req.Path)
	}
	return f, provider.Default.Best(f), nil
}

// readError maps workspace read errors to HTTP statuses.
func readError(err error) *httpError {
	switch {
	case errors.Is(err, workspace.ErrOutside):
		return &httpError{http.StatusForbidden, err.Error()}
	case errors.Is(err, fs.ErrNotExist):
		// The OS wording ("openat x: The system cannot find…") differs
		// per platform and says too much; name the path only.
		msg := "file not found"
		if pe, ok := errors.AsType[*fs.PathError](err); ok {
			msg = pe.Path + ": not found"
		}
		return &httpError{http.StatusNotFound, msg}
	case errors.Is(err, workspace.ErrTooLarge):
		return &httpError{http.StatusRequestEntityTooLarge, err.Error()}
	case errors.Is(err, workspace.ErrBinary):
		return &httpError{http.StatusUnsupportedMediaType, err.Error()}
	case errors.Is(err, workspace.ErrNotFile):
		return &httpError{http.StatusUnprocessableEntity, err.Error()}
	}
	return &httpError{http.StatusInternalServerError, err.Error()}
}

// cancellable runs fn on its own goroutine and stops waiting when ctx
// ends: the editor cancels a request as soon as the user types again,
// and there is no point sending a result nobody reads. (fn runs to
// completion regardless; parsing can't be interrupted midway.)
func cancellable[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	type result struct {
		v   T
		err error
	}
	done := make(chan result, 1) // buffered: fn's goroutine never blocks
	go func() {
		v, err := fn()
		done <- result{v, err}
	}()
	select {
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	case r := <-done:
		return r.v, r.err
	}
}

// serveYAML is the shared shape of the editor endpoints: decode,
// compute (cancellably), respond.
func (s *Server) serveYAML(w http.ResponseWriter, r *http.Request, fn func(req *yamlRequest, f *provider.File, p provider.Provider) any) {
	var req yamlRequest
	if !decode(w, r, &req) {
		return
	}
	out, err := cancellable(r.Context(), func() (any, error) {
		f, p, err := s.yamlFile(&req)
		if err != nil {
			return nil, err
		}
		return fn(&req, f, p), nil
	})
	var he *httpError
	switch {
	case errors.Is(err, context.Canceled):
		return // the client went away
	case errors.As(err, &he):
		writeError(w, he.status, he.msg)
	case err != nil:
		writeError(w, http.StatusInternalServerError, err.Error())
	default:
		writeJSON(w, http.StatusOK, out)
	}
}

func pos(req *yamlRequest, f *provider.File) yamlkit.Pos {
	return yamlkit.PosAt(f.Content, max(1, req.Line), max(1, req.Col))
}

// POST /api/v2/yaml/analyze
func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	s.serveYAML(w, r, func(_ *yamlRequest, f *provider.File, _ provider.Provider) any {
		a := s.check.Analyze(r.Context(), f, editorSchemaWait)
		if a.Type == "argo-workflows" {
			// templateRef targets live in other files of the folder.
			if ix := s.argoIndexFor(r.Context(), f); ix != nil {
				a.Diagnostics = append(a.Diagnostics, argo.CheckRefs(argo.Read(f.Path, f.Content, f.YAML), ix)...)
				slices.SortStableFunc(a.Diagnostics, func(x, y yamlkit.Diagnostic) int { return cmp.Compare(x.Range.Start.Offset, y.Range.Start.Offset) })
			}
		}
		if opts, ok := s.ciFor(r.Context(), f, a.Type); ok {
			// With the repository, includes and templates are followed:
			// replace the file-only findings.
			a.Diagnostics = slices.DeleteFunc(a.Diagnostics, func(d yamlkit.Diagnostic) bool { return d.Source == "ci" })
			a.Diagnostics = append(a.Diagnostics, ci.Diagnose(ci.Analyze(a.Type, f.Path, f.Content, f.YAML, opts), f.Path)...)
			slices.SortStableFunc(a.Diagnostics, func(x, y yamlkit.Diagnostic) int { return cmp.Compare(x.Range.Start.Offset, y.Range.Start.Offset) })
		}
		if a.Type == compose.ID || a.Type == ansible.PlaybookID {
			// Compose layers and .env, Ansible roles and vars: other files.
			a.Diagnostics = s.lensDiagnostics(r.Context(), f, a.Type, a.Diagnostics)
			slices.SortStableFunc(a.Diagnostics, func(x, y yamlkit.Diagnostic) int { return cmp.Compare(x.Range.Start.Offset, y.Range.Start.Offset) })
		}
		return a
	})
}

// argoIndexFor indexes the folder's Argo files for editor requests on
// f, with f's buffer standing in for the file; nil when f isn't an
// Argo file or no folder is open.
func (s *Server) argoIndexFor(ctx context.Context, f *provider.File) *argo.Index {
	ws := s.current()
	if ws == nil || !bytes.Contains(f.Content, argoMarker) {
		return nil
	}
	ix, err := s.argoIndex(ctx, ws, map[string]string{f.Path: string(f.Content)})
	if err != nil {
		return nil
	}
	return ix
}

// POST /api/v2/yaml/hover
func (s *Server) handleHover(w http.ResponseWriter, r *http.Request) {
	s.serveYAML(w, r, func(req *yamlRequest, f *provider.File, p provider.Provider) any {
		at := pos(req, f)
		h := cronHover(f, p.ID(), at, time.Now()) // schedules in plain words
		if h == nil {
			if ix := s.argoIndexFor(r.Context(), f); ix != nil {
				h = argo.HoverIn(f, at, ix) // also in raw Helm templates of Argo files
			}
		}
		if opts, ok := s.ciFor(r.Context(), f, p.ID()); ok && h == nil {
			h = ci.HoverIn(p.ID(), f, at, opts) // includes and templates too
		}
		if h == nil {
			h = s.lensHover(r.Context(), f, p.ID(), at) // Compose and Ansible with the folder
		}
		if h == nil {
			h = provider.HoverAt(p, f, at)
		}
		if h != nil && strings.HasSuffix(h.Title, "template expression") {
			if ws := s.current(); ws != nil {
				h.Rows = append(h.Rows, s.helmHoverRows(ws, req.Path, h.Code)...)
			}
		} else if h != nil {
			ctx, cancel := context.WithTimeout(r.Context(), time.Second)
			defer cancel()
			h.Rows = append(h.Rows, s.check.HoverRows(ctx, f, req.Type, at)...)
		}
		return map[string]any{"hover": h}
	})
}

// POST /api/v2/yaml/definition
func (s *Server) handleDefinition(w http.ResponseWriter, r *http.Request) {
	s.serveYAML(w, r, func(req *yamlRequest, f *provider.File, p provider.Provider) any {
		var locs []provider.Location
		if ix := s.argoIndexFor(r.Context(), f); ix != nil {
			locs = argo.DefinitionIn(f, pos(req, f), ix) // templateRef across files
		}
		if opts, ok := s.ciFor(r.Context(), f, p.ID()); ok && len(locs) == 0 {
			locs = ci.DefinitionIn(p.ID(), f, pos(req, f), opts) // extends, templates, actions in other files
		}
		if len(locs) == 0 {
			locs = s.lensDefinition(r.Context(), f, p.ID(), pos(req, f)) // roles, handlers, layers, .env
		}
		if len(locs) == 0 {
			locs = provider.DefinitionAt(p, f, pos(req, f))
		}
		if locs == nil {
			locs = []provider.Location{}
		}
		return map[string]any{"locations": locs}
	})
}

// POST /api/v2/yaml/complete
func (s *Server) handleComplete(w http.ResponseWriter, r *http.Request) {
	s.serveYAML(w, r, func(req *yamlRequest, f *provider.File, p provider.Provider) any {
		at := pos(req, f)
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		items := append(s.check.Complete(ctx, f, req.Type, at), provider.CompleteAt(p, f, at)...)
		// The buffer may not parse mid-edit: the last known type wins.
		items = append(items, scaffold.Snippets(f, cmp.Or(req.Type, p.ID()), at)...)
		if items == nil {
			items = []provider.Completion{}
		}
		return map[string]any{"items": items}
	})
}

// POST /api/v2/yaml/path: the path at a position in every notation.
func (s *Server) handlePath(w http.ResponseWriter, r *http.Request) {
	s.serveYAML(w, r, func(req *yamlRequest, f *provider.File, _ provider.Provider) any {
		at := pos(req, f)
		var path yamlkit.Path
		if doc := f.YAML.DocAt(at); doc != nil {
			path, _ = doc.PathAt(at)
		}
		formats := map[yamlkit.PathStyle]string{}
		if len(path) > 0 {
			for _, st := range yamlkit.PathStyles {
				formats[st] = path.Format(st)
			}
		}
		if path == nil {
			path = yamlkit.Path{}
		}
		return map[string]any{"segments": path, "formats": formats}
	})
}

// POST /api/v2/files/diff {path, content}: a unified diff from the file
// on disk to the what-if buffer, ready for `git apply`.
func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	var req yamlRequest
	if !decode(w, r, &req) {
		return
	}
	ws := s.current()
	switch {
	case ws == nil:
		writeError(w, http.StatusConflict, "no folder is open")
		return
	case req.Content == nil:
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}
	c, err := ws.Read(req.Path)
	if err != nil {
		he := readError(err)
		writeError(w, he.status, he.msg)
		return
	}
	disk := c.Text
	if c.BOM {
		disk = "\xEF\xBB\xBF" + disk
	}
	diff := textdiff.Patch(req.Path, []byte(disk), *req.Content)
	writeJSON(w, http.StatusOK, map[string]any{"diff": diff, "changed": strings.TrimSpace(diff) != ""})
}
