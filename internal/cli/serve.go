package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/internal/web"
)

var servePort int

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the local web UI",
	Long: `serve starts a small local web server and hosts the codec UI:
a page where you paste JSON, base64 or a JWT and get the transformed
result, with one-click copy.

It binds to 127.0.0.1 only — nothing on your network can reach it.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		addr := fmt.Sprintf("127.0.0.1:%d", servePort)
		return web.Serve(addr)
	},
}

func init() {
	serveCmd.Flags().IntVar(&servePort, "port", 8765, "port to listen on")
	rootCmd.AddCommand(serveCmd)
}
