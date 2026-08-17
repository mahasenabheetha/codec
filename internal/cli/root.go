// Package cli wires the codec core to the command line. It owns
// everything about *presentation* — flags, stdin/stdout, exit codes,
// the clipboard — and contains no encoding logic of its own.
package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"
)

// copyToClipboard is bound to the global --copy / -c flag.
var copyToClipboard bool

var rootCmd = &cobra.Command{
	Use:   "codec",
	Short: "Encode, decode and reformat base64, JSON and JWTs",
	Long: `codec transforms the things a DevOps engineer pastes all day:
base64 blobs, JSON payloads, Kubernetes secrets and JWTs.

Input comes from an argument or from stdin, so both of these work:

  codec auto '{"a":1}'
  kubectl get secret db -o jsonpath='{.data.password}' | codec b64 decode`,

	// On a runtime error (bad input), print the error only — not the
	// full usage text, which buries the actual problem.
	SilenceUsage: true,
	// We print errors ourselves in Execute, with an "error:" prefix.
	SilenceErrors: true,
}

// Execute runs the CLI and is the only thing main() calls.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&copyToClipboard, "copy", "c", false,
		"also copy the output to the system clipboard")
}

// readInput returns the first positional argument if present, otherwise
// reads all of stdin. Trailing whitespace is trimmed because terminal
// pipes almost always append a newline.
func readInput(args []string) (string, error) {
	if len(args) > 0 {
		return strings.TrimSpace(args[0]), nil
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// emit prints the result to stdout and, when --copy is set, also places
// it on the system clipboard. Clipboard failure is a warning, not an
// error: the output is already on screen, so the run still succeeded.
func emit(out string) error {
	fmt.Println(out)

	if copyToClipboard {
		if err := clipboard.WriteAll(out); err != nil {
			fmt.Fprintln(os.Stderr, "warning: clipboard unavailable:", err)
		} else {
			fmt.Fprintln(os.Stderr, "(copied to clipboard)")
		}
	}
	return nil
}
