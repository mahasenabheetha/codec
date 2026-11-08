package workspace

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// virtualise makes w's os.Root act like a folder under a packaged
// app's virtualised %LOCALAPPDATA% (see open): every name except the
// root itself, or only those under one of dirs, fails with err when
// opened relative to the root handle, while by path it is all there.
func virtualise(w *Workspace, err error, dirs ...string) {
	miss := func(name string) bool {
		p := filepath.ToSlash(name)
		return p != "." && (len(dirs) == 0 || underAny(p, dirs))
	}
	open, stat := w.rootOpen, w.rootStat
	w.rootOpen = func(name string) (*os.File, error) {
		if miss(name) {
			return nil, &fs.PathError{Op: "openat", Path: name, Err: err}
		}
		return open(name)
	}
	w.rootStat = func(name string) (fs.FileInfo, error) {
		if miss(name) {
			return nil, &fs.PathError{Op: "statat", Path: name, Err: err}
		}
		return stat(name)
	}
}

// Regression: the sample opened from a Claude desktop terminal
// (%LOCALAPPDATA%\codec\sample) listed every file, but each read failed
// with "openat k8s\base\deployment.yaml: too many levels of symbolic
// links", because the app's file-system virtualisation redirects names
// relative to the os.Root handle and os.Root refuses the redirect.
func TestReadVirtualisedFolder(t *testing.T) {
	root := writeTree(t, map[string]string{
		"README.md":                "# sample\n",
		"k8s/base/deployment.yaml": "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n",
		"k8s/base/cleanup.yaml":    "a: 1\n",
		"k8s/.gitignore":           "local.yaml\n",
		"k8s/local.yaml":           "a: 1\n",
		"logs/app.log":             "line\n",
	})
	w := open(t, root)
	virtualise(w, syscall.ELOOP)

	c, err := w.Read("k8s/base/deployment.yaml")
	if err != nil {
		t.Fatalf("Read nested: %v", err)
	}
	if c.Type == "" || c.Type == "yaml" || !strings.Contains(c.Text, "Deployment") {
		t.Errorf("Read nested = type %q, text %q", c.Type, c.Text)
	}
	if _, err := w.Read("README.md"); err != nil {
		t.Errorf("Read top level: %v", err)
	}
	if _, err := w.ReadText("k8s/base/cleanup.yaml"); err != nil {
		t.Errorf("ReadText: %v", err)
	}
	if _, err := w.ReadLog("logs/app.log", 1<<10); err != nil {
		t.Errorf("ReadLog: %v", err)
	}
	if f, _, err := w.Open("k8s/base/cleanup.yaml"); err != nil {
		t.Errorf("Open: %v", err)
	} else {
		io.Copy(io.Discard, f)
		f.Close()
	}
	if size, _, err := w.Stat("k8s/base/cleanup.yaml"); err != nil || size != 5 {
		t.Errorf("Stat = %d, %v", size, err)
	}
	tree, err := w.ReadTree("k8s", nil)
	if err != nil || len(tree) != 4 {
		t.Errorf("ReadTree = %d files, %v; want 4", len(tree), err)
	}
	// .gitignore files are read the same way, so they still apply.
	files, _, err := w.Files(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Path == "k8s/local.yaml" {
			t.Error("k8s/.gitignore was not applied")
		}
	}

	// A missing file is still reported as missing.
	if _, err := w.Read("k8s/none.yaml"); !errors.Is(err, syscall.ELOOP) && !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Read missing = %v", err)
	}
}

// The fallback by path must not open what os.Root would refuse.
func TestReadVirtualisedRejectsEscapes(t *testing.T) {
	outside := writeTree(t, map[string]string{"secret.yaml": "token: x\n"})
	root := writeTree(t, map[string]string{"a.yaml": "a: 1\n"})
	w := open(t, root)
	if err := os.Symlink(filepath.Join(outside, "secret.yaml"), filepath.Join(root, "link.yaml")); err != nil {
		t.Skipf("symlinks unavailable here: %v", err)
	}
	for _, miss := range []error{syscall.ELOOP, fs.ErrNotExist} {
		virtualise(w, miss)
		if _, err := w.Read("link.yaml"); !errors.Is(err, ErrOutside) {
			t.Errorf("%v: symlink escape = %v, want ErrOutside", miss, err)
		}
	}
}

// Regression: with the sample open, a subfolder created inside it
// (zz-t7/cart.go) appeared in the tree after a refresh, but opening the
// file said "zz-t7\cart.go: not found" until codec was restarted: the
// tree lists by path, reads went through the root handle, and under
// virtualisation the handle doesn't see the new folder.
func TestReadSubfolderCreatedAfterOpen(t *testing.T) {
	tests := []struct {
		name string
		miss error // what the root handle reports for the new folder; nil = plain folder
	}{
		{"plain folder", nil},
		{"virtualised, not found", fs.ErrNotExist},
		{"virtualised, reparse refused", syscall.ELOOP},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := writeTree(t, map[string]string{"k8s/base/deployment.yaml": "a: 1\n"})
			w := open(t, root)
			if _, _, err := w.Files(context.Background()); err != nil {
				t.Fatal(err)
			}
			if tt.miss != nil {
				virtualise(w, tt.miss, "zz-t7", "k8s")
			}

			// A new subfolder, and an existing one removed and written
			// again (as tools that regenerate a folder do).
			if err := os.MkdirAll(filepath.Join(root, "zz-t7"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "zz-t7", "cart.go"), []byte("package cart\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.RemoveAll(filepath.Join(root, "k8s")); err != nil {
				t.Fatal(err)
			}
			writeAt(t, root, "k8s/base/deployment.yaml", "b: 2\n")

			if _, _, err := w.rescan(context.Background(), ""); err != nil {
				t.Fatal(err)
			}
			c, err := w.Read("zz-t7/cart.go")
			if err != nil || c.Text != "package cart\n" {
				t.Errorf("Read new subfolder = %v", err)
			}
			c, err = w.Read("k8s/base/deployment.yaml")
			if err != nil || c.Text != "b: 2\n" {
				t.Errorf("Read rewritten subfolder = %v", err)
			}
		})
	}
}

func writeAt(t *testing.T, root, p, content string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(p))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
