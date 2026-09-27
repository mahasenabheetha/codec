package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mahasenabheetha/codec/v2/internal/config"
	"github.com/mahasenabheetha/codec/v2/internal/helm"
)

// testServer backs do(); v1 routes need no workspace or token.
var testServer = New(testOptions(false))

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
	s := New(testOptions(false))
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

	s := New(testOptions(true))
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

func TestYAMLAPI(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "v.yaml"), []byte("base: &b\n  x: 1\nuse: *b\n"), 0o644)
	s := New(testOptions(true))
	defer s.Close()
	if err := s.OpenWorkspace(root); err != nil {
		t.Fatal(err)
	}
	post := func(target, body string) map[string]any {
		t.Helper()
		rec := call(s, "POST", target, "127.0.0.1:8765", s.Token(), "", body)
		if rec.Code != 200 {
			t.Fatalf("%s: %d %s", target, rec.Code, rec.Body)
		}
		var out map[string]any
		json.Unmarshal(rec.Body.Bytes(), &out)
		return out
	}

	a := post("/api/v2/yaml/analyze", `{"path":"v.yaml"}`) // from disk
	if a["type"] != "yaml" || len(a["docs"].([]any)) != 1 {
		t.Errorf("analyze = %v", a)
	}
	a = post("/api/v2/yaml/analyze", `{"path":"v.yaml","content":"a:\n\tb: 1\n"}`) // what-if buffer
	if d := a["diagnostics"].([]any); len(d) != 1 || d[0].(map[string]any)["code"] != "tab-indent" {
		t.Errorf("diagnostics = %v", a["diagnostics"])
	}
	def := post("/api/v2/yaml/definition", `{"path":"v.yaml","line":3,"col":7}`)
	if locs := def["locations"].([]any); len(locs) != 1 {
		t.Errorf("definition = %v", def)
	}
	p := post("/api/v2/yaml/path", `{"path":"v.yaml","line":2,"col":4}`)
	if p["formats"].(map[string]any)["yq"] != ".base.x" {
		t.Errorf("path = %v", p)
	}
	d := post("/api/v2/files/diff", `{"path":"v.yaml","content":"base: &b\n  x: 2\nuse: *b\n"}`)
	if !strings.Contains(d["diff"].(string), "-  x: 1\n+  x: 2\n") {
		t.Errorf("diff = %q", d["diff"])
	}
}

func TestHelmAPI(t *testing.T) {
	root, _ := filepath.Abs("../helm/testdata")
	s := New(testOptions(true))
	defer s.Close()
	if err := s.OpenWorkspace(root); err != nil {
		t.Fatal(err)
	}
	s.current().Classify(t.Context())
	rec := call(s, "GET", "/api/v2/helm/charts", "127.0.0.1:8765", s.Token(), "", "")
	var charts struct {
		Charts []chartView `json:"charts"`
	}
	json.Unmarshal(rec.Body.Bytes(), &charts)
	if len(charts.Charts) != 1 || charts.Charts[0].Path != "app" || !slices.Contains(charts.Charts[0].Candidates, "values-prod.yaml") {
		t.Fatalf("charts = %+v", charts)
	}

	body := `{"chart":"app","values":["values-prod.yaml"],"overrides":{"values-prod.yaml":"replicaCount: 9\n"}}`
	rec = call(s, "POST", "/api/v2/helm/render", "127.0.0.1:8765", s.Token(), "", body)
	var res struct {
		Manifest    string            `json:"manifest"`
		ValuesYAML  string            `json:"valuesYAML"`
		Diagnostics []helm.Diagnostic `json:"diagnostics"`
	}
	json.Unmarshal(rec.Body.Bytes(), &res)
	if rec.Code != 200 || !strings.Contains(res.Manifest, "replicas: 9") || !strings.Contains(res.ValuesYAML, "replicaCount: 9") {
		t.Fatalf("render: %d %s", rec.Code, rec.Body.String()[:min(400, rec.Body.Len())])
	}
	for _, d := range res.Diagnostics {
		if d.Severity == "error" {
			t.Errorf("unexpected error %+v", d)
		}
		if d.File != "" && !strings.HasPrefix(d.File, "app/") && d.File != "values-prod.yaml" {
			t.Errorf("diagnostic path not workspace-relative: %+v", d)
		}
	}
}

// testOptions keeps settings in memory with schema validation off, so
// tests never download schemas.
func testOptions(poll bool) Options {
	cfg := config.Memory()
	cfg.Update(func(s *config.Settings) { s.Lint.NoSchemas = true })
	return Options{Config: cfg, Poll: poll}
}
