package compose

import (
	"bytes"
	"cmp"
	"fmt"
	"path"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// ID is the provider id of Compose files.
const ID = "compose"

// Provider is the Compose lens in the provider registry: an outline
// that names services, diagnostics, hover and go to definition for the
// names services refer to and for ${VAR}, and ${VAR} tints.
type Provider struct{ provider.Provider }

// Register installs the lens in reg over the built-in detector.
func Register(reg *provider.Registry) {
	if base := reg.Lookup(ID); base != nil {
		if _, done := base.(Provider); !done {
			reg.Register(Provider{base})
		}
	}
}

func init() { Register(provider.Default) }

// Symbols names services, networks, volumes, secrets and configs.
func (p Provider) Symbols(d *yamlkit.Document) []provider.Symbol {
	syms := provider.Outline(d)
	if d == nil || d.Root == nil {
		return syms
	}
	labels := map[string]string{"services": "Service", "networks": "Network", "volumes": "Volume", "secrets": "Secret", "configs": "Config"}
	for i := range syms {
		s := &syms[i]
		kind, ok := labels[s.Name]
		if !ok {
			continue
		}
		group := d.Root.Get(s.Name)
		for j := range s.Children {
			c := &s.Children[j]
			name := c.Name
			c.Name = kind + " " + name
			if kind == "Service" {
				c.Detail = serviceDetail(group.Get(name))
			}
		}
	}
	return syms
}

// serviceDetail summarizes a service for the outline.
func serviceDetail(n *yamlkit.Node) string {
	var parts []string
	if img := text(n.Get("image")); img != "" {
		parts = append(parts, img)
	} else if b := n.Get("build"); b != nil {
		parts = append(parts, "build "+cmp.Or(text(b), text(b.Get("context")), "."))
	}
	var ports []string
	for _, it := range items(n.Get("ports")) {
		if pt := parsePort(it); pt.Published != "" {
			ports = append(ports, pt.Published+"→"+pt.Target)
		} else {
			ports = append(ports, pt.Target)
		}
	}
	if len(ports) > 0 {
		parts = append(parts, "ports "+strings.Join(ports, ", "))
	}
	var deps []string
	if d := n.Get("depends_on"); d != nil {
		deps = strs(d)
		for _, pr := range pairs(d) {
			deps = append(deps, pr.Key.Value)
		}
	}
	if len(deps) > 0 {
		parts = append(parts, "after "+strings.Join(deps, ", "))
	}
	return strings.Join(parts, " · ")
}

// Diagnostics reports what the file alone can tell; the web server
// replaces them with the whole project's.
func (p Provider) Diagnostics(f *provider.File) []yamlkit.Diagnostic {
	return Diagnose(AnalyzeFile(f, Options{}), f.Path)
}

// Expressions tints ${VAR} (interpolated before Compose runs anything).
func (p Provider) Expressions(f *provider.File) []yamlkit.Expression {
	var out []yamlkit.Expression
	if f.YAML != nil {
		out = append(out, f.YAML.Expressions...)
	}
	lines := bytes.Split(f.Content, []byte("\n"))
	off := 0
	for i, l := range lines {
		for _, r := range scanRefs(string(l)) {
			rng := yamlkit.Range{
				Start: yamlkit.Pos{Offset: off + r.start, Line: i + 1, Col: col(l, r.start)},
				End:   yamlkit.Pos{Offset: off + r.end, Line: i + 1, Col: col(l, r.end)},
			}
			out = append(out, yamlkit.Expression{Range: rng, Text: string(l[r.start:r.end]), Syntax: "compose", Phase: yamlkit.PhaseTemplate})
		}
		off += len(l) + 1
	}
	return out
}

// col is the 1-based column (in runes) of byte offset b in line.
func col(line []byte, b int) int {
	return len([]rune(string(line[:b]))) + 1
}

// AnalyzeFile analyzes the project an editor file belongs to: with
// opts.Files (a folder is open), the default layers and .env; without,
// the file alone.
func AnalyzeFile(f *provider.File, opts Options) *Project {
	if opts.Load == nil {
		content := f.Content
		opts.Load = func(p string) ([]byte, error) {
			if p == f.Path {
				return content, nil
			}
			return nil, fmt.Errorf("%s: not open", p)
		}
	}
	files := []string{f.Path}
	if opts.Files != nil {
		files = DefaultFiles(f.Path, opts.Files)
	}
	return Analyze(files, opts)
}

// Diagnose turns the project's problems located in file into editor
// diagnostics.
func Diagnose(p *Project, file string) []yamlkit.Diagnostic {
	var out []yamlkit.Diagnostic
	for _, pr := range p.Problems {
		if pr.Source == nil || pr.Source.Line == 0 || pr.Source.File != file {
			continue
		}
		out = append(out, yamlkit.Diagnostic{Severity: pr.Severity, Code: pr.Code, Message: pr.Message, Hint: pr.Hint, Source: "compose", Range: pr.Source.Range})
	}
	return out
}

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
	kind  string // var, service, network, volume, secret, config, file
	name  string
	rng   yamlkit.Range
	where *Source
	p     *Project
}

