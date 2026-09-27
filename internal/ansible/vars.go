package ansible

import (
	"cmp"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// VarDef is one place a variable is given a value.
type VarDef struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	// Kind is where in Ansible's precedence it sits: role default,
	// group_vars, host_vars, inventory, play vars, vars_files, role
	// vars, block vars, task vars, include_vars, set_fact, register,
	// role param.
	Kind   string  `json:"kind"`
	Scope  string  `json:"scope,omitempty"` // the role, group, host or play it applies to
	Rank   int     `json:"rank"`            // Ansible's precedence: higher wins
	Source *Source `json:"source,omitempty"`
}

// Ranks follow Ansible's variable precedence list (1 lowest … 22
// extra vars); the inventory/playbook distinction for group_vars is
// not made.
var ranks = map[string]int{
	"role default": 2, "inventory": 3, "group_vars all": 5, "group_vars": 7, "host_vars": 10,
	"play vars": 12, "vars_files": 14, "role vars": 15, "block vars": 16, "task vars": 17,
	"include_vars": 18, "set_fact": 19, "register": 19, "role param": 20,
}

// Magic variables Ansible provides.
var Magic = map[string]string{
	"inventory_hostname": "the host's name in the inventory", "inventory_hostname_short": "the host's name up to the first dot",
	"hostvars": "every host's variables, by host name", "groups": "every group's hosts, by group name",
	"group_names": "the groups the current host is in", "play_hosts": "the hosts of the current play",
	"ansible_play_hosts": "the hosts of the current play still active", "ansible_play_batch": "the hosts of the current batch",
	"item": "the current loop item", "ansible_loop": "loop details (with loop_control: extended)",
	"omit": "leaves the module argument out", "playbook_dir": "the playbook's folder",
	"role_path": "the current role's folder", "role_name": "the current role's name",
	"inventory_dir": "the inventory file's folder", "ansible_facts": "facts gathered from the host",
	"ansible_check_mode": "true in --check runs", "ansible_version": "the running Ansible version",
	"ansible_env": "the host's environment variables (a fact)", "ansible_host": "the address to connect to",
	"ansible_user": "the user to connect as", "ansible_connection": "the connection plugin",
	"environment": "", "lookup": "a lookup plugin call", "query": "a lookup plugin call returning a list",
	"q": "a lookup plugin call returning a list", "vars": "all variables of the current host",
	"true": "", "false": "", "none": "", "True": "", "False": "", "None": "",
	"ansible_date_time": "the date and time on the host (a fact)", "ansible_hostname": "the host's name (a fact)",
	"ansible_distribution": "the OS distribution (a fact)", "ansible_os_family": "the OS family (a fact)",
	"ansible_default_ipv4": "the default IPv4 interface (a fact)", "ansible_run_tags": "the --tags given",
}

// def records a definition.
func (a *analyzer) def(name, kind, scope, value string, at *Source) {
	if name == "" {
		return
	}
	a.p.Variables = append(a.p.Variables, VarDef{Name: name, Kind: kind, Scope: scope, Value: short1(value), Rank: ranks[kind], Source: at})
}

func short1(s string) string {
	s = strings.TrimSpace(strings.SplitN(s, "\n", 2)[0])
	if len(s) > 100 {
		s = s[:97] + "…"
	}
	return s
}

// defsIn records every top-level key of a vars map.
func (a *analyzer) defsIn(m *yamlkit.Node, file, kind, scope string) {
	if m == nil || m.Kind != yamlkit.KindMap {
		return
	}
	for _, pr := range m.Pairs {
		a.def(pr.Key.Value, kind, scope, flowText(pr.Value), ptr(src(file, pr.Key)))
	}
}

// playVars records a play's vars and vars_files.
func (a *analyzer) playVars(n *yamlkit.Node, file, play string) {
	a.defsIn(n.Get("vars"), file, "play vars", play)
	for _, vf := range items(n.Get("vars_files")) {
		name := text(vf)
		if name == "" || strings.Contains(name, "{{") || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "~") {
			continue
		}
		p := path.Join(path.Dir(file), name)
		if body := a.load(p, "vars"); body != nil {
			a.defsIn(body, p, "vars_files", play)
		} else if !a.standalone {
			a.problem(yamlkit.SeverityError, "ansible-file-missing", ptr(src(file, vf)), "Can't find vars file "+name, "")
		}
	}
}

// taskVars records what a task defines: set_fact, register, vars,
// include_vars and role parameters.
func (a *analyzer) taskVars(n *yamlkit.Node, t *Task, file string) {
	scope := t.Role
	if t.Register != "" {
		a.def(t.Register, "register", scope, "result of "+t.Name, ptr(src(file, n.Get("register"))))
	}
	kind := "task vars"
	switch {
	case t.Kind == "block":
		kind = "block vars"
	case t.Kind == "include" || t.Kind == "import":
		if short(t.Module) == "include_role" || short(t.Module) == "import_role" {
			kind = "role param"
			scope = text(n.Get(t.Module).Get("name"))
		}
	}
	a.defsIn(n.Get("vars"), file, kind, scope)
	switch short(t.Module) {
	case "set_fact":
		args := n.Get(t.Module)
		for _, pr := range pairs(args) {
			if pr.Key.Value != "cacheable" {
				a.def(pr.Key.Value, "set_fact", scope, flowText(pr.Value), ptr(src(file, pr.Key)))
			}
		}
	case "include_vars":
		args := n.Get(t.Module)
		name := cmp.Or(text(args.Get("file")), text(args))
		if name == "" || strings.Contains(name, "{{") || args.Get("dir") != nil {
			break
		}
		cands := []string{path.Join(path.Dir(file), name)}
		if r := a.roles[t.Role]; r != nil && r.Path != "" {
			cands = append(cands, path.Join(r.Path, "vars", name))
		}
		if p := a.firstExisting(cands...); p != "" {
			a.defsIn(a.load(p, "vars"), p, "include_vars", scope)
		}
	}
}

