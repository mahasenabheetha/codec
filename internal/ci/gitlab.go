package ci

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/names"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// GitLab CI builds a job's configuration in this order, and so does
// this reader:
//
//  1. anchors and << merge keys, per file (YAML itself);
//  2. include: files are merged under the including file, hashes
//     deep-merged, the including file winning;
//  3. !reference [.job, key…] tags are replaced by what they point to;
//  4. extends: templates are deep-merged in order, the job winning;
//  5. default: (and the old top-level image, services, …) fill in the
//     keywords a job doesn't set, and global variables are merged in,
//     unless inherit: says otherwise.

// glKeywords are top-level keys that are not jobs.
var glKeywords = map[string]bool{
	"image": true, "services": true, "cache": true, "before_script": true, "after_script": true,
	"variables": true, "stages": true, "types": true, "include": true, "default": true, "workflow": true, "spec": true,
}

// glDefaults are the keywords default: can set.
var glDefaults = []string{"after_script", "artifacts", "before_script", "cache", "hooks", "id_tokens", "image", "interruptible", "retry", "services", "tags", "timeout"}

type gitlab struct {
	p    *Pipeline
	opts Options
	t    tags

	cfg   map[string]*glEntry // merged top-level config
	order []string
	read  map[string]bool // files already included

	// unresolved lists includes codec can't read: a name missing here
	// may be defined there.
	unresolved []string

	eff  map[string]*glEff
	busy map[string]bool
}

type glEntry struct {
	node *yamlkit.Node // owned tree: maps copied, keys tagged with their file
	file string
	key  *yamlkit.Node // where the entry is first written
}

type glEff struct {
	node   *yamlkit.Node
	chain  []string // templates merged in, in order
	unread bool     // some template lives in an include codec doesn't read
}

func readGitLab(p *Pipeline, content []byte, f *yamlkit.File, opts Options) {
	g := &gitlab{p: p, opts: opts, t: tags{}, cfg: map[string]*glEntry{}, read: map[string]bool{p.File: true}, eff: map[string]*glEff{}, busy: map[string]bool{}}
	g.readFile(p.File, f, true)
	g.pipeline()
}

// readFile adds a file's configuration (its includes first) to g.cfg.
func (g *gitlab) readFile(file string, f *yamlkit.File, main bool) {
	var header, body *yamlkit.Node
	for _, d := range f.Docs {
		if d.Root == nil || d.Root.Kind != yamlkit.KindMap {
			continue
		}
		// A spec: header is its own document, before the configuration.
		if header == nil && body == nil && len(d.Root.Pairs) == 1 && d.Root.Get("spec") != nil && len(f.Docs) > 1 {
			header = d.Root
			continue
		}
		body = d.Root
		break
	}
	if main && header != nil {
		g.specInputs(file, header.Get("spec").Get("inputs"))
	}
	if body == nil {
		return
	}
	body = resolved(body)
	g.includes(file, body.Get("include"))
	for _, pr := range body.Pairs {
		name := keyName(pr.Key)
		if name == "include" || name == "" {
			continue
		}
		owned := g.t.Own(pr.Value, file)
		if e, ok := g.cfg[name]; ok {
			// The including file wins; hashes merge.
			e.node = g.t.Merge(e.node, owned, "", nil)
			continue
		}
		g.cfg[name] = &glEntry{node: owned, file: file, key: pr.Key}
		g.order = append(g.order, name)
	}
}

func (g *gitlab) specInputs(file string, in *yamlkit.Node) {
	if in == nil || in.Kind != yamlkit.KindMap {
		return
	}
	for _, pr := range in.Pairs {
		v := pr.Value
		i := Input{Name: pr.Key.Value, From: "spec", Source: src(file, pr.Key), Type: v.Get("type").Str(), Description: v.Get("description").Str(), Options: strs(v.Get("options"))}
		if i.Type == "" {
			i.Type = "string"
		}
		if d := v.Get("default"); d != nil {
			i.Default, i.HasDefault = text(d), true
		} else {
			i.Required = true
		}
		g.p.Inputs = append(g.p.Inputs, i)
	}
}

