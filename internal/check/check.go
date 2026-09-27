// Package check runs everything codec knows how to check on a YAML
// file: syntax (yamlkit), the file type's own diagnostics (provider),
// lint rules (lint) and schema validation (schema, with schemas from
// schemacache). The web UI, `codec yaml lint` and the Helm view all
// go through it, so they report the same findings.
package check

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mahasenabheetha/codec/v2/internal/config"
	"github.com/mahasenabheetha/codec/v2/internal/lint"
	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/schema"
	"github.com/mahasenabheetha/codec/v2/internal/schemacache"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Checker holds the current settings. It is safe for concurrent use.
type Checker struct {
	reg     *provider.Registry
	schemas *schemacache.Store

	mu      sync.RWMutex
	cfg     lint.Config
	enabled bool // schema validation on
}

// New returns a checker configured from settings.
func New(reg *provider.Registry, st config.Lint, version string) *Checker {
	c := &Checker{reg: reg, schemas: schemacache.New(schemacache.Options{UserAgent: "codec/" + version})}
	c.Apply(st)
	return c
}

// Apply switches to new settings.
func (c *Checker) Apply(st config.Lint) {
	c.schemas.SetOptions(schemacache.Options{CustomDir: st.SchemaDir, Offline: st.Offline})
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cfg = lint.Config{Levels: st.Rules, LineLength: st.LineLength, K8sVersion: st.K8sVersion}
	c.enabled = !st.NoSchemas
}

func (c *Checker) settings() (lint.Config, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cfg, c.enabled && c.cfg.Level("schema") != lint.Off
}

// Analyze is provider.Analyze plus lint and schema findings. Schemas
// not yet downloaded are waited for up to wait; documents still
// waiting after that are marked "pending", so the caller can ask again.
func (c *Checker) Analyze(ctx context.Context, f *provider.File, wait time.Duration) *provider.Analysis {
	cfg, schemasOn := c.settings()
	a := c.reg.Analyze(f)
	a.Diagnostics = append(a.Diagnostics, lint.Run(lint.Input{YAML: f.YAML, Content: f.Content, Type: a.Type}, cfg)...)
	if schemasOn {
		ctx, cancel := context.WithTimeout(ctx, wait)
		defer cancel()
		for i, d := range f.YAML.Docs {
			ref, ok := schema.For(a.Type, f.Path, d.Root, cfg.Version())
			if !ok || i >= len(a.Docs) || removed(d.Root, cfg) {
				continue
			}
			ref.Partial = f.Patch
			st, ds := c.validate(ctx, ref, d)
			a.Docs[i].Schema = st
			a.Diagnostics = append(a.Diagnostics, ds...)
		}
	}
	a.Diagnostics = lint.Apply(a.Diagnostics, cfg)
	sortDiagnostics(a.Diagnostics)
	return a
}

// validate checks one document against ref's schema.
func (c *Checker) validate(ctx context.Context, ref schema.Ref, d *yamlkit.Document) (*provider.SchemaStatus, []yamlkit.Diagnostic) {
	st := &provider.SchemaStatus{Title: ref.Title, URL: ref.URL, State: "ok"}
	sch, err := c.schemas.Get(ctx, ref.URL)
	switch {
	case err == nil:
		return st, schema.Validate(sch, d, ref)
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		st.State, st.Message = "pending", "Downloading the schema…"
	case errors.Is(err, schemacache.ErrNotPublished):
		st.State, st.Message = "none", "No schema is published for "+ref.Title
	default:
		st.State, st.Message = "unavailable", "Schema unavailable: "+err.Error()
	}
	return st, nil
}

// Manifest checks rendered Helm output as Kubernetes objects.
func (c *Checker) Manifest(ctx context.Context, manifest string, wait time.Duration) []yamlkit.Diagnostic {
	f := &provider.File{Path: "manifest.yaml", Content: []byte(manifest), YAML: yamlkit.Parse([]byte(manifest))}
	cfg, schemasOn := c.settings()
	out := lint.Run(lint.Input{YAML: f.YAML, Content: f.Content, Type: "kubernetes"}, cfg)
	if schemasOn {
		ctx, cancel := context.WithTimeout(ctx, wait)
		defer cancel()
		for _, d := range f.YAML.Docs {
			if ref, ok := schema.For("kubernetes", f.Path, d.Root, cfg.Version()); ok && !removed(d.Root, cfg) {
				_, ds := c.validate(ctx, ref, d)
				out = append(out, ds...)
			}
		}
	}
	out = lint.Apply(out, cfg)
	sortDiagnostics(out)
	return out
}

