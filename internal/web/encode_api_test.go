package web

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEncodeAPI(t *testing.T) {
	s := New(testOptions(false))
	defer s.Close()
	post := func(kind, body string) (int, map[string]any) {
		rec := call(s, "POST", "/api/v2/encode/"+kind, "127.0.0.1:8765", s.Token(), "", body)
		var out map[string]any
		json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	if code, out := post("url", `{"op":"encode","input":"a b"}`); code != 200 || out["output"] != "a%20b" {
		t.Errorf("url encode: %d %v", code, out)
	}
	if code, out := post("url", `{"op":"decode","input":"%zz"}`); code != 422 || !strings.Contains(out["error"].(string), "invalid escape") {
		t.Errorf("url decode error: %d %v", code, out)
	}
	if code, out := post("hex", `{"op":"decode","input":"ff00"}`); code != 200 || out["text"] != false || out["bytes"] != 2.0 {
		t.Errorf("hex binary: %d %v", code, out)
	}
	if code, out := post("hash", `{"input":"abc","key":"","algorithms":["sha256"]}`); code != 200 || out["hmac"] != true || len(out["digests"].([]any)) != 1 {
		t.Errorf("hmac with an empty key: %d %v", code, out)
	}
	// Secret options are nested, so "upper" there isn't the hex flag.
	code, out := post("secret", `{"count":3,"secret":{"length":12,"upper":true}}`)
	if code != 200 || len(out["secrets"].([]any)) != 3 {
		t.Fatalf("secrets: %d %v", code, out)
	}
	if v := out["secrets"].([]any)[0].(map[string]any)["value"].(string); strings.ToUpper(v) != v {
		t.Errorf("upper-case only: %q", v)
	}
	if code, out := post("uuid", `{"count":1000}`); code != 200 || len(out["uuids"].([]any)) != maxCount {
		t.Errorf("uuid count capped: %d", len(out["uuids"].([]any)))
	}
	code, out = post("htpasswd", `{"op":"make","user":"u","password":"p","cost":4}`)
	if code != 200 {
		t.Fatalf("htpasswd: %d %v", code, out)
	}
	line, _ := json.Marshal(out["line"])
	if code, out := post("htpasswd", `{"op":"check","password":"p","line":`+string(line)+`}`); code != 200 || out["match"] != true {
		t.Errorf("check: %d %v", code, out)
	}
	if code, _ := post("rot13", `{}`); code != 400 {
		t.Errorf("unknown kind: %d", code)
	}
}
