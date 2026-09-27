package ci

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/names"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Azure Pipelines: templates (template: file.yml, with parameters) are
// inserted where they are referenced, and ${{ }} template expressions
// (if, each, parameters, static variables) are evaluated, as Azure
// DevOps does when it compiles the pipeline. The result is read as
// stages → jobs → steps: stages run one after another unless dependsOn
// says otherwise; jobs in a stage run in parallel unless dependsOn
// orders them.

type azure struct {
	p    *Pipeline
	opts Options
	t    tags
	root *yamlkit.Node
}

// azMaxDepth is how deeply templates may include templates.
const azMaxDepth = 20

// azExpr matches a whole ${{ … }} expression.
var azExpr = regexp.MustCompile(`\$\{\{(.*?)\}\}`)

func readAzure(p *Pipeline, content []byte, f *yamlkit.File, opts Options) {
	r := root(f)
	if r == nil {
		return
	}
	r = resolved(r)
	az := &azure{p: p, opts: opts, t: tags{}, root: r}
	p.Name = r.Get("name").Str()
	p.Triggers = azTriggers(r)

	decl, _ := azParams(p.File, r.Get("parameters"))
	p.Inputs = inputsOf(decl)
	sc := &azScope{params: map[string]*yamlkit.Node{}, vars: map[string]string{}}
	for _, in := range decl {
		v := in.node
		if s, ok := opts.Params[in.Name]; ok {
			v = scalarNode(s)
		}
		if v != nil {
			sc.params[in.Name] = v
		}
	}
	at := origin{File: p.File}
	body := r
	if ext := r.Get("extends"); ext != nil {
		body = az.extends(ext, sc, at)
	}
	vars := az.expandSeqOrMap(body.Get("variables"), "variables", sc, at, 0)
	if v := r.Get("variables"); body != r && v != nil {
		vars = az.expandSeqOrMap(v, "variables", sc, at, 0)
	}
	p.Variables = az.variables(vars, "pipeline", sc)
	pool := poolText(body.Get("pool"))

	switch {
	case body.Get("stages") != nil:
		for _, st := range items(az.expandSeq(body.Get("stages"), "stages", sc, at, 0)) {
			az.stage(st, pool, sc)
		}
	case body.Get("jobs") != nil:
		p.Stages = []Stage{{Name: "", Defined: true}}
		for _, j := range items(az.expandSeq(body.Get("jobs"), "jobs", sc, at, 0)) {
			az.job(&p.Stages[0], j, pool, sc)
		}
	case body.Get("steps") != nil:
		steps := az.expandSeq(body.Get("steps"), "steps", sc, at, 0)
		j := &Job{ID: "Job", Name: "Job", Kind: "job", Runner: pool, Source: *keySrc(p.File, body, "steps"), node: body, Needs: []Need{}}
		j.Steps = az.steps(steps)
		j.Effective = emit(az.t, &yamlkit.Node{Kind: yamlkit.KindMap, Pairs: []*yamlkit.Pair{{Key: body.Pair("steps").Key, Value: steps}}}, at)
		p.Stages = []Stage{{Name: "", Defined: true, Jobs: []string{j.ID}}}
		p.Jobs = append(p.Jobs, j)
	}
	if len(p.Stages) > 0 || body != r {
		p.Kind = "workflow"
	}
	if len(p.Inputs) > 0 && r.Get("trigger") == nil && r.Get("pr") == nil && r.Get("schedules") == nil && !strings.HasPrefix(path.Base(p.File), "azure-pipelines") {
		p.Kind = "template"
	}
	az.edges()
}

