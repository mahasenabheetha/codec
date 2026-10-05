// Package workspace gives read-only access to one folder on disk: a
// file tree that honours .gitignore, file content, file-type
// classification and change notifications.
//
// It is an adapter: it does the I/O and hands content to the pure
// engine packages (yamlkit, provider). It never writes. The folder is
// opened through os.Root, which refuses any path that would leave it,
// including through symlinks.
package workspace

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

const (
	// MaxFileSize is the largest file listed or read. Bigger files are
	// almost never hand-written YAML and would make the UI sluggish.
	MaxFileSize = 8 << 20
	// MaxFiles caps the tree, so opening a home folder or a drive by
	// mistake can't exhaust memory.
	MaxFiles = 50_000
)

// Errors callers branch on (the web layer maps them to status codes).
var (
	ErrOutside  = errors.New("path is outside the workspace")
	ErrNotDir   = errors.New("not a folder")
	ErrNotFile  = errors.New("not a file")
	ErrTooLarge = errors.New("file is larger than 8 MB")
	ErrBinary   = errors.New("binary file")
)

// skipDirs are never listed, whatever .gitignore says.
var skipDirs = map[string]bool{".git": true, "node_modules": true}

// File is one entry of the tree.
type File struct {
	Path    string    `json:"path"` // slash-separated, relative to the root
	Size    int64     `json:"size"`
	ModTime time.Time `json:"-"`
	Lang    string    `json:"lang"`           // yaml, json, markdown, …, text
	Type    string    `json:"type,omitempty"` // provider id, YAML files only
}

// Content is a file read from disk. Text has any UTF-8 BOM removed, so
// yamlkit positions (decision #20) line up with it; BOM and CRLF record
// what the file really looks like.
type Content struct {
	File
	Text string
	BOM  bool
	CRLF bool
	// YAML is the parse of Text for YAML files (cached; treat as read-only).
	YAML *yamlkit.File
}

// entry is a File plus its lazily computed classification. Parse
// results are cached here and dropped whenever mtime or size change.
type entry struct {
	File
	classified bool
	parsed     *yamlkit.File
}

// Workspace is one opened folder.
type Workspace struct {
	root string   // absolute path as opened (display, watching)
	real string   // root with symlinks resolved (escape checks)
	dir  *os.Root // every read goes through this
	fsys fs.FS    // dir as an fs.FS, for fs.WalkDir
	fold bool     // case-insensitive filesystem conventions

	scanMu     sync.Mutex // one rescan at a time
	classifyMu sync.Mutex // one Classify at a time (no duplicate parsing)

	mu        sync.RWMutex // guards the fields below
	files     map[string]*entry
	ignores   map[string]*ignoreFile // by directory; only dirs with a .gitignore
	truncated bool
}

// Open opens dir read-only. Nothing is scanned until Files or Watch.
func Open(dir string) (*Workspace, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s: %w", abs, ErrNotDir)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		real = abs
	}
	root, err := os.OpenRoot(abs)
	if err != nil {
		return nil, err
	}
	return &Workspace{
		root:    abs,
		real:    real,
		dir:     root,
		fsys:    root.FS(),
		fold:    runtime.GOOS == "windows" || runtime.GOOS == "darwin",
		ignores: map[string]*ignoreFile{},
	}, nil
}

// Root is the absolute path of the folder.
func (w *Workspace) Root() string { return w.root }

// RealRoot is Root with symlinks (and junctions) resolved.
func (w *Workspace) RealRoot() string { return w.real }

// Name is the folder's base name, e.g. "codec".
func (w *Workspace) Name() string { return filepath.Base(w.root) }

// Close releases the folder handle. Stop any Watch first.
func (w *Workspace) Close() error { return w.dir.Close() }

