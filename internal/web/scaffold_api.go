package web

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/mahasenabheetha/codec/v2/internal/config"
	"github.com/mahasenabheetha/codec/v2/internal/helm"
	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/scaffold"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Scaffolding endpoints. Everything they make is returned to the
// browser (text or a zip download); nothing is written anywhere.

// starters returns the built-in starters and the personal ones in the
// user's templates folder, read fresh each time so edits there show up
// without a restart.
func starters() (all []*scaffold.Starter, dir string, problems []string) {
	all = append(all, scaffold.Builtin()...)
	dir, err := config.TemplatesDir()
	if err != nil {
		return all, "", nil
	}
	if _, err := os.Stat(dir); err != nil {
		return all, dir, nil // not created yet: nothing personal
	}
	personal, errs := scaffold.Load(os.DirFS(dir), true)
	for _, e := range errs {
		problems = append(problems, e.Error())
	}
	return append(all, personal...), dir, problems
}

// GET /api/v2/scaffold/starters
func (s *Server) handleStarters(w http.ResponseWriter, r *http.Request) {
	all, dir, problems := starters()
	if problems == nil {
		problems = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"starters": all, "personalDir": dir, "problems": problems})
}

// checkedFile is a generated file with what codec finds in it.
type checkedFile struct {
	Path        string               `json:"path"`
	Content     string               `json:"content"`
	Type        string               `json:"type"` // provider id; "" = not YAML
	Diagnostics []yamlkit.Diagnostic `json:"diagnostics"`
}

// chartCheck is the render of a generated Helm chart.
type chartCheck struct {
	Chart       string            `json:"chart"` // folder of Chart.yaml
	Manifest    string            `json:"manifest"`
	Notes       string            `json:"notes,omitempty"`
	Diagnostics []helm.Diagnostic `json:"diagnostics"`
}

type checkResult struct {
	Files  []checkedFile `json:"files"`
	Charts []chartCheck  `json:"charts"`
}

// check lints each YAML file (as the editor would) and renders every
// chart among the files with helm template's defaults.
func (s *Server) checkFiles(ctx context.Context, files []scaffold.File) checkResult {
	res := checkResult{Files: []checkedFile{}, Charts: []chartCheck{}}
	for _, f := range files {
		cf := checkedFile{Path: f.Path, Content: f.Content, Diagnostics: []yamlkit.Diagnostic{}}
		if ext := path.Ext(f.Path); ext == ".yaml" || ext == ".yml" {
			pf := &provider.File{Path: f.Path, Content: []byte(f.Content), YAML: yamlkit.Parse([]byte(f.Content))}
			a := s.check.Analyze(ctx, pf, editorSchemaWait)
			cf.Type, cf.Diagnostics = a.Type, a.Diagnostics
		}
		res.Files = append(res.Files, cf)
	}
	for _, f := range files {
		if path.Base(f.Path) != "Chart.yaml" {
			continue
		}
		chart := path.Dir(f.Path)
		var in []helm.File
		for _, g := range files {
			if rel, ok := strings.CutPrefix(g.Path, chart+"/"); ok || chart == "." {
				if chart == "." {
					rel = g.Path
				}
				in = append(in, helm.File{Name: rel, Data: []byte(g.Content)})
			}
		}
		out := helm.Render(ctx, in, helm.Options{})
		s.lintRendered(ctx, out, chart, helmSchemaWait)
		for i, d := range out.Diagnostics {
			if d.File != "" && !strings.HasPrefix(d.File, chart+"/") && chart != "." {
				out.Diagnostics[i].File = path.Join(chart, d.File) // load errors name chart-relative files
			}
		}
		res.Charts = append(res.Charts, chartCheck{Chart: chart, Manifest: out.Manifest, Notes: out.Notes, Diagnostics: out.Diagnostics})
	}
	return res
}

