package web

import (
	"net/http"

	"github.com/mahasenabheetha/codec/v2/internal/encode"
	"github.com/mahasenabheetha/codec/v2/internal/netutil"
)

// netRequest carries every Network tool job; each reads its fields.
type netRequest struct {
	Input string `json:"input"`
	Op    string `json:"op"` // cidr: "", split, contains; url: parse, build
	// cidr
	Prefix int      `json:"prefix"` // split: the subnets' length
	IP     string   `json:"ip"`     // contains: the address to look for
	Lines  []string `json:"lines"`  // overlaps: one range per line
	// url
	Parts encode.URLParts `json:"parts"`
}

// POST /api/v2/net/{kind}: cidr (details, split, contains), overlaps,
// ip, url (parse, build). Nothing is looked up on the network and
// nothing is kept (decision 19). Bad input is a 422 with the reason.
func (s *Server) handleNet(w http.ResponseWriter, r *http.Request) {
	var req netRequest
	if !decode(w, r, &req) {
		return
	}
	fail := func(err error) { writeError(w, http.StatusUnprocessableEntity, err.Error()) }
	switch r.PathValue("kind") + ":" + req.Op {
	case "cidr:":
		rng, err := netutil.Details(req.Input)
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"range": rng})
	case "cidr:split":
		sub, err := netutil.Split(req.Input, req.Prefix)
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, http.StatusOK, sub)
	case "cidr:contains":
		ok, err := netutil.Contains(req.Input, req.IP)
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"contains": ok})
	case "overlaps:":
		list, pairs, err := netutil.Overlaps(req.Lines)
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ranges": list, "overlaps": pairs})
	case "ip:":
		ip, err := netutil.ParseIP(req.Input)
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ip": ip})
	case "url:parse":
		p, err := encode.ParseURL(req.Input)
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"parts": p})
	case "url:build":
		u, err := encode.BuildURL(req.Parts)
		if err != nil {
			fail(err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"url": u})
	default:
		writeError(w, http.StatusNotFound, "unknown network job "+r.PathValue("kind")+" "+req.Op)
	}
}
