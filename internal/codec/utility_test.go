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
		{"{\"level\":\"info\",\"msg\":\"a\"}\n{\"level\":\"error\",\"msg\":\"b\"}", "log", "viewer", "A log: 2 entries · json 2\n1 error"},
		{"time=2026-10-22T10:00:00Z level=info msg=up\ntime=2026-10-22T10:00:01Z level=warn msg=slow\n", "log", "viewer", "logfmt 2"},
		{"pod-1234567890: 2026-10-22T10:00:00Z started\npod-1234567890: 2026-10-22T10:00:01Z ERROR boom", "log", "viewer", "text 2"},
	} {
		u, ok := DetectUtility(c.in, now)
		if !ok || u.Kind != c.kind || u.Tab != c.tab || !strings.Contains(u.Summary, c.has) {
			t.Errorf("%q: %+v %v", c.in, u, ok)
		}
	}
	// Left to the other detectors, or to nothing.
	for _, in := range []string{"12", "12345678", `{"url":"a%20b"}`, "aGVsbG8gd29ybGQ=", "1 2 3 4 5", "hello world", "100%", "@someone",
		"{\n  \"level\": \"info\",\n  \"msg\": \"one pretty object\"\n}", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: x\n", "first line\nsecond line\nthird"} {
		if u, ok := DetectUtility(in, now); ok {
			t.Errorf("%q detected as %+v", in, u)
		}
	}
}

func TestDetectNetwork(t *testing.T) {
	now := time.Date(2026, 2, 3, 12, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		in, kind, tab, has string
	}{
		{"10.0.0.0/16", "cidr", "cidr", "Hosts      65534"},
		{"2001:db8::/48", "cidr", "cidr", "documentation"},
		{"192.168.1.77/26", "cidr", "cidr", "192.168.1.64/26"},
		{"10.0.0.1", "ip", "ip", "Integer  167772161"},
		{"2001:db8::1", "ip", "ip", "ip6.arpa"},
		{"[::1]", "ip", "ip", "loopback"},
		{"curl -X POST https://x.io -d a=1", "curl", "http", `method: "POST"`},
		{"$ curl -k https://x.io", "curl", "http", "// note: fetch can't turn off certificate checks"},
		{"curl 'https://x.io' \\\n  -H 'Accept: application/json'", "curl", "http", `"Accept": "application/json"`},
	} {
		u, ok := DetectUtility(c.in, now)
		if !ok || u.Kind != c.kind || u.Tool != "network" || u.Tab != c.tab || !strings.Contains(u.Summary, c.has) {
			t.Errorf("%q: %+v %v", c.in, u, ok)
		}
	}
	// No guesses: numbers, versions, times and prose stay out.
	for _, in := range []string{"167772161", "1.2.3", "v1.2.3.4", "12:30", "10:00:00", "999.1.1.1", "10.0.0.0/40", "curl is a tool", "curls", "a:b", "C:/Users", "Version 1.2.3.4 is out"} {
		if u, ok := DetectUtility(in, now); ok && (u.Kind == "ip" || u.Kind == "cidr" || u.Kind == "curl") {
			t.Errorf("%q detected as %s", in, u.Kind)
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
		`{"a":"x%20y" "b"}`, // broken JSON keeps its line and column error
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
