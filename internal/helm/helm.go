// Package helm renders Helm charts the way `helm template` does, from
// files held in memory, and explains where every value came from.
//
// It embeds the Helm 4 SDK (decision #9): no helm binary is needed and
// nothing touches the disk or the network. Rendering runs the SDK's
// Install action in client-only dry-run mode — exactly what `helm
// template` does — so output matches the CLI of the same Helm version.
package helm

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"helm.sh/helm/v4/pkg/action"
	ci "helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/common"
	"helm.sh/helm/v4/pkg/chart/common/util"
	"helm.sh/helm/v4/pkg/chart/loader/archive"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"
	release "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/strvals"
)

// File is one chart file, its path relative to the chart root with
// forward slashes (e.g. "templates/deployment.yaml", "charts/db.tgz").
type File struct {
	Name string
	Data []byte
}

// Layer is one user values file (-f), in precedence order.
type Layer struct {
	Name string // shown in provenance, e.g. "values-prod.yaml"
	Data []byte
}

// Options are the `helm template` inputs codec supports.
type Options struct {
	Values      []Layer  // -f, lowest precedence first
	Set         []string // --set, applied after all files
	Release     string   // default "release-name", as helm template
	Namespace   string   // default "default"
	KubeVersion string   // e.g. "1.31"; default is Helm's built-in
	APIVersions []string // extra capabilities (-a)
	IncludeCRDs bool
}

// Doc is one rendered manifest document.
type Doc struct {
	Source string `json:"source"`         // template path, e.g. "app/templates/svc.yaml"
	Kind   string `json:"kind,omitempty"` // from the manifest
	Name   string `json:"name,omitempty"`
	Hook   bool   `json:"hook,omitempty"`
	Line   int    `json:"line"` // 1-based line of "---" in Manifest
}

// Diagnostic is a problem found while loading or rendering, pointing
// at a chart file (or a values layer) when it can.
type Diagnostic struct {
	Severity string `json:"severity"` // error, warning, info
	Code     string `json:"code"`
	Message  string `json:"message"`
	Hint     string `json:"hint,omitempty"`
	File     string `json:"file,omitempty"` // chart-relative path or layer name
	Line     int    `json:"line,omitempty"`
	Col      int    `json:"col,omitempty"`
	// Manifest is the line in Manifest, for findings on the rendered
	// output (lint, schema); File is then the template that made it.
	Manifest int    `json:"manifest,omitempty"`
	Why      string `json:"why,omitempty"`
	Source   string `json:"source,omitempty"` // lint group, as in yamlkit.Diagnostic
}

// Result is a render.
type Result struct {
	Chart       ChartInfo      `json:"chart"`
	Manifest    string         `json:"manifest"` // byte-for-byte `helm template` output
	Docs        []Doc          `json:"docs"`
	Notes       string         `json:"notes,omitempty"`
	Values      map[string]any `json:"values"`     // final merged .Values
	Provenance  Provenance     `json:"provenance"` // path -> origins
	Diagnostics []Diagnostic   `json:"diagnostics"`
	Duration    time.Duration  `json:"durationNs"`
}

