package ansible

import (
	"cmp"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Graph is the shape of a playbook as a diagram: plays, the roles and
// task files they pull in (and how), and the handlers tasks notify.
// It is built from the execution-order analysis.
type Graph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// GraphNode is a play, role, task file or handler.
type GraphNode struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"` // play, role, tasks, handler
	Label string `json:"label"`
	Sub   string `json:"sub,omitempty"`
	// Where it is written: a play's playbook, a task file, a role's
	// folder, a handler's file. Line is 0 for folders.
	File string `json:"file,omitempty"`
	Line int    `json:"line,omitempty"`
	Role string `json:"role,omitempty"` // role nodes and handlers in a role
	Play string `json:"play,omitempty"` // play nodes: the play's name
	// Tasks counts the tasks directly inside (not in nested files).
	Tasks    int  `json:"tasks,omitempty"`
	External bool `json:"external,omitempty"` // a role from a collection or Galaxy
	Missing  bool `json:"missing,omitempty"`  // named but not found
	// A dynamic include whose file name holds a variable: resolved and
	// marked Assumed when the variable has one definition, otherwise
	// the Candidates (one per definition), or neither.
	Assumed    bool     `json:"assumed,omitempty"`
	Candidates []string `json:"candidates,omitempty"`
}

// GraphEdge links a node to one it pulls in or notifies.
type GraphEdge struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Kind    string `json:"kind"`              // role, dependency, import, include, notify
	Dynamic bool   `json:"dynamic,omitempty"` // include_*: decided while running (drawn dashed)
}

var varRef = regexp.MustCompile(`\{\{\s*([A-Za-z_]\w*)\s*\}\}`)

// BuildGraph draws a playbook (or a task file) as a graph.
func BuildGraph(pb *Playbook) *Graph {
	b := &graphBuilder{g: &Graph{Nodes: []GraphNode{}, Edges: []GraphEdge{}}, at: map[string]int{}, pb: pb}
	if pb.Kind == "tasks" {
		root := b.node(GraphNode{ID: "tasks:" + pb.File, Kind: "tasks", Label: path.Base(pb.File), Sub: "task file", File: pb.File})
		b.walk(pb.Tasks, root, nil)
	}
	for i, pl := range pb.Plays {
		n := GraphNode{ID: "play:" + strconv.Itoa(i), Kind: "play", Label: cmp.Or(pl.Name, "play "+strconv.Itoa(i+1)), Play: pl.Name, Sub: "play"}
		if pl.Hosts != "" {
			n.Sub = "play · hosts: " + pl.Hosts
		}
		if pl.Source != nil {
			n.File, n.Line = pl.Source.File, pl.Source.Line
		}
		id := b.node(n)
		b.walk(pl.Steps, id, pl.Handlers)
	}
	// Role dependencies from meta/main.yml, also when the dependency had
	// already run in the play (and so isn't expanded again).
	for _, r := range pb.Roles {
		if _, ok := b.at["role:"+r.Name]; !ok {
			continue
		}
		for _, d := range r.Dependencies {
			if _, ok := b.at["role:"+d]; ok {
				b.edge(GraphEdge{From: "role:" + r.Name, To: "role:" + d, Kind: "dependency"})
			}
		}
	}
	return b.g
}

type graphBuilder struct {
	g  *Graph
	at map[string]int // node id → index
	pb *Playbook
}

func (b *graphBuilder) node(n GraphNode) string {
	if _, ok := b.at[n.ID]; !ok {
		b.at[n.ID] = len(b.g.Nodes)
		b.g.Nodes = append(b.g.Nodes, n)
	}
	return n.ID
}

func (b *graphBuilder) edge(e GraphEdge) {
	for _, x := range b.g.Edges {
		if x.From == e.From && x.To == e.To {
			return
		}
	}
	b.g.Edges = append(b.g.Edges, e)
}

