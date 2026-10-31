package jsonkit

import (
	"errors"
	"fmt"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// ToYAML writes v as block-style YAML, keeping key order (yamlkit).
func ToYAML(v *Value) (string, error) {
	out, err := yamlkit.FromJSON([]byte(Encode(v, "")))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// FromYAML reads YAML (anchors and aliases resolved, key order kept) and
// returns it as a JSON value. Several documents become an array of them.
func FromYAML(src string) (*Value, error) {
	js, err := yamlkit.ToJSON(yamlkit.Parse([]byte(src)), "")
	if err != nil {
		var ye *yamlkit.Error
		if errors.As(err, &ye) {
			return nil, &SyntaxError{Line: ye.Range.Start.Line, Column: ye.Range.Start.Col, Msg: ye.Message}
		}
		return nil, err
	}
	v, _, err := Parse(string(js))
	if err != nil {
		return nil, fmt.Errorf("this YAML has no JSON form: %w", err)
	}
	return v, nil
}
