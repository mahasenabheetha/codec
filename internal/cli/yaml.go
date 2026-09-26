package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Flags for the yaml subcommands.
var (
	yamlJSON       bool
	fmtIndent      int
	fmtSortKeys    bool
	fmtK8sOrder    bool
	fmtResolve     bool
	convertTo      string
	convertCompact bool
	flatReverse    bool
	pathLine       int
	pathCol        int
	pathGet        string
	pathDoc        int
	pathStyle      string
)

var yamlCmd = &cobra.Command{
	Use:   "yaml",
	Short: "Understand, reformat and convert YAML files",
	Long: `yaml works on one YAML file (or stdin): identify what kind of file it
is, outline it, find the path at a position, reformat or convert it.

Template files (Helm, Jinja, GitHub Actions expressions) are understood
too: expressions are recognised instead of reported as syntax errors.
Output goes to stdout; codec never modifies the file.`,
}

var yamlIdentifyCmd = &cobra.Command{
	Use:   "identify [file]",
	Short: "Detect the file type, list documents and report problems",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		src, name, err := readSource(args)
		if err != nil {
			return err
		}
		f := yamlkit.Parse(src)
		matches := provider.Default.Detect(&provider.File{Path: filepath.ToSlash(name), YAML: f})

		if yamlJSON {
			type match struct {
				ID         string `json:"id"`
				Title      string `json:"title"`
				Confidence int    `json:"confidence"`
			}
			out := struct {
				Matches     []match              `json:"matches"`
				Documents   []*yamlkit.Document  `json:"documents"`
				Expressions []yamlkit.Expression `json:"expressions"`
				Diagnostics []yamlkit.Diagnostic `json:"diagnostics"`
			}{Documents: f.Docs, Expressions: f.Expressions, Diagnostics: f.Diagnostics}
			for _, m := range matches {
				out.Matches = append(out.Matches, match{m.Provider.ID(), m.Provider.Title(), int(m.Confidence)})
			}
			// Documents carry full trees; the summary only needs their names.
			for _, d := range out.Documents {
				d.Root = nil
			}
			return emitJSON(out)
		}

		var b strings.Builder
		best := matches[0]
		fmt.Fprintf(&b, "type:        %s (%s, confidence %d)\n", best.Provider.Title(), best.Provider.ID(), best.Confidence)
		for _, m := range matches[1:] {
			if m.Confidence > 1 {
				fmt.Fprintf(&b, "also:        %s (%d)\n", m.Provider.Title(), m.Confidence)
			}
		}
		fmt.Fprintf(&b, "documents:   %d\n", len(f.Docs))
		for _, d := range f.Docs {
			label := d.Name
			if label == "" {
				label = "(unnamed)"
			}
			fmt.Fprintf(&b, "  %-3d %-40s lines %d-%d\n", d.Index+1, label, d.Range.Start.Line, d.Range.End.Line)
		}
		if n := len(f.Expressions); n > 0 {
			fmt.Fprintf(&b, "expressions: %d (%s)\n", n, expressionSummary(f.Expressions))
		}
		fmt.Fprintf(&b, "problems:    %d", len(f.Diagnostics))
		printDiagnostics(cmd.ErrOrStderr(), name, f.Diagnostics)
		return emit(b.String())
	},
}

var yamlOutlineCmd = &cobra.Command{
	Use:   "outline [file]",
	Short: "Print the key/item tree with line numbers",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		src, name, err := readSource(args)
		if err != nil {
			return err
		}
		f := yamlkit.Parse(src)
		p := provider.Default.Best(&provider.File{Path: filepath.ToSlash(name), YAML: f})
		printDiagnostics(cmd.ErrOrStderr(), name, f.Diagnostics)

		if yamlJSON {
			type docOutline struct {
				Name    string            `json:"name,omitempty"`
				Symbols []provider.Symbol `json:"symbols"`
			}
			var out []docOutline
			for _, d := range f.Docs {
				out = append(out, docOutline{d.Name, p.Symbols(d)})
			}
			return emitJSON(out)
		}

		var b strings.Builder
		for _, d := range f.Docs {
			if len(f.Docs) > 1 {
				label := d.Name
				if label == "" {
					label = fmt.Sprintf("document %d", d.Index+1)
				}
				fmt.Fprintf(&b, "# %s\n", label)
			}
			writeSymbols(&b, p.Symbols(d), 0)
		}
		return emit(strings.TrimRight(b.String(), "\n"))
	},
}

