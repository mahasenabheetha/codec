package workspace

import "testing"

func TestIgnore(t *testing.T) {
	gitignore := `# build output
build/
*.log
!keep.log
/root-only.txt
docs/*.tmp
**/cache
secrets/**
a/**/z.yaml
\#hash
trailing.txt
[Tt]emp?
`
	f := parseIgnore("", []byte(gitignore), false)

	tests := []struct {
		path  string
		isDir bool
		want  bool
	}{
		{"build", true, true},
		{"build", false, false}, // dir-only rule
		{"src/build", true, true},
		{"app.log", false, true},
		{"deep/nested/app.log", false, true},
		{"keep.log", false, false}, // negated
		{"root-only.txt", false, true},
		{"sub/root-only.txt", false, false}, // anchored
		{"docs/a.tmp", false, true},
		{"docs/sub/a.tmp", false, false}, // * doesn't cross /
		{"x/y/cache", true, true},
		{"secrets/a/b.yaml", false, true},
		{"a/z.yaml", false, true},
		{"a/b/c/z.yaml", false, true},
		{"#hash", false, true},
		{"trailing.txt", false, true},
		{"Temp1", false, true},
		{"temp12", false, false},
		{"values.yaml", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got, _ := f.match(tt.path, tt.isDir)
			if got != tt.want {
				t.Errorf("match(%q, dir=%v) = %v, want %v", tt.path, tt.isDir, got, tt.want)
			}
		})
	}
}

func TestIgnoreNestedAndCase(t *testing.T) {
	sub := parseIgnore("charts/app", []byte("*.tgz\n/local.yaml\n"), true)
	for p, want := range map[string]bool{
		"charts/app/dep.TGZ":        true, // case-folded
		"charts/app/sub/dep.tgz":    true,
		"charts/app/local.yaml":     true,
		"charts/app/sub/local.yaml": false,
	} {
		if got, _ := sub.match(p, false); got != want {
			t.Errorf("match(%q) = %v, want %v", p, got, want)
		}
	}
}
