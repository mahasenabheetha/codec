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

// Findings from the 2.3.0 review: what smart paste must leave alone.
func TestDetectUtilityReview(t *testing.T) {
	now := time.Date(2026, 2, 3, 12, 0, 0, 0, time.UTC)
	for _, in := range []string{
		"SELECT * FROM orders LIMIT 10",
		"What time is it now?",
		"is this a valid input?",
		"* Update the readme file",
		"%02x:%02x:%02x",
		"printf %10s",
		`{"a":"x%20y",}`, // broken JSON keeps its line and column error
		"TASK [web : fetch] ***\nok: [h] => {\"url\": \"https://x.io/a%2Fb\"}", // an Ansible log
		"0701234567",
	} {
		if u, ok := DetectUtility(in, now); ok {
			t.Errorf("%q detected as %s", in, u.Kind)
		}
	}
	// A CRON_TZ= prefix sets the zone of the runs.
	u, ok := DetectUtility("CRON_TZ=Europe/Stockholm 0 9 * * *", now)
	if !ok || !strings.Contains(u.Summary, "Next runs (Europe/Stockholm)") || !strings.Contains(u.Summary, "09:00") {
		t.Errorf("CRON_TZ: %+v", u)
	}
	if u, ok := DetectUtility("0 0 * * 7/2", now); !ok || u.Kind != "cron" {
		t.Errorf("7/2: %+v", u) // was a panic
	}
}
