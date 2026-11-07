package sanitize

import (
	"fmt"
	"strings"
	"testing"
)

// A Writer gives what Sanitize gives for the whole text, though it
// sanitizes a chunk at a time: the same stand-ins throughout, and a
// private key that straddles a chunk taken out whole.
func TestWriterMatchesSanitize(t *testing.T) {
	var b strings.Builder
	for i := 0; b.Len() < chunk-200; i++ {
		fmt.Fprintf(&b, "%d level=info ip=10.0.%d.%d user=jane@contoso.com token=t%d\n", i, i/250%250, i%250, i%7)
	}
	b.WriteString("-----BEGIN RSA PRIVATE KEY-----\n")
	b.WriteString(strings.Repeat("MIIEowIBAAKCAQEA0123456789abcdef\n", 40))
	b.WriteString("-----END RSA PRIVATE KEY-----\nafter ip=10.0.0.0 user=jane@contoso.com\n")
	text := b.String()

	var out strings.Builder
	w := NewWriter(&out, All())
	for p := text; p != ""; {
		n := min(len(p), 4096)
		if _, err := w.Write([]byte(p[:n])); err != nil {
			t.Fatal(err)
		}
		p = p[n:]
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	want := Sanitize(text, All())
	if out.String() != want.Text {
		t.Fatal("the streamed text differs from Sanitize's")
	}
	if strings.Contains(out.String(), "MIIE") {
		t.Error("part of the private key is left")
	}
	if len(w.Findings()) != len(want.Findings) {
		t.Errorf("%d findings streamed, %d at once", len(w.Findings()), len(want.Findings))
	}
}
