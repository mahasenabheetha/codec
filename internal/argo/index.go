package argo

import (
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Index holds every workflow spec of a scope (a folder, a render) so
// templateRef and workflowTemplateRef resolve across files, plus the
// ConfigMaps parameters can read with valueFrom.
type Index struct {
	Specs []*Spec
	cms   map[string]map[string]string // ConfigMap name -> data
}

// NewIndex returns an empty index.
func NewIndex() *Index { return &Index{cms: map[string]map[string]string{}} }

// Add indexes a parsed file.
func (ix *Index) Add(file string, content []byte, f *yamlkit.File) {
	ix.Specs = append(ix.Specs, Read(file, content, f)...)
	for _, d := range f.Docs {
		r := d.Root
		if r.Get("kind").Str() != "ConfigMap" || r.Get("apiVersion").Str() != "v1" {
			continue
		}
		data := map[string]string{}
		if m := r.Get("data"); m != nil && m.Kind == yamlkit.KindMap {
			for _, p := range m.Pairs {
				if p.Value != nil && p.Value.Kind == yamlkit.KindScalar && !p.Value.Templated {
					data[p.Key.Value] = p.Value.Value
				}
			}
		}
		if name := r.Get("metadata").Get("name"); name.Str() != "" && !name.Templated {
			ix.cms[name.Value] = data
		}
	}
}

// Lookup finds the spec a reference points at: a WorkflowTemplate or
// a ClusterWorkflowTemplate by name. Namespaces aren't compared: they
// are usually set at install time, not written in the file.
func (ix *Index) Lookup(kind, name string) *Spec {
	for _, s := range ix.Specs {
		if s.Kind == kind && s.Name == name {
			return s
		}
	}
	return nil
}

// Find returns the spec with a Key, a "Kind/name", or a name.
func (ix *Index) Find(id string) *Spec {
	kind, name, ok := strings.Cut(id, "/")
	for _, s := range ix.Specs {
		if s.Key() == id || ok && s.Kind == kind && s.Name == name || !ok && s.Name == id {
			return s
		}
	}
	return nil
}

func (ix *Index) configMap(name, key string) (string, bool) {
	v, ok := ix.cms[name][key]
	return v, ok
}

// RefTarget resolves a templateRef: the spec and its template, with a
// reason when either is missing.
func (ix *Index) RefTarget(r *TemplateRef) (*Spec, *Template, string) {
	s := ix.Lookup(r.Kind(), r.Name)
	if s == nil {
		return nil, nil, r.Kind() + " " + r.Name + " isn't in this folder"
	}
	t := s.Template(r.Template)
	if t == nil {
		return s, nil, r.Kind() + " " + r.Name + " has no template " + r.Template
	}
	return s, t, ""
}