// roleVars records a role's defaults and vars once.
func (a *analyzer) roleVars(r *Role) {
	for _, part := range []struct{ dir, kind string }{{"defaults", "role default"}, {"vars", "role vars"}} {
		p := a.firstExisting(path.Join(r.Path, part.dir, "main.yml"), path.Join(r.Path, part.dir, "main.yaml"))
		if p != "" {
			a.defsIn(a.load(p, "role"), p, part.kind, r.Name)
		}
	}
}

// variables adds group_vars and host_vars from the repository and
// orders every definition: by name, then highest precedence first.
func (a *analyzer) variables() {
	if !a.standalone {
		for f := range a.have {
			if !strings.HasPrefix(f, a.rootPrefix()) || !isYAML(f) {
				continue
			}
			parts := strings.Split(f, "/")
			for i := len(parts) - 2; i >= 0; i-- {
				if parts[i] != "group_vars" && parts[i] != "host_vars" {
					continue
				}
				scope := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(parts[i+1], ".yml"), ".yaml"), ".json")
				kind := parts[i]
				if kind == "group_vars" && scope == "all" {
					kind = "group_vars all"
				}
				a.defsIn(a.load(f, parts[i]), f, kind, scope)
				break
			}
		}
	}
	// A file included twice defines the same things twice.
	seen := map[string]bool{}
	a.p.Variables = slices.DeleteFunc(a.p.Variables, func(v VarDef) bool {
		k := v.Name + "\x00" + v.Kind + "\x00" + fileOr(v.Source) + "\x00" + strconv.Itoa(lineOr(v.Source))
		if seen[k] {
			return true
		}
		seen[k] = true
		return false
	})
	slices.SortStableFunc(a.p.Variables, func(x, y VarDef) int {
		return cmp.Or(cmp.Compare(x.Name, y.Name), cmp.Compare(y.Rank, x.Rank))
	})
}

func (a *analyzer) rootPrefix() string {
	if a.opts.Root == "" {
		return ""
	}
	return a.opts.Root + "/"
}

func isYAML(f string) bool {
	return strings.HasSuffix(f, ".yml") || strings.HasSuffix(f, ".yaml")
}

// Definitions lists where name is defined, highest precedence first.
func (p *Playbook) Definitions(name string) []VarDef {
	var out []VarDef
	for _, v := range p.Variables {
		if v.Name == name {
			out = append(out, v)
		}
	}
	return out
}

// --- Jinja references ---

// jinjaWords are Jinja's own words, never variables.
var jinjaWords = map[string]bool{
	"and": true, "or": true, "not": true, "in": true, "is": true, "if": true, "else": true, "elif": true, "endif": true,
	"for": true, "endfor": true, "set": true, "true": true, "false": true, "none": true, "True": true, "False": true,
	"None": true, "defined": true, "undefined": true, "block": true, "endblock": true, "with": true, "recursive": true,
}

// VarAt returns the variable name at byte offset i of a Jinja
// expression (the root of a.b.c, not a filter or test name), and its
// byte range; ok is false when i isn't on a variable.
func VarAt(expr string, i int) (name string, start, end int, ok bool) {
	if i < 0 || i >= len(expr) || !isIdent(expr[i]) {
		return "", 0, 0, false
	}
	start, end = i, i
	for start > 0 && isIdent(expr[start-1]) {
		start--
	}
	for end < len(expr) && isIdent(expr[end]) {
		end++
	}
	word := expr[start:end]
	if word[0] >= '0' && word[0] <= '9' || jinjaWords[word] {
		return "", 0, 0, false
	}
	// Attributes (x.attr), filters (| name), tests (is name) and
	// strings aren't variables.
	before := strings.TrimRight(expr[:start], " \t")
	switch {
	case strings.HasSuffix(before, "."), strings.HasSuffix(before, "|"),
		strings.HasSuffix(before, " is") || before == "is", strings.HasSuffix(before, " is not"):
		return "", 0, 0, false
	case inString(expr, start):
		return "", 0, 0, false
	}
	after := strings.TrimLeft(expr[end:], " ")
	if strings.HasPrefix(after, "=") && !strings.HasPrefix(after, "==") {
		return "", 0, 0, false // a keyword argument: default(value=…)
	}
	return word, start, end, true
}

func isIdent(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// inString reports whether offset i is inside a quoted string.
func inString(s string, i int) bool {
	var q byte
	for j := 0; j < i; j++ {
		switch {
		case q != 0 && s[j] == q:
			q = 0
		case q == 0 && (s[j] == '\'' || s[j] == '"'):
			q = s[j]
		}
	}
	return q != 0
}

// --- helpers ---

func pairs(n *yamlkit.Node) []*yamlkit.Pair {
	if n == nil || n.Kind != yamlkit.KindMap {
		return nil
	}
	return n.Pairs
}

func fileOr(s *Source) string {
	if s == nil {
		return ""
	}
	return s.File
}

func lineOr(s *Source) int {
	if s == nil {
		return 0
	}
	return s.Line
}
