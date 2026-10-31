package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/names"
)

// The Text & data tool's commands: case.

var (
	caseTo    string
	caseCheck bool
	caseJSON  bool
)

// caseAliases are the short names --to takes.
var caseAliases = map[string]string{
	"camel": "camelCase", "pascal": "PascalCase", "snake": "snake_case", "kebab": "kebab-case",
	"constant": "CONSTANT_CASE", "dot": "dot.case", "title": "Title Case", "sentence": "Sentence case",
	"lower": "lower case", "path": "path/case",
}

var caseCmd = &cobra.Command{
	Use:   "case [text]",
	Short: "A name in camel, Pascal, snake, kebab, CONSTANT and other cases; Kubernetes, Helm and env-var rules",
	Long: `case writes a name or phrase in every common case, splitting it into words
at separators and case changes and keeping acronyms (HTTP, ID) whole.
--to prints one case, for each line of stdin when the input is piped.
--check tests the name against the Kubernetes name and label rules, an
environment variable name and a Helm release name, and suggests a name
that passes (exit 2 when one fails).`,
	Example: `  codec case "user ID from HTTP request"
  codec case getHTTPResponse --to snake
  printf 'orderService\nPaymentGateway\n' | codec case --to kebab
  codec case "Order Service v2" --check`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		in, err := readInput(args)
		if err != nil {
			return err
		}
		if caseCheck {
			checks := names.Checks(in)
			if caseJSON {
				return emitJSON(checks)
			}
			var b strings.Builder
			failed := false
			for _, c := range checks {
				if c.Valid {
					fmt.Fprintf(&b, "ok    %s\n", c.Rule)
				} else {
					failed = true
					fmt.Fprintf(&b, "fails %s\n", c.Rule)
				}
				for _, p := range c.Problems {
					fmt.Fprintf(&b, "        %s\n", p)
				}
				if c.Fixed != "" {
					fmt.Fprintf(&b, "        try: %s\n", c.Fixed)
				}
			}
			if err := emit(strings.TrimRight(b.String(), "\n")); err != nil {
				return err
			}
			if failed {
				return exitCode(2)
			}
			return nil
		}
		if caseTo != "" {
			name, ok := caseAliases[strings.ToLower(caseTo)]
			if !ok {
				return fmt.Errorf("unknown case %q (camel, pascal, snake, kebab, constant, dot, title, sentence, lower, path)", caseTo)
			}
			var out []string
			for _, line := range strings.Split(in, "\n") {
				for _, c := range names.Cases(strings.TrimSpace(line)) {
					if c.Name == name {
						out = append(out, c.Value)
					}
				}
			}
			return emit(strings.Join(out, "\n"))
		}
		cases := names.Cases(in)
		if caseJSON {
			return emitJSON(cases)
		}
		rows := make([][2]string, len(cases))
		for i, c := range cases {
			rows[i] = [2]string{c.Name, c.Value}
		}
		return emit(table(rows))
	},
}

func init() {
	caseCmd.Flags().StringVar(&caseTo, "to", "", "print only this case: camel, pascal, snake, kebab, constant, dot, title, sentence, lower, path")
	caseCmd.Flags().BoolVar(&caseCheck, "check", false, "test the name against Kubernetes, Helm and environment-variable rules (exit 2 when one fails)")
	caseCmd.Flags().BoolVar(&caseJSON, "json", false, "print JSON")
	rootCmd.AddCommand(caseCmd)
}
