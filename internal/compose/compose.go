// Package compose reads Docker Compose projects the way `docker compose
// config` does: every file is interpolated from .env and what-if
// values, then the files (compose.yaml, then its override, or any files
// chosen as layers) are merged with the Compose merge rules, services
// are extended, and !reset/!override are applied. yamlkit's layered
// merge keeps, for every field, the file that set it.
//
// It is an engine package: files come through Options.Load, and the
// process environment is never read (what-if values stand in for it).
package compose

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

// Project is a Compose project: its files merged and read.
type Project struct {
	Name       string     `json:"name,omitempty"`
	Files      []File     `json:"files"`      // everything read: layers, includes, extends, the env file
	Candidates []string   `json:"candidates"` // other Compose files in the folder that can be layered
	EnvFile    string     `json:"envFile,omitempty"`
	Services   []Service  `json:"services"`
	Networks   []Resource `json:"networks"`
	Volumes    []Resource `json:"volumes"`
	Secrets    []Resource `json:"secrets"`
	Configs    []Resource `json:"configs"`
	Variables  []Variable `json:"variables"`
	Edges      []Edge     `json:"edges"`
	Ports      []Port     `json:"ports"`
	Mounts     []Mount    `json:"mounts"`
	Problems   []Problem  `json:"problems"`
}

// File is one file the project was read from.
type File struct {
	Path string `json:"path"`
	// Role is "layer" (a -f file, in merge order), "include",
	// "extends" or "env".
	Role string `json:"role"`
	Read bool   `json:"read"`
}

// Service is one merged service.
type Service struct {
	Name      string   `json:"name"`
	Image     string   `json:"image,omitempty"`
	Build     string   `json:"build,omitempty"` // build context
	Command   string   `json:"command,omitempty"`
	Profiles  []string `json:"profiles"`
	DependsOn []Dep    `json:"dependsOn"`
	Networks  []string `json:"networks"`
	Mode      string   `json:"networkMode,omitempty"`
	Extends   string   `json:"extends,omitempty"`
	Restart   string   `json:"restart,omitempty"`
	Replicas  string   `json:"replicas,omitempty"`
	Health    string   `json:"healthcheck,omitempty"`
	EnvFiles  []string `json:"envFiles"`
	Env       []KV     `json:"environment"`
	Source    *Source  `json:"source,omitempty"` // first definition
	Files     []string `json:"files"`            // the files that set its fields, in merge order
	Effective []Line   `json:"effective"`
}

// Dep is one depends_on entry.
type Dep struct {
	Service   string  `json:"service"`
	Condition string  `json:"condition,omitempty"` // service_started, service_healthy, service_completed_successfully
	Optional  bool    `json:"optional,omitempty"`  // required: false
	Source    *Source `json:"source,omitempty"`
}

// KV is a name and a value.
type KV struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Resource is a top-level network, volume, secret or config.
type Resource struct {
	Name     string   `json:"name"`
	Detail   string   `json:"detail,omitempty"` // driver, file, name
	External bool     `json:"external,omitempty"`
	Implicit bool     `json:"implicit,omitempty"` // the default network, not declared
	UsedBy   []string `json:"usedBy"`
	Source   *Source  `json:"source,omitempty"`
}

// Variable is one ${VAR} the project uses.
type Variable struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Set   bool   `json:"set"`
	// From is where the value came from: "what-if", the env file's
	// path, "default" (the ${VAR:-…} fallback), or "" (unset).
	From     string   `json:"from,omitempty"`
	Default  string   `json:"default,omitempty"`
	Required bool     `json:"required,omitempty"`
	Defined  *Source  `json:"defined,omitempty"` // its line in the env file
	Uses     []Source `json:"uses"`
}

// Edge links two graph nodes: services, and "network:x" / "volume:x".
type Edge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Kind  string `json:"kind"` // depends_on, links, network_mode, volumes_from, network, volume
	Label string `json:"label,omitempty"`
}

// Port is one published or exposed port.
type Port struct {
	Service   string  `json:"service"`
	HostIP    string  `json:"hostIp,omitempty"`
	Published string  `json:"published,omitempty"`
	Target    string  `json:"target"`
	Protocol  string  `json:"protocol"`
	Layer     string  `json:"layer,omitempty"` // the file that set it
	Source    *Source `json:"source,omitempty"`
}

// Mount is one service volume: a bind mount, a named or anonymous
// volume, or a tmpfs.
type Mount struct {
	Service  string  `json:"service"`
	Type     string  `json:"type"` // bind, volume, tmpfs, npipe
	From     string  `json:"from,omitempty"`
	Target   string  `json:"target"`
	ReadOnly bool    `json:"readOnly,omitempty"`
	Layer    string  `json:"layer,omitempty"`
	Source   *Source `json:"source,omitempty"`
}

// Line is one line of a service's effective configuration.
type Line struct {
	Text   string  `json:"text"`
	From   string  `json:"from,omitempty"` // "extends web"; "" when written in Source.File itself
	Source *Source `json:"source,omitempty"`
}

// Problem is something wrong or worth knowing.
type Problem struct {
	Severity yamlkit.Severity `json:"severity"`
	Code     string           `json:"code"`
	Message  string           `json:"message"`
	Hint     string           `json:"hint,omitempty"`
	Service  string           `json:"service,omitempty"`
	Source   *Source          `json:"source,omitempty"`
}

// Loader reads a workspace file by slash path.
type Loader func(path string) ([]byte, error)

// Options feed an analysis.
type Options struct {
	Load  Loader
	Files []string          // the workspace's files, to find .env, overrides and candidates
	Env   map[string]string // what-if values; they win over the env file
	// EnvFile replaces the project's .env (a workspace path); "none"
	// reads no env file.
	EnvFile string
}

