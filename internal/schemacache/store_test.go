package schemacache

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

func TestStore(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/missing.json" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"type":"object","properties":{"a":{"type":"integer"}}}`))
	}))
	defer srv.Close()
	ctx := context.Background()
	cache := t.TempDir()

	s := New(Options{Dir: cache})
	if _, err := s.Get(ctx, srv.URL+"/s.json"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(ctx, srv.URL+"/missing.json"); !errors.Is(err, ErrNotPublished) {
		t.Errorf("missing: got %v", err)
	}

	// A new store (a restart) in offline mode serves the cached copy.
	off := New(Options{Dir: cache, Offline: true})
	if _, err := off.Get(ctx, srv.URL+"/s.json"); err != nil {
		t.Errorf("offline cached: %v", err)
	}
	if _, err := off.Get(ctx, srv.URL+"/other.json"); !errors.Is(err, ErrOffline) {
		t.Errorf("offline uncached: got %v", err)
	}
	if hits.Load() != 2 {
		t.Errorf("%d downloads, want 2", hits.Load())
	}

	// A custom folder wins, even with just the file name in it.
	custom := t.TempDir()
	os.WriteFile(filepath.Join(custom, "other.json"), []byte(`{"type":"string"}`), 0o644)
	cs := New(Options{Dir: t.TempDir(), CustomDir: custom, Offline: true})
	if _, err := cs.Get(ctx, srv.URL+"/deep/path/other.json"); err != nil {
		t.Errorf("custom folder: %v", err)
	}
}
