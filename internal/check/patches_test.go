package check

import (
	"io/fs"
	"testing"
)

func TestPatches(t *testing.T) {
	files := map[string]string{
		"base/kustomization.yaml": "resources: [deploy.yaml]\n",
		"prod/kustomization.yml": "resources: [../base]\n" +
			"patches:\n  - path: replicas.yaml\n  - patch: |-\n      kind: Deployment\n" +
			"patchesStrategicMerge:\n  - patches/limits.yaml\n",
		"deep/a/b/c/kustomization.yaml": "patches: [{path: ../../../x.yaml}]\n",
	}
	read := func(p string) ([]byte, error) {
		if s, ok := files[p]; ok {
			return []byte(s), nil
		}
		return nil, fs.ErrNotExist
	}
	ps := NewPatches(read)
	for p, want := range map[string]bool{
		"prod/replicas.yaml":       true,
		"prod/patches/limits.yaml": true, // listed one folder up
		"base/deploy.yaml":         false,
		"prod/other.yaml":          false,
		"x.yaml":                   false, // its kustomization is too far below
		"":                         false,
	} {
		if got := ps.Is(p); got != want {
			t.Errorf("Is(%q) = %v, want %v", p, got, want)
		}
	}
}
