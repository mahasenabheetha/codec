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

	"github.com/mahasenabheetha/codec/v2/internal/codec"
	"github.com/mahasenabheetha/codec/v2/internal/version"
)

// staticFiles holds the frontend, compiled INTO the binary at build
// time by the go:embed directive below. The shipped executable needs
// no files next to it — the web page travels inside it.
//
//go:embed static
var staticFiles embed.FS

// distFiles holds the Vite build of frontend/ (written to dist/app by
// `npm run build`). The "all:" prefix also embeds the committed
// dist/.keep, so the package compiles even before the frontend has
// ever been built — go:embed rejects a directory with nothing in it.
//
//go:embed all:dist
var distFiles embed.FS

// notBuiltPage is served in place of the new UI when the binary was
// compiled without a frontend build.
const notBuiltPage = `<!doctype html>
<title>codec</title>
<body style="background:#0b0d12;color:#e6e8ee;font-family:system-ui;padding:2rem">
<h1>Frontend not built</h1>
<p>This binary was compiled without the web UI. Run
<code>npm --prefix frontend ci &amp;&amp; npm --prefix frontend run build</code>,
then rebuild codec.</p>
</body>`

// transformRequest is the JSON body the browser sends. Mode and
// URLSafe are optional: a request carrying only "input" behaves
// exactly as before this field existed (auto-detect, standard
// alphabet), because their zero values select those defaults.
type transformRequest struct {
	Input   string `json:"input"`
	Mode    string `json:"mode"`
	URLSafe bool   `json:"urlSafe"`
}

// transformResponse is what a successful transform returns. Task is
// only present for ansible results: it carries the structured form so
// the frontend can render a rich, color-coded view, while Output always
// holds the plain-text rendering (used by Copy and Swap).
type transformResponse struct {
	Output string            `json:"output"`
	Kind   string            `json:"kind"`
	Task   *codec.ParsedTask `json:"task,omitempty"`
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

	appRoot, err := fs.Sub(distFiles, "dist/app")
	if err != nil {
		panic(err) // same reasoning as above: only a bad literal path fails
	}

	mux.Handle("/", http.FileServer(http.FS(staticRoot)))
	// The new Svelte UI lives under /app/ until it replaces the v1 UI.
	mux.Handle("/app/", http.StripPrefix("/app/", appHandler(appRoot)))
	mux.HandleFunc("POST /api/transform", handleTransform)
	mux.HandleFunc("GET /api/version", handleVersion)

	// Non-POST requests to the API path would otherwise fall through
	// to the "/" file server above and produce a confusing 404. This
	// method-less pattern is more specific than "/" (so it wins for
	// GET etc.) but less specific than "POST /api/transform" (so real
	// API calls never land here).
	mux.HandleFunc("/api/transform", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed; use POST", http.StatusMethodNotAllowed)
	})

	return mux
}

// appHandler serves the built frontend from fsys, or a short "not
// built" page when fsys holds no index.html. It takes the FS as a
// parameter so tests can exercise both cases without a real build.
func appHandler(fsys fs.FS) http.Handler {
	if _, err := fs.Stat(fsys, "index.html"); err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, notBuiltPage)
		})
	}
	return http.FileServer(http.FS(fsys))
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

	mode := codec.Mode(req.Mode)
	if mode == "" {
		mode = codec.ModeAuto
	}

	opts := codec.Options{}
	if req.URLSafe {
		opts.Variant = codec.VariantURL
	}

	out, kind, err := codec.Apply(mode, req.Input, opts)
	if err != nil {
		// An unknown mode is the caller's bug (400); input that
		// cannot be transformed is the input's fault (422).
		status := http.StatusUnprocessableEntity
		if errors.Is(err, codec.ErrUnknownMode) {
			status = http.StatusBadRequest
		}

		resp := errorResponse{Error: err.Error()}

		// If the failure was a JSON syntax error, surface the
		// position as structured fields — this is why SyntaxError
		// carries real Line/Column ints instead of just a message.
		var syn *codec.SyntaxError
		if errors.As(err, &syn) {
			resp.Line, resp.Column = syn.Line, syn.Column
		}

		writeJSON(w, status, resp)
		return
	}

	resp := transformResponse{Output: out, Kind: kind.String()}

	// Attach the structured task for ansible results, whether the
	// user picked the mode explicitly or auto-detect found it.
	if kind == codec.KindAnsible {
		if task, perr := codec.ParseAnsible(req.Input); perr == nil {
			resp.Task = task
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleVersion reports which build is serving the page, so the UI
// (and an installed PWA, which outlives server restarts) can show it.
func handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, version.Get())
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
