// Package provider defines how codec understands a kind of YAML file
// (Kubernetes, Helm, GitHub Actions, …). Each provider recognises its
// files and describes them; later phases add diagnostics, hover,
// definitions and completion to the same interface (design/architecture.md).
//
// Like yamlkit, providers are pure: they work on parsed content only.
package provider

import (
	"sort"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// File is what providers inspect.
type File struct {
	// Path is slash-separated (e.g. "charts/app/values.yaml"); it may be
	// empty for pasted text, so detection must not depend on it alone.
	Path string
	YAML *yamlkit.File
}

// Confidence is how sure a provider is that a file is its kind:
// 0 = not mine, 100 = certain.
type Confidence int

// Symbol is one entry of a document outline.
type Symbol struct {
	Name     string        `json:"name"`
	Detail   string        `json:"detail,omitempty"`
	Kind     string        `json:"kind"` // map, seq, scalar, alias
	Range    yamlkit.Range `json:"range"`
	Children []Symbol      `json:"children,omitempty"`
}

// Provider understands one kind of YAML file.
type Provider interface {
	ID() string    // stable id, e.g. "kubernetes"
	Title() string // display name, e.g. "Kubernetes"
	Detect(f *File) Confidence
	Symbols(d *yamlkit.Document) []Symbol
}

// Match is a provider with its confidence for one file.
type Match struct {
	Provider   Provider
	Confidence Confidence
}

// Registry holds the providers codec knows about.
type Registry struct {
	providers []Provider
}

// NewRegistry returns a registry with the given providers.
func NewRegistry(ps ...Provider) *Registry {
	return &Registry{providers: ps}
}

// Register adds a provider. Later phases register full lens providers
// that replace the built-in detection-only ones by using the same ID.
func (r *Registry) Register(p Provider) {
	for i, existing := range r.providers {
		if existing.ID() == p.ID() {
			r.providers[i] = p
			return
		}
	}
	r.providers = append(r.providers, p)
}

// Detect returns every provider that claims f, most confident first.
func (r *Registry) Detect(f *File) []Match {
	var out []Match
	for _, p := range r.providers {
		if c := p.Detect(f); c > 0 {
			out = append(out, Match{Provider: p, Confidence: min(c, 100)})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Confidence > out[j].Confidence })
	return out
}

// Best returns the most confident provider for f. The generic YAML
// provider always matches, so this is never nil with Default.
func (r *Registry) Best(f *File) Provider {
	if m := r.Detect(f); len(m) > 0 {
		return m[0].Provider
	}
	return nil
}

// Default is the registry with every built-in provider.
var Default = NewRegistry(builtins()...)
