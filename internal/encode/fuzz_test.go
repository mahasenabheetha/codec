package encode

import (
	"bytes"
	"testing"
)

// FuzzHex: hex encoded with any separator and case decodes back.
func FuzzHex(f *testing.F) {
	f.Add([]byte("hi"), false, ":", "0x4a 6b")
	f.Fuzz(func(t *testing.T, data []byte, upper bool, sep, s string) {
		for _, sp := range []string{"", " ", ":"} {
			back, err := HexDecode(HexEncode(data, upper, sp))
			if err != nil || !bytes.Equal(back, data) {
				t.Fatalf("sep %q: %q came back as %q (%v)", sp, data, back, err)
			}
		}
		HexEncode(data, upper, sep)
		HexDecode(s)
		Printable(data)
	})
}

// FuzzBuildURL: query pairs of any text survive a build and a parse.
func FuzzBuildURL(f *testing.F) {
	f.Add("q", "a b&c=d", "tag", "")
	f.Add("", "=", "%zz", "+")
	f.Fuzz(func(t *testing.T, k1, v1, k2, v2 string) {
		in := []Param{{k1, v1}, {k2, v2}}
		s, err := BuildURL(URLParts{Query: in})
		if err != nil {
			t.Fatal(err)
		}
		p, err := ParseURL(s)
		if err != nil || len(p.Query) != 2 || p.Query[0] != in[0] || p.Query[1] != in[1] {
			t.Fatalf("%+v built as %q, parsed as %+v (%v)", in, s, p, err)
		}
	})
}

func FuzzParseURL(f *testing.F) {
	for _, s := range []string{"https://u:p@host:8080/a/b?x=1&y=%20#frag", "/relative?q", "mailto:a@b", "http://[::1]:80/", "%zz"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		ParseURL(s)
	})
}
