package argo

import (
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// App is an Argo CD Application (or an ApplicationSet's template):
// what it deploys, from where, to where.
type App struct {
	Kind        string      `json:"kind"`
	Name        string      `json:"name"`
	Namespace   string      `json:"namespace,omitempty"`
	Project     string      `json:"project,omitempty"`
	Source      Source      `json:"source"`
	Sources     []AppSource `json:"sources"`
	Destination Destination `json:"destination"`
	Sync        string      `json:"sync,omitempty"`
	Generators  []string    `json:"generators,omitempty"` // ApplicationSet
}

// AppSource is one source of an application.
type AppSource struct {
	RepoURL  string   `json:"repoURL,omitempty"`
	Path     string   `json:"path,omitempty"`
	Chart    string   `json:"chart,omitempty"`
	Revision string   `json:"revision,omitempty"`
	Ref      string   `json:"ref,omitempty"`
	Helm     *AppHelm `json:"helm,omitempty"`
	Tool     string   `json:"tool"` // helm, kustomize, directory, plugin, or "" when Argo CD detects it
	// Local is the chart in the open folder this source deploys, if
	// the caller found one.
	Local *LocalChart `json:"local,omitempty"`
}

// AppHelm is a source's helm section.
type AppHelm struct {
	Release    string   `json:"releaseName,omitempty"`
	ValueFiles []string `json:"valueFiles,omitempty"`
	Parameters []string `json:"parameters,omitempty"` // name=value, like --set
	Values     string   `json:"values,omitempty"`     // inline values (values or valuesObject)
}

// LocalChart links a source to a chart in the workspace, with the
// value files that exist there.
type LocalChart struct {
	Chart   string   `json:"chart"`             // workspace path of the chart
	Values  []string `json:"values,omitempty"`  // workspace paths, in order
	Missing []string `json:"missing,omitempty"` // value files not found
	Match   string   `json:"match"`             // "path" or "name" (a chart from a registry)
}

// Destination is where an application deploys.
type Destination struct {
	Server    string `json:"server,omitempty"`
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

// Apps reads the Argo CD applications of a parsed file.
func Apps(file string, f *yamlkit.File) []App {
	var out []App
	for i, d := range f.Docs {
		r := d.Root
		if r == nil || !strings.HasPrefix(r.Get("apiVersion").Str(), "argoproj.io/") {
			continue
		}
		kind := r.Get("kind").Str()
		spec := r.Get("spec")
		switch kind {
		case "Application":
		case "ApplicationSet":
			spec = spec.Get("template").Get("spec")
		default:
			continue
		}
		a := App{
			Sources:   []AppSource{},
			Kind:      kind,
			Name:      r.Get("metadata").Get("name").Str(),
			Namespace: r.Get("metadata").Get("namespace").Str(),
			Project:   spec.Get("project").Str(),
			Source:    Source{File: file, Doc: i, Line: r.Range.Start.Line, Range: r.Range},
		}
		for _, s := range append([]*yamlkit.Node{spec.Get("source")}, items(spec.Get("sources"))...) {
			if s != nil {
				a.Sources = append(a.Sources, appSource(s))
			}
		}
		dst := spec.Get("destination")
		a.Destination = Destination{Server: dst.Get("server").Str(), Name: dst.Get("name").Str(), Namespace: dst.Get("namespace").Str()}
		if auto := spec.Get("syncPolicy").Get("automated"); auto != nil {
			var opts []string
			for _, k := range []string{"prune", "selfHeal"} {
				if auto.Get(k).Str() == "true" {
					opts = append(opts, k)
				}
			}
			a.Sync = "automated"
			if len(opts) > 0 {
				a.Sync += " (" + strings.Join(opts, ", ") + ")"
			}
		} else if spec.Get("syncPolicy") != nil || kind == "Application" {
			a.Sync = "manual"
		}
		if kind == "ApplicationSet" {
			for _, g := range items(r.Get("spec").Get("generators")) {
				a.Generators = append(a.Generators, generator(g))
			}
		}
		out = append(out, a)
	}
	return out
}

func appSource(s *yamlkit.Node) AppSource {
	as := AppSource{
		RepoURL:  s.Get("repoURL").Str(),
		Path:     s.Get("path").Str(),
		Chart:    s.Get("chart").Str(),
		Revision: s.Get("targetRevision").Str(),
		Ref:      s.Get("ref").Str(),
	}
	switch {
	case s.Get("helm") != nil || as.Chart != "":
		as.Tool = "helm"
		h := s.Get("helm")
		as.Helm = &AppHelm{Release: h.Get("releaseName").Str(), ValueFiles: scalars(h.Get("valueFiles"))}
		for _, p := range items(h.Get("parameters")) {
			as.Helm.Parameters = append(as.Helm.Parameters, p.Get("name").Str()+"="+p.Get("value").Str())
		}
		if v := h.Get("values"); v != nil {
			as.Helm.Values = v.Str()
		} else if v := h.Get("valuesObject"); v != nil {
			as.Helm.Values = yamlkit.ValueText(v)
		}
	case s.Get("kustomize") != nil:
		as.Tool = "kustomize"
	case s.Get("directory") != nil:
		as.Tool = "directory"
	case s.Get("plugin") != nil:
		as.Tool = "plugin"
	}
	return as
}

// generator names an ApplicationSet generator: "list (3)", "git
// directories", "matrix: git × clusters".
func generator(g *yamlkit.Node) string {
	if g == nil || g.Kind != yamlkit.KindMap || len(g.Pairs) == 0 {
		return "?"
	}
	k := g.Pairs[0].Key.Value
	v := g.Pairs[0].Value
	switch k {
	case "list":
		return "list (" + plural(len(items(v.Get("elements"))), "element") + ")"
	case "git":
		switch {
		case v.Get("directories") != nil:
			return "git directories in " + v.Get("repoURL").Str()
		case v.Get("files") != nil:
			return "git files in " + v.Get("repoURL").Str()
		}
		return "git " + v.Get("repoURL").Str()
	case "matrix", "merge":
		var parts []string
		for _, sub := range items(v.Get("generators")) {
			parts = append(parts, generator(sub))
		}
		sep := " × "
		if k == "merge" {
			sep = " + "
		}
		return k + ": " + strings.Join(parts, sep)
	}
	return k
}

// SourceText is a source in one line: repo, path or chart, revision.
func (s AppSource) SourceText() string {
	what := s.Path
	if s.Chart != "" {
		what = "chart " + s.Chart
	}
	out := strings.TrimSpace(s.RepoURL + " " + what)
	if s.Revision != "" {
		out += " @ " + s.Revision
	}
	return out
}

// CDProvider is the Argo CD lens: an outline that says where an
// application deploys from and to.
type CDProvider struct {
	provider.Provider
}

// Symbols labels source(s) and destination.
func (p CDProvider) Symbols(d *yamlkit.Document) []provider.Symbol {
	syms := provider.Outline(d)
	if d == nil || d.Root == nil {
		return syms
	}
	apps := Apps("", &yamlkit.File{Docs: []*yamlkit.Document{d}})
	if len(apps) == 0 {
		return syms
	}
	a := apps[0]
	spec := d.Root.Get("spec")
	if a.Kind == "ApplicationSet" {
		spec = spec.Get("template").Get("spec")
	}
	if pr := spec.Pair("source"); pr != nil && len(a.Sources) > 0 {
		if s := symbolAt(syms, pr.Key.Range); s != nil {
			s.Detail = a.Sources[0].SourceText()
		}
	}
	if pr := spec.Pair("destination"); pr != nil {
		if s := symbolAt(syms, pr.Key.Range); s != nil {
			s.Detail = strings.Trim(strings.Join(nonEmpty([]string{a.Destination.Server + a.Destination.Name, a.Destination.Namespace}), " · "), " ")
		}
	}
	return syms
}
