package cli

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/sanitize"
)

var (
	sanKinds       []string
	sanWords       []string
	sanKeepPrivate bool
	sanMask        bool
	sanList        bool
	sanJSON        bool
)

var sanKindNames = []string{"secrets", "emails", "ips", "hosts", "uuids", "azure", "users"}

var sanitizeCmd = &cobra.Command{
	Use:   "sanitize [file|-]",
	Short: "Take secrets, addresses and names out of a text before sharing it",
	Long: `sanitize prints a text with what shouldn't leave the machine taken out:
secrets (password=, tokens, keys, connection strings, JWTs, passwords in
URLs), emails, IP addresses, host names, UUIDs, Azure resource names and
user names in paths, and words you name. Each value becomes the same
stand-in wherever it is (192.0.2.1, user-1@example.com,
host-1.example.internal), so the text still reads; --mask writes •••.

--list prints what was replaced to stderr. It is a safety net, not a
guarantee: read what you share.`,
	Example: `  codec sanitize app.log > app-shared.log
  kubectl logs deploy/web | codec sanitize --word acme --list
  codec sanitize --kinds secrets,emails --keep-private config.yaml`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var data []byte
		var err error
		if len(args) == 0 || args[0] == "-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(args[0])
		}
		if err != nil {
			return err
		}
		o := sanitize.Options{Words: sanWords, KeepPrivate: sanKeepPrivate, Mask: sanMask}
		for _, k := range sanKinds {
			switch strings.TrimSpace(k) {
			case "secrets":
				o.Secrets = true
			case "emails":
				o.Emails = true
			case "ips":
				o.IPs = true
			case "hosts":
				o.Hosts = true
			case "uuids":
				o.UUIDs = true
			case "azure":
				o.Azure = true
			case "users":
				o.Users = true
			default:
				return fmt.Errorf("no kind %q; the kinds are %s", k, strings.Join(sanKindNames, ", "))
			}
		}
		r := sanitize.Sanitize(string(data), o)
		if sanJSON {
			return emitJSON(r)
		}
		fmt.Fprint(cmd.OutOrStdout(), r.Text)
		if sanList {
			for _, f := range r.Findings {
				fmt.Fprintf(os.Stderr, "%-7s %s → %s (%d×)\n", f.Kind, f.Value, f.Replacement, f.Count)
			}
		}
		return nil
	},
}

func init() {
	f := sanitizeCmd.Flags()
	f.StringSliceVarP(&sanKinds, "kinds", "k", slices.Clone(sanKindNames), "what to take out: "+strings.Join(sanKindNames, ", "))
	f.StringArrayVarP(&sanWords, "word", "w", nil, "a word of your own to take out (a customer, a project); repeat for more")
	f.BoolVar(&sanKeepPrivate, "keep-private", false, "leave private addresses (10.x, 192.168.x) as they are")
	f.BoolVar(&sanMask, "mask", false, "write ••• instead of stand-ins")
	f.BoolVarP(&sanList, "list", "l", false, "print what was replaced to stderr")
	f.BoolVar(&sanJSON, "json", false, "print the text and what was replaced as JSON")
	rootCmd.AddCommand(sanitizeCmd)
}
