package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// do sends one request through the full route table and returns the
// recorded response. No network, no port — pure function calls.
func do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, req)
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

func TestStaticPageIsServed(t *testing.T) {
	rec := do(t, http.MethodGet, "/", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<title>codec</title>") {
		t.Error("index.html does not appear to be served at /")
	}
}
