package ansible

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// workspace backs the analysis with testdata as the open folder.
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
	return Options{Files: all, Root: "repo", Load: func(p string) ([]byte, error) {
		return os.ReadFile(filepath.Join("testdata", filepath.FromSlash(p)))
	}}
}

func analyze(t *testing.T, file string, opts Options) *Playbook {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", filepath.FromSlash(file)))
	if err != nil {
		t.Fatal(err)
	}
	return Analyze(file, b, yamlkit.Parse(b), opts)
}

// tree prints steps as an indented outline.
func tree(ts []Task, depth int, b *strings.Builder) {
	for _, t := range ts {
		fmt.Fprintf(b, "%s%s [%s]\n", strings.Repeat("  ", depth), t.Name, t.Kind)
		tree(t.Children, depth+1, b)
	}
}

func TestExecutionOrder(t *testing.T) {
	p := analyze(t, "repo/playbooks/site.yml", workspace(t))
	var b strings.Builder
	for _, pl := range p.Plays {
		fmt.Fprintf(&b, "PLAY %s\n", pl.Name)
		tree(pl.Steps, 1, &b)
	}
	// In the file, tasks come before roles and pre_tasks last; Ansible
	// runs facts, pre_tasks, roles, tasks, post_tasks. nginx depends on
	// common, which already ran in this play.
	want := `PLAY Web servers
  Gathering Facts [facts]
  pre_tasks [section]
    Update cache [task]
  Run notified handlers [flush]
  roles [section]
    common [role]
      Install packages [task]
      include_tasks users.yml [include]
        Create users [task]
    nginx [role]
      Install nginx [task]
  tasks [section]
    Deploy app {{ app_version }} [task]
    import_tasks tasks/extra.yml [import]
      Extra step [task]
    block [block]
      Try it [task]
      rescue [section]
        Recover [task]
  Run notified handlers [flush]
  post_tasks [section]
    Done [task]
PLAY db
  tasks [section]
    include_role comon [include]
    Check [task]
    include_role shared [include]
      Shared step [task]
    include_role galaxy_role [include]
  Run notified handlers [flush]
`
	if b.String() != want {
		t.Errorf("steps:\n%s\nwant:\n%s", b.String(), want)
	}
	if pl := p.Plays[1]; pl.Imported != "repo/playbooks/db.yml" {
		t.Errorf("imported = %q", pl.Imported)
	}
	var handlers []string
	for _, h := range p.Plays[0].Handlers {
		handlers = append(handlers, h.Name+"@"+h.Source.File)
	}
	if got := strings.Join(handlers, ","); got != "restart app@repo/playbooks/site.yml,restart nginx@repo/roles/nginx/handlers/main.yml" {
		t.Errorf("handlers = %s", got)
	}
	var probs []string
	for _, pr := range p.Problems {
		probs = append(probs, string(pr.Severity)+" "+pr.Code+" "+pr.Hint)
	}
	// "web restart" is a listen topic of the nginx role's handler.
	wantProbs := `warning ansible-role-missing Did you mean "common"?|` +
		`info ansible-role-elsewhere At run time it must be installed where Ansible looks (roles/, roles_path).|` +
		`info ansible-role-missing Listed in repo/requirements.yml, so it is installed when the playbook runs.|` +
		`warning ansible-handler-unknown `
	if got := strings.Join(probs, "|"); got != wantProbs {
		t.Errorf("problems = %s", got)
	}
}

func TestVariables(t *testing.T) {
	p := analyze(t, "repo/playbooks/site.yml", workspace(t))
	for _, tc := range []struct{ name, want string }{
		// Highest precedence first.
		{"app_port", "8080 play vars; 9090 group_vars"},
		{"app_version", "2.0 group_vars all; 1.0 role default"},
		{"nginx_port", "{{ app_port }} role param; 80 role default"},
		{"users_result", "result of Create users register"},
		{"timezone", "UTC vars_files"},
	} {
		var got []string
		for _, d := range p.Definitions(tc.name) {
			got = append(got, d.Value+" "+d.Kind)
		}
		if strings.Join(got, "; ") != tc.want {
			t.Errorf("%s: %q, want %q", tc.name, strings.Join(got, "; "), tc.want)
		}
	}
}

