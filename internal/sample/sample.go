// Package sample holds the sample workspace: a small made-up repository
// with every kind of YAML codec understands, so a new user can try
// codec before (or without) opening their own repository.
//
// The files are embedded in the binary. Extract writes them to codec's
// cache folder, never to a repository, and the workspace then opens
// that folder read-only like any other.
package sample

import (
	"bytes"
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

// all: keeps .github, .gitlab-ci.yml, .env and .helmignore, which embed
// would otherwise skip for starting with a dot.
//
//go:embed all:files
var embedded embed.FS

// Files is the sample repository as a read-only file system.
func Files() fs.FS {
	sub, err := fs.Sub(embedded, "files")
	if err != nil {
		panic(err) // the embed pattern guarantees the folder exists
	}
	return sub
}

// Dir is where the sample is written: codec's folder in the user cache
// directory, or the temp folder where there is none (e.g. a container
// without a home directory).
func Dir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "codec", "sample")
}

// Extract writes the sample into dir. Files whose content already
// matches are left alone, so reopening the sample doesn't wake file
// watchers; anything changed since (by hand, outside codec) is put
// back, so the sample always starts the same.
func Extract(dir string) error {
	src := Files()
	return fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dst := filepath.Join(dir, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		want, err := fs.ReadFile(src, p)
		if err != nil {
			return err
		}
		if have, err := os.ReadFile(dst); err == nil && bytes.Equal(have, want) {
			return nil
		}
		return os.WriteFile(dst, want, 0o644)
	})
}