// includes reads include: entries, local ones from the workspace.
func (g *gitlab) includes(file string, n *yamlkit.Node) {
	if n == nil {
		return
	}
	list := items(n)
	if n.Kind != yamlkit.KindSeq {
		list = []*yamlkit.Node{n}
	}
	for _, it := range list {
		inc := Include{Source: src(file, it), State: "unresolved"}
		var local []string
		switch {
		case it.Kind == yamlkit.KindScalar:
			if isURL(it.Value) {
				inc.Kind, inc.Target = "remote", it.Value
			} else {
				inc.Kind, inc.Target, local = "local", it.Value, []string{it.Value}
			}
		case it.Get("local") != nil:
			inc.Kind, inc.Target, local = "local", it.Get("local").Str(), strs(it.Get("local"))
		case it.Get("project") != nil:
			inc.Kind = "project"
			inc.Target = it.Get("project").Str() + ": " + strings.Join(strs(it.Get("file")), ", ")
			if ref := it.Get("ref").Str(); ref != "" {
				inc.Target += " @ " + ref
			}
			inc.Note = "another project: not in this folder"
		case it.Get("remote") != nil:
			inc.Kind, inc.Target, inc.Note = "remote", it.Get("remote").Str(), "fetched by GitLab; codec never downloads"
		case it.Get("template") != nil:
			inc.Kind, inc.Target, inc.Note = "template", it.Get("template").Str(), "a GitLab-provided template"
		case it.Get("component") != nil:
			inc.Kind, inc.Target, inc.Note = "component", it.Get("component").Str(), "a CI/CD component"
		default:
			inc.Kind, inc.Target = "unknown", text(it)
		}
		if it.Get("rules") != nil {
			inc.Note = strings.TrimPrefix(inc.Note+"; only when its rules match", "; ")
		}
		if inc.Kind != "local" {
			short := inc.Kind + " " + inc.Target
			if inc.Kind == "project" {
				short = "project " + it.Get("project").Str()
			}
			g.unresolved = append(g.unresolved, short)
			g.p.Includes = append(g.p.Includes, inc)
			continue
		}
		for _, l := range local {
			g.includeLocal(file, inc, l)
		}
	}
}

func (g *gitlab) includeLocal(file string, inc Include, target string) {
	if g.opts.Load == nil {
		inc.Note = "open the folder to follow includes"
		g.unresolved = append(g.unresolved, "local "+target)
		g.p.Includes = append(g.p.Includes, inc)
		return
	}
	paths := []string{g.opts.repoPath(target)}
	if strings.Contains(target, "*") {
		paths = g.glob(target)
		if len(paths) == 0 {
			inc.State, inc.Note = "missing", "the pattern matches no file"
			g.p.Includes = append(g.p.Includes, inc)
			g.p.problem(yamlkit.SeverityWarning, "gitlab-include-missing", "", &inc.Source, fmt.Sprintf("include %s matches no file", target), "")
			return
		}
	}
	for _, pth := range paths {
		one := inc
		one.File = pth
		if len(paths) > 1 {
			one.Target = pth
		}
		if g.read[pth] {
			one.State, one.Note = "resolved", "already included"
			g.p.Includes = append(g.p.Includes, one)
			continue
		}
		_, f, err := g.opts.load(pth)
		if err != nil {
			one.State = "missing"
			g.p.Includes = append(g.p.Includes, one)
			g.p.problem(yamlkit.SeverityError, "gitlab-include-missing", "", &inc.Source, fmt.Sprintf("Included file %s doesn't exist", pth),
				"Local includes are relative to the repository root.")
			continue
		}
		g.read[pth] = true
		one.State = "resolved"
		g.p.Includes = append(g.p.Includes, one)
		g.readFile(pth, f, false)
	}
}

