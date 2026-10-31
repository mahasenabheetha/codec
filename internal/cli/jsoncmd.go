package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/jsonkit"
)

// The JSON & YAML tool on the command line (3.7.0): files or stdin,
// read as JSON, JSONC or JSON5, written as strict JSON.

var (
	jsonSort      bool
	jsonOut       bool // --json: machine-readable output
	jqCompact     bool
	jsonSchemaMin bool
)

// readJSON reads a file (or stdin) and parses it leniently.
func readJSON(args []string) (*jsonkit.Value, string, error) {
	src, name, err := readSource(args)
	if err != nil {
		return nil, "", err
	}
	v, d, err := jsonkit.Parse(string(src))
	if err != nil {
		return nil, "", inFile(name, err)
	}
	return v, string(d), nil
}

// inFile prefixes a parse error with the file name (file:line:col).
func inFile(name string, err error) error {
	if name == "" {
		name = "<stdin>"
	}
	if se, ok := err.(*jsonkit.SyntaxError); ok {
		return fmt.Errorf("%s:%d:%d: %s", name, se.Line, se.Column, se.Msg)
	}
	return fmt.Errorf("%s: %w", name, err)
}

// reformat reads JSON text (the argument, or stdin) leniently and writes
// it as strict JSON with indent ("" for one line), keys sorted with --sort.
func reformat(args []string, ind string) error {
	in, err := readInput(args)
	if err != nil {
		return err
	}
	v, _, err := jsonkit.Parse(in)
	if err != nil {
		return inFile("", err)
	}
	if jsonSort {
		v = jsonkit.SortKeys(v)
	}
	return emit(jsonkit.Encode(v, ind))
}

func jsonIndentFlag() string {
	if indent == "" {
		return "  "
	}
	return indent
}

var jsonYAMLCmd = &cobra.Command{
	Use:   "yaml [file]",
	Short: "Convert JSON (or JSONC, JSON5) to YAML, keeping key order",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		v, _, err := readJSON(args)
		if err != nil {
			return err
		}
		y, err := jsonkit.ToYAML(v)
		if err != nil {
			return err
		}
		return emit(strings.TrimRight(y, "\n"))
	},
}

var jsonFromYAMLCmd = &cobra.Command{
	Use:   "from-yaml [file]",
	Short: "Convert YAML to JSON (several documents become an array)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		src, name, err := readSource(args)
		if err != nil {
			return err
		}
		v, err := jsonkit.FromYAML(string(src))
		if err != nil {
			return inFile(name, err)
		}
		return emit(jsonkit.Encode(v, jsonIndentFlag()))
	},
}

var jsonFlattenCmd = &cobra.Command{
	Use:   "flatten [file]",
	Short: "List every value as a path and value (.spec.replicas = 3)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		v, _, err := readJSON(args)
		if err != nil {
			return err
		}
		if jsonOut {
			return emitJSON(jsonkit.Flatten(v))
		}
		return emit(strings.TrimRight(jsonkit.FlattenText(v), "\n"))
	},
}

var jsonUnflattenCmd = &cobra.Command{
	Use:   "unflatten [file]",
	Short: "Rebuild JSON from path = value lines (as flatten writes them)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		src, name, err := readSource(args)
		if err != nil {
			return err
		}
		v, err := jsonkit.Unflatten(string(src))
		if err != nil {
			return inFile(name, err)
		}
		return emit(jsonkit.Encode(v, jsonIndentFlag()))
	},
}

var jsonDiffCmd = &cobra.Command{
	Use:   "diff <old> <new>",
	Short: "Show what changed between two JSON files, by path; exit code 2 when they differ",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		a, _, err := readJSON(args[:1])
		if err != nil {
			return err
		}
		b, _, err := readJSON(args[1:])
		if err != nil {
			return err
		}
		changes := jsonkit.Diff(a, b)
		if jsonOut {
			if err := emitJSON(changes); err != nil {
				return err
			}
		} else {
			var out strings.Builder
			for _, c := range changes {
				switch c.Kind {
				case "added":
					fmt.Fprintf(&out, "+ %s: %s\n", c.Path, c.New)
				case "removed":
					fmt.Fprintf(&out, "- %s: %s\n", c.Path, c.Old)
				case "changed":
					fmt.Fprintf(&out, "~ %s: %s → %s\n", c.Path, c.Old, c.New)
				default:
					fmt.Fprintf(&out, "± %s: %s → %s (type changed)\n", c.Path, c.Old, c.New)
				}
			}
			if len(changes) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "no differences")
				return nil
			}
			if err := emit(strings.TrimRight(out.String(), "\n")); err != nil {
				return err
			}
		}
		if len(changes) > 0 {
			return exitCode(2)
		}
		return nil
	},
}

