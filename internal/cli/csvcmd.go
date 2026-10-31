package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	csvtable "github.com/mahasenabheetha/codec/v2/internal/table"
)

// The CSV & TSV tab on the command line (3.7.0): a file or stdin, the
// delimiter and header row guessed unless given.

var (
	csvDelim  string
	csvHeader string
	csvText   bool
	csvTSV    bool
	csvJSON   bool
)

var csvCmd = &cobra.Command{
	Use:   "csv",
	Short: "Describe CSV or TSV and convert it to JSON, YAML, Markdown and back",
}

// readTable reads a file (or stdin) as delimited text.
func readTable(args []string) (*csvtable.Table, error) {
	src, name, err := readSource(args)
	if err != nil {
		return nil, err
	}
	var d rune
	switch strings.ToLower(csvDelim) {
	case "", "auto":
	case ",", "comma":
		d = ','
	case "\t", "tab", `\t`:
		d = '\t'
	case ";", "semicolon":
		d = ';'
	case "|", "pipe":
		d = '|'
	default:
		return nil, fmt.Errorf("--delimiter is comma, tab, semicolon or pipe, not %q", csvDelim)
	}
	t, err := csvtable.Parse(string(src), csvtable.Options{Delim: d, Header: csvHeader})
	if err != nil {
		if name == "" {
			name = "<stdin>"
		}
		if se, ok := err.(*csvtable.SyntaxError); ok {
			return nil, fmt.Errorf("%s:%d:%d: %s", name, se.Line, se.Column, se.Msg)
		}
		return nil, err
	}
	return t, nil
}

func delimName(d rune) string {
	switch d {
	case '\t':
		return "tab"
	case ';':
		return "semicolon"
	case '|':
		return "pipe"
	}
	return "comma"
}

var csvInfoCmd = &cobra.Command{
	Use:   "info [file]",
	Short: "Show the delimiter, header, rows and each column's type and counts",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		t, err := readTable(args)
		if err != nil {
			return err
		}
		if csvJSON {
			return emitJSON(map[string]any{
				"delimiter": string(t.Delim), "header": t.Header, "rows": len(t.Rows), "ragged": t.Ragged, "columns": t.Columns,
			})
		}
		guessed := func(g bool) string {
			if g {
				return " (guessed)"
			}
			return ""
		}
		var b strings.Builder
		fmt.Fprintf(&b, "delimiter %s%s\n", delimName(t.Delim), guessed(t.DelimGuessed))
		fmt.Fprintf(&b, "header    %v%s\n", t.Header, guessed(t.HeaderGuessed))
		fmt.Fprintf(&b, "rows      %d\n", len(t.Rows))
		if t.Ragged > 0 {
			fmt.Fprintf(&b, "ragged    %d rows with more or fewer fields\n", t.Ragged)
		}
		width := 4
		for _, c := range t.Columns {
			width = max(width, len(c.Name))
		}
		fmt.Fprintf(&b, "\n%-*s  %-8s %8s %8s %8s\n", width, "name", "type", "filled", "empty", "distinct")
		for _, c := range t.Columns {
			more := ""
			if c.DistinctMore {
				more = "+"
			}
			fmt.Fprintf(&b, "%-*s  %-8s %8d %8d %8d%s\n", width, c.Name, c.Type, c.Filled, c.Empty, c.Distinct, more)
		}
		return emit(strings.TrimRight(b.String(), "\n"))
	},
}

var csvToCmd = &cobra.Command{
	Use:   "to <json|jsonl|yaml|markdown|csv|tsv> [file]",
	Short: "Convert CSV or TSV (values typed by column unless --text)",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		t, err := readTable(args[1:])
		if err != nil {
			return err
		}
		out, err := csvtable.Convert(t, args[0], !csvText, jsonIndentFlag())
		if err != nil {
			return err
		}
		return emit(strings.TrimRight(out, "\n"))
	},
}

var csvFromJSONCmd = &cobra.Command{
	Use:   "from-json [file]",
	Short: "Write a JSON array of objects (or arrays, or JSON Lines) as CSV",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		src, name, err := readSource(args)
		if err != nil {
			return err
		}
		t, err := csvtable.FromJSON(string(src))
		if err != nil {
			return inFile(name, err)
		}
		d := ','
		if csvTSV {
			d = '\t'
		}
		return emit(strings.TrimRight(t.Delimited(d), "\n"))
	},
}

func init() {
	for _, c := range []*cobra.Command{csvInfoCmd, csvToCmd} {
		c.Flags().StringVar(&csvDelim, "delimiter", "", "comma, tab, semicolon or pipe (default: guessed)")
		c.Flags().StringVar(&csvHeader, "header", "auto", "whether the first row names the columns: yes, no or auto")
	}
	csvInfoCmd.Flags().BoolVar(&csvJSON, "json", false, "JSON output")
	csvToCmd.Flags().BoolVar(&csvText, "text", false, "every JSON or YAML value as text")
	csvToCmd.Flags().StringVar(&indent, "indent", "", "JSON indentation string (default two spaces)")
	csvFromJSONCmd.Flags().BoolVar(&csvTSV, "tsv", false, "write TSV instead of CSV")
	csvCmd.AddCommand(csvInfoCmd, csvToCmd, csvFromJSONCmd)
	rootCmd.AddCommand(csvCmd)
}
