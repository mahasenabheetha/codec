package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mahasenabheetha/codec/v2/internal/encode"
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
	if code, out := post("sha", `{"count":3,"form":"short"}`); code != 200 || len(out["shas"].([]any)) != 3 || len(out["shas"].([]any)[0].(string)) != 7 {
		t.Errorf("sha short: %d %v", code, out)
	}
	if code, _ := post("sha", `{"form":"md5"}`); code != http.StatusUnprocessableEntity {
		t.Errorf("sha unknown form: %d, want 422", code)
	}
	if code, out := post("uuid", `{"count":5000}`); code != 200 || len(out["uuids"].([]any)) != encode.MaxUUIDs {
		t.Errorf("uuid count capped: %d", len(out["uuids"].([]any)))
	}
	if code, out := post("uuid", `{"uuid":{"version":5,"namespace":"dns","name":"www.example.com"}}`); code != 200 || out["uuids"].([]any)[0] != "2ed6657d-e927-568b-95e1-2665a8aea6a2" {
		t.Errorf("uuid v5: %d %v", code, out)
	}
	if code, out := post("uuid", `{"op":"parse","input":"017F22E2-79B0-7CC3-98C4-DC0C0C07398F"}`); code != 200 || out["version"] != 7.0 || out["time"] == nil {
		t.Errorf("uuid parse: %d %v", code, out)
	}
	if code, _ := post("uuid", `{"op":"parse","input":"x"}`); code != 422 {
		t.Errorf("uuid parse garbage: %d", code)
	}
	if code, out := post("password", `{"count":2,"rules":{"length":12,"minLower":1,"minUpper":1,"minDigits":1,"minSymbols":-1}}`); code != 200 || len(out["passwords"].([]any)) != 2 {
		t.Errorf("password: %d %v", code, out)
	}
	if code, out := post("password", `{"op":"passphrase","phrase":{"words":5,"separator":"-"}}`); code != 200 || out["passwords"].([]any)[0].(map[string]any)["bits"] != 64.0 {
		t.Errorf("passphrase: %d %v", code, out)
	}
	if code, _ := post("password", `{"rules":{"length":3,"minDigits":4}}`); code != 422 {
		t.Errorf("impossible rules: %d", code)
	}
	if code, out := post("expected", `{"input":"900150983cd24fb0d6963f7d28e17f72  a.txt"}`); code != 200 || out["expected"].([]any)[0].(map[string]any)["algorithm"] != "MD5" {
		t.Errorf("expected: %d %v", code, out)
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

func TestChecksumAPI(t *testing.T) {
	s := New(testOptions(false))
	defer s.Close()
	digest := func(rec *httptest.ResponseRecorder) (string, float64) {
		var out struct {
			Digests []encode.Digest `json:"digests"`
			Size    float64         `json:"size"`
		}
		json.Unmarshal(rec.Body.Bytes(), &out)
		if len(out.Digests) == 0 {
			return "", 0
		}
		return out.Digests[0].Hex, out.Size
	}
	const abc = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	rec := call(s, "POST", "/api/v2/checksum", "127.0.0.1:8765", s.Token(), "", "abc")
	if h, n := digest(rec); rec.Code != 200 || h != abc || n != 3 {
		t.Errorf("upload: %d %s", rec.Code, rec.Body)
	}
	if rec := call(s, "GET", "/api/v2/checksum?path=a.bin", "127.0.0.1:8765", s.Token(), "", ""); rec.Code != http.StatusConflict {
		t.Errorf("no folder: %d", rec.Code)
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "a.bin"), []byte("abc"), 0o644)
	if err := s.OpenWorkspace(root); err != nil {
		t.Fatal(err)
	}
	rec = call(s, "GET", "/api/v2/checksum?path=a.bin", "127.0.0.1:8765", s.Token(), "", "")
	if h, _ := digest(rec); rec.Code != 200 || h != abc {
		t.Errorf("file: %d %s", rec.Code, rec.Body)
	}
	for _, p := range []string{"../a.bin", "missing.bin", ""} {
		if rec := call(s, "GET", "/api/v2/checksum?path="+p, "127.0.0.1:8765", s.Token(), "", ""); rec.Code == 200 {
			t.Errorf("%q: 200", p)
		}
	}
}
