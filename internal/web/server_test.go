package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// do sends one request through the full route table and returns the
// recorded response. No network, no port — pure function calls.
func do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	req.Host = "127.0.0.1:8765"
	testServer.Handler().ServeHTTP(rec, req)
	return rec
}

// TestTransformJSON sends no mode field at all, proving the API stays
// backward compatible: old clients get auto-detect, as before.
func TestTransformJSON(t *testing.T) {
	rec := do(t, http.MethodPost, "/api/transform", `{"input":"{\"a\":1}"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}

	var resp transformResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if resp.Output != "eyJhIjoxfQ==" {
		t.Errorf("Output = %q, want %q", resp.Output, "eyJhIjoxfQ==")
	}
	if resp.Kind != "json" {
		t.Errorf("Kind = %q, want %q", resp.Kind, "json")
	}
}

func TestExplicitEncodeAcceptsAnyText(t *testing.T) {
	rec := do(t, http.MethodPost, "/api/transform",
		`{"input":"my name is mahasen","mode":"b64-encode"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}

	var resp transformResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if resp.Output != "bXkgbmFtZSBpcyBtYWhhc2Vu" {
		t.Errorf("Output = %q, want %q", resp.Output, "bXkgbmFtZSBpcyBtYWhhc2Vu")
	}
}

func TestURLSafeFlag(t *testing.T) {
	rec := do(t, http.MethodPost, "/api/transform",
		`{"input":">>>???","mode":"b64-encode","urlSafe":true}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}

	var resp transformResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if resp.Output != "Pj4-Pz8_" {
		t.Errorf("Output = %q, want %q", resp.Output, "Pj4-Pz8_")
	}
}

func TestUnknownModeIsBadRequest(t *testing.T) {
	rec := do(t, http.MethodPost, "/api/transform",
		`{"input":"x","mode":"banana"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body)
	}
}

func TestValidateModeReportsPosition(t *testing.T) {
	rec := do(t, http.MethodPost, "/api/transform",
		`{"input":"{\"a\":}","mode":"validate"}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body: %s", rec.Code, rec.Body)
	}

	var resp errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if resp.Line != 1 || resp.Column == 0 {
		t.Errorf("Line/Column = %d/%d, want 1/nonzero", resp.Line, resp.Column)
	}
}

func TestTransformInvalidInput(t *testing.T) {
	rec := do(t, http.MethodPost, "/api/transform", `{"input":"???"}`)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body: %s", rec.Code, rec.Body)
	}

	var resp errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if resp.Error == "" {
		t.Error("Error field is empty, want a message")
	}
}

func TestTransformBadBody(t *testing.T) {
	rec := do(t, http.MethodPost, "/api/transform", `this is not json at all`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body)
	}
}

func TestTransformWrongMethod(t *testing.T) {
	rec := do(t, http.MethodGet, "/api/transform", "")

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
}

// TestAppIsServedAtRoot checks routing only: whether this test binary
// embedded a real frontend build (200) or not (503 placeholder) depends
// on how it was built, so both are accepted. TestAppHandler covers the
// two cases themselves.
func TestAppIsServedAtRoot(t *testing.T) {
	rec := do(t, http.MethodGet, "/", "")

	if rec.Code != http.StatusOK && rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 200 or 503", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<title>codec</title>") {
		t.Error("the codec page does not appear to be served at /")
	}
}

func TestOldAppPathRedirects(t *testing.T) {
	rec := do(t, http.MethodGet, "/app/", "")

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want 301", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Errorf("Location = %q, want %q", loc, "/")
	}
}

func TestPrettyIndent(t *testing.T) {
	tests := []struct {
		name   string
		indent string
		want   string
	}{
		{"default is two spaces", "", "{\n  \"a\": 1\n}"},
		{"four spaces", "    ", "{\n    \"a\": 1\n}"},
		{"tab", "\t", "{\n\t\"a\": 1\n}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(transformRequest{Input: `{"a":1}`, Mode: "json-pretty", Indent: tt.indent})
			rec := do(t, http.MethodPost, "/api/transform", string(body))

			var resp transformResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("response is not valid JSON: %v", err)
			}
			if resp.Output != tt.want {
				t.Errorf("Output = %q, want %q", resp.Output, tt.want)
			}
		})
	}
}

// TestJWTStructured checks the structured jwt field is attached both
// in explicit jwt mode and when auto-detect finds a token.
func TestJWTStructured(t *testing.T) {
	// {"alg":"HS256"} . {"sub":"1"} . signature
	const token = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.c2ln"

	for _, mode := range []string{"jwt", "auto"} {
		t.Run(mode, func(t *testing.T) {
			rec := do(t, http.MethodPost, "/api/transform",
				`{"input":"`+token+`","mode":"`+mode+`"}`)

			var resp transformResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("response is not valid JSON: %v", err)
			}
			if resp.JWT == nil {
				t.Fatalf("jwt field missing; body: %s", rec.Body)
			}
			if !strings.Contains(resp.JWT.Payload, `"sub": "1"`) {
				t.Errorf("Payload = %q, want it to contain the sub claim", resp.JWT.Payload)
			}
			if resp.JWT.Signature != "c2ln" {
				t.Errorf("Signature = %q, want %q", resp.JWT.Signature, "c2ln")
			}
		})
	}
}

func TestVersionEndpoint(t *testing.T) {
	rec := do(t, http.MethodGet, "/api/version", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}

	var resp struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if resp.Version == "" {
		t.Error("version is empty")
	}
}

func TestAppHandler(t *testing.T) {
	tests := []struct {
		name       string
		fsys       fstest.MapFS
		wantStatus int
		wantBody   string
	}{
		{
			name:       "built frontend is served",
			fsys:       fstest.MapFS{"index.html": {Data: []byte("<head></head><h1>app</h1>")}},
			wantStatus: http.StatusOK,
			wantBody:   `<meta name="codec-token" content="tok">`,
		},
		{
			name:       "missing build shows the not-built page",
			fsys:       fstest.MapFS{},
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   "Frontend not built",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			appHandler(tt.fsys, "tok", false).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want it to contain %q", rec.Body, tt.wantBody)
			}
		})
	}
}
