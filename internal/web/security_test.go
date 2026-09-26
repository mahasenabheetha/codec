package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testServer backs do(); v1 routes need no workspace or token.
var testServer = New(Options{})

// call sends one request to s with the given Host, token and Origin
// ("" = header not sent).
func call(s *Server, method, target, host, token, origin, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = host
	if token != "" {
		req.Header.Set("X-Codec-Token", token)
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestGuard(t *testing.T) {
	s := New(Options{})
	tok := s.Token()
	tests := []struct {
		name           string
		method, target string
		host, token    string
		origin         string
		want           int
	}{
		{"loopback host", "GET", "/api/v2/workspace", "127.0.0.1:8765", tok, "", 200},
		{"localhost on a mapped port", "GET", "/api/v2/workspace", "localhost:9000", tok, "", 200},
		{"IPv6 loopback", "GET", "/api/v2/workspace", "[::1]:8765", tok, "", 200},
		{"DNS rebinding host", "GET", "/api/v2/workspace", "evil.example:8765", tok, "", 403},
		{"rebinding host on the page too", "GET", "/", "evil.example:8765", "", "", 403},
		{"missing token", "GET", "/api/v2/workspace", "127.0.0.1:8765", "", "", 401},
		{"wrong token", "GET", "/api/v2/workspace", "127.0.0.1:8765", "nope", "", 401},
		{"token in query only for events", "GET", "/api/v2/workspace?token=" + tok, "127.0.0.1:8765", "", "", 401},
		{"foreign origin POST", "POST", "/api/transform", "127.0.0.1:8765", "", "https://evil.example", 403},
		{"same-machine origin POST", "POST", "/api/transform", "127.0.0.1:8765", "", "http://localhost:5173", 200},
		{"v1 needs no token", "GET", "/api/version", "127.0.0.1:8765", "", "", 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := ""
			if tt.method == "POST" {
				body = `{"input":"{}"}`
			}
			rec := call(s, tt.method, tt.target, tt.host, tt.token, tt.origin, body)
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d (%s)", rec.Code, tt.want, rec.Body)
			}
		})
	}
}

func TestWorkspaceAPI(t *testing.T) {
	outside := t.TempDir()
	os.WriteFile(filepath.Join(outside, "secret.yaml"), []byte("token: x\n"), 0o644)
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "k8s"), 0o755)
	os.WriteFile(filepath.Join(root, "k8s", "svc.yaml"), []byte("apiVersion: v1\nkind: Service\nmetadata:\n  name: web\n"), 0o644)

	s := New(Options{Poll: true})
	defer s.Close()
	get := func(target string) *httptest.ResponseRecorder {
		return call(s, "GET", target, "127.0.0.1:8765", s.Token(), "", "")
	}

	if rec := get("/api/v2/files/tree"); rec.Code != http.StatusConflict {
		t.Errorf("tree before open: %d, want 409", rec.Code)
	}
	body, _ := json.Marshal(map[string]string{"path": `"` + root + `"`}) // quotes as pasted from Explorer
	if rec := call(s, "POST", "/api/v2/workspace/open", "127.0.0.1:8765", s.Token(), "", string(body)); rec.Code != 200 {
		t.Fatalf("open: %d %s", rec.Code, rec.Body)
	}

	s.current().Classify(t.Context()) // normally in the background
	var tree treeView
	json.Unmarshal(get("/api/v2/files/tree").Body.Bytes(), &tree)
	if len(tree.Files) != 1 || tree.Files[0].Path != "k8s/svc.yaml" || tree.Files[0].Type != "kubernetes" {
		t.Errorf("tree = %+v", tree.Files)
	}

	var c contentView
	rec := get("/api/v2/files/content?path=k8s/svc.yaml")
	json.Unmarshal(rec.Body.Bytes(), &c)
	if rec.Code != 200 || !strings.Contains(c.Text, "kind: Service") {
		t.Errorf("content: %d %s", rec.Code, rec.Body)
	}

	rel, _ := filepath.Rel(root, filepath.Join(outside, "secret.yaml"))
	for _, p := range []string{"../secret.yaml", filepath.ToSlash(rel), filepath.Join(outside, "secret.yaml")} {
		if rec := get("/api/v2/files/content?path=" + url.QueryEscape(p)); rec.Code != http.StatusForbidden {
			t.Errorf("content %q: %d, want 403", p, rec.Code)
		}
	}
	if rec := get("/api/v2/files/content?path=missing.yaml"); rec.Code != http.StatusNotFound {
		t.Errorf("missing file: %d, want 404", rec.Code)
	}
}