// NameRE matches Compose file names (compose.yaml,
// docker-compose.override.yml, compose.prod.yaml, …).
var NameRE = regexp.MustCompile(`^(docker-)?compose([.-].*)?\.ya?ml$`)

var (
	baseNames     = []string{"compose.yaml", "compose.yml", "docker-compose.yaml", "docker-compose.yml"}
	overrideNames = []string{"compose.override.yaml", "compose.override.yml", "docker-compose.override.yaml", "docker-compose.override.yml"}
)

// DefaultFiles is what `docker compose` would read for file: the base
// file and its override when file is either of them, else file alone.
func DefaultFiles(file string, files []string) []string {
	dir, base := path.Dir(file), path.Base(file)
	have := map[string]bool{}
	for _, f := range files {
		if path.Dir(f) == dir {
			have[path.Base(f)] = true
		}
	}
	first := func(list []string) string {
		for _, n := range list {
			if have[n] {
				return path.Join(dir, n)
			}
		}
		return ""
	}
	switch {
	case slices.Contains(overrideNames, base):
		if b := first(baseNames); b != "" {
			return []string{b, file}
		}
	case slices.Contains(baseNames, base):
		if o := first(overrideNames); o != "" {
			return []string{file, o}
		}
	}
	return []string{file}
}

// analyzer carries one analysis.
type analyzer struct {
	p      *Project
	opts   Options
	t      yamlkit.Origins
	env    map[string]string // the env file's values
	envAt  map[string]Source
	vars   map[string]*Variable
	defs   map[string][]Source // service definitions, per file in merge order
	loaded map[string]*yamlkit.Node
	merged map[string]*yamlkit.Node // services after merging, by name
	netAt  map[string]*Source       // where "service/network" is written
	busy   map[string]bool
	// standalone: one file, no folder (findings needing other files are
	// skipped); partial: that file is an override, which only adds to
	// a base the lens can't see.
	standalone, partial bool
}

// Analyze reads the project made of files, merged in order.
func Analyze(files []string, opts Options) *Project {
	p := &Project{}
	a := &analyzer{p: p, opts: opts, t: yamlkit.Origins{}, env: map[string]string{}, envAt: map[string]Source{},
		vars: map[string]*Variable{}, defs: map[string][]Source{}, loaded: map[string]*yamlkit.Node{}, merged: map[string]*yamlkit.Node{}, netAt: map[string]*Source{}, busy: map[string]bool{}}
	if len(files) == 0 {
		return normalize(p)
	}
	a.standalone = opts.Files == nil
	a.partial = a.standalone && slices.Contains(overrideNames, path.Base(files[0]))
	dir := path.Dir(files[0])
	a.readEnv(dir)
	for _, f := range opts.Files {
		if path.Dir(f) == dir && NameRE.MatchString(path.Base(f)) && !slices.Contains(files, f) {
			p.Candidates = append(p.Candidates, f)
		}
	}

	var root *yamlkit.Node
	for _, f := range files {
		n := a.load(f, "layer", 0)
		if n == nil {
			continue
		}
		root = a.t.Merge(root, n, "", rules)
	}
	if root == nil {
		return normalize(p)
	}
	p.Name = text(root.Get("name"))
	if p.Name == "" && dir != "." {
		p.Name = path.Base(dir)
	}
	if svcs := root.Get("services"); yamlkit.IsPlainMap(svcs) {
		for _, pr := range svcs.Pairs {
			pr.Value = a.extend(pr.Key.Value, pr.Value, svcs, a.t[pr.Key].File, 0, map[string]bool{})
		}
		reset(root)
	}
	a.read(root)
	a.check()
	return normalize(p)
}

func normalize(p *Project) *Project {
	for _, l := range []*[]string{&p.Candidates} {
		if *l == nil {
			*l = []string{}
		}
	}
	p.Files = orEmpty(p.Files)
	p.Services = orEmpty(p.Services)
	p.Networks, p.Volumes, p.Secrets, p.Configs = orEmpty(p.Networks), orEmpty(p.Volumes), orEmpty(p.Secrets), orEmpty(p.Configs)
	p.Variables, p.Edges, p.Ports, p.Mounts, p.Problems = orEmpty(p.Variables), orEmpty(p.Edges), orEmpty(p.Ports), orEmpty(p.Mounts), orEmpty(p.Problems)
	return p
}

func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func (a *analyzer) problem(sev yamlkit.Severity, code, service string, at *Source, msg, hint string) {
	if (a.standalone && needsFolder[code]) || (a.partial && needsBase[code]) {
		return
	}
	a.p.Problems = append(a.p.Problems, Problem{Severity: sev, Code: code, Service: service, Source: at, Message: msg, Hint: hint})
}

// readEnv reads the project's env file (.env next to the first file,
// unless another was chosen).
func (a *analyzer) readEnv(dir string) {
	file := a.opts.EnvFile
	switch file {
	case "none":
		return
	case "":
		file = path.Join(dir, ".env")
		if !slices.Contains(a.opts.Files, file) {
			return
		}
	}
	data, err := a.opts.Load(file)
	a.p.Files = append(a.p.Files, File{Path: file, Role: "env", Read: err == nil})
	if err != nil {
		a.problem(yamlkit.SeverityError, "compose-env-file", "", nil, "Can't read the env file "+file, "")
		return
	}
	a.p.EnvFile = file
	for _, e := range parseEnvFile(string(data), a.whatIf) {
		a.env[e.name] = e.value
		a.envAt[e.name] = Source{File: file, Line: e.line}
	}
}

func (a *analyzer) whatIf(name string) (string, bool) {
	v, ok := a.opts.Env[name]
	return v, ok
}

// get is the value a variable resolves to: a what-if value, else the
// env file's.
func (a *analyzer) get(name string) (string, bool) {
	if v, ok := a.opts.Env[name]; ok {
		return v, true
	}
	v, ok := a.env[name]
	return v, ok
}

