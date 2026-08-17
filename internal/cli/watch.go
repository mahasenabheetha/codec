package cli

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/internal/codec"
)

// interval is bound to the --interval flag: how often to poll.
var interval time.Duration

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch the clipboard and auto-transform anything recognizable",
	Long: `watch polls the system clipboard. Whenever you copy something new
that looks like JSON, base64 or a JWT, it transforms it and puts the
result straight back on the clipboard, ready to paste.

Copy JSON anywhere -> paste base64. Copy base64 -> paste decoded JSON.
Press Ctrl+C to stop.`,
	Args: cobra.NoArgs,
	RunE: runWatch,
}

func runWatch(cmd *cobra.Command, args []string) error {
	// time.NewTicker panics on a non-positive interval, so reject bad
	// flag values with a proper error instead of a crash.
	if interval <= 0 {
		return fmt.Errorf("--interval must be positive, got %v", interval)
	}

	// Seed with whatever is on the clipboard right now, so starting the
	// watcher never transforms stale content copied minutes ago.
	last, _ := clipboard.ReadAll()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Ask the OS to deliver Ctrl+C as a value on this channel instead
	// of killing the process outright, so we can exit cleanly.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	fmt.Printf("watching clipboard (every %v) — Ctrl+C to stop\n", interval)

	for {
		select {
		case <-sig:
			fmt.Println("\nstopped")
			return nil

		case <-ticker.C:
			current, err := clipboard.ReadAll()
			if err != nil || current == "" || current == last {
				continue
			}
			last = current

			out, kind, err := codec.Transform(current)
			if err != nil {
				// Not JSON/base64/JWT — the user copied prose, a URL,
				// a password... none of our business. Stay quiet.
				continue
			}

			if err := clipboard.WriteAll(out); err != nil {
				fmt.Fprintln(os.Stderr, "warning: clipboard write failed:", err)
				continue
			}
			// Remember our own write, otherwise the next tick would see
			// "new" clipboard content and transform it straight back —
			// an endless encode/decode ping-pong.
			last = out

			fmt.Printf("[%s] %s → %s\n",
				time.Now().Format("15:04:05"), kind, preview(out))
		}
	}
}

// preview flattens a result to a single short line for the log.
// Truncation counts runes, not bytes, so multi-byte characters are
// never cut in half.
func preview(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) > 60 {
		return string(r[:60]) + "..."
	}
	return s
}

func init() {
	watchCmd.Flags().DurationVar(&interval, "interval", 300*time.Millisecond,
		"how often to check the clipboard (e.g. 200ms, 1s)")
	rootCmd.AddCommand(watchCmd)
}
