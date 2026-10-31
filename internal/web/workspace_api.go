package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/config"
	"github.com/mahasenabheetha/codec/v2/internal/sample"
	"github.com/mahasenabheetha/codec/v2/internal/workspace"
)

// workspaceView describes the open folder (if any) and recent folders.
type workspaceView struct {
	Open   bool     `json:"open"`
	Root   string   `json:"root,omitempty"`
	Name   string   `json:"name,omitempty"`
	Watch  string   `json:"watch,omitempty"` // native | poll
	Recent []string `json:"recent"`
	Sep    string   `json:"sep"` // OS path separator, for display
	// Sample is true while the sample workspace is open.
	Sample bool `json:"sample,omitempty"`
}

// filesEvent is the payload of the "files" event.
type filesEvent struct {
	Root    string             `json:"root"`
	Changes []workspace.Change `json:"changes"`
}

// OpenWorkspace opens dir, starts watching it and makes it the current
// workspace, closing the previous one. Clients are told via the
// "workspace" event.
func (s *Server) OpenWorkspace(dir string) error {
	ws, err := workspace.Open(dir)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	root := ws.Root()

	// Make it current first, so the watcher's mode callback (which may
	// fire any moment after Watch returns) finds it.
	s.mu.Lock()
	oldWS, oldStop := s.ws, s.stop
	s.ws, s.stop, s.watch = ws, cancel, ""
	s.mu.Unlock()
	if oldStop != nil {
		oldStop()
		oldWS.Close()
	}

	err = ws.Watch(ctx, s.opts.Poll,
		func(changes []workspace.Change) {
			s.hub.publish("files", filesEvent{Root: root, Changes: changes})
		},
		func(mode string) {
			s.mu.Lock()
			if s.ws == ws {
				s.watch = mode
			}
			s.mu.Unlock()
			s.hub.publish("workspace", s.view())
		})
	if err != nil {
		s.mu.Lock()
		if s.ws == ws {
			s.ws, s.stop, s.watch = nil, nil, ""
		}
		s.mu.Unlock()
		cancel()
		ws.Close()
		return err
	}

	// The sample has its own button; it would only crowd real folders out.
	if !s.isSample(root) {
		if err := s.opts.Config.Update(func(st *config.Settings) {
			st.RecentFolders = config.AddRecent(st.RecentFolders, root)
		}); err != nil {
			log.Printf("remember recent folder: %v", err)
		}
	}
	s.hub.publish("workspace", s.view())

	// The tree is usable now; file types follow once every YAML file is
	// parsed, and clients refetch the tree on this event.
	go func() {
		ws.Classify(ctx)
		if ctx.Err() == nil {
			s.hub.publish("tree", map[string]string{"root": root})
		}
	}()
	return nil
}

// current returns the open workspace, or nil.
func (s *Server) current() *workspace.Workspace {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ws
}

func (s *Server) view() workspaceView {
	v := workspaceView{Recent: s.opts.Config.Get().RecentFolders, Sep: string(filepath.Separator)}
	if v.Recent == nil {
		v.Recent = []string{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ws != nil {
		v.Open, v.Root, v.Name, v.Watch = true, s.ws.Root(), s.ws.Name(), s.watch
		v.Sample = s.isSample(v.Root)
	}
	return v
}

// GET /api/v2/workspace
func (s *Server) handleWorkspace(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.view())
}

// POST /api/v2/workspace/open {"path": "..."}
func (s *Server) handleOpen(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	dir := cleanInputPath(req.Path)
	if dir == "" {
		writeError(w, http.StatusUnprocessableEntity, "enter a folder path")
		return
	}
	if err := s.OpenWorkspace(dir); err != nil {
		switch {
		case errors.Is(err, fs.ErrNotExist):
			// A recent folder that was moved or deleted: forget it.
			s.opts.Config.Update(func(st *config.Settings) {
				st.RecentFolders = config.RemoveRecent(st.RecentFolders, dir)
			})
			writeError(w, http.StatusNotFound, "folder not found: "+dir)
		case errors.Is(err, workspace.ErrNotDir):
			writeError(w, http.StatusUnprocessableEntity, "not a folder: "+dir)
		case errors.Is(err, fs.ErrPermission):
			writeError(w, http.StatusUnprocessableEntity, "permission denied: "+dir)
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, s.view())
}

// cleanInputPath tidies a pasted path: surrounding quotes (Windows
// "Copy as path" adds them), whitespace and a leading ~ for home.
func cleanInputPath(p string) string {
	p = strings.Trim(strings.TrimSpace(p), `"'`)
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, p[1:])
		}
	}
	return p
}

// dirsView is a folder-browser listing plus places to jump to.
type dirsView struct {
	*workspace.Listing
	Home  string   `json:"home,omitempty"`
	Roots []string `json:"roots,omitempty"` // only when no path was given
	Sep   string   `json:"sep"`
}

// GET /api/v2/fs/dirs?path=  (no path: the home folder, plus drives)
func (s *Server) handleDirs(w http.ResponseWriter, r *http.Request) {
	home, _ := os.UserHomeDir()
	dir := cleanInputPath(r.URL.Query().Get("path"))
	v := dirsView{Home: home, Sep: string(filepath.Separator)}
	if dir == "" {
		dir = home
		v.Roots = workspace.Roots() // probing drives can be slow; only on first open
	}
	l, err := workspace.ListDirs(dir)
	if err != nil {
		status := http.StatusUnprocessableEntity
		if errors.Is(err, fs.ErrNotExist) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}
	v.Listing = l
	writeJSON(w, http.StatusOK, v)
}

