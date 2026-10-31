package web

import (
	"errors"
	"net/http"

	"github.com/mahasenabheetha/codec/v2/internal/jsonkit"
	"github.com/mahasenabheetha/codec/v2/internal/table"
)

// tableRowLimit caps the rows sent to the viewer; conversions use all.
const tableRowLimit = 100_000

type tableRequest struct {
	Input  string `json:"input"`
	Delim  string `json:"delimiter"` // "" = guess; ",", "\t", ";" or "|"
	Header string `json:"header"`    // "auto" (or ""), "yes", "no"
	To     string `json:"to"`        // convert: json, jsonl, yaml, markdown, csv, tsv
	Typed  bool   `json:"typed"`     // convert: values by column type
	Indent string `json:"indent"`    // convert: JSON indent, "min" = one line
}

// POST /api/v2/table/{kind}: parse (columns and rows for the viewer),
// convert (to another format), from-json (a JSON array to CSV or TSV).
// Nothing is kept (decision 19). Bad input is a 422 with the line and
// column when known.
func (s *Server) handleTable(w http.ResponseWriter, r *http.Request) {
	var req tableRequest
	if !decode(w, r, &req) {
		return
	}
	indent, ok := jsonIndent(req.Indent)
	if !ok {
		writeError(w, http.StatusBadRequest, "indent is spaces or tabs (at most 8), or \"min\"")
		return
	}
	var delim rune
	switch req.Delim {
	case "":
	case ",", "\t", ";", "|":
		delim = rune(req.Delim[0])
	default:
		writeError(w, http.StatusBadRequest, "delimiter is a comma, tab, semicolon or pipe, or empty to guess")
		return
	}
	fail := func(err error) {
		resp := errorResponse{Error: err.Error()}
		var se *table.SyntaxError
		var je *jsonkit.SyntaxError
		switch {
		case errors.As(err, &se):
			resp.Error, resp.Line, resp.Column = se.Msg, se.Line, se.Column
		case errors.As(err, &je):
			resp.Error, resp.Line, resp.Column = je.Msg, je.Line, je.Column
		}
		writeJSON(w, http.StatusUnprocessableEntity, resp)
	}

	switch r.PathValue("kind") {
	case "parse", "convert":
		t, err := table.Parse(req.Input, table.Options{Delim: delim, Header: req.Header})
		if err != nil {
			fail(err)
			return
		}
		about := map[string]any{
			"delimiter":     string(t.Delim),
			"header":        t.Header,
			"delimGuessed":  t.DelimGuessed,
			"headerGuessed": t.HeaderGuessed,
			"columns":       nonNil(t.Columns),
			"total":         len(t.Rows),
			"ragged":        t.Ragged,
		}
		if r.PathValue("kind") == "parse" {
			rows := t.Rows
			if len(rows) > tableRowLimit {
				rows = rows[:tableRowLimit]
			}
			about["rows"] = nonNil(rows)
			writeJSON(w, http.StatusOK, about)
			return
		}
		out, err := table.Convert(t, req.To, req.Typed, indent)
		if err != nil {
			fail(err)
			return
		}
		about["output"] = out
		writeJSON(w, http.StatusOK, about)
	case "from-json":
		t, err := table.FromJSON(req.Input)
		if err != nil {
			fail(err)
			return
		}
		d := ','
		if req.To == "tsv" {
			d = '\t'
		}
		writeJSON(w, http.StatusOK, map[string]any{"output": t.Delimited(d), "columns": nonNil(t.Columns), "total": len(t.Rows)})
	default:
		writeError(w, http.StatusNotFound, "unknown table job "+r.PathValue("kind"))
	}
}

// nonNil makes an empty slice encode as [] rather than null.
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
