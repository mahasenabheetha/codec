package web

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/mahasenabheetha/codec/v2/internal/config"
	"github.com/mahasenabheetha/codec/v2/internal/helm"
	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/workspace"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// helmCache keeps the latest render per chart, so hovering a .Values
// expression in a template can show the value it rendered with.
type helmCache struct {
	mu   sync.Mutex
	last map[string]*helm.Result // workspace root + chart path -> result
}

func (c *helmCache) put(key string, r *helm.Result) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.last == nil {
		c.last = map[string]*helm.Result{}
	}
	c.last[key] = r
}

func (c *helmCache) get(key string) *helm.Result {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.last[key]
}

// chartView is one chart for the Helm view's picker.
type chartView struct {
	Path       string                        `json:"path"` // workspace-relative; "." = root
	Name       string                        `json:"name"`
	Version    string                        `json:"version,omitempty"`
	Candidates []string                      `json:"candidates"` // values files to layer
	Profiles   map[string]config.HelmProfile `json:"profiles"`
	Active     string                        `json:"active,omitempty"`
}

// chartKey identifies a chart across workspaces: its absolute path.
func chartKey(ws *workspace.Workspace, chart string) string {
	return filepath.Join(ws.Root(), filepath.FromSlash(chart))
}

// chartDir turns a chart path into ReadTree's directory ("" = root).
func chartDir(chart string) string {
	if chart == "." {
		return ""
	}
	return chart
}

// GET /api/v2/helm/charts
func (s *Server) handleHelmCharts(w http.ResponseWriter, r *http.Request) {
	ws := s.current()
	if ws == nil {
		writeError(w, http.StatusConflict, "no folder is open")
		return
	}
	files, _, err := ws.Files(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}
	settings := s.opts.Config.Get()
	out := []chartView{}
	for _, root := range helm.FindCharts(paths) {
		cv := chartView{Path: root, Name: path.Base(root), Candidates: valuesCandidates(root, files)}
		if c, err := ws.Read(path.Join(root, "Chart.yaml")); err == nil && c.YAML != nil && len(c.YAML.Docs) > 0 {
			if n := c.YAML.Docs[0].Root; n != nil {
				cv.Name = cmp.Or(n.Get("name").Str(), cv.Name)
				cv.Version = n.Get("version").Str()
			}
		}
		if cv.Path == "." && cv.Name == "." {
			cv.Name = ws.Name()
		}
		saved := settings.Helm[chartKey(ws, root)]
		cv.Profiles, cv.Active = saved.Profiles, saved.Active
		if cv.Profiles == nil {
			cv.Profiles = map[string]config.HelmProfile{}
		}
		out = append(out, cv)
	}
	writeJSON(w, http.StatusOK, map[string]any{"charts": out})
}

// valuesCandidates lists files worth offering as values layers: YAML
// files next to Chart.yaml, and anything detected as Helm values
// elsewhere (environment folders, Argo CD value files). Files inside
// the chart come first.
func valuesCandidates(root string, files []workspace.File) []string {
	in := func(p string) bool { return root == "." || strings.HasPrefix(p, root+"/") }
	own := path.Join(root, "values.yaml")
	var inside, outside []string
	for _, f := range files {
		if f.Lang != "yaml" || f.Path == own {
			continue
		}
		dir, base := path.Dir(f.Path), path.Base(f.Path)
		switch {
		case dir == path.Clean(root) && base != "Chart.yaml" && base != "Chart.lock":
			inside = append(inside, f.Path)
		case f.Type == "helm-values" && !strings.Contains(f.Path, "/charts/") && !(in(f.Path) && strings.Contains(f.Path, "/templates/")):
			if in(f.Path) {
				inside = append(inside, f.Path)
			} else {
				outside = append(outside, f.Path)
			}
		}
	}
	slices.Sort(inside)
	slices.Sort(outside)
	return append(inside, outside...)
}

type helmRenderRequest struct {
	Chart       string            `json:"chart"`
	Values      []string          `json:"values"`    // workspace paths, lowest precedence first
	Set         []string          `json:"set"`       // --set expressions
	Overrides   map[string]string `json:"overrides"` // workspace path -> what-if content
	Release     string            `json:"release"`
	Namespace   string            `json:"namespace"`
	KubeVersion string            `json:"kubeVersion"`
	APIVersions []string          `json:"apiVersions"`
}

