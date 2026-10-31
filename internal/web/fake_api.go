package web

import (
	"net/http"

	"github.com/mahasenabheetha/codec/v2/internal/fake"
)

type fakeRequest struct {
	// Seed repeats a run; 0 or none picks one, which comes back.
	Seed uint64 `json:"seed"`
	// Lorem
	Unit    string `json:"unit"`
	Classic bool   `json:"classic"`
	// Rows
	Columns []fake.Column `json:"columns"`
	Format  string        `json:"format"`
	Table   string        `json:"table"`
	Count   int           `json:"count"`
}

// GET /api/v2/fake/kinds: the column kinds, presets and formats.
func (s *Server) handleFakeKinds(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"kinds": fake.Kinds, "presets": fake.Presets, "formats": fake.Formats})
}

// POST /api/v2/fake/{kind}: lorem, rows. Made-up text and data only;
// nothing is stored (decision 19).
func (s *Server) handleFake(w http.ResponseWriter, r *http.Request) {
	var req fakeRequest
	if !decode(w, r, &req) {
		return
	}
	seed := req.Seed
	if seed == 0 {
		seed = fake.NewSeed()
	}
	var out string
	var err error
	switch r.PathValue("kind") {
	case "lorem":
		out, err = fake.Lorem(req.Unit, req.Count, seed, req.Classic)
	case "rows":
		out, err = fake.Write(req.Columns, req.Count, seed, req.Format, req.Table)
	default:
		writeError(w, http.StatusNotFound, "unknown fake job "+r.PathValue("kind"))
		return
	}
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"output": out, "seed": seed})
}