// ref finds the schema for the document around pos, using typ (the
// file's known type) when the text doesn't parse mid-edit.
func (c *Checker) ref(f *provider.File, typ string, pos yamlkit.Pos) (schema.Ref, bool) {
	cfg, schemasOn := c.settings()
	if !schemasOn {
		return schema.Ref{}, false
	}
	if d := f.YAML.DocAt(pos); d != nil && d.Root != nil {
		if typ == "" || typ == "yaml" {
			typ = c.reg.Best(f).ID()
		}
		return schema.For(typ, f.Path, d.Root, cfg.Version())
	}
	if ref, ok := schema.For(typ, f.Path, &yamlkit.Node{Kind: yamlkit.KindMap}, cfg.Version()); ok {
		return ref, true // typed by file (CI, Compose)
	}
	apiVersion, kind := schema.Identity(f.Content, pos.Line)
	if apiVersion == "" || kind == "" || typ == "helm-template" {
		return schema.Ref{}, false
	}
	if kind == "Kustomization" {
		return schema.Ref{URL: schema.Kustomization, Title: "Kustomization"}, true
	}
	return schema.KubernetesRef(apiVersion, kind, cfg.Version())
}

// Complete suggests schema keys and values at pos.
func (c *Checker) Complete(ctx context.Context, f *provider.File, typ string, pos yamlkit.Pos) []provider.Completion {
	ref, ok := c.ref(f, typ, pos)
	if !ok {
		return nil
	}
	sch, err := c.schemas.Get(ctx, ref.URL)
	if err != nil {
		return nil
	}
	items, from := schema.Complete(sch, f.Content, pos.Line, pos.Col)
	start := yamlkit.PosAt(f.Content, pos.Line, from)
	out := make([]provider.Completion, 0, len(items))
	for _, it := range items {
		out = append(out, provider.Completion{
			Label: it.Label, Insert: it.Insert, Detail: it.Detail, Doc: it.Doc, Kind: it.Kind,
			Range: yamlkit.Range{Start: start, End: pos},
		})
	}
	return out
}

// HoverRows describes the value at pos from its schema.
func (c *Checker) HoverRows(ctx context.Context, f *provider.File, typ string, pos yamlkit.Pos) []provider.HoverRow {
	d := f.YAML.DocAt(pos)
	if d == nil || d.Root == nil {
		return nil
	}
	ref, ok := c.ref(f, typ, pos)
	if !ok {
		return nil
	}
	sch, err := c.schemas.Get(ctx, ref.URL)
	if err != nil {
		return nil
	}
	path, _ := d.PathAt(pos)
	info, ok := schema.InfoAt(sch, schema.StepsOf(path))
	if !ok || len(path) == 0 {
		return nil
	}
	var rows []provider.HoverRow
	if info.Description != "" {
		rows = append(rows, provider.HoverRow{Label: "Docs", Value: clipText(info.Description, 400)})
	}
	if len(info.Types) > 0 {
		rows = append(rows, provider.HoverRow{Label: "Schema type", Value: strings.Join(info.Types, " | ")})
	}
	if len(info.Enum) > 0 {
		rows = append(rows, provider.HoverRow{Label: "Allowed", Value: strings.Join(info.Enum, ", ")})
	}
	if info.Default != "" {
		rows = append(rows, provider.HoverRow{Label: "Default", Value: info.Default})
	}
	if info.Deprecated {
		rows = append(rows, provider.HoverRow{Label: "Deprecated", Value: "yes"})
	}
	return append(rows, provider.HoverRow{Label: "Schema", Value: ref.Title})
}

// removed reports a document whose API the target version dropped:
// api-removed already says so, and no schema exists for it.
func removed(root *yamlkit.Node, cfg lint.Config) bool {
	return lint.Removed(root.Get("apiVersion").Str(), root.Get("kind").Str(), cfg.Version())
}

func clipText(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

func sortDiagnostics(ds []yamlkit.Diagnostic) {
	slices.SortStableFunc(ds, func(x, y yamlkit.Diagnostic) int {
		switch {
		case x.Range.Start.Before(y.Range.Start):
			return -1
		case y.Range.Start.Before(x.Range.Start):
			return 1
		}
		return 0
	})
}
