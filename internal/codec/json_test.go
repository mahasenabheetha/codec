package codec

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestPrettyJSON(t *testing.T) {
	tests := []struct {
		name   string
		src    string
		indent string
		want   string
	}{
		{
			name: "flat object",
			src:  `{"a":1}`,
			want: "{\n  \"a\": 1\n}",
		},
		{
			name: "nested object and array",
			src:  `{"a":{"b":[1,2]}}`,
			want: "{\n  \"a\": {\n    \"b\": [\n      1,\n      2\n    ]\n  }\n}",
		},
		{
			name:   "tab indent",
			src:    `{"a":1}`,
			indent: "\t",
			want:   "{\n\t\"a\": 1\n}",
		},
		{
			name: "top level scalar is untouched",
			src:  `42`,
			want: `42`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PrettyJSON(tt.src, tt.indent)
			if err != nil {
				t.Fatalf("PrettyJSON(%q) returned error: %v", tt.src, err)
			}
			if got != tt.want {
				t.Errorf("PrettyJSON(%q)\n got: %q\nwant: %q", tt.src, got, tt.want)
			}
		})
	}
}

func TestMinifyJSON(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"spaces removed", `{ "a" : 1 }`, `{"a":1}`},
		{"newlines removed", "{\n  \"a\": 1\n}", `{"a":1}`},
		{"whitespace inside strings is preserved", `{"a":"b  c"}`, `{"a":"b  c"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MinifyJSON(tt.src)
			if err != nil {
				t.Fatalf("MinifyJSON(%q) returned error: %v", tt.src, err)
			}
			if got != tt.want {
				t.Errorf("MinifyJSON(%q) = %q, want %q", tt.src, got, tt.want)
			}
		})
	}
}

func TestValidateJSON(t *testing.T) {
	valid := []string{`{}`, `[]`, `{"a":[1,{"b":null}]}`, `"just a string"`, `true`}
	for _, src := range valid {
		if err := ValidateJSON(src); err != nil {
			t.Errorf("ValidateJSON(%q) = %v, want nil", src, err)
		}
	}

	invalid := []string{``, `{`, `{"a":}`, `{"a":1,}`, `{'a':1}`, `{"a":1} trailing`}
	for _, src := range invalid {
		if err := ValidateJSON(src); err == nil {
			t.Errorf("ValidateJSON(%q) = nil, want an error", src)
		}
	}
}

func TestSyntaxErrorPosition(t *testing.T) {
	src := "{\n  \"a\": 1,\n  \"b\": ,\n}"

	err := ValidateJSON(src)
	if err == nil {
		t.Fatal("expected an error for malformed JSON")
	}

	var syn *SyntaxError
	if !errors.As(err, &syn) {
		t.Fatalf("error is %T, want *codec.SyntaxError", err)
	}

	if syn.Line != 3 {
		t.Errorf("Line = %d, want 3", syn.Line)
	}
	if syn.Column != 8 {
		t.Errorf("Column = %d, want 8", syn.Column)
	}

	// The wrapped stdlib error must still be reachable.
	var stdErr *json.SyntaxError
	if !errors.As(err, &stdErr) {
		t.Error("wrapped *json.SyntaxError is not reachable via errors.As")
	}
}
