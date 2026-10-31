package encode

import (
	_ "embed"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

// Passwords for people (3.8.0): rules a site or policy asks for, or a
// passphrase of random words. Every choice comes from crypto/rand
// through pick, which has no modulo bias. Nothing is stored.

//go:embed words/eff_large.txt
var effWords string

// wordList is the EFF large list (CC BY 3.0 US; see words/README.md).
var wordList = strings.Fields(effWords)

// MaxPassword bounds a password's length; MaxWords a passphrase's.
const (
	MaxPassword = 256
	MaxWords    = 20
)

// PasswordOptions are the rules for a password. A class is used when its
// minimum is 0 or more and left out when it is -1; Exclude takes
// characters out of every class.
type PasswordOptions struct {
	Length      int    `json:"length"`
	MinLower    int    `json:"minLower"`
	MinUpper    int    `json:"minUpper"`
	MinDigits   int    `json:"minDigits"`
	MinSymbols  int    `json:"minSymbols"`
	NoAmbiguous bool   `json:"noAmbiguous"` // leave out I l 1 O 0 o
	Exclude     string `json:"exclude"`
}

// PassphraseOptions are the rules for a passphrase.
type PassphraseOptions struct {
	Words      int    `json:"words"`
	Separator  string `json:"separator"`
	Capitalize bool   `json:"capitalize"` // every word
	Number     bool   `json:"number"`     // a digit after one random word
}

// Password is a generated password and its strength.
type Password struct {
	Value string `json:"value"`
	// Bits of entropy: how many guesses, as a power of two, an attacker
	// who knows the rules needs at worst.
	Bits int `json:"bits"`
}

// charClass is one kind of character a password may hold.
type charClass struct {
	name, set string
	min       int
}

// NewPassword makes a password that follows o: each class's minimum,
// the rest from every class in use, in a random order.
func NewPassword(o PasswordOptions) (Password, error) {
	if o.Length < 1 || o.Length > MaxPassword {
		return Password{}, fmt.Errorf("length must be 1 to %d", MaxPassword)
	}
	type class = charClass
	drop := o.Exclude
	if o.NoAmbiguous {
		drop += ambiguous
	}
	keep := func(set string) string {
		return strings.Map(func(r rune) rune {
			if strings.ContainsRune(drop, r) {
				return -1
			}
			return r
		}, set)
	}
	var classes []class
	need := 0
	for _, c := range []class{{"lower-case letters", lowerSet, o.MinLower}, {"capitals", upperSet, o.MinUpper}, {"digits", digitSet, o.MinDigits}, {"symbols", symbolSet, o.MinSymbols}} {
		if c.min < 0 {
			continue
		}
		c.set = keep(c.set)
		if c.set == "" {
			if c.min > 0 {
				return Password{}, fmt.Errorf("every one of the %s is excluded, but %d are required", c.name, c.min)
			}
			continue
		}
		need += c.min
		classes = append(classes, c)
	}
	if len(classes) == 0 {
		return Password{}, errors.New("choose at least one kind of character")
	}
	if need > o.Length {
		return Password{}, fmt.Errorf("the minimums add up to %d, more than the length %d", need, o.Length)
	}
	var all strings.Builder
	out := make([]byte, 0, o.Length)
	for _, c := range classes {
		all.WriteString(c.set)
		for range c.min {
			out = append(out, c.set[pick(len(c.set))])
		}
	}
	pool := all.String()
	for len(out) < o.Length {
		out = append(out, pool[pick(len(pool))])
	}
	p := perm(len(out))
	shuffled := make([]byte, len(out))
	for i, j := range p {
		shuffled[i] = out[j]
	}
	return Password{Value: string(shuffled), Bits: passwordBits(classes, o.Length)}, nil
}

// passwordBits counts the passwords the rules allow (inclusion and
// exclusion over the classes that must appear) and takes log2: exact
// for the rules as given, so minimums lower it a little, as they should.
func passwordBits(classes []charClass, length int) int {
	// With minimums of 0 or 1 the count is exact by inclusion–exclusion;
	// larger minimums are rare, and length·log2(pool) bounds them.
	total := 0
	for _, c := range classes {
		total += len(c.set)
	}
	for _, c := range classes {
		if c.min > 1 {
			return int(float64(length) * math.Log2(float64(total)))
		}
	}
	var required []int
	for _, c := range classes {
		if c.min == 1 {
			required = append(required, len(c.set))
		}
	}
	// Σ over subsets S of required classes: (−1)^|S| (total − size(S))^length,
	// in log space to stay in range.
	sum := 0.0
	n := len(required)
	for mask := 0; mask < 1<<n; mask++ {
		size, bits := 0, 0
		for i := range n {
			if mask&(1<<i) != 0 {
				size += required[i]
				bits++
			}
		}
		term := math.Pow(float64(total-size)/float64(total), float64(length))
		if bits%2 == 1 {
			term = -term
		}
		sum += term
	}
	if sum <= 0 {
		return 0
	}
	return int(float64(length)*math.Log2(float64(total)) + math.Log2(sum))
}

// NewPassphrase picks words from the EFF list (12.9 bits each).
func NewPassphrase(o PassphraseOptions) (Password, error) {
	if o.Words < 1 || o.Words > MaxWords {
		return Password{}, fmt.Errorf("words must be 1 to %d", MaxWords)
	}
	if utf8.RuneCountInString(o.Separator) > 3 {
		return Password{}, errors.New("the separator is at most 3 characters")
	}
	words := make([]string, o.Words)
	for i := range words {
		w := wordList[pick(len(wordList))]
		if o.Capitalize {
			w = strings.ToUpper(w[:1]) + w[1:]
		}
		words[i] = w
	}
	bits := float64(o.Words) * math.Log2(float64(len(wordList)))
	if o.Number {
		i := pick(o.Words)
		words[i] += string(digitSet[pick(10)])
		bits += math.Log2(10) + math.Log2(float64(o.Words))
	}
	return Password{Value: strings.Join(words, o.Separator), Bits: int(bits)}, nil
}

// WordCount is the size of the passphrase word list.
func WordCount() int { return len(wordList) }
