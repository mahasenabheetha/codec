package sanitize

import (
	"strings"
	"testing"
)

func TestSanitize(t *testing.T) {
	for _, c := range []struct{ name, in, want string }{
		{"json secret", `{"user":"bob","password":"hunter2"}`, `{"user":"bob","password":"[secret-1]"}`},
		{"escaped json secret", `msg="{\"apiKey\":\"abc123\"}"`, `msg="{\"apiKey\":\"[secret-1]\"}"`},
		{"logfmt and query", `level=info db_password=s3cret url=/x?token=abc&page=2`, `level=info db_password=[secret-1] url=/x?token=[secret-2]&page=2`},
		{"connection string", `Server=db;User Id=sa;Password=P@ss;Encrypt=true`, `Server=db;User Id=sa;Password=[secret-1];Encrypt=true`},
		{"azure storage", `DefaultEndpointsProtocol=https;AccountName=acct;AccountKey=abc+def/ghi==;EndpointSuffix=core.windows.net`, `DefaultEndpointsProtocol=https;AccountName=acct;AccountKey=[secret-1];EndpointSuffix=core.windows.net`},
		{"yaml", "db:\n  password: hunter2\n  user: bob", "db:\n  password: [secret-1]\n  user: bob"},
		{"header", "Authorization: Bearer abcdefghijkl.mnop", "Authorization: Bearer [secret-1]"},
		{"url password", "postgres://app:pa55@10.1.2.3:5432/db", "postgres://app:[secret-1]@192.0.2.1:5432/db"},
		{"aws and github", "key AKIAIOSFODNN7EXAMPLE and ghp_" + strings.Repeat("a", 36), "key [secret-1] and [secret-2]"},
		{"jwt", "token eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0In0.abcdefghij", "token [secret-1]"},
		{"private key", "-----BEGIN RSA PRIVATE KEY-----\nMIIE\nabc\n-----END RSA PRIVATE KEY-----", "[secret-1]"},
		{"same value, same stand-in", "a 10.0.0.5 b 10.0.0.6 c 10.0.0.5", "a 192.0.2.1 b 192.0.2.2 c 192.0.2.1"},
		{"email", "mail Jane.Doe@contoso.com and jane.doe@contoso.com", "mail user-1@example.com and user-1@example.com"},
		{"host in a url", "GET https://api.contoso.internal/v1 from web-01.prod.contoso.com", "GET https://host-1.example.internal/v1 from host-2.example.internal"},
		{"uuid", "id 3f2504e0-4f89-11d3-9a0c-0305e82c3301 ok", "id 00000000-0000-0000-0000-000000000001 ok"},
		{"azure id", "/subscriptions/3f2504e0-4f89-11d3-9a0c-0305e82c3301/resourceGroups/rg-shop-prod/providers/Microsoft.KeyVault/vaults/kv-shop", "/subscriptions/00000000-0000-0000-0000-000000000001/resourceGroups/name-1/providers/Microsoft.KeyVault/vaults/name-2"},
		{"user paths", `/home/alice/app and C:\Users\Bob\src and C:\Users\Public`, `/home/user-1/app and C:\Users\user-2\src and C:\Users\Public`},
		{"ipv6", "from 2a01:111:f400::1 and ::1", "from 2001:db8::1 and ::1"},
		// What stays.
		{"not a secret field", `max_tokens=500 passed=3 key=name`, `max_tokens=500 passed=3 key=name`},
		{"templated values", `password: ${DB_PASSWORD}` + "\n" + `token: "{{ .Values.token }}"`, `password: ${DB_PASSWORD}` + "\n" + `token: "{{ .Values.token }}"`},
		{"files and code", "edit values.yaml and main.go; call fmt.Println", "edit values.yaml and main.go; call fmt.Println"},
		{"loopback, docs, versions", "127.0.0.1 0.0.0.0 192.0.2.7 v01.02.03.04 12:30:45", "127.0.0.1 0.0.0.0 192.0.2.7 v01.02.03.04 12:30:45"},
		{"public hosts", "pull ghcr.io/x and github.com/y; see example.com", "pull ghcr.io/x and github.com/y; see example.com"},
		{"stand-ins are left", "user-1@example.com host-1.example.internal [secret-1]", "user-1@example.com host-1.example.internal [secret-1]"},
	} {
		if got := Sanitize(c.in, All()).Text; got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.name, got, c.want)
		}
	}
}

func TestOptions(t *testing.T) {
	in := "acme-prod at 10.0.0.5 and 8.8.8.8 for ACME, pw=x1"
	o := All()
	o.KeepPrivate = true
	o.Words = []string{"acme"}
	r := Sanitize(in, o)
	// acme-prod: the word is whole before the -; 10.0.0.5 is private.
	if r.Text != "word-1-prod at 10.0.0.5 and 192.0.2.1 for word-1, pw=[secret-1]" {
		t.Errorf("words and private: %s", r.Text)
	}
	o.Mask = true
	if r := Sanitize(in, o); r.Text != "•••-prod at 10.0.0.5 and ••• for •••, pw=•••" {
		t.Errorf("mask: %s", r.Text)
	}
	o.Mask = false
	o.Keep = []string{"8.8.8.8"}
	r = Sanitize(in, o)
	if !strings.Contains(r.Text, "8.8.8.8") {
		t.Errorf("kept: %s", r.Text)
	}
	var kept bool
	for _, f := range r.Findings {
		if f.Value == "8.8.8.8" && f.Kept && f.Replacement == "8.8.8.8" {
			kept = true
		}
		if f.Value == "acme" && f.Count != 2 {
			t.Errorf("acme counted %d", f.Count)
		}
	}
	if !kept {
		t.Errorf("findings: %+v", r.Findings)
	}
	if r := Sanitize(in, Options{}); r.Text != in || len(r.Findings) != 0 {
		t.Errorf("nothing asked: %+v", r)
	}
}

func TestSecretName(t *testing.T) {
	for name, want := range map[string]bool{
		"password": true, "DB_PASSWORD": true, "accessToken": true, "x-auth-token": true, "AccountKey": true,
		"client_secret": true, "apiKey": true, "api_key": true, "Set-Cookie": true, "sig": true,
		"max_tokens": false, "passed": false, "key": false, "keyboard": false, "user": false, "tokenizer": false,
	} {
		if got := secretName(name); got != want {
			t.Errorf("secretName(%q) = %v", name, got)
		}
	}
}

func TestSecretsCount(t *testing.T) {
	if n := Secrets(`{"password":"a1"} Bearer abcdefghijk`); n != 2 {
		t.Errorf("%d", n)
	}
	if n := Secrets("nothing here"); n != 0 {
		t.Errorf("%d", n)
	}
}

func FuzzSanitize(f *testing.F) {
	for _, s := range []string{`{"password":"x"}`, "a=1 token=b", "10.0.0.1 ::1", "/home/a/b", "x@y.com", "-----BEGIN PRIVATE KEY-----"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		r := Sanitize(s, All())
		// What it gives back is already clean: a second pass changes nothing.
		if again := Sanitize(r.Text, All()); again.Text != r.Text {
			t.Fatalf("%q -> %q -> %q", s, r.Text, again.Text)
		}
	})
}
