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
	External bool `json:"external,omitempty"` // a role installed at run time
	Missing  bool `json:"missing,omitempty"`  // named but not found
	// A dynamic include whose file name holds a variable: resolved and
	// marked Assumed when the variable has one definition in scope,
	// otherwise the Candidates (one per definition), or neither.
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

// A plain variable reference; anything else in {{ }} (filters,
// expressions) leaves the file name to run time.
var varRef = regexp.MustCompile(`\{\{\s*([A-Za-z_]\w*)\s*\}\}`)

// BuildGraph draws a playbook (or a task file) as a graph.
func BuildGraph(pb *Playbook) *Graph {
	b := &graphBuilder{g: &Graph{Nodes: []GraphNode{}, Edges: []GraphEdge{}}, at: map[string]int{}, edges: map[[2]string]int{}, seen: map[string]bool{}, pb: pb}
	if pb.Kind == "tasks" {
		root := b.node(GraphNode{ID: "tasks:" + pb.File, Kind: "tasks", Label: path.Base(pb.File), Sub: "task file", File: pb.File})
		b.seen[root] = true
		b.walk(pb.Tasks, root, scope{handlers: pb.Handlers, role: pb.Role, id: "file"}, true)
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
		b.walk(pl.Steps, id, scope{handlers: pl.Handlers, play: pl.Name, id: strconv.Itoa(i)}, true)
	}
	// Role dependencies from meta/main.yml, also when the dependency had
	// already run in the play (and so isn't expanded again).
	for _, r := range pb.Roles {
		if _, ok := b.at["role:"+r.Name]; !ok {
			continue
		}
		for _, d := range r.Dependencies {
			if _, ok := b.at["role:"+d]; ok && d != r.Name {
				b.edge(GraphEdge{From: "role:" + r.Name, To: "role:" + d, Kind: "dependency"})
			}
		}
	}
	return b.g
}

type graphBuilder struct {
	g     *Graph
	at    map[string]int    // node id → index
	edges map[[2]string]int // from, to → edge index
	seen  map[string]bool   // nodes whose tasks are counted
	pb    *Playbook
}

// scope is where a walk is: the play (its handlers and name) and the
// role whose tasks these are, for handler edges and variables.
type scope struct {
	handlers []Task
	play     string
	role     string
	id       string // the play's index: handler nodes are per play
}

func (b *graphBuilder) node(n GraphNode) string {
	if _, ok := b.at[n.ID]; !ok {
		b.at[n.ID] = len(b.g.Nodes)
		b.g.Nodes = append(b.g.Nodes, n)
	}
	return n.ID
}

// edge adds e once per pair; a pair reached both statically and
// dynamically counts as static (it is always pulled in).
func (b *graphBuilder) edge(e GraphEdge) {
	k := [2]string{e.From, e.To}
	if i, ok := b.edges[k]; ok {
		b.g.Edges[i].Dynamic = b.g.Edges[i].Dynamic && e.Dynamic
		return
	}
	b.edges[k] = len(b.g.Edges)
	b.g.Edges = append(b.g.Edges, e)
}

// expand reports whether a node's tasks are counted on this visit: a
// role or file pulled in twice counts its tasks once.
func (b *graphBuilder) expand(id string) bool {
	if b.seen[id] {
		return false
	}
	b.seen[id] = true
	return true
}

// walk adds what steps pull in, as children of node from. count says
// whether tasks found here add to from's task count.
func (b *graphBuilder) walk(steps []Task, from string, sc scope, count bool) {
	for _, t := range steps {
		switch t.Kind {
		case "section", "block":
			b.walk(t.Children, from, sc, count)
		case "role":
			dep := strings.Contains(t.Name, "(dependency of")
			name := cmp.Or(t.Role, strings.TrimSpace(strings.SplitN(t.Name, " (dependency of", 2)[0]))
			id := b.roleNode(name, t.Missing)
			kind := "role"
			if dep {
				kind = "dependency"
			}
			if id != from {
				b.edge(GraphEdge{From: from, To: id, Kind: kind, Dynamic: t.Dynamic})
			}
			inner := sc
			inner.role = name
			b.walk(t.Children, id, inner, b.expand(id))
		case "include", "import":
			if strings.HasSuffix(t.Module, "_role") {
				name := cmp.Or(t.Target, strings.TrimPrefix(strings.TrimPrefix(t.Name, "include_role "), "import_role "))
				id := b.roleNode(name, t.Missing)
				b.edge(GraphEdge{From: from, To: id, Kind: t.Kind, Dynamic: t.Dynamic})
				inner := sc
				inner.role = name
				b.walk(t.Children, id, inner, b.expand(id))
				continue
			}
			id := b.fileNode(t, sc)
			b.edge(GraphEdge{From: from, To: id, Kind: t.Kind, Dynamic: t.Dynamic})
			b.walk(t.Children, id, sc, b.expand(id))
		default:
			if t.Kind == "task" && count {
				b.g.Nodes[b.at[from]].Tasks++
			}
			for _, name := range t.Notify {
				for _, h := range b.handlerNodes(name, sc) {
					b.edge(GraphEdge{From: from, To: h, Kind: "notify"})
				}
			}
			b.walk(t.Children, from, sc, count)
		}
	}
}