// load reads, interpolates and normalizes one file (and what it
// includes), with every key tagged as written there. Results are
// cached, so a file extended twice is read once.
func (a *analyzer) load(file, role string, depth int) *yamlkit.Node {
	if n, ok := a.loaded[file]; ok {
		return n
	}
	if a.busy[file] || depth > 8 {
		a.problem(yamlkit.SeverityError, "compose-include-cycle", "", nil, file+" includes itself", "")
		return nil
	}
	a.busy[file] = true
	defer delete(a.busy, file)
	data, err := a.opts.Load(file)
	a.p.Files = append(a.p.Files, File{Path: file, Role: role, Read: err == nil})
	if err != nil {
		a.problem(yamlkit.SeverityError, "compose-file-missing", "", nil, "Can't read "+file, "")
		a.loaded[file] = nil
		return nil
	}
	f := yamlkit.Parse(data)
	for _, d := range f.Diagnostics {
		if d.Severity == yamlkit.SeverityError {
			a.problem(yamlkit.SeverityError, "compose-parse", "", &Source{File: file, Line: d.Range.Start.Line, Range: d.Range}, d.Message, d.Hint)
			break
		}
	}
	if len(f.Docs) == 0 || f.Docs[0].Root == nil {
		a.loaded[file] = nil
		return nil
	}
	root := f.Docs[0].Root
	if r, err := yamlkit.Resolve(root); err == nil {
		root = r // anchors, aliases and << merge keys (x-common: &common)
	}
	root = a.interpolate(root, file)
	root = a.t.Own(normalizeLists(root), file)
	if svcs := root.Get("services"); yamlkit.IsPlainMap(svcs) {
		for _, pr := range svcs.Pairs {
			a.defs[pr.Key.Value] = append(a.defs[pr.Key.Value], src(file, pr.Key))
		}
	}
	root = a.include(root, file, depth)
	a.loaded[file] = root
	return root
}

// include merges the files root includes beneath it. Compose refuses
// conflicting definitions; codec reports them and lets root win.
func (a *analyzer) include(root *yamlkit.Node, file string, depth int) *yamlkit.Node {
	inc := root.Get("include")
	if inc == nil || inc.Kind != yamlkit.KindSeq {
		return root
	}
	var acc *yamlkit.Node
	for _, it := range inc.Items {
		var paths []*yamlkit.Node
		switch {
		case it.Kind == yamlkit.KindScalar:
			paths = []*yamlkit.Node{it}
		case it.Get("path") != nil && it.Get("path").Kind == yamlkit.KindSeq:
			paths = items(it.Get("path"))
		case it.Get("path") != nil:
			paths = []*yamlkit.Node{it.Get("path")}
		}
		for _, pn := range paths {
			p := path.Join(path.Dir(file), pn.Value)
			n := a.load(p, "include", depth+1)
			if n == nil {
				a.problem(yamlkit.SeverityError, "compose-include-missing", "", ptr(src(file, pn)), "Can't read included file "+pn.Value, "")
				continue
			}
			for _, kind := range []string{"services", "networks", "volumes", "secrets", "configs"} {
				for _, pr := range pairs(n.Get(kind)) {
					if root.Get(kind) != nil && root.Get(kind).Get(pr.Key.Value) != nil {
						a.problem(yamlkit.SeverityWarning, "compose-include-conflict", "", ptr(src(file, pn)),
							fmt.Sprintf("%s %s is defined both here and in included %s", strings.TrimSuffix(kind, "s"), pr.Key.Value, pn.Value), "Compose refuses conflicting definitions from includes.")
					}
				}
			}
			acc = a.t.Merge(acc, n, "include "+path.Base(p), rules)
		}
	}
	if acc == nil {
		return root
	}
	return a.t.Merge(acc, root, "", rules)
}

// extend applies a service's extends: the base service (from this
// project, or another file) is merged beneath it.
func (a *analyzer) extend(name string, svc, services *yamlkit.Node, file string, depth int, seen map[string]bool) *yamlkit.Node {
	ext := svc.Get("extends")
	if ext == nil || depth > 8 {
		return svc
	}
	base, from := text(ext), ""
	if ext.Kind == yamlkit.KindMap {
		base, from = text(ext.Get("service")), text(ext.Get("file"))
	}
	at := ptr(src(file, ext))
	key := from + "#" + base
	if seen[key] {
		a.problem(yamlkit.SeverityError, "compose-extends-cycle", name, at, "Service "+name+" extends itself through "+base, "")
		return svc
	}
	seen[key] = true
	var parent *yamlkit.Node
	parentFile := file
	if from != "" {
		parentFile = path.Join(path.Dir(file), from)
		other := a.load(parentFile, "extends", depth+1)
		if other == nil {
			a.problem(yamlkit.SeverityError, "compose-extends-missing", name, at, "Can't read "+from+", which service "+name+" extends", "")
			return svc
		}
		services = other.Get("services")
	}
	if yamlkit.IsPlainMap(services) {
		parent = services.Get(base)
	}
	if parent == nil {
		var known []string
		for _, pr := range pairs(services) {
			known = append(known, pr.Key.Value)
		}
		a.problem(yamlkit.SeverityError, "compose-extends-unknown", name, at, fmt.Sprintf("Service %s extends %s, which isn't a service%s", name, base, inFile(from)), names.DidYouMean(base, known))
		return svc
	}
	parent = a.extend(base, parent, services, parentFile, depth+1, seen)
	return a.t.Merge(a.t.Copy(parent, yamlkit.Origin{Via: "extends " + base}, false), svc, "", serviceRules)
}

func inFile(f string) string {
	if f == "" {
		return ""
	}
	return " in " + f
}

