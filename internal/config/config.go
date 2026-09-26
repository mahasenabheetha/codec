// Package config stores codec's own settings (recent folders, UI
// preferences) as one JSON file in the user's profile:
//
//	Windows  %AppData%\codec\settings.json
//	macOS    ~/Library/Application Support/codec/settings.json
//	Linux    ~/.config/codec/settings.json
//
// Settings never live inside a workspace: codec must not leave files in
// the repositories it reads (design/decisions.md #6).
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// MaxRecent is how many recently opened folders are remembered.
const MaxRecent = 10

// Settings is everything codec persists. Add fields with omitempty so
// older settings files keep loading.
type Settings struct {
	RecentFolders []string `json:"recentFolders,omitempty"`
	// Helm holds render profiles per chart, keyed by the chart's
	// absolute path (so they never live in the repository).
	Helm map[string]HelmChart `json:"helm,omitempty"`
	Lint Lint                 `json:"lint,omitzero"`
}

// Lint holds the lint and schema choices. Zero values mean defaults.
type Lint struct {
	Rules      map[string]string `json:"rules,omitempty"`      // rule id → error|warning|info|off
	LineLength int               `json:"lineLength,omitempty"` // 0 = 120
	K8sVersion string            `json:"k8sVersion,omitempty"` // "" = codec's default
	NoSchemas  bool              `json:"noSchemas,omitempty"`  // don't validate against schemas at all
	Offline    bool              `json:"offline,omitempty"`    // only cached/custom schemas, no downloads
	SchemaDir  string            `json:"schemaDir,omitempty"`  // custom schema folder, checked first
}

// HelmChart is the saved state of one chart's Helm view.
type HelmChart struct {
	Profiles map[string]HelmProfile `json:"profiles,omitempty"`
	Active   string                 `json:"active,omitempty"`
}

// HelmProfile is a named way to render a chart, e.g. "prod": ordered
// values files (workspace paths) plus --set overrides.
type HelmProfile struct {
	Values      []string `json:"values,omitempty"`
	Set         []string `json:"set,omitempty"`
	Release     string   `json:"release,omitempty"`
	Namespace   string   `json:"namespace,omitempty"`
	KubeVersion string   `json:"kubeVersion,omitempty"`
}

// Store is the settings file plus an in-memory copy. It is safe for
// concurrent use: HTTP handlers run on many goroutines at once.
type Store struct {
	path string // "" = memory only (no usable profile directory)

	mu sync.Mutex
	s  Settings
}

// Dir returns the settings directory. CODEC_CONFIG_DIR overrides it
// (tests, Docker volumes).
func Dir() (string, error) {
	if d := os.Getenv("CODEC_CONFIG_DIR"); d != "" {
		return d, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "codec"), nil
}

// Open loads the settings file. A missing file is a fresh start. If
// there is no usable profile directory (e.g. a container without
// $HOME) the store still works, in memory only. A corrupt file is
// reported but not fatal: defaults are used and the next save replaces it.
func Open() (*Store, error) {
	dir, err := Dir()
	if err != nil {
		return &Store{}, nil
	}
	st := &Store{path: filepath.Join(dir, "settings.json")}
	data, err := os.ReadFile(st.path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return st, nil
	case err != nil:
		return st, fmt.Errorf("read settings: %w", err)
	}
	if err := json.Unmarshal(data, &st.s); err != nil {
		return st, fmt.Errorf("settings file %s is corrupt, using defaults: %w", st.path, err)
	}
	return st, nil
}

// Get returns a copy of the current settings.
func (st *Store) Get() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	// A JSON round trip is the simplest deep copy of nested maps.
	var s Settings
	data, _ := json.Marshal(st.s)
	_ = json.Unmarshal(data, &s)
	return s
}

// Update changes the settings with fn and saves them.
func (st *Store) Update(fn func(*Settings)) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	fn(&st.s)
	return st.save()
}

// save writes the file atomically: write a temp file next to it, then
// rename over the old one, so a crash never leaves half a file behind.
func (st *Store) save() error {
	if st.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(st.s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(st.path), 0o755); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(st.path), "settings-*.json")
	if err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	defer os.Remove(tmp.Name()) // no-op once renamed
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return fmt.Errorf("save settings: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	if err := os.Rename(tmp.Name(), st.path); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	return nil
}

// AddRecent puts dir first in list, removing an earlier copy, and keeps
// at most MaxRecent entries. Paths compare case-insensitively on
// Windows and macOS, whose filesystems usually are.
func AddRecent(list []string, dir string) []string {
	out := []string{dir}
	for _, p := range list {
		if !samePath(p, dir) && len(out) < MaxRecent {
			out = append(out, p)
		}
	}
	return out
}

// RemoveRecent drops dir from list.
func RemoveRecent(list []string, dir string) []string {
	var out []string
	for _, p := range list {
		if !samePath(p, dir) {
			out = append(out, p)
		}
	}
	return out
}

func samePath(a, b string) bool {
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// Memory returns a store that is never saved (tests, no profile).
func Memory() *Store { return &Store{} }
