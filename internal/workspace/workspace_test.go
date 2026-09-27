package workspace

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// writeTree creates files (slash paths → content) under a temp dir.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for p, content := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// snapshot records every path, size and mtime under root, to prove
// that nothing was written.
func snapshot(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// NTFS updates directory mtimes lazily, so only names count.
			fmt.Fprintln(&b, p)
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		fmt.Fprintln(&b, p, info.ModTime(), info.Size())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func open(t *testing.T, root string) *Workspace {
	t.Helper()
	w, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { w.Close() })
	return w
}

func TestFilesAndClassification(t *testing.T) {
	root := writeTree(t, map[string]string{
		".gitignore":                  "build/\n*.log\n!keep.log\n",
		"charts/app/Chart.yaml":       "apiVersion: v2\nname: app\nversion: 1.0.0\n",
		"charts/app/.gitignore":       "local-*.yaml\n",
		"charts/app/local-dev.yaml":   "a: 1\n",
		"deploy.yaml":                 "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\n",
		"notes.md":                    "# hi\n",
		"build/out.yaml":              "a: 1\n",
		"debug.log":                   "x",
		"keep.log":                    "x",
		"logo.png":                    "\x89PNG",
		".git/config":                 "[core]\n",
		"node_modules/x/package.json": "{}",
	})
	big := filepath.Join(root, "big.yaml")
	if err := os.WriteFile(big, make([]byte, MaxFileSize+1), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)

	w := open(t, root)
	if _, _, err := w.Files(context.Background()); err != nil {
		t.Fatal(err)
	}
	w.Classify(context.Background())
	files, truncated, err := w.Files(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	types := map[string]string{}
	for _, f := range files {
		got = append(got, f.Path)
		types[f.Path] = f.Type
	}
	want := []string{".gitignore", "charts/app/.gitignore", "charts/app/Chart.yaml", "deploy.yaml", "keep.log", "notes.md"}
	if !slices.Equal(got, want) || truncated {
		t.Errorf("files = %v (truncated %v), want %v", got, truncated, want)
	}
	if types["charts/app/Chart.yaml"] != "helm-chart" || types["deploy.yaml"] != "kubernetes" || types["notes.md"] != "" {
		t.Errorf("types = %v", types)
	}

	c, err := w.Read("deploy.yaml")
	if err != nil || c.Type != "kubernetes" || !strings.HasPrefix(c.Text, "apiVersion") {
		t.Errorf("Read = %+v, %v", c, err)
	}
	if _, err := w.Read("big.yaml"); !errors.Is(err, ErrTooLarge) {
		t.Errorf("big file: %v, want ErrTooLarge", err)
	}
	if after := snapshot(t, root); after != before {
		t.Errorf("the workspace was modified:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestReadBOMAndCRLF(t *testing.T) {
	root := writeTree(t, map[string]string{"a.yaml": "\xEF\xBB\xBFa: 1\r\nb: 2\r\n", "bin.txt": "ab\x00cd"})
	w := open(t, root)
	c, err := w.Read("a.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !c.BOM || !c.CRLF || !strings.HasPrefix(c.Text, "a: 1") {
		t.Errorf("got bom=%v crlf=%v text=%q", c.BOM, c.CRLF, c.Text)
	}
	if _, err := w.Read("bin.txt"); !errors.Is(err, ErrBinary) {
		t.Errorf("binary: %v", err)
	}
}

func TestReadRejectsEscapes(t *testing.T) {
	outside := writeTree(t, map[string]string{"secret.yaml": "token: x\n"})
	root := writeTree(t, map[string]string{"a.yaml": "a: 1\n"})
	w := open(t, root)

	for _, p := range []string{"../secret.yaml", "a/../../secret.yaml", "/etc/passwd", `..\secret.yaml`, "C:/x", ""} {
		if _, err := w.Read(p); !errors.Is(err, ErrOutside) {
			t.Errorf("Read(%q) = %v, want ErrOutside", p, err)
		}
	}

	link := filepath.Join(root, "link.yaml")
	if err := os.Symlink(filepath.Join(outside, "secret.yaml"), link); err != nil {
		t.Skipf("symlinks unavailable here: %v", err)
	}
	if _, err := w.Read("link.yaml"); !errors.Is(err, ErrOutside) {
		t.Errorf("symlink escape: %v, want ErrOutside", err)
	}
	files, _, _ := w.Files(context.Background())
	for _, f := range files {
		if f.Path == "link.yaml" {
			t.Error("a link leaving the workspace must not be listed")
		}
	}
}

func TestRescanReportsChanges(t *testing.T) {
	root := writeTree(t, map[string]string{"a.yaml": "a: 1\n", "b.yaml": "b: 1\n", "dir/c.yaml": "c: 1\n"})
	w := open(t, root)
	ctx := context.Background()
	if _, _, err := w.Files(ctx); err != nil {
		t.Fatal(err)
	}

	later := time.Now().Add(time.Minute)
	os.WriteFile(filepath.Join(root, "a.yaml"), []byte("a: 22\n"), 0o644)
	os.Chtimes(filepath.Join(root, "a.yaml"), later, later)
	os.Remove(filepath.Join(root, "b.yaml"))
	os.RemoveAll(filepath.Join(root, "dir"))
	os.WriteFile(filepath.Join(root, "new.yaml"), []byte("n: 1\n"), 0o644)

	changes, _, err := w.rescan(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	want := []Change{{Changed, "a.yaml"}, {Removed, "b.yaml"}, {Removed, "dir/c.yaml"}, {Added, "new.yaml"}}
	if !slices.Equal(changes, want) {
		t.Errorf("changes = %v, want %v", changes, want)
	}
}

func TestWatch(t *testing.T) {
	debounce, pollInterval = 20*time.Millisecond, 30*time.Millisecond
	for _, poll := range []bool{false, true} {
		t.Run(map[bool]string{false: "native", true: "poll"}[poll], func(t *testing.T) {
			root := writeTree(t, map[string]string{"a.yaml": "a: 1\n"})
			w := open(t, root)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			got := make(chan []Change, 10)
			ready := make(chan string, 1)
			if err := w.Watch(ctx, poll, func(c []Change) { got <- c }, func(m string) { ready <- m }); err != nil {
				t.Fatal(err)
			}
			// Change files only once watching has started, so this tests
			// the watcher rather than the catch-up rescan.
			if m := <-ready; poll && m != WatchPoll {
				t.Fatalf("mode = %s, want poll", m)
			}
			os.MkdirAll(filepath.Join(root, "sub"), 0o755)
			os.WriteFile(filepath.Join(root, "sub", "new.yaml"), []byte("n: 1\n"), 0o644)
			deadline := time.After(5 * time.Second)
			for {
				select {
				case c := <-got:
					if slices.Contains(c, Change{Added, "sub/new.yaml"}) {
						return
					}
				case <-deadline:
					t.Fatal("no change reported for sub/new.yaml")
				}
			}
		})
	}
}

func TestReadTree(t *testing.T) {
	root := writeTree(t, map[string]string{"chart/Chart.yaml": "name: a\n", "chart/templates/a.yaml": "x: 1\n", "chart/tmp/skip.txt": "no", "other.yaml": "o: 1\n"})
	w := open(t, root)
	files, err := w.ReadTree("chart", func(rel string, isDir bool) bool { return rel == "tmp" })
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range files {
		got = append(got, f.Path)
	}
	if strings.Join(got, ",") != "Chart.yaml,templates/a.yaml" {
		t.Errorf("ReadTree = %v", got)
	}
	if _, err := w.ReadTree("../x", nil); !errors.Is(err, ErrOutside) {
		t.Errorf("escape: %v", err)
	}
}
