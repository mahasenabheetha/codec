// Package cli wires the codec core to the command line. It owns
// everything about *presentation* — flags, stdin/stdout, exit codes,
// the clipboard — and contains no encoding logic of its own.
package cli

import (
	"errors"
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
	Long: `codec is a read-only workbench for the YAML a DevOps engineer lives in
(Kubernetes, Helm, Kustomize, Argo, CI pipelines, Compose, Ansible) and
the text they paste all day (base64, JSON, JWTs, Ansible logs).
It never modifies files.

  codec serve --open --sample         the web UI, on a sample repository
  codec yaml lint deploy/             lint and schema checks, exit 2 on findings
  codec helm values ./chart --provenance
  kubectl get secret db -o jsonpath='{.data.password}' | codec b64 decode

Docs: https://mahasenabheetha.github.io/codec/`,

	// On a runtime error (bad input), print the error only — not the
	// full usage text, which buries the actual problem.
	SilenceUsage: true,
	// We print errors ourselves in Execute, with an "error:" prefix.
	SilenceErrors: true,
}

// Execute runs the CLI and is the only thing main() calls.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		if ec, ok := errors.AsType[exitCode](err); ok {
			os.Exit(int(ec)) // the command already reported why
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// exitCode ends the process with a specific code and no extra message,
// e.g. 2 for "lint found problems" (design/conventions.md).
type exitCode int

func (e exitCode) Error() string { return fmt.Sprintf("exit status %d", int(e)) }

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
