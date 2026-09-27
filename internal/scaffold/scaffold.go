// Package scaffold speeds up writing new YAML: starters (templates with
// a small form), editor snippets and clone-with-rename. Everything it
// makes is text for the user to copy; codec never writes files.
//
// Starters are folders of text/template files. The built-in ones are
// embedded in the binary; personal ones live in the user's settings
// folder and are handed in as an fs.FS, so this package stays pure.
package scaffold

import (
	"bytes"
	"cmp"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"text/template"
	"text/template/parse"
	"unicode"

	"go.yaml.in/yaml/v3"
)

// Placeholders are written <% .name %>. The usual {{ }} would clash
// with what the generated files contain themselves (Helm, Argo, Jinja,
// GitHub ${{ }}), and [[ ]] with bash tests in CI scripts.
const (
	leftDelim  = "<%"
	rightDelim = "%>"
)

// Manifest is the optional starter.yaml of a starter folder.
const Manifest = "starter.yaml"

//go:embed all:starters
var builtinFS embed.FS

// Starter is one template the user can fill in.
type Starter struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Category    string  `json:"category"`
	Description string  `json:"description,omitempty"`
	Personal    bool    `json:"personal,omitempty"`
	Fields      []Field `json:"fields"`
	// Files are the output paths as written in the starter (they may
	// contain placeholders), for the list view.
	Files []string `json:"files"`

	tmpl  *template.Template
	specs []fileSpec
}

// Field is one input of a starter's form.
type Field struct {
	Name    string   `json:"name" yaml:"name"`
	Label   string   `json:"label" yaml:"label"`
	Help    string   `json:"help,omitempty" yaml:"help"`
	Type    string   `json:"type" yaml:"type"` // text (default), bool, choice
	Default string   `json:"default,omitempty" yaml:"default"`
	Choices []string `json:"choices,omitempty" yaml:"choices"`
	// Pattern is a named check (dns-label, dns-subdomain, port, number,
	// identifier, cron, image, host) or a regular expression.
	Pattern string `json:"pattern,omitempty" yaml:"pattern"`
	// Optional text fields may stay empty.
	Optional bool `json:"optional,omitempty" yaml:"optional"`
	// When names a bool field; the form shows this field only when it
	// is on.
	When string `json:"when,omitempty" yaml:"when"`
}

// File is one generated file.
type File struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// FieldError is a value the form must fix.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type fileSpec struct {
	template string // name in the template set
	path     string // output path, may hold placeholders
}

// manifest is starter.yaml.
type manifest struct {
	Title       string  `yaml:"title"`
	Category    string  `yaml:"category"`
	Description string  `yaml:"description"`
	Fields      []Field `yaml:"fields"`
	Files       []struct {
		Template string `yaml:"template"`
		Path     string `yaml:"path"`
	} `yaml:"files"`
}

var builtins = func() []*Starter {
	sub, err := fs.Sub(builtinFS, "starters")
	if err != nil {
		panic(err)
	}
	ss, errs := Load(sub, false)
	if len(errs) > 0 {
		panic(errors.Join(errs...)) // a broken built-in is a bug; tests catch it
	}
	return ss
}()

// Builtin returns the starters embedded in codec.
func Builtin() []*Starter { return builtins }

// Find returns the starter with id, or nil.
func Find(ss []*Starter, id string) *Starter {
	for _, s := range ss {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// Load reads every starter in fsys: each folder is one starter (with an
// optional starter.yaml), and for personal templates each loose file is
// a single-file starter too. Broken starters are reported and skipped.
func Load(fsys fs.FS, personal bool) ([]*Starter, []error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, []error{err}
	}
	var out []*Starter
	var errs []error
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		var s *Starter
		var err error
		switch {
		case e.IsDir():
			s, err = loadDir(fsys, name)
		case personal:
			s, err = loadFile(fsys, name)
		default:
			continue
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
			continue
		}
		s.Personal = personal
		if personal {
			s.ID = "personal/" + s.ID
			s.Category = cmp.Or(s.Category, "Personal")
		}
		out = append(out, s)
	}
	slices.SortStableFunc(out, func(a, b *Starter) int {
		return cmp.Or(cmp.Compare(a.Category, b.Category), cmp.Compare(a.Title, b.Title))
	})
	return out, errs
}

func newSet(name string) *template.Template {
	return template.New(name).Delims(leftDelim, rightDelim).Funcs(funcs).Option("missingkey=error")
}

