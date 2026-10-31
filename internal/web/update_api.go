package web

import (
	"net/http"

	"github.com/mahasenabheetha/codec/v2/internal/config"
	"github.com/mahasenabheetha/codec/v2/internal/update"
	"github.com/mahasenabheetha/codec/v2/internal/version"
)

// updateView is the update check as the page sees it: whether the
// daily check is on, its answer, and what went wrong, if anything (a
// failed check is not an error for the page, just nothing to show).
type updateView struct {
	update.Result
	Enabled bool   `json:"enabled"`
	Error   string `json:"error,omitempty"`
}

// handleUpdate answers whether a newer codec is out (decision 85). With
// the daily check off it asks nothing, unless ?force=1 ("Check now"),
// which also skips the day's cached answer.
func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	force := r.URL.Query().Get("force") == "1"
	current := version.Get().Version
	v := updateView{Enabled: s.opts.Config.Get().Updates.Check}
	if !v.Enabled && !force {
		v.Result = update.Result{Current: current, Skipped: "off"}
		writeJSON(w, http.StatusOK, v)
		return
	}
	res, err := s.opts.Updates.Check(r.Context(), current, force)
	v.Result = res
	if err != nil {
		v.Error = err.Error()
	}
	writeJSON(w, http.StatusOK, v)
}

// NewerRelease is the newer release the day's check found, or nil: with
// the check off, before it ran, or when codec is up to date. It never
// asks GitHub itself; the desktop tray menu shows it.
func (s *Server) NewerRelease() *update.Release {
	if !s.opts.Config.Get().Updates.Check {
		return nil
	}
	if r, ok := s.opts.Updates.Cached(version.Get().Version); ok && r.Newer {
		return r.Latest
	}
	return nil
}

// handleUpdateSettings switches the daily check on or off.
func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Check bool `json:"check"`
	}
	if !decode(w, r, &req) {
		return
	}
	if err := s.opts.Config.Update(func(st *config.Settings) { st.Updates.Check = req.Check }); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"check": req.Check})
}
