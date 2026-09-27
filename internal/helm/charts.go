package helm

import (
	"bytes"
	"errors"
	"io/fs"
	"path"
	"slices"
	"strings"
	"time"

	"helm.sh/helm/v4/pkg/ignore"
)

// FindCharts returns the chart roots among workspace file paths: every
// directory with a Chart.yaml, except subcharts vendored under another
// chart's charts/ directory. "." stands for the workspace root.
func FindCharts(paths []string) []string {
	var roots []string
	for _, p := range paths {
		if path.Base(p) != "Chart.yaml" {
			continue
		}
		roots = append(roots, path.Dir(p))
	}
	slices.Sort(roots)
	var out []string
	for _, r := range roots {
		if !isVendored(r, roots) {
			out = append(out, r)
		}
	}
	return out
}

// isVendored reports whether dir sits in another root's charts/ tree.
func isVendored(dir string, roots []string) bool {
	for _, r := range roots {
		prefix := r + "/charts/"
		if r == "." {
			prefix = "charts/"
		}
		if r != dir && strings.HasPrefix(dir, prefix) {
			return true
		}
	}
	return false
}

// ChartOf returns the chart root that contains p, or "" if none does.
func ChartOf(p string, roots []string) string {
	best := ""
	for _, r := range roots {
		if (r == "." || p == r || strings.HasPrefix(p, r+"/")) && len(r) > len(best) {
			best = r
		}
	}
	return best
}

// Ignorer implements .helmignore (plus Helm's defaults) for loading a
// chart from disk: it reports whether a chart-relative path is skipped.
func Ignorer(helmignore []byte) (func(p string, isDir bool) bool, error) {
	rules := ignore.Empty()
	if helmignore != nil {
		r, err := ignore.Parse(bytes.NewReader(helmignore))
		if err != nil {
			return nil, err
		}
		rules = r
	}
	rules.AddDefaults()
	return func(p string, isDir bool) bool {
		return rules.Ignore(p, fakeInfo{name: path.Base(p), dir: isDir})
	}, nil
}

// fakeInfo is the little of fs.FileInfo that ignore rules look at.
type fakeInfo struct {
	name string
	dir  bool
}

func (f fakeInfo) Name() string       { return f.name }
func (f fakeInfo) Size() int64        { return 0 }
func (f fakeInfo) Mode() fs.FileMode  { return map[bool]fs.FileMode{true: fs.ModeDir, false: 0}[f.dir] }
func (f fakeInfo) ModTime() time.Time { return time.Time{} }
func (f fakeInfo) IsDir() bool        { return f.dir }
func (f fakeInfo) Sys() any           { return nil }

// ErrNotChart is returned for a directory without a Chart.yaml.
var ErrNotChart = errors.New("not a Helm chart")
