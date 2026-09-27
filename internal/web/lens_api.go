package web

import (
	"context"
	"net/http"
	"slices"

	"github.com/mahasenabheetha/codec/v2/internal/ansible"
	"github.com/mahasenabheetha/codec/v2/internal/compose"
	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/workspace"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Lenses that read other files of the open folder (Compose layers and
// .env, Ansible roles and vars) get them through a loader over the
// workspace, with what-if buffers standing in for files.

// wsLoader reads workspace files, overrides first.
func wsLoader(ws *workspace.Workspace, overrides map[string]string) func(string) ([]byte, error) {
	return func(p string) ([]byte, error) {
		if text, ok := overrides[p]; ok {
			return []byte(text), nil
		}
		c, err := ws.Read(p)
		if err != nil {
			return nil, err
		}
		return []byte(c.Text), nil
	}
}

// wsFiles lists the workspace's files ([] when the scan fails, so the
// lenses still know a folder is open).
func wsFiles(ctx context.Context, ws *workspace.Workspace) []string {
	out := []string{}
	if files, _, err := ws.Files(ctx); err == nil {
		for _, f := range files {
			out = append(out, f.Path)
		}
	}
	return out
}

// --- editor hooks ---

// folderLens returns options for f's lens when it reads the folder:
// the file's buffer stands in for it on disk.
func (s *Server) folderLens(ctx context.Context, f *provider.File, tool string) (compose.Options, ansible.Options, bool) {
	ws := s.current()
	if ws == nil || (tool != compose.ID && tool != ansible.PlaybookID) {
		return compose.Options{}, ansible.Options{}, false
	}
	load := wsLoader(ws, map[string]string{f.Path: string(f.Content)})
	files := wsFiles(ctx, ws)
	if !slices.Contains(files, f.Path) {
		files = append(files, f.Path) // an unsaved or ignored file
	}
	return compose.Options{Load: load, Files: files}, ansible.Options{Load: load, Files: files, Root: repoRoot(ws, f.Path)}, true
}

// lensDiagnostics replaces the file-only findings of the Compose and
// Ansible lenses with the folder's.
func (s *Server) lensDiagnostics(ctx context.Context, f *provider.File, tool string, diags []yamlkit.Diagnostic) []yamlkit.Diagnostic {
	co, ao, ok := s.folderLens(ctx, f, tool)
	if !ok {
		return diags
	}
	var more []yamlkit.Diagnostic
	switch tool {
	case compose.ID:
		diags = slices.DeleteFunc(diags, func(d yamlkit.Diagnostic) bool { return d.Source == "compose" })
		more = compose.Diagnose(compose.AnalyzeFile(f, co), f.Path)
	case ansible.PlaybookID:
		diags = slices.DeleteFunc(diags, func(d yamlkit.Diagnostic) bool { return d.Source == "ansible" })
		more = ansible.Diagnose(ansible.Analyze(f.Path, f.Content, f.YAML, ao), f.Path)
	}
	return append(diags, more...)
}

// lensHover answers a hover with the folder, or nil.
func (s *Server) lensHover(ctx context.Context, f *provider.File, tool string, at yamlkit.Pos) *provider.Hover {
	co, ao, ok := s.folderLens(ctx, f, tool)
	switch {
	case !ok:
		return nil
	case tool == compose.ID:
		return compose.HoverIn(f, at, co)
	default:
		return ansible.HoverIn(f, at, ao)
	}
}

// lensDefinition answers go to definition with the folder, or nil.
func (s *Server) lensDefinition(ctx context.Context, f *provider.File, tool string, at yamlkit.Pos) []provider.Location {
	co, ao, ok := s.folderLens(ctx, f, tool)
	switch {
	case !ok:
		return nil
	case tool == compose.ID:
		return compose.DefinitionIn(f, at, co)
	default:
		return ansible.DefinitionIn(f, at, ao)
	}
}

// --- Compose ---

type composeRequest struct {
	Path      string            `json:"path"`
	Files     []string          `json:"files"`   // layers in merge order; empty = the default ones for path
	Env       map[string]string `json:"env"`     // what-if values
	EnvFile   string            `json:"envFile"` // "" = .env next to the first file, "none" = no env file
	Overrides map[string]string `json:"overrides"`
}

// POST /api/v2/compose/analyze {path, files, env, envFile, overrides}
func (s *Server) handleComposeAnalyze(w http.ResponseWriter, r *http.Request) {
	var req composeRequest
	if !decode(w, r, &req) {
		return
	}
	ws := s.current()
	if ws == nil {
		writeError(w, http.StatusConflict, "no folder is open")
		return
	}
	files := wsFiles(r.Context(), ws)
	layers := req.Files
	if len(layers) == 0 {
		layers = compose.DefaultFiles(req.Path, files)
	}
	opts := compose.Options{Load: wsLoader(ws, req.Overrides), Files: files, Env: req.Env, EnvFile: req.EnvFile}
	p, err := cancellable(r.Context(), func() (*compose.Project, error) { return compose.Analyze(layers, opts), nil })
	if err != nil {
		return // the client moved on
	}
	writeJSON(w, http.StatusOK, p)
}

// --- Ansible ---

type ansibleRequest struct {
	Path      string            `json:"path"`
	Overrides map[string]string `json:"overrides"`
}

// POST /api/v2/ansible/analyze {path, overrides}: a playbook or task
// file expanded, or an inventory's groups and hosts.
func (s *Server) handleAnsibleAnalyze(w http.ResponseWriter, r *http.Request) {
	var req ansibleRequest
	if !decode(w, r, &req) {
		return
	}
	ws := s.current()
	if ws == nil {
		writeError(w, http.StatusConflict, "no folder is open")
		return
	}
	load := wsLoader(ws, req.Overrides)
	content, err := load(req.Path)
	if err != nil {
		he := readError(err)
		writeError(w, he.status, he.msg)
		return
	}
	f := yamlkit.Parse(content)
	switch provider.Default.Best(&provider.File{Path: req.Path, YAML: f, Content: content}).ID() {
	case ansible.InventoryID:
		writeJSON(w, http.StatusOK, map[string]any{"kind": "inventory", "inventory": ansible.ReadInventory(req.Path, f)})
	case ansible.PlaybookID:
		opts := ansible.Options{Load: load, Files: wsFiles(r.Context(), ws), Root: repoRoot(ws, req.Path)}
		pb, err := cancellable(r.Context(), func() (*ansible.Playbook, error) { return ansible.Analyze(req.Path, content, f, opts), nil })
		if err != nil {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"kind": "playbook", "playbook": pb})
	default:
		writeError(w, http.StatusUnprocessableEntity, req.Path+" isn't an Ansible playbook, task file or YAML inventory")
	}
}

type findTaskRequest struct {
	Name string `json:"name"` // as the log shows it: "role : task name"
	Path string `json:"path"` // the log's "task path:", if any
}

// POST /api/v2/ansible/find-task {name, path}: where a task from a log
// is defined in the open folder.
func (s *Server) handleFindTask(w http.ResponseWriter, r *http.Request) {
	var req findTaskRequest
	if !decode(w, r, &req) {
		return
	}
	ws := s.current()
	if ws == nil {
		writeError(w, http.StatusConflict, "no folder is open")
		return
	}
	files := wsFiles(r.Context(), ws)
	ms, err := cancellable(r.Context(), func() ([]ansible.Match, error) {
		return ansible.FindTask(req.Name, req.Path, files, wsLoader(ws, nil)), nil
	})
	if err != nil {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"matches": ms})
}
