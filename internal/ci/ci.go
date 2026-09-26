// Package ci is the CI pipelines lens: it reads GitHub Actions
// workflows, GitLab CI pipelines and Azure Pipelines and explains what
// runs, in which order, with which configuration — after the things
// that make pipelines hard to read are applied: GitLab includes,
// extends, anchors and !reference; GitHub matrices, reusable workflows
// and composite actions; Azure templates and their parameters.
//
// Like yamlkit the package is pure. Other files (includes, templates,
// actions) come through Options.Load, which the adapter backs with the
// workspace; nothing here touches the disk or the network.
package ci

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Tools this package reads, by provider id.
const (
	GitHub = "github-actions"
	GitLab = "gitlab-ci"
	Azure  = "azure-pipelines"
)

// Source says where something is written.
type Source struct {
	File  string        `json:"file,omitempty"` // workspace path; "" = the analyzed file when it has none
	Line  int           `json:"line"`
	Range yamlkit.Range `json:"range"`
}

// Pipeline is one pipeline file, read and resolved.
type Pipeline struct {
	Tool string `json:"tool"`
	File string `json:"file"`
	Name string `json:"name,omitempty"`
	// Kind is "workflow", "action" (a GitHub composite action), or
	// "template" (a file only meant to be included or extended).
	Kind      string     `json:"kind"`
	Triggers  []string   `json:"triggers"`
	Inputs    []Input    `json:"inputs"`
	Variables []Var      `json:"variables"`
	Stages    []Stage    `json:"stages"`
	Jobs      []*Job     `json:"jobs"`
	Edges     []Edge     `json:"edges"`
	Includes  []Include  `json:"includes"`
	Templates []Template `json:"templates"` // GitLab hidden jobs
	Problems  []Problem  `json:"problems"`
}

// Stage is a GitLab or Azure stage, in execution order.
type Stage struct {
	Name      string   `json:"name"`
	Title     string   `json:"title,omitempty"` // displayName
	Jobs      []string `json:"jobs"`            // job ids
	DependsOn []string `json:"dependsOn,omitempty"`
	If        string   `json:"if,omitempty"`
	Source    *Source  `json:"source,omitempty"`
	From      string   `json:"from,omitempty"` // template file it came from
	Defined   bool     `json:"defined"`        // listed in stages (GitLab); false = used but not declared
}

// Job is one job (or one GitHub job of a called reusable workflow).
type Job struct {
	ID    string `json:"id"`   // unique in the pipeline
	Name  string `json:"name"` // shown name
	Stage string `json:"stage,omitempty"`
	// Kind is job, reusable (GitHub uses:), trigger (GitLab child or
	// multi-project pipeline), deployment (Azure), template (an Azure
	// template that couldn't be read).
	Kind   string `json:"kind"`
	Group  string `json:"group,omitempty"` // GitHub: the job that called this job's workflow
	Source Source `json:"source"`

	Needs        []Need   `json:"needs"`
	If           string   `json:"if,omitempty"`   // GitHub if, Azure condition
	When         string   `json:"when,omitempty"` // GitLab when (manual, always, …)
	Rules        []string `json:"rules,omitempty"`
	Runner       string   `json:"runner,omitempty"` // runs-on, image, pool
	Environment  string   `json:"environment,omitempty"`
	AllowFailure bool     `json:"allowFailure,omitempty"`

	Matrix    *Matrix  `json:"matrix,omitempty"`
	Instances []string `json:"instances,omitempty"` // GitLab parallel jobs
	Steps     []Step   `json:"steps"`
	Outputs   []KV     `json:"outputs,omitempty"`
	Extends   []string `json:"extends,omitempty"` // GitLab: templates merged in, in order
	Uses      string   `json:"uses,omitempty"`    // reusable workflow; GitLab trigger target
	With      []Arg    `json:"with,omitempty"`    // inputs passed to a reusable workflow or template
	From      string   `json:"from,omitempty"`    // template file the job came from

	// Effective is the job's configuration after includes, extends,
	// default, anchors, !reference and templates, one line per entry,
	// each saying where it was written.
	Effective []Line `json:"effective"`

	node *yamlkit.Node
}

