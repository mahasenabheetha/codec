package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/encode"
	"github.com/mahasenabheetha/codec/v2/internal/netutil"
)

// The Network tool's commands: cidr, ip, and url build.

var (
	cidrSplit    int
	cidrContains string
	netJSON      bool

	buildScheme, buildUser, buildHost, buildPort, buildPath, buildFragment string
	buildSet, buildAdd, buildDel                                           []string
)

var cidrCmd = &cobra.Command{
	Use:   "cidr <range> [range…]",
	Short: "A CIDR range in detail, split into subnets, or a list checked for overlaps",
	Long: `cidr shows a range's network, broadcast, first and last host, mask,
wildcard, address and host counts and what kind of range it is (private,
shared, documentation, public…), for IPv4 and IPv6. A range is
address/length, an IPv4 address and netmask, or one address.

--split lists its subnets of a given length, --contains tells whether
an address is inside (exit 2 when it isn't). With several ranges, as
arguments or one per line on stdin, it reports the pairs that overlap
(exit 2 when any do): VNets, VPCs, pod and service ranges.`,
	Example: `  codec cidr 10.0.0.0/16
  codec cidr 10.0.0.0/16 --split 20
  codec cidr 10.0.0.0/16 --contains 10.0.12.7
  codec cidr 10.0.0.0/16 10.0.4.0/22 10.1.0.0/16
  cat ranges.txt | codec cidr`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			in, err := readInput(nil)
			if err != nil {
				return err
			}
			args = strings.Split(in, "\n")
		}
		if len(args) > 1 {
			return cidrOverlaps(args)
		}
		switch {
		case cidrContains != "":
			ok, err := netutil.Contains(args[0], cidrContains)
			if err != nil {
				return err
			}
			if !ok {
				fmt.Fprintf(os.Stderr, "%s is not in %s\n", cidrContains, args[0])
				return exitCode(2)
			}
			return emit(fmt.Sprintf("%s is in %s", cidrContains, args[0]))
		case cidrSplit > 0:
			sub, err := netutil.Split(args[0], cidrSplit)
			if err != nil {
				return err
			}
			if netJSON {
				return emitJSON(sub)
			}
			out := strings.Join(sub.Subnets, "\n")
			if sub.Truncated {
				out += fmt.Sprintf("\n… the first %d of %s subnets", len(sub.Subnets), sub.Count)
			}
			return emit(out)
		}
		r, err := netutil.Details(args[0])
		if err != nil {
			return err
		}
		if netJSON {
			return emitJSON(r)
		}
		return emit(table([][2]string{
			{"range", r.CIDR}, {"host", r.Host}, {"kind", r.Kind}, {"network", r.Network}, {"broadcast", r.Broadcast},
			{"first host", r.First}, {"last host", r.Last}, {"mask", r.Mask}, {"wildcard", r.Wildcard},
			{"addresses", r.Addresses}, {"hosts", r.Hosts},
		}))
	},
}

func cidrOverlaps(lines []string) error {
	list, pairs, err := netutil.Overlaps(lines)
	if err != nil {
		return err
	}
	if netJSON {
		if err := emitJSON(map[string]any{"ranges": list, "overlaps": pairs}); err != nil {
			return err
		}
	} else {
		var b strings.Builder
		for _, l := range list {
			if l.Error != "" {
				fmt.Fprintf(&b, "skipped %s: %s\n", l.Input, l.Error)
			}
		}
		for _, p := range pairs {
			verb := "contains"
			if p.Relation == "same" {
				verb = "is the same as"
			}
			fmt.Fprintf(&b, "%s %s %s\n", list[p.A].CIDR, verb, list[p.B].CIDR)
		}
		if len(pairs) == 0 {
			fmt.Fprintf(&b, "no overlaps in %d ranges\n", len(list))
		}
		if err := emit(strings.TrimRight(b.String(), "\n")); err != nil {
			return err
		}
	}
	if len(pairs) > 0 {
		return exitCode(2)
	}
	return nil
}

var ipCmd = &cobra.Command{
	Use:   "ip <address>",
	Short: "An IP address as integer, hex and binary, IPv6 expanded, and its reverse-DNS name",
	Long: `ip shows an address in other notations: decimal integer, hex, binary,
IPv6 expanded and shortened, IPv4-mapped IPv6, and the PTR name for
reverse DNS. It reads an address as text, or as a decimal, 0x hex or 0b
binary number (more than 32 bits, or more than 8 hex digits, is IPv6).`,
	Example: "  codec ip 10.0.0.1\n  codec ip 167772161\n  codec ip 2001:db8::1",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		in, err := readInput(args)
		if err != nil {
			return err
		}
		ip, err := netutil.ParseIP(in)
		if err != nil {
			return err
		}
		if netJSON {
			return emitJSON(ip)
		}
		mapped := "IPv4-mapped"
		if ip.Version == 6 {
			mapped = "IPv4 inside"
		}
		return emit(table([][2]string{
			{"address", ip.Address}, {"version", fmt.Sprint(ip.Version)}, {"kind", ip.Kind}, {"expanded", ip.Expanded},
			{"integer", ip.Integer}, {"hex", ip.Hex}, {"binary", ip.Binary}, {mapped, ip.Mapped}, {"reverse DNS", ip.Reverse},
		}))
	},
}