// loadDir reads a starter folder.
func loadDir(fsys fs.FS, dir string) (*Starter, error) {
	s := &Starter{ID: dir, Title: dir, tmpl: newSet(dir)}
	var m manifest
	if data, err := fs.ReadFile(fsys, path.Join(dir, Manifest)); err == nil {
		if err := yaml.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("%s: %w", Manifest, err)
		}
		s.Title = cmp.Or(m.Title, dir)
		s.Category, s.Description, s.Fields = m.Category, m.Description, m.Fields
	}

	// Every other file in the folder (recursively) is a template.
	err := fs.WalkDir(fsys, dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel := strings.TrimPrefix(p, dir+"/")
		if rel == Manifest {
			return nil
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		if _, err := s.tmpl.New(rel).Parse(string(data)); err != nil {
			return err
		}
		s.specs = append(s.specs, fileSpec{template: rel, path: strings.TrimSuffix(rel, ".tmpl")})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(m.Files) > 0 {
		// The manifest chooses the files and their output names.
		s.specs = s.specs[:0]
		for _, f := range m.Files {
			if s.tmpl.Lookup(f.Template) == nil {
				return nil, fmt.Errorf("%s: no template %q", Manifest, f.Template)
			}
			s.specs = append(s.specs, fileSpec{template: f.Template, path: cmp.Or(f.Path, strings.TrimSuffix(f.Template, ".tmpl"))})
		}
	}
	if len(s.specs) == 0 {
		return nil, errors.New("no template files")
	}
	return s, s.finish()
}

// loadFile reads a single-file personal template; its fields are the
// placeholders it uses.
func loadFile(fsys fs.FS, name string) (*Starter, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, err
	}
	out := strings.TrimSuffix(name, ".tmpl")
	s := &Starter{ID: out, Title: out, tmpl: newSet(name)}
	if _, err := s.tmpl.New(name).Parse(string(data)); err != nil {
		return nil, err
	}
	s.specs = []fileSpec{{template: name, path: out}}
	return s, s.finish()
}

// finish parses the output paths, adds fields the templates use but
// the manifest doesn't declare, and checks the declarations.
func (s *Starter) finish() error {
	used := map[string]bool{} // used only as an if/with condition
	var order []string
	note := func(name string, cond bool) {
		if _, seen := used[name]; !seen {
			order = append(order, name)
			used[name] = cond
		} else if !cond {
			used[name] = false
		}
	}
	for i, sp := range s.specs {
		pt := "path:" + sp.template
		if _, err := s.tmpl.New(pt).Parse(sp.path); err != nil {
			return fmt.Errorf("output path %q: %w", sp.path, err)
		}
		s.specs[i].path = pt
		s.Files = append(s.Files, sp.path)
		fieldsIn(s.tmpl.Lookup(pt).Tree.Root, note)
		fieldsIn(s.tmpl.Lookup(sp.template).Tree.Root, note)
	}
	declared := map[string]bool{}
	for i, f := range s.Fields {
		if f.Name == "" {
			return errors.New("a field has no name")
		}
		declared[f.Name] = true
		s.Fields[i].Type = cmp.Or(f.Type, "text")
		s.Fields[i].Label = cmp.Or(f.Label, humanize(f.Name))
		switch s.Fields[i].Type {
		case "text", "bool":
		case "choice":
			if len(f.Choices) == 0 {
				return fmt.Errorf("field %s: a choice needs choices", f.Name)
			}
		default:
			return fmt.Errorf("field %s: unknown type %q", f.Name, f.Type)
		}
		if _, err := checker(f.Pattern); err != nil {
			return fmt.Errorf("field %s: %w", f.Name, err)
		}
	}
	for _, name := range order {
		if !declared[name] {
			f := Field{Name: name, Label: humanize(name), Type: "text"}
			if used[name] {
				f.Type = "bool"
			}
			s.Fields = append(s.Fields, f)
		}
	}
	return nil
}