type helmRenderResponse struct {
	*helm.Result
	ValuesYAML  string         `json:"valuesYAML"`
	ValuesLines map[int]string `json:"valuesLines"`
}

// POST /api/v2/helm/render
func (s *Server) handleHelmRender(w http.ResponseWriter, r *http.Request) {
	var req helmRenderRequest
	if !decode(w, r, &req) {
		return
	}
	ws := s.current()
	if ws == nil {
		writeError(w, http.StatusConflict, "no folder is open")
		return
	}
	req.Chart = cmp.Or(req.Chart, ".")
	res, he := s.renderHelm(r.Context(), ws, req)
	if he != nil {
		writeError(w, he.status, he.msg)
		return
	}
	if r.Context().Err() != nil {
		return // the client moved on
	}
	s.lintRendered(r.Context(), res, req.Chart)
	s.helm.put(chartKey(ws, req.Chart), res)
	text, lines := helm.ValuesYAML(res.Values)
	writeJSON(w, http.StatusOK, helmRenderResponse{Result: res, ValuesYAML: text, ValuesLines: lines})
}

// renderHelm renders a chart of the open folder; the Helm view,
// compare and query share it.
func (s *Server) renderHelm(ctx context.Context, ws *workspace.Workspace, req helmRenderRequest) (*helm.Result, *httpError) {
	files, err := loadChart(ws, req.Chart, req.Overrides)
	if err != nil {
		he := readError(err)
		return nil, &httpError{he.status, "load chart: " + he.msg}
	}
	opts := helm.Options{Set: req.Set, Release: req.Release, Namespace: req.Namespace, KubeVersion: req.KubeVersion, APIVersions: req.APIVersions}
	for _, p := range req.Values {
		text, ok := req.Overrides[p]
		if !ok {
			c, err := ws.Read(p)
			if err != nil {
				he := readError(err)
				return nil, &httpError{he.status, p + ": " + he.msg}
			}
			text = c.Text
		}
		opts.Values = append(opts.Values, helm.Layer{Name: p, Data: []byte(text)})
	}
	res, _ := cancellable(ctx, func() (*helm.Result, error) {
		return helm.Render(ctx, files, opts), nil
	})
	if res == nil {
		return nil, &httpError{http.StatusRequestTimeout, "cancelled"}
	}
	// Diagnostics name chart-relative files; the UI wants workspace paths.
	for i, d := range res.Diagnostics {
		if d.File != "" && d.File != "--set" && !slices.Contains(req.Values, d.File) {
			res.Diagnostics[i].File = path.Join(req.Chart, d.File)
		}
	}
	return res, nil
}

// renderProfile renders a chart with one of its saved profiles ("" =
// the chart's defaults).
func (s *Server) renderProfile(ctx context.Context, ws *workspace.Workspace, chart, profile string) (*helm.Result, *httpError) {
	chart = cmp.Or(chart, ".")
	req := helmRenderRequest{Chart: chart}
	if profile != "" {
		p, ok := s.opts.Config.Get().Helm[chartKey(ws, chart)].Profiles[profile]
		if !ok {
			return nil, &httpError{http.StatusNotFound, "no profile " + profile + " for chart " + chart}
		}
		req.Values, req.Set, req.Release, req.Namespace, req.KubeVersion = p.Values, p.Set, p.Release, p.Namespace, p.KubeVersion
	}
	return s.renderHelm(ctx, ws, req)
}

// helmSourcePath maps a rendered document's source, named after the
// chart ("app/templates/x.yaml"), to its workspace path.
func helmSourcePath(chart, src string) string {
	_, rest, ok := strings.Cut(src, "/")
	if !ok {
		return ""
	}
	return path.Join(chart, rest)
}

// loadChart reads a chart directory through the workspace, applying
// .helmignore like `helm template` does, with what-if overrides for
// files inside it.
func loadChart(ws *workspace.Workspace, chart string, overrides map[string]string) ([]helm.File, error) {
	var ignoreRules []byte
	if c, err := ws.Read(path.Join(chart, ".helmignore")); err == nil {
		ignoreRules = []byte(c.Text)
	}
	skip, err := helm.Ignorer(ignoreRules)
	if err != nil {
		return nil, err
	}
	tree, err := ws.ReadTree(chartDir(chart), skip)
	if err != nil {
		return nil, err
	}
	files := make([]helm.File, 0, len(tree))
	for _, f := range tree {
		data := f.Data
		if text, ok := overrides[path.Join(chart, f.Path)]; ok {
			data = []byte(text)
		}
		files = append(files, helm.File{Name: f.Path, Data: data})
	}
	return files, nil
}

