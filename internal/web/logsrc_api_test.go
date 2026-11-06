package web

import (
	"os"
	"path/filepath"
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
