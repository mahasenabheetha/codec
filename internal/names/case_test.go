package names

import (
	"reflect"
	"strings"
	"testing"
)

func TestWords(t *testing.T) {
	for in, want := range map[string][]string{
		"getHTTPResponse":      {"get", "HTTP", "Response"},
		"HTTPServer":           {"HTTP", "Server"},
		"ipv6Address":          {"ipv6", "Address"},
		"user_id":              {"user", "id"},
		"MAX_RETRY_COUNT":      {"MAX", "RETRY", "COUNT"},
		"order-service.v2":     {"order", "service", "v2"},
		"  Shop API  gateway ": {"Shop", "API", "gateway"},
		"base64Encode":         {"base64", "Encode"},
		"v2API":                {"v2", "API"},
		"crème brûlée":         {"crème", "brûlée"},
		"":                     nil,
	} {
		if got := Words(in); !reflect.DeepEqual(got, want) {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

func TestCases(t *testing.T) {
	got := map[string]string{}
	for _, c := range Cases("user ID from HTTP request") {
		got[c.Name] = c.Value
	}
	want := map[string]string{
		"camelCase": "userIDFromHTTPRequest", "PascalCase": "UserIDFromHTTPRequest", "snake_case": "user_id_from_http_request",
		"kebab-case": "user-id-from-http-request", "CONSTANT_CASE": "USER_ID_FROM_HTTP_REQUEST", "dot.case": "user.id.from.http.request",
		"Title Case": "User ID From HTTP Request", "Sentence case": "User ID from HTTP request", "lower case": "user id from http request",
		"path/case": "user/id/from/http/request",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("cases:\n got %v\nwant %v", got, want)
	}
	// All upper case: no acronyms to keep.
	for _, c := range Cases("MAX_RETRY_COUNT") {
		if c.Name == "camelCase" && c.Value != "maxRetryCount" || c.Name == "PascalCase" && c.Value != "MaxRetryCount" {
			t.Errorf("constant input: %s = %s", c.Name, c.Value)
		}
	}
	if c := Cases(""); c[0].Value != "" || len(c) != 10 {
		t.Errorf("empty: %+v", c)
	}
}

func TestChecks(t *testing.T) {
	byRule := func(s string) map[string]Check {
		m := map[string]Check{}
		for _, c := range Checks(s) {
			m[c.Rule] = c
		}
		return m
	}
	ok := byRule("order-service")
	for rule, c := range ok {
		if rule != "Environment variable" && (!c.Valid || c.Fixed != "") {
			t.Errorf("order-service, %s: %+v", rule, c)
		}
	}
	// A dash doesn't work in a shell variable.
	if c := ok["Environment variable"]; c.Valid || c.Fixed != "ORDER_SERVICE" {
		t.Errorf("env order-service: %+v", c)
	}
	// Valid but lower case: the upper-case form is offered.
	if c := byRule("log_level")["Environment variable"]; !c.Valid || c.Fixed != "LOG_LEVEL" || len(c.Problems) != 1 {
		t.Errorf("env lower case: %+v", c)
	}

	bad := byRule("Order Service_v2!")
	if c := bad["Kubernetes name (DNS-1123 label)"]; c.Valid || c.Fixed != "order-service-v2" || len(c.Problems) != 2 {
		t.Errorf("label: %+v", c)
	}
	if c := bad["Environment variable"]; c.Valid || c.Fixed != "ORDER_SERVICE_V2" {
		t.Errorf("env: %+v", c)
	}
	if c := bad["Kubernetes label value"]; c.Valid || c.Fixed != "Order-Service_v2" {
		t.Errorf("label value: %+v", c)
	}

	long := strings.Repeat("abc-", 20) // 80 characters
	if c := byRule(long)["Helm release name"]; c.Valid || len(c.Fixed) > 53 || strings.HasSuffix(c.Fixed, "-") || !strings.Contains(c.Problems[0], "80 characters") {
		t.Errorf("helm: %+v", c)
	}
	if c := byRule("my.app")["Kubernetes name (DNS-1123 subdomain)"]; !c.Valid {
		t.Errorf("subdomain with a dot: %+v", c)
	}
	if c := byRule("my.app")["Kubernetes name (DNS-1123 label)"]; c.Valid {
		t.Errorf("label with a dot: %+v", c)
	}
	if c := byRule("9lives")["Environment variable"]; c.Valid || c.Fixed != "_9LIVES" {
		t.Errorf("env digit: %+v", c)
	}
	if c := byRule("LOG_LEVEL")["Environment variable"]; !c.Valid || len(c.Problems) != 0 || c.Fixed != "" {
		t.Errorf("env ok: %+v", c)
	}
	if c := byRule("")["Kubernetes label value"]; !c.Valid {
		t.Errorf("empty label value is allowed: %+v", c)
	}
}