// fieldsIn walks a template's parse tree and reports every top-level
// field it reads (.name), and whether it is only an if/with condition.
func fieldsIn(n parse.Node, note func(name string, cond bool)) {
	var pipe func(p *parse.PipeNode, cond bool)
	var walk func(n parse.Node)
	arg := func(a parse.Node, cond bool) {
		switch a := a.(type) {
		case *parse.FieldNode:
			note(a.Ident[0], cond)
		case *parse.ChainNode:
			if f, ok := a.Node.(*parse.FieldNode); ok {
				note(f.Ident[0], false)
			}
		case *parse.PipeNode:
			pipe(a, false)
		}
	}
	pipe = func(p *parse.PipeNode, cond bool) {
		if p == nil {
			return
		}
		for _, c := range p.Cmds {
			// Only a bare `if .x` makes x a switch; `if eq .x "a"` compares text.
			alone := cond && len(p.Cmds) == 1 && len(c.Args) == 1
			for _, a := range c.Args {
				arg(a, alone)
			}
		}
	}
	walk = func(n parse.Node) {
		switch n := n.(type) {
		case *parse.ListNode:
			if n != nil {
				for _, c := range n.Nodes {
					walk(c)
				}
			}
		case *parse.ActionNode:
			pipe(n.Pipe, false)
		case *parse.IfNode:
			pipe(n.Pipe, true)
			walk(n.List)
			walk(n.ElseList)
		case *parse.WithNode:
			pipe(n.Pipe, true)
			walk(n.List)
			walk(n.ElseList)
		case *parse.RangeNode:
			pipe(n.Pipe, false)
			walk(n.List)
			walk(n.ElseList)
		case *parse.TemplateNode:
			pipe(n.Pipe, false)
		}
	}
	walk(n)
}

// Defaults returns the default value of every field.
func (s *Starter) Defaults() map[string]string {
	out := map[string]string{}
	for _, f := range s.Fields {
		out[f.Name] = f.Default
	}
	return out
}

// Render fills the starter in. Values missing from values take their
// defaults. Files that come out empty (a part switched off) are left
// out. Field problems are returned instead of files.
func (s *Starter) Render(values map[string]string) ([]File, []FieldError, error) {
	data := map[string]any{}
	var problems []FieldError
	for _, f := range s.Fields {
		v, ok := values[f.Name]
		if !ok {
			v = f.Default
		}
		v = strings.TrimSpace(v)
		if f.Type == "bool" {
			data[f.Name] = v == "true"
			continue
		}
		data[f.Name] = v
		if msg := validate(f, v); msg != "" && !hidden(s.Fields, f, values) {
			problems = append(problems, FieldError{Field: f.Name, Message: msg})
		}
	}
	if len(problems) > 0 {
		return nil, problems, nil
	}

	var out []File
	for _, sp := range s.specs {
		var p, body bytes.Buffer
		if err := s.tmpl.ExecuteTemplate(&p, sp.path, data); err != nil {
			return nil, nil, err
		}
		if err := s.tmpl.ExecuteTemplate(&body, sp.template, data); err != nil {
			return nil, nil, err
		}
		text := tidy(body.String())
		name := strings.TrimSpace(p.String())
		if text == "" || name == "" {
			continue
		}
		if !fs.ValidPath(name) {
			return nil, nil, fmt.Errorf("output path %q is not a relative path", name)
		}
		out = append(out, File{Path: name, Content: text})
	}
	return out, nil, nil
}

// hidden reports whether f is switched off by its When field.
func hidden(fields []Field, f Field, values map[string]string) bool {
	if f.When == "" {
		return false
	}
	for _, w := range fields {
		if w.Name == f.When {
			v, ok := values[w.Name]
			if !ok {
				v = w.Default
			}
			return v != "true"
		}
	}
	return false
}

// validate checks one text or choice value; "" = fine.
func validate(f Field, v string) string {
	if v == "" {
		if f.Optional {
			return ""
		}
		return "Required"
	}
	if f.Type == "choice" && !slices.Contains(f.Choices, v) {
		return "Choose one of " + strings.Join(f.Choices, ", ")
	}
	check, _ := checker(f.Pattern)
	return check(v)
}

