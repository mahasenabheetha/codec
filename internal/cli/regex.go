package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/regex"
)

var (
	rxStyle   string
	rxFlags   string
	rxFirst   bool
	rxReplace string
	rxExplain bool
	rxJSON    bool
)

var regexCmd = &cobra.Command{
	Use:   "regex <pattern> [file|-]",
	Short: "Test a regular expression on text, replace with it, or explain it",
	Long: `regex runs a pattern over a file or stdin and lists each match with its
line and groups. --replace prints the text with matches replaced ($1,
${name}; \1 and \g<name> work too). --explain describes the pattern
part by part instead.

--style go is Go's RE2 (linear time, no lookarounds or backreferences);
--style pcre is the Python / .NET / JavaScript style, with lookarounds
and backreferences, stopped after 2 seconds if it backtracks badly.

Exit status: 0 matched, 1 error, 2 no match.`,
	Example: "  codec regex '(\\d+) ms' build.log\n  kubectl logs pod | codec regex -f i 'error|fatal' -\n  codec regex --style pcre '\\d+(?= ms)' build.log\n  codec regex '(\\d{4})-(\\d\\d)-(\\d\\d)' dates.txt --replace '$3.$2.$1'\n  codec regex --explain '^v(\\d+)\\.(\\d+)$'",
	Args:    cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		o := regex.Options{Pattern: args[0], Style: rxStyle, Flags: rxFlags, All: !rxFirst}
		out := cmd.OutOrStdout()
		if rxExplain {
			nodes := regex.Explain(o.Pattern, o.Style, o.Flags)
			if rxJSON {
				return emitJSON(nodes)
			}
			var b strings.Builder
			writeExplain(&b, nodes, 0)
			return emit(strings.TrimRight(b.String(), "\n"))
		}
		var data []byte
		var err error
		if len(args) < 2 || args[1] == "-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(args[1])
		}
		if err != nil {
			return err
		}
		text := string(data)
		if cmd.Flags().Changed("replace") {
			replaced, err := regex.Replace(o, text, rxReplace)
			if err != nil {
				return regexHint(err)
			}
			fmt.Fprint(out, replaced)
			return nil
		}
		res, err := regex.Run(o, text)
		if err != nil {
			return regexHint(err)
		}
		if rxJSON {
			if err := emitJSON(res); err != nil {
				return err
			}
		} else {
			for _, m := range res.Matches {
				fmt.Fprintf(out, "%d: %s\n", m.Line, m.Text)
				for _, g := range m.Groups {
					label := fmt.Sprint(g.Number)
					if g.Name != "" {
						label += " " + g.Name
					}
					if g.Start < 0 {
						fmt.Fprintf(out, "   %s: (no match)\n", label)
					} else {
						fmt.Fprintf(out, "   %s: %s\n", label, g.Text)
					}
				}
			}
			switch {
			case res.TimedOut:
				fmt.Fprintln(os.Stderr, "stopped after 2 seconds: the pattern backtracks badly (nested quantifiers such as (a+)+)")
			case res.Truncated:
				fmt.Fprintf(os.Stderr, "stopped at %d matches\n", regex.MaxMatches)
			}
		}
		if len(res.Matches) == 0 {
			if res.TimedOut {
				return exitCode(1) // not "no match": we don't know
			}
			return exitCode(2)
		}
		return nil
	},
}

func regexHint(err error) error {
	if e, ok := err.(*regex.Error); ok && e.Hint != "" {
		return fmt.Errorf("%s\n  hint: %s", e.Message, e.Hint)
	}
	return err
}

func writeExplain(b *strings.Builder, nodes []*regex.Node, depth int) {
	for _, n := range nodes {
		fmt.Fprintf(b, "%s%-*s  %s\n", strings.Repeat("  ", depth), max(1, 18-2*depth), n.Text, n.Desc)
		writeExplain(b, n.Children, depth+1)
	}
}

func init() {
	f := regexCmd.Flags()
	f.StringVar(&rxStyle, "style", "go", "go (RE2) or pcre (Python, .NET, JavaScript)")
	f.StringVarP(&rxFlags, "flags", "f", "", "i ignore case, m ^/$ per line, s . matches line breaks, x extended (pcre)")
	f.BoolVar(&rxFirst, "first", false, "only the first match")
	f.StringVar(&rxReplace, "replace", "", "print the text with matches replaced by this")
	f.BoolVar(&rxExplain, "explain", false, "describe the pattern part by part")
	f.BoolVar(&rxJSON, "json", false, "print as JSON")
	rootCmd.AddCommand(regexCmd)
}
