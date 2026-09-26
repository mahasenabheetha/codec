package workspace

import (
	"errors"
	"io/fs"
	"path"
	"strings"
)

// MaxTreeSize caps ReadTree, so a chart with huge vendored files can't
// exhaust memory (Helm's own loader budget is similar).
const MaxTreeSize = 64 << 20

// TreeFile is one file read by ReadTree.
type TreeFile struct {
	Path string // relative to the directory read, slash-separated
	Data []byte
}

// ErrTreeTooLarge is returned when a directory exceeds MaxTreeSize.
var ErrTreeTooLarge = errors.New("folder is larger than 64 MB")

// ReadTree reads every regular file under dir ("" = the root) through
// the root, for loaders that need a whole directory, such as a Helm
// chart. skip, if set, filters paths relative to dir (return true to
// leave a file out, or a whole directory). Unlike the tree listing it
// ignores .gitignore and size limits per file: a chart is what it is.
func (w *Workspace) ReadTree(dir string, skip func(rel string, isDir bool) bool) ([]TreeFile, error) {
	start := dir
	if start == "" {
		start = "."
	} else if err := validPath(dir); err != nil {
		return nil, err
	}
	var out []TreeFile
	total := 0
	err := fs.WalkDir(w.fsys, start, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(p, start), "/")
		if start == "." {
			rel = p
		}
		if rel == "" || rel == "." {
			return nil
		}
		if skip != nil && skip(rel, d.IsDir()) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() || (!d.Type().IsRegular() && d.Type()&fs.ModeSymlink == 0) {
			return nil
		}
		data, err := fs.ReadFile(w.fsys, path.Clean(p))
		if err != nil {
			return w.wrapErr(p, err)
		}
		if total += len(data); total > MaxTreeSize {
			return ErrTreeTooLarge
		}
		out = append(out, TreeFile{Path: rel, Data: data})
		return nil
	})
	return out, err
}
