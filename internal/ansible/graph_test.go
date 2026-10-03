package ansible

import (
	"fmt"
	"strings"
	"testing"
)

// drawn lists a graph's nodes and edges, one per line.
func drawn(g *Graph) string {
	var out []string
	for _, n := range g.Nodes {
		s := fmt.Sprintf("node %s %q %s tasks=%d", n.ID, n.Label, n.Sub, n.Tasks)
		for _, f := range []struct {
			on   bool
			name string
		}{{n.External, "external"}, {n.Missing, "missing"}, {n.Assumed, "assumed"}} {
			if f.on {
				s += " " + f.name
			}
		}
		if len(n.Candidates) > 0 {
			s += " candidates=" + strings.Join(n.Candidates, ",")
		}
		out = append(out, s)
	}
	for _, e := range g.Edges {
		s := fmt.Sprintf("edge %s -%s-> %s", e.From, e.Kind, e.To)
		if e.Dynamic {
			s += " dashed"
		}
		out = append(out, s)
	}
	return strings.Join(out, "\n")
}

func TestGraphSite(t *testing.T) {
	g := BuildGraph(analyze(t, "repo/playbooks/site.yml", workspace(t)))
	want := strings.Join([]string{
		`node play:0 "Web servers" play · hosts: web tasks=5`,
		`node handler:restart nginx "restart nginx" handler tasks=0`,
		`node role:common "common" role tasks=1`,
		`node tasks:repo/roles/common/tasks/users.yml "users.yml" task file tasks=1`,
		`node role:nginx "nginx" role tasks=1`,
		`node handler:restart app "restart app" handler tasks=0`,
		`node tasks:repo/playbooks/tasks/extra.yml "extra.yml" task file tasks=1`,
		`node play:1 "db" play · hosts: db tasks=1`,
		`node role:comon "comon" role tasks=0 missing`,
		`node handler:restart db "restart db" handler · not found tasks=0 missing`,
		`node role:shared "shared" role tasks=1`,
		`node role:galaxy_role "galaxy_role" role · external tasks=0 external`,
		`edge play:0 -notify-> handler:restart nginx`,
		`edge play:0 -role-> role:common`,
		`edge role:common -include-> tasks:repo/roles/common/tasks/users.yml dashed`,
		`edge play:0 -role-> role:nginx`,
		`edge role:nginx -notify-> handler:restart nginx`,
		`edge play:0 -notify-> handler:restart app`,
		`edge play:0 -import-> tasks:repo/playbooks/tasks/extra.yml`,
		`edge play:1 -include-> role:comon dashed`,
		`edge play:1 -notify-> handler:restart db`,
		`edge play:1 -include-> role:shared dashed`,
		`edge play:1 -include-> role:galaxy_role dashed`,
		`edge role:nginx -dependency-> role:common`,
	}, "\n")
	if got := drawn(g); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestGraphDynamicIncludes(t *testing.T) {
	g := BuildGraph(analyze(t, "repo/playbooks/map.yml", workspace(t)))
	got := drawn(g)
	for _, want := range []string{
		`node tasks:{{ setup_file }} "extra.yml" task file · assumed tasks=0 assumed`,
		`node tasks:tasks/{{ flavour }}.yml "tasks/{{ flavour }}.yml" task file · one of 2 tasks=0 candidates=tasks/green.yml,tasks/blue.yml`, // highest precedence first,
		`node tasks:tasks/{{ nowhere }}.yml "tasks/{{ nowhere }}.yml" task file · decided while running tasks=0`,
		`edge play:0 -include-> tasks:{{ setup_file }} dashed`,
		`edge play:0 -notify-> handler:restart nginx`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in\n%s", want, got)
		}
	}
}
