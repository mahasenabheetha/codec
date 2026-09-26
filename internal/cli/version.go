package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the codec version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(cmd.OutOrStdout(), "codec", version.Get())
	},
}

func init() {
	// Setting Version also gives the root command a --version flag;
	// the template makes both spellings print the same line.
	rootCmd.Version = version.Get().String()
	rootCmd.SetVersionTemplate("codec {{.Version}}\n")

	rootCmd.AddCommand(versionCmd)
}
