package codec

import (
	"fmt"
	"strings"
)

// JWT holds the two readable parts of a decoded token.
// The signature is raw bytes and cannot be "decoded" further without
// the signing key, so we keep it only to report its presence.
type JWT struct {
	Header    string // decoded JSON, pretty-printed
	Payload   string // decoded JSON, pretty-printed
	Signature string // base64url as-is, NOT decoded
}

// DecodeJWT splits a JWT into its three dot-separated segments and
// decodes the header and payload into readable JSON.
//
// It does NOT verify the signature. This is a deliberate limitation:
// this tool is for inspecting tokens you already have, not for
// authenticating them. Verification requires the signing key and
// belongs in an auth library, not a formatting tool.
func DecodeJWT(token string) (*JWT, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("expected 3 dot-separated segments, got %d", len(parts))
	}

	header, err := decodeSegment(parts[0])
	if err != nil {
		return nil, fmt.Errorf("header: %w", err)
	}

	payload, err := decodeSegment(parts[1])
	if err != nil {
		return nil, fmt.Errorf("payload: %w", err)
	}

	return &JWT{
		Header:    header,
		Payload:   payload,
		Signature: parts[2],
	}, nil
}

// decodeSegment base64url-decodes one JWT segment and pretty-prints
// the JSON inside it.
func decodeSegment(seg string) (string, error) {
	raw, err := Decode(seg, VariantURL)
	if err != nil {
		return "", err
	}

	pretty, err := PrettyJSON(string(raw), "")
	if err != nil {
		return "", fmt.Errorf("segment is not JSON: %w", err)
	}
	return pretty, nil
}