func azTriggers(r *yamlkit.Node) []string {
	var out []string
	for _, k := range []string{"trigger", "pr"} {
		v := r.Get(k)
		switch {
		case v == nil:
			continue
		case v.Kind == yamlkit.KindScalar && v.Value == "none":
			out = append(out, k+": none")
		case v.Kind == yamlkit.KindSeq:
			out = append(out, k+" · branches "+strings.Join(strs(v), ", "))
		case isMap(v):
			t := k
			if b := strs(v.Get("branches").Get("include")); len(b) > 0 {
				t += " · branches " + strings.Join(b, ", ")
			}
			if ps := strs(v.Get("paths").Get("include")); len(ps) > 0 {
				t += " · paths " + strings.Join(ps, ", ")
			}
			out = append(out, t)
		default:
			out = append(out, k+" "+text(v))
		}
	}
	for _, s := range items(r.Get("schedules")) {
		out = append(out, "schedule · "+s.Get("cron").Str())
	}
	for _, res := range items(r.Get("resources").Get("pipelines")) {
		if res.Get("trigger") != nil {
			out = append(out, "after pipeline "+res.Get("source").Str())
		}
	}
	return out
}

// azInput is a declared template parameter with its default node.
type azInput struct {
	Input
	node   *yamlkit.Node
	values []string
}

// azParams reads parameters: in either form (a list of {name, type,
// default, values}, or the old map of name: default).
func azParams(file string, n *yamlkit.Node) ([]azInput, map[string]bool) {
	var out []azInput
	names := map[string]bool{}
	switch {
	case n == nil:
	case n.Kind == yamlkit.KindSeq:
		for _, it := range n.Items {
			name := it.Get("name").Str()
			if name == "" {
				continue
			}
			in := azInput{Input: Input{Name: name, Type: it.Get("type").Str(), Description: it.Get("displayName").Str(), From: "parameters", Source: src(file, it)}}
			if in.Type == "" {
				in.Type = "string"
			}
			in.values = strs(it.Get("values"))
			in.Options = in.values
			if d := it.Pair("default"); d != nil {
				in.node, in.HasDefault, in.Default = d.Value, true, text(d.Value)
			} else {
				in.Required = true
			}
			out = append(out, in)
			names[name] = true
		}
	case isMap(n):
		for _, pr := range n.Pairs {
			name := keyName(pr.Key)
			out = append(out, azInput{Input: Input{Name: name, Type: "object", Default: text(pr.Value), HasDefault: true, From: "parameters", Source: src(file, pr.Key)}, node: pr.Value})
			names[name] = true
		}
	}
	return out, names
}

// Inputs flattens azInputs for the pipeline.
func inputsOf(in []azInput) []Input {
	out := make([]Input, len(in))
	for i, x := range in {
		out[i] = x.Input
	}
	return out
}

func scalarNode(s string) *yamlkit.Node {
	return &yamlkit.Node{Kind: yamlkit.KindScalar, Tag: yamlkit.TagStr, Value: s, Style: yamlkit.StylePlain}
}

// --- template expansion ---

// expandSeqOrMap expands variables:, which may be a map or a list.
func (az *azure) expandSeqOrMap(n *yamlkit.Node, ctx string, sc *azScope, at origin, depth int) *yamlkit.Node {
	if n != nil && n.Kind == yamlkit.KindSeq {
		return az.expandSeq(n, ctx, sc, at, depth)
	}
	return az.expand(n, sc, at, depth)
}

// expand evaluates template expressions in n. Nodes it creates are
// tagged with at (where they were written).
func (az *azure) expand(n *yamlkit.Node, sc *azScope, at origin, depth int) *yamlkit.Node {
	if n == nil {
		return nil
	}
	switch n.Kind {
	case yamlkit.KindMap:
		return az.expandMap(n, sc, at, depth)
	case yamlkit.KindSeq:
		ctx := ""
		return az.expandSeq(n, ctx, sc, at, depth)
	case yamlkit.KindScalar:
		if !strings.Contains(n.Value, "${{") {
			return az.tag(n, at)
		}
		// A value that is exactly one expression takes the value's type
		// (a list of steps, an object); otherwise text is interpolated.
		if m := azExpr.FindStringSubmatchIndex(n.Value); m != nil && m[0] == 0 && m[1] == len(strings.TrimSpace(n.Value)) {
			v := azEval(n.Value[m[2]:m[3]], sc)
			if !v.known {
				return az.tag(n, at)
			}
			if v.n.Kind != yamlkit.KindScalar {
				return v.n
			}
			c := *v.n
			c.Range = n.Range
			return az.tag(&c, at)
		}
		c := *n
		c.Value = azExpr.ReplaceAllStringFunc(n.Value, func(e string) string {
			v := azEval(e[3:len(e)-2], sc)
			if !v.known {
				return e
			}
			return v.str()
		})
		c.Templated = strings.Contains(c.Value, "${{")
		return az.tag(&c, at)
	}
	return n
}

