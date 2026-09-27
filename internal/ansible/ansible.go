// Package ansible reads Ansible playbooks, roles and YAML inventories
// the way ansible-playbook runs them: plays in order, and in each play
// fact gathering, pre_tasks, roles (their dependencies first), tasks
// and post_tasks, with handlers flushed after each part. Roles,
// import/include_tasks and import/include_role are followed into their
// files, and variable definitions are collected with their precedence
// so a {{ variable }} can be explained.
//
// It is an engine package: files come through Options.Load.
package ansible

import (
	"cmp"
	"fmt"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/names"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Source says where something is written.
type Source struct {
	File  string        `json:"file"` // workspace path
	Line  int           `json:"line"`
	Range yamlkit.Range `json:"range"`
}

// Playbook is a playbook (or a task file), read and expanded.
type Playbook struct {
	File string `json:"file"`
	// Kind is "playbook", or "tasks" for a task file (a role's
	// tasks/main.yml, a file for include_tasks).
	Kind      string    `json:"kind"`
	Role      string    `json:"role,omitempty"` // the role a task file belongs to
	Plays     []Play    `json:"plays"`
	Tasks     []Task    `json:"tasks"`    // a task file's tasks
	Handlers  []Task    `json:"handlers"` // a role task file's handlers
	Roles     []Role    `json:"roles"`    // roles used, in first-use order
	Variables []VarDef  `json:"variables"`
	Files     []File    `json:"files"`
	Problems  []Problem `json:"problems"`
}

// Play is one play; Steps are in the order Ansible runs them.
type Play struct {
	Name      string   `json:"name"`
	Hosts     string   `json:"hosts,omitempty"`
	Imported  string   `json:"imported,omitempty"` // the playbook it came from through import_playbook
	Source    *Source  `json:"source,omitempty"`
	VarsFiles []string `json:"varsFiles"`
	Steps     []Task   `json:"steps"`
	Handlers  []Task   `json:"handlers"`
}

// Task is one step of a run: a task, a block, an include, a role, or
// a section (pre_tasks, roles, …) holding others.
type Task struct {
	Name string `json:"name"`
	// Kind is task, block, section, role, include, import, facts,
	// flush (handlers run here), or handler.
	Kind     string   `json:"kind"`
	Module   string   `json:"module,omitempty"`
	Detail   string   `json:"detail,omitempty"` // the file or role an include names; module arguments
	When     string   `json:"when,omitempty"`
	Loop     string   `json:"loop,omitempty"`
	Notify   []string `json:"notify"`
	Listen   []string `json:"listen"`
	Tags     []string `json:"tags"`
	Register string   `json:"register,omitempty"`
	Role     string   `json:"role,omitempty"`    // the role the task belongs to
	Dynamic  bool     `json:"dynamic,omitempty"` // include_*: decided while running
	Missing  bool     `json:"missing,omitempty"` // the file or role it names wasn't found
	Source   *Source  `json:"source,omitempty"`
	Children []Task   `json:"children"`
}

// Role is a role the playbook uses.
type Role struct {
	Name         string   `json:"name"`
	Path         string   `json:"path,omitempty"` // the role's folder; "" when not found
	Found        bool     `json:"found"`
	External     bool     `json:"external,omitempty"` // from a collection: not in the folder
	Parts        []string `json:"parts"`              // tasks, handlers, defaults, vars, meta, templates, files
	Dependencies []string `json:"dependencies"`
	Elsewhere    bool     `json:"elsewhere,omitempty"` // found in another repository of the open folder
}

// File is one file the analysis read.
type File struct {
	Path string `json:"path"`
	Role string `json:"role"` // playbook, tasks, role, vars, group_vars, host_vars, config
	Read bool   `json:"read"`
}

// Problem is something wrong or worth knowing.
type Problem struct {
	Severity yamlkit.Severity `json:"severity"`
	Code     string           `json:"code"`
	Message  string           `json:"message"`
	Hint     string           `json:"hint,omitempty"`
	Source   *Source          `json:"source,omitempty"`
}

// Loader reads a workspace file by slash path.
type Loader func(path string) ([]byte, error)

// Options feed an analysis. Without Files (no folder open), only the
// file itself is read and findings about other files are skipped.
type Options struct {
	Load  Loader
	Files []string // the workspace's files
	Root  string   // the repository's folder, as a workspace path ("" = the workspace)
}

// analyzer carries one analysis.
type analyzer struct {
	p          *Playbook
	opts       Options
	standalone bool
	have       map[string]bool
	dirs       map[string]bool // every folder holding a workspace file
	roles      map[string]*Role
	rolesPath  []string // from ansible.cfg
	read       map[string]*yamlkit.Node
	playRoles  map[string]bool   // roles already run in the current play
	reqs       map[string]string // requirements files, read once
	depth      int
}

// Analyze reads file (content is its text).
func Analyze(file string, content []byte, f *yamlkit.File, opts Options) *Playbook {
	p := &Playbook{File: file, Kind: "playbook"}
	a := &analyzer{p: p, opts: opts, standalone: opts.Files == nil, have: map[string]bool{}, dirs: map[string]bool{}, playRoles: map[string]bool{}, roles: map[string]*Role{}, read: map[string]*yamlkit.Node{}}
	for _, x := range opts.Files {
		a.have[x] = true
		for d := path.Dir(x); d != "." && d != "/" && !a.dirs[d]; d = path.Dir(d) {
			a.dirs[d] = true
		}
	}
	var root *yamlkit.Node
	if f != nil && len(f.Docs) > 0 {
		root = f.Docs[0].Root
	}
	a.read[file] = root
	a.config(path.Dir(file))
	if !isPlaybook(root) {
		p.Kind = "tasks"
		var dir string
		p.Role, dir = roleOf(file)
		if p.Role != "" {
			// A role's own file: its defaults, vars and handlers apply.
			r := a.findRole(p.Role, path.Dir(dir), nil)
			if r.Found && slices.Contains(r.Parts, "handlers") {
				p.Handlers = a.roleFile(*r, "handlers", "main")
			}
		}
		p.Tasks = a.tasks(root, file, p.Role, 0)
	} else {
		a.plays(root, file, "", 0)
	}
	a.variables()
	return normalize(p)
}

// isPlaybook tells a playbook (a list of plays) from a task list.
func isPlaybook(root *yamlkit.Node) bool {
	if root == nil || root.Kind != yamlkit.KindSeq {
		return false
	}
	return slices.ContainsFunc(root.Items, func(n *yamlkit.Node) bool {
		return n.Get("hosts") != nil || n.Get("import_playbook") != nil || n.Get("ansible.builtin.import_playbook") != nil
	})
}

// roleOf is the role a file of roles/<name>/… belongs to, and the
// role's folder.
func roleOf(file string) (string, string) {
	parts := strings.Split(file, "/")
	for i := len(parts) - 3; i >= 0; i-- {
		if parts[i] == "roles" && i+2 < len(parts) {
			return parts[i+1], strings.Join(parts[:i+2], "/")
		}
	}
	return "", ""
}

func normalize(p *Playbook) *Playbook {
	p.Plays, p.Tasks, p.Roles, p.Variables, p.Files, p.Problems = orEmpty(p.Plays), orEmpty(p.Tasks), orEmpty(p.Roles), orEmpty(p.Variables), orEmpty(p.Files), orEmpty(p.Problems)
	for i := range p.Plays {
		p.Plays[i].VarsFiles, p.Plays[i].Steps, p.Plays[i].Handlers = orEmpty(p.Plays[i].VarsFiles), fill(p.Plays[i].Steps), fill(p.Plays[i].Handlers)
	}
	p.Tasks, p.Handlers = fill(p.Tasks), fill(p.Handlers)
	for i := range p.Roles {
		p.Roles[i].Parts, p.Roles[i].Dependencies = orEmpty(p.Roles[i].Parts), orEmpty(p.Roles[i].Dependencies)
	}
	return p
}

// fill gives every task's lists a value, so JSON has [] not null.
func fill(ts []Task) []Task {
	ts = orEmpty(ts)
	for i := range ts {
		t := &ts[i]
		t.Notify, t.Listen, t.Tags = orEmpty(t.Notify), orEmpty(t.Listen), orEmpty(t.Tags)
		t.Children = fill(t.Children)
	}
	return ts
}

func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func (a *analyzer) problem(sev yamlkit.Severity, code string, at *Source, msg, hint string) {
	a.p.Problems = append(a.p.Problems, Problem{Severity: sev, Code: code, Source: at, Message: msg, Hint: hint})
}

// load reads a workspace file once.
func (a *analyzer) load(file, role string) *yamlkit.Node {
	if n, ok := a.read[file]; ok {
		return n
	}
	a.read[file] = nil
	if a.opts.Load == nil || (!a.standalone && !a.have[file]) {
		return nil
	}
	data, err := a.opts.Load(file)
	a.p.Files = append(a.p.Files, File{Path: file, Role: role, Read: err == nil})
	if err != nil {
		return nil
	}
	f := yamlkit.Parse(data)
	if len(f.Docs) == 0 {
		return nil
	}
	a.read[file] = f.Docs[0].Root
	return f.Docs[0].Root
}

// exists reports whether a workspace file exists.
func (a *analyzer) exists(file string) bool { return a.have[file] }

// firstExisting returns the first candidate that exists.
func (a *analyzer) firstExisting(cands ...string) string {
	for _, c := range cands {
		if a.exists(c) {
			return c
		}
	}
	return ""
}

var rolesPathRE = regexp.MustCompile(`(?m)^\s*roles_path\s*[=:]\s*(.+)$`)

// config reads roles_path from the nearest ansible.cfg above dir.
func (a *analyzer) config(dir string) {
	for d := dir; ; d = path.Dir(d) {
		cfg := path.Join(d, "ansible.cfg")
		if a.exists(cfg) {
			data, err := a.opts.Load(cfg)
			a.p.Files = append(a.p.Files, File{Path: cfg, Role: "config", Read: err == nil})
			if m := rolesPathRE.FindSubmatch(data); m != nil {
				for _, rp := range strings.Split(string(m[1]), ":") {
					if rp = strings.TrimSpace(rp); rp != "" && !strings.HasPrefix(rp, "/") && !strings.HasPrefix(rp, "~") {
						a.rolesPath = append(a.rolesPath, path.Join(d, rp))
					}
				}
			}
			return
		}
		if d == "." || d == "/" || d == a.opts.Root || d == "" {
			return
		}
	}
}

// --- plays ---

func (a *analyzer) plays(root *yamlkit.Node, file, imported string, depth int) {
	for _, n := range root.Items {
		if imp := cmp.Or(n.Get("import_playbook"), n.Get("ansible.builtin.import_playbook")); imp != nil {
			target := path.Join(path.Dir(file), text(imp))
			at := ptr(src(file, imp))
			other := a.load(target, "playbook")
			switch {
			case depth >= 4:
			case other == nil:
				if !a.standalone {
					a.problem(yamlkit.SeverityError, "ansible-file-missing", at, "Can't find playbook "+text(imp), "")
				}
				a.p.Plays = append(a.p.Plays, Play{Name: "import_playbook " + text(imp), Imported: imported, Source: at})
			case other != nil:
				a.plays(other, target, target, depth+1)
			}
			continue
		}
		a.p.Plays = append(a.p.Plays, a.play(n, file, imported))
	}
}

func (a *analyzer) play(n *yamlkit.Node, file, imported string) Play {
	pl := Play{Name: text(n.Get("name")), Hosts: flowText(n.Get("hosts")), Imported: imported, Source: ptr(src(file, n))}
	if pl.Name == "" {
		pl.Name = pl.Hosts
	}
	a.playRoles = map[string]bool{}
	a.playVars(n, file, pl.Name)
	for _, vf := range items(n.Get("vars_files")) {
		pl.VarsFiles = append(pl.VarsFiles, text(vf))
	}
	dir := path.Dir(file)
	flush := func(after []Task) {
		if notifies(after) {
			pl.Steps = append(pl.Steps, Task{Name: "Run notified handlers", Kind: "flush"})
		}
	}
	if g := n.Get("gather_facts"); g == nil || !isFalse(g) {
		pl.Steps = append(pl.Steps, Task{Name: "Gathering Facts", Kind: "facts", Module: "setup"})
	}
	if pre := a.tasks(n.Get("pre_tasks"), file, "", 0); len(pre) > 0 {
		pl.Steps = append(pl.Steps, section("pre_tasks", file, n, pre))
		flush(pre)
	}
	var roles []Task
	for _, r := range items(n.Get("roles")) {
		name := text(r)
		if r.Kind == yamlkit.KindMap {
			name = cmp.Or(text(r.Get("role")), text(r.Get("name")))
		}
		t := a.role(name, "main", ptr(src(file, r)), dir, false)
		if r.Kind == yamlkit.KindMap {
			for _, pr := range r.Pairs {
				if k := pr.Key.Value; k == "vars" {
					a.defsIn(pr.Value, file, "role param", name)
				} else if !roleKeywords[k] {
					a.def(k, "role param", name, flowText(pr.Value), ptr(src(file, pr.Key)))
				}
			}
		}
		t.When = oneLine(flowText(r.Get("when")))
		t.Tags = strs(r.Get("tags"))
		roles = append(roles, t)
	}
	var tasks []Task
	if len(roles) > 0 {
		pl.Steps = append(pl.Steps, section("roles", file, n, roles))
		tasks = append(tasks, roles...)
	}
	if ts := a.tasks(n.Get("tasks"), file, "", 0); len(ts) > 0 {
		pl.Steps = append(pl.Steps, section("tasks", file, n, ts))
		tasks = append(tasks, ts...)
	}
	flush(tasks)
	if post := a.tasks(n.Get("post_tasks"), file, "", 0); len(post) > 0 {
		pl.Steps = append(pl.Steps, section("post_tasks", file, n, post))
		flush(post)
	}
	pl.Handlers = a.tasks(n.Get("handlers"), file, "", 0)
	for i := range pl.Handlers {
		pl.Handlers[i].Kind = kindOr(pl.Handlers[i].Kind, "handler")
	}
	// Handlers of the play's roles are reachable too.
	for _, r := range a.p.Roles {
		if a.playRoles[r.Name] && slices.Contains(r.Parts, "handlers") {
			for _, h := range a.roleFile(r, "handlers", "main") {
				h.Kind = kindOr(h.Kind, "handler")
				pl.Handlers = append(pl.Handlers, h)
			}
		}
	}
	a.checkNotify(&pl)
	return pl
}

func kindOr(k, def string) string {
	if k == "task" {
		return def
	}
	return k
}

func section(name, file string, play *yamlkit.Node, children []Task) Task {
	t := Task{Name: name, Kind: "section", Children: children}
	if pr := play.Pair(name); pr != nil {
		t.Source = ptr(src(file, pr.Key))
	}
	return t
}

// notifies reports whether any task (or a task inside) notifies a
// handler.
func notifies(ts []Task) bool {
	return slices.ContainsFunc(ts, func(t Task) bool { return len(t.Notify) > 0 || notifies(t.Children) })
}

// checkNotify reports notify: names no handler has (by name or listen).
func (a *analyzer) checkNotify(pl *Play) {
	known := map[string]bool{}
	var list []string
	for _, h := range pl.Handlers {
		known[h.Name] = true
		list = append(list, h.Name)
		for _, l := range h.Listen {
			known[l] = true
		}
	}
	if a.standalone {
		return // role handlers can't be seen
	}
	var walk func(ts []Task)
	walk = func(ts []Task) {
		for _, t := range ts {
			for _, n := range t.Notify {
				if !known[n] && !strings.Contains(n, "{{") {
					a.problem(yamlkit.SeverityWarning, "ansible-handler-unknown", t.Source, fmt.Sprintf("No handler of play %s is named %q (or listens to it)", pl.Name, n), names.DidYouMean(n, list))
				}
			}
			walk(t.Children)
		}
	}
	walk(pl.Steps)
}

// --- tasks ---

// taskKeywords are the keys of a task that aren't its module.
var taskKeywords = map[string]bool{}

func init() {
	for _, k := range strings.Fields(`name when loop loop_control register notify tags become become_user become_method
		become_flags vars environment ignore_errors ignore_unreachable failed_when changed_when delegate_to delegate_facts
		run_once no_log retries delay until block rescue always listen args async poll check_mode diff any_errors_fatal
		throttle timeout collections module_defaults debugger connection local_action action port remote_user
		with_items with_dict with_fileglob with_first_found with_together with_subelements with_sequence with_nested
		with_list with_indexed_items with_random_choice with_lines with_inventory_hostnames with_flattened`) {
		taskKeywords[k] = true
	}
}

// short drops the ansible.builtin./ansible.legacy. prefix of a module.
func short(m string) string {
	return strings.TrimPrefix(strings.TrimPrefix(m, "ansible.builtin."), "ansible.legacy.")
}

// tasks reads a list of tasks from file. role is the role the file
// belongs to ("" for a playbook).
func (a *analyzer) tasks(list *yamlkit.Node, file, role string, depth int) []Task {
	var out []Task
	for _, n := range items(list) {
		if n.Kind == yamlkit.KindMap {
			out = append(out, a.task(n, file, role, depth))
		}
	}
	return out
}

func (a *analyzer) task(n *yamlkit.Node, file, role string, depth int) Task {
	t := Task{Name: text(n.Get("name")), Kind: "task", Role: role, Source: ptr(src(file, n)),
		When: oneLine(flowText(n.Get("when"))), Notify: strs(n.Get("notify")), Listen: strs(n.Get("listen")), Tags: strs(n.Get("tags")), Register: text(n.Get("register"))}
	for _, pr := range n.Pairs {
		k := pr.Key.Value
		if strings.HasPrefix(k, "with_") || k == "loop" {
			t.Loop = cmp.Or(t.Loop, flowText(pr.Value))
		}
		if t.Module == "" && !taskKeywords[k] {
			t.Module = k
			t.Detail = argsText(pr.Value)
		}
	}
	if act := cmp.Or(n.Get("action"), n.Get("local_action")); act != nil && t.Module == "" {
		t.Module, _, _ = strings.Cut(strings.TrimSpace(text(act)), " ")
		if act.Kind == yamlkit.KindMap {
			t.Module = text(act.Get("module"))
		}
	}
	if b := n.Get("block"); b != nil {
		t.Kind, t.Module = "block", ""
		t.Children = a.tasks(b, file, role, depth)
		for _, part := range []string{"rescue", "always"} {
			if pr := n.Pair(part); pr != nil {
				t.Children = append(t.Children, Task{Name: part, Kind: "section", Source: ptr(src(file, pr.Key)), Children: a.tasks(pr.Value, file, role, depth)})
			}
		}
		if t.Name == "" {
			t.Name = "block"
		}
		a.taskVars(n, &t, file)
		return t
	}
	mod := short(t.Module)
	arg := n.Get(t.Module)
	switch mod {
	case "include_tasks", "import_tasks":
		t.Kind, t.Dynamic = strings.TrimSuffix(mod, "_tasks"), mod == "include_tasks"
		target := cmp.Or(text(arg.Get("file")), text(arg))
		t.Detail = target
		if t.Name == "" {
			t.Name = mod + " " + target
		}
		if strings.Contains(target, "{{") {
			break // decided while running
		}
		p := a.taskFile(target, file, role)
		body := a.load(p, "tasks")
		if body == nil {
			t.Missing = true
			if !a.standalone {
				a.problem(yamlkit.SeverityError, "ansible-file-missing", ptr(src(file, arg)), "Can't find task file "+target, "Paths are relative to the including file, or to the role's tasks/ folder.")
			}
			break
		}
		if depth < 8 {
			t.Children = a.tasks(body, p, role, depth+1)
		}
	case "include_role", "import_role":
		t.Kind = strings.TrimSuffix(mod, "_role")
		name := text(arg.Get("name"))
		from := cmp.Or(strings.TrimSuffix(strings.TrimSuffix(text(arg.Get("tasks_from")), ".yml"), ".yaml"), "main")
		if t.Name == "" {
			t.Name = mod + " " + name
		}
		if depth < 8 {
			r := a.role(name, from, ptr(src(file, arg)), path.Dir(file), mod == "include_role")
			t.Children, t.Missing, t.Detail = r.Children, r.Missing, r.Detail
		}
		t.Dynamic = mod == "include_role"
	}
	if t.Name == "" {
		t.Name = cmp.Or(t.Module, "task")
	}
	a.taskVars(n, &t, file)
	return t
}

// taskFile finds the file an include/import_tasks names: next to the
// including file, or in the role's tasks/ folder.
func (a *analyzer) taskFile(target, from, role string) string {
	cands := []string{path.Join(path.Dir(from), target)}
	if _, dir := roleOf(from); dir != "" {
		cands = append(cands, path.Join(dir, "tasks", target))
	}
	if r := a.roles[role]; r != nil && r.Path != "" {
		cands = append(cands, path.Join(r.Path, "tasks", target))
	}
	if a.standalone {
		return cands[0]
	}
	return cmp.Or(a.firstExisting(cands...), cands[0])
}

// --- roles ---

// role expands a role into a step: its dependencies' tasks, then its
// own (tasks/<from>.yml).
func (a *analyzer) role(name, from string, at *Source, dir string, dynamic bool) Task {
	t := Task{Name: name, Kind: "role", Role: name, Source: at, Dynamic: dynamic}
	if name == "" || strings.Contains(name, "{{") {
		return t
	}
	r := a.findRole(name, dir, at)
	t.Detail = r.Path
	if !r.Found {
		t.Missing = !r.External
		if r.External {
			t.Detail = "from a collection"
		}
		return t
	}
	// A role runs once per play unless its parameters differ; codec
	// shows the dependencies the first time.
	if !a.playRoles[name] && from == "main" {
		for _, dep := range r.Dependencies {
			if !a.playRoles[dep] {
				d := a.role(dep, "main", at, dir, false)
				d.Kind, d.Name = "role", dep+" (dependency of "+name+")"
				t.Children = append(t.Children, d)
			}
		}
	}
	a.playRoles[name] = true
	t.Children = append(t.Children, a.roleFile(*r, "tasks", from)...)
	return t
}

// roleFile reads a role's tasks/<file> or handlers/<file>.
func (a *analyzer) roleFile(r Role, part, file string) []Task {
	p := a.firstExisting(path.Join(r.Path, part, file+".yml"), path.Join(r.Path, part, file+".yaml"), path.Join(r.Path, part, file))
	if p == "" {
		return nil
	}
	return a.tasks(a.load(p, "role"), p, r.Name, 1)
}

// findRole resolves a role name the way Ansible searches: roles/ next
// to the playbook, roles_path from ansible.cfg, then (codec's best
// effort) roles/ in folders above, and a folder of that name.
func (a *analyzer) findRole(name, dir string, at *Source) *Role {
	if r := a.roles[name]; r != nil {
		return r
	}
	r := &Role{Name: name}
	a.roles[name] = r
	defer func() { a.p.Roles = append(a.p.Roles, *r) }()
	if strings.Count(name, ".") >= 2 && !strings.Contains(name, "/") {
		r.External = true // namespace.collection.role
		return r
	}
	var cands []string
	if strings.Contains(name, "/") {
		cands = append(cands, path.Join(dir, name))
	}
	cands = append(cands, path.Join(dir, "roles", name))
	for _, rp := range a.rolesPath {
		cands = append(cands, path.Join(rp, name))
	}
	for d := path.Dir(dir); ; d = path.Dir(d) {
		cands = append(cands, path.Join(d, "roles", name))
		if d == "." || d == "/" || d == a.opts.Root {
			break
		}
	}
	cands = append(cands, path.Join(dir, name))
	for _, c := range cands {
		if r.Parts = a.roleParts(c); len(r.Parts) > 0 {
			r.Path, r.Found = path.Clean(c), true
			break
		}
	}
	if !r.Found && !a.standalone {
		// Roles are often installed when the playbook runs (galaxy
		// requirements, a folder copied in by CI), like GitLab's remote
		// includes: not an error unless a close local name suggests a
		// typo. A folder holding several repositories may have it in
		// another one; codec uses that copy and says so.
		if c := a.elsewhere(name); c != "" {
			r.Parts, r.Path, r.Found, r.Elsewhere = a.roleParts(c), c, true, true
			a.problem(yamlkit.SeverityInfo, "ansible-role-elsewhere", at, fmt.Sprintf("Role %s isn't in this repository; codec reads the copy in %s", name, c), "At run time it must be installed where Ansible looks (roles/, roles_path).")
		} else if hint := names.DidYouMean(name, a.allRoles()); hint != "" {
			a.problem(yamlkit.SeverityWarning, "ansible-role-missing", at, "Can't find role "+name, hint)
		} else {
			hint := "It may be installed when the playbook runs (ansible-galaxy, or copied in by CI). codec searched roles/ next to the playbook, roles_path in ansible.cfg, and roles/ in the folders above."
			if req := a.listedIn(name); req != "" {
				hint = "Listed in " + req + ", so it is installed when the playbook runs."
			}
			a.problem(yamlkit.SeverityInfo, "ansible-role-missing", at, "Role "+name+" isn't in the folder", hint)
		}
	}
	if !r.Found {
		return r
	}
	a.roleVars(r)
	if meta := a.load(a.firstExisting(path.Join(r.Path, "meta/main.yml"), path.Join(r.Path, "meta/main.yaml")), "role"); meta != nil {
		for _, d := range items(meta.Get("dependencies")) {
			name := text(d)
			if d.Kind == yamlkit.KindMap {
				name = cmp.Or(text(d.Get("role")), text(d.Get("name")))
			}
			if name != "" {
				r.Dependencies = append(r.Dependencies, name)
			}
		}
	}
	return r
}

// hasDir reports whether any workspace file lives under dir.
func (a *analyzer) hasDir(dir string) bool { return a.dirs[dir] }

// allRoles lists the role folder names in the workspace, for "did you
// mean".
func (a *analyzer) allRoles() []string {
	seen := map[string]bool{}
	for f := range a.have {
		if r, _ := roleOf(f); r != "" {
			seen[r] = true
		}
	}
	return slices.Sorted(maps.Keys(seen))
}

// --- helpers ---

func src(file string, n *yamlkit.Node) Source {
	return Source{File: file, Line: n.Range.Start.Line, Range: n.Range}
}

func ptr[T any](v T) *T { return &v }

func items(n *yamlkit.Node) []*yamlkit.Node {
	if n == nil || n.Kind != yamlkit.KindSeq {
		return nil
	}
	return n.Items
}

func text(n *yamlkit.Node) string {
	if n == nil || n.Kind != yamlkit.KindScalar || n.Tag == yamlkit.TagNull {
		return ""
	}
	return n.Value
}

// flowText shows any value on one line.
func flowText(n *yamlkit.Node) string {
	if n == nil {
		return ""
	}
	if n.Kind == yamlkit.KindScalar {
		return text(n)
	}
	if n.Kind == yamlkit.KindSeq && !n.Flow {
		parts := make([]string, len(n.Items))
		for i, it := range n.Items {
			parts[i] = flowText(it)
		}
		return strings.Join(parts, " and ")
	}
	return yamlkit.InlineText(n)
}

func strs(n *yamlkit.Node) []string {
	switch {
	case n == nil:
		return nil
	case n.Kind == yamlkit.KindScalar:
		return []string{n.Value}
	}
	var out []string
	for _, it := range n.Items {
		out = append(out, text(it))
	}
	return out
}

// argsText summarizes a module's arguments on one line.
func argsText(n *yamlkit.Node) string {
	s := flowText(n)
	if len(s) > 120 {
		s = s[:117] + "…"
	}
	return s
}

func isFalse(n *yamlkit.Node) bool {
	switch strings.ToLower(text(n)) {
	case "false", "no", "off", "0":
		return true
	}
	return false
}

// roleKeywords are the keys of a roles: entry that aren't parameters.
var roleKeywords = map[string]bool{"role": true, "name": true, "when": true, "tags": true, "become": true, "become_user": true,
	"delegate_to": true, "environment": true, "vars": true, "ignore_errors": true, "any_errors_fatal": true, "no_log": true}

// roleParts lists the parts of a role folder that exist.
func (a *analyzer) roleParts(dir string) []string {
	var parts []string
	for _, part := range []string{"tasks", "handlers", "defaults", "vars", "meta", "templates", "files"} {
		if a.hasDir(path.Join(dir, part)) {
			parts = append(parts, part)
		}
	}
	return parts
}

// elsewhere finds roles/<name> anywhere in the open folder (the
// shortest path), for folders holding several repositories.
func (a *analyzer) elsewhere(name string) string {
	best := ""
	for d := range a.dirs {
		if (d == "roles/"+name || strings.HasSuffix(d, "/roles/"+name)) && a.hasDir(d+"/tasks") {
			if best == "" || len(d) < len(best) || len(d) == len(best) && d < best {
				best = d
			}
		}
	}
	return best
}

// listedIn names a requirements file of the repository that lists the
// role (requirements.yml, galaxy-requirements.yml, requirements/*.txt).
func (a *analyzer) listedIn(name string) string {
	if a.reqs == nil {
		a.reqs = map[string]string{}
		for f := range a.have {
			if !strings.HasPrefix(f, a.rootPrefix()) || !strings.Contains(path.Base(f)+"/"+path.Base(path.Dir(f)), "requirements") {
				continue
			}
			if data, err := a.opts.Load(f); err == nil && len(data) < 1<<20 {
				a.reqs[f] = string(data)
			}
		}
	}
	word := regexp.MustCompile(`(^|[\s/:'"])` + regexp.QuoteMeta(name) + `($|[\s,'".])`)
	var found []string
	for f, text := range a.reqs {
		for _, l := range strings.Split(text, "\n") {
			if word.MatchString(strings.TrimSpace(l)) {
				found = append(found, f)
				break
			}
		}
	}
	slices.Sort(found)
	if len(found) == 0 {
		return ""
	}
	return found[0]
}

// oneLine collapses whitespace (folded when: conditions).
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
