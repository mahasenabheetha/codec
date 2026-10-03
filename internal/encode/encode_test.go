package encode

import (
	"math"
	"regexp"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestURL(t *testing.T) {
	for _, c := range []struct {
		in, want    string
		whole, plus bool
	}{
		{"a b&c=d/é~", "a%20b%26c%3Dd%2F%C3%A9~", false, false},
		{"a b", "a+b", false, true},
		{"https://x.io/a b?q=1&r=é#top", "https://x.io/a%20b?q=1&r=%C3%A9#top", true, false},
		{"https://x.io/a%20b c", "https://x.io/a%20b%20c", true, false}, // existing escapes kept
		{"100%", "100%25", true, false},
	} {
		if got := URLEncode(c.in, c.whole, c.plus); got != c.want {
			t.Errorf("URLEncode(%q, %v, %v) = %q, want %q", c.in, c.whole, c.plus, got, c.want)
		}
		if back, err := URLDecode(c.want, c.plus); err != nil || back != c.in && !strings.Contains(c.in, "%20") {
			t.Errorf("URLDecode(%q) = %q, %v", c.want, back, err)
		}
	}
	if got, _ := URLDecode("a+b", false); got != "a+b" {
		t.Errorf("plus kept without form decoding: %q", got)
	}
	if _, err := URLDecode("50%zz", false); err == nil || !strings.Contains(err.Error(), "character 3") {
		t.Errorf("bad escape: %v", err)
	}
}

func TestParseURL(t *testing.T) {
	p, err := ParseURL("https://bob:secret@x.io:8443/api/v1?tag=a&tag=b+c&empty=&flag#frag")
	if err != nil {
		t.Fatal(err)
	}
	if p.Scheme != "https" || p.User != "bob" || p.Host != "x.io" || p.Port != "8443" || p.Path != "/api/v1" || p.Fragment != "frag" {
		t.Errorf("parts: %+v", p)
	}
	want := []Param{{"tag", "a"}, {"tag", "b c"}, {"empty", ""}, {"flag", ""}}
	if len(p.Query) != len(want) {
		t.Fatalf("query: %+v", p.Query)
	}
	for i := range want {
		if p.Query[i] != want[i] {
			t.Errorf("param %d: %+v, want %+v", i, p.Query[i], want[i])
		}
	}
	if q, _ := ParseURL("?a=1&b=%2F"); q.Host != "" || len(q.Query) != 2 || q.Query[1].Value != "/" {
		t.Errorf("bare query: %+v", q)
	}
}

func TestHex(t *testing.T) {
	if got := HexEncode([]byte("Hi!"), false, ""); got != "486921" {
		t.Errorf("encode: %q", got)
	}
	if got := HexEncode([]byte{0xde, 0xad}, true, ":"); got != "DE:AD" {
		t.Errorf("upper with colons: %q", got)
	}
	for _, in := range []string{"486921", "48 69 21", "48:69:21", "0x48 0x69 0x21", `\x48\x69\x21`, "48-69-21\n"} {
		if b, err := HexDecode(in); err != nil || string(b) != "Hi!" {
			t.Errorf("decode %q: %q %v", in, b, err)
		}
	}
	if b, err := HexDecode("100x"); err == nil {
		t.Errorf("a 0 inside a byte isn't a prefix: %q", b)
	}
	if _, err := HexDecode("486"); err == nil || !strings.Contains(err.Error(), "odd") {
		t.Errorf("odd: %v", err)
	}
	if _, err := HexDecode("48g9"); err == nil || !strings.Contains(err.Error(), "character 3") {
		t.Errorf("bad digit: %v", err)
	}
	if s, ok := Printable([]byte{'a', 0xff, 'b'}); ok || s != `a\xffb` {
		t.Errorf("printable: %q %v", s, ok)
	}
}

// Published test vectors: FIPS 180 ("abc"), RFC 4231 and RFC 2202
// test case 2 for HMAC.
func TestHashVectors(t *testing.T) {
	plain := map[string]string{
		"SHA-256": "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		"SHA-384": "cb00753f45a35e8bb5a03d699ac65007272c32ab0eded1631a8b605a43ff5bed8086072ba1e7cc2358baeca134c825a7",
		"SHA-512": "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f",
		"SHA-1":   "a9993e364706816aba3e25717850c26c9cd0d89d",
		"MD5":     "900150983cd24fb0d6963f7d28e17f72",
	}
	mac := map[string]string{
		"SHA-256": "5bdcc146bf60754e6a042426089575c75a003f089d2739839dec58b964ec3843",
		"SHA-384": "af45d2e376484031617f78d2b58a6b1b9c7ef464f5a01b47e42ec3736322445e8e2240ca5e69e2c78b3239ecfab21649",
		"SHA-512": "164b7a7bfcf819e2e395fbe73b56e0a387bd64222e831fd610270cd7ea2505549758bf75c05a994a6d034f65f8f0e6fdcaeab1a34d4a6b4b636e070a38bce737",
		"SHA-1":   "effcdf6ae5eb2fa2d27416d5f184df9c259a7c79",
		"MD5":     "750c783e6ab0b503eaa86e310a5db738",
	}
	check := func(name string, got []Digest, want map[string]string) {
		if len(got) != len(want) {
			t.Fatalf("%s: %d digests", name, len(got))
		}
		for _, d := range got {
			if d.Hex != want[d.Algorithm] {
				t.Errorf("%s %s = %s", name, d.Algorithm, d.Hex)
			}
			if d.Weak != (d.Algorithm == "MD5" || d.Algorithm == "SHA-1") {
				t.Errorf("%s weak = %v", d.Algorithm, d.Weak)
			}
		}
	}
	d, _ := Hash([]byte("abc"), nil)
	check("hash", d, plain)
	d, _ = Hash([]byte("what do ya want for nothing?"), []byte("Jefe"))
	check("hmac", d, mac)

	d, err := Hash([]byte("abc"), nil, "sha256", "md5")
	if err != nil || len(d) != 2 || d[0].Base64 != "ungWv48Bz+pBQUDeXa4iI7ADYaOWF3qctBD/YfIAFa0=" {
		t.Errorf("chosen: %+v %v", d, err)
	}
	if _, err := Hash(nil, nil, "sha3"); err == nil || !strings.Contains(err.Error(), `"sha3"`) {
		t.Errorf("unknown: %v", err)
	}
}

func TestSecret(t *testing.T) {
	s, err := NewSecret(SecretOptions{Length: 4, Lower: true, Upper: true, Digits: true, Symbols: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, set := range []string{lowerSet, upperSet, digitSet, symbolSet} {
		if !strings.ContainsAny(s.Value, set) {
			t.Errorf("%q has none of %q", s.Value, set)
		}
	}
	s, _ = NewSecret(SecretOptions{Length: 200, Lower: true, Upper: true, Digits: true, NoAmbiguous: true})
	if strings.ContainsAny(s.Value, ambiguous) || len(s.Value) != 200 {
		t.Errorf("ambiguous left in: %q", s.Value)
	}
	if want := int(200 * math.Log2(56)); s.Bits != want { // 62 letters and digits, 6 ambiguous
		t.Errorf("bits %d, want %d", s.Bits, want)
	}
	if s, _ := NewSecret(SecretOptions{Length: 32, Format: "hex"}); len(s.Value) != 64 || s.Bits != 256 {
		t.Errorf("hex: %+v", s)
	}
	if s, _ := NewSecret(SecretOptions{Length: 32, Format: "base64url"}); len(s.Value) != 43 || strings.ContainsAny(s.Value, "+/=") {
		t.Errorf("base64url: %+v", s)
	}
	for _, o := range []SecretOptions{{Length: 0, Lower: true}, {Length: 8}, {Length: 8, Format: "rot13"}} {
		if _, err := NewSecret(o); err == nil {
			t.Errorf("%+v: no error", o)
		}
	}
}

func TestUUID(t *testing.T) {
	v4 := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	seen := map[string]bool{}
	for range 100 {
		u := UUID()
		if !v4.MatchString(u) || seen[u] {
			t.Fatalf("UUID %q", u)
		}
		seen[u] = true
	}
}

func TestHtpasswd(t *testing.T) {
	line, err := Htpasswd("admin", "s3cret", MinCost)
	if err != nil {
		t.Fatal(err)
	}
	user, hash, _ := strings.Cut(line, ":")
	if user != "admin" || !strings.HasPrefix(hash, "$2y$04$") {
		t.Fatalf("line %q", line)
	}
	// The hash verifies with bcrypt itself, and through the checker.
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte("s3cret")) != nil {
		t.Error("bcrypt rejects its own hash")
	}
	if ok, err := HtpasswdCheck(line, "s3cret"); !ok || err != nil {
		t.Errorf("check: %v %v", ok, err)
	}
	if ok, err := HtpasswdCheck(hash, "wrong"); ok || err != nil {
		t.Errorf("wrong password: %v %v", ok, err)
	}
	if _, err := HtpasswdCheck("bob:$apr1$abc$def", "x"); err == nil || !strings.Contains(err.Error(), "only bcrypt") {
		t.Errorf("apr1: %v", err)
	}
	for _, c := range []struct{ user, pass string }{{"", "x"}, {"a:b", "x"}, {"a", ""}, {"a", strings.Repeat("x", 73)}} {
		if _, err := Htpasswd(c.user, c.pass, MinCost); err == nil {
			t.Errorf("%q/%d chars: no error", c.user, len(c.pass))
		}
	}
	if _, err := Htpasswd("a", "b", 31); err == nil {
		t.Error("cost 31 accepted")
	}
}