func (az *azure) tag(n *yamlkit.Node, at origin) *yamlkit.Node {
	if _, ok := az.t[n]; !ok {
		c := *n
		az.t[&c] = at
		return &c
	}
	return n
}

// directive reads a ${{ if|elseif|else|each }} key.
func directive(key string) (kind, arg string) {
	m := azExpr.FindStringSubmatch(key)
	if m == nil || strings.TrimSpace(key) != m[0] {
		return "", ""
	}
	body := strings.TrimSpace(m[1])
	word, rest, _ := strings.Cut(body, " ")
	switch word {
	case "if", "elseif", "each":
		return word, strings.TrimSpace(rest)
	case "else", "insert":
		return word, ""
	}
	return "", ""
}

// cond tracks an if/elseif/else chain: true once a branch was taken,
// unknown when a condition couldn't be decided.
type cond struct {
	taken, unsure bool
}

// branch decides whether a directive's content is included, and with
// which note (for conditions that can't be decided).
func (c *cond) branch(kind, arg string, sc *azScope) (bool, string) {
	switch kind {
	case "if":
		*c = cond{}
		fallthrough
	case "elseif":
		if c.taken {
			return false, ""
		}
		v := azEval(arg, sc)
		if !v.known {
			c.unsure = true
			return true, "if " + arg
		}
		c.taken = v.truthy()
		return c.taken, ""
	case "else":
		if c.taken {
			return false, ""
		}
		if c.unsure {
			return true, "else of an undecided if"
		}
		c.taken = true
		return true, ""
	}
	return false, ""
}

func noted(at origin, note string) origin {
	if note == "" {
		return at
	}
	at.Via = strings.TrimPrefix(at.Via+" · "+note, " · ")
	return at
}

func (az *azure) expandMap(n *yamlkit.Node, sc *azScope, at origin, depth int) *yamlkit.Node {
	out := &yamlkit.Node{Kind: yamlkit.KindMap, Range: n.Range, Flow: n.Flow}
	var c cond
	for _, pr := range n.Pairs {
		kind, arg := directive(keyName(pr.Key))
		switch kind {
		case "if", "elseif", "else":
			ok, note := c.branch(kind, arg, sc)
			if ok {
				if sub := az.expand(pr.Value, sc, noted(at, note), depth); isMap(sub) {
					out.Pairs = append(out.Pairs, sub.Pairs...)
				}
			}
			continue
		case "insert":
			if sub := az.expand(pr.Value, sc, at, depth); isMap(sub) {
				out.Pairs = append(out.Pairs, sub.Pairs...)
			}
			continue
		case "each":
			az.each(arg, sc, func(sc2 *azScope) {
				if sub := az.expand(pr.Value, sc2, at, depth); isMap(sub) {
					out.Pairs = append(out.Pairs, sub.Pairs...)
				}
			})
			continue
		}
		c = cond{}
		k := az.expand(pr.Key, sc, at, depth)
		v := pr.Value
		switch keyName(pr.Key) {
		case "steps", "jobs", "stages", "variables":
			if v != nil && v.Kind == yamlkit.KindSeq {
				v = az.expandSeq(v, keyName(pr.Key), sc, at, depth)
				out.Pairs = append(out.Pairs, &yamlkit.Pair{Key: k, Value: v})
				continue
			}
		}
		out.Pairs = append(out.Pairs, &yamlkit.Pair{Key: k, Value: az.expand(v, sc, at, depth)})
	}
	az.t[out] = at
	return out
}

