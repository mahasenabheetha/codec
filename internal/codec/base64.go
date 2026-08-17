// Package codec provides the core transformations used by every codec
// front-end: base64, JSON, and JWT. It has no knowledge of CLIs, HTTP,
// or user interfaces — it only transforms data.
package codec

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// Variant selects which base64 alphabet to use.
type Variant int

const (
	// VariantStd is standard base64 (RFC 4648 §4): alphabet A-Za-z0-9+/
	VariantStd Variant = iota
	// VariantURL is URL-safe base64 (RFC 4648 §5): alphabet A-Za-z0-9-_
	// This is what JWTs and most URL query parameters use.
	VariantURL
)

// encoding maps a Variant to the stdlib encoder that implements it.
func (v Variant) encoding() *base64.Encoding {
	switch v {
	case VariantURL:
		return base64.URLEncoding
	default:
		return base64.StdEncoding
	}
}

// Encode returns the base64 representation of data, always with padding.
func Encode(data []byte, v Variant) string {
	return v.encoding().EncodeToString(data)
}

// Decode parses s as base64 and returns the raw bytes.
//
// It is deliberately lenient about input: surrounding whitespace and
// newlines are stripped, and missing '=' padding is restored. Real-world
// base64 arrives wrapped across lines (Kubernetes secrets) or unpadded
// (JWT segments), and a tool that rejects those is annoying to use.
func Decode(s string, v Variant) ([]byte, error) {
	out, err := v.encoding().DecodeString(normalize(s))
	if err != nil {
		return nil, fmt.Errorf("decode base64: %w", err)
	}
	return out, nil
}

// normalize strips whitespace and restores missing padding.
func normalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r':
			continue
		}
		b.WriteRune(r)
	}

	s = b.String()
	if rem := len(s) % 4; rem != 0 {
		s += strings.Repeat("=", 4-rem)
	}
	return s
}
