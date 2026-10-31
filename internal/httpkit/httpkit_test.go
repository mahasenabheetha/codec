package httpkit

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplitShell(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want []string
	}{
		{`curl 'a b' "c \"d\" \$e" f\ g`, []string{"curl", "a b", `c "d" $e`, "f g"}},
		{"curl \\\n  -H 'x: 1' \\\n  url", []string{"curl", "-H", "x: 1", "url"}},
		{"curl ^\n  -X POST ^\n  url", []string{"curl", "-X", "POST", "url"}},
		{"curl `\r\n  url", []string{"curl", "url"}},
		{`curl $'a\nb\x41\u00e9\'' ''`, []string{"curl", "a\nbAé'", ""}},
		{"curl url # comment\n", []string{"curl", "url"}},
		{"curl url | jq .", []string{"curl", "url"}},
		{`curl "it's" 'say "hi"'`, []string{"curl", "it's", `say "hi"`}},
	} {
		got, _, err := splitShell(tt.in)
		if err != nil || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q:\n got %q (%v)\nwant %q", tt.in, got, err, tt.want)
		}
	}
	for _, bad := range []string{`curl 'open`, `curl "open`, `curl $'open`} {
		if _, _, err := splitShell(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestParseCurl(t *testing.T) {
	// As Chrome's "Copy as cURL (bash)" writes it.
	r, err := ParseCurl(`curl 'https://api.example.com/v1/orders?page=2' \
  -H 'accept: application/json' \
  -H 'authorization: Bearer abc.def' \
  -H 'content-type: application/json' \
  --data-raw '{"item":"book","qty":2}' \
  --compressed`)
	want := &Request{Method: "POST", URL: "https://api.example.com/v1/orders?page=2", Headers: []Header{
		{"accept", "application/json"}, {"authorization", "Bearer abc.def"}, {"content-type", "application/json"},
	}, Body: `{"item":"book","qty":2}`}
	if err != nil || !reflect.DeepEqual(r, want) {
		t.Fatalf("chrome:\n got %+v (%v)\nwant %+v", r, err, want)
	}

	for _, tt := range []struct {
		name, in string
		check    func(*Request) bool
	}{
		{"-d is a form post", `curl -d name=a -d b=2 example.com/x`, func(r *Request) bool {
			return r.Method == "POST" && r.Body == "name=a&b=2" && r.URL == "http://example.com/x" && r.Headers[0] == Header{"Content-Type", "application/x-www-form-urlencoded"} && len(r.Notes) == 1
		}},
		{"-G puts data in the query", `curl -G https://x.io/s?a=1 --data-urlencode 'q=a b&c'`, func(r *Request) bool {
			return r.Method == "GET" && r.URL == "https://x.io/s?a=1&q=a+b%26c" && r.Body == ""
		}},
		{"--json", `curl --json '{"a":1}' https://x.io`, func(r *Request) bool {
			v, _ := r.header("content-type")
			a, _ := r.header("Accept")
			return r.Method == "POST" && v == "application/json" && a == "application/json"
		}},
		{"short flags together", `curl -sSLk -XPUT -H'X-A: 1' https://x.io`, func(r *Request) bool {
			return r.Method == "PUT" && r.Headers[0] == Header{"X-A", "1"} && len(r.Notes) == 1 && strings.Contains(r.Notes[0], "certificate")
		}},
		{"-u", `curl -u admin:s3cret https://x.io`, func(r *Request) bool {
			return r.Headers[0] == Header{"Authorization", "Basic YWRtaW46czNjcmV0"}
		}},
		{"-F", `curl -F name=report -F file=@out.csv https://x.io/up`, func(r *Request) bool {
			return r.Method == "POST" && len(r.Form) == 2 && r.Form[1] == Field{"file", "out.csv", true} && len(r.Notes) == 1
		}},
		{"-I and -m", `curl -I -m 2.5 https://x.io`, func(r *Request) bool { return r.Method == "HEAD" && r.TimeoutMS == 2500 }},
		{"-b and -A", `curl -b 'sid=1; t=2' -A codec/3 --referer https://from https://x.io`, func(r *Request) bool {
			return reflect.DeepEqual(r.Headers, []Header{{"Cookie", "sid=1; t=2"}, {"User-Agent", "codec/3"}, {"Referer", "https://from"}})
		}},
		{"@file noted", `curl -d @body.json https://x.io`, func(r *Request) bool {
			return r.Body == "" && strings.Contains(r.Notes[0], "body.json")
		}},
		{"windows cmd", "curl.exe ^\n -X DELETE ^\n https://x.io/1", func(r *Request) bool { return r.Method == "DELETE" }},
		{"empty header", `curl -H 'X-Empty;' -H 'X-Gone:' https://x.io`, func(r *Request) bool {
			return reflect.DeepEqual(r.Headers, []Header{{"X-Empty", ""}})
		}},
		{"unknown flag noted", `curl --frobnicate https://x.io`, func(r *Request) bool { return strings.Contains(r.Notes[0], "--frobnicate") }},
		{"--url=", `curl --url=https://x.io/a -X patch`, func(r *Request) bool { return r.URL == "https://x.io/a" && r.Method == "PATCH" }},
	} {
		r, err := ParseCurl(tt.in)
		if err != nil || !tt.check(r) {
			t.Errorf("%s: %+v (%v)", tt.name, r, err)
		}
	}
	for _, bad := range []string{"", "wget https://x.io", "curl -X", "curl -H 'a: b'", "curl 'x"} {
		if _, err := ParseCurl(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestFetch(t *testing.T) {
	r, _ := ParseCurl(`curl https://x.io/o -H 'Content-Type: application/json' -d '{"a":[1,2],"b":"é"}' -m 10`)
	want := `const response = await fetch("https://x.io/o", {
  method: "POST",
  headers: {
    "Content-Type": "application/json",
  },
  body: JSON.stringify({
    "a": [
      1,
      2
    ],
    "b": "é"
  }),
  signal: AbortSignal.timeout(10000),
});
`
	if got := r.Fetch(); got != want {
		t.Errorf("fetch:\n%s\nwant\n%s", got, want)
	}
	if got := (&Request{Method: "GET", URL: "https://x.io"}).Fetch(); got != "const response = await fetch(\"https://x.io\");\n" {
		t.Errorf("plain GET: %s", got)
	}
	r, _ = ParseCurl(`curl -F a=1 -F f=@x.png https://x.io`)
	if got := r.Fetch(); !strings.Contains(got, `form.append("a", "1");`) || !strings.Contains(got, `form.append("f", fileInput.files[0], "x.png")`) || !strings.Contains(got, "body: form") {
		t.Errorf("form:\n%s", got)
	}
}

func TestParseFetch(t *testing.T) {
	r, err := ParseFetch(`
// from the docs
const res = await fetch('https://api.example.com/items', {
  method: 'post', // lower case is fine
  headers: new Headers({ 'Content-Type': "application/json", Authorization: ` + "`Bearer t`" + ` }),
  body: JSON.stringify({ name: "a \"b\"", tags: ['x', 'y'], n: 1.5e3, ok: true, none: null }),
  credentials: 'include',
  signal: AbortSignal.timeout(5000),
  mode: 'cors',
});`)
	want := &Request{Method: "POST", URL: "https://api.example.com/items", Headers: []Header{{"Content-Type", "application/json"}, {"Authorization", "Bearer t"}},
		Body: `{"name":"a \"b\"","tags":["x","y"],"n":1.5e3,"ok":true,"none":null}`, TimeoutMS: 5000,
		Notes: []string{`credentials: "include" sends the page's cookies; add them with -b.`}}
	if err != nil || !reflect.DeepEqual(r, want) {
		t.Fatalf("fetch:\n got %+v (%v)\nwant %+v", r, err, want)
	}
	r, _ = ParseFetch("fetch(url, { body: payload, headers: { 'X-Id': id } })")
	if r != nil {
		t.Errorf("a variable URL accepted: %+v", r)
	}
	r, err = ParseFetch("fetch('https://x.io', { method: 'PUT', body: payload, headers: { 'X-Id': id } })")
	if err != nil || r.Body != "" || len(r.Notes) != 2 || r.Headers[0] != (Header{"X-Id", "id"}) {
		t.Errorf("variables: %+v (%v)", r, err)
	}
	r, err = ParseFetch("const form = new FormData();\nform.append('a', '1');\nform.append('f', input.files[0]);\nfetch('https://x.io', { method: 'POST', body: form })")
	if err != nil || len(r.Form) != 2 || r.Form[0] != (Field{"a", "1", false}) || !r.Form[1].File {
		t.Errorf("form: %+v (%v)", r, err)
	}
	for _, bad := range []string{"", "axios.get('x')", "fetch(", "fetch('x', {", "fetch('x', 'y')"} {
		if _, err := ParseFetch(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestCurl(t *testing.T) {
	r := &Request{Method: "POST", URL: "https://x.io/a?b=c", Headers: []Header{{"Content-Type", "application/json"}, {"X-Quote", "it's"}}, Body: `{"a":1}`, TimeoutMS: 1500}
	want := "curl 'https://x.io/a?b=c' \\\n  -H 'Content-Type: application/json' \\\n  -H 'X-Quote: it'\\''s' \\\n  --data-raw '{\"a\":1}' \\\n  --max-time 1.5"
	if got := r.Curl(); got != want {
		t.Errorf("curl:\n%s\nwant\n%s", got, want)
	}
	if got := (&Request{Method: "DELETE", URL: "https://x.io/1"}).Curl(); got != "curl 'https://x.io/1' \\\n  -X 'DELETE'" {
		t.Errorf("delete: %s", got)
	}
}

func TestStatuses(t *testing.T) {
	if s := Statuses("404"); len(s) != 1 || s[0].Name != "Not Found" {
		t.Errorf("404: %+v", s)
	}
	if s := Statuses("5"); len(s) < 10 || s[0].Code != 500 {
		t.Errorf("5xx: %d", len(s))
	}
	if s := Statuses("rate limit"); len(s) != 1 || s[0].Code != 429 {
		t.Errorf("rate limit: %+v", s)
	}
	if s := Statuses("gateway"); len(s) < 2 {
		t.Errorf("gateway: %+v", s)
	}
	if len(Statuses("")) != len(statuses) || len(Statuses("nothing like this")) != 0 {
		t.Error("all or none")
	}
	seen := map[int]bool{}
	for _, s := range statuses {
		if seen[s.Code] || s.Name == "" || s.Meaning == "" || s.Retry == "" {
			t.Errorf("entry %d", s.Code)
		}
		seen[s.Code] = true
	}
}
