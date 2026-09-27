package compose

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// workspace backs editor features with a testdata folder.
func workspace(t *testing.T) Options {
	t.Helper()
	var all []string
	filepath.WalkDir("testdata", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel("testdata", p)
			all = append(all, filepath.ToSlash(rel))
		}
		return nil
	})
	return Options{Files: all, Load: func(p string) ([]byte, error) { return os.ReadFile(filepath.Join("testdata", filepath.FromSlash(p))) }}
}

func editorFile(t *testing.T, p string) *provider.File {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", filepath.FromSlash(p)))
	if err != nil {
		t.Fatal(err)
	}
	return &provider.File{Path: p, Content: b, YAML: yamlkit.Parse(b)}
}

// posOf is the position of the first occurrence of needle on line.
func posOf(t *testing.T, f *provider.File, line int, needle string) yamlkit.Pos {
	t.Helper()
	l := strings.Split(string(f.Content), "\n")[line-1]
	c := strings.Index(l, needle)
	if c < 0 {
		t.Fatalf("%q not on line %d: %q", needle, line, l)
	}
	return yamlkit.PosAt(f.Content, line, c+2)
}

func TestEditor(t *testing.T) {
	ws := workspace(t)
	f := editorFile(t, "app/compose.yaml")
	for _, tc := range []struct {
		line       int
		needle     string
		title, row string // hover title and one expected "Label: value"
		defPath    string
		defLine    int
	}{
		{25, "${PG_VERSION", "${PG_VERSION} · Compose variable", "Value: 15", "app/.env", 2},
		{16, "${API_PORT", "${API_PORT} · Compose variable", "Value: 3000 (the default: not set)", "", 0},
		{22, "db", "Service db", "Image: postgres:15", "", 24},
		{37, "db", "Service db", "Health check: CMD true", "", 24},
		{35, "base", "Service base", "", "app/common.yml", 2},
		{20, "front", "Network front", "Used by: web", "", 41},
		{18, "static", "Volume static", "Used by: web", "", 45},
	} {
		pos := posOf(t, f, tc.line, tc.needle)
		h := HoverIn(f, pos, ws)
		if h == nil || h.Title != tc.title {
			t.Errorf("line %d %s: hover %+v, want %s", tc.line, tc.needle, h, tc.title)
			continue
		}
		if tc.row != "" {
			var rows []string
			for _, r := range h.Rows {
				rows = append(rows, r.Label+": "+r.Value)
			}
			if !strings.Contains(strings.Join(rows, "|"), tc.row) {
				t.Errorf("line %d: hover rows %v lack %s", tc.line, rows, tc.row)
			}
		}
		locs := DefinitionIn(f, pos, ws)
		got := "none"
		if len(locs) == 1 {
			got = fmt.Sprintf("%s:%d", locs[0].Path, locs[0].Range.Start.Line)
		}
		want := "none"
		if tc.defLine > 0 {
			want = fmt.Sprintf("%s:%d", tc.defPath, tc.defLine)
		}
		if got != want {
			t.Errorf("line %d %s: definition %s, want %s", tc.line, tc.needle, got, want)
		}
	}
}

func TestOutlineAndStandalone(t *testing.T) {
	f := editorFile(t, "app/compose.yaml")
	syms := Provider{}.Symbols(f.YAML.Docs[0])
	var names []string
	for _, s := range syms {
		for _, c := range s.Children {
			names = append(names, c.Name+" ["+c.Detail+"]")
		}
	}
	got := strings.Join(names, "|")
	for _, want := range []string{"Service web [build ./web · ports 8080→80 · after db]", "Service db [postgres:${PG_VERSION:-16}]", "Network front", "Volume dbdata"} {
		if !strings.Contains(got, want) {
			t.Errorf("outline %s lacks %s", got, want)
		}
	}
	// Alone, an override has no image and refers to nothing it defines;
	// that is fine. Unset variables aren't reported without .env.
	o := editorFile(t, "app/compose.override.yaml")
	if d := (Provider{}).Diagnostics(o); len(d) != 0 {
		t.Errorf("override alone: %+v", d)
	}
	if d := (Provider{}).Diagnostics(f); len(d) != 0 {
		t.Errorf("compose.yaml alone: %+v", d)
	}
	if n := len(Provider{}.Expressions(f)); n != 3 {
		t.Errorf("expressions = %d, want 4", n)
	}
}