// each runs fn once per element of the collection in "x in coll".
func (az *azure) each(arg string, sc *azScope, fn func(*azScope)) {
	name, coll, ok := strings.Cut(arg, " in ")
	if !ok {
		return
	}
	v := azEval(coll, sc)
	if !v.known || v.n == nil {
		az.p.problem(yamlkit.SeverityInfo, "azure-each-unknown", "", nil, fmt.Sprintf("each over %s: not known before the pipeline runs, so its items aren't shown", strings.TrimSpace(coll)), "")
		return
	}
	name = strings.TrimSpace(name)
	switch v.n.Kind {
	case yamlkit.KindSeq:
		for _, it := range v.n.Items {
			fn(sc.with(name, it))
		}
	case yamlkit.KindMap:
		for _, pr := range v.n.Pairs {
			pair := &yamlkit.Node{Kind: yamlkit.KindMap, Pairs: []*yamlkit.Pair{
				{Key: scalarNode("key"), Value: pr.Key},
				{Key: scalarNode("value"), Value: pr.Value},
			}}
			fn(sc.with(name, pair))
		}
	}
}

// expandSeq expands a list: directives and inserted parameter lists
// splice items in, and template: items are replaced by the template's
// items of the same kind (ctx: steps, jobs, stages, variables).
func (az *azure) expandSeq(n *yamlkit.Node, ctx string, sc *azScope, at origin, depth int) *yamlkit.Node {
	if n == nil {
		return &yamlkit.Node{Kind: yamlkit.KindSeq, Items: []*yamlkit.Node{}}
	}
	out := &yamlkit.Node{Kind: yamlkit.KindSeq, Range: n.Range, Items: []*yamlkit.Node{}}
	if n.Kind != yamlkit.KindSeq {
		v := az.expand(n, sc, at, depth)
		if v != nil && v.Kind == yamlkit.KindSeq {
			return v
		}
		return out
	}
	var c cond
	splice := func(v *yamlkit.Node) {
		switch {
		case v == nil:
		case v.Kind == yamlkit.KindSeq:
			out.Items = append(out.Items, v.Items...)
		default:
			out.Items = append(out.Items, v)
		}
	}
	for _, it := range n.Items {
		if isMap(it) && len(it.Pairs) == 1 {
			if kind, arg := directive(keyName(it.Pairs[0].Key)); kind != "" {
				body := it.Pairs[0].Value
				switch kind {
				case "if", "elseif", "else":
					if ok, note := c.branch(kind, arg, sc); ok {
						splice(az.expandSeq(body, ctx, sc, noted(at, note), depth))
					}
				case "each":
					az.each(arg, sc, func(sc2 *azScope) { splice(az.expandSeq(body, ctx, sc2, at, depth)) })
				case "insert":
					splice(az.expandSeq(body, ctx, sc, at, depth))
				}
				continue
			}
		}
		c = cond{}
		if isMap(it) && it.Get("template") != nil && ctx != "" {
			splice(az.template(it, ctx, sc, at, depth))
			continue
		}
		v := az.expand(it, sc, at, depth)
		// An item that is one expression may insert a list (a stepList
		// parameter, or an each loop variable holding steps).
		splice(v)
	}
	return out
}

