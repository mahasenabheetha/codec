package codec

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// DefaultIndent is used by PrettyJSON when no indent is supplied.
const DefaultIndent = "  "

// SyntaxError reports malformed JSON at a human-readable position.
// The standard library reports a byte offset, which is useless when you
// are staring at a 400-line payload; we translate it to line and column.
type SyntaxError struct {
	Line   int
	Column int
	Err    error // the underlying *json.SyntaxError
}

// Error makes SyntaxError satisfy the built-in error interface.
func (e *SyntaxError) Error() string {
	return fmt.Sprintf("invalid JSON at line %d, column %d: %v", e.Line, e.Column, e.Err)
}

// Unwrap lets errors.Is and errors.As reach the wrapped stdlib error.
func (e *SyntaxError) Unwrap() error { return e.Err }

// PrettyJSON reformats src with newlines and indentation.
// An empty indent means DefaultIndent.
func PrettyJSON(src, indent string) (string, error) {
	if indent == "" {
		indent = DefaultIndent
	}

	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(src), "", indent); err != nil {
		return "", locate(src, err)
	}
	return buf.String(), nil
}

// MinifyJSON removes all insignificant whitespace.
func MinifyJSON(src string) (string, error) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(src)); err != nil {
		return "", locate(src, err)
	}
	return buf.String(), nil
}

// ValidateJSON reports whether src is well-formed JSON.
// It returns nil when valid, or a *SyntaxError describing the problem.
func ValidateJSON(src string) error {
	_, err := MinifyJSON(src)
	return err
}

// locate upgrades a stdlib JSON error into our position-aware SyntaxError.
// Errors of any other kind are passed through untouched.
func locate(src string, err error) error {
	var syn *json.SyntaxError
	if !errors.As(err, &syn) {
		return err
	}

	// json.Compact and json.Indent often report Offset as 0, which is
	// useless. json.Unmarshal's validity scan reports real offsets, so
	// when the offset looks broken we re-scan the input just to get a
	// correctly positioned error.
	if syn.Offset == 0 {
		var rescan *json.SyntaxError
		if err2 := json.Unmarshal([]byte(src), new(json.RawMessage)); errors.As(err2, &rescan) {
			syn = rescan
		}
	}

	// Offset counts bytes consumed *including* the offending byte,
	// so the character itself sits one index earlier.
	idx := int(syn.Offset) - 1
	if idx < 0 {
		idx = 0
	}

	line, col := lineColumn(src, idx)
	return &SyntaxError{Line: line, Column: col, Err: syn}
}

// lineColumn converts a byte index into 1-based line and column numbers.
func lineColumn(src string, idx int) (line, col int) {
	line, col = 1, 1
	for i, r := range src {
		if i >= idx {
			break
		}
		if r == '\n' {
			line++
			col = 1
			continue
		}
		col++
	}
	return line, col
}