// treeView is the file list of the open workspace.
type treeView struct {
	Root      string           `json:"root"`
	Files     []workspace.File `json:"files"`
	Truncated bool             `json:"truncated,omitempty"`
}

// GET /api/v2/files/tree
func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	ws := s.current()
	if ws == nil {
		writeError(w, http.StatusConflict, "no folder is open")
		return
	}
	files, truncated, err := ws.Files(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, treeView{Root: ws.Root(), Files: files, Truncated: truncated})
}

// contentView is one file's text and what codec knows about it.
type contentView struct {
	workspace.File
	Root    string `json:"root"`
	Text    string `json:"text"`
	ModTime int64  `json:"modTime"` // Unix milliseconds
	BOM     bool   `json:"bom,omitempty"`
	CRLF    bool   `json:"crlf,omitempty"`
}

// GET /api/v2/files/content?path=
func (s *Server) handleContent(w http.ResponseWriter, r *http.Request) {
	ws := s.current()
	if ws == nil {
		writeError(w, http.StatusConflict, "no folder is open")
		return
	}
	p := r.URL.Query().Get("path")
	c, err := ws.Read(p)
	if err != nil {
		he := readError(err)
		writeError(w, he.status, he.msg)
		return
	}
	writeJSON(w, http.StatusOK, contentView{
		File:    c.File,
		Root:    ws.Root(),
		Text:    c.Text,
		ModTime: c.ModTime.UnixMilli(),
		BOM:     c.BOM,
		CRLF:    c.CRLF,
	})
}

// GET /api/v2/files/stat?path=: a file's size and change time, at any
// size; the Log viewer polls it to follow a log as it grows.
func (s *Server) handleStat(w http.ResponseWriter, r *http.Request) {
	ws := s.current()
	if ws == nil {
		writeError(w, http.StatusConflict, "no folder is open")
		return
	}
	size, mod, err := ws.Stat(r.URL.Query().Get("path"))
	if err != nil {
		he := readError(err)
		writeError(w, he.status, he.msg)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		Size    int64 `json:"size"`
		ModTime int64 `json:"modTime"` // Unix milliseconds
	}{size, mod.UnixMilli()})
}

// maxImage caps an image the Markdown preview shows.
const maxImage = 10 << 20

// imageTypes are the images the preview may show, by extension; the
// bytes must agree (SVG is text, so it is checked for an <svg> element).
var imageTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
}

// GET /api/v2/files/image?path=: an image inside the open folder, for
// the Markdown preview (decision 84). Images only, through the root like
// every read; the UI fetches it with the token and shows it as a blob.
func (s *Server) handleImage(w http.ResponseWriter, r *http.Request) {
	ws := s.current()
	if ws == nil {
		writeError(w, http.StatusConflict, "no folder is open")
		return
	}
	p := r.URL.Query().Get("path")
	typ, ok := imageTypes[strings.ToLower(path.Ext(p))]
	if !ok {
		writeError(w, http.StatusUnsupportedMediaType, "not an image")
		return
	}
	data, err := ws.ReadLog(p, maxImage) // through the root, a size cap, binary allowed
	if errors.Is(err, workspace.ErrOverLimit) {
		writeError(w, http.StatusRequestEntityTooLarge, p+" is over 10 MB")
		return
	}
	if err != nil {
		he := readError(err)
		writeError(w, he.status, he.msg)
		return
	}
	if !isImage(typ, data) {
		writeError(w, http.StatusUnsupportedMediaType, p+" is not a "+strings.TrimPrefix(typ, "image/")+" image")
		return
	}
	h := w.Header()
	h.Set("Content-Type", typ)
	h.Set("Cache-Control", "no-store")
	// Opened on its own, an SVG must not run anything.
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	w.Write(data)
}

// isImage reports whether data really is an image of type typ.
func isImage(typ string, data []byte) bool {
	if typ == "image/svg+xml" {
		head := bytes.ToLower(data[:min(len(data), 4096)])
		return bytes.Contains(head, []byte("<svg"))
	}
	got := http.DetectContentType(data)
	return got == typ
}

// OpenSample writes the sample workspace to codec's cache folder (never
// a repository) and opens it like any folder.
func (s *Server) OpenSample() error {
	dir := s.sampleDir()
	if err := sample.Extract(dir); err != nil {
		return fmt.Errorf("write the sample workspace: %w", err)
	}
	return s.OpenWorkspace(dir)
}

func (s *Server) sampleDir() string {
	if s.opts.SampleDir != "" {
		return s.opts.SampleDir
	}
	return sample.Dir()
}

func (s *Server) isSample(root string) bool {
	return filepath.Clean(root) == filepath.Clean(s.sampleDir())
}

// POST /api/v2/workspace/sample: open the sample workspace.
func (s *Server) handleOpenSample(w http.ResponseWriter, r *http.Request) {
	if err := s.OpenSample(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.view())
}
