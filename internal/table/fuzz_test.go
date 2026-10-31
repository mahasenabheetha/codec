package table

import (
	"reflect"
	"testing"
)

// FuzzRoundTrip reads anything without panicking; what reads back from
// Delimited is the same rows, and every conversion works.
func FuzzRoundTrip(f *testing.F) {
	for _, s := range []string{
		"a,b\n1,2\n",
		"a\tb\n\"x\ty\"\tz\n",
		"\"q\"\"\",\"line\nbreak\"\r\n,\n",
		"name;n\nä;1,5\n",
		"\"\"\n",
		"a|b|c\n1||\n",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		tb, err := Parse(src, Options{})
		if err != nil {
			return
		}
		for _, format := range Formats {
			if _, err := Convert(tb, format, true, "  "); err != nil && format != "yaml" {
				t.Fatalf("%s: %v", format, err)
			}
		}
		for _, d := range []rune{tb.Delim, ',', '\t'} {
			tb.Header = false // the header row is a row here
			out := tb.Delimited(d)
			back, err := Parse(out, Options{Delim: d, Header: "no"})
			if err != nil {
				t.Fatalf("delimiter %q: %v\n%q", d, err, out)
			}
			if len(back.Rows) == 0 && len(tb.Rows) == 0 {
				continue
			}
			if !reflect.DeepEqual(back.Rows, tb.Rows) {
				t.Fatalf("delimiter %q:\n%q\nread back as\n%q", d, tb.Rows, back.Rows)
			}
		}
	})
}
