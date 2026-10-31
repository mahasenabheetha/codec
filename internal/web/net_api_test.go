package web

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestNetAPI(t *testing.T) {
	s := New(testOptions(false))
	defer s.Close()
	post := func(kind, body string) (int, map[string]any) {
		rec := call(s, "POST", "/api/v2/net/"+kind, "127.0.0.1:8765", s.Token(), "", body)
		var out map[string]any
		json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	if code, out := post("cidr", `{"input":"10.0.0.0/24"}`); code != 200 || out["range"].(map[string]any)["hosts"] != "254" {
		t.Errorf("cidr: %d %v", code, out)
	}
	if code, out := post("cidr", `{"input":"10.0.0.0/24","op":"split","prefix":25}`); code != 200 || len(out["subnets"].([]any)) != 2 {
		t.Errorf("split: %d %v", code, out)
	}
	if code, out := post("cidr", `{"input":"10.0.0.0/24","op":"contains","ip":"10.0.0.9"}`); code != 200 || out["contains"] != true {
		t.Errorf("contains: %d %v", code, out)
	}
	if code, out := post("overlaps", `{"lines":["10.0.0.0/16","10.0.1.0/24"]}`); code != 200 || len(out["overlaps"].([]any)) != 1 {
		t.Errorf("overlaps: %d %v", code, out)
	}
	if code, out := post("ip", `{"input":"167772161"}`); code != 200 || out["ip"].(map[string]any)["address"] != "10.0.0.1" {
		t.Errorf("ip: %d %v", code, out)
	}
	if code, out := post("url", `{"op":"build","parts":{"scheme":"https","host":"x.io","query":[{"key":"q","value":"a b"}]}}`); code != 200 || out["url"] != "https://x.io?q=a+b" {
		t.Errorf("url build: %d %v", code, out)
	}
	if code, out := post("url", `{"op":"parse","input":"https://x.io/a?q=1"}`); code != 200 || out["parts"].(map[string]any)["host"] != "x.io" {
		t.Errorf("url parse: %d %v", code, out)
	}
	for _, tt := range []struct {
		kind, body string
		code       int
	}{
		{"cidr", `{"input":"10.0.0.0/40"}`, http.StatusUnprocessableEntity},
		{"ip", `{"input":"1.2.3"}`, http.StatusUnprocessableEntity},
		{"cidr", `{"input":"10.0.0.0/24","op":"divide"}`, http.StatusNotFound},
	} {
		if code, _ := post(tt.kind, tt.body); code != tt.code {
			t.Errorf("%s %s: %d, want %d", tt.kind, tt.body, code, tt.code)
		}
	}
}