// template inserts a template file's items.
func (az *azure) template(it *yamlkit.Node, ctx string, sc *azScope, at origin, depth int) *yamlkit.Node {
	ref := az.expand(it.Get("template"), sc, at, depth).Str()
	inc := Include{Kind: "template-file", Target: ref, Source: src(at.File, it.Get("template")), State: "unresolved"}
	placeholder := func(note string) *yamlkit.Node {
		inc.Note = note
		az.p.Includes = append(az.p.Includes, inc)
		c := *it
		az.t[&c] = at
		return &c
	}
	file, repo, _ := strings.Cut(ref, "@")
	if repo != "" && repo != "self" {
		return placeholder("in repository " + repo)
	}
	if az.opts.Load == nil {
		return placeholder("open the folder to follow templates")
	}
	if depth >= azMaxDepth {
		az.p.problem(yamlkit.SeverityError, "azure-template-depth", "", &inc.Source, "Templates nest more than 20 levels deep", "")
		return placeholder("too deep")
	}
	p := az.templatePath(at.File, file)
	_, f, err := az.opts.load(p)
	if err != nil {
		inc.State = "missing"
		az.p.problem(yamlkit.SeverityError, "azure-template-missing", "", &inc.Source, fmt.Sprintf("Template %s doesn't exist", p),
			"Template paths are relative to the file that uses them; start with / for the repository root.")
		return placeholder("")
	}
	inc.State, inc.File = "resolved", p
	if !slices.ContainsFunc(az.p.Includes, func(i Include) bool { return i.File == p && i.Source == inc.Source }) {
		az.p.Includes = append(az.p.Includes, inc)
	}
	tr := resolved(root(f))
	if tr == nil {
		return nil
	}
	sc2 := az.bind(p, tr.Get("parameters"), it.Get("parameters"), sc, at, depth, &inc.Source)
	tat := origin{File: p, Via: "template " + path.Base(p)}
	if at.Via != "" && !strings.HasPrefix(at.Via, "template ") {
		tat.Via = at.Via + " · " + tat.Via
	}
	body := tr.Get(ctx)
	if body == nil {
		return nil
	}
	return az.expandSeqOrMap(body, ctx, sc2, tat, depth+1)
}

// templatePath resolves a template reference: relative to the file
// that uses it, or to the repository root when it starts with /.
func (az *azure) templatePath(from, ref string) string {
	if strings.HasPrefix(ref, "/") {
		return az.opts.repoPath(ref)
	}
	return strings.TrimPrefix(path.Join(path.Dir(from), ref), "./")
}

// bind computes a template's parameter values from what the caller
// passes (evaluated in the caller's scope) and the defaults.
func (az *azure) bind(file string, decl, passed *yamlkit.Node, sc *azScope, at origin, depth int, s *Source) *azScope {
	params, declared := azParams(file, decl)
	out := &azScope{params: map[string]*yamlkit.Node{}, vars: sc.vars}
	var given *yamlkit.Node
	if passed != nil {
		given = az.expand(passed, sc, at, depth)
	}
	for _, in := range params {
		v := given.Get(in.Name)
		switch {
		case v != nil:
			if len(in.values) > 0 && v.Kind == yamlkit.KindScalar && !v.Templated && !slices.ContainsFunc(in.values, func(x string) bool { return strings.EqualFold(x, v.Value) }) {
				az.p.problem(yamlkit.SeverityError, "azure-param-value", "", s, fmt.Sprintf("Parameter %s of %s is %q; allowed: %s", in.Name, path.Base(file), v.Value, strings.Join(in.values, ", ")), "")
			}
			out.params[in.Name] = v
		case in.node != nil:
			out.params[in.Name] = az.expand(in.node, out, origin{File: file, Via: "template " + path.Base(file)}, depth)
		case in.Required:
			az.p.problem(yamlkit.SeverityError, "azure-param-missing", "", s, fmt.Sprintf("Template %s needs parameter %s", path.Base(file), in.Name), "")
		}
	}
	if isMap(given) && decl != nil {
		for _, pr := range given.Pairs {
			if name := keyName(pr.Key); !declared[name] {
				az.p.problem(yamlkit.SeverityError, "azure-param-unknown", "", s, fmt.Sprintf("Template %s has no parameter %s", path.Base(file), name), names.DidYouMean(name, keys(declared)))
			}
		}
	}
	return out
}

