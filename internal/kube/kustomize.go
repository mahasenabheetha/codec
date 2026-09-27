package kube

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// KustomizationFiles are the names kustomize looks for, in its order.
var KustomizationFiles = []string{"kustomization.yaml", "kustomization.yml", "Kustomization"}

// ErrRemote is returned for remote bases: codec never downloads.
var ErrRemote = errors.New("remote resources (git or http URLs) aren't supported: codec never downloads")

// Reader gives read-only access to a workspace by slash path.
type Reader interface {
	ReadFile(p string) ([]byte, error)
	IsDir(p string) bool
}

// KustomizeFiles collects every file the kustomization in dir needs:
// its resources, bases, components, patches and generator inputs,
// following directories to their own kustomizations. The result feeds
// an in-memory filesystem, so builds never touch the disk and what-if
// edits can stand in for files.
func KustomizeFiles(r Reader, dir string) (map[string][]byte, error) {
	c := &collector{r: r, files: map[string][]byte{}, seen: map[string]bool{}}
	if err := c.dir(path.Clean(dir)); err != nil {
		return nil, err
	}
	return c.files, nil
}

type collector struct {
	r     Reader
	files map[string][]byte
	seen  map[string]bool
}

func (c *collector) file(p string) error {
	p = path.Clean(p)
	if _, ok := c.files[p]; ok {
		return nil
	}
	data, err := c.r.ReadFile(p)
	if err != nil {
		return fmt.Errorf("%s: %w", p, err)
	}
	c.files[p] = data
	return nil
}

func (c *collector) dir(dir string) error {
	if c.seen[dir] {
		return nil
	}
	c.seen[dir] = true
	var kfile string
	for _, name := range KustomizationFiles {
		p := path.Join(dir, name)
		if _, err := c.r.ReadFile(p); err == nil {
			kfile = p
			break
		}
	}
	if kfile == "" {
		return fmt.Errorf("%s: no kustomization.yaml here", orText(dir, "."))
	}
	if err := c.file(kfile); err != nil {
		return err
	}
	f := yamlkit.Parse(c.files[kfile])
	if len(f.Docs) == 0 || f.Docs[0].Root == nil {
		return fmt.Errorf("%s: not a valid kustomization", kfile)
	}
	k := f.Docs[0].Root
	if k.Get("helmCharts") != nil || k.Get("helmChartInflationGenerator") != nil {
		return fmt.Errorf("%s: helmCharts need the helm binary, which codec doesn't run; render the chart in the Helm view instead", kfile)
	}

	// Entries that may be files or directories (kustomizations).
	for _, key := range []string{"resources", "bases", "components", "generators", "transformers", "validators"} {
		for _, it := range items(k.Get(key)) {
			if err := c.entry(dir, it.Str()); err != nil {
				return err
			}
		}
	}
	// Entries that are files.
	files := func(n *yamlkit.Node) error {
		if p := n.Str(); p != "" && !strings.Contains(p, "\n") {
			return c.file(path.Join(dir, p))
		}
		return nil
	}
	for _, key := range []string{"patchesStrategicMerge", "crds"} {
		for _, it := range items(k.Get(key)) {
			if err := files(it); err != nil {
				return err
			}
		}
	}
	for _, key := range []string{"patches", "patchesJson6902", "replacements"} {
		for _, it := range items(k.Get(key)) {
			if err := files(it.Get("path")); err != nil {
				return err
			}
		}
	}
	if err := files(k.Get("openapi").Get("path")); err != nil {
		return err
	}
	for _, key := range []string{"configMapGenerator", "secretGenerator"} {
		for _, g := range items(k.Get(key)) {
			for _, it := range items(g.Get("files")) {
				p := it.Str()
				if _, after, ok := strings.Cut(p, "="); ok { // key=path
					p = after
				}
				if err := c.file(path.Join(dir, p)); err != nil {
					return err
				}
			}
			for _, it := range items(g.Get("envs")) {
				if err := files(it); err != nil {
					return err
				}
			}
			if err := files(g.Get("env")); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *collector) entry(dir, p string) error {
	if p == "" {
		return nil
	}
	if strings.Contains(p, "://") || strings.HasPrefix(p, "github.com/") || strings.HasPrefix(p, "git@") || strings.Contains(p, "?ref=") {
		return fmt.Errorf("%s: %w", p, ErrRemote)
	}
	full := path.Join(dir, p)
	if c.r.IsDir(full) {
		return c.dir(full)
	}
	return c.file(full)
}

// Kustomize builds the kustomization in dir from files (slash paths as
// collected by KustomizeFiles), like `kubectl kustomize dir`.
func Kustomize(files map[string][]byte, dir string) (string, error) {
	fs := filesys.MakeFsInMemory()
	for p, data := range files {
		if err := fs.WriteFile("/"+p, data); err != nil {
			return "", err
		}
	}
	opts := krusty.MakeDefaultOptions()
	// Like kubectl: honour sortOptions, else kustomize's legacy kind order.
	opts.Reorder = krusty.ReorderOptionUnspecified
	m, err := krusty.MakeKustomizer(opts).Run(fs, "/"+path.Clean(dir))
	if err != nil {
		return "", kustomizeError(err)
	}
	out, err := m.AsYaml()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// kustomizeError drops the in-memory root from paths in messages.
func kustomizeError(err error) error {
	return errors.New(strings.ReplaceAll(err.Error(), "'/", "'"))
}

// PatchFiles returns the files (slash paths, joined to dir) that the
// kustomization in dir applies as patches. They hold partial objects,
// so checks that want complete ones should go easy on them.
func PatchFiles(dir string, kustomization []byte) []string {
	f := yamlkit.Parse(kustomization)
	if len(f.Docs) == 0 || f.Docs[0].Root == nil {
		return nil
	}
	k := f.Docs[0].Root
	var out []string
	add := func(n *yamlkit.Node) {
		// An inline patch is YAML text, not a file name.
		if p := n.Str(); p != "" && !strings.Contains(p, "\n") {
			out = append(out, path.Join(dir, p))
		}
	}
	for _, it := range items(k.Get("patchesStrategicMerge")) {
		add(it)
	}
	for _, key := range []string{"patches", "patchesJson6902"} {
		for _, it := range items(k.Get(key)) {
			add(it.Get("path"))
		}
	}
	return out
}
