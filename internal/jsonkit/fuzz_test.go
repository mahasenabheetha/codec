package jsonkit

import "testing"

// FuzzRoundTrips: whatever Parse accepts (JSON, JSONC or JSON5) writes
// back as strict JSON that reads to the same value; a value has no diff
// with itself; flattening and unflattening gives it back; sorting keys
// twice is sorting once.
func FuzzRoundTrips(f *testing.F) {
	for _, s := range []string{
		`{"a": [1, 2.5, -3e2, "x"], "b": null, "c": true}`,
		"{a: 'b', // c\n d: [0x1F, .5, +1,],}",
		`{"app.kubernetes.io/name": "api", "": {"]": "\""}}`,
		`[[], {}, [[]], {"x": {}}]`,
		`"\ud83d\ude00 \u2028"`,
		`12345678901234567890123`,
		`{"a": 1, "a": 2}`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		v, _, err := Parse(src)
		if err != nil {
			return
		}
		out := Encode(v, "")
		w, d, err := Parse(out)
		if err != nil {
			t.Fatalf("Encode wrote what Parse rejects: %q from %q: %v", out, src, err)
		}
		if d != JSON {
			t.Fatalf("Encode wrote %s, not strict JSON: %q", d, out)
		}
		if Encode(w, "  ") != Encode(v, "  ") {
			t.Fatalf("round trip changed %q", src)
		}
		if diff := Diff(v, w); len(diff) != 0 {
			t.Fatalf("diff with itself: %+v", diff)
		}
		if !hugeArray(v) {
			back, err := Unflatten(FlattenText(v))
			if err != nil {
				t.Fatalf("Unflatten(FlattenText(%q)): %v", out, err)
			}
			if Encode(back, "") != out {
				t.Fatalf("flatten round trip: %q became %q", out, Encode(back, ""))
			}
		}
		once := Encode(SortKeys(v), "")
		if Encode(SortKeys(SortKeys(v)), "") != once {
			t.Fatalf("SortKeys not stable for %q", out)
		}
	})
}

// hugeArray: an array longer than a flattened path may index.
func hugeArray(v *Value) bool {
	if len(v.Items) > maxIndex {
		return true
	}
	for _, it := range v.Items {
		if hugeArray(it) {
			return true
		}
	}
	for _, m := range v.Members {
		if hugeArray(m.Value) {
			return true
		}
	}
	return false
}
