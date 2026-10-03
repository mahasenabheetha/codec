package codec

import (
	"strings"
	"testing"
	"time"
)

func TestDetectUtility(t *testing.T) {
	now := time.Date(2026, 2, 3, 12, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		in, kind, tab, has string
	}{
		{"1770120309", "epoch", "timestamp", "2026-02-03T12:05:09Z"},
		{"1770120309123", "epoch", "timestamp", "milliseconds"},
		{"a%20b%26c", "url", "url", "a b&c"},
		{"q=shop+prod%2Feu", "url", "url", "q=shop prod/eu"},
		{"*/15 2-6 * * 1-5", "cron", "cron", "Every 15 minutes"},
		{"@daily", "cron", "cron", "At 00:00"},
		{"0 0 12 * * ?", "cron", "cron", "Quartz"},
	} {
		u, ok := DetectUtility(c.in, now)
		if !ok || u.Kind != c.kind || u.Tab != c.tab || !strings.Contains(u.Summary, c.has) {
			t.Errorf("%q: %+v %v", c.in, u, ok)
		}
	}
	// Left to the other detectors, or to nothing.
	for _, in := range []string{"12", "12345678", `{"url":"a%20b"}`, "aGVsbG8gd29ybGQ=", "1 2 3 4 5", "hello world", "100%", "@someone"} {
		if u, ok := DetectUtility(in, now); ok {
			t.Errorf("%q detected as %+v", in, u)
		}
	}
}