// reset applies Compose's merge tags on the merged tree: an entry
// tagged !reset is removed; !override has done its work (it replaced
// instead of merging), so the tag is dropped.
func reset(n *yamlkit.Node) {
	if n == nil {
		return
	}
	switch n.Kind {
	case yamlkit.KindMap:
		n.Pairs = slices.DeleteFunc(n.Pairs, func(p *yamlkit.Pair) bool { return p.Value != nil && p.Value.Tag == "!reset" })
		for _, p := range n.Pairs {
			if p.Value != nil && p.Value.Tag == "!override" {
				v := *p.Value
				v.Tag = ""
				p.Value = &v
			}
			reset(p.Value)
		}
	case yamlkit.KindSeq:
		for _, it := range n.Items {
			reset(it)
		}
	}
}

// interpolate returns root with ${VAR} expanded in every value, and
// records each variable use. Unchanged subtrees are shared.
func (a *analyzer) interpolate(n *yamlkit.Node, file string) *yamlkit.Node {
	if n == nil {
		return nil
	}
	switch n.Kind {
	case yamlkit.KindScalar:
		if !strings.Contains(n.Value, "$") {
			return n
		}
		ex := interpolate(n.Value, a.get)
		at := src(file, n)
		for _, r := range ex.refs {
			a.use(r, at)
		}
		for _, msg := range ex.errs {
			a.problem(yamlkit.SeverityError, "compose-var-required", "", &at, msg, "Set it in "+a.envName()+" or as a what-if value.")
		}
		if ex.text == n.Value {
			return n
		}
		c := *n
		c.Value = ex.text
		return &c
	case yamlkit.KindMap:
		var out *yamlkit.Node
		for i, p := range n.Pairs {
			if v := a.interpolate(p.Value, file); v != p.Value {
				if out == nil {
					cp := *n
					cp.Pairs = slices.Clone(n.Pairs)
					out = &cp
				}
				out.Pairs[i] = &yamlkit.Pair{Key: p.Key, Value: v}
			}
		}
		if out != nil {
			return out
		}
	case yamlkit.KindSeq:
		var out *yamlkit.Node
		for i, it := range n.Items {
			if v := a.interpolate(it, file); v != it {
				if out == nil {
					cp := *n
					cp.Items = slices.Clone(n.Items)
					out = &cp
				}
				out.Items[i] = v
			}
		}
		if out != nil {
			return out
		}
	}
	return n
}

func (a *analyzer) envName() string {
	if a.p.EnvFile != "" {
		return a.p.EnvFile
	}
	return ".env"
}

// use records one reference to a variable.
func (a *analyzer) use(r ref, at Source) {
	v := a.vars[r.name]
	if v == nil {
		v = &Variable{Name: r.name}
		a.vars[r.name] = v
		if val, ok := a.opts.Env[r.name]; ok {
			v.Value, v.Set, v.From = val, true, "what-if"
		} else if val, ok := a.env[r.name]; ok {
			v.Value, v.Set, v.From = val, true, a.p.EnvFile
			d := a.envAt[r.name]
			v.Defined = &d
		}
	}
	switch r.op {
	case ":-", "-":
		if v.Default == "" {
			v.Default = r.arg
		}
		if !v.Set || (r.op == ":-" && v.Value == "") {
			v.From = "default"
		}
	case ":?", "?":
		v.Required = true
	case "":
		if !v.Set {
			a.problem(yamlkit.SeverityWarning, "compose-var-unset", "", &at, fmt.Sprintf("%s isn't set, so it becomes an empty string", r.name), "Set it in "+a.envName()+", give a default with ${"+r.name+":-value}, or try a what-if value.")
		}
	}
	v.Uses = append(v.Uses, at)
}

func src(file string, n *yamlkit.Node) Source {
	return Source{File: file, Line: n.Range.Start.Line, Range: n.Range}
}

func ptr[T any](v T) *T { return &v }

// srcOf is where a merged node was written, from its origin.
func (a *analyzer) srcOf(key *yamlkit.Node) *Source {
	if key == nil || key.Range.Start.Line == 0 {
		return nil
	}
	return ptr(src(a.t[key].File, key))
}

// --- merge rules ---

// rules are Compose's list merge rules for whole files.
var rules = &yamlkit.MergeRules{List: func(p []string) func(*yamlkit.Node) string {
	if len(p) >= 3 && p[0] == "services" {
		return serviceList(p[2:])
	}
	return nil
}}

// serviceRules are the same rules for one service (extends).
var serviceRules = &yamlkit.MergeRules{List: serviceList}

// serviceList says how a service's lists merge: commands replace,
// ports/volumes/secrets/configs/devices merge by what they are, and
// other lists append the items they don't have yet.
func serviceList(p []string) func(*yamlkit.Node) string {
	if len(p) == 0 {
		return nil
	}
	switch p[0] {
	case "command", "entrypoint", "healthcheck":
		return nil
	case "ports":
		return func(n *yamlkit.Node) string {
			pt := parsePort(n)
			return pt.HostIP + "|" + pt.Published + "|" + pt.Target + "|" + pt.Protocol
		}
	case "volumes":
		return func(n *yamlkit.Node) string { return parseMount(n).Target }
	case "secrets", "configs":
		return func(n *yamlkit.Node) string {
			if n.Kind == yamlkit.KindMap {
				return cmp.Or(text(n.Get("target")), text(n.Get("source")))
			}
			return text(n)
		}
	case "devices":
		return func(n *yamlkit.Node) string {
			parts := strings.Split(text(n), ":")
			if len(parts) > 1 {
				return parts[1]
			}
			return parts[0]
		}
	}
	return yamlkit.InlineText
}

// listMaps are service fields that may be written as a list of
// NAME=value; they merge by name, so they are read as maps.
var listMaps = []string{"environment", "labels", "annotations", "sysctls", "extra_hosts"}

