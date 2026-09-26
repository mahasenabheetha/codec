package workspace

import (
	"cmp"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// Dir is one folder in a folder-browser listing.
type Dir struct {
	Name string `json:"name"`
	Path string `json:"path"`           // absolute OS path
	Repo bool   `json:"repo,omitempty"` // contains .git
}

// Listing is the content of one folder for the open-folder dialog.
type Listing struct {
	Path   string `json:"path"`
	Parent string `json:"parent,omitempty"` // "" at a filesystem root
	Dirs   []Dir  `json:"dirs"`
}

// maxRepoChecks bounds the per-folder .git stat calls on huge folders.
const maxRepoChecks = 500

// ListDirs lists the subfolders of dir (any folder on the machine: this
// is how the user picks a workspace). Only names are read, never file
// contents. Hidden and system folders are left out.
func ListDirs(dir string) (*Listing, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	l := &Listing{Path: dir, Dirs: []Dir{}}
	if parent := filepath.Dir(dir); parent != dir {
		l.Parent = parent
	}
	for _, e := range entries {
		name := e.Name()
		if hiddenDir(name) {
			continue
		}
		full := filepath.Join(dir, name)
		isDir := e.IsDir()
		if !isDir && e.Type()&os.ModeSymlink != 0 {
			info, err := os.Stat(full) // follow links to folders
			isDir = err == nil && info.IsDir()
		}
		if isDir {
			l.Dirs = append(l.Dirs, Dir{Name: name, Path: full})
		}
	}
	slices.SortFunc(l.Dirs, func(a, b Dir) int {
		return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	for i := range l.Dirs[:min(len(l.Dirs), maxRepoChecks)] {
		if _, err := os.Lstat(filepath.Join(l.Dirs[i].Path, ".git")); err == nil {
			l.Dirs[i].Repo = true
		}
	}
	return l, nil
}

func hiddenDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	if runtime.GOOS == "windows" {
		return strings.HasPrefix(name, "$") || name == "System Volume Information"
	}
	return false
}

// Roots lists the filesystem roots: drive letters on Windows, "/"
// elsewhere.
func Roots() []string {
	if runtime.GOOS != "windows" {
		return []string{"/"}
	}
	var out []string
	for c := 'C'; c <= 'Z'; c++ { // A: and B: are floppy drives; probing them can hang
		d := string(c) + `:\`
		if _, err := os.Stat(d); err == nil {
			out = append(out, d)
		}
	}
	return out
}
