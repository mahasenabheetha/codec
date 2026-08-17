package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/internal/codec"
)

// urlSafe is bound to the --url flag on the b64 subcommands.
var urlSafe bool

// indent is bound to the --indent flag on `json pretty`.
var indent string

// variant maps the --url flag to the core package's enum.
func variant() codec.Variant {
	if urlSafe {
		return codec.VariantURL
	}
	return codec.VariantStd
}

var b64Cmd = &cobra.Command{
	Use:   "b64",
	Short: "Base64 encode and decode",
}

var b64EncodeCmd = &cobra.Command{
	Use:   "encode [text]",
	Short: "Encode text (or stdin) to base64",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		in, err := readInput(args)
		if err != nil {
			return err
		}
		return emit(codec.Encode([]byte(in), variant()))
	},
}

var b64DecodeCmd = &cobra.Command{
	Use:   "decode [base64]",
	Short: "Decode base64 (or stdin) to text",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		in, err := readInput(args)
		if err != nil {
			return err
		}
		raw, err := codec.Decode(in, variant())
		if err != nil {
			return err
		}
		return emit(string(raw))
	},
}

var jsonCmd = &cobra.Command{
	Use:   "json",
	Short: "Pretty-print, minify or validate JSON",
}

var jsonPrettyCmd = &cobra.Command{
	Use:   "pretty [json]",
	Short: "Reformat JSON with indentation",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		in, err := readInput(args)
		if err != nil {
			return err
		}
		out, err := codec.PrettyJSON(in, indent)
		if err != nil {
			return err
		}
		return emit(out)
	},
}

var jsonMinCmd = &cobra.Command{
	Use:   "min [json]",
	Short: "Minify JSON to a single line",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		in, err := readInput(args)
		if err != nil {
			return err
		}
		out, err := codec.MinifyJSON(in)
		if err != nil {
			return err
		}
		return emit(out)
	},
}

var jsonValidateCmd = &cobra.Command{
	Use:   "validate [json]",
	Short: "Check JSON syntax; exit code 1 when invalid",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		in, err := readInput(args)
		if err != nil {
			return err
		}
		if err := codec.ValidateJSON(in); err != nil {
			return err
		}
		fmt.Println("valid")
		return nil
	},
}

var jwtCmd = &cobra.Command{
	Use:   "jwt",
	Short: "Inspect JSON Web Tokens",
}

var jwtDecodeCmd = &cobra.Command{
	Use:   "decode [token]",
	Short: "Decode a JWT's header and payload (does NOT verify the signature)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		in, err := readInput(args)
		if err != nil {
			return err
		}
		jwt, err := codec.DecodeJWT(in)
		if err != nil {
			return err
		}
		out := "header:\n" + jwt.Header + "\n\npayload:\n" + jwt.Payload
		return emit(out)
	},
}

var autoCmd = &cobra.Command{
	Use:   "auto [input]",
	Short: "Detect the input type and apply the obvious transformation",
	Long: `auto detects what you pasted and does the right thing:
JSON is encoded to base64, base64 is decoded (and pretty-printed if it
contains JSON), and JWTs are expanded.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		in, err := readInput(args)
		if err != nil {
			return err
		}
		out, kind, err := codec.Transform(in)
		if err != nil {
			return err
		}
		// The detection note goes to stderr so that piping stdout
		// onward captures only the payload.
		fmt.Fprintln(cmd.ErrOrStderr(), "detected:", kind)
		return emit(out)
	},
}

func init() {
	b64Cmd.PersistentFlags().BoolVar(&urlSafe, "url", false,
		"use the URL-safe alphabet (-_ instead of +/)")
	jsonPrettyCmd.Flags().StringVar(&indent, "indent", "",
		"indentation string (default two spaces)")

	b64Cmd.AddCommand(b64EncodeCmd, b64DecodeCmd)
	jsonCmd.AddCommand(jsonPrettyCmd, jsonMinCmd, jsonValidateCmd)
	jwtCmd.AddCommand(jwtDecodeCmd)

	rootCmd.AddCommand(b64Cmd, jsonCmd, jwtCmd, autoCmd)
}