// normalizeLists turns list-form environment, labels and build args
// into maps (as `docker compose config` prints them), keeping each
// entry's position.
func normalizeLists(root *yamlkit.Node) *yamlkit.Node {
	svcs := root.Get("services")
	if !yamlkit.IsPlainMap(svcs) {
		return root
	}
	for _, sp := range svcs.Pairs {
		svc := sp.Value
		if !yamlkit.IsPlainMap(svc) {
			continue
		}
		for _, p := range svc.Pairs {
			if slices.Contains(listMaps, p.Key.Value) {
				p.Value = listToMap(p.Value, p.Key.Value == "extra_hosts")
			}
			if p.Key.Value == "build" && yamlkit.IsPlainMap(p.Value) {
				for _, bp := range p.Value.Pairs {
					if bp.Key.Value == "args" || bp.Key.Value == "labels" {
						bp.Value = listToMap(bp.Value, false)
					}
				}
			}
		}
	}
	return root
}

func listToMap(n *yamlkit.Node, hosts bool) *yamlkit.Node {
	if n == nil || n.Kind != yamlkit.KindSeq || yamlkit.IsCustomTag(n) {
		return n
	}
	out := &yamlkit.Node{Kind: yamlkit.KindMap, Range: n.Range}
	for _, it := range n.Items {
		s := text(it)
		k, v, ok := strings.Cut(s, "=")
		if !ok && hosts {
			k, v, ok = strings.Cut(s, ":")
		}
		key := &yamlkit.Node{Kind: yamlkit.KindScalar, Tag: yamlkit.TagStr, Value: k, Style: yamlkit.StylePlain, Range: it.Range}
		val := &yamlkit.Node{Kind: yamlkit.KindScalar, Tag: yamlkit.TagStr, Value: v, Style: yamlkit.StylePlain, Range: it.Range}
		if !ok {
			val.Tag, val.Value = yamlkit.TagNull, "" // NAME alone: from the shell, unset here
		}
		out.Pairs = append(out.Pairs, &yamlkit.Pair{Key: key, Value: val})
	}
	return out
}

// --- reading the merged project ---

func (a *analyzer) read(root *yamlkit.Node) {
	p := a.p
	for _, kind := range []struct {
		key string
		out *[]Resource
	}{{"networks", &p.Networks}, {"volumes", &p.Volumes}, {"secrets", &p.Secrets}, {"configs", &p.Configs}} {
		for _, pr := range pairs(root.Get(kind.key)) {
			r := Resource{Name: pr.Key.Value, Source: a.srcOf(pr.Key)}
			v := pr.Value
			switch ext := v.Get("external"); {
			case ext != nil && (ext.Value == "true" || ext.Kind == yamlkit.KindMap):
				r.External, r.Detail = true, cmp.Or(text(v.Get("name")), text(ext.Get("name")), "external")
			case v.Get("file") != nil:
				r.Detail = "file " + text(v.Get("file"))
			case v.Get("environment") != nil:
				r.Detail = "from $" + text(v.Get("environment"))
			default:
				r.Detail = strings.TrimSpace(cmp.Or(text(v.Get("driver")), "") + nameDetail(v))
			}
			*kind.out = append(*kind.out, r)
		}
	}
	for _, pr := range pairs(root.Get("services")) {
		a.merged[pr.Key.Value] = pr.Value
		p.Services = append(p.Services, a.service(pr.Key.Value, pr.Value))
	}
}

func nameDetail(v *yamlkit.Node) string {
	if n := text(v.Get("name")); n != "" {
		return " · name " + n
	}
	return ""
}