// Need is a job this job waits for.
type Need struct {
	Job      string  `json:"job"` // job id
	Optional bool    `json:"optional,omitempty"`
	Note     string  `json:"note,omitempty"` // e.g. "artifacts: false"
	Source   *Source `json:"source,omitempty"`
}

// Step is one step of a job.
type Step struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`             // run, uses, script, task, checkout, template, …
	Detail   string `json:"detail,omitempty"` // action, task or the first line of a script
	If       string `json:"if,omitempty"`
	Source   Source `json:"source"`
	From     string `json:"from,omitempty"` // template or action file
	Children []Step `json:"children,omitempty"`
	Note     string `json:"note,omitempty"`
}

// Matrix is a GitHub (or Azure) job matrix and the jobs it expands to.
type Matrix struct {
	Axes        []Axis  `json:"axes"`
	Include     []Combo `json:"include,omitempty"`
	Exclude     []Combo `json:"exclude,omitempty"`
	Combos      []Combo `json:"combos"`
	Runtime     string  `json:"runtime,omitempty"` // set when the matrix is an expression known at run time
	MaxParallel string  `json:"maxParallel,omitempty"`
	FailFast    string  `json:"failFast,omitempty"`
}

// Axis is one matrix variable and its values.
type Axis struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

// Combo is one matrix combination.
type Combo struct {
	Name   string `json:"name,omitempty"` // the job's name for it, e.g. "test (ubuntu, 20)"
	Values []KV   `json:"values"`
	// Origin: "" (the product), "include" (extended by an include),
	// "added" (an include that matched nothing becomes its own job).
	Origin string `json:"origin,omitempty"`
}

// KV is a key and a value, in order.
type KV struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Arg is an input passed to a reusable workflow or a template.
type Arg struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	State    string `json:"state"` // passed, default, missing, unknown (not declared)
	Type     string `json:"type,omitempty"`
	Required bool   `json:"required,omitempty"`
}

// Input is a workflow input, GitLab spec input or Azure parameter.
type Input struct {
	Name        string   `json:"name"`
	Type        string   `json:"type,omitempty"`
	Default     string   `json:"default,omitempty"`
	HasDefault  bool     `json:"hasDefault,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Description string   `json:"description,omitempty"`
	Options     []string `json:"options,omitempty"`
	From        string   `json:"from,omitempty"` // workflow_dispatch, workflow_call, secret, spec, parameters
	Source      Source   `json:"source"`
}

// Var is a variable (GitLab, Azure) or env entry (GitHub).
type Var struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Scope  string `json:"scope"` // workflow, job <id>, stage <name>, group
	Source Source `json:"source"`
}

// Include is another file the pipeline pulls in, and whether codec
// could read it.
type Include struct {
	Kind   string `json:"kind"`   // local, project, remote, template, component, workflow, action, template-file
	Target string `json:"target"` // what the file says
	State  string `json:"state"`  // resolved, unresolved (not in this folder), missing
	File   string `json:"file,omitempty"`
	Note   string `json:"note,omitempty"`
	Source Source `json:"source"`
}

// Template is a GitLab hidden job, usable with extends and !reference.
type Template struct {
	Name   string `json:"name"`
	Source Source `json:"source"`
}

// Edge is "From runs before To".
type Edge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label,omitempty"` // needs, optional, stage
}

// Line is one line of an effective configuration.
type Line struct {
	Text   string  `json:"text"`
	From   string  `json:"from,omitempty"` // e.g. "extends .base", "default", "template build.yml"
	Source *Source `json:"source,omitempty"`
}