var jsonSchemaCmd = &cobra.Command{
	Use:   "schema [file...]",
	Short: "Infer a JSON Schema from samples (files, or JSON Lines on stdin)",
	RunE: func(cmd *cobra.Command, args []string) error {
		var samples []*jsonkit.Value
		read := func(src []byte, name string) error {
			vs, _, err := jsonkit.ParseAll(string(src))
			if err != nil {
				return inFile(name, err)
			}
			samples = append(samples, vs...)
			return nil
		}
		if len(args) == 0 {
			src, _, err := readSource(nil)
			if err != nil {
				return err
			}
			if err := read(src, ""); err != nil {
				return err
			}
		}
		for _, f := range args {
			src, err := os.ReadFile(f)
			if err != nil {
				return err
			}
			if err := read(src, f); err != nil {
				return err
			}
		}
		ind := jsonIndentFlag()
		if jsonSchemaMin {
			ind = ""
		}
		return emit(jsonkit.Encode(jsonkit.Infer(samples), ind))
	},
}

var jsonQueryCmd = &cobra.Command{
	Use:   "query <filter> [file]",
	Short: "Run a jq filter (no environment, at most 1000 results, 5 seconds)",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		v, _, err := readJSON(args[1:])
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		ind := "  "
		if jqCompact {
			ind = ""
		}
		res, err := jsonkit.Query(ctx, v, args[0], ind)
		if err != nil {
			return err
		}
		if res.Truncated {
			fmt.Fprintf(cmd.ErrOrStderr(), "stopped after %d results\n", jsonkit.QueryLimit)
		}
		if len(res.Results) == 0 {
			return nil
		}
		return emit(strings.Join(res.Results, "\n"))
	},
}

var jsonStatsCmd = &cobra.Command{
	Use:   "stats [file]",
	Short: "Count what large JSON holds, reading it as a stream (strict JSON)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var r io.Reader = os.Stdin
		if len(args) > 0 && args[0] != "-" {
			f, err := os.Open(args[0])
			if err != nil {
				return err
			}
			defer f.Close()
			r = f
		}
		st, err := jsonkit.Scan(r)
		if err != nil {
			return err
		}
		if jsonOut {
			return emitJSON(st)
		}
		b, _ := json.Marshal(st)
		var m map[string]int64
		_ = json.Unmarshal(b, &m)
		var out strings.Builder
		for _, k := range []string{"bytes", "values", "objects", "arrays", "keys", "strings", "numbers", "bools", "nulls", "maxDepth"} {
			fmt.Fprintf(&out, "%-9s %d\n", k, m[k])
		}
		return emit(strings.TrimRight(out.String(), "\n"))
	},
}

func init() {
	jsonCmd.Short = "Format, convert, compare, query and describe JSON"
	jsonPrettyCmd.Flags().BoolVar(&jsonSort, "sort", false, "sort object keys")
	jsonMinCmd.Flags().BoolVar(&jsonSort, "sort", false, "sort object keys")
	for _, c := range []*cobra.Command{jsonFromYAMLCmd, jsonUnflattenCmd, jsonSchemaCmd} {
		c.Flags().StringVar(&indent, "indent", "", "indentation string (default two spaces)")
	}
	jsonSchemaCmd.Flags().BoolVar(&jsonSchemaMin, "min", false, "write the schema on one line")
	jsonQueryCmd.Flags().BoolVar(&jqCompact, "compact", false, "one line per result")
	for _, c := range []*cobra.Command{jsonFlattenCmd, jsonDiffCmd, jsonStatsCmd} {
		c.Flags().BoolVar(&jsonOut, "json", false, "JSON output")
	}
	jsonCmd.AddCommand(jsonYAMLCmd, jsonFromYAMLCmd, jsonFlattenCmd, jsonUnflattenCmd, jsonDiffCmd, jsonSchemaCmd, jsonQueryCmd, jsonStatsCmd)
}
