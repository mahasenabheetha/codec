package web

import (
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/config"
	"github.com/mahasenabheetha/codec/v2/internal/schemacache"
)

// Settings endpoints for what the other features don't already cover:
// where codec keeps its files, and saved state to clean up.

type settingsFolder struct {
	ID     string `json:"id"`
	Path   string `json:"path"` // "" = none on this system
	Exists bool   `json:"exists"`
}

type helmProfiles struct {
	Chart    string                        `json:"chart"` // absolute chart folder
	Exists   bool                          `json:"exists"`
	Profiles map[string]config.HelmProfile `json:"profiles"`
	Active   string                        `json:"active,omitempty"`
}

type settingsView struct {
	Folders []settingsFolder `json:"folders"`
	Helm    []helmProfiles   `json:"helm"`
	Recent  []string         `json:"recent"`
}

func (s *Server) settingsView() settingsView {
	st := s.opts.Config.Get()
	templates, _ := config.TemplatesDir()
	v := settingsView{Folders: []settingsFolder{}, Helm: []helmProfiles{}, Recent: st.RecentFolders}
	for _, f := range []settingsFolder{
		{ID: "settings", Path: s.opts.Config.Path()},
		{ID: "templates", Path: templates},
		{ID: "schemas", Path: schemacache.DefaultDir()},
		{ID: "sample", Path: s.sampleDir()},
	} {
		if f.Path != "" {
			_, err := os.Stat(f.Path)
			f.Exists = err == nil
		}
		v.Folders = append(v.Folders, f)
	}
	for chart, h := range st.Helm {
		_, err := os.Stat(chart)
		v.Helm = append(v.Helm, helmProfiles{Chart: chart, Exists: err == nil, Profiles: h.Profiles, Active: h.Active})
	}
	slices.SortFunc(v.Helm, func(a, b helmProfiles) int { return strings.Compare(a.Chart, b.Chart) })
	if v.Recent == nil {
		v.Recent = []string{}
	}
	return v
}

// GET /api/v2/settings
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.settingsView())
}

// POST /api/v2/settings/forget {helm?: {chart, profile}, recent?: bool}:
// removes a saved Helm profile ("" profile = every profile of the
// chart, e.g. one that no longer exists) or the recent folders list.
func (s *Server) handleSettingsForget(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Helm *struct {
			Chart   string `json:"chart"`
			Profile string `json:"profile"`
		} `json:"helm"`
		Recent bool `json:"recent"`
	}
	if !decode(w, r, &req) {
		return
	}
	err := s.opts.Config.Update(func(st *config.Settings) {
		if req.Recent {
			st.RecentFolders = nil
		}
		if h := req.Helm; h != nil {
			c, ok := st.Helm[h.Chart]
			switch {
			case !ok:
			case h.Profile == "":
				delete(st.Helm, h.Chart)
			default:
				delete(c.Profiles, h.Profile)
				if c.Active == h.Profile {
					c.Active = ""
				}
				if len(c.Profiles) == 0 {
					delete(st.Helm, h.Chart)
				} else {
					st.Helm[h.Chart] = c
				}
			}
		}
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.settingsView())
}
