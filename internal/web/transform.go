package web

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mahasenabheetha/codec/v2/internal/codec"
	"github.com/mahasenabheetha/codec/v2/internal/version"
)

// The v1 API: text transforms (base64, JSON, JWT, Ansible logs) and the
// build version. Kept stable for the tools panel and old clients.

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