// at works out what the cursor is on and where that is defined.
func at(f *provider.File, pos yamlkit.Pos, opts Options) *target {
	if f.YAML == nil {
		return nil
	}
	// ${VAR} first: it can sit in any value.
	lines := bytes.Split(f.Content, []byte("\n"))
	if pos.Line >= 1 && pos.Line <= len(lines) {
		l := lines[pos.Line-1]
		for _, r := range scanRefs(string(l)) {
			if c1, c2 := col(l, r.start), col(l, r.end); pos.Col >= c1 && pos.Col < c2 {
				p := AnalyzeFile(f, opts)
				t := &target{kind: "var", name: r.name, p: p, rng: yamlkit.Range{Start: yamlkit.Pos{Line: pos.Line, Col: c1}, End: yamlkit.Pos{Line: pos.Line, Col: c2}}}
				for _, v := range p.Variables {
					if v.Name == r.name {
						t.where = v.Defined
					}
				}
				return t
			}
		}
	}
	doc := f.YAML.DocAt(pos)
	path, n := doc.PathAt(pos)
	if n == nil || len(path) < 3 || path[0].Key != "services" {
		return nil
	}
	field := path[2].Key
	t := &target{rng: n.Range}
	switch {
	case (field == "depends_on" || field == "links") && len(path) == 4:
		t.kind, t.name = "service", strings.SplitN(keyOrValue(path, n), ":", 2)[0]
		if !path[3].IsIndex {
			t.rng = keyRange(doc, path, n)
		}
	case field == "extends" && (len(path) == 3 || path[len(path)-1].Key == "service"):
		t.kind, t.name = "service", n.Value
	case field == "extends" && path[len(path)-1].Key == "file", field == "env_file":
		t.kind, t.name = "file", cmp.Or(n.Value, text(n.Get("path")))
	case field == "network_mode" && strings.HasPrefix(n.Value, "service:"):
		t.kind, t.name = "service", strings.TrimPrefix(n.Value, "service:")
	case field == "volumes_from" && len(path) == 4:
		t.kind, t.name = "service", strings.SplitN(n.Value, ":", 2)[0]
	case field == "networks" && len(path) == 4:
		t.kind, t.name = "network", keyOrValue(path, n)
		if !path[3].IsIndex {
			t.rng = keyRange(doc, path, n)
		}
	case field == "volumes" && len(path) >= 4:
		m := parseMount(n)
		if len(path) == 5 && n.Kind == yamlkit.KindScalar && path[4].Key == "source" {
			m = Mount{Type: "volume", From: n.Value}
		}
		if m.Type != "volume" || m.From == "" {
			return nil
		}
		t.kind, t.name = "volume", m.From
	case (field == "secrets" || field == "configs") && len(path) >= 4:
		name := n.Value
		if len(path) == 5 && path[4].Key != "source" {
			return nil
		}
		t.kind, t.name = strings.TrimSuffix(field, "s"), name
	default:
		return nil
	}
	if t.name == "" {
		return nil
	}
	t.p = AnalyzeFile(f, opts)
	switch t.kind {
	case "service":
		for _, s := range t.p.Services {
			if s.Name == t.name {
				t.where = s.Source
			}
		}
		// extends: {file, service} names a service of another file.
		if ext := doc.Root.Get("services").Get(path[1].Key).Get("extends"); field == "extends" && text(ext.Get("file")) != "" {
			t.where = serviceIn(resolvePath(f.Path, text(ext.Get("file"))), t.name, opts)
		}
	case "file":
		t.where = &Source{File: resolvePath(f.Path, t.name), Line: 1}
	default:
		for _, r := range t.p.resources(t.kind) {
			if r.Name == t.name {
				t.where = r.Source
			}
		}
	}
	return t
}

// keyOrValue is the name an entry of a list (the item) or a map (the
// key) holds.
func keyOrValue(p yamlkit.Path, n *yamlkit.Node) string {
	if last := p[len(p)-1]; !last.IsIndex {
		return last.Key
	}
	return text(n)
}

// keyRange is the range of the key a path ends on.
func keyRange(doc *yamlkit.Document, p yamlkit.Path, v *yamlkit.Node) yamlkit.Range {
	parent := doc.Root
	for _, s := range p[:len(p)-1] {
		parent = parent.Get(s.Key)
	}
	if pr := parent.Pair(p[len(p)-1].Key); pr != nil {
		return pr.Key.Range
	}
	if v != nil {
		return v.Range
	}
	return yamlkit.Range{}
}

