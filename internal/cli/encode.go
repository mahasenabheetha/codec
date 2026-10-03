package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/mahasenabheetha/codec/v2/internal/encode"
)

// The Encode & hash tool's commands: url, hex, hash, secret, uuid and
// htpasswd. Logic lives in internal/encode.

var (
	urlWhole, urlPlus, urlJSON bool
	hexUpper                   bool
	hexSep                     string
	hashAlgos                  []string
	hashKeyEnv                 string
	hashBase64                 bool
	secretOpts                 encode.SecretOptions
	secretSets                 []string
	genCount                   int
	htCost                     int
	htCheck                    string
)

var urlCmd = &cobra.Command{
	Use:   "url",
	Short: "Percent-encode, decode or take apart URLs",
}

var urlEncodeCmd = &cobra.Command{
	Use:     "encode [text]",
	Short:   "Percent-encode a value (or a whole URL with --whole)",
	Example: "  codec url encode 'a b&c'\n  codec url encode --whole 'https://x.io/a b?q=é'",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		in, err := readInput(args)
		if err != nil {
			return err
		}
		return emit(encode.URLEncode(in, urlWhole, urlPlus))
	},
}

var urlDecodeCmd = &cobra.Command{
	Use:   "decode [text]",
	Short: "Decode percent-encoding (+ as space with --plus)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		in, err := readInput(args)
		if err != nil {
			return err
		}
		out, err := encode.URLDecode(in, urlPlus)
		if err != nil {
			return err
		}
		return emit(out)
	},
}

var urlParseCmd = &cobra.Command{
	Use:     "parse [url]",
	Short:   "Show a URL's parts and its query string as a table",
	Example: "  codec url parse 'https://x.io/search?q=a+b&tag=1&tag=2'\n  codec url parse 'a=1&b=2'",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		in, err := readInput(args)
		if err != nil {
			return err
		}
		p, err := encode.ParseURL(in)
		if err != nil {
			return err
		}
		if urlJSON {
			return emitJSON(p)
		}
		var b strings.Builder
		tw := tabwriter.NewWriter(&b, 0, 4, 2, ' ', 0)
		for _, kv := range [][2]string{{"scheme", p.Scheme}, {"user", p.User}, {"host", p.Host}, {"port", p.Port}, {"path", p.Path}, {"fragment", p.Fragment}} {
			if kv[1] != "" {
				fmt.Fprintf(tw, "%s\t%s\n", kv[0], kv[1])
			}
		}
		if len(p.Query) > 0 {
			if b.Len() > 0 || p.Scheme != "" {
				fmt.Fprintln(tw)
			}
			fmt.Fprintln(tw, "KEY\tVALUE")
			for _, q := range p.Query {
				fmt.Fprintf(tw, "%s\t%s\n", q.Key, q.Value)
			}
		}
		tw.Flush()
		return emit(strings.TrimRight(b.String(), "\n"))
	},
}

var hexCmd = &cobra.Command{
	Use:   "hex",
	Short: "Hex to text and back",
}

var hexEncodeCmd = &cobra.Command{
	Use:     "encode [text]",
	Short:   "Write text (or stdin) as hex",
	Example: "  codec hex encode hello\n  codec hex encode --sep : --upper hello",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		in, err := exactInput(args)
		if err != nil {
			return err
		}
		return emit(encode.HexEncode([]byte(in), hexUpper, hexSep))
	},
}

var hexDecodeCmd = &cobra.Command{
	Use:   "decode [hex]",
	Short: "Read hex (spaces, colons, 0x and \\x allowed) back to text",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		in, err := readInput(args)
		if err != nil {
			return err
		}
		b, err := encode.HexDecode(in)
		if err != nil {
			return err
		}
		out, text := encode.Printable(b)
		if !text {
			fmt.Fprintln(os.Stderr, "(not text: bytes that aren't UTF-8 are shown as \\xNN)")
		}
		return emit(out)
	},
}

