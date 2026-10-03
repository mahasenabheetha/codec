package web

import (
	"errors"
	"net/http"
	"time"

	"github.com/mahasenabheetha/codec/v2/internal/cron"
	"github.com/mahasenabheetha/codec/v2/internal/timeutil"
)

const maxRuns = 50

type timeRequest struct {
	Input string     `json:"input"` // timestamp: an epoch number or a date
	Expr  string     `json:"expr"`  // cron
	Zone  string     `json:"zone"`  // "" or "UTC", "Local", or a zone name
	Count int        `json:"count"` // cron: next runs, 10 by default
	Form  *cron.Form `json:"form"`  // build
}

type cronField struct {
	cron.Field
	Meaning string `json:"meaning"`
}

// POST /api/v2/time/{kind}: timestamp, cron, build. Nothing is stored.
func (s *Server) handleTime(w http.ResponseWriter, r *http.Request) {
	var req timeRequest
	if !decode(w, r, &req) {
		return
	}
	now := time.Now()
	switch kind := r.PathValue("kind"); kind {
	case "timestamp":
		loc, err := timeutil.Zone(req.Zone)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		st, err := timeutil.Parse(req.Input, loc, now)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, st)
	case "cron":
		sched, err := cron.Parse(req.Expr)
		if err != nil {
			var ce *cron.Error
			if errors.As(err, &ce) {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"error": ce.Msg, "dialect": ce.Dialect, "field": ce.Field})
			} else {
				writeError(w, http.StatusUnprocessableEntity, err.Error())
			}
			return
		}
		// A CRON_TZ= prefix in the expression wins over the chosen zone.
		zone := req.Zone
		if sched.Zone != "" {
			zone = sched.Zone
		}
		loc, err := timeutil.Zone(zone)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		n := req.Count
		if n == 0 {
			n = 10
		}
		runs, skipped := sched.Next(now, min(max(n, 1), maxRuns), loc)
		if runs == nil {
			runs = []time.Time{}
		}
		if skipped == nil {
			skipped = []cron.Skipped{}
		}
		fields := make([]cronField, len(sched.Fields))
		for i, f := range sched.Fields {
			fields[i] = cronField{f, f.Meaning()}
		}
		resp := map[string]any{
			"description": sched.Describe(),
			"standard":    sched.Standard,
			"fields":      fields,
			"zone":        loc.String(),
			"runs":        runs,
			"skipped":     skipped,
		}
		if f, ok := cron.FormOf(sched); ok {
			resp["form"] = f
		}
		writeJSON(w, http.StatusOK, resp)
	case "build":
		if req.Form == nil {
			writeError(w, http.StatusBadRequest, "form is missing")
			return
		}
		expr, err := cron.Build(*req.Form)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"expr": expr})
	default:
		writeError(w, http.StatusBadRequest, "unknown job "+kind)
	}
}
