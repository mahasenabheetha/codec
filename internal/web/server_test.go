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
