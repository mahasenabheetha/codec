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

// distFiles holds the Vite build of frontend/ (written to dist/app by
// `npm run build`), compiled INTO the binary so the shipped executable
// needs no files next to it. The "all:" prefix also embeds the
// committed dist/.keep, so the package compiles even before the
// frontend has ever been built — go:embed rejects an empty directory.
//
//go:embed all:dist
var distFiles embed.FS

// notBuiltPage is served in place of the UI when the binary was
// compiled without a frontend build.
const notBuiltPage = `<!doctype html>
<title>codec</title>
<body style="background:#0b0d12;color:#e6e8ee;font-family:system-ui;padding:2rem">
<h1>Frontend not built</h1>
<p>This binary was compiled without the web UI. Run
<code>npm --prefix frontend ci &amp;&amp; npm --prefix frontend run build</code>,
then rebuild codec.</p>
</body>`

// transformRequest is the JSON body the browser sends. Every field but
// Input is optional: a request carrying only "input" behaves exactly as
// before the others existed (auto-detect, standard alphabet, default
// indentation), because their zero values select those defaults.
type transformRequest struct {
	Input   string `json:"input"`
	Mode    string `json:"mode"`
	URLSafe bool   `json:"urlSafe"`
	Indent  string `json:"indent"` // json-pretty only; "" = two spaces
}

// transformResponse is what a successful transform returns. Output
// always holds the plain-text rendering (used by Copy and Swap). Task
// and JWT carry structured forms of the same result, so the frontend
// can render rich views without re-parsing anything itself.
type transformResponse struct {
	Output string            `json:"output"`
	Kind   string            `json:"kind"`
	Task   *codec.ParsedTask `json:"task,omitempty"`
	JWT    *jwtView          `json:"jwt,omitempty"`
}

// jwtView is the wire form of codec.JWT. The engine type has no JSON
// tags on purpose: how a token looks over HTTP is this layer's call.
type jwtView struct {
	Header    string `json:"header"`    // pretty-printed JSON
	Payload   string `json:"payload"`   // pretty-printed JSON
	Signature string `json:"signature"` // base64url, not verified
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

	// The embedded FS is rooted above "dist/app"; re-root it so the
	// browser requests /index.html, not /dist/app/index.html.
	appRoot, err := fs.Sub(distFiles, "dist/app")
	if err != nil {
		// Unreachable unless the literal path above is malformed,
		// which would be a programming mistake, not a runtime one.
		panic(err)
	}

	mux.Handle("/", appHandler(appRoot))
	mux.HandleFunc("POST /api/transform", handleTransform)
	mux.HandleFunc("GET /api/version", handleVersion)

	// The v2 UI was previewed under /app/ before it replaced v1;
	// send old bookmarks and installed shortcuts to the real page.
	mux.Handle("/app/", http.RedirectHandler("/", http.StatusMovedPermanently))

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

	opts := codec.Options{Indent: req.Indent}
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

	// Attach structured forms for rich views, whether the user picked
	// the mode explicitly or auto-detect found it. The second parse is
	// cheap and keeps codec.Apply's signature free of UI concerns.
	switch kind {
	case codec.KindAnsible:
		if task, perr := codec.ParseAnsible(req.Input); perr == nil {
			resp.Task = task
		}
	case codec.KindJWT:
		if tok, perr := codec.DecodeJWT(req.Input); perr == nil {
			resp.JWT = &jwtView{Header: tok.Header, Payload: tok.Payload, Signature: tok.Signature}
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
