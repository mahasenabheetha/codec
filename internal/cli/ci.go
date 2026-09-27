package cli

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/ci"
	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

var (
	ciRoot   string
	ciParams []string
	ciJSON   bool
	ciFormat string
)

var ciCmd = &cobra.Command{
	Use:   "ci",
	Short: "Understand CI pipelines: GitHub Actions, GitLab CI, Azure Pipelines",
}

var ciJobsCmd = &cobra.Command{
	Use:   "jobs <pipeline file>",
	Short: "List a pipeline's stages and jobs in execution order",
	Long: `jobs reads a GitHub Actions workflow, a .gitlab-ci.yml or an Azure
pipeline and lists its jobs by stage, with what each waits for — after
GitLab includes and extends, GitHub reusable workflows and matrices, and
Azure templates are applied. Files it pulls in are read from the
repository (--root, by default the folder holding .git); remote ones
are listed as not read.

Exit status: 0 fine, 1 error, 2 problems (unknown jobs, missing files…).`,
	Example: `  codec ci jobs .gitlab-ci.yml
  codec ci jobs .github/workflows/ci.yml --json
  codec ci jobs azure-pipelines.yml -p env=prod`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := ciAnalyze(args[0])
		if err != nil {
			return err
		}
		if ciJSON {
			if err := emitJSON(p); err != nil {
				return err
			}
		} else {
			printPipeline(os.Stdout, p)
		}
		return ciProblems(p)
	},
}

var ciJobCmd = &cobra.Command{
	Use:   "job <pipeline file> <job>",
	Short: "Show one job's effective configuration and where each line comes from",
	Long: `job prints a job's configuration as the CI system sees it: for GitLab
after includes, !reference, extends, default: and global variables; for
Azure after templates and template expressions; for GitHub the job as
written, with its matrix expanded. Each line says where it came from.`,
	Example: "  codec ci job .gitlab-ci.yml build\n  codec ci job .github/workflows/ci.yml test",
	Args:    cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := ciAnalyze(args[0])
		if err != nil {
			return err
		}
		j := p.Job(args[1])
		if j == nil {
			for _, x := range p.Jobs {
				if x.Name == args[1] {
					j = x
				}
			}
		}
		if j == nil {
			var ids []string
			for _, x := range p.Jobs {
				ids = append(ids, x.ID)
			}
			return fmt.Errorf("no job %q (jobs: %s)", args[1], strings.Join(ids, ", "))
		}
		if ciJSON {
			return emitJSON(j)
		}
		printJob(os.Stdout, p, j)
		return nil
	},
}

var ciGraphCmd = &cobra.Command{
	Use:     "graph <pipeline file>",
	Short:   "Draw a pipeline's jobs in execution order as Mermaid or Graphviz DOT",
	Example: "  codec ci graph .gitlab-ci.yml\n  codec ci graph .github/workflows/ci.yml --format dot | dot -Tsvg > ci.svg",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := ciAnalyze(args[0])
		if err != nil {
			return err
		}
		switch ciFormat {
		case "mermaid":
			fmt.Print(ci.Mermaid(p))
		case "dot":
			fmt.Print(ci.Dot(p))
		default:
			return fmt.Errorf("unknown format %q (use mermaid or dot)", ciFormat)
		}
		return nil
	},
}

func init() {
	for _, c := range []*cobra.Command{ciJobsCmd, ciJobCmd, ciGraphCmd} {
		c.Flags().StringVar(&ciRoot, "root", "", "repository root for includes and templates (default: the folder holding .git)")
		c.Flags().StringArrayVarP(&ciParams, "input", "p", nil, "set a workflow input or template parameter, name=value (repeatable)")
	}
	ciJobsCmd.Flags().BoolVar(&ciJSON, "json", false, "print JSON")
	ciJobCmd.Flags().BoolVar(&ciJSON, "json", false, "print JSON")
	ciGraphCmd.Flags().StringVar(&ciFormat, "format", "mermaid", "mermaid or dot")
	ciCmd.AddCommand(ciJobsCmd, ciJobCmd, ciGraphCmd)
	rootCmd.AddCommand(ciCmd)
}

