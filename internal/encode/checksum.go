package encode

import (
	"bufio"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"regexp"
	"strings"
)

// Checksum digests a stream with every algorithm at once (or the named
// ones), reading it once without keeping it: a file of any size costs
// a buffer. It returns the digests and how many bytes were read.
func Checksum(r io.Reader, names ...string) ([]Digest, int64, error) {
	if _, err := Hash(nil, nil, names...); err != nil { // checks the names
		return nil, 0, err
	}
	want := map[string]bool{}
	for _, n := range names {
		want[fold(n)] = true
	}
	var hs []hash.Hash
	var used []algorithm
	var ws []io.Writer
	for _, a := range algorithms {
		if len(want) > 0 && !want[fold(a.name)] {
			continue
		}
		h := a.new()
		hs, used, ws = append(hs, h), append(used, a), append(ws, h)
	}
	n, err := io.Copy(io.MultiWriter(ws...), r)
	if err != nil {
		return nil, n, err
	}
	out := make([]Digest, len(hs))
	for i, h := range hs {
		sum := h.Sum(nil)
		out[i] = Digest{Algorithm: used[i].name, Hex: hex.EncodeToString(sum), Base64: base64.StdEncoding.EncodeToString(sum), Weak: used[i].weak}
	}
	return out, n, nil
}

// Expected is a checksum to compare with: its hex, the algorithm its
// length (or a BSD line) says, and the file it names, if any.
type Expected struct {
	Hex       string `json:"hex"`
	Algorithm string `json:"algorithm"`
	Name      string `json:"name,omitempty"`
	Line      int    `json:"line"`
}

// byLength names the algorithm of a digest from its size in bytes.
var byLength = map[int]string{16: "MD5", 20: "SHA-1", 32: "SHA-256", 48: "SHA-384", 64: "SHA-512"}

var (
	// sha256sum and friends: "<hex>  name", or "<hex> *name" in binary mode.
	gnuLine = regexp.MustCompile(`^\\?([0-9A-Fa-f]{32,128}) [ *](.+)$`)
	// BSD and openssl: "SHA256 (name) = <hex>" or "SHA2-256(name)= <hex>".
	bsdLine = regexp.MustCompile(`^([A-Za-z0-9-]+) ?\((.+)\) ?= ?([0-9A-Fa-f]{32,128})$`)
)

// ParseExpected reads checksums to compare with: one value pasted as
// hex (any case, spaces or colons between bytes), as base64, or a
// checksums file with one line per file in the sha256sum or BSD form.
// Blank lines and # comments are skipped.
func ParseExpected(text string) ([]Expected, error) {
	var out []Expected
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	line := 0
	for sc.Scan() {
		line++
		s := strings.TrimSpace(sc.Text())
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		e, err := parseExpectedLine(s)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		e.Line = line
		out = append(out, e)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no checksum found")
	}
	return out, nil
}

func parseExpectedLine(s string) (Expected, error) {
	if m := gnuLine.FindStringSubmatch(s); m != nil {
		return digestOf(m[1], m[2], "")
	}
	if m := bsdLine.FindStringSubmatch(s); m != nil {
		return digestOf(m[3], m[2], m[1])
	}
	// One value: hex with optional separators, else base64.
	compact := strings.NewReplacer(" ", "", ":", "", "-", "").Replace(s)
	if b, err := hex.DecodeString(compact); err == nil {
		return digestFrom(b, "", "")
	}
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(s); err == nil {
			return digestFrom(b, "", "")
		}
	}
	return Expected{}, fmt.Errorf("not a checksum: give hex, base64, or a line such as \"<hex>  file.tar.gz\"")
}

func digestOf(hx, name, label string) (Expected, error) {
	b, err := hex.DecodeString(hx)
	if err != nil {
		return Expected{}, fmt.Errorf("odd hex length")
	}
	return digestFrom(b, name, label)
}

// digestFrom checks the size against the algorithms, and the BSD label
// (SHA256, SHA2-256, MD5…) against the size when there is one.
func digestFrom(b []byte, name, label string) (Expected, error) {
	alg, ok := byLength[len(b)]
	if !ok {
		return Expected{}, fmt.Errorf("a %d-byte digest matches no supported algorithm (MD5, SHA-1, SHA-256, SHA-384, SHA-512)", len(b))
	}
	if label != "" {
		l := strings.ToLower(strings.ReplaceAll(label, "-", ""))
		if strings.HasPrefix(l, "sha2") && len(l) == 7 { // openssl's SHA2-256
			l = "sha" + l[4:]
		}
		if l != fold(alg) {
			return Expected{}, fmt.Errorf("the line says %s, but the digest has the size of %s", label, alg)
		}
	}
	return Expected{Hex: hex.EncodeToString(b), Algorithm: alg, Name: name}, nil
}

// Match says whether digests (from Checksum) contain e.
func (e Expected) Match(digests []Digest) (found, equal bool) {
	for _, d := range digests {
		if d.Algorithm == e.Algorithm {
			return true, d.Hex == e.Hex
		}
	}
	return false, false
}