func (a *analyzer) service(name string, n *yamlkit.Node) Service {
	s := Service{Name: name, Image: text(n.Get("image")), Restart: text(n.Get("restart")), Mode: text(n.Get("network_mode")),
		Profiles: strs(n.Get("profiles")), Replicas: text(n.Get("deploy").Get("replicas"))}
	if defs := a.defs[name]; len(defs) > 0 {
		s.Source = &defs[0]
	}
	switch b := n.Get("build"); {
	case b == nil:
	case b.Kind == yamlkit.KindScalar:
		s.Build = b.Value
	default:
		s.Build = cmp.Or(text(b.Get("context")), ".")
		if df := text(b.Get("dockerfile")); df != "" {
			s.Build += " · " + df
		}
	}
	s.Command = cmdText(n.Get("command"))
	if e := n.Get("extends"); e != nil {
		s.Extends = cmp.Or(text(e.Get("service")), text(e))
	}
	if h := n.Get("healthcheck"); h != nil {
		if h.Get("disable") != nil && h.Get("disable").Value == "true" {
			s.Health = "disabled"
		} else {
			s.Health = cmdText(h.Get("test"))
		}
	}
	// depends_on: a list of names, or a map of name → condition.
	if d := n.Get("depends_on"); d != nil {
		for _, it := range d.Items {
			s.DependsOn = append(s.DependsOn, Dep{Service: text(it), Source: a.srcAt(it, n.Pair("depends_on").Key)})
		}
		for _, pr := range pairs(d) {
			dep := Dep{Service: pr.Key.Value, Condition: text(pr.Value.Get("condition")), Source: a.srcOf(pr.Key)}
			dep.Optional = text(pr.Value.Get("required")) == "false"
			s.DependsOn = append(s.DependsOn, dep)
		}
	}
	for _, l := range items(n.Get("links")) {
		target, _, _ := strings.Cut(text(l), ":")
		a.p.Edges = append(a.p.Edges, Edge{From: target, To: name, Kind: "links"})
	}
	if nets := n.Get("networks"); nets != nil {
		for _, it := range items(nets) {
			s.Networks = append(s.Networks, text(it))
			a.netAt[name+"/"+text(it)] = a.srcAt(it, n.Pair("networks").Key)
		}
		for _, pr := range pairs(nets) {
			s.Networks = append(s.Networks, pr.Key.Value)
			a.netAt[name+"/"+pr.Key.Value] = a.srcOf(pr.Key)
		}
	} else if s.Mode == "" {
		s.Networks = []string{"default"}
	}
	for _, f := range envFiles(n.Get("env_file")) {
		s.EnvFiles = append(s.EnvFiles, f)
	}
	for _, pr := range pairs(n.Get("environment")) {
		s.Env = append(s.Env, KV{Name: pr.Key.Value, Value: text(pr.Value)})
	}
	for _, it := range items(n.Get("ports")) {
		pt := parsePort(it)
		pt.Service, pt.Source, pt.Layer = name, a.srcAt(it, n.Pair("ports").Key), a.fileOf(it, n.Pair("ports").Key)
		a.p.Ports = append(a.p.Ports, pt)
	}
	for _, it := range items(n.Get("volumes")) {
		m := parseMount(it)
		m.Service, m.Source, m.Layer = name, a.srcAt(it, n.Pair("volumes").Key), a.fileOf(it, n.Pair("volumes").Key)
		a.p.Mounts = append(a.p.Mounts, m)
	}
	for _, it := range items(n.Get("tmpfs")) {
		a.p.Mounts = append(a.p.Mounts, Mount{Service: name, Type: "tmpfs", Target: text(it), Source: a.srcAt(it, n.Pair("tmpfs").Key)})
	}
	if t := n.Get("tmpfs"); t != nil && t.Kind == yamlkit.KindScalar {
		a.p.Mounts = append(a.p.Mounts, Mount{Service: name, Type: "tmpfs", Target: t.Value, Source: a.srcAt(t, n.Pair("tmpfs").Key)})
	}
	// Which files set its fields, in the order they were read.
	seen := map[string]bool{}
	yamlkit.Walk(n, func(x *yamlkit.Node) bool {
		if o, ok := a.t[x]; ok && o.File != "" {
			seen[o.File] = true
		}
		return true
	})
	for _, f := range a.p.Files {
		if seen[f.Path] && !slices.Contains(s.Files, f.Path) {
			s.Files = append(s.Files, f.Path)
		}
	}
	for _, l := range a.t.Emit(n, yamlkit.Origin{}) {
		line := Line{Text: l.Text, From: l.Origin.Via}
		if l.At != nil {
			line.Source = ptr(src(l.Origin.File, l.At))
		}
		s.Effective = append(s.Effective, line)
	}
	s.Profiles, s.DependsOn, s.Networks, s.EnvFiles, s.Env, s.Files, s.Effective =
		orEmpty(s.Profiles), orEmpty(s.DependsOn), orEmpty(s.Networks), orEmpty(s.EnvFiles), orEmpty(s.Env), orEmpty(s.Files), orEmpty(s.Effective)
	return s
}

// srcAt locates a list item: its own origin if it has one (merged
// lists tag items), else its list key's file.
func (a *analyzer) srcAt(it, key *yamlkit.Node) *Source {
	if it == nil || it.Range.Start.Line == 0 {
		return nil
	}
	return ptr(src(a.fileOf(it, key), it))
}

func (a *analyzer) fileOf(it, key *yamlkit.Node) string {
	if o, ok := a.t[it]; ok && o.File != "" {
		return o.File
	}
	if key != nil {
		return a.t[key].File
	}
	return ""
}

func envFiles(n *yamlkit.Node) []string {
	switch {
	case n == nil:
		return nil
	case n.Kind == yamlkit.KindScalar:
		return []string{n.Value}
	}
	var out []string
	for _, it := range n.Items {
		out = append(out, cmp.Or(text(it.Get("path")), text(it)))
	}
	return out
}

// parsePort reads "[[ip:]published:]target[/protocol]" or the long
// syntax.
func parsePort(n *yamlkit.Node) Port {
	if n.Kind == yamlkit.KindMap {
		return Port{Target: text(n.Get("target")), Published: text(n.Get("published")), HostIP: text(n.Get("host_ip")), Protocol: cmp.Or(text(n.Get("protocol")), "tcp")}
	}
	s := text(n)
	pt := Port{Protocol: "tcp"}
	if i := strings.LastIndexByte(s, '/'); i >= 0 {
		s, pt.Protocol = s[:i], s[i+1:]
	}
	// An IPv6 host address is written in brackets: [::1]:8080:80.
	if strings.HasPrefix(s, "[") {
		if end := strings.Index(s, "]:"); end > 0 {
			pt.HostIP, s = s[1:end], s[end+2:]
		}
	}
	parts := strings.Split(s, ":")
	pt.Target = parts[len(parts)-1]
	if len(parts) >= 2 {
		pt.Published = parts[len(parts)-2]
	}
	if len(parts) >= 3 {
		pt.HostIP = strings.Join(parts[:len(parts)-2], ":")
	}
	return pt
}

// parseMount reads "[source:]target[:mode]" or the long syntax.
func parseMount(n *yamlkit.Node) Mount {
	if n.Kind == yamlkit.KindMap {
		m := Mount{Type: cmp.Or(text(n.Get("type")), "volume"), From: text(n.Get("source")), Target: text(n.Get("target"))}
		m.ReadOnly = text(n.Get("read_only")) == "true"
		return m
	}
	s := text(n)
	var parts []string
	// A Windows drive (C:\data:/data) keeps its colon.
	if len(s) > 2 && s[1] == ':' && (s[2] == '\\' || s[2] == '/') {
		rest := strings.Split(s[2:], ":")
		parts = append([]string{s[:2] + rest[0]}, rest[1:]...)
	} else {
		parts = strings.Split(s, ":")
	}
	m := Mount{}
	switch len(parts) {
	case 1:
		m.Target = parts[0]
	default:
		m.From, m.Target = parts[0], parts[1]
		if len(parts) > 2 {
			m.ReadOnly = slices.Contains(strings.Split(parts[2], ","), "ro")
		}
	}
	m.Type = mountType(m.From)
	return m
}

