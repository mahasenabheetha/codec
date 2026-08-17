package codec

import (
	"fmt"
	"strings"
)

// Kind classifies what a piece of input looks like.
type Kind int

const (
	KindUnknown Kind = iota
	KindJSON         // valid JSON
	KindJWT          // three base64url segments, first two decode to JSON
	KindBase64       // valid base64 (and not one of the above)
)

// String returns a human-readable name for the Kind.
func (k Kind) String() string {
	switch k {
	case KindJSON:
		return "json"
	case KindJWT:
		return "jwt"
	case KindBase64:
		return "base64"
	default:
		return "unknown"
	}
}

// Detect guesses what s is. Order matters: every JWT is also valid
// base64url, and a bare number like "12" is both valid JSON and valid
// base64 — so we check from most specific to least specific.
func Detect(s string) Kind {
	s = strings.TrimSpace(s)
	if s == "" {
		return KindUnknown
	}

	if _, err := DecodeJWT(s); err == nil {
		return KindJWT
	}

	if ValidateJSON(s) == nil {
		return KindJSON
	}

	if isLikelyBase64(s) {
		return KindBase64
	}

	return KindUnknown
}

// Transform applies the "obvious" transformation for the detected kind:
// JSON is encoded to base64, base64 is decoded (and pretty-printed when
// the result is JSON), JWTs are expanded. It returns the result and the
// kind it detected, so callers can tell the user what happened.
func Transform(s string) (string, Kind, error) {
	kind := Detect(s)

	switch kind {
	case KindJWT:
		jwt, err := DecodeJWT(s)
		if err != nil {
			return "", kind, err
		}
		return "header:\n" + jwt.Header + "\n\npayload:\n" + jwt.Payload, kind, nil

	case KindJSON:
		return Encode([]byte(strings.TrimSpace(s)), VariantStd), kind, nil

	case KindBase64:
		// Detection accepts both alphabets, so decoding must too:
		// try standard first, fall back to URL-safe.
		raw, err := Decode(s, VariantStd)
		if err != nil {
			raw, err = Decode(s, VariantURL)
		}
		if err != nil {
			return "", kind, err
		}
		// If the decoded bytes are themselves JSON, pretty-print them:
		// this is the k8s-secret-containing-JSON case.
		if pretty, jerr := PrettyJSON(string(raw), ""); jerr == nil {
			return pretty, kind, nil
		}
		return string(raw), kind, nil

	default:
		return "", kind, fmt.Errorf("input is not JSON, base64, or a JWT")
	}
}

// isLikelyBase64 reports whether s both decodes successfully and looks
// like base64 was the *intent* — at least 8 chars long and valid. The
// length floor exists because short plain words ("test", "hello") are
// coincidentally valid base64, and misdetecting them is worse than
// asking the user to be explicit.
func isLikelyBase64(s string) bool {
	if len(s) < 8 {
		return false
	}
	if _, err := Decode(s, VariantStd); err == nil {
		return true
	}
	_, err := Decode(s, VariantURL)
	return err == nil
}