// extends replaces the pipeline body with the template it extends.
func (az *azure) extends(ext *yamlkit.Node, sc *azScope, at origin) *yamlkit.Node {
	one := &yamlkit.Node{Kind: yamlkit.KindMap, Pairs: []*yamlkit.Pair{{Key: scalarNode("template"), Value: ext.Get("template")}, {Key: scalarNode("parameters"), Value: ext.Get("parameters")}}}
	ref := ext.Get("template").Str()
	file, repo, _ := strings.Cut(ref, "@")
	if repo != "" && repo != "self" || az.opts.Load == nil {
		az.template(one, "stages", sc, at, 0) // records it as unresolved
		return az.root
	}
	p := az.templatePath(az.p.File, file)
	_, f, err := az.opts.load(p)
	if err != nil {
		az.template(one, "stages", sc, at, 0) // reports it missing
		return az.root
	}
	az.p.Includes = append(az.p.Includes, Include{Kind: "template-file", Target: ref, File: p, State: "resolved", Note: "extends", Source: src(az.p.File, ext.Get("template"))})
	tr := resolved(root(f))
	sc2 := az.bind(p, tr.Get("parameters"), ext.Get("parameters"), sc, at, 0, srcp(az.p.File, ext))
	tat := origin{File: p, Via: "extends " + path.Base(p)}
	body := &yamlkit.Node{Kind: yamlkit.KindMap}
	for _, k := range []string{"variables", "pool", "stages", "jobs", "steps"} {
		if pr := tr.Pair(k); pr != nil {
			body.Pairs = append(body.Pairs, &yamlkit.Pair{Key: pr.Key, Value: az.expandSeqOrMap(pr.Value, k, sc2, tat, 1)})
		}
	}
	return body
}

// --- reading the expanded pipeline ---

func poolText(v *yamlkit.Node) string {
	switch {
	case v == nil:
		return ""
	case isMap(v):
		if img := v.Get("vmImage").Str(); img != "" {
			return img
		}
		return v.Get("name").Str()
	}
	return v.Str()
}

func (az *azure) variables(n *yamlkit.Node, scope string, sc *azScope) []Var {
	var out []Var
	add := func(name, value string, key *yamlkit.Node) {
		o := az.t[key]
		out = append(out, Var{Name: name, Value: value, Scope: scope, Source: src(orFile(o.File, az.p.File), key)})
		if scope == "pipeline" && !strings.Contains(value, "$") {
			sc.vars[name] = value
		}
	}
	switch {
	case n == nil:
	case isMap(n):
		for _, pr := range n.Pairs {
			add(keyName(pr.Key), text(pr.Value), pr.Key)
		}
	case n.Kind == yamlkit.KindSeq:
		for _, it := range n.Items {
			switch {
			case it.Get("group") != nil:
				out = append(out, Var{Name: it.Get("group").Str(), Value: "(variable group)", Scope: "group", Source: src(orFile(az.t[it].File, az.p.File), it)})
			case it.Get("name") != nil:
				add(it.Get("name").Str(), text(it.Get("value")), it)
			}
		}
	}
	return out
}

func orFile(f, def string) string {
	if f == "" {
		return def
	}
	return f
}

// fileOf is the file an expanded node was written in.
func (az *azure) fileOf(n *yamlkit.Node) string {
	return orFile(az.t[n].File, az.p.File)
}

func (az *azure) stage(st *yamlkit.Node, pool string, sc *azScope) {
	p := az.p
	name := st.Get("stage").Str()
	if name == "" && st.Get("template") != nil {
		p.Stages = append(p.Stages, Stage{Name: "template " + st.Get("template").Str(), Defined: true, Source: srcp(az.fileOf(st), st)})
		return
	}
	if name == "" {
		return
	}
	s := Stage{Name: name, Title: st.Get("displayName").Str(), If: text(st.Get("condition")), Defined: true, Source: srcp(az.fileOf(st), st)}
	if o := az.t[st]; strings.HasPrefix(o.Via, "template ") || strings.Contains(o.Via, "template ") {
		s.From = o.File
	}
	if d := st.Get("dependsOn"); d != nil {
		s.DependsOn = orEmpty(strs(d))
	}
	if pl := poolText(st.Get("pool")); pl != "" {
		pool = pl
	}
	p.Stages = append(p.Stages, s)
	idx := len(p.Stages) - 1
	for _, j := range items(st.Get("jobs")) {
		az.job(&p.Stages[idx], j, pool, sc)
	}
}

