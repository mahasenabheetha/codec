package web

import (
	"context"
	"net/http"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/mahasenabheetha/codec/v2/internal/config"
	"github.com/mahasenabheetha/codec/v2/internal/helm"
	"github.com/mahasenabheetha/codec/v2/internal/lint"
	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// How long a request waits for a schema that is still downloading.
// The editor asks again when a document comes back "pending".
const (
	editorSchemaWait    = 3 * time.Second
	workspaceSchemaWait = 60 * time.Second
	helmSchemaWait      = 5 * time.Second
)

type lintSettingsResponse struct {
	Settings          config.Lint `json:"settings"`
	Rules             []lint.Rule `json:"rules"`
	K8sVersions       []string    `json:"k8sVersions"`
	DefaultK8sVersion string      `json:"defaultK8sVersion"`
}

// GET /api/v2/lint/settings: the user's choices plus the rule catalogue.
func (s *Server) handleLintSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, lintSettingsResponse{
		Settings:          s.opts.Config.Get().Lint,
		Rules:             lint.Rules(),
		K8sVersions:       lint.K8sVersions,
		DefaultK8sVersion: lint.DefaultK8sVersion,
	})
}

// POST /api/v2/lint/settings {settings}: save and apply at once.
func (s *Server) handleSaveLintSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Settings config.Lint `json:"settings"`
	}
	if !decode(w, r, &req) {
		return
	}
	st := req.Settings
	if st.K8sVersion != "" && !slices.Contains(lint.K8sVersions, st.K8sVersion) {
		writeError(w, http.StatusBadRequest, "unknown Kubernetes version "+st.K8sVersion)
		return
	}
	for id, level := range st.Rules {
		if (lint.Config{}).Level(id) == "" {
			delete(st.Rules, id) // a rule this version doesn't have
			continue
		}
		switch level {
		case "error", "warning", "info", lint.Off:
		default:
			writeError(w, http.StatusBadRequest, "invalid level "+level+" for "+id)
			return
		}
	}
	if err := s.opts.Config.Update(func(all *config.Settings) { all.Lint = st }); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.check.Apply(st)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type lintFile struct {
	Path        string               `json:"path"`
	Type        string               `json:"type"`
	Diagnostics []yamlkit.Diagnostic `json:"diagnostics"`
}

// POST /api/v2/lint/workspace: every YAML file of the open folder.
// Files are read from disk; open tabs' what-if edits aren't included
// (they are checked live in the editor).
func (s *Server) handleLintWorkspace(w http.ResponseWriter, r *http.Request) {
	ws := s.current()
	if ws == nil {
		writeError(w, http.StatusConflict, "no folder is open")
		return
	}
	start := time.Now()
	files, _, err := ws.Files(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var paths []string
	for _, f := range files {
		if f.Lang == "yaml" {
			paths = append(paths, f.Path)
		}
	}

	var (
		mu  sync.Mutex
		out []lintFile
		wg  sync.WaitGroup
		sem = make(chan struct{}, 8)
	)
	for _, p := range paths {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			if r.Context().Err() != nil {
				return
			}
			c, err := ws.Read(p)
			if err != nil {
				return // deleted meanwhile, or too large: not lint's business
			}
			f := &provider.File{Path: p, Content: []byte(c.Text), YAML: c.YAML}
			if f.YAML == nil {
				f.YAML = yamlkit.Parse(f.Content)
			}
			a := s.check.Analyze(r.Context(), f, workspaceSchemaWait)
			if len(a.Diagnostics) == 0 {
				return
			}
			mu.Lock()
			out = append(out, lintFile{Path: p, Type: a.Type, Diagnostics: a.Diagnostics})
			mu.Unlock()
		})
	}
	wg.Wait()
	if r.Context().Err() != nil {
		return
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	if out == nil {
		out = []lintFile{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"files":      out,
		"checked":    len(paths),
		"durationMs": time.Since(start).Milliseconds(),
	})
}

// lintRendered adds lint and schema findings on the rendered manifest
// to a Helm result, each pointing at its manifest line and the
// template that produced it.
func (s *Server) lintRendered(ctx context.Context, res *helm.Result, chart string) {
	if res.Manifest == "" {
		return
	}
	for _, d := range s.check.Manifest(ctx, res.Manifest, helmSchemaWait) {
		line := d.Range.Start.Line
		src, obj := "", ""
		for _, doc := range res.Docs { // sorted by line
			if doc.Line <= line {
				src, obj = doc.Source, doc.Kind+"/"+doc.Name
			}
		}
		msg := d.Message
		if obj != "/" && obj != "" {
			msg = obj + ": " + msg
		}
		file := ""
		if src != "" {
			file = helmSourcePath(chart, src)
		}
		res.Diagnostics = append(res.Diagnostics, helm.Diagnostic{
			Severity: string(d.Severity), Code: d.Code, Message: msg, Hint: d.Hint, Why: d.Why, Source: d.Source,
			File: file, Manifest: line, Col: d.Range.Start.Col,
		})
	}
}