// mountType tells a bind mount (a path) from a named or anonymous
// volume.
func mountType(from string) string {
	switch {
	case from == "":
		return "volume"
	case strings.HasPrefix(from, "/"), strings.HasPrefix(from, "."), strings.HasPrefix(from, "~"), strings.HasPrefix(from, "$"),
		len(from) > 1 && from[1] == ':', strings.HasPrefix(from, `\\`):
		return "bind"
	}
	return "volume"
}

func cmdText(n *yamlkit.Node) string {
	if n == nil {
		return ""
	}
	if n.Kind == yamlkit.KindSeq {
		return strings.Join(strs(n), " ")
	}
	return text(n)
}

// --- checks ---

func (a *analyzer) check() {
	p := a.p
	svc := map[string]*Service{}
	var svcNames []string
	for i := range p.Services {
		svc[p.Services[i].Name] = &p.Services[i]
		svcNames = append(svcNames, p.Services[i].Name)
	}
	resources := func(list []Resource) map[string]*Resource {
		m := map[string]*Resource{}
		for i := range list {
			m[list[i].Name] = &list[i]
		}
		return m
	}
	nets, vols, secrets, configs := resources(p.Networks), resources(p.Volumes), resources(p.Secrets), resources(p.Configs)
	resNames := func(m map[string]*Resource) []string { return slices.Sorted(maps.Keys(m)) }

	for i := range p.Services {
		s := &p.Services[i]
		for _, d := range s.DependsOn {
			if svc[d.Service] == nil {
				a.problem(yamlkit.SeverityError, "compose-depends-unknown", s.Name, d.Source, fmt.Sprintf("Service %s depends on %s, which isn't a service", s.Name, d.Service), names.DidYouMean(d.Service, svcNames))
				continue
			}
			p.Edges = append(p.Edges, Edge{From: d.Service, To: s.Name, Kind: "depends_on", Label: depLabel(d)})
		}
		if ref, ok := strings.CutPrefix(s.Mode, "service:"); ok {
			p.Edges = append(p.Edges, Edge{From: ref, To: s.Name, Kind: "network_mode"})
		}
		for _, net := range s.Networks {
			r := nets[net]
			if r == nil && net == "default" {
				p.Networks = append(p.Networks, Resource{Name: "default", Detail: "created by Compose", Implicit: true})
				nets = resources(p.Networks)
				r = nets[net]
			}
			if r == nil {
				a.problem(yamlkit.SeverityError, "compose-network-undefined", s.Name, cmp.Or(a.netAt[s.Name+"/"+net], s.Source), fmt.Sprintf("Service %s uses network %s, which isn't declared under networks:", s.Name, net), names.DidYouMean(net, resNames(nets)))
				continue
			}
			r.UsedBy = append(r.UsedBy, s.Name)
			p.Edges = append(p.Edges, Edge{From: s.Name, To: "network:" + net, Kind: "network"})
		}
		for _, f := range s.EnvFiles {
			fp := path.Join(path.Dir(fileOr(s.Source)), f)
			if a.opts.Files != nil && !slices.Contains(a.opts.Files, fp) {
				a.problem(yamlkit.SeverityWarning, "compose-env-file-missing", s.Name, s.Source, fmt.Sprintf("Service %s reads env_file %s, which isn't in the folder", s.Name, f), "Compose fails unless the entry says required: false.")
			}
		}
		if s.Image == "" && s.Build == "" && s.Extends == "" {
			a.problem(yamlkit.SeverityWarning, "compose-no-image", s.Name, s.Source, "Service "+s.Name+" has neither image nor build", "")
		}
	}
	for _, m := range p.Mounts {
		if m.Type != "volume" || m.From == "" {
			continue
		}
		r := vols[m.From]
		if r == nil {
			a.problem(yamlkit.SeverityError, "compose-volume-undefined", m.Service, m.Source, fmt.Sprintf("Service %s mounts volume %s, which isn't declared under volumes:", m.Service, m.From), cmp.Or(names.DidYouMean(m.From, resNames(vols)), "Declare it, or write a path (./"+m.From+") for a bind mount."))
			continue
		}
		if !slices.Contains(r.UsedBy, m.Service) {
			r.UsedBy = append(r.UsedBy, m.Service)
		}
		p.Edges = append(p.Edges, Edge{From: m.Service, To: "volume:" + m.From, Kind: "volume", Label: m.Target})
	}
	a.checkSecrets(secrets, configs)
	a.checkCycles(svcNames)
	a.checkPorts()
	for _, e := range p.Edges {
		if (e.Kind == "links" || e.Kind == "network_mode") && svc[e.From] == nil {
			a.problem(yamlkit.SeverityError, "compose-"+strings.ReplaceAll(e.Kind, "_", "-")+"-unknown", e.To, svc[e.To].Source,
				fmt.Sprintf("Service %s refers to service %s (%s), which doesn't exist", e.To, e.From, e.Kind), names.DidYouMean(e.From, svcNames))
		}
	}
	p.Edges = slices.DeleteFunc(p.Edges, func(e Edge) bool {
		return (e.Kind == "links" || e.Kind == "network_mode") && svc[e.From] == nil
	})
	// Variables in the order they were first used.
	for _, name := range slices.SortedFunc(maps.Keys(a.vars), func(x, y string) int {
		u, w := a.vars[x].Uses[0], a.vars[y].Uses[0]
		return cmp.Or(cmp.Compare(u.File, w.File), cmp.Compare(u.Line, w.Line), cmp.Compare(x, y))
	}) {
		p.Variables = append(p.Variables, *a.vars[name])
	}
	slices.SortStableFunc(p.Problems, func(x, y Problem) int {
		return cmp.Or(cmp.Compare(fileOr(x.Source), fileOr(y.Source)), cmp.Compare(lineOr(x.Source), lineOr(y.Source)))
	})
}

