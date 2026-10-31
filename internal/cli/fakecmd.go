package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mahasenabheetha/codec/v2/internal/fake"
)

// codec fake (3.8.0): lorem ipsum and rows of made-up data. The seed
// goes to stderr so a run can be repeated with --seed.

var (
	fakeSeed   uint64
	fakeCount  int
	fakeFormat string
	fakeTable  string
	loremUnit  string
	loremPlain bool
)

var fakeCmd = &cobra.Command{
	Use:   "fake",
	Short: "Make placeholder text and rows of made-up test data",
}

func seedOrNew() uint64 {
	if fakeSeed != 0 {
		return fakeSeed
	}
	s := fake.NewSeed()
	fmt.Fprintf(os.Stderr, "seed %d\n", s)
	return s
}

var fakeLoremCmd = &cobra.Command{
	Use:   "lorem",
	Short: "Lorem ipsum: --count words, sentences or paragraphs",
	Example: `  codec fake lorem
  codec fake lorem --unit words --count 12 --plain`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		out, err := fake.Lorem(loremUnit, fakeCount, seedOrNew(), !loremPlain)
		if err != nil {
			return err
		}
		return emit(out)
	},
}

var fakeRowsCmd = &cobra.Command{
	Use:   "rows <columns|preset>",
	Short: "Rows of made-up data as JSON, NDJSON, CSV, YAML or SQL",
	Long: `rows makes --count rows from columns written as name:kind[:args],
comma-separated, or a preset (users, orders, servers). integer, number,
date and datetime take :min:max (number a third, decimals); choice takes
values split by |; ?N after a column leaves N% of it blank. Names,
emails and usernames in a row belong to the same made-up person; emails
use example.com/.org/.net, IPs the documentation ranges, phones 555-01xx.

Kinds: ` + kindList() + `.`,
	Example: `  codec fake rows users --count 5
  codec fake rows "id:id,name:name,email:email,age:integer:18:90,plan:choice:free|pro" --format csv
  codec fake rows orders --format sql --table orders --seed 42`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cols, err := fake.ParseSpec(args[0])
		if err != nil {
			return err
		}
		out, err := fake.Write(cols, fakeCount, seedOrNew(), fakeFormat, fakeTable)
		if err != nil {
			return err
		}
		return emit(strings.TrimRight(out, "\n"))
	},
}

func kindList() string {
	names := make([]string, len(fake.Kinds))
	for i, k := range fake.Kinds {
		names[i] = k.Kind
	}
	return strings.Join(names, ", ")
}

func init() {
	for _, c := range []*cobra.Command{fakeLoremCmd, fakeRowsCmd} {
		c.Flags().Uint64Var(&fakeSeed, "seed", 0, "repeat a run (the seed of a run goes to stderr)")
	}
	fakeLoremCmd.Flags().StringVar(&loremUnit, "unit", "paragraphs", "words, sentences or paragraphs")
	fakeLoremCmd.Flags().IntVarP(&fakeCount, "count", "n", 3, "how many")
	fakeLoremCmd.Flags().BoolVar(&loremPlain, "plain", false, `don't start with "Lorem ipsum dolor sit amet"`)
	fakeRowsCmd.Flags().IntVarP(&fakeCount, "count", "n", 10, fmt.Sprintf("rows (at most %d)", fake.MaxRows))
	fakeRowsCmd.Flags().StringVarP(&fakeFormat, "format", "f", "json", "json, ndjson, csv, yaml or sql")
	fakeRowsCmd.Flags().StringVar(&fakeTable, "table", "items", "the table name for sql")
	fakeCmd.AddCommand(fakeLoremCmd, fakeRowsCmd)
	rootCmd.AddCommand(fakeCmd)
}
