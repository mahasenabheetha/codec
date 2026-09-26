package web

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/config"
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

	if err := s.opts.Config.Update(func(st *config.Settings) {
		st.RecentFolders = config.AddRecent(st.RecentFolders, root)
	}); err != nil {
		log.Printf("remember recent folder: %v", err)
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
