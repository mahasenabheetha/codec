package search

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

var files = map[string]string{
	"README.md":                          "# Shop\nThe shop service.\n",
	"charts/shop/values.yaml":            "image: shop:1.2 # Shop image\nreplicas: 2\n",
	"charts/shop/tests/test.yaml":        "shop: test\n",
	"k8s/deploy.yaml":                    "name: shopping\r\nimage: shop:1.2\r\n",
	"bin/tool.yaml":                      "shop\x00binary",
	"docs/naïve.md":                      "naïve 😀 shop\n",
	"scripts/run.sh":                     "echo SHOP\n",
	"charts/other/templates/shop-ui.tpl": "{{ .Values.shop }}\n",
}

func read(p string) ([]byte, error) {
	if s, ok := files[p]; ok {
		return []byte(s), nil
	}
	return nil, errors.New("gone")
}

func run(t *testing.T, o Options) Result {
	t.Helper()
	s, err := Compile(o)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for p := range files {
		paths = append(paths, p)
	}
	res, err := s.Run(context.Background(), append(paths, "missing.yaml"), read)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func where(r Result) string {
	var out []string
	for _, m := range r.Matches {
		out = append(out, fmt.Sprintf("%s:%d", m.Path, m.Line))
	}
	return strings.Join(out, " ")
}

func TestOptions(t *testing.T) {
	tests := []struct {
		name string
		o    Options
		want string
	}{
		{"case-insensitive by default, binaries skipped", Options{Query: "shop"},
			"README.md:1 README.md:2 charts/other/templates/shop-ui.tpl:1 charts/shop/tests/test.yaml:1 charts/shop/values.yaml:1 docs/naïve.md:1 k8s/deploy.yaml:1 k8s/deploy.yaml:2 scripts/run.sh:1"},
		{"match case", Options{Query: "Shop", MatchCase: true}, "README.md:1 charts/shop/values.yaml:1"},
		{"whole word", Options{Query: "shop", WholeWord: true, Globs: []string{"k8s/**"}}, "k8s/deploy.yaml:2"},
		{"regex", Options{Query: `shop:\d+\.\d+`, Regex: true}, "charts/shop/values.yaml:1 k8s/deploy.yaml:2"},
		{"literal by default", Options{Query: `shop:\d`}, ""},
		{"include and exclude", Options{Query: "shop", Globs: []string{"charts/**", "!**/tests/**"}},
			"charts/other/templates/shop-ui.tpl:1 charts/shop/values.yaml:1"},
		{"name glob at any depth", Options{Query: "shop", Globs: []string{"*.sh"}}, "scripts/run.sh:1"},
		{"folder name excludes its contents", Options{Query: "shop", Globs: []string{"!tests", "!charts/other", "*.yaml"}},
			"charts/shop/values.yaml:1 k8s/deploy.yaml:1 k8s/deploy.yaml:2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := where(run(t, tt.o)); got != tt.want {
				t.Errorf("got  %s\nwant %s", got, tt.want)
			}
		})
	}
}

func TestSpansAndColumns(t *testing.T) {
	r := run(t, Options{Query: "shop", Globs: []string{"docs/**"}})
	m := r.Matches[0]
	// "naïve 😀 " is 9 UTF-16 units: ï is one, the emoji two.
	if m.Col != 10 || len(m.Spans) != 1 || m.Spans[0] != (Span{9, 13}) {
		t.Errorf("match = %+v", m)
	}
	r = run(t, Options{Query: "1.2", Globs: []string{"k8s/**"}})
	if m := r.Matches[0]; m.Text != "image: shop:1.2" || m.Spans[0] != (Span{12, 15}) {
		t.Errorf("CRLF line = %+v", m)
	}
}

func TestLongLineWindow(t *testing.T) {
	s, _ := Compile(Options{Query: "needle"})
	line := strings.Repeat("a", 1000) + "needle" + strings.Repeat("b", 1000)
	m := s.File("min.js", []byte(line), 10)[0]
	if !strings.HasPrefix(m.Text, "…") || !strings.HasSuffix(m.Text, "…") || len(m.Text) > maxLine+8 {
		t.Fatalf("window = %q", m.Text)
	}
	if got := m.Text[len("…"):][m.Spans[0].Start-1 : m.Spans[0].End-1]; got != "needle" || m.Col != 1001 {
		t.Errorf("span %q, col %d", got, m.Col)
	}
}

func TestLimit(t *testing.T) {
	r := run(t, Options{Query: "shop", Limit: 3})
	if len(r.Matches) != 3 || !r.Truncated || where(r) != "README.md:1 README.md:2 charts/other/templates/shop-ui.tpl:1" {
		t.Errorf("limited = %s truncated=%v", where(r), r.Truncated)
	}
	if r := run(t, Options{Query: "replicas"}); r.Truncated || r.Files != 1 {
		t.Errorf("unlimited = %+v", r)
	}
}

func TestCompileErrors(t *testing.T) {
	if _, err := Compile(Options{Query: "  "}); !errors.Is(err, ErrEmpty) {
		t.Errorf("blank: %v", err)
	}
	if _, err := Compile(Options{Query: "(", Regex: true}); err == nil {
		t.Error("bad regex: no error")
	}
}
