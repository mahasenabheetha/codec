package cron

import (
	"testing"
	"time"
)

// FuzzParse: any expression is read or refused, and a schedule read is
// described and its next runs found quickly, even one that never runs.
func FuzzParse(f *testing.F) {
	for _, s := range []string{"*/5 * * * *", "0 9 * * MON-FRI", "@hourly", "0 0 31 2 *", "0 0 1 1 * 2027", "CRON_TZ=Europe/Oslo 0 3 * * *", "0 12 L * ?", "1-59/7 2,4 */3 JAN-MAR SUN#2"} {
		f.Add(s)
	}
	from := time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC)
	f.Fuzz(func(t *testing.T, s string) {
		sc, err := Parse(s)
		if err != nil {
			return
		}
		sc.Describe()
		sc.Notes()
		start := time.Now()
		runs, _ := sc.Next(from, 5, time.UTC)
		if d := time.Since(start); d > 2*time.Second {
			t.Fatalf("%q: next runs took %v", s, d)
		}
		for i, r := range runs {
			if !r.After(from) || i > 0 && !r.After(runs[i-1]) {
				t.Fatalf("%q: runs out of order: %v", s, runs)
			}
		}
	})
}
