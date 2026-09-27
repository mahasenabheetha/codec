package check

import (
	"path"
	"slices"
	"sync"

	"github.com/mahasenabheetha/codec/v2/internal/kube"
)

// Patches finds the files kustomizations apply as patches, so their
// partial objects aren't flagged for missing required fields. Each
// folder's kustomization is read once, which keeps a workspace lint of
// thousands of files cheap.
type Patches struct {
	read func(p string) ([]byte, error) // slash path → content

	mu   sync.Mutex
	dirs map[string][]string // folder → the patch files its kustomization lists
}

// NewPatches looks files up with read (e.g. a workspace's reader).
func NewPatches(read func(p string) ([]byte, error)) *Patches {
	return &Patches{read: read, dirs: map[string][]string{}}
}

// Is reports whether a kustomization in p's folder, or in one of the
// two folders above it, lists p as a patch. Patches almost always sit
// next to their kustomization or just below it.
func (ps *Patches) Is(p string) bool {
	if ps == nil || p == "" {
		return false
	}
	p = path.Clean(p)
	dir := path.Dir(p)
	for range 3 {
		if slices.Contains(ps.patchesIn(dir), p) {
			return true
		}
		if dir == "." || dir == "/" {
			break
		}
		dir = path.Dir(dir)
	}
	return false
}

func (ps *Patches) patchesIn(dir string) []string {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if list, ok := ps.dirs[dir]; ok {
		return list
	}
	var list []string
	for _, name := range kube.KustomizationFiles {
		if data, err := ps.read(path.Join(dir, name)); err == nil {
			list = kube.PatchFiles(dir, data)
			break
		}
	}
	ps.dirs[dir] = list
	return list
}