func TestTaskFile(t *testing.T) {
	p := analyze(t, "repo/roles/common/tasks/main.yml", workspace(t))
	if p.Kind != "tasks" || p.Role != "common" || len(p.Tasks) != 2 || len(p.Tasks[1].Children) != 1 {
		t.Errorf("task file: %+v", p)
	}
	// Alone (no folder), nothing outside the file is reported missing.
	alone := analyze(t, "repo/playbooks/site.yml", Options{})
	if len(alone.Problems) != 0 {
		t.Errorf("alone: %+v", alone.Problems)
	}
}

func TestInventory(t *testing.T) {
	b, _ := os.ReadFile("testdata/repo/inventory/hosts.yml")
	inv := ReadInventory("repo/inventory/hosts.yml", yamlkit.Parse(b))
	var groups []string
	for _, g := range inv.Groups {
		groups = append(groups, fmt.Sprintf("%s hosts=%v children=%v vars=%d", g.Name, g.Hosts, g.Children, len(g.Vars)))
	}
	want := "all hosts=[] children=[web db prod] vars=1|web hosts=[web[01:03].example.com] children=[] vars=0|db hosts=[db1] children=[] vars=1|prod hosts=[] children=[web db] vars=0"
	if got := strings.Join(groups, "|"); got != want {
		t.Errorf("groups:\n%s\nwant\n%s", got, want)
	}
	if h := inv.Hosts[0]; h.Count != 3 || strings.Join(h.Groups, ",") != "web" {
		t.Errorf("host %+v", h)
	}
	if h := inv.Hosts[1]; h.Name != "db1" || len(h.Vars) != 1 || h.Vars[0].Value != "10.0.0.5" {
		t.Errorf("host %+v", h)
	}
}

func TestFindTask(t *testing.T) {
	ws := workspace(t)
	for _, tc := range []struct{ name, path, want string }{
		{"common : Create users", "/builds/team/repo/roles/common/tasks/users.yml:1", "repo/roles/common/tasks/users.yml:1 task path Create users"},
		{"common : Install packages", "", "repo/roles/common/tasks/main.yml:1 exact name Install packages"},
		{"Deploy app 2.0", "", "repo/playbooks/site.yml:9 name with variables Deploy app {{ app_version }}"},
		{"nginx : restart nginx", "", "repo/roles/nginx/handlers/main.yml:1 exact name restart nginx"},
		{"Nothing like it", "", ""},
		// Another role's tasks/main.yml is not this task's file.
		{"app : Run migrations", "/builds/team/repo/roles/app/tasks/main.yml:9", ""},
		// A "roles" folder higher up in the CI path does not count.
		{"common : Create users", "/builds/roles/repo/roles/common/tasks/users.yml:1", "repo/roles/common/tasks/users.yml:1 task path Create users"},
		{"Done", "/srv/roles/repo/playbooks/site.yml:30", "repo/playbooks/site.yml:30 task path Done"},
	} {
		got := ""
		if ms := FindTask(tc.name, tc.path, ws.Files, ws.Load); len(ms) > 0 {
			got = fmt.Sprintf("%s:%d %s %s", ms[0].File, ms[0].Line, ms[0].Why, ms[0].Name)
		}
		if got != tc.want {
			t.Errorf("FindTask(%q, %q) = %q, want %q", tc.name, tc.path, got, tc.want)
		}
	}
}

func TestVarAt(t *testing.T) {
	expr := `{{ item.name | default(fallback) if x is defined else 'lit' }}`
	for _, tc := range []struct {
		at   string
		want string
	}{
		{"item", "item"}, {"name", ""}, {"default", ""}, {"fallback", "fallback"}, {"x ", "x"}, {"defined", ""}, {"lit", ""},
	} {
		i := strings.Index(expr, tc.at)
		name, _, _, _ := VarAt(expr, i)
		if name != tc.want {
			t.Errorf("VarAt(%q) = %q, want %q", tc.at, name, tc.want)
		}
	}
}

func TestNameScore(t *testing.T) {
	for _, tc := range []struct {
		written, logged string
		want            int
	}{
		{"Deploy app {{ v }}", "Deploy app 2.0", 80},
		{"{{ script_name }}", "Deploy SLA config", 0}, // all variables: matches nothing
		{"Run {{ x }}", "Run anything", 0},            // "Run" is too little text
		{"Install nginx", "install NGINX", 70},
	} {
		if got, _ := nameScore(tc.written, tc.logged); got != tc.want {
			t.Errorf("nameScore(%q, %q) = %d, want %d", tc.written, tc.logged, got, tc.want)
		}
	}
}