var hashCmd = &cobra.Command{
	Use:   "hash [text]",
	Short: "SHA-256, SHA-384, SHA-512, SHA-1 and MD5 digests, or HMACs",
	Long: `hash prints digests of the text argument or of stdin, byte for byte as
given: "echo hi | codec hash" includes the newline, as sha256sum does.
With one --algo only the digest is printed, for scripts.

For an HMAC, put the key in an environment variable and name it with
--hmac-key-env, so the key stays out of your shell history.

MD5 and SHA-1 are broken for security use: checksums only.`,
	Example: "  codec hash 'hello'\n  codec hash -a sha256 < file.txt\n  KEY=s3cret codec hash --hmac-key-env KEY 'payload'",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		in, err := exactInput(args)
		if err != nil {
			return err
		}
		var key []byte
		if hashKeyEnv != "" {
			v, ok := os.LookupEnv(hashKeyEnv)
			if !ok {
				return fmt.Errorf("environment variable %s is not set", hashKeyEnv)
			}
			key = []byte(v)
		}
		ds, err := encode.Hash([]byte(in), key, hashAlgos...)
		if err != nil {
			return err
		}
		value := func(d encode.Digest) string {
			if hashBase64 {
				return d.Base64
			}
			return d.Hex
		}
		if len(ds) == 1 {
			return emit(value(ds[0]))
		}
		var b strings.Builder
		tw := tabwriter.NewWriter(&b, 0, 4, 2, ' ', 0)
		for _, d := range ds {
			name := d.Algorithm
			if key != nil {
				name = "HMAC-" + name
			}
			fmt.Fprintf(tw, "%s\t%s\n", name, value(d))
		}
		tw.Flush()
		return emit(strings.TrimRight(b.String(), "\n"))
	},
}

var secretCmd = &cobra.Command{
	Use:   "secret",
	Short: "Generate random secrets",
	Long: `secret prints random secrets from the operating system's secure random
source. As text it draws --length characters from the chosen --sets,
with at least one of each; as hex or base64 it encodes --length random
bytes. Symbols leave out quotes, backslash, backtick and space, so a
secret pastes into YAML, JSON and a shell as is.`,
	Example: "  codec secret\n  codec secret --length 24 --sets lower,upper,digits,symbols\n  codec secret --format hex --length 32 -n 3",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		o := secretOpts
		for _, s := range secretSets {
			switch strings.TrimSpace(s) {
			case "lower":
				o.Lower = true
			case "upper":
				o.Upper = true
			case "digits":
				o.Digits = true
			case "symbols":
				o.Symbols = true
			default:
				return fmt.Errorf("unknown set %q (lower, upper, digits, symbols)", s)
			}
		}
		lines := make([]string, 0, min(max(genCount, 1), 10000))
		for range min(max(genCount, 1), 10000) {
			s, err := encode.NewSecret(o)
			if err != nil {
				return err
			}
			lines = append(lines, s.Value)
		}
		return emit(strings.Join(lines, "\n"))
	},
}

var uuidCmd = &cobra.Command{
	Use:   "uuid",
	Short: "Generate random (version 4) UUIDs",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		lines := make([]string, min(max(genCount, 1), 10000))
		for i := range lines {
			lines[i] = encode.UUID()
		}
		return emit(strings.Join(lines, "\n"))
	},
}

var htpasswdCmd = &cobra.Command{
	Use:   "htpasswd <user>",
	Short: "Make a bcrypt user:hash line for basic auth, or check one",
	Long: `htpasswd prints a "user:hash" line with a bcrypt hash, as htpasswd -nbB
does, for nginx ingress and Traefik basic auth. The password is asked
for (not echoed), or read from stdin when it is piped, so it stays out
of your shell history.

With --check, the password is checked against a line instead.`,
	Example: "  codec htpasswd admin\n  codec htpasswd admin < password.txt\n  codec htpasswd --check 'admin:$2y$10$…'",
	Args: func(cmd *cobra.Command, args []string) error {
		if htCheck != "" {
			return cobra.NoArgs(cmd, args)
		}
		return cobra.ExactArgs(1)(cmd, args)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		pw, err := readPassword(htCheck == "")
		if err != nil {
			return err
		}
		if htCheck != "" {
			ok, err := encode.HtpasswdCheck(htCheck, pw)
			if err != nil {
				return err
			}
			if !ok {
				fmt.Println("no match")
				return exitCode(2)
			}
			return emit("match")
		}
		line, err := encode.Htpasswd(args[0], pw, htCost)
		if err != nil {
			return err
		}
		return emit(line)
	},
}