func depLabel(d Dep) string {
	l := strings.TrimPrefix(d.Condition, "service_")
	if l == "started" {
		l = ""
	}
	if l == "completed_successfully" {
		l = "completed"
	}
	if d.Optional {
		l = strings.TrimSpace(l + " optional")
	}
	return l
}

// checkSecrets checks the secrets and configs services use are
// declared; the short and long syntax both name the top-level entry.
func (a *analyzer) checkSecrets(secrets, configs map[string]*Resource) {
	for _, kind := range []struct {
		key string
		m   map[string]*Resource
	}{{"secrets", secrets}, {"configs", configs}} {
		for _, s := range a.p.Services {
			for _, ref := range a.serviceRefs(s.Name, kind.key) {
				r := kind.m[ref.name]
				if r == nil {
					a.problem(yamlkit.SeverityError, "compose-"+strings.TrimSuffix(kind.key, "s")+"-undefined", s.Name, ref.at,
						fmt.Sprintf("Service %s uses %s %s, which isn't declared under %s:", s.Name, strings.TrimSuffix(kind.key, "s"), ref.name, kind.key), names.DidYouMean(ref.name, slices.Sorted(maps.Keys(kind.m))))
					continue
				}
				if !slices.Contains(r.UsedBy, s.Name) {
					r.UsedBy = append(r.UsedBy, s.Name)
				}
			}
		}
	}
}

type namedRef struct {
	name string
	at   *Source
}

// serviceRefs lists the names a merged service's secrets: or configs:
// refer to.
func (a *analyzer) serviceRefs(service, key string) []namedRef {
	n := a.merged[service]
	if n == nil {
		return nil
	}
	list := n.Get(key)
	if list == nil {
		return nil
	}
	var out []namedRef
	for _, it := range list.Items {
		name := text(it)
		if it.Kind == yamlkit.KindMap {
			name = text(it.Get("source"))
		}
		out = append(out, namedRef{name, a.srcAt(it, n.Pair(key).Key)})
	}
	return out
}

// checkCycles reports services that (transitively) depend on
// themselves: Compose can't start them.
func (a *analyzer) checkCycles(svcNames []string) {
	next := map[string][]string{}
	for _, e := range a.p.Edges {
		if e.Kind == "depends_on" || e.Kind == "links" || e.Kind == "network_mode" {
			next[e.To] = append(next[e.To], e.From)
		}
	}
	state := map[string]int{} // 1 visiting, 2 done
	var stack []string
	var visit func(n string) bool
	visit = func(n string) bool {
		switch state[n] {
		case 1:
			i := slices.Index(stack, n)
			loop := append(slices.Clone(stack[i:]), n)
			var at *Source
			for _, s := range a.p.Services {
				if s.Name == n {
					at = s.Source
				}
			}
			a.problem(yamlkit.SeverityError, "compose-cycle", n, at, "Services depend on each other in a loop: "+strings.Join(loop, " → "), "")
			return true
		case 2:
			return false
		}
		state[n] = 1
		stack = append(stack, n)
		for _, m := range next[n] {
			if visit(m) {
				break
			}
		}
		stack = stack[:len(stack)-1]
		state[n] = 2
		return false
	}
	for _, n := range svcNames {
		visit(n)
	}
}

// checkPorts warns when two services publish the same host port: the
// second can't start.
func (a *analyzer) checkPorts() {
	type key struct{ ip, port, proto string }
	first := map[key]Port{}
	for _, pt := range a.p.Ports {
		if pt.Published == "" || strings.Contains(pt.Published, "-") {
			continue
		}
		k := key{pt.HostIP, pt.Published, pt.Protocol}
		if prev, ok := first[k]; ok && prev.Service != pt.Service {
			a.problem(yamlkit.SeverityWarning, "compose-port-clash", pt.Service, pt.Source,
				fmt.Sprintf("Services %s and %s both publish host port %s/%s", prev.Service, pt.Service, pt.Published, pt.Protocol), "Only one of them can start, unless they are in different profiles.")
			continue
		}
		first[k] = pt
	}
}

// --- helpers ---

func pairs(n *yamlkit.Node) []*yamlkit.Pair {
	if !yamlkit.IsPlainMap(n) {
		return nil
	}
	return n.Pairs
}

func text(n *yamlkit.Node) string {
	if n == nil || n.Kind != yamlkit.KindScalar || n.Tag == yamlkit.TagNull {
		return ""
	}
	return n.Value
}

func strs(n *yamlkit.Node) []string {
	if n == nil {
		return nil
	}
	if n.Kind == yamlkit.KindScalar {
		return []string{n.Value}
	}
	var out []string
	for _, it := range n.Items {
		out = append(out, text(it))
	}
	return out
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

// items is a list's items, or nil.
func items(n *yamlkit.Node) []*yamlkit.Node {
	if n == nil || n.Kind != yamlkit.KindSeq {
		return nil
	}
	return n.Items
}

// Findings that depend on files a lone file can't see.
var (
	needsFolder = map[string]bool{"compose-var-unset": true, "compose-var-required": true, "compose-file-missing": true,
		"compose-extends-missing": true, "compose-include-missing": true, "compose-env-file": true}
	needsBase = map[string]bool{"compose-no-image": true, "compose-depends-unknown": true, "compose-network-undefined": true,
		"compose-volume-undefined": true, "compose-secret-undefined": true, "compose-config-undefined": true,
		"compose-links-unknown": true, "compose-network-mode-unknown": true, "compose-extends-unknown": true}
)