func (az *azure) job(s *Stage, n *yamlkit.Node, pool string, sc *azScope) {
	p := az.p
	id := n.Get("job").Str()
	kind := "job"
	if d := n.Get("deployment").Str(); d != "" {
		id, kind = d, "deployment"
	}
	file := az.fileOf(n)
	if id == "" {
		if t := n.Get("template").Str(); t != "" {
			id, kind = "template "+t, "template"
		} else {
			id = "Job"
		}
	}
	prefix := ""
	if s.Name != "" {
		prefix = s.Name + "."
	}
	j := &Job{ID: prefix + id, Name: id, Stage: s.Name, Kind: kind, Source: src(file, n), node: n, Needs: []Need{}}
	if dn := n.Get("displayName").Str(); dn != "" {
		j.Name = dn
	}
	if o := az.t[n]; strings.Contains(o.Via, "template ") {
		j.From = o.File
	}
	j.If = text(n.Get("condition"))
	j.Runner = pool
	if pl := poolText(n.Get("pool")); pl != "" {
		j.Runner = pl
	}
	if c := n.Get("container"); c != nil {
		img := c.Str()
		if isMap(c) {
			img = c.Get("image").Str()
		}
		j.Runner = strings.TrimPrefix(j.Runner+" · container "+img, " · ")
	}
	if env := n.Get("environment"); env != nil {
		j.Environment = env.Str()
		if isMap(env) {
			j.Environment = env.Get("name").Str()
		}
	}
	j.AllowFailure = n.Get("continueOnError").Str() == "true"
	if d := n.Get("dependsOn"); d != nil {
		for _, x := range strs(d) {
			j.Needs = append(j.Needs, Need{Job: prefix + x, Source: srcp(file, d)})
		}
	}
	if strat := n.Get("strategy"); strat != nil {
		if m := strat.Get("matrix"); isMap(m) {
			mx := &Matrix{Axes: []Axis{}, Combos: []Combo{}, MaxParallel: strat.Get("maxParallel").Str()}
			for _, pr := range m.Pairs {
				c := Combo{Name: j.Name + " " + keyName(pr.Key), Values: []KV{}}
				if isMap(pr.Value) {
					for _, v := range pr.Value.Pairs {
						c.Values = append(c.Values, KV{keyName(v.Key), text(v.Value)})
					}
				}
				mx.Combos = append(mx.Combos, c)
			}
			j.Matrix = mx
		} else if m != nil {
			j.Matrix = &Matrix{Axes: []Axis{}, Combos: []Combo{}, Runtime: text(m)}
		}
		if par := strat.Get("parallel").Str(); par != "" {
			j.Instances = []string{j.Name + " × " + par}
		}
	}
	if steps := n.Get("steps"); steps != nil {
		j.Steps = az.steps(steps)
	}
	// Deployment jobs keep their steps under strategy.<kind>.<hook>.
	if strat := n.Get("strategy"); kind == "deployment" && isMap(strat) {
		for _, sk := range strat.Pairs {
			if !isMap(sk.Value) {
				continue
			}
			for _, hook := range sk.Value.Pairs {
				steps := hook.Value.Get("steps")
				if hook.Key.Value == "on" {
					for _, h := range hook.Value.Pairs {
						j.Steps = append(j.Steps, Step{Name: sk.Key.Value + " · on " + h.Key.Value, Kind: "section", Source: src(file, h.Key), Children: az.steps(h.Value.Get("steps"))})
					}
					continue
				}
				if steps != nil {
					j.Steps = append(j.Steps, Step{Name: sk.Key.Value + " · " + hook.Key.Value, Kind: "section", Source: src(file, hook.Key), Children: az.steps(steps)})
				}
			}
		}
	}
	j.Effective = emit(az.t, n, origin{File: file})
	s.Jobs = append(s.Jobs, j.ID)
	p.Jobs = append(p.Jobs, j)
}

