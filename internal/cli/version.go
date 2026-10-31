package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/config"
	"github.com/mahasenabheetha/codec/v2/internal/update"
	"github.com/mahasenabheetha/codec/v2/internal/version"
)

var versionCheck bool

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the codec version",
	Long: `Print the codec version.

--check asks GitHub whether a newer release is out (one request, at most
5 seconds); it is the only time the CLI goes online for this.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		info := version.Get()
		fmt.Fprintln(out, "codec", info)
		if !versionCheck {
			return nil
		}
		dir, _ := config.Dir()
		r, err := update.New(dir).Check(cmd.Context(), info.Version, true)
		switch {
		case err != nil:
			return err
		case r.Skipped == "dev":
			fmt.Fprintln(out, "This is a development build; there is no release to compare it with.")
		case r.Newer:
			fmt.Fprintf(out, "codec %s is available: %s\n", r.Latest.Version, r.Latest.URL)
		case r.Latest == nil:
			fmt.Fprintln(out, "No releases found.")
		default:
			fmt.Fprintln(out, "You have the latest release.")
		}
		return nil
	},
}

func init() {
	// Setting Version also gives the root command a --version flag;
	// the template makes both spellings print the same line.
	rootCmd.Version = version.Get().String()
	rootCmd.SetVersionTemplate("codec {{.Version}}\n")

	versionCmd.Flags().BoolVar(&versionCheck, "check", false, "also check GitHub for a newer release")
	rootCmd.AddCommand(versionCmd)
}
