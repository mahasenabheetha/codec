package httpkit

import (
	"encoding/json"
	"reflect"
	"testing"
)

// same reports whether two requests send the same thing: JSON bodies
// may be spaced differently.
func same(a, b *Request) bool {
	if a.Method != b.Method || a.URL != b.URL || a.TimeoutMS != b.TimeoutMS || !reflect.DeepEqual(a.Headers, b.Headers) || !reflect.DeepEqual(a.Form, b.Form) {
		return false
	}
	if a.Body == b.Body {
		return true
	}
	var x, y any
	return json.Unmarshal([]byte(a.Body), &x) == nil && json.Unmarshal([]byte(b.Body), &y) == nil && reflect.DeepEqual(x, y)
}

// FuzzCurl: a command curl reads comes back the same through curl and
// through fetch.
func FuzzCurl(f *testing.F) {
	for _, s := range []string{
		`curl 'https://x.io/a?b=1' -H 'content-type: application/json' --data-raw '{"a":[1,"é"]}' --compressed`,
		"curl -sSL -XPUT -u a:b -b 'k=v' -A ua -e ref -m 3 https://x.io",
		`curl -G --data-urlencode 'q=a b' -d x=1 x.io`,
		"curl -F a=1 -F f=@x.png --form-string 'c=@d' https://x.io",
		`curl $'https://x.io/\x41' -H $'X: é\n' -I`,
		"curl.exe ^\n -X DELETE ^\n https://x.io/1",
		`curl --json '{"a":1}' -H 'X-E;' https://x.io | jq .`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, cmd string) {
		r, err := ParseCurl(cmd)
		if err != nil {
			return
		}
		back, err := ParseCurl(r.Curl())
		if err != nil || !same(r, back) {
			t.Fatalf("%q\nread as %+v\nits curl\n%s\nread back as %+v (%v)", cmd, r, r.Curl(), back, err)
		}
		js, err := ParseFetch(r.Fetch())
		if err != nil || !same(r, js) {
			t.Fatalf("%q\nread as %+v\nits fetch\n%s\nread back as %+v (%v)", cmd, r, r.Fetch(), js, err)
		}
	})
}

// FuzzFetch: any text either fails or gives a request, never a panic.
func FuzzFetch(f *testing.F) {
	for _, s := range []string{
		"fetch('https://x.io', { method: 'POST', headers: { a: 'b' }, body: JSON.stringify({ x: [1, 2, { y: null }] }) })",
		"fetch(`https://x.io/${id}`)",
		"const form = new FormData(); form.append('a', '1'); fetch('u', { body: form })",
		"fetch('\\u{1F600}\\x41\\ud83d\\ude00', { signal: AbortSignal.timeout(9) /* c */ })",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, code string) {
		r, err := ParseFetch(code)
		if err == nil {
			r.Curl()
			r.Fetch()
		}
	})
}
