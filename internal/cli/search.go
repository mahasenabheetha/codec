package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/search"
	"github.com/mahasenabheetha/codec/v2/internal/workspace"
)

var (
	searchOpts search.Options
	searchJSON bool
)

var searchCmd = &cobra.Command{
	Use:   "search <text> [folder]",
	Short: "Find text in a folder's files (honours .gitignore)",
	Long: `search finds a phrase in every file the web UI's explorer lists:
.gitignore and the usual build folders are skipped, as are binaries and
files over 8 MB. It is case-insensitive unless -s (--case-sensitive) is given.

Exit status: 0 found, 1 not found or error.`,
	Example: `  codec search "image: nginx"
  codec search -w replicas charts/
  codec search -e 'tag: v?\d+\.\d+' -g '*.yaml' -g '!**/tests/**'`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runSearch,
}

func init() {
	f := searchCmd.Flags()
	f.BoolVarP(&searchOpts.Regex, "regex", "e", false, "the text is a regular expression (Go RE2 syntax)")
	f.BoolVarP(&searchOpts.MatchCase, "case-sensitive", "s", false, "match case")
	f.BoolVarP(&searchOpts.WholeWord, "word", "w", false, "match whole words only")
	f.StringArrayVarP(&searchOpts.Globs, "glob", "g", nil, `only files matching this glob; "!" excludes (repeatable)`)
	f.IntVar(&searchOpts.Limit, "limit", search.DefaultLimit, "stop after this many matching lines")
	f.BoolVar(&searchJSON, "json", false, "print matches as JSON")
	rootCmd.AddCommand(searchCmd)
}

func runSearch(cmd *cobra.Command, args []string) error {
	searchOpts.Query = args[0]
	dir := "."
	if len(args) == 2 {
		dir = args[1]
	}
	s, err := search.Compile(searchOpts)
	if err != nil {
		return err
	}
	ws, err := workspace.Open(dir)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	files, _, err := ws.Files(ctx)
	if err != nil {
		return err
	}
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}
	res, err := s.Run(ctx, paths, ws.ReadText)
	if err != nil {
		return err
	}
	if searchJSON {
		if err := emitJSON(res); err != nil {
			return err
		}
	} else {
		w := cmd.OutOrStdout()
		for _, m := range res.Matches {
			fmt.Fprintf(w, "%s:%d:%d: %s\n", m.Path, m.Line, m.Col, m.Text)
		}
		if res.Truncated {
			fmt.Fprintf(os.Stderr, "stopped at %d matches (--limit)\n", len(res.Matches))
		}
	}
	if len(res.Matches) == 0 {
		return exitCode(1)
	}
	return nil
}
