package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/httpkit"
)

// The Network tool's HTTP commands: curl and http status.

var (
	curlFromFetch bool
	curlJSON      bool
)

var curlCmd = &cobra.Command{
	Use:   "curl [command]",
	Short: "Turn a curl command into a fetch call, or a fetch call into curl",
	Long: `curl reads a curl command, as a terminal, docs or a browser's
"Copy as cURL" give it (quotes, $'…' and line continuations included), and
prints the same request as JavaScript fetch. With --from-fetch it reads a
fetch call and prints curl. What can't be converted (files, proxies,
-k) is said on stderr. Nothing is sent.`,
	Example: `  codec curl "curl -X POST https://api.example.com/v1/orders -H 'Content-Type: application/json' -d '{\"qty\":2}'"
  pbpaste | codec curl
  codec curl --from-fetch "fetch('https://x.io', { method: 'DELETE' })"`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		in, err := readInput(args)
		if err != nil {
			return err
		}
		var r *httpkit.Request
		if curlFromFetch {
			r, err = httpkit.ParseFetch(in)
		} else {
			if !strings.HasPrefix(strings.TrimSpace(in), "curl") {
				in = "curl " + in // the arguments alone
			}
			r, err = httpkit.ParseCurl(in)
		}
		if err != nil {
			return err
		}
		for _, n := range r.Notes {
			fmt.Fprintln(os.Stderr, "note:", n)
		}
		switch {
		case curlJSON:
			return emitJSON(r)
		case curlFromFetch:
			return emit(r.Curl())
		}
		return emit(strings.TrimSuffix(r.Fetch(), "\n"))
	},
}

var httpCmd = &cobra.Command{
	Use:   "http",
	Short: "HTTP helpers",
}

var httpStatusCmd = &cobra.Command{
	Use:   "status [code | words]",
	Short: "What an HTTP status code means, its usual cause, and whether to retry",
	Long: `status explains HTTP status codes: by number (404), by the first digits
(5 for every 5xx, 50 for 500-509) or by words ("gateway", "rate limit").
Nothing at all lists every code.`,
	Example: "  codec http status 502\n  codec http status 4\n  codec http status timeout",
	RunE: func(cmd *cobra.Command, args []string) error {
		list := httpkit.Statuses(strings.Join(args, " "))
		if curlJSON {
			return emitJSON(list)
		}
		if len(list) == 0 {
			return fmt.Errorf("no status code matches %q", strings.Join(args, " "))
		}
		if len(list) == 1 {
			s := list[0]
			return emit(table([][2]string{
				{"code", strconv.Itoa(s.Code) + " " + s.Name}, {"meaning", s.Meaning}, {"usual cause", s.Cause}, {"retry", s.Retry},
			}))
		}
		var b strings.Builder
		for _, s := range list {
			fmt.Fprintf(&b, "%d  %-32s %s\n", s.Code, s.Name, s.Meaning)
		}
		return emit(strings.TrimRight(b.String(), "\n"))
	},
}

func init() {
	curlCmd.Flags().BoolVar(&curlFromFetch, "from-fetch", false, "read a fetch call and print curl")
	for _, c := range []*cobra.Command{curlCmd, httpStatusCmd} {
		c.Flags().BoolVar(&curlJSON, "json", false, "print JSON")
	}
	httpCmd.AddCommand(httpStatusCmd)
	rootCmd.AddCommand(curlCmd, httpCmd)
}
