// Package web serves the browser UI for codec. Like the cli package,
// it is a presentation layer only: it translates HTTP requests into
// calls on internal/codec and translates the results back into JSON.
package web

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"

	"github.com/mahasenabheetha/codec/internal/codec"
)

// staticFiles holds the frontend, compiled INTO the binary at build
// time by the go:embed directive below. The shipped executable needs
// no files next to it — the web page travels inside it.
//
//go:embed static
var staticFiles embed.FS

// transformRequest is the JSON body the browser sends.
type transformRequest struct {
	Input string `json:"input"`
}

// transformResponse is what a successful transform returns.
type transformResponse struct {
	Output string `json:"output"`
	Kind   string `json:"kind"`
}

// errorResponse is returned for any failure. Line and Column are only
// present for JSON syntax errors, so the frontend can point at the spot.
type errorResponse struct {
	Error  string `json:"error"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}

// Handler builds the full route table. It is exported (rather than
// buried inside Serve) so tests can exercise the exact same routes
// without opening a real network port.
func Handler() http.Handler {
	mux := http.NewServeMux()

	// The embedded FS is rooted above "static/"; strip that prefix so
	// the browser can request /index.html, not /static/index.html.
	staticRoot, err := fs.Sub(staticFiles, "static")
	if err != nil {
		// Unreachable unless the embed directive itself is broken,
		// which would be a compile-time mistake, not a runtime one.
		panic(err)
	}

	mux.Handle("/", http.FileServer(http.FS(staticRoot)))
	mux.HandleFunc("POST /api/transform", handleTransform)

	return mux
}

// Serve blocks forever, listening on addr.
func Serve(addr string) error {
	fmt.Printf("codec web UI running at http://%s — Ctrl+C to stop\n", addr)
	return http.ListenAndServe(addr, Handler())
}

// handleTransform is the API: JSON in, JSON out, status code says how
// it went. All actual work happens in internal/codec.
func handleTransform(w http.ResponseWriter, r *http.Request) {
	var req transformRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest,
			errorResponse{Error: "invalid request body: " + err.Error()})
		return
	}

	out, kind, err := codec.Transform(req.Input)
	if err != nil {
		resp := errorResponse{Error: err.Error()}

		// If the failure was a JSON syntax error, surface the
		// position as structured fields — this is why SyntaxError
		// carries real Line/Column ints instead of just a message.
		var syn *codec.SyntaxError
		if errors.As(err, &syn) {
			resp.Line, resp.Column = syn.Line, syn.Column
		}

		writeJSON(w, http.StatusUnprocessableEntity, resp)
		return
	}

	writeJSON(w, http.StatusOK, transformResponse{
		Output: out,
		Kind:   kind.String(),
	})
}

// writeJSON sends v as a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Too late to tell the client anything — the status line is
		// already sent. Log it and move on.
		log.Printf("write response: %v", err)
	}
}
