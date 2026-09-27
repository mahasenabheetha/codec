package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/argo"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

var (
	argoWorkflow  string
	argoParams    []string
	argoParamFile string
	argoJSON      bool
	argoTemplate  string
	argoFormat    string
)

var argoCmd = &cobra.Command{
	Use:   "argo",
	Short: "Understand Argo workflows: resolved parameters, stitched templates, graphs",
}

var argoResolveCmd = &cobra.Command{
	Use:   "resolve [file|folder…]",
	Short: "Show every step of a workflow with the template it runs and the values it gets",
	Long: `resolve walks a workflow from its entrypoint, following local
templates and templateRef across the given files and folders (or stdin),
and shows each step's inputs with the values they resolve to. Values
only known while the workflow runs (step outputs, workflow.uid, event
data) are shown as written and marked "run time" — never guessed.

The workflow is the only one in the first file, or the one named by
--workflow. Raw Helm templates work too; render the chart first
(helm template … | codec argo resolve) to fill in its values.

Exit status: 0 fine, 1 error, 2 problems (missing inputs or templates).`,
	Example: `  codec argo resolve wf.yaml templates/ -p env=prod
  codec argo resolve chart/templates --workflow deploy-wf
  helm template ./chart | codec argo resolve --workflow deploy-wf`,
	RunE: func(cmd *cobra.Command, args []string) error {
		res, err := argoResolve(cmd.Context(), args, false)
		if err != nil {
			return err
		}
		if argoJSON {
			if err := emitJSON(res); err != nil {
				return err
			}
		} else {
			printResolved(os.Stdout, res)
		}
		return argoProblems(res)
	},
}

var argoGraphCmd = &cobra.Command{
	Use:   "graph [file|folder…]",
	Short: "Draw a workflow's steps or DAG as Mermaid or Graphviz DOT",
	Long: `graph prints the steps or DAG tasks of one template — the entrypoint,
or --template — as a Mermaid flowchart or Graphviz DOT. Edges come from
step groups, dependencies and depends expressions (labelled with the
result they wait for, e.g. Failed).`,
	Example: "  codec argo graph wf.yaml --template build-dag\n  codec argo graph wf.yaml --format dot | dot -Tsvg > wf.svg",
	RunE: func(cmd *cobra.Command, args []string) error {
		res, err := argoResolve(cmd.Context(), args, false)
		if err != nil {
			return err
		}
		c := res.Container(argoTemplate)
		if c == nil {
			return fmt.Errorf("no template %q with steps or tasks", argoTemplate)
		}
		if len(c.Children) == 0 {
			return fmt.Errorf("template %s is a %s template: it has no steps or tasks to draw", c.Template, c.Type)
		}
		switch argoFormat {
		case "mermaid":
			fmt.Print(argo.Mermaid(c, res))
		case "dot":
			fmt.Print(argo.Dot(c, res))
		default:
			return fmt.Errorf("unknown format %q (use mermaid or dot)", argoFormat)
		}
		return nil
	},
}

func init() {
	for _, c := range []*cobra.Command{argoResolveCmd, argoGraphCmd} {
		c.Flags().StringVarP(&argoWorkflow, "workflow", "w", "", "the workflow to show, by name (or Kind/name)")
		c.Flags().StringArrayVarP(&argoParams, "parameter", "p", nil, "set a workflow parameter, name=value (repeatable)")
		c.Flags().StringVar(&argoParamFile, "parameter-file", "", "YAML or JSON file of parameters, like argo submit --parameter-file")
	}
	argoResolveCmd.Flags().BoolVar(&argoJSON, "json", false, "print JSON")
	argoGraphCmd.Flags().StringVarP(&argoTemplate, "template", "t", "", "the steps or DAG template to draw (default: the entrypoint)")
	argoGraphCmd.Flags().StringVar(&argoFormat, "format", "mermaid", "mermaid or dot")
	argoCmd.AddCommand(argoResolveCmd, argoGraphCmd)
	rootCmd.AddCommand(argoCmd)
}

// argoResolve indexes the inputs, picks the workflow and resolves it.
func argoResolve(ctx context.Context, args []string, bodies bool) (*argo.Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ix := argo.NewIndex()
	var first []*argo.Spec
	if len(args) == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, err
		}
		ix.Add("", data, yamlkit.Parse(data))
		first = ix.Specs
	} else {
		for i, a := range args {
			targets, err := yamlTargets(ctx, []string{a})
			if err != nil {
				return nil, err
			}
			before := len(ix.Specs)
			for _, t := range targets {
				data, err := t.read()
				if err != nil {
					return nil, err
				}
				ix.Add(t.show, data, yamlkit.Parse(data))
			}
			if i == 0 {
				first = ix.Specs[before:]
			}
		}
	}
	wf, err := pickWorkflow(ix, first)
	if err != nil {
		return nil, err
	}
	params, err := argoParamValues()
	if err != nil {
		return nil, err
	}
	return argo.Resolve(ix, wf, argo.Options{Params: params, Bodies: bodies}), nil
}

func pickWorkflow(ix *argo.Index, first []*argo.Spec) (*argo.Spec, error) {
	if argoWorkflow != "" {
		if s := ix.Find(argoWorkflow); s != nil {
			return s, nil
		}
		for _, s := range ix.Specs {
			if s.Via != nil && s.Via.Name == argoWorkflow {
				return s, nil
			}
		}
		return nil, fmt.Errorf("no workflow %q (found: %s)", argoWorkflow, specNames(ix.Specs))
	}
	switch len(first) {
	case 0:
		return nil, fmt.Errorf("no Argo workflow found")
	case 1:
		return first[0], nil
	}
	return nil, fmt.Errorf("%d workflows found; pick one with --workflow: %s", len(first), specNames(first))
}

