package yamlkit

import (
	"bytes"
	"os"
	"testing"
)

// FuzzParseFormat: parsing never fails hard, and reformatting what
// formats is stable (formatting the result again changes nothing).
func FuzzParseFormat(f *testing.F) {
	if b, err := os.ReadFile("testdata/helm-deployment.yaml"); err == nil {
		f.Add(b)
	}
	for _, s := range []string{
		"a: 1\nb: [x, y]\nc:\n  - d: e\n    f: |\n      text\n",
		"---\nkind: X\n...\n---\n# comment\nk: &a v\nl: *a\nm: {n: o}\n",
		"key: \"quoted \\\" value\"\nother: 'single'\nlist:\n- a\n-   b\n",
		"? complex\n: value\n!!str tagged: x\n",
		"a:\n\tb: tab\n",
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, in []byte) {
		for _, d := range Parse(in).Docs {
			checkRanges(t, d.Root, len(in))
		}
		out, err := Format(in, FormatOptions{})
		if err != nil {
			return
		}
		again, err := Format(out, FormatOptions{})
		if err != nil {
			t.Fatalf("formatted text doesn't format: %v\n%q", err, out)
		}
		// yaml.v3 may add a blank line before a trailing comment on the
		// second pass; past that, formatting changes nothing.
		if !bytes.Equal(noBlanks(out), noBlanks(again)) {
			t.Fatalf("format is not stable:\nonce  %q\ntwice %q", out, again)
		}
		if third, _ := Format(again, FormatOptions{}); !bytes.Equal(third, again) {
			t.Fatalf("format keeps changing:\ntwice  %q\nthrice %q", again, third)
		}
	})
}

// checkRanges: every node lies in order within the text.
func checkRanges(t *testing.T, n *Node, size int) {
	if n == nil {
		return
	}
	if r := n.Range; r.Start.Offset < 0 || r.End.Offset < r.Start.Offset || r.End.Offset > size {
		t.Fatalf("node %q at %d-%d of %d bytes", n.Value, r.Start.Offset, r.End.Offset, size)
	}
	for _, p := range n.Pairs {
		checkRanges(t, p.Key, size)
		checkRanges(t, p.Value, size)
	}
	for _, it := range n.Items {
		checkRanges(t, it, size)
	}
}

func noBlanks(b []byte) []byte {
	var out [][]byte
	for _, l := range bytes.Split(b, []byte("\n")) {
		if len(bytes.TrimSpace(l)) > 0 {
			out = append(out, l)
		}
	}
	return bytes.Join(out, []byte("\n"))
}

func FuzzParsePath(f *testing.F) {
	for _, s := range []string{"a.b[0].c", `spec["x.y"]`, "[*].name", "a..b", ""} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		ParsePath(s)
	})
}