// exactInput is the argument as given, or stdin byte for byte: a hash
// or hex dump of trimmed input would be of different bytes.
func exactInput(args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	return string(data), nil
}

// readPassword asks on the terminal without echo (twice when confirm),
// or reads the first line of piped stdin.
func readPassword(confirm bool) (string, error) {
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		// Git Bash (mintty) gives programs a pipe, not a console, so typing
		// would show the password: say so, unless a file is piped in.
		if fi, err := os.Stdin.Stat(); err == nil && !fi.Mode().IsRegular() && os.Getenv("MSYSTEM") != "" {
			fmt.Fprintln(os.Stderr, "Reading the password from stdin; typed here it would show. In Git Bash run: winpty codec htpasswd …")
		}
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	ask := func(prompt string) (string, error) {
		fmt.Fprint(os.Stderr, prompt)
		b, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		return string(b), err
	}
	pw, err := ask("Password: ")
	if err != nil || !confirm {
		return pw, err
	}
	again, err := ask("Again: ")
	if err != nil {
		return "", err
	}
	if again != pw {
		return "", errors.New("the passwords don't match")
	}
	return pw, nil
}

func init() {
	urlEncodeCmd.Flags().BoolVar(&urlWhole, "whole", false, "encode a whole URL: keep : / ? # & = and existing %XX")
	for _, c := range []*cobra.Command{urlEncodeCmd, urlDecodeCmd} {
		c.Flags().BoolVar(&urlPlus, "plus", false, "a space is + (HTML form encoding)")
	}
	urlParseCmd.Flags().BoolVar(&urlJSON, "json", false, "print the parts as JSON")
	urlCmd.AddCommand(urlEncodeCmd, urlDecodeCmd, urlParseCmd)

	hexEncodeCmd.Flags().BoolVar(&hexUpper, "upper", false, "upper-case digits")
	hexEncodeCmd.Flags().StringVar(&hexSep, "sep", "", `between bytes, e.g. " " or ":"`)
	hexCmd.AddCommand(hexEncodeCmd, hexDecodeCmd)

	hashCmd.Flags().StringSliceVarP(&hashAlgos, "algo", "a", nil, "only these: sha256, sha384, sha512, sha1, md5")
	hashCmd.Flags().StringVar(&hashKeyEnv, "hmac-key-env", "", "compute HMACs with the key in this environment variable")
	hashCmd.Flags().BoolVar(&hashBase64, "base64", false, "print digests as base64 instead of hex")

	secretCmd.Flags().IntVarP(&secretOpts.Length, "length", "l", 32, fmt.Sprintf("characters, or bytes for hex and base64 (at most %d)", encode.MaxSecret))
	secretCmd.Flags().StringVar(&secretOpts.Format, "format", "text", "text, hex, base64 or base64url")
	secretCmd.Flags().StringSliceVar(&secretSets, "sets", []string{"lower", "upper", "digits"}, "character sets for text: lower, upper, digits, symbols")
	secretCmd.Flags().BoolVar(&secretOpts.NoAmbiguous, "no-ambiguous", false, "leave out I l 1 O 0 o")
	for _, c := range []*cobra.Command{secretCmd, uuidCmd} {
		c.Flags().IntVarP(&genCount, "count", "n", 1, "how many to print")
	}

	htpasswdCmd.Flags().IntVar(&htCost, "cost", encode.DefaultCost, fmt.Sprintf("bcrypt cost, %d to %d (each step doubles the time)", encode.MinCost, encode.MaxCost))
	htpasswdCmd.Flags().StringVar(&htCheck, "check", "", `check the password against this "user:hash" line`)

	rootCmd.AddCommand(urlCmd, hexCmd, hashCmd, secretCmd, uuidCmd, htpasswdCmd)
}