// glob matches a GitLab include pattern (* within a folder, ** across
// folders) against the workspace files under the repository root.
func (g *gitlab) glob(pattern string) []string {
	var re strings.Builder
	re.WriteString("^")
	p := strings.TrimPrefix(pattern, "/")
	for i := 0; i < len(p); i++ {
		switch {
		case strings.HasPrefix(p[i:], "**"):
			re.WriteString(".*")
			i++
		case p[i] == '*':
			re.WriteString("[^/]*")
		default:
			re.WriteString(regexp.QuoteMeta(p[i : i+1]))
		}
	}
	re.WriteString("$")
	rx, err := regexp.Compile(re.String())
	if err != nil {
		return nil
	}
	var out []string
	for _, f := range g.opts.Files {
		rel := f
		if g.opts.Root != "" {
			if !strings.HasPrefix(f, g.opts.Root+"/") {
				continue
			}
			rel = f[len(g.opts.Root)+1:]
		}
		if rx.MatchString(rel) {
			out = append(out, f)
		}
	}
	return out
}

func isURL(s string) bool { return strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://") }

// unknown grades a name that isn't defined. A near miss of a local
// name is likely a typo; otherwise, when includes codec can't read
// exist, the name probably comes from one of them — worth knowing, not
// a problem.
func (g *gitlab) unknown(name string, candidates []string) (yamlkit.Severity, string) {
	if g.opts.Load == nil && len(g.unresolved) > 0 {
		return "", "" // local includes weren't read: nothing to say
	}
	if c := names.Closest(name, candidates); c != "" {
		if len(g.unresolved) > 0 {
			return yamlkit.SeverityWarning, names.DidYouMean(name, candidates)
		}
		return yamlkit.SeverityError, names.DidYouMean(name, candidates)
	}
	if len(g.unresolved) == 0 {
		return yamlkit.SeverityError, ""
	}
	return yamlkit.SeverityInfo, "It may come from " + g.unresolved[0] + ", which codec doesn't read."
}

// --- !reference ---

// deref replaces !reference tags in n with what they point to. n is not
// changed; maps are rebuilt only where something was replaced.
func (g *gitlab) deref(n *yamlkit.Node, file string, depth int) *yamlkit.Node {
	if n == nil || depth > 10 {
		return n
	}
	if isRef(n) {
		return g.reference(n, file, depth)
	}
	switch n.Kind {
	case yamlkit.KindMap:
		var out *yamlkit.Node
		for i, pr := range n.Pairs {
			v := g.deref(pr.Value, file, depth)
			if v == pr.Value {
				continue
			}
			if out == nil {
				c := *n
				c.Pairs = slices.Clone(n.Pairs)
				out = &c
			}
			out.Pairs[i] = &yamlkit.Pair{Key: pr.Key, Value: v}
		}
		if out != nil {
			return out
		}
	case yamlkit.KindSeq:
		var out *yamlkit.Node
		for i, it := range n.Items {
			v := g.deref(it, file, depth)
			if v == it {
				continue
			}
			if out == nil {
				c := *n
				c.Items = slices.Clone(n.Items)
				out = &c
			}
			out.Items[i] = v
		}
		if out != nil {
			return out
		}
	}
	return n
}

func (g *gitlab) reference(n *yamlkit.Node, file string, depth int) *yamlkit.Node {
	path := strs(n)
	s := srcp(file, n)
	if len(path) < 2 {
		g.p.problem(yamlkit.SeverityError, "gitlab-reference", "", s, "!reference needs a job and at least one key: !reference [.job, script]", "")
		return n
	}
	e, ok := g.cfg[path[0]]
	if !ok {
		sev, hint := g.unknown(path[0], g.order)
		g.p.problem(sev, "gitlab-reference", "", s, fmt.Sprintf("!reference points to %s, which isn't defined", path[0]), hint)
		return n
	}
	v := e.node
	for _, k := range path[1:] {
		next := v.Get(k)
		if next == nil {
			g.p.problem(yamlkit.SeverityError, "gitlab-reference", "", s, fmt.Sprintf("!reference: %s has no %s", strings.Join(path[:slices.Index(path, k)], "."), k), "")
			return n
		}
		v = next
	}
	v = g.deref(v, e.file, depth+1)
	via := origin{File: e.file, Via: "!reference " + path[0]}
	if v.Kind == yamlkit.KindMap {
		return g.t.Copy(v, via, true)
	}
	c := *v
	g.t[&c] = via
	return &c
}

// --- extends ---

// effective is a job's (or template's) configuration with !reference
// and extends applied.
func (g *gitlab) effective(name string) *glEff {
	if e, ok := g.eff[name]; ok {
		return e
	}
	e := g.cfg[name]
	g.busy[name] = true
	defer delete(g.busy, name)
	own := g.deref(e.node, e.file, 0)
	res := &glEff{}
	var acc *yamlkit.Node
	for _, parent := range strs(own.Get("extends")) {
		s := keySrc(e.file, own, "extends")
		switch {
		case g.busy[parent]:
			g.p.problem(yamlkit.SeverityError, "gitlab-extends-cycle", name, s, fmt.Sprintf("%s extends %s, which extends it back", name, parent), "")
			continue
		case g.cfg[parent] == nil || !isMap(g.cfg[parent].node):
			sev, hint := g.unknown(parent, g.order)
			g.p.problem(sev, "gitlab-extends-unknown", name, s, fmt.Sprintf("%s extends %s, which isn't defined", name, parent), hint)
			mark := " (not found)"
			if sev == yamlkit.SeverityInfo || sev == "" {
				mark, res.unread = " (not read)", true
			}
			res.chain = append(res.chain, parent+mark)
			continue
		case len(g.busy) > 11:
			g.p.problem(yamlkit.SeverityError, "gitlab-extends-depth", name, s, "extends nests more than 11 levels deep", "")
			continue
		}
		pe := g.effective(parent)
		res.unread = res.unread || pe.unread
		res.chain = append(res.chain, pe.chain...)
		res.chain = append(res.chain, parent)
		acc = g.t.Merge(acc, pe.node, "extends "+parent, nil)
	}
	res.node = g.t.Merge(acc, own, "", nil)
	if i := indexOf(res.node, "extends"); i >= 0 {
		res.node.Pairs = slices.Delete(res.node.Pairs, i, i+1)
	}
	g.eff[name] = res
	return res
}

// --- the pipeline ---

func (g *gitlab) pipeline() {
	p := g.p
	if wf := g.get("workflow"); wf != nil {
		p.Name = wf.Get("name").Str()
		for _, r := range items(wf.Get("rules")) {
			p.Triggers = append(p.Triggers, "workflow: "+ruleText(r))
		}
	}
	if vars := g.get("variables"); vars != nil {
		for _, pr := range vars.Pairs {
			p.Variables = append(p.Variables, Var{Name: pr.Key.Value, Value: varValue(pr.Value), Scope: "global", Source: g.srcOf(pr.Key)})
		}
	}

	// Stages in order: .pre, the declared ones (or GitLab's defaults), .post.
	declared := strs(g.get("stages"))
	if declared == nil {
		declared = strs(g.get("types"))
	}
	if g.get("stages") == nil && g.get("types") == nil {
		declared = []string{"build", "test", "deploy"}
	}
	stageNames := append(append([]string{".pre"}, declared...), ".post")
	stageAt := map[string]int{}
	for _, s := range stageNames {
		if _, dup := stageAt[s]; !dup {
			stageAt[s] = len(p.Stages)
			p.Stages = append(p.Stages, Stage{Name: s, Defined: true})
		}
	}

	for _, name := range g.order {
		e := g.cfg[name]
		if strings.HasPrefix(name, ".") {
			if isMap(e.node) {
				p.Templates = append(p.Templates, Template{Name: name, Source: src(e.file, e.key)})
			}
			continue
		}
		if glKeywords[name] || !isMap(e.node) {
			continue
		}
		p.Jobs = append(p.Jobs, g.job(name))
	}

	// Stage membership. An undeclared stage goes before .post (GitLab
	// rejects it; showing it keeps the job visible).
	for _, j := range p.Jobs {
		if _, ok := stageAt[j.Stage]; !ok {
			p.problem(yamlkit.SeverityError, "gitlab-stage-undefined", j.ID, keySrc(j.Source.File, j.node, "stage"),
				fmt.Sprintf("Job %s uses stage %q, which isn't in stages", j.ID, j.Stage), names.DidYouMean(j.Stage, declared))
			p.Stages = append(p.Stages[:len(p.Stages)-1], Stage{Name: j.Stage}, p.Stages[len(p.Stages)-1])
			for i := range p.Stages {
				stageAt[p.Stages[i].Name] = i
			}
		}
		s := &p.Stages[stageAt[j.Stage]]
		s.Jobs = append(s.Jobs, j.ID)
	}
	// .pre and .post only show when used.
	p.Stages = slices.DeleteFunc(p.Stages, func(s Stage) bool {
		return (s.Name == ".pre" || s.Name == ".post") && len(s.Jobs) == 0
	})
	g.edges()
}

// get is a top-level entry with !reference applied, or nil.
func (g *gitlab) get(name string) *yamlkit.Node {
	e := g.cfg[name]
	if e == nil {
		return nil
	}
	return g.deref(e.node, e.file, 0)
}

// srcOf is where a tagged key was written.
func (g *gitlab) srcOf(k *yamlkit.Node) Source {
	return src(g.t[k].File, k)
}

func (g *gitlab) job(name string) *Job {
	e := g.cfg[name]
	eff := g.effective(name)
	// A copy: the memoized tree may be extended by other jobs, and
	// default: applies after extends.
	n := g.t.Copy(eff.node, origin{}, false)
	g.inherit(n)
	for _, k := range []string{"before_script", "script", "after_script"} {
		if i := indexOf(n, k); i >= 0 {
			n.Pairs[i].Value = g.flatten(n.Pairs[i].Value)
		}
	}
	j := &Job{ID: name, Name: name, Kind: "job", Source: src(e.file, e.key), Extends: eff.chain, node: n}
	j.Stage = n.Get("stage").Str()
	if j.Stage == "" {
		j.Stage = "test"
	}
	j.When = n.Get("when").Str()
	j.AllowFailure = n.Get("allow_failure").Str() == "true" || isMap(n.Get("allow_failure"))
	for _, r := range items(n.Get("rules")) {
		j.Rules = append(j.Rules, ruleText(r))
	}
	if img := n.Get("image"); img != nil {
		j.Runner = img.Str()
		if isMap(img) {
			j.Runner = img.Get("name").Str()
		}
	}
	if tags := strs(n.Get("tags")); len(tags) > 0 {
		j.Runner = strings.TrimSpace(j.Runner + " · runner " + strings.Join(tags, ", "))
		j.Runner = strings.TrimPrefix(j.Runner, "· ")
	}
	if env := n.Get("environment"); env != nil {
		j.Environment = env.Str()
		if isMap(env) {
			j.Environment = env.Get("name").Str()
		}
	}
	g.trigger(j, n)
	g.parallel(j, n.Get("parallel"))
	for _, sec := range []string{"before_script", "script", "after_script"} {
		v := n.Get(sec)
		if v == nil {
			continue
		}
		st := Step{Name: sec, Kind: "section", Source: g.nodeSrc(n, sec)}
		lines := items(v)
		if v.Kind == yamlkit.KindScalar {
			lines = []*yamlkit.Node{v}
		}
		for _, l := range lines {
			o := g.originOf(n, sec, l)
			st.Children = append(st.Children, Step{Name: firstLine(text(l)), Kind: "script", Source: src(o.File, l), From: o.Via})
		}
		j.Steps = append(j.Steps, st)
	}
	if nd := n.Pair("needs"); nd != nil {
		j.Needs = []Need{}
		for _, it := range items(nd.Value) {
			need := Need{Source: srcp(g.t[nd.Key].File, it)}
			if it.Kind == yamlkit.KindScalar {
				need.Job = it.Value
			} else {
				need.Job = it.Get("job").Str()
				need.Optional = it.Get("optional").Str() == "true"
				var notes []string
				if it.Get("artifacts").Str() == "false" {
					notes = append(notes, "no artifacts")
				}
				if pr := it.Get("project").Str(); pr != "" {
					notes = append(notes, "from project "+pr)
				}
				if pl := it.Get("pipeline").Str(); pl != "" {
					notes = append(notes, "from pipeline "+pl)
				}
				need.Note = strings.Join(notes, ", ")
			}
			j.Needs = append(j.Needs, need)
		}
	}
	j.Effective = emit(g.t, n, origin{File: e.file})
	return j
}

// inherit applies default: (and old-style top-level defaults) and
// global variables to a job's configuration, as its inherit: allows.
func (g *gitlab) inherit(n *yamlkit.Node) {
	allow := func(what string, name string) bool {
		in := n.Get("inherit").Get(what)
		switch {
		case in == nil:
			return true
		case in.Kind == yamlkit.KindScalar:
			return in.Value != "false"
		}
		return slices.Contains(strs(in), name)
	}
	defaults := g.get("default")
	for _, k := range glDefaults {
		if n.Get(k) != nil || !allow("default", k) {
			continue
		}
		var pr *yamlkit.Pair
		via := "default"
		if defaults != nil {
			pr = defaults.Pair(k)
		}
		if pr == nil && (k == "image" || k == "services" || k == "cache" || k == "before_script" || k == "after_script") {
			if top := g.cfg[k]; top != nil {
				pr = &yamlkit.Pair{Key: top.key, Value: g.get(k)}
				g.t[top.key] = origin{File: top.file}
				via = "global " + k
			}
		}
		if pr != nil {
			n.Pairs = append(n.Pairs, g.t.CopyPair(pr, origin{Via: via}))
		}
	}
	global := g.get("variables")
	if global == nil || !isMap(global) {
		return
	}
	in := &yamlkit.Node{Kind: yamlkit.KindMap}
	for _, pr := range global.Pairs {
		if allow("variables", pr.Key.Value) {
			in.Pairs = append(in.Pairs, pr)
		}
	}
	if len(in.Pairs) == 0 {
		return
	}
	merged := g.t.Merge(in, &yamlkit.Node{Kind: yamlkit.KindMap}, "", nil)
	for _, pr := range merged.Pairs {
		o := g.t[pr.Key]
		o.Via = "global variables"
		g.t[pr.Key] = o
	}
	if i := indexOf(n, "variables"); i >= 0 {
		n.Pairs[i].Value = g.t.Merge(merged, n.Pairs[i].Value, "", nil)
		return
	}
	k := &yamlkit.Node{Kind: yamlkit.KindScalar, Tag: yamlkit.TagStr, Value: "variables", Style: yamlkit.StylePlain}
	g.t[k] = origin{File: g.cfg["variables"].file, Via: "global variables"}
	n.Pairs = append(n.Pairs, &yamlkit.Pair{Key: k, Value: merged})
}

// flatten turns nested script lists (from !reference) into one list,
// as GitLab does. Items keep where they came from.
func (g *gitlab) flatten(v *yamlkit.Node) *yamlkit.Node {
	if v == nil || v.Kind != yamlkit.KindSeq {
		return v
	}
	nested := slices.ContainsFunc(v.Items, func(it *yamlkit.Node) bool { return it.Kind == yamlkit.KindSeq })
	if !nested {
		return v
	}
	out := &yamlkit.Node{Kind: yamlkit.KindSeq, Range: v.Range}
	var walk func(n *yamlkit.Node, o origin, tagged bool)
	walk = func(n *yamlkit.Node, o origin, tagged bool) {
		if t, ok := g.t[n]; ok {
			o, tagged = t, true
		}
		if n.Kind != yamlkit.KindSeq {
			if tagged {
				c := *n
				g.t[&c] = o
				n = &c
			}
			out.Items = append(out.Items, n)
			return
		}
		for _, it := range n.Items {
			walk(it, o, tagged)
		}
	}
	for _, it := range v.Items {
		walk(it, origin{}, false)
	}
	if t, ok := g.t[v]; ok {
		g.t[out] = t
	}
	return out
}

// originOf is where item l of section key in job n was written.
func (g *gitlab) originOf(n *yamlkit.Node, key string, l *yamlkit.Node) origin {
	if o, ok := g.t[l]; ok {
		return o
	}
	pr := n.Pair(key)
	if o, ok := g.t[pr.Value]; ok {
		return o
	}
	return g.t[pr.Key]
}

func (g *gitlab) nodeSrc(n *yamlkit.Node, key string) Source {
	pr := n.Pair(key)
	return src(g.t[pr.Key].File, pr.Key)
}

// trigger reads a trigger: job (child or multi-project pipeline).
func (g *gitlab) trigger(j *Job, n *yamlkit.Node) {
	t := n.Get("trigger")
	if t == nil {
		return
	}
	j.Kind = "trigger"
	if t.Kind == yamlkit.KindScalar {
		j.Uses = "project " + t.Value
		return
	}
	if pr := t.Get("project").Str(); pr != "" {
		j.Uses = "project " + pr
		if b := t.Get("branch").Str(); b != "" {
			j.Uses += " @ " + b
		}
		return
	}
	inc := t.Get("include")
	list := items(inc)
	if inc != nil && inc.Kind != yamlkit.KindSeq {
		list = []*yamlkit.Node{inc}
	}
	for _, it := range list {
		target := it.Str()
		if l := it.Get("local").Str(); l != "" {
			target = l
		}
		if a := it.Get("artifact").Str(); a != "" {
			j.Uses = "child pipeline from artifact " + a
			if from := it.Get("job").Str(); from != "" {
				j.Uses += " of job " + from
			}
			continue
		}
		if target == "" {
			j.Uses = "child pipeline " + text(it)
			continue
		}
		j.Uses = "child pipeline " + target
		pth := g.opts.repoPath(target)
		inc := Include{Kind: "child", Target: target, File: pth, State: "resolved", Source: src(g.t[n.Pair("trigger").Key].File, it), Note: "child pipeline of " + j.ID}
		if g.opts.Load == nil {
			inc.State, inc.File = "unresolved", ""
		} else if _, _, err := g.opts.load(pth); err != nil {
			inc.State = "missing"
			g.p.problem(yamlkit.SeverityError, "gitlab-include-missing", j.ID, &inc.Source, fmt.Sprintf("Child pipeline file %s doesn't exist", pth), "")
		}
		g.p.Includes = append(g.p.Includes, inc)
	}
}

// parallel expands parallel: N and parallel: matrix: into the job
// names GitLab gives them.
func (g *gitlab) parallel(j *Job, v *yamlkit.Node) {
	if v == nil {
		return
	}
	if v.Kind == yamlkit.KindScalar {
		var n int
		if _, err := fmt.Sscanf(v.Value, "%d", &n); err == nil && n > 1 {
			for i := 1; i <= n; i++ {
				j.Instances = append(j.Instances, fmt.Sprintf("%s %d/%d", j.ID, i, n))
			}
		}
		return
	}
	for _, entry := range items(v.Get("matrix")) {
		if !isMap(entry) {
			continue
		}
		combos := [][]string{{}}
		for _, pr := range entry.Pairs {
			vals := strs(pr.Value)
			var next [][]string
			for _, c := range combos {
				for _, x := range vals {
					next = append(next, append(slices.Clip(c), x))
				}
			}
			combos = next
		}
		for _, c := range combos {
			j.Instances = append(j.Instances, fmt.Sprintf("%s: [%s]", j.ID, strings.Join(c, ", ")))
		}
	}
}

// edges orders the jobs: needs where given, otherwise every job of the
// earlier stages (only the ones not already waited for through others,
// to keep the graph readable).
func (g *gitlab) edges() {
	p := g.p
	stageOf := map[string]int{}
	for i, s := range p.Stages {
		for _, id := range s.Jobs {
			stageOf[id] = i
		}
	}
	names := make([]string, len(p.Jobs))
	for i, j := range p.Jobs {
		names[i] = j.ID
	}
	ancestors := map[string]map[string]bool{}
	anc := func(id string) map[string]bool {
		if a := ancestors[id]; a != nil {
			return a
		}
		return map[string]bool{}
	}
	add := func(from, to, label string) {
		p.Edges = append(p.Edges, Edge{From: from, To: to, Label: label})
		a := ancestors[to]
		if a == nil {
			a = map[string]bool{}
			ancestors[to] = a
		}
		a[from] = true
		for x := range anc(from) {
			a[x] = true
		}
	}
	for si, s := range p.Stages {
		for _, id := range s.Jobs {
			j := p.Job(id)
			if j.node.Pair("needs") != nil {
				for _, nd := range j.Needs {
					switch {
					case nd.Job == "" || strings.Contains(nd.Note, "from project") || strings.Contains(nd.Note, "from pipeline"):
						continue
					case p.Job(nd.Job) == nil:
						sev, hint := g.unknown(nd.Job, names)
						if nd.Optional && sev == yamlkit.SeverityError {
							sev = yamlkit.SeverityWarning
						}
						p.problem(sev, "gitlab-needs-unknown", id, nd.Source, fmt.Sprintf("Job %s needs %s, which isn't defined", id, nd.Job), hint)
					case stageOf[nd.Job] > si:
						p.problem(yamlkit.SeverityError, "gitlab-needs-later-stage", id, nd.Source,
							fmt.Sprintf("Job %s needs %s, which runs in a later stage (%s)", id, nd.Job, p.Stages[stageOf[nd.Job]].Name), "")
					default:
						label := "needs"
						if nd.Optional {
							label = "optional"
						}
						add(nd.Job, id, label)
					}
				}
				continue
			}
			// Everything in earlier stages, minus what those already wait for.
			var earlier []string
			for _, prev := range p.Stages[:si] {
				earlier = append(earlier, prev.Jobs...)
			}
			covered := map[string]bool{}
			for _, e := range earlier {
				for x := range anc(e) {
					covered[x] = true
				}
			}
			for _, e := range earlier {
				if !covered[e] {
					add(e, id, "stage")
				}
			}
		}
	}
	for _, j := range p.Jobs {
		if j.Kind != "trigger" && j.node.Get("script") == nil && j.node.Get("run") == nil {
			sev, hint := yamlkit.SeverityError, "Add a script (or trigger) — or start the name with a dot to make it a template."
			if g.eff[j.ID].unread {
				sev, hint = yamlkit.SeverityInfo, "It probably comes from a template in "+g.unresolved[0]+", which codec doesn't read."
			}
			if g.eff[j.ID].unread && g.opts.Load == nil {
				continue
			}
			p.problem(sev, "gitlab-job-no-script", j.ID, &j.Source, fmt.Sprintf("Job %s has no script", j.ID), hint)
		}
	}
}

// ruleText summarizes one rules: entry: "if $X == y → manual".
func ruleText(r *yamlkit.Node) string {
	var parts []string
	if c := r.Get("if").Str(); c != "" {
		parts = append(parts, "if "+c)
	}
	for _, k := range []string{"changes", "exists"} {
		if v := r.Get(k); v != nil {
			parts = append(parts, k+" "+strings.Join(strs(v.Get("paths")), ", ")+strings.Join(strs(v), ", "))
		}
	}
	out := strings.Join(parts, " · ")
	if out == "" {
		out = "always"
	}
	if w := r.Get("when").Str(); w != "" {
		out += " → " + w
	}
	if r.Get("allow_failure").Str() == "true" {
		out += " (may fail)"
	}
	return out
}

// varValue is a variable's value: plain, or the value: of the long form.
func varValue(v *yamlkit.Node) string {
	if isMap(v) {
		return text(v.Get("value"))
	}
	return text(v)
}
