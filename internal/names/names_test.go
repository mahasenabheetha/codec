package names

import "testing"

func TestClosest(t *testing.T) {
	cands := []string{"build", "test", "deploy", "UHA Precheck - (PROD)"}
	for _, tc := range []struct{ in, want string }{
		{"biuld", "build"}, // a swap is one edit
		{"tset", "test"},   // short names allow one edit only
		{"deplyo", "deploy"},
		{"UHA DR Precheck - (PROD)", "UHA Precheck - (PROD)"},
		{"build", ""}, // never itself
		{"lint", ""},  // too far from everything
	} {
		if got := Closest(tc.in, cands); got != tc.want {
			t.Errorf("Closest(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