// serviceIn locates a service's key in another file.
func serviceIn(file, name string, opts Options) *Source {
	if opts.Load == nil {
		return nil
	}
	data, err := opts.Load(file)
	if err != nil {
		return nil
	}
	for _, d := range yamlkit.Parse(data).Docs {
		if pr := d.Root.Get("services").Pair(name); pr != nil {
			return ptr(src(file, pr.Key))
		}
	}
	return nil
}

func resolvePath(from, rel string) string {
	return path.Join(path.Dir(from), rel)
}

func (p *Project) resources(kind string) []Resource {
	switch kind {
	case "network":
		return p.Networks
	case "volume":
		return p.Volumes
	case "secret":
		return p.Secrets
	case "config":
		return p.Configs
	}
	return nil
}

// DefinitionIn jumps to what the cursor refers to, in whatever file of
// the project it is written.
func DefinitionIn(f *provider.File, pos yamlkit.Pos, opts Options) []provider.Location {
	t := at(f, pos, opts)
	if t == nil || t.where == nil {
		return nil
	}
	w := t.where
	loc := provider.Location{Range: w.Range}
	if w.File != "" && w.File != f.Path {
		loc.Path = w.File
	}
	if w.Line > 0 && w.Range.Start.Line == 0 {
		loc.Range = yamlkit.Range{Start: yamlkit.Pos{Line: w.Line, Col: 1}, End: yamlkit.Pos{Line: w.Line, Col: 1}}
	}
	return []provider.Location{loc}
}

// HoverIn explains what the cursor is on with the whole project.
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
	where := func() {
		if t.where != nil && t.where.File != "" && t.where.File != f.Path {
			row("Defined in", fmt.Sprintf("%s:%d", t.where.File, t.where.Line))
		} else if t.where != nil && t.where.Line > 0 {
			row("Defined at", fmt.Sprintf("line %d", t.where.Line))
		}
	}
	switch t.kind {
	case "var":
		h.Title = "${" + t.name + "} · Compose variable"
		for _, v := range t.p.Variables {
			if v.Name != t.name {
				continue
			}
			switch {
			case v.From == "what-if":
				row("Value", quoteEmpty(v.Value)+" (what-if)")
			case v.Set:
				row("Value", quoteEmpty(v.Value))
				row("From", v.From)
			case v.From == "default":
				row("Value", quoteEmpty(v.Default)+" (the default: not set)")
			case opts.Files == nil:
				row("Value", "open the folder to read .env")
			default:
				row("Value", `not set: becomes ""`)
			}
			if v.Default != "" && v.From != "default" {
				row("Default", v.Default)
			}
			if v.Required {
				row("Required", "Compose stops when it isn't set")
			}
			row("Used", plural(len(v.Uses), "time"))
		}
		row("Note", "codec reads .env and what-if values, not your shell's environment")
	case "service":
		h.Title = "Service " + t.name
		var s *Service
		for i := range t.p.Services {
			if t.p.Services[i].Name == t.name {
				s = &t.p.Services[i]
			}
		}
		if s == nil {
			if t.where != nil {
				where() // extended from another file
			} else {
				row("Not found", "no service of this project has that name")
			}
			break
		}
		row("Image", s.Image)
		row("Build", s.Build)
		var ports []string
		for _, pt := range t.p.Ports {
			if pt.Service == s.Name {
				ports = append(ports, portText(pt))
			}
		}
		row("Ports", strings.Join(ports, ", "))
		var deps []string
		for _, d := range s.DependsOn {
			deps = append(deps, d.Service+label(depLabel(d)))
		}
		row("Starts after", strings.Join(deps, ", "))
		row("Health check", s.Health)
		if len(s.Profiles) > 0 {
			row("Profiles", strings.Join(s.Profiles, ", ")+" (not started by default)")
		}
		where()
	default:
		h.Title = strings.ToUpper(t.kind[:1]) + t.kind[1:] + " " + t.name
		var r *Resource
		for _, x := range t.p.resources(t.kind) {
			if x.Name == t.name {
				r = &x
			}
		}
		if r == nil {
			row("Not declared", "add it under "+t.kind+"s: at the top level")
			break
		}
		row("Detail", r.Detail)
		row("Used by", strings.Join(r.UsedBy, ", "))
		where()
	}
	return h
}

func label(s string) string {
	if s == "" {
		return ""
	}
	return " (" + s + ")"
}

func quoteEmpty(s string) string {
	if s == "" {
		return `""`
	}
	return s
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// portText shows a port as written in short syntax.
func portText(pt Port) string {
	s := pt.Target
	if pt.Published != "" {
		s = pt.Published + ":" + s
	}
	if pt.HostIP != "" {
		s = pt.HostIP + ":" + s
	}
	if pt.Protocol != "tcp" {
		s += "/" + pt.Protocol
	}
	return s
}
