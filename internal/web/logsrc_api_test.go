package web

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogEntryRefs(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "internal", "shop"), 0o755)
	os.WriteFile(filepath.Join(root, "internal", "shop", "cart.go"), []byte("package shop\n"), 0o644)
	s := New(testOptions(false))
	defer s.Close()

	// "é" and "😀" take one and two UTF-16 units: the offsets must count
	// those, not bytes.
	log := "é😀 failed at /build/src/internal/shop/cart.go:12:3\nok in missing.go:4\n"
	id := openLog(t, s, [][2]string{{"run.log", log}})

	// No folder open: references, but no sources.
	_, v := send(s, "GET", "/api/v2/logs/"+id+"/entries/0", "", "")
	e := v["entry"].(map[string]any)
	refs := e["refs"].(map[string]any)["message"].([]any)
	r := refs[0].(map[string]any)
	if len(refs) != 1 || r["start"] != 14.0 || r["end"] != 51.0 || r["line"] != 12.0 || r["col"] != 3.0 || e["sources"] != nil {
		t.Errorf("no folder: %v", e["refs"])
	}

	if err := s.OpenWorkspace(root); err != nil {
		t.Fatal(err)
	}
	_, v = send(s, "GET", "/api/v2/logs/"+id+"/entries/0", "", "")
	src := v["entry"].(map[string]any)["sources"].(map[string]any)
	m := src["/build/src/internal/shop/cart.go"].([]any)
	if len(m) != 1 || m[0].(map[string]any)["file"] != "internal/shop/cart.go" || m[0].(map[string]any)["quality"] != "exact" {
		t.Errorf("sources: %v", src)
	}
	_, v = send(s, "GET", "/api/v2/logs/"+id+"/entries/1", "", "")
	if m := v["entry"].(map[string]any)["sources"].(map[string]any)["missing.go"].([]any); len(m) != 0 {
		t.Errorf("missing file: %v", m)
	}
}

func TestFindSource(t *testing.T) {
	root := t.TempDir()
	for p, body := range map[string]string{
		"internal/shop/cart.go":   "package shop\n\nfunc add() error {\n\treturn fmt.Errorf(\"cart is full\")\n}\n",
		"docs/notes.md":           "When the Cart Is Full, retry.\n",
		"run.log":                 "ERROR cart is full\n",              // a log: its own copy of the line
		"node_modules/x/index.js": "throw new Error('cart is full')\n", // a dependency
		"build/out/cart.go":       "cart is full\n",
	} {
		os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0o755)
		os.WriteFile(filepath.Join(root, p), []byte(body), 0o644)
	}
	s := New(testOptions(false))
	defer s.Close()

	if code, _ := send(s, "POST", "/api/v2/logs/find-source", "application/json", `{"text":"cart is full"}`); code != 409 {
		t.Errorf("no folder: %d", code)
	}
	if err := s.OpenWorkspace(root); err != nil {
		t.Fatal(err)
	}
	code, v := send(s, "POST", "/api/v2/logs/find-source", "application/json", `{"text":"  cart is full "}`)
	var got []string
	for _, m := range v["matches"].([]any) {
		m := m.(map[string]any)
		got = append(got, fmt.Sprintf("%s:%v", m["path"], m["line"]))
	}
	// Literal and case-insensitive, logs and build output left out.
	if want := "docs/notes.md:1 internal/shop/cart.go:4"; code != 200 || strings.Join(got, " ") != want {
		t.Errorf("find-source = %d %v, want %s", code, got, want)
	}
	for body, want := range map[string]int{`{"text":"  "}`: 422, `{"text":"` + strings.Repeat("x", 301) + `"}`: 422, `{"text":"a.*b"}`: 200,
		// 200 characters, 400 bytes: counted as characters, as the UI does.
		`{"text":"` + strings.Repeat("é", 200) + `"}`: 200} {
		if code, v := send(s, "POST", "/api/v2/logs/find-source", "application/json", body); code != want {
			t.Errorf("%.20s: %d %v", body, code, v)
		}
	}
}

func TestFindLine(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "shop"), 0o755)
	os.WriteFile(filepath.Join(root, "shop", "cart.go"), []byte("package shop\n\nfunc add() error {\n\treturn fmt.Errorf(\"cart is full\")\n}\n"), 0o644)
	s := New(testOptions(false))
	defer s.Close()
	body := `{"message":"checkout failed","error":"reserve stock: cart is full","fields":["order_id"]}`
	if code, _ := send(s, "POST", "/api/v2/logs/find-line", "application/json", body); code != 409 {
		t.Errorf("no folder: %d", code)
	}
	if err := s.OpenWorkspace(root); err != nil {
		t.Fatal(err)
	}
	code, v := send(s, "POST", "/api/v2/logs/find-line", "application/json", body)
	layers := v["layers"].([]any)
	last := layers[len(layers)-1].(map[string]any)
	hit := last["hits"].([]any)[0].(map[string]any)
	if code != 200 || len(layers) != 3 || last["text"] != "cart is full" || hit["path"] != "shop/cart.go" || hit["line"] != 4.0 || hit["quality"] != "exact" {
		t.Errorf("find-line: %d %v", code, v)
	}
	if code, v := send(s, "POST", "/api/v2/logs/find-line", "application/json", `{"message":"42"}`); code != 200 || len(v["layers"].([]any)) != 0 {
		t.Errorf("nothing to search: %d %v", code, v)
	}
}