// ChartInfo describes the rendered chart.
type ChartInfo struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	AppVersion   string   `json:"appVersion,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
}

// Render loads the chart from files and renders it with opts. It always
// returns a Result; failures are reported as diagnostics, with whatever
// output Helm produced before failing.
func Render(ctx context.Context, files []File, opts Options) *Result {
	start := time.Now()
	res := &Result{Docs: []Doc{}, Values: map[string]any{}, Provenance: Provenance{}, Diagnostics: []Diagnostic{}}
	defer func() { res.Duration = time.Since(start) }()

	buffered := make([]*archive.BufferedFile, 0, len(files))
	for _, f := range files {
		buffered = append(buffered, &archive.BufferedFile{Name: f.Name, Data: bytes.TrimPrefix(f.Data, bom)})
	}
	ch, err := loader.LoadFiles(buffered)
	if err != nil {
		res.fail("load", "The chart can't be loaded: "+err.Error(), "")
		return res
	}
	res.Chart = ChartInfo{Name: ch.Metadata.Name, Version: ch.Metadata.Version, AppVersion: ch.Metadata.AppVersion}
	for _, d := range ch.Metadata.Dependencies {
		res.Chart.Dependencies = append(res.Chart.Dependencies, d.Name)
	}
	if t := ch.Metadata.Type; t != "" && t != "application" {
		res.fail("not-installable", fmt.Sprintf("%s charts can't be rendered on their own", t), "Library charts are used through a chart that depends on them.")
		return res
	}
	if reqs := ch.Metadata.Dependencies; len(reqs) > 0 {
		if err := checkDependencies(ch, reqs); err != nil {
			res.Diagnostics = append(res.Diagnostics, Diagnostic{
				Severity: "error", Code: "missing-dependency", File: "Chart.yaml",
				Message: err.Error(),
				Hint:    "Vendor the dependency into charts/ (helm dependency build). codec never downloads charts.",
			})
			return res
		}
	}

	// User values exactly as the helm CLI builds them.
	vals := map[string]any{}
	for _, l := range opts.Values {
		m, err := loader.LoadValues(bytes.NewReader(l.Data))
		if err != nil {
			res.Diagnostics = append(res.Diagnostics, Diagnostic{Severity: "error", Code: "values", File: l.Name, Message: "Can't read values: " + err.Error()})
			return res
		}
		vals = loader.MergeMaps(vals, m)
	}
	for _, s := range opts.Set {
		if err := strvals.ParseInto(s, vals); err != nil {
			res.Diagnostics = append(res.Diagnostics, Diagnostic{Severity: "error", Code: "set", File: "--set", Message: fmt.Sprintf("Invalid --set %q: %v", s, err)})
			return res
		}
	}

	inst := action.NewInstall(action.NewConfiguration(action.ConfigurationSetLogger(slog.NewTextHandler(io.Discard, nil))))
	inst.DryRunStrategy = action.DryRunClient
	inst.Replace = true // skip the release-name check, as helm template does
	inst.ReleaseName = orDefault(opts.Release, "release-name")
	inst.Namespace = orDefault(opts.Namespace, "default")
	inst.IncludeCRDs = opts.IncludeCRDs
	inst.APIVersions = common.VersionSet(opts.APIVersions)
	if opts.KubeVersion != "" {
		kv, err := common.ParseKubeVersion(opts.KubeVersion)
		if err != nil {
			res.fail("kube-version", fmt.Sprintf("Invalid Kubernetes version %q: %v", opts.KubeVersion, err), "Use a version like 1.31 or v1.31.2.")
			return res
		}
		inst.KubeVersion = kv
	}

	rel, err := inst.RunWithContext(ctx, ch, vals)
	var r *release.Release
	switch v := rel.(type) {
	case *release.Release:
		r = v
	case release.Release:
		r = &v
	}
	if r != nil {
		res.Manifest, res.Docs = assemble(r)
		if r.Info != nil {
			res.Notes = r.Info.Notes
		}
	}
	if err != nil {
		res.Diagnostics = append(res.Diagnostics, renderError(err))
	}

	// The SDK is the authority on the merged values; provenance only
	// explains them. Install has already processed dependencies
	// (conditions, tags, import-values) on ch.
	if final, err := util.CoalesceValues(ch, vals); err == nil {
		res.Values = final
	}
	res.Provenance = provenance(ch, files, opts, res.Values)
	res.Diagnostics = append(res.Diagnostics, checkReferences(ch, files, res.Values)...)
	res.Diagnostics = append(res.Diagnostics, lookupNotes(files)...)
	return res
}

var bom = []byte{0xEF, 0xBB, 0xBF}

// checkDependencies wraps the SDK's check, which wants interface values.
func checkDependencies(ch *chart.Chart, reqs []*chart.Dependency) error {
	deps := make([]ci.Dependency, 0, len(reqs))
	for _, d := range reqs {
		deps = append(deps, d)
	}
	return action.CheckDependencies(ch, deps)
}

// assemble builds the output exactly as `helm template` prints it:
// the manifest, then each hook under its own "# Source:" header.
func assemble(r *release.Release) (string, []Doc) {
	var b strings.Builder
	manifest := strings.TrimSpace(r.Manifest)
	b.WriteString(manifest + "\n")
	for _, h := range r.Hooks {
		fmt.Fprintf(&b, "---\n# Source: %s\n%s\n", h.Path, h.Manifest)
	}
	out := b.String()

	hooks := map[string]bool{}
	for _, h := range r.Hooks {
		hooks[h.Path] = true
	}
	return out, splitDocs(out, hooks)
}

// splitDocs indexes the documents of a manifest by their "# Source:"
// comment, with kind and name read from the document's own lines.
func splitDocs(manifest string, hooks map[string]bool) []Doc {
	docs := []Doc{}
	lines := strings.Split(manifest, "\n")
	var cur *Doc
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "---"):
			docs = append(docs, Doc{Line: i + 1})
			cur = &docs[len(docs)-1]
		case cur == nil:
			continue
		case strings.HasPrefix(line, "# Source: "):
			cur.Source = strings.TrimPrefix(line, "# Source: ")
			cur.Hook = hooks[cur.Source]
		case strings.HasPrefix(line, "kind:") && cur.Kind == "":
			cur.Kind = strings.TrimSpace(strings.TrimPrefix(line, "kind:"))
		case strings.HasPrefix(line, "  name:") && cur.Name == "":
			cur.Name = strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "  name:")), `"'`)
		}
	}
	return docs
}

func (r *Result) fail(code, msg, hint string) {
	r.Diagnostics = append(r.Diagnostics, Diagnostic{Severity: "error", Code: code, Message: msg, Hint: hint, File: "Chart.yaml"})
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
