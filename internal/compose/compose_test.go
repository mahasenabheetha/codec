package compose

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// fixture analyzes a testdata folder as a workspace.
func fixture(t *testing.T, dir string, files []string, env map[string]string) *Project {
	t.Helper()
	var all []string
	filepath.WalkDir(filepath.Join("testdata", dir), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel("testdata", p)
			all = append(all, filepath.ToSlash(rel))
		}
		return nil
	})
	var paths []string
	for _, f := range files {
		paths = append(paths, path.Join(dir, f))
	}
	return Analyze(paths, Options{Files: all, Env: env, Load: func(p string) ([]byte, error) {
		return os.ReadFile(filepath.Join("testdata", filepath.FromSlash(p)))
	}})
}

func TestDefaultFiles(t *testing.T) {
	ws := []string{"app/compose.yaml", "app/compose.override.yaml", "app/compose.prod.yaml", "old/docker-compose.yml"}
	for _, tc := range []struct {
		file string
		want []string
	}{
		{"app/compose.yaml", []string{"app/compose.yaml", "app/compose.override.yaml"}},
		{"app/compose.override.yaml", []string{"app/compose.yaml", "app/compose.override.yaml"}},
		{"app/compose.prod.yaml", []string{"app/compose.prod.yaml"}},
		{"old/docker-compose.yml", []string{"old/docker-compose.yml"}},
	} {
		if got := DefaultFiles(tc.file, ws); !slices.Equal(got, tc.want) {
			t.Errorf("DefaultFiles(%s) = %v, want %v", tc.file, got, tc.want)
		}
	}
}

func TestInterpolate(t *testing.T) {
	vars := map[string]string{"SET": "x", "EMPTY": ""}
	get := func(n string) (string, bool) { v, ok := vars[n]; return v, ok }
	for _, tc := range []struct{ in, want, err string }{
		{"$SET-${SET}", "x-x", ""},
		{"a$$b", "a$b", ""},
		{"${UNSET:-d}|${EMPTY:-d}|${EMPTY-d}", "d|d|", ""},
		{"${UNSET:-${SET}y}", "xy", ""},
		{"${SET:+alt}|${EMPTY:+alt}|${EMPTY+alt}", "alt||alt", ""},
		{"${UNSET:?need it}", "", "required variable UNSET is missing a value: need it"},
		{"${EMPTY?no}", "", ""},
		{"$ 5 ${", "$ 5 ${", ""},
	} {
		ex := interpolate(tc.in, get)
		if ex.text != tc.want || strings.Join(ex.errs, ";") != tc.err {
			t.Errorf("interpolate(%q) = %q %v, want %q %q", tc.in, ex.text, ex.errs, tc.want, tc.err)
		}
	}
	if r := interpolate("a ${UNSET:-${SET}}", get).refs; len(r) != 2 || r[1].name != "SET" || r[1].start != 11 {
		t.Errorf("nested refs = %+v", r)
	}
}

func TestEnvFile(t *testing.T) {
	env := `# c
A=1
export B = 'lit ${A}'
C="q\n${A}"
D=v # note
bad line
E=${A}${Z}
`
	got := parseEnvFile(env, func(string) (string, bool) { return "", false })
	want := []envEntry{{"A", "1", 2}, {"B", "lit ${A}", 3}, {"C", "q\n1", 4}, {"D", "v", 5}, {"E", "1", 7}}
	if !slices.Equal(got, want) {
		t.Errorf("parseEnvFile = %+v, want %+v", got, want)
	}
}

// line finds a service's effective line and says where it came from.
func line(s Service, text string) string {
	for _, l := range s.Effective {
		if strings.TrimSpace(l.Text) == text {
			if l.Source == nil {
				return "?"
			}
			return strings.TrimSpace(fmt.Sprintf("%s:%d %s", path.Base(l.Source.File), l.Source.Line, l.From))
		}
	}
	return "missing"
}

func service(p *Project, name string) Service {
	for _, s := range p.Services {
		if s.Name == name {
			return s
		}
	}
	return Service{}
}

