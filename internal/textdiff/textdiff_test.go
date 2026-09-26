package textdiff

import (
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// apply replays an edit script, checking it against both inputs.
func apply(t *testing.T, a, b []string, edits []Edit) {
	t.Helper()
	var gotA, gotB []string
	for _, e := range edits {
		if e.Op != Insert {
			gotA = append(gotA, e.Text)
		}
		if e.Op != Delete {
			gotB = append(gotB, e.Text)
		}
	}
	if strings.Join(gotA, "") != strings.Join(a, "") || strings.Join(gotB, "") != strings.Join(b, "") {
		t.Fatalf("edit script doesn't reproduce the inputs:\na=%q\nb=%q\nedits=%v", a, b, edits)
	}
}

func count(edits []Edit) int {
	n := 0
	for _, e := range edits {
		if e.Op != Equal {
			n++
		}
	}
	return n
}

func TestLinesMinimal(t *testing.T) {
	tests := []struct {
		a, b string
		want int // number of inserted + deleted lines in a shortest script
	}{
		{"", "", 0},
		{"a\nb\nc\n", "a\nb\nc\n", 0},
		{"", "a\n", 1},
		{"a\n", "", 1},
		{"a\nb\nc\n", "a\nc\n", 1},
		{"a\nb\nc\n", "a\nx\nc\n", 2},
		{"a\nb\nc\na\nb\nb\na\n", "c\nb\na\nb\na\nc\n", 5}, // the paper's example
		{"a\nb", "a\nb\n", 2},                              // newline at EOF counts
	}
	for _, tt := range tests {
		a, b := SplitLines(tt.a), SplitLines(tt.b)
		edits := Lines(a, b)
		apply(t, a, b, edits)
		if got := count(edits); got != tt.want {
			t.Errorf("%q -> %q: %d edits, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestLinesRandom(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	words := []string{"a\n", "b\n", "c\n", "d\n"}
	gen := func() []string {
		out := make([]string, r.IntN(30))
		for i := range out {
			out[i] = words[r.IntN(len(words))]
		}
		return out
	}
	for range 500 {
		a, b := gen(), gen()
		apply(t, a, b, Lines(a, b))
	}
}

// TestUnifiedWithGitApply checks the acceptance criterion directly: the
// patch applies cleanly with git apply and yields the edited text.
func TestUnifiedWithGitApply(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	long := strings.Repeat("line\n", 40)
	tests := map[string][2]string{
		"change one line":      {"a: 1\nb: 2\nc: 3\n", "a: 1\nb: 20\nc: 3\n"},
		"insert at start":      {"a: 1\nb: 2\n", "x: 0\na: 1\nb: 2\n"},
		"append at end":        {"a: 1\n", "a: 1\nb: 2\n"},
		"delete everything":    {"a: 1\nb: 2\n", ""},
		"from empty":           {"", "a: 1\n"},
		"no newline at EOF":    {"a: 1\nb: 2", "a: 1\nb: 3"},
		"add newline at EOF":   {"a: 1", "a: 1\n"},
		"two distant hunks":    {"top\n" + long + "bottom\n", "TOP\n" + long + "BOTTOM\n"},
		"crlf file":            {"a: 1\r\nb: 2\r\n", "a: 1\r\nb: 3\r\nc: 4\r\n"},
		"bom and unicode":      {"\xEF\xBB\xBFname: café\nx: 1\n", "\xEF\xBB\xBFname: café\nx: 2\n"},
		"path with spaces dir": {"k: v\n", "k: w\n"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			rel := "charts/my app/values.yaml"
			file := filepath.Join(dir, filepath.FromSlash(rel))
			os.MkdirAll(filepath.Dir(file), 0o755)
			if err := os.WriteFile(file, []byte(tt[0]), 0o644); err != nil {
				t.Fatal(err)
			}
			patch := Unified(rel, tt[0], tt[1], 3)
			if patch == "" {
				t.Fatal("empty patch")
			}
			os.WriteFile(filepath.Join(dir, "p.diff"), []byte(patch), 0o644)
			cmd := exec.Command("git", "apply", "p.diff")
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git apply failed: %v\n%s\npatch:\n%s", err, out, patch)
			}
			got, _ := os.ReadFile(file)
			if string(got) != tt[1] {
				t.Errorf("after apply = %q, want %q", got, tt[1])
			}
		})
	}
	if Unified("x", "same\n", "same\n", 3) != "" {
		t.Error("equal texts must give an empty diff")
	}
}

// TestPatchKeepsFileEndings: the editor sends LF text without a BOM;
// the patch must still apply to the real file and change only what
// was edited.
func TestPatchKeepsFileEndings(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	tests := map[string]struct{ disk, edited, want string }{
		"crlf file":        {"a: 1\r\nb: 2\r\n", "a: 1\nb: 3\nc: 4\n", "a: 1\r\nb: 3\r\nc: 4\r\n"},
		"mixed endings":    {"a: 1\r\nb: 2\nc: 3\r\n", "a: 1\nb: 2\nc: 30\n", "a: 1\r\nb: 2\nc: 30\r\n"},
		"bom kept":         {"\xEF\xBB\xBFa: 1\nb: 2\n", "a: 1\nb: 5\n", "\xEF\xBB\xBFa: 1\nb: 5\n"},
		"first line edits": {"\xEF\xBB\xBFa: 1\r\n", "a: 2\n", "\xEF\xBB\xBFa: 2\r\n"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			os.WriteFile(filepath.Join(dir, "f.yaml"), []byte(tt.disk), 0o644)
			patch := Patch("f.yaml", []byte(tt.disk), tt.edited)
			os.WriteFile(filepath.Join(dir, "p.diff"), []byte(patch), 0o644)
			cmd := exec.Command("git", "apply", "p.diff")
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git apply: %v\n%s\n%q", err, out, patch)
			}
			if got, _ := os.ReadFile(filepath.Join(dir, "f.yaml")); string(got) != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
	if p := Patch("f.yaml", []byte("a: 1\r\n"), "a: 1\n"); p != "" {
		t.Errorf("line endings alone are not an edit, got %q", p)
	}
}
