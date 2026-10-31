package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/mahasenabheetha/codec/v2/internal/update"
	"github.com/mahasenabheetha/codec/v2/internal/version"
)

func TestUpdateAPI(t *testing.T) {
	var hits atomic.Int32
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`[{"tag_name":"v3.7.0","html_url":"https://example.test/370"}]`))
	}))
	defer gh.Close()
	checker := update.New("")
	checker.BaseURL = gh.URL
	opts := testOptions(false)
	opts.Updates = checker
	s := New(opts)
	defer s.Close()

	old := version.Version
	version.Version = "v3.6.0"
	defer func() { version.Version = old }()

	get := func(target string) updateView {
		t.Helper()
		rec := call(s, "GET", target, "127.0.0.1:8765", s.Token(), "", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", target, rec.Code, rec.Body)
		}
		var v updateView
		if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		return v
	}

	// Off by default: nothing is asked.
	if v := get("/api/v2/update"); v.Enabled || v.Skipped != "off" || hits.Load() != 0 {
		t.Errorf("off: %+v, %d requests", v, hits.Load())
	}
	// "Check now" asks even when off.
	if v := get("/api/v2/update?force=1"); !v.Newer || v.Latest == nil || v.Latest.Version != "v3.7.0" || hits.Load() != 1 {
		t.Errorf("forced: %+v, %d requests", v, hits.Load())
	}

	// Switched on, the daily check answers.
	rec := call(s, "POST", "/api/v2/update/settings", "127.0.0.1:8765", s.Token(), "", `{"check":true}`)
	if rec.Code != http.StatusOK || !s.opts.Config.Get().Updates.Check {
		t.Fatalf("settings: %d %s", rec.Code, rec.Body)
	}
	if v := get("/api/v2/update"); !v.Enabled || !v.Newer {
		t.Errorf("on: %+v", v)
	}

	// The tray reads the day's answer only. The checker here keeps no
	// cache, so there is none to read; with one, it is the newer release.
	if rel := s.NewerRelease(); rel != nil {
		t.Errorf("no cache, yet NewerRelease = %+v", rel)
	}
	cached := update.New(t.TempDir())
	cached.BaseURL = gh.URL
	s.opts.Updates = cached
	before := hits.Load()
	if rel := s.NewerRelease(); rel != nil || hits.Load() != before {
		t.Errorf("before a check: %+v, %d new requests", rel, hits.Load()-before)
	}
	get("/api/v2/update?force=1")
	if rel := s.NewerRelease(); rel == nil || rel.Version != "v3.7.0" {
		t.Errorf("after a check: %+v", rel)
	}
}
