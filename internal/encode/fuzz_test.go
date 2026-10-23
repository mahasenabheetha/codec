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

func FuzzParseURL(f *testing.F) {
	for _, s := range []string{"https://u:p@host:8080/a/b?x=1&y=%20#frag", "/relative?q", "mailto:a@b", "http://[::1]:80/", "%zz"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		ParseURL(s)
	})
}
