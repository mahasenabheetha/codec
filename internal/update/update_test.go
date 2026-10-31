package update

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestCompare(t *testing.T) {
	// Each version is older than the next (the order in semver.org §11).
	order := []string{
		"v1.0.0-alpha", "v1.0.0-alpha.1", "v1.0.0-alpha.beta", "v1.0.0-beta",
		"v1.0.0-beta.2", "v1.0.0-beta.11", "v1.0.0-rc.1", "v1.0.0", "v1.0.1",
		"v1.2.0", "v1.10.0", "v2.0.0", "v3.6.0", "v3.6.1", "v3.10.0",
	}
	for i := range order {
		for j := range order {
			a, ok1 := parse(order[i])
			b, ok2 := parse(order[j])
			if !ok1 || !ok2 {
				t.Fatalf("parse %s or %s failed", order[i], order[j])
			}
			want := cmpInt(i, j)
			if got := compare(a, b); got != want {
				t.Errorf("compare(%s, %s) = %d, want %d", order[i], order[j], got, want)
			}
		}
	}
	// Build metadata doesn't count; the leading v is optional.
	a, _ := parse("3.6.0+build.7")
	b, _ := parse("v3.6.0")
	if compare(a, b) != 0 {
		t.Error("build metadata changed the order")
	}
}

func TestParseRejects(t *testing.T) {
	for _, v := range []string{"dev", "", "v1", "v1.2", "v1.2.3.4", "v01.2.3", "v1.2.x", "v1.2.3-", "v1.2.3-a..b", "(devel)"} {
		if _, ok := parse(v); ok {
			t.Errorf("parse(%q) accepted", v)
		}
	}
}

// fakeGitHub serves a releases list and counts the requests.
func fakeGitHub(t *testing.T, releases []apiRelease, status int) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/repos/"+Repo+"/releases" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if ua := r.Header.Get("User-Agent"); ua == "" {
			t.Error("no User-Agent")
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(releases)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func checker(srv *httptest.Server, dir string, now time.Time) *Checker {
	c := New(dir)
	c.BaseURL = srv.URL
	c.Now = func() time.Time { return now }
	return c
}

var releases = []apiRelease{
	{Tag: "v3.7.0-rc.1", URL: "https://example.test/rc", Prerelease: true},
	{Tag: "v3.6.1", URL: "https://example.test/361"},
	{Tag: "v3.8.0", URL: "https://example.test/draft", Draft: true},
	{Tag: "v3.6.0", URL: "https://example.test/360"},
	{Tag: "not-a-version", URL: "https://example.test/x"},
}

func TestCheck(t *testing.T) {
	srv, _ := fakeGitHub(t, releases, http.StatusOK)
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name, current, latest string
		newer                 bool
	}{
		{"older release sees the newest release", "v3.6.0", "v3.6.1", true},
		{"up to date", "v3.6.1", "v3.6.1", false},
		{"newer than every release (a local tag)", "v3.9.0", "v3.6.1", false},
		{"a pre-release also sees pre-releases", "v3.7.0-beta.1", "v3.7.0-rc.1", true},
		{"no v prefix", "3.6.0", "v3.6.1", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := checker(srv, "", now).Check(context.Background(), tt.current, true)
			if err != nil {
				t.Fatal(err)
			}
			if r.Latest == nil || r.Latest.Version != tt.latest || r.Newer != tt.newer {
				t.Errorf("got latest %+v newer %v, want %s newer %v", r.Latest, r.Newer, tt.latest, tt.newer)
			}
		})
	}
}

func TestCheckSkipsDevBuilds(t *testing.T) {
	srv, hits := fakeGitHub(t, releases, http.StatusOK)
	r, err := checker(srv, "", time.Now()).Check(context.Background(), "dev", true)
	if err != nil || r.Skipped != "dev" || hits.Load() != 0 {
		t.Errorf("dev build: %+v, %v, %d requests", r, err, hits.Load())
	}
}

func TestCheckCachesADay(t *testing.T) {
	srv, hits := fakeGitHub(t, releases, http.StatusOK)
	dir := t.TempDir()
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

	if _, err := checker(srv, dir, now).Check(context.Background(), "v3.6.0", false); err != nil {
		t.Fatal(err)
	}
	r, err := checker(srv, dir, now.Add(23*time.Hour)).Check(context.Background(), "v3.6.0", false)
	if err != nil || hits.Load() != 1 || !r.Newer {
		t.Errorf("within a day: %d requests, %+v, %v; want 1 and the cached answer", hits.Load(), r, err)
	}
	// Forced, older than a day, or another version: ask again.
	_, _ = checker(srv, dir, now.Add(time.Hour)).Check(context.Background(), "v3.6.0", true)
	_, _ = checker(srv, dir, now.Add(26*time.Hour)).Check(context.Background(), "v3.6.0", false) // a day after the forced one
	_, _ = checker(srv, dir, now.Add(26*time.Hour)).Check(context.Background(), "v3.6.1", false)
	if hits.Load() != 4 {
		t.Errorf("requests = %d, want 4", hits.Load())
	}
	if _, err := filepath.Glob(filepath.Join(dir, "update-check.json")); err != nil {
		t.Fatal(err)
	}
}

func TestCheckErrors(t *testing.T) {
	srv, _ := fakeGitHub(t, nil, http.StatusForbidden) // rate limited
	if _, err := checker(srv, "", time.Now()).Check(context.Background(), "v3.6.0", true); err == nil {
		t.Error("a 403 was not an error")
	}

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	defer slow.Close()
	c := checker(slow, "", time.Now())
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	if _, err := c.Check(ctx, "v3.6.0", true); err == nil {
		t.Error("a server that never answers was not an error")
	}
	if time.Since(start) > 2*time.Second {
		t.Error("the check didn't give up in time")
	}
}