// POST /api/v2/scaffold/render {id, values}: the starter filled in and
// checked, or the form's problems.
func (s *Server) handleScaffoldRender(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string            `json:"id"`
		Values map[string]string `json:"values"`
	}
	if !decode(w, r, &req) {
		return
	}
	all, _, _ := starters()
	st := scaffold.Find(all, req.ID)
	if st == nil {
		writeError(w, http.StatusNotFound, "no starter "+req.ID)
		return
	}
	files, problems, err := st.Render(req.Values)
	switch {
	case err != nil:
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	case len(problems) > 0:
		writeJSON(w, http.StatusOK, map[string]any{"fieldErrors": problems, "files": []checkedFile{}, "charts": []chartCheck{}})
		return
	}
	writeJSON(w, http.StatusOK, s.checkFiles(r.Context(), files))
}

// generated is a request carrying files the browser holds (a starter's
// output, perhaps edited).
type generated struct {
	Name  string          `json:"name"`
	Files []scaffold.File `json:"files"`
}

func (g *generated) valid() error {
	if len(g.Files) == 0 {
		return errors.New("no files")
	}
	for _, f := range g.Files {
		if !fs.ValidPath(f.Path) || f.Path == "." {
			return fmt.Errorf("%q is not a relative path", f.Path)
		}
	}
	return nil
}

// POST /api/v2/scaffold/check {files}: re-check files after what-if
// edits in the preview.
func (s *Server) handleScaffoldCheck(w http.ResponseWriter, r *http.Request) {
	var req generated
	if !decode(w, r, &req) {
		return
	}
	if err := req.valid(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.checkFiles(r.Context(), req.Files))
}

// POST /api/v2/scaffold/zip {name, files}: the files as a zip for the
// browser to save. Folders in paths become folders in the zip.
func (s *Server) handleScaffoldZip(w http.ResponseWriter, r *http.Request) {
	var req generated
	if !decode(w, r, &req) {
		return
	}
	if err := req.valid(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	now := time.Now()
	for _, f := range req.Files {
		// CreateHeader rather than Create, to stamp a real time: zip
		// tools show 1980 for a zero one.
		fw, err := zw.CreateHeader(&zip.FileHeader{Name: f.Path, Method: zip.Deflate, Modified: now})
		if err == nil {
			_, err = fw.Write([]byte(f.Content))
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	if err := zw.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	name := strings.Map(func(r rune) rune {
		if r == '"' || r == '/' || r == '\\' || r < ' ' {
			return '-'
		}
		return r
	}, req.Name)
	if name == "" {
		name = "codec-new"
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`.zip"`)
	w.Write(buf.Bytes())
}

// POST /api/v2/scaffold/clone {path, content?, doc, from, to, flip}:
// a renamed copy of a file or one of its documents, checked as the
// original's file type.
func (s *Server) handleScaffoldClone(w http.ResponseWriter, r *http.Request) {
	var req struct {
		yamlRequest
		Doc  int      `json:"doc"`
		From string   `json:"from"`
		To   string   `json:"to"`
		Flip []string `json:"flip"`
	}
	if !decode(w, r, &req) {
		return
	}
	f, _, err := s.yamlFile(&req.yamlRequest)
	if err != nil {
		he := &httpError{}
		if errors.As(err, &he) {
			writeError(w, he.status, he.msg)
		} else {
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	type docInfo struct {
		Index int    `json:"index"`
		Name  string `json:"name,omitempty"`
	}
	docs := []docInfo{}
	for _, d := range f.YAML.Docs {
		docs = append(docs, docInfo{d.Index, d.Name})
	}
	// The name that would be replaced, for the form: known before the
	// user has typed a new one.
	from := scaffold.OldName(f.Content, req.Doc)
	if strings.TrimSpace(req.To) == "" {
		writeJSON(w, http.StatusOK, map[string]any{"from": from, "docs": docs})
		return
	}
	res, err := scaffold.Clone(f.Content, scaffold.CloneOptions{Doc: req.Doc, From: req.From, To: req.To, Flip: req.Flip})
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"error": err.Error(), "from": from, "docs": docs})
		return
	}
	out := &provider.File{Path: f.Path, Content: []byte(res.Text), YAML: yamlkit.Parse([]byte(res.Text))}
	a := s.check.Analyze(r.Context(), out, editorSchemaWait)
	writeJSON(w, http.StatusOK, map[string]any{"result": res, "from": from, "docs": docs, "type": a.Type, "typeTitle": a.TypeTitle, "diagnostics": a.Diagnostics})
}
