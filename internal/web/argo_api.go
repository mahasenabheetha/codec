package web

import (
	"bytes"
	"context"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/mahasenabheetha/codec/v2/internal/argo"
	"github.com/mahasenabheetha/codec/v2/internal/helm"
	"github.com/mahasenabheetha/codec/v2/internal/workspace"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// argoCache remembers which workspace files are Argo files (and their
// content), so templateRef lookups on hover don't re-read the folder.
type argoCache struct {
	mu    sync.Mutex
	root  string
	files map[string]argoEntry
}

type argoEntry struct {
	mod  time.Time
	size int64
	data []byte // nil: not an Argo file
	yaml *yamlkit.File
}

var argoMarker = []byte("argoproj.io/")

// argoIndex indexes the Argo files of the open folder, with what-if
// buffers standing in for files. Raw Helm templates are included:
// their names are usually literal.
func (s *Server) argoIndex(ctx context.Context, ws *workspace.Workspace, overrides map[string]string) (*argo.Index, error) {
	files, _, err := ws.Files(ctx)
	if err != nil {
		return nil, err
	}
	c := &s.argo
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.root != ws.Root() {
		c.root, c.files = ws.Root(), map[string]argoEntry{}
	}
	ix := argo.NewIndex()
	for _, f := range files {
		if f.Lang != "yaml" || (f.Type != "" && f.Type != "argo-workflows" && f.Type != "helm-template") {
			continue
		}
		if text, ok := overrides[f.Path]; ok {
			ix.Add(f.Path, []byte(text), yamlkit.Parse([]byte(text)))
			continue
		}
		e, ok := c.files[f.Path]
		if !ok || !e.mod.Equal(f.ModTime) || e.size != f.Size {
			e = argoEntry{mod: f.ModTime, size: f.Size}
			if content, err := ws.Read(f.Path); err == nil && content.YAML != nil && bytes.Contains([]byte(content.Text), argoMarker) {
				e.data, e.yaml = []byte(content.Text), content.YAML
			}
			c.files[f.Path] = e
		}
		if e.data != nil {
			ix.Add(f.Path, e.data, e.yaml)
		}
	}
	return ix, nil
}

// argoScope is what the Argo view looks at.
type argoScope struct {
	Kind      string            `json:"kind"` // file, text
	Path      string            `json:"path"` // file: the workflow's file
	Text      string            `json:"text"` // text: manifests (a Helm render)
	Workflow  string            `json:"workflow"`
	Params    map[string]string `json:"params"`
	ParamFile string            `json:"paramFile"` // workspace path of a parameter file
	Overrides map[string]string `json:"overrides"` // what-if buffers by path
}

// argoWorkflow is one entry of the view's workflow picker.
type argoWorkflow struct {
	Key      string        `json:"key"`
	Kind     string        `json:"kind"`
	Name     string        `json:"name"`
	Via      *argo.Trigger `json:"via,omitempty"`
	Schedule string        `json:"schedule,omitempty"`
	Source   argo.Source   `json:"source"`
}

type argoResponse struct {
	Workflows []argoWorkflow    `json:"workflows"`
	Apps      []argo.App        `json:"apps"`
	Result    *argo.Result      `json:"result"`
	Params    map[string]string `json:"params"` // the parameter file's values, merged with yours
	Error     string            `json:"error,omitempty"`
}

// POST /api/v2/argo/analyze {kind, path|text, workflow, params…}
func (s *Server) handleArgoAnalyze(w http.ResponseWriter, r *http.Request) {
	var req argoScope
	if !decode(w, r, &req) {
		return
	}
	ws := s.current()
	if ws == nil {
		writeError(w, http.StatusConflict, "no folder is open")
		return
	}
	resp := argoResponse{Workflows: []argoWorkflow{}, Apps: []argo.App{}, Params: map[string]string{}}
	var ix *argo.Index
	var mine []*argo.Spec
	var f *yamlkit.File
	switch req.Kind {
	case "file":
		var err error
		if ix, err = s.argoIndex(r.Context(), ws, req.Overrides); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
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
		f = yamlkit.Parse([]byte(text))
		for _, sp := range ix.Specs {
			if sp.Source.File == req.Path {
				mine = append(mine, sp)
			}
		}
		if len(mine) == 0 { // not indexed (e.g. no argoproj.io yet): read it alone
			mine = argo.Read(req.Path, []byte(text), f)
			ix.Specs = append(ix.Specs, mine...)
		}
		resp.Apps = argo.Apps(req.Path, f)
	case "text":
		ix = argo.NewIndex()
		f = yamlkit.Parse([]byte(req.Text))
		ix.Add("", []byte(req.Text), f)
		mine = ix.Specs
		resp.Apps = argo.Apps("", f)
	default:
		writeError(w, http.StatusBadRequest, "unknown scope kind "+req.Kind)
		return
	}
	if resp.Apps == nil {
		resp.Apps = []argo.App{} // JSON clients get [], not null
	}
	if err := s.linkApps(r.Context(), ws, resp.Apps); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, sp := range mine {
		resp.Workflows = append(resp.Workflows, argoWorkflow{Key: sp.Key(), Kind: sp.Kind, Name: sp.Name, Via: sp.Via, Schedule: sp.Schedule, Source: sp.Source})
	}

	if req.ParamFile != "" {
		c, err := ws.Read(req.ParamFile)
		if err != nil {
			he := readError(err)
			writeError(w, he.status, he.msg)
			return
		}
		pf := c.YAML
		if pf == nil || len(pf.Docs) == 0 || pf.Docs[0].Root == nil || pf.Docs[0].Root.Kind != yamlkit.KindMap {
			resp.Error = req.ParamFile + ": expected a map of name: value"
		} else {
			for _, p := range pf.Docs[0].Root.Pairs {
				resp.Params[p.Key.Value] = yamlkit.ValueText(p.Value)
			}
		}
	}
	for k, v := range req.Params {
		resp.Params[k] = v
	}

	var wf *argo.Spec
	for _, sp := range mine {
		if sp.Key() == req.Workflow {
			wf = sp
		}
	}
	if wf == nil && len(mine) > 0 {
		wf = mine[0]
	}
	if wf != nil {
		res, _ := cancellable(r.Context(), func() (*argo.Result, error) {
			return argo.Resolve(ix, wf, argo.Options{Params: resp.Params, Bodies: true}), nil
		})
		if res == nil {
			return // the client moved on
		}
		resp.Result = res
	}
	writeJSON(w, http.StatusOK, resp)
}

// linkApps finds, for each Helm source, the chart in the open folder it
// deploys: by path (the chart is in this repository) or by chart name
// (a chart published to a registry, with a local copy here).
func (s *Server) linkApps(ctx context.Context, ws *workspace.Workspace, apps []argo.App) error {
	if len(apps) == 0 {
		return nil
	}
	files, _, err := ws.Files(ctx)
	if err != nil {
		return err
	}
	have := map[string]bool{}
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i], have[f.Path] = f.Path, true
	}
	charts := helm.FindCharts(paths)
	names := map[string]string{} // Chart.yaml name -> chart path
	for _, c := range charts {
		if content, err := ws.Read(path.Join(c, "Chart.yaml")); err == nil && content.YAML != nil && len(content.YAML.Docs) > 0 {
			if n := content.YAML.Docs[0].Root.Get("name").Str(); n != "" {
				names[n] = c
			}
		}
	}
	for i := range apps {
		for j := range apps[i].Sources {
			src := &apps[i].Sources[j]
			local := &argo.LocalChart{}
			if p := strings.Trim(path.Clean("/"+src.Path), "/"); src.Path != "" && src.Chart == "" {
				for _, c := range charts {
					if c == p || strings.HasSuffix(c, "/"+p) || p == "" && c == "." {
						local.Chart, local.Match = c, "path"
						break
					}
				}
			} else if src.Chart != "" {
				name := src.Chart[strings.LastIndex(src.Chart, "/")+1:]
				if c, ok := names[name]; ok {
					local.Chart, local.Match = c, "name"
				}
			}
			if local.Chart == "" {
				continue
			}
			if src.Helm != nil {
				for _, vf := range src.Helm.ValueFiles {
					switch p := path.Join(local.Chart, vf); {
					case p == path.Join(local.Chart, "values.yaml"):
						// the chart's defaults apply anyway
					case have[p]:
						local.Values = append(local.Values, p)
					default:
						local.Missing = append(local.Missing, vf)
					}
				}
			}
			src.Local = local
		}
	}
	return nil
}
