package codec

import (
	"bytes"
	"testing"
)

// FuzzTransform: smart paste takes anything — detection, the obvious
// transformation, pretty JSON and Ansible task output never fail hard.
func FuzzTransform(f *testing.F) {
	for _, s := range []string{
		`{"a":[1,2,{"b":null}]}`, "aGVsbG8gd29ybGQ=", "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.c2ln",
		"TASK [x] ***\nfatal: [h]: FAILED! => {\"msg\": \"boom\"}\n", "1729500000", "a%20b%2Fc", "*/5 * * * *",
		`{"level":"info","msg":"hi"}` + "\n" + `{"level":"error","msg":"no"}`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		Detect(s)
		Transform(s)
		PrettyJSON(s, "  ")
		ParseAnsible(s)
	})
}

// FuzzBase64: what is encoded decodes back, in both alphabets.
func FuzzBase64(f *testing.F) {
	f.Add([]byte("hello"), "aGVsbG8")
	f.Fuzz(func(t *testing.T, data []byte, s string) {
		for _, v := range []Variant{VariantStd, VariantURL} {
			back, err := Decode(Encode(data, v), v)
			if err != nil || !bytes.Equal(back, data) {
				t.Fatalf("variant %d: %q came back as %q (%v)", v, data, back, err)
			}
			Decode(s, v)
		}
	})
}