var yamlPathCmd = &cobra.Command{
	Use:   "path <file> (--line N [--col N] | --get PATH)",
	Short: "Show the path at a position, or the value at a path",
	Long: `path answers "where am I?" and "what's there?":

  codec yaml path deploy.yaml --line 12 --col 9   # → spec.containers[0].image
  codec yaml path deploy.yaml --line 12 --style all
  codec yaml path values.yaml --get image.tag     # → value and position

Styles: dot, yq, jsonpath, helm (.Values…), set (helm --set), all.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		src, _, err := readSource(args)
		if err != nil {
			return err
		}
		f := yamlkit.Parse(src)
		if len(f.Docs) == 0 {
			return errors.New("the file has no YAML documents")
		}

		if pathGet != "" {
			if pathDoc < 1 || pathDoc > len(f.Docs) {
				return fmt.Errorf("--doc must be between 1 and %d", len(f.Docs))
			}
			p, err := yamlkit.ParsePath(pathGet)
			if err != nil {
				return err
			}
			n := f.Docs[pathDoc-1].NodeAt(p)
			if n == nil {
				return fmt.Errorf("nothing at %s", p)
			}
			value := n.Value
			if n.Kind == yamlkit.KindMap || n.Kind == yamlkit.KindSeq {
				value = fmt.Sprintf("(%s)", n.Kind)
			}
			return emit(fmt.Sprintf("line %d, col %d: %s", n.Range.Start.Line, n.Range.Start.Col, value))
		}

		if pathLine < 1 {
			return errors.New("give --line (and optionally --col), or --get PATH")
		}
		pos := yamlkit.Pos{Line: pathLine, Col: max(pathCol, 1)}
		doc := docAtLine(f, pathLine)
		p, _ := doc.PathAt(pos)
		if len(p) == 0 {
			return fmt.Errorf("no key or item at line %d", pathLine)
		}
		if pathStyle == "all" {
			var b strings.Builder
			for _, s := range yamlkit.PathStyles {
				fmt.Fprintf(&b, "%-9s %s\n", s, p.Format(s))
			}
			return emit(strings.TrimRight(b.String(), "\n"))
		}
		return emit(p.Format(yamlkit.PathStyle(pathStyle)))
	},
}

var yamlFmtCmd = &cobra.Command{
	Use:   "fmt [file]",
	Short: "Re-indent YAML consistently, keeping comments (prints to stdout)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		src, _, err := readSource(args)
		if err != nil {
			return err
		}
		var out []byte
		if fmtResolve {
			out, err = yamlkit.ResolvedYAML(yamlkit.Parse(src))
		} else {
			out, err = yamlkit.Format(src, yamlkit.FormatOptions{
				Indent: fmtIndent, SortKeys: fmtSortKeys, KubernetesOrder: fmtK8sOrder,
			})
		}
		if err != nil {
			return withHint(err)
		}
		return emit(strings.TrimRight(string(out), "\r\n"))
	},
}

var yamlConvertCmd = &cobra.Command{
	Use:   "convert [file] --to json|yaml",
	Short: "Convert YAML to JSON or JSON to YAML, keeping key order",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		src, _, err := readSource(args)
		if err != nil {
			return err
		}
		var out []byte
		switch convertTo {
		case "json":
			indent := "  "
			if convertCompact {
				indent = ""
			}
			out, err = yamlkit.ToJSON(yamlkit.Parse(src), indent)
		case "yaml":
			out, err = yamlkit.FromJSON(src)
		default:
			return fmt.Errorf("--to must be json or yaml, got %q", convertTo)
		}
		if err != nil {
			return withHint(err)
		}
		return emit(strings.TrimRight(string(out), "\n"))
	},
}

var yamlFlattenCmd = &cobra.Command{
	Use:   "flatten [file]",
	Short: "List every value as a \"path: value\" line (or --reverse)",
	Long: `flatten prints one "path: value" line per leaf — easy to grep and diff:

  image.repository: nginx
  ports[0]: 80

--reverse turns such lines back into YAML.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		src, _, err := readSource(args)
		if err != nil {
			return err
		}
		var out []byte
		if flatReverse {
			out, err = yamlkit.Unflatten(src)
		} else {
			out, err = yamlkit.FlattenText(yamlkit.Parse(src))
		}
		if err != nil {
			return withHint(err)
		}
		return emit(strings.TrimRight(string(out), "\n"))
	},
}

