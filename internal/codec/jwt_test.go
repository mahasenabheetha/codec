package codec

import (
	"strings"
	"testing"
)

// buildJWT assembles an unsigned test token from two JSON strings.
// Helpers like this keep test cases readable: the alternative is
// pasting opaque base64 blobs nobody can review.
func buildJWT(t *testing.T, headerJSON, payloadJSON string) string {
	t.Helper()
	h := Encode([]byte(headerJSON), VariantURL)
	p := Encode([]byte(payloadJSON), VariantURL)
	return h + "." + p + ".fakesignature"
}

func TestDecodeJWT(t *testing.T) {
	token := buildJWT(t, `{"alg":"HS256","typ":"JWT"}`, `{"sub":"mahasen","admin":true}`)

	jwt, err := DecodeJWT(token)
	if err != nil {
		t.Fatalf("DecodeJWT returned error: %v", err)
	}

	if !strings.Contains(jwt.Header, `"alg": "HS256"`) {
		t.Errorf("Header does not contain expected claim:\n%s", jwt.Header)
	}
	if !strings.Contains(jwt.Payload, `"sub": "mahasen"`) {
		t.Errorf("Payload does not contain expected claim:\n%s", jwt.Payload)
	}
	if jwt.Signature != "fakesignature" {
		t.Errorf("Signature = %q, want %q", jwt.Signature, "fakesignature")
	}
}

func TestDecodeJWTErrors(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"empty string", ""},
		{"two segments", "aaaa.bbbb"},
		{"four segments", "a.b.c.d"},
		{"header is not base64", "!!!!.eyJhIjoxfQ.sig"},
		{"payload is not JSON", "eyJhIjoxfQ." + Encode([]byte("not json"), VariantURL) + ".sig"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := DecodeJWT(tt.token); err == nil {
				t.Errorf("DecodeJWT(%q) succeeded, want an error", tt.token)
			}
		})
	}
}

func TestDetect(t *testing.T) {
	jwt := buildJWT(t, `{"alg":"none"}`, `{"sub":"x"}`)

	tests := []struct {
		name  string
		input string
		want  Kind
	}{
		{"json object", `{"a":1}`, KindJSON},
		{"json array", `[1,2,3]`, KindJSON},
		{"json with surrounding space", "  {\"a\":1}\n", KindJSON},
		{"jwt", jwt, KindJWT},
		{"base64 of text", "aGVsbG8gd29ybGQ=", KindBase64},
		{"empty", "", KindUnknown},
		{"plain sentence", "not any of these!", KindUnknown},
		{"short word is not base64", "test", KindUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Detect(tt.input); got != tt.want {
				t.Errorf("Detect(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestTransform(t *testing.T) {
	t.Run("json becomes base64", func(t *testing.T) {
		out, kind, err := Transform(`{"a":1}`)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if kind != KindJSON {
			t.Errorf("kind = %v, want KindJSON", kind)
		}
		if out != "eyJhIjoxfQ==" {
			t.Errorf("out = %q, want %q", out, "eyJhIjoxfQ==")
		}
	})

	t.Run("base64 of json is decoded and prettified", func(t *testing.T) {
		out, kind, err := Transform("eyJhIjoxfQ==")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if kind != KindBase64 {
			t.Errorf("kind = %v, want KindBase64", kind)
		}
		want := "{\n  \"a\": 1\n}"
		if out != want {
			t.Errorf("out = %q, want %q", out, want)
		}
	})

	t.Run("round trip: transform of transform returns original", func(t *testing.T) {
		original := `{"a":1}`
		encoded, _, err := Transform(original)
		if err != nil {
			t.Fatalf("first transform failed: %v", err)
		}
		back, _, err := Transform(encoded)
		if err != nil {
			t.Fatalf("second transform failed: %v", err)
		}
		if minified, _ := MinifyJSON(back); minified != original {
			t.Errorf("round trip gave %q, want %q", minified, original)
		}
	})

	t.Run("url-safe base64 is decoded too", func(t *testing.T) {
		// ">>>???" encodes to "Pj4-Pz8_" in the URL alphabet, which is
		// invalid in the standard alphabet — this exercises the fallback.
		enc := Encode([]byte(">>>???"), VariantURL)
		out, kind, err := Transform(enc)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if kind != KindBase64 {
			t.Errorf("kind = %v, want KindBase64", kind)
		}
		if out != ">>>???" {
			t.Errorf("out = %q, want %q", out, ">>>???")
		}
	})

	t.Run("unknown input is an error", func(t *testing.T) {
		_, kind, err := Transform("???")
		if err == nil {
			t.Fatal("expected an error for unrecognizable input")
		}
		if kind != KindUnknown {
			t.Errorf("kind = %v, want KindUnknown", kind)
		}
	})
}
