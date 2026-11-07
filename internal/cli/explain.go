package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/commands"
)

var (
	explainJSON bool
	howTool     string
	howLimit    int
	howJSON     bool
)

var explainCmd = &cobra.Command{
	Use:   "explain <command line>",
	Short: "Explain a command line part by part",
	Long: `explain says what each part of a command line does: the command and
its subcommand, each flag and its value, the arguments, pipes and
redirections. Risky parts are marked with !, with a safer way when
there is one. Nothing is run.

Quote the line so your shell passes it as it is, or pipe it in. codec
knows git, kubectl, helm, docker and compose, and the common Linux
commands; others are split into their parts without explanations.`,
	Example: `  codec explain "git push --force origin main"
  codec explain 'kubectl logs deploy/web -f --tail=100 | grep -i error'
  history | tail -1 | cut -c8- | codec explain`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		line, err := readInput(args)
		if err != nil {
			return err
		}
		if line == "" {
			return fmt.Errorf("give a command line to explain")
		}
		ex := commands.Explain(line)
		if explainJSON {
			return emitJSON(explainOut(line, ex))
		}
		return emit(commands.Describe(ex))
	},
}

var howCmd = &cobra.Command{
	Use:   "how <what you want to do>",
	Short: "Find a command by what it does (the cheat sheet)",
	Long: `how searches codec's cheat sheet by purpose: describe what you want to
do in your own words and get recipes, commands and flags that do it,
best first, with what matched (the word, a synonym ≈ or a typo fixed →).
With only a tool's name, it lists that tool's recipes (-n 0 for all of them).

{name} in a command is a value to fill in.`,
	Example: `  codec how undo my last commit
  codec how "what is using port 8080"
  codec how --tool kubectl logs of a crashed pod
  codec how git`,
	Args: cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		q := strings.Join(args, " ")
		if q == "" && howTool == "" {
			return fmt.Errorf("say what you want to do, such as: codec how undo my last commit")
		}
		res, err := commands.Search(q, howTool, howLimit)
		if err != nil {
			return err
		}
		if howLimit > 0 && len(res) > howLimit {
			res = res[:howLimit]
		}
		if howJSON {
			return emitJSON(res)
		}
		if len(res) == 0 {
			return fmt.Errorf("nothing for %q; say what you want to happen rather than the command", q)
		}
		var b strings.Builder
		for i, r := range res {
			if i > 0 {
				b.WriteString("\n")
			}
			title := r.Title
			if r.Kind != "recipe" {
				title = r.Summary
			}
			fmt.Fprintf(&b, "%s\n  %s\n", title, strings.TrimSpace(r.Command))
			if r.Kind == "recipe" && r.Summary != "" {
				fmt.Fprintf(&b, "  note: %s\n", r.Summary)
			}
			if r.Danger != nil {
				fmt.Fprintf(&b, "  ! %s: %s\n", r.Danger.Level, r.Danger.Note)
			}
			if len(r.Why) > 0 {
				fmt.Fprintf(&b, "  matched %s\n", strings.Join(r.Why, ", "))
			}
		}
		return emit(strings.TrimRight(b.String(), "\n"))
	},
}

// explainJSONPart is a part of the line for --json; offsets are bytes.
type explainJSONPart struct {
	Start   int              `json:"start"`
	End     int              `json:"end"`
	Kind    string           `json:"kind"`
	Text    string           `json:"text"`
	Summary string           `json:"summary,omitempty"`
	Danger  *commands.Danger `json:"danger,omitempty"`
}

type explainJSONCommand struct {
	Path    []string          `json:"path"`
	Summary string            `json:"summary,omitempty"`
	Danger  *commands.Danger  `json:"danger,omitempty"`
	Parts   []explainJSONPart `json:"parts"`
}

// explainOut is an explanation for --json: the commands with their parts,
// the operators between them as parts of their own.
func explainOut(line string, ex commands.Explanation) any {
	out := struct {
		Line     string               `json:"line"`
		Commands []explainJSONCommand `json:"commands"`
	}{Line: line, Commands: []explainJSONCommand{}}
	for _, s := range ex.Segments {
		c := explainJSONCommand{Path: s.Path, Summary: s.Summary, Danger: s.Danger, Parts: []explainJSONPart{}}
		end := s.To
		if end < len(ex.Parts) && ex.Parts[end].Kind == commands.KindOp {
			end++ // the operator after it
		}
		for _, p := range ex.Parts[s.From:end] {
			c.Parts = append(c.Parts, explainJSONPart{p.Start, p.End, p.Kind, p.Text, p.Summary, p.Danger})
		}
		out.Commands = append(out.Commands, c)
	}
	return out
}

func init() {
	explainCmd.Flags().BoolVar(&explainJSON, "json", false, "print the explanation as JSON")
	howCmd.Flags().StringVarP(&howTool, "tool", "t", "", "only this tool's recipes, commands and flags")
	howCmd.Flags().IntVarP(&howLimit, "limit", "n", 5, "show at most this many")
	howCmd.Flags().BoolVar(&howJSON, "json", false, "print the results as JSON")
	rootCmd.AddCommand(explainCmd, howCmd)
}
