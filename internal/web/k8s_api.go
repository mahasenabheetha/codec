package web

import (
	"context"
	"errors"
	"net/http"
	"path"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/check"
	"github.com/mahasenabheetha/codec/v2/internal/kube"
	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/workspace"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// k8sScope is what the resources view looks at.
type k8sScope struct {
	Kind      string            `json:"kind"`      // folder, text, kustomize, helm
	Path      string            `json:"path"`      // folder or kustomization dir ("." = root)
	Text      string            `json:"text"`      // text: manifests (e.g. a Helm render)
	Chart     string            `json:"chart"`     // helm
	Profile   string            `json:"profile"`   // helm
	Overrides map[string]string `json:"overrides"` // kustomize: what-if buffers by path
}

type k8sResponse struct {
	Manifest  string               `json:"manifest,omitempty"` // kustomize and helm builds
	Cards     []kube.Card          `json:"cards"`
	Inventory kube.Inventory       `json:"inventory"`
	Graph     kube.Graph           `json:"graph"`
	Problems  []yamlkit.Diagnostic `json:"problems"` // lint + schema on built output
	Error     string               `json:"error,omitempty"`
}

// POST /api/v2/k8s/analyze {kind, path|text|chart…}
func (s *Server) handleK8sAnalyze(w http.ResponseWriter, r *http.Request) {
	var req k8sScope
	if !decode(w, r, &req) {
		return
	}
	ws := s.current()
	if ws == nil {
		writeError(w, http.StatusConflict, "no folder is open")
		return
	}
	resp := k8sResponse{Problems: []yamlkit.Diagnostic{}}
	var objs []kube.Object
	switch req.Kind {
	case "folder":
		var err error
		if objs, err = folderObjects(r.Context(), ws, req.Path); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	case "text":
		objs = kube.Collect("", yamlkit.Parse([]byte(req.Text)))
	case "kustomize", "helm":
		manifest, err := s.buildManifest(r.Context(), ws, req)
		if err != nil {
			resp.Error = err.Error() // a build error is a result to show, not a failed request
			resp.Cards, resp.Graph = []kube.Card{}, kube.Graph{Nodes: []kube.Node{}, Edges: []kube.Edge{}, Findings: []kube.Finding{}}
			resp.Inventory = kube.Summarize(nil, resp.Graph)
			writeJSON(w, http.StatusOK, resp)
			return
		}
		resp.Manifest = manifest
		objs = kube.Collect("", yamlkit.Parse([]byte(manifest)))
		if req.Kind == "kustomize" {
			resp.Problems = s.check.Manifest(r.Context(), manifest, helmSchemaWait)
		}
	default:
		writeError(w, http.StatusBadRequest, "unknown scope kind "+req.Kind)
		return
	}
	resp.Graph = kube.Relate(objs)
	resp.Cards = kube.Cards(objs, resp.Graph)
	resp.Inventory = kube.Summarize(objs, resp.Graph)
	writeJSON(w, http.StatusOK, resp)
}

// folderObjects collects the objects of every YAML file under dir,
// skipping raw Helm templates (their render is its own scope).
func folderObjects(ctx context.Context, ws *workspace.Workspace, dir string) ([]kube.Object, error) {
	files, _, err := ws.Files(ctx)
	if err != nil {
		return nil, err
	}
	dir = strings.Trim(path.Clean("/"+dir), "/")
	var objs []kube.Object
	for _, f := range files {
		if f.Lang != "yaml" || (dir != "" && !strings.HasPrefix(f.Path, dir+"/")) {
			continue
		}
		c, err := ws.Read(f.Path)
		if err != nil || c.YAML == nil {
			continue
		}
		switch provider.Default.Best(&provider.File{Path: f.Path, YAML: c.YAML}).ID() {
		case "helm-template", "helm-values", "helm-chart", "kustomize":
			continue
		}
		objs = append(objs, kube.Collect(f.Path, c.YAML)...)
	}
	return objs, nil
}

// buildManifest renders a kustomization or a Helm profile.
func (s *Server) buildManifest(ctx context.Context, ws *workspace.Workspace, req k8sScope) (string, error) {
	if req.Kind == "helm" {
		res, he := s.renderProfile(ctx, ws, req.Chart, req.Profile)
		if he != nil {
			return "", errors.New(he.msg)
		}
		return res.Manifest, nil
	}
	dir := strings.Trim(path.Clean("/"+req.Path), "/")
	rd, err := newWSReader(ctx, ws, req.Overrides)
	if err != nil {
		return "", err
	}
	files, err := kube.KustomizeFiles(rd, dir)
	if err != nil {
		return "", err
	}
	return kube.Kustomize(files, dir)
}

// wsReader gives kustomize read-only access to the workspace, with
// what-if buffers standing in for files.
type wsReader struct {
	ws        *workspace.Workspace
	overrides map[string]string
	dirs      map[string]bool
}

func newWSReader(ctx context.Context, ws *workspace.Workspace, overrides map[string]string) (*wsReader, error) {
	files, _, err := ws.Files(ctx)
	if err != nil {
		return nil, err
	}
	dirs := map[string]bool{"": true, ".": true}
	for _, f := range files {
		for d := path.Dir(f.Path); d != "." && !dirs[d]; d = path.Dir(d) {
			dirs[d] = true
		}
	}
	return &wsReader{ws: ws, overrides: overrides, dirs: dirs}, nil
}

func (r *wsReader) ReadFile(p string) ([]byte, error) {
	if text, ok := r.overrides[p]; ok {
		return []byte(text), nil
	}
	c, err := r.ws.Read(p)
	if err != nil {
		return nil, err
	}
	return []byte(c.Text), nil
}

func (r *wsReader) IsDir(p string) bool { return r.dirs[p] }

// POST /api/v2/k8s/neat {path, content?}: the file without cluster noise.
func (s *Server) handleK8sNeat(w http.ResponseWriter, r *http.Request) {
	s.serveYAML(w, r, func(_ *yamlRequest, f *provider.File, _ provider.Provider) any {
		out, err := kube.Neat(f.Content)
		if err != nil {
			return map[string]string{"error": err.Error()}
		}
		return map[string]string{"text": string(out)}
	})
}

// patchesOf finds the Kustomize patches among the workspace's files.
func patchesOf(ws *workspace.Workspace) *check.Patches {
	return check.NewPatches(func(p string) ([]byte, error) {
		c, err := ws.Read(p)
		if err != nil {
			return nil, err
		}
		return []byte(c.Text), nil
	})
}