// Files returns the tree, sorted by path. The first call walks the
// folder; later calls reuse the index the watcher keeps current. YAML
// files not classified yet (see Classify) have an empty Type.
// truncated reports that the folder holds more than MaxFiles files.
func (w *Workspace) Files(ctx context.Context) (files []File, truncated bool, err error) {
	if err := w.ensureScanned(ctx); err != nil {
		return nil, false, err
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	files = make([]File, 0, len(w.files))
	for _, e := range w.files {
		files = append(files, e.File)
	}
	slices.SortFunc(files, func(a, b File) int { return strings.Compare(a.Path, b.Path) })
	return files, w.truncated, nil
}

func (w *Workspace) ensureScanned(ctx context.Context) error {
	w.mu.RLock()
	scanned := w.files != nil
	w.mu.RUnlock()
	if scanned {
		return nil
	}
	_, _, err := w.rescan(ctx, "")
	return err
}

// Read returns a file's content. Any file inside the root can be read,
// even one the tree hides (e.g. gitignored), but nothing outside it.
func (w *Workspace) Read(p string) (*Content, error) {
	if err := validPath(p); err != nil {
		return nil, err
	}
	var data []byte
	var info fs.FileInfo
	// OneDrive and antivirus scanners briefly lock files that just
	// changed; retry a transient failure before reporting it.
	err := retry(func() (err error) {
		data, info, err = w.readFile(p)
		return w.wrapErr(p, err)
	})
	if err != nil {
		return nil, err
	}

	c := &Content{File: File{
		Path:    p,
		Size:    info.Size(),
		ModTime: info.ModTime(),
		Lang:    langOf(path.Base(p)),
	}}
	if rest, ok := bytes.CutPrefix(data, bom); ok {
		data, c.BOM = rest, true
	}
	c.CRLF = bytes.Contains(data, []byte("\r\n"))
	c.Text = string(data)
	if c.Lang == "yaml" {
		c.YAML, c.Type = w.parse(c.File, data)
	}
	return c, nil
}

var bom = []byte{0xEF, 0xBB, 0xBF}

// ReadText returns a file's bytes without a BOM and without parsing
// them, for full-text search. Like Read it opens through the root, so a
// symlink can't lead outside the folder.
func (w *Workspace) ReadText(p string) ([]byte, error) {
	if err := validPath(p); err != nil {
		return nil, err
	}
	var data []byte
	err := retry(func() (err error) { // see Read
		data, _, err = w.readFile(p)
		return w.wrapErr(p, err)
	})
	if err != nil {
		return nil, err
	}
	return bytes.TrimPrefix(data, bom), nil
}

// readFile reads p through the root, refusing directories, big files
// and binaries. Every path a client asks for is read this way.
func (w *Workspace) readFile(p string) ([]byte, fs.FileInfo, error) {
	f, err := w.dir.Open(filepath.FromSlash(p))
	if err != nil {
		return nil, nil, err
	}
	return readAll(f)
}

// readListed reads a file the walk itself found, by its real path.
// Background classification uses it because os.Root opens serialise
// on Windows (1 s vs 0.3 s for 2.3k files on 22 cores). The paths never
// come from a client, the walk doesn't follow symlinked directories,
// and only the resulting file type is ever exposed.
func (w *Workspace) readListed(p string) ([]byte, error) {
	f, err := os.Open(w.abs(p))
	if err != nil {
		return nil, err
	}
	data, _, err := readAll(f)
	return data, err
}

// readAll reads and closes f with the size and binary checks.
func readAll(f *os.File) ([]byte, fs.FileInfo, error) {
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	if info.IsDir() {
		return nil, nil, ErrNotFile
	}
	if info.Size() > MaxFileSize {
		return nil, nil, ErrTooLarge
	}
	// The limit also covers files that grow between Stat and Read.
	data, err := io.ReadAll(io.LimitReader(f, MaxFileSize+1))
	if err != nil {
		return nil, nil, err
	}
	if len(data) > MaxFileSize {
		return nil, nil, ErrTooLarge
	}
	// A NUL byte early on is the classic "this is binary" signal.
	if bytes.IndexByte(data[:min(len(data), 8000)], 0) >= 0 {
		return nil, nil, ErrBinary
	}
	return data, info, nil
}

// parse returns the parse tree and provider id of a YAML file the UI
// opened, from the cache (keyed by mtime+size) when it is current.
func (w *Workspace) parse(f File, data []byte) (*yamlkit.File, string) {
	w.mu.RLock()
	e := w.files[f.Path]
	if e != nil && e.parsed != nil && e.ModTime.Equal(f.ModTime) && e.Size == f.Size {
		parsed, typ := e.parsed, e.Type
		w.mu.RUnlock()
		return parsed, typ
	}
	w.mu.RUnlock()
	parsed, typ := classify(f.Path, data)
	w.store(f, parsed, typ)
	return parsed, typ
}

// Classify parses and classifies every YAML file not classified yet,
// on one goroutine per CPU: parsing is CPU-bound and files are
// independent. Opening a folder runs it in the background (the tree
// appears first, file types a moment later); the watcher runs it for
// changed files before reporting them.
func (w *Workspace) Classify(ctx context.Context) {
	w.classifyMu.Lock()
	defer w.classifyMu.Unlock()
	w.mu.RLock()
	var todo []File
	for _, e := range w.files {
		if !e.classified {
			todo = append(todo, e.File)
		}
	}
	w.mu.RUnlock()
	if len(todo) == 0 {
		return
	}

	jobs := make(chan File)
	var wg sync.WaitGroup
	for range min(runtime.NumCPU(), len(todo)) {
		wg.Go(func() {
			for f := range jobs {
				data, err := w.readListed(f.Path)
				if err != nil {
					// Unreadable right now: show it as plain YAML rather
					// than retrying forever; the next change reclassifies.
					w.store(f, nil, "yaml")
					continue
				}
				// Keep only the type: holding every parse tree would cost
				// ~70 MB on a 5k-file repo. Files opened in the UI are
				// parsed again and cached (typeOf).
				_, typ := classify(f.Path, data)
				w.store(f, nil, typ)
			}
		})
	}
feed:
	for _, f := range todo {
		select {
		case jobs <- f:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()
}

// store records a classification, unless the file changed meanwhile
// (then the newer entry is left for the next pass).
func (w *Workspace) store(f File, parsed *yamlkit.File, typ string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if e := w.files[f.Path]; e != nil && e.ModTime.Equal(f.ModTime) && e.Size == f.Size {
		e.Type, e.parsed, e.classified = typ, parsed, true
	}
}

// classify parses YAML and asks the provider registry what it is. A
// parser panic on some odd file must not take the server down, so it
// is recovered and the file shown as plain YAML.
func classify(p string, data []byte) (parsed *yamlkit.File, typ string) {
	defer func() {
		if r := recover(); r != nil {
			parsed, typ = nil, "yaml"
		}
	}()
	parsed = yamlkit.Parse(data)
	if best := provider.Default.Best(&provider.File{Path: p, YAML: parsed}); best != nil {
		return parsed, best.ID()
	}
	return parsed, "yaml"
}

// wrapErr turns a failed read into ErrOutside when the path resolves
// outside the root. os.Root's own "escapes" error isn't exported, so
// check with EvalSymlinks.
func (w *Workspace) wrapErr(p string, err error) error {
	if err == nil || definite(err) {
		return err
	}
	real, e := filepath.EvalSymlinks(filepath.Join(w.root, filepath.FromSlash(p)))
	if e == nil && !within(w.real, real) {
		return ErrOutside
	}
	return err
}

// validPath accepts slash-separated relative paths that stay inside
// the root lexically; os.Root then guards against symlink escapes.
func validPath(p string) error {
	if p == "" || !fs.ValidPath(p) || strings.ContainsAny(p, `\:`) {
		return ErrOutside
	}
	return nil
}

func within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func under(p, prefix string) bool {
	return prefix == "" || p == prefix || strings.HasPrefix(p, prefix+"/")
}

func underAny(p string, dirs []string) bool {
	for _, d := range dirs {
		if under(p, d) {
			return true
		}
	}
	return false
}

func parentDir(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return ""
}

// retry runs fn up to three times while it fails without a definite
// answer, e.g. a sharing violation from a sync client.
func retry(fn func() error) error {
	var err error
	for i := range 3 {
		if err = fn(); err == nil || definite(err) {
			return err
		}
		time.Sleep(time.Duration(i+1) * 40 * time.Millisecond)
	}
	return err
}

// definite errors won't change on a retry.
func definite(err error) bool {
	for _, target := range []error{fs.ErrNotExist, ErrOutside, ErrNotFile, ErrTooLarge, ErrBinary} {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}
