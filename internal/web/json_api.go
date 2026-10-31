package web

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/mahasenabheetha/codec/v2/internal/jsonkit"
)

// jsonRequest carries every JSON & YAML tool job; each reads its fields.
type jsonRequest struct {
	Input  string `json:"input"`
	Indent string `json:"indent"` // "" = two spaces; "min" = one line
	Sort   bool   `json:"sort"`   // format: sort object keys
	Other  string `json:"other"`  // diff: the right-hand side
	Expr   string `json:"expr"`   // query: the jq filter
}

// jqTimeout bounds one jq run: a filter can loop forever.
const jqTimeout = 5 * time.Second

// POST /api/v2/json/{kind}: format, to-yaml, from-yaml, flatten,
// unflatten, diff, schema, query, stats (decision 90). JSON is read as
// JSON, JSONC or JSON5 and written as strict JSON. Nothing is kept
// (decision 19). Bad input is a 422 with the line and column when known.
func (s *Server) handleJSON(w http.ResponseWriter, r *http.Request) {
	var req jsonRequest
	if !decode(w, r, &req) {
		return
	}
	indent, ok := jsonIndent(req.Indent)
	if !ok {
		writeError(w, http.StatusBadRequest, "indent is spaces or tabs (at most 8), or \"min\"")
		return
	}
	fail := func(err error) {
		resp := errorResponse{Error: err.Error()}
		var se *jsonkit.SyntaxError
		if errors.As(err, &se) {
			resp.Error, resp.Line, resp.Column = se.Msg, se.Line, se.Column
		}
		writeJSON(w, http.StatusUnprocessableEntity, resp)
	}
	ok200 := func(v any) { writeJSON(w, http.StatusOK, v) }

	switch r.PathValue("kind") {
	case "format":
		v, d, err := jsonkit.Parse(req.Input)
		if err != nil {
			fail(err)
			return
		}
		if req.Sort {
			v = jsonkit.SortKeys(v)
		}
		ok200(map[string]any{"output": jsonkit.Encode(v, indent), "dialect": d})
	case "to-yaml":
		v, d, err := jsonkit.Parse(req.Input)
		if err != nil {
			fail(err)
			return
		}
		y, err := jsonkit.ToYAML(v)
		if err != nil {
			fail(err)
			return
		}
		ok200(map[string]any{"output": y, "dialect": d})
	case "from-yaml":
		v, err := jsonkit.FromYAML(req.Input)
		if err != nil {
			fail(err)
			return
		}
		ok200(map[string]any{"output": jsonkit.Encode(v, indent)})
	case "flatten":
		v, d, err := jsonkit.Parse(req.Input)
		if err != nil {
			fail(err)
			return
		}
		leaves := jsonkit.Flatten(v)
		ok200(map[string]any{"output": jsonkit.FlattenText(v), "leaves": leaves, "dialect": d})
	case "unflatten":
		v, err := jsonkit.Unflatten(req.Input)
		if err != nil {
			fail(err)
			return
		}
		ok200(map[string]any{"output": jsonkit.Encode(v, indent)})
	case "diff":
		a, _, err := jsonkit.Parse(req.Input)
		if err != nil {
			fail(sideError("the left side", err))
			return
		}
		b, _, err := jsonkit.Parse(req.Other)
		if err != nil {
			fail(sideError("the right side", err))
			return
		}
		ok200(map[string]any{"changes": jsonkit.Diff(a, b)})
	case "schema":
		samples, _, err := jsonkit.ParseAll(req.Input)
		if err != nil {
			fail(err)
			return
		}
		ok200(map[string]any{"output": jsonkit.Encode(jsonkit.Infer(samples), indent), "samples": len(samples)})
	case "query":
		v, _, err := jsonkit.Parse(req.Input)
		if err != nil {
			fail(err)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), jqTimeout)
		defer cancel()
		res, err := jsonkit.Query(ctx, v, req.Expr, indent)
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			fail(errors.New("the filter ran for 5 seconds and was stopped"))
			return
		}
		if err != nil {
			fail(err)
			return
		}
		ok200(res)
	case "stats":
		st, err := jsonkit.Scan(strings.NewReader(req.Input))
		if err != nil {
			fail(err)
			return
		}
		ok200(st)
	default:
		writeError(w, http.StatusNotFound, "unknown JSON job "+r.PathValue("kind"))
	}
}

// jsonIndent turns the request's indent into what Encode takes.
func jsonIndent(s string) (string, bool) {
	switch {
	case s == "":
		return "  ", true
	case s == "min":
		return "", true
	case len(s) <= 8 && strings.Trim(s, " \t") == "":
		return s, true
	}
	return "", false
}

// sideError keeps a SyntaxError's position but says which side it is in.
func sideError(side string, err error) error {
	var se *jsonkit.SyntaxError
	if errors.As(err, &se) {
		c := *se
		c.Msg = side + ": " + c.Msg
		return &c
	}
	return err
}