func init() {
	for _, c := range []*cobra.Command{yamlIdentifyCmd, yamlOutlineCmd} {
		c.Flags().BoolVar(&yamlJSON, "json", false, "print machine-readable JSON")
	}
	yamlFmtCmd.Flags().IntVar(&fmtIndent, "indent", 2, "spaces per indentation level")
	yamlFmtCmd.Flags().BoolVar(&fmtSortKeys, "sort-keys", false, "sort map keys alphabetically")
	yamlFmtCmd.Flags().BoolVar(&fmtK8sOrder, "k8s-order", false, "put apiVersion, kind, metadata, spec first")
	yamlFmtCmd.Flags().BoolVar(&fmtResolve, "resolve", false, "expand anchors/aliases and merge keys (drops comments)")
	yamlConvertCmd.Flags().StringVar(&convertTo, "to", "", "target format: json or yaml (required)")
	yamlConvertCmd.Flags().BoolVar(&convertCompact, "compact", false, "single-line JSON")
	_ = yamlConvertCmd.MarkFlagRequired("to")
	yamlFlattenCmd.Flags().BoolVar(&flatReverse, "reverse", false, "turn path: value lines back into YAML")
	yamlPathCmd.Flags().IntVar(&pathLine, "line", 0, "1-based line number")
	yamlPathCmd.Flags().IntVar(&pathCol, "col", 1, "1-based column")
	yamlPathCmd.Flags().StringVar(&pathGet, "get", "", "print the value at this path instead")
	yamlPathCmd.Flags().IntVar(&pathDoc, "doc", 1, "document number for --get in multi-document files")
	yamlPathCmd.Flags().StringVar(&pathStyle, "style", "dot", "dot, yq, jsonpath, helm, set, or all")

	yamlCmd.AddCommand(yamlIdentifyCmd, yamlOutlineCmd, yamlPathCmd, yamlFmtCmd, yamlConvertCmd, yamlFlattenCmd)
	rootCmd.AddCommand(yamlCmd)
}

// readSource reads the file named by the first argument, or stdin when
// there is none (or it is "-"). Unlike readInput it keeps whitespace:
// in YAML, indentation is meaning.
func readSource(args []string) ([]byte, string, error) {
	if len(args) > 0 && args[0] != "-" {
		b, err := os.ReadFile(args[0])
		return b, args[0], err
	}
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, "", fmt.Errorf("read stdin: %w", err)
	}
	return b, "", nil
}

// printDiagnostics writes problems compiler-style (file:line:col), a
// format terminals and editors turn into clickable links.
func printDiagnostics(w io.Writer, name string, diags []yamlkit.Diagnostic) {
	if name == "" {
		name = "<stdin>"
	}
	for _, d := range diags {
		fmt.Fprintf(w, "%s:%d:%d: %s: %s\n", name, d.Range.Start.Line, d.Range.Start.Col, d.Severity, d.Message)
		if d.Hint != "" {
			fmt.Fprintf(w, "    hint: %s\n", d.Hint)
		}
	}
}

// withHint appends a positioned error's fix-it hint.
func withHint(err error) error {
	var e *yamlkit.Error
	if errors.As(err, &e) && e.Hint != "" {
		return fmt.Errorf("%w\n  hint: %s", err, e.Hint)
	}
	return err
}

func emitJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return emit(string(b))
}

func writeSymbols(b *strings.Builder, syms []provider.Symbol, depth int) {
	for _, s := range syms {
		label := s.Name
		switch {
		case s.Kind == "scalar" || s.Kind == "alias":
			label += ": " + s.Detail
		case s.Detail != "":
			label += " " + s.Detail
		}
		// Empty values (often filled by template logic) end at the colon.
		fmt.Fprintf(b, "%5d  %s%s\n", s.Range.Start.Line, strings.Repeat("  ", depth), strings.TrimRight(label, " "))
		writeSymbols(b, s.Children, depth+1)
	}
}

func expressionSummary(exprs []yamlkit.Expression) string {
	counts := map[string]int{}
	var order []string
	for _, e := range exprs {
		if counts[e.Syntax] == 0 {
			order = append(order, e.Syntax)
		}
		counts[e.Syntax]++
	}
	parts := make([]string, len(order))
	for i, s := range order {
		parts[i] = fmt.Sprintf("%s %d", s, counts[s])
	}
	return strings.Join(parts, ", ")
}

// docAtLine returns the document containing line, or the last one that
// starts before it.
func docAtLine(f *yamlkit.File, line int) *yamlkit.Document {
	doc := f.Docs[0]
	for _, d := range f.Docs {
		if d.Range.Start.Line <= line {
			doc = d
		}
	}
	return doc
}
