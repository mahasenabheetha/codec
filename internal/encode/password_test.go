package encode

import (
	"math"
	"strings"
	"testing"
	"unicode"
)

func count(s string, f func(rune) bool) int {
	n := 0
	for _, r := range s {
		if f(r) {
			n++
		}
	}
	return n
}

func TestNewPassword(t *testing.T) {
	sym := func(r rune) bool { return strings.ContainsRune(symbolSet, r) }
	for range 200 {
		p, err := NewPassword(PasswordOptions{Length: 14, MinLower: 1, MinUpper: 2, MinDigits: 3, MinSymbols: 1})
		if err != nil {
			t.Fatal(err)
		}
		v := p.Value
		if len(v) != 14 || count(v, unicode.IsLower) < 1 || count(v, unicode.IsUpper) < 2 || count(v, unicode.IsDigit) < 3 || count(v, sym) < 1 {
			t.Fatalf("rules broken: %q", v)
		}
	}
	// Classes left out, exclusions and look-alikes.
	for range 100 {
		p, err := NewPassword(PasswordOptions{Length: 30, MinLower: 0, MinUpper: -1, MinDigits: 0, MinSymbols: -1, NoAmbiguous: true, Exclude: "xyz"})
		if err != nil {
			t.Fatal(err)
		}
		if strings.ContainsAny(p.Value, "ABCXYZxyz!#Il1O0o") {
			t.Fatalf("excluded character in %q", p.Value)
		}
	}
	// A PIN: digits only.
	p, err := NewPassword(PasswordOptions{Length: 6, MinLower: -1, MinUpper: -1, MinDigits: 6, MinSymbols: -1})
	if err != nil || count(p.Value, unicode.IsDigit) != 6 || p.Bits != 19 {
		t.Errorf("PIN: %+v %v", p, err)
	}
	for _, o := range []PasswordOptions{
		{Length: 0},
		{Length: 4, MinLower: 2, MinUpper: 2, MinDigits: 1},
		{Length: 8, MinLower: -1, MinUpper: -1, MinDigits: -1, MinSymbols: -1},
		{Length: 8, MinLower: -1, MinUpper: -1, MinDigits: 2, MinSymbols: -1, Exclude: "0123456789"},
	} {
		if _, err := NewPassword(o); err == nil {
			t.Errorf("%+v accepted", o)
		}
	}
}

func TestPasswordBits(t *testing.T) {
	// No minimums: length · log2(pool). 16 from 90 characters ≈ 103.9.
	p, _ := NewPassword(PasswordOptions{Length: 16})
	if p.Bits != 103 {
		t.Errorf("no minimums: %d bits", p.Bits)
	}
	// One of each of four classes costs a little: under 103.9, over 102.
	p, _ = NewPassword(PasswordOptions{Length: 16, MinLower: 1, MinUpper: 1, MinDigits: 1, MinSymbols: 1})
	if p.Bits < 102 || p.Bits > 103 {
		t.Errorf("one of each: %d bits", p.Bits)
	}
	// Exact for a tiny case: length 2 from {a,b} ∪ {0,1}, one of each:
	// ab-style strings with one letter and one digit = 2·2·2 = 8 → 3 bits.
	bits := passwordBits([]charClass{{set: "ab", min: 1}, {set: "01", min: 1}}, 2)
	if bits != 3 {
		t.Errorf("tiny case: %d bits, want 3", bits)
	}
}

func TestNewPassphrase(t *testing.T) {
	if WordCount() != 7776 {
		t.Fatalf("word list has %d words", WordCount())
	}
	p, err := NewPassphrase(PassphraseOptions{Words: 6, Separator: "-"})
	if err != nil || len(strings.Split(p.Value, "-")) < 6 || p.Bits != int(6*math.Log2(7776)) {
		t.Errorf("six words: %+v %v", p, err)
	}
	p, err = NewPassphrase(PassphraseOptions{Words: 4, Separator: " ", Capitalize: true, Number: true})
	words := strings.Fields(p.Value)
	if err != nil || len(words) != 4 || count(p.Value, unicode.IsDigit) != 1 {
		t.Fatalf("capitalised with a number: %q %v", p.Value, err)
	}
	for _, w := range words {
		if !unicode.IsUpper([]rune(w)[0]) {
			t.Errorf("not capitalised: %q", w)
		}
	}
	for _, o := range []PassphraseOptions{{Words: 0}, {Words: 21}, {Words: 3, Separator: "----"}} {
		if _, err := NewPassphrase(o); err == nil {
			t.Errorf("%+v accepted", o)
		}
	}
}
