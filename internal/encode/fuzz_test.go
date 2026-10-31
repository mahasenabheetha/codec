package encode

import (
	"bytes"
	"strings"
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

// FuzzParseExpected: any pasted text either fails or gives digests of a
// supported size, in lower-case hex.
func FuzzParseExpected(f *testing.F) {
	for _, s := range []string{
		"ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad  a.tar.gz",
		"SHA256 (a) = ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		"ungWv48Bz+pBQUDeXa4iI7ADYaOWF3qctBD/YfIAFa0=",
		"# c\n900150983cd24fb0d6963f7d28e17f72 *b.zip",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		es, err := ParseExpected(s)
		if err != nil {
			return
		}
		for _, e := range es {
			if byLength[len(e.Hex)/2] != e.Algorithm || strings.ToLower(e.Hex) != e.Hex {
				t.Fatalf("%q gave %+v", s, e)
			}
		}
	})
}
