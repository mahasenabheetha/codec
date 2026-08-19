package codec

import (
	"errors"
	"testing"
)

func TestApply(t *testing.T) {
	tests := []struct {
		name    string
		mode    Mode
		input   string
		opts    Options
		want    string
		wantErr bool
	}{
		{
			name:  "encode arbitrary text, not just JSON",
			mode:  ModeB64Encode,
			input: "my name is mahasen",
			want:  "bXkgbmFtZSBpcyBtYWhhc2Vu",
		},
		{
			name:  "encode with url-safe alphabet",
			mode:  ModeB64Encode,
			input: ">>>???",
			opts:  Options{Variant: VariantURL},
			want:  "Pj4-Pz8_",
		},
		{
			name:  "decode",
			mode:  ModeB64Decode,
			input: "aGVsbG8=",
			want:  "hello",
		},
		{
			name:    "explicit decode does not fall back to the other alphabet",
			mode:    ModeB64Decode,
			input:   "Pj4-Pz8_", // url-safe, but decoding with std
			wantErr: true,
		},
		{
			name:  "pretty",
			mode:  ModeJSONPretty,
			input: `{"a":1}`,
			want:  "{\n  \"a\": 1\n}",
		},
		{
			name:  "pretty with custom indent",
			mode:  ModeJSONPretty,
			input: `{"a":1}`,
			opts:  Options{Indent: "\t"},
			want:  "{\n\t\"a\": 1\n}",
		},
		{
			name:  "minify",
			mode:  ModeJSONMin,
			input: "{ \"a\" : 1 }",
			want:  `{"a":1}`,
		},
		{
			name:  "validate valid json",
			mode:  ModeValidate,
			input: `{"a":1}`,
			want:  "valid json",
		},
		{
			name:  "validate valid base64",
			mode:  ModeValidate,
			input: "aGVsbG8gd29ybGQ=",
			want:  "valid base64",
		},
		{
			name:    "validate garbage reports an error",
			mode:    ModeValidate,
			input:   `{"a":}`,
			wantErr: true,
		},
		{
			name:  "auto passes through to Transform",
			mode:  ModeAuto,
			input: `{"a":1}`,
			want:  "eyJhIjoxfQ==",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := Apply(tt.mode, tt.input, tt.opts)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Apply(%v, %q) succeeded, want an error", tt.mode, tt.input)
				}
				return
			}

			if err != nil {
				t.Fatalf("Apply(%v, %q) returned unexpected error: %v", tt.mode, tt.input, err)
			}
			if got != tt.want {
				t.Errorf("Apply(%v, %q) = %q, want %q", tt.mode, tt.input, got, tt.want)
			}
		})
	}
}

func TestApplyUnknownMode(t *testing.T) {
	_, _, err := Apply("banana", "anything", Options{})

	if err == nil {
		t.Fatal("Apply with unknown mode succeeded, want an error")
	}
	if !errors.Is(err, ErrUnknownMode) {
		t.Errorf("error = %v, want it to wrap ErrUnknownMode", err)
	}
}
