package ansible

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

func editorFile(t *testing.T, p string) *provider.File {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", filepath.FromSlash(p)))
	if err != nil {
		t.Fatal(err)
	}
	return &provider.File{Path: p, Content: b, YAML: yamlkit.Parse(b)}
}

func posOf(t *testing.T, f *provider.File, line int, needle string) yamlkit.Pos {
	t.Helper()
	l := strings.Split(string(f.Content), "\n")[line-1]
	c := strings.Index(l, needle)
	if c < 0 {
		t.Fatalf("%q not on line %d: %q", needle, line, l)
	}
	return yamlkit.PosAt(f.Content, line, c+2)
}

func TestOutline(t *testing.T) {
	f := editorFile(t, "repo/playbooks/site.yml")
	var b strings.Builder
	var walk func(ss []provider.Symbol, depth int)
	walk = func(ss []provider.Symbol, depth int) {
		for _, s := range ss {
			fmt.Fprintf(&b, "%s%s · %s\n", strings.Repeat("  ", depth), s.Name, s.Detail)
			walk(s.Children, depth+1)
		}
	}
	walk(Provider{}.Symbols(f.YAML.Docs[0]), 0)
	want := `Play Web servers · hosts web
  pre_tasks · 1 step
    Task Update cache · apt
  roles · 2 steps
    Role common · role
    Role nginx · role
  tasks · 3 steps
    Task Deploy app {{ app_version }} · template
    Task import_tasks tasks/extra.yml · tasks/extra.yml
    Block · block
      Task Try it · command
      rescue · 
        Task Recover · debug
  post_tasks · 1 step
    Task Done · debug · when app_port is defined
  handlers · 1 handler
    Task restart app · service
import_playbook db.yml · 
`
	if b.String() != want {
		t.Errorf("outline:\n%s\nwant:\n%s", b.String(), want)
	}
}

func TestEditor(t *testing.T) {
	ws := workspace(t)
	site := editorFile(t, "repo/playbooks/site.yml")
	common := editorFile(t, "repo/roles/common/tasks/main.yml")
	for _, tc := range []struct {
		f      *provider.File
		line   int
		needle string
		title  string
		row    string // one expected "Label: value"
		def    string // first definition "path:line"
	}{
		{site, 9, "app_version", "app_version · variable", "Wins: group_vars all (all): 2.0 · repo/group_vars/all.yml:1", "repo/group_vars/all.yml:1"},
		{site, 33, "app_port", "app_port · variable", "Wins: play vars (Web servers): 8080 · line 5", "repo/playbooks/site.yml:5"},
		{site, 32, "inventory_hostname", "inventory_hostname · variable", "Provided by Ansible: the host's name in the inventory", "none"},
		{site, 23, "nginx", "Role nginx", "Depends on: common (run first)", "repo/roles/nginx/tasks/main.yml:1"},
		{site, 28, "web restart", "Handler web restart", "Defined at: repo/roles/nginx/handlers/main.yml:1", "repo/roles/nginx/handlers/main.yml:1"},
		{site, 13, "restart app", "Handler restart app", "Defined at: line 35", "repo/playbooks/site.yml:35"},
		{site, 14, "tasks/extra", "", "", "repo/playbooks/tasks/extra.yml:1"},
		{site, 7, "../vars", "", "", "repo/vars/common.yml:1"},
		{common, 5, "users.yml", "", "", "repo/roles/common/tasks/users.yml:1"},
		{common, 3, "item", "item · variable", "Provided by Ansible: the current loop item", "none"},
	} {
		pos := posOf(t, tc.f, tc.line, tc.needle)
		h := HoverIn(tc.f, pos, ws)
		switch {
		case tc.title == "" && h != nil:
			t.Errorf("line %d %s: unexpected hover %+v", tc.line, tc.needle, h)
		case tc.title != "" && (h == nil || h.Title != tc.title):
			t.Errorf("line %d %s: hover %+v, want %s", tc.line, tc.needle, h, tc.title)
		case tc.row != "":
			var rows []string
			for _, r := range h.Rows {
				rows = append(rows, r.Label+": "+r.Value)
			}
			if !strings.Contains(strings.Join(rows, "|"), tc.row) {
				t.Errorf("line %d %s: rows %q lack %q", tc.line, tc.needle, rows, tc.row)
			}
		}
		got := "none"
		if locs := DefinitionIn(tc.f, pos, ws); len(locs) > 0 {
			p := locs[0].Path
			if p == "" {
				p = tc.f.Path
			}
			got = fmt.Sprintf("%s:%d", p, locs[0].Range.Start.Line)
		}
		if got != tc.def {
			t.Errorf("line %d %s: definition %s, want %s", tc.line, tc.needle, got, tc.def)
		}
	}
}

func TestInventoryOutline(t *testing.T) {
	f := editorFile(t, "repo/inventory/hosts.yml")
	var names []string
	var walk func(ss []provider.Symbol)
	walk = func(ss []provider.Symbol) {
		for _, s := range ss {
			if strings.HasPrefix(s.Name, "Group") || strings.HasPrefix(s.Name, "Host") {
				names = append(names, strings.TrimSpace(s.Name+" "+s.Detail))
			}
			walk(s.Children)
		}
	}
	walk(InventoryProvider{}.Symbols(f.YAML.Docs[0]))
	want := "Group all|Group web|Host web[01:03].example.com 3 hosts|Group db|Host db1|Group prod|Group web|Group db"
	if got := strings.Join(names, "|"); got != want {
		t.Errorf("inventory outline = %s", got)
	}
}
