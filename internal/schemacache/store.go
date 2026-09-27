// Package schemacache gets JSON Schemas for internal/schema: from a
// custom folder the user chose, from the on-disk cache, or from the
// network, and keeps compiled schemas in memory.
//
// Downloads happen on demand, the first time a file needs a schema;
// after that everything works offline. Proxies are honoured through
// the standard HTTPS_PROXY/NO_PROXY variables.
package schemacache

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/mahasenabheetha/codec/v2/internal/schema"
)

// Errors a caller can explain to the user.
var (
	ErrNotPublished = errors.New("no schema is published for it")
	ErrOffline      = errors.New("not downloaded yet, and offline mode is on")
)

// Options configure a Store.
type Options struct {
	Dir       string // cache folder; "" = <user cache>/codec/schemas
	CustomDir string // checked first; same layout as the cache, or just the file
	Offline   bool   // never touch the network
	UserAgent string
}

const (
	maxAge     = 7 * 24 * time.Hour // re-download moving schemas weekly
	retryAfter = time.Minute        // after a network error
	maxSize    = 32 << 20

	missingSuffix = ".404" // marker file: the URL returned 404
)

// Store loads and compiles schemas. It is safe for concurrent use.
type Store struct {
	client *http.Client

	mu       sync.Mutex
	opts     Options
	compiled map[string]*entry
}

// entry is one compile, shared by everyone who asks for the same URL.
type entry struct {
	done chan struct{}
	sch  *jsonschema.Schema
	err  error
	at   time.Time
}

// New returns a Store.
func New(opts Options) *Store {
	s := &Store{
		client:   &http.Client{Timeout: 30 * time.Second}, // DefaultTransport: proxy from environment
		compiled: map[string]*entry{},
	}
	s.SetOptions(opts)
	return s
}

// SetOptions applies changed settings. Compiled schemas are dropped,
// since a custom folder may now answer differently.
func (s *Store) SetOptions(opts Options) {
	if opts.Dir == "" {
		opts.Dir = DefaultDir()
	}
	if opts.UserAgent == "" {
		opts.UserAgent = "codec"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.opts != opts {
		s.opts = opts
		s.compiled = map[string]*entry{}
	}
}

// Get returns the compiled schema at url, downloading it if needed. It
// waits until ctx ends; the work continues in the background either
// way, so a later call finds it ready.
func (s *Store) Get(ctx context.Context, u string) (*jsonschema.Schema, error) {
	e := s.start(u)
	select {
	case <-e.done:
		return e.sch, e.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// start begins compiling url unless that is done or under way. A
// failure caused by the network is retried after a while.
func (s *Store) start(u string) *entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.compiled[u]; ok {
		select {
		case <-e.done:
			if e.err == nil || errors.Is(e.err, ErrNotPublished) || time.Since(e.at) < retryAfter {
				return e
			}
		default:
			return e
		}
	}
	e := &entry{done: make(chan struct{})}
	s.compiled[u] = e
	go func() {
		e.sch, e.err = schema.Compile(s, u)
		// The compiler wraps our loader's error in a type without
		// Unwrap; unwrap it so callers can use errors.Is.
		if le, ok := errors.AsType[*jsonschema.LoadURLError](e.err); ok {
			e.err = le.Err
		}
		e.at = time.Now()
		close(e.done)
	}()
	return e
}

// Load implements schema.Loader (and jsonschema.URLLoader): custom
// folder, then a fresh cache file, then the network, then a stale
// cache file when the network fails.
func (s *Store) Load(u string) (any, error) {
	s.mu.Lock()
	opts := s.opts
	s.mu.Unlock()

	parts, err := urlParts(u)
	if err != nil {
		return nil, err
	}
	if opts.CustomDir != "" {
		// Try host/a/b/c.json, a/b/c.json, b/c.json and c.json, so the
		// folder may mirror the cache or the catalog, or be flat.
		for i := range parts {
			if v, err := readJSON(filepath.Join(append([]string{opts.CustomDir}, parts[i:]...)...)); err == nil {
				return v, nil
			}
		}
	}
	cached := ""
	if opts.Dir != "" {
		cached = filepath.Join(append([]string{opts.Dir}, parts...)...)
	}
	fresh := func(file string) bool {
		fi, err := os.Stat(file)
		return err == nil && (opts.Offline || time.Since(fi.ModTime()) < maxAge || versioned(u))
	}
	if cached != "" {
		if fresh(cached) {
			if v, err := readJSON(cached); err == nil {
				return v, nil
			}
		}
		// A remembered 404 ("no schema for this CRD"), so offline mode
		// and later runs don't ask again every time.
		if fresh(cached + missingSuffix) {
			return nil, ErrNotPublished
		}
	}
	if opts.Offline {
		if v, err := readJSON(cached); err == nil {
			return v, nil
		}
		return nil, ErrOffline
	}
	data, err := s.download(u, opts.UserAgent)
	if errors.Is(err, ErrNotPublished) && cached != "" {
		writeAtomic(cached+missingSuffix, nil)
	}
	if err != nil {
		if v, rerr := readJSON(cached); rerr == nil {
			return v, nil // stale beats nothing
		}
		return nil, err
	}
	v, err := jsonschema.UnmarshalJSON(strings.NewReader(string(data)))
	if err != nil {
		return nil, fmt.Errorf("%s: not valid JSON: %w", u, err)
	}
	if cached != "" {
		writeAtomic(cached, data) // best effort: a read-only cache still works online
	}
	return v, nil
}

func (s *Store) download(u, agent string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", agent)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, ErrNotPublished
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("download failed: %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxSize))
}

// versioned reports whether a URL names an immutable schema (one per
// Kubernetes release), so a cached copy never needs refreshing.
func versioned(u string) bool {
	return strings.HasPrefix(u, schema.KubernetesBase)
}

// urlParts maps a URL to safe path components: host, then its path.
func urlParts(u string) ([]string, error) {
	pu, err := url.Parse(u)
	if err != nil || pu.Host == "" || (pu.Scheme != "https" && pu.Scheme != "http") {
		return nil, fmt.Errorf("unsupported schema URL %q", u)
	}
	parts := []string{sanitize(pu.Host)}
	for _, p := range strings.Split(path.Clean("/"+pu.Path), "/") {
		if p != "" {
			parts = append(parts, sanitize(p))
		}
	}
	return parts, nil
}

// sanitize keeps a URL component usable as a file name on every OS.
func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"\|?*`, r) || r < 32 {
			return '_'
		}
		return r
	}, s)
}

func readJSON(file string) (any, error) {
	if file == "" {
		return nil, os.ErrNotExist
	}
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return jsonschema.UnmarshalJSON(f)
}

// writeAtomic writes via a temp file so a crash never leaves half a schema.
func writeAtomic(file string, data []byte) {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), ".dl-*")
	if err != nil {
		return
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Rename(tmp.Name(), file) != nil {
		os.Remove(tmp.Name())
	}
}

// DefaultDir is the schema cache: codec/schemas in the user cache
// directory ("" if the system has none, which turns caching off).
func DefaultDir() string {
	d, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(d, "codec", "schemas")
}
