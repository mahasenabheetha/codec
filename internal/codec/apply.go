package codec

import (
	"errors"
	"fmt"
)

// Mode names one explicit transformation. Unlike Variant and Kind
// (int enums), Mode is string-based because its values cross a
// serialization boundary: they arrive verbatim in JSON from UIs.
// A string enum needs no int<->string mapping table at that edge.
type Mode string

const (
	ModeAuto       Mode = "auto"
	ModeB64Encode  Mode = "b64-encode"
	ModeB64Decode  Mode = "b64-decode"
	ModeJSONPretty Mode = "json-pretty"
	ModeJSONMin    Mode = "json-min"
	ModeValidate   Mode = "validate"
	ModeJWT        Mode = "jwt"
)

// ErrUnknownMode is returned when Apply receives a mode it does not
// recognize. Callers can test for it with errors.Is to distinguish
// "caller sent nonsense" from "input could not be transformed".
var ErrUnknownMode = errors.New("unknown mode")

// Options carries the knobs that modify how a mode behaves. New
// options can be added as fields without changing Apply's signature,
// and the zero value is always a sensible default: standard base64
// alphabet, default indentation.
type Options struct {
	Variant Variant // base64 alphabet for the b64 modes
	Indent  string  // indentation for json-pretty; "" means default
}

// Apply runs one explicit transformation. It is the single dispatch
// point shared by every UI (web, and later the desktop app), so the
// meaning of a mode is defined exactly once.
//
// The returned Kind is only meaningful for ModeAuto and ModeValidate,
// where the input's type is discovered rather than stated.
func Apply(mode Mode, input string, opts Options) (string, Kind, error) {
	switch mode {
	case ModeAuto:
		return Transform(input)

	case ModeB64Encode:
		// Encodes the input byte-for-byte, whatever it is. No
		// detection, no trimming: explicit mode means the user's
		// bytes are taken literally.
		return Encode([]byte(input), opts.Variant), KindUnknown, nil

	case ModeB64Decode:
		// Explicit mode also means an explicit alphabet — no
		// fallback guessing like Transform does.
		raw, err := Decode(input, opts.Variant)
		if err != nil {
			return "", KindUnknown, err
		}
		return string(raw), KindUnknown, nil

	case ModeJSONPretty:
		out, err := PrettyJSON(input, opts.Indent)
		return out, KindJSON, err

	case ModeJSONMin:
		out, err := MinifyJSON(input)
		return out, KindJSON, err

	case ModeValidate:
		kind := Detect(input)
		if kind != KindUnknown {
			return "valid " + kind.String(), kind, nil
		}
		// Nothing recognized. Report the JSON validation error:
		// JSON is the most common intent and its error carries a
		// line/column position, which is the most useful diagnostic.
		if err := ValidateJSON(input); err != nil {
			return "", KindUnknown, err
		}
		return "", KindUnknown, fmt.Errorf("input is not JSON, base64, or a JWT")

	case ModeJWT:
		jwt, err := DecodeJWT(input)
		if err != nil {
			return "", KindJWT, err
		}
		return formatJWT(jwt), KindJWT, nil

	default:
		return "", KindUnknown, fmt.Errorf("%w: %q", ErrUnknownMode, mode)
	}
}