func specNames(ss []*argo.Spec) string {
	var names []string
	for _, s := range ss {
		n := s.Name
		if s.Via != nil {
			n = s.Via.Name + " (Sensor " + s.Via.Sensor + ")"
		}
		names = append(names, n)
	}
	if len(names) > 12 {
		names = append(names[:12], "…")
	}
	return strings.Join(names, ", ")
}

// argoParamValues reads --parameter-file, then -p on top.
func argoParamValues() (map[string]string, error) {
	out := map[string]string{}
	if argoParamFile != "" {
		data, err := os.ReadFile(argoParamFile)
		if err != nil {
			return nil, err
		}
		f := yamlkit.Parse(data)
		if f.HasErrors() || len(f.Docs) == 0 || f.Docs[0].Root == nil || f.Docs[0].Root.Kind != yamlkit.KindMap {
			return nil, fmt.Errorf("%s: expected a map of name: value", argoParamFile)
		}
		for _, p := range f.Docs[0].Root.Pairs {
			out[p.Key.Value] = yamlkit.ValueText(p.Value)
		}
	}
	for _, p := range argoParams {
		name, value, ok := strings.Cut(p, "=")
		if !ok {
			return nil, fmt.Errorf("-p %q: expected name=value", p)
		}
		out[name] = value
	}
	return out, nil
}

func argoProblems(res *argo.Result) error {
	bad := 0
	for _, p := range res.Problems {
		where := ""
		if p.Source != nil {
			where = fmt.Sprintf("%s:%d: ", orStdin(p.Source.File), p.Source.Line)
		}
		fmt.Fprintf(os.Stderr, "%s%s: %s\n", where, p.Severity, p.Message)
		if p.Hint != "" {
			fmt.Fprintf(os.Stderr, "    %s\n", p.Hint)
		}
		if p.Severity != yamlkit.SeverityInfo {
			bad++
		}
	}
	if bad > 0 {
		return exitCode(2)
	}
	return nil
}

func orStdin(f string) string {
	if f == "" {
		return "<stdin>"
	}
	return f
}

// printResolved prints the workflow as an indented tree.
func printResolved(w io.Writer, res *argo.Result) {
	wf := res.Workflow
	head := wf.Kind + " " + wf.Name
	if wf.Via != nil {
		head = "Workflow submitted by Sensor " + wf.Via.Sensor + " (trigger " + wf.Via.Name + ")"
	}
	fmt.Fprintf(w, "%s  %s:%d\n", head, orStdin(wf.Source.File), wf.Source.Line)
	if res.Base != nil {
		fmt.Fprintf(w, "runs WorkflowTemplate %s  %s:%d\n", res.Base.Name, orStdin(res.Base.Source.File), res.Base.Source.Line)
	}
	if len(res.Params) > 0 {
		fmt.Fprintln(w, "\nparameters")
		printValues(w, res.Params, "  ")
	}
	for _, id := range res.Roots {
		fmt.Fprintln(w)
		printNode(w, res, res.Node(id), "")
	}
}

func printNode(w io.Writer, res *argo.Result, n *argo.Node, indent string) {
	line := indent + n.Name
	if cap := n.Caption(); cap != "" {
		line += " → " + cap
	}
	if n.Image != "" {
		line += " · " + n.Image
	}
	var notes []string
	if n.When != nil {
		cond := "when " + oneLine(n.When.Value.Text)
		switch n.When.Result {
		case "true", "false":
			cond += " → " + n.When.Result
		default:
			cond += " (known at run time)"
		}
		notes = append(notes, cond)
	}
	for _, s := range []string{n.Depends, n.ContinueOn, n.Error, n.Note} {
		if s != "" {
			notes = append(notes, s)
		}
	}
	if n.Loop != "" && (n.Item == nil || strings.HasSuffix(n.Name, "(0:"+n.Item.Text+")") || strings.HasSuffix(n.Name, "(…)")) {
		notes = append(notes, n.Loop)
	}
	fmt.Fprintln(w, line)
	for _, s := range notes {
		fmt.Fprintf(w, "%s    · %s\n", indent, s)
	}
	printValues(w, n.Inputs, indent+"    ")
	for gi, id := range n.Children {
		c := res.Node(id)
		prefix := indent + "  "
		if n.Type == "steps" && c.Group > 0 && (gi == 0 || res.Node(n.Children[gi-1]).Group != c.Group) {
			fmt.Fprintf(w, "%s[%d]\n", prefix, c.Group)
		}
		printNode(w, res, c, prefix+"  ")
	}
}

func printValues(w io.Writer, vs []argo.Value, indent string) {
	width := 0
	for _, v := range vs {
		width = max(width, len(v.Name))
	}
	for _, v := range vs {
		text := oneLine(v.Text)
		switch v.State {
		case argo.StateRuntime:
			text += "   (run time)"
		case argo.StateMissing:
			text = "(missing: " + v.From + ")"
		case argo.StateTemplate:
			text += "   (render the chart for this value)"
		}
		if v.Hint != "" {
			text += "   " + v.Hint
		}
		fmt.Fprintf(w, "%s%-*s = %s\n", indent, width, v.Name, text)
	}
}

// oneLine shortens a value for the tree: its first line, clipped.
func oneLine(s string) string {
	first, rest, multi := strings.Cut(strings.TrimRight(s, "\n"), "\n")
	if r := []rune(first); len(r) > 72 {
		first, multi = string(r[:71]), true
	}
	if multi || rest != "" {
		first += "…"
	}
	if first == "" {
		return `""`
	}
	return first
}
