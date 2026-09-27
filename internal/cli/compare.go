package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/config"
	"github.com/mahasenabheetha/codec/v2/internal/query"
	"github.com/mahasenabheetha/codec/v2/internal/textdiff"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

var (
	diffIgnore []string
	diffText   bool
	diffJSON   bool

	queryJSON     bool
	queryLocation bool
	queryLimit    int
)

var yamlDiffCmd = &cobra.Command{
	Use:   "diff <a> <b>",
	Short: "Compare two YAML files by meaning (key order and list order don't matter)",
	Long: `diff compares two YAML files semantically: key order is ignored,
documents pair up by kind/namespace/name, and list items (containers,
env, ports, volumes…) by their name. It prints what was added, removed
or changed, by path. Files with syntax errors are compared as text.

Ignore noise with --ignore (repeatable, "*" matches anything), e.g.
--ignore 'metadata.labels.helm.sh/chart'. Patterns saved in the web
UI's compare view apply too.

Exit status: 0 same, 1 error, 2 different.`,
	Example: `  codec yaml diff dev.yaml prod.yaml
  codec yaml diff a.yaml b.yaml --ignore 'spec.template.spec.containers[*].image'`,
	Args: cobra.ExactArgs(2),
	RunE: runDiff,
}

var yamlQueryCmd = &cobra.Command{
	Use:   "query <expr> [file|folder…]",
	Short: "Run a jq expression over YAML files (each document is one input)",
	Long: `query runs a jq expression (gojq) over every document of the given
files and folders (folders honour .gitignore), or over stdin. Results
are printed as YAML; with several files, or --with-location, each is
prefixed with file:line so terminals and editors can jump to it.`,
	Example: `  codec yaml query '.spec.template.spec.containers[].image' deploy.yaml
  codec yaml query 'select(.kind == "Ingress") | .spec.rules[].host' charts/
  codec yaml query '.. | objects | select(has("hostPath"))' . --json`,
	Args: cobra.MinimumNArgs(1),
	RunE: runQuery,
}

func init() {
	yamlDiffCmd.Flags().StringArrayVar(&diffIgnore, "ignore", nil, "path pattern to ignore (repeatable)")
	yamlDiffCmd.Flags().BoolVar(&diffText, "text", false, "compare as text (a unified diff)")
	yamlDiffCmd.Flags().BoolVar(&diffJSON, "json", false, "print changes as JSON")
	yamlQueryCmd.Flags().BoolVar(&queryJSON, "json", false, "print results as JSON with file, line and path")
	yamlQueryCmd.Flags().BoolVar(&queryLocation, "with-location", false, "prefix every result with file:line")
	yamlQueryCmd.Flags().IntVar(&queryLimit, "limit", 1000, "stop after this many results")
	yamlCmd.AddCommand(yamlDiffCmd, yamlQueryCmd)
}

func runDiff(cmd *cobra.Command, args []string) error {
	a, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	b, err := os.ReadFile(args[1])
	if err != nil {
		return err
	}
	ignore := diffIgnore
	if cfg, err := config.Open(); err == nil {
		ignore = append(ignore, cfg.Get().DiffIgnore...)
	}
	if !diffText {
		changes, err := yamlkit.Diff(yamlkit.Parse(a), yamlkit.Parse(b), yamlkit.DiffOptions{Ignore: ignore})
		switch {
		case err == nil:
			if diffJSON {
				if changes == nil {
					changes = []yamlkit.Change{}
				}
				if err := emitJSON(changes); err != nil {
					return err
				}
			} else {
				printChanges(os.Stdout, changes)
			}
			if len(changes) > 0 {
				return exitCode(2)
			}
			return nil
		case errors.Is(err, yamlkit.ErrUnparsed):
			fmt.Fprintln(os.Stderr, "note:", err, "— showing a text diff")
		default:
			return err
		}
	}
	d := textdiff.Unified(args[1], string(a), string(b), 3)
	fmt.Print(d)
	if strings.TrimSpace(d) != "" {
		return exitCode(2)
	}
	return nil
}

// printChanges writes changes grouped by document:
//
//	Deployment api
//	  ~ spec.replicas: 2 → 3
//	  + metadata.annotations: …
func printChanges(w io.Writer, changes []yamlkit.Change) {
	doc := "\x00"
	for _, c := range changes {
		if c.Doc != doc {
			doc = c.Doc
			fmt.Fprintln(w, doc)
		}
		path := c.Path
		if path == "" {
			path = "(whole document)"
		}
		switch c.Kind {
		case yamlkit.Changed:
			if strings.Contains(c.Old+c.New, "\n") {
				fmt.Fprintf(w, "  ~ %s:\n%s\n    →\n%s\n", path, indentBy(c.Old, 6), indentBy(c.New, 6))
			} else {
				fmt.Fprintf(w, "  ~ %s: %s → %s\n", path, c.Old, c.New)
			}
		case yamlkit.Added:
			fmt.Fprintf(w, "  + %s:%s\n", path, inline(c.New))
		case yamlkit.Removed:
			fmt.Fprintf(w, "  - %s:%s\n", path, inline(c.Old))
		case yamlkit.Reordered:
			fmt.Fprintf(w, "  ↕ %s: same items, new order\n", path)
		}
	}
	if len(changes) == 0 {
		fmt.Fprintln(w, "No differences.")
	}
}

func inline(v string) string {
	if strings.Contains(v, "\n") {
		return "\n" + indentBy(v, 6)
	}
	return " " + v
}

func indentBy(s string, n int) string {
	pad := strings.Repeat(" ", n)
	return pad + strings.ReplaceAll(s, "\n", "\n"+pad)
}

type queryResult struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Col  int    `json:"col"`
	query.Result
}

func runQuery(cmd *cobra.Command, args []string) error {
	q, err := query.Compile(args[0])
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	var targets []yamlTarget
	if len(args) == 1 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		targets = []yamlTarget{{show: "<stdin>", read: func() ([]byte, error) { return data, nil }}}
	} else if targets, err = yamlTargets(ctx, args[1:]); err != nil {
		return err
	}
	withLoc := queryLocation || len(targets) > 1 || (len(args) > 2)

	var all []queryResult
	var firstErr error
	for _, t := range targets {
		data, err := t.read()
		if err != nil {
			return err
		}
		f := yamlkit.Parse(data)
		rs, err := q.Run(ctx, f, queryLimit-len(all))
		for _, r := range rs {
			qr := queryResult{File: t.show, Result: r}
			if r.Range != nil {
				qr.Line, qr.Col = r.Range.Start.Line, r.Range.Start.Col
			}
			all = append(all, qr)
		}
		if errors.Is(err, query.ErrLimit) {
			fmt.Fprintf(os.Stderr, "note: stopped after %d results (--limit)\n", queryLimit)
			break
		}
		if err != nil && firstErr == nil && len(targets) == 1 {
			firstErr = err
		}
	}
	if firstErr != nil && len(all) == 0 {
		return firstErr
	}
	if queryJSON {
		if all == nil {
			all = []queryResult{}
		}
		return emitJSON(all)
	}
	for _, r := range all {
		switch {
		case !withLoc:
			fmt.Println(r.Value)
		case strings.Contains(r.Value, "\n"):
			fmt.Printf("%s:%d:\n%s\n", r.File, r.Line, indentBy(r.Value, 2))
		default:
			fmt.Printf("%s:%d: %s\n", r.File, r.Line, r.Value)
		}
	}
	return nil
}
