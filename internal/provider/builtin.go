package provider

import (
	"path"
	"regexp"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// detector is a detection-only provider with the generic outline. The
// lens phases replace these, one by one, with full providers.
type detector struct {
	id, title string
	detect    func(f *File) Confidence
}

func (d detector) ID() string                                  { return d.id }
func (d detector) Title() string                               { return d.title }
func (d detector) Detect(f *File) Confidence                   { return d.detect(f) }
func (d detector) Symbols(doc *yamlkit.Document) []Symbol      { return Outline(doc) }
func det(id, title string, fn func(*File) Confidence) detector { return detector{id, title, fn} }

// Confidence levels, so detectors agree on what "sure" means.
const (
	byName    Confidence = 95 // a conventional file name/location
	byContent Confidence = 85 // unmistakable content (apiVersion + kind)
	byShape   Confidence = 70 // a plausible structure
	fallback  Confidence = 1  // plain YAML
)

// builtins lists the providers in tie-break order: when two report the
// same confidence, the earlier one wins.
func builtins() []Provider {
	return []Provider{
		det("helm-chart", "Helm chart", detectHelmChart),
		det("helm-template", "Helm template", detectHelmTemplate),
		det("helm-values", "Helm values", detectHelmValues),
		det("kustomize", "Kustomize", detectKustomize),
		det("argo-workflows", "Argo Workflows", detectArgoWorkflows),
		det("argocd", "Argo CD", detectArgoCD),
		det("kubernetes", "Kubernetes", detectKubernetes),
		det("github-actions", "GitHub Actions", detectGitHubActions),
		det("gitlab-ci", "GitLab CI", detectGitLabCI),
		det("azure-pipelines", "Azure Pipelines", detectAzurePipelines),
		det("compose", "Docker Compose", detectCompose),
		det("ansible-playbook", "Ansible playbook", detectAnsiblePlaybook),
		det("ansible-inventory", "Ansible inventory", detectAnsibleInventory),
		det("yaml", "YAML", func(*File) Confidence { return fallback }),
	}
}

// --- helpers ---

func base(f *File) string { return strings.ToLower(path.Base(f.Path)) }

func inDir(f *File, dir string) bool {
	return strings.Contains("/"+strings.ToLower(f.Path), "/"+dir+"/")
}

// roots returns the parsed root of every document.
func roots(f *File) []*yamlkit.Node {
	var out []*yamlkit.Node
	if f.YAML == nil {
		return out
	}
	for _, d := range f.YAML.Docs {
		if d.Root != nil {
			out = append(out, d.Root)
		}
	}
	return out
}

func anyRoot(f *File, pred func(*yamlkit.Node) bool) bool {
	for _, r := range roots(f) {
		if pred(r) {
			return true
		}
	}
	return false
}

// hasKeys reports whether n is a map containing all keys.
func hasKeys(n *yamlkit.Node, keys ...string) bool {
	if n == nil || n.Kind != yamlkit.KindMap {
		return false
	}
	for _, k := range keys {
		if n.Get(k) == nil {
			return false
		}
	}
	return true
}

// anyValue reports whether some value of map n satisfies pred.
func anyValue(n *yamlkit.Node, pred func(*yamlkit.Node) bool) bool {
	if n == nil || n.Kind != yamlkit.KindMap {
		return false
	}
	for _, p := range n.Pairs {
		if pred(p.Value) {
			return true
		}
	}
	return false
}

func anyItem(n *yamlkit.Node, pred func(*yamlkit.Node) bool) bool {
	if n == nil || n.Kind != yamlkit.KindSeq {
		return false
	}
	for _, it := range n.Items {
		if pred(it) {
			return true
		}
	}
	return false
}

func hasGoTemplates(f *File) bool {
	if f.YAML == nil {
		return false
	}
	for _, e := range f.YAML.Expressions {
		if e.Syntax == "go-template" {
			return true
		}
	}
	return false
}

func apiGroupKind(n *yamlkit.Node) (string, string) {
	return n.Get("apiVersion").Str(), n.Get("kind").Str()
}

// --- detectors ---

func detectHelmChart(f *File) Confidence {
	if path.Base(f.Path) == "Chart.yaml" {
		return byName
	}
	if anyRoot(f, func(r *yamlkit.Node) bool {
		v := r.Get("apiVersion").Str()
		return (v == "v1" || v == "v2") && hasKeys(r, "name", "version") && r.Get("kind") == nil
	}) {
		return byShape
	}
	return 0
}

func detectHelmTemplate(f *File) Confidence {
	if !hasGoTemplates(f) {
		return 0
	}
	if inDir(f, "templates") {
		return byName
	}
	for _, e := range f.YAML.Expressions {
		if strings.Contains(e.Text, ".Values") || strings.Contains(e.Text, "include ") || strings.Contains(e.Text, ".Release") {
			return byContent
		}
	}
	return byShape
}

var valuesName = regexp.MustCompile(`^values([.-].*)?\.ya?ml$`)

func detectHelmValues(f *File) Confidence {
	if valuesName.MatchString(base(f)) {
		return byName - 5 // below chart/template, above generic content
	}
	return 0
}

func detectKustomize(f *File) Confidence {
	switch base(f) {
	case "kustomization.yaml", "kustomization.yml":
		return byName
	}
	if anyRoot(f, func(r *yamlkit.Node) bool { return r.Get("kind").Str() == "Kustomization" }) {
		return byContent + 5
	}
	return 0
}

var argoWorkflowKinds = map[string]bool{
	"Workflow": true, "WorkflowTemplate": true, "ClusterWorkflowTemplate": true,
	"CronWorkflow": true, "WorkflowEventBinding": true,
}

var argoCDKinds = map[string]bool{"Application": true, "ApplicationSet": true, "AppProject": true}

func detectArgoWorkflows(f *File) Confidence {
	if anyRoot(f, func(r *yamlkit.Node) bool {
		v, k := apiGroupKind(r)
		return strings.HasPrefix(v, "argoproj.io/") && argoWorkflowKinds[k]
	}) {
		return byName
	}
	return 0
}

func detectArgoCD(f *File) Confidence {
	if anyRoot(f, func(r *yamlkit.Node) bool {
		v, k := apiGroupKind(r)
		return strings.HasPrefix(v, "argoproj.io/") && argoCDKinds[k]
	}) {
		return byName
	}
	return 0
}

func detectKubernetes(f *File) Confidence {
	if anyRoot(f, func(r *yamlkit.Node) bool {
		v, k := apiGroupKind(r)
		return v != "" && k != ""
	}) {
		return byContent
	}
	return 0
}

func detectGitHubActions(f *File) Confidence {
	if inDir(f, ".github/workflows") {
		return byName
	}
	if b := base(f); (b == "action.yml" || b == "action.yaml") && anyRoot(f, func(r *yamlkit.Node) bool { return hasKeys(r, "runs") }) {
		return byName
	}
	if anyRoot(f, func(r *yamlkit.Node) bool {
		return hasKeys(r, "on", "jobs") && anyValue(r.Get("jobs"), func(j *yamlkit.Node) bool {
			return hasKeys(j, "runs-on") || hasKeys(j, "uses")
		})
	}) {
		return byContent
	}
	return 0
}

func detectGitLabCI(f *File) Confidence {
	if b := base(f); b == ".gitlab-ci.yml" || b == ".gitlab-ci.yaml" || inDir(f, ".gitlab-ci") {
		return byName
	}
	if anyRoot(f, func(r *yamlkit.Node) bool {
		return anyValue(r, func(j *yamlkit.Node) bool {
			return hasKeys(j, "script") && (hasKeys(j, "stage") || hasKeys(j, "image"))
		})
	}) {
		return byShape
	}
	return 0
}

var azureName = regexp.MustCompile(`^azure-pipelines.*\.ya?ml$`)

func detectAzurePipelines(f *File) Confidence {
	if azureName.MatchString(base(f)) {
		return byName
	}
	if anyRoot(f, func(r *yamlkit.Node) bool {
		stage := func(n *yamlkit.Node) bool { return hasKeys(n, "stage") }
		task := func(n *yamlkit.Node) bool { return hasKeys(n, "task") }
		return anyItem(r.Get("stages"), stage) || anyItem(r.Get("steps"), task) ||
			(hasKeys(r, "pool") && (hasKeys(r, "steps") || hasKeys(r, "jobs") || hasKeys(r, "stages")))
	}) {
		return byShape + 5
	}
	return 0
}

var composeName = regexp.MustCompile(`^(docker-)?compose([.-].*)?\.ya?ml$`)

func detectCompose(f *File) Confidence {
	if composeName.MatchString(base(f)) {
		return byName
	}
	if anyRoot(f, func(r *yamlkit.Node) bool {
		return r.Get("kind") == nil && anyValue(r.Get("services"), func(s *yamlkit.Node) bool {
			return hasKeys(s, "image") || hasKeys(s, "build")
		})
	}) {
		return byShape
	}
	return 0
}

func detectAnsiblePlaybook(f *File) Confidence {
	play := func(n *yamlkit.Node) bool {
		return hasKeys(n, "import_playbook") ||
			(hasKeys(n, "hosts") && (hasKeys(n, "tasks") || hasKeys(n, "roles") || hasKeys(n, "pre_tasks") || hasKeys(n, "post_tasks")))
	}
	if anyRoot(f, func(r *yamlkit.Node) bool { return anyItem(r, play) }) {
		return byContent + 5
	}
	// A role's task/handler file: a list of named tasks.
	if (inDir(f, "tasks") || inDir(f, "handlers")) && anyRoot(f, func(r *yamlkit.Node) bool {
		return anyItem(r, func(n *yamlkit.Node) bool { return hasKeys(n, "name") && len(n.Pairs) >= 2 })
	}) {
		return byShape
	}
	return 0
}

func detectAnsibleInventory(f *File) Confidence {
	if anyRoot(f, func(r *yamlkit.Node) bool {
		all := r.Get("all")
		return hasKeys(all, "hosts") || hasKeys(all, "children")
	}) {
		return byContent
	}
	if b := base(f); (b == "hosts.yml" || b == "hosts.yaml" || inDir(f, "inventory")) && anyRoot(f, func(r *yamlkit.Node) bool {
		return anyValue(r, func(g *yamlkit.Node) bool { return hasKeys(g, "hosts") || hasKeys(g, "children") })
	}) {
		return byShape
	}
	return 0
}
