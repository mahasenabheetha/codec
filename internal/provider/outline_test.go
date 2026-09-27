package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

func TestTrimOutline(t *testing.T) {
	sym := func(name string, children ...Symbol) Symbol {
		return Symbol{Name: name, Kind: "map", Children: children}
	}
	deep := []DocSummary{{Index: 0, Name: "a", Symbols: []Symbol{sym("x", sym("y", sym("z", sym("w"))))}}}

	for _, tt := range []struct {
		max       int
		trimmed   bool
		wantNames string // outline of doc 0, depth first
	}{
		{10, false, "x y z w"},
		{3, true, "x y z"},
		{1, true, "x"},
		{0, true, "a"}, // documents only
	} {
		docs := []DocSummary{deep[0]}
		if got := trimOutline(docs, tt.max); got != tt.trimmed {
			t.Errorf("max %d: trimmed %v", tt.max, got)
		}
		var names []string
		var walk func([]Symbol)
		walk = func(s []Symbol) {
			for _, c := range s {
				names = append(names, c.Name)
				walk(c.Children)
			}
		}
		walk(docs[0].Symbols)
		if strings.Join(names, " ") != tt.wantNames {
			t.Errorf("max %d: outline %v, want %s", tt.max, names, tt.wantNames)
		}
	}
	if len(deep[0].Symbols[0].Children) != 1 {
		t.Error("trimming changed the caller's symbols")
	}

	// A big file is flagged by Analyze.
	var b strings.Builder
	for i := range 3000 {
		fmt.Fprintf(&b, "---\nkind: ConfigMap\nmetadata:\n  name: c%d\ndata:\n  a: \"1\"\n  b: \"2\"\n  c: \"3\"\n", i)
	}
	src := []byte(b.String())
	a := Default.Analyze(&File{Path: "big.yaml", Content: src, YAML: yamlkit.Parse(src)})
	if !a.OutlineTrimmed || countSymbols(a.Docs, -1) > maxSymbols {
		t.Errorf("big file: trimmed %v, %d symbols", a.OutlineTrimmed, countSymbols(a.Docs, -1))
	}
}
