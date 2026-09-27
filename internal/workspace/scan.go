package workspace

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

// scan collects one walk's results.
type scan struct {
	ctx       context.Context
	found     map[string]File
	dirs      []string // walked directories (for the watcher)
	failed    []string // unreadable directories: keep what we knew
	truncated bool
}

// rescan walks prefix ("" = the whole folder), updates the index and
// returns what changed plus the directories it walked.
//
// The walk lists directories with os.ReadDir on the real path rather
// than through os.Root: listing names and sizes is not a content read,
// and on Windows it is several times faster (the entries come with
// their metadata). Symlinks are not followed while walking, and every
// content read still goes through os.Root.
func (w *Workspace) rescan(ctx context.Context, prefix string) ([]Change, []string, error) {
	w.scanMu.Lock()
	defer w.scanMu.Unlock()

	s := &scan{ctx: ctx, found: map[string]File{}}
	info, err := os.Lstat(w.abs(prefix))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		// Gone: everything under prefix is removed.
	case err != nil:
		s.failed = append(s.failed, prefix)
	case prefix == "":
		w.walk(s, "", nil)
	case info.IsDir():
		parent := w.stackFor(parentDir(prefix))
		if !skipDirs[path.Base(prefix)] && !ignoredBy(parent, prefix, true) {
			w.walk(s, prefix, parent)
		}
	default:
		if f, ok := w.fileInfo(prefix, fs.FileInfoToDirEntry(info), w.stackFor(parentDir(prefix))); ok {
			s.found[prefix] = f
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	changes := w.apply(prefix, s.found, s.failed)
	if prefix == "" {
		w.mu.Lock()
		w.truncated = s.truncated
		w.mu.Unlock()
	}
	return changes, s.dirs, nil
}

// walk lists dir and recurses into subdirectories. stack holds the
// .gitignore files of dir's ancestors, outermost first.
func (w *Workspace) walk(s *scan, dir string, stack []*ignoreFile) {
	if s.ctx.Err() != nil || s.truncated {
		return
	}
	entries, err := os.ReadDir(w.abs(dir))
	if err != nil {
		s.failed = append(s.failed, dir)
		return
	}
	s.dirs = append(s.dirs, dir)
	// This directory's own .gitignore applies to its entries, so read
	// it before looking at them.
	stack = w.loadIgnore(dir, entries, stack)

	for _, e := range entries {
		p := join(dir, e.Name())
		if e.IsDir() {
			if !skipDirs[e.Name()] && !ignoredBy(stack, p, true) {
				w.walk(s, p, stack)
			}
			continue
		}
		if f, ok := w.fileInfo(p, e, stack); ok {
			s.found[p] = f
			if len(s.found) >= MaxFiles {
				s.truncated = true
				return
			}
		}
	}
}

// fileInfo decides whether a walked file belongs in the tree.
func (w *Workspace) fileInfo(p string, d fs.DirEntry, stack []*ignoreFile) (File, bool) {
	name := d.Name()
	if binaryExt[strings.ToLower(path.Ext(name))] || ignoredBy(stack, p, false) {
		return File{}, false
	}
	var info fs.FileInfo
	var err error
	switch {
	case d.Type().IsRegular():
		info, err = d.Info()
	case d.Type()&fs.ModeSymlink != 0:
		// Follow links that stay inside the root (os.Root refuses the
		// others). Links to directories are skipped so the walk can't loop.
		info, err = w.dir.Stat(filepath.FromSlash(p))
		if err == nil && !info.Mode().IsRegular() {
			return File{}, false
		}
	default:
		return File{}, false // devices, sockets, Windows junctions
	}
	if err != nil || info.Size() > MaxFileSize {
		return File{}, false
	}
	return File{Path: p, Size: info.Size(), ModTime: info.ModTime(), Lang: langOf(name)}, true
}

// apply merges one walk into the index and returns the differences.
func (w *Workspace) apply(prefix string, found map[string]File, failed []string) []Change {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.files == nil {
		w.files = map[string]*entry{}
	}
	var out []Change
	for p := range w.files {
		if !under(p, prefix) || underAny(p, failed) {
			continue
		}
		if _, ok := found[p]; !ok {
			delete(w.files, p)
			out = append(out, Change{Op: Removed, Path: p})
		}
	}
	for p, f := range found {
		old, ok := w.files[p]
		switch {
		case !ok:
			out = append(out, Change{Op: Added, Path: p})
		case old.ModTime.Equal(f.ModTime) && old.Size == f.Size:
			continue
		default:
			out = append(out, Change{Op: Changed, Path: p})
		}
		w.files[p] = &entry{File: f, classified: f.Lang != "yaml"}
	}
	slices.SortFunc(out, func(a, b Change) int { return strings.Compare(a.Path, b.Path) })
	return out
}

// loadIgnore (re)reads dir/.gitignore if the listing has one, records
// it for later single-path checks, and returns stack extended with it.
func (w *Workspace) loadIgnore(dir string, entries []fs.DirEntry, stack []*ignoreFile) []*ignoreFile {
	var f *ignoreFile
	for _, e := range entries {
		if e.Name() == ".gitignore" && !e.IsDir() {
			if data, err := w.dir.ReadFile(filepath.FromSlash(join(dir, ".gitignore"))); err == nil {
				f = parseIgnore(dir, data, w.fold)
			}
			break
		}
	}
	w.mu.Lock()
	if f != nil {
		w.ignores[dir] = f
	} else {
		delete(w.ignores, dir)
	}
	w.mu.Unlock()
	if f == nil {
		return stack
	}
	// Full slice expression: siblings share the parent's stack, so an
	// append must never write into its backing array.
	return append(stack[:len(stack):len(stack)], f)
}

// stackFor returns the known .gitignore files of dir and its
// ancestors, outermost first.
func (w *Workspace) stackFor(dir string) []*ignoreFile {
	w.mu.RLock()
	defer w.mu.RUnlock()
	var stack []*ignoreFile
	for d := dir; ; d = parentDir(d) {
		if f := w.ignores[d]; f != nil {
			stack = append(stack, f)
		}
		if d == "" {
			break
		}
	}
	slices.Reverse(stack)
	return stack
}

// ignoredBy asks the stack innermost first; the deepest .gitignore
// with a matching rule decides, as in git.
func ignoredBy(stack []*ignoreFile, p string, isDir bool) bool {
	for i := len(stack) - 1; i >= 0; i-- {
		if ign, ok := stack[i].match(p, isDir); ok {
			return ign
		}
	}
	return false
}

// excluded reports whether p lies inside a skipped or ignored
// directory (or is one of the always-skipped names). The watcher uses
// it to drop events from .git, node_modules, build output and so on.
func (w *Workspace) excluded(p string) bool {
	parts := strings.Split(p, "/")
	for i, name := range parts {
		if skipDirs[name] {
			return true
		}
		if i < len(parts)-1 {
			dir := strings.Join(parts[:i+1], "/")
			if ignoredBy(w.stackFor(parentDir(dir)), dir, true) {
				return true
			}
		}
	}
	return false
}

// join is path.Join for a directory ("" = root) and a name.
func join(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}