var urlBuildCmd = &cobra.Command{
	Use:   "build [url]",
	Short: "Build a URL from parts, or change one: its host, path or query parameters",
	Long: `build puts a URL together from --scheme, --host, --port, --path, --query
and --fragment, each escaped as its place needs. Given a URL, it starts
from that one: --set replaces a parameter (or adds it), --add adds one
more, --del removes every one of a name.`,
	Example: `  codec url build --scheme https --host api.example.com --path /v1/orders --add status=open --add page=2
  codec url build 'https://x.io/search?q=go&page=1' --set page=2 --del q`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p := &encode.URLParts{}
		if len(args) == 1 {
			var err error
			if p, err = encode.ParseURL(args[0]); err != nil {
				return err
			}
		}
		for _, f := range []struct {
			name string
			val  string
			to   *string
		}{{"scheme", buildScheme, &p.Scheme}, {"user", buildUser, &p.User}, {"host", buildHost, &p.Host}, {"port", buildPort, &p.Port}, {"path", buildPath, &p.Path}, {"fragment", buildFragment, &p.Fragment}} {
			if cmd.Flags().Changed(f.name) {
				*f.to = f.val
			}
		}
		for _, k := range buildDel {
			p.Query = withoutKey(p.Query, k)
		}
		for _, kv := range buildSet {
			k, v, _ := strings.Cut(kv, "=")
			at := -1
			for i, q := range p.Query {
				if q.Key == k {
					at = i
					break
				}
			}
			if at < 0 {
				p.Query = append(p.Query, encode.Param{Key: k, Value: v})
				continue
			}
			p.Query[at].Value = v
			p.Query = append(p.Query[:at+1], withoutKey(p.Query[at+1:], k)...)
		}
		for _, kv := range buildAdd {
			k, v, _ := strings.Cut(kv, "=")
			p.Query = append(p.Query, encode.Param{Key: k, Value: v})
		}
		u, err := encode.BuildURL(*p)
		if err != nil {
			return err
		}
		return emit(u)
	},
}

func withoutKey(q []encode.Param, key string) []encode.Param {
	out := q[:0:0]
	for _, p := range q {
		if p.Key != key {
			out = append(out, p)
		}
	}
	return out
}

// table prints name/value rows, leaving out empty values.
func table(rows [][2]string) string {
	var b strings.Builder
	tw := tabwriter.NewWriter(&b, 0, 4, 2, ' ', 0)
	for _, r := range rows {
		if r[1] != "" {
			fmt.Fprintf(tw, "%s\t%s\n", r[0], r[1])
		}
	}
	tw.Flush()
	return strings.TrimRight(b.String(), "\n")
}

func init() {
	cidrCmd.Flags().IntVar(&cidrSplit, "split", 0, "list the subnets of this length, e.g. 24")
	cidrCmd.Flags().StringVar(&cidrContains, "contains", "", "tell whether this address is in the range (exit 2 when not)")
	for _, c := range []*cobra.Command{cidrCmd, ipCmd} {
		c.Flags().BoolVar(&netJSON, "json", false, "print JSON")
	}

	urlBuildCmd.Flags().StringVar(&buildScheme, "scheme", "", "https, postgres, …")
	urlBuildCmd.Flags().StringVar(&buildUser, "user", "", "user name (never a password)")
	urlBuildCmd.Flags().StringVar(&buildHost, "host", "", "host name or IP address")
	urlBuildCmd.Flags().StringVar(&buildPort, "port", "", "port number")
	urlBuildCmd.Flags().StringVar(&buildPath, "path", "", "path, e.g. /v1/orders")
	urlBuildCmd.Flags().StringVar(&buildFragment, "fragment", "", "the part after #")
	urlBuildCmd.Flags().StringArrayVar(&buildSet, "set", nil, "key=value: replace a query parameter, or add it")
	urlBuildCmd.Flags().StringArrayVar(&buildAdd, "add", nil, "key=value: add a query parameter (repeats allowed)")
	urlBuildCmd.Flags().StringArrayVar(&buildDel, "del", nil, "key: remove every parameter of this name")
	urlCmd.AddCommand(urlBuildCmd)

	rootCmd.AddCommand(cidrCmd, ipCmd)
}
