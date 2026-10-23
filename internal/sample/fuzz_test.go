package sample

import (
	"context"
	"io/fs"
	"strings"
	"testing"

	"github.com/mahasenabheetha/codec/v2/internal/argo"
	"github.com/mahasenabheetha/codec/v2/internal/check"
	"github.com/mahasenabheetha/codec/v2/internal/ci"
	"github.com/mahasenabheetha/codec/v2/internal/config"
	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// FuzzWorkbench runs what the editor runs on a file — detection, every
// analysis and lint rule, outline, hover, definition and completion —
// on mutations of the sample's files, each under its own path so its
// type is detected as in the sample. Schemas are off: no network.
func FuzzWorkbench(f *testing.F) {
	src := Files()
	var paths []string
	fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && (strings.HasSuffix(p, ".yaml") || strings.HasSuffix(p, ".yml")) {
			data, _ := fs.ReadFile(src, p)
			f.Add(uint8(len(paths)), data, uint16(3), uint16(5))
			paths = append(paths, p)
		}
		return nil
	})
	chk := check.New(provider.Default, config.Lint{NoSchemas: true, Offline: true}, "test")
	f.Fuzz(func(t *testing.T, which uint8, data []byte, line, col uint16) {
		p := paths[int(which)%len(paths)]
		file := &provider.File{Path: p, Content: data, YAML: yamlkit.Parse(data)}
		a := chk.Analyze(context.Background(), file, 0)
		for _, d := range a.Diagnostics {
			if d.Range.Start.Offset < 0 || d.Range.End.Offset > len(data) || d.Range.End.Offset < d.Range.Start.Offset {
				t.Fatalf("%s: diagnostic %q at %d-%d of %d bytes", p, d.Code, d.Range.Start.Offset, d.Range.End.Offset, len(data))
			}
		}
		if pv := provider.Default.Lookup(a.Type); pv != nil {
			lines := strings.Count(string(data), "\n") + 1
			pos := yamlkit.PosAt(data, int(line)%lines+1, int(col)%120+1)
			provider.HoverAt(pv, file, pos)
			provider.DefinitionAt(pv, file, pos)
			provider.CompleteAt(pv, file, pos)
		}
		switch a.Type {
		case "argo-workflows":
			argo.Read(p, data, file.YAML)
		case "github-actions", "gitlab-ci", "azure-pipelines":
			ci.Diagnose(ci.Analyze(a.Type, p, data, file.YAML, ci.Options{}), p)
			ci.Expressions(a.Type, data, file.YAML)
		}
	})
}
