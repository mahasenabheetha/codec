package ansible

import (
	"bytes"
	"cmp"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Provider ids.
const (
	PlaybookID  = "ansible-playbook"
	InventoryID = "ansible-inventory"
)

// Provider is the Ansible lens for playbooks and task files: an
// outline in execution order, Jinja tints, diagnostics, and hover / go
// to definition for variables, roles, handlers and included files.
type Provider struct{ provider.Provider }

// InventoryProvider names groups and hosts in YAML inventories.
type InventoryProvider struct{ provider.Provider }

// Register installs the lenses in reg over the built-in detectors.
func Register(reg *provider.Registry) {
	if base := reg.Lookup(PlaybookID); base != nil {
		if _, done := base.(Provider); !done {
			reg.Register(Provider{base})
		}
	}
	if base := reg.Lookup(InventoryID); base != nil {
		if _, done := base.(InventoryProvider); !done {
			reg.Register(InventoryProvider{base})
		}
	}
}

func init() { Register(provider.Default) }

// --- outline ---

// Symbols lists plays and their parts in the order Ansible runs them
// (pre_tasks, roles, tasks, post_tasks, then handlers), whatever their
// order in the file.
func (p Provider) Symbols(d *yamlkit.Document) []provider.Symbol {
	if d == nil || d.Root == nil {
		return nil
	}
	pb := Analyze("", nil, &yamlkit.File{Docs: []*yamlkit.Document{d}}, Options{})
	if pb.Kind == "tasks" {
		return taskSymbols(pb.Tasks)
	}
	var out []provider.Symbol
	for _, pl := range pb.Plays {
		if pl.Source == nil || pl.Imported != "" {
			continue
		}
		s := provider.Symbol{Name: "Play " + pl.Name, Detail: "hosts " + pl.Hosts, Kind: "map", Range: pl.Source.Range}
		if strings.HasPrefix(pl.Name, "import_playbook") {
			s.Name, s.Detail = pl.Name, ""
		}
		for _, st := range pl.Steps {
			if st.Kind == "section" && st.Source != nil {
				s.Children = append(s.Children, provider.Symbol{Name: st.Name, Detail: plural(len(st.Children), "step"), Kind: "seq", Range: st.Source.Range, Children: taskSymbols(st.Children)})
			}
		}
		if len(pl.Handlers) > 0 && pl.Handlers[0].Source != nil && pl.Handlers[0].Source.File == "" {
			s.Children = append(s.Children, provider.Symbol{Name: "handlers", Detail: plural(len(pl.Handlers), "handler"), Kind: "seq", Range: pl.Handlers[0].Source.Range, Children: taskSymbols(pl.Handlers)})
		}
		out = append(out, s)
	}
	for _, pl := range pb.Plays {
		if pl.Imported != "" && pl.Source != nil && pl.Source.File == "" {
			out = append(out, provider.Symbol{Name: pl.Name, Kind: "map", Range: pl.Source.Range})
		}
	}
	return out
}

// taskSymbols names tasks with their module.
func taskSymbols(ts []Task) []provider.Symbol {
	var out []provider.Symbol
	for _, t := range ts {
		if t.Source == nil || t.Source.File != "" {
			continue // steps of other files
		}
		s := provider.Symbol{Name: t.Name, Kind: "map", Range: t.Source.Range}
		switch t.Kind {
		case "role":
			s.Name, s.Detail = "Role "+t.Name, "role"
		case "block":
			s.Name, s.Detail = "Block "+strings.TrimPrefix(t.Name, "block"), "block"
			s.Name = strings.TrimSpace(s.Name)
		case "section":
		default:
			s.Name = "Task " + t.Name
			if !strings.HasPrefix(t.Name, short(t.Module)) {
				s.Detail = short(t.Module)
			}
			if t.Detail != "" && (t.Kind == "include" || t.Kind == "import") {
				s.Detail = strings.TrimPrefix(s.Detail+" · "+t.Detail, " · ")
			}
		}
		if t.When != "" {
			s.Detail = strings.TrimPrefix(s.Detail+" · when "+t.When, " · ")
		}
		s.Children = taskSymbols(t.Children)
		out = append(out, s)
	}
	return out
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// Symbols names groups and hosts.
func (p InventoryProvider) Symbols(d *yamlkit.Document) []provider.Symbol {
	syms := provider.Outline(d)
	var label func(ss []provider.Symbol, parent string)
	label = func(ss []provider.Symbol, parent string) {
		for i := range ss {
			s := &ss[i]
			switch parent {
			case "", "children":
				s.Name, s.Detail = "Group "+s.Name, ""
			case "hosts":
				s.Name = "Host " + s.Name
				if n := hostCount(strings.TrimPrefix(s.Name, "Host ")); n > 1 {
					s.Detail = plural(n, "host")
				} else {
					s.Detail = ""
				}
			default:
				continue
			}
			for j := range s.Children {
				c := &s.Children[j]
				if c.Name == "hosts" || c.Name == "children" {
					label(c.Children, c.Name)
				}
			}
		}
	}
	label(syms, "")
	return syms
}

// --- diagnostics and tints ---

// Diagnostics reports what the file alone can tell; the web server
// replaces them with the folder's.
func (p Provider) Diagnostics(f *provider.File) []yamlkit.Diagnostic {
	return Diagnose(Analyze(f.Path, f.Content, f.YAML, Options{}), f.Path)
}

// Diagnose turns an analysis's problems located in file into editor
// diagnostics.
func Diagnose(pb *Playbook, file string) []yamlkit.Diagnostic {
	var out []yamlkit.Diagnostic
	for _, pr := range pb.Problems {
		if pr.Source == nil || pr.Source.Line == 0 || (pr.Source.File != "" && pr.Source.File != file) {
			continue
		}
		out = append(out, yamlkit.Diagnostic{Severity: pr.Severity, Code: pr.Code, Message: pr.Message, Hint: pr.Hint, Source: "ansible", Range: pr.Source.Range})
	}
	return out
}

// Expressions marks {{ }} as Jinja (the YAML reader can only tell Jinja
// from Go templates when it sees {% %}).
func (p Provider) Expressions(f *provider.File) []yamlkit.Expression {
	if f.YAML == nil {
		return nil
	}
	out := slices.Clone(f.YAML.Expressions)
	for i := range out {
		if out[i].Syntax == "go-template" {
			out[i].Syntax = "jinja"
		}
	}
	return out
}

// Expressions marks inventory {{ }} as Jinja too.
func (p InventoryProvider) Expressions(f *provider.File) []yamlkit.Expression {
	return Provider{}.Expressions(f)
}

// --- hover and definition ---

// Hover explains what the cursor is on, within the file.
func (p Provider) Hover(f *provider.File, pos yamlkit.Pos) *provider.Hover {
	return HoverIn(f, pos, Options{})
}

// Definition jumps to what the cursor refers to, within the file.
func (p Provider) Definition(f *provider.File, pos yamlkit.Pos) []provider.Location {
	return DefinitionIn(f, pos, Options{})
}

// target is what the cursor is on.
type target struct {
	kind  string // var, role, file, handler
	name  string
	rng   yamlkit.Range
	where []Source
	pb    *Playbook
	role  *Role
}

// rawJinja are keys whose values are Jinja expressions without braces.
var rawJinja = map[string]bool{"when": true, "failed_when": true, "changed_when": true, "until": true, "that": true}

var (
	includeTasks = map[string]bool{"include_tasks": true, "import_tasks": true}
	includeRole  = map[string]bool{"include_role": true, "import_role": true}
)

func at(f *provider.File, pos yamlkit.Pos, opts Options) *target {
	if f.YAML == nil {
		return nil
	}
	doc := f.YAML.DocAt(pos)
	p, n := doc.PathAt(pos)
	lines := bytes.Split(f.Content, []byte("\n"))
	if pos.Line < 1 || pos.Line > len(lines) {
		return nil
	}
	line := string(bytes.TrimRight(lines[pos.Line-1], "\r"))
	byteCol := byteAt(line, pos.Col)

	// A variable inside {{ }}, or in a when:-like value. Only the
	// expression's own text is scanned: YAML quotes around it aren't
	// Jinja strings.
	from, to := -1, -1
	if i := slices.IndexFunc(f.YAML.Expressions, func(e yamlkit.Expression) bool { return e.Range.Contains(pos) }); i >= 0 {
		if e := f.YAML.Expressions[i]; e.Range.Start.Line == pos.Line {
			from, to = byteAt(line, e.Range.Start.Col), len(line)
			if e.Range.End.Line == pos.Line {
				to = byteAt(line, e.Range.End.Col)
			}
		}
	} else if n != nil && n.Kind == yamlkit.KindScalar && n.Range.Contains(pos) && len(p) > 0 && n.Range.Start.Line == pos.Line {
		last := p[len(p)-1]
		if rawJinja[last.Key] || (last.IsIndex && len(p) > 1 && rawJinja[p[len(p)-2].Key]) {
			from, to = byteAt(line, n.Range.Start.Col), len(line)
			if n.Range.End.Line == pos.Line {
				to = byteAt(line, n.Range.End.Col)
			}
			if n.Style == yamlkit.StyleDouble || n.Style == yamlkit.StyleSingle {
				from, to = from+1, max(from+1, to-1)
			}
		}
	}
	if from >= 0 && byteCol >= from && byteCol < to {
		name, s, e, ok := VarAt(line[from:to], byteCol-from)
		if !ok {
			return nil
		}
		s, e = s+from, e+from
		t := &target{kind: "var", name: name, rng: lineRange(line, pos.Line, s, e)}
		t.pb = Analyze(f.Path, f.Content, f.YAML, opts)
		for _, d := range t.pb.Definitions(name) {
			if d.Source != nil {
				t.where = append(t.where, *d.Source)
			}
		}
		return t
	}
	if n == nil || n.Kind != yamlkit.KindScalar || len(p) == 0 {
		return nil
	}
	key := func(i int) string { // the key i segments from the end ("" for a list index)
		if len(p) < i {
			return ""
		}
		return short(p[len(p)-i].Key)
	}
	t := &target{rng: n.Range, name: n.Value}
	switch {
	case key(1) == "notify" || (p[len(p)-1].IsIndex && key(2) == "notify"):
		t.kind = "handler"
	case includeTasks[key(1)] || (key(1) == "file" && includeTasks[key(2)]):
		t.kind = "file"
	case key(1) == "import_playbook", p[len(p)-1].IsIndex && key(2) == "vars_files",
		key(1) == "include_vars" || (key(1) == "file" && key(2) == "include_vars"):
		t.kind = "file"
	case key(1) == "src" && (key(2) == "template" || key(2) == "copy" || key(2) == "script"):
		t.kind = "file"
	case p[len(p)-1].IsIndex && key(2) == "roles",
		(key(1) == "role" || key(1) == "name") && len(p) >= 3 && p[len(p)-2].IsIndex && key(3) == "roles",
		key(1) == "name" && includeRole[key(2)]:
		t.kind = "role"
	default:
		return nil
	}
	if t.name == "" || strings.Contains(t.name, "{{") {
		return nil
	}
	t.pb = Analyze(f.Path, f.Content, f.YAML, opts)
	a := &analyzer{have: map[string]bool{}}
	for _, x := range opts.Files {
		a.have[x] = true
	}
	switch t.kind {
	case "handler":
		for _, h := range t.pb.handlers() {
			if h.Name == t.name || slices.Contains(h.Listen, t.name) {
				if h.Source != nil {
					s := *h.Source
					if s.File == "" {
						s.File = f.Path
					}
					t.where = append(t.where, s)
				}
			}
		}
	case "role":
		for i, r := range t.pb.Roles {
			if r.Name == t.name {
				t.role = &t.pb.Roles[i]
				if r.Found {
					main := cmp.Or(a.firstExisting(path.Join(r.Path, "tasks/main.yml"), path.Join(r.Path, "tasks/main.yaml")), path.Join(r.Path, "tasks/main.yml"))
					t.where = []Source{{File: main, Line: 1}}
				}
			}
		}
	case "file":
		dir := path.Dir(f.Path)
		cands := []string{path.Join(dir, t.name)}
		_, roleDir := roleOf(f.Path)
		switch {
		case key(2) == "template" && roleDir != "":
			cands = append([]string{path.Join(roleDir, "templates", t.name)}, cands...)
		case key(2) == "template":
			cands = append(cands, path.Join(dir, "templates", t.name))
		case (key(2) == "copy" || key(2) == "script") && roleDir != "":
			cands = append([]string{path.Join(roleDir, "files", t.name)}, cands...)
		case key(2) == "copy" || key(2) == "script":
			cands = append(cands, path.Join(dir, "files", t.name))
		case roleDir != "" && (includeTasks[key(1)] || includeTasks[key(2)]):
			cands = append(cands, path.Join(roleDir, "tasks", t.name))
		case roleDir != "":
			cands = append(cands, path.Join(roleDir, "vars", t.name))
		}
		if found := a.firstExisting(cands...); found != "" {
			t.where = []Source{{File: found, Line: 1}}
		}
	}
	return t
}

// handlers are every handler the analysis saw.
func (pb *Playbook) handlers() []Task {
	out := slices.Clone(pb.Handlers)
	for _, pl := range pb.Plays {
		out = append(out, pl.Handlers...)
	}
	return out
}

func lineRange(line string, n, s, e int) yamlkit.Range {
	col := func(b int) int { return len([]rune(line[:b])) + 1 }
	return yamlkit.Range{Start: yamlkit.Pos{Line: n, Col: col(s)}, End: yamlkit.Pos{Line: n, Col: col(e)}}
}

// DefinitionIn jumps to what the cursor refers to: a variable's
// definitions (highest precedence first), a role's tasks, a handler,
// an included file.
func DefinitionIn(f *provider.File, pos yamlkit.Pos, opts Options) []provider.Location {
	t := at(f, pos, opts)
	if t == nil {
		return nil
	}
	var out []provider.Location
	for _, w := range t.where {
		loc := provider.Location{Range: w.Range}
		if w.File != "" && w.File != f.Path {
			loc.Path = w.File
		}
		if w.Range.Start.Line == 0 {
			loc.Range = yamlkit.Range{Start: yamlkit.Pos{Line: max(w.Line, 1), Col: 1}, End: yamlkit.Pos{Line: max(w.Line, 1), Col: 1}}
		}
		out = append(out, loc)
	}
	return out
}

// HoverIn explains what the cursor is on, reading the folder when opts
// has it.
func HoverIn(f *provider.File, pos yamlkit.Pos, opts Options) *provider.Hover {
	t := at(f, pos, opts)
	if t == nil || t.kind == "file" {
		return nil
	}
	h := &provider.Hover{Range: t.rng}
	row := func(label, value string) {
		if value != "" {
			h.Rows = append(h.Rows, provider.HoverRow{Label: label, Value: value})
		}
	}
	rel := func(s *Source) string {
		if s == nil {
			return ""
		}
		if s.File == "" || s.File == f.Path {
			return fmt.Sprintf("line %d", s.Line)
		}
		return fmt.Sprintf("%s:%d", s.File, s.Line)
	}
	switch t.kind {
	case "var":
		h.Title = t.name + " · variable"
		defs := t.pb.Definitions(t.name)
		if doc, ok := Magic[t.name]; ok && len(defs) == 0 {
			row("Provided by Ansible", cmp.Or(doc, "a Jinja/Ansible name"))
			break
		}
		if len(defs) == 0 {
			if opts.Files == nil {
				row("Defined", "open the folder to look in roles, vars files and group_vars")
			} else {
				row("Defined", "not in the files codec read: it may come from the inventory, extra vars (-e), a fact or a registered result")
			}
			break
		}
		for i, d := range defs {
			if i == 6 {
				row("…", plural(len(defs)-6, "more definition"))
				break
			}
			label := d.Kind
			if d.Scope != "" {
				label += " (" + d.Scope + ")"
			}
			if i == 0 && len(defs) > 1 {
				label = "Wins: " + label
			}
			row(label, d.Value+" · "+rel(d.Source))
		}
		if len(defs) > 1 {
			h.Code = "Order is Ansible's variable precedence, highest first; group_vars and host_vars apply only to hosts in that group or host."
		}
	case "role":
		h.Title = "Role " + t.name
		switch {
		case t.role == nil:
		case t.role.External:
			row("From", t.role.Origin()+" (not in the folder)")
		case !t.role.Found && opts.Files == nil:
			row("Found", "open the folder to look for it")
		case !t.role.Found:
			row("Found", "no: not in roles/, roles_path, or roles/ above")
		default:
			row("Folder", t.role.Path)
			row("Has", strings.Join(t.role.Parts, ", "))
			if len(t.role.Dependencies) > 0 {
				row("Depends on", strings.Join(t.role.Dependencies, ", ")+" (run first)")
			}
			var defaults int
			for _, v := range t.pb.Variables {
				if v.Kind == "role default" && v.Scope == t.name {
					defaults++
				}
			}
			if defaults > 0 {
				row("Defaults", plural(defaults, "variable"))
			}
		}
	case "handler":
		h.Title = "Handler " + t.name
		if len(t.where) == 0 {
			row("Found", cmp.Or(map[bool]string{true: "open the folder to see role handlers"}[opts.Files == nil], "no handler has this name or listens to it"))
		}
		for _, w := range t.where {
			row("Defined at", rel(&w))
		}
		row("Runs", "at the end of the section that notified it, once, if the task changed something")
	}
	return h
}

// byteAt is the byte offset of 1-based rune column col in line.
func byteAt(line string, col int) int {
	r := []rune(line)
	return len(string(r[:min(max(col-1, 0), len(r))]))
}
