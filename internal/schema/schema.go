// Package schema validates YAML documents against JSON Schemas and
// uses the same schemas for editor help: key completion inside a
// Deployment spec, allowed values, and field descriptions on hover.
//
// It is pure. Which schema applies to a document is decided here (For);
// getting the schema's bytes is the job of a Loader supplied by an
// adapter (internal/schemacache fetches and caches them).
package schema

import (
	"path"
	"regexp"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Where schemas come from. Kubernetes schemas are published per
// version by yannh/kubernetes-json-schema ("strict" variants reject
// unknown fields, which catches typos); CRDs by the datreeio catalog;
// CI and Compose schemas by their projects, as listed on SchemaStore.
const (
	KubernetesBase = "https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master/"
	CRDBase        = "https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/"
	GitHubWorkflow = "https://www.schemastore.org/github-workflow.json"
	GitHubAction   = "https://www.schemastore.org/github-action.json"
	GitLabCI       = "https://gitlab.com/gitlab-org/gitlab-foss/-/raw/master/app/assets/javascripts/editor/schema/ci.json"
	AzurePipelines = "https://raw.githubusercontent.com/microsoft/azure-pipelines-vscode/master/service-schema.json"
	Compose        = "https://raw.githubusercontent.com/compose-spec/compose-go/master/schema/compose-spec.json"
	Kustomization  = "https://www.schemastore.org/kustomization.json"
)

// Ref names the schema for one document.
type Ref struct {
	URL   string `json:"url"`
	Title string `json:"title"` // e.g. "Kubernetes 1.34 · Deployment"
	// LooseScalars ignores a scalar of the wrong scalar type (true
	// where a string is wanted, or "1" for a number). Azure Pipelines
	// converts between them itself, and its schema mixes both.
	LooseScalars bool `json:"-"`
}

// For returns the schema for a document of a file of type typ (a
// provider id) at path, or false when there is none. Documents whose
// identity is templated are skipped: their shape is only known after
// rendering.
func For(typ, filePath string, root *yamlkit.Node, k8sVersion string) (Ref, bool) {
	if root == nil || root.Kind != yamlkit.KindMap {
		return Ref{}, false
	}
	switch typ {
	case "helm-template", "helm-values", "helm-chart", "ansible-playbook", "ansible-inventory", "yaml":
		return Ref{}, false
	case "github-actions":
		if b := strings.ToLower(path.Base(filePath)); b == "action.yml" || b == "action.yaml" {
			return Ref{URL: GitHubAction, Title: "GitHub Action"}, true
		}
		return Ref{URL: GitHubWorkflow, Title: "GitHub Actions workflow"}, true
	case "gitlab-ci":
		return Ref{URL: GitLabCI, Title: "GitLab CI"}, true
	case "azure-pipelines":
		return Ref{URL: AzurePipelines, Title: "Azure Pipelines", LooseScalars: true}, true
	case "compose":
		return Ref{URL: Compose, Title: "Compose specification"}, true
	}
	apiVersion, kind := root.Get("apiVersion"), root.Get("kind")
	if apiVersion.Str() == "" || kind.Str() == "" || apiVersion.Templated || kind.Templated {
		return Ref{}, false
	}
	if kind.Value == "Kustomization" {
		return Ref{URL: Kustomization, Title: "Kustomization"}, true
	}
	return KubernetesRef(apiVersion.Value, kind.Value, k8sVersion)
}

var reName = regexp.MustCompile(`^[A-Za-z0-9.-]+$`)

// KubernetesRef is the schema of a Kubernetes object: built-in kinds
// from the versioned Kubernetes schemas, anything else from the CRD
// catalog.
func KubernetesRef(apiVersion, kind, k8sVersion string) (Ref, bool) {
	group, version, ok := strings.Cut(apiVersion, "/")
	if !ok {
		group, version = "", apiVersion
	}
	if !reName.MatchString(kind) || !reName.MatchString(version) || (group != "" && !reName.MatchString(group)) {
		return Ref{}, false
	}
	lower := strings.ToLower(kind)
	if builtinGroup(group) {
		file := lower + "-" + version + ".json"
		if group != "" {
			first, _, _ := strings.Cut(group, ".")
			file = lower + "-" + first + "-" + version + ".json"
		}
		return Ref{
			URL:   KubernetesBase + "v" + k8sVersion + ".0-standalone-strict/" + file,
			Title: "Kubernetes " + k8sVersion + " · " + kind,
		}, true
	}
	return Ref{URL: CRDBase + group + "/" + lower + "_" + version + ".json", Title: kind + " (" + apiVersion + ")"}, true
}

// builtinGroup reports whether an API group ships with Kubernetes.
func builtinGroup(g string) bool {
	switch g {
	case "", "apps", "batch", "autoscaling", "policy":
		return true
	}
	return strings.HasSuffix(g, ".k8s.io")
}

// Loader returns the parsed JSON of a schema URL (as from
// jsonschema.UnmarshalJSON), from cache or network.
type Loader interface {
	Load(url string) (any, error)
}

// Compile compiles the schema at url, loading it and every schema it
// references through l.
func Compile(l Loader, url string) (*jsonschema.Schema, error) {
	c := jsonschema.NewCompiler()
	c.UseLoader(l)
	c.UseRegexpEngine(lenientRegexp)
	return c.Compile(url)
}

// lenientRegexp compiles patterns with Go's regexp. Some published
// schemas use ECMAScript-only syntax (lookaheads); such a pattern
// matches everything rather than making the whole schema unusable.
func lenientRegexp(s string) (jsonschema.Regexp, error) {
	if re, err := regexp.Compile(s); err == nil {
		return re, nil
	}
	return anything(s), nil
}

type anything string

func (a anything) MatchString(string) bool { return true }
func (a anything) String() string          { return string(a) }
