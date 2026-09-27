package web

import (
	"context"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/ci"
	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/workspace"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// isCI reports whether a provider id is one of the CI lenses.
func isCI(tool string) bool { return tool == ci.GitHub || tool == ci.GitLab || tool == ci.Azure }

// ciOptions backs the CI lens with the open folder: includes,
// templates and actions are read from the workspace, what-if buffers
// standing in for files.
func (s *Server) ciOptions(ctx context.Context, ws *workspace.Workspace, file string, overrides map[string]string, params map[string]string) ci.Options {
	opts := ci.Options{
		Root:   repoRoot(ws, file),
		Params: params,
		Load: func(p string) ([]byte, error) {
			if text, ok := overrides[p]; ok {
				return []byte(text), nil
			}
			c, err := ws.Read(p)
			if err != nil {
				return nil, err
			}
			return []byte(c.Text), nil
		},
	}
	if files, _, err := ws.Files(ctx); err == nil {
		for _, f := range files {
			opts.Files = append(opts.Files, f.Path)
		}
	}
	return opts
}

// repoRoot is the repository a pipeline file belongs to, as a
// workspace path: the folder above .github/, or the nearest folder
// holding .git (an opened folder may hold several repositories).
func repoRoot(ws *workspace.Workspace, file string) string {
	if i := strings.Index("/"+file, "/.github/"); i >= 0 {
		return strings.Trim(("/" + file)[:i], "/")
	}
	for dir := path.Dir(file); ; dir = path.Dir(dir) {
		rel := dir
		if dir == "." {
			rel = ""
		}
		if _, err := os.Stat(filepath.Join(ws.Root(), filepath.FromSlash(rel), ".git")); err == nil {
			return rel
		}
		if dir == "." || dir == "/" {
			return ""
		}
	}
}

// ciFor analyzes an editor file with the whole repository, or returns
// nil when it isn't a pipeline or no folder is open.
func (s *Server) ciFor(ctx context.Context, f *provider.File, tool string) (ci.Options, bool) {
	ws := s.current()
	if ws == nil || !isCI(tool) {
		return ci.Options{}, false
	}
	return s.ciOptions(ctx, ws, f.Path, map[string]string{f.Path: string(f.Content)}, nil), true
}

type ciRequest struct {
	Path      string            `json:"path"`
	Params    map[string]string `json:"params"`
	Overrides map[string]string `json:"overrides"` // what-if buffers by path
}

// POST /api/v2/ci/analyze {path, params, overrides}
func (s *Server) handleCIAnalyze(w http.ResponseWriter, r *http.Request) {
	var req ciRequest
	if !decode(w, r, &req) {
		return
	}
	ws := s.current()
	if ws == nil {
		writeError(w, http.StatusConflict, "no folder is open")
		return
	}
	text, ok := req.Overrides[req.Path]
	if !ok {
		c, err := ws.Read(req.Path)
		if err != nil {
			he := readError(err)
			writeError(w, he.status, he.msg)
			return
		}
		text = c.Text
	}
	content := []byte(text)
	f := yamlkit.Parse(content)
	tool := provider.Default.Best(&provider.File{Path: req.Path, YAML: f, Content: content}).ID()
	if !isCI(tool) {
		writeError(w, http.StatusUnprocessableEntity, req.Path+" isn't a GitHub Actions, GitLab CI or Azure Pipelines file")
		return
	}
	opts := s.ciOptions(r.Context(), ws, req.Path, req.Overrides, req.Params)
	p, err := cancellable(r.Context(), func() (*ci.Pipeline, error) {
		return ci.Analyze(tool, req.Path, content, f, opts), nil
	})
	if err != nil {
		return // the client moved on
	}
	writeJSON(w, http.StatusOK, p)
}