var azStepKinds = []string{"script", "bash", "pwsh", "powershell", "task", "checkout", "download", "downloadBuild", "publish", "getPackage", "reviewApp", "template"}

func (az *azure) steps(n *yamlkit.Node) []Step {
	var out []Step
	for _, st := range items(n) {
		s := Step{Name: st.Get("displayName").Str(), If: text(st.Get("condition")), Source: src(az.fileOf(st), st)}
		for _, k := range azStepKinds {
			v := st.Get(k)
			if v == nil {
				continue
			}
			s.Kind = k
			switch k {
			case "script", "bash", "pwsh", "powershell":
				s.Detail = firstLine(v.Str())
			default:
				s.Detail = text(v)
			}
			break
		}
		if s.Kind == "" {
			s.Kind = "step"
		}
		if s.Name == "" {
			s.Name = s.Detail
		}
		if s.Name == "" {
			s.Name = s.Kind
		}
		if o := az.t[st]; strings.Contains(o.Via, "template ") || strings.Contains(o.Via, "if ") {
			s.From = o.File
			if i := strings.Index(o.Via, "if "); i >= 0 {
				s.Note = "only " + o.Via[i:] + " (not known before the run)"
			}
		}
		if s.Kind == "template" {
			s.Note = "template not read"
		}
		out = append(out, s)
	}
	return out
}

// edges: stages in order (or by dependsOn), jobs by dependsOn.
func (az *azure) edges() {
	p := az.p
	byStage := map[string]*Stage{}
	var stageNames []string
	for i := range p.Stages {
		byStage[p.Stages[i].Name] = &p.Stages[i]
		stageNames = append(stageNames, p.Stages[i].Name)
	}
	for _, j := range p.Jobs {
		var ids []string
		for _, id := range byStage[j.Stage].Jobs {
			ids = append(ids, strings.TrimPrefix(id, j.Stage+"."))
		}
		for _, nd := range j.Needs {
			if p.Job(nd.Job) == nil {
				name := strings.TrimPrefix(nd.Job, j.Stage+".")
				p.problem(yamlkit.SeverityError, "azure-depends-unknown", j.ID, nd.Source, fmt.Sprintf("Job %s depends on %s, which isn't a job of stage %s", j.Name, name, orFile(j.Stage, "(the pipeline)")), names.DidYouMean(name, ids))
				continue
			}
			p.Edges = append(p.Edges, Edge{From: nd.Job, To: j.ID, Label: "dependsOn"})
		}
	}
	// leaves are the jobs of a stage nothing else in it waits for.
	leaves := func(s *Stage) []string {
		needed := map[string]bool{}
		for _, id := range s.Jobs {
			for _, nd := range p.Job(id).Needs {
				needed[nd.Job] = true
			}
		}
		var out []string
		for _, id := range s.Jobs {
			if !needed[id] {
				out = append(out, id)
			}
		}
		return out
	}
	for i := range p.Stages {
		s := &p.Stages[i]
		deps := s.DependsOn
		if deps == nil && i > 0 {
			deps = []string{p.Stages[i-1].Name}
		}
		for _, d := range deps {
			ds := byStage[d]
			if ds == nil {
				p.problem(yamlkit.SeverityError, "azure-depends-unknown", "", s.Source, fmt.Sprintf("Stage %s depends on %s, which isn't a stage", s.Name, d), names.DidYouMean(d, stageNames))
				continue
			}
			for _, id := range s.Jobs {
				if len(p.Job(id).Needs) > 0 {
					continue
				}
				for _, from := range leaves(ds) {
					p.Edges = append(p.Edges, Edge{From: from, To: id, Label: "stage"})
				}
			}
		}
	}
}
