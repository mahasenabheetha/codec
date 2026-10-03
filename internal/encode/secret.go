package encode

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
)

// Character sets for text secrets. Symbols leave out quotes, the
// backslash, the backtick and space, so a secret pastes into YAML, JSON
// and a shell without escaping.
const (
	lowerSet  = "abcdefghijklmnopqrstuvwxyz"
	upperSet  = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitSet  = "0123456789"
	symbolSet = "!#$%&()*+,-./:;<=>?@[]^_{|}~"
	ambiguous = "Il1O0o"
)

// MaxSecret bounds a secret's length, in characters or bytes.
const MaxSecret = 1024

// SecretOptions says what to generate. Format "text" draws Length
// characters from the chosen sets; "hex", "base64" and "base64url"
// encode Length random bytes.
type SecretOptions struct {
	Length      int    `json:"length"`
	Format      string `json:"format"`
	Lower       bool   `json:"lower"`
	Upper       bool   `json:"upper"`
	Digits      bool   `json:"digits"`
	Symbols     bool   `json:"symbols"`
	NoAmbiguous bool   `json:"noAmbiguous"` // leave out I l 1 O 0 o
}

// Secret is a generated value and how hard it is to guess.
type Secret struct {
	Value string `json:"value"`
	Bits  int    `json:"bits"` // entropy
}

// NewSecret generates a random secret from crypto/rand. A text secret
// has at least one character of each chosen set when it is long enough.
func NewSecret(o SecretOptions) (Secret, error) {
	if o.Length < 1 || o.Length > MaxSecret {
		return Secret{}, fmt.Errorf("length must be 1 to %d", MaxSecret)
	}
	switch o.Format {
	case "hex", "base64", "base64url":
		b := make([]byte, o.Length)
		rand.Read(b)
		v := hex.EncodeToString(b)
		if o.Format == "base64" {
			v = base64.StdEncoding.EncodeToString(b)
		} else if o.Format == "base64url" {
			v = base64.RawURLEncoding.EncodeToString(b)
		}
		return Secret{Value: v, Bits: 8 * o.Length}, nil
	case "", "text":
	default:
		return Secret{}, fmt.Errorf("unknown format %q (text, hex, base64, base64url)", o.Format)
	}
	var sets []string
	for _, s := range []struct {
		on  bool
		set string
	}{{o.Lower, lowerSet}, {o.Upper, upperSet}, {o.Digits, digitSet}, {o.Symbols, symbolSet}} {
		if !s.on {
			continue
		}
		if o.NoAmbiguous {
			s.set = strings.Map(func(r rune) rune {
				if strings.ContainsRune(ambiguous, r) {
					return -1
				}
				return r
			}, s.set)
		}
		sets = append(sets, s.set)
	}
	if len(sets) == 0 {
		return Secret{}, errors.New("choose at least one character set")
	}
	all := strings.Join(sets, "")
	out := make([]byte, o.Length)
	for i := range out {
		out[i] = all[pick(len(all))]
	}
	// One of each set, at distinct random places.
	if o.Length >= len(sets) {
		places := perm(o.Length)
		for i, s := range sets {
			out[places[i]] = s[pick(len(s))]
		}
	}
	return Secret{Value: string(out), Bits: int(float64(o.Length) * math.Log2(float64(len(all))))}, nil
}

// UUID returns a random (version 4) UUID.
func UUID() string {
	var b [16]byte
	rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40 // version 4
	b[8] = b[8]&0x3f | 0x80 // RFC 9562 variant
	h := hex.EncodeToString(b[:])
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}

// pick is a uniform random index below n.
func pick(n int) int {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic(err) // crypto/rand doesn't fail on supported systems
	}
	return int(v.Int64())
}

// perm is a random permutation of 0..n-1 (Fisher–Yates).
func perm(n int) []int {
	p := make([]int, n)
	for i := range p {
		p[i] = i
	}
	for i := n - 1; i > 0; i-- {
		j := pick(i + 1)
		p[i], p[j] = p[j], p[i]
	}
	return p
}