// POST /api/v2/helm/profiles {chart, profiles, active}: save the chart's
// render profiles in the user's settings.
func (s *Server) handleHelmProfiles(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Chart    string                        `json:"chart"`
		Profiles map[string]config.HelmProfile `json:"profiles"`
		Active   string                        `json:"active"`
	}
	if !decode(w, r, &req) {
		return
	}
	ws := s.current()
	if ws == nil {
		writeError(w, http.StatusConflict, "no folder is open")
		return
	}
	key := chartKey(ws, cmp.Or(req.Chart, "."))
	err := s.opts.Config.Update(func(st *config.Settings) {
		if st.Helm == nil {
			st.Helm = map[string]config.HelmChart{}
		}
		if len(req.Profiles) == 0 {
			delete(st.Helm, key)
			return
		}
		st.Helm[key] = config.HelmChart{Profiles: req.Profiles, Active: req.Active}
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// lookup returns the latest render of the chart containing the file
// at abs (an absolute path), and the file's path inside that chart.
func (c *helmCache) lookup(abs string) (*helm.Result, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	best := ""
	for key := range c.last {
		if strings.HasPrefix(abs, key+string(filepath.Separator)) && len(key) > len(best) {
			best = key
		}
	}
	if best == "" {
		return nil, ""
	}
	rel, _ := filepath.Rel(best, abs)
	return c.last[best], filepath.ToSlash(rel)
}

var reHoverValues = regexp.MustCompile(`\$?\.Values((?:\.[A-Za-z_][A-Za-z0-9_]*)+)`)

// helmHoverRows adds what the chart's last render knows to a hover on a
// template expression that reads .Values: the value it rendered with
// and where that value came from.
func (s *Server) helmHoverRows(ws *workspace.Workspace, file, expr string) []provider.HoverRow {
	m := reHoverValues.FindStringSubmatch(expr)
	if m == nil {
		return nil
	}
	res, rel := s.helm.lookup(filepath.Join(ws.Root(), filepath.FromSlash(file)))
	if res == nil {
		return nil
	}
	var p yamlkit.Path
	// A subchart's templates see the values under its key.
	if rest, ok := strings.CutPrefix(rel, "charts/"); ok {
		if dir, _, ok := strings.Cut(rest, "/"); ok {
			p = append(p, yamlkit.Segment{Key: dir})
		}
	}
	for _, k := range strings.Split(strings.TrimPrefix(m[1], "."), ".") {
		p = append(p, yamlkit.Segment{Key: k})
	}
	var cur any = res.Values
	for _, seg := range p {
		mm, ok := cur.(map[string]any)
		if !ok {
			cur = nil
			break
		}
		cur = mm[seg.Key]
	}
	rows := []provider.HoverRow{{Label: "Rendered with", Value: fmt.Sprint(summarizeValue(cur))}}
	if chain := res.Provenance[p.String()]; len(chain) > 0 {
		rows = append(rows, provider.HoverRow{Label: "From", Value: describeChain(chain)})
	} else if cur == nil {
		rows = append(rows, provider.HoverRow{Label: "From", Value: "not set in any values layer"})
	}
	return rows
}

func summarizeValue(v any) any {
	switch t := v.(type) {
	case nil:
		return "(not set)"
	case map[string]any:
		return fmt.Sprintf("{%d keys}", len(t))
	case []any:
		return fmt.Sprintf("[%d items]", len(t))
	}
	return v
}

// describeChain renders "values-prod.yaml:12 (overrides values.yaml:4)".
func describeChain(chain []helm.Origin) string {
	where := func(o helm.Origin) string {
		if o.Line > 0 {
			return fmt.Sprintf("%s:%d", o.Layer, o.Line)
		}
		return o.Layer
	}
	s := where(chain[0])
	if chain[0].Deleted {
		s += " (null: removed)"
	}
	if len(chain) > 1 {
		var rest []string
		for _, o := range chain[1:] {
			rest = append(rest, where(o))
		}
		s += " (overrides " + strings.Join(rest, ", ") + ")"
	}
	return s
}
