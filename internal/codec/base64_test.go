package codec

import "testing"

func TestEncode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		variant Variant
		want    string
	}{
		{"empty input", "", VariantStd, ""},
		{"simple word", "hello", VariantStd, "aGVsbG8="},
		{"one pad char", "hi", VariantStd, "aGk="},
		{"std alphabet uses +/", "\xfb\xff", VariantStd, "+/8="},
		{"url alphabet uses -_", "\xfb\xff", VariantURL, "-_8="},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Encode([]byte(tt.input), tt.variant)
			if got != tt.want {
				t.Errorf("Encode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDecode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		variant Variant
		want    string
		wantErr bool
	}{
		{"round trip", "aGVsbG8=", VariantStd, "hello", false},
		{"missing padding is tolerated", "aGVsbG8", VariantStd, "hello", false},
		{"whitespace is stripped", "aGVs\n bG8=", VariantStd, "hello", false},
		{"url variant", "-_8=", VariantURL, "\xfb\xff", false},
		{"invalid characters", "!!!!", VariantStd, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decode(tt.input, tt.variant)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("Decode(%q) succeeded, want an error", tt.input)
				}
				return
			}

			if err != nil {
				t.Fatalf("Decode(%q) returned unexpected error: %v", tt.input, err)
			}
			if string(got) != tt.want {
				t.Errorf("Decode(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestRoundTrip checks the property that decoding an encoded value
// always returns the original bytes, for both variants.
func TestRoundTrip(t *testing.T) {
	inputs := []string{"", "a", "ab", "abc", "abcd", `{"kind":"Secret"}`, "日本語"}

	for _, variant := range []Variant{VariantStd, VariantURL} {
		for _, in := range inputs {
			encoded := Encode([]byte(in), variant)
			decoded, err := Decode(encoded, variant)
			if err != nil {
				t.Fatalf("round trip of %q failed: %v", in, err)
			}
			if string(decoded) != in {
				t.Errorf("round trip of %q gave %q", in, decoded)
			}
		}
	}
}
