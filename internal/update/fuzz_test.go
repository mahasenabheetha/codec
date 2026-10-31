package update

import "testing"

// FuzzVersions checks the version parser and order on any input: no
// panics, an order that is antisymmetric and reflexive, and a release
// newer than its own pre-releases.
func FuzzVersions(f *testing.F) {
	for _, s := range [][2]string{
		{"v3.6.0", "v3.6.1"}, {"v1.0.0-alpha.1", "v1.0.0-alpha.beta"}, {"1.2.3+build", "v1.2.3"},
		{"v10.0.0-rc.11", "v10.0.0-rc.2"}, {"dev", "v0.0.0"}, {"v1.2.3-", "v01.2.3"},
	} {
		f.Add(s[0], s[1])
	}
	f.Fuzz(func(t *testing.T, x, y string) {
		a, okA := parse(x)
		b, okB := parse(y)
		if !okA || !okB {
			return
		}
		if compare(a, a) != 0 {
			t.Fatalf("compare(%q, itself) != 0", x)
		}
		if compare(a, b) != -compare(b, a) {
			t.Fatalf("compare(%q, %q) = %d but the reverse is %d", x, y, compare(a, b), compare(b, a))
		}
		if len(a.pre) > 0 {
			release := semver{nums: a.nums}
			if compare(a, release) >= 0 {
				t.Fatalf("pre-release %q not before its release", x)
			}
		}
	})
}