// ciAnalyze reads a pipeline file, with the repository around it.
func ciAnalyze(file string) (*ci.Pipeline, error) {
	abs, err := filepath.Abs(file)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	root := ciRoot
	if root == "" {
		root = repoRoot(abs)
	}
	if root, err = filepath.Abs(root); err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return nil, fmt.Errorf("%s is outside the repository root %s (set --root)", file, root)
	}
	rel = filepath.ToSlash(rel)
	f := yamlkit.Parse(data)
	tool := provider.Default.Best(&provider.File{Path: rel, YAML: f, Content: data}).ID()
	if tool != ci.GitHub && tool != ci.GitLab && tool != ci.Azure {
		return nil, fmt.Errorf("%s doesn't look like a GitHub Actions, GitLab CI or Azure Pipelines file (it reads as %s)", file, tool)
	}
	params := map[string]string{}
	for _, p := range ciParams {
		name, value, ok := strings.Cut(p, "=")
		if !ok {
			return nil, fmt.Errorf("-p %q: expected name=value", p)
		}
		params[name] = value
	}
	opts := ci.Options{
		Params: params,
		Load: func(p string) ([]byte, error) {
			return os.ReadFile(filepath.Join(root, filepath.FromSlash(p)))
		},
	}
	if tool == ci.GitLab {
		opts.Files = listFiles(root)
	}
	return ci.Analyze(tool, rel, data, f, opts), nil
}

// repoRoot is the nearest folder above file holding .git, or the
// current folder.
func repoRoot(file string) string {
	for dir := filepath.Dir(file); ; {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return filepath.Dir(file)
}

// listFiles lists the repository's files (for include globs), skipping
// .git and dependency folders.
func listFiles(root string) []string {
	var out []string
	filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if rel, err := filepath.Rel(root, p); err == nil && len(out) < 50000 {
			out = append(out, filepath.ToSlash(rel))
		}
		return nil
	})
	return out
}

var toolTitle = map[string]string{ci.GitHub: "GitHub Actions", ci.GitLab: "GitLab CI", ci.Azure: "Azure Pipelines"}

func printPipeline(w io.Writer, p *ci.Pipeline) {
	head := toolTitle[p.Tool] + " · " + p.File
	if p.Name != "" {
		head += " · " + p.Name
	}
	if p.Kind != "workflow" {
		head += " (" + p.Kind + ")"
	}
	fmt.Fprintln(w, head)
	for _, t := range p.Triggers {
		fmt.Fprintln(w, "  on "+t)
	}
	if len(p.Inputs) > 0 {
		fmt.Fprintln(w, "\ninputs")
		for _, in := range p.Inputs {
			line := "  " + in.Name + " (" + in.Type
			if in.Required {
				line += ", required"
			}
			line += ")"
			if in.HasDefault {
				line += " = " + in.Default
			}
			if len(in.Options) > 0 {
				line += "   one of " + strings.Join(in.Options, " | ")
			}
			fmt.Fprintln(w, line)
		}
	}
	if len(p.Includes) > 0 {
		fmt.Fprintln(w, "\nincludes")
		for _, i := range p.Includes {
			mark := map[string]string{"resolved": "read", "unresolved": "not read", "missing": "MISSING"}[i.State]
			line := fmt.Sprintf("  %-9s %s  (%s", i.Kind, i.Target, mark)
			if i.Note != "" {
				line += ": " + i.Note
			}
			fmt.Fprintln(w, line+")")
		}
	}
	after := map[string][]string{}
	for _, e := range p.Edges {
		from := e.From
		if e.Label == "optional" {
			from += " (optional)"
		}
		after[e.To] = append(after[e.To], from)
	}
	printed := map[string]bool{}
	job := func(id, indent string) {
		j := p.Job(id)
		if j == nil {
			return
		}
		printed[id] = true
		line := indent + j.ID
		if j.Name != j.ID && !strings.HasSuffix(j.ID, "."+j.Name) {
			line += " \"" + j.Name + "\""
		}
		if c := j.Caption(); c != "" {
			line += "  · " + c
		}
		fmt.Fprintln(w, line)
		switch {
		case len(after[id]) > 0:
			fmt.Fprintf(w, "%s    after %s\n", indent, strings.Join(after[id], ", "))
		case j.Needs != nil && len(j.Needs) == 0 && p.Tool == ci.GitLab:
			fmt.Fprintf(w, "%s    needs nothing: starts at once\n", indent)
		}
		if len(j.Extends) > 0 {
			fmt.Fprintf(w, "%s    extends %s\n", indent, strings.Join(j.Extends, " → "))
		}
		if j.From != "" {
			fmt.Fprintf(w, "%s    from %s\n", indent, j.From)
		}
		for _, r := range j.Rules {
			fmt.Fprintf(w, "%s    rule %s\n", indent, r)
		}
		if j.If != "" {
			fmt.Fprintf(w, "%s    if %s\n", indent, oneLine(j.If))
		}
	}
	for _, s := range p.Stages {
		if s.Name == "" {
			continue
		}
		title := "stage " + s.Name
		if !s.Defined {
			title += " (not in stages)"
		}
		fmt.Fprintln(w, "\n"+title)
		for _, id := range s.Jobs {
			job(id, "  ")
		}
	}
	var rest []string
	for _, j := range p.Jobs {
		if !printed[j.ID] {
			rest = append(rest, j.ID)
		}
	}
	if len(rest) > 0 {
		fmt.Fprintln(w, "\njobs")
		for _, id := range rest {
			job(id, "  ")
		}
	}
}

