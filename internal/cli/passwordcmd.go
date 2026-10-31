package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/encode"
)

// codec secret password and passphrase (3.8.0): passwords for people,
// by the rules a site asks for, or random words. The strength goes to
// stderr so stdout carries only the passwords.

var (
	pwOpts     encode.PasswordOptions
	pwClasses  []string
	pwMinEach  int
	pwMins     map[string]int
	phraseOpts encode.PassphraseOptions
)

var passwordCmd = &cobra.Command{
	Use:   "password",
	Short: "Generate passwords by rules: length, classes with a minimum each, exclusions",
	Long: `password prints passwords that follow the rules a site or policy asks
for: --length characters from the chosen --classes (lower, upper,
digits, symbols), at least --min-each of every class (or per class with
--min upper=2,digits=3), leaving out --exclude characters and, with
--no-ambiguous, I l 1 O 0 o. The strength in bits goes to stderr.`,
	Example: `  codec secret password
  codec secret password --length 14 --classes lower,upper,digits,symbols
  codec secret password --length 6 --classes digits      # a PIN`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		o := pwOpts
		o.MinLower, o.MinUpper, o.MinDigits, o.MinSymbols = -1, -1, -1, -1
		mins := map[string]*int{"lower": &o.MinLower, "upper": &o.MinUpper, "digits": &o.MinDigits, "symbols": &o.MinSymbols}
		for _, c := range pwClasses {
			p, ok := mins[strings.TrimSpace(c)]
			if !ok {
				return fmt.Errorf("unknown class %q (lower, upper, digits, symbols)", c)
			}
			*p = pwMinEach
		}
		for c, n := range pwMins {
			p, ok := mins[c]
			if !ok || *p < 0 {
				return fmt.Errorf("--min %s: not one of the --classes", c)
			}
			*p = n
		}
		return printPasswords(func() (encode.Password, error) { return encode.NewPassword(o) })
	},
}

var passphraseCmd = &cobra.Command{
	Use:   "passphrase",
	Short: "Generate passphrases of random words from the EFF list (12.9 bits a word)",
	Long: `passphrase prints --words random words from the EFF large word list
(7,776 words, CC BY 3.0 US), joined by --sep; --capitalize starts each
with a capital, --number puts a digit after one of them, for sites that
ask for both. The strength in bits goes to stderr.`,
	Example: "  codec secret passphrase\n  codec secret passphrase --words 5 --sep . --capitalize --number",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return printPasswords(func() (encode.Password, error) { return encode.NewPassphrase(phraseOpts) })
	},
}

func printPasswords(gen func() (encode.Password, error)) error {
	n := min(max(genCount, 1), 1000)
	lines := make([]string, 0, n)
	bits := 0
	for range n {
		p, err := gen()
		if err != nil {
			return err
		}
		lines = append(lines, p.Value)
		bits = p.Bits
	}
	fmt.Fprintf(os.Stderr, "%d bits each\n", bits)
	return emit(strings.Join(lines, "\n"))
}

func init() {
	passwordCmd.Flags().IntVarP(&pwOpts.Length, "length", "l", 20, fmt.Sprintf("characters (at most %d)", encode.MaxPassword))
	passwordCmd.Flags().StringSliceVar(&pwClasses, "classes", []string{"lower", "upper", "digits", "symbols"}, "kinds of character: lower, upper, digits, symbols")
	passwordCmd.Flags().IntVar(&pwMinEach, "min-each", 1, "at least this many of every class")
	passwordCmd.Flags().StringToIntVar(&pwMins, "min", nil, "per-class minimums, e.g. upper=2,digits=3")
	passwordCmd.Flags().StringVar(&pwOpts.Exclude, "exclude", "", "characters to leave out")
	passwordCmd.Flags().BoolVar(&pwOpts.NoAmbiguous, "no-ambiguous", false, "leave out I l 1 O 0 o")
	passphraseCmd.Flags().IntVarP(&phraseOpts.Words, "words", "w", 6, fmt.Sprintf("how many words (at most %d)", encode.MaxWords))
	passphraseCmd.Flags().StringVar(&phraseOpts.Separator, "sep", "-", "between words (at most 3 characters)")
	passphraseCmd.Flags().BoolVar(&phraseOpts.Capitalize, "capitalize", false, "start each word with a capital")
	passphraseCmd.Flags().BoolVar(&phraseOpts.Number, "number", false, "put a digit after one word")
	for _, c := range []*cobra.Command{passwordCmd, passphraseCmd} {
		c.Flags().IntVarP(&genCount, "count", "n", 1, "how many to print")
	}
	secretCmd.AddCommand(passwordCmd, passphraseCmd)
}