func (b *graphBuilder) roleNode(name string, missing bool) string {
	n := GraphNode{ID: "role:" + name, Kind: "role", Label: name, Sub: "role", Role: name, Missing: missing}
	for _, r := range b.pb.Roles {
		if r.Name == name {
			n.File, n.External = r.Path, r.External
			if r.External {
				n.Sub, n.Missing = "role · external", false
			}
		}
	}
	if strings.Contains(name, "{{") {
		n.Sub = "role · decided while running"
	}
	return b.node(n)
}

// fileNode is the task file an include or import pulls in. Files that
// weren't read (missing, or named with variables) are told apart by the
// folder of the file that includes them.
func (b *graphBuilder) fileNode(t Task, sc scope) string {
	if len(t.Children) > 0 && t.Children[0].Source != nil {
		f := t.Children[0].Source.File
		return b.node(GraphNode{ID: "tasks:" + f, Kind: "tasks", Label: path.Base(f), Sub: "task file", File: f})
	}
	dir := ""
	if t.Source != nil {
		dir = path.Dir(t.Source.File)
	}
	n := GraphNode{ID: "tasks:" + path.Join(dir, t.Detail), Kind: "tasks", Label: t.Detail, Sub: "task file", Missing: t.Missing}
	if !strings.Contains(t.Detail, "{{") {
		n.Label, n.File = path.Base(t.Detail), path.Join(dir, t.Detail)
		if n.Missing {
			n.Sub = "task file · not found"
		}
		return b.node(n)
	}
	n.ID = "tasks:" + dir + "|" + t.Detail
	cands := b.resolve(t.Detail, sc)
	switch {
	case len(cands) == 1:
		n.Label, n.File, n.Assumed = path.Base(cands[0]), path.Join(dir, cands[0]), true
		n.Sub = "task file · assumed"
	case len(cands) > 1:
		n.Candidates = cands
		n.Sub = "task file · one of " + strconv.Itoa(len(cands))
	default:
		n.Sub = "task file · decided while running"
	}
	return b.node(n)
}

// Variables that only exist while running say nothing about a file name.
var runtimeKinds = map[string]bool{"register": true, "set_fact": true}

// resolve fills a templated file name from the variables' definitions
// in scope: one result per combination, highest precedence first; nil
// when a reference isn't a plain variable, a variable has no
// definition, or there would be too many to be useful.
func (b *graphBuilder) resolve(target string, sc scope) []string {
	out := []string{target}
	done := map[string]bool{}
	for _, m := range varRef.FindAllStringSubmatch(target, -1) {
		name := m[1]
		if done[name] {
			continue
		}
		done[name] = true
		var vals []string
		for _, v := range b.pb.Variables {
			if v.Name != name || runtimeKinds[v.Kind] || !inScope(v, sc) {
				continue
			}
			if !strings.Contains(v.Value, "{{") && !strings.ContainsAny(v.Value, "\n[{") && !slices.Contains(vals, v.Value) {
				vals = append(vals, v.Value)
			}
		}
		if len(vals) == 0 {
			return nil
		}
		ref := regexp.MustCompile(`\{\{\s*` + regexp.QuoteMeta(name) + `\s*\}\}`)
		var next []string
		for _, o := range out {
			for _, v := range vals {
				next = append(next, ref.ReplaceAllLiteralString(o, strings.Trim(v, `"'`)))
			}
		}
		if len(next) > 8 {
			return nil
		}
		out = next
	}
	for _, o := range out {
		if strings.Contains(o, "{{") {
			return nil // filters or expressions: decided while running
		}
	}
	return out
}

// inScope reports whether a definition applies where the include is:
// play variables to their play, role variables to their role.
func inScope(v VarDef, sc scope) bool {
	switch {
	case strings.HasPrefix(v.Kind, "role"):
		return v.Scope == "" || v.Scope == sc.role
	case strings.HasPrefix(v.Kind, "play") || v.Kind == "vars_files":
		return v.Scope == "" || v.Scope == sc.play
	}
	return true
}

// handlerNodes are the handlers of this play that a notify reaches: by
// name, or every handler listening to that topic.
func (b *graphBuilder) handlerNodes(name string, sc scope) []string {
	var ids []string
	for _, h := range sc.handlers {
		if h.Name != name && !slices.Contains(h.Listen, name) {
			continue
		}
		n := GraphNode{ID: "handler:" + sc.id + ":" + h.Name, Kind: "handler", Label: h.Name, Sub: "handler", Role: h.Role}
		if h.Source != nil {
			n.File, n.Line = h.Source.File, h.Source.Line
			n.ID = "handler:" + sc.id + ":" + h.Source.File + ":" + strconv.Itoa(h.Source.Line)
		}
		ids = append(ids, b.node(n))
	}
	if len(ids) == 0 {
		ids = append(ids, b.node(GraphNode{ID: "handler:" + sc.id + ":" + name, Kind: "handler", Label: name, Sub: "handler · not found", Missing: true}))
	}
	return ids
}