func printJob(w io.Writer, p *ci.Pipeline, j *ci.Job) {
	fmt.Fprintf(w, "# %s  %s:%d\n", j.Name, j.Source.File, j.Source.Line)
	if len(j.Extends) > 0 {
		fmt.Fprintf(w, "# extends %s\n", strings.Join(j.Extends, " → "))
	}
	width := 0
	for _, l := range j.Effective {
		width = max(width, len([]rune(l.Text)))
	}
	width = min(width, 72)
	for _, l := range j.Effective {
		note := l.From
		if l.Source != nil && l.Source.File != "" && l.Source.File != j.Source.File {
			note = strings.TrimSpace(note + "  " + fmt.Sprintf("%s:%d", l.Source.File, l.Source.Line))
		}
		if note == "" {
			fmt.Fprintln(w, l.Text)
			continue
		}
		fmt.Fprintf(w, "%-*s  # ← %s\n", width, l.Text, note)
	}
	if j.Matrix != nil {
		fmt.Fprintln(w, "\n# matrix")
		if j.Matrix.Runtime != "" {
			fmt.Fprintln(w, "#   known at run time: "+j.Matrix.Runtime)
		}
		for _, c := range j.Matrix.Combos {
			line := "#   " + c.Name
			if c.Origin != "" {
				line += "   (" + map[string]string{"include": "extended by include", "added": "added by include"}[c.Origin] + ")"
			}
			fmt.Fprintln(w, line)
		}
	}
	if len(j.Instances) > 0 {
		fmt.Fprintln(w, "\n# runs as")
		for _, i := range j.Instances {
			fmt.Fprintln(w, "#   "+i)
		}
	}
}

func ciProblems(p *ci.Pipeline) error {
	bad := 0
	for _, pr := range p.Problems {
		where := ""
		if pr.Source != nil {
			where = fmt.Sprintf("%s:%d: ", orFile(pr.Source.File, p.File), pr.Source.Line)
		}
		fmt.Fprintf(os.Stderr, "%s%s: %s\n", where, pr.Severity, pr.Message)
		if pr.Hint != "" {
			fmt.Fprintf(os.Stderr, "    %s\n", pr.Hint)
		}
		if pr.Severity != yamlkit.SeverityInfo {
			bad++
		}
	}
	if bad > 0 {
		return exitCode(2)
	}
	return nil
}

func orFile(f, def string) string {
	if f == "" {
		return def
	}
	return f
}