// Problem is something wrong or worth knowing.
type Problem struct {
	Severity yamlkit.Severity `json:"severity"`
	Code     string           `json:"code"`
	Message  string           `json:"message"`
	Hint     string           `json:"hint,omitempty"`
	Job      string           `json:"job,omitempty"`
	Source   *Source          `json:"source,omitempty"`
}

// Loader reads a workspace file by slash path. Missing files return an
// error that wraps fs.ErrNotExist.
type Loader func(path string) ([]byte, error)

// Options tell Analyze how to reach other files.
type Options struct {
	// Root is the repository root as a workspace path ("" = the
	// workspace root). Repository-relative paths (GitHub uses: ./…,
	// GitLab include: local, Azure /templates) start here.
	Root string
	Load Loader
	// Files lists workspace paths, for include patterns with globs.
	Files []string
	// Params are what-if values for workflow inputs, GitLab spec inputs
	// and Azure parameters.
	Params map[string]string
}

// Analyze reads the pipeline in f (parsed from content) as tool.
func Analyze(tool, file string, content []byte, f *yamlkit.File, opts Options) *Pipeline {
	p := &Pipeline{Tool: tool, File: file, Kind: "workflow"}
	if f != nil {
		switch tool {
		case GitHub:
			readGitHub(p, content, f, opts)
		case GitLab:
			readGitLab(p, content, f, opts)
		case Azure:
			readAzure(p, content, f, opts)
		}
	}
	p.normalize()
	return p
}

// normalize makes every list non-nil (JSON clients get [], not null)
// and sorts problems by position.
func (p *Pipeline) normalize() {
	p.Triggers = orEmpty(p.Triggers)
	p.Inputs = orEmpty(p.Inputs)
	p.Variables = orEmpty(p.Variables)
	p.Stages = orEmpty(p.Stages)
	p.Jobs = orEmpty(p.Jobs)
	p.Edges = orEmpty(p.Edges)
	p.Includes = orEmpty(p.Includes)
	p.Templates = orEmpty(p.Templates)
	p.Problems = orEmpty(p.Problems)
	for i := range p.Stages {
		p.Stages[i].Jobs = orEmpty(p.Stages[i].Jobs)
	}
	for _, j := range p.Jobs {
		j.Needs = orEmpty(j.Needs)
		j.Steps = orEmpty(j.Steps)
		j.Effective = orEmpty(j.Effective)
	}
	slices.SortStableFunc(p.Problems, func(a, b Problem) int {
		return cmp.Compare(line(a.Source), line(b.Source))
	})
}

func line(s *Source) int {
	if s == nil {
		return 0
	}
	return s.Line
}

func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// Job returns the job with id, or nil.
func (p *Pipeline) Job(id string) *Job {
	for _, j := range p.Jobs {
		if j.ID == id {
			return j
		}
	}
	return nil
}

func (p *Pipeline) problem(sev yamlkit.Severity, code, job string, src *Source, msg, hint string) {
	if sev == "" { // not decidable without other files
		return
	}
	p.Problems = append(p.Problems, Problem{Severity: sev, Code: code, Message: msg, Hint: hint, Job: job, Source: src})
}

// --- helpers shared by the readers ---

// src is where node n (in file) is written.
func src(file string, n *yamlkit.Node) Source {
	if n == nil {
		return Source{File: file}
	}
	return Source{File: file, Line: n.Range.Start.Line, Range: n.Range}
}

func srcp(file string, n *yamlkit.Node) *Source {
	s := src(file, n)
	return &s
}

// keySrc is where the key of entry k in map m is written, falling back
// to the map itself.
func keySrc(file string, m *yamlkit.Node, k string) *Source {
	if pr := m.Pair(k); pr != nil {
		return srcp(file, pr.Key)
	}
	return srcp(file, m)
}

// items returns a sequence's items, or nil.
func items(n *yamlkit.Node) []*yamlkit.Node {
	if n == nil || n.Kind != yamlkit.KindSeq {
		return nil
	}
	return n.Items
}

