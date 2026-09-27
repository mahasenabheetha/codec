package yamlkit

import (
	"fmt"
	"strings"
	"testing"
)

// TestParseManyDocs parses enough documents to go parallel and checks
// that order, indexes and per-document problems come out as they would
// one by one.
func TestParseManyDocs(t *testing.T) {
	var b strings.Builder
	for i := range 3 * parallelDocs {
		if i > 0 {
			b.WriteString("---\n")
		}
		switch i {
		case 7:
			b.WriteString("kind: Broken\nmetadata: [unclosed\n")
		case 20, 40:
			fmt.Fprintf(&b, "kind: Dup\nmetadata:\n  name: d%d\n  name: again\n", i)
		default:
			fmt.Fprintf(&b, "kind: ConfigMap\nmetadata:\n  name: cm-%d\n", i)
		}
	}
	f := Parse([]byte(b.String()))
	if len(f.Docs) != 3*parallelDocs {
		t.Fatalf("%d docs", len(f.Docs))
	}
	for i, d := range f.Docs {
		if d.Index != i {
			t.Errorf("doc %d has index %d", i, d.Index)
		}
	}
	if got := f.Docs[5].Name; got != "ConfigMap/cm-5" {
		t.Errorf("doc 5 name %q", got)
	}
	var codes []string
	for _, d := range f.Diagnostics {
		codes = append(codes, fmt.Sprintf("%s@%d", d.Code, d.Range.Start.Line))
	}
	// Each duplicate is reported on its second "name:" line.
	want := []string{"syntax@30"}
	for n, line := range strings.Split(b.String(), "\n") {
		if line == "  name: again" {
			want = append(want, fmt.Sprintf("duplicate-key@%d", n+1))
		}
	}
	if strings.Join(codes, " ") != strings.Join(want, " ") {
		t.Errorf("diagnostics %v, want %v", codes, want)
	}
}