var (
	reLabel     = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	reSubdomain = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	reIdent     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	reHost      = regexp.MustCompile(`^(\*\.)?[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	reCronField = regexp.MustCompile(`^[0-9A-Za-z*/,?LW#-]+$`)
)

// checker returns the check for a pattern: a named one or a regular
// expression that must match the whole value.
func checker(pattern string) (func(string) string, error) {
	match := func(re *regexp.Regexp, msg string) func(string) string {
		return func(v string) string {
			if re.MatchString(v) {
				return ""
			}
			return msg
		}
	}
	switch pattern {
	case "":
		return func(string) string { return "" }, nil
	case "dns-label":
		return func(v string) string {
			if len(v) > 63 {
				return "At most 63 characters"
			}
			return match(reLabel, "Lower-case letters, digits and '-', starting and ending with a letter or digit")(v)
		}, nil
	case "dns-subdomain":
		return func(v string) string {
			if len(v) > 253 {
				return "At most 253 characters"
			}
			return match(reSubdomain, "Lower-case letters, digits, '-' and '.'")(v)
		}, nil
	case "host":
		return match(reHost, "A host name such as app.example.com"), nil
	case "identifier":
		return match(reIdent, "Letters, digits and '_', not starting with a digit"), nil
	case "port":
		return func(v string) string {
			if n, err := strconv.Atoi(v); err != nil || n < 1 || n > 65535 {
				return "A port number from 1 to 65535"
			}
			return ""
		}, nil
	case "number":
		return func(v string) string {
			if _, err := strconv.Atoi(v); err != nil {
				return "A whole number"
			}
			return ""
		}, nil
	case "image":
		return func(v string) string {
			if strings.ContainsAny(v, " \t") {
				return "An image reference has no spaces"
			}
			return ""
		}, nil
	case "cron":
		return func(v string) string {
			if strings.HasPrefix(v, "@") {
				return ""
			}
			parts := strings.Fields(v)
			if len(parts) != 5 {
				return "Five fields: minute hour day-of-month month day-of-week"
			}
			for _, p := range parts {
				if !reCronField.MatchString(p) {
					return "Not a cron field: " + p
				}
			}
			return ""
		}, nil
	}
	re, err := regexp.Compile("^(?:" + pattern + ")$")
	if err != nil {
		return nil, fmt.Errorf("pattern: %w", err)
	}
	return match(re, "Must match "+pattern), nil
}

// tidy trims what conditionals leave behind: trailing spaces, runs of
// blank lines, blank lines at the start and end.
func tidy(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	var out []string
	blank := 0
	for _, l := range lines {
		l = strings.TrimRight(l, " \t")
		if l == "" {
			blank++
			continue
		}
		if blank > 0 && len(out) > 0 {
			out = append(out, "")
		}
		blank = 0
		out = append(out, l)
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n") + "\n"
}

// funcs are the helpers templates may use.
var funcs = template.FuncMap{
	"quote": strconv.Quote,
	"yaml":  yamlString,
	"lower": strings.ToLower,
	"upper": strings.ToUpper,
	"snake": func(s string) string { return strings.NewReplacer("-", "_", ".", "_", " ", "_").Replace(s) },
	"env": func(s string) string {
		return strings.ToUpper(strings.NewReplacer("-", "_", ".", "_", " ", "_").Replace(s))
	},
	"title": func(s string) string {
		if s == "" {
			return s
		}
		return strings.ToUpper(s[:1]) + s[1:]
	},
}

// yamlString writes s as a YAML scalar that reads back as the same
// string: plain when that is safe, double-quoted otherwise.
func yamlString(s string) string {
	if plainSafe(s) {
		return s
	}
	return strconv.Quote(s)
}

var reNotPlain = regexp.MustCompile(`^(?i:true|false|yes|no|on|off|y|n|null|~|[-+]?(\.inf|\.nan|[0-9][0-9_.:eE+-]*|0x[0-9a-f]+|0o[0-7]+))$`)

func plainSafe(s string) bool {
	if s == "" || s != strings.TrimSpace(s) || reNotPlain.MatchString(s) {
		return false
	}
	if strings.ContainsAny(s[:1], "-?:,[]{}#&*!|>'\"%@`") {
		return false
	}
	return !strings.Contains(s, ": ") && !strings.Contains(s, " #") && !strings.HasSuffix(s, ":") &&
		!strings.ContainsAny(s, "\n\t")
}

// humanize turns a field name into a label: "ingressClass" and
// "ingress_class" become "Ingress class".
func humanize(name string) string {
	var b strings.Builder
	for i, r := range name {
		switch {
		case r == '_' || r == '-' || r == '.':
			b.WriteByte(' ')
		case i > 0 && unicode.IsUpper(r) && !unicode.IsUpper(rune(name[i-1])):
			b.WriteByte(' ')
			b.WriteRune(unicode.ToLower(r))
		case i == 0:
			b.WriteRune(unicode.ToUpper(r))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
