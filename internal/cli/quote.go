package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/quote"
)

var (
	quoteFor     string
	quoteUnquote bool
	quoteAll     bool
	quoteJSON    bool
)

var quoteCmd = &cobra.Command{
	Use:   "quote [value]",
	Short: "Quote a value for a shell, a data format or a language, or read one back",
	Long: `quote writes a value so the target reads it back exactly, and prints
the best way. --all prints every way, with why, and what the target
would otherwise make of the value. --unquote reads a quoted value back.

Targets: ` + targetList() + `.

The value is the argument, or stdin; one newline at the end of stdin is
dropped, as echo adds it (end with two to keep one).`,
	Example: `  codec quote --for bash "it's \$5"
  codec quote --for yaml --all no
  printf 'line one\nline two' | codec quote --for bash
  codec quote --for powershell --unquote "'it''s'"`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var v string
		if len(args) == 1 {
			v = args[0] // as given: spaces at the ends are part of a value
		} else {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}
			// One newline at the end is echo's, not the value's.
			v = strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
		}
		if quoteUnquote {
			out, err := quote.Unquote(quoteFor, v)
			if err != nil {
				return err
			}
			return emit(out)
		}
		r, err := quote.Quote(quoteFor, v)
		if err != nil {
			return err
		}
		if quoteJSON {
			return emitJSON(r)
		}
		if !quoteAll {
			return emit(r.Forms[0].Value)
		}
		var b strings.Builder
		for _, f := range r.Forms {
			fmt.Fprintf(&b, "%s\n  %s\n  %s\n\n", f.Label, strings.ReplaceAll(f.Value, "\n", "\n  "), f.Why)
		}
		for _, n := range r.Notes {
			fmt.Fprintf(&b, "! %s\n", n)
		}
		return emit(strings.TrimRight(b.String(), "\n"))
	},
}

func targetList() string {
	var ids []string
	for _, t := range quote.Targets {
		ids = append(ids, t.ID)
	}
	return strings.Join(ids, ", ")
}

func init() {
	quoteCmd.Flags().StringVarP(&quoteFor, "for", "f", "bash", "where the value goes: "+targetList())
	quoteCmd.Flags().BoolVarP(&quoteUnquote, "unquote", "u", false, "read a quoted value back")
	quoteCmd.Flags().BoolVarP(&quoteAll, "all", "a", false, "every way to write it, with why")
	quoteCmd.Flags().BoolVar(&quoteJSON, "json", false, "print the forms and notes as JSON")
	rootCmd.AddCommand(quoteCmd)
}
