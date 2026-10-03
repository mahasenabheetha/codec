package encode

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"
	"slices"
	"strings"
)

// Digest is one algorithm's result, in the two forms tools print.
type Digest struct {
	Algorithm string `json:"algorithm"`
	Hex       string `json:"hex"`
	Base64    string `json:"base64"`
	// Weak marks algorithms broken for security use: fine for
	// checksums, never for passwords or signatures.
	Weak bool `json:"weak,omitempty"`
}

type algorithm struct {
	name string
	new  func() hash.Hash
	weak bool
}

var algorithms = []algorithm{
	{"SHA-256", sha256.New, false},
	{"SHA-384", sha512.New384, false},
	{"SHA-512", sha512.New, false},
	{"SHA-1", sha1.New, true},
	{"MD5", md5.New, true},
}

// Algorithms lists the names Hash accepts, strongest first.
func Algorithms() []string {
	out := make([]string, len(algorithms))
	for i, a := range algorithms {
		out[i] = a.name
	}
	return out
}

// Hash digests data with each named algorithm (all when names is
// empty), or computes HMACs when key is not nil. Names match without
// case or dash ("sha256", "SHA-256").
func Hash(data, key []byte, names ...string) ([]Digest, error) {
	want := make([]string, len(names))
	for i, n := range names {
		want[i] = fold(n)
	}
	for i, w := range want {
		if !slices.ContainsFunc(algorithms, func(a algorithm) bool {
			return fold(a.name) == w
		}) {
			return nil, fmt.Errorf("unknown algorithm %q (have %s)", names[i], strings.Join(Algorithms(), ", "))
		}
	}
	var out []Digest
	for _, a := range algorithms {
		if len(want) > 0 && !slices.Contains(want, fold(a.name)) {
			continue
		}
		var h hash.Hash
		if key != nil {
			h = hmac.New(a.new, key)
		} else {
			h = a.new()
		}
		h.Write(data)
		sum := h.Sum(nil)
		out = append(out, Digest{Algorithm: a.name, Hex: hex.EncodeToString(sum), Base64: base64.StdEncoding.EncodeToString(sum), Weak: a.weak})
	}
	return out, nil
}

func fold(name string) string {
	return strings.ToLower(strings.ReplaceAll(name, "-", ""))
}