func TestLayers(t *testing.T) {
	p := fixture(t, "app", []string{"compose.yaml", "compose.override.yaml"}, nil)
	if len(p.Problems) != 0 {
		t.Fatalf("problems: %+v", p.Problems)
	}
	web, db, worker := service(p, "web"), service(p, "db"), service(p, "worker")
	for _, tc := range []struct {
		s          Service
		text, want string
	}{
		{web, "driver: json-file", "compose.yaml:5"},             // from the x-logging anchor
		{web, "command: npm run dev", "compose.override.yaml:3"}, // replaced
		{web, "ports:", "compose.override.yaml:4"},               // a merged key: the override's file and line
		{web, "- 8080:80", "compose.yaml:13"},                    // ports append
		{web, "- 9229:9229", "compose.override.yaml:5"},
		{web, "NODE_ENV: development", "compose.override.yaml:7"},     // list-form environment merged by name
		{web, "API_URL: http://api:3000", "compose.yaml:16"},          // ${API_PORT:-3000}
		{web, "- ./web/conf-dev:/etc/web", "compose.override.yaml:9"}, // same target replaces
		{web, "- ./web/conf:/etc/web:ro", "missing"},
		{db, "image: postgres:15", "compose.yaml:25"}, // PG_VERSION from .env
		{db, "interval: 5s", "missing"},               // !override replaced the healthcheck
		{db, "- \"true\"", "compose.override.yaml:12"},
		{worker, "image: shop/worker:v2", "common.yml:3 extends base"},
		{worker, "- jobs", "compose.yaml:36"},
	} {
		if got := line(tc.s, tc.text); got != tc.want {
			t.Errorf("%s %q: got %s, want %s", tc.s.Name, tc.text, got, tc.want)
		}
	}
	if !slices.Equal(web.Files, []string{"app/compose.yaml", "app/compose.override.yaml"}) || !slices.Equal(worker.Files, []string{"app/compose.yaml", "app/common.yml"}) {
		t.Errorf("files: web %v worker %v", web.Files, worker.Files)
	}
	if len(p.Ports) != 3 || p.Ports[1].Layer != "app/compose.override.yaml" {
		t.Errorf("ports = %+v", p.Ports)
	}
	var edges []string
	for _, e := range p.Edges {
		edges = append(edges, e.From+">"+e.To+":"+e.Label)
	}
	for _, want := range []string{"db>web:healthy", "db>worker:", "web>network:front:", "web>volume:static:/srv/static"} {
		if !slices.Contains(edges, want) {
			t.Errorf("edges %v lack %s", edges, want)
		}
	}
}

func TestVariables(t *testing.T) {
	p := fixture(t, "app", []string{"compose.yaml"}, map[string]string{"PG_VERSION": "17"})
	vars := map[string]Variable{}
	for _, v := range p.Variables {
		vars[v.Name] = v
	}
	if v := vars["PG_VERSION"]; v.Value != "17" || v.From != "what-if" || v.Default != "16" {
		t.Errorf("PG_VERSION = %+v", v)
	}
	if v := vars["TAG"]; v.Value != "v2" || v.From != "app/.env" || v.Defined == nil || v.Defined.Line != 3 {
		t.Errorf("TAG = %+v", v)
	}
	if v := vars["API_PORT"]; v.Set || v.From != "default" {
		t.Errorf("API_PORT = %+v", v)
	}
	if got := line(service(p, "db"), "image: postgres:17"); got != "compose.yaml:25" {
		t.Errorf("what-if image: %s", got)
	}
}

func TestProblems(t *testing.T) {
	p := fixture(t, "broken", []string{"docker-compose.yml"}, nil)
	var got []string
	for _, pr := range p.Problems {
		got = append(got, pr.Code+" "+pr.Hint)
	}
	for _, want := range []string{
		`compose-depends-unknown Did you mean "db"?`,
		"compose-network-undefined ",
		"compose-volume-undefined Declare it, or write a path (./data) for a bind mount.",
		"compose-cycle ",
		"compose-port-clash Only one of them can start, unless they are in different profiles.",
		"compose-var-required Set it in .env or as a what-if value.",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("problems %q lack %q", got, want)
		}
	}
	if !slices.ContainsFunc(got, func(s string) bool { return strings.HasPrefix(s, "compose-var-unset") }) {
		t.Errorf("no unset warning for REDIS_TAG")
	}
}
