package web

import (
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

// Desktop is what the desktop app (cmd/codec-desktop) adds to the
// server: native window features the page asks for. It is nil under
// `codec serve`, where these routes don't exist and the page doesn't
// offer them.
type Desktop interface {
	// SetTheme matches the window frame (title bar, native menus) to
	// the page's theme.
	SetTheme(dark bool)
	// PickFolder shows the system folder dialog, starting in start
	// when set; "" means cancelled.
	PickFolder(title, start string) (string, error)
	// Settings and SetSettings read and change the desktop-only
	// choices shown in Settings → Desktop.
	Settings() (DesktopSettings, error)
	SetSettings(DesktopSettings) error
}

// DesktopSettings are the desktop app's choices.
type DesktopSettings struct {
	StartAtLogin bool `json:"startAtLogin"`
	KeepInTray   bool `json:"keepInTray"`
}

// AppFile returns a file of the embedded UI build, such as the app
// icons the desktop app shows in the tray and title bar.
func AppFile(name string) ([]byte, error) {
	return fs.ReadFile(distFiles, "dist/app/"+name)
}

// desktopMeta tells the page it runs in the desktop window
// (frontend/src/lib/platform.ts).
const desktopMeta = `<meta name="codec-desktop" content="1">`

// OpenPath opens dir as the workspace and, when file is set ('/'-
// separated, relative to dir), asks open pages to show it with the
// "open-file" event. A file inside the workspace already open stays in
// it. The desktop app calls this when launched again with a path.
func (s *Server) OpenPath(dir, file string) error {
	if file != "" {
		if rel, ok := s.inWorkspace(filepath.Join(dir, filepath.FromSlash(file))); ok {
			s.hub.publish("open-file", map[string]string{"path": rel})
			return nil
		}
	}
	if err := s.OpenWorkspace(dir); err != nil {
		return err
	}
	if file != "" {
		s.hub.publish("open-file", map[string]string{"path": file})
	}
	return nil
}

// inWorkspace returns path relative to the open workspace, '/'-
// separated, when it lies inside it.
func (s *Server) inWorkspace(path string) (string, bool) {
	s.mu.Lock()
	ws := s.ws
	s.mu.Unlock()
	if ws == nil {
		return "", false
	}
	rel, err := filepath.Rel(ws.Root(), path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func (s *Server) desktopRoutes(mux *http.ServeMux) {
	if s.opts.Desktop == nil {
		return
	}
	mux.HandleFunc("POST /api/v2/desktop/theme", s.handleDesktopTheme)
	mux.HandleFunc("POST /api/v2/desktop/pick-folder", s.handlePickFolder)
	mux.HandleFunc("GET /api/v2/desktop/settings", s.handleDesktopSettings)
	mux.HandleFunc("POST /api/v2/desktop/settings", s.handleDesktopSettings)
}

// handleDesktopSettings answers the current choices; a POST with the
// whole object changes them first.
func (s *Server) handleDesktopSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		var req DesktopSettings
		if !decode(w, r, &req) {
			return
		}
		if err := s.opts.Desktop.SetSettings(req); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	st, err := s.opts.Desktop.Settings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// handlePickFolder answers once the dialog closes, which can take as
// long as the user likes (the server has no write timeout).
func (s *Server) handlePickFolder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
		Start string `json:"start"`
	}
	if !decode(w, r, &req) {
		return
	}
	path, err := s.opts.Desktop.PickFolder(req.Title, req.Start)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "folder dialog: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": path})
}

func (s *Server) handleDesktopTheme(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Theme string `json:"theme"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Theme != "dark" && req.Theme != "light" {
		writeError(w, http.StatusBadRequest, `theme must be "dark" or "light"`)
		return
	}
	s.opts.Desktop.SetTheme(req.Theme == "dark")
	w.WriteHeader(http.StatusNoContent)
}