// walk adds what steps pull in, as children of node from; handlers are
// the play's, for notify edges.
func (b *graphBuilder) walk(steps []Task, from string, handlers []Task) {
	for _, t := range steps {
		switch t.Kind {
		case "section", "block":
			b.walk(t.Children, from, handlers)
		case "role":
			name := strings.TrimSpace(strings.SplitN(t.Name, " (dependency of", 2)[0])
			if t.Role != "" && !strings.Contains(t.Name, "(dependency of") {
				name = t.Role
			}
			id := b.roleNode(name, t)
			kind := "role"
			if strings.Contains(t.Name, "(dependency of") {
				kind = "dependency"
			}
			b.edge(GraphEdge{From: from, To: id, Kind: kind, Dynamic: t.Dynamic})
			b.walk(t.Children, id, handlers)
		case "include", "import":
			if strings.HasSuffix(t.Module, "_role") {
				name := roleNameOf(t)
				id := b.roleNode(name, t)
				b.edge(GraphEdge{From: from, To: id, Kind: t.Kind, Dynamic: t.Dynamic})
				b.walk(t.Children, id, handlers)
				continue
			}
			id := b.fileNode(t)
			b.edge(GraphEdge{From: from, To: id, Kind: t.Kind, Dynamic: t.Dynamic})
			b.walk(t.Children, id, handlers)
		default:
			if t.Kind == "task" {
				b.g.Nodes[b.at[from]].Tasks++
			}
			for _, name := range t.Notify {
				b.edge(GraphEdge{From: from, To: b.handlerNode(name, handlers), Kind: "notify"})
			}
			b.walk(t.Children, from, handlers)
		}
	}
}

func (b *graphBuilder) roleNode(name string, t Task) string {
	n := GraphNode{ID: "role:" + name, Kind: "role", Label: name, Sub: "role", Role: name, Missing: t.Missing}
	for _, r := range b.pb.Roles {
		if r.Name == name {
			n.File, n.External = r.Path, r.External
			if r.External {
				n.Sub = "role · external"
			}
		}
	}
	if strings.Contains(name, "{{") {
		n.Sub = "role · decided while running"
	}
	return b.node(n)
}

// roleNameOf is the role an include_role/import_role names: from the
// tasks it expanded to, else its folder, else its step name.
func roleNameOf(t Task) string {
	for _, c := range t.Children {
		if c.Role != "" {
			return c.Role
		}
	}
	if t.Detail != "" && !strings.Contains(t.Detail, " ") {
		return path.Base(t.Detail)
	}
	for _, p := range []string{"include_role ", "import_role "} {
		if rest, ok := strings.CutPrefix(t.Name, p); ok {
			return rest
		}
	}
	return t.Name
}

func (b *graphBuilder) fileNode(t Task) string {
	file := t.Detail
	if len(t.Children) > 0 && t.Children[0].Source != nil {
		file = t.Children[0].Source.File
	}
	n := GraphNode{ID: "tasks:" + file, Kind: "tasks", Label: path.Base(file), Sub: "task file", File: file, Missing: t.Missing}
	if !strings.Contains(file, "{{") {
		return b.node(n)
	}
	// A file name with variables: try their definitions.
	n.ID, n.File, n.Label = "tasks:"+t.Detail, "", t.Detail
	cands := b.resolve(t.Detail)
	switch {
	case len(cands) == 1:
		n.Label, n.File, n.Assumed = path.Base(cands[0]), cands[0], true
		n.Sub = "task file · assumed"
	case len(cands) > 1:
		n.Candidates = cands
		n.Sub = "task file · one of " + strconv.Itoa(len(cands))
	default:
		n.Sub = "task file · decided while running"
	}
	return b.node(n)
}

// resolve fills a templated file name from the variables' definitions:
// one result per combination, nil when a variable has none or there
// would be too many to be useful.
func (b *graphBuilder) resolve(target string) []string {
	out := []string{target}
	for _, m := range varRef.FindAllStringSubmatch(target, -1) {
		var vals []string
		for _, v := range b.pb.Variables {
			if v.Name == m[1] && !strings.Contains(v.Value, "{{") && !strings.ContainsAny(v.Value, "\n[{") && !slices.Contains(vals, v.Value) {
				vals = append(vals, v.Value)
			}
		}
		if len(vals) == 0 {
			return nil
		}
		var next []string
		for _, o := range out {
			for _, v := range vals {
				next = append(next, strings.Replace(o, m[0], v, 1))
			}
		}
		if len(next) > 8 {
			return nil
		}
		out = next
	}
	return out
}

func (b *graphBuilder) handlerNode(name string, handlers []Task) string {
	for _, h := range handlers {
		if h.Name == name || slices.Contains(h.Listen, name) {
			n := GraphNode{ID: "handler:" + h.Name, Kind: "handler", Label: h.Name, Sub: "handler", Role: h.Role}
			if h.Source != nil {
				n.File, n.Line = h.Source.File, h.Source.Line
			}
			return b.node(n)
		}
	}
	return b.node(GraphNode{ID: "handler:" + name, Kind: "handler", Label: name, Sub: "handler · not found", Missing: true})
}