// strs reads a scalar or a list of scalars.
func strs(n *yamlkit.Node) []string {
	if n == nil {
		return nil
	}
	if n.Kind == yamlkit.KindScalar {
		if n.Tag == yamlkit.TagNull {
			return nil
		}
		return []string{n.Value}
	}
	var out []string
	for _, it := range items(n) {
		if it.Kind == yamlkit.KindScalar {
			out = append(out, it.Value)
		}
	}
	return out
}

// text shows any value on one line: a scalar as is, anything else as
// compact YAML.
func text(n *yamlkit.Node) string {
	if n == nil {
		return ""
	}
	if n.Kind == yamlkit.KindScalar {
		if n.Tag == yamlkit.TagNull {
			return ""
		}
		return n.Value
	}
	return strings.ReplaceAll(yamlkit.ValueText(n), "\n", " ")
}

// firstLine is the first non-empty line of s, marked when more follow.
func firstLine(s string) string {
	s = strings.TrimSpace(s)
	first, rest, more := strings.Cut(s, "\n")
	first = strings.TrimSpace(first)
	if more && strings.TrimSpace(rest) != "" {
		first += " …"
	}
	return first
}

// root is the first document root of f that is a map, or nil.
func root(f *yamlkit.File) *yamlkit.Node {
	for _, d := range f.Docs {
		if d.Root != nil && d.Root.Kind == yamlkit.KindMap {
			return d.Root
		}
	}
	return nil
}

// resolved returns n with anchors and merge keys applied; n itself if
// that fails (the parser reports the problem).
func resolved(n *yamlkit.Node) *yamlkit.Node {
	if n == nil {
		return nil
	}
	r, err := yamlkit.Resolve(n)
	if err != nil {
		return n
	}
	return r
}

// load reads and parses a workspace file through opts.Load.
func (o Options) load(p string) ([]byte, *yamlkit.File, error) {
	if o.Load == nil {
		return nil, nil, fs.ErrNotExist
	}
	data, err := o.Load(p)
	if err != nil {
		return nil, nil, err
	}
	return data, yamlkit.Parse(data), nil
}

// repoPath joins a repository-relative path onto the root.
func (o Options) repoPath(p string) string {
	return strings.TrimPrefix(path.Join(o.Root, strings.TrimPrefix(p, "/")), "/")
}

// missing reports whether err means "the file isn't there".
func missing(err error) bool { return errors.Is(err, fs.ErrNotExist) }

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// closest suggests the candidate nearest to s ("did you mean"), or "":
// at most 3 edits away, and fewer for short names, so long names that
// merely share words aren't offered.
func closest(s string, candidates []string) string {
	best, bestD := "", min(3, len(s)/3)+1
	for _, c := range candidates {
		if c == s {
			continue
		}
		if d := distance(strings.ToLower(s), strings.ToLower(c)); d < bestD {
			best, bestD = c, d
		}
	}
	return best
}

// distance counts the edits between a and b: inserts, deletes, changes
// and swaps of neighbours (a common typo), each one edit.
func distance(a, b string) int {
	d := make([][]int, len(a)+1)
	for i := range d {
		d[i] = make([]int, len(b)+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			c := 1
			if a[i-1] == b[j-1] {
				c = 0
			}
			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+c)
			if i > 1 && j > 1 && a[i-1] == b[j-2] && a[i-2] == b[j-1] {
				d[i][j] = min(d[i][j], d[i-2][j-2]+1)
			}
		}
	}
	return d[len(a)][len(b)]
}

func didYouMean(s string, candidates []string) string {
	if c := closest(s, candidates); c != "" {
		return fmt.Sprintf("Did you mean %q?", c)
	}
	return ""
}

// pairs returns a map's entries, or nil.
func pairs(n *yamlkit.Node) []*yamlkit.Pair {
	if n == nil || n.Kind != yamlkit.KindMap {
		return nil
	}
	return n.Pairs
}
