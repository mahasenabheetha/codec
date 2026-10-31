// Package update finds out whether a newer codec has been released
// (decision 85). It asks the GitHub releases API, at most once a day,
// and only when asked to: the check is off unless switched on, and a
// development build never checks. Updating stays the user's own step
// (the installer run over the top, or a new archive).
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Repo is codec's GitHub repository.
const Repo = "mahasenabheetha/codec"

// Timeout bounds one check; a slow or absent network never holds codec up.
const Timeout = 5 * time.Second

// MaxAge is how long a check's answer is reused before asking again.
const MaxAge = 24 * time.Hour

// Release is a published codec release.
type Release struct {
	Version    string    `json:"version"` // as tagged, e.g. "v3.6.0"
	URL        string    `json:"url"`     // its release page
	Published  time.Time `json:"published"`
	Prerelease bool      `json:"prerelease,omitempty"`
}

// Result is the answer to a check.
type Result struct {
	Current string `json:"current"`
	// Latest is the newest release this build would update to: releases
	// only, or pre-releases too when the running build is one.
	Latest *Release `json:"latest,omitempty"`
	// Newer is true when Latest is newer than Current.
	Newer   bool      `json:"newer"`
	Checked time.Time `json:"checked,omitzero"`
	// Skipped says why no check was made: "dev" for a development build.
	Skipped string `json:"skipped,omitempty"`
}

// Checker checks for releases. The zero value is not ready; use New.
type Checker struct {
	Client    *http.Client
	BaseURL   string // the GitHub API, replaced in tests
	CacheFile string // "" keeps no cache
	Now       func() time.Time
}

// New returns a Checker that caches its answer in dir ("" for none).
func New(dir string) *Checker {
	c := &Checker{
		Client:  &http.Client{Timeout: Timeout},
		BaseURL: "https://api.github.com",
		Now:     time.Now,
	}
	if dir != "" {
		c.CacheFile = filepath.Join(dir, "update-check.json")
	}
	return c
}

// Check finds the newest release for the running version current. An
// answer younger than MaxAge for the same version is reused unless
// force is set.
func (c *Checker) Check(ctx context.Context, current string, force bool) (Result, error) {
	cur, ok := parse(current)
	if !ok {
		return Result{Current: current, Skipped: "dev"}, nil
	}
	if !force {
		if r, ok := c.cached(current); ok {
			return r, nil
		}
	}
	releases, err := c.fetch(ctx, current)
	if err != nil {
		return Result{Current: current}, err
	}
	r := Result{Current: current, Checked: c.Now().UTC()}
	var best semver
	for _, rel := range releases {
		v, ok := parse(rel.Version)
		if !ok || (rel.Prerelease && len(cur.pre) == 0) {
			continue
		}
		if r.Latest == nil || compare(v, best) > 0 {
			rel := rel
			r.Latest, best = &rel, v
		}
	}
	r.Newer = r.Latest != nil && compare(best, cur) > 0
	c.save(r)
	return r, nil
}

// apiRelease is the part of GitHub's release object codec reads.
type apiRelease struct {
	Tag        string    `json:"tag_name"`
	URL        string    `json:"html_url"`
	Draft      bool      `json:"draft"`
	Prerelease bool      `json:"prerelease"`
	Published  time.Time `json:"published_at"`
}

func (c *Checker) fetch(ctx context.Context, current string) ([]Release, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/repos/"+Repo+"/releases?per_page=20", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "codec/"+current)
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("checking for a new release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checking for a new release: GitHub answered %s", resp.Status)
	}
	var list []apiRelease
	if err := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 4<<20)).Decode(&list); err != nil {
		return nil, fmt.Errorf("checking for a new release: reading GitHub's answer: %w", err)
	}
	out := make([]Release, 0, len(list))
	for _, a := range list {
		if a.Draft {
			continue
		}
		out = append(out, Release{Version: a.Tag, URL: a.URL, Published: a.Published, Prerelease: a.Prerelease})
	}
	return out, nil
}

// Cached is the day's answer for current without asking anyone: ok is
// false when there is none (never checked, too old, or another version).
func (c *Checker) Cached(current string) (Result, bool) {
	return c.cached(current)
}

// cached returns the last answer when it is fresh and for this version.
func (c *Checker) cached(current string) (Result, bool) {
	if c.CacheFile == "" {
		return Result{}, false
	}
	b, err := os.ReadFile(c.CacheFile)
	if err != nil {
		return Result{}, false
	}
	var r Result
	if json.Unmarshal(b, &r) != nil || r.Current != current {
		return Result{}, false
	}
	age := c.Now().Sub(r.Checked)
	if age < 0 || age > MaxAge {
		return Result{}, false
	}
	return r, true
}

// save keeps the answer for the next day; failing to is not an error.
func (c *Checker) save(r Result) {
	if c.CacheFile == "" {
		return
	}
	b, err := json.Marshal(r)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(c.CacheFile), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(c.CacheFile, b, 0o644)
}
