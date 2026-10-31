package names

import "testing"

// FuzzChecks: a suggested name passes the rule it was suggested for,
// and the cases of any text are written without panicking.
func FuzzChecks(f *testing.F) {
	for _, s := range []string{"Order Service_v2!", "getHTTPResponse", "9lives", "crème brûlée", "-a-", "a..b", "", "ÀÉÎ_ô", "x" + string(rune(0x1F600))} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		Cases(s)
		for _, c := range Checks(s) {
			if c.Fixed == "" {
				continue
			}
			for _, again := range Checks(c.Fixed) {
				if again.Rule == c.Rule && !again.Valid {
					t.Fatalf("%q: %s suggested %q, which breaks it: %v", s, c.Rule, c.Fixed, again.Problems)
				}
			}
		}
	})
}
